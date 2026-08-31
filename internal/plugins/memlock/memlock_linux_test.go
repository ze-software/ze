// Design: docs/architecture/plugin/plugin-system.md -- memlock plugin
// Overview: memlock.go -- why the executable is locked
// Related: memlock_linux.go -- the init() these tests judge

//go:build linux

package memlock

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	lockexe "filippo.io/mlockexe"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// recordedOutcome returns the setup result memlock's init() left in the
// registry. A missing row is a failure of the thing under test, so it fails
// here rather than being read as an absent feature.
func recordedOutcome(t *testing.T) registry.SetupResult {
	t.Helper()
	for _, result := range registry.SetupResults() {
		if result.Module == pluginName {
			return result
		}
	}
	t.Fatalf("memlock recorded no setup outcome: %+v", registry.SetupResults())
	return registry.SetupResult{}
}

// TestMemlockRecordsItsOutcome proves AC-6: the outcome reaches the registry,
// and it reaches it from init() rather than from a package variable only this
// package could read.
//
// VALIDATES: memlock records succeeded when the lock is taken, and a soft
// failure carrying the mlockexe error and the remedy when it is not.
//
// PREVENTS: a feature that is silently absent. Before the record existed, a
// failed lock was visible only to a doctor check that answered for the wrong
// process, so a daemon running unlocked said so nowhere.
func TestMemlockRecordsItsOutcome(t *testing.T) {
	result := recordedOutcome(t)

	switch result.Outcome {
	case registry.SetupSucceeded:
		if result.Reason != "" {
			t.Errorf("a successful lock carries reason %q, want none", result.Reason)
		}
	case registry.SetupFailedSoft:
		for _, want := range []string{"RLIMIT_MEMLOCK", "LimitMEMLOCK=infinity"} {
			if !strings.Contains(result.Reason, want) {
				t.Errorf("the soft-failure reason does not carry %q: %s", want, result.Reason)
			}
		}
	default:
		t.Fatalf("memlock recorded outcome %v, want succeeded or soft-failure", result.Outcome)
	}
}

// TestMemlockNeverRecordsAHardFailure pins the classification the owner set:
// memlock is the soft exemplar.
//
// VALIDATES: memlock's recorded outcome is never SetupFailedHard.
//
// PREVENTS: a daemon refusing to boot because the executable could not be
// pinned. It serves every session correctly unlocked, and only pays a page
// fault when the kernel has evicted a page under memory pressure.
func TestMemlockNeverRecordsAHardFailure(t *testing.T) {
	if outcome := recordedOutcome(t).Outcome; outcome == registry.SetupFailedHard {
		t.Fatalf("memlock recorded a hard failure, which would stop the daemon")
	}
}

// TestTheRecordedLockIsALockTheKernelHolds proves the record is not a lie.
//
// VALIDATES: when memlock recorded success, the kernel holds locked pages for
// this process, and it holds at least as many octets as the locking call
// reports.
//
// PREVENTS: a green result from a call that returned a number and locked
// nothing, which no assertion over the return value alone can tell apart.
// VmLck in /proc/self/status is maintained by the kernel and this package
// never writes it.
func TestTheRecordedLockIsALockTheKernelHolds(t *testing.T) {
	result := recordedOutcome(t)
	if result.Outcome == registry.SetupFailedSoft {
		t.Skipf("mlock is not permitted here, RLIMIT_MEMLOCK is too low: %s", result.Reason)
	}

	locked := vmLck(t)
	if locked <= 0 {
		t.Fatalf("memlock recorded a successful lock and the kernel holds %d locked octets", locked)
	}

	// The same call init() made, over the same already-locked mappings. It
	// reports the octet count the module claims to have pinned, and the kernel
	// must hold at least that many.
	octets, err := lockexe.OnFault()
	if err != nil {
		t.Fatalf("the lock succeeded at init() and fails now: %v", err)
	}
	if locked < octets {
		t.Fatalf("the kernel holds %d locked octets, mlockexe reports %d", locked, octets)
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
	if !slices.Contains(registry.Names(), pluginName) {
		t.Fatalf("memlock is not in the registry: %v", registry.Names())
	}
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
