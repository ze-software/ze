// VALIDATES: the ze.test.ike.port override (test infrastructure, mirroring
// ze.test.bgp.port) reroutes IKE listen/dial addresses so two unprivileged
// local daemons can negotiate on a high port; without it UDP 500 is hardcoded
// and unprivileged IKE tests are impossible (spec-test-coverage-gaps AC-3).
// PREVENTS: the override silently regressing to the fixed well-known port.
package engine

import "testing"

func TestIKEAddrDefaultPort(t *testing.T) {
	orig := ikeTestPortFn
	ikeTestPortFn = func() string { return "" }
	t.Cleanup(func() { ikeTestPortFn = orig })

	if got := ikeAddr("192.0.2.7"); got != "192.0.2.7:500" {
		t.Fatalf("ikeAddr = %q, want 192.0.2.7:500", got)
	}
	if got := ikeAddr("0.0.0.0"); got != "0.0.0.0:500" {
		t.Fatalf("ikeAddr(any) = %q, want 0.0.0.0:500", got)
	}
}

func TestIKEAddrOverriddenPort(t *testing.T) {
	orig := ikeTestPortFn
	ikeTestPortFn = func() string { return "45500" }
	t.Cleanup(func() { ikeTestPortFn = orig })

	if got := ikeAddr("127.0.0.2"); got != "127.0.0.2:45500" {
		t.Fatalf("ikeAddr = %q, want 127.0.0.2:45500", got)
	}
}

func TestIKEListenHostHonorsLocalAddressUnderOverride(t *testing.T) {
	orig := ikeTestPortFn
	ikeTestPortFn = func() string { return "45500" }
	t.Cleanup(func() { ikeTestPortFn = orig })

	// Two unprivileged daemons share one port knob by binding their own
	// loopback addresses; only the test override changes the bind host.
	if got := ikeListenHost("", "127.0.0.2"); got != "127.0.0.2" {
		t.Fatalf("ikeListenHost(override) = %q, want the peer local-address", got)
	}
	// An interface-resolved host always wins.
	if got := ikeListenHost("192.0.2.9", "127.0.0.2"); got != "192.0.2.9" {
		t.Fatalf("ikeListenHost(iface) = %q, want 192.0.2.9", got)
	}
}

func TestIKEListenHostDefaultsToWildcardWithoutOverride(t *testing.T) {
	orig := ikeTestPortFn
	ikeTestPortFn = func() string { return "" }
	t.Cleanup(func() { ikeTestPortFn = orig })

	// Production (no override): local-address never changes the bind host.
	if got := ikeListenHost("", "10.0.0.1"); got != "0.0.0.0" {
		t.Fatalf("ikeListenHost(prod) = %q, want 0.0.0.0", got)
	}
}

func TestIKEDataplaneNameOverride(t *testing.T) {
	orig := ikeDataplaneFn
	ikeDataplaneFn = func() string { return "" }
	t.Cleanup(func() { ikeDataplaneFn = orig })

	if got := ikeDataplaneName(); got != "xfrm" {
		t.Fatalf("ikeDataplaneName(default) = %q, want xfrm", got)
	}
	ikeDataplaneFn = func() string { return "noop" }
	if got := ikeDataplaneName(); got != "noop" {
		t.Fatalf("ikeDataplaneName(override) = %q, want noop", got)
	}
}

func TestIKEAddrRejectsGarbageOverride(t *testing.T) {
	orig := ikeTestPortFn
	ikeTestPortFn = func() string { return "not-a-port" }
	t.Cleanup(func() { ikeTestPortFn = orig })

	// Garbage overrides fall back to the well-known port instead of
	// producing an unusable address.
	if got := ikeAddr("127.0.0.2"); got != "127.0.0.2:500" {
		t.Fatalf("ikeAddr = %q, want fallback to :500", got)
	}
}
