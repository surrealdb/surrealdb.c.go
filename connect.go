package surrealdb

import (
	"context"
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.c.go/driver/cgo"
)

// Open connects to a SurrealDB instance at the given endpoint.
//
// The scheme determines which backend is used:
//   - "mem://", "surrealkv://...", "rocksdb://..." — always uses the C backend
//   - "ws://", "wss://", "http://", "https://" — uses a registered remote
//     driver (e.g. import _ ".../driver/go"), or falls back to the C backend
//     if no remote driver is registered (requires the corresponding protocol
//     feature to be enabled when building libsurrealdb_c)
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
		if opener := getRemoteDriver(); opener != nil {
			return opener(ctx, endpoint)
		}
		return openEmbedded(ctx, endpoint, cfg)
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
	sdbCfg := cgo.DefaultConfig()
	sdbCfg.MaxPendingOps = cfg.maxPendingOps
	return cgo.New(ctx, endpoint, sdbCfg)
}
