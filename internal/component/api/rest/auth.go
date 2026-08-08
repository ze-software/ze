// Design: docs/architecture/api/architecture.md -- REST transport authentication
// Related: server.go -- RESTServer holds the credentials this file reads and replaces
//
// Request authentication for the REST transport: the per-request gate, the
// caller identity it publishes, and the reload path that replaces the
// credentials on a running server.
//
// The credentials are a single shared bearer token, a per-user authenticator,
// or neither. They were written once at construction and read unsynchronized
// until a SIGHUP reload could change them; every access now goes through
// authSnapshot, so a request in flight during a reload reads one consistent
// pair. UpdateAuth is the hub's reload seam (an AuthUpdatable in the hub's
// listener migrator), and it never touches a listener.

package rest

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// usernameKey is the request-context key for the authenticated username.
type usernameKeyType struct{}

var usernameKey = usernameKeyType{}

type readOnlyKeyType struct{}

var readOnlyKey = readOnlyKeyType{}

// authSnapshot returns the credentials gating requests right now. The read is
// synchronized because UpdateAuth replaces them while the server is serving, so
// a request in flight during a reload sees the old pair or the new one and
// never a mix of the two.
func (s *RESTServer) authSnapshot() (string, Authenticator) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token, s.authenticator
}

// Authenticated reports whether every request is gated right now. It reads the
// live credentials rather than the ones the server was constructed with, so the
// reload exposure guard classifies what the server actually serves. A server
// whose authentication an earlier reload rebuilt answers for its current state,
// which is what makes that guard safe to re-run on every reload.
func (s *RESTServer) Authenticated() bool {
	token, authenticator := s.authSnapshot()
	return authenticator != nil || token != ""
}

// UpdateAuth installs reloaded credentials without rebinding any listener, and
// returns a function that puts the previous credentials back. A reload that
// fails after this call runs that function, so a partially applied reload never
// leaves the server less authenticated than it was.
//
// An empty token with a nil authenticator means no authentication. That is safe
// to install here and only here: NewRESTServer and Reconfigure both refuse
// every non-loopback address, so a REST server without credentials stays
// reachable from the local host alone.
//
// The undo needs no address check for the same reason, and that is the whole of
// why REST differs from gRPC here. gRPC's undo re-runs checkGRPCListenAddr,
// because a gRPC listener CAN be non-loopback and a reload moves it between the
// moment credentials are captured and the moment they are put back. REST has no
// such moment: there is no address a REST listener can hold that makes removing
// its credentials an exposure. If REST ever accepts a non-loopback address,
// this undo becomes the same defect and must gain the same check.
func (s *RESTServer) UpdateAuth(token string, authenticator func(authHeader string) (username string, ok bool)) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return nil, errors.New("REST server has been shut down")
	}

	prevToken, prevAuthenticator := s.token, s.authenticator
	s.token = token
	s.authenticator = authenticator
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.token = prevToken
		s.authenticator = prevAuthenticator
	}, nil
}

// withAuth wraps a handler with Bearer token authentication and CORS.
// On success, stores the authenticated username in the request context.
func (s *RESTServer) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
		}

		// One snapshot for the whole request: reading the fields twice would
		// let a reload land between the read-only decision and the credential
		// check, and answer the two from different configurations.
		token, authenticator := s.authSnapshot()

		username := "api" // default for no-auth mode
		readOnly := authenticator == nil && token == ""

		// Per-user authenticator takes precedence over single token.
		if authenticator != nil {
			auth := r.Header.Get("Authorization")
			user, ok := authenticator(auth)
			if !ok {
				s.recordAuthFailure(r, attemptedBearerUser(auth))
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			username = user
			readOnly = false
		} else if token != "" {
			auth := r.Header.Get("Authorization")
			var tb textbuf.Buffer
			expected := tb.Str("Bearer ").Str(token).String()
			gotHash := sha256.Sum256([]byte(auth))
			wantHash := sha256.Sum256([]byte(expected))
			if subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) != 1 {
				s.recordAuthFailure(r, attemptedBearerUser(auth))
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			readOnly = false
		}

		ctx := context.WithValue(r.Context(), usernameKey, username)
		ctx = context.WithValue(ctx, readOnlyKey, readOnly)
		next(w, r.WithContext(ctx))
	}
}

// callerIdentity extracts trusted caller metadata from the request.
func (s *RESTServer) callerIdentity(r *http.Request) api.CallerIdentity {
	readOnly, _ := r.Context().Value(readOnlyKey).(bool)
	if user, ok := r.Context().Value(usernameKey).(string); ok {
		return api.CallerIdentity{Username: user, RemoteAddr: r.RemoteAddr, Surface: audit.REST, ReadOnly: readOnly}
	}
	token, authenticator := s.authSnapshot()
	return api.CallerIdentity{Username: "api", RemoteAddr: r.RemoteAddr, Surface: audit.REST, ReadOnly: authenticator == nil && token == ""}
}

func (s *RESTServer) requireWriteAccess(w http.ResponseWriter, caller api.CallerIdentity, command string) bool {
	if caller.ReadOnly {
		writeError(w, http.StatusForbidden, "read-only API caller cannot modify configuration")
		return false
	}
	if s.authorizer == nil {
		return true
	}
	if s.authorizer.Authorize(caller.Username, caller.RemoteAddr, command, false) {
		return true
	}
	writeError(w, http.StatusForbidden, "API caller is not authorized to modify configuration")
	return false
}
