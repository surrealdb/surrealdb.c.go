package surrealdb_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	surrealdb "github.com/surrealdb/surrealdb.c.go"
)

// InteropNested is a typed nested object used inside InteropRecord.
type InteropNested struct {
	Foo string `cbor:"foo"`
	Num int64  `cbor:"num"`
	Ok  bool   `cbor:"ok"`
}

// InteropObj is a typed nested object that exercises all supported Go
// numeric types plus other value types in a single SurrealDB object.
// unmarshalAny coerces both drivers' raw representations (C: int64,
// Go CBOR: uint64 for positive ints) into the target field types,
// so this struct verifies that coercion works for every kind.
type InteropObj struct {
	I64  int64         `cbor:"i64"`
	I32  int32         `cbor:"i32"`
	I16  int16         `cbor:"i16"`
	U64  uint64        `cbor:"u64"`
	U32  uint32        `cbor:"u32"`
	U16  uint16        `cbor:"u16"`
	F64  float64       `cbor:"f64"`
	F32  float32       `cbor:"f32"`
	B    bool          `cbor:"b"`
	S    string        `cbor:"s"`
	Dur  time.Duration `cbor:"dur"`
}

// InteropRecord covers the SurrealDB data types that both the CGo and
// pure-Go drivers can round-trip identically through Query[T].
//
// Excluded types (drivers produce different raw Go types):
//   - bytes     — C driver returns tag name, Go SDK returns []byte
type InteropRecord struct {
	ID       string                     `cbor:"id"`
	BoolT    bool                       `cbor:"bool_t"`
	BoolF    bool                       `cbor:"bool_f"`
	IntPos   int64                      `cbor:"int_pos"`
	IntNeg   int64                      `cbor:"int_neg"`
	IntZero  int64                      `cbor:"int_zero"`
	IntBig   int64                      `cbor:"int_big"`
	FloatPos float64                    `cbor:"float_pos"`
	FloatNeg float64                    `cbor:"float_neg"`
	Str      string                     `cbor:"str_val"`
	StrEmpty string                     `cbor:"str_empty"`
	NullVal  any                        `cbor:"null_val"`
	ArrInt   []int64                    `cbor:"arr_int"`
	ArrStr   []string                   `cbor:"arr_str"`
	ArrBool  []bool                     `cbor:"arr_bool"`
	ArrEmpty []int64                    `cbor:"arr_empty"`
	Nested   InteropNested              `cbor:"nested"`
	RID      surrealdb.RecordID[string] `cbor:"rid_val"`
	DurVal   time.Duration              `cbor:"dur_val"`
	DurZero  time.Duration              `cbor:"dur_zero"`
	DtVal    time.Time                  `cbor:"dt_val"`
	DecVal   surrealdb.Decimal          `cbor:"dec_val"`
	DecNeg   surrealdb.Decimal          `cbor:"dec_neg"`
	UuidVal  surrealdb.UUID             `cbor:"uuid_val"`
	Point    surrealdb.GeometryPoint   `cbor:"point_val"`
	ObjMap   map[string]string        `cbor:"obj_map"`
	ObjTyped InteropObj              `cbor:"obj_typed"`
}

// interopCreateSQL populates the record using SurrealQL type-cast functions.
const interopCreateSQL = `
CREATE interop:test SET
    bool_t    = type::bool("true"),
    bool_f    = type::bool("false"),
    int_pos   = type::int("42"),
    int_neg   = type::int("-17"),
    int_zero  = type::int("0"),
    int_big   = type::int("1099511627776"),
    float_pos = type::float("3.14"),
    float_neg = type::float("-2.718"),
    str_val   = type::string("hello world"),
    str_empty = type::string(""),
    null_val  = NULL,
    arr_int   = [type::int("1"), type::int("2"), type::int("3")],
    arr_str   = [type::string("a"), type::string("b"), type::string("c")],
    arr_bool  = [type::bool("true"), type::bool("false"), type::bool("true")],
    arr_empty = [],
    nested    = {
        foo: type::string("bar"),
        num: type::int("99"),
        ok:  type::bool("true")
    },
    rid_val   = type::thing("person", "alice"),
    dur_val   = type::duration("1h30m10s"),
    dur_zero  = type::duration("0s"),
    dt_val    = type::datetime("2024-01-15T10:30:00Z"),
    dec_val   = <decimal> 99.99,
    dec_neg   = <decimal> -0.001,
    uuid_val  = <uuid> "a1a2a3a4-b1b2-c1c2-d1d2-d3d4d5d6d7d8",
    point_val = type::point([1.5, 2.5]),
    obj_map   = { x: type::string("one"), y: type::string("two"), z: type::string("three") },
    obj_typed = {
        i64: type::int("9007199254740992"),
        i32: type::int("2147483647"),
        i16: type::int("32767"),
        u64: type::int("9007199254740992"),
        u32: type::int("2147483647"),
        u16: type::int("32767"),
        f64: type::float("2.718281828"),
        f32: type::float("1.5"),
        b:   type::bool("true"),
        s:   type::string("mixed"),
        dur: type::duration("2h45m")
    }
`

