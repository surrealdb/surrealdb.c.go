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

func firstString(m map[string]any, keys []string) (string, error) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("surrealdb: auth field %q must be a string, got %T", k, v)
		}
		return s, nil
	}
	return "", nil
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
	var err error
	if r.ns, err = firstString(m, nsKeys); err != nil {
		return r, err
	}
	if r.db, err = firstString(m, dbKeys); err != nil {
		return r, err
	}
	if r.ac, err = firstString(m, acKeys); err != nil {
		return r, err
	}
	if r.user, err = firstString(m, userKeys); err != nil {
		return r, err
	}
	if r.pass, err = firstString(m, passKeys); err != nil {
		return r, err
	}

	switch {
	case r.ac != "":
		r.level = authRecord
		r.recordParams = make(map[string]any, len(m))
		for k, v := range m {
			if !isAnyOf(k, nsKeys, dbKeys, acKeys) {
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

func isAnyOf(k string, lists ...[]string) bool {
	for _, l := range lists {
		for _, s := range l {
			if k == s {
				return true
			}
		}
	}
	return false
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
