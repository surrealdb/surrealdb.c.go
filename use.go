package surrealdb

import "context"

// Use selects the namespace and database for subsequent operations.
// Both are created automatically on first write.
//
// The context controls the timeout. If ctx is cancelled while the C calls
// are in progress, the goroutine continues to completion in the background.
func (db *DB) Use(ctx context.Context, ns, database string) error {
	return db.conn.Use(ctx, ns, database)
}
