// Design: ai/rules/plugins.md -- ze_fleet_ui dormant-without-a-source validation
//
//go:build ze_web && ze_fleet_ui && !ze_fleet_ui_sample

package hub

// VALIDATES: AC-7 at the hub -- a build with the fleet package and no data
// source (every shipped build: ze_fleet_ui_sample is in no manifest) holds the
// fleet routes dormant. startWebServer skips a route whose Enabled answers
// false, so it mounts none of them and registers no fleet portal entry.
// PREVENTS: a standard build serving a fleet page, or listing Fleet in the
// portal menu, with no estate behind it.

import "testing"

func TestBuildTag_FleetUI_DormantWithoutSource(t *testing.T) {
	routes := registeredFleetRoutes()
	if len(routes) == 0 {
		t.Fatal("ze_web && ze_fleet_ui build: no fleet route registered, so this test proves nothing")
	}

	for _, route := range routes {
		if route.Enabled == nil {
			t.Fatalf("fleet route %q carries no Enabled gate, so every build mounts it", route.Pattern)
		}
		if route.Enabled() {
			t.Fatalf("fleet route %q is enabled in a build that links no data source", route.Pattern)
		}
	}
}
