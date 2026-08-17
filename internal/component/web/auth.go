// Design: docs/architecture/web-interface.md -- Authentication and session management
// Related: editor.go -- Per-user editor management

// Package web provides the ze web interface, including session-based
// authentication middleware and security headers for all HTTP responses.
package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/errorfragment"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/version"
)

// contextKey is an unexported type used for context keys in this package,
// preventing collisions with keys defined in other packages.
type contextKey struct{ name string }

// ctxKeyUsername is the context key used to store the authenticated username.
// Set by the authentication middleware, read by GetUsernameFromRequest.
var ctxKeyUsername = &contextKey{"username"}

// ctxKeyProfiles is the context key used to store the authenticated user's
// authz profile names. Set by the authentication middleware, read by getProfilesFromRequest.
var ctxKeyProfiles = &contextKey{"profiles"}

// withUsername returns a derived context carrying the authenticated username.
func withUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, ctxKeyUsername, username)
}

// withProfiles returns a derived context carrying the authenticated user's
// authz profile names.
func withProfiles(ctx context.Context, profiles []string) context.Context {
	return context.WithValue(ctx, ctxKeyProfiles, profiles)
}

// GetUsernameFromRequest extracts the authenticated username from the request
// context. Returns an empty string if the context does not carry a username
// (e.g., the request was not processed by the authentication middleware).
func GetUsernameFromRequest(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyUsername).(string); ok {
		return v
	}

	return ""
}

// getProfilesFromRequest extracts the authenticated user's authz profile names
// from the request context. Returns nil if the context does not carry profiles.
func getProfilesFromRequest(r *http.Request) []string {
	if v, ok := r.Context().Value(ctxKeyProfiles).([]string); ok {
		return v
	}

	return nil
}

// InsecureMiddleware wraps a handler to inject a default username without
// authentication. Used only with --insecure-web for local testing.
func InsecureMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(withUsername(r.Context(), "insecure")))
	})
}

var logger = slogutil.Logger("web.auth")

// sessionTTL is the maximum lifetime of a web session before it must be
// re-authenticated. Expired sessions are invalidated on next validation.
const sessionTTL = 24 * time.Hour

// webSession represents an authenticated user session.
type webSession struct {
	Username  string
	Token     string
	CreatedAt time.Time
	// Profiles are the authz profile names the authenticator returned for this
	// user (AuthResult.Profiles). Carried so route gates and nav rendering can
	// reason about the session's authorization without re-querying (AC-2).
	Profiles []string
	// LocalAnchored reports that the LOCAL backend granted this session, and
	// therefore that removing the user from the local list MUST end it (AC-10).
	// It is the authenticator's own answer, carried on AuthResult.Source and
	// recorded verbatim at createSession.
	//
	// It is not re-derived from the user list. A list read taken after the
	// authenticator has answered is a second question asked at a later instant:
	// a reload landing in between made a locally-authenticated session report
	// "not local", which left it un-revocable for the rest of the 24h TTL, and
	// a name held by both the local list and a remote backend reported "local"
	// for a session the remote backend granted.
	//
	// It is false for a user a remote backend authenticated (RADIUS, TACACS+).
	// The local list cannot revoke a session it never granted, and re-checking
	// one against it would log out every remote-backend operator on the next
	// request.
	LocalAnchored bool
}

// SessionStore manages active user sessions. It maps session tokens to
// webSession objects and enforces one session per user by tracking the current
// token for each username.
//
// SessionStore serializes access to its mutable maps with an internal mutex.
// Callers MUST NOT hold references to webSession fields across concurrent
// operations without their own synchronization.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*webSession // token -> session
	users    map[string]string      // username -> token

	// localUsers returns the local credentials that are valid RIGHT NOW. The
	// store never holds a user list of its own: a session outlives the request
	// that created it, so any list copied here would answer for the config the
	// daemon booted with rather than the one it runs.
	localUsers func() ([]authz.UserConfig, error)
}

// AuthConfig holds configuration for authentication middleware.
type AuthConfig struct {
	Users         []authz.UserConfig
	LoginRenderer func(w http.ResponseWriter, r *http.Request)
}

// errNoLocalUserSource reports that the store was built with no live view of
// the local user list, so it cannot say whether a user is still declared.
var errNoLocalUserSource = errors.New("no live local user source")

