package web

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"codeberg.org/thomas-mangin/ze/internal/component/ssh"
)

// setupTestServer creates a running HTTPS server with auth middleware, login
// handler, and a test content handler. It returns the base URL (https://...),
// an http.Client configured for TLS (InsecureSkipVerify) with a cookie jar,
// and a cleanup function that shuts down the server.
//
// The test user is "testuser" with password "testpass" (bcrypt MinCost).
func setupTestServer(t *testing.T) (baseURL string, client *http.Client, cleanup func()) {
	t.Helper()

	// Generate self-signed cert for the test server.
	certPEM, keyPEM, err := GenerateWebCert()
	require.NoError(t, err, "GenerateWebCert must succeed")

	// Create test user with bcrypt hash at MinCost for speed.
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.MinCost)
	require.NoError(t, err, "bcrypt hash generation must succeed")

	users := []ssh.UserConfig{
		{Name: "testuser", Hash: string(hash)},
	}

	store := NewSessionStore()

	// Create a renderer for the login page.
	renderer, err := NewRenderer()
	require.NoError(t, err, "NewRenderer must succeed")

	loginRenderer := func(w http.ResponseWriter, r *http.Request) {
		if renderErr := renderer.RenderLogin(w, LoginData{}); renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
	}

	// Test content handler that responds based on content negotiation.
	contentHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parsed, parseErr := ParseURL(r)
		if parseErr != nil {
			http.Error(w, parseErr.Error(), http.StatusBadRequest)
			return
		}

		if parsed.Format == formatJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			if encErr := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); encErr != nil {
				http.Error(w, "encode error", http.StatusInternalServerError)
			}

			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if _, writeErr := w.Write([]byte("<html><body>ok</body></html>")); writeErr != nil {
			return // client disconnected, nothing to do
		}
	})

	// Build the server with routes.
	srv, err := NewWebServer(WebConfig{
		ListenAddr: "127.0.0.1:0",
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
	})
	require.NoError(t, err, "NewWebServer must succeed")

	// Register routes on the server's mux.
	loginHandler := LoginHandler(store, users, loginRenderer)
	authMiddleware := AuthMiddleware(store, users, loginRenderer, contentHandler)
	assetHandler := http.StripPrefix("/assets/", renderer.AssetHandler())

	srv.HandleFunc("POST /login", loginHandler)
	srv.Handle("/assets/", assetHandler)
	srv.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/show/", http.StatusFound)

			return
		}

		authMiddleware.ServeHTTP(w, r)
	})

	// Start the server in a goroutine.
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if serveErr := srv.ListenAndServe(ctx); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			// Server error is expected on shutdown; only unexpected errors matter.
			t.Logf("ListenAndServe error: %v", serveErr)
		}
	}()

	// Wait for the server to bind.
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()

	require.NoError(t, srv.WaitReady(readyCtx), "server must be ready within 5s")

	addr := srv.Address()
	require.NotEqual(t, "127.0.0.1:0", addr, "server must bind to a real port")

	base := fmt.Sprintf("https://%s", addr)

	// Create HTTP client with TLS InsecureSkipVerify and cookie jar.
	jar, err := cookiejar.New(nil)
	require.NoError(t, err, "cookie jar creation must succeed")

	httpClient := &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test client connecting to self-signed cert
			},
		},
		// Do not follow redirects automatically for tests that inspect redirect responses.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	cleanupFn := func() {
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Logf("server shutdown error: %v", shutdownErr)
		}
	}

	return base, httpClient, cleanupFn
}

