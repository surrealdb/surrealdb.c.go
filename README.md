# surrealdb.c.go

Go bindings for embedded SurrealDB via the [surrealdb.c](https://github.com/surrealdb/surrealdb.c) C FFI library.

> **Compatibility:** Currently tested with **SurrealDB 2.4.1** (the surrealdb.c
> library is locked to this version). Support for v3 is coming soon.

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
    ID   string `cbor:"id"`
    Name string `cbor:"name"`
    Age  int64  `cbor:"age"`
}

func main() {
    ctx := context.Background()

    db, err := surrealdb.Open(ctx, "mem://")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    db.Use(ctx, "test", "test")
    db.Query(ctx, `CREATE person:alice SET name = "Alice", age = 30`, nil)

    results, _ := surrealdb.Query[Person](ctx, db, "SELECT * FROM person", nil)
    for _, p := range results[0].Values() {
        fmt.Printf("%s: %s (age %d)\n", p.ID, p.Name, p.Age)
    }
}
```

## Documentation

See [docs/](docs/README.md) for building, API reference, storage backends, and
internals.

