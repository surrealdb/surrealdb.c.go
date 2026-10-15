package surrealdb

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"
)

func TestLetAndUnset(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	type pet struct {
		Name string `cbor:"name"`
		Age  int64  `cbor:"age"`
	}

	for _, tc := range []struct {
		name string
		val  any
		want any
	}{
		{"string", "hello", "hello"},
		{"int", int64(42), int64(42)},
		{"float", 1.5, 1.5},
		{"bool", true, true},
		{"nil", nil, nil},
		{"map", map[string]any{"a": int64(1), "b": "two"}, map[string]any{"a": int64(1), "b": "two"}},
		{"struct", pet{Name: "Rex", Age: 3}, map[string]any{"name": "Rex", "age": int64(3)}},
		{"slice", []any{int64(1), "x", map[string]any{"k": true}}, []any{int64(1), "x", map[string]any{"k": true}}},
		{"duration", 90 * time.Second, 90 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Let(ctx, "v", tc.val); err != nil {
				t.Fatalf("Let: %v", err)
			}
			res := mustQuery(t, db, ctx, "RETURN $v", nil)
			if got := returned(res[0].Value); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("$v = %#v, want %#v", got, tc.want)
			}
		})
	}

	if err := db.Let(ctx, "rid", NewRecordID("person", "alice")); err != nil {
		t.Fatalf("Let record id: %v", err)
	}
	res := mustQuery(t, db, ctx, "RETURN record::tb($rid) + ':' + <string>record::id($rid)", nil)
	if returned(res[0].Value) != "person:alice" {
		t.Fatalf("record id variable = %v", res[0].Value)
	}

	if err := db.Unset(ctx, "v"); err != nil {
		t.Fatalf("Unset: %v", err)
	}
	res = mustQuery(t, db, ctx, "RETURN $v", nil)
	if returned(res[0].Value) != nil {
		t.Fatalf("$v after Unset = %v, want none", res[0].Value)
	}

	if err := db.Let(ctx, "bad", make(chan int)); err == nil {
		t.Error("Let with an unsupported type succeeded")
	}
}

func TestHealthAndVersion(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := db.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}

	v, err := db.Version(ctx)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if !regexp.MustCompile(`^\d+\.\d+\.\d+`).MatchString(v.Version) {
		t.Fatalf("Version = %q, want a semver", v.Version)
	}
	t.Logf("engine version %s", v.Version)

	db.Close()
	if err := db.Health(ctx); !errors.Is(err, ErrClosed) {
		t.Errorf("Health after Close: got %v, want ErrClosed", err)
	}
	if _, err := db.Version(ctx); !errors.Is(err, ErrClosed) {
		t.Errorf("Version after Close: got %v, want ErrClosed", err)
	}
}
