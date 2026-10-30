// Package driver defines the interface and types shared by all SurrealDB backends.
package driver

import (
	"context"
	"errors"
)

// ErrNotSupported is returned when an operation is not supported by the current backend.
var ErrNotSupported = errors.New("surrealdb: operation not supported by this backend")

// ErrClosed is returned when an operation is attempted on a closed connection.
var ErrClosed = errors.New("surrealdb: connection closed")

// Driver is the interface that each SurrealDB backend must implement.
type Driver interface {
	Close()
	Use(ctx context.Context, ns, database string) error
	Query(ctx context.Context, sql string, vars any) ([]QueryResult, error)
	Live(ctx context.Context, resource string) (liveQueryID string, ch <-chan Notification, kill func(), err error)
	Kill(liveQueryID string) error

	// SignIn signs in as a root, namespace, database or record user and
	// returns the access token. authData is an Auth or a map[string]any.
	SignIn(ctx context.Context, authData any) (string, error)
	// SignUp signs up a record user through a record access method and
	// returns the access token. authData is an Auth or a map[string]any.
	SignUp(ctx context.Context, authData any) (string, error)
	// Authenticate authenticates the connection with an access token.
	Authenticate(ctx context.Context, token string) error
	// Invalidate clears the connection's authentication.
	Invalidate(ctx context.Context) error
}
