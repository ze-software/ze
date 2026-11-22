//go:build ze_web

// Design: docs/architecture/web-interface.md -- web session invalidation on local credential rotation
// Related: aaa_lifecycle.go -- publishAcceptedLocalIdentity, stampLocalIdentityGeneration, liveAcceptedLocalUsers

package hub

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

// TestWebSessionCookieInvalidatedByPasswordRotationReload drives the production
// hub identity lifecycle (publishAcceptedLocalIdentity + liveAcceptedLocalUsers)
// together with web.SessionStore, web.LoginHandlerWithAudit, and
// web.AuthMiddlewareWithAudit.
//
// VALIDATES: S5 end-to-end -- an unrelated reload that keeps local credentials
// identical preserves open ze-session cookies, while rotating a local user's
// password advances its UserGeneration and immediately invalidates existing
// cookies.
// PREVENTS: hub generation stamping drifting from SessionStore.validateToken.
func TestWebSessionCookieInvalidatedByPasswordRotationReload(t *testing.T) {
	resetAAABundleForTest(t)

	aliceV1 := bcryptUser(t, "alice", "initial-pass", "admin")
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity([]authz.UserConfig{aliceV1}, nil, nil, ""))

	authenticator := &authz.LocalAuthenticator{UsersFunc: liveAcceptedLocalUsers}
	store := zeweb.NewSessionStore(liveAcceptedLocalUsers)
	noopRenderer := func(http.ResponseWriter, *http.Request) {}
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	login := zeweb.LoginHandlerWithAudit(store, authenticator, noopRenderer, nil)
	protected := zeweb.AuthMiddlewareWithAudit(store, authenticator, noopRenderer, okHandler, nil)

	loginCookie := func(username, password string) string {
		t.Helper()
		form := url.Values{"username": {username}, "password": {password}}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		login(rec, req)
		resp := rec.Result()
		defer resp.Body.Close() //nolint:errcheck // cookies only
		for _, c := range resp.Cookies() {
			if c.Name == "ze-session" {
				return c.Value
			}
		}
		t.Fatalf("login for %q issued no ze-session cookie (status %d)", username, resp.StatusCode)
		return ""
	}

	requestStatus := func(token string) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		req.AddCookie(&http.Cookie{Name: "ze-session", Value: token})
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)
		return rec.Code
	}

	cookieV1 := loginCookie("alice", "initial-pass")
	require.Equal(t, http.StatusOK, requestStatus(cookieV1))

	// Unrelated config reload: identical local credentials keep UserGeneration.
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity([]authz.UserConfig{aliceV1}, nil, nil, ""))
	assert.Equal(t, http.StatusOK, requestStatus(cookieV1),
		"unrelated reload must keep the existing ze-session cookie valid")

	// Password rotation: hash changes, so publishAcceptedLocalIdentity advances
	// alice's UserGeneration and invalidates the pre-rotation cookie at once.
	aliceV2 := bcryptUser(t, "alice", "rotated-pass", "admin")
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity([]authz.UserConfig{aliceV2}, nil, nil, ""))
	assert.Equal(t, http.StatusUnauthorized, requestStatus(cookieV1),
		"pre-rotation ze-session cookie must be rejected immediately after password rotation")

	// Fresh login with the rotated password receives a valid cookie at the new generation.
	cookieV2 := loginCookie("alice", "rotated-pass")
	assert.Equal(t, http.StatusOK, requestStatus(cookieV2),
		"fresh cookie issued after password rotation must succeed")
}

// TestWebSessionGenerationIsPerUser drives the same production lifecycle with
// two users.
//
// VALIDATES: a locally anchored session is revoked when ITS user's password,
// profiles or presence change, and survives a change to any other user: another
// user's new password, a user added, another user's new SSH key.
// PREVENTS: one counter for the whole local credential set logging out every
// web session, the committing admin's own included, whenever any local user
// changed.
func TestWebSessionGenerationIsPerUser(t *testing.T) {
	resetAAABundleForTest(t)

	alice := bcryptUser(t, "alice", "alice-pass", "admin")
	bob := bcryptUser(t, "bob", "bob-pass", "admin")
	publish := func(users ...authz.UserConfig) {
		t.Helper()
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(users, nil, nil, ""))
	}
	publish(alice, bob)

	authenticator := &authz.LocalAuthenticator{UsersFunc: liveAcceptedLocalUsers}
	store := zeweb.NewSessionStore(liveAcceptedLocalUsers)
	noopRenderer := func(http.ResponseWriter, *http.Request) {}
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	login := zeweb.LoginHandlerWithAudit(store, authenticator, noopRenderer, nil)
	protected := zeweb.AuthMiddlewareWithAudit(store, authenticator, noopRenderer, okHandler, nil)

	loginCookie := func(username, password string) string {
		t.Helper()
		form := url.Values{"username": {username}, "password": {password}}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		login(rec, req)
		resp := rec.Result()
		defer resp.Body.Close() //nolint:errcheck // cookies only
		for _, c := range resp.Cookies() {
			if c.Name == "ze-session" {
				return c.Value
			}
		}
		t.Fatalf("login for %q issued no ze-session cookie (status %d)", username, resp.StatusCode)
		return ""
	}
	requestStatus := func(token string) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		req.AddCookie(&http.Cookie{Name: "ze-session", Value: token})
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)
		return rec.Code
	}

	cookie := loginCookie("alice", "alice-pass")
	require.Equal(t, http.StatusOK, requestStatus(cookie))

	// Other users change: alice's session survives each of them.
	bobRotated := bcryptUser(t, "bob", "bob-rotated", "admin")
	publish(alice, bobRotated)
	assert.Equal(t, http.StatusOK, requestStatus(cookie),
		"changing another user's password must leave the session valid")

	carol := bcryptUser(t, "carol", "carol-pass", "admin")
	publish(alice, bobRotated, carol)
	assert.Equal(t, http.StatusOK, requestStatus(cookie),
		"adding a user must leave the session valid")

	bobKeyed := bobRotated
	bobKeyed.PublicKeys = []aaa.SSHPublicKey{{Name: "laptop", Type: "ssh-ed25519", Key: "AAAA"}}
	publish(alice, bobKeyed, carol)
	assert.Equal(t, http.StatusOK, requestStatus(cookie),
		"adding another user's SSH key must leave the session valid")

	// The session user's own profiles change: revoked.
	aliceDemoted := alice
	aliceDemoted.Profiles = []string{"read-only"}
	publish(aliceDemoted, bobKeyed, carol)
	assert.Equal(t, http.StatusUnauthorized, requestStatus(cookie),
		"changing the session user's own profiles must revoke the session")

	// The session user's own password changes: revoked.
	cookie = loginCookie("alice", "alice-pass")
	require.Equal(t, http.StatusOK, requestStatus(cookie))
	aliceRotated := bcryptUser(t, "alice", "alice-rotated", "read-only")
	publish(aliceRotated, bobKeyed, carol)
	assert.Equal(t, http.StatusUnauthorized, requestStatus(cookie),
		"changing the session user's own password must revoke the session")

	// The session user is removed: revoked.
	cookie = loginCookie("alice", "alice-rotated")
	require.Equal(t, http.StatusOK, requestStatus(cookie))
	publish(bobKeyed, carol)
	assert.Equal(t, http.StatusUnauthorized, requestStatus(cookie),
		"removing the session user must revoke the session")
}
