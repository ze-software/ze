// Design: docs/features/ai-first.md -- the readiness check this component owns
// Detail: doctor.go -- checkSysctlProcfs and its registration

package sysctl

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withUnwritableProcSys stands in a kernel that refuses every write to /proc/sys
// for the life of one test.
func withUnwritableProcSys(t *testing.T) {
	t.Helper()
	previous := procSysWritable
	procSysWritable = func(string) error { return os.ErrPermission }
	t.Cleanup(func() { procSysWritable = previous })
}

// sysctlTree builds the smallest config that applies one sysctl.
func sysctlTree() *config.Tree {
	tree := config.NewTree()
	block := tree.GetOrCreateContainer("sysctl")
	setting := config.NewTree()
	setting.Set("value", "1")
	block.AddListEntry("setting", "net.ipv4.ip_forward", setting)
	return tree
}

// TestCheckSysctlProcfsReportsAnUnwritableTree drives the check over a config
// that applies a sysctl on a kernel that refuses the write.
//
// VALIDATES: the diagnostic carries doctor-sysctl-procfs as a warning, and
// names the path the daemon would have written.
// PREVENTS: a sysctl apply failure surfacing at daemon start rather than in
// the readiness report.
func TestCheckSysctlProcfsReportsAnUnwritableTree(t *testing.T) {
	withUnwritableProcSys(t)

	diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{Tree: sysctlTree()})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(diags))
	}
	if diags[0].Code != doctorSysctlProcfsCode {
		t.Fatalf("code = %q, want %q", diags[0].Code, doctorSysctlProcfsCode)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("severity = %q, want warning", diags[0].Severity)
	}
	if diags[0].Path == "" {
		t.Fatal("the diagnostic names no path")
	}
}

// TestCheckSysctlProcfsIsSilentWithoutASysctlBlock is the negative half: a
// config that applies no sysctl names no dependency on /proc/sys.
//
// VALIDATES: no diagnostic for a config without a sysctl block, for a nil
// tree, and for a writable tree.
// PREVENTS: a warning on every box whose config never touches a sysctl, which
// an operator learns to ignore.
func TestCheckSysctlProcfsIsSilentWithoutASysctlBlock(t *testing.T) {
	withUnwritableProcSys(t)

	if diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("no sysctl block: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}

	procSysWritable = func(string) error { return nil }
	if diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{Tree: sysctlTree()}); len(diags) != 0 {
		t.Fatalf("writable tree: diagnostics = %d, want 0", len(diags))
	}
}

// TestSysctlProcfsDoctorCheckRegistered asks the registry the doctor runner
// reads whether it holds this component's check, at the phase and order the
// declaration states, and whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestSysctlProcfsDoctorCheckRegistered(t *testing.T) {
	want := sysctlProcfsDoctorCheck()
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
	if diagnostic.Lookup(doctorSysctlProcfsCode) == nil {
		t.Fatalf("diagnostic code %q is not registered", doctorSysctlProcfsCode)
	}
}
