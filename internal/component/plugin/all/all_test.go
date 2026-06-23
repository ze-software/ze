package all

import (
	"context"
	"os/exec"
	"runtime"
	"slices"
	"sort"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func assertSnapshot(t *testing.T, label string, got, expected []string) {
	t.Helper()
	if slices.Equal(got, expected) {
		return
	}
	have := make(map[string]bool, len(got))
	for _, s := range got {
		have[s] = true
	}
	want := make(map[string]bool, len(expected))
	for _, s := range expected {
		want[s] = true
		if !have[s] {
			t.Errorf("missing %s: %q", label, s)
		}
	}
	for _, s := range got {
		if !want[s] {
			t.Errorf("unexpected %s: %q (add to expected list if intentional)", label, s)
		}
	}
}

// TestRegisteredPluginNames snapshots the full set of registered plugin names.
//
// VALIDATES: Every expected plugin is registered after init().
// PREVENTS: Silent removal of a plugin (deleted register.go, dropped import).
func TestRegisteredPluginNames(t *testing.T) {
	expected := []string{
		"bfd",
		"bgp",
		"bgp-adj-rib-in",
		"bgp-aigp",
		"bgp-bmp",
		"bgp-capa",
		"bgp-filter-aspath",
		"bgp-filter-aspath-length",
		"bgp-filter-community",
		"bgp-filter-community-match",
		"bgp-filter-irr",
		"bgp-filter-modify",
		"bgp-filter-prefix",
		"bgp-filter-remove-private-as",
		"bgp-gr",
		"bgp-healthcheck",
		"bgp-hostname",
		"bgp-llnh",
		"bgp-nlri-evpn",
		"bgp-nlri-flowspec",
		"bgp-nlri-labeled",
		"bgp-nlri-ls",
		"bgp-nlri-mup",
		"bgp-nlri-mvpn",
		"bgp-nlri-rtc",
		"bgp-nlri-srpolicy",
		"bgp-nlri-vpls",
		"bgp-nlri-vpn",
		"bgp-persist",
		"bgp-redistribute",
		"bgp-rib",
		"bgp-role",
		"bgp-route-refresh",
		"bgp-rpki",
		"bgp-rpki-decorator",
		"bgp-rr",
		"bgp-rs",
		"bgp-softver",
		"bgp-watchdog",
		"connected",
		"cos",
		"dhcpserver",
		"fib-kernel",
		"fib-p4",
		"fib-vpp",
		"firewall",
		"firewall-irr",
		"flow-export",
		"flowspec-firewall",
		"ike",
		"imageserver",
		"interface",
		"isis",
		"kernel",
		"l2tp-auth-local",
		"l2tp-auth-radius",
		"l2tp-pool",
		"l2tp-shaper",
		"ldp",
		"loop",
		"mrt",
		"ntp",
		"ospf",
		"policy-routes",
		"redistribute-orchestrator",
		"rib",
		"routing-table",
		"rsvp-te",
		"static",
		"sysctl",
		"tftpserver",
		"traffic",
		"vpp",
	}

	names := registry.Names()
	sort.Strings(names)

	// linux-only plugins (e.g. iface-dhcp) are excluded from the
	// cross-platform snapshot; TestPlatformPlugins covers them.
	platformOnly := map[string]bool{"iface-dhcp": true}

	var filtered []string
	for _, n := range names {
		if !platformOnly[n] {
			filtered = append(filtered, n)
		}
	}

	assertSnapshot(t, "plugin", filtered, expected)
}

// TestFilterTypeMappings snapshots the registered filter types.
//
// VALIDATES: Every expected policy filter type is registered.
// PREVENTS: Silent removal of a policy feature (e.g. prefix-list filtering).
func TestFilterTypeMappings(t *testing.T) {
	expected := map[string]string{
		"as-path-length":    "bgp-filter-aspath-length",
		"as-path-list":      "bgp-filter-aspath",
		"community-match":   "bgp-filter-community-match",
		"modify":            "bgp-filter-modify",
		"prefix-list":       "bgp-filter-prefix",
		"remove-private-as": "bgp-filter-remove-private-as",
	}

	fm := registry.FilterTypesMap()
	for ft, wantPlugin := range expected {
		if got := fm[ft]; got != wantPlugin {
			t.Errorf("FilterTypesMap[%q] = %q, want %q", ft, got, wantPlugin)
		}
	}

	for ft, plugin := range fm {
		if _, ok := expected[ft]; !ok {
			t.Errorf("unexpected filter type %q -> %q (add to expected map if intentional)", ft, plugin)
		}
	}
}

// TestRegisteredWireMethods snapshots the full set of RPC wire methods.
//
// VALIDATES: Every expected CLI/API command is registered after init().
// PREVENTS: Silent removal of a user-facing command (deleted handler, dropped register).
func TestRegisteredWireMethods(t *testing.T) {
	expected := []string{
		"ze-bfd-api:show-profile",
		"ze-bfd-api:show-session",
		"ze-bfd-api:show-sessions",
		"ze-bgp:cache-expire",
		"ze-bgp:cache-forward",
		"ze-bgp:cache-list",
		"ze-bgp:cache-release",
		"ze-bgp:cache-retain",
		"ze-bgp:command-complete",
		"ze-bgp:command-help",
		"ze-bgp:command-list",
		"ze-bgp:commit",
		"ze-bgp:event-list",
		"ze-bgp:help",
		"ze-bgp:log-levels",
		"ze-bgp:log-recent",
		"ze-bgp:log-set",
		"ze-bgp:metrics-list",
		"ze-bgp:metrics-values",
		"ze-bgp:monitor",
		"ze-bgp:peer-borr",
		"ze-bgp:peer-capabilities",
		"ze-bgp:peer-clear-soft",
		"ze-bgp:peer-detail",
		"ze-bgp:peer-eorr",
		"ze-bgp:peer-flush",
		"ze-bgp:peer-history",
		"ze-bgp:peer-list",
		"ze-bgp:peer-pause",
		"ze-bgp:peer-raw",
		"ze-bgp:peer-refresh",
		"ze-bgp:peer-resume",
		"ze-bgp:peer-rib",
		"ze-bgp:peer-statistics",
		"ze-bgp:peer-teardown",
		"ze-bgp:peer-update",
		"ze-bgp:plugin-ack",
		"ze-bgp:plugin-encoding",
		"ze-bgp:plugin-format",
		"ze-bgp:pool-stats",
		"ze-bgp:subscribe",
		"ze-bgp:summary",
		"ze-bgp:unsubscribe",
		"ze-clear:dns-cache",
		"ze-clear:interface-counters",
		"ze-clear:isis-adjacency",
		"ze-clear:isis-counters",
		"ze-clear:ospf-counters",
		"ze-clear:ospf-neighbor",
		"ze-clear:ospf-process",
		"ze-clear:vpn-ipsec-sa",
		"ze-config-archive:trigger",
		"ze-debug:debug-state",
		"ze-delete:bgp-peer",
		"ze-editor:mode-command",
		"ze-editor:mode-edit",
		"ze-event:monitor",
		"ze-iface:interface-addr-add",
		"ze-iface:interface-addr-del",
		"ze-iface:interface-create-bridge",
		"ze-iface:interface-create-dummy",
		"ze-iface:interface-create-veth",
		"ze-iface:interface-delete",
		"ze-iface:interface-down",
		"ze-iface:interface-mac",
		"ze-iface:interface-migrate",
		"ze-iface:interface-mtu",
		"ze-iface:interface-unit-add",
		"ze-iface:interface-unit-del",
		"ze-iface:interface-up",
		"ze-l2tp-api:config",
		"ze-l2tp-api:cqm",
		"ze-l2tp-api:echo",
		"ze-l2tp-api:listeners",
		"ze-l2tp-api:observer",
		"ze-l2tp-api:reliable",
		"ze-l2tp-api:session",
		"ze-l2tp-api:session-history",
		"ze-l2tp-api:session-teardown",
		"ze-l2tp-api:session-teardown-all",
		"ze-l2tp-api:session-traffic",
		"ze-l2tp-api:sessions",
		"ze-l2tp-api:statistics",
		"ze-l2tp-api:summary",
		"ze-l2tp-api:tunnel",
		"ze-l2tp-api:tunnel-history",
		"ze-l2tp-api:tunnel-teardown",
		"ze-l2tp-api:tunnel-teardown-all",
		"ze-l2tp-api:tunnels",
		"ze-monitor:interface-rate",
		"ze-monitor:ping",
		"ze-monitor:system-netlink",
		"ze-monitor:traceroute",
		"ze-monitor:vpn-ipsec",
		"ze-plugin:command-complete",
		"ze-plugin:command-help",
		"ze-plugin:command-list",
		"ze-plugin:help",
		"ze-plugin:session-bye",
		"ze-plugin:session-peer-ready",
		"ze-plugin:session-ping",
		"ze-plugin:session-ready",
		"ze-pppoe-api:interfaces",
		"ze-pppoe-api:session",
		"ze-pppoe-api:sessions",
		"ze-pppoe-api:statistics",
		"ze-pppoe-api:summary",
		"ze-resolve:cymru-asn-name",
		"ze-resolve:dns-a",
		"ze-resolve:dns-aaaa",
		"ze-resolve:dns-ptr",
		"ze-resolve:dns-txt",
		"ze-resolve:irr-expand",
		"ze-resolve:irr-prefix",
		"ze-resolve:peeringdb-as-set",
		"ze-resolve:peeringdb-max-prefix",
		"ze-resolve:ping",
		"ze-resolve:traceroute",
		"ze-rib-api:best",
		"ze-rib-api:best-status",
		"ze-rib-api:clear-in",
		"ze-rib-api:clear-out",
		"ze-rib-api:inject",
		"ze-rib-api:routes",
		"ze-rib-api:rpf",
		"ze-rib-api:status",
		"ze-rib-api:withdraw",
		"ze-set:system-file-descriptors",
		"ze-show:aaa-accounting",
		"ze-show:audit",
		"ze-show:bgp-health",
		"ze-show:bmp-collectors",
		"ze-show:bmp-peers",
		"ze-show:bmp-rib",
		"ze-show:bmp-sessions",
		"ze-show:capture",
		"ze-show:capture-interface",
		"ze-show:capture-raw",
		"ze-show:crashes",
		"ze-show:dns-cache",
		"ze-show:dns-lookup",
		"ze-show:doctor",
		"ze-show:errors",
		"ze-show:event-namespaces",
		"ze-show:event-recent",
		"ze-show:firewall-group",
		"ze-show:firewall-irr-prefix",
		"ze-show:firewall-irr-status",
		"ze-show:firewall-ruleset",
		"ze-show:flow-export",
		"ze-show:gnmi",
		"ze-show:health",
		"ze-show:host-all",
		"ze-show:host-cpu",
		"ze-show:host-dmi",
		"ze-show:host-kernel",
		"ze-show:host-memory",
		"ze-show:host-nic",
		"ze-show:host-platform",
		"ze-show:host-storage",
		"ze-show:host-thermal",
		"ze-show:irr-check",
		"ze-show:irr-prefix",
		"ze-show:irr-status",
		"ze-show:interface",
		"ze-show:interface-counters",
		"ze-show:interface-detail",
		"ze-show:interface-scan",
		"ze-show:ip-arp",
		"ze-show:ip-route",
		"ze-show:ospf",
		"ze-show:ospf-border-routers",
		"ze-show:ospf-database",
		"ze-show:ospf-database-asbr-summary",
		"ze-show:ospf-database-external",
		"ze-show:ospf-database-network",
		"ze-show:ospf-database-nssa-external",
		"ze-show:ospf-database-router",
		"ze-show:ospf-database-summary",
		"ze-show:ospf-interface",
		"ze-show:ospf-neighbor",
		"ze-show:ospf-route",
		"ze-show:ospf-spf",
		"ze-show:isis-database",
		"ze-show:isis-database-detail",
		"ze-show:isis-hostname",
		"ze-show:isis-interface",
		"ze-show:isis-neighbor",
		"ze-show:isis-route",
		"ze-show:isis-route-ipv6",
		"ze-show:isis-spf-log",
		"ze-show:kernel-routes",
		"ze-show:l2tp-health",
		"ze-show:ldp-binding",
		"ze-show:ldp-neighbor",
		"ze-show:metrics-query",
		"ze-show:mpls-forwarding",
		"ze-show:neighbors",
		"ze-show:ping",
		"ze-show:pki-certificate",
		"ze-show:pki-certificates",
		"ze-show:policy-chain",
		"ze-show:policy-list",
		"ze-show:policy-routes",
		"ze-show:policy-test",
		"ze-show:probe-round",
		"ze-show:route-lookup",
		"ze-show:rr-peers",
		"ze-show:rr-status",
		"ze-show:rsvp-te-fast-reroute",
		"ze-show:rsvp-te-interface",
		"ze-show:rsvp-te-lsp",
		"ze-show:rsvp-te-tunnel",
		"ze-show:static",
		"ze-show:storage-smart",
		"ze-show:system-conntrack",
		"ze-show:system-cpu",
		"ze-show:system-date",
		"ze-show:system-file-descriptors",
		"ze-show:system-goroutines",
		"ze-show:system-kernel-log",
		"ze-show:system-memory",
		"ze-show:system-memory-map",
		"ze-show:system-ntp",
		"ze-show:system-ntp-peers",
		"ze-show:system-platform",
		"ze-show:system-profile",
		"ze-show:system-sockets",
		"ze-show:system-subsystem-list",
		"ze-show:system-update",
		"ze-show:system-update-history",
		"ze-show:tcp-check",
		"ze-show:traceroute",
		"ze-show:traffic",
		"ze-show:uptime",
		"ze-show:version",
		"ze-show:vpn-ipsec-peer",
		"ze-show:vpn-ipsec-sa",
		"ze-show:vpn-ipsec-status",
		"ze-show:vpp-runtime",
		"ze-show:vpp-trace-clear",
		"ze-show:vpp-trace-show",
		"ze-show:vpp-trace-start",
		"ze-show:warnings",
		"ze-subscriber-api:detail",
		"ze-subscriber-api:summary",
		"ze-system:command-complete",
		"ze-system:command-help",
		"ze-system:command-list",
		"ze-system:daemon-quit",
		"ze-system:daemon-reboot",
		"ze-system:daemon-reload",
		"ze-system:daemon-shutdown",
		"ze-system:daemon-status",
		"ze-system:dispatch",
		"ze-system:help",
		"ze-system:subsystem-list",
		"ze-system:version-api",
		"ze-system:version-software",
		"ze-update:bgp-peer-prefix",
		"ze-update:firewall-irr-all",
		"ze-update:firewall-irr-as-set",
		"ze-update:firewall-irr-asn",
		"ze-update:irr-all",
		"ze-update:irr-as-set",
		"ze-update:irr-asn",
		"ze-update:system-firmware-apply",
		"ze-update:system-firmware-check",
		"ze-update:system-firmware-download",
		"ze-update:system-firmware-restart",
		"ze-update:system-firmware-rollback",
	}

	rpcs := pluginserver.AllBuiltinRPCs()
	var methods []string
	for _, r := range rpcs {
		methods = append(methods, r.WireMethod)
	}
	sort.Strings(methods)

	assertSnapshot(t, "wire method", methods, expected)
}

// TestYANGSchemaProviders snapshots the set of plugins that provide YANG schemas.
//
// VALIDATES: Every expected plugin provides a YANG schema.
// PREVENTS: Silent removal of configuration surface (deleted YANG field).
func TestYANGSchemaProviders(t *testing.T) {
	expected := []string{
		"bfd",
		"bgp",
		"bgp-adj-rib-in",
		"bgp-bmp",
		"bgp-filter-aspath",
		"bgp-filter-aspath-length",
		"bgp-filter-community",
		"bgp-filter-community-match",
		"bgp-filter-irr",
		"bgp-filter-modify",
		"bgp-filter-prefix",
		"bgp-filter-remove-private-as",
		"bgp-gr",
		"bgp-healthcheck",
		"bgp-hostname",
		"bgp-llnh",
		"bgp-rib",
		"bgp-role",
		"bgp-route-refresh",
		"bgp-rpki",
		"bgp-rpki-decorator",
		"bgp-rs",
		"bgp-softver",
		"connected",
		"cos",
		"dhcpserver",
		"fib-kernel",
		"fib-p4",
		"fib-vpp",
		"firewall",
		"firewall-irr",
		"flow-export",
		"imageserver",
		"interface",
		"isis",
		"kernel",
		"l2tp-auth-local",
		"l2tp-auth-radius",
		"l2tp-pool",
		"l2tp-shaper",
		"ldp",
		"mrt",
		"ntp",
		"ospf",
		"policy-routes",
		"rib",
		"routing-table",
		"rsvp-te",
		"static",
		"sysctl",
		"tftpserver",
		"traffic",
		"vpp",
	}

	schemas := registry.YANGSchemas()
	var names []string
	for n := range schemas {
		names = append(names, n)
	}
	sort.Strings(names)

	assertSnapshot(t, "YANG provider", names, expected)
}

// TestGeneratedPluginImportsCurrent verifies that the generated blank-import
// file matches register.go discovery.
//
// VALIDATES: plugin/all generation is checked by the same generator that writes it.
// PREVENTS: Missing plugin registration when a register.go package is not imported.
func TestGeneratedPluginImportsCurrent(t *testing.T) {
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "go", "run", "../../../../scripts/codegen/plugin_imports.go", "--check")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin/all generated imports are stale: %v\n%s", err, out)
	}
}

