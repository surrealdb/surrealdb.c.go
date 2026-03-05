# To build/test against a local surrealdb.c fork (use absolute path, not ~):
#   make test SURREALDB_C_DIR=/path/to/surrealdb.c
# If the library is already built there, cargo build is skipped automatically.
SURREALDB_C_REPO ?= https://github.com/surrealdb/surrealdb.c.git
SURREALDB_C_REF  ?= main
SURREALDB_C_DIR  ?= build/surrealdb.c

# When CARGO_BUILD_TARGET is set (e.g. aarch64-apple-darwin), cargo places
# artifacts under target/$(CARGO_BUILD_TARGET)/release instead of
# target/release.  Detect this so the paths work on both Linux CI and macOS.
ifdef CARGO_BUILD_TARGET
SURREALDB_C_LIBDIR = $(SURREALDB_C_DIR)/target/$(CARGO_BUILD_TARGET)/release
else
SURREALDB_C_LIBDIR = $(SURREALDB_C_DIR)/target/release
endif

SURREALDB_C_LIB     = $(SURREALDB_C_LIBDIR)/libsurrealdb_c.a
SURREALDB_C_INCLUDE = $(SURREALDB_C_DIR)/include

# Auto-detect pre-built: when SURREALDB_C_DIR is overridden and the
# library already exists, skip clone+build entirely.
ifneq ($(SURREALDB_C_DIR),build/surrealdb.c)
  ifneq (,$(wildcard $(SURREALDB_C_LIB)))
    _SURREALDB_C_PREBUILT := 1
  endif
endif

# Resolve SURREALDB_C_LIBDIR to an absolute path for CGO_LDFLAGS.
# Paths starting with / are already absolute; relative paths get $(CURDIR)/ prepended.
ifeq ($(filter /%,$(SURREALDB_C_LIBDIR)),)
  _SURREALDB_C_LIBDIR_ABS = $(CURDIR)/$(SURREALDB_C_LIBDIR)
else
  _SURREALDB_C_LIBDIR_ABS = $(SURREALDB_C_LIBDIR)
endif
export CGO_LDFLAGS  := -L$(_SURREALDB_C_LIBDIR_ABS)

# Rocksdb-enabled build uses a separate --target-dir so it doesn't
# overwrite the default (mem + surrealkv) library.
ifdef CARGO_BUILD_TARGET
SURREALDB_C_ROCKSDB_LIBDIR = $(SURREALDB_C_DIR)/target-rocksdb/$(CARGO_BUILD_TARGET)/release
else
SURREALDB_C_ROCKSDB_LIBDIR = $(SURREALDB_C_DIR)/target-rocksdb/release
endif
SURREALDB_C_ROCKSDB_LIB = $(SURREALDB_C_ROCKSDB_LIBDIR)/libsurrealdb_c.a

# ---------------------------------------------------------------------------
# macOS: use Apple's system clang and SDK so CGo works even inside a Nix shell
# (Nix-provided clang may be too old or have an incomplete SDK).
# ---------------------------------------------------------------------------
ifeq ($(shell uname -s),Darwin)
  APPLE_CLT_CC := /Library/Developer/CommandLineTools/usr/bin/clang
  ifneq (,$(wildcard $(APPLE_CLT_CC)))
    export CC := $(APPLE_CLT_CC)
  endif
  MACOS_SDK_CLT := /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk
  ifneq (,$(wildcard $(MACOS_SDK_CLT)/usr/lib))
    export SDKROOT := $(MACOS_SDK_CLT)
  endif
endif

GOLANGCI_LINT_VERSION ?= v2.10.1
GOLANGCI_LINT         = $(CURDIR)/bin/golangci-lint

.PHONY: all build test test/mem test/surrealkv test/rocksdb test/example test/example/basic test/example/surrealql clean example example/basic example/surrealql lint lint/install

all: build

# ---------------------------------------------------------------------------
# build/surrealdb.c — clone + build the C library (or use pre-built)
# ---------------------------------------------------------------------------

