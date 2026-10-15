package cgo

// #include <stdlib.h>
// #include "surrealdb.h"
import "C"

import (
	"fmt"
	"reflect"
	"time"
	"unsafe"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

// toCValue converts a Go value to a newly allocated C value, which the
// caller must free with sr_value_free. It supports the same types as query
// variables.
func toCValue(v any) (*C.sr_value_t, error) {
	return reflectToCValue(reflect.ValueOf(v))
}

func reflectToCValue(fv reflect.Value) (*C.sr_value_t, error) {
	if !fv.IsValid() {
		return C.sr_value_null(), nil
	}
	if fv.Kind() == reflect.Interface {
		if fv.IsNil() {
			return C.sr_value_null(), nil
		}
		fv = fv.Elem()
	}
	if fv.Kind() == reflect.Struct && driver.IsRecordIDType(fv.Type()) {
		cTable := cstring(fv.Field(0).String())
		defer freeCString(cTable)
		cID := cstring(fmt.Sprintf("%v", fv.Field(1).Interface()))
		defer freeCString(cID)
		return C.sr_value_thing(cTable, cID), nil
	}
	switch fv.Type() {
	case timeDurationType:
		d := time.Duration(fv.Int())
		return C.sr_value_duration(C.uint64_t(d/time.Second), C.uint32_t(d%time.Second)), nil
	case timeTimeType:
		cRFC := cstring(fv.Interface().(time.Time).UTC().Format(time.RFC3339Nano))
		defer freeCString(cRFC)
		return C.sr_value_datetime(cRFC), nil
	case geometryPointType:
		pt := fv.Interface().(driver.GeometryPoint)
		return C.sr_value_point(C.double(pt.Longitude), C.double(pt.Latitude)), nil
	case geometryLineType:
		line := fv.Interface().(driver.GeometryLine)
		if len(line) == 0 {
			return C.sr_value_linestring(nil, 0), nil
		}
		coords := make([]C.sr_sr_g_coord, len(line))
		for i, pt := range line {
			coords[i] = C.sr_sr_g_coord{x: C.double(pt.Longitude), y: C.double(pt.Latitude)}
		}
		return C.sr_value_linestring(&coords[0], C.int(len(coords))), nil
	}

	switch fv.Kind() {
	case reflect.String:
		cs := cstring(fv.String())
		defer freeCString(cs)
		return C.sr_value_string(cs), nil
	case reflect.Bool:
		return C.sr_value_bool(C._Bool(fv.Bool())), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return C.sr_value_int(C.int64_t(fv.Int())), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return C.sr_value_int(C.int64_t(fv.Uint())), nil
	case reflect.Float32, reflect.Float64:
		return C.sr_value_float(C.double(fv.Float())), nil
	case reflect.Slice, reflect.Array:
		if fv.Kind() == reflect.Slice && fv.IsNil() {
			return C.sr_value_null(), nil
		}
		if fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() == reflect.Uint8 {
			b := fv.Bytes()
			if len(b) == 0 {
				return C.sr_value_bytes(nil, 0), nil
			}
			return C.sr_value_bytes((*C.uint8_t)(unsafe.Pointer(&b[0])), C.int(len(b))), nil
		}
		elems := make([]*C.sr_value_t, 0, fv.Len())
		defer func() {
			for _, e := range elems {
				C.sr_value_free(e)
			}
		}()
		for i := 0; i < fv.Len(); i++ {
			e, err := reflectToCValue(fv.Index(i))
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			elems = append(elems, e)
		}
		return buildArray(elems), nil
	case reflect.Map:
		if fv.IsNil() {
			return C.sr_value_null(), nil
		}
		// Unlike marshalMap (query variables), keep nil entries as null so
		// the object has the same shape as on the remote backend.
		obj := newObject()
		defer obj.Free()
		iter := fv.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			e, err := reflectToCValue(iter.Value())
			if err != nil {
				return nil, fmt.Errorf("key %s: %w", key, err)
			}
			obj.insertCValue(key, e)
			C.sr_value_free(e)
		}
		return C.sr_value_object(obj.cptr()), nil
	case reflect.Struct:
		obj, err := marshalStruct(fv)
		if err != nil {
			return nil, err
		}
		defer obj.Free()
		return C.sr_value_object(obj.cptr()), nil
	case reflect.Ptr:
		if fv.IsNil() {
			return C.sr_value_null(), nil
		}
		return reflectToCValue(fv.Elem())
	default:
		return nil, fmt.Errorf("unsupported type %s", fv.Type())
	}
}
