// Design: docs/architecture/mcp/overview.md -- MCP bearer + bearer-list auth strategies
// Related: auth.go -- authenticator interface + Identity types
// Related: streamable.go -- HTTP dispatcher routes through auth per AuthMode

// Bearer and bearer-list authenticator strategies.
//
// Bearer: one shared secret configured via the legacy `token` leaf. Matches
// the pre-Phase-2 behavior; exposed now as AuthMode=Bearer. Anonymous
// identity on success (single-principal).
//
// BearerList: per-identity token table configured via the `identity[]` YANG
// list. Each row pairs a principal name with a bearer token and an optional
// scope set. On match the session carries the matched Identity.
//
// Both strategies scan in constant time to avoid leaking valid-token presence
// via timing. noneAuthenticator is also defined here to keep the authenticator
// impls together.

package mcp

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

// hashToken returns a fixed-length SHA-256 digest of a token so
// ConstantTimeCompare cannot leak length information via its length-mismatch
// shortcut (which returns 0 in O(1) when inputs differ in length). All
// compared buffers are exactly 32 bytes, independent of the underlying token
// lengths. Not a cryptographic commitment -- purely a timing-side-channel
// mitigation for the equality test.
func hashToken(s string) [32]byte {
	return sha256.Sum256([]byte(s))
}

// noneAuthenticator accepts every request with an anonymous identity.
// Used only when AuthMode=None (bound in NewStreamable).
type noneAuthenticator struct{}

func (noneAuthenticator) Authenticate(_ *http.Request) (Identity, *authError) {
	return Identity{}, nil
}

// bearerAuthenticator compares the Authorization Bearer token against a
// single configured shared secret. The secret's SHA-256 digest is
// precomputed at build time so the request hot path does a single
// ConstantTimeCompare over fixed-length 32-byte digests (matches
// bearerListEntry for symmetry and defangs the ConstantTimeCompare
// length-mismatch shortcut). The plaintext token still lives in
// StreamableConfig for the life of the Streamable; this field is NOT a
// secret-zeroising optimisation.
type bearerAuthenticator struct {
	hash [32]byte // hashToken(configured token); all-zero if token was empty
}

func (a bearerAuthenticator) Authenticate(r *http.Request) (Identity, *authError) {
	got := extractBearerToken(r)
	if got == "" {
		return Identity{}, &authError{
			Status:           http.StatusUnauthorized,
			Scheme:           "Bearer",
			Realm:            mcpRealm,
			ErrorCode:        "invalid_request",
			ErrorDescription: "missing or malformed Authorization header",
		}
	}
	gotHash := hashToken(got)
	if subtle.ConstantTimeCompare(gotHash[:], a.hash[:]) != 1 {
		return Identity{}, &authError{
			Status:           http.StatusUnauthorized,
			Scheme:           "Bearer",
			Realm:            mcpRealm,
			ErrorCode:        "invalid_token",
			ErrorDescription: "bearer token does not match",
		}
	}
	return Identity{}, nil
}

// bearerListAuthenticator matches the Authorization Bearer token against a
// table of per-identity entries. The scan visits every row on every request
// regardless of early match so a network observer cannot infer which entry
// matched (or whether any matched) from response timing. Tokens are compared
// via SHA-256 hashes so ConstantTimeCompare's length-mismatch shortcut
// cannot leak token-length information either.
type bearerListAuthenticator struct {
	entries []bearerListEntry
}

type bearerListEntry struct {
	name   string
	hash   [32]byte // precomputed at build time so the hot path hashes only the incoming token
	scopes []string
}

func (a bearerListAuthenticator) Authenticate(r *http.Request) (Identity, *authError) {
	got := extractBearerToken(r)
	if got == "" {
		return Identity{}, &authError{
			Status:           http.StatusUnauthorized,
			Scheme:           "Bearer",
			Realm:            mcpRealm,
			ErrorCode:        "invalid_request",
			ErrorDescription: "missing or malformed Authorization header",
		}
	}
	gotHash := hashToken(got)
	// Walk every entry so timing does not reveal a match position. All
	// compares are over fixed-length 32-byte digests; length information
	// never reaches ConstantTimeCompare.
	matchIdx := -1
	for i, entry := range a.entries {
		if subtle.ConstantTimeCompare(gotHash[:], entry.hash[:]) == 1 {
			if matchIdx < 0 {
				matchIdx = i
			}
		}
	}
	if matchIdx < 0 {
		return Identity{}, &authError{
			Status:           http.StatusUnauthorized,
			Scheme:           "Bearer",
			Realm:            mcpRealm,
			ErrorCode:        "invalid_token",
			ErrorDescription: "bearer token does not match any identity",
		}
	}
	entry := a.entries[matchIdx]
	return Identity{Name: entry.name, Scopes: entry.scopes}, nil
}

const mcpRealm = "ze-mcp"

// buildAuthenticator returns the authenticator implementing mode.
//
// AuthOAuth requires runtime plumbing (AS metadata + JWKS cache) that the
// caller wires separately; pass a pre-built oauthAuthenticator via
// buildAuthenticatorWithOAuth when AuthMode=OAuth. This function covers the
// simple modes that need only the static config.
func buildAuthenticator(mode AuthMode, cfg StreamableConfig) authenticator {
	switch mode {
	case AuthBearer:
		return bearerAuthenticator{hash: hashToken(cfg.Token)}
	case AuthBearerList:
		entries := make([]bearerListEntry, len(cfg.BearerList))
		for i, id := range cfg.BearerList {
			entries[i] = bearerListEntry{name: id.Name, hash: hashToken(id.Token), scopes: id.Scopes}
		}
		return bearerListAuthenticator{entries: entries}
	default:
		// AuthNone / AuthUnspecified fall back to the permissive no-op so
		// the dispatcher attaches an anonymous identity. AuthOAuth is
		// handled by the caller (Phase H wires AS metadata + JWKS).
		return noneAuthenticator{}
	}
}
