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
		"internal/component/cmd/pppoe":      "internal/component/l2tp/pppoe/cmd",
		"internal/component/cmd/subscriber": "internal/component/l2tp/subscriber/cmd",
		"internal/component/cmd/bfd":        "internal/component/bfd/cmd",
		"internal/component/cmd/archive":    "internal/component/config/archive/cmd",
		// cache/commit handlers live in bgp/plugins/cmd/{cache,commit}; their
		// YANG schema was the last central remnant and now lives beside them.
		"internal/component/cmd/cache":  "internal/component/bgp/plugins/cmd/cache/yang",
		"internal/component/cmd/commit": "internal/component/bgp/plugins/cmd/commit/yang",
	}
	for central, owner := range moved {
		if _, err := os.Stat(filepath.Join(root, central)); err == nil {
			t.Errorf("central daemon-command package %s still exists; it must live in %s", central, owner)
		}
		if _, err := os.Stat(filepath.Join(root, owner)); err != nil {
			t.Errorf("owner daemon-command package %s is missing: %v", owner, err)
		}
	}
	// Every clear command is fully owned: its handler AND its YANG schema live
	// in the owning component. The central clear verb package is a bare
	// verb-root anchor that declares no owner command (each owner merges its own
	// `clear <noun> ...` subtree). See ai/rules/plugins.md.
	for _, ownerHandler := range []string{
		"internal/component/resolve/cmd/dns.go", // ze-clear:dns-cache (schema: resolve/yang)
		"internal/component/ike/cmd/ipsec.go",   // ze-clear:vpn-ipsec-sa (schema: ike/yang)
		"internal/component/iface/cmd/clear.go", // ze-clear:interface-counters (schema: iface/yang)
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
	centralMetricsSchema := "internal/component/cmd/metrics/yang/ze-cli-metrics-cmd.yang"
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
		t.Error("central metrics schema still declares ze-bgp:pool-stats; it is owned by bgp/plugins/cmd/rib/yang")
	}

	// The ping feature (show ping, monitor ping, resolve ping) is owned by
	// the dedicated ping module. show/monitor ping run as local handlers.
	// None of its handlers may remain in the central show, BGP monitor,
	// resolve, or diag packages.
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
	diagPath := filepath.Join(root, "cmd", "ze", "diag", "diag.go")
	if _, err := os.Stat(diagPath); err == nil {
		body, readErr := os.ReadFile(diagPath)
		if readErr != nil {
			t.Errorf("read %s: %v", diagPath, readErr)
		} else if strings.Contains(string(body), "func RunPing") {
			t.Errorf("cmd/ze/diag/diag.go still defines \"func RunPing\"; the ping feature is owned by %s", pingOwner)
		}
	}

	// The traceroute feature (show traceroute, show probe-round, monitor
	// traceroute, resolve traceroute) is owned by the dedicated traceroute
	// module. None of its handlers may remain in the central show, BGP monitor,
	// or resolve packages. show/monitor traceroute run as local handlers.
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
	ikeMonitorSchema := "internal/component/ike/yang/ze-ipsec-cmd.yang"
	centralMonitorSchema := "internal/component/cmd/monitor/yang/ze-cli-monitor-cmd.yang"
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

	// The `show bgp-health` overview and the `delete bgp peer` command surface
	// are owned by the BGP peer command package: their handlers live in
	// bgp/plugins/cmd/peer and the YANG nodes live in that owner's schema, not
	// the central show/delete schemas. Removing the delete command leaves the
	// central delete schema a bare verb-root anchor.
	peerOwnerSchema := "internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang"
	peerSchemaBody, err := os.ReadFile(filepath.Join(root, peerOwnerSchema))
	if err != nil {
		t.Fatalf("read peer owner schema: %v", err)
	}
	for _, tok := range []string{
		"ze-show:bgp-health",
		"ze-delete:bgp-peer",
	} {
		if !strings.Contains(string(peerSchemaBody), tok) {
			t.Errorf("peer owner schema %s must declare %q", peerOwnerSchema, tok)
		}
	}
	if strings.Contains(string(showBody), "func handleShowBGPHealth") {
		t.Error("central show.go still defines handleShowBGPHealth; show bgp-health is owned by bgp/plugins/cmd/peer")
	}
	deleteSchema := "internal/component/cmd/delete/yang/ze-cli-delete-cmd.yang"
	deleteBody, err := os.ReadFile(filepath.Join(root, deleteSchema))
	if err != nil {
		t.Fatalf("read delete schema: %v", err)
	}
	if strings.Contains(string(deleteBody), "ze:command") {
		t.Errorf("central delete schema %s must be a bare verb-root anchor (no ze:command); delete bgp peer is owned by the BGP peer command package", deleteSchema)
	}
}

