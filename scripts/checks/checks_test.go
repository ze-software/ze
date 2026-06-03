// Smoke test for the //go:build ignore checker in this directory.
//
// command_ownership.go uses //go:build ignore so it is excluded from normal
// compilation and from golangci-lint's type-checking pipeline. This test file
// does NOT have the ignore tag, so it is the only buildable file in the package
// and gives the linter and verify-changed a real target. It runs the checker as
// a subprocess and asserts the current tree passes the command-surface-ownership
// gate.

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const checkTimeout = 60 * time.Second

// TestNoOwnerAllowlistIsEnforced runs scripts/checks/command_ownership.go and
// asserts the repository passes the ownership gate: no owner command package
// imports cmd/ze (TestOwnerCommandRegistrationHasNoCmdZeImport), every
// RegisterRootHandler lives in an internal owner, and every central root is in
// the no-owner allowlist (AC-1, AC-2, AC-4, AC-8).
func TestNoOwnerAllowlistIsEnforced(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/command_ownership.go")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command-ownership gate failed (the migration left a violation):\n%s", out)
	}
	if !strings.Contains(string(out), "command-ownership: OK") {
		t.Fatalf("command_ownership.go did not report OK:\n%s", out)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}
