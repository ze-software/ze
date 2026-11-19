package sample

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web/fleetui"
)

// mountFleet serves every fleet route over a sample source at the test clock,
// the way a caller in process does: through fleetui.NewRoutes, which the
// daemon's registration reads the same route list as. The source is closed
// when the test ends.
func mountFleet(t *testing.T) http.Handler {
	t.Helper()

	source := New(fixedClock)
	t.Cleanup(source.Close)

	return fleetui.NewRoutes(source, "insecure")
}

// get answers one request against the mounted fleet routes.
func get(t *testing.T, mux http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	answer := httptest.NewRecorder()
	mux.ServeHTTP(answer, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody))

	return answer
}

// TestFleetShellRendersSampleEstate requests the fleet address through the
// routes NewRoutes serves over a sample source.
//
// VALIDATES: AC-1 at the handler -- the shell draws the estate dashboard of
// the sample estate.
// PREVENTS: a sample build whose fleet address answers a page with no estate
// in it.
func TestFleetShellRendersSampleEstate(t *testing.T) {
	answer := get(t, mountFleet(t), "/fleet/")

	require.Equal(t, http.StatusOK, answer.Code)
	assert.True(t, strings.HasPrefix(answer.Header().Get("Content-Type"), "text/html"))

	page := answer.Body.String()
	assert.Contains(t, page, estateName)
	assert.Contains(t, page, "fleet-dashboard-slot", "the estate dashboard draws its cards")
}

// TestFleetDevicePageRendersSampleDevice requests a sample device's
// dashboard through the routes NewRoutes serves.
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
// shell links, through the assets route NewRoutes serves.
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
