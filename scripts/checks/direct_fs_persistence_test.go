// Smoke + selftest for scripts/checks/direct_fs_persistence.go (the //go:build
// ignore runtime-state persistence guard). The checker is ignore-tagged so it is
// excluded from normal compilation; this test gives the package a buildable
// target and runs the checker as a subprocess, asserting the current tree passes
// -- no runtime state is persisted with raw os writes instead of the managed zefs
// store (internal/core/statestore). Regression guard for the loose-state-file
// sweep (ddos-detect baseline, traffic tc-snapshot, ntp time, bfd auth seq, the
// config health/pushed hashes).

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestNoDirectFSStatePersistence runs the guard against the live tree and asserts
// no runtime-state persister bypasses the zefs store with a raw filesystem write.
func TestNoDirectFSStatePersistence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/direct_fs_persistence.go")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("direct-fs-persistence gate failed (a runtime-state persister writes a loose file instead of using internal/core/statestore; or a new legitimate non-state writer needs an allowlist entry):\n%s", out)
	}
	if !strings.Contains(string(out), "direct-fs-persistence: OK") {
		t.Fatalf("direct_fs_persistence.go did not report OK:\n%s", out)
	}
}

// TestDirectFSPersistenceSelftest proves the AST detection actually fires --
// isolated temp-dir fixtures (os.WriteFile/os.Rename/os.OpenFile-with-write-flag
// are flagged; statestore usage, reads, O_RDONLY opens, and Mkdir/Remove/CreateTemp
// are not) -- so a regression in the detector is caught even while the tree is clean.
func TestDirectFSPersistenceSelftest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/direct_fs_persistence.go", "--selftest")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("direct-fs-persistence --selftest failed:\n%s", out)
	}
	if !strings.Contains(string(out), "direct-fs-persistence selftest OK") {
		t.Fatalf("direct_fs_persistence.go --selftest did not report OK:\n%s", out)
	}
}
