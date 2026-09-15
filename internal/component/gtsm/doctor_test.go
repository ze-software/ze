// Design: docs/architecture/doctor-and-health-checks.md -- the readiness check this component owns
// Detail: doctor.go -- checkKernelState and its registration
//
// The behavior cases are in doctor_linux_test.go, because the kernel readers
// they stand in are the Linux route calls. What runs on every platform is the
// registration and the gates every check shares.

package gtsm

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// registeredKernelStateCheck answers the check as the doctor runner reads it:
// out of the registry, for its phase, by its name.
//
// It is the entry point every test below drives, so a check that init()
// stopped installing fails here rather than in a test that called the helper
// directly (ai/rules/evidence.md).
func registeredKernelStateCheck(t *testing.T) diagnostic.DoctorCheck {
	t.Helper()
	want := kernelStateDoctorCheck()
	for _, check := range diagnostic.DoctorChecksForPhase(want.Phase) {
		if check.Name == want.Name {
			return check
		}
	}
	t.Fatalf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
	return diagnostic.DoctorCheck{}
}

// bgpTree is the smallest config tree that carries a bgp{} block, which is
// the gate the check opens on.
func bgpTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("bgp")
	return tree
}

// TestGTSMKernelStateDoctorCheckRegistered asks the registry the doctor
// runner reads whether it holds this component's check, at the phase, order
// and component the declaration states, and whether the code it emits
// resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it; codes.go names the code.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green, and a diagnostic an operator cannot look up.
func TestGTSMKernelStateDoctorCheckRegistered(t *testing.T) {
	want := kernelStateDoctorCheck()
	found := registeredKernelStateCheck(t)
	if found.Order != want.Order {
		t.Fatalf("order = %d, want %d", found.Order, want.Order)
	}
	if found.Component != want.Component {
		t.Fatalf("component = %q, want %q", found.Component, want.Component)
	}

	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup(doctorKernelStateCode)
	if meta == nil {
		t.Fatalf("diagnostic code %q is not registered", doctorKernelStateCode)
	}
	if meta.Title == "" || meta.Description == "" {
		t.Fatalf("diagnostic code %q has no title or description", doctorKernelStateCode)
	}
}

// TestGTSMKernelStateDoctorCheckIsSilentWithoutBGP is the gate: no tree, a
// tree of another type, and a tree with no bgp{} block each name no peer, so
// nothing is asked of the kernel.
//
// VALIDATES: no diagnostic for the missing-config phase and for a box that
// runs no BGP.
// PREVENTS: a warning on every box, which an operator learns to ignore.
func TestGTSMKernelStateDoctorCheckIsSilentWithoutBGP(t *testing.T) {
	check := registeredKernelStateCheck(t)
	for name, tree := range map[string]any{
		"nil tree":     nil,
		"no bgp block": config.NewTree(),
		"wrong type":   "not a tree",
	} {
		if diags := check.Check(diagnostic.DoctorCheckContext{Tree: tree}); len(diags) != 0 {
			t.Fatalf("%s: diagnostics = %v, want none", name, diags)
		}
	}
}

// TestGTSMKernelStateDoctorCheckRefusesToGuessWithoutADerivation is the
// offline answer with no BGP engine in the binary: the peer set cannot be
// derived, and the check says so rather than reading an empty set as "no
// GTSM peer".
//
// VALIDATES: a nil seam with a bgp{} block present is one warning naming the
// missing derivation.
// PREVENTS: silence standing in for an answer (ai/rules/principles.md).
func TestGTSMKernelStateDoctorCheckRefusesToGuessWithoutADerivation(t *testing.T) {
	captureSeams(t)
	previous := configPeers
	configPeers = nil
	t.Cleanup(func() { configPeers = previous })

	diags := registeredKernelStateCheck(t).Check(diagnostic.DoctorCheckContext{Tree: bgpTree()})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(diags))
	}
	if diags[0].Code != doctorKernelStateCode || diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("diagnostic = %+v, want a %s warning", diags[0], doctorKernelStateCode)
	}
}
