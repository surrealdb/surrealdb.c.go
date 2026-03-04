module github.com/surrealdb/surrealdb.c.go/driver/go

go 1.23

require (
	github.com/surrealdb/surrealdb.c.go v0.0.0
	github.com/surrealdb/surrealdb.go v1.3.0
)

require (
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/x448/float16 v0.8.4 // indirect
)

replace github.com/surrealdb/surrealdb.c.go => ../..
