module github.com/surrealdb/surrealdb.c.go/examples/basic

go 1.24

require github.com/surrealdb/surrealdb.c.go v0.0.0

replace (
	github.com/surrealdb/surrealdb.c.go => ../../
	github.com/surrealdb/surrealdb.c.go/driver/go => ../../driver/go
)
