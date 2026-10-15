package cgo

/*
#include "surrealdb.h"

// Builds an sr_value_t array from an array of value pointers.
static struct sr_value_t *build_array(struct sr_value_t **values, int count) {
    struct sr_value_t *arr_val = sr_value_array();
    for (int i = 0; i < count; i++) {
        struct sr_array_t *old = arr_val->sr_value_array;
        arr_val->sr_value_array = sr_array_push(old, values[i]);
        if (old) sr_array_free(old);
    }
    return arr_val;
}
*/
import "C"

import (
	"time"
	"unsafe"
)

type object struct {
	c     C.sr_object_t
	freed bool
}

func newObject() *object {
	return &object{c: C.sr_object_new()}
}

func (o *object) Free() {
	if !o.freed {
		C.sr_free_object(o.c)
		o.freed = true
	}
}

func (o *object) insertStr(key, value string) {
	cKey := cstring(key)
	cVal := cstring(value)
	C.sr_object_insert_str(&o.c, cKey, cVal)
	freeCString(cKey)
	freeCString(cVal)
}

func (o *object) insertInt(key string, value int64) {
	cKey := cstring(key)
	cVal := C.sr_value_int(C.int64_t(value))
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

func (o *object) insertFloat(key string, value float64) {
	cKey := cstring(key)
	C.sr_object_insert_double(&o.c, cKey, C.double(value))
	freeCString(cKey)
}

func (o *object) insertBool(key string, value bool) {
	cKey := cstring(key)
	cVal := C.sr_value_bool(C._Bool(value))
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

func (o *object) insertThing(key, table, id string) {
	cKey := cstring(key)
	cTable := cstring(table)
	cID := cstring(id)
	cVal := C.sr_value_thing(cTable, cID)
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
	freeCString(cTable)
	freeCString(cID)
}

func (o *object) insertBytes(key string, data []byte) {
	cKey := cstring(key)
	var cVal *C.sr_value_t
	if len(data) > 0 {
		cVal = C.sr_value_bytes((*C.uint8_t)(unsafe.Pointer(&data[0])), C.int(len(data)))
	} else {
		cVal = C.sr_value_bytes(nil, 0)
	}
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

func (o *object) insertPoint(key string, x, y float64) {
	cKey := cstring(key)
	cVal := C.sr_value_point(C.double(x), C.double(y))
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

func (o *object) insertLineString(key string, coords []C.sr_sr_g_coord) {
	cKey := cstring(key)
	var cVal *C.sr_value_t
	if len(coords) > 0 {
		cVal = C.sr_value_linestring(&coords[0], C.int(len(coords)))
	} else {
		cVal = C.sr_value_linestring(nil, 0)
	}
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

func (o *object) insertNull(key string) {
	cKey := cstring(key)
	cVal := C.sr_value_null()
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

func (o *object) insertDuration(key string, d time.Duration) {
	cKey := cstring(key)
	secs := C.uint64_t(d / time.Second)
	nanos := C.uint32_t((d % time.Second) / time.Nanosecond)
	cVal := C.sr_value_duration(secs, nanos)
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

func (o *object) insertDatetime(key string, t time.Time) {
	cKey := cstring(key)
	cRFC := cstring(t.UTC().Format(time.RFC3339Nano))
	cVal := C.sr_value_datetime(cRFC)
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
	freeCString(cRFC)
}

// buildArray returns a new array value holding copies of cValues. The
// caller still owns cValues.
func buildArray(cValues []*C.sr_value_t) *C.sr_value_t {
	if len(cValues) == 0 {
		return C.sr_value_array()
	}
	return C.build_array(&cValues[0], C.int(len(cValues)))
}

func (o *object) insertArray(key string, cValues []*C.sr_value_t) {
	cKey := cstring(key)
	cArr := buildArray(cValues)
	C.sr_object_insert(&o.c, cKey, cArr)
	C.sr_value_free(cArr)
	freeCString(cKey)
	for _, v := range cValues {
		C.sr_value_free(v)
	}
}

func (o *object) insertObject(key string, nested *object) {
	cKey := cstring(key)
	cVal := C.sr_value_object(&nested.c)
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

func (o *object) len() int {
	return int(C.sr_object_len(&o.c))
}

func (o *object) cptr() *C.sr_object_t {
	return &o.c
}

func (o *object) keys() []string {
	var cKeys **C.char
	n := int(C.sr_object_keys(&o.c, &cKeys))
	if n <= 0 {
		return nil
	}
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		p := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cKeys)) + uintptr(i)*unsafe.Sizeof(*cKeys)))
		keys[i] = C.GoString(*p)
	}
	C.sr_free_string_arr(cKeys, C.int(n))
	return keys
}
