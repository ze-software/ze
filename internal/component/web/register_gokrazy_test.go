// VALIDATES: the /gokrazy/ web mount is self-registered with the enable gate,
// portal entry, and a Build() that proxies to the configured socket end to end;
// the mount judges each request by the session's Ze authorization, so a
// read-only account reaches gokrazy's status pages and never its update or
// uploaded-program handlers.
// PREVENTS: the gokrazy portal silently losing its route registration, enable
// gate, or socket plumbing through the web server, and any web account
// reaching gokrazy's update handlers with the device's update credential.
package web

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
)

func findRoute(t *testing.T, pattern string) WebRoute {
	t.Helper()
	for _, r := range RegisteredWebRoutes() {
		if r.Pattern == pattern {
			return r
		}
	}
	t.Fatalf("web route %q not registered", pattern)
	return WebRoute{}
}

// TestGokrazyRouteRegistered proves the portal route is present with the
// expected wrap kind and portal metadata.
func TestGokrazyRouteRegistered(t *testing.T) {
	r := findRoute(t, "/gokrazy/")
	if r.Wrap != WrapAuth {
		t.Errorf("Wrap = %v, want WrapAuth", r.Wrap)
	}
	if r.Enabled == nil {
		t.Fatal("Enabled gate is nil, want env-flag gate")
	}
	if r.Portal == nil || r.Portal.Path != "/gokrazy/" || r.Portal.Key != "gokrazy" {
		t.Errorf("Portal = %+v, want key=gokrazy path=/gokrazy/", r.Portal)
	}
}

// TestGokrazyRouteEnableGate proves the route is wired only when
// ze.gokrazy.enabled is set.
func TestGokrazyRouteEnableGate(t *testing.T) {
	r := findRoute(t, "/gokrazy/")
	t.Cleanup(func() { _ = env.Set("ze.gokrazy.enabled", "") })

	if err := env.Set("ze.gokrazy.enabled", ""); err != nil {
		t.Fatal(err)
	}
	if r.Enabled() {
		t.Error("route enabled with flag unset")
	}
	if err := env.SetBool("ze.gokrazy.enabled", true); err != nil {
		t.Fatal(err)
	}
	if !r.Enabled() {
		t.Error("route not enabled with flag set")
	}
}

// TestGokrazyRouteBuildProxies proves Build() returns a handler that proxies
// through the configured socket, so the whole web mount is wired end to end.
func TestGokrazyRouteBuildProxies(t *testing.T) {
	// macOS caps unix socket paths at ~104 bytes; t.TempDir() under $TMPDIR
	// (/var/folders/...) already exceeds that, so the bind fails with EINVAL on
	// Darwin. Use a short /tmp dir (mirrors shortTempDir in
	// internal/component/vpp/vpp_test.go) so the socket binds on Darwin as on Linux.
	dir, err := os.MkdirTemp("/tmp", "gk")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "s.sock")
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	var mu sync.Mutex
	gotPath := ""
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		mu.Unlock()
		if _, werr := io.WriteString(w, "ok"); werr != nil {
			return
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	t.Cleanup(func() { _ = env.Set("ze.gokrazy.socket", "") })
	if err := env.Set("ze.gokrazy.socket", socketPath); err != nil {
		t.Fatal(err)
	}

	h := findRoute(t, "/gokrazy/").Build(RouteDeps{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/gokrazy/status", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 through the web mount", rec.Code)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/status" {
		t.Fatalf("upstream path = %q, want /status (prefix stripped by mount)", gotPath)
	}
}

// gokrazyUpstream starts a fake gokrazy on a short Unix socket path, points
// ze.gokrazy.socket at it, and answers the log of the request paths it
// received.
func gokrazyUpstream(t *testing.T) func() []string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "gk")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "s.sock")
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	var mu sync.Mutex
	var received []string
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = append(received, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if _, werr := io.WriteString(w, "ok"); werr != nil {
			return
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	t.Cleanup(func() { _ = env.Set("ze.gokrazy.socket", "") })
	if err := env.Set("ze.gokrazy.socket", socketPath); err != nil {
		t.Fatal(err)
	}
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), received...)
	}
}

