// Design: docs/architecture/doctor-and-health-checks.md -- memlock pre-flight check
// Related: doctor_linux.go -- the check these tests judge

//go:build linux

package memlock

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// oneMiB and fortyMiB stand for a limit far below a ze binary and a binary
// size a real build reaches. The numbers only have to sit either side of each
// other for the comparison under test.
const (
	oneMiB   = 1 << 20
	fortyMiB = 40 << 20
)

// probe returns a reader that answers with the given environment and no error,
// so a test drives the verdict without changing the rlimit or the capabilities
// of the process running it.
func probe(host memlockEnvironment) func() (memlockEnvironment, error) {
	return func() (memlockEnvironment, error) { return host, nil }
}

// TestMemlockCheckIsSilentWhenTheLimitCoversTheBinary proves the check does not
// warn on a correctly configured host.
//
// VALIDATES: a limit at or above the executable's size produces no diagnostic.
//
// PREVENTS: a warning on every host that already lifts the limit, which is the
// noise that teaches an operator to ignore `ze doctor`.
func TestMemlockCheckIsSilentWhenTheLimitCoversTheBinary(t *testing.T) {
	for _, host := range []memlockEnvironment{
		{LimitOctets: fortyMiB, ExecutableOctets: fortyMiB},
		{LimitOctets: fortyMiB + 1, ExecutableOctets: fortyMiB},
		{LimitOctets: ^uint64(0), ExecutableOctets: fortyMiB},
	} {
		if found := memlockLimitDiagnostics(probe(host)); len(found) != 0 {
			t.Errorf("limit %d over a %d octet executable warned: %+v",
				host.LimitOctets, host.ExecutableOctets, found)
		}
	}
}

// TestMemlockCheckWarnsWhenTheLimitCannotHoldTheBinary is the check's reason to
// exist.
//
// VALIDATES: a limit below the executable's size produces one warning under
// doctor-memlock-rlimit-low, and the message carries both numbers and the
// remedy an operator applies.
//
// PREVENTS: a host prepared with the systemd default of 8 MiB, where ze starts,
// runs unlocked, and says so only after the daemon is already up.
func TestMemlockCheckWarnsWhenTheLimitCannotHoldTheBinary(t *testing.T) {
	found := memlockLimitDiagnostics(probe(memlockEnvironment{
		LimitOctets:      oneMiB,
		ExecutableOctets: fortyMiB,
	}))

	if len(found) != 1 {
		t.Fatalf("a limit below the executable produced %d diagnostics, want 1: %+v", len(found), found)
	}
	if found[0].Code != codeMemlockRlimitLow {
		t.Errorf("diagnostic code = %q, want %q", found[0].Code, codeMemlockRlimitLow)
	}
	if found[0].Severity != "warning" {
		t.Errorf("severity = %q, want warning", found[0].Severity)
	}
	for _, want := range []string{"1048576", "41943040", "LimitMEMLOCK=infinity", "IPC_LOCK"} {
		if !strings.Contains(found[0].Message, want) {
			t.Errorf("the message does not carry %q: %s", want, found[0].Message)
		}
	}
}

// TestMemlockCheckIsSilentWhenCAPIPCLockIsHeld pins the case that makes the
// check safe to ship enabled.
//
// VALIDATES: a process holding CAP_IPC_LOCK produces no diagnostic, however
// small the limit is.
//
// PREVENTS: a false warning on every appliance. mlock(2): "a privileged
// process (CAP_IPC_LOCK) can lock as much memory as it likes", so the limit
// does not decide the outcome there, and ze runs as root on an appliance.
func TestMemlockCheckIsSilentWhenCAPIPCLockIsHeld(t *testing.T) {
	found := memlockLimitDiagnostics(probe(memlockEnvironment{
		LimitOctets:      0,
		ExecutableOctets: fortyMiB,
		PrivilegedLock:   true,
	}))
	if len(found) != 0 {
		t.Errorf("a process holding CAP_IPC_LOCK was warned about the limit: %+v", found)
	}
}

// TestMemlockCheckSaysSoWhenItCannotReadTheHost proves the check never answers
// "fine" from a read it could not make.
//
// VALIDATES: a reader that fails produces one diagnostic under
// doctor-memlock-rlimit-unknown, carrying the underlying error.
//
// PREVENTS: the silently-wrong value. A zero environment compares equal, so
// returning it would render as a host that passes.
func TestMemlockCheckSaysSoWhenItCannotReadTheHost(t *testing.T) {
	found := memlockLimitDiagnostics(func() (memlockEnvironment, error) {
		return memlockEnvironment{}, errors.New("/proc is not mounted")
	})

	if len(found) != 1 {
		t.Fatalf("an unreadable host produced %d diagnostics, want 1: %+v", len(found), found)
	}
	if found[0].Code != codeMemlockRlimitUnknown {
		t.Errorf("diagnostic code = %q, want %q", found[0].Code, codeMemlockRlimitUnknown)
	}
	if !strings.Contains(found[0].Message, "/proc is not mounted") {
		t.Errorf("the message drops the underlying error: %s", found[0].Message)
	}
}

// TestReadMemlockEnvironmentReadsThisHost exercises the real reader, which the
// injected probe otherwise leaves untested.
//
// VALIDATES: the reader answers without error on a Linux host and reports a
// positive executable size.
//
// PREVENTS: a check whose four verdict tests all pass over a reader that never
// worked, which is what an injected probe alone would prove.
func TestReadMemlockEnvironmentReadsThisHost(t *testing.T) {
	host, err := readMemlockEnvironment()
	if err != nil {
		t.Fatalf("readMemlockEnvironment on a Linux host: %v", err)
	}
	if host.ExecutableOctets == 0 {
		t.Error("the reader reports a zero-octet executable, which no test binary is")
	}
}

// TestMemlockDoctorCheckIsRegistered proves the check reaches `ze doctor`.
//
// VALIDATES: the memlock registration declares the check, under both codes.
//
// PREVENTS: a check that exists as a function nobody calls, which every unit
// test above would still pass.
func TestMemlockDoctorCheckIsRegistered(t *testing.T) {
	var declared []string
	for _, check := range registry.PluginDoctorChecks() {
		if check.PluginName != pluginName {
			continue
		}
		if check.Name != "memlock-rlimit" {
			continue
		}
		declared = check.Codes
	}
	if declared == nil {
		t.Fatalf("memlock declares no memlock-rlimit doctor check: %+v", registry.PluginDoctorChecks())
	}
	for _, want := range []string{codeMemlockRlimitLow, codeMemlockRlimitUnknown} {
		if !slices.Contains(declared, want) {
			t.Errorf("the registered check does not declare %q: %v", want, declared)
		}
	}
}
