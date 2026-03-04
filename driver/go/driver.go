// Package surrealdbgo registers a remote SurrealDB driver backed by the
// pure Go SDK (github.com/surrealdb/surrealdb.go).
//
// Import this package with a blank identifier to enable ws://, wss://,
// http://, and https:// endpoints in surrealdb.Open:
//
//	import _ "github.com/surrealdb/surrealdb.c.go/driver/go"
package surrealdbgo

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	surrealdb "github.com/surrealdb/surrealdb.c.go"
	"github.com/surrealdb/surrealdb.c.go/driver"
	surrealdbgo "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func init() {
	surrealdb.RegisterRemoteDriver(openRemote)
}

func openRemote(ctx context.Context, endpoint string) (driver.Driver, error) {
	goDb, err := surrealdbgo.FromEndpointURLString(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return &GoDriver{db: goDb}, nil
}

// GoDriver implements driver.Driver using the pure Go SDK.
type GoDriver struct {
	db *surrealdbgo.DB
}

func (d *GoDriver) Close() {
	d.db.Close(context.Background())
}

func (d *GoDriver) Use(ctx context.Context, ns, database string) error {
	return d.db.Use(ctx, ns, database)
}

func (d *GoDriver) Query(ctx context.Context, sql string, vars any) ([]driver.QueryResult, error) {
	varsMap, err := toMapVars(vars)
	if err != nil {
		return nil, err
	}

	raw, qErr := surrealdbgo.Query[any](ctx, d.db, sql, varsMap)
	if raw == nil && qErr != nil {
		return nil, qErr
	}

	out := make([]driver.QueryResult, len(*raw))
	for i, r := range *raw {
		if r.Error != nil {
			out[i].Err = r.Error
			continue
		}
		out[i].Value = normalizeValue(r.Result)
	}
	return out, nil
}

func (d *GoDriver) Live(_ context.Context, _ string) (string, <-chan driver.Notification, func(), error) {
	return "", nil, nil, driver.ErrNotSupported
}

func (d *GoDriver) Kill(_ string) error {
	return driver.ErrNotSupported
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func toMapVars(vars any) (map[string]any, error) {
	if vars == nil {
		return nil, nil
	}
	var m map[string]any
	if raw, ok := vars.(map[string]any); ok {
		m = raw
	} else {
		var err error
		m, err = structToMap(vars)
		if err != nil {
			return nil, err
		}
	}
	for k, v := range m {
		m[k] = denormalizeValue(v)
	}
	return m, nil
}

func structToMap(v any) (map[string]any, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("surrealdb: expected struct or map[string]any for vars, got %s", rv.Kind())
	}

	rt := rv.Type()
	m := make(map[string]any, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		key := cborFieldKey(f)
		if key == "-" {
			continue
		}
		m[key] = fieldToAny(rv.Field(i))
	}
	return m, nil
}

func cborFieldKey(f reflect.StructField) string {
	tag := f.Tag.Get("cbor")
	if tag != "" {
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			tag = tag[:idx]
		}
		return tag
	}
	return strings.ToLower(f.Name)
}

func fieldToAny(fv reflect.Value) any {
	switch fv.Kind() {
	case reflect.Struct:
		if driver.IsRecordIDType(fv.Type()) {
			return models.NewRecordID(fv.Field(0).String(), fv.Field(1).Interface())
		}
		rt := fv.Type()
		m := make(map[string]any, rt.NumField())
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if !f.IsExported() {
				continue
			}
			key := cborFieldKey(f)
			if key == "-" {
				continue
			}
			m[key] = fieldToAny(fv.Field(i))
		}
		return m
	case reflect.Map:
		m := make(map[string]any, fv.Len())
		iter := fv.MapRange()
		for iter.Next() {
			m[iter.Key().String()] = iter.Value().Interface()
		}
		return m
	default:
		return fv.Interface()
	}
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case models.CustomDuration:
		return val.Duration
	case *models.CustomDuration:
		if val == nil {
			return nil
		}
		return val.Duration
	case models.RecordID:
		return driver.RecordID[any]{Table: val.Table, ID: val.ID}
	case *models.RecordID:
		if val == nil {
			return nil
		}
		return driver.RecordID[any]{Table: val.Table, ID: val.ID}
	case models.Table:
		return string(val)
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = normalizeValue(elem)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, elem := range val {
			out[k] = normalizeValue(elem)
		}
		return out
	default:
		return v
	}
}

func denormalizeValue(v any) any {
	switch val := v.(type) {
	case driver.RecordID[any]:
		return models.NewRecordID(val.Table, val.ID)
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, elem := range val {
			out[k] = denormalizeValue(elem)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = denormalizeValue(elem)
		}
		return out
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Struct && driver.IsRecordIDType(rv.Type()) {
			return models.NewRecordID(rv.Field(0).String(), rv.Field(1).Interface())
		}
		return v
	}
}