ifndef _SURREALDB_C_PREBUILT

build/surrealdb.c: $(SURREALDB_C_LIB)

$(SURREALDB_C_DIR)/Cargo.toml:
	git clone --depth 1 --branch $(SURREALDB_C_REF) $(SURREALDB_C_REPO) $(SURREALDB_C_DIR)

$(SURREALDB_C_LIB): $(SURREALDB_C_DIR)/Cargo.toml
	cd $(SURREALDB_C_DIR) && cargo build --release || { $(CURDIR)/scripts/diagnose-build.sh; exit 1; }
	cp $(SURREALDB_C_INCLUDE)/surrealdb.h surrealdb.h

$(SURREALDB_C_DIR)/.features-patched: $(SURREALDB_C_DIR)/Cargo.toml
	@grep -q '\[features\]' $(SURREALDB_C_DIR)/Cargo.toml || { \
		printf '\n[features]\nkv-rocksdb = ["surrealdb/kv-rocksdb"]\n' \
			>> $(SURREALDB_C_DIR)/Cargo.toml; \
	}
	@cd $(SURREALDB_C_DIR) && git diff -- Cargo.toml | grep -q '+kv-rocksdb' || { \
		echo "ERROR: Cargo.toml patch failed — expected [features] with kv-rocksdb"; \
		echo "git diff -- Cargo.toml:"; \
		cd $(SURREALDB_C_DIR) && git diff -- Cargo.toml; \
		exit 1; \
	}
	@touch $@
	@test -f $(SURREALDB_C_LIB) && touch $(SURREALDB_C_LIB) || true

$(SURREALDB_C_ROCKSDB_LIB): $(SURREALDB_C_DIR)/.features-patched
	cd $(SURREALDB_C_DIR) && cargo build --release --features kv-rocksdb --target-dir target-rocksdb || { $(CURDIR)/scripts/diagnose-build.sh; exit 1; }
	cp $(SURREALDB_C_INCLUDE)/surrealdb.h surrealdb.h

else

$(SURREALDB_C_LIB):
	@echo "Using pre-built library: $@"
	@cp -f $(SURREALDB_C_INCLUDE)/surrealdb.h surrealdb.h 2>/dev/null || true

endif

# ---------------------------------------------------------------------------
# Go targets
# ---------------------------------------------------------------------------

build: $(SURREALDB_C_LIB)
	go build ./...

test: test/mem test/surrealkv test/example

test/mem: $(SURREALDB_C_LIB)
	SURREALDB_ENDPOINT=mem:// go test -v -count=1 ./...

test/surrealkv: $(SURREALDB_C_LIB)
	SURREALDB_ENDPOINT=surrealkv:// go test -v -count=1 ./...

test/rocksdb: $(SURREALDB_C_ROCKSDB_LIB)
	CGO_LDFLAGS="-L$(CURDIR)/$(SURREALDB_C_ROCKSDB_LIBDIR)" SURREALDB_ENDPOINT=rocksdb:// go test -v -count=1 ./...

test/example: test/example/basic test/example/surrealql

test/example/basic: $(SURREALDB_C_LIB)
	cd examples/basic && go build ./... && go test -v -count=1 ./...

test/example/surrealql: $(SURREALDB_C_LIB)
	cd examples/surrealql && go build ./... && go test -v -count=1 ./...

example: example/basic example/surrealql

example/basic: $(SURREALDB_C_LIB)
	cd examples/basic && go run .

example/surrealql: $(SURREALDB_C_LIB)
	cd examples/surrealql && go run .

# ---------------------------------------------------------------------------
# Lint
# ---------------------------------------------------------------------------

lint/install:
	@test -x $(GOLANGCI_LINT) && $(GOLANGCI_LINT) version | grep -q $(GOLANGCI_LINT_VERSION) || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(CURDIR)/bin $(GOLANGCI_LINT_VERSION)

lint: $(SURREALDB_C_LIB) lint/install
	$(GOLANGCI_LINT) run ./...

clean:
	rm -rf build
