package surrealdb

import (
	"context"

	"github.com/surrealdb/surrealdb.c.go/internal/surrealdbc"
)

// LiveQuery is a handle to an active live query session.
// Read notifications from Ch. Call db.Kill to stop.
type LiveQuery struct {
	// ID is the session identifier assigned by the library.
	ID string
	// Ch receives notifications as they arrive.
	// The channel is closed when the stream ends.
	Ch     <-chan Notification
	handle *surrealdbc.LiveHandle
}

// Live creates a live query on the given resource (table name) and returns
// a LiveQuery whose Ch channel receives notifications as they arrive.
//
// The context controls the timeout for the initial subscription call.
//
// Use lq.ID to identify the session, and db.Kill(lq) to stop it.
func (db *DB) Live(ctx context.Context, resource string) (*LiveQuery, error) {
	handle, err := db.conn.Live(ctx, resource)
	if err != nil {
		return nil, err
	}
	return &LiveQuery{
		ID:     handle.ID,
		Ch:     handle.Ch,
		handle: handle,
	}, nil
}

// Kill stops a live query session and frees its resources.
func (db *DB) Kill(lq *LiveQuery) {
	db.conn.Kill(lq.handle)
}
