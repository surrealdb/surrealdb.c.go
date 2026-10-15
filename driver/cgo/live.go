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
	"time"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

// readerExitTimeout bounds how long Close waits for stream readers to exit
// after the connection has been freed.
var readerExitTimeout = 10 * time.Second

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
//
// Each live query has two goroutines. The reader sits in sr_stream_next,
// which blocks until the next notification and cannot be interrupted:
// surrealdb.c has no way to wake it, and in the embedded engine a KILL does
// not reach the stream either. The forwarder owns Ch and hands notifications
// from the reader to the caller, so stopping can close Ch at once.
//
// After Kill, the reader stays parked until the live query's next event (it
// then drops the event and frees the stream, which also kills the live query
// in the engine) or until Close frees the connection, which ends every
// stream.
type LiveHandle struct {
	// ID is the server's live query id when it could be found at creation
	// time (see Conn.Live), otherwise a random id local to this connection.
	ID string
	Ch <-chan driver.Notification

	queryID  atomic.Pointer[string]
	stopOnce sync.Once
	stop     chan struct{} // closed once stopping has begun
	closed   chan struct{} // closed once Ch has been closed
	done     chan struct{} // closed once the reader has freed the stream
}

func (lq *LiveHandle) setQueryID(id string) {
	lq.queryID.CompareAndSwap(nil, &id)
}

// beginStop stops delivery: Ch is closed and no more notifications are
// handed to the caller.
func (lq *LiveHandle) beginStop() {
	lq.stopOnce.Do(func() { close(lq.stop) })
}

func uuidToString(b [16]C.uint8_t) string {
	var raw [16]byte
	for i := range raw {
		raw[i] = byte(b[i])
	}
	return formatUUID(raw)
}

func formatUUID(raw [16]byte) string {
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
	return formatUUID(b)
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

func (m *liveQueryMap) remove(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

// find returns the handle whose local or server id is id.
func (m *liveQueryMap) find(id string) *LiveHandle {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lq, ok := m.sessions[id]; ok {
		return lq
	}
	for _, lq := range m.sessions {
		if qid := lq.queryID.Load(); qid != nil && *qid == id {
			return lq
		}
	}
	return nil
}

func (m *liveQueryMap) all() []*LiveHandle {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*LiveHandle, 0, len(m.sessions))
	for _, lq := range m.sessions {
		out = append(out, lq)
	}
	return out
}

// stopAll stops delivery on every live query and returns their handles.
func (m *liveQueryMap) stopAll() []*LiveHandle {
	handles := m.all()
	for _, lq := range handles {
		lq.beginStop()
		<-lq.closed
	}
	return handles
}

// rawKill runs KILL for a live query id. The caller must hold c.mu.
func (c *Conn) rawKill(id string) error {
	cID := cstring(id)
	defer freeCString(cID)
	var errPtr C.sr_string_t
	rc := C.sr_kill(c.c, &errPtr, cID)
	return surrealError(rc, errPtr)
}

// tableOf returns the table a live query resource refers to. surrealdb.c
// treats "table:key" as a record id and anything else as a table.
func tableOf(resource string) string {
	if tb, key, ok := strings.Cut(resource, ":"); ok && tb != "" && key != "" {
		return tb
	}
	return resource
}

// quoteIdent escapes a table name for use as a SurrealQL identifier.
func quoteIdent(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "`", "\\`")
	return "`" + s + "`"
}

// tableLives returns the ids of the live queries registered on a table.
// The caller must hold c.mu.
func (c *Conn) tableLives(tb string) (map[string]struct{}, error) {
	res, err := c.rawQuery("INFO FOR TABLE "+quoteIdent(tb), nil)
	if err != nil {
		return nil, err
	}
	if len(res) != 1 || res[0].Err != nil {
		return nil, fmt.Errorf("INFO FOR TABLE failed")
	}
	v := res[0].Value
	if arr, ok := v.([]any); ok && len(arr) == 1 {
		v = arr[0]
	}
	info, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected INFO FOR TABLE result %T", v)
	}
	lives, ok := info["lives"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("INFO FOR TABLE has no lives")
	}
	out := make(map[string]struct{}, len(lives))
	for id := range lives {
		out[id] = struct{}{}
	}
	return out, nil
}

// Live creates a live query on the given resource.
func (c *Conn) Live(ctx context.Context, resource string) (*LiveHandle, error) {
	return run(ctx, c, func() (*LiveHandle, error) {
		// sr_select_live does not return the live query id. Creating live
		// queries one at a time per connection, the live query that
		// appears on the table is the one just created. If that cannot be
		// determined, the id is taken from the first notification instead.
		c.liveMu.Lock()
		defer c.liveMu.Unlock()

		tb := tableOf(resource)
		before, beforeErr := c.tableLives(tb)

		cRes := cstring(resource)
		defer freeCString(cRes)

		var errPtr C.sr_string_t
		var streamPtr *C.sr_stream_t
		rc := C.sr_select_live(c.c, &errPtr, &streamPtr, cRes)
		if err := surrealError(rc, errPtr); err != nil {
			return nil, err
		}

		ch := make(chan driver.Notification, 64)
		lq := &LiveHandle{
			Ch:     ch,
			stop:   make(chan struct{}),
			closed: make(chan struct{}),
			done:   make(chan struct{}),
		}

		if beforeErr == nil {
			if after, err := c.tableLives(tb); err == nil {
				var added []string
				for id := range after {
					if _, ok := before[id]; !ok {
						added = append(added, id)
					}
				}
				if len(added) == 1 {
					lq.setQueryID(added[0])
				}
			}
		}
		if id := lq.queryID.Load(); id != nil {
			lq.ID = *id
		} else {
			lq.ID = generateID()
		}

		c.live.add(lq)

		handoff := make(chan driver.Notification)

		// Reader: the only goroutine that touches the stream. The handle
		// stays in c.live until the stream is freed.
		go func() {
			defer close(lq.done)
			defer c.live.remove(lq.ID)
			defer C.sr_stream_kill(streamPtr)
			defer close(handoff)

			for {
				var cn C.sr_notification_t
				if rc := C.sr_stream_next(streamPtr, &cn); rc <= 0 {
					return
				}
				n := extractNotification(&cn)
				lq.setQueryID(n.QueryID)
				select {
				case handoff <- n:
				case <-lq.stop:
					return
				}
			}
		}()

		// Forwarder: the only goroutine that sends on or closes Ch.
		go func() {
			defer close(lq.closed)
			defer close(ch)
			for {
				select {
				case n, ok := <-handoff:
					if !ok {
						return
					}
					select {
					case ch <- n:
					case <-lq.stop:
						return
					}
				case <-lq.stop:
					return
				}
			}
		}()

		return lq, nil
	}, c.Kill)
}

// Kill stops a live query session. When it returns, the session's channel
// has been closed and no further notifications are delivered. It writes
// nothing to the database.
func (c *Conn) Kill(lq *LiveHandle) {
	lq.beginStop()
	<-lq.closed
}

// KillID stops a live query by its id. If the id belongs to a live query
// started on this connection, it behaves like Kill. Otherwise the KILL is
// sent to the server as is.
func (c *Conn) KillID(ctx context.Context, id string) error {
	if lq := c.live.find(id); lq != nil {
		c.Kill(lq)
		return nil
	}
	_, err := run(ctx, c, func() (struct{}, error) {
		return struct{}{}, c.rawKill(id)
	}, nil)
	return err
}