// NewSessionStore returns an initialized SessionStore ready for use.
//
// localUsers returns the credentials the running configuration declares right
// now, read per call (the hub passes liveLocalUsers). Sessions it granted are
// re-checked against it on every request, so a user an operator removes and
// reloads loses an open browser tab at once instead of keeping full rights for
// the rest of the 24h session TTL.
//
// A nil localUsers leaves the store unable to answer whether a user is still
// declared, so it REFUSES every session the local backend granted rather than
// serving one it cannot check. Only a caller with no local backend at all may
// pass it.
func NewSessionStore(localUsers func() ([]authz.UserConfig, error)) *SessionStore {
	return &SessionStore{
		sessions:   make(map[string]*webSession),
		users:      make(map[string]string),
		localUsers: localUsers,
	}
}

// localUserDeclared reports whether the live local user list declares username.
//
// An unreadable list is an ERROR, never a "no". The two are indistinguishable
// to a caller that gets a bare false, and only one of them may keep a session
// alive (ai/rules/evidence.md, "a guard fails closed or says something").
//
// COST: this runs on every request that carries an anchored session, not only
// at login. validateToken calls it, so each `/fragment/*` and `/show/` request
// pays the hub's liveLocalUsers chain: one RLock and a shallow copy of the
// `system` root (config.Provider.Root), a walk of `authentication/user` that
// allocates one UserConfig slice plus a key slice per user
// (infra.ExtractAuthUsers), and one more slice and map to merge the power users
// (mergeAuthUsers). ExtractAuthUsers then SORTS, by name, the users and each
// user's public keys: its callers merge and log the result, and the map form it
// reads has no order to give them. No I/O and no parse. The sort is over the
// configured users, so it costs what a handful of names costs. Web requests are
// human-paced and this is not one of the paths ai/rules/performance.md governs,
// so the cost is accepted and NOT cached: a cache is the snapshot the
// per-request re-check exists to delete. Measure first if that ever changes,
// and any cache needs the reload to invalidate it.
func (s *SessionStore) localUserDeclared(username string) (bool, error) {
	if s.localUsers == nil {
		return false, errNoLocalUserSource
	}
	users, err := s.localUsers()
	if err != nil {
		return false, fmt.Errorf("reading the live local user list: %w", err)
	}
	for _, u := range users {
		if u.Name == username {
			return true, nil
		}
	}
	return false, nil
}

// createSession generates a new session for the given username from the result
// the authenticator returned. If the user already has an active session, the
// previous session is invalidated first. The session token is 32 bytes from
// crypto/rand, hex-encoded to 64 characters.
//
// result decides two things and is the sole input for both: the authz profile
// names carried on the session, and whether the local user list may later
// revoke it (AuthResult.GrantedByLocalBackend). Taking the second from the
// result rather than re-reading the user list is what makes the anchor the
// authenticator's answer instead of a guess about it: the store cannot observe
// the instant the authenticator answered, and a reload landing after it made
// the guess wrong in both directions (webSession.LocalAnchored).
func (s *SessionStore) createSession(username string, result authz.AuthResult) (*webSession, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generating session token: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Invalidate previous session for this user (one session per user).
	if oldToken, exists := s.users[username]; exists {
		delete(s.sessions, oldToken)
		logger.Debug("invalidated previous session", "username", username)
	}

	session := &webSession{
		Username:      username,
		Token:         token,
		CreatedAt:     time.Now(),
		Profiles:      result.Profiles,
		LocalAnchored: result.GrantedByLocalBackend(),
	}
	s.sessions[token] = session
	s.users[username] = token

	logger.Info("session created", "username", username)

	return session, nil
}

// validateToken returns the session associated with the given token, or nil
// if the token is not valid, has expired (older than sessionTTL), or belongs to
// a user the running configuration no longer declares. A session that fails any
// of these is invalidated automatically.
//
// The last check is what makes deleting a user take effect. A cookie is
// credential material this middleware accepted BEFORE the reload, so testing
// only the 24h TTL left a removed operator with full config-edit rights in an
// open tab for the rest of that day, while the reload reported success (AC-10).
// It is read per request from the same live list the local authenticator
// answers from, so the cookie path and the password path cannot disagree about
// who exists.
func (s *SessionStore) validateToken(token string) *webSession {
	s.mu.RLock()
	session := s.sessions[token]
	s.mu.RUnlock()

	if session == nil {
		return nil
	}

	if time.Since(session.CreatedAt) > sessionTTL {
		s.invalidateSession(session)
		return nil
	}

	if !session.LocalAnchored {
		return session
	}

	declared, err := s.localUserDeclared(session.Username)
	switch {
	case err != nil:
		// Deny rather than serve: a session granted by the local user list
		// cannot be renewed against a list that cannot be read.
		logger.Warn("session refused: cannot read the live local user list",
			"username", session.Username, "error", err)
	case !declared:
		logger.Info("session refused: the running configuration no longer declares this user",
			"username", session.Username)
	default:
		return session
	}

	s.invalidateSession(session)

	return nil
}

