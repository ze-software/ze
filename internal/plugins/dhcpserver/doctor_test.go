// Design: docs/architecture/provisioning/dhcp-server.md -- the readiness check this plugin owns
// Detail: doctor.go -- checkDHCPInterfaces and its registration

package dhcpserver

import (
	"errors"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// dhcpServerTree builds the smallest config that enables the server on the
// named listen interfaces.
func dhcpServerTree(enabled bool, interfaces ...string) *config.Tree {
	tree := config.NewTree()
	dhcp := tree.GetOrCreateContainer(configRootService).GetOrCreateContainer("dhcp-server")
	if enabled {
		dhcp.Set("enabled", "true")
	}
	dhcp.SetSlice("listen-interface", interfaces)
	return tree
}

// withHostInterfaces stands in a host that holds exactly the named devices.
func withHostInterfaces(t *testing.T, names ...string) {
	t.Helper()
	previous := dhcpInterfaceByName
	dhcpInterfaceByName = func(name string) (*net.Interface, error) {
		if slices.Contains(names, name) {
			return &net.Interface{Name: name}, nil
		}
		return nil, errors.New("route ip+net: no such network interface")
	}
	t.Cleanup(func() { dhcpInterfaceByName = previous })
}

// TestCheckDHCPInterfacesReportsAMissingInterface drives the check over an
// enabled server whose listen interface the host does not hold.
//
// VALIDATES: the diagnostic carries doctor-dhcp-iface at error severity, names
// the interface, and names the config path.
// PREVENTS: DHCP bind failures surfacing only when the daemon starts.
func TestCheckDHCPInterfacesReportsAMissingInterface(t *testing.T) {
	withHostInterfaces(t, "enp1s0")

	diags := checkDHCPInterfaces(diagnostic.DoctorCheckContext{Tree: dhcpServerTree(true, "ze-doctor-missing0")})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != codeDHCPIface || diags[0].Severity != diagnostic.SeverityError {
		t.Fatalf("diagnostic = %s/%s, want %s at error", diags[0].Code, diags[0].Severity, codeDHCPIface)
	}
	if !strings.Contains(diags[0].Message, "not found: ze-doctor-missing0") {
		t.Fatalf("message = %q, want the interface named", diags[0].Message)
	}
	if diags[0].Path != listenInterfacePath {
		t.Fatalf("path = %q, want %q", diags[0].Path, listenInterfacePath)
	}
}

// TestCheckDHCPInterfacesRefusesANameThatIsNotADevice covers the name shapes
// the check rejects before asking the host: a path separator, a NUL, or a
// parent reference.
//
// VALIDATES: each is reported with the invalid-name wording and never reaches
// the host probe.
// PREVENTS: a config value that is not an interface name being handed to the
// kernel as one.
func TestCheckDHCPInterfacesRefusesANameThatIsNotADevice(t *testing.T) {
	probed := false
	previous := dhcpInterfaceByName
	dhcpInterfaceByName = func(string) (*net.Interface, error) {
		probed = true
		return nil, errors.New("the host probe must not be reached")
	}
	t.Cleanup(func() { dhcpInterfaceByName = previous })

	diags := checkDHCPInterfaces(diagnostic.DoctorCheckContext{Tree: dhcpServerTree(true, "eth/0", "..", "a\x00b")})
	if len(diags) != 3 {
		t.Fatalf("diagnostics = %d, want 3: %+v", len(diags), diags)
	}
	for i := range diags {
		if !strings.Contains(diags[i].Message, "invalid name") {
			t.Fatalf("diagnostic %d = %q, want the invalid-name wording", i, diags[i].Message)
		}
	}
	if probed {
		t.Fatal("an invalid name reached the host probe")
	}
}

// TestCheckDHCPInterfacesIsSilentWhenTheServerIsOff is the negative half: a
// disabled server, a config without the block, a nil tree, and an interface
// the host holds each name no dependency.
//
// VALIDATES: no diagnostic in each case.
// PREVENTS: an error on every box that never enabled the server.
func TestCheckDHCPInterfacesIsSilentWhenTheServerIsOff(t *testing.T) {
	withHostInterfaces(t, "enp1s0")

	if diags := checkDHCPInterfaces(diagnostic.DoctorCheckContext{Tree: dhcpServerTree(false, "ze-doctor-missing0")}); len(diags) != 0 {
		t.Fatalf("disabled: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkDHCPInterfaces(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("no block: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkDHCPInterfaces(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkDHCPInterfaces(diagnostic.DoctorCheckContext{Tree: dhcpServerTree(true, "enp1s0")}); len(diags) != 0 {
		t.Fatalf("held interface: diagnostics = %d, want 0", len(diags))
	}
}

// TestDHCPDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this plugin's check, at the phase and order the declaration
// states, and whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestDHCPDoctorCheckRegistered(t *testing.T) {
	want := dhcpDoctorCheck
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
	if diagnostic.Lookup(codeDHCPIface) == nil {
		t.Fatalf("diagnostic code %q is not registered", codeDHCPIface)
	}
}
