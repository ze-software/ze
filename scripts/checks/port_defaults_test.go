// VALIDATES: the hand-maintained Go listener-default table
// (internal/component/config/listener_defaults.go) stays pinned to each
// service's YANG `refine port { default N }`, by running
// scripts/checks/port_defaults.go over the live tree (AC-11).
// PREVENTS: a YANG refine port default drifting from the Go table (or vice
// versa) so the daemon binds a port the schema documents differently.

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestPortDefaultsGate runs scripts/checks/port_defaults.go and asserts the
// current tree passes: every central listener service's Go default port equals
// its YANG refine port default. repoRoot and checkTimeout come from
// checks_test.go in this package.
func TestPortDefaultsGate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/port_defaults.go")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if !strings.Contains(string(out), "port-defaults: OK") {
		t.Fatalf("port-defaults gate did not pass (err=%v):\n%s", err, out)
	}
}

// TestPortDefaultsSelftest runs the checker's --selftest, which exercises the
// comparison logic with synthetic inputs (match, value drift, unmapped service,
// missing YANG default, stale mapping) so a broken comparison is caught
// independently of the live tree.
func TestPortDefaultsSelftest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/port_defaults.go", "--selftest")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("port-defaults selftest failed:\n%s", out)
	}
	if !strings.Contains(string(out), "port-defaults: selftest OK") {
		t.Fatalf("port-defaults --selftest did not report OK:\n%s", out)
	}
}
