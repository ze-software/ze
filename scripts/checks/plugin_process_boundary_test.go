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
	"os"
	"os/exec"
	"slices"
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
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-process-boundary gate failed (a plugin calls a same-process-effect function with no IsInternal()/warnIfExternal() guard):\n%s", out)
	}
	if !strings.Contains(string(out), "plugin-process-boundary: OK") {
		t.Fatalf("plugin_process_boundary.go did not report OK:\n%s", out)
	}
}

// TestBoundaryScanRootsDerivedFromGenerator asserts the checker derives its
// scan roots from the generator's plugin-namespace source of truth
// (scripts/codegen/plugin_imports.go pluginDirs + nestedPluginDomains) rather
// than a private hardcoded list. The l2tp and firewall nested plugin
// namespaces hold sdk.NewWithConn engines that the old two-root list never
// scanned (spec-layout-0-umbrella child 2).
// PREVENTS: a plugin namespace added to the generator being silently
// invisible to the process-boundary guard.
func TestBoundaryScanRootsDerivedFromGenerator(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/plugin_process_boundary.go", "--print-roots")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin_process_boundary.go --print-roots failed:\n%s", out)
	}
	for _, root := range []string{
		"internal/plugins",
		"internal/component/bgp/plugins",
		"internal/component/l2tp/plugins",
		"internal/component/firewall/plugins",
	} {
		if !slices.Contains(strings.Split(strings.TrimSpace(string(out)), "\n"), root) {
			t.Errorf("derived scan roots missing %q:\n%s", root, out)
		}
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
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-process-boundary --selftest failed:\n%s", out)
	}
	if !strings.Contains(string(out), "plugin-process-boundary selftest OK") {
		t.Fatalf("plugin_process_boundary.go --selftest did not report OK:\n%s", out)
	}
}
