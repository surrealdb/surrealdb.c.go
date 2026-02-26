# surrealdb.c.go

Go bindings for embedded SurrealDB via the [surrealdb.c](https://github.com/surrealdb/surrealdb.c) C FFI library.

## Quick start

```go
package main

import (
    "fmt"
    "log"

    surrealdb "github.com/surrealdb/surrealdb.c.go"
)

func main() {
    db, err := surrealdb.Open("mem://")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    db.Use("test", "test")

    results, _ := db.Query(`CREATE person:alice SET name = "Alice", age = 30`, nil)
    for _, r := range results {
        for _, v := range r.Values() {
            fmt.Println(v.String())
        }
    }
}
```

## Prerequisites

- Go >= 1.25
- Rust toolchain (to build surrealdb.c)
- C compiler (gcc/clang) for CGO

## Using the Makefile (easiest)

The Makefile clones surrealdb.c, builds the static library, and wires up CGO
automatically:

```bash
make build          # clone surrealdb.c + cargo build + go build
make test           # clone + build + go test -v ./...
make example        # clone + build + run example/main.go
```

The C library is cloned into `build/surrealdb.c/` by default. You can override
the source:

```bash
# Use a specific branch/tag
make test SURREALDB_C_REF=v0.1.0

# Use a fork
make test SURREALDB_C_REPO=https://github.com/you/surrealdb.c.git

# Point to an existing checkout (skips clone + cargo build)
make test SURREALDB_C_DIR=/path/to/existing/surrealdb.c
```

### Building your own application with the Makefile

Copy this Makefile into your project to automate the surrealdb.c build:

```makefile
SURREALDB_C_REPO ?= https://github.com/surrealdb/surrealdb.c.git
SURREALDB_C_REF  ?= main
SURREALDB_C_DIR  ?= build/surrealdb.c
SURREALDB_C_LIB   = $(SURREALDB_C_DIR)/target/release/libsurrealdb_c.a

export CGO_LDFLAGS := -L$(CURDIR)/$(SURREALDB_C_DIR)/target/release

.PHONY: build run test clean

# Clone + build surrealdb.c (only runs once)
$(SURREALDB_C_DIR)/Cargo.toml:
	git clone --depth 1 --branch $(SURREALDB_C_REF) $(SURREALDB_C_REPO) $(SURREALDB_C_DIR)

$(SURREALDB_C_LIB): $(SURREALDB_C_DIR)/Cargo.toml
	cd $(SURREALDB_C_DIR) && cargo build --release

build: $(SURREALDB_C_LIB)
	go build -o myapp .

run: build
	./myapp

test: $(SURREALDB_C_LIB)
	go test -v ./...

clean:
	rm -rf build myapp
```

Then:

```bash
make run    # clones surrealdb.c, cargo build, go build, runs your app
make test   # same, but runs tests
```

The first `make run` takes a while (Rust compilation), but subsequent runs are
instant because Make skips the clone and cargo steps when the `.a` file exists.

You can point to an existing surrealdb.c checkout to skip the clone entirely:

```bash
make run SURREALDB_C_DIR=/path/to/existing/surrealdb.c
```

## Manual setup (without Make)

Build the C library yourself, then point CGO to it. Only `CGO_LDFLAGS` is
needed — the header (`surrealdb.h`) is vendored in the module directory.

```bash
# 1. Build the C library
git clone --depth 1 https://github.com/surrealdb/surrealdb.c.git
cd surrealdb.c
cargo build --release
# Produces: target/release/libsurrealdb_c.a (~212 MB)
```

```bash
# 2. Point CGO to the library (header is vendored, no CGO_CFLAGS needed)
export CGO_LDFLAGS="-L/path/to/surrealdb.c/target/release"

# 3. Build / test / run
go build ./...
go test -v ./...
cd example && go run .
```

Or install system-wide:

```bash
sudo cp target/release/libsurrealdb_c.a /usr/local/lib/
# Then go build works without env vars
```

### Building your own application manually

After building the C library (steps above), add the dependency and build:

```bash
go get github.com/surrealdb/surrealdb.c.go@latest
CGO_LDFLAGS="-L/path/to/surrealdb.c/target/release" go build -o myapp .
```

If you installed `libsurrealdb_c.a` system-wide (`/usr/local/lib/`), no
environment variables are needed — just `go build`.

## API overview

### Connection lifecycle

```go
db, err := surrealdb.Open("mem://")     // or "surrealkv://path", "ws://host:port"
defer db.Close()

db.Use("my_namespace", "my_database")

ver, _ := db.Version()  // "2.4.1"
db.Health()              // nil if healthy
```

### Querying

```go
// Simple query
results, err := db.Query("SELECT * FROM person", nil)

// Parameterized query
vars := surrealdb.NewObject()
vars.InsertInt("min_age", 18)
defer vars.Free()

results, err := db.Query("SELECT * FROM person WHERE age >= $min_age", vars)

// Multi-statement
results, err := db.Query(
    "SELECT count() AS n FROM person GROUP ALL;" +
    "SELECT math::mean(age) AS avg FROM person GROUP ALL;",
    nil,
)
// results[0] has the count, results[1] has the average
```

### Reading results

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

### Building objects

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

## Linking strategy

The `#cgo` directive uses `-Wl,-Bstatic -lsurrealdb_c -Wl,-Bdynamic` to force
static linking of the SurrealDB library while keeping system libraries dynamic.
This produces a self-contained binary with no runtime dependency on `libsurrealdb_c.so`.

Users only need to set `CGO_LDFLAGS` to the directory containing `libsurrealdb_c.a`.

## Vendored header (`surrealdb.h`)

The C header from surrealdb.c is vendored in the module directory as `surrealdb.h`.
This means CGO can resolve all C symbols without any `CGO_CFLAGS` — IDE tooling
(gopls, clangd) works out of the box with no extra configuration.

To refresh the vendored header after updating surrealdb.c:

```bash
cp /path/to/surrealdb.c/include/surrealdb.h .
# or, if using the Makefile:
go generate ./...
```
