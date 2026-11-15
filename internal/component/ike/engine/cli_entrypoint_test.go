// Design: docs/architecture/cli/plugin-modes.md -- registered IKE plugin CLI
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestIKECLIRegisteredFeatures checks that the registered IKE CLI reaches the
// shared flag handler. The features request returns before any hub connection;
// this test does not establish that the live engine can complete startup.
func TestIKECLIRegisteredFeatures(t *testing.T) {
	reg := registry.Lookup("ike")
	if reg == nil {
		t.Fatal("IKE plugin is not registered")
	}
	if reg.CLIHandler == nil {
		t.Fatal("IKE plugin has no CLI handler")
	}
	if got := reg.CLIHandler([]string{"--features"}); got != 0 {
		t.Fatalf("IKE plugin --features exit code = %d, want 0", got)
	}
}
