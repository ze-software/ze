package all

import (
	"context"
	"os/exec"
	"runtime"
	"slices"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

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
