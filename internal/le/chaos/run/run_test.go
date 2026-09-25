// VALIDATES: `le chaos run <options>` reaches the chaos orchestrator's command
// line with the options unchanged.
// PREVENTS: an le command that registers and answers, while the orchestrator
// is never called.

package chaosrun

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/le/root"
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

// TestChaosRunHelpIsTheOrchestratorsHelp drives `le chaos run --help` through
// the le dispatcher. `le chaos run` forwards every word, so the help word
// reaches the orchestrator, which prints its own flag list and answers 0.
func TestChaosRunHelpIsTheOrchestratorsHelp(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = writer
	code := leroot.Dispatch("le", []string{"chaos", "run", "--help"})
	os.Stderr = previous
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	page, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("le chaos run --help exited %d, want 0", code)
	}
	if !strings.Contains(string(page), "--config-only") {
		t.Errorf("le chaos run --help printed no orchestrator flag list:\n%s", page)
	}
}
