// VALIDATES: spec-ospfv3-3-ipv6-transport -- the OSPFv3 multicast group constants
// are byte-exact (ff02::5 AllSPFRouters, ff02::6 AllDRouters) and the protocol
// number is 89. PREVENTS a wrong group address (no adjacency) or an OSPFv2 IPv4
// group leaking into the IPv6 transport.

package transport

import (
	"net/netip"
	"testing"
)

func TestOSPFv3MulticastGroupConstants(t *testing.T) {
	if Protocol != 89 {
		t.Fatalf("Protocol = %d, want 89 (RFC 5340 §2.9)", Protocol)
	}

	wantSPF := [16]byte{0xff, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x05}
	wantDR := [16]byte{0xff, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x06}

	if got := AllSPFRouters.As16(); got != wantSPF {
		t.Fatalf("AllSPFRouters = % x, want % x (ff02::5)", got, wantSPF)
	}
	if got := AllDRouters.As16(); got != wantDR {
		t.Fatalf("AllDRouters = % x, want % x (ff02::6)", got, wantDR)
	}

	// Both groups are IPv6 multicast in the link-local (interface) scope.
	for name, g := range map[string]netip.Addr{"AllSPFRouters": AllSPFRouters, "AllDRouters": AllDRouters} {
		if !g.Is6() {
			t.Fatalf("%s is not an IPv6 address: %s", name, g)
		}
		if !g.IsMulticast() {
			t.Fatalf("%s is not multicast: %s", name, g)
		}
		if !g.IsLinkLocalMulticast() {
			t.Fatalf("%s is not link-local-scope multicast: %s", name, g)
		}
	}
}
