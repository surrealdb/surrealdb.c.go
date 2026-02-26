package surrealdb

// #include "surrealdb.h"
import "C"

import "unsafe"

// Object is a key-value map used to pass structured data to SurrealDB.
// It wraps the C sr_object_t type.
// Call Free when the object is no longer needed.
type Object struct {
	c     C.sr_object_t
	freed bool
}

// NewObject creates a new empty Object.
func NewObject() *Object {
	return &Object{c: C.sr_object_new()}
}

// Free releases the memory held by this Object.
// Must be called when the Object is no longer needed.
func (o *Object) Free() {
	if !o.freed {
		C.sr_free_object(o.c)
		o.freed = true
	}
}

// InsertStr inserts a string value for the given key.
func (o *Object) InsertStr(key, value string) {
	cKey := cstring(key)
	cVal := cstring(value)
	C.sr_object_insert_str(&o.c, cKey, cVal)
	freeCString(cKey)
	freeCString(cVal)
}

// InsertInt inserts an integer value for the given key.
func (o *Object) InsertInt(key string, value int) {
	cKey := cstring(key)
	C.sr_object_insert_int(&o.c, cKey, C.int(value))
	freeCString(cKey)
}

// InsertFloat inserts a float64 value for the given key.
func (o *Object) InsertFloat(key string, value float64) {
	cKey := cstring(key)
	C.sr_object_insert_double(&o.c, cKey, C.double(value))
	freeCString(cKey)
}

// InsertBool inserts a boolean value for the given key.
func (o *Object) InsertBool(key string, value bool) {
	cKey := cstring(key)
	cVal := C.sr_value_bool(C._Bool(value))
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

// InsertObject inserts a nested Object as a value for the given key.
// The nested object's ownership is not transferred — you must still Free it yourself.
func (o *Object) InsertObject(key string, nested *Object) {
	cKey := cstring(key)
	cVal := C.sr_value_object(&nested.c)
	C.sr_object_insert(&o.c, cKey, cVal)
	C.sr_value_free(cVal)
	freeCString(cKey)
}

// Len returns the number of key-value pairs in the object.
func (o *Object) Len() int {
	return int(C.sr_object_len(&o.c))
}

// cptr returns the C pointer to the underlying sr_object_t.
// Used internally by query.go and struct.go.
func (o *Object) cptr() *C.sr_object_t {
	return &o.c
}

// Keys returns all keys in the object.
func (o *Object) Keys() []string {
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
