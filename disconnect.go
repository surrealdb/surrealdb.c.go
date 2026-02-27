package surrealdb

// Close disconnects from the database and releases all resources.
// The DB must not be used after Close is called.
func (db *DB) Close() {
	if db.conn != nil {
		db.conn.Close()
		db.conn = nil
	}
}
