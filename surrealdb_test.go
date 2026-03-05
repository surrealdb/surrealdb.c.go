package surrealdb

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

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

func openTestDB(t *testing.T) Driver {
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

func mustQuery(t *testing.T, db Driver, ctx context.Context, sql string, vars any) []QueryResult {
	t.Helper()
	results, err := db.Query(ctx, sql, vars)
	if err != nil {
		t.Fatalf("Query(%q): %v", sql, err)
	}
	return results
}

func TestOpenClose(t *testing.T) {
	db, err := Open(context.Background(), testEndpoint(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	db.Close()
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

	results, err := db.Query(ctx, `CREATE person:alice SET name = "Alice", age = 30`, nil)
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 statement result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("CREATE statement error: %v", results[0].Err)
	}

	rows, ok := results[0].Value.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", results[0].Value)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 created record, got %d", len(rows))
	}

	results, err = db.Query(ctx, "SELECT * FROM person", nil)
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}

	rows, ok = results[0].Value.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", results[0].Value)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 record, got %d", len(rows))
	}

	rec, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", rows[0])
	}
	if rec["name"] != "Alice" {
		t.Errorf("name: expected Alice, got %v", rec["name"])
	}
}

func TestQueryParameterized(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, `CREATE person:a SET name = "A", age = 20`, nil)
	mustQuery(t, db, ctx, `CREATE person:b SET name = "B", age = 30`, nil)
	mustQuery(t, db, ctx, `CREATE person:c SET name = "C", age = 40`, nil)

	results, err := db.Query(ctx,
		"SELECT * FROM person WHERE age >= $min_age ORDER BY age ASC",
		map[string]any{"min_age": 25},
	)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	rows, ok := results[0].Value.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", results[0].Value)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 records, got %d", len(rows))
	}
}

func TestQueryMultiStatement(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, `CREATE person:a SET name = "A", age = 20`, nil)
	mustQuery(t, db, ctx, `CREATE person:b SET name = "B", age = 30`, nil)

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
	if results[0].Err != nil {
		t.Fatalf("statement 1 error: %v", results[0].Err)
	}
	if results[1].Err != nil {
		t.Fatalf("statement 2 error: %v", results[1].Err)
	}
}

func TestStructVarsUsedInQuery(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	type Person struct {
		Name   string `cbor:"name"`
		Age    int    `cbor:"age"`
		Active bool   `cbor:"active"`
	}

	type Vars struct {
		Data Person `cbor:"data"`
	}

	results, err := db.Query(ctx, "CREATE person:eve CONTENT $data",
		Vars{Data: Person{"Eve", 22, true}},
	)
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}
	if results[0].Err != nil {
		t.Fatalf("CREATE statement error: %v", results[0].Err)
	}

	results, err = db.Query(ctx, "SELECT * FROM person:eve", nil)
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}

	rows, ok := results[0].Value.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", results[0].Value)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 record, got %d", len(rows))
	}

	rec, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", rows[0])
	}
	if rec["name"] != "Eve" {
		t.Errorf("expected name=Eve, got %v", rec["name"])
	}
}

func TestLiveAndKill(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, "DEFINE TABLE person SCHEMALESS", nil)

	id, ch, kill, err := db.Live(ctx, "person")
	if err != nil {
		t.Fatalf("Live: %v", err)
	}
	_ = id

	go func() {
		time.Sleep(50 * time.Millisecond)
		mustQuery(t, db, ctx, `CREATE person:livetest SET name = "Live"`, nil)
	}()

	select {
	case n, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before receiving notification")
		}
		if n.Action != "CREATE" {
			t.Errorf("expected CREATE, got %s", n.Action)
		}
		if n.QueryID == "" {
			t.Error("QueryID is empty")
		}
		t.Logf("notification: action=%s queryID=%s data=%v", n.Action, n.QueryID, n.Data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for notification")
	}

	kill()
}
