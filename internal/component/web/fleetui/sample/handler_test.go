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

// fleetPrefix is the path every fleet route pattern starts with, after its
// method.
const fleetPrefix = "/fleet/"

// fleetRouteCount is every fleet route: the shell, the assets, the
// fragments, the event stream, the audit export, the actions and the
// console, and the step and hold routes the sample's script mounts.
const fleetRouteCount = 9

// mountFleet mounts the registered fleet routes the way the hub does when the
// web server starts without login: it skips a route whose Enabled answers
// false, builds the others, and wraps each in the insecure middleware.
func mountFleet(t *testing.T) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	mounted := 0

	for _, route := range web.RegisteredWebRoutes() {
		_, path, _ := strings.Cut(route.Pattern, " ")
		if !strings.HasPrefix(path, fleetPrefix) {
			continue
		}
		if route.Enabled != nil && !route.Enabled() {
			continue
		}

		mux.Handle(route.Pattern, web.InsecureMiddleware(route.Build(web.RouteDeps{})))
		mounted++
	}

	require.Equal(t, fleetRouteCount, mounted, "linking the sample source mounts every fleet route, the script's included")

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
// registered, and the shell draws the estate dashboard of the sample estate.
// PREVENTS: a sample build whose fleet address answers a page with no estate
// in it, and a registration that never reaches the web route registry.
func TestFleetShellRendersSampleEstate(t *testing.T) {
	answer := get(t, mountFleet(t), "/fleet/")

	require.Equal(t, http.StatusOK, answer.Code)
	assert.True(t, strings.HasPrefix(answer.Header().Get("Content-Type"), "text/html"))

	page := answer.Body.String()
	assert.Contains(t, page, estateName)
	assert.Contains(t, page, "fleet-dashboard-slot", "the estate dashboard draws its cards")
}

// TestFleetDevicePageRendersSampleDevice requests a sample device's
// dashboard through the registered routes.
//
// VALIDATES: a device of the sample answers its page, named in its header.
// PREVENTS: a sample whose devices, below the second level of the tree, are
// reachable by no address.
func TestFleetDevicePageRendersSampleDevice(t *testing.T) {
	answer := get(t, mountFleet(t), "/fleet/device/edge-lon-04/dashboard/")

	require.Equal(t, http.StatusOK, answer.Code)
	assert.Contains(t, answer.Body.String(), "edge-lon-04")
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
