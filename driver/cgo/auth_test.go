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

// For record access, user/pass are ordinary variables and may be any type.
func TestParseAuthRecordUserPassAnyType(t *testing.T) {
	r, err := parseAuth(map[string]any{
		"NS": "n", "DB": "d", "AC": "a",
		"user": map[string]any{"id": 1}, "pass": 42,
	})
	if err != nil {
		t.Fatalf("parseAuth: %v", err)
	}
	if _, ok := r.recordParams["user"].(map[string]any); !ok {
		t.Errorf("user param = %#v, want the map", r.recordParams["user"])
	}
	if r.recordParams["pass"] != 42 {
		t.Errorf("pass param = %#v, want 42", r.recordParams["pass"])
	}
	if _, err := parseAuth(map[string]any{"user": 1, "pass": "p"}); err == nil {
		t.Error("root auth with a non-string user succeeded")
	}
}
