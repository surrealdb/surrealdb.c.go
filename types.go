package surrealdb

// Option configures the behavior of Open.
type Option func(*openConfig)

type openConfig struct {
	maxPendingOps int
}

// WithMaxPendingOps sets the maximum number of goroutines that may be blocked
// inside C FFI calls per DB instance at any time.
// Default: 16. Only applies to the embedded (C) backend.
func WithMaxPendingOps(n int) Option {
	return func(c *openConfig) { c.maxPendingOps = n }
}
