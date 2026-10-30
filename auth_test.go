package surrealdb

import (
	"context"
	"strings"
	"testing"
)

func isJWT(s string) bool {
	return len(strings.Split(s, ".")) == 3
}

// returned unwraps the result of a RETURN statement, which the C backend
// delivers as a one-element array for non-array values.
func returned(v any) any {
	if arr, ok := v.([]any); ok && len(arr) == 1 {
		return arr[0]
	}
	return v
}

func TestSignInSystemUsers(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, `
		DEFINE USER root_user ON ROOT PASSWORD 'root_pw' ROLES OWNER;
		DEFINE USER ns_user ON NAMESPACE PASSWORD 'ns_pw' ROLES EDITOR;
		DEFINE USER db_user ON DATABASE PASSWORD 'db_pw' ROLES VIEWER;
	`, nil)

	for _, tc := range []struct {
		name string
		auth any
	}{
		{"root", Auth{Username: "root_user", Password: "root_pw"}},
		{"namespace", Auth{Namespace: "test_ns", Username: "ns_user", Password: "ns_pw"}},
		{"database", &Auth{Namespace: "test_ns", Database: "test_db", Username: "db_user", Password: "db_pw"}},
		{"database map", map[string]any{"NS": "test_ns", "DB": "test_db", "user": "db_user", "pass": "db_pw"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := db.SignIn(ctx, tc.auth)
			if err != nil {
				t.Fatalf("SignIn: %v", err)
			}
			if !isJWT(token) {
				t.Fatalf("SignIn returned %q, want a JWT", token)
			}
		})
	}

	if _, err := db.SignIn(ctx, Auth{Username: "root_user", Password: "wrong"}); err == nil {
		t.Error("SignIn with a wrong password succeeded")
	}
	if _, err := db.SignIn(ctx, Auth{Database: "test_db", Username: "db_user", Password: "db_pw"}); err == nil {
		t.Error("SignIn with a database but no namespace succeeded")
	}
	if _, err := db.SignIn(ctx, 42); err == nil {
		t.Error("SignIn with an unsupported auth type succeeded")
	}
}

func TestRecordAccess(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	mustQuery(t, db, ctx, `
		DEFINE TABLE user SCHEMALESS PERMISSIONS FOR select WHERE id = $auth.id;
		DEFINE ACCESS account ON DATABASE TYPE RECORD
			SIGNUP (CREATE user SET email = $email, pass = crypto::argon2::generate($pass))
			SIGNIN (SELECT * FROM user WHERE email = $email AND crypto::argon2::compare(pass, $pass))
			DURATION FOR SESSION 1h;
	`, nil)

	creds := map[string]any{
		"NS": "test_ns", "DB": "test_db", "AC": "account",
		"email": "ada@example.com", "pass": "s3cret",
	}

	token, err := db.SignUp(ctx, creds)
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if !isJWT(token) {
		t.Fatalf("SignUp returned %q, want a JWT", token)
	}

	token, err = db.SignIn(ctx, creds)
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	if err := db.Invalidate(ctx); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	res := mustQuery(t, db, ctx, "RETURN $auth", nil)
	if returned(res[0].Value) != nil {
		t.Fatalf("$auth after Invalidate = %v, want none", res[0].Value)
	}

	if err := db.Authenticate(ctx, token); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	res = mustQuery(t, db, ctx, "RETURN $auth.email", nil)
	if returned(res[0].Value) != "ada@example.com" {
		t.Fatalf("$auth.email after Authenticate = %v", res[0].Value)
	}

	if err := db.Authenticate(ctx, "not-a-token"); err == nil {
		t.Error("Authenticate with a bad token succeeded")
	}

	bad := map[string]any{"NS": "test_ns", "DB": "test_db", "AC": "account", "email": "ada@example.com", "pass": "wrong"}
	if _, err := db.SignIn(ctx, bad); err == nil {
		t.Error("record SignIn with a wrong password succeeded")
	}
	if _, err := db.SignUp(ctx, Auth{Username: "x", Password: "y"}); err == nil {
		t.Error("SignUp without an access method succeeded")
	}
}
