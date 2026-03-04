package surrealdb

import (
	"context"
	"fmt"
)

// Result holds the outcome of a single statement, with values unmarshaled to T.
type Result[T any] struct {
	values []T
	err    error
}

// Values returns the unmarshaled result records for this statement.
func (r *Result[T]) Values() []T { return r.values }

// Err returns the per-statement error, if any.
func (r *Result[T]) Err() error { return r.err }

// Query executes a SurrealQL query and unmarshals each result row into T.
//
// T is typically a struct with `cbor:"..."` tags matching the returned fields.
// Primitive types (string, int, int64, float64, bool) are also supported.
//
// Example:
//
//	type Person struct {
//	    Name string `cbor:"name"`
//	    Age  int64  `cbor:"age"`
//	}
//	results, err := surrealdb.Query[Person](ctx, db, "SELECT * FROM person", nil)
//	for _, p := range results[0].Values() {
//	    fmt.Println(p.Name, p.Age)
//	}
func Query[T any](ctx context.Context, db Driver, sql string, vars any) ([]Result[T], error) {
	raw, err := db.Query(ctx, sql, vars)
	if err != nil {
		return nil, err
	}

	out := make([]Result[T], len(raw))
	for i, r := range raw {
		if r.Err != nil {
			out[i].err = r.Err
			continue
		}

		v := r.Value
		if v == nil {
			continue
		}

		switch val := v.(type) {
		case []any:
			out[i].values = make([]T, len(val))
			for j, elem := range val {
				t, err := unmarshalAny[T](elem)
				if err != nil {
					return nil, fmt.Errorf("statement %d, row %d: %w", i, j, err)
				}
				out[i].values[j] = t
			}
		default:
			t, err := unmarshalAny[T](val)
			if err != nil {
				return nil, fmt.Errorf("statement %d: %w", i, err)
			}
			out[i].values = []T{t}
		}
	}
	return out, nil
}
