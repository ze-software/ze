package ifacevpp

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func vppWireguardTree(backend string, pluginEnabled bool) *config.Tree {
	tree := config.NewTree()
	ifaceTree := tree.GetOrCreateContainer("interface")
	ifaceTree.Set("backend", backend)
	wg := config.NewTree()
	wg.Set("name", "wg0")
	ifaceTree.AddListEntry("wireguard", "wg0", wg)
	if pluginEnabled {
		plugins := tree.GetOrCreateContainer("vpp").GetOrCreateContainer("plugins")
		plugins.Set("wireguard", "true")
	}
	return tree
}

// TestDoctorWireguardPluginMissing verifies AC-8: a wireguard interface under
// backend vpp with the plugin toggle off emits doctor-vpp-wireguard.
func TestDoctorWireguardPluginMissing(t *testing.T) {
	diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: vppWireguardTree("vpp", false)})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-vpp-wireguard" {
		t.Errorf("code = %q, want doctor-vpp-wireguard", diags[0].Code)
	}
}

// TestDoctorWireguardPluginEnabled verifies no warning when the toggle is on.
func TestDoctorWireguardPluginEnabled(t *testing.T) {
	diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: vppWireguardTree("vpp", true)})
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics with plugin enabled, got %d", len(diags))
	}
}

// TestDoctorWireguardNetlinkBackend verifies the check is scoped to the vpp
// backend: a wireguard interface under netlink does not warn.
func TestDoctorWireguardNetlinkBackend(t *testing.T) {
	diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: vppWireguardTree("netlink", false)})
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics under netlink backend, got %d", len(diags))
	}
}

// TestDoctorWireguardNoInterface verifies no warning when no wireguard interface
// is configured, and a nil tree is tolerated.
func TestDoctorWireguardNoInterface(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("interface").Set("backend", "vpp")
	if diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: tree}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics without a wireguard interface, got %d", len(diags))
	}
	if diags := checkVPPWireguardPlugin(diagnostic.DoctorCheckContext{Tree: nil}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for nil tree, got %d", len(diags))
	}
}

func vppLCPTree(bgp bool, netns string, lcpEnabled bool) *config.Tree {
	tree := config.NewTree()
	if bgp {
		tree.GetOrCreateContainer("bgp").Set("router-id", "10.0.0.1")
	}
	lcp := tree.GetOrCreateContainer("vpp").GetOrCreateContainer("lcp")
	if !lcpEnabled {
		lcp.Set("enabled", "false")
	}
	if netns != "" {
		lcp.Set("netns", netns)
	}
	return tree
}

// TestDoctorLCPNetnsIsolated verifies A-4/AC-8: BGP enabled + LCP in an isolated
// netns (the default "dataplane") emits doctor-vpp-lcp-netns.
func TestDoctorLCPNetnsIsolated(t *testing.T) {
	diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "dataplane", true)})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-vpp-lcp-netns" {
		t.Errorf("code = %q, want doctor-vpp-lcp-netns", diags[0].Code)
	}
}

// TestDoctorLCPNetnsDefaultIsolated verifies the omitted-netns case uses the
// YANG default "dataplane" and still warns.
func TestDoctorLCPNetnsDefaultIsolated(t *testing.T) {
	diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "", true)})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for default netns, got %d", len(diags))
	}
}

// TestDoctorLCPNetnsRootReachable verifies no warning when netns is host/root.
func TestDoctorLCPNetnsRootReachable(t *testing.T) {
	for _, ns := range []string{"host", "root"} {
		if diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, ns, true)}); len(diags) != 0 {
			t.Errorf("netns %q: expected 0 diagnostics, got %d", ns, len(diags))
		}
	}
}

// TestDoctorLCPNetnsNoBGP verifies no warning when BGP is not configured (the
// constraint only matters for the BGP-bind goal).
func TestDoctorLCPNetnsNoBGP(t *testing.T) {
	if diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(false, "dataplane", true)}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics without bgp, got %d", len(diags))
	}
}

// TestDoctorLCPNetnsDisabled verifies no warning when LCP is disabled.
func TestDoctorLCPNetnsDisabled(t *testing.T) {
	if diags := checkVPPLCPNetns(diagnostic.DoctorCheckContext{Tree: vppLCPTree(true, "dataplane", false)}); len(diags) != 0 {
		t.Errorf("expected 0 diagnostics with lcp disabled, got %d", len(diags))
	}
}
