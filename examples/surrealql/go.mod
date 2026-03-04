module github.com/surrealdb/surrealdb.c.go/examples/surrealql

go 1.24

replace (
	github.com/surrealdb/surrealdb.c.go => ../../
	github.com/surrealdb/surrealdb.c.go/driver/go => ../../driver/go
)

require (
	github.com/surrealdb/surrealdb.c.go v0.0.0-00010101000000-000000000000
	github.com/surrealdb/surrealdb.go v1.3.0
)

require (
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/x448/float16 v0.8.4 // indirect
)
