// VALIDATES: the /gokrazy/ web mount is self-registered with the enable gate,
// portal entry, and a Build() that proxies to the configured socket end to end.
// PREVENTS: the gokrazy portal silently losing its route registration, enable
// gate, or socket plumbing through the web server.
package web

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
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
	socketPath := filepath.Join(t.TempDir(), "gokrazy.sock")
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
	req := httptest.NewRequest(http.MethodGet, "/gokrazy/status", http.NoBody)
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
