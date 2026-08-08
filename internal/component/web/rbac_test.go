package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
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

func TestSessionStoresProfiles(t *testing.T) {
	// VALIDATES: AC-2 -- WebSession carries the authenticated user's profiles,
	// preserved across ValidateToken.
	store := NewSessionStore(nil)
	session, err := store.CreateSession("alice", authz.AuthResult{Profiles: []string{"read-only"}})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if len(session.Profiles) != 1 || session.Profiles[0] != "read-only" {
		t.Fatalf("session profiles = %v, want [read-only]", session.Profiles)
	}
	got := store.ValidateToken(session.Token)
	if got == nil || len(got.Profiles) != 1 || got.Profiles[0] != "read-only" {
		t.Fatalf("validated session lost profiles: %+v", got)
	}
}

func TestRouteGateDeniesReadOnly(t *testing.T) {
	// VALIDATES: AC-1 -- a read-only user gets 403 on an edit-gated route.
	gate := RequireEditAuthz(fakeAuthorizer{allowEdit: false}, okHandler())
	req := httptest.NewRequest(http.MethodGet, "/config/edit/", http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/config/edit/", http.NoBody)
	req = req.WithContext(withUsername(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin user: status %d, want 200", rec.Code)
	}
}

func TestRouteGateOpenWhenUnassigned(t *testing.T) {
	// VALIDATES: R-1 -- a nil authorizer (no authz assignments configured) fails
	// open, so single-admin deployments keep working.
	gate := RequireEditAuthz(nil, okHandler())
	req := httptest.NewRequest(http.MethodGet, "/config/edit/", http.NoBody)
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unassigned (nil authorizer): status %d, want 200", rec.Code)
	}
}

func TestProfilesInRequestContext(t *testing.T) {
	// VALIDATES: AC-2 -- AuthMiddleware carries session profiles into the
	// request context for route gates and nav rendering.
	store := NewSessionStore(nil)
	session, err := store.CreateSession("carol", authz.AuthResult{Profiles: []string{"admin"}})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var gotProfiles []string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProfiles = GetProfilesFromRequest(r)
		w.WriteHeader(http.StatusOK)
	})
	handler := AuthMiddleware(store, &authz.LocalAuthenticator{}, noopRenderer, next)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "ze-session", Value: session.Token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if len(gotProfiles) != 1 || gotProfiles[0] != "admin" {
		t.Fatalf("context profiles = %v, want [admin]", gotProfiles)
	}
}

func TestRouteGateRealAuthzChain(t *testing.T) {
	// VALIDATES: AC-1 end-to-end -- the gate decision flows through the REAL
	// chain: authz.Store section routing (isReadOnly=false -> Edit section),
	// the builtin profiles' Edit defaults (read-only: Deny; admin: Allow), and
	// the StoreAuthorizer adapter. Guards the rbac.go premise that the
	// representative edit command is denied by the builtin read-only profile.
	// PREVENTS: a stub-only test suite hiding a regression in any real link
	// (section routing, builtin profile defaults, adapter mapping).
	store := authz.NewStore()
	store.AddProfile(authz.BuiltinReadOnlyProfile())
	store.AddProfile(authz.BuiltinAdminProfile())
	store.AssignProfiles("bob", []string{"read-only"})
	store.AssignProfiles("root", []string{"admin"})
	authorizer := authz.StoreAuthorizer{Store: store}

	gate := RequireEditAuthz(authorizer, okHandler())

	asUser := func(user string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/config/edit/", http.NoBody)
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

	reqBob := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	reqBob = reqBob.WithContext(withUsername(reqBob.Context(), "bob"))
	if CanEdit(reqBob, authorizer) {
		t.Fatal("read-only profile via real store: CanEdit = true, want false")
	}
}

func TestCanEditReflectsAuthorizer(t *testing.T) {
	// VALIDATES: AC-1 -- CanEdit (used for nav hiding) mirrors the gate decision.
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req = req.WithContext(withUsername(req.Context(), "bob"))
	if CanEdit(req, fakeAuthorizer{allowEdit: false}) {
		t.Fatal("read-only user: CanEdit = true, want false")
	}
	if !CanEdit(req, fakeAuthorizer{allowEdit: true}) {
		t.Fatal("admin user: CanEdit = false, want true")
	}
	if !CanEdit(req, nil) {
		t.Fatal("nil authorizer: CanEdit = false, want true (fail open)")
	}
}
