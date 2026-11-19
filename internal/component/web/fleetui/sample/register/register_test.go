package register

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

// TestRegisteredSampleMountsTheFleetRoutes mounts the registered fleet routes
// the way the hub does when the web server starts without login: it skips a
// route whose Enabled answers false, builds the others, and wraps each in the
// insecure middleware. Then it asks for the estate dashboard.
//
// VALIDATES: linking this package registers the sample as the build's
// source: every fleet route is enabled, the prototype-only ones included, and
// the shell draws the sample estate.
// PREVENTS: a review build whose registration never reaches the web route
// registry, so its fleet address answers nothing.
func TestRegisteredSampleMountsTheFleetRoutes(t *testing.T) {
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

	require.Equal(t, fleetRouteCount, mounted, "linking the registration mounts every fleet route, the script's included")

	answer := httptest.NewRecorder()
	mux.ServeHTTP(answer, httptest.NewRequestWithContext(t.Context(), http.MethodGet, fleetPrefix, http.NoBody))

	require.Equal(t, http.StatusOK, answer.Code)
	assert.Contains(t, answer.Body.String(), "fleet-dashboard-slot", "the estate dashboard draws its cards")
}
