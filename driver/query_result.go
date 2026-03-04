package driver

// QueryResult holds the outcome of a single statement within a query.
type QueryResult struct {
	// Value is the result for this statement. It is typically []any for
	// multi-row queries, or a single map[string]any / primitive for queries
	// like SELECT ... ONLY.
	Value any

	// Err is the per-statement error, if any.
	Err error
}
