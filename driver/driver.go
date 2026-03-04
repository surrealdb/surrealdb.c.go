// Package driver defines the interface and types shared by all SurrealDB backends.
package driver

import (
	"context"
	"errors"
)

// ErrNotSupported is returned when an operation is not supported by the current backend.
var ErrNotSupported = errors.New("surrealdb: operation not supported by this backend")

// Driver is the interface that each SurrealDB backend must implement.
type Driver interface {
	Close()
	Use(ctx context.Context, ns, database string) error
	Query(ctx context.Context, sql string, vars any) ([]QueryResult, error)
	Live(ctx context.Context, resource string) (liveQueryID string, ch <-chan Notification, kill func(), err error)
	Kill(liveQueryID string) error
}
