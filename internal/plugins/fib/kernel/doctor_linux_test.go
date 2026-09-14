//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the readiness check this plugin owns
// Detail: doctor_linux.go -- checkKernelNexthop and its registration

package fibkernel

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withNexthopFile stands in a kernel that does or does not expose
// /proc/net/nexthop for the life of one test.
func withNexthopFile(t *testing.T, err error) {
	t.Helper()
	previous := nexthopStatPath
	nexthopStatPath = func(string) (os.FileInfo, error) { return nil, err }
	t.Cleanup(func() { nexthopStatPath = previous })
}

// TestCheckKernelNexthopWarnsWithoutNexthopObjects drives the check over a
// kernel whose /proc/net/nexthop is absent.
//
// VALIDATES: the diagnostic carries doctor-kernel-nexthop as a warning and
// names the path it looked for.
// PREVENTS: an operator finding out from route dumps alone that ECMP is
// expressed as legacy multipath.
func TestCheckKernelNexthopWarnsWithoutNexthopObjects(t *testing.T) {
	withNexthopFile(t, os.ErrNotExist)

	diags := checkKernelNexthop(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != codeKernelNexthop || diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("diagnostic = %s/%s, want %s at warning", diags[0].Code, diags[0].Severity, codeKernelNexthop)
	}
	if !strings.Contains(diags[0].Message, "nexthop") || !strings.Contains(diags[0].Message, "legacy multipath") {
		t.Fatalf("message = %q, want the nexthop path and the multipath consequence", diags[0].Message)
	}
}

// TestCheckKernelNexthopIsSilentWithNexthopObjects is the positive half.
//
// VALIDATES: no diagnostic on a kernel that exposes the file.
// PREVENTS: a warning on every modern kernel, which an operator learns to
// ignore.
func TestCheckKernelNexthopIsSilentWithNexthopObjects(t *testing.T) {
	withNexthopFile(t, nil)

	if diags := checkKernelNexthop(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("diagnostics = %d, want 0: %+v", len(diags), diags)
	}
}

// TestKernelNexthopDoctorCheckRegistered asks the registry the doctor runner
// reads whether it holds this plugin's check, at the phase and order the
// declaration states, and whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register_linux.go installed the check and the
// registry accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestKernelNexthopDoctorCheckRegistered(t *testing.T) {
	want := kernelNexthopDoctorCheck
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
	if diagnostic.Lookup(codeKernelNexthop) == nil {
		t.Fatalf("diagnostic code %q is not registered", codeKernelNexthop)
	}
}