// TestAllPluginsRegistered verifies that importing the all package populates
// the production plugin registry.
//
// VALIDATES: Production plugin aggregation registers plugins from plugin/all.
// VALIDATES: Production plugin aggregation excludes internal/test/plugins.
// PREVENTS: Shipping test scaffolding in cmd/ze.
func TestAllPluginsRegistered(t *testing.T) {
	names := registry.Names()
	if len(names) == 0 {
		t.Fatal("plugin/all registered no plugins")
	}

	for _, testOnly := range []string{"fakel2tp", "fakeredist"} {
		if slices.Contains(names, testOnly) {
			t.Errorf("test-only plugin %q registered in production plugin/all", testOnly)
		}
	}
}

// TestPlatformPlugins verifies platform-gated plugin registration.
//
// VALIDATES: Linux-only plugins register only on Linux builds.
// PREVENTS: Accidental non-Linux registration of Linux-only plugins.
func TestPlatformPlugins(t *testing.T) {
	names := registry.Names()
	hasIfaceDHCP := slices.Contains(names, "iface-dhcp")
	if runtime.GOOS == "linux" {
		if !hasIfaceDHCP {
			t.Error("linux-only plugin \"iface-dhcp\" not registered on linux")
		}
		return
	}
	if hasIfaceDHCP {
		t.Errorf("linux-only plugin %q registered on %s", "iface-dhcp", runtime.GOOS)
	}
}

