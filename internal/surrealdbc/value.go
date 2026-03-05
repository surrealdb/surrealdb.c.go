package surrealdbc

// #include "surrealdb.h"
import "C"

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

// ValueTag identifies the type of a Value.
type ValueTag int

const (
	ValueNone     ValueTag = C.SR_VALUE_NONE
	ValueNull     ValueTag = C.SR_VALUE_NULL
	ValueBool     ValueTag = C.SR_VALUE_BOOL
	ValueNumber   ValueTag = C.SR_VALUE_NUMBER
	ValueString   ValueTag = C.SR_VALUE_STRAND
	ValueDuration ValueTag = C.SR_VALUE_DURATION
	ValueDatetime ValueTag = C.SR_VALUE_DATETIME
	ValueUUID     ValueTag = C.SR_VALUE_UUID
	ValueArray    ValueTag = C.SR_VALUE_ARRAY
	ValueObject   ValueTag = C.SR_VALUE_OBJECT
	ValueGeometry ValueTag = C.SR_GEOMETRY_OBJECT
	ValueBytes    ValueTag = C.SR_VALUE_BYTES
	ValueThing    ValueTag = C.SR_VALUE_THING
)

var tagNames = map[ValueTag]string{
	ValueNone:     "None",
	ValueNull:     "Null",
	ValueBool:     "Bool",
	ValueNumber:   "Number",
	ValueString:   "String",
	ValueDuration: "Duration",
	ValueDatetime: "Datetime",
	ValueUUID:     "UUID",
	ValueArray:    "Array",
	ValueObject:   "Object",
	ValueGeometry: "Geometry",
	ValueBytes:    "Bytes",
	ValueThing:    "Thing",
}

func (t ValueTag) String() string {
	if name, ok := tagNames[t]; ok {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", int(t))
}

// Value wraps a single SurrealDB value returned from a query.
// Values are copied out of the C result set — they do not hold C resources.
type Value struct {
	tag    ValueTag
	str    string
	num    int64
	float  float64
	b      bool
	dur    time.Duration
	bytes  []byte
	geom   any
	values []Value
	object []KV
	thing  [2]string
}

// KV is a key-value pair within an Object value.
type KV struct {
	Key   string
	Value Value
}

// Tag returns the type tag of this value.
func (v *Value) Tag() ValueTag { return v.tag }

// AsBool returns the boolean value. Panics if Tag != ValueBool.
func (v *Value) AsBool() bool { return v.b }

// AsInt returns the integer value. Panics if Tag != ValueNumber or not an int.
func (v *Value) AsInt() int64 { return v.num }

// AsFloat returns the float value.
func (v *Value) AsFloat() float64 { return v.float }

// AsString returns the string value.
func (v *Value) AsString() string { return v.str }

// AsArray returns child values for an Array value.
func (v *Value) AsArray() []Value { return v.values }

// AsObject returns key-value pairs for an Object value.
func (v *Value) AsObject() []KV { return v.object }

// AsDuration returns the duration value.
func (v *Value) AsDuration() time.Duration { return v.dur }

// AsUUID returns the UUID string for a UUID value.
func (v *Value) AsUUID() string { return v.str }

// AsBytes returns the raw bytes for a Bytes value.
func (v *Value) AsBytes() []byte { return v.bytes }

// AsGeometry returns the extracted geometry value, or nil if unsupported.
func (v *Value) AsGeometry() any { return v.geom }

// AsThingTable returns the table name for a Thing (record ID) value.
func (v *Value) AsThingTable() string { return v.thing[0] }

// AsThingID returns the id part for a Thing (record ID) value.
func (v *Value) AsThingID() string { return v.thing[1] }

// String returns a human-readable representation of the value.
func (v *Value) String() string {
	switch v.tag {
	case ValueNone:
		return "NONE"
	case ValueNull:
		return "NULL"
	case ValueBool:
		return fmt.Sprintf("%t", v.b)
	case ValueNumber:
		if v.str != "" {
			return v.str
		}
		if v.num != 0 {
			return fmt.Sprintf("%d", v.num)
		}
		return fmt.Sprintf("%g", v.float)
	case ValueDuration:
		return v.dur.String()
	case ValueUUID:
		return v.str
	case ValueString:
		return v.str
	case ValueDatetime:
		return v.str
	case ValueThing:
		return v.thing[0] + ":" + v.thing[1]
	case ValueArray:
		return fmt.Sprintf("[%d values]", len(v.values))
	case ValueObject:
		return fmt.Sprintf("{%d fields}", len(v.object))
	default:
		return v.tag.String()
	}
}

// QueryResult holds the outcome of a single statement within a query.
// A multi-statement query produces multiple QueryResults.
type QueryResult struct {
	values []Value
	err    error
}

// Values returns the result records for this statement.
func (r *QueryResult) Values() []Value { return r.values }

// Err returns the per-statement error, if any.
func (r *QueryResult) Err() error { return r.err }

func extractValue(cv *C.sr_value_t) Value {
	v := Value{tag: ValueTag(cv.tag)}
	switch v.tag {
	case ValueBool:
		v.b = bool(*(*C._Bool)(unsafe.Pointer(&cv.anon0)))
	case ValueNumber:
		extractNumber(&v, (*C.sr_number_t)(unsafe.Pointer(&cv.anon0)))
	case ValueString:
		v.str = C.GoString(*(*C.sr_string_t)(unsafe.Pointer(&cv.anon0)))
	case ValueDuration:
		d := (*C.sr_duration_t)(unsafe.Pointer(&cv.anon0))
		v.dur = time.Duration(d.secs)*time.Second + time.Duration(d.nanos)*time.Nanosecond
	case ValueDatetime:
		v.str = C.GoString(*(*C.sr_string_t)(unsafe.Pointer(&cv.anon0)))
	case ValueArray:
		arr := *(**C.sr_array_t)(unsafe.Pointer(&cv.anon0))
		n := int(arr.len)
		v.values = make([]Value, n)
		for i := 0; i < n; i++ {
			elem := (*C.sr_value_t)(unsafe.Pointer(
				uintptr(unsafe.Pointer(arr.arr)) + uintptr(i)*unsafe.Sizeof(C.sr_value_t{}),
			))
			v.values[i] = extractValue(elem)
		}
	case ValueObject:
		cobj := (*C.sr_object_t)(unsafe.Pointer(&cv.anon0))
		v.object = extractObject(cobj)
	case ValueUUID:
		cuuid := (*C.sr_uuid_t)(unsafe.Pointer(&cv.anon0))
		b := cuuid._0
		v.str = fmt.Sprintf(
			"%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
			b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7],
			b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15],
		)
	case ValueBytes:
		cb := (*C.sr_bytes_t)(unsafe.Pointer(&cv.anon0))
		if cb.len > 0 {
			v.bytes = C.GoBytes(unsafe.Pointer(cb.arr), cb.len)
		} else {
			v.bytes = []byte{}
		}
	case ValueGeometry:
		geom := (*C.sr_sr_geometry)(unsafe.Pointer(&cv.anon0))
		v.geom = extractGeometry(geom)
	case ValueThing:
		thing := (*C.sr_thing_t)(unsafe.Pointer(&cv.anon0))
		v.thing[0] = C.GoString(thing.table)
		v.thing[1] = extractID(&thing.id)
	default:
		// All supported value types are extracted above.
	}
	return v
}

