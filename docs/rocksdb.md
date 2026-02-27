# RocksDB backend

By default surrealdb.c is built with the `kv-mem` and `kv-surrealkv` storage
backends.  To use `rocksdb://` endpoints you need a separate build of the C
library with the `kv-rocksdb` feature enabled.

## Using the Makefile (easiest)

The project Makefile handles everything automatically — patching the upstream
`Cargo.toml`, building into a separate target directory, and wiring up the
correct `CGO_LDFLAGS`:

```bash
make test/rocksdb   # build rocksdb-enabled library + run tests
```

The rocksdb-enabled `libsurrealdb_c.a` is placed in
`build/surrealdb.c/target-rocksdb/release/` (or the `CARGO_BUILD_TARGET`
subdirectory on macOS) so it never collides with the default library.

You can run all three backends in sequence:

```bash
make test/mem test/surrealkv test/rocksdb
```

## Manual build (without Make)

### 1. Clone surrealdb.c

```bash
git clone --depth 1 https://github.com/surrealdb/surrealdb.c.git
cd surrealdb.c
```

### 2. Add the `kv-rocksdb` feature

Append a `[features]` section to `Cargo.toml` that forwards the feature to the
`surrealdb` dependency:

```bash
cat >> Cargo.toml <<'EOF'

[features]
kv-rocksdb = ["surrealdb/kv-rocksdb"]
EOF
```

Verify the patch:

```bash
git diff -- Cargo.toml
# Should show:
#   +[features]
#   +kv-rocksdb = ["surrealdb/kv-rocksdb"]
```

### 3. Build with the feature enabled

Use `--target-dir` to keep the output separate from a default (non-rocksdb)
build:

```bash
cargo build --release --features kv-rocksdb --target-dir target-rocksdb
# Produces: target-rocksdb/release/libsurrealdb_c.a
```

### 4. Point CGO to the library

```bash
export CGO_LDFLAGS="-L/path/to/surrealdb.c/target-rocksdb/release"

go build ./...
go test -v ./...
```

If `CARGO_BUILD_TARGET` is set, adjust the path accordingly (e.g.
`target-rocksdb/aarch64-apple-darwin/release`).

## Building your own application with RocksDB

Copy this Makefile into your project.  It builds a rocksdb-enabled
`libsurrealdb_c.a` from source and links your Go application against it:

```makefile
SURREALDB_C_REPO ?= https://github.com/surrealdb/surrealdb.c.git
SURREALDB_C_REF  ?= main
SURREALDB_C_DIR  ?= build/surrealdb.c

ifdef CARGO_BUILD_TARGET
SURREALDB_C_LIBDIR = $(SURREALDB_C_DIR)/target-rocksdb/$(CARGO_BUILD_TARGET)/release
else
SURREALDB_C_LIBDIR = $(SURREALDB_C_DIR)/target-rocksdb/release
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

$(SURREALDB_C_DIR)/Cargo.toml:
	git clone --depth 1 --branch $(SURREALDB_C_REF) $(SURREALDB_C_REPO) $(SURREALDB_C_DIR)

# Patch Cargo.toml to expose kv-rocksdb, then build with the feature.
$(SURREALDB_C_LIB): $(SURREALDB_C_DIR)/Cargo.toml
	@grep -q '\[features\]' $(SURREALDB_C_DIR)/Cargo.toml || { \
		printf '\n[features]\nkv-rocksdb = ["surrealdb/kv-rocksdb"]\n' \
			>> $(SURREALDB_C_DIR)/Cargo.toml; \
	}
	cd $(SURREALDB_C_DIR) && cargo build --release --features kv-rocksdb --target-dir target-rocksdb

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
make run    # clones surrealdb.c, patches, cargo build --features kv-rocksdb, go build
make test   # same, but runs tests
```

The first build takes a while (RocksDB compiles from C++ source as part of the
Rust crate), but subsequent runs are instant.

## Connecting to a RocksDB database

```go
ctx := context.Background()
db, err := surrealdb.Open(ctx, "rocksdb:///var/data/mydb")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

db.Use(ctx, "my_ns", "my_db")
```

The path after `rocksdb://` is the directory where RocksDB stores its data
files.  The directory is created automatically on first use.
