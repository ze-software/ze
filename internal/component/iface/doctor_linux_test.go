//go:build linux

// Design: docs/architecture/iface/logical-name-resolution.md -- the readiness check this component owns
// Detail: doctor_linux.go -- checkEthernetInterfaces and its registration

package iface

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// fakeSysClassNet builds a sysfs-shaped tree where each named device carries
// the given MAC and an "up" operstate, points the interface check at it, and
// returns its root so a test can add the links between devices.
func fakeSysClassNet(t *testing.T, devices map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for device, mac := range devices {
		dir := filepath.Join(root, device)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "address"), []byte(mac+"\n"), 0o600); err != nil {
			t.Fatalf("write address: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "operstate"), []byte("up\n"), 0o600); err != nil {
			t.Fatalf("write operstate: %v", err)
		}
	}
	old := doctorSysClassNet
	doctorSysClassNet = root
	t.Cleanup(func() { doctorSysClassNet = old })
	return root
}

// sysClassNetStack writes the links the kernel writes when upper stands on
// lower: a lower_<lower> link in the upper device's directory, and an
// upper_<upper> link in the lower one's. It covers both relations the hardware
// selector must skip, a vlan or macvlan on a parent and a bridge or bond on a
// member, because sysfs spells them the same way.
func sysClassNetStack(t *testing.T, root, upper, lower string) {
	t.Helper()
	sysClassNetLink(t, filepath.Join(root, upper, "lower_"+lower), filepath.Join("..", lower))
	sysClassNetLink(t, filepath.Join(root, lower, "upper_"+upper), filepath.Join("..", upper))
}

// sysClassNetLink creates one symlink of the fake sysfs tree, failing the test
// rather than leaving a fixture that says something other than what the test
// asked for.
func sysClassNetLink(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Symlink(to, from); err != nil {
		t.Fatalf("symlink %s -> %s: %v", from, to, err)
	}
}

// ethernetTree returns a config tree holding one netlink-backed ethernet entry.
func ethernetTree(name string, leaves, mac map[string]string) *config.Tree {
	tree := config.NewTree()
	ifaceBlock := tree.GetOrCreateContainer(configRootInterface)
	ifaceBlock.Set("backend", "netlink")
	entry := config.NewTree()
	for k, v := range leaves {
		entry.Set(k, v)
	}
	if len(mac) > 0 {
		macBlock := entry.GetOrCreateContainer("mac")
		for k, v := range mac {
			macBlock.Set(k, v)
		}
	}
	ifaceBlock.AddListEntry("ethernet", name, entry)
	return tree
}

// ethernetDiags drives the check over tree through the context the runner
// hands it.
func ethernetDiags(tree *config.Tree) []diagnostic.Diagnostic {
	return checkEthernetInterfaces(diagnostic.DoctorCheckContext{Tree: tree})
}

func requireDiag(t *testing.T, diags []diagnostic.Diagnostic, code string, severity diagnostic.Severity) {
	t.Helper()
	for i := range diags {
		if diags[i].Code == code {
			if diags[i].Severity != severity {
				t.Fatalf("severity for %s = %q, want %q", code, diags[i].Severity, severity)
			}
			return
		}
	}
	t.Fatalf("expected %s in %+v", code, diags)
}

func assertNoDiagCode(t *testing.T, diags []diagnostic.Diagnostic, code string) {
	t.Helper()
	for i := range diags {
		if diags[i].Code == code {
			t.Fatalf("unexpected diagnostic: %+v", diags[i])
		}
	}
}

// TestCheckEthernetInterfacesFollowsOSNameAlias verifies the check judges an
// aliased entry by the device the alias names, not by the entry name.
//
// VALIDATES: spec-fixit-iface-selector-ignored-by-apply, the doctor surface.
// PREVENTS: `ze doctor` calling a correct os-name config a missing interface,
// which is the same name-versus-device confusion the config apply path had.
func TestCheckEthernetInterfacesFollowsOSNameAlias(t *testing.T) {
	fakeSysClassNet(t, map[string]string{"enp1s0": "aa:bb:cc:00:00:01"})

	tree := ethernetTree("wan", map[string]string{"os-name": "enp1s0"}, nil)

	assertNoDiagCode(t, ethernetDiags(tree), codeIfaceMissing)
}

