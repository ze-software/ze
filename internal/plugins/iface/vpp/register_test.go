// VPP interface register: init() side-effects and backend conformance -- the
// "vpp" backend registered into iface's registry, the show-vpp wire methods
// registered with the plugin server, the registered doctor checks, the health
// check, and the compile-time iface.Backend interface satisfaction.
package ifacevpp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
	"codeberg.org/thomas-mangin/ze/internal/core/health"
)

func TestShowVPP_RegisteredWireMethods(t *testing.T) {
	wanted := map[string]bool{
		"ze-show:vpp-trace-start": false,
		"ze-show:vpp-trace-show":  false,
		"ze-show:vpp-trace-clear": false,
		"ze-show:vpp-runtime":     false,
	}
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if _, ok := wanted[r.WireMethod]; ok {
			require.NotNil(t, r.Handler, "%s handler must not be nil", r.WireMethod)
			wanted[r.WireMethod] = true
		}
	}
	for wm, seen := range wanted {
		require.True(t, seen, "%s not registered via pluginserver.RegisterRPCs", wm)
	}
}

func TestVPPHealthCheckSocketMissing(t *testing.T) {
	status, _ := checkVPPHealth()
	assert.Equal(t, health.StatusHealthy, status, "healthy when VPP socket does not exist")
}

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

// TestVPPBackendImplementsInterface verifies compile-time interface compliance.
// VALIDATES: AC-1 -- Backend "vpp" implements all methods
// PREVENTS: missing method causing compile error at integration time.
func TestVPPBackendImplementsInterface(t *testing.T) {
	// Compile-time check: vppBackendImpl implements iface.Backend.
	var _ iface.Backend = (*vppBackendImpl)(nil)
}

// sentinelBackend is a non-nil factory used only as the duplicate-registration
// probe in TestVPPBackendRegistered; it must never actually run.
func sentinelBackend() (iface.Backend, error) {
	return nil, errors.New("ifacevpp test sentinel: real backend was NOT registered at init")
}

// TestVPPBackendRegistered verifies init() (register.go) wired the "vpp" iface
// backend into iface's registry: RegisterBackend rejects duplicates, so a probe
// re-registration must fail. If it succeeds, the real backend was not registered.
// PREVENTS: a silently-unregistered VPP backend the composition root cannot load.
func TestVPPBackendRegistered(t *testing.T) {
	if err := iface.RegisterBackend("vpp", sentinelBackend); err == nil {
		t.Fatal("expected duplicate-registration error, got nil (backend not registered at init?)")
	}
}