// TestAllPluginsHaveRunEngine verifies that every registered plugin has a RunEngine handler.
//
// VALIDATES: No plugin was registered without an engine handler.
// PREVENTS: Nil pointer dereference when starting plugin in engine mode.
func TestAllPluginsHaveRunEngine(t *testing.T) {
	for _, reg := range registry.All() {
		if reg.RunEngine == nil {
			t.Errorf("plugin %q has nil RunEngine", reg.Name)
		}
	}
}

// TestAllPluginsHaveCLIHandler verifies that every registered plugin has a CLI handler.
//
// VALIDATES: No plugin was registered without a CLI handler.
// PREVENTS: Nil pointer dereference when dispatching CLI command.
func TestAllPluginsHaveCLIHandler(t *testing.T) {
	for _, reg := range registry.All() {
		if reg.CLIHandler == nil {
			t.Errorf("plugin %q has nil CLIHandler", reg.Name)
		}
	}
}

// TestAllPluginsHaveDescription verifies that every registered plugin has a description.
//
// VALIDATES: Help text will have descriptions for all plugins.
// PREVENTS: Empty lines in `ze plugin help` output.
func TestAllPluginsHaveDescription(t *testing.T) {
	for _, reg := range registry.All() {
		if reg.Description == "" {
			t.Errorf("plugin %q has empty Description", reg.Name)
		}
	}
}

