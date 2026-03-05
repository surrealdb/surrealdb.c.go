package cgo

// #include "surrealdb.h"
import "C"

import (
	"context"
	"sync"
)

const defaultMaxPendingConnects = 16

var (
	connectSemOnce sync.Once
	connectSem     chan struct{}
)

func getConnectSem() chan struct{} {
	connectSemOnce.Do(func() {
		connectSem = make(chan struct{}, defaultMaxPendingConnects)
	})
	return connectSem
}

// Connect opens a connection to a SurrealDB instance.
func Connect(ctx context.Context, endpoint string, cfg Config) (*Conn, error) {
	sem := getConnectSem()

	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	type connectResult struct {
		surreal *C.sr_surreal_t
		err     error
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
		<-sem
		if res.err != nil {
			return nil, res.err
		}
		maxOps := cfg.MaxPendingOps
		if maxOps <= 0 {
			maxOps = DefaultConfig().MaxPendingOps
		}
		return &Conn{
			c:    res.surreal,
			live: newLiveQueryMap(),
			sem:  make(chan struct{}, maxOps),
		}, nil

	case <-ctx.Done():
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
