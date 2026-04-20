// Design: docs/architecture/mcp/overview.md -- MCP authentication dispatcher and identity types
// Related: streamable.go -- HTTP dispatcher; routes requests through Authenticate
// Related: session.go -- session carries Identity after initialize

// Authentication types and dispatcher for MCP 2025-06-18.
//
// Four auth modes: none, bearer (single shared token), bearer-list (per-identity
// tokens), oauth (OAuth 2.1 resource server with local JWT verify). The concrete
// strategies live in bearer.go and oauth.go; this file holds the enum, the
// Identity value type, and error sentinels shared across strategies.

package mcp

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// AuthMode enumerates MCP authentication strategies.
//
// Zero value is AuthUnspecified (invalid) so uninitialised configurations
// surface as errors rather than silently defaulting to None.
type AuthMode uint8

const (
	AuthUnspecified AuthMode = 0
	AuthNone        AuthMode = 1
	AuthBearer      AuthMode = 2
	AuthBearerList  AuthMode = 3
	AuthOAuth       AuthMode = 4
)

// ErrAuthModeInvalid wraps ParseAuthMode errors for typed detection.
var ErrAuthModeInvalid = errors.New("auth-mode: invalid value")

// String returns the YANG-string form of a mode; unknown values return
// "unspecified" so log messages never panic on a corrupted enum.
func (m AuthMode) String() string {
	switch m {
	case AuthNone:
		return "none"
	case AuthBearer:
		return "bearer"
	case AuthBearerList:
		return "bearer-list"
	case AuthOAuth:
		return "oauth"
	default:
		return "unspecified"
	}
}

// ParseAuthMode converts a YANG enumeration string to a typed AuthMode.
//
// The empty string returns AuthUnspecified with no error so callers can
// distinguish "operator did not set auth-mode" from "operator set an invalid
// value". Any other unknown value returns a wrapped ErrAuthModeInvalid.
func ParseAuthMode(s string) (AuthMode, error) {
	switch s {
	case "":
		return AuthUnspecified, nil
	case "none":
		return AuthNone, nil
	case "bearer":
		return AuthBearer, nil
	case "bearer-list":
		return AuthBearerList, nil
	case "oauth":
		return AuthOAuth, nil
	default:
		return AuthUnspecified, fmt.Errorf("%w: %q", ErrAuthModeInvalid, s)
	}
}

// Identity names the authenticated principal for a session.
//
// Value type by design: Phase 4 (tasks) scopes ownership by Identity and
// must be able to compare / copy it safely across the MCP component's
// internal seams without pointer-aliasing concerns.
//
// Zero value (empty Name, nil Scopes) represents "anonymous" -- the state
// used for AuthMode=None and for the period between accepting a request
// and running the auth dispatcher.
type Identity struct {
	Name   string
	Scopes []string
}

// IsAnonymous reports whether no identity was authenticated.
func (i Identity) IsAnonymous() bool {
	return i.Name == ""
}

// HasScope reports whether the identity carries the exact scope token.
// Exact-match only; no prefix logic. Callers that need hierarchical
// matching compose HasScope calls.
func (i Identity) HasScope(scope string) bool {
	return slices.Contains(i.Scopes, scope)
}

// authenticator is the strategy interface implemented by each auth mode.
//
// Authenticate inspects the request and returns the attached Identity on
// success, or an authError describing the failure.
type authenticator interface {
	// Authenticate inspects the request headers and returns the identity
	// that should ride on the session. Returns a non-nil *authError on any
	// rejection; the caller renders the HTTP response from its fields.
	Authenticate(r *http.Request) (Identity, *authError)
}

// authError carries the information needed to render a 401 / 403 response.
// Fields map to RFC 6750 Bearer challenge components plus the RFC 9728
// resource_metadata parameter. Zero value renders a bare 401 with no
// WWW-Authenticate header.
type authError struct {
	Status           int
	Scheme           string // "Bearer" for bearer / bearer-list / oauth; empty for none-mode rejection
	Realm            string
	ErrorCode        string // RFC 6750: invalid_request, invalid_token, insufficient_scope
	ErrorDescription string
	Scope            string // required scope when ErrorCode == insufficient_scope
	ResourceMetadata string // RFC 9728: absolute URL of the protected-resource metadata document
}

// Error satisfies error so authError flows through standard error channels.
func (e *authError) Error() string {
	if e == nil {
		return ""
	}
	if e.ErrorDescription != "" {
		return e.ErrorDescription
	}
	return e.ErrorCode
}

// WWWAuthenticate renders the RFC 6750 / RFC 9728 challenge header value.
// Returns "" when Scheme is unset (non-Bearer rejection).
func (e *authError) WWWAuthenticate() string {
	if e == nil || e.Scheme == "" {
		return ""
	}
	var b []byte
	b = append(b, e.Scheme...)
	first := true
	appendParam := func(k, v string) {
		if v == "" {
			return
		}
		if first {
			b = append(b, ' ')
			first = false
		} else {
			b = append(b, ',', ' ')
		}
		b = append(b, k...)
		b = append(b, '=', '"')
		// Per RFC 7235 the value is a quoted-string; backslash / quote are
		// the only characters that require escaping. Our values come from
		// config and are simple URLs or tokens, but escape defensively.
		for i := range len(v) {
			c := v[i]
			if c == '"' || c == '\\' {
				b = append(b, '\\')
			}
			b = append(b, c)
		}
		b = append(b, '"')
	}
	appendParam("realm", e.Realm)
	appendParam("error", e.ErrorCode)
	appendParam("error_description", e.ErrorDescription)
	appendParam("scope", e.Scope)
	appendParam("resource_metadata", e.ResourceMetadata)
	return string(b)
}

// extractBearerToken returns the bearer token from the Authorization header,
// or "" when absent / malformed / not a Bearer scheme.
func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	// Scheme is case-insensitive per RFC 7235; token follows single space.
	const prefix = "Bearer "
	if len(h) <= len(prefix) {
		return ""
	}
	if !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return h[len(prefix):]
}
