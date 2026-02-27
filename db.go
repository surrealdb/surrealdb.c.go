package surrealdb

/*
#cgo LDFLAGS: -Wl,-Bstatic -lsurrealdb_c -Wl,-Bdynamic -lpthread -ldl -lm -lrt -lstdc++
#include <stdlib.h>
#include "surrealdb.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// DB is a handle to an open SurrealDB connection.
// It wraps the C sr_surreal_t pointer and is safe to use from multiple goroutines.
// Call Close when done to release the underlying resources.
type DB struct {
	c    *C.sr_surreal_t
	live *liveQueryMap
}

// surrealError converts a C return code and error string into a Go error.
// If rc >= 0 the call succeeded and nil is returned.
// The C error string is freed if non-nil.
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

// cstring allocates a null-terminated C string from a Go string.
// The caller must free the result with C.free.
func cstring(s string) *C.char {
	return C.CString(s)
}

// freeCString frees a C string allocated by cstring.
func freeCString(p *C.char) {
	C.free(unsafe.Pointer(p))
}

// Version returns the SurrealDB engine version string.
func (db *DB) Version() (string, error) {
	var errPtr C.sr_string_t
	var ver C.sr_string_t
	rc := C.sr_version(db.c, &errPtr, &ver)
	if err := surrealError(rc, errPtr); err != nil {
		return "", err
	}
	s := C.GoString(ver)
	C.sr_free_string(ver)
	return s, nil
}

// Health checks whether the database connection is healthy.
func (db *DB) Health() error {
	var errPtr C.sr_string_t
	rc := C.sr_health(db.c, &errPtr)
	return surrealError(rc, errPtr)
}
