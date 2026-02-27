package surrealdb

import (
	"fmt"
	"reflect"
)

// MarshalMap converts a Go map into a SurrealDB Object.
//
// Map keys become object keys. Values are marshaled by type:
// string, bool, int (all sizes), uint (all sizes), float32, float64,
// nested structs, and nested maps are supported.
//
// Nil values are skipped.
//
// The caller must call Free on the returned Object when done.
func MarshalMap(m map[string]any) (*Object, error) {
	return marshalMap(reflect.ValueOf(m))
}

func marshalMap(rv reflect.Value) (*Object, error) {
	obj := NewObject()
	iter := rv.MapRange()
	for iter.Next() {
		key := iter.Key().String()
		val := iter.Value()
		if val.Kind() == reflect.Interface {
			if val.IsNil() {
				continue
			}
			val = val.Elem()
		}
		if err := insertValue(obj, key, val); err != nil {
			obj.Free()
			return nil, fmt.Errorf("key %s: %w", key, err)
		}
	}
	return obj, nil
}
