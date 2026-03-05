package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.c.go"
)

// PersonData holds the fields we write to SurrealDB.
// Separating content from the result struct avoids passing
// an "id" field in CONTENT that would conflict with the
// record ID specified in the UPSERT target.
type PersonData struct {
	Name string `cbor:"name"`
	Age  int64  `cbor:"age"`
}

// Person is what we read back — it includes the record ID.
type Person struct {
	ID   surrealdb.RecordID[string] `cbor:"id"`
	Name string                     `cbor:"name"`
	Age  int64                      `cbor:"age"`
}

func ExampleQuery() {
	ctx := context.Background()

	db, err := surrealdb.Open(ctx, "mem://")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err := db.Use(ctx, "example_ns", "example_db"); err != nil {
		panic(err)
	}

	// Define a schemaless table.
	if _, err := db.Query(ctx, "DEFINE TABLE person SCHEMALESS", nil); err != nil {
		panic(err)
	}

	// Upsert records using typed parameters instead of string interpolation.
	for _, p := range []struct {
		id   string
		data PersonData
	}{
		{"alice", PersonData{Name: "Alice", Age: 30}},
		{"bob", PersonData{Name: "Bob", Age: 25}},
	} {
		_, err := db.Query(ctx, "UPSERT $rid CONTENT $content", map[string]any{
			"rid":     surrealdb.NewRecordID("person", p.id),
			"content": p.data,
		})
		if err != nil {
			panic(err)
		}
	}

	// Query back with typed results.
	results, err := surrealdb.Query[Person](ctx, db,
		"SELECT * FROM person ORDER BY name ASC", nil)
	if err != nil {
		panic(err)
	}

	for _, p := range results[0].Values() {
		fmt.Printf("%s: name=%s age=%d\n", p.ID, p.Name, p.Age)
	}

	// Output:
	// person:alice: name=Alice age=30
	// person:bob: name=Bob age=25
}