// serveAs sends one request through h as the authenticated user alice. A
// non-nil session is the authorizer the auth middleware bound to the login.
func serveAs(t *testing.T, h http.Handler, session plugin.Authorizer, method, target string) int {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
	ctx := withUsername(req.Context(), "alice")
	if session != nil {
		ctx = plugin.WithCallerAuthorizer(ctx, session)
	}
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// TestGokrazyRouteReadOnlyAccountReachesNoDeviceChange proves, through the
// registered route and the real authorization store, that an account holding
// the read-only profile gets 403 on gokrazy's update, reboot and divert
// handlers with no request reaching gokrazy, while its status page still
// proxies.
func TestGokrazyRouteReadOnlyAccountReachesNoDeviceChange(t *testing.T) {
	received := gokrazyUpstream(t)
	h := findRoute(t, "/gokrazy/").Build(RouteDeps{Authorizer: readOnlyWebAuthorizer()})

	for _, tc := range []struct{ method, target string }{
		{http.MethodPut, "/gokrazy/update/root"},
		{http.MethodPost, "/gokrazy/update/switch"},
		{http.MethodPost, "/gokrazy/reboot"},
		{http.MethodPost, "/gokrazy/divert"},
		{http.MethodPut, "/gokrazy/uploadtemp/ze"},
	} {
		if code := serveAs(t, h, nil, tc.method, tc.target); code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", tc.method, tc.target, code)
		}
	}
	if got := received(); len(got) != 0 {
		t.Fatalf("gokrazy received %v from a read-only account, want nothing", got)
	}

	if code := serveAs(t, h, nil, http.MethodGet, "/gokrazy/status"); code != http.StatusOK {
		t.Fatalf("GET /gokrazy/status: status = %d, want 200", code)
	}
	if got := received(); len(got) != 1 || got[0] != "GET /status" {
		t.Fatalf("gokrazy received %v, want [GET /status]", got)
	}
}

// TestGokrazyRouteAdminReachesUpdate proves an account whose profile grants
// the edit section reaches gokrazy's update handler through the route, so the
// gate refuses by authorization and not for every account.
func TestGokrazyRouteAdminReachesUpdate(t *testing.T) {
	received := gokrazyUpstream(t)
	h := findRoute(t, "/gokrazy/").Build(RouteDeps{Authorizer: adminWebAuthorizer()})

	if code := serveAs(t, h, nil, http.MethodPut, "/gokrazy/update/root"); code != http.StatusOK {
		t.Fatalf("PUT /gokrazy/update/root as admin: status = %d, want 200", code)
	}
	if code := serveAs(t, h, nil, http.MethodPost, "/gokrazy/divert"); code != http.StatusForbidden {
		t.Fatalf("POST /gokrazy/divert as admin: status = %d, want 403", code)
	}
	if got := received(); len(got) != 1 || got[0] != "PUT /update/root" {
		t.Fatalf("gokrazy received %v, want [PUT /update/root]", got)
	}
}

// TestGokrazyRouteSessionAuthorizerWins proves the session's own authorizer,
// which the auth middleware binds to the login's profiles, decides over the
// hub's fallback: a read-only session is refused even when the fallback
// would allow.
func TestGokrazyRouteSessionAuthorizerWins(t *testing.T) {
	received := gokrazyUpstream(t)
	h := findRoute(t, "/gokrazy/").Build(RouteDeps{Authorizer: adminWebAuthorizer()})

	if code := serveAs(t, h, readOnlyWebAuthorizer(), http.MethodPut, "/gokrazy/update/root"); code != http.StatusForbidden {
		t.Fatalf("PUT /gokrazy/update/root with a read-only session: status = %d, want 403", code)
	}
	if got := received(); len(got) != 0 {
		t.Fatalf("gokrazy received %v, want nothing", got)
	}
}
