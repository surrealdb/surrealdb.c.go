package surrealdb

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.c.go/driver"
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

	srcRV := reflect.ValueOf(src)
	if srcRV.Kind() == reflect.Struct && driver.IsRecordIDType(srcRV.Type()) {
		return setRecordIDTarget(rv, srcRV)
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
		if driver.IsRecordIDType(rv.Type()) {
			return fmt.Errorf("unmarshal: expected driver.RecordID source for RecordID target, got %T", src)
		}
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

// setRecordIDTarget handles assigning a source RecordID[T] (accessed via
// reflection) to a target reflect.Value, which may be a string,
// another RecordID[U], or an interface{}.
func setRecordIDTarget(rv reflect.Value, srcRID reflect.Value) error {
	table := srcRID.Field(0).String()
	rawID := srcRID.Field(1).Interface()

	switch rv.Kind() {
	case reflect.String:
		rv.SetString(fmt.Sprintf("%s:%v", table, rawID))
		return nil
	case reflect.Struct:
		if driver.IsRecordIDType(rv.Type()) {
			rv.Field(0).SetString(table)
			if err := setFromAny(rv.Field(1), rawID); err != nil {
				return setIDFromNumericString(rv.Field(1), rawID, err)
			}
			return nil
		}
		return fmt.Errorf("unmarshal: cannot assign RecordID to struct %s", rv.Type())
	case reflect.Interface:
		rv.Set(srcRID)
		return nil
	default:
		return fmt.Errorf("unmarshal: cannot assign RecordID to %s", rv.Kind())
	}
}

// setIDFromNumericString is a fallback for when the C backend returns
// numeric record IDs as strings (e.g. "123" instead of int64(123)).
func setIDFromNumericString(rv reflect.Value, rawID any, origErr error) error {
	s, ok := rawID.(string)
	if !ok {
		return origErr
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return origErr
		}
		rv.SetInt(n)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return origErr
		}
		rv.SetUint(n)
		return nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return origErr
		}
		rv.SetFloat(f)
		return nil
	default:
		return origErr
	}
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
	case time.Duration:
		return int64(n), nil
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
