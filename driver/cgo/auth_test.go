package cgo

import (
	"reflect"
	"testing"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

func TestParseAuthLevels(t *testing.T) {
	for _, tc := range []struct {
		name string
		auth any
		want authLevel
	}{
		{"root", driver.Auth{Username: "u", Password: "p"}, authRoot},
		{"namespace", driver.Auth{Namespace: "n", Username: "u", Password: "p"}, authNamespace},
		{"database", &driver.Auth{Namespace: "n", Database: "d", Username: "u", Password: "p"}, authDatabase},
		{"record", map[string]any{"namespace": "n", "database": "d", "access": "a"}, authRecord},
	} {
		r, err := parseAuth(tc.auth)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if r.level != tc.want {
			t.Errorf("%s: level %v, want %v", tc.name, r.level, tc.want)
		}
	}
}

// Only the selector keys actually used are removed from record params; a
// custom field sharing an alias with a selector is passed through.
func TestParseAuthKeepsCustomFieldsNamedLikeAliases(t *testing.T) {
	r, err := parseAuth(map[string]any{
		"NS": "n", "DB": "d", "AC": "account",
		"access": "free", "database": "custom", "email": "e",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.ns != "n" || r.db != "d" || r.ac != "account" {
		t.Fatalf("selectors = %q %q %q", r.ns, r.db, r.ac)
	}
	want := map[string]any{"access": "free", "database": "custom", "email": "e"}
	if !reflect.DeepEqual(r.recordParams, want) {
		t.Fatalf("recordParams = %v, want %v", r.recordParams, want)
	}
}
