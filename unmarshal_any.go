package surrealdb

import (
	"fmt"
	"reflect"
	"strings"
)

// unmarshalAny converts a plain Go value (map[string]any, primitives) into T.
// Struct fields are matched by `cbor:"..."` tags.
func unmarshalAny[T any](src any) (T, error) {
	var zero T
	rt := reflect.TypeOf(zero)
	if rt == nil {
		return zero, fmt.Errorf("unmarshal: cannot unmarshal into untyped interface")
	}

	isPtr := rt.Kind() == reflect.Ptr
	if isPtr {
		rt = rt.Elem()
	}

	rv := reflect.New(rt).Elem()
	if err := setFromAny(rv, src); err != nil {
		return zero, err
	}

	if isPtr {
		ptr := reflect.New(rt)
		ptr.Elem().Set(rv)
		return ptr.Interface().(T), nil
	}
	return rv.Interface().(T), nil
}

func setFromAny(rv reflect.Value, src any) error {
	if src == nil {
		rv.Set(reflect.Zero(rv.Type()))
		return nil
	}

	switch rv.Kind() {
	case reflect.String:
		s, ok := src.(string)
		if !ok {
			return fmt.Errorf("unmarshal: expected string, got %T", src)
		}
		rv.SetString(s)

	case reflect.Bool:
		b, ok := src.(bool)
		if !ok {
			return fmt.Errorf("unmarshal: expected bool, got %T", src)
		}
		rv.SetBool(b)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := toInt64Any(src)
		if err != nil {
			return err
		}
		rv.SetInt(n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := toInt64Any(src)
		if err != nil {
			return err
		}
		rv.SetUint(uint64(n))

	case reflect.Float32, reflect.Float64:
		f, err := toFloat64Any(src)
		if err != nil {
			return err
		}
		rv.SetFloat(f)

	case reflect.Struct:
		m, ok := src.(map[string]any)
		if !ok {
			return fmt.Errorf("unmarshal: expected map[string]any for struct, got %T", src)
		}
		return unmarshalStructFromMap(rv, m)

	case reflect.Slice:
		srcSlice, ok := src.([]any)
		if !ok {
			return fmt.Errorf("unmarshal: expected []any for slice, got %T", src)
		}
		slice := reflect.MakeSlice(rv.Type(), len(srcSlice), len(srcSlice))
		for i, elem := range srcSlice {
			if err := setFromAny(slice.Index(i), elem); err != nil {
				return fmt.Errorf("index %d: %w", i, err)
			}
		}
		rv.Set(slice)

	case reflect.Map:
		m, ok := src.(map[string]any)
		if !ok {
			return fmt.Errorf("unmarshal: expected map[string]any, got %T", src)
		}
		newMap := reflect.MakeMapWithSize(rv.Type(), len(m))
		for k, v := range m {
			val := reflect.New(rv.Type().Elem()).Elem()
			if err := setFromAny(val, v); err != nil {
				return fmt.Errorf("key %q: %w", k, err)
			}
			newMap.SetMapIndex(reflect.ValueOf(k), val)
		}
		rv.Set(newMap)

	case reflect.Interface:
		rv.Set(reflect.ValueOf(src))

	default:
		return fmt.Errorf("unmarshal: unsupported target type %s", rv.Kind())
	}
	return nil
}

func unmarshalStructFromMap(rv reflect.Value, m map[string]any) error {
	rt := rv.Type()

	keyToField := make(map[string]int, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		key := cborFieldKey(f)
		if key == "-" {
			continue
		}
		keyToField[key] = i
	}

	for k, v := range m {
		idx, ok := keyToField[k]
		if !ok {
			continue
		}
		if err := setFromAny(rv.Field(idx), v); err != nil {
			return fmt.Errorf("field %s: %w", rt.Field(idx).Name, err)
		}
	}
	return nil
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

func toInt64Any(v any) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint:
		return int64(n), nil
	case uint8:
		return int64(n), nil
	case uint16:
		return int64(n), nil
	case uint32:
		return int64(n), nil
	case uint64:
		return int64(n), nil
	case float32:
		return int64(n), nil
	case float64:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("unmarshal: expected numeric, got %T", v)
	}
}

func toFloat64Any(v any) (float64, error) {
	switch n := v.(type) {
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("unmarshal: expected numeric, got %T", v)
	}
}
