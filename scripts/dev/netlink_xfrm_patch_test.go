package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	netlinkXFRMPatchPath = "scripts/dev/patches/netlink-xfrm-fixes.patch"
	netlinkXFRMRecovery  = "git apply scripts/dev/patches/netlink-xfrm-fixes.patch"
)

// TestNetlinkXFRMPatchApplied keeps go mod vendor from removing the XFRM fixes.
func TestNetlinkXFRMPatchApplied(t *testing.T) {
	root := repoRoot(t)
	patch := filepath.Join(root, filepath.FromSlash(netlinkXFRMPatchPath))
	// t.Context() ends the child with the test; it cannot bound this call, since
	// it is canceled only just before the Cleanup functions run. The timeout is
	// what kills a git that never returns.
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "apply", "--reverse", "--check", patch)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("vendored netlink XFRM fixes are missing or differ from %s: %v\n%s\nRecovery: run `%s` from the repository root after `go mod vendor`.",
			netlinkXFRMPatchPath, err, out, netlinkXFRMRecovery)
	}
}