// TestCheckEthernetInterfacesFollowsMACMatch verifies the check resolves a
// mac/match selector to the device carrying the address as its own, and
// reports the two selector verdicts the config apply path distinguishes: an
// unmatched selector defers (warning), an ambiguous one is refused (error). A
// device wearing another device's address is not a second answer, so the last
// two cases fix the population the check counts to the one the apply path
// counts.
//
// VALIDATES: spec-fixit-iface-selector-ignored-by-apply, the doctor surface.
// PREVENTS: doctor staying silent about a selector that leaves an interface
// unconfigured, calling a bound one missing, and calling a bridged or
// VLAN-bearing port ambiguous while the daemon binds to it.
func TestCheckEthernetInterfacesFollowsMACMatch(t *testing.T) {
	const wanted = "aa:bb:cc:00:00:01"

	t.Run("one device carries it", func(t *testing.T) {
		fakeSysClassNet(t, map[string]string{"enp1s0": wanted, "enp2s0": "aa:bb:cc:00:00:02"})
		tree := ethernetTree("wan", nil, map[string]string{"match": wanted})
		diags := ethernetDiags(tree)
		assertNoDiagCode(t, diags, codeIfaceMissing)
		assertNoDiagCode(t, diags, diagnostic.CodeDoctorIfaceSelectorUnmatched)
		assertNoDiagCode(t, diags, diagnostic.CodeDoctorIfaceSelectorAmbiguous)
	})

	t.Run("no device carries it", func(t *testing.T) {
		// A device named after the entry exists, so a name-based check would
		// report nothing at all while the interface stays unconfigured.
		fakeSysClassNet(t, map[string]string{"wan": "aa:bb:cc:00:00:02"})
		tree := ethernetTree("wan", nil, map[string]string{"match": wanted})
		requireDiag(t, ethernetDiags(tree), diagnostic.CodeDoctorIfaceSelectorUnmatched, diagnostic.SeverityWarning)
	})

	t.Run("two devices carry it", func(t *testing.T) {
		fakeSysClassNet(t, map[string]string{"enp1s0": wanted, "enp2s0": wanted})
		tree := ethernetTree("wan", nil, map[string]string{"match": wanted})
		requireDiag(t, ethernetDiags(tree), diagnostic.CodeDoctorIfaceSelectorAmbiguous, diagnostic.SeverityError)
	})

	t.Run("a bridge wears its member's address", func(t *testing.T) {
		// The config ze itself creates: the selected port is a bridge member,
		// so the bridge reports the port's address and two devices answer to
		// the selector. The apply path binds to the port
		// (test/plugin/iface-bridge-mac-match-apply.ci), so a doctor calling
		// this ambiguous is wrong about a working box.
		root := fakeSysClassNet(t, map[string]string{"enp1s0": wanted, "zebr0": wanted})
		sysClassNetStack(t, root, "zebr0", "enp1s0")

		tree := ethernetTree("wan", nil, map[string]string{"match": wanted})

		diags := ethernetDiags(tree)
		assertNoDiagCode(t, diags, diagnostic.CodeDoctorIfaceSelectorAmbiguous)
		assertNoDiagCode(t, diags, diagnostic.CodeDoctorIfaceSelectorUnmatched)
		assertNoDiagCode(t, diags, codeIfaceMissing)
	})

	t.Run("a vlan wears its parent's address", func(t *testing.T) {
		// The other relation the apply path skips: a vlan inherits the parent's
		// hardware address, so one vlan unit on the entry makes its own
		// selector look ambiguous.
		root := fakeSysClassNet(t, map[string]string{"enp1s0": wanted, "enp1s0.100": wanted})
		sysClassNetStack(t, root, "enp1s0.100", "enp1s0")

		tree := ethernetTree("wan", nil, map[string]string{"match": wanted})

		diags := ethernetDiags(tree)
		assertNoDiagCode(t, diags, diagnostic.CodeDoctorIfaceSelectorAmbiguous)
		assertNoDiagCode(t, diags, diagnostic.CodeDoctorIfaceSelectorUnmatched)
		assertNoDiagCode(t, diags, codeIfaceMissing)
	})
}

// TestCheckEthernetInterfacesUnselectedEntryUnchanged pins the behavior of an
// entry with no selector: its name IS its device, and an absent one is still
// an error.
//
// VALIDATES: the selector work changes nothing for a plain entry.
// PREVENTS: the selector branches swallowing the missing-interface report.
func TestCheckEthernetInterfacesUnselectedEntryUnchanged(t *testing.T) {
	fakeSysClassNet(t, map[string]string{"enp1s0": "aa:bb:cc:00:00:01"})

	tree := ethernetTree("eth9", nil, nil)

	requireDiag(t, ethernetDiags(tree), codeIfaceMissing, diagnostic.SeverityError)
}