// TestFamilyMappings verifies that expected families are mapped to plugins.
//
// VALIDATES: Family-to-plugin mapping works after init() registration.
// PREVENTS: Broken auto-discovery when a family plugin is configured.
func TestFamilyMappings(t *testing.T) {
	fm := registry.FamilyMap()

	expected := map[string]string{
		"ipv4/flow":         "bgp-nlri-flowspec",
		"ipv6/flow":         "bgp-nlri-flowspec",
		"ipv4/flow-vpn":     "bgp-nlri-flowspec",
		"ipv6/flow-vpn":     "bgp-nlri-flowspec",
		"l2vpn/evpn":        "bgp-nlri-evpn",
		"ipv4/mpls-vpn":     "bgp-nlri-vpn",
		"ipv6/mpls-vpn":     "bgp-nlri-vpn",
		"bgp-ls/bgp-ls":     "bgp-nlri-ls",
		"bgp-ls/bgp-ls-vpn": "bgp-nlri-ls",
	}

	for fam, wantPlugin := range expected {
		if got := fm[fam]; got != wantPlugin {
			t.Errorf("FamilyMap[%q] = %q, want %q", fam, got, wantPlugin)
		}
	}
}

// TestBgpRSDependsOnAdjRibIn verifies bgp-rs still declares its relationship
// with bgp-adj-rib-in after the spec-rs-fastpath-2-adjrib soft-dep refactor.
//
// VALIDATES: bgp-rs has OptionalDependencies containing "bgp-adj-rib-in".
// PREVENTS: accidental removal of the relationship -- which would let bgp-rs
// silently start without the replay-on-peer-up capability and without the
// soft-dep resolver pulling adj-rib-in in when it is registered.
func TestBgpRSDependsOnAdjRibIn(t *testing.T) {
	reg := registry.Lookup("bgp-rs")
	if reg == nil {
		t.Fatal("bgp-rs not registered")
		return
	}

	if slices.Contains(reg.Dependencies, "bgp-adj-rib-in") {
		t.Errorf("bgp-rs Dependencies=%v must NOT contain bgp-adj-rib-in (moved to OptionalDependencies by spec-rs-fastpath-2-adjrib)", reg.Dependencies)
	}
	if !slices.Contains(reg.OptionalDependencies, "bgp-adj-rib-in") {
		t.Errorf("bgp-rs OptionalDependencies=%v, want to contain bgp-adj-rib-in", reg.OptionalDependencies)
	}
}

