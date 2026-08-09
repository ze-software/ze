// Design: docs/architecture/ldp/mpls-ldp.md -- local FEC origination tests (AC-3)
package ldp

import (
	"net"
	"net/netip"
	"testing"
)

// VALIDATES: AC-3 -- the LSR-ID is advertised as a host route and listed first,
// connected prefixes are normalised to their network address.
func TestLocalFECsLSRIDFirst(t *testing.T) {
	lsrID := netip.MustParseAddr("10.0.0.1")
	connected := []netip.Prefix{netip.MustParsePrefix("192.168.1.5/24")}

	fecs := localFECs(lsrID, connected)

	if len(fecs) != 2 {
		t.Fatalf("localFECs returned %d, want 2", len(fecs))
	}
	if fecs[0] != netip.MustParsePrefix("10.0.0.1/32") {
		t.Errorf("first FEC = %s, want 10.0.0.1/32 (LSR-ID host route)", fecs[0])
	}
	if fecs[1] != netip.MustParsePrefix("192.168.1.0/24") {
		t.Errorf("connected FEC = %s, want masked 192.168.1.0/24", fecs[1])
	}
}

// VALIDATES: duplicate prefixes (including LSR-ID overlapping a connected /32)
// collapse to a single FEC.
func TestLocalFECsDedup(t *testing.T) {
	lsrID := netip.MustParseAddr("10.0.0.1")
	connected := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParsePrefix("192.168.1.7/24"), // same network as above
	}

	fecs := localFECs(lsrID, connected)

	if len(fecs) != 2 {
		t.Fatalf("localFECs returned %d, want 2 (deduped)", len(fecs))
	}
}

// VALIDATES: an invalid LSR-ID is skipped without producing a bogus FEC.
func TestLocalFECsNoLSRID(t *testing.T) {
	connected := []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")}

	fecs := localFECs(netip.Addr{}, connected)

	if len(fecs) != 1 {
		t.Fatalf("localFECs returned %d, want 1", len(fecs))
	}
	if fecs[0] != netip.MustParsePrefix("192.168.1.0/24") {
		t.Errorf("FEC = %s, want 192.168.1.0/24", fecs[0])
	}
}

// VALIDATES: AC-3 -- non-advertisable addresses (loopback, link-local,
// unspecified, multicast) are rejected as FECs.
func TestPrefixFromIPNetFilters(t *testing.T) {
	tests := []struct {
		name string
		cidr string
		want bool
	}{
		{"global v4", "192.168.1.0/24", true},
		{"global v6", "2001:db8::/64", true},
		{"loopback v4", "127.0.0.1/8", false},
		{"loopback v6", "::1/128", false},
		{"link-local v4", "169.254.1.1/16", false},
		{"link-local v6", "fe80::1/64", false},
		{"unspecified", "0.0.0.0/0", false},
		{"multicast", "224.0.0.1/4", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipNet, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatalf("ParseCIDR(%q): %v", tt.cidr, err)
			}
			// net.ParseCIDR returns the masked network in ipNet.IP; restore the
			// host address so the filter sees the real interface address.
			ipNet.IP = mustHostIP(t, tt.cidr)
			_, ok := prefixFromIPNet(ipNet)
			if ok != tt.want {
				t.Errorf("prefixFromIPNet(%q) ok = %v, want %v", tt.cidr, ok, tt.want)
			}
		})
	}
}

// VALIDATES: AC-4 -- next-hop selection prefers a directly-connected transport
// address, falls back to a connected peer interface address, then to the transport.
func TestPickNextHop(t *testing.T) {
	local := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}
	transport := netip.MustParseAddr("10.0.0.1")
	peerAddrs := []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("192.0.2.1")}

	t.Run("connected transport wins", func(t *testing.T) {
		got := pickNextHop(transport, peerAddrs, local)
		if got != transport {
			t.Errorf("got %s, want %s (connected transport)", got, transport)
		}
	})

	t.Run("falls back to connected peer address", func(t *testing.T) {
		// Transport is a loopback the peer advertises but is not on a local subnet;
		// one of the peer's interface addresses is on the connected /24.
		loopbackTransport := netip.MustParseAddr("192.0.2.9")
		addrs := []netip.Addr{netip.MustParseAddr("203.0.113.5"), netip.MustParseAddr("10.0.0.2")}
		got := pickNextHop(loopbackTransport, addrs, local)
		if got != netip.MustParseAddr("10.0.0.2") {
			t.Errorf("got %s, want 10.0.0.2 (connected peer address)", got)
		}
	})

	t.Run("falls back to transport when nothing connected", func(t *testing.T) {
		off := netip.MustParseAddr("198.51.100.1")
		got := pickNextHop(off, []netip.Addr{netip.MustParseAddr("203.0.113.1")}, local)
		if got != off {
			t.Errorf("got %s, want %s (transport fallback)", got, off)
		}
	})
}

// mustHostIP returns the host portion of a CIDR (the address before the slash),
// since net.ParseCIDR masks it away in the returned IPNet.
func mustHostIP(t *testing.T, cidr string) net.IP {
	t.Helper()
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", cidr, err)
	}
	return ip
}
