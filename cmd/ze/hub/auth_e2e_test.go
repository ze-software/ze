//go:build ze_web

package hub

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/authz"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/audit"
)

func bcryptHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(h)
}

// e2eAuthUsers mirrors what startWebServer / the API setup do: the always-on
// zefs power user merged with config-file users.
func e2eAuthUsers(t *testing.T) (users []authz.UserConfig, powerPass, configPass string) {
	powerPass, configPass = "power-secret", "operator-secret"
	power := []authz.UserConfig{{Name: "admin", Hash: bcryptHash(t, powerPass)}}
	cfg := []authz.UserConfig{{Name: "operator", Hash: bcryptHash(t, configPass)}}
	return mergeAuthUsers(power, cfg), powerPass, configPass
}

// TestWebLoginAdmitsPowerAndConfigUsers drives the real web login HTTP handler
// (zeweb.LoginHandlerWithAudit) over httptest with the merged user set, proving
// that both the zefs power user and a config-file user can log in, and that a
// wrong password / unknown user is rejected (fail closed).
func TestWebLoginAdmitsPowerAndConfigUsers(t *testing.T) {
	users, powerPass, configPass := e2eAuthUsers(t)

	// The store reads the same user set the authenticator answers from, which is
	// how startWebServer wires it (one live producer, two readers).
	store := zeweb.NewSessionStore(func() ([]authz.UserConfig, error) { return users, nil })
	recorder, err := audit.NewMemory(100)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	noopRenderer := func(http.ResponseWriter, *http.Request) {}
	handler := zeweb.LoginHandlerWithAudit(store, &authz.LocalAuthenticator{Users: users}, noopRenderer, recorder)

	// login posts credentials to the real handler and returns the HTTP status
	// plus the issued ze-session token (empty when no session was created). The
	// response body is closed here so it never escapes the helper.
	login := func(user, pass string) (status int, sessionToken string) {
		form := url.Values{"username": {user}, "password": {pass}}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler(rec, req)
		resp := rec.Result()
		defer resp.Body.Close() //nolint:errcheck // status and cookies only
		for _, c := range resp.Cookies() {
			if c.Name == "ze-session" {
				sessionToken = c.Value
			}
		}
		return resp.StatusCode, sessionToken
	}

	t.Run("power user", func(t *testing.T) {
		if _, tok := login("admin", powerPass); tok == "" {
			t.Error("power user could not log into the web UI")
		}
	})
	t.Run("config user", func(t *testing.T) {
		if _, tok := login("operator", configPass); tok == "" {
			t.Error("config-file user could not log into the web UI")
		}
	})
	t.Run("wrong password rejected", func(t *testing.T) {
		status, tok := login("operator", "nope")
		if status != http.StatusUnauthorized {
			t.Errorf("wrong password: status %d, want 401", status)
		}
		if tok != "" {
			t.Error("wrong password produced a session")
		}
	})
	t.Run("unknown user rejected", func(t *testing.T) {
		if status, _ := login("ghost", "x"); status != http.StatusUnauthorized {
			t.Errorf("unknown user: status %d, want 401", status)
		}
	})
}

// TestAPILoginAdmitsPowerAndConfigUsers drives the real API per-user
// authenticator (buildUserAuthenticator) with the merged user set, proving that
// both the power user and a config-file user authenticate via the Bearer
// "<user>:<password>" credential, and that bad credentials are rejected.
func TestAPILoginAdmitsPowerAndConfigUsers(t *testing.T) {
	users, powerPass, configPass := e2eAuthUsers(t)

	// The live source the hub threads in: the same merged set, read per request.
	validate := buildUserAuthenticator(users, func() ([]authz.UserConfig, error) { return users, nil })
	if validate == nil {
		t.Fatal("buildUserAuthenticator returned nil for a non-empty user set")
	}

	cases := []struct {
		name     string
		header   string
		wantUser string
		wantOK   bool
	}{
		{"power user", "Bearer admin:" + powerPass, "admin", true},
		{"config user", "Bearer operator:" + configPass, "operator", true},
		{"wrong password", "Bearer operator:nope", "", false},
		{"unknown user", "Bearer ghost:x", "", false},
		{"missing bearer prefix", "admin:" + powerPass, "", false},
		{"empty", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user, ok := validate(c.header)
			if ok != c.wantOK || user != c.wantUser {
				t.Errorf("validate(%q) = (%q, %v), want (%q, %v)", c.header, user, ok, c.wantUser, c.wantOK)
			}
		})
	}
}

// TestLiveAAABundleAuthorizerHonorsConfiguredProfiles proves the live bundle
// authorizer path uses the extracted authz store, not just authentication.
func TestLiveAAABundleAuthorizerHonorsConfiguredProfiles(t *testing.T) {
	resetAAABundleForTest(t)

	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "read-only",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AssignProfiles("operator", []string{"read-only"})
	users := []authz.UserConfig{{Name: "operator", Hash: bcryptHash(t, "operator-secret")}}

	// nil liveUsers: this test asserts authorization, not credential freshness,
	// so the backend keeps the snapshot behavior.
	swapLocalAuthzStore(store)
	bundle, err := buildAAABundle(nil, users, nil, nil)
	if err != nil {
		t.Fatalf("buildAAABundle: %v", err)
	}
	swapAAABundle(bundle, nil)

	authorizer := liveAAABundleAuthorizer{}
	if authorizer.Authorize("operator", "", api.ConfigAuthSet, false) {
		t.Fatal("read-only operator unexpectedly allowed config set")
	}
	if authorizer.Authorize("operator", "", api.ConfigAuthCommit, false) {
		t.Fatal("read-only operator unexpectedly allowed config commit")
	}
}
