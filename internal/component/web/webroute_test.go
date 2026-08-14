package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// restoreWebRoutes puts the route registry back when the test ends.
// RegisterWebRoute has no removal. A test that registers a route therefore
// leaves it in the registry every later test reads. A test that asks the
// registry what the hub serves then gets a route no hub ever wired.
func restoreWebRoutes(t *testing.T) {
	t.Helper()

	routeMu.RLock()
	saved := make([]WebRoute, len(webRoutes))
	copy(saved, webRoutes)
	routeMu.RUnlock()

	t.Cleanup(func() {
		routeMu.Lock()
		webRoutes = saved
		routeMu.Unlock()
	})
}

func lookupWebRoute(routes []WebRoute, pattern string) (WebRoute, bool) {
	for _, r := range routes {
		if r.Pattern == pattern {
			return r, true
		}
	}
	return WebRoute{}, false
}

// TestPluginWebRouteRegistration is the AC-5 wiring proof: in-tree web features
// register their routes via init() and are discoverable + servable through the
// registry, so the hub iterates instead of hardcoding srv.Handle blocks.
//
// VALIDATES: the L2TP/IS-IS/OSPF/gokrazy routes self-register with the right
// wrap kind; the gokrazy route carries an Enabled gate and a Portal nav entry;
// a real registered route builds a serving handler.
// PREVENTS: regression to the hardcoded per-feature route blocks that used to
// live in cmd/ze/hub/service_web.go.
func TestPluginWebRouteRegistration(t *testing.T) {
	routes := RegisteredWebRoutes()

	// Each in-tree feature self-registered via its register_*.go init().
	for _, pat := range []string{
		"GET /l2tp", "POST /l2tp/{sid}/disconnect",
		"GET /isis", "GET /isis/database",
		"GET /ospf", "GET /ospfv3/database",
		"/gokrazy/",
	} {
		_, ok := lookupWebRoute(routes, pat)
		assert.Truef(t, ok, "route %q must be registered", pat)
	}

	// Reads use WrapAuth; the disconnect mutation uses WrapMutation.
	if r, ok := lookupWebRoute(routes, "GET /l2tp"); ok {
		assert.Equal(t, WrapAuth, r.Wrap)
	}
	if r, ok := lookupWebRoute(routes, "POST /l2tp/{sid}/disconnect"); ok {
		assert.Equal(t, WrapMutation, r.Wrap)
	}

	// gokrazy gates on its env flag (Enabled) and carries a portal nav entry.
	gok, ok := lookupWebRoute(routes, "/gokrazy/")
	require.True(t, ok)
	require.NotNil(t, gok.Enabled, "gokrazy route must gate on its env flag")
	require.NotNil(t, gok.Portal, "gokrazy route must carry a portal nav entry")
	assert.Equal(t, "gokrazy", gok.Portal.Key)

	// A real registered route builds a handler that serves without panicking.
	renderer, err := NewRenderer()
	require.NoError(t, err)
	l2tp, ok := lookupWebRoute(routes, "GET /l2tp")
	require.True(t, ok)
	h := l2tp.Build(RouteDeps{Renderer: renderer})
	require.NotNil(t, h)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/l2tp", http.NoBody))
	assert.GreaterOrEqual(t, rec.Code, 200, "handler must respond with an HTTP status")

	// Mirror the hub loop: register every enabled route onto a fresh ServeMux.
	// http.ServeMux panics on a duplicate pattern, so this proves the hub's
	// iteration in service_web.go wires all feature routes without collision.
	deps := RouteDeps{Renderer: renderer}
	mux := http.NewServeMux()
	for _, route := range routes {
		if route.Enabled != nil && !route.Enabled() {
			continue
		}
		mux.Handle(route.Pattern, route.Build(deps))
	}
}

// TestWebRouteRegistryRoundTrip checks the registry mechanics in isolation: a
// registered route is retrievable and its Build produces a working handler.
func TestWebRouteRegistryRoundTrip(t *testing.T) {
	restoreWebRoutes(t)

	before := len(RegisteredWebRoutes())
	RegisterWebRoute(WebRoute{
		Pattern: "GET /__test_wiring",
		Wrap:    WrapAuth,
		Build: func(RouteDeps) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			})
		},
	})
	routes := RegisteredWebRoutes()
	assert.Equal(t, before+1, len(routes))

	r, ok := lookupWebRoute(routes, "GET /__test_wiring")
	require.True(t, ok)
	rec := httptest.NewRecorder()
	r.Build(RouteDeps{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/__test_wiring", http.NoBody))
	assert.Equal(t, http.StatusTeapot, rec.Code)
}
