# Unified Go interface (experimental)

surrealdb.c.go also provides a unified `Driver` interface that lets you swap between the embedded C backend and the pure Go SDK ([surrealdb.go](https://github.com/surrealdb/surrealdb.go)) based on the connection URL — same code, different backend.

By default, the CGo backend handles all schemes. Importing `driver/go` overrides `ws://` and `http://` to use the pure Go SDK instead:

| Scheme | Default backend | With `import _ "driver/go"` |
|--------|-----------------|----------------------------|
| `mem://`, `surrealkv://`, `rocksdb://` | [surrealdb.c](https://github.com/surrealdb/surrealdb.c) (CGo) | same |
| `ws://`, `wss://`, `http://`, `https://` | [surrealdb.c](https://github.com/surrealdb/surrealdb.c) (CGo) | [surrealdb.go](https://github.com/surrealdb/surrealdb.go) (pure Go) |

```go
import (
    surrealdb "github.com/surrealdb/surrealdb.c.go"
    _ "github.com/surrealdb/surrealdb.c.go/driver/go"
)

// Both use the same Driver interface and Query[T] function.
embedded, _ := surrealdb.Open(ctx, "mem://")
remote, _   := surrealdb.Open(ctx, "ws://localhost:8000")
```
