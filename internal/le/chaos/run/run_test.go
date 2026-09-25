// VALIDATES: `le chaos run <options>` reaches the chaos orchestrator's command
// line with the options unchanged.
// PREVENTS: an le command that registers and answers, while the orchestrator
// ze-chaos ran is never called.

package chaosrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/leroot"
)

// TestChaosRunReachesTheOrchestrator drives `le chaos run --config-only` through
// the le dispatcher. Only the orchestrator writes a ze config naming
// `chaos-peer-` peers, so the file it leaves at --config-out proves the options
// arrived, and the exit code proves its verdict reached the caller.
func TestChaosRunReachesTheOrchestrator(t *testing.T) {
	out := filepath.Join(t.TempDir(), "chaos.conf")
	code := leroot.Dispatch("le", []string{
		"chaos", "run", "--config-only", "--seed", "42", "--peers", "3", "--config-out", out, "--quiet",
	})
	if code != 0 {
		t.Fatalf("le chaos run --config-only exited %d, want 0", code)
	}
	data, err := os.ReadFile(out) //nolint:gosec // the path is this test's own temp file
	if err != nil {
		t.Fatalf("the orchestrator wrote no config: %v", err)
	}
	if !strings.Contains(string(data), "peer chaos-peer-") {
		t.Errorf("config at %s names no chaos peer:\n%s", out, data)
	}

	if code := leroot.Dispatch("le", []string{"chaos", "run", "--peers", "0", "--config-only"}); code == 0 {
		t.Error("le chaos run --peers 0 exited 0; the orchestrator's refusal did not reach the caller")
	}
}
