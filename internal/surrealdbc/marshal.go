package surrealdbc

import (
	"fmt"
	"reflect"
	"strings"
)

func marshalVars(v any) (*object, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		return marshalMap(reflect.ValueOf(v))
	default:
		return marshalObject(v)
	}
}

func marshalObject(v any) (*object, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("surrealdb: expected struct, got %s", rv.Kind())
	}
	return marshalStruct(rv)
}

func marshalStruct(rv reflect.Value) (*object, error) {
	rt := rv.Type()
	obj := newObject()

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
	tag := f.Tag.Get("cbor")
	if tag != "" {
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			tag = tag[:idx]
		}
		return tag
	}
	return strings.ToLower(f.Name)
}

func insertValue(obj *object, key string, fv reflect.Value) error {
	switch fv.Kind() {
	case reflect.String:
		obj.insertStr(key, fv.String())
	case reflect.Bool:
		obj.insertBool(key, fv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		obj.insertInt(key, int(fv.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		obj.insertInt(key, int(fv.Uint()))
	case reflect.Float32, reflect.Float64:
		obj.insertFloat(key, fv.Float())
	case reflect.Struct:
		nested, err := marshalStruct(fv)
		if err != nil {
			return err
		}
		obj.insertObject(key, nested)
		nested.Free()
	case reflect.Map:
		nested, err := marshalMap(fv)
		if err != nil {
			return err
		}
		obj.insertObject(key, nested)
		nested.Free()
	default:
		return fmt.Errorf("unsupported type %s", fv.Kind())
	}
	return nil
}

func marshalMap(rv reflect.Value) (*object, error) {
	obj := newObject()
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
