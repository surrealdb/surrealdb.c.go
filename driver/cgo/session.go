package cgo

// #include "surrealdb.h"
import "C"

import (
	"context"
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

// Let defines a variable on the connection, available as $key in later
// queries.
//
// val is converted before Let can return, so the caller may change it as
// soon as Let returns, even when ctx is cancelled first.
func (c *Conn) Let(ctx context.Context, key string, val any) error {
	cVal, err := toCValue(val)
	if err != nil {
		return fmt.Errorf("marshal value for %q: %w", key, err)
	}
	free := func() { C.sr_value_free(cVal) }
	_, err = runOwning(ctx, c, func() (struct{}, error) {
		defer free()
		cKey := cstring(key)
		defer freeCString(cKey)
		var errPtr C.sr_string_t
		rc := C.sr_set(c.c, &errPtr, cKey, cVal)
		return struct{}{}, surrealError(rc, errPtr)
	}, nil, free)
	return err
}

// Unset removes a variable defined with Let.
func (c *Conn) Unset(ctx context.Context, key string) error {
	_, err := run(ctx, c, func() (struct{}, error) {
		cKey := cstring(key)
		defer freeCString(cKey)
		var errPtr C.sr_string_t
		rc := C.sr_unset(c.c, &errPtr, cKey)
		return struct{}{}, surrealError(rc, errPtr)
	}, nil)
	return err
}

// Health checks that the database is reachable and healthy.
func (c *Conn) Health(ctx context.Context) error {
	_, err := run(ctx, c, func() (struct{}, error) {
		var errPtr C.sr_string_t
		rc := C.sr_health(c.c, &errPtr)
		return struct{}{}, surrealError(rc, errPtr)
	}, nil)
	return err
}

// Version returns the version of the SurrealDB engine.
func (c *Conn) Version(ctx context.Context) (*driver.VersionData, error) {
	return run(ctx, c, func() (*driver.VersionData, error) {
		var errPtr, res C.sr_string_t
		rc := C.sr_version(c.c, &errPtr, &res)
		if err := surrealError(rc, errPtr); err != nil {
			return nil, err
		}
		if res == nil {
			return nil, fmt.Errorf("surrealdb: empty version")
		}
		defer C.sr_free_string(res)
		return &driver.VersionData{Version: strings.TrimPrefix(C.GoString(res), "surrealdb-")}, nil
	}, nil)
}
