// Design: ai/rules/plugins.md -- ze_fleet_ui present build validation
//
//go:build ze_web && ze_fleet_ui

package hub

// VALIDATES: with BOTH ze_fleet_ui and ze_web (the fleet interface is a
// DEPENDENT gate -- it nests under the web package and registers into its
// route registry) the fleet routes are registered. The && matches
// register_fleet_ui.go's own guard.
// PREVENTS: a regression where ze_fleet_ui is set but the fleet package is not
// linked (the blank import dropped from register_fleet_ui.go).

import (
	"strings"
	"testing"

	zeweb "github.com/ze-software/ze/internal/component/web"
)

// fleetRoutePrefix is the method and path every fleet route pattern starts
// with.
const fleetRoutePrefix = "GET /fleet/"

// registeredFleetRoutes answers the fleet routes the web route registry
// holds, which are the ones startWebServer iterates.
func registeredFleetRoutes() []zeweb.WebRoute {
	var found []zeweb.WebRoute
	for _, route := range zeweb.RegisteredWebRoutes() {
		if strings.HasPrefix(route.Pattern, fleetRoutePrefix) {
			found = append(found, route)
		}
	}

	return found
}

func TestBuildTag_FleetUI_Present(t *testing.T) {
	if len(registeredFleetRoutes()) == 0 {
		t.Fatal("ze_web && ze_fleet_ui build: no fleet route registered")
	}
}
