# API overview

Everything goes through the `surrealdb.Driver` interface returned by
`surrealdb.Open`. Every method that talks to the database takes a
`context.Context`.

## Connection lifecycle

```go
ctx := context.Background()

db, err := surrealdb.Open(ctx, "mem://") // or "surrealkv://path", "rocksdb://path", "ws://host:port"
if err != nil {
    log.Fatal(err)
}
defer db.Close()

if err := db.Use(ctx, "my_namespace", "my_database"); err != nil {
    log.Fatal(err)
}

ver, _ := db.Version(ctx) // ver.Version == "3.0.1" for the embedded engine
err = db.Health(ctx)      // nil if healthy
```

With the embedded backend (`mem://`, `surrealkv://`, `rocksdb://`, and
`ws://`/`http://` when no remote driver is registered), `Close` waits for calls
already running on other goroutines, stops all live queries, then frees the
connection. It is safe to call more than once and from several goroutines. Any
call made after `Close` returns `surrealdb.ErrClosed`. A cancelled context
makes the call return `ctx.Err()` straight away, but the C call it started
cannot be interrupted and finishes in the background.

With the remote Go driver (`driver/go`), `Close` closes the surrealdb.go
connection, and errors after `Close` come from surrealdb.go rather than
`surrealdb.ErrClosed`.

## Querying

`vars` accepts `nil`, a `map[string]any`, or a struct. Struct fields use
`cbor:"name"` tags (`cbor:"name,omitempty"` skips zero values); untagged
fields use the lower-cased field name.

```go
results, err := db.Query(ctx, "SELECT * FROM person", nil)

results, err = db.Query(ctx,
    "SELECT * FROM person WHERE age >= $min_age",
    map[string]any{"min_age": 18},
)

// Multi-statement: results[0] is the count, results[1] the average.
results, err = db.Query(ctx,
    "SELECT count() AS n FROM person GROUP ALL;"+
        "SELECT math::mean(age) AS avg FROM person GROUP ALL;",
    nil,
)
for _, r := range results {
    if r.Err != nil {
        log.Println("statement error:", r.Err)
        continue
    }
    fmt.Println(r.Value) // []any, map[string]any or a scalar
}
```

### Typed queries with `Query[T]`

```go
type Person struct {
    ID   surrealdb.RecordID[string] `cbor:"id,omitempty"`
    Name string                     `cbor:"name"`
    Age  int64                      `cbor:"age"`
}

results, err := surrealdb.Query[Person](ctx, db, "SELECT * FROM person", nil)
for _, p := range results[0].Values() {
    fmt.Printf("%s: %s (age %d)\n", p.ID, p.Name, p.Age)
}
```

## Connection variables

```go
db.Let(ctx, "min_age", 18) // available as $min_age in later queries
db.Unset(ctx, "min_age")
```

`Let` accepts the same value types as query variables.

## Authentication

```go
// Root, namespace or database user, chosen by which fields are set.
token, err := db.SignIn(ctx, surrealdb.Auth{
    Namespace: "app", Database: "app",
    Username:  "admin", Password: "secret",
})

// Record access: set Access. Use a map to pass custom fields to the
// access method's SIGNIN/SIGNUP clause ($email, $pass below).
token, err = db.SignUp(ctx, map[string]any{
    "NS": "app", "DB": "app", "AC": "account",
    "email": "ada@example.com", "pass": "s3cret",
})

db.Authenticate(ctx, token) // reuse a token
db.Invalidate(ctx)          // sign out
```

## Live queries

```go
id, ch, kill, err := db.Live(ctx, "person") // a table, or a record id like "person:alice"
if err != nil {
    log.Fatal(err)
}

go func() {
    for n := range ch { // closed when the live query stops
        fmt.Println(n.Action, n.Data)
    }
}()

kill()       // stop it, or:
db.Kill(id)  // stop it by id
```

`id` is the server's live query id, the same as `Notification.QueryID`,
when `Live` can find it (it uses `INFO FOR TABLE`, which a record user may not
be allowed to run); otherwise it is a local id. `Kill` accepts either.

Stopping a live query writes nothing to the database. When `kill` returns,
the channel is closed and nothing more is delivered. surrealdb.c cannot
interrupt a stream that is waiting for its next event, so the engine-side
live query is released on its next event, or when `Close` frees the
connection.

## Compared with surrealdb.go

The method names and types follow [surrealdb.go](https://github.com/surrealdb/surrealdb.go)
where they overlap: `Use`, `SignIn`, `SignUp`, `Authenticate`, `Invalidate`,
`Let`, `Unset`, `Version` (returning `*VersionData`), and `Auth` (same fields
and wire names). The remaining differences:

| | surrealdb.c.go | surrealdb.go |
|---|---|---|
| Close | `Close()` | `Close(ctx) error` |
| Query | `db.Query(ctx, sql, vars)` and `surrealdb.Query[T](ctx, db, sql, vars)` returning `[]Result[T]` | `surrealdb.Query[T](ctx, db, sql, vars)` returning `*[]QueryResult[T]` |
| Live queries | `Live` returns the id, a channel and a stop function | `Live` returns the id; `LiveNotifications(id)` returns the channel |
| Kill | `Kill(id)` | `Kill(ctx, db, id)` |
| Health | `Health(ctx)` | none |
| CRUD helpers, `Info`, refresh tokens, sessions, transactions | not available | available |

Sessions and interactive transactions are not available because surrealdb.c
does not support them yet.
