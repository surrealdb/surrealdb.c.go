package main

import (
	"context"
	"fmt"
	"log"

	surrealdb "github.com/surrealdb/surrealdb.c.go"
)

type Person struct {
	Name   string `db:"name"`
	Age    int    `db:"age"`
	Active bool   `db:"active"`
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

	// CREATE records using struct vars
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
			Data Person `db:"data"`
		}
		results, err := db.Query(ctx,
			fmt.Sprintf("CREATE %s CONTENT $data", entry.id),
			ContentVars{Data: entry.p},
		)
		if err != nil {
			return fmt.Errorf("CREATE %s: %w", entry.id, err)
		}
		printResults(fmt.Sprintf("    Created %s", entry.id), results)
	}

	// SELECT all records
	fmt.Println()
	fmt.Println("[2] SELECT * FROM person ORDER BY name...")
	results, err := db.Query(ctx, "SELECT * FROM person ORDER BY name ASC", nil)
	if err != nil {
		return fmt.Errorf("SELECT: %w", err)
	}
	printResults("", results)

	// UPDATE a record
	fmt.Println()
	fmt.Println("[3] UPDATE person:bob...")
	results, err = db.Query(ctx,
		`UPDATE person:bob SET name = "Bob Updated", age = 26, active = true, email = "bob@example.com"`,
		nil,
	)
	if err != nil {
		return fmt.Errorf("UPDATE: %w", err)
	}
	printResults("", results)

	// Parameterized SELECT using map vars
	fmt.Println()
	fmt.Println("[4] SELECT WHERE age >= $min_age (30)...")
	results, err = db.Query(ctx,
		"SELECT * FROM person WHERE age >= $min_age ORDER BY age ASC",
		map[string]any{"min_age": 30},
	)
	if err != nil {
		return fmt.Errorf("SELECT parameterized: %w", err)
	}
	printResults("", results)

	// Multi-statement query
	fmt.Println()
	fmt.Println("[5] Multi-statement: count + average age...")
	results, err = db.Query(ctx,
		"SELECT count() AS total FROM person GROUP ALL;"+
			"SELECT math::mean(age) AS avg_age FROM person GROUP ALL;",
		nil,
	)
	if err != nil {
		return fmt.Errorf("Multi-statement: %w", err)
	}
	printResults("", results)

	// Final state
	fmt.Println()
	fmt.Println("[6] Final state...")
	results, err = db.Query(ctx, "SELECT * FROM person ORDER BY name ASC", nil)
	if err != nil {
		return fmt.Errorf("Final SELECT: %w", err)
	}
	printResults("", results)

	fmt.Println()
	fmt.Println("=== Demo complete! ===")
	return nil
}

func printResults(prefix string, results []surrealdb.QueryResult) {
	for i, r := range results {
		if r.Err() != nil {
			fmt.Printf("  Statement %d ERROR: %v\n", i+1, r.Err())
			continue
		}
		if prefix != "" {
			fmt.Println(prefix)
			continue
		}
		fmt.Printf("  Statement %d: %d result(s)\n", i+1, len(r.Values()))
		for j, v := range r.Values() {
			fmt.Printf("    [%d] ", j)
			printValue(&v)
			fmt.Println()
		}
	}
}

func printValue(v *surrealdb.Value) {
	switch v.Tag() {
	case surrealdb.ValueObject:
		fmt.Print("{")
		for i, kv := range v.AsObject() {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s: ", kv.Key)
			printValue(&kv.Value)
		}
		fmt.Print("}")
	case surrealdb.ValueArray:
		fmt.Print("[")
		for i, elem := range v.AsArray() {
			if i > 0 {
				fmt.Print(", ")
			}
			printValue(&elem)
		}
		fmt.Print("]")
	case surrealdb.ValueThing:
		fmt.Printf("%s:%s", v.AsThingTable(), v.AsThingID())
	case surrealdb.ValueString:
		fmt.Printf("%q", v.AsString())
	case surrealdb.ValueNumber:
		fmt.Print(v.String())
	default:
		fmt.Print(v.String())
	}
}
