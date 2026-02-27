# API overview

## Connection lifecycle

```go
ctx := context.Background()

db, err := surrealdb.Open(ctx, "mem://")     // or "surrealkv://path", "ws://host:port"
defer db.Close()

db.Use(ctx, "my_namespace", "my_database")

ver, _ := db.Version()  // "2.4.1"
db.Health()              // nil if healthy
```

## Querying

All query methods accept a `context.Context` for timeout/cancellation.
The `vars` parameter accepts `nil`, `*Object`, `map[string]any`, or any struct
with `db:"..."` tags — marshaling is automatic.

```go
// Simple query
results, err := db.Query(ctx, "SELECT * FROM person", nil)

// Parameterized with a map
results, err := db.Query(ctx,
    "SELECT * FROM person WHERE age >= $min_age",
    map[string]any{"min_age": 18},
)

// Parameterized with a struct
results, err := db.Query(ctx,
    "SELECT * FROM person WHERE age >= $min_age",
    struct { MinAge int `db:"min_age"` }{MinAge: 18},
)

// Multi-statement
results, err := db.Query(ctx,
    "SELECT count() AS n FROM person GROUP ALL;" +
    "SELECT math::mean(age) AS avg FROM person GROUP ALL;",
    nil,
)
// results[0] has the count, results[1] has the average
```

## Typed queries with `Query[T]`

`surrealdb.Query[T]` unmarshals each result row into a Go struct automatically:

```go
type Person struct {
    ID   string `db:"id"`
    Name string `db:"name"`
    Age  int64  `db:"age"`
}

results, err := surrealdb.Query[Person](ctx, db, "SELECT * FROM person", nil)
for _, p := range results[0].Values() {
    fmt.Printf("%s: %s (age %d)\n", p.ID, p.Name, p.Age)
}
```

## Reading untyped results

```go
for _, stmt := range results {
    if stmt.Err() != nil {
        log.Println("statement error:", stmt.Err())
        continue
    }
    for _, val := range stmt.Values() {
        switch val.Tag() {
        case surrealdb.ValueObject:
            for _, kv := range val.AsObject() {
                fmt.Printf("%s = %s\n", kv.Key, kv.Value.String())
            }
        case surrealdb.ValueString:
            fmt.Println(val.AsString())
        case surrealdb.ValueNumber:
            fmt.Println(val.AsInt())
        }
    }
}
```

## Building objects

```go
// Manual
obj := surrealdb.NewObject()
obj.InsertStr("name", "Alice")
obj.InsertInt("age", 30)
obj.InsertBool("active", true)
defer obj.Free()

// From a Go struct
type Person struct {
    Name   string `db:"name"`
    Age    int    `db:"age"`
    Active bool   `db:"active"`
}
obj, err := surrealdb.MarshalObject(Person{"Alice", 30, true})
defer obj.Free()
```

## Live queries

```go
// Start listening for changes on the "person" table
lq, err := db.Live(ctx, "person")
if err != nil {
    log.Fatal(err)
}

// Read notifications from the channel
go func() {
    for n := range lq.Ch {
        fmt.Printf("%s: %s\n", n.Action, n.Data.String())
    }
}()

// ... later, stop the live query
db.Kill(lq)
```
