package cgo

// #include "surrealdb.h"
import "C"

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

// Query executes a SurrealQL query, automatically marshaling vars.
func (c *Conn) Query(ctx context.Context, sql string, vars any) ([]driver.QueryResult, error) {
	// Marshal before returning to the caller, who may reuse vars as soon as
	// Query returns, even if the query keeps running in the background.
	obj, err := marshalVars(vars)
	if err != nil {
		return nil, fmt.Errorf("marshal query vars: %w", err)
	}
	free := func() {
		if obj != nil {
			obj.Free()
		}
	}

	return runOwning(ctx, c, func() ([]driver.QueryResult, error) {
		defer free()
		return c.rawQuery(sql, obj)
	}, nil, free)
}

func (c *Conn) rawQuery(sql string, vars *object) ([]driver.QueryResult, error) {
	cSQL := cstring(sql)
	defer freeCString(cSQL)

	var cVars *C.sr_object_t
	if vars != nil {
		cVars = vars.cptr()
	}

	var errPtr C.sr_string_t
	var results *C.sr_arr_res_t

	rc := C.sr_query(c.c, &errPtr, &results, cSQL, cVars)
	if err := surrealError(rc, errPtr); err != nil {
		return nil, err
	}

	numStatements := int(rc)
	out := extractResults(results, numStatements)
	C.sr_free_arr_res_arr(results, rc)
	return out, nil
}
