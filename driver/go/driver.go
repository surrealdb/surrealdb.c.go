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
	"time"

	surrealdb "github.com/surrealdb/surrealdb.c.go"
	"github.com/surrealdb/surrealdb.c.go/driver"
	surrealdbgo "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

var (
	timeDurationType  = reflect.TypeOf(time.Duration(0))
	timeTimeType      = reflect.TypeOf(time.Time{})
	geometryPointType = reflect.TypeOf(driver.GeometryPoint{})
	geometryLineType  = reflect.TypeOf(driver.GeometryLine{})
)

func init() {
	surrealdb.RegisterRemoteDriver(openRemote)
}

// Register explicitly registers the pure Go remote driver.
// This is an alternative to the blank import:
//
//	import _ "github.com/surrealdb/surrealdb.c.go/driver/go"
//
// Useful when you want to conditionally enable the Go driver at runtime
// (e.g. in tests) rather than at import time.
func Register() {
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

// goAuth converts driver.Auth to the Go SDK's Auth, which has the same
// wire names. Map values are converted like query variables, so record
// access parameters such as record ids keep their SurrealDB types.
func goAuth(authData any) any {
	switch a := authData.(type) {
	case map[string]any:
		return denormalizeValue(a)
	case driver.Auth:
		return &surrealdbgo.Auth{Namespace: a.Namespace, Database: a.Database, Access: a.Access, Username: a.Username, Password: a.Password}
	case *driver.Auth:
		if a == nil {
			return nil
		}
		return goAuth(*a)
	default:
		return authData
	}
}

func (d *GoDriver) SignIn(ctx context.Context, authData any) (string, error) {
	return d.db.SignIn(ctx, goAuth(authData))
}

func (d *GoDriver) SignUp(ctx context.Context, authData any) (string, error) {
	return d.db.SignUp(ctx, goAuth(authData))
}

func (d *GoDriver) Authenticate(ctx context.Context, token string) error {
	return d.db.Authenticate(ctx, token)
}

func (d *GoDriver) Invalidate(ctx context.Context) error {
	return d.db.Invalidate(ctx)
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
		key, omit := parseCborTag(f)
		if key == "-" {
			continue
		}
		fv := rv.Field(i)
		if omit && isGoZero(fv) {
			continue
		}
		m[key] = fieldToAny(fv)
	}
	return m, nil
}

func parseCborTag(f reflect.StructField) (key string, omitempty bool) {
	tag := f.Tag.Get("cbor")
	if tag == "" {
		return strings.ToLower(f.Name), false
	}
	if idx := strings.IndexByte(tag, ','); idx >= 0 {
		omitempty = strings.Contains(tag[idx+1:], "omitempty")
		tag = tag[:idx]
	}
	return tag, omitempty
}

func isGoZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	default:
		return false
	}
}

// cborFieldKey is kept for callers that don't need omitempty.
func cborFieldKey(f reflect.StructField) string {
	key, _ := parseCborTag(f)
	return key
}

func fieldToAny(fv reflect.Value) any {
	switch fv.Kind() {
	case reflect.Struct:
		if driver.IsRecordIDType(fv.Type()) {
			return models.NewRecordID(fv.Field(0).String(), fv.Field(1).Interface())
		}
		if fv.Type() == timeTimeType {
			return &models.CustomDateTime{Time: fv.Interface().(time.Time)}
		}
		if fv.Type() == geometryPointType {
			pt := fv.Interface().(driver.GeometryPoint)
			return &models.GeometryPoint{Longitude: pt.Longitude, Latitude: pt.Latitude}
		}
		rt := fv.Type()
		m := make(map[string]any, rt.NumField())
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if !f.IsExported() {
				continue
			}
			key, omit := parseCborTag(f)
			if key == "-" {
				continue
			}
			if omit && isGoZero(fv.Field(i)) {
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
	case reflect.Slice:
		if fv.Type() == geometryLineType {
			line := fv.Interface().(driver.GeometryLine)
			mLine := make(models.GeometryLine, len(line))
			for i, pt := range line {
				mLine[i] = models.GeometryPoint{Longitude: pt.Longitude, Latitude: pt.Latitude}
			}
			return mLine
		}
		return fv.Interface()
	default:
		if fv.Type() == timeDurationType {
			return &models.CustomDuration{Duration: time.Duration(fv.Int())}
		}
		return fv.Interface()
	}
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case models.CustomNil:
		_ = val
		return nil
	case *models.CustomNil:
		return nil
	case models.CustomDateTime:
		return val.Time.UTC()
	case *models.CustomDateTime:
		if val == nil {
			return nil
		}
		return val.Time.UTC()
	case models.CustomDuration:
		return val.Duration
	case *models.CustomDuration:
		if val == nil {
			return nil
		}
		return val.Duration
	case models.GeometryPoint:
		return driver.GeometryPoint{Longitude: val.Longitude, Latitude: val.Latitude}
	case *models.GeometryPoint:
		if val == nil {
			return nil
		}
		return driver.GeometryPoint{Longitude: val.Longitude, Latitude: val.Latitude}
	case models.GeometryLine:
		line := make(driver.GeometryLine, len(val))
		for i, pt := range val {
			line[i] = driver.GeometryPoint{Longitude: pt.Longitude, Latitude: pt.Latitude}
		}
		return line
	case models.DecimalString:
		return string(val)
	case models.UUID:
		return val.String()
	case *models.UUID:
		if val == nil {
			return nil
		}
		return val.String()
	case models.UUIDString:
		return string(val)
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
	case driver.GeometryLine:
		mLine := make(models.GeometryLine, len(val))
		for i, pt := range val {
			mLine[i] = models.GeometryPoint{Longitude: pt.Longitude, Latitude: pt.Latitude}
		}
		return mLine
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Struct {
			return fieldToAny(rv)
		}
		return v
	}
}
