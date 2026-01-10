package plugin

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// TestMPReachWireIPv6 verifies IPv6 unicast MP_REACH_NLRI parsing.
//
// VALIDATES: AFI, SAFI, NextHop, Prefixes correctly extracted from wire.
// PREVENTS: Off-by-one errors in offset calculations.
func TestMPReachWireIPv6(t *testing.T) {
	// Build MP_REACH_NLRI for IPv6 unicast with:
	// - AFI=2 (IPv6), SAFI=1 (unicast)
	// - Next-hop: 2001:db8::1 (16 bytes)
	// - NLRI: 2001:db8:1::/48

	// Wire format (RFC 4760 Section 3):
	// AFI (2) + SAFI (1) + NH_Len (1) + NextHop (16) + Reserved (1) + NLRI
	nextHop := netip.MustParseAddr("2001:db8::1")
	nhBytes := nextHop.As16()

	data := make([]byte, 0, 64)
	// AFI = 2 (IPv6)
	data = append(data, 0x00, 0x02)
	// SAFI = 1 (unicast)
	data = append(data, 0x01)
	// Next-hop length = 16
	data = append(data, 0x10)
	// Next-hop address
	data = append(data, nhBytes[:]...)
	// Reserved = 0
	data = append(data, 0x00)
	// NLRI: 2001:db8:1::/48 = prefix_len(48) + 6 bytes
	data = append(data, 48)                                 // prefix length
	data = append(data, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01) // 2001:db8:1::

	wire := MPReachWire(data)

	// Test AFI
	if afi := wire.AFI(); afi != 2 {
		t.Errorf("AFI() = %d, want 2", afi)
	}

	// Test SAFI
	if safi := wire.SAFI(); safi != 1 {
		t.Errorf("SAFI() = %d, want 1", safi)
	}

	// Test Family
	wantFamily := nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
	if family := wire.Family(); family != wantFamily {
		t.Errorf("Family() = %v, want %v", family, wantFamily)
	}

	// Test NextHop
	if nh := wire.NextHop(); nh != nextHop {
		t.Errorf("NextHop() = %s, want %s", nh, nextHop)
	}

	// Test Prefixes
	prefixes := wire.Prefixes()
	if len(prefixes) != 1 {
		t.Fatalf("Prefixes() len = %d, want 1", len(prefixes))
	}
	wantPrefix := netip.MustParsePrefix("2001:db8:1::/48")
	if prefixes[0] != wantPrefix {
		t.Errorf("Prefixes()[0] = %s, want %s", prefixes[0], wantPrefix)
	}
}

// TestMPReachWireIPv4 verifies IPv4 unicast MP_REACH_NLRI parsing.
//
// VALIDATES: AFI=1 path works correctly.
// PREVENTS: IPv4/IPv6 confusion.
func TestMPReachWireIPv4(t *testing.T) {
	// MP_REACH_NLRI for IPv4 unicast (used when IPv4 is sent via MP extension)
	// - AFI=1, SAFI=1
	// - Next-hop: 10.0.0.1 (4 bytes)
	// - NLRI: 192.168.1.0/24

	nextHop := netip.MustParseAddr("10.0.0.1")
	nhBytes := nextHop.As4()

	data := make([]byte, 0, 32)
	// AFI = 1 (IPv4)
	data = append(data, 0x00, 0x01)
	// SAFI = 1 (unicast)
	data = append(data, 0x01)
	// Next-hop length = 4
	data = append(data, 0x04)
	// Next-hop address
	data = append(data, nhBytes[:]...)
	// Reserved = 0
	data = append(data, 0x00)
	// NLRI: 192.168.1.0/24 = prefix_len(24) + 3 bytes
	data = append(data, 24)          // prefix length
	data = append(data, 192, 168, 1) // 192.168.1.0

	wire := MPReachWire(data)

	if afi := wire.AFI(); afi != 1 {
		t.Errorf("AFI() = %d, want 1", afi)
	}

	if safi := wire.SAFI(); safi != 1 {
		t.Errorf("SAFI() = %d, want 1", safi)
	}

	if nh := wire.NextHop(); nh != nextHop {
		t.Errorf("NextHop() = %s, want %s", nh, nextHop)
	}

	prefixes := wire.Prefixes()
	if len(prefixes) != 1 {
		t.Fatalf("Prefixes() len = %d, want 1", len(prefixes))
	}
	wantPrefix := netip.MustParsePrefix("192.168.1.0/24")
	if prefixes[0] != wantPrefix {
		t.Errorf("Prefixes()[0] = %s, want %s", prefixes[0], wantPrefix)
	}
}

// TestMPReachWireMultiplePrefixes verifies multiple NLRI parsing.
//
// VALIDATES: All prefixes extracted from NLRI section.
// PREVENTS: Only first prefix returned.
func TestMPReachWireMultiplePrefixes(t *testing.T) {
	nextHop := netip.MustParseAddr("2001:db8::1")
	nhBytes := nextHop.As16()

	data := make([]byte, 0, 64)
	data = append(data, 0x00, 0x02) // AFI=2
	data = append(data, 0x01)       // SAFI=1
	data = append(data, 0x10)       // NH len=16
	data = append(data, nhBytes[:]...)
	data = append(data, 0x00) // Reserved

	// NLRI 1: 2001:db8:1::/48
	data = append(data, 48)
	data = append(data, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01)

	// NLRI 2: 2001:db8:2::/48
	data = append(data, 48)
	data = append(data, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02)

	// NLRI 3: 2001:db8:3::/64
	data = append(data, 64)
	data = append(data, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x03, 0x00, 0x00)

	wire := MPReachWire(data)
	prefixes := wire.Prefixes()

	if len(prefixes) != 3 {
		t.Fatalf("Prefixes() len = %d, want 3", len(prefixes))
	}

	want := []string{"2001:db8:1::/48", "2001:db8:2::/48", "2001:db8:3::/64"}
	for i, p := range prefixes {
		if p.String() != want[i] {
			t.Errorf("Prefixes()[%d] = %s, want %s", i, p, want[i])
		}
	}
}