// interopCreateContent populates the record using Go struct vars via CONTENT.
const interopCreateContent = `CREATE interop:test CONTENT $content`

// interopContentVars returns the InteropRecord as a map suitable for
// passing to db.Query as the $content variable.
func interopContentVars() map[string]any {
	return map[string]any{
		"content": map[string]any{
			"bool_t":    true,
			"bool_f":    false,
			"int_pos":   int64(42),
			"int_neg":   int64(-17),
			"int_zero":  int64(0),
			"int_big":   int64(1099511627776),
			"float_pos": 3.14,
			"float_neg": -2.718,
			"str_val":   "hello world",
			"str_empty": "",
			"null_val":  nil,
			"arr_int":   []int64{1, 2, 3},
			"arr_str":   []string{"a", "b", "c"},
			"arr_bool":  []bool{true, false, true},
			"arr_empty": []int64{},
			"nested": map[string]any{
				"foo": "bar",
				"num": int64(99),
				"ok":  true,
			},
			"rid_val":  surrealdb.NewRecordID("person", "alice"),
			"dur_val":  time.Hour + 30*time.Minute + 10*time.Second,
			"dur_zero": time.Duration(0),
			"dt_val":   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			"dec_val":  "99.99",
			"dec_neg":  "-0.001",
			"uuid_val":  "a1a2a3a4-b1b2-c1c2-d1d2-d3d4d5d6d7d8",
			"point_val": surrealdb.GeometryPoint{Longitude: 1.5, Latitude: 2.5},
			"obj_map": map[string]string{"x": "one", "y": "two", "z": "three"},
			"obj_typed": map[string]any{
				"i64": int64(9007199254740992),
				"i32": int64(2147483647),
				"i16": int64(32767),
				"u64": int64(9007199254740992),
				"u32": int64(2147483647),
				"u16": int64(32767),
				"f64": 2.718281828,
				"f32": 1.5,
				"b":   true,
				"s":   "mixed",
				"dur": 2*time.Hour + 45*time.Minute,
			},
		},
	}
}

// InteropContent is InteropRecord without the ID field so it can be
// passed as CONTENT to CREATE without conflicting with the record ID.
type InteropContent struct {
	BoolT    bool                       `cbor:"bool_t"`
	BoolF    bool                       `cbor:"bool_f"`
	IntPos   int64                      `cbor:"int_pos"`
	IntNeg   int64                      `cbor:"int_neg"`
	IntZero  int64                      `cbor:"int_zero"`
	IntBig   int64                      `cbor:"int_big"`
	FloatPos float64                    `cbor:"float_pos"`
	FloatNeg float64                    `cbor:"float_neg"`
	Str      string                     `cbor:"str_val"`
	StrEmpty string                     `cbor:"str_empty"`
	NullVal  any                        `cbor:"null_val"`
	ArrInt   []int64                    `cbor:"arr_int"`
	ArrStr   []string                   `cbor:"arr_str"`
	ArrBool  []bool                     `cbor:"arr_bool"`
	ArrEmpty []int64                    `cbor:"arr_empty"`
	Nested   InteropNested              `cbor:"nested"`
	RID      surrealdb.RecordID[string] `cbor:"rid_val"`
	DurVal   time.Duration              `cbor:"dur_val"`
	DurZero  time.Duration              `cbor:"dur_zero"`
	DtVal    time.Time                  `cbor:"dt_val"`
	DecVal   surrealdb.Decimal          `cbor:"dec_val"`
	DecNeg   surrealdb.Decimal          `cbor:"dec_neg"`
	UuidVal  surrealdb.UUID             `cbor:"uuid_val"`
	Point    surrealdb.GeometryPoint   `cbor:"point_val"`
	ObjMap   map[string]string        `cbor:"obj_map"`
	ObjTyped InteropObj              `cbor:"obj_typed"`
}

