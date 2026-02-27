package surrealdb

import "github.com/surrealdb/surrealdb.c.go/internal/surrealdbc"

// DB is a handle to an open SurrealDB connection.
// It is safe to use from multiple goroutines.
// Call Close when done to release the underlying resources.
type DB struct {
	conn *surrealdbc.Conn
}

// Version returns the SurrealDB engine version string.
func (db *DB) Version() (string, error) {
	return db.conn.Version()
}

// Health checks whether the database connection is healthy.
func (db *DB) Health() error {
	return db.conn.Health()
}