// TestMPReachWireEmpty verifies empty/short data handling.
//
// VALIDATES: No panic on malformed data.
// PREVENTS: Index out of bounds.
func TestMPReachWireEmpty(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"too short", []byte{0x00}},
		{"just AFI", []byte{0x00, 0x02}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := MPReachWire(tt.data)

			// Should not panic
			_ = wire.AFI()
			_ = wire.SAFI()
			_ = wire.NextHop()
			_ = wire.Prefixes()
		})
	}
}

// TestMPUnreachWireIPv6 verifies IPv6 unicast MP_UNREACH_NLRI parsing.
//
// VALIDATES: AFI, SAFI, Prefixes correctly extracted.
// PREVENTS: Wrong offset (no next-hop in UNREACH).
func TestMPUnreachWireIPv6(t *testing.T) {
	// MP_UNREACH_NLRI format (RFC 4760 Section 4):
	// AFI (2) + SAFI (1) + Withdrawn Routes

	data := make([]byte, 0, 32)
	data = append(data, 0x00, 0x02) // AFI=2 (IPv6)
	data = append(data, 0x01)       // SAFI=1 (unicast)

	// Withdrawn: 2001:db8:1::/48
	data = append(data, 48)
	data = append(data, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01)

	wire := MPUnreachWire(data)

	if afi := wire.AFI(); afi != 2 {
		t.Errorf("AFI() = %d, want 2", afi)
	}

	if safi := wire.SAFI(); safi != 1 {
		t.Errorf("SAFI() = %d, want 1", safi)
	}

	wantFamily := nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
	if family := wire.Family(); family != wantFamily {
		t.Errorf("Family() = %v, want %v", family, wantFamily)
	}

	prefixes := wire.Prefixes()
	if len(prefixes) != 1 {
		t.Fatalf("Prefixes() len = %d, want 1", len(prefixes))
	}

	wantPrefix := netip.MustParsePrefix("2001:db8:1::/48")
	if prefixes[0] != wantPrefix {
		t.Errorf("Prefixes()[0] = %s, want %s", prefixes[0], wantPrefix)
	}
}

// TestMPUnreachWireEmpty verifies empty data handling.
//
// VALIDATES: No panic on malformed data.
// PREVENTS: Index out of bounds.
func TestMPUnreachWireEmpty(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"just AFI", []byte{0x00, 0x02}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := MPUnreachWire(tt.data)
			_ = wire.AFI()
			_ = wire.SAFI()
			_ = wire.Prefixes()
		})
	}
}

// TestIPv4ReachNextHop verifies IPv4 legacy path next-hop extraction.
//
// VALIDATES: Next-hop parsed from NEXT_HOP attribute bytes.
// PREVENTS: Wrong byte order or offset.
func TestIPv4ReachNextHop(t *testing.T) {
	// NEXT_HOP attribute value is just 4 bytes of IPv4 address
	nhBytes := []byte{10, 0, 0, 1}       // 10.0.0.1
	nlriBytes := []byte{24, 192, 168, 1} // 192.168.1.0/24

	reach := IPv4Reach{
		nh:   nhBytes,
		nlri: nlriBytes,
	}

	nh := reach.NextHop()
	want := netip.MustParseAddr("10.0.0.1")
	if nh != want {
		t.Errorf("NextHop() = %s, want %s", nh, want)
	}
}

// TestIPv4ReachPrefixes verifies IPv4 legacy path prefix extraction.
//
// VALIDATES: Prefixes parsed from body NLRI section.
// PREVENTS: Wrong prefix parsing.
func TestIPv4ReachPrefixes(t *testing.T) {
	nlriBytes := []byte{
		24, 192, 168, 1, // 192.168.1.0/24
		16, 10, 0, // 10.0.0.0/16
		32, 1, 2, 3, 4, // 1.2.3.4/32
	}

	reach := IPv4Reach{
		nlri: nlriBytes,
	}

	prefixes := reach.Prefixes()
	if len(prefixes) != 3 {
		t.Fatalf("Prefixes() len = %d, want 3", len(prefixes))
	}

	want := []string{"192.168.1.0/24", "10.0.0.0/16", "1.2.3.4/32"}
	for i, p := range prefixes {
		if p.String() != want[i] {
			t.Errorf("Prefixes()[%d] = %s, want %s", i, p, want[i])
		}
	}
}

// TestIPv4ReachEmpty verifies empty data handling.
//
// VALIDATES: No panic, valid zero values returned.
// PREVENTS: Nil pointer dereference.
func TestIPv4ReachEmpty(t *testing.T) {
	reach := IPv4Reach{}

	nh := reach.NextHop()
	if nh.IsValid() {
		t.Errorf("NextHop() should be invalid for empty reach")
	}

	prefixes := reach.Prefixes()
	if len(prefixes) != 0 {
		t.Errorf("Prefixes() len = %d, want 0", len(prefixes))
	}
}
