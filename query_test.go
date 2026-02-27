package surrealdb

import (
	"context"
	"testing"
)

func TestQuery(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	db.Query(ctx, `CREATE person:a SET name = "Alice", age = 30`, nil)
	db.Query(ctx, `CREATE person:b SET name = "Bob", age = 25`, nil)

	type Person struct {
		ID   string `db:"id"`
		Name string `db:"name"`
		Age  int64  `db:"age"`
	}

	results, err := Query[Person](ctx, db, "SELECT * FROM person ORDER BY age ASC", nil)
	if err != nil {
		t.Fatalf("Query[Person]: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 statement result, got %d", len(results))
	}
	if results[0].Err() != nil {
		t.Fatalf("statement error: %v", results[0].Err())
	}

	people := results[0].Values()
	if len(people) != 2 {
		t.Fatalf("expected 2 records, got %d", len(people))
	}

	if people[0].Name != "Bob" || people[0].Age != 25 {
		t.Errorf("first: expected Bob/25, got %s/%d", people[0].Name, people[0].Age)
	}
	if people[1].Name != "Alice" || people[1].Age != 30 {
		t.Errorf("second: expected Alice/30, got %s/%d", people[1].Name, people[1].Age)
	}

	// Verify ID was unmarshaled as "table:id"
	if people[0].ID != "person:b" {
		t.Errorf("expected id=person:b, got %s", people[0].ID)
	}
}

func TestQueryWithStructVars(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	db.Query(ctx, `CREATE person:a SET name = "A", age = 20`, nil)
	db.Query(ctx, `CREATE person:b SET name = "B", age = 30`, nil)
	db.Query(ctx, `CREATE person:c SET name = "C", age = 40`, nil)

	type Row struct {
		Name string `db:"name"`
		Age  int64  `db:"age"`
	}

	results, err := Query[Row](ctx, db,
		"SELECT name, age FROM person WHERE age >= $min_age ORDER BY age ASC",
		map[string]any{"min_age": 25},
	)
	if err != nil {
		t.Fatalf("Query[Row]: %v", err)
	}

	rows := results[0].Values()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Name != "B" || rows[0].Age != 30 {
		t.Errorf("first: expected B/30, got %s/%d", rows[0].Name, rows[0].Age)
	}
	if rows[1].Name != "C" || rows[1].Age != 40 {
		t.Errorf("second: expected C/40, got %s/%d", rows[1].Name, rows[1].Age)
	}
}
