package driver

// Notification is a live query event received from the database.
type Notification struct {
	QueryID string
	Action  string
	Data    any
}
