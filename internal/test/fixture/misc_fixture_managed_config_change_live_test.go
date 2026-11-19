// Design: docs/functional-tests.md -- a compiled driver's decisions are unit-tested, its .ci proves the path
// Related: misc_fixture_managed_config_change_live.go -- the applied-line count under test

package fixture

import (
	"strings"
	"testing"
)

// TestFleetConfigChangeLiveAppliedLines proves which lines of a hub's output
// the config-change driver counts as the client applying a version. The
// method is a hub log with the client's applied lines for two versions,
// another client's applied line for the counted version, and a refusal of
// the client naming it: only the named client's applied lines for the named
// version may count, or "exactly one applied line" could pass on the first
// connection's line or on another client's.
func TestFleetConfigChangeLiveAppliedLines(t *testing.T) {
	const served, imported = "1111111111111111", "2222222222222222"
	output := strings.Join([]string{
		`level=INFO msg="managed client connected" subsystem=hub.managed-server name=edge-01`,
		`level=INFO msg="managed client applied config" subsystem=hub.managed-server name=edge-01 version=` + served,
		`level=INFO msg="managed client applied config" subsystem=hub.managed-server name=edge-02 version=` + imported,
		`level=WARN msg="managed client rejected config" subsystem=hub.managed-server name=edge-01 version=` + imported,
		`level=INFO msg="managed client applied config" subsystem=hub.managed-server name=edge-01 version=` + imported,
		``,
	}, "\n")

	if got := fleetConfigChangeLiveAppliedLines(output, imported); got != 1 {
		t.Fatalf("applied lines of edge-01 for %s = %d, want 1", imported, got)
	}
	if got := fleetConfigChangeLiveAppliedLines(output, served); got != 1 {
		t.Fatalf("applied lines of edge-01 for %s = %d, want 1", served, got)
	}
	if got := fleetConfigChangeLiveAppliedLines("", imported); got != 0 {
		t.Fatalf("applied lines in an empty output = %d, want 0", got)
	}
}
