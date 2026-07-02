// Smoke test for scripts/checks/plugin_process_boundary.go (the //go:build
// ignore process-boundary guard). The checker is ignore-tagged so it is
// excluded from normal compilation; this test gives the package a buildable
// target and runs the checker as a subprocess, asserting the current tree
// passes -- no plugin calls a same-process-effect function (the dangerous
// pattern list in the checker) without an IsInternal()/warnIfExternal()
// guard somewhere in its own package.

package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// TestNoUnguardedPluginProcessBoundaryCall runs
// scripts/checks/plugin_process_boundary.go and asserts the repository
// passes the gate: every known same-process-effect call (iface.
// RegisterOwnedAddresses/UnregisterOwnedAddresses/GetBackend/
// SubscribeCollectNotify/UnsubscribeCollectNotify, trafficstat.EnsureGlobal/
// Global) made by a plugin package outside the owning package is guarded by
// an IsInternal() or warnIfExternal() call somewhere in that same package.
func TestNoUnguardedPluginProcessBoundaryCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/plugin_process_boundary.go")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-process-boundary gate failed (a plugin calls a same-process-effect function with no IsInternal()/warnIfExternal() guard):\n%s", out)
	}
	if !strings.Contains(string(out), "plugin-process-boundary: OK") {
		t.Fatalf("plugin_process_boundary.go did not report OK:\n%s", out)
	}
}

// VALIDATES: --selftest (isolated temp-dir fixtures, no repo mutation --
// mirrors dep_audit.py's --selftest convention) proves the checker actually
// detects a dangerous call made through a RENAMED import alias (e.g.
// `ifcomp "internal/component/iface"` then `ifcomp.GetBackend(...)`), not
// just the literal unaliased package name -- the real tree currently has
// zero aliased dangerous calls, so the plain smoke test above could not have
// caught a regression in the alias-resolution logic.
// PREVENTS: a plugin bypassing the gate entirely by importing a watched
// package under any local name other than its default.
func TestPluginProcessBoundarySelftest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/plugin_process_boundary.go", "--selftest")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-process-boundary --selftest failed:\n%s", out)
	}
	if !strings.Contains(string(out), "plugin-process-boundary selftest OK") {
		t.Fatalf("plugin_process_boundary.go --selftest did not report OK:\n%s", out)
	}
}
