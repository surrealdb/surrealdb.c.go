package surrealdb

// #include "surrealdb.h"
import "C"

// Use selects the namespace and database for subsequent operations.
// Both are created automatically on first write.
func (db *DB) Use(ns, database string) error {
	cNS := cstring(ns)
	defer freeCString(cNS)

	var errPtr C.sr_string_t
	rc := C.sr_use_ns(db.c, &errPtr, cNS)
	if err := surrealError(rc, errPtr); err != nil {
		return err
	}

	cDB := cstring(database)
	defer freeCString(cDB)

	rc = C.sr_use_db(db.c, &errPtr, cDB)
	return surrealError(rc, errPtr)
}
