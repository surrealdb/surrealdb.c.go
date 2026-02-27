package surrealdbc

// #include "surrealdb.h"
import "C"

import "unsafe"

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

func (o *object) insertInt(key string, value int) {
	cKey := cstring(key)
	C.sr_object_insert_int(&o.c, cKey, C.int(value))
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