// TestPolicyRoutesDependsOnFirewall verifies policy-routes startup is ordered
// after the firewall plugin that owns nftables apply/reconcile.
//
// VALIDATES: policy-routes has Dependencies containing "firewall".
// PREVENTS: policy-routes applying firewall tables before firewall startup.
func TestPolicyRoutesDependsOnFirewall(t *testing.T) {
	reg := registry.Lookup("policy-routes")
	if reg == nil {
		t.Fatal("policy-routes not registered")
		return
	}

	if !slices.Contains(reg.Dependencies, "firewall") {
		t.Errorf("policy-routes Dependencies=%v, want to contain firewall", reg.Dependencies)
	}
}

// TestCapabilityMappings verifies that capability codes are mapped to plugins.
//
// VALIDATES: Capability-to-plugin mapping works after init() registration.
// PREVENTS: Broken capability decode in OPEN message handling.
func TestCapabilityMappings(t *testing.T) {
	cm := registry.CapabilityMap()

	if cm[64] != "bgp-gr" {
		t.Errorf("CapabilityMap[64] = %q, want bgp-gr", cm[64])
	}
	if cm[73] != "bgp-hostname" {
		t.Errorf("CapabilityMap[73] = %q, want bgp-hostname", cm[73])
	}
	if cm[75] != "bgp-softver" {
		t.Errorf("CapabilityMap[75] = %q, want bgp-softver", cm[75])
	}
	if cm[77] != "bgp-llnh" {
		t.Errorf("CapabilityMap[77] = %q, want bgp-llnh", cm[77])
	}
	if cm[9] != "bgp-role" {
		t.Errorf("CapabilityMap[9] = %q, want bgp-role", cm[9])
	}
	if cm[2] != "bgp-route-refresh" {
		t.Errorf("CapabilityMap[2] = %q, want bgp-route-refresh", cm[2])
	}
	if cm[70] != "bgp-route-refresh" {
		t.Errorf("CapabilityMap[70] = %q, want bgp-route-refresh", cm[70])
	}
}
