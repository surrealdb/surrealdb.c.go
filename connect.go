package surrealdb

// #include "surrealdb.h"
import "C"

import (
	"context"
	"sync"
)

// MaxPendingConnects is the maximum number of goroutines that may be blocked
// inside sr_connect at any time. This bounds the worst-case goroutine leak when
// callers cancel their context before sr_connect returns.
var MaxPendingConnects = 16

var (
	connectSemOnce sync.Once
	connectSem     chan struct{}
)

func getConnectSem() chan struct{} {
	connectSemOnce.Do(func() {
		connectSem = make(chan struct{}, MaxPendingConnects)
	})
	return connectSem
}

type connectResult struct {
	surreal *C.sr_surreal_t
	err     error
}

// Open connects to a SurrealDB instance at the given endpoint.
//
// Supported endpoints:
//   - "mem://"                   — in-memory embedded database
//   - "surrealkv://path/to/file" — file-backed embedded database
//   - "ws://host:port"           — remote WebSocket connection
//   - "http://host:port"         — remote HTTP connection
//
// The context controls the connection timeout. If ctx is cancelled while
// sr_connect is in progress, the goroutine continues to completion in the
// background (at most MaxPendingConnects such goroutines can exist) and
// any successfully opened connection is immediately closed.
//
// The caller must call Close on the returned DB when done.
func Open(ctx context.Context, endpoint string) (*DB, error) {
	sem := getConnectSem()

	// Acquire semaphore slot, respecting context.
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	ch := make(chan connectResult, 1)

	go func() {
		cEndpoint := cstring(endpoint)
		defer freeCString(cEndpoint)

		var errPtr C.sr_string_t
		var surreal *C.sr_surreal_t

		rc := C.sr_connect(&errPtr, &surreal, cEndpoint)
		err := surrealError(rc, errPtr)
		ch <- connectResult{surreal: surreal, err: err}
	}()

	select {
	case res := <-ch:
		<-sem // release semaphore
		if res.err != nil {
			return nil, res.err
		}
		return &DB{c: res.surreal, live: newLiveQueryMap(), sem: make(chan struct{}, DefaultMaxPendingOps)}, nil

	case <-ctx.Done():
		// sr_connect is still running. The goroutine will finish eventually,
		// release the semaphore slot, and clean up the connection if it succeeded.
		go func() {
			res := <-ch
			<-sem
			if res.err == nil {
				C.sr_surreal_disconnect(res.surreal)
			}
		}()
		return nil, ctx.Err()
	}
}
