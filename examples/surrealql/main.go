package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	surrealdb "github.com/surrealdb/surrealdb.c.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	fmt.Println("=== SurrealDB + SurrealQL Query Builder Demo ===")
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
			return fmt.Errorf("CREATE %s: %w", p.id, err)
		}
		if results[0].Err() != nil {
			return fmt.Errorf("CREATE %s statement error: %w", p.id, results[0].Err())
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
		return fmt.Errorf("SELECT: %w", err)
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
		return fmt.Errorf("SELECT WHERE: %w", err)
	}
	printResults(results)

	// 4. UPDATE a record
	fmt.Println()
	fmt.Println("[4] UPDATE person:bob ...")

	uq := surrealql.Update("person:bob").
		Set("age", 26).
		Set("email", "bob.new@example.com")
	sql, vars = uq.Build()
	printQuery(sql, vars)
	results, err = db.Query(ctx, sql, vars)
	if err != nil {
		return fmt.Errorf("UPDATE: %w", err)
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
		return fmt.Errorf("SELECT multi-where: %w", err)
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
		return fmt.Errorf("COUNT: %w", err)
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
		return fmt.Errorf("DELETE: %w", err)
	}
	fmt.Printf("    Deleted (statement returned %d results)\n", len(results[0].Values()))

	// 8. Final state
	fmt.Println()
	fmt.Println("[8] Final state ...")

	q = surrealql.Select("person").OrderBy("name")
	sql, vars = q.Build()
	results, err = db.Query(ctx, sql, vars)
	if err != nil {
		return fmt.Errorf("Final SELECT: %w", err)
	}
	printResults(results)

	fmt.Println()
	fmt.Println("=== Demo complete! ===")
	return nil
}

func printQuery(sql string, vars map[string]any) {
	fmt.Printf("    SurrealQL: %s\n", sql)
	if len(vars) > 0 {
		keys := make([]string, 0, len(vars))
		for k := range vars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = fmt.Sprintf("%s:%v", k, vars[k])
		}
		fmt.Printf("    Vars: [%s]\n", strings.Join(parts, " "))
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
