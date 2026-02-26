package surrealdb

// #include "surrealdb.h"
import "C"

// Open connects to a SurrealDB instance at the given endpoint.
//
// Supported endpoints:
//   - "mem://"                   — in-memory embedded database
//   - "surrealkv://path/to/file" — file-backed embedded database
//   - "ws://host:port"           — remote WebSocket connection
//   - "http://host:port"         — remote HTTP connection
//
// The caller must call Close on the returned DB when done.
func Open(endpoint string) (*DB, error) {
	cEndpoint := cstring(endpoint)
	defer freeCString(cEndpoint)

	var errPtr C.sr_string_t
	var surreal *C.sr_surreal_t

	rc := C.sr_connect(&errPtr, &surreal, cEndpoint)
	if err := surrealError(rc, errPtr); err != nil {
		return nil, err
	}
	return &DB{c: surreal}, nil
}
