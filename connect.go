package surrealdb

import (
	"context"

	"github.com/surrealdb/surrealdb.c.go/internal/surrealdbc"
)

// Option configures the behavior of Open.
type Option func(*surrealdbc.Config)

// WithMaxPendingOps sets the maximum number of goroutines that may be blocked
// inside C FFI calls (query, use, live) per DB instance at any time.
// This bounds the worst-case goroutine leak when callers cancel their context
// before a C call returns.
// Default: 16.
func WithMaxPendingOps(n int) Option {
	return func(c *surrealdbc.Config) { c.MaxPendingOps = n }
}

// Open connects to a SurrealDB instance at the given endpoint.
//
// Supported endpoints:
//   - "mem://"                   — in-memory embedded database
//   - "surrealkv://path/to/file" — file-backed embedded database
//   - "ws://host:port"           — remote WebSocket connection
//   - "http://host:port"         — remote HTTP connection
//
// The context controls the connection timeout. The caller must call Close
// on the returned DB when done.
func Open(ctx context.Context, endpoint string, opts ...Option) (*DB, error) {
	cfg := surrealdbc.DefaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	conn, err := surrealdbc.Connect(ctx, endpoint, cfg)
	if err != nil {
		return nil, err
	}
	return &DB{conn: conn}, nil
}
