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
	"os"
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

// TestMigratedDaemonCommandsLiveInOwners asserts the command-surface-ownership
// daemon-RPC migrations stay migrated: each owner-specific command package
// lives under its owner, not under the central internal/component/cmd/ verb
// tree. Prevents a regression that re-adds an owner command centrally.
func TestMigratedDaemonCommandsLiveInOwners(t *testing.T) {
	root := repoRoot(t)
	moved := map[string]string{
		"internal/component/cmd/l2tp":       "internal/component/l2tp/cmd",
		"internal/component/cmd/pppoe":      "internal/component/pppoe/cmd",
		"internal/component/cmd/subscriber": "internal/component/subscriber/cmd",
		"internal/component/cmd/bfd":        "internal/component/bfd/cmd",
		"internal/component/cmd/archive":    "internal/component/config/archive/cmd",
		// cache/commit handlers live in bgp/plugins/cmd/{cache,commit}; their
		// YANG schema was the last central remnant and now lives beside them.
		"internal/component/cmd/cache":  "internal/component/bgp/plugins/cmd/cache/schema",
		"internal/component/cmd/commit": "internal/component/bgp/plugins/cmd/commit/schema",
	}
	for central, owner := range moved {
		if _, err := os.Stat(filepath.Join(root, central)); err == nil {
			t.Errorf("central daemon-command package %s still exists; it must live in %s", central, owner)
		}
		if _, err := os.Stat(filepath.Join(root, owner)); err != nil {
			t.Errorf("owner daemon-command package %s is missing: %v", owner, err)
		}
	}
	// The generic clear verb keeps only its schema; owner-specific clear
	// handlers were extracted to their owners.
	for _, ownerHandler := range []string{
		"internal/component/resolve/cmd/dns.go", // ze-clear:dns-cache
		"internal/component/ike/cmd/ipsec.go",   // ze-clear:vpn-ipsec-sa
	} {
		if _, err := os.Stat(filepath.Join(root, ownerHandler)); err != nil {
			t.Errorf("extracted clear handler %s is missing: %v", ownerHandler, err)
		}
	}

	// The metrics verb is generic (Prometheus registry); only ze-bgp:pool-stats
	// is owner-specific (reads the BGP RIB attribute pools). Its handler moved to
	// the RIB command cluster and must not return to the central metrics package.
	poolStatsHandler := "internal/component/bgp/plugins/cmd/rib/pool_stats.go"
	centralMetrics := "internal/component/cmd/metrics/metrics.go"
	centralMetricsSchema := "internal/component/cmd/metrics/schema/ze-cli-metrics-cmd.yang"
	if _, err := os.Stat(filepath.Join(root, poolStatsHandler)); err != nil {
		t.Errorf("pool-stats handler must live in the RIB command owner: %v", err)
	}
	metricsBody, err := os.ReadFile(filepath.Join(root, centralMetrics))
	if err != nil {
		t.Fatalf("read central metrics handler: %v", err)
	}
	if strings.Contains(string(metricsBody), "handlePoolStats") {
		t.Error("central metrics package still defines handlePoolStats; pool-stats is owned by the BGP RIB command cluster")
	}
	metricsSchema, err := os.ReadFile(filepath.Join(root, centralMetricsSchema))
	if err != nil {
		t.Fatalf("read central metrics schema: %v", err)
	}
	if strings.Contains(string(metricsSchema), "ze-bgp:pool-stats") {
		t.Error("central metrics schema still declares ze-bgp:pool-stats; it is owned by bgp/plugins/cmd/rib/schema")
	}
}
