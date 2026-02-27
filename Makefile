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

export CGO_LDFLAGS  := -L$(CURDIR)/$(SURREALDB_C_LIBDIR)

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
