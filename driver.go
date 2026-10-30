package surrealdb

import (
	"context"
	"sync"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

// Re-export shared types so callers can use surrealdb.Driver, etc.
// without importing the driver sub-package directly.
type Driver = driver.Driver
type QueryResult = driver.QueryResult
type Notification = driver.Notification
type Auth = driver.Auth
type VersionData = driver.VersionData
type RecordID[T any] = driver.RecordID[T]
type Decimal = driver.Decimal
type UUID = driver.UUID
type GeometryPoint = driver.GeometryPoint
type GeometryLine = driver.GeometryLine

// NewRecordID creates a RecordID from a table name and an identifier.
func NewRecordID[T any](table string, id T) RecordID[T] {
	return driver.NewRecordID(table, id)
}

// ErrNotSupported is re-exported from the driver package.
var ErrNotSupported = driver.ErrNotSupported

// ErrClosed is re-exported from the driver package.
var ErrClosed = driver.ErrClosed

// RemoteOpener is a function that opens a remote SurrealDB connection.
type RemoteOpener func(ctx context.Context, endpoint string) (Driver, error)

var (
	remoteDriverMu sync.RWMutex
	remoteDriver   RemoteOpener
)

// RegisterRemoteDriver registers an opener for remote endpoints
// (ws://, wss://, http://, https://). Typically called from an init()
// function in a driver package, e.g.:
//
//	import _ "github.com/surrealdb/surrealdb.c.go/driver/go"
func RegisterRemoteDriver(opener RemoteOpener) {
	remoteDriverMu.Lock()
	defer remoteDriverMu.Unlock()
	remoteDriver = opener
}

// UnregisterRemoteDriver removes the registered remote driver, causing
// ws/http endpoints to fall through to the C backend. Useful in tests
// to verify the C backend handles remote protocols directly.
func UnregisterRemoteDriver() {
	remoteDriverMu.Lock()
	defer remoteDriverMu.Unlock()
	remoteDriver = nil
}

func getRemoteDriver() RemoteOpener {
	remoteDriverMu.RLock()
	defer remoteDriverMu.RUnlock()
	return remoteDriver
}
