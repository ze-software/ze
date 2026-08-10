// Design: docs/architecture/chaos-web-dashboard.md -- RunOrchestrator entry validation

package orchestrator

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/chaos/scenario"
)

// TestRunConfigRangeConflict proves the wiring: RunOrchestrator validates the
// assembled OrchestratorConfig at entry and rejects a single-port listener that
// falls inside the peer port ranges, returning exit code 1 before any setup
// (AC-10). A non-conflicting config would proceed into a blocking run, so only
// the reject path is exercised here.
func TestRunConfigRangeConflict(t *testing.T) {
	cfg := &orchestratorConfig{
		Profiles: []scenario.PeerProfile{
			{Index: 0, ZePort: 1790, Port: 1890},
			{Index: 1, ZePort: 1791, Port: 1891},
			{Index: 2, ZePort: 1792, Port: 1892},
		},
		// Collides with the derived bgp range [1790, 1796).
		WebAddr: "127.0.0.1:1791",
	}

	code := RunOrchestrator(context.Background(), cfg)
	if code != 1 {
		t.Fatalf("RunOrchestrator = %d, want 1 (range conflict must be rejected at entry)", code)
	}
}
