package surrealdb

// #include "surrealdb.h"
import "C"

// Close disconnects from the database and releases all resources.
// The DB must not be used after Close is called.
func (db *DB) Close() {
	if db.c != nil {
		C.sr_surreal_disconnect(db.c)
		db.c = nil
	}
}
