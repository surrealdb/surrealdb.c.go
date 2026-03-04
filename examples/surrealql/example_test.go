package main

func ExampleRun() {
	if err := Run(); err != nil {
		panic(err)
	}
	// Output:
	// === SurrealDB + SurrealQL Query Builder Demo ===
	//
	// [1] Creating records with surrealql.Create ...
	//     Created person:alice
	//     Created person:bob
	//     Created person:charlie
	//     Created person:diana
	//
	// [2] SELECT * FROM person ORDER BY name ...
	//     SurrealQL: SELECT * FROM person ORDER BY name
	//     [0] map[age:30 email:alice@example.com id:person:alice name:Alice]
	//     [1] map[age:25 email:bob@example.com id:person:bob name:Bob]
	//     [2] map[age:35 email:charlie@example.com id:person:charlie name:Charlie]
	//     [3] map[age:28 email:diana@example.com id:person:diana name:Diana]
	//
	// [3] SELECT WHERE age >= 30 ...
	//     SurrealQL: SELECT name, age FROM person WHERE age >= $param_1 ORDER BY age
	//     Vars: [param_1:30]
	//     [0] map[age:30 name:Alice]
	//     [1] map[age:35 name:Charlie]
	//
	// [4] UPDATE person:bob ...
	//     SurrealQL: UPDATE person:bob SET age = $param_1, email = $param_2
	//     Vars: [param_1:26 param_2:bob.new@example.com]
	//     [0] map[age:26 email:bob.new@example.com id:person:bob name:Bob]
	//
	// [5] SELECT WHERE age >= 26 AND age < 35 ...
	//     SurrealQL: SELECT * FROM person WHERE age >= $param_1 AND age < $param_2 ORDER BY name
	//     Vars: [param_1:26 param_2:35]
	//     [0] map[age:30 email:alice@example.com id:person:alice name:Alice]
	//     [1] map[age:26 email:bob.new@example.com id:person:bob name:Bob]
	//     [2] map[age:28 email:diana@example.com id:person:diana name:Diana]
	//
	// [6] SELECT count() FROM person GROUP ALL ...
	//     SurrealQL: SELECT count() FROM person GROUP ALL
	//     [0] map[count:4]
	//
	// [7] DELETE person:diana ...
	//     SurrealQL: DELETE person:diana
	//     Deleted (statement returned 0 results)
	//
	// [8] Final state ...
	//     [0] map[age:30 email:alice@example.com id:person:alice name:Alice]
	//     [1] map[age:26 email:bob.new@example.com id:person:bob name:Bob]
	//     [2] map[age:35 email:charlie@example.com id:person:charlie name:Charlie]
	//
	// === Demo complete! ===
}
