package surrealdb

// #include "surrealdb.h"
import "C"

import "fmt"

// Query executes a SurrealQL query and returns results for each statement.
//
// The sql string may contain multiple semicolon-separated statements.
// Each statement produces one QueryResult in the returned slice.
//
// vars is automatically marshaled to a SurrealDB object:
//   - nil: no parameters
//   - *Object: passed directly (caller manages its lifetime)
//   - struct or *struct: marshaled via MarshalObject using `db:"..."` tags
//   - map[string]any: marshaled via MarshalMap
//
// Example with a struct:
//
//	results, err := db.Query("SELECT * FROM person WHERE age >= $min_age", struct {
//	    MinAge int `db:"min_age"`
//	}{MinAge: 18})
func (db *DB) Query(sql string, vars any) ([]QueryResult, error) {
	switch v := vars.(type) {
	case nil:
		return db.query(sql, nil)
	case *Object:
		return db.query(sql, v)
	case map[string]any:
		obj, err := MarshalMap(v)
		if err != nil {
			return nil, fmt.Errorf("marshal query vars: %w", err)
		}
		defer obj.Free()
		return db.query(sql, obj)
	default:
		obj, err := MarshalObject(v)
		if err != nil {
			return nil, fmt.Errorf("marshal query vars: %w", err)
		}
		defer obj.Free()
		return db.query(sql, obj)
	}
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
