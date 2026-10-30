# surrealdb.c.go

Go bindings for embedded SurrealDB via the [surrealdb.c](https://github.com/surrealdb/surrealdb.c) C FFI library.

> **Compatibility:** The embedded engine is whatever surrealdb.c build you
> link. CI pins surrealdb.c to a fixed commit (`SURREALDB_C_V3_REF` in
> `.github/workflows/test.yml`, currently Rust SDK 3.0.1) and runs every test
> with `-race`. Remote tests run against SurrealDB v2, v3.0.1, the latest v3,
> and the nightly image (the 4.0 line, allowed to fail until 4.0 is tagged).
> `make` defaults `SURREALDB_C_REF` to `main`; for v2, pin it to a
> v2-compatible surrealdb.c commit when you build the static library.

## Quick start

1. [Build surrealdb.c and set up CGO](docs/build.md) (or just run `make build`)
2. `go get github.com/surrealdb/surrealdb.c.go`
3. Write code:

> **New to CGO?** This module links against a C static library (`libsurrealdb_c.a`)
> at compile time. You must build surrealdb.c **and** set `CGO_LDFLAGS` to its
> location before `go build` or `go test` will work. Without this, the Go linker
> cannot find the SurrealDB symbols and the build will fail. See
> [docs/build.md](docs/build.md) for step-by-step instructions — or use the
> provided Makefile which handles everything automatically (`make build`).

```go
package main

import (
    "context"
    "fmt"
    "log"

    surrealdb "github.com/surrealdb/surrealdb.c.go"
)

type Person struct {
    ID   surrealdb.RecordID[string] `cbor:"id,omitempty"`
    Name string                     `cbor:"name"`
    Age  int64                      `cbor:"age"`
}

func main() {
    ctx := context.Background()

    db, err := surrealdb.Open(ctx, "mem://")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    db.Use(ctx, "test", "test")
    db.Query(ctx, "DEFINE TABLE person SCHEMALESS", nil)
    db.Query(ctx, "CREATE $rid CONTENT $content", map[string]any{
        "rid":     surrealdb.NewRecordID("person", "alice"),
        "content": Person{Name: "Alice", Age: 30},
    })

    results, _ := surrealdb.Query[Person](ctx, db, "SELECT * FROM person", nil)
    for _, p := range results[0].Values() {
        fmt.Printf("%s: %s (age %d)\n", p.ID, p.Name, p.Age)
    }
}
```

## Documentation

See [docs/](docs/README.md) for building, API reference, storage backends, and
internals.

