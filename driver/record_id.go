package driver

import (
	"fmt"
	"reflect"
)

// RecordID represents a SurrealDB record ID (a.k.a. "Thing").
// It consists of a table name and an identifier, which together
// uniquely identify a record within a SurrealDB database.
//
// The type parameter T specifies the Go type of the identifier.
// Use RecordID[any] when the ID type is unknown or mixed,
// RecordID[string] for string IDs, RecordID[int] for numeric IDs, etc.
type RecordID[T any] struct {
	Table string
	ID    T
}

// NewRecordID creates a RecordID from a table name and an identifier.
func NewRecordID[T any](table string, id T) RecordID[T] {
	return RecordID[T]{Table: table, ID: id}
}

// String returns the canonical "table:id" representation.
func (r RecordID[T]) String() string {
	return fmt.Sprintf("%s:%v", r.Table, r.ID)
}

// IsRecordIDType reports whether t is a RecordID[T] instantiation
// for any T. This uses structural matching (exported Table string +
// ID fields) so it works across all generic instantiations.
func IsRecordIDType(t reflect.Type) bool {
	if t.Kind() != reflect.Struct || t.NumField() != 2 {
		return false
	}
	f0, f1 := t.Field(0), t.Field(1)
	return f0.Name == "Table" && f0.Type.Kind() == reflect.String && f1.Name == "ID"
}