// doGet performs an HTTP GET with a background context.
// Caller MUST close the response body.
func doGet(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

// doLogin performs a POST /login with the given credentials and returns the response.
// The cookie jar in the client retains any Set-Cookie headers.
// Caller MUST close the response body.
func doLogin(t *testing.T, client *http.Client, baseURL, username, password string) *http.Response { //nolint:unparam // username is explicit for test readability
	t.Helper()

	form := url.Values{
		"username": {username},
		"password": {password},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/login", strings.NewReader(form.Encode()))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	require.NoError(t, err, "POST /login must not return a transport error")

	return resp
}

// TestIntegration_ServerStartsTLS connects to the server and verifies that the
// TLS handshake succeeds with a self-signed certificate.
// VALIDATES: AC-1 (HTTPS server starts).
// PREVENTS: TLS misconfiguration preventing server startup.
func TestIntegration_ServerStartsTLS(t *testing.T) {
	baseURL, client, cleanup := setupTestServer(t)
	defer cleanup()

	// Any request exercises the TLS handshake. A successful response (any status)
	// proves TLS negotiation completed.
	resp := doGet(t, client, baseURL+"/")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.NotNil(t, resp.TLS, "response must have TLS connection state")
}

// TestIntegration_UnauthenticatedReturnsLogin verifies that GET /show/ without
// cookies returns 401 with login page HTML and no WWW-Authenticate header.
// VALIDATES: AC-2 (unauthenticated returns login page).
// PREVENTS: browser auth popup instead of login page.
func TestIntegration_UnauthenticatedReturnsLogin(t *testing.T) {
	baseURL, client, cleanup := setupTestServer(t)
	defer cleanup()

	resp := doGet(t, client, baseURL+"/show/")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"unauthenticated GET /show/ must return 401")

	// Must not include WWW-Authenticate (which triggers browser auth popup).
	assert.Empty(t, resp.Header.Get("WWW-Authenticate"),
		"401 response must not include WWW-Authenticate header")

	// Response body must contain HTML (the login page).
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "<", "response body must contain HTML markup")
}

// TestIntegration_LoginFlow verifies the full login flow: POST /login with valid
// credentials gets a Set-Cookie, then GET /show/ with the cookie returns 200.
// VALIDATES: AC-3 (session created on login), AC-11 (cookie-based auth).
// PREVENTS: session not persisted, cookie not sent on subsequent requests.
func TestIntegration_LoginFlow(t *testing.T) {
	baseURL, client, cleanup := setupTestServer(t)
	defer cleanup()

	// Login with valid credentials.
	loginResp := doLogin(t, client, baseURL, "testuser", "testpass")
	defer func() { require.NoError(t, loginResp.Body.Close()) }()

	assert.Equal(t, http.StatusSeeOther, loginResp.StatusCode,
		"successful login must redirect with 303")

	// Verify Set-Cookie header is present.
	var sessionCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == "ze-session" {
			sessionCookie = c

			break
		}
	}
	require.NotNil(t, sessionCookie, "login response must include ze-session cookie")
	assert.Len(t, sessionCookie.Value, 64, "session token must be 64 hex chars") //nolint:mnd // 32 bytes hex-encoded

	// The cookie jar now has the session cookie. GET /show/ must succeed.
	showResp := doGet(t, client, baseURL+"/show/")
	defer func() { require.NoError(t, showResp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, showResp.StatusCode,
		"authenticated GET /show/ must return 200")
}

// TestIntegration_InvalidLogin verifies that POST /login with bad credentials
// returns 401.
// VALIDATES: AC-4 (invalid credentials rejected).
// PREVENTS: login accepting any credentials.
func TestIntegration_InvalidLogin(t *testing.T) {
	baseURL, client, cleanup := setupTestServer(t)
	defer cleanup()

	resp := doLogin(t, client, baseURL, "testuser", "wrongpass")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"login with invalid credentials must return 401")

	// No session cookie should be set.
	for _, c := range resp.Cookies() {
		assert.NotEqual(t, "ze-session", c.Name,
			"failed login must not set ze-session cookie")
	}
}

// TestIntegration_ContentNegotiationJSON verifies that after login, GET /show/?format=json
// returns Content-Type: application/json.
// VALIDATES: AC-5 (content negotiation for JSON).
// PREVENTS: JSON format parameter being ignored.
func TestIntegration_ContentNegotiationJSON(t *testing.T) {
	baseURL, client, cleanup := setupTestServer(t)
	defer cleanup()

	// Login first.
	loginResp := doLogin(t, client, baseURL, "testuser", "testpass")
	defer func() { require.NoError(t, loginResp.Body.Close()) }()

	require.Equal(t, http.StatusSeeOther, loginResp.StatusCode)

	// Request JSON format via query parameter.
	resp := doGet(t, client, baseURL+"/show/?format=json")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json",
		"?format=json must return application/json Content-Type")

	// Verify the body is valid JSON.
	var result map[string]string
	err := json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err, "response body must be valid JSON")
}

