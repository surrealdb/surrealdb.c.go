package surrealdbc

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
)

// Action represents the type of change in a live query notification.
type Action int

const (
	ActionCreate        Action = C.SR_ACTION_CREATE
	ActionUpdate        Action = C.SR_ACTION_UPDATE
	ActionDelete        Action = C.SR_ACTION_DELETE
	ActionUnimplemented Action = C.SR_ACTION_UNIMPLEMENTED
)

func (a Action) String() string {
	switch a {
	case ActionCreate:
		return "CREATE"
	case ActionUpdate:
		return "UPDATE"
	case ActionDelete:
		return "DELETE"
	default:
		return "UNIMPLEMENTED"
	}
}

// Notification is a live query event received from the database.
type Notification struct {
	QueryID string
	Action  Action
	Data    Value
}

// LiveHandle is a handle to an active live query session.
type LiveHandle struct {
	ID         string
	Ch         <-chan Notification
	stream     *C.sr_stream_t
	resource   string
	done       chan struct{}
	stopping   atomic.Bool
	srvQueryID atomic.Value // string — server-side query UUID
}

func uuidToString(b [16]C.uint8_t) string {
	var raw [16]byte
	for i := range raw {
		raw[i] = byte(b[i])
	}
	h := hex.EncodeToString(raw[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

func extractNotification(cn *C.sr_notification_t) Notification {
	return Notification{
		QueryID: uuidToString(cn.query_id._0),
		Action:  Action(cn.action),
		Data:    extractValue(&cn.data),
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

// closeAll kills all active live query sessions. Must be called before
// sr_surreal_disconnect so the stream pointers are still valid.
func (m *liveQueryMap) closeAll(c *Conn) {
	m.mu.Lock()
	sessions := make([]*LiveHandle, 0, len(m.sessions))
	for _, lq := range m.sessions {
		sessions = append(sessions, lq)
	}
	m.sessions = make(map[string]*LiveHandle)
	m.mu.Unlock()

	for _, lq := range sessions {
		lq.stopping.Store(true)
		sendPoisonPill(c, lq)
	}
	for _, lq := range sessions {
		<-lq.done
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

// Live creates a live query on the given resource (table name) and returns
// a LiveHandle whose Ch channel receives notifications as they arrive.
func (c *Conn) Live(ctx context.Context, resource string) (*LiveHandle, error) {
	return runWithSem(ctx, c.sem, func() (*LiveHandle, error) {
		cRes := cstring(resource)
		defer freeCString(cRes)

		var errPtr C.sr_string_t
		var streamPtr *C.sr_stream_t
		rc := C.sr_select_live(c.c, &errPtr, &streamPtr, cRes)
		if err := surrealError(rc, errPtr); err != nil {
			return nil, err
		}

		ch := make(chan Notification, 64)
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
			defer close(done)
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
	}, func(lq *LiveHandle) {
		lq.stopping.Store(true)
		sendPoisonPill(c, lq)
		go func() {
			<-lq.done
			killServerSide(c, lq)
			cleanupPoisonPill(c, lq)
			c.live.remove(lq.ID)
		}()
	})
}

// Kill stops a live query session and frees its resources.
func (c *Conn) Kill(lq *LiveHandle) {
	if lq, ok := c.live.remove(lq.ID); ok {
		lq.stopping.Store(true)
		sendPoisonPill(c, lq)
		<-lq.done
		killServerSide(c, lq)
		cleanupPoisonPill(c, lq)
	}
}
