package all

import (
	"slices"
	"sort"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// TestAllPluginsRegistered verifies that importing the all package
// registers all 26 expected internal plugins.
//
// VALIDATES: Every internal plugin registers via init().
// PREVENTS: Missing plugin registration when a register.go is forgotten.
func TestAllPluginsRegistered(t *testing.T) {
	expected := []string{
		"bgp-adj-rib-in",
		"bgp-aigp",
		"bgp-gr",
		"bgp-hostname",
		"bgp-llnh",
		"bgp-nlri-evpn",
		"bgp-nlri-flowspec",
		"bgp-nlri-labeled",
		"bgp-nlri-ls",
		"bgp-nlri-mup",
		"bgp-nlri-mvpn",
		"bgp-nlri-rtc",
		"bgp-nlri-vpls",
		"bgp-nlri-vpn",
		"bgp-persist",
		"bgp-rib",
		"bgp-route-refresh",
		"bgp-rpki",
		"bgp-rpki-decorator",
		"bgp-rs",
		"bgp-softver",
		"bgp-watchdog",
		"filter-community",
		"iface",
		"loop",
		"role",
	}

	names := registry.Names()
	sort.Strings(names)

	if len(names) != len(expected) {
		t.Fatalf("expected %d plugins, got %d: %v", len(expected), len(names), names)
	}

	for i, want := range expected {
		if names[i] != want {
			t.Errorf("plugin[%d] = %q, want %q", i, names[i], want)
		}
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
		"ipv4/vpn":          "bgp-nlri-vpn",
		"ipv6/vpn":          "bgp-nlri-vpn",
		"bgp-ls/bgp-ls":     "bgp-nlri-ls",
		"bgp-ls/bgp-ls-vpn": "bgp-nlri-ls",
	}

	for family, wantPlugin := range expected {
		if got := fm[family]; got != wantPlugin {
			t.Errorf("FamilyMap[%q] = %q, want %q", family, got, wantPlugin)
		}
	}
}

// TestBgpRSDependsOnAdjRibIn verifies bgp-rs declares its dependency.
//
// VALIDATES: bgp-rs has Dependencies containing "bgp-adj-rib-in".
// PREVENTS: bgp-rs starting without adj-rib-in, causing silent replay failure.
func TestBgpRSDependsOnAdjRibIn(t *testing.T) {
	reg := registry.Lookup("bgp-rs")
	if reg == nil {
		t.Fatal("bgp-rs not registered")
		return
	}

	if !slices.Contains(reg.Dependencies, "bgp-adj-rib-in") {
		t.Errorf("bgp-rs Dependencies=%v, want to contain bgp-adj-rib-in", reg.Dependencies)
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
	if cm[9] != "role" {
		t.Errorf("CapabilityMap[9] = %q, want role", cm[9])
	}
	if cm[2] != "bgp-route-refresh" {
		t.Errorf("CapabilityMap[2] = %q, want bgp-route-refresh", cm[2])
	}
	if cm[70] != "bgp-route-refresh" {
		t.Errorf("CapabilityMap[70] = %q, want bgp-route-refresh", cm[70])
	}
}
