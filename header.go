// This file vendors the surrealdb.c header so that:
//   - IDE tooling (gopls) resolves C symbols without CGO_CFLAGS
//   - go build finds the header without external configuration
//
// The vendored header is surrealdb.h in this directory. To refresh it
// from a local surrealdb.c build, run:
//
//	go generate ./...
//
// Or copy manually:
//
//	cp /path/to/surrealdb.c/include/surrealdb.h .

package surrealdb

//go:generate cp -f build/surrealdb.c/include/surrealdb.h surrealdb.h
