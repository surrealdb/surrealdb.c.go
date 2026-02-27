package surrealdb

// #include "surrealdb.h"
import "C"

import (
	"context"
	"fmt"
)

// Query executes a SurrealQL query and returns results for each statement.
//
// The sql string may contain multiple semicolon-separated statements.
// Each statement produces one QueryResult in the returned slice.
//
// The context controls the query timeout. If ctx is cancelled while the
// C call is in progress, the goroutine continues to completion in the
// background (bounded by DefaultMaxPendingOps per DB).
//
// vars is automatically marshaled to a SurrealDB object:
//   - nil: no parameters
//   - *Object: passed directly (caller manages its lifetime)
//   - struct or *struct: marshaled via MarshalObject using `db:"..."` tags
//   - map[string]any: marshaled via MarshalMap
//
// Example with a struct:
//
//	results, err := db.Query(ctx, "SELECT * FROM person WHERE age >= $min_age", struct {
//	    MinAge int `db:"min_age"`
//	}{MinAge: 18})
func (db *DB) Query(ctx context.Context, sql string, vars any) ([]QueryResult, error) {
	// Marshal vars before entering the semaphore-gated goroutine,
	// so marshaling errors are returned immediately.
	var obj *Object
	var freeObj bool
	switch v := vars.(type) {
	case nil:
		// no vars
	case *Object:
		obj = v
	case map[string]any:
		o, err := MarshalMap(v)
		if err != nil {
			return nil, fmt.Errorf("marshal query vars: %w", err)
		}
		obj = o
		freeObj = true
	default:
		o, err := MarshalObject(v)
		if err != nil {
			return nil, fmt.Errorf("marshal query vars: %w", err)
		}
		obj = o
		freeObj = true
	}

	return runWithSem(ctx, db.sem, func() ([]QueryResult, error) {
		if freeObj {
			defer obj.Free()
		}
		return db.query(sql, obj)
	}, nil)
}

func (db *DB) query(sql string, vars *Object) ([]QueryResult, error) {
	cSQL := cstring(sql)
	defer freeCString(cSQL)

	var cVars *C.sr_object_t
	if vars != nil {
		cVars = vars.cptr()
	}

	var errPtr C.sr_string_t
	var results *C.sr_arr_res_t

	rc := C.sr_query(db.c, &errPtr, &results, cSQL, cVars)
	if err := surrealError(rc, errPtr); err != nil {
		return nil, err
	}

	numStatements := int(rc)
	out := extractResults(results, numStatements)
	C.sr_free_arr_res_arr(results, rc)
	return out, nil
}
