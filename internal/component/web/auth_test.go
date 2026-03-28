package web

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"codeberg.org/thomas-mangin/ze/internal/component/ssh"
)

// testUsers returns a slice of UserConfig with a known bcrypt hash for "testpass".
func testUsers(t *testing.T) []ssh.UserConfig {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	require.NoError(t, err)

	return []ssh.UserConfig{
		{Name: "alice", Hash: string(hash)},
	}
}

// okHandler is a simple handler that returns 200 "ok" for wrapping with AuthMiddleware.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck // test helper
	})
}

// noopRenderer is a login renderer that writes nothing (used where the rendered content is not under test).
func noopRenderer(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("login page")) //nolint:errcheck // test helper
}

// TestSessionCookieValidation verifies that AuthMiddleware passes requests with
// a valid session cookie and rejects requests with an invalid or missing cookie.
// VALIDATES: AC-2 (missing session returns login page), AC-3 (valid session passes)
// PREVENTS: unauthenticated access to protected routes.
func TestSessionCookieValidation(t *testing.T) {
	store := NewSessionStore()
	users := testUsers(t)

	session, err := store.CreateSession("alice")
	require.NoError(t, err)

	handler := AuthMiddleware(store, users, noopRenderer, okHandler())

	tests := []struct {
		name       string
		cookie     string
		wantStatus int
	}{
		{
			name:       "valid session cookie",
			cookie:     session.Token,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid session cookie",
			cookie:     "bad-token-value",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing session cookie",
			cookie:     "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "ze-session", Value: tt.cookie})
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			// Unauthenticated responses MUST NOT include WWW-Authenticate header
			// (we show a login page, not a browser auth popup).
			if tt.wantStatus == http.StatusUnauthorized {
				assert.Empty(t, rec.Header().Get("WWW-Authenticate"),
					"401 response must not include WWW-Authenticate header")
			}
		})
	}
}

// TestSessionCreation verifies that CreateSession produces a valid 64-hex-char
// token and that the session is stored and retrievable.
// VALIDATES: AC-3 (session created on login)
// PREVENTS: weak or predictable session tokens.
func TestSessionCreation(t *testing.T) {
	store := NewSessionStore()

	session, err := store.CreateSession("alice")
	require.NoError(t, err)
	require.NotNil(t, session)

	// Token must be 64 hex characters (32 bytes hex-encoded).
	assert.Len(t, session.Token, 64, "token must be 64 hex characters")
	assert.Regexp(t, `^[0-9a-f]{64}$`, session.Token, "token must be lowercase hex")

	// Session must be stored and retrievable by token.
	found := store.ValidateToken(session.Token)
	require.NotNil(t, found, "session must be retrievable by token")
	assert.Equal(t, "alice", found.Username)

	// Username must be set correctly.
	assert.Equal(t, "alice", session.Username)
	assert.False(t, session.CreatedAt.IsZero(), "CreatedAt must be set")
}

// TestSessionInvalidation verifies that creating a new session for the same user
// invalidates the previous session token.
// VALIDATES: AC-10 (new login invalidates previous)
// PREVENTS: stale sessions remaining valid after re-login.
func TestSessionInvalidation(t *testing.T) {
	store := NewSessionStore()

	// Create first session.
	first, err := store.CreateSession("alice")
	require.NoError(t, err)
	firstToken := first.Token

	// Verify first session is valid.
	require.NotNil(t, store.ValidateToken(firstToken), "first session must be valid initially")

	// Create second session for the same user.
	second, err := store.CreateSession("alice")
	require.NoError(t, err)

	// First token must now be invalid.
	assert.Nil(t, store.ValidateToken(firstToken),
		"previous session token must be invalidated after new session creation")

	// Second token must be valid.
	assert.NotNil(t, store.ValidateToken(second.Token),
		"new session token must be valid")

	// Tokens must be different.
	assert.NotEqual(t, firstToken, second.Token,
		"new session must have a different token")
}

