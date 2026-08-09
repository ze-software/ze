// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- raw-socket doctor check tests

package transport

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// vrrpTree builds a config tree with a vrrp container nested under an interface
// unit, matching the umbrella config surface
// (interface ... unit ... ipv4 vrrp group).
func vrrpTree() *config.Tree {
	tree := config.NewTree()
	ipv4 := tree.
		GetOrCreateContainer("interface").
		GetOrCreateContainer("ethernet").
		GetOrCreateContainer("eth0").
		GetOrCreateContainer("unit").
		GetOrCreateContainer("0").
		GetOrCreateContainer("ipv4")
	ipv4.GetOrCreateContainer("vrrp").GetOrCreateContainer("group")
	return tree
}

func TestVRRPRawSocketDoctorWarn(t *testing.T) {
	// VALIDATES: AC-11 -- vrrp configured + probe false -> one SeverityWarning with
	// code doctor-vrrp-raw-socket.
	old := rawSocketProbe
	rawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rawSocketProbe = old })

	diags := checkVRRPRawSocket(diagnostic.DoctorCheckContext{Tree: vrrpTree()})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(diags))
	}
	if diags[0].Code != "doctor-vrrp-raw-socket" {
		t.Errorf("code = %q, want doctor-vrrp-raw-socket", diags[0].Code)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("severity = %q, want warning", diags[0].Severity)
	}

	// With the probe available, no warning even when vrrp is configured.
	rawSocketProbe = func() bool { return true }
	if diags := checkVRRPRawSocket(diagnostic.DoctorCheckContext{Tree: vrrpTree()}); len(diags) != 0 {
		t.Errorf("probe available: got %d diagnostics, want 0", len(diags))
	}
}

func TestVRRPRawSocketDoctorSilent(t *testing.T) {
	// VALIDATES: AC-11 -- no vrrp group in the tree -> no diagnostics regardless of
	// the probe (boxes not running VRRP get no spurious warning).
	old := rawSocketProbe
	rawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rawSocketProbe = old })

	if diags := checkVRRPRawSocket(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Errorf("empty tree: got %d diagnostics, want 0", len(diags))
	}
	// A tree with an interface but no vrrp container stays silent.
	tree := config.NewTree()
	tree.GetOrCreateContainer("interface").GetOrCreateContainer("ethernet")
	if diags := checkVRRPRawSocket(diagnostic.DoctorCheckContext{Tree: tree}); len(diags) != 0 {
		t.Errorf("no-vrrp tree: got %d diagnostics, want 0", len(diags))
	}
	if diags := checkVRRPRawSocket(diagnostic.DoctorCheckContext{Tree: nil}); len(diags) != 0 {
		t.Errorf("nil tree: got %d diagnostics, want 0", len(diags))
	}
}

func TestVRRPRawSocketDoctorRegistered(t *testing.T) {
	// VALIDATES: the transport registers the vrrp-raw-socket doctor check so
	// `ze doctor` runs it (removing the package removes the check).
	if !slices.Contains(diagnostic.DoctorCheckNames(), "vrrp-raw-socket") {
		t.Fatal("doctor check vrrp-raw-socket not registered via diagnostic.RegisterDoctorCheck")
	}
}

func TestVRRPRawSocketCodeRegistered(t *testing.T) {
	// VALIDATES: AC-11 -- the doctor-vrrp-raw-socket diagnostic code is explainable
	// via `ze explain` (CodeMeta registered in codes.go).
	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup("doctor-vrrp-raw-socket")
	if meta == nil {
		t.Fatal("doctor-vrrp-raw-socket not registered in codes.go")
	}
	if meta.Title == "" || meta.Description == "" {
		t.Error("doctor-vrrp-raw-socket missing title/description")
	}
}
