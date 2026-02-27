package surrealdb

import "testing"

func TestDBQueryWithStructVars(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	db.Query(`CREATE person:a SET name = "A", age = 20`, nil)
	db.Query(`CREATE person:b SET name = "B", age = 30`, nil)
	db.Query(`CREATE person:c SET name = "C", age = 40`, nil)

	type Vars struct {
		MinAge int `db:"min_age"`
	}

	results, err := db.Query(
		"SELECT * FROM person WHERE age >= $min_age ORDER BY age ASC",
		Vars{MinAge: 25},
	)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	vals := results[0].Values()
	if len(vals) != 2 {
		t.Fatalf("expected 2 records, got %d", len(vals))
	}

	// Verify the results are B (age 30) and C (age 40)
	for i, v := range vals {
		kvs := v.AsObject()
		for _, kv := range kvs {
			if kv.Key == "age" {
				age := kv.Value.AsInt()
				if i == 0 && age != 30 {
					t.Errorf("first result: expected age=30, got %d", age)
				}
				if i == 1 && age != 40 {
					t.Errorf("second result: expected age=40, got %d", age)
				}
			}
		}
	}
}
