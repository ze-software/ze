//go:build linux

// Design: docs/research/vpp-deployment-reference.md -- DPDK readiness check tests
// Detail: doctor_dpdk_linux.go -- checkVPPDPDK and its registration

package vpp

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// dpdkTree builds a config that selects the vpp backend and binds one DPDK
// interface at pci.
func dpdkTree(pci string) *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("interface").Set("backend", componentVPP)
	dpdk := tree.GetOrCreateContainer(componentVPP).GetOrCreateContainer("dpdk")
	if pci != "" {
		entry := config.NewTree()
		entry.Set("name", "xe0")
		dpdk.AddListEntry("interface", pci, entry)
	}
	return tree
}

// withDPDKHost stands in a host for one test: the module list the kernel
// reports and whether sysfs holds the PCI device.
func withDPDKHost(t *testing.T, loaded map[string]bool, deviceErr error) {
	t.Helper()
	previousModules, previousStat := dpdkLoadedModules, dpdkStatPath
	dpdkLoadedModules = func() map[string]bool { return loaded }
	dpdkStatPath = func(string) (os.FileInfo, error) { return nil, deviceErr }
	t.Cleanup(func() {
		dpdkLoadedModules = previousModules
		dpdkStatPath = previousStat
	})
}

// TestCheckVPPDPDKReportsMissingModulesAndDevice drives the check over a host
// with no VFIO module loaded and no such PCI device.
//
// VALIDATES: one doctor-vpp-dpdk error per absent module and one for the
// device, each naming what is missing.
// PREVENTS: BindAll being the first place an operator learns the host cannot
// bind the NIC.
func TestCheckVPPDPDKReportsMissingModulesAndDevice(t *testing.T) {
	withDPDKHost(t, map[string]bool{}, os.ErrNotExist)

	diags := checkVPPDPDK(diagnostic.DoctorCheckContext{Tree: dpdkTree("0000:03:00.0")})
	if len(diags) != len(vfioModules)+1 {
		t.Fatalf("diagnostics = %d, want %d: %+v", len(diags), len(vfioModules)+1, diags)
	}
	modules, devices := 0, 0
	for i := range diags {
		if diags[i].Code != doctorVPPDPDKCode || diags[i].Severity != diagnostic.SeverityError {
			t.Fatalf("diagnostic %d = %s/%s, want %s at error", i, diags[i].Code, diags[i].Severity, doctorVPPDPDKCode)
		}
		switch {
		case strings.Contains(diags[i].Message, "VFIO"):
			modules++
		case strings.Contains(diags[i].Message, "PCI device not found: 0000:03:00.0"):
			devices++
			if diags[i].Path != sysfsDevDir+"/0000:03:00.0" {
				t.Fatalf("path = %q, want the sysfs device directory", diags[i].Path)
			}
		}
	}
	if modules != len(vfioModules) || devices != 1 {
		t.Fatalf("module diagnostics = %d, device diagnostics = %d", modules, devices)
	}
}

// TestCheckVPPDPDKIsSilentWhenTheHostIsReady is the positive half: every
// module loaded and the device present.
//
// VALIDATES: no diagnostic on a host BindAll would succeed on.
// PREVENTS: an error on every DPDK box, which an operator learns to ignore.
func TestCheckVPPDPDKIsSilentWhenTheHostIsReady(t *testing.T) {
	loaded := map[string]bool{}
	for _, mod := range vfioModules {
		loaded[mod] = true
	}
	withDPDKHost(t, loaded, nil)

	if diags := checkVPPDPDK(diagnostic.DoctorCheckContext{Tree: dpdkTree("0000:03:00.0")}); len(diags) != 0 {
		t.Fatalf("diagnostics = %d, want 0: %+v", len(diags), diags)
	}
}

// TestCheckVPPDPDKIsSilentWithoutDPDKInterfaces is the negative half: no DPDK
// interface, another backend, or no config at all names no dependency.
//
// VALIDATES: no diagnostic for an empty dpdk block, a netlink backend, and a
// nil tree, on a host that has nothing.
// PREVENTS: VFIO errors on a box whose VPP binds no NIC through DPDK.
func TestCheckVPPDPDKIsSilentWithoutDPDKInterfaces(t *testing.T) {
	withDPDKHost(t, map[string]bool{}, os.ErrNotExist)

	if diags := checkVPPDPDK(diagnostic.DoctorCheckContext{Tree: dpdkTree("")}); len(diags) != 0 {
		t.Fatalf("no interfaces: diagnostics = %d, want 0", len(diags))
	}
	netlinkTree := config.NewTree()
	netlinkTree.GetOrCreateContainer("interface")
	if diags := checkVPPDPDK(diagnostic.DoctorCheckContext{Tree: netlinkTree}); len(diags) != 0 {
		t.Fatalf("other backend: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkVPPDPDK(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}
}

// TestVPPLinuxDoctorChecksRegistered asks the registry the doctor runner reads
// whether it holds every check this component declares on Linux, at the phase
// and order each declaration states, and whether each code resolves for
// `ze explain`.
//
// VALIDATES: the init() in register_linux.go installed the table and the
// registry accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestVPPLinuxDoctorChecksRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	table := vppLinuxDoctorChecks()
	if len(table) == 0 {
		t.Fatal("this component declares no check: the loop below would pass over an empty table")
	}
	for _, want := range table {
		var found *diagnostic.DoctorCheck
		checks := diagnostic.DoctorChecksForPhase(want.Phase)
		for i := range checks {
			if checks[i].Name == want.Name {
				found = &checks[i]
				break
			}
		}
		if found == nil {
			t.Errorf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
			continue
		}
		if found.Order != want.Order {
			t.Errorf("check %q: order = %d, want %d", want.Name, found.Order, want.Order)
		}
		if found.Component != want.Component {
			t.Errorf("check %q: component = %q, want %q", want.Name, found.Component, want.Component)
		}
		for _, code := range want.Codes {
			if diagnostic.Lookup(code) == nil {
				t.Errorf("check %q declares code %q, which the code registry does not hold", want.Name, code)
			}
		}
	}
}
