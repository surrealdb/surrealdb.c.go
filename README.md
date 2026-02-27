# surrealdb.c.go

Go bindings for embedded SurrealDB via the [surrealdb.c](https://github.com/surrealdb/surrealdb.c) C FFI library.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    surrealdb "github.com/surrealdb/surrealdb.c.go"
)

type Person struct {
    ID   string `db:"id"`
    Name string `db:"name"`
    Age  int64  `db:"age"`
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

ifdef CARGO_BUILD_TARGET
SURREALDB_C_LIBDIR = $(SURREALDB_C_DIR)/target/$(CARGO_BUILD_TARGET)/release
else
SURREALDB_C_LIBDIR = $(SURREALDB_C_DIR)/target/release
endif

SURREALDB_C_LIB = $(SURREALDB_C_LIBDIR)/libsurrealdb_c.a

export CGO_LDFLAGS := -L$(CURDIR)/$(SURREALDB_C_LIBDIR)

# macOS: ensure Apple's system clang and SDK are used
ifeq ($(shell uname -s),Darwin)
  ifneq (,$(wildcard /Library/Developer/CommandLineTools/usr/bin/clang))
    export CC := /Library/Developer/CommandLineTools/usr/bin/clang
  endif
  ifneq (,$(wildcard /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/lib))
    export SDKROOT := /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk
  endif
endif

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

### macOS notes

On macOS you need Apple's system clang (not a third-party clang such as the
one provided by Nix or Homebrew's `llvm`) and the real macOS SDK. The Makefile
sets these automatically, but when running `go build` directly you may need to
export them yourself:

```bash
# Use Apple's Command Line Tools clang
export CC=/Library/Developer/CommandLineTools/usr/bin/clang

# Point to the real macOS SDK (important inside Nix shells where SDKROOT
# may reference an incomplete SDK)
export SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk

# If CARGO_BUILD_TARGET is set, cargo places output in a target-specific
# subdirectory (e.g. target/aarch64-apple-darwin/release instead of
# target/release). Adjust CGO_LDFLAGS accordingly:
export CGO_LDFLAGS="-L/path/to/surrealdb.c/target/aarch64-apple-darwin/release"
```

If you don't use Nix and have Xcode or Command Line Tools installed normally,
`CC` and `SDKROOT` typically don't need to be set — the defaults just work.

### Building your own application manually

After building the C library (steps above), add the dependency and build:

```bash
go get github.com/surrealdb/surrealdb.c.go@latest
CGO_LDFLAGS="-L/path/to/surrealdb.c/target/release" go build -o myapp .
```

If you installed `libsurrealdb_c.a` system-wide (`/usr/local/lib/`), no
environment variables are needed — just `go build`.

## API overview

See [docs/api_overview.md](docs/api_overview.md) for the full API reference
(connection lifecycle, querying, typed results, live queries, etc.).

## Storage backends

The default build includes `mem://` (in-memory) and `surrealkv://` (file-backed)
storage backends.  For RocksDB support, see [docs/rocksdb.md](docs/rocksdb.md).

## Internals

See [docs/internals.md](docs/internals.md) to learn about the internals of this
module (linking strategy, vendored header, etc.).