// TestIntegration_BasicAuthJSON verifies that GET /show/ with Basic Auth and
// Accept: application/json returns 200 with JSON content, without needing a cookie.
// VALIDATES: AC-12 (JSON API with Basic Auth).
// PREVENTS: API clients being forced to use cookie-based sessions.
func TestIntegration_BasicAuthJSON(t *testing.T) {
	baseURL, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Use a fresh client with no cookies to ensure only Basic Auth is used.
	freshClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test client connecting to self-signed cert
			},
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/show/?format=json", http.NoBody)
	require.NoError(t, err)

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+
		base64.StdEncoding.EncodeToString([]byte("testuser:testpass")))

	resp, err := freshClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"Basic Auth with valid credentials must return 200")
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json",
		"response must have application/json Content-Type")

	// Verify the body is valid JSON.
	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err, "response body must be valid JSON")
}

// TestIntegration_SecurityHeaders verifies that authenticated responses include
// all required security headers.
// VALIDATES: AC-13 (security headers).
// PREVENTS: clickjacking, MIME sniffing, protocol downgrade, caching of sensitive data.
func TestIntegration_SecurityHeaders(t *testing.T) {
	baseURL, client, cleanup := setupTestServer(t)
	defer cleanup()

	// Login first.
	loginResp := doLogin(t, client, baseURL, "testuser", "testpass")
	defer func() { require.NoError(t, loginResp.Body.Close()) }()

	require.Equal(t, http.StatusSeeOther, loginResp.StatusCode)

	// GET /show/ as an authenticated user.
	resp := doGet(t, client, baseURL+"/show/")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"authenticated request must succeed before checking headers")

	expectedHeaders := map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
		"Cache-Control":             "no-store",
	}

	for header, expected := range expectedHeaders {
		actual := resp.Header.Get(header)
		assert.Equal(t, expected, actual, "header %s must be set correctly", header)
	}
}

// TestIntegration_PathTraversal verifies that path traversal attempts in
// authenticated requests are rejected with 400.
// VALIDATES: AC-16 (path traversal rejected).
// PREVENTS: directory traversal via URL manipulation.
func TestIntegration_PathTraversal(t *testing.T) {
	baseURL, client, cleanup := setupTestServer(t)
	defer cleanup()

	// Login first.
	loginResp := doLogin(t, client, baseURL, "testuser", "testpass")
	defer func() { require.NoError(t, loginResp.Body.Close()) }()

	require.Equal(t, http.StatusSeeOther, loginResp.StatusCode)

	// Attempt path traversal. Go's http client normalizes ".." in URLs,
	// so we use percent-encoded ".." to preserve it through the client.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/show/..%2Fetc%2Fpasswd", http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"path traversal must be rejected with 400")
}

// TestIntegration_SessionInvalidation verifies that logging in again invalidates
// the previous session: the old cookie returns 401.
// VALIDATES: AC-10 (new login invalidates previous session).
// PREVENTS: stale sessions remaining valid after re-login.
func TestIntegration_SessionInvalidation(t *testing.T) {
	baseURL, client, cleanup := setupTestServer(t)
	defer cleanup()

	// First login.
	firstResp := doLogin(t, client, baseURL, "testuser", "testpass")
	defer func() { require.NoError(t, firstResp.Body.Close()) }()

	require.Equal(t, http.StatusSeeOther, firstResp.StatusCode)

	// Extract the first session cookie.
	var firstToken string
	for _, c := range firstResp.Cookies() {
		if c.Name == "ze-session" {
			firstToken = c.Value

			break
		}
	}
	require.NotEmpty(t, firstToken, "first login must produce a session cookie")

	// Second login (same user) -- this should invalidate the first session.
	// Use a fresh client so the second login does not send the first cookie.
	jar2, err := cookiejar.New(nil)
	require.NoError(t, err)

	client2 := &http.Client{
		Jar: jar2,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test client
			},
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	secondResp := doLogin(t, client2, baseURL, "testuser", "testpass")
	defer func() { require.NoError(t, secondResp.Body.Close()) }()

	require.Equal(t, http.StatusSeeOther, secondResp.StatusCode)

	// Now try the first session cookie on a fresh client (no jar interference).
	freshClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test client
			},
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/show/", http.NoBody)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "ze-session", Value: firstToken})

	oldResp, err := freshClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, oldResp.Body.Close()) }()

	assert.Equal(t, http.StatusUnauthorized, oldResp.StatusCode,
		"old session cookie must return 401 after re-login")

	// Verify the second session still works.
	showResp := doGet(t, client2, baseURL+"/show/")
	defer func() { require.NoError(t, showResp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, showResp.StatusCode,
		"new session cookie must still be valid")
}
