// Design: docs/architecture/plugin/plugin-system.md -- memlock plugin
// Overview: memlock.go -- why the executable is locked

//go:build linux

package memlock

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestInitLocksTheExecutable proves the lock the package init() reports is a
// lock the kernel actually holds. It compares the octet count init() recorded
// against VmLck in /proc/self/status, which the kernel maintains and this
// package never writes.
//
// VALIDATES: init() locks the running executable's mappings, and lockedOctets
// says how many octets are locked.
//
// PREVENTS: A green result from a call that returned a number and locked
// nothing, which no assertion over the return value alone can tell apart.
func TestInitLocksTheExecutable(t *testing.T) {
	if lockErr != nil {
		if errors.Is(lockErr, syscall.ENOMEM) || errors.Is(lockErr, syscall.EPERM) {
			t.Skipf("mlock is not permitted here, RLIMIT_MEMLOCK is too low: %v", lockErr)
		}
		t.Fatalf("init() did not lock the executable: %v", lockErr)
	}

	if lockedOctets <= 0 {
		t.Fatalf("init() reported no error and %d locked octets", lockedOctets)
	}

	locked := vmLck(t)
	if locked < lockedOctets {
		t.Fatalf("the kernel holds %d locked octets, init() reported %d", locked, lockedOctets)
	}
}

// TestCheckExecutableLockedSaysNothingWhenLocked drives the doctor check over
// a successful lock.
//
// VALIDATES: A successful lock produces no diagnostic.
//
// PREVENTS: A warning on every healthy daemon, which trains an operator to
// ignore the whole doctor report.
func TestCheckExecutableLockedSaysNothingWhenLocked(t *testing.T) {
	restore := setLockOutcome(t, 42, nil)
	defer restore()

	if diags := checkExecutableLocked(registry.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("a locked executable produced %d diagnostics: %v", len(diags), diags)
	}
}

// TestCheckExecutableLockedReportsTheFailure drives the doctor check over the
// other outcome.
//
// VALIDATES: A failed lock produces one warning that carries the cause and the
// remedy an operator acts on.
//
// PREVENTS: A silent failure. Nothing else tells the operator that the daemon
// can be paged out, because init() runs before the logger exists.
func TestCheckExecutableLockedReportsTheFailure(t *testing.T) {
	restore := setLockOutcome(t, 0, syscall.ENOMEM)
	defer restore()

	diags := checkExecutableLocked(registry.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("a failed lock produced %d diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Code != diagnosticNotLocked {
		t.Errorf("diagnostic code is %q, want %q", diags[0].Code, diagnosticNotLocked)
	}
	if diags[0].Severity != "warning" {
		t.Errorf("diagnostic severity is %q, want %q", diags[0].Severity, "warning")
	}
	for _, want := range []string{"cannot allocate memory", "RLIMIT_MEMLOCK", "LimitMEMLOCK=infinity"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("diagnostic message does not carry %q: %s", want, diags[0].Message)
		}
	}
}

// TestPluginRegistered proves the package's init() puts the plugin in the
// registry the daemon reads, under the name the engine run reports against.
//
// VALIDATES: The memlock plugin registers.
//
// PREVENTS: A package that locks memory but never appears in `ze plugin list`,
// so an operator cannot tell whether the feature is present.
func TestPluginRegistered(t *testing.T) {
	if !slices.Contains(registry.Names(), "memlock") {
		t.Fatalf("memlock is not in the registry: %v", registry.Names())
	}
}

// setLockOutcome replaces the outcome init() recorded and returns the function
// that puts it back. The tests that use it run in one goroutine and never in
// parallel, which is what makes writing these two variables safe.
func setLockOutcome(t *testing.T, octets int64, err error) func() {
	t.Helper()
	octetsWas, errWas := lockedOctets, lockErr
	lockedOctets, lockErr = octets, err
	return func() { lockedOctets, lockErr = octetsWas, errWas }
}

// vmLck returns the octet count the kernel has locked for this process.
func vmLck(t *testing.T) int64 {
	t.Helper()
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatalf("read /proc/self/status: %v", err)
	}
	for line := range strings.Lines(string(status)) {
		kb, found := strings.CutPrefix(line, "VmLck:")
		if !found {
			continue
		}
		kb = strings.TrimSuffix(strings.TrimSpace(kb), " kB")
		octets, err := strconv.ParseInt(kb, 10, 64)
		if err != nil {
			t.Fatalf("malformed VmLck line %q: %v", line, err)
		}
		return octets * 1024
	}
	t.Fatal("no VmLck line in /proc/self/status")
	return 0
}