// interopContentStruct returns an InteropContent struct wrapped for the
// $content variable. Tests that drivers can marshal Go structs with
// cbor tags into SurrealDB record content.
func interopContentStruct() map[string]any {
	return map[string]any{
		"content": InteropContent{
			BoolT:    true,
			BoolF:    false,
			IntPos:   42,
			IntNeg:   -17,
			IntZero:  0,
			IntBig:   1099511627776,
			FloatPos: 3.14,
			FloatNeg: -2.718,
			Str:      "hello world",
			StrEmpty: "",
			NullVal:  nil,
			ArrInt:   []int64{1, 2, 3},
			ArrStr:   []string{"a", "b", "c"},
			ArrBool:  []bool{true, false, true},
			ArrEmpty: []int64{},
			Nested:   InteropNested{Foo: "bar", Num: 99, Ok: true},
			RID:      surrealdb.NewRecordID("person", "alice"),
			DurVal:   time.Hour + 30*time.Minute + 10*time.Second,
			DurZero:  0,
			DtVal:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			DecVal:   "99.99",
			DecNeg:   "-0.001",
			UuidVal:  "a1a2a3a4-b1b2-c1c2-d1d2-d3d4d5d6d7d8",
			Point:    surrealdb.GeometryPoint{Longitude: 1.5, Latitude: 2.5},
			ObjMap: map[string]string{"x": "one", "y": "two", "z": "three"},
			ObjTyped: InteropObj{
				I64: 9007199254740992,
				I32: 2147483647,
				I16: 32767,
				U64: 9007199254740992,
				U32: 2147483647,
				U16: 32767,
				F64: 2.718281828,
				F32: 1.5,
				B:   true,
				S:   "mixed",
				Dur: 2*time.Hour + 45*time.Minute,
			},
		},
	}
}

type recordPopulator struct {
	name string
	fn   func(t *testing.T, db surrealdb.Driver, ctx context.Context)
}

var populators = []recordPopulator{
	{
		name: "sql",
		fn: func(t *testing.T, db surrealdb.Driver, ctx context.Context) {
			t.Helper()
			_, err := db.Query(ctx, interopCreateSQL, nil)
			require.NoError(t, err)
		},
	},
	{
		name: "map",
		fn: func(t *testing.T, db surrealdb.Driver, ctx context.Context) {
			t.Helper()
			_, err := db.Query(ctx, interopCreateContent, interopContentVars())
			require.NoError(t, err)
		},
	},
	{
		name: "struct",
		fn: func(t *testing.T, db surrealdb.Driver, ctx context.Context) {
			t.Helper()
			_, err := db.Query(ctx, interopCreateContent, interopContentStruct())
			require.NoError(t, err)
		},
	},
}

func TestInteropTypes(t *testing.T) {
	wsURL := startSurrealDBContainer(t)

	endpoints := []struct {
		name     string
		endpoint string
	}{
		{"cgo", "mem://"},
		{"go", wsURL},
	}

	// Collect results for all 4 combinations, then cross-compare.
	results := make(map[string]InteropRecord)
	for _, ep := range endpoints {
		for _, pop := range populators {
			key := ep.name + "/" + pop.name
			t.Run(key, func(t *testing.T) {
				rec := runInterop(t, ep.endpoint, pop, key)
				results[key] = rec
			})
		}
	}

	// All 6 results should be identical.
	keys := []string{"cgo/sql", "cgo/map", "cgo/struct", "go/sql", "go/map", "go/struct"}
	for i := 1; i < len(keys); i++ {
		assert.Equal(t, results[keys[0]], results[keys[i]],
			"%s and %s should produce identical results", keys[0], keys[i])
	}
}

func runInterop(t *testing.T, endpoint string, pop recordPopulator, key string) InteropRecord {
	t.Helper()
	ctx := context.Background()

	db, err := surrealdb.Open(ctx, endpoint)
	require.NoError(t, err)
	defer db.Close()

	ns := fmt.Sprintf("interop_%s", key)
	require.NoError(t, db.Use(ctx, ns, "interop_db"))

	pop.fn(t, db, ctx)
	return selectInteropRecord(t, db, ctx)
}

func selectInteropRecord(t *testing.T, db surrealdb.Driver, ctx context.Context) InteropRecord {
	t.Helper()
	results, err := surrealdb.Query[InteropRecord](ctx, db, "SELECT * FROM interop:test", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NoError(t, results[0].Err())

	got := results[0].Values()
	require.Len(t, got, 1)
	return got[0]
}
