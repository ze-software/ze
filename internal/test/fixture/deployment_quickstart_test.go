// Design: docs/guide/quickstart.md -- the supplied two-peer lab is reachable as a functional proof.
package fixture

import (
	"slices"
	"testing"
)

// TestDeploymentQuickstartFixtureRegistered checks the same registry the .ci
// dispatcher reads, so an isolated helper cannot stand in for the user flow.
func TestDeploymentQuickstartFixtureRegistered(t *testing.T) {
	names := Names()
	if !slices.Contains(names, "ui/local-command-daemon-ssh") {
		t.Fatal("the fixture registry is missing its known SSH driver")
	}
	if !slices.Contains(names, "ui/deployment-quickstart") {
		t.Fatal("the supplied two-peer quickstart has no registered functional proof")
	}
}
