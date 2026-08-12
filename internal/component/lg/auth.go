// Design: docs/architecture/web-interface.md -- optional looking-glass bearer gate
// Related: server.go -- NewLGServer wraps the mux with bearerAuth before securityHeaders

package lg

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// bearerAuthPrefix is the only Authorization scheme the looking glass accepts.
// The trailing space is part of the match: "Bearer" alone carries no token.
// RFC 7235 Section 2.1 makes the auth-scheme case-insensitive, so the scheme is
// compared with EqualFold; the token after it is compared exactly.
const bearerAuthPrefix = "Bearer "

// bearerAuth wraps a handler with an optional constant-time bearer-token check.
//
// An empty token returns next unchanged: a looking glass is an intentionally
// public read-only surface, so the gate is opt-in and its absence must cost
// nothing. When a token IS set, every request through the wrapped handler must
// carry `Authorization: Bearer <token>`; the wrapper sits above the mux so a
// route added later is gated by construction rather than by remembering to
// gate it.
//
// The compare is over sha256 digests with subtle.ConstantTimeCompare, the same
// model as the gNMI server: fixed-length inputs, so neither the token's length
// nor its content leaks through timing.
func bearerAuth(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !bearerTokenMatches(r.Header.Get("Authorization"), want) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="looking glass"`)
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bearerTokenMatches reports whether an Authorization header value carries the
// expected token. Fail-closed: a missing header, a wrong scheme, a bare token
// with no scheme, and the prefix with nothing after it all return false rather
// than degrading to an empty-token comparison. The scheme match is
// case-insensitive per RFC 7235 Section 2.1, so a conforming client sending
// `bearer <token>` is accepted; the token itself stays case-sensitive.
func bearerTokenMatches(header string, want [sha256.Size]byte) bool {
	if len(header) <= len(bearerAuthPrefix) || !strings.EqualFold(header[:len(bearerAuthPrefix)], bearerAuthPrefix) {
		return false
	}
	got := sha256.Sum256([]byte(header[len(bearerAuthPrefix):]))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}
