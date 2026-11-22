// Design: docs/features/ai-first.md -- the readiness check this component owns
// Detail: doctor.go -- checkSysctlProcfs and its registration

package sysctl

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
)

// withUnwritableProcSys stands in a kernel that refuses every write to /proc/sys
// for the life of one test.
func withUnwritableProcSys(t *testing.T) {
	t.Helper()
	previous := probeProcSysPath
	probeProcSysPath = func(string) (procSysVerdict, error) { return procSysRefused, os.ErrPermission }
	t.Cleanup(func() { probeProcSysPath = previous })
}

// sysctlTree builds the smallest config that applies one sysctl.
func sysctlTree() *config.Tree {
	return sysctlTreeWithKeys("net.ipv4.ip_forward")
}

// sysctlTreeWithKeys builds a config applying the given sysctl keys.
func sysctlTreeWithKeys(keys ...string) *config.Tree {
	tree := config.NewTree()
	block := tree.GetOrCreateContainer("sysctl")
	for _, key := range keys {
		setting := config.NewTree()
		setting.Set("value", "1")
		block.AddListEntry("setting", key, setting)
	}
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

	probeProcSysPath = func(string) (procSysVerdict, error) { return procSysWritable, nil }
	if diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{Tree: sysctlTree()}); len(diags) != 0 {
		t.Fatalf("writable tree: diagnostics = %d, want 0", len(diags))
	}
}

// TestCheckSysctlProcfsVerdictsAndRemedies checks each verdict checkSysctlProcfs
// can receive from probeProcSysPath: net.* refusal uses CapabilityRemedy for
// CAP_NET_ADMIN (systemd only under ze.service), non-net.* refusal explains the
// root requirement without naming CAP_NET_ADMIN as a fix, read-only procfs
// names remount, and unknown says "cannot determine".
//
// VALIDATES: per-key probing and distinct messages/fixes for net.* EACCES,
// non-net.* EACCES, EROFS, and unknown (ENOENT/ENOSYS/other).
// PREVENTS: a non-root CAP_NET_ADMIN daemon falsely warned about net.* keys,
// a systemd command offered outside ze.service, or an unreadable probe reported
// as writable or refused.
func TestCheckSysctlProcfsVerdictsAndRemedies(t *testing.T) {
	t.Run("net key refused under ze.service", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
		withUnwritableProcSys(t)
		diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{Tree: sysctlTreeWithKeys("net.ipv4.ip_forward")})
		if len(diags) != 1 {
			t.Fatalf("diagnostics = %v, want 1", diags)
		}
		if !strings.HasSuffix(diags[0].Path, "/net/ipv4/ip_forward") {
			t.Errorf("path = %q, want suffix /net/ipv4/ip_forward", diags[0].Path)
		}
		if diags[0].Fix != privilege.DaemonCapabilitiesFix {
			t.Errorf("fix = %q, want %q", diags[0].Fix, privilege.DaemonCapabilitiesFix)
		}
		if !strings.Contains(diags[0].Message, "CAP_NET_ADMIN") {
			t.Errorf("message = %q, want CAP_NET_ADMIN", diags[0].Message)
		}
	})

	t.Run("net key refused outside ze.service", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
		withUnwritableProcSys(t)
		diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{Tree: sysctlTreeWithKeys("net.ipv4.ip_forward")})
		if len(diags) != 1 {
			t.Fatalf("diagnostics = %v, want 1", diags)
		}
		if strings.Contains(diags[0].Fix, "systemctl") || strings.Contains(diags[0].Fix, "install systemd") {
			t.Errorf("fix = %q offers systemd outside ze.service", diags[0].Fix)
		}
		if !strings.Contains(diags[0].Fix, "CAP_NET_ADMIN") {
			t.Errorf("fix = %q does not name CAP_NET_ADMIN", diags[0].Fix)
		}
	})

	t.Run("non-net key refused requires root", func(t *testing.T) {
		t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
		withUnwritableProcSys(t)
		diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{Tree: sysctlTreeWithKeys("vm.swappiness")})
		if len(diags) != 1 {
			t.Fatalf("diagnostics = %v, want 1", diags)
		}
		if !strings.HasSuffix(diags[0].Path, "/vm/swappiness") {
			t.Errorf("path = %q, want suffix /vm/swappiness", diags[0].Path)
		}
		if !strings.Contains(diags[0].Message, "requires root") {
			t.Errorf("message = %q, want root requirement", diags[0].Message)
		}
		if diags[0].Fix != "delete sysctl setting vm.swappiness" {
			t.Errorf("fix = %q, want delete sysctl setting vm.swappiness", diags[0].Fix)
		}
	})

	t.Run("read-only filesystem", func(t *testing.T) {
		previous := probeProcSysPath
		probeProcSysPath = func(string) (procSysVerdict, error) { return procSysReadOnlyFS, errors.New("read-only file system") }
		t.Cleanup(func() { probeProcSysPath = previous })
		diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{Tree: sysctlTreeWithKeys("net.ipv4.ip_forward")})
		if len(diags) != 1 {
			t.Fatalf("diagnostics = %v, want 1", diags)
		}
		if !strings.Contains(diags[0].Message, "mounted read-only") {
			t.Errorf("message = %q, want mounted read-only", diags[0].Message)
		}
		if !strings.Contains(diags[0].Fix, "mount -o remount,rw") {
			t.Errorf("fix = %q, want remount command", diags[0].Fix)
		}
	})

	t.Run("cannot determine", func(t *testing.T) {
		previous := probeProcSysPath
		probeProcSysPath = func(string) (procSysVerdict, error) { return procSysUnknown, os.ErrNotExist }
		t.Cleanup(func() { probeProcSysPath = previous })
		diags := checkSysctlProcfs(diagnostic.DoctorCheckContext{Tree: sysctlTreeWithKeys("net.ipv4.conf.eth99.forwarding")})
		if len(diags) != 1 {
			t.Fatalf("diagnostics = %v, want 1", diags)
		}
		if !strings.Contains(diags[0].Message, "cannot determine") {
			t.Errorf("message = %q, want cannot determine", diags[0].Message)
		}
		if diags[0].Fix == "" {
			t.Errorf("fix is empty")
		}
	})
}

// TestSysctlProcfsDoctorCheckRegistered asks the registry the doctor runner
// reads whether it holds this component's check, at the phase and order the
// declaration states, and whether the code it emits resolves for `ze show diagnostic`.
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
