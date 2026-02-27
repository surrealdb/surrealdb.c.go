package main

func Example() {
	if err := run(); err != nil {
		panic(err)
	}
	// Output:
	// === SurrealDB Embedded Go Library Demo ===
	//
	// [1] Creating records...
	//     Created person:alice
	//     Created person:bob
	//     Created person:charlie
	//
	// [2] SELECT * FROM person ORDER BY name...
	//   Statement 1: 3 result(s)
	//     [0] {active: true, age: 30, id: person:alice, name: "Alice"}
	//     [1] {active: false, age: 25, id: person:bob, name: "Bob"}
	//     [2] {active: true, age: 35, id: person:charlie, name: "Charlie"}
	//
	// [3] UPDATE person:bob...
	//   Statement 1: 1 result(s)
	//     [0] {active: true, age: 26, email: "bob@example.com", id: person:bob, name: "Bob Updated"}
	//
	// [4] SELECT WHERE age >= $min_age (30)...
	//   Statement 1: 2 result(s)
	//     [0] {active: true, age: 30, id: person:alice, name: "Alice"}
	//     [1] {active: true, age: 35, id: person:charlie, name: "Charlie"}
	//
	// [5] Multi-statement: count + average age...
	//   Statement 1: 1 result(s)
	//     [0] {total: 3}
	//   Statement 2: 1 result(s)
	//     [0] {avg_age: 30.333333333333332}
	//
	// [6] Final state...
	//   Statement 1: 3 result(s)
	//     [0] {active: true, age: 30, id: person:alice, name: "Alice"}
	//     [1] {active: true, age: 26, email: "bob@example.com", id: person:bob, name: "Bob Updated"}
	//     [2] {active: true, age: 35, id: person:charlie, name: "Charlie"}
	//
	// === Demo complete! ===
}
