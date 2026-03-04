package surrealdb

import (
	"context"
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.c.go/driver/cgo"
	"github.com/surrealdb/surrealdb.c.go/internal/surrealdbc"
)

// Open connects to a SurrealDB instance at the given endpoint.
//
// The scheme determines which backend is used:
//   - "mem://", "surrealkv://...", "rocksdb://..." — embedded C backend (requires CGo + libsurrealdb_c)
//   - "ws://...", "wss://...", "http://...", "https://..." — remote backend via a registered driver
//
// Remote schemes require a driver to be registered first, typically via a
// blank import:
//
//	import _ "github.com/surrealdb/surrealdb.c.go/driver/go"
//
// The context controls the connection timeout. The caller must call Close
// on the returned Driver when done.
func Open(ctx context.Context, endpoint string, opts ...Option) (Driver, error) {
	cfg := openConfig{maxPendingOps: 16}
	for _, o := range opts {
		o(&cfg)
	}

	scheme := schemeOf(endpoint)

	switch scheme {
	case "mem", "surrealkv", "rocksdb", "file":
		return openEmbedded(ctx, endpoint, cfg)
	case "ws", "wss", "http", "https":
		opener := getRemoteDriver()
		if opener == nil {
			return nil, fmt.Errorf("surrealdb: no remote driver registered for scheme %q; import _ \"github.com/surrealdb/surrealdb.c.go/driver/go\" to enable", scheme)
		}
		return opener(ctx, endpoint)
	default:
		return nil, fmt.Errorf("surrealdb: unsupported scheme %q in endpoint %q", scheme, endpoint)
	}
}

func schemeOf(endpoint string) string {
	if idx := strings.Index(endpoint, "://"); idx >= 0 {
		return endpoint[:idx]
	}
	return ""
}

func openEmbedded(ctx context.Context, endpoint string, cfg openConfig) (Driver, error) {
	sdbCfg := surrealdbc.DefaultConfig()
	sdbCfg.MaxPendingOps = cfg.maxPendingOps
	return cgo.New(ctx, endpoint, sdbCfg)
}