// TestCheckEthernetInterfacesReportsADownDevice covers the operstate verdict:
// a device that exists and is not up.
//
// VALIDATES: doctor-iface-down at warning severity, naming the state read.
// PREVENTS: a configured port that is administratively down passing readiness
// in silence.
func TestCheckEthernetInterfacesReportsADownDevice(t *testing.T) {
	root := fakeSysClassNet(t, map[string]string{"enp1s0": "aa:bb:cc:00:00:01"})
	if err := os.WriteFile(filepath.Join(root, "enp1s0", "operstate"), []byte("down\n"), 0o600); err != nil {
		t.Fatalf("write operstate: %v", err)
	}

	requireDiag(t, ethernetDiags(ethernetTree("enp1s0", nil, nil)), codeIfaceDown, diagnostic.SeverityWarning)
}

// TestCheckEthernetInterfacesMACOverrideBoundByName covers the entry shape
// that writes one NIC's address onto another. `mac { address }` is applied to
// whichever device the entry resolves to, so an entry that reaches its device
// by NAME hands that address to a different port the first time the kernel
// renames one. The check reads the config alone: after an apply the device
// already carries the configured address, so comparing the two answers
// nothing.
//
// VALIDATES: the reporting half of the never-guess-a-NIC work.
// PREVENTS: doctor staying silent about a name-bound MAC override, the shape
// `ze init` wrote for every discovered NIC until 2026-09-02.
func TestCheckEthernetInterfacesMACOverrideBoundByName(t *testing.T) {
	const address = "aa:bb:cc:00:00:01"

	t.Run("override bound by the entry name", func(t *testing.T) {
		fakeSysClassNet(t, map[string]string{"wan": address})
		tree := ethernetTree("wan", nil, map[string]string{"address": address})
		requireDiag(t, ethernetDiags(tree), codeIfaceMACOverrideByName, diagnostic.SeverityWarning)
	})

	t.Run("override bound by os-name", func(t *testing.T) {
		// An os-name alias is still a name. It reaches whichever device holds
		// that kernel name, so the exposure is the same.
		fakeSysClassNet(t, map[string]string{"enp1s0": address})
		tree := ethernetTree("wan", map[string]string{"os-name": "enp1s0"}, map[string]string{"address": address})
		requireDiag(t, ethernetDiags(tree), codeIfaceMACOverrideByName, diagnostic.SeverityWarning)
	})

	t.Run("override bound by mac match", func(t *testing.T) {
		// The override follows the NIC, which is the fix the warning asks for.
		fakeSysClassNet(t, map[string]string{"enp1s0": "aa:bb:cc:00:00:02"})
		tree := ethernetTree("wan", nil, map[string]string{"match": "aa:bb:cc:00:00:02", "address": address})
		assertNoDiagCode(t, ethernetDiags(tree), codeIfaceMACOverrideByName)
	})

	t.Run("no override at all", func(t *testing.T) {
		fakeSysClassNet(t, map[string]string{"wan": address})
		tree := ethernetTree("wan", nil, nil)
		assertNoDiagCode(t, ethernetDiags(tree), codeIfaceMACOverrideByName)
	})
}

// TestCheckEthernetInterfacesIsSilentOffTheNetlinkBackend is the negative
// half: a VPP backend, an interface block with no ethernet, no interface
// block, and no config each name no sysfs device.
//
// VALIDATES: no diagnostic in each case, on a sysfs that holds nothing.
// PREVENTS: a missing-interface error for every VPP port, which sysfs never
// lists.
func TestCheckEthernetInterfacesIsSilentOffTheNetlinkBackend(t *testing.T) {
	fakeSysClassNet(t, map[string]string{})

	vppTree := ethernetTree("wan", nil, nil)
	vppTree.GetContainer(configRootInterface).Set("backend", vppBackendName)
	if diags := ethernetDiags(vppTree); len(diags) != 0 {
		t.Fatalf("vpp backend: diagnostics = %d, want 0: %+v", len(diags), diags)
	}

	empty := config.NewTree()
	empty.GetOrCreateContainer(configRootInterface)
	if diags := ethernetDiags(empty); len(diags) != 0 {
		t.Fatalf("no ethernet: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if diags := ethernetDiags(config.NewTree()); len(diags) != 0 {
		t.Fatalf("no interface block: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if diags := checkEthernetInterfaces(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
}

// TestEthernetDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this component's check, at the phase and order the
// declaration states, and whether every code it emits resolves for
// `ze explain`.
//
// VALIDATES: the init() in register_linux.go installed the check and the
// registry accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestEthernetDoctorCheckRegistered(t *testing.T) {
	want := ethernetDoctorCheck
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
	for _, code := range want.Codes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is not registered", code)
		}
	}
}
