package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/plugin"
)

// fakeAuthorizer implements aaa.Authorizer for RBAC tests: edit access is
// governed by allowEdit; read-only (isReadOnly) requests are always allowed.
type fakeAuthorizer struct{ allowEdit bool }

func (f fakeAuthorizer) Authorize(_, _, _ string, isReadOnly bool) bool {
	if isReadOnly {
		return true
	}
	return f.allowEdit
}

func testAdminProfile() authz.Profile {
	return authz.Profile{
		Name: "admin",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Allow},
	}
}

func testReadOnlyProfile() authz.Profile {
	return authz.Profile{
		Name: "read-only",
		Run: authz.Section{Default: authz.Allow, Entries: []authz.Entry{
			{Number: 10, Action: authz.Deny, Match: "restart"},
			{Number: 20, Action: authz.Deny, Match: "kill"},
			{Number: 30, Action: authz.Deny, Match: "clear"},
			{Number: 40, Action: authz.Deny, Match: "debug"},
		}},
		Edit: authz.Section{Default: authz.Deny},
	}
}

func TestSessionStoresProfiles(t *testing.T) {
	// VALIDATES: AC-2 -- webSession carries the authenticated user's profiles,
	// preserved across validateToken.
	store := NewSessionStore(nil)
	sessionAuthorizer := fakeAuthorizer{allowEdit: true}
	session, err := store.createSession("alice", authz.AuthResult{
		Profiles: []string{"read-only"}, Authorizer: sessionAuthorizer,
	})
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if len(session.Profiles) != 1 || session.Profiles[0] != "read-only" {
		t.Fatalf("session profiles = %v, want [read-only]", session.Profiles)
	}
	got := store.validateToken(session.Token)
	if got == nil || len(got.Profiles) != 1 || got.Profiles[0] != "read-only" {
		t.Fatalf("validated session lost profiles: %+v", got)
	}
	if got.Authorizer == nil {
		t.Fatal("validated session lost its bound authorizer")
	}
}

func TestRouteGateDeniesReadOnly(t *testing.T) {
	// VALIDATES: AC-1 -- a read-only user gets 403 on an edit-gated route.
	gate := RequireEditAuthz(fakeAuthorizer{allowEdit: false}, okHandler())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/config/edit/", http.NoBody)
	req = req.WithContext(withUsername(req.Context(), "bob"))
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("read-only user: status %d, want 403", rec.Code)
	}
}

func TestRouteGateAllowsAdmin(t *testing.T) {
	// VALIDATES: AC-1/AC-2 -- an edit-authorized user reaches the gated route.
	gate := RequireEditAuthz(fakeAuthorizer{allowEdit: true}, okHandler())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/config/edit/", http.NoBody)
	req = req.WithContext(withUsername(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin user: status %d, want 200", rec.Code)
	}
}

// VALIDATES: the login-bound policy takes precedence over mutable live fallback.
// PREVENTS: a later same-username login granting an established session edits.
func TestRouteGateUsesSessionBoundAuthorizer(t *testing.T) {
	gate := RequireEditAuthz(fakeAuthorizer{allowEdit: true}, okHandler())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/config/edit/", http.NoBody)
	ctx := withUsername(req.Context(), "same-user")
	ctx = plugin.WithCallerAuthorizer(ctx, fakeAuthorizer{allowEdit: false})
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req.WithContext(ctx))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("session-bound read-only user: status %d, want 403", rec.Code)
	}
}

func TestRouteGateOpenWhenUnassigned(t *testing.T) {
	// VALIDATES: R-1 -- a nil authorizer (no authz assignments configured) fails
	// open, so single-admin deployments keep working.
	gate := RequireEditAuthz(nil, okHandler())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/config/edit/", http.NoBody)
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unassigned (nil authorizer): status %d, want 200", rec.Code)
	}
}

func TestProfilesInRequestContext(t *testing.T) {
	// VALIDATES: AC-2 -- the authentication middleware carries session profiles
	// into the request context for route gates and nav rendering.
	store := NewSessionStore(nil)
	sessionAuthorizer := fakeAuthorizer{allowEdit: true}
	session, err := store.createSession("carol", authz.AuthResult{
		Profiles: []string{"admin"}, Authorizer: sessionAuthorizer,
	})
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	var gotProfiles []string
	var gotAuthorizer plugin.Authorizer
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProfiles = getProfilesFromRequest(r)
		gotAuthorizer = plugin.CallerAuthorizer(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := authMiddleware(store, &authz.LocalAuthenticator{}, noopRenderer, next)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "ze-session", Value: session.Token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if len(gotProfiles) != 1 || gotProfiles[0] != "admin" {
		t.Fatalf("context profiles = %v, want [admin]", gotProfiles)
	}
	if gotAuthorizer == nil || !gotAuthorizer.Authorize("carol", "", "set system host-name router", false) {
		t.Fatal("request context lost the session-bound authorizer")
	}
}

func TestRouteGateRealAuthzChain(t *testing.T) {
	// VALIDATES: AC-1 end-to-end -- the gate decision flows through the REAL
	// chain: authz.Store section routing (isReadOnly=false -> Edit section),
	// the test profiles' Edit defaults (read-only: Deny; admin: Allow), and
	// the StoreAuthorizer adapter. Guards the rbac.go premise that the
	// representative edit command is denied by the read-only profile.
	// PREVENTS: a stub-only test suite hiding a regression in any real link
	// (section routing, profile defaults, adapter mapping).
	store := authz.NewStore()
	store.AddProfile(testReadOnlyProfile())
	store.AddProfile(testAdminProfile())
	store.AssignProfiles("bob", []string{"read-only"})
	store.AssignProfiles("root", []string{"admin"})
	authorizer := authz.StoreAuthorizer{Store: store}

	gate := RequireEditAuthz(authorizer, okHandler())

	asUser := func(user string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/config/edit/", http.NoBody)
		req = req.WithContext(withUsername(req.Context(), user))
		rec := httptest.NewRecorder()
		gate.ServeHTTP(rec, req)
		return rec
	}

	if rec := asUser("bob"); rec.Code != http.StatusForbidden {
		t.Fatalf("read-only profile via real store: status %d, want 403", rec.Code)
	}
	if rec := asUser("root"); rec.Code != http.StatusOK {
		t.Fatalf("admin profile via real store: status %d, want 200", rec.Code)
	}

	reqBob := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	reqBob = reqBob.WithContext(withUsername(reqBob.Context(), "bob"))
	if canEdit(reqBob, authorizer) {
		t.Fatal("read-only profile via real store: CanEdit = true, want false")
	}
}

func TestCanEditReflectsAuthorizer(t *testing.T) {
	// VALIDATES: AC-1 -- CanEdit (used for nav hiding) mirrors the gate decision.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	req = req.WithContext(withUsername(req.Context(), "bob"))
	if canEdit(req, fakeAuthorizer{allowEdit: false}) {
		t.Fatal("read-only user: CanEdit = true, want false")
	}
	if !canEdit(req, fakeAuthorizer{allowEdit: true}) {
		t.Fatal("admin user: CanEdit = false, want true")
	}
	if !canEdit(req, nil) {
		t.Fatal("nil authorizer: CanEdit = false, want true (fail open)")
	}
}
