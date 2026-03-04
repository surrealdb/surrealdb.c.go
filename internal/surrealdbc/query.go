package surrealdbc

// #include "surrealdb.h"
import "C"

import (
	"context"
	"fmt"
)

// Query executes a SurrealQL query, automatically marshaling vars.
//
// vars may be nil, a struct (with `cbor:"..."` tags), or map[string]any.
func (c *Conn) Query(ctx context.Context, sql string, vars any) ([]QueryResult, error) {
	obj, err := marshalVars(vars)
	if err != nil {
		return nil, fmt.Errorf("marshal query vars: %w", err)
	}

	return runWithSem(ctx, c.sem, func() ([]QueryResult, error) {
		if obj != nil {
			defer obj.Free()
		}
		return c.rawQuery(sql, obj)
	}, nil)
}

func (c *Conn) rawQuery(sql string, vars *object) ([]QueryResult, error) {
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
