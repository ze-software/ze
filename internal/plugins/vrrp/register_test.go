// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- live platform wiring
//
// VALIDATES: waitDevicePresent returns only once a kernel device with the given
// name exists, and errors (never hangs) when it does not appear. This is the
// guard added after the keepalived interop lab proved the original code opened
// the transport before the macvlan existed: createMacvlan (RegisterOwnedMacvlan)
// only records desired state, so without this wait every VRRP instance died
// with "resolve macvlan <dev>: no such network interface".
// PREVENTS: a regression back to a synchronous-looking createMacvlan that
// returns before the device is real -- the engine_test.go fake platform cannot
// catch that, because it stubs createMacvlan out entirely.

package vrrp

import (
	"net"
	"strings"
	"testing"
	"time"
)

// TestWaitDevicePresentExisting: a device that already exists returns nil
// promptly. Loopback is present on every platform this test runs on (including
// the darwin dev machine), so it is a stable stand-in for an already-created
// macvlan without needing root or netlink.
func TestWaitDevicePresentExisting(t *testing.T) {
	lo := loopbackName(t)

	// Inject a huge poll interval: a first-probe hit still returns at once, but a
	// sleep-before-probe regression would block for the whole interval. This keeps
	// the "first probe, no poll" assertion deterministic even on a loaded machine,
	// where a single net.InterfaceByName syscall can itself exceed a 20ms interval
	// and be mistaken for a poll cycle.
	const hugeInterval = 30 * time.Second
	start := time.Now()
	if err := waitDevicePresentEvery(lo, 2*time.Second, hugeInterval); err != nil {
		t.Fatalf("waitDevicePresentEvery(%q) = %v, want nil for an existing device", lo, err)
	}
	// Well under one poll interval => no sleep happened; the device was found on
	// the first probe. The bound is generous enough to absorb a slow syscall yet
	// far below hugeInterval, so a poll would be unmistakable.
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("waitDevicePresentEvery returned after %s for an existing device; expected a first-probe hit, not a %s poll", elapsed, hugeInterval)
	}
}

// TestWaitDevicePresentMissing: a device that never appears makes
// waitDevicePresent return an error at the deadline rather than hang. The error
// must name the device and the timeout so an operator can tell a slow reconcile
// from a wedged one.
func TestWaitDevicePresentMissing(t *testing.T) {
	const missing = "zzz-vrrp-nodev0"
	if _, err := net.InterfaceByName(missing); err == nil {
		t.Skipf("test device %q unexpectedly exists on this host", missing)
	}

	start := time.Now()
	err := waitDevicePresent(missing, 120*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("waitDevicePresent(%q) = nil, want an error for a device that never appears", missing)
	}
	// It waited (did not give up before the deadline) but did not hang: the
	// deadline is honored within a poll interval of slack.
	if elapsed < 100*time.Millisecond {
		t.Fatalf("waitDevicePresent returned after only %s; it must poll until the deadline", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("waitDevicePresent took %s for a 120ms timeout; the deadline is not bounding the loop", elapsed)
	}
	msg := err.Error()
	if !strings.Contains(msg, missing) || !strings.Contains(msg, "did not appear") {
		t.Fatalf("error %q must name the device %q and say it did not appear", err, missing)
	}
}

// loopbackName returns the platform's loopback interface name, skipping the test
// if the host has none (it always does in practice; the guard keeps the test
// honest rather than asserting a name that could differ).
func loopbackName(t *testing.T) string {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("net.Interfaces: %v", err)
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagLoopback != 0 {
			return ifi.Name
		}
	}
	t.Skip("no loopback interface on this host")
	return ""
}
