package driver

// VersionData is the version of the SurrealDB server or embedded engine.
// It matches surrealdb.go's VersionData. Build and Timestamp are empty when
// the backend does not report them.
type VersionData struct {
	Version   string `json:"version"`
	Build     string `json:"build"`
	Timestamp string `json:"timestamp"`
}
