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
	//     [0] map[active:true age:30 id:person:alice name:Alice]
	//     [1] map[active:false age:25 id:person:bob name:Bob]
	//     [2] map[active:true age:35 id:person:charlie name:Charlie]
	//
	// [3] Typed query via Query[Person]...
	//     Alice (age=30, active=true)
	//     Bob (age=25, active=false)
	//     Charlie (age=35, active=true)
	//
	// === Demo complete! ===
}