// invalidateSession removes exactly the session it is given, and only while
// that session is still the user's current one. A no-op otherwise.
//
// The identity check is the point. Invalidating by USERNAME destroys whatever
// token the user holds now, which is not always the token that failed: a
// request still carrying a revoked cookie arrives after the operator has
// re-added the user and that user has logged in again, and a username-scoped
// delete then kills the new session on behalf of the old one. The store already
// drops the previous token in createSession, so a session that is no longer
// s.users[username] has nothing left to remove.
func (s *SessionStore) invalidateSession(session *webSession) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.users[session.Username] != session.Token {
		return
	}

	delete(s.sessions, session.Token)
	delete(s.users, session.Username)

	logger.Info("session invalidated", "username", session.Username)
}

// authMiddleware returns an http.Handler that wraps next with authentication.
// It checks for a valid session cookie first, then falls back to Basic Auth
// for JSON API requests (no session is created for Basic Auth). Unauthenticated
// requests receive a 401 response rendered by loginRenderer.
//
// The cookie is not a bypass of the user list: store.validateToken re-checks
// the session against the credentials the running configuration declares right
// now, so a cookie issued before a reload that removed its user is refused here
// like any other bad credential.
//
// HTMX requests (HX-Request header) with expired sessions receive a 401 with
// a login overlay instead of a full page, enabling in-place session recovery.
func authMiddleware(store *SessionStore, authenticator authz.Authenticator, loginRenderer func(w http.ResponseWriter, r *http.Request), next http.Handler) http.Handler {
	return AuthMiddlewareWithAudit(store, authenticator, loginRenderer, next, nil)
}

// AuthMiddlewareWithAudit wraps next with authentication and records failed Basic Auth attempts.
func AuthMiddlewareWithAudit(store *SessionStore, authenticator authz.Authenticator, loginRenderer func(w http.ResponseWriter, r *http.Request), next http.Handler, recorder audit.Recorder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check session cookie first.
		if cookie, err := r.Cookie("ze-session"); err == nil {
			if session := store.validateToken(cookie.Value); session != nil {
				addSecurityHeaders(w)
				ctx := withProfiles(withUsername(r.Context(), session.Username), session.Profiles)
				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}
		}

		// Fall back to Basic Auth for JSON API requests.
		if username, password, ok := parseBasicAuth(r); ok {
			if result, err := authenticator.Authenticate(authz.AuthRequest{
				Username:   username,
				Password:   password,
				RemoteAddr: r.RemoteAddr,
			}); err == nil && result.Authenticated {
				logger.Debug("basic auth accepted", "username", username)
				addSecurityHeaders(w)
				ctx := withProfiles(withUsername(r.Context(), username), result.Profiles)
				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}

			logger.Warn("basic auth failed", "username", username, "remote", r.RemoteAddr)
			recordWebAuthFailure(recorder, username, r.RemoteAddr)
		}

		// Unauthenticated: return 401 without WWW-Authenticate header.
		addSecurityHeaders(w)
		w.WriteHeader(http.StatusUnauthorized)
		loginRenderer(w, r)
	})
}

// loginHandler returns an http.HandlerFunc that processes POST login requests.
// On successful authentication, it creates a session, sets the ze-session cookie,
// and redirects to "/". On failure, it returns 401 with the login page.
func loginHandler(store *SessionStore, authenticator authz.Authenticator, loginRenderer func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return LoginHandlerWithAudit(store, authenticator, loginRenderer, nil)
}

// LoginHandlerWithAudit returns a login handler that records failed login attempts.
func LoginHandlerWithAudit(store *SessionStore, authenticator authz.Authenticator, loginRenderer func(w http.ResponseWriter, r *http.Request), recorder audit.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 65536)

		username := r.FormValue("username")
		password := r.FormValue("password")

		result, err := authenticator.Authenticate(authz.AuthRequest{
			Username:   username,
			Password:   password,
			RemoteAddr: r.RemoteAddr,
		})
		if err != nil || !result.Authenticated {
			logger.Warn("login failed", "username", username, "remote", r.RemoteAddr)
			recordWebAuthFailure(recorder, username, r.RemoteAddr)
			addSecurityHeaders(w)
			w.WriteHeader(http.StatusUnauthorized)
			loginRenderer(w, r)

			return
		}

		session, err := store.createSession(username, result)
		if err != nil {
			logger.Error("failed to create session", "username", username, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)

			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "ze-session",
			Value:    session.Token,
			Path:     "/",
			MaxAge:   int(sessionTTL.Seconds()),
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})

		logger.Info("login successful", "username", username, "remote", r.RemoteAddr)

		target := sanitizeReturnTo(r.FormValue("return_to"))

		// HTMX login: respond with redirect header so HTMX replaces the page.
		if r.Header.Get("HX-Request") == htmxRequestTrue {
			w.Header().Set("HX-Redirect", target)
			w.WriteHeader(http.StatusOK)

			return
		}

		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}

