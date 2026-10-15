package cgo

// #include "surrealdb.h"
import "C"

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

type action int

const (
	actionCreate action = C.SR_ACTION_CREATE
	actionUpdate action = C.SR_ACTION_UPDATE
	actionDelete action = C.SR_ACTION_DELETE
)

func (a action) String() string {
	switch a {
	case actionCreate:
		return "CREATE"
	case actionUpdate:
		return "UPDATE"
	case actionDelete:
		return "DELETE"
	default:
		return "UNIMPLEMENTED"
	}
}

// LiveHandle is a handle to an active live query session.
type LiveHandle struct {
	ID         string
	Ch         <-chan driver.Notification
	stream     *C.sr_stream_t
	resource   string
	done       chan struct{}
	stopping   atomic.Bool
	srvQueryID atomic.Value
}

func uuidToString(b [16]C.uint8_t) string {
	var raw [16]byte
	for i := range raw {
		raw[i] = byte(b[i])
	}
	h := hex.EncodeToString(raw[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

func extractNotification(cn *C.sr_notification_t) driver.Notification {
	return driver.Notification{
		QueryID: uuidToString(cn.query_id._0),
		Action:  action(cn.action).String(),
		Data:    extractAny(&cn.data),
	}
}

func generateID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	h := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

type liveQueryMap struct {
	mu       sync.Mutex
	sessions map[string]*LiveHandle
}

func newLiveQueryMap() *liveQueryMap {
	return &liveQueryMap{sessions: make(map[string]*LiveHandle)}
}

func (m *liveQueryMap) add(lq *LiveHandle) {
	m.mu.Lock()
	m.sessions[lq.ID] = lq
	m.mu.Unlock()
}

func (m *liveQueryMap) remove(id string) (*LiveHandle, bool) {
	m.mu.Lock()
	lq, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	return lq, ok
}

// closeAll stops every live query and waits for their readers to exit.
// The caller must hold c.mu for writing.
func (m *liveQueryMap) closeAll(c *Conn) {
	m.mu.Lock()
	sessions := make([]*LiveHandle, 0, len(m.sessions))
	for _, lq := range m.sessions {
		sessions = append(sessions, lq)
	}
	m.mu.Unlock()

	for _, lq := range sessions {
		if lq.stopping.CompareAndSwap(false, true) {
			sendPoisonPill(c, lq)
		}
	}
	for _, lq := range sessions {
		<-lq.done
		killServerSide(c, lq)
		cleanupPoisonPill(c, lq)
	}
}

func poisonRecordID(lq *LiveHandle) string {
	return fmt.Sprintf("%s:__lq_%s", lq.resource, strings.ReplaceAll(lq.ID, "-", ""))
}

func sendPoisonPill(c *Conn, lq *LiveHandle) {
	rid := poisonRecordID(lq)
	_, _ = c.rawQuery(fmt.Sprintf("CREATE %s SET __lq = true", rid), nil)
}

func cleanupPoisonPill(c *Conn, lq *LiveHandle) {
	rid := poisonRecordID(lq)
	_, _ = c.rawQuery(fmt.Sprintf("DELETE %s", rid), nil)
}

func killServerSide(c *Conn, lq *LiveHandle) {
	if v := lq.srvQueryID.Load(); v != nil {
		id := v.(string)
		cID := cstring(id)
		defer freeCString(cID)
		var errPtr C.sr_string_t
		C.sr_kill(c.c, &errPtr, cID)
		if errPtr != nil {
			C.sr_free_string(errPtr)
		}
	}
}

// Live creates a live query on the given resource.
func (c *Conn) Live(ctx context.Context, resource string) (*LiveHandle, error) {
	return run(ctx, c, func() (*LiveHandle, error) {
		cRes := cstring(resource)
		defer freeCString(cRes)

		var errPtr C.sr_string_t
		var streamPtr *C.sr_stream_t
		rc := C.sr_select_live(c.c, &errPtr, &streamPtr, cRes)
		if err := surrealError(rc, errPtr); err != nil {
			return nil, err
		}

		ch := make(chan driver.Notification, 64)
		done := make(chan struct{})
		lq := &LiveHandle{
			ID:       generateID(),
			Ch:       ch,
			stream:   streamPtr,
			resource: resource,
			done:     done,
		}

		c.live.add(lq)

		go func() {
			// The handle stays in c.live until the stream is freed, so
			// Close never disconnects while this goroutine uses it.
			defer close(done)
			defer c.live.remove(lq.ID)
			defer close(ch)
			defer C.sr_stream_kill(streamPtr)

			for {
				var cn C.sr_notification_t
				rc := C.sr_stream_next(streamPtr, &cn)
				if rc <= 0 {
					return
				}
				if lq.stopping.Load() {
					return
				}
				n := extractNotification(&cn)
				lq.srvQueryID.CompareAndSwap(nil, n.QueryID)
				ch <- n
			}
		}()

		return lq, nil
	}, c.Kill)
}

// Kill stops a live query session and waits for its reader to exit.
func (c *Conn) Kill(lq *LiveHandle) {
	if !lq.stopping.CompareAndSwap(false, true) {
		// Already being stopped by Close or another Kill.
		<-lq.done
		return
	}

	release, err := c.acquire()
	if err != nil {
		// Close has already stopped every live query.
		return
	}
	sendPoisonPill(c, lq)
	release()

	<-lq.done

	if release, err := c.acquire(); err == nil {
		killServerSide(c, lq)
		cleanupPoisonPill(c, lq)
		release()
	}
}
