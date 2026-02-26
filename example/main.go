package main

import (
	"fmt"
	"log"

	surrealdb "github.com/surrealdb/surrealdb.c.go"
)

// Person is a Go struct that can be marshaled into a SurrealDB Object.
type Person struct {
	Name   string `db:"name"`
	Age    int    `db:"age"`
	Active bool   `db:"active"`
}

func main() {
	fmt.Println("=== SurrealDB Embedded Go Library Demo ===")
	fmt.Println()

	// 1. Open an in-memory embedded database
	db, err := surrealdb.Open("mem://")
	if err != nil {
		log.Fatalf("Open: %v", err)
	}
	defer db.Close()

	ver, err := db.Version()
	if err != nil {
		log.Fatalf("Version: %v", err)
	}
	fmt.Printf("[1] Connected — SurrealDB %s\n", ver)

	// 2. Select namespace and database
	if err := db.Use("demo_ns", "demo_db"); err != nil {
		log.Fatalf("Use: %v", err)
	}
	fmt.Println("[2] Namespace: demo_ns, Database: demo_db")

	// 4. CREATE records using Query + MarshalObject
	fmt.Println()
	fmt.Println("[4] Creating records...")
	people := []struct {
		id string
		p  Person
	}{
		{"person:alice", Person{"Alice", 30, true}},
		{"person:bob", Person{"Bob", 25, false}},
		{"person:charlie", Person{"Charlie", 35, true}},
	}
	for _, entry := range people {
		vars, err := surrealdb.MarshalObject(entry.p)
		if err != nil {
			log.Fatalf("MarshalObject: %v", err)
		}
		outer := wrapData(vars)
		results, err := db.Query(
			fmt.Sprintf("CREATE %s CONTENT $data", entry.id),
			outer,
		)
		outer.Free()
		vars.Free()
		if err != nil {
			log.Fatalf("CREATE %s: %v", entry.id, err)
		}
		printResults(fmt.Sprintf("    Created %s", entry.id), results)
	}

	// 5. SELECT all records
	fmt.Println()
	fmt.Println("[5] SELECT * FROM person ORDER BY name...")
	results, err := db.Query("SELECT * FROM person ORDER BY name ASC", nil)
	if err != nil {
		log.Fatalf("SELECT: %v", err)
	}
	printResults("", results)

	// 6. UPDATE a record
	fmt.Println()
	fmt.Println("[6] UPDATE person:bob...")
	results, err = db.Query(
		`UPDATE person:bob SET name = "Bob Updated", age = 26, active = true, email = "bob@example.com"`,
		nil,
	)
	if err != nil {
		log.Fatalf("UPDATE: %v", err)
	}
	printResults("", results)

	// 7. Parameterized SELECT
	fmt.Println()
	fmt.Println("[7] SELECT WHERE age >= $min_age (30)...")
	vars := surrealdb.NewObject()
	vars.InsertInt("min_age", 30)
	results, err = db.Query(
		"SELECT * FROM person WHERE age >= $min_age ORDER BY age ASC",
		vars,
	)
	vars.Free()
	if err != nil {
		log.Fatalf("SELECT parameterized: %v", err)
	}
	printResults("", results)

	// 8. Multi-statement query
	fmt.Println()
	fmt.Println("[8] Multi-statement: count + average age...")
	results, err = db.Query(
		"SELECT count() AS total FROM person GROUP ALL;"+
			"SELECT math::mean(age) AS avg_age FROM person GROUP ALL;",
		nil,
	)
	if err != nil {
		log.Fatalf("Multi-statement: %v", err)
	}
	printResults("", results)

	// 9. Final state
	fmt.Println()
	fmt.Println("[9] Final state...")
	results, err = db.Query("SELECT * FROM person ORDER BY name ASC", nil)
	if err != nil {
		log.Fatalf("Final SELECT: %v", err)
	}
	printResults("", results)

	fmt.Println()
	fmt.Println("=== Demo complete! ===")
}

// wrapData wraps a marshaled Object in an outer Object with key "data"
// so it can be used as $data in a CONTENT clause.
func wrapData(inner *surrealdb.Object) *surrealdb.Object {
	outer := surrealdb.NewObject()
	outer.InsertObject("data", inner)
	return outer
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
			printValue(&v, "")
			fmt.Println()
		}
	}
}

func printValue(v *surrealdb.Value, indent string) {
	switch v.Tag() {
	case surrealdb.ValueObject:
		fmt.Print("{")
		for i, kv := range v.AsObject() {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s: ", kv.Key)
			printValue(&kv.Value, indent)
		}
		fmt.Print("}")
	case surrealdb.ValueArray:
		fmt.Print("[")
		for i, elem := range v.AsArray() {
			if i > 0 {
				fmt.Print(", ")
			}
			printValue(&elem, indent)
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
