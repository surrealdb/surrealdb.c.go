# Upstream fix ideas for surrealdb.c

This document collects limitations in the surrealdb.c C API that affect
surrealdb.c.go, along with proposed upstream fixes.

## Numeric Record ID in query variables

### Problem

The C API exposes a single function for creating Thing (record ID) values:

```c
struct sr_value_t *sr_value_thing(const char *table, const char *id);
```

Both the table and the ID are strings. SurrealDB internally distinguishes
between `person:123` (numeric ID) and `person:"123"` (string ID); they are
different records. Because `sr_value_thing` always creates a string ID, it
is impossible to pass a numeric record ID as a query variable through the C
API. A query like `SELECT * FROM $rid` with `$rid = person:123` will not
match a record created with `CREATE person:123 ...`.

The internal `sr_id_t` struct already supports numeric IDs:

```c
typedef enum sr_id_t_Tag {
  SR_ID_NUMBER,
  SR_ID_STRING,
  SR_ID_ARRAY,
  SR_ID_OBJECT,
} sr_id_t_Tag;

typedef struct sr_id_t {
  sr_id_t_Tag tag;
  union {
    struct { int64_t sr_id_number; };
    struct { sr_string_t sr_id_string; };
    struct { struct sr_array_t *sr_id_array; };
    struct { struct sr_object_t sr_id_object; };
  };
} sr_id_t;
```

### Proposed upstream fix

Add typed constructors alongside the existing `sr_value_thing`:

```c
struct sr_value_t *sr_value_thing_int(const char *table, int64_t id);
struct sr_value_t *sr_value_thing_array(const char *table, const struct sr_array_t *id);
struct sr_value_t *sr_value_thing_object(const char *table, const struct sr_object_t *id);
```

### Workaround (not currently applied)

Until the upstream API is extended, a CGo shim can create a Thing with a
numeric ID by allocating via `sr_value_thing` and patching the `sr_id_t`
union in-place. This leaks the short dummy string that `sr_value_thing`
allocated for the original string ID, since there is no public API to free
just the ID portion of a Thing value.

```c
static struct sr_value_t *sr_value_thing_int(const char *table, int64_t id) {
    struct sr_value_t *val = sr_value_thing(table, "0");
    val->sr_value_thing.id.tag = SR_ID_NUMBER;
    val->sr_value_thing.id.sr_id_number = id;
    return val;
}
```

The corresponding Go side would add an `insertThingInt` method that calls
this helper and dispatch to it from `insertValue` when the RecordID's ID
field has an integer reflect kind:

```go
func (o *object) insertThingInt(key, table string, id int64) {
    cKey := cstring(key)
    cTable := cstring(table)
    cVal := C.sr_value_thing_int(cTable, C.int64_t(id))
    C.sr_object_insert(&o.c, cKey, cVal)
    C.sr_value_free(cVal)
    freeCString(cKey)
    freeCString(cTable)
}
```

And the dispatch in `insertValue` (marshal.go):

```go
if fv.Kind() == reflect.Struct && driver.IsRecordIDType(fv.Type()) {
    table := fv.Field(0).String()
    idField := fv.Field(1)
    if idField.Kind() == reflect.Interface && !idField.IsNil() {
        idField = idField.Elem()
    }
    switch idField.Kind() {
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        obj.insertThingInt(key, table, idField.Int())
    case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
        obj.insertThingInt(key, table, int64(idField.Uint()))
    default:
        obj.insertThing(key, table, fmt.Sprintf("%v", idField.Interface()))
    }
    return nil
}
```

This workaround is not applied in the codebase because it relies on
patching C struct internals and leaks memory. The driver test skips the
numeric RecordID round-trip for embedded (C) endpoints accordingly.
