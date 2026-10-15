package cgo

// #include <stdlib.h>
// #include "surrealdb.h"
import "C"

import (
	"context"
	"errors"
	"fmt"

	"github.com/surrealdb/surrealdb.c.go/driver"
)

type authLevel int

const (
	authRoot      authLevel = C.ROOT
	authNamespace authLevel = C.NAMESPACE
	authDatabase  authLevel = C.DATABASE
	authRecord    authLevel = C.RECORD
)

type authRequest struct {
	level        authLevel
	ns, db, ac   string
	user, pass   string
	recordParams map[string]any
}

var (
	nsKeys   = []string{"NS", "ns", "namespace"}
	dbKeys   = []string{"DB", "db", "database"}
	acKeys   = []string{"AC", "ac", "access"}
	userKeys = []string{"user", "username"}
	passKeys = []string{"pass", "password"}
)

// firstString returns the value of the first of keys present in m, and the
// key it came from.
func firstString(m map[string]any, keys []string) (val, key string, err error) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			return "", k, fmt.Errorf("surrealdb: auth field %q must be a string, got %T", k, v)
		}
		return s, k, nil
	}
	return "", "", nil
}

// parseAuth turns a driver.Auth or map[string]any into an authRequest.
func parseAuth(authData any) (authRequest, error) {
	var m map[string]any
	switch a := authData.(type) {
	case driver.Auth:
		m = authMap(a)
	case *driver.Auth:
		if a == nil {
			return authRequest{}, errors.New("surrealdb: nil auth")
		}
		m = authMap(*a)
	case map[string]any:
		m = a
	default:
		return authRequest{}, fmt.Errorf("surrealdb: unsupported auth type %T (want Auth or map[string]any)", authData)
	}

	var r authRequest
	var nsKey, dbKey, acKey string
	var err error
	if r.ns, nsKey, err = firstString(m, nsKeys); err != nil {
		return r, err
	}
	if r.db, dbKey, err = firstString(m, dbKeys); err != nil {
		return r, err
	}
	if r.ac, acKey, err = firstString(m, acKeys); err != nil {
		return r, err
	}
	switch {
	case r.ac != "":
		r.level = authRecord
		r.recordParams = make(map[string]any, len(m))
		// Drop only the selector keys actually used, so a custom field
		// that happens to share an alias (e.g. "access" next to "AC")
		// still reaches the access method as a variable.
		for k, v := range m {
			if k != nsKey && k != dbKey && k != acKey {
				r.recordParams[k] = v
			}
		}
	case r.ns != "" && r.db != "":
		r.level = authDatabase
	case r.ns != "":
		r.level = authNamespace
	case r.db != "":
		return r, errors.New("surrealdb: auth with a database needs a namespace")
	default:
		r.level = authRoot
	}

	// For record access, user/pass are ordinary variables of any type and
	// travel in recordParams. Only system users need them as strings.
	if r.level != authRecord {
		if r.user, _, err = firstString(m, userKeys); err != nil {
			return r, err
		}
		if r.pass, _, err = firstString(m, passKeys); err != nil {
			return r, err
		}
	}
	return r, nil
}

func authMap(a driver.Auth) map[string]any {
	m := map[string]any{}
	set := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	set("NS", a.Namespace)
	set("DB", a.Database)
	set("AC", a.Access)
	set("user", a.Username)
	set("pass", a.Password)
	return m
}

// SignIn signs in and returns the access token.
func (c *Conn) SignIn(ctx context.Context, authData any) (string, error) {
	return c.authenticateWith(ctx, authData, false)
}

// SignUp signs up a record user and returns the access token.
func (c *Conn) SignUp(ctx context.Context, authData any) (string, error) {
	return c.authenticateWith(ctx, authData, true)
}

func (c *Conn) authenticateWith(ctx context.Context, authData any, signup bool) (string, error) {
	req, err := parseAuth(authData)
	if err != nil {
		return "", err
	}
	if signup && req.level != authRecord {
		return "", errors.New("surrealdb: signup needs a record access method (set Access)")
	}

	return run(ctx, c, func() (string, error) {
		var params *object
		if req.level == authRecord {
			params, err = marshalVars(req.recordParams)
			if err != nil {
				return "", fmt.Errorf("marshal auth params: %w", err)
			}
			defer params.Free()
		}

		var cStrs []*C.char
		cs := func(s string) C.sr_string_t {
			p := cstring(s)
			cStrs = append(cStrs, p)
			return C.sr_string_t(p)
		}
		defer func() {
			for _, p := range cStrs {
				freeCString(p)
			}
		}()

		creds := C.sr_credentials{username: cs(req.user), password: cs(req.pass)}
		details := C.sr_credentials_access{namespace_: cs(req.ns), database: cs(req.db), access: cs(req.ac)}
		scope := C.enum_sr_credentials_scope(req.level)

		var cParams *C.sr_object_t
		if params != nil {
			cParams = params.cptr()
		}

		var errPtr, token C.sr_string_t
		var rc C.int
		if signup {
			rc = C.sr_signup(c.c, &errPtr, &token, &scope, &creds, &details, cParams)
		} else {
			rc = C.sr_signin(c.c, &errPtr, &token, &scope, &creds, &details, cParams)
		}
		if err := surrealError(rc, errPtr); err != nil {
			return "", err
		}
		if token == nil {
			return "", nil
		}
		defer C.sr_free_string(token)
		return C.GoString(token), nil
	}, nil)
}

// Authenticate authenticates the connection with an access token.
func (c *Conn) Authenticate(ctx context.Context, token string) error {
	_, err := run(ctx, c, func() (struct{}, error) {
		cTok := cstring(token)
		defer freeCString(cTok)
		var errPtr C.sr_string_t
		rc := C.sr_authenticate(c.c, &errPtr, cTok)
		return struct{}{}, surrealError(rc, errPtr)
	}, nil)
	return err
}

// Invalidate clears the connection's authentication.
func (c *Conn) Invalidate(ctx context.Context) error {
	_, err := run(ctx, c, func() (struct{}, error) {
		var errPtr C.sr_string_t
		rc := C.sr_invalidate(c.c, &errPtr)
		return struct{}{}, surrealError(rc, errPtr)
	}, nil)
	return err
}
