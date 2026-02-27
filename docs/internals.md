# Internals

## Linking strategy

The `#cgo` directives statically link `libsurrealdb_c.a` while keeping system
libraries dynamic, producing a self-contained binary with no runtime dependency
on a shared `libsurrealdb_c`.

- **Linux** — uses `-Wl,-Bstatic -lsurrealdb_c -Wl,-Bdynamic` (GNU ld) plus
  `-lpthread -ldl -lm -lrt -lstdc++`.
- **macOS** — uses `-lsurrealdb_c -framework CoreFoundation -lresolv -lm -lc++`.

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
