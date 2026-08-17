// Smoke + selftest for scripts/checks/cli_dash_stdio.go (the //go:build ignore
// gate enforcing that a filename-accepting command reads/writes user paths through
// internal/core/cliio so "-" means stdin/stdout). The checker is ignore-tagged, so
// this test runs it as a subprocess: the live run asserts the migrated tree is
// clean, and --selftest proves the AST taint detector actually fires on the
// pre-migration shapes (direct fs.Arg reads, alias chains, range-over-args,
// flag derefs, funnel parameters) and stays quiet on derived/helper-routed paths.

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestCliDashStdioLive runs the gate against the live tree and asserts no command
// reads/writes a user-supplied path with a raw os call instead of cliio.
func TestCliDashStdioLive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/cli_dash_stdio.go")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cli-dash-stdio gate failed (a command reads/writes a CLI-supplied path with a raw os call; route it through internal/core/cliio, or allowlist a genuinely non-\"-\" path with a reason):\n%s", out)
	}
	if !strings.Contains(string(out), "cli-dash-stdio: OK") {
		t.Fatalf("cli_dash_stdio.go did not report OK:\n%s", out)
	}
}

// TestDashStdioGate proves the detector fires: --selftest builds isolated fixtures
// reproducing the pre-migration violation shapes (all flagged) and the legitimate
// shapes (none flagged), so a regression in the detector is caught while the tree
// is clean (R-2, AC-16).
func TestDashStdioGate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/cli_dash_stdio.go", "--selftest")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cli-dash-stdio --selftest failed:\n%s", out)
	}
	if !strings.Contains(string(out), "cli-dash-stdio selftest OK") {
		t.Fatalf("cli_dash_stdio.go --selftest did not report OK:\n%s", out)
	}
}
