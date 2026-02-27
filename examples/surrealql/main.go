package main

import (
	"context"
	"fmt"
	"log"

	surrealdb "github.com/surrealdb/surrealdb.c.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func main() {
	fmt.Println("=== SurrealDB + SurrealQL Query Builder Demo ===")
	fmt.Println()

	ctx := context.Background()
	db, err := surrealdb.Open(ctx, "mem://")
	if err != nil {
		log.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.Use(ctx, "demo_ns", "demo_db"); err != nil {
		log.Fatalf("Use: %v", err)
	}

	// 1. CREATE records using the query builder
	fmt.Println("[1] Creating records with surrealql.Create ...")

	people := []struct {
		id    string
		name  string
		age   int
		email string
	}{
		{"alice", "Alice", 30, "alice@example.com"},
		{"bob", "Bob", 25, "bob@example.com"},
		{"charlie", "Charlie", 35, "charlie@example.com"},
		{"diana", "Diana", 28, "diana@example.com"},
	}

	for _, p := range people {
		q := surrealql.Create("person:" + p.id).
			Set("name", p.name).
			Set("age", p.age).
			Set("email", p.email)

		sql, vars := q.Build()
		results, err := db.Query(ctx, sql, vars)
		if err != nil {
			log.Fatalf("CREATE %s: %v", p.id, err)
		}
		if results[0].Err() != nil {
			log.Fatalf("CREATE %s statement error: %v", p.id, results[0].Err())
		}
		fmt.Printf("    Created person:%s\n", p.id)
	}

	// 2. SELECT all records
	fmt.Println()
	fmt.Println("[2] SELECT * FROM person ORDER BY name ...")

	q := surrealql.Select("person").OrderBy("name")
	sql, vars := q.Build()
	printQuery(sql, vars)
	results, err := db.Query(ctx, sql, vars)
	if err != nil {
		log.Fatalf("SELECT: %v", err)
	}
	printResults(results)

	// 3. SELECT with WHERE using parameterized values
	fmt.Println()
	fmt.Println("[3] SELECT WHERE age >= 30 ...")

	q = surrealql.Select("person").
		Fields("name", "age").
		Where("age >= ?", 30).
		OrderBy("age")
	sql, vars = q.Build()
	printQuery(sql, vars)
	results, err = db.Query(ctx, sql, vars)
	if err != nil {
		log.Fatalf("SELECT WHERE: %v", err)
	}
	printResults(results)

	// 4. UPDATE a record
	fmt.Println()
	fmt.Println("[4] UPDATE person:bob SET age = 26, email = 'bob.new@example.com' ...")

	uq := surrealql.Update("person:bob").
		Set("age", 26).
		Set("email", "bob.new@example.com")
	sql, vars = uq.Build()
	printQuery(sql, vars)
	results, err = db.Query(ctx, sql, vars)
	if err != nil {
		log.Fatalf("UPDATE: %v", err)
	}
	printResults(results)

	// 5. SELECT with multiple WHERE conditions
	fmt.Println()
	fmt.Println("[5] SELECT WHERE age >= 26 AND age < 35 ...")

	q = surrealql.Select("person").
		Where("age >= ?", 26).
		Where("age < ?", 35).
		OrderBy("name")
	sql, vars = q.Build()
	printQuery(sql, vars)
	results, err = db.Query(ctx, sql, vars)
	if err != nil {
		log.Fatalf("SELECT multi-where: %v", err)
	}
	printResults(results)

	// 6. COUNT with GROUP ALL
	fmt.Println()
	fmt.Println("[6] SELECT count() FROM person GROUP ALL ...")

	q = surrealql.Select("person").
		Field("count()").
		GroupAll()
	sql, vars = q.Build()
	printQuery(sql, vars)
	results, err = db.Query(ctx, sql, vars)
	if err != nil {
		log.Fatalf("COUNT: %v", err)
	}
	printResults(results)

	// 7. DELETE a record
	fmt.Println()
	fmt.Println("[7] DELETE person:diana ...")

	dq := surrealql.Delete("person:diana")
	sql, vars = dq.Build()
	printQuery(sql, vars)
	results, err = db.Query(ctx, sql, vars)
	if err != nil {
		log.Fatalf("DELETE: %v", err)
	}
	fmt.Printf("    Deleted (statement returned %d results)\n", len(results[0].Values()))

	// 8. Final state
	fmt.Println()
	fmt.Println("[8] Final state ...")

	q = surrealql.Select("person").OrderBy("name")
	sql, vars = q.Build()
	results, err = db.Query(ctx, sql, vars)
	if err != nil {
		log.Fatalf("Final SELECT: %v", err)
	}
	printResults(results)

	fmt.Println()
	fmt.Println("=== Demo complete! ===")
}

func printQuery(sql string, vars map[string]any) {
	fmt.Printf("    SurrealQL: %s\n", sql)
	if len(vars) > 0 {
		fmt.Printf("    Vars: %v\n", vars)
	}
}

func printResults(results []surrealdb.QueryResult) {
	for i, r := range results {
		if r.Err() != nil {
			fmt.Printf("    Statement %d ERROR: %v\n", i+1, r.Err())
			continue
		}
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
