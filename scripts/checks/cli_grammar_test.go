// VALIDATES: the whole built-in CLI command surface obeys the grammar rules
// R1-R8 and carries no --flag in any .yang (ai/rules/cli.md), by running
// scripts/cli_grammar.go over the compile-time YANG command tree.
// PREVENTS: a noun-first, flagged, or mis-typed command from regressing the surface.

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestCLIGrammarGateStatic runs scripts/checks/cli_grammar.go and asserts the
// current command surface passes the grammar gate (Feeder 1). repoRoot and
// checkTimeout come from checks_test.go in this package.
func TestCLIGrammarGateStatic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/cli_grammar.go")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if !strings.Contains(string(out), "cli-grammar: OK") {
		t.Fatalf("cli-grammar gate did not pass (err=%v):\n%s", err, out)
	}
}
