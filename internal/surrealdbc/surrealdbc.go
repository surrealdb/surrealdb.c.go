// Package surrealdbc provides the CGO binding layer for the SurrealDB C library.
// This is an internal package — use the parent surrealdb package instead.
package surrealdbc

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
	"unsafe"
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
// It wraps the C sr_surreal_t pointer and is safe to use from multiple goroutines.
type Conn struct {
	c    *C.sr_surreal_t
	live *liveQueryMap
	sem  chan struct{}
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

// Version returns the SurrealDB engine version string.
func (c *Conn) Version() (string, error) {
	var errPtr C.sr_string_t
	var ver C.sr_string_t
	rc := C.sr_version(c.c, &errPtr, &ver)
	if err := surrealError(rc, errPtr); err != nil {
		return "", err
	}
	s := C.GoString(ver)
	C.sr_free_string(ver)
	return s, nil
}

// Health checks whether the database connection is healthy.
func (c *Conn) Health() error {
	var errPtr C.sr_string_t
	rc := C.sr_health(c.c, &errPtr)
	return surrealError(rc, errPtr)
}

// Close disconnects from the database and releases all resources.
func (c *Conn) Close() {
	if c.c != nil {
		c.live.closeAll(c)
		C.sr_surreal_disconnect(c.c)
		c.c = nil
	}
}

// Use selects the namespace and database for subsequent operations.
func (c *Conn) Use(ctx context.Context, ns, database string) error {
	_, err := runWithSem(ctx, c.sem, func() (struct{}, error) {
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
