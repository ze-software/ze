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

	// The ping feature (show ping, monitor ping, resolve ping, and the offline
	// `ze ping` root) is owned by the dedicated ping module. None of its handlers
	// may remain in the central show, BGP monitor, resolve, or diag packages.
	pingOwner := "internal/component/ping/cmd"
	if _, err := os.Stat(filepath.Join(root, pingOwner)); err != nil {
		t.Errorf("ping feature module %s is missing: %v", pingOwner, err)
	}
	for _, gone := range []string{
		"internal/component/cmd/show/ping.go",        // show ping handler -> ping module
		"internal/component/cmd/show/ping_stream.go", // monitor ping stream -> ping module
	} {
		if _, err := os.Stat(filepath.Join(root, gone)); err == nil {
			t.Errorf("central ping file %s still exists; the ping feature is owned by %s", gone, pingOwner)
		}
	}
	for _, c := range []struct{ file, symbol string }{
		{"internal/component/bgp/plugins/cmd/monitor/monitor.go", "handleMonitorPing"},
		{"internal/component/resolve/cmd/resolve.go", "func handlePing"},
		{"cmd/ze/diag/diag.go", "func RunPing"},
	} {
		body, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Errorf("read %s: %v", c.file, err)
			continue
		}
		if strings.Contains(string(body), c.symbol) {
			t.Errorf("%s still defines %q; the ping feature is owned by %s", c.file, c.symbol, pingOwner)
		}
	}

	// The traceroute feature (show traceroute, show probe-round, monitor
	// traceroute, resolve traceroute) is owned by the dedicated traceroute
	// module. None of its handlers may remain in the central show, BGP monitor,
	// or resolve packages. There is no offline `ze traceroute` root.
	tracerouteOwner := "internal/component/traceroute/cmd"
	if _, err := os.Stat(filepath.Join(root, tracerouteOwner)); err != nil {
		t.Errorf("traceroute feature module %s is missing: %v", tracerouteOwner, err)
	}
	for _, gone := range []string{
		"internal/component/cmd/show/traceroute.go",          // show traceroute -> traceroute module
		"internal/component/cmd/show/traceroute_parallel.go", // show probe-round -> traceroute module
		"internal/component/cmd/show/traceroute_stream.go",   // monitor traceroute stream -> traceroute module
	} {
		if _, err := os.Stat(filepath.Join(root, gone)); err == nil {
			t.Errorf("central traceroute file %s still exists; the traceroute feature is owned by %s", gone, tracerouteOwner)
		}
	}
	for _, c := range []struct{ file, symbol string }{
		{"internal/component/bgp/plugins/cmd/monitor/monitor.go", "handleMonitorTraceroute"},
		{"internal/component/resolve/cmd/resolve.go", "func handleTraceroute"},
	} {
		body, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Errorf("read %s: %v", c.file, err)
			continue
		}
		if strings.Contains(string(body), c.symbol) {
			t.Errorf("%s still defines %q; the traceroute feature is owned by %s", c.file, c.symbol, tracerouteOwner)
		}
	}

	// The `show interface` family (interface, detail, counters, scan, rate) and
	// `monitor interface rate` are owned by the iface component: every handler
	// reads interface state through the iface backend. None may remain in the
	// central show package.
	ifaceOwner := "internal/component/iface/cmd"
	for _, want := range []string{
		"internal/component/iface/cmd/show_interface.go", // show interface family
		"internal/component/iface/cmd/interface_rate.go", // show/monitor interface rate
	} {
		if _, err := os.Stat(filepath.Join(root, want)); err != nil {
			t.Errorf("iface interface command file %s is missing: %v", want, err)
		}
	}
	centralRate := "internal/component/cmd/show/interface_rate.go"
	if _, err := os.Stat(filepath.Join(root, centralRate)); err == nil {
		t.Errorf("central %s still exists; the interface rate surface is owned by %s", centralRate, ifaceOwner)
	}
	centralShow := "internal/component/cmd/show/show.go"
	showBody, err := os.ReadFile(filepath.Join(root, centralShow))
	if err != nil {
		t.Fatalf("read central show.go: %v", err)
	}
	for _, symbol := range []string{"func handleShowInterface", "func handleMonitorInterfaceRate"} {
		if strings.Contains(string(showBody), symbol) {
			t.Errorf("central show.go still defines %q; the interface surface is owned by %s", symbol, ifaceOwner)
		}
	}

	// The `show traffic` (QoS) command is owned by the traffic component: its
	// handler reads traffic.GetBackend(). It must not remain in central show.
	trafficOwner := "internal/component/traffic/cmd/traffic.go"
	if _, err := os.Stat(filepath.Join(root, trafficOwner)); err != nil {
		t.Errorf("traffic command handler %s is missing: %v", trafficOwner, err)
	}
	if strings.Contains(string(showBody), "func handleShowTraffic") {
		t.Errorf("central show.go still defines handleShowTraffic; the traffic surface is owned by %s", trafficOwner)
	}

	// The `monitor vpn ipsec` command is owned by the ike component: its
	// handler reads ike/engine events. The YANG node must live in ike/schema,
	// not in the central monitor schema.
	ikeMonitorSchema := "internal/component/ike/schema/ze-ipsec-cmd.yang"
	centralMonitorSchema := "internal/component/cmd/monitor/schema/ze-cli-monitor-cmd.yang"
	ikeSchemaBody, err := os.ReadFile(filepath.Join(root, ikeMonitorSchema))
	if err != nil {
		t.Fatalf("read ike schema: %v", err)
	}
	if !strings.Contains(string(ikeSchemaBody), `ze:command "ze-monitor:vpn-ipsec"`) {
		t.Errorf("ike schema %s must declare ze-monitor:vpn-ipsec", ikeMonitorSchema)
	}
	centralMonBody, err := os.ReadFile(filepath.Join(root, centralMonitorSchema))
	if err != nil {
		t.Fatalf("read central monitor schema: %v", err)
	}
	if strings.Contains(string(centralMonBody), `ze-monitor:vpn-ipsec`) {
		t.Errorf("central monitor schema still declares ze-monitor:vpn-ipsec; it is owned by %s", ikeMonitorSchema)
	}
}