func extractNumber(v *Value, cn *C.sr_number_t) {
	switch cn.tag {
	case C.SR_NUMBER_INT:
		v.num = int64(*(*C.int64_t)(unsafe.Pointer(&cn.anon0)))
	case C.SR_NUMBER_FLOAT:
		v.float = float64(*(*C.double)(unsafe.Pointer(&cn.anon0)))
	case C.SR_NUMBER_DECIMAL:
		v.str = C.GoString(*(*C.sr_string_t)(unsafe.Pointer(&cn.anon0)))
	}
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

func extractObject(cobj *C.sr_object_t) []KV {
	var cKeys **C.char
	n := int(C.sr_object_keys(cobj, &cKeys))
	if n <= 0 {
		return nil
	}
	kvs := make([]KV, n)
	for i := 0; i < n; i++ {
		p := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cKeys)) + uintptr(i)*unsafe.Sizeof(*cKeys)))
		key := C.GoString(*p)
		cVal := C.sr_object_get(cobj, *p)
		kvs[i] = KV{Key: key, Value: extractValue(cVal)}
	}
	C.sr_free_string_arr(cKeys, C.int(n))
	return kvs
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

func extractResults(results *C.sr_arr_res_t, numStatements int) []QueryResult {
	out := make([]QueryResult, numStatements)
	for i := 0; i < numStatements; i++ {
		cres := (*C.sr_arr_res_t)(unsafe.Pointer(
			uintptr(unsafe.Pointer(results)) + uintptr(i)*unsafe.Sizeof(*results),
		))
		if cres.err.msg != nil {
			out[i].err = errors.New(C.GoString(cres.err.msg))
			continue
		}
		n := int(cres.ok.len)
		out[i].values = make([]Value, n)
		for j := 0; j < n; j++ {
			cv := (*C.sr_value_t)(unsafe.Pointer(
				uintptr(unsafe.Pointer(cres.ok.arr)) + uintptr(j)*unsafe.Sizeof(C.sr_value_t{}),
			))
			out[i].values[j] = extractValue(cv)
		}
	}
	return out
}
