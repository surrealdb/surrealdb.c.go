module github.com/surrealdb/surrealdb.c.go

go 1.23

require (
	github.com/stretchr/testify v1.8.4
	github.com/surrealdb/surrealdb.c.go/driver/go v0.0.0
)

replace github.com/surrealdb/surrealdb.c.go/driver/go => ./driver/go

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/surrealdb/surrealdb.go v1.3.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
