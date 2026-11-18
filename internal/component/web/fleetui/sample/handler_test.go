package sample

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web"
)

// fleetPrefix is the method and path every fleet route pattern starts with.
const fleetPrefix = "GET /fleet/"

// mountFleet mounts the registered fleet routes the way the hub does when the
// web server starts without login: it skips a route whose Enabled answers
// false, builds the others, and wraps each in the insecure middleware.
func mountFleet(t *testing.T) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	mounted := 0

	for _, route := range web.RegisteredWebRoutes() {
		if !strings.HasPrefix(route.Pattern, fleetPrefix) {
			continue
		}
		if route.Enabled != nil && !route.Enabled() {
			continue
		}

		mux.Handle(route.Pattern, web.InsecureMiddleware(route.Build(web.RouteDeps{})))
		mounted++
	}

	require.Equal(t, 2, mounted, "linking the sample source mounts the shell route and the assets route")

	return mux
}

// get answers one request against the mounted fleet routes.
func get(t *testing.T, mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	t.Helper()

	answer := httptest.NewRecorder()
	mux.ServeHTTP(answer, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody))

	return answer
}

// TestFleetShellRendersSampleEstate requests the fleet address through the
// registered routes, in a test binary that links the sample source.
//
// VALIDATES: AC-1 at the handler -- the route is mounted because a source is
// registered, and the shell names the sample estate and every sample device.
// PREVENTS: a sample build whose fleet address answers a page with no estate
// in it, and a registration that never reaches the web route registry.
func TestFleetShellRendersSampleEstate(t *testing.T) {
	answer := get(t, mountFleet(t), "/fleet/")

	require.Equal(t, http.StatusOK, answer.Code)
	assert.True(t, strings.HasPrefix(answer.Header().Get("Content-Type"), "text/html"))

	page := answer.Body.String()
	assert.Contains(t, page, estateName)
	assert.Contains(t, page, "edge-lon-04")

	// A device is too large to copy per iteration, so the loop indexes.
	sampled := devices()
	for index := range sampled {
		assert.Contains(t, page, sampled[index].Name)
	}
}

// TestFleetShellAssetsAreServed requests the stylesheet and the script the
// shell links, through the registered assets route.
//
// VALIDATES: both files answer 200 with a body.
// PREVENTS: a shell that renders and then loads nothing in the browser.
func TestFleetShellAssetsAreServed(t *testing.T) {
	mux := mountFleet(t)

	for _, asset := range []string{"/fleet/assets/fleet-shell.css", "/fleet/assets/fleet.js"} {
		answer := get(t, mux, asset)

		assert.Equal(t, http.StatusOK, answer.Code, asset)
		assert.NotZero(t, answer.Body.Len(), asset)
	}
}

// TestFleetUnknownAddressIsNotFound requests an address under the fleet
// prefix that no page owns.
//
// VALIDATES: the shell route answers 404 for it.
// PREVENTS: the estate page answering every address under the prefix, which
// would hide a mistyped link behind a 200.
func TestFleetUnknownAddressIsNotFound(t *testing.T) {
	answer := get(t, mountFleet(t), "/fleet/no-such-page/")

	assert.Equal(t, http.StatusNotFound, answer.Code)
}
