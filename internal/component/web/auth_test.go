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

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/core/audit"
)

// testUsers returns a slice of UserConfig with a known bcrypt hash for "testpass".
func testUsers(t *testing.T) []authz.UserConfig {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	require.NoError(t, err)

	return []authz.UserConfig{
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

type recordingAuthenticator struct {
	request authz.AuthRequest
	result  authz.AuthResult
	err     error
}

func (r *recordingAuthenticator) Authenticate(request authz.AuthRequest) (authz.AuthResult, error) {
	r.request = request
	return r.result, r.err
}

// TestSessionCookieValidation verifies that AuthMiddleware passes requests with
// a valid session cookie and rejects requests with an invalid or missing cookie.
// VALIDATES: AC-2 (missing session returns login page), AC-3 (valid session passes)
// PREVENTS: unauthenticated access to protected routes.
func TestSessionCookieValidation(t *testing.T) {
	store := NewSessionStore()
	users := testUsers(t)

	session, err := store.CreateSession("alice", nil)
	require.NoError(t, err)

	handler := AuthMiddleware(store, &authz.LocalAuthenticator{Users: users}, noopRenderer, okHandler())

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

	session, err := store.CreateSession("alice", nil)
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
	first, err := store.CreateSession("alice", nil)
	require.NoError(t, err)
	firstToken := first.Token

	// Verify first session is valid.
	require.NotNil(t, store.ValidateToken(firstToken), "first session must be valid initially")

	// Create second session for the same user.
	second, err := store.CreateSession("alice", nil)
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
	handler := AuthMiddleware(store, &authz.LocalAuthenticator{Users: users}, noopRenderer, okHandler())

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

// VALIDATES: web auth passes HTTP remote address into the shared authenticator request.
// PREVENTS: AAA backends seeing empty rem_addr for browser/API logins.
func TestAuthMiddlewarePassesRemoteAddrToAuthenticator(t *testing.T) {
	store := NewSessionStore()
	authenticator := &recordingAuthenticator{
		result: authz.AuthResult{Authenticated: true, Source: "test"},
	}

	handler := AuthMiddleware(store, authenticator, noopRenderer, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/status", http.NoBody)
	req.RemoteAddr = "198.51.100.10:4444"
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+
		base64.StdEncoding.EncodeToString([]byte("alice:testpass")))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice", authenticator.request.Username)
	assert.Equal(t, "testpass", authenticator.request.Password)
	assert.Equal(t, "198.51.100.10:4444", authenticator.request.RemoteAddr)
}

// VALIDATES: the web Basic-auth path never sets AuthRequest.Local, so the shared
// authenticator always treats web logins as remote (hash-as-token disabled).
// PREVENTS: a loopback reverse-proxy web request accidentally enabling
// hash-as-token.
func TestWebBasicAuthNeverSetsLocal(t *testing.T) {
	store := NewSessionStore()
	authenticator := &recordingAuthenticator{
		result: authz.AuthResult{Authenticated: true, Source: "test"},
	}
	handler := AuthMiddleware(store, authenticator, noopRenderer, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/status", http.NoBody)
	req.RemoteAddr = "127.0.0.1:5555" // even from loopback, web must stay remote
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+
		base64.StdEncoding.EncodeToString([]byte("alice:testpass")))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, authenticator.request.Local,
		"web auth must never set Local (hash-as-token must stay disabled for web)")
}

// VALIDATES: AC-1 — presenting a user's stored bcrypt hash as the Basic-auth
// password over web is rejected; the real plaintext still authenticates.
// PREVENTS: a leaked config backup being replayed as a web credential.
func TestWebBasicAuthRejectsHashAsToken(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.MinCost)
	require.NoError(t, err)
	users := []authz.UserConfig{{Name: "alice", Hash: string(hash)}}

	store := NewSessionStore()
	handler := AuthMiddleware(store, &authz.LocalAuthenticator{Users: users}, noopRenderer, okHandler())

	basic := func(user, pass string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/status", http.NoBody)
		req.RemoteAddr = "127.0.0.1:5555"
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Basic "+
			base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusUnauthorized, basic("alice", string(hash)),
		"the stored hash must be rejected as a web credential")
	assert.Equal(t, http.StatusOK, basic("alice", "testpass"),
		"the real plaintext password must still authenticate over web")
}

// TestSecurityHeaders verifies that authenticated responses include all required
// security headers.
// VALIDATES: AC-13 (security headers)
// PREVENTS: clickjacking, MIME sniffing, protocol downgrade, caching of sensitive data.
func TestSecurityHeaders(t *testing.T) {
	store := NewSessionStore()
	users := testUsers(t)

	session, err := store.CreateSession("alice", nil)
	require.NoError(t, err)

	handler := AuthMiddleware(store, &authz.LocalAuthenticator{Users: users}, noopRenderer, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "ze-session", Value: session.Token})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "authenticated request must succeed")

	expectedHeaders := map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'; style-src 'self'",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
		"Cache-Control":             "no-store",
	}

	for header, expected := range expectedHeaders {
		actual := rec.Header().Get(header)
		assert.Equal(t, expected, actual, "header %s must be set correctly", header)
	}

	zeVer := rec.Header().Get("X-Ze-Version")
	assert.NotEmpty(t, zeVer, "X-Ze-Version header must be present")
	assert.Contains(t, zeVer, "ze/", "X-Ze-Version must start with ze/ prefix")
}

// TestLoginHandler verifies that the login endpoint creates sessions for valid
// credentials and rejects invalid ones.
// VALIDATES: AC-3 (session created on login), AC-4 (invalid credentials rejected)
// PREVENTS: unauthenticated session creation, missing Set-Cookie on login.
func TestLoginHandler(t *testing.T) {
	store := NewSessionStore()
	users := testUsers(t)
	handler := LoginHandler(store, &authz.LocalAuthenticator{Users: users}, noopRenderer)

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

// VALIDATES: AC-16 -- Web failed login emits an audit record with source IP and attempted user.
// PREVENTS: Browser login failures being visible only in web logs.
func TestLoginHandlerAuthFailureAuditRecord(t *testing.T) {
	store := NewSessionStore()
	users := testUsers(t)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	handler := LoginHandlerWithAudit(store, &authz.LocalAuthenticator{Users: users}, noopRenderer, recorder)
	form := url.Values{"username": {"alice"}, "password": {"wrongpass"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.0.2.10:4444"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "192.0.2.10:4444", entries[0].RemoteAddr)
	assert.Equal(t, audit.Web, entries[0].Surface)
	assert.Equal(t, audit.OutcomeDenied, entries[0].Outcome)
}

// TestInvalidateUser verifies that InvalidateUser removes the session for a user.
// PREVENTS: stale sessions persisting after explicit logout.
func TestInvalidateUser(t *testing.T) {
	store := NewSessionStore()

	session, err := store.CreateSession("alice", nil)
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
