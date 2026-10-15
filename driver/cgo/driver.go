// Package cgo provides a SurrealDB driver backed by the C library (libsurrealdb_c)
// via CGo. It supports embedded endpoints: mem://, surrealkv://, rocksdb://.
package cgo

import (
	"context"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

// CDriver implements driver.Driver using the embedded C backend.
type CDriver struct {
	conn *Conn
}

// New creates a CDriver by connecting to the given embedded endpoint.
func New(ctx context.Context, endpoint string, cfg Config) (*CDriver, error) {
	conn, err := Connect(ctx, endpoint, cfg)
	if err != nil {
		return nil, err
	}
	return &CDriver{conn: conn}, nil
}

func (d *CDriver) Close() {
	d.conn.Close()
}

func (d *CDriver) Use(ctx context.Context, ns, database string) error {
	return d.conn.Use(ctx, ns, database)
}

func (d *CDriver) Query(ctx context.Context, sql string, vars any) ([]driver.QueryResult, error) {
	return d.conn.Query(ctx, sql, vars)
}

func (d *CDriver) Live(ctx context.Context, resource string) (string, <-chan driver.Notification, func(), error) {
	handle, err := d.conn.Live(ctx, resource)
	if err != nil {
		return "", nil, nil, err
	}

	kill := func() { d.conn.Kill(handle) }
	return handle.ID, handle.Ch, kill, nil
}

// Kill stops a live query by the id returned from Live, or by the server's
// live query id.
func (d *CDriver) Kill(liveQueryID string) error {
	return d.conn.KillID(context.Background(), liveQueryID)
}