// TestGenericCentralCommandsStayCentral is the inverse of
// TestMigratedDaemonCommandsLiveInOwners: it asserts that generic, cross-cutting
// commands that have NO removable owner intentionally remain in the central verb
// packages. Prevents a future session from migrating a command that was already
// decided to stay central.
//
// Criterion for inclusion: the handler reads from multiple subsystems, the
// process, or a cross-plugin registry. Removing any single component must not
// remove these commands.
func TestGenericCentralCommandsStayCentral(t *testing.T) {
	root := repoRoot(t)

	// Central verb packages that must continue to exist.
	centralDirs := []string{
		"internal/component/cmd/show",
		"internal/component/cmd/meta",
		"internal/component/cmd/log",
		"internal/component/cmd/subscribe",
		"internal/component/cmd/set",
		"internal/component/cmd/update",
		"internal/component/cmd/metrics",
		"internal/component/cmd/monitor",
		"internal/component/cmd/clear",
	}
	for _, d := range centralDirs {
		if _, err := os.Stat(filepath.Join(root, d)); err != nil {
			t.Errorf("generic central verb package %s must exist: %v", d, err)
		}
	}

	// Generic show handlers that read process/system state, not a single
	// removable component. Each must remain in cmd/show/show.go.
	showFile := filepath.Join(root, "internal", "component", "cmd", "show", "show.go")
	showBody, err := os.ReadFile(showFile)
	if err != nil {
		t.Fatalf("read show.go: %v", err)
	}
	genericShowHandlers := []string{
		"ze-show:version",
		"ze-show:uptime",
		"ze-show:warnings",
		"ze-show:errors",
		"ze-show:health",
	}
	for _, h := range genericShowHandlers {
		if !strings.Contains(string(showBody), h) {
			t.Errorf("generic central handler %q missing from show.go; it has no removable owner and must stay central", h)
		}
	}

	// Generic show subcommands that read process/kernel/cross-cutting state.
	// Listed by schema file so the test survives handler refactoring within
	// the central show package.
	showSchema := filepath.Join(root, "internal", "component", "cmd", "show", "yang", "ze-cli-show-cmd.yang")
	schemaBody, err := os.ReadFile(showSchema)
	if err != nil {
		t.Fatalf("read show schema: %v", err)
	}
	genericShowCommands := []string{
		`"ze-show:version"`,
		`"ze-show:uptime"`,
		`"ze-show:warnings"`,
		`"ze-show:errors"`,
		`"ze-show:health"`,
		`"ze-show:system-memory"`,
		`"ze-show:system-cpu"`,
		`"ze-show:system-date"`,
	}
	for _, cmd := range genericShowCommands {
		if !strings.Contains(string(schemaBody), cmd) {
			t.Errorf("generic central YANG command %s missing from show schema; it has no removable owner and must stay central", cmd)
		}
	}

	// The central monitor schema is a bare verb-root anchor; all monitor
	// subcommands are owned by their respective components (BGP, iface, command).
	monSchema := filepath.Join(root, "internal", "component", "cmd", "monitor", "yang", "ze-cli-monitor-cmd.yang")
	if _, err := os.Stat(monSchema); err != nil {
		t.Fatalf("central monitor verb-root schema %s must exist: %v", monSchema, err)
	}
}
