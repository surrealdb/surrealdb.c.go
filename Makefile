SURREALDB_C_REPO ?= https://github.com/surrealdb/surrealdb.c.git
SURREALDB_C_REF  ?= main
SURREALDB_C_DIR  ?= build/surrealdb.c

SURREALDB_C_LIB     = $(SURREALDB_C_DIR)/target/release/libsurrealdb_c.a
SURREALDB_C_INCLUDE = $(SURREALDB_C_DIR)/include

# Only CGO_LDFLAGS is needed — the header is vendored in surrealdb.h
export CGO_LDFLAGS  := -L$(CURDIR)/$(SURREALDB_C_DIR)/target/release

.PHONY: all build test clean example

all: build

# ---------------------------------------------------------------------------
# build/surrealdb.c — clone + build the C library
# ---------------------------------------------------------------------------

build/surrealdb.c: $(SURREALDB_C_LIB)

$(SURREALDB_C_DIR)/Cargo.toml:
	git clone --depth 1 --branch $(SURREALDB_C_REF) $(SURREALDB_C_REPO) $(SURREALDB_C_DIR)

$(SURREALDB_C_LIB): $(SURREALDB_C_DIR)/Cargo.toml
	cd $(SURREALDB_C_DIR) && cargo build --release
	cp $(SURREALDB_C_INCLUDE)/surrealdb.h surrealdb.h

# ---------------------------------------------------------------------------
# Go targets
# ---------------------------------------------------------------------------

build: $(SURREALDB_C_LIB)
	go build ./...

test: $(SURREALDB_C_LIB)
	go test -v ./...

example: $(SURREALDB_C_LIB)
	cd example && go run .

clean:
	rm -rf build
	rm -f example/demo
