//go:build linux

// Design: docs/features/interfaces.md -- the readiness checks this plugin owns
// Detail: doctor_linux.go -- checkNTPClockPrivilege under test

package ntp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withProcess stands in a process for one test: its uid and the
// /proc/self/status text carrying its effective capability mask.
func withProcess(t *testing.T, uid int, status string) {
	t.Helper()
	previousUID, previousRead := ntpCurrentUID, ntpReadProcess
	ntpCurrentUID = func() int { return uid }
	ntpReadProcess = func(string) ([]byte, error) { return []byte(status), nil }
	t.Cleanup(func() {
		ntpCurrentUID = previousUID
		ntpReadProcess = previousRead
	})
}

// TestCheckNTPClockPrivilegeWarnsWithoutCapSysTime drives the check over an
// enabled client in an unprivileged process whose effective mask is empty.
//
// VALIDATES: doctor-ntp-clock-privilege at warning severity.
// PREVENTS: a clock adjustment that fails at the first sync rather than in the
// readiness report.
func TestCheckNTPClockPrivilegeWarnsWithoutCapSysTime(t *testing.T) {
	withProcess(t, 1000, "Name:\tze\nCapEff:\t0000000000000000\n")

	diags := checkNTPClockPrivilege(diagnostic.DoctorCheckContext{Tree: ntpTree(true)})
	requireOneDiag(t, diags, codeNTPClockPrivilege, diagnostic.SeverityWarning)
}

// TestCheckNTPClockPrivilegeIsSilentWhenGranted is the positive half, and the
// cases that ask nothing: root, a disabled client, and no config.
//
// VALIDATES: no diagnostic when the mask carries CAP_SYS_TIME (bit 25), when
// the process is root, when the client is disabled, and for a nil tree.
// PREVENTS: a privilege warning on every root daemon and every box without the
// client.
func TestCheckNTPClockPrivilegeIsSilentWhenGranted(t *testing.T) {
	withProcess(t, 1000, "Name:\tze\nCapEff:\t0000000002000000\n")
	if diags := checkNTPClockPrivilege(diagnostic.DoctorCheckContext{Tree: ntpTree(true)}); len(diags) != 0 {
		t.Fatalf("granted: diagnostics = %d, want 0: %+v", len(diags), diags)
	}

	withProcess(t, 0, "Name:\tze\nCapEff:\t0000000000000000\n")
	if diags := checkNTPClockPrivilege(diagnostic.DoctorCheckContext{Tree: ntpTree(true)}); len(diags) != 0 {
		t.Fatalf("root: diagnostics = %d, want 0: %+v", len(diags), diags)
	}

	withProcess(t, 1000, "Name:\tze\nCapEff:\t0000000000000000\n")
	if diags := checkNTPClockPrivilege(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("disabled: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if diags := checkNTPClockPrivilege(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0: %+v", len(diags), diags)
	}
}
