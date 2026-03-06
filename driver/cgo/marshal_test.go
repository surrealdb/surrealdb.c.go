package cgo

import (
	"testing"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

func TestMarshalObjectFromStruct(t *testing.T) {
	type Address struct {
		City  string `cbor:"city"`
		State string `cbor:"state"`
	}
	type Person struct {
		Name    string  `cbor:"name"`
		Age     int     `cbor:"age"`
		Active  bool    `cbor:"active"`
		Score   float64 `cbor:"score"`
		Skipped string  `cbor:"-"`
		Address Address `cbor:"address"`
	}

	obj, err := marshalObject(Person{
		Name:    "Alice",
		Age:     30,
		Active:  true,
		Score:   9.5,
		Skipped: "should not appear",
		Address: Address{City: "NYC", State: "NY"},
	})
	if err != nil {
		t.Fatalf("marshalObject: %v", err)
	}
	defer obj.Free()

	if obj.len() != 5 {
		t.Fatalf("expected 5 keys, got %d", obj.len())
	}

	keys := obj.keys()
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	for _, exp := range []string{"name", "age", "active", "score", "address"} {
		if !keySet[exp] {
			t.Errorf("missing key: %s", exp)
		}
	}
	if keySet["skipped"] || keySet["Skipped"] {
		t.Error("skipped field should not be present")
	}
}

func TestMarshalMapVars(t *testing.T) {
	obj, err := marshalVars(map[string]any{
		"name": "test",
		"age":  42,
	})
	if err != nil {
		t.Fatalf("marshalVars: %v", err)
	}
	defer obj.Free()

	if obj.len() != 2 {
		t.Fatalf("expected 2 keys, got %d", obj.len())
	}
}

func TestMarshalOmitempty(t *testing.T) {
	type Record struct {
		ID   driver.RecordID[string] `cbor:"id,omitempty"`
		Name string                  `cbor:"name"`
		Age  int                     `cbor:"age,omitempty"`
	}

	obj, err := marshalObject(Record{Name: "Alice"})
	if err != nil {
		t.Fatalf("marshalObject: %v", err)
	}
	defer obj.Free()

	keys := obj.keys()
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}

	if keySet["id"] {
		t.Error("id with omitempty and zero value should not be present")
	}
	if keySet["age"] {
		t.Error("age with omitempty and zero value should not be present")
	}
	if !keySet["name"] {
		t.Error("name without omitempty should be present")
	}
	if obj.len() != 1 {
		t.Fatalf("expected 1 key, got %d (keys: %v)", obj.len(), keys)
	}
}

func TestMarshalVarsNil(t *testing.T) {
	obj, err := marshalVars(nil)
	if err != nil {
		t.Fatalf("marshalVars(nil): %v", err)
	}
	if obj != nil {
		t.Fatal("expected nil object for nil vars")
	}
}
