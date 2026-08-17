// Smoke test for scripts/checks/iface_resolution.go (the //go:build ignore
// no-direct-resolution guard). The checker is ignore-tagged so it is excluded
// from normal compilation; this test gives the package a buildable target and
// runs the checker as a subprocess, asserting the current tree passes -- no Ze
// code resolves a configured interface name straight against the kernel outside
// the documented allowlist (umbrella AC-U1, sub-spec 7).

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestNoDirectInterfaceResolution runs scripts/checks/iface_resolution.go and
// asserts the repository passes the no-direct-resolution gate: every
// netlink.LinkByName / net.InterfaceByName / SIOCGIFINDEX site in internal/ is
// either routed through the shared iface resolver or listed in the checker's
// allowlist with a reason.
func TestNoDirectInterfaceResolution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/iface_resolution.go")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("no-direct-resolution gate failed (a consumer resolves the kernel directly):\n%s", out)
	}
	if !strings.Contains(string(out), "iface-resolution: OK") {
		t.Fatalf("iface_resolution.go did not report OK:\n%s", out)
	}
}
