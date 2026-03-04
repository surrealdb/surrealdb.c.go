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

// InteropRecord covers the SurrealDB data types that both the CGo and
// pure-Go drivers can round-trip identically through Query[T].
//
// Excluded types (drivers produce different raw Go types):
//   - bytes     — C driver returns tag name, Go SDK returns []byte
//   - geometry  — C driver returns tag name, Go SDK returns custom type
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
	DecVal   surrealdb.Decimal            `cbor:"dec_val"`
	DecNeg   surrealdb.Decimal            `cbor:"dec_neg"`
	UuidVal  surrealdb.UUID               `cbor:"uuid_val"`
}

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
    uuid_val  = <uuid> "a1a2a3a4-b1b2-c1c2-d1d2-d3d4d5d6d7d8"
`

func TestInteropTypes(t *testing.T) {
	wsURL := startSurrealDBContainer(t)

	cgoResult := queryInteropRecord(t, "mem://")
	goResult := queryInteropRecord(t, wsURL)

	assert.Equal(t, cgoResult, goResult,
		"CGo and Go drivers should produce identical typed results")
}

func queryInteropRecord(t *testing.T, endpoint string) InteropRecord {
	t.Helper()
	ctx := context.Background()

	db, err := surrealdb.Open(ctx, endpoint)
	require.NoError(t, err)
	defer db.Close()

	ns := fmt.Sprintf("interop_ns_%s", endpoint)
	require.NoError(t, db.Use(ctx, ns, "interop_db"))

	_, err = db.Query(ctx, interopCreateSQL, nil)
	require.NoError(t, err)

	results, err := surrealdb.Query[InteropRecord](ctx, db, "SELECT * FROM interop:test", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NoError(t, results[0].Err())

	got := results[0].Values()
	require.Len(t, got, 1)
	return got[0]
}
