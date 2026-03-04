package surrealdb

import (
	"context"
	"testing"
)

func TestDBQueryWithMapVars(t *testing.T) {
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

	for i, row := range rows {
		rec, ok := row.(map[string]any)
		if !ok {
			t.Fatalf("expected map[string]any, got %T", row)
		}
		age, _ := rec["age"].(int64)
		if i == 0 && age != 30 {
			t.Errorf("first result: expected age=30, got %d", age)
		}
		if i == 1 && age != 40 {
			t.Errorf("second result: expected age=40, got %d", age)
		}
	}
}
