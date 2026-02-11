package all

import (
	"sort"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
)

// TestAllPluginsRegistered verifies that importing the all package
// registers all 10 expected internal plugins.
//
// VALIDATES: Every internal plugin registers via init().
// PREVENTS: Missing plugin registration when a register.go is forgotten.
func TestAllPluginsRegistered(t *testing.T) {
	expected := []string{
		"bgpls",
		"evpn",
		"flowspec",
		"gr",
		"hostname",
		"llnh",
		"rib",
		"role",
		"rr",
		"vpn",
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
// PREVENTS: Empty lines in `ze bgp plugin help` output.
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
		"ipv4/flow":         "flowspec",
		"ipv6/flow":         "flowspec",
		"ipv4/flow-vpn":     "flowspec",
		"ipv6/flow-vpn":     "flowspec",
		"l2vpn/evpn":        "evpn",
		"ipv4/vpn":          "vpn",
		"ipv6/vpn":          "vpn",
		"bgp-ls/bgp-ls":     "bgpls",
		"bgp-ls/bgp-ls-vpn": "bgpls",
	}

	for family, wantPlugin := range expected {
		if got := fm[family]; got != wantPlugin {
			t.Errorf("FamilyMap[%q] = %q, want %q", family, got, wantPlugin)
		}
	}
}

// TestCapabilityMappings verifies that capability codes are mapped to plugins.
//
// VALIDATES: Capability-to-plugin mapping works after init() registration.
// PREVENTS: Broken capability decode in OPEN message handling.
func TestCapabilityMappings(t *testing.T) {
	cm := registry.CapabilityMap()

	if cm[73] != "hostname" {
		t.Errorf("CapabilityMap[73] = %q, want hostname", cm[73])
	}
	if cm[77] != "llnh" {
		t.Errorf("CapabilityMap[77] = %q, want llnh", cm[77])
	}
	if cm[9] != "role" {
		t.Errorf("CapabilityMap[9] = %q, want role", cm[9])
	}
}
