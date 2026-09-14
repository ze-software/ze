//go:build linux

// Design: docs/architecture/core-design.md -- the readiness check the nft backend owns
// Detail: doctor_linux.go -- checkFirewallNftables and its registration

package firewallnft

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withLoadedModules stands in a kernel whose module list is exactly names.
func withLoadedModules(t *testing.T, names ...string) {
	t.Helper()
	previous := nftLoadedModules
	loaded := make(map[string]bool, len(names))
	for _, name := range names {
		loaded[name] = true
	}
	nftLoadedModules = func() map[string]bool { return loaded }
	t.Cleanup(func() { nftLoadedModules = previous })
}

// firewallTree builds the smallest config that carries a firewall block, on
// the named backend or on the default when backend is empty.
func firewallTree(backend string) *config.Tree {
	tree := config.NewTree()
	firewall := tree.GetOrCreateContainer("firewall")
	if backend != "" {
		firewall.Set("backend", backend)
	}
	return tree
}

// TestCheckFirewallNftablesWarnsWithoutTheModule drives the check over a
// firewall config on a kernel that has not loaded nf_tables, on the default
// backend and on the named one.
//
// VALIDATES: doctor-firewall-nftables at warning severity in both cases.
// PREVENTS: nftables kernel support gaps being missed until firewall apply.
func TestCheckFirewallNftablesWarnsWithoutTheModule(t *testing.T) {
	withLoadedModules(t)

	for _, backend := range []string{"", "nft"} {
		diags := checkFirewallNftables(diagnostic.DoctorCheckContext{Tree: firewallTree(backend)})
		if len(diags) != 1 {
			t.Fatalf("backend %q: diagnostics = %d, want 1: %+v", backend, len(diags), diags)
		}
		if diags[0].Code != codeFirewallNftables || diags[0].Severity != diagnostic.SeverityWarning {
			t.Fatalf("backend %q: diagnostic = %s/%s, want %s at warning", backend, diags[0].Code, diags[0].Severity, codeFirewallNftables)
		}
	}
}

// TestCheckFirewallNftablesIsSilentOffThisBackend is the negative half: a
// loaded module, another backend, no firewall block, and no config each name
// no dependency on nf_tables.
//
// VALIDATES: no diagnostic in each case.
// PREVENTS: an nf_tables warning on a box programmed through VPP, or on one
// whose kernel already carries the module.
func TestCheckFirewallNftablesIsSilentOffThisBackend(t *testing.T) {
	withLoadedModules(t, nftModule)
	if diags := checkFirewallNftables(diagnostic.DoctorCheckContext{Tree: firewallTree("")}); len(diags) != 0 {
		t.Fatalf("module loaded: diagnostics = %d, want 0", len(diags))
	}

	withLoadedModules(t)
	if diags := checkFirewallNftables(diagnostic.DoctorCheckContext{Tree: firewallTree("vpp")}); len(diags) != 0 {
		t.Fatalf("vpp backend: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkFirewallNftables(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("no firewall block: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkFirewallNftables(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}
}

// TestNftDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this backend's check, at the phase and order the
// declaration states, and whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register_linux.go installed the check and the
// registry accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestNftDoctorCheckRegistered(t *testing.T) {
	want := nftDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
	}
	if found.Order != want.Order {
		t.Fatalf("order = %d, want %d", found.Order, want.Order)
	}
	if found.Component != want.Component {
		t.Fatalf("component = %q, want %q", found.Component, want.Component)
	}

	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(codeFirewallNftables) == nil {
		t.Fatalf("diagnostic code %q is not registered", codeFirewallNftables)
	}
}
