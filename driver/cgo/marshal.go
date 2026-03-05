package cgo

// #include "surrealdb.h"
import "C"

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

var (
	timeDurationType  = reflect.TypeOf(time.Duration(0))
	timeTimeType      = reflect.TypeOf(time.Time{})
	geometryPointType = reflect.TypeOf(driver.GeometryPoint{})
	geometryLineType  = reflect.TypeOf(driver.GeometryLine{})
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
	if fv.Kind() == reflect.Interface {
		if fv.IsNil() {
			obj.insertNull(key)
			return nil
		}
		fv = fv.Elem()
	}
	if fv.Kind() == reflect.Struct && driver.IsRecordIDType(fv.Type()) {
		table := fv.Field(0).String()
		id := fmt.Sprintf("%v", fv.Field(1).Interface())
		obj.insertThing(key, table, id)
		return nil
	}
	if fv.Type() == timeDurationType {
		obj.insertDuration(key, time.Duration(fv.Int()))
		return nil
	}
	if fv.Type() == timeTimeType {
		obj.insertDatetime(key, fv.Interface().(time.Time))
		return nil
	}
	if fv.Type() == geometryPointType {
		pt := fv.Interface().(driver.GeometryPoint)
		obj.insertPoint(key, pt.Longitude, pt.Latitude)
		return nil
	}
	if fv.Type() == geometryLineType {
		line := fv.Interface().(driver.GeometryLine)
		coords := make([]C.sr_sr_g_coord, len(line))
		for i, pt := range line {
			coords[i] = C.sr_sr_g_coord{x: C.double(pt.Longitude), y: C.double(pt.Latitude)}
		}
		obj.insertLineString(key, coords)
		return nil
	}

	switch fv.Kind() {
	case reflect.String:
		obj.insertStr(key, fv.String())
	case reflect.Bool:
		obj.insertBool(key, fv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		obj.insertInt(key, fv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		obj.insertInt(key, int64(fv.Uint()))
	case reflect.Float32, reflect.Float64:
		obj.insertFloat(key, fv.Float())
	case reflect.Slice:
		if fv.Type().Elem().Kind() == reflect.Uint8 {
			obj.insertBytes(key, fv.Bytes())
			return nil
		}
		elems := make([]*C.sr_value_t, fv.Len())
		for i := 0; i < fv.Len(); i++ {
			v, err := scalarToCValue(fv.Index(i))
			if err != nil {
				for _, prev := range elems[:i] {
					C.sr_value_free(prev)
				}
				return fmt.Errorf("index %d: %w", i, err)
			}
			elems[i] = v
		}
		obj.insertArray(key, elems)
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

func scalarToCValue(fv reflect.Value) (*C.sr_value_t, error) {
	if fv.Kind() == reflect.Interface {
		if fv.IsNil() {
			return C.sr_value_null(), nil
		}
		fv = fv.Elem()
	}
	switch fv.Kind() {
	case reflect.String:
		cs := cstring(fv.String())
		v := C.sr_value_string(cs)
		freeCString(cs)
		return v, nil
	case reflect.Bool:
		return C.sr_value_bool(C._Bool(fv.Bool())), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return C.sr_value_int(C.int64_t(fv.Int())), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return C.sr_value_int(C.int64_t(fv.Uint())), nil
	case reflect.Float32, reflect.Float64:
		return C.sr_value_float(C.double(fv.Float())), nil
	default:
		return nil, fmt.Errorf("unsupported array element type %s", fv.Kind())
	}
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
