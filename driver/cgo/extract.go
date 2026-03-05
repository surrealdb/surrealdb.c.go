package cgo

// #include "surrealdb.h"
import "C"

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

// extractAny converts a C sr_value_t directly to a plain Go any value.
// This is the single-hop extraction path — no intermediate Value type.
func extractAny(cv *C.sr_value_t) any {
	switch cv.tag {
	case C.SR_VALUE_NONE:
		return nil
	case C.SR_VALUE_NULL:
		return nil
	case C.SR_VALUE_BOOL:
		return bool(*(*C._Bool)(unsafe.Pointer(&cv.anon0)))
	case C.SR_VALUE_NUMBER:
		return extractNumber((*C.sr_number_t)(unsafe.Pointer(&cv.anon0)))
	case C.SR_VALUE_STRAND:
		return C.GoString(*(*C.sr_string_t)(unsafe.Pointer(&cv.anon0)))
	case C.SR_VALUE_DURATION:
		d := (*C.sr_duration_t)(unsafe.Pointer(&cv.anon0))
		return time.Duration(d.secs)*time.Second + time.Duration(d.nanos)*time.Nanosecond
	case C.SR_VALUE_DATETIME:
		s := C.GoString(*(*C.sr_string_t)(unsafe.Pointer(&cv.anon0)))
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return s
		}
		return t.UTC()
	case C.SR_VALUE_UUID:
		cuuid := (*C.sr_uuid_t)(unsafe.Pointer(&cv.anon0))
		b := cuuid._0
		return fmt.Sprintf(
			"%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
			b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7],
			b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15],
		)
	case C.SR_VALUE_ARRAY:
		arr := *(**C.sr_array_t)(unsafe.Pointer(&cv.anon0))
		n := int(arr.len)
		out := make([]any, n)
		for i := 0; i < n; i++ {
			elem := (*C.sr_value_t)(unsafe.Pointer(
				uintptr(unsafe.Pointer(arr.arr)) + uintptr(i)*unsafe.Sizeof(C.sr_value_t{}),
			))
			out[i] = extractAny(elem)
		}
		return out
	case C.SR_VALUE_OBJECT:
		return extractObjectMap((*C.sr_object_t)(unsafe.Pointer(&cv.anon0)))
	case C.SR_VALUE_BYTES:
		cb := (*C.sr_bytes_t)(unsafe.Pointer(&cv.anon0))
		if cb.len > 0 {
			return C.GoBytes(unsafe.Pointer(cb.arr), cb.len)
		}
		return []byte{}
	case C.SR_GEOMETRY_OBJECT:
		return extractGeometry((*C.sr_sr_geometry)(unsafe.Pointer(&cv.anon0)))
	case C.SR_VALUE_THING:
		thing := (*C.sr_thing_t)(unsafe.Pointer(&cv.anon0))
		return driver.RecordID[any]{
			Table: C.GoString(thing.table),
			ID:    extractID(&thing.id),
		}
	default:
		return nil
	}
}

func extractNumber(cn *C.sr_number_t) any {
	switch cn.tag {
	case C.SR_NUMBER_INT:
		return int64(*(*C.int64_t)(unsafe.Pointer(&cn.anon0)))
	case C.SR_NUMBER_FLOAT:
		return float64(*(*C.double)(unsafe.Pointer(&cn.anon0)))
	case C.SR_NUMBER_DECIMAL:
		return C.GoString(*(*C.sr_string_t)(unsafe.Pointer(&cn.anon0)))
	default:
		return int64(0)
	}
}

func extractObjectMap(cobj *C.sr_object_t) map[string]any {
	var cKeys **C.char
	n := int(C.sr_object_keys(cobj, &cKeys))
	if n <= 0 {
		return map[string]any{}
	}
	m := make(map[string]any, n)
	for i := 0; i < n; i++ {
		p := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cKeys)) + uintptr(i)*unsafe.Sizeof(*cKeys)))
		key := C.GoString(*p)
		cVal := C.sr_object_get(cobj, *p)
		m[key] = extractAny(cVal)
	}
	C.sr_free_string_arr(cKeys, C.int(n))
	return m
}

func extractGeometry(geom *C.sr_sr_geometry) any {
	switch geom.tag {
	case C.sr_g_point:
		pt := (*C.sr_sr_g_point)(unsafe.Pointer(&geom.anon0))
		return driver.GeometryPoint{
			Longitude: float64(pt._0.x),
			Latitude:  float64(pt._0.y),
		}
	case C.sr_g_linestring:
		ls := (*C.sr_sr_g_linestring)(unsafe.Pointer(&geom.anon0))
		return extractLineString(ls)
	default:
		return nil
	}
}

func extractLineString(ls *C.sr_sr_g_linestring) driver.GeometryLine {
	n := int(ls._0.len)
	line := make(driver.GeometryLine, n)
	for i := 0; i < n; i++ {
		coord := (*C.sr_sr_g_coord)(unsafe.Pointer(
			uintptr(unsafe.Pointer(ls._0.ptr)) + uintptr(i)*unsafe.Sizeof(C.sr_sr_g_coord{}),
		))
		line[i] = driver.GeometryPoint{
			Longitude: float64(coord.x),
			Latitude:  float64(coord.y),
		}
	}
	return line
}

func extractID(id *C.sr_id_t) string {
	switch id.tag {
	case C.SR_ID_STRING:
		return C.GoString(*(*C.sr_string_t)(unsafe.Pointer(&id.anon0)))
	case C.SR_ID_NUMBER:
		return fmt.Sprintf("%d", *(*C.int64_t)(unsafe.Pointer(&id.anon0)))
	default:
		return "?"
	}
}

// extractResults converts C query results directly to []driver.QueryResult.
func extractResults(results *C.sr_arr_res_t, numStatements int) []driver.QueryResult {
	out := make([]driver.QueryResult, numStatements)
	for i := 0; i < numStatements; i++ {
		cres := (*C.sr_arr_res_t)(unsafe.Pointer(
			uintptr(unsafe.Pointer(results)) + uintptr(i)*unsafe.Sizeof(*results),
		))
		if cres.err.msg != nil {
			out[i].Err = errors.New(C.GoString(cres.err.msg))
			continue
		}
		n := int(cres.ok.len)
		rows := make([]any, n)
		for j := 0; j < n; j++ {
			cv := (*C.sr_value_t)(unsafe.Pointer(
				uintptr(unsafe.Pointer(cres.ok.arr)) + uintptr(j)*unsafe.Sizeof(C.sr_value_t{}),
			))
			rows[j] = extractAny(cv)
		}
		out[i].Value = rows
	}
	return out
}