func recordWebAuthFailure(recorder audit.Recorder, username, remoteAddr string) {
	if recorder == nil {
		return
	}
	if err := recorder.Record(audit.Entry{
		Actor:      username,
		RemoteAddr: remoteAddr,
		Surface:    audit.Web,
		Action:     audit.ActionAuthFail,
		Outcome:    audit.OutcomeDenied,
	}); err != nil {
		logger.Warn("audit record failed", "action", audit.ActionAuthFail, "user", username, "error", err)
	}
}

// sanitizeReturnTo validates the return_to parameter to prevent open redirects.
// Only same-origin paths starting with "/" are accepted; everything else
// falls back to "/".
func sanitizeReturnTo(raw string) string {
	if !isSameOriginPath(raw) {
		return "/"
	}
	return raw
}

// isSameOriginPath reports whether raw is a redirect target a browser resolves
// against the current origin. It must be a rooted path ("/..."), and must be
// neither scheme-relative ("//host", which a browser treats as an absolute URL
// to another site) nor backslash-escaped ("/\host", which several browsers
// normalize to "//host").
//
// Every redirect target derived from a request (query parameter, Referer,
// HX-Current-URL) MUST pass this before it reaches http.Redirect or an
// HX-Redirect header. A target that fails is not repairable: fall back to a
// known-safe path.
func isSameOriginPath(raw string) bool {
	if raw == "" || raw[0] != '/' || strings.HasPrefix(raw, "//") {
		return false
	}
	return len(raw) < 2 || raw[1] != '\\'
}

// addSecurityHeaders sets standard security headers on authenticated responses.
func addSecurityHeaders(w http.ResponseWriter) {
	setSecurityHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Ze-Version", version.HTTPHeader())
}

// setSecurityHeaders sets the four headers every response owes the browser,
// authenticated or not. securityHeaders applies them to the whole mux and
// addSecurityHeaders adds the two that belong to an authenticated page.
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'")
	w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
}

// securityHeaders wraps next so every response carries the security headers,
// whatever route served it.
//
// The headers used to be set per handler, inside the authentication
// middleware. Five responses never reach that middleware. They are the redirect
// at "/", "GET /favicon.ico", a hit and a miss under "/assets/", and the login
// redirect that hands out the session cookie. The root document is among them.
// The policy that constrains every sibling page did not constrain it.
//
// Cache-Control and the version header stay with addSecurityHeaders. A static
// asset is cacheable, and no-store over it would refetch the stylesheet and the
// scripts on every page load.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		next.ServeHTTP(w, r)
	})
}

// serverHandler wraps a mux with every middleware a response passes through,
// whatever route served it.
//
// It exists so the daemon and the golden capture cannot disagree about that
// chain. Both wrap their own mux, and a middleware added to one of them alone
// is a middleware the other's tests never see (server.go, handler_golden_test.go).
//
// It is unexported because every caller is in this package: the daemon builds
// its server here (server.go) and the captures live beside it. An exported name
// with no cross-package caller is what `make ze-repository-check` refuses.
func serverHandler(mux http.Handler) http.Handler {
	return securityHeaders(errorfragment.Middleware(mux))
}

// generateToken creates a cryptographically random 32-byte token, hex-encoded
// to 64 characters. Returns an error if the system's random source fails.
func generateToken() (string, error) {
	b := make([]byte, 32) //nolint:mnd // 32 bytes = 256 bits of entropy
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading crypto/rand: %w", err)
	}

	return hex.EncodeToString(b), nil
}

// parseBasicAuth extracts username and password from the Authorization header.
// Returns empty strings and false if the header is missing or malformed.
func parseBasicAuth(r *http.Request) (string, string, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", "", false
	}

	const basicLen = 6 // len("basic ")
	if len(auth) < basicLen || !strings.EqualFold(auth[:basicLen], "basic ") {
		return "", "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[basicLen:])
	if err != nil {
		return "", "", false
	}

	username, password, ok := strings.Cut(string(decoded), ":")

	return username, password, ok
}
