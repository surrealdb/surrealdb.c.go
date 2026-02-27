package surrealdb

import (
	"fmt"
	"reflect"
	"strings"
)

// MarshalObject converts a Go struct into a SurrealDB Object.
//
// Exported struct fields are mapped to object keys. The key name is determined by:
//  1. The `db:"name"` struct tag, if present
//  2. Otherwise, the field name lowercased
//
// Fields tagged `db:"-"` are skipped.
//
// Supported field types: string, bool, int (all sizes), uint (all sizes),
// float32, float64, nested structs, and maps (recursively marshaled).
//
// The caller must call Free on the returned Object when done.
func MarshalObject(v any) (*Object, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("surrealdb.MarshalObject: expected struct, got %s", rv.Kind())
	}
	return marshalStruct(rv)
}

func marshalStruct(rv reflect.Value) (*Object, error) {
	rt := rv.Type()
	obj := NewObject()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		key := fieldKey(field)
		if key == "-" {
			continue
		}

		fv := rv.Field(i)
		if err := insertValue(obj, key, fv); err != nil {
			obj.Free()
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}
	}
	return obj, nil
}

func fieldKey(f reflect.StructField) string {
	tag := f.Tag.Get("db")
	if tag != "" {
		// Support `db:"name,omitempty"` — take only the name part.
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			tag = tag[:idx]
		}
		return tag
	}
	return strings.ToLower(f.Name)
}

func insertValue(obj *Object, key string, fv reflect.Value) error {
	switch fv.Kind() {
	case reflect.String:
		obj.InsertStr(key, fv.String())
	case reflect.Bool:
		obj.InsertBool(key, fv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		obj.InsertInt(key, int(fv.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		obj.InsertInt(key, int(fv.Uint()))
	case reflect.Float32, reflect.Float64:
		obj.InsertFloat(key, fv.Float())
	case reflect.Struct:
		nested, err := marshalStruct(fv)
		if err != nil {
			return err
		}
		obj.InsertObject(key, nested)
		nested.Free()
	case reflect.Map:
		nested, err := marshalMap(fv)
		if err != nil {
			return err
		}
		obj.InsertObject(key, nested)
		nested.Free()
	default:
		return fmt.Errorf("unsupported type %s", fv.Kind())
	}
	return nil
}
