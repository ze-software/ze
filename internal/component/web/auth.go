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
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ssh"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// contextKey is an unexported type used for context keys in this package,
// preventing collisions with keys defined in other packages.
type contextKey struct{ name string }

// ctxKeyUsername is the context key used to store the authenticated username.
// Set by AuthMiddleware, read by getUsernameFromContext.
var ctxKeyUsername = &contextKey{"username"}

// withUsername returns a derived context carrying the authenticated username.
func withUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, ctxKeyUsername, username)
}

// getUsernameFromContext extracts the authenticated username from the request
// context. Returns an empty string if the context does not carry a username
// (e.g., the request was not processed by AuthMiddleware).
func getUsernameFromContext(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyUsername).(string); ok {
		return v
	}

	return ""
}

var logger = slogutil.Logger("web.auth")

// sessionTTL is the maximum lifetime of a web session before it must be
// re-authenticated. Expired sessions are invalidated on next validation.
const sessionTTL = 24 * time.Hour

// WebSession represents an authenticated user session.
type WebSession struct {
	Username  string
	Token     string
	CreatedAt time.Time
}

// SessionStore manages active user sessions. It maps session tokens to
// WebSession objects and enforces one session per user by tracking the current
// token for each username.
//
// NOT safe for concurrent use without the internal mutex -- all exported methods
// acquire the lock, but callers MUST NOT hold references to WebSession fields
// across concurrent operations without their own synchronization.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*WebSession // token -> session
	users    map[string]string      // username -> token
}

// AuthConfig holds configuration for authentication middleware.
type AuthConfig struct {
	Users         []ssh.UserConfig
	LoginRenderer func(w http.ResponseWriter, r *http.Request)
}

// NewSessionStore returns an initialized SessionStore ready for use.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*WebSession),
		users:    make(map[string]string),
	}
}

// CreateSession generates a new session for the given username. If the user
// already has an active session, the previous session is invalidated first.
// The session token is 32 bytes from crypto/rand, hex-encoded to 64 characters.
func (s *SessionStore) CreateSession(username string) (*WebSession, error) {
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

	session := &WebSession{
		Username:  username,
		Token:     token,
		CreatedAt: time.Now(),
	}
	s.sessions[token] = session
	s.users[username] = token

	logger.Info("session created", "username", username)

	return session, nil
}

// ValidateToken returns the session associated with the given token, or nil
// if the token is not valid or has expired (older than sessionTTL).
// Expired sessions are invalidated automatically.
func (s *SessionStore) ValidateToken(token string) *WebSession {
	s.mu.RLock()
	session := s.sessions[token]
	s.mu.RUnlock()

	if session == nil {
		return nil
	}

	if time.Since(session.CreatedAt) > sessionTTL {
		s.InvalidateUser(session.Username)
		return nil
	}

	return session
}

// InvalidateUser removes the session for the given username. This is a no-op
// if the user has no active session.
func (s *SessionStore) InvalidateUser(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, exists := s.users[username]
	if !exists {
		return
	}

	delete(s.sessions, token)
	delete(s.users, username)

	logger.Info("session invalidated", "username", username)
}

// AuthMiddleware returns an http.Handler that wraps next with authentication.
// It checks for a valid session cookie first, then falls back to Basic Auth
// for JSON API requests (no session is created for Basic Auth). Unauthenticated
// requests receive a 401 response rendered by loginRenderer.
//
// HTMX requests (HX-Request header) with expired sessions receive a 401 with
// a login overlay instead of a full page, enabling in-place session recovery.
func AuthMiddleware(store *SessionStore, users []ssh.UserConfig, loginRenderer func(w http.ResponseWriter, r *http.Request), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check session cookie first.
		if cookie, err := r.Cookie("ze-session"); err == nil {
			if session := store.ValidateToken(cookie.Value); session != nil {
				addSecurityHeaders(w)
				next.ServeHTTP(w, r.WithContext(withUsername(r.Context(), session.Username)))

				return
			}
		}

		// Fall back to Basic Auth for JSON API requests.
		if username, password, ok := parseBasicAuth(r); ok {
			if ssh.AuthenticateUser(users, username, password) {
				logger.Debug("basic auth accepted", "username", username)
				addSecurityHeaders(w)
				next.ServeHTTP(w, r.WithContext(withUsername(r.Context(), username)))

				return
			}

			logger.Warn("basic auth failed", "username", username, "remote", r.RemoteAddr)
		}

		// Unauthenticated: return 401 without WWW-Authenticate header.
		w.WriteHeader(http.StatusUnauthorized)
		loginRenderer(w, r)
	})
}

// LoginHandler returns an http.HandlerFunc that processes POST login requests.
// On successful authentication, it creates a session, sets the ze-session cookie,
// and redirects to "/". On failure, it returns 401 with the login page.
func LoginHandler(store *SessionStore, users []ssh.UserConfig, loginRenderer func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		if !ssh.AuthenticateUser(users, username, password) {
			logger.Warn("login failed", "username", username, "remote", r.RemoteAddr)
			w.WriteHeader(http.StatusUnauthorized)
			loginRenderer(w, r)

			return
		}

		session, err := store.CreateSession(username)
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

		// HTMX login: respond with redirect header so HTMX replaces the page.
		if r.Header.Get("HX-Request") == htmxRequestTrue {
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusOK)

			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// addSecurityHeaders sets standard security headers on authenticated responses.
func addSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
	w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	w.Header().Set("Cache-Control", "no-store")
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
