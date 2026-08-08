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

// TestNATTAddrKeepsWellKnownPortInProduction pins the half of nattAddr that is
// not negotiable: RFC 3948 Section 2.1 puts UDP-encapsulated ESP on port 4500,
// and no configuration moves it.
//
// VALIDATES: without the test override the NAT-T socket asks for 4500.
// PREVENTS: the test seam leaking into a deployment, where a peer sends to 4500
// and nothing is listening.
func TestNATTAddrKeepsWellKnownPortInProduction(t *testing.T) {
	orig := ikeTestPortFn
	ikeTestPortFn = func() string { return "" }
	t.Cleanup(func() { ikeTestPortFn = orig })

	if got := nattAddr("192.0.2.7"); got != "192.0.2.7:4500" {
		t.Fatalf("nattAddr = %q, want 192.0.2.7:4500", got)
	}
	if got := nattAddr("0.0.0.0"); got != "0.0.0.0:4500" {
		t.Fatalf("nattAddr(any) = %q, want 0.0.0.0:4500", got)
	}
}

// TestNATTAddrFollowsTheTestPortSeam covers the other half: under the override
// the NAT-T socket must NOT take a host-wide well-known port, because the .ci
// suite runs several IKE daemon pairs at once and they all lose to whichever one
// binds first (linux CI run 31225029268).
//
// VALIDATES: the derived port is the one after the IKE port, and a garbage or
// out-of-range override falls back to 4500 rather than wrapping to 0.
// PREVENTS: concurrent functional tests contending for one UDP port.
func TestNATTAddrFollowsTheTestPortSeam(t *testing.T) {
	orig := ikeTestPortFn
	t.Cleanup(func() { ikeTestPortFn = orig })

	for _, tc := range []struct {
		name     string
		override string
		want     string
	}{
		{name: "next port after the IKE one", override: "45500", want: "127.0.0.2:45501"},
		{name: "garbage falls back", override: "not-a-port", want: "127.0.0.2:4500"},
		{name: "the last port cannot wrap to zero", override: "65535", want: "127.0.0.2:4500"},
		{name: "one below the last still derives", override: "65534", want: "127.0.0.2:65535"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ikeTestPortFn = func() string { return tc.override }
			if got := nattAddr("127.0.0.2"); got != tc.want {
				t.Fatalf("nattAddr(override %q) = %q, want %q", tc.override, got, tc.want)
			}
		})
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
