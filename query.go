package surrealdb

import "fmt"

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
// T is typically a struct with `db:"..."` tags matching the returned fields.
// Primitive types (string, int, int64, float64, bool) are also supported.
//
// vars follows the same rules as DB.Query (nil, *Object, struct, map[string]any).
//
// Example:
//
//	type Person struct {
//	    Name string `db:"name"`
//	    Age  int64  `db:"age"`
//	}
//	results, err := surrealdb.Query[Person](db, "SELECT * FROM person", nil)
//	for _, p := range results[0].Values() {
//	    fmt.Println(p.Name, p.Age)
//	}
func Query[T any](db *DB, sql string, vars any) ([]Result[T], error) {
	raw, err := db.Query(sql, vars)
	if err != nil {
		return nil, err
	}

	out := make([]Result[T], len(raw))
	for i, r := range raw {
		if r.err != nil {
			out[i].err = r.err
			continue
		}
		out[i].values = make([]T, len(r.values))
		for j, v := range r.values {
			t, err := unmarshalValue[T](v)
			if err != nil {
				return nil, fmt.Errorf("statement %d, row %d: %w", i, j, err)
			}
			out[i].values[j] = t
		}
	}
	return out, nil
}
