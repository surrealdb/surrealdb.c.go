package surrealdb

import "context"

// Query executes a SurrealQL query and returns results for each statement.
//
// The sql string may contain multiple semicolon-separated statements.
// Each statement produces one QueryResult in the returned slice.
//
// The context controls the query timeout. If ctx is cancelled while the
// C call is in progress, the goroutine continues to completion in the
// background.
//
// vars is automatically marshaled to a SurrealDB object:
//   - nil: no parameters
//   - struct or *struct: marshaled using `db:"..."` tags
//   - map[string]any: marshaled by key
//
// Example with a struct:
//
//	results, err := db.Query(ctx, "SELECT * FROM person WHERE age >= $min_age", struct {
//	    MinAge int `db:"min_age"`
//	}{MinAge: 18})
func (db *DB) Query(ctx context.Context, sql string, vars any) ([]QueryResult, error) {
	return db.conn.Query(ctx, sql, vars)
}
