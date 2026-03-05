// Package cgo provides a SurrealDB driver backed by the C library (libsurrealdb_c)
// via CGo. It supports embedded endpoints: mem://, surrealkv://, rocksdb://.
package cgo

import (
	"context"
	"time"

	"github.com/surrealdb/surrealdb.c.go/driver"
	"github.com/surrealdb/surrealdb.c.go/internal/surrealdbc"
)

// CDriver wraps surrealdbc.Conn and implements driver.Driver.
type CDriver struct {
	conn *surrealdbc.Conn
}

// New creates a CDriver by connecting to the given embedded endpoint.
func New(ctx context.Context, endpoint string, cfg surrealdbc.Config) (*CDriver, error) {
	conn, err := surrealdbc.Connect(ctx, endpoint, cfg)
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
	raw, err := d.conn.Query(ctx, sql, vars)
	if err != nil {
		return nil, err
	}
	out := make([]driver.QueryResult, len(raw))
	for i, r := range raw {
		if r.Err() != nil {
			out[i].Err = r.Err()
			continue
		}
		vals := r.Values()
		rows := make([]any, len(vals))
		for j, v := range vals {
			rows[j] = valueToAny(&v)
		}
		out[i].Value = rows
	}
	return out, nil
}

func (d *CDriver) Live(ctx context.Context, resource string) (string, <-chan driver.Notification, func(), error) {
	handle, err := d.conn.Live(ctx, resource)
	if err != nil {
		return "", nil, nil, err
	}

	ch := make(chan driver.Notification, 64)
	go func() {
		defer close(ch)
		for n := range handle.Ch {
			ch <- driver.Notification{
				QueryID: n.QueryID,
				Action:  n.Action.String(),
				Data:    valueToAny(&n.Data),
			}
		}
	}()

	kill := func() { d.conn.Kill(handle) }
	return handle.ID, ch, kill, nil
}

func (d *CDriver) Kill(liveQueryID string) error {
	return driver.ErrNotSupported
}

// valueToAny converts an internal surrealdbc.Value into plain Go types.
func valueToAny(v *surrealdbc.Value) any {
	switch v.Tag() {
	case surrealdbc.ValueNone:
		return nil
	case surrealdbc.ValueNull:
		return nil
	case surrealdbc.ValueBool:
		return v.AsBool()
	case surrealdbc.ValueNumber:
		if v.AsInt() != 0 {
			return v.AsInt()
		}
		if v.AsFloat() != 0 {
			return v.AsFloat()
		}
		if v.AsString() != "" {
			return v.AsString()
		}
		return int64(0)
	case surrealdbc.ValueString:
		return v.AsString()
	case surrealdbc.ValueUUID:
		return v.AsUUID()
	case surrealdbc.ValueDuration:
		return v.AsDuration()
	case surrealdbc.ValueDatetime:
		t, err := time.Parse(time.RFC3339Nano, v.AsString())
		if err != nil {
			return v.AsString()
		}
		return t.UTC()
	case surrealdbc.ValueBytes:
		return v.AsBytes()
	case surrealdbc.ValueGeometry:
		return v.AsGeometry()
	case surrealdbc.ValueThing:
		return driver.RecordID[any]{Table: v.AsThingTable(), ID: v.AsThingID()}
	case surrealdbc.ValueArray:
		arr := v.AsArray()
		out := make([]any, len(arr))
		for i := range arr {
			out[i] = valueToAny(&arr[i])
		}
		return out
	case surrealdbc.ValueObject:
		kvs := v.AsObject()
		m := make(map[string]any, len(kvs))
		for _, kv := range kvs {
			m[kv.Key] = valueToAny(&kv.Value)
		}
		return m
	default:
		return v.String()
	}
}
