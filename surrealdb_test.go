package surrealdb

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// testEndpoint returns the SurrealDB endpoint to use for tests.
// Set SURREALDB_ENDPOINT to override (default: "mem://").
// For file-backed schemes (rocksdb://, surrealkv://), a per-test temp
// directory is appended automatically when no path is provided.
func testEndpoint(t *testing.T) string {
	t.Helper()
	ep := os.Getenv("SURREALDB_ENDPOINT")
	if ep == "" {
		ep = "mem://"
	}
	for _, scheme := range []string{"rocksdb://", "surrealkv://"} {
		if strings.HasPrefix(ep, scheme) && len(ep) == len(scheme) {
			return scheme + t.TempDir()
		}
	}
	return ep
}

// openTestDB is a helper that opens a DB with namespace/database set.
// The backend is determined by testEndpoint (SURREALDB_ENDPOINT env var).
func openTestDB(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, testEndpoint(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Use(ctx, "test_ns", "test_db"); err != nil {
		db.Close()
		t.Fatalf("Use: %v", err)
	}
	return db
}

func TestOpenClose(t *testing.T) {
	db, err := Open(context.Background(), testEndpoint(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	db.Close()
	// Double close should not panic.
	db.Close()
}

func TestVersion(t *testing.T) {
	db, err := Open(context.Background(), testEndpoint(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	ver, err := db.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if ver == "" {
		t.Fatal("Version returned empty string")
	}
	t.Logf("SurrealDB version: %s", ver)
}

func TestHealth(t *testing.T) {
	db, err := Open(context.Background(), testEndpoint(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.Health(); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestUse(t *testing.T) {
	db, err := Open(context.Background(), testEndpoint(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.Use(context.Background(), "my_ns", "my_db"); err != nil {
		t.Fatalf("Use: %v", err)
	}
}

func TestQueryCreateAndSelect(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	// CREATE
	results, err := db.Query(ctx, `CREATE person:alice SET name = "Alice", age = 30`, nil)
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 statement result, got %d", len(results))
	}
	if results[0].Err() != nil {
		t.Fatalf("CREATE statement error: %v", results[0].Err())
	}
	if len(results[0].Values()) != 1 {
		t.Fatalf("expected 1 created record, got %d", len(results[0].Values()))
	}

	// SELECT
	results, err = db.Query(ctx, "SELECT * FROM person", nil)
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 statement result, got %d", len(results))
	}
	vals := results[0].Values()
	if len(vals) != 1 {
		t.Fatalf("expected 1 record, got %d", len(vals))
	}

	// Verify the record is an Object with expected fields
	if vals[0].Tag() != ValueObject {
		t.Fatalf("expected ValueObject, got %s", vals[0].Tag())
	}
	kvs := vals[0].AsObject()
	found := map[string]bool{}
	for _, kv := range kvs {
		found[kv.Key] = true
		switch kv.Key {
		case "name":
			if kv.Value.Tag() != ValueString {
				t.Errorf("name: expected ValueString, got %s", kv.Value.Tag())
			}
			if kv.Value.AsString() != "Alice" {
				t.Errorf("name: expected Alice, got %s", kv.Value.AsString())
			}
		case "age":
			if kv.Value.Tag() != ValueNumber {
				t.Errorf("age: expected ValueNumber, got %s", kv.Value.Tag())
			}
			if kv.Value.AsInt() != 30 {
				t.Errorf("age: expected 30, got %d", kv.Value.AsInt())
			}
		case "id":
			if kv.Value.Tag() != ValueThing {
				t.Errorf("id: expected ValueThing, got %s", kv.Value.Tag())
			}
			if kv.Value.AsThingTable() != "person" {
				t.Errorf("id table: expected person, got %s", kv.Value.AsThingTable())
			}
		}
	}
	if !found["name"] || !found["age"] || !found["id"] {
		t.Errorf("missing expected keys; got: %v", found)
	}
}

func TestQueryParameterized(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	db.Query(ctx, `CREATE person:a SET name = "A", age = 20`, nil)
	db.Query(ctx, `CREATE person:b SET name = "B", age = 30`, nil)
	db.Query(ctx, `CREATE person:c SET name = "C", age = 40`, nil)

	vars := NewObject()
	vars.InsertInt("min_age", 25)
	defer vars.Free()

	results, err := db.Query(ctx, "SELECT * FROM person WHERE age >= $min_age ORDER BY age ASC", vars)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	vals := results[0].Values()
	if len(vals) != 2 {
		t.Fatalf("expected 2 records, got %d", len(vals))
	}
}

func TestQueryMultiStatement(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	db.Query(ctx, `CREATE person:a SET name = "A", age = 20`, nil)
	db.Query(ctx, `CREATE person:b SET name = "B", age = 30`, nil)

	results, err := db.Query(ctx,
		"SELECT count() AS total FROM person GROUP ALL;"+
			"SELECT math::mean(age) AS avg_age FROM person GROUP ALL;",
		nil,
	)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 statement results, got %d", len(results))
	}

	// Statement 1: count
	if results[0].Err() != nil {
		t.Fatalf("statement 1 error: %v", results[0].Err())
	}
	countVals := results[0].Values()
	if len(countVals) != 1 {
		t.Fatalf("expected 1 count result, got %d", len(countVals))
	}

	// Statement 2: average
	if results[1].Err() != nil {
		t.Fatalf("statement 2 error: %v", results[1].Err())
	}
}

func TestObject(t *testing.T) {
	obj := NewObject()
	defer obj.Free()

	obj.InsertStr("name", "test")
	obj.InsertInt("count", 42)
	obj.InsertFloat("ratio", 3.14)
	obj.InsertBool("active", true)

	if obj.Len() != 4 {
		t.Fatalf("expected 4 keys, got %d", obj.Len())
	}

	keys := obj.Keys()
	if len(keys) != 4 {
		t.Fatalf("expected 4 keys, got %d", len(keys))
	}

	expected := map[string]bool{"name": false, "count": false, "ratio": false, "active": false}
	for _, k := range keys {
		if _, ok := expected[k]; !ok {
			t.Errorf("unexpected key: %s", k)
		}
		expected[k] = true
	}
	for k, found := range expected {
		if !found {
			t.Errorf("missing key: %s", k)
		}
	}
}

func TestMarshalObject(t *testing.T) {
	type Address struct {
		City  string `db:"city"`
		State string `db:"state"`
	}
	type Person struct {
		Name    string  `db:"name"`
		Age     int     `db:"age"`
		Active  bool    `db:"active"`
		Score   float64 `db:"score"`
		Skipped string  `db:"-"`
		Address Address `db:"address"`
	}

	obj, err := MarshalObject(Person{
		Name:    "Alice",
		Age:     30,
		Active:  true,
		Score:   9.5,
		Skipped: "should not appear",
		Address: Address{City: "NYC", State: "NY"},
	})
	if err != nil {
		t.Fatalf("MarshalObject: %v", err)
	}
	defer obj.Free()

	// Should have 5 keys (Skipped is excluded)
	if obj.Len() != 5 {
		t.Fatalf("expected 5 keys, got %d", obj.Len())
	}

	keys := obj.Keys()
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	for _, expected := range []string{"name", "age", "active", "score", "address"} {
		if !keySet[expected] {
			t.Errorf("missing key: %s", expected)
		}
	}
	if keySet["skipped"] || keySet["Skipped"] {
		t.Error("skipped field should not be present")
	}
}

func TestMarshalObjectUsedInQuery(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	type Person struct {
		Name   string `db:"name"`
		Age    int    `db:"age"`
		Active bool   `db:"active"`
	}

	data, err := MarshalObject(Person{"Eve", 22, true})
	if err != nil {
		t.Fatalf("MarshalObject: %v", err)
	}

	vars := NewObject()
	vars.InsertObject("data", data)
	data.Free()

	results, err := db.Query(ctx, "CREATE person:eve CONTENT $data", vars)
	vars.Free()
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}
	if results[0].Err() != nil {
		t.Fatalf("CREATE statement error: %v", results[0].Err())
	}

	// Verify
	results, err = db.Query(ctx, "SELECT * FROM person:eve", nil)
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	vals := results[0].Values()
	if len(vals) != 1 {
		t.Fatalf("expected 1 record, got %d", len(vals))
	}
	kvs := vals[0].AsObject()
	for _, kv := range kvs {
		if kv.Key == "name" && kv.Value.AsString() != "Eve" {
			t.Errorf("expected name=Eve, got %s", kv.Value.AsString())
		}
		if kv.Key == "age" && kv.Value.AsInt() != 22 {
			t.Errorf("expected age=22, got %d", kv.Value.AsInt())
		}
		if kv.Key == "active" && !kv.Value.AsBool() {
			t.Error("expected active=true")
		}
	}
}

func TestLiveAndKill(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	lq, err := db.Live(ctx, "person")
	if err != nil {
		t.Fatalf("Live: %v", err)
	}

	// Write a record from a separate goroutine so the notification arrives.
	go func() {
		time.Sleep(50 * time.Millisecond)
		db.Query(ctx, `CREATE person:livetest SET name = "Live"`, nil)
	}()

	select {
	case n, ok := <-lq.Ch:
		if !ok {
			t.Fatal("channel closed before receiving notification")
		}
		if n.Action != ActionCreate {
			t.Errorf("expected ActionCreate, got %s", n.Action)
		}
		if n.QueryID == "" {
			t.Error("QueryID is empty")
		}
		if n.Data.Tag() != ValueObject {
			t.Errorf("expected ValueObject, got %s", n.Data.Tag())
		}
		t.Logf("notification: action=%s queryID=%s data=%s", n.Action, n.QueryID, n.Data.String())
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for notification")
	}

	db.Kill(lq)
}
