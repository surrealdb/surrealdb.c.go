package main

import (
	"context"
	"fmt"
	"log"

	surrealdb "github.com/surrealdb/surrealdb.c.go"
)

type Person struct {
	Name   string `cbor:"name"`
	Age    int    `cbor:"age"`
	Active bool   `cbor:"active"`
}

type PersonWithID struct {
	ID     surrealdb.RecordID[string] `cbor:"id"`
	Name   string                     `cbor:"name"`
	Age    int                        `cbor:"age"`
	Active bool                       `cbor:"active"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	fmt.Println("=== SurrealDB Embedded Go Library Demo ===")
	fmt.Println()

	ctx := context.Background()
	db, err := surrealdb.Open(ctx, "mem://")
	if err != nil {
		return fmt.Errorf("Open: %w", err)
	}
	defer db.Close()

	if err := db.Use(ctx, "demo_ns", "demo_db"); err != nil {
		return fmt.Errorf("Use: %w", err)
	}

	fmt.Println("[1] Creating records...")
	people := []struct {
		id string
		p  Person
	}{
		{"person:alice", Person{"Alice", 30, true}},
		{"person:bob", Person{"Bob", 25, false}},
		{"person:charlie", Person{"Charlie", 35, true}},
	}
	for _, entry := range people {
		type ContentVars struct {
			Data Person `cbor:"data"`
		}
		results, err := db.Query(ctx,
			fmt.Sprintf("CREATE %s CONTENT $data", entry.id),
			ContentVars{Data: entry.p},
		)
		if err != nil {
			return fmt.Errorf("CREATE %s: %w", entry.id, err)
		}
		if results[0].Err != nil {
			return fmt.Errorf("CREATE %s statement error: %w", entry.id, results[0].Err)
		}
		fmt.Printf("    Created %s\n", entry.id)
	}

	fmt.Println()
	fmt.Println("[2] SELECT * FROM person ORDER BY name...")
	results, err := db.Query(ctx, "SELECT * FROM person ORDER BY name ASC", nil)
	if err != nil {
		return fmt.Errorf("SELECT: %w", err)
	}
	printResults(results)

	fmt.Println()
	fmt.Println("[3] Typed query via Query[Person]...")
	typed, err := surrealdb.Query[Person](ctx, db, "SELECT name, age, active FROM person ORDER BY name ASC", nil)
	if err != nil {
		return fmt.Errorf("Query[Person]: %w", err)
	}
	for _, p := range typed[0].Values() {
		fmt.Printf("    %s (age=%d, active=%v)\n", p.Name, p.Age, p.Active)
	}

	// RecordID round-trip: use RecordID in query vars, get structured RecordID back
	fmt.Println()
	fmt.Println("[4] RecordID round-trip...")
	rid := surrealdb.NewRecordID("person", "alice")
	ridResults, err := surrealdb.Query[PersonWithID](ctx, db, "SELECT * FROM $rid", map[string]any{"rid": rid})
	if err != nil {
		return fmt.Errorf("Query with RecordID var: %w", err)
	}
	p := ridResults[0].Values()[0]
	fmt.Printf("    Queried by RecordID: %s (table=%s, id=%v, name=%s)\n", p.ID, p.ID.Table, p.ID.ID, p.Name)

	fmt.Println()
	fmt.Println("=== Demo complete! ===")
	return nil
}

func printResults(results []surrealdb.QueryResult) {
	for i, r := range results {
		if r.Err != nil {
			fmt.Printf("  Statement %d ERROR: %v\n", i+1, r.Err)
			continue
		}
		if rows, ok := r.Value.([]any); ok {
			fmt.Printf("  Statement %d: %d result(s)\n", i+1, len(rows))
			for j, row := range rows {
				fmt.Printf("    [%d] %v\n", j, row)
			}
		} else {
			fmt.Printf("  Statement %d: %v\n", i+1, r.Value)
		}
	}
}
