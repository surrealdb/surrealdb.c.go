package cgo

/*
#cgo CFLAGS: -I${SRCDIR}/../..
#cgo linux LDFLAGS: -Wl,-Bstatic -lsurrealdb_c -Wl,-Bdynamic -lpthread -ldl -lm -lrt -lstdc++
#cgo darwin LDFLAGS: -lsurrealdb_c -framework CoreFoundation -lresolv -lm -lc++
#include <stdlib.h>
#include "surrealdb.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

// Config holds connection configuration.
type Config struct {
	MaxPendingOps int
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		MaxPendingOps: 16,
	}
}

// Conn is an open connection to a SurrealDB instance.
type Conn struct {
	// mu guards c. Every call into the C library that uses c holds mu for
	// reading until the call returns. Close holds it for writing, so it
	// waits for in-flight calls to finish, and later calls see c == nil and
	// fail with driver.ErrClosed instead of using a freed handle.
	mu   sync.RWMutex
	c    *C.sr_surreal_t
	live *liveQueryMap
	sem  chan struct{}
}

// acquire takes a read lock on the connection and returns the function that
// releases it. It fails with driver.ErrClosed once Close has run.
func (c *Conn) acquire() (release func(), err error) {
	c.mu.RLock()
	if c.c == nil {
		c.mu.RUnlock()
		return nil, driver.ErrClosed
	}
	return c.mu.RUnlock, nil
}

func surrealError(rc C.int, errPtr C.sr_string_t) error {
	if rc >= 0 {
		return nil
	}
	if errPtr != nil {
		msg := C.GoString(errPtr)
		C.sr_free_string(errPtr)
		if rc == C.sr_SR_FATAL {
			return fmt.Errorf("surrealdb fatal: %s", msg)
		}
		return errors.New(msg)
	}
	return fmt.Errorf("surrealdb error (code %d)", rc)
}

func cstring(s string) *C.char {
	return C.CString(s)
}

func freeCString(p *C.char) {
	C.free(unsafe.Pointer(p))
}

// Close disconnects from the database and releases all resources.
//
// Close waits for calls already running on other goroutines to return, then
// stops all live queries and frees the connection. Calls made after Close
// return driver.ErrClosed. Close is safe to call more than once and from
// several goroutines.
func (c *Conn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.c == nil {
		return
	}
	c.live.closeAll(c)
	C.sr_surreal_disconnect(c.c)
	c.c = nil
}

// Use selects the namespace and database for subsequent operations.
func (c *Conn) Use(ctx context.Context, ns, database string) error {
	_, err := run(ctx, c, func() (struct{}, error) {
		cNS := cstring(ns)
		defer freeCString(cNS)

		var errPtr C.sr_string_t
		rc := C.sr_use_ns(c.c, &errPtr, cNS)
		if err := surrealError(rc, errPtr); err != nil {
			return struct{}{}, err
		}

		cDB := cstring(database)
		defer freeCString(cDB)

		rc = C.sr_use_db(c.c, &errPtr, cDB)
		return struct{}{}, surrealError(rc, errPtr)
	}, nil)
	return err
}
