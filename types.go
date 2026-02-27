package surrealdb

import "github.com/surrealdb/surrealdb.c.go/internal/surrealdbc"

// Value wraps a single SurrealDB value returned from a query.
// Values are copied out of the C result set — they do not hold C resources.
type Value = surrealdbc.Value

// ValueTag identifies the type of a Value.
type ValueTag = surrealdbc.ValueTag

// KV is a key-value pair within an Object value.
type KV = surrealdbc.KV

// QueryResult holds the outcome of a single statement within a query.
// A multi-statement query produces multiple QueryResults.
type QueryResult = surrealdbc.QueryResult

// Action represents the type of change in a live query notification.
type Action = surrealdbc.Action

// Notification is a live query event received from the database.
type Notification = surrealdbc.Notification

const (
	ValueNone     = surrealdbc.ValueNone
	ValueNull     = surrealdbc.ValueNull
	ValueBool     = surrealdbc.ValueBool
	ValueNumber   = surrealdbc.ValueNumber
	ValueString   = surrealdbc.ValueString
	ValueDuration = surrealdbc.ValueDuration
	ValueDatetime = surrealdbc.ValueDatetime
	ValueUUID     = surrealdbc.ValueUUID
	ValueArray    = surrealdbc.ValueArray
	ValueObject   = surrealdbc.ValueObject
	ValueGeometry = surrealdbc.ValueGeometry
	ValueBytes    = surrealdbc.ValueBytes
	ValueThing    = surrealdbc.ValueThing
)

const (
	ActionCreate        = surrealdbc.ActionCreate
	ActionUpdate        = surrealdbc.ActionUpdate
	ActionDelete        = surrealdbc.ActionDelete
	ActionUnimplemented = surrealdbc.ActionUnimplemented
)
