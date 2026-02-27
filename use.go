package surrealdb

// #include "surrealdb.h"
import "C"

import "context"

// Use selects the namespace and database for subsequent operations.
// Both are created automatically on first write.
//
// The context controls the timeout. If ctx is cancelled while the C calls
// are in progress, the goroutine continues to completion in the background
// (bounded by DefaultMaxPendingOps per DB).
func (db *DB) Use(ctx context.Context, ns, database string) error {
	_, err := runWithSem(ctx, db.sem, func() (struct{}, error) {
		cNS := cstring(ns)
		defer freeCString(cNS)

		var errPtr C.sr_string_t
		rc := C.sr_use_ns(db.c, &errPtr, cNS)
		if err := surrealError(rc, errPtr); err != nil {
			return struct{}{}, err
		}

		cDB := cstring(database)
		defer freeCString(cDB)

		rc = C.sr_use_db(db.c, &errPtr, cDB)
		return struct{}{}, surrealError(rc, errPtr)
	}, nil)
	return err
}