// TestBasicAuthForJSONAPI verifies that AuthMiddleware accepts Basic Auth for
// API requests when no session cookie is present.
// VALIDATES: AC-12 (JSON API with Basic Auth)
// PREVENTS: API clients being forced to use cookie-based sessions.
func TestBasicAuthForJSONAPI(t *testing.T) {
	store := NewSessionStore()
	users := testUsers(t)
	handler := AuthMiddleware(store, users, noopRenderer, okHandler())

	tests := []struct {
		name       string
		username   string
		password   string
		wantStatus int
	}{
		{
			name:       "valid basic auth credentials",
			username:   "alice",
			password:   "testpass",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid basic auth password",
			username:   "alice",
			password:   "wrongpass",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "unknown basic auth user",
			username:   "unknown",
			password:   "testpass",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/status", http.NoBody)
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Authorization", "Basic "+
				base64.StdEncoding.EncodeToString([]byte(tt.username+":"+tt.password)))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

// TestSecurityHeaders verifies that authenticated responses include all required
// security headers.
// VALIDATES: AC-13 (security headers)
// PREVENTS: clickjacking, MIME sniffing, protocol downgrade, caching of sensitive data.
func TestSecurityHeaders(t *testing.T) {
	store := NewSessionStore()
	users := testUsers(t)

	session, err := store.CreateSession("alice")
	require.NoError(t, err)

	handler := AuthMiddleware(store, users, noopRenderer, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "ze-session", Value: session.Token})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "authenticated request must succeed")

	expectedHeaders := map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
		"Cache-Control":             "no-store",
	}

	for header, expected := range expectedHeaders {
		actual := rec.Header().Get(header)
		assert.Equal(t, expected, actual, "header %s must be set correctly", header)
	}
}

// TestLoginHandler verifies that the login endpoint creates sessions for valid
// credentials and rejects invalid ones.
// VALIDATES: AC-3 (session created on login), AC-4 (invalid credentials rejected)
// PREVENTS: unauthenticated session creation, missing Set-Cookie on login.
func TestLoginHandler(t *testing.T) {
	store := NewSessionStore()
	users := testUsers(t)
	handler := LoginHandler(store, users, noopRenderer)

	t.Run("valid credentials set session cookie", func(t *testing.T) {
		form := url.Values{
			"username": {"alice"},
			"password": {"testpass"},
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Login redirects to "/" on success.
		assert.Equal(t, http.StatusSeeOther, rec.Code)

		// Response must include a Set-Cookie header with ze-session.
		cookies := rec.Result().Cookies() //nolint:bodyclose // httptest recorder, no body to close
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "ze-session" {
				sessionCookie = c
				break
			}
		}
		require.NotNil(t, sessionCookie, "response must include ze-session cookie")
		assert.Len(t, sessionCookie.Value, 64, "session token must be 64 hex chars")
		assert.True(t, sessionCookie.HttpOnly, "cookie must be HttpOnly")
		assert.True(t, sessionCookie.Secure, "cookie must be Secure")
		assert.Equal(t, http.SameSiteStrictMode, sessionCookie.SameSite, "cookie must be SameSite=Strict")

		// Token must be valid in the store.
		assert.NotNil(t, store.ValidateToken(sessionCookie.Value),
			"session token from cookie must be valid in store")
	})

	t.Run("invalid credentials return 401", func(t *testing.T) {
		form := url.Values{
			"username": {"alice"},
			"password": {"wrongpass"},
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		// No session cookie should be set on failure.
		cookies := rec.Result().Cookies() //nolint:bodyclose // httptest recorder, no body to close
		for _, c := range cookies {
			assert.NotEqual(t, "ze-session", c.Name,
				"failed login must not set session cookie")
		}
	})

	t.Run("GET method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/login", http.NoBody)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}

// TestInvalidateUser verifies that InvalidateUser removes the session for a user.
// PREVENTS: stale sessions persisting after explicit logout.
func TestInvalidateUser(t *testing.T) {
	store := NewSessionStore()

	session, err := store.CreateSession("alice")
	require.NoError(t, err)

	// Session must be valid before invalidation.
	require.NotNil(t, store.ValidateToken(session.Token))

	store.InvalidateUser("alice")

	// Session must be invalid after explicit invalidation.
	assert.Nil(t, store.ValidateToken(session.Token),
		"session must be invalid after InvalidateUser")

	// Invalidating a non-existent user is a no-op (no panic).
	store.InvalidateUser("nonexistent")
}
