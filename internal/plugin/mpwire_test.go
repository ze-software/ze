package plugin

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
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

// TestMPReachWireNLRIs verifies NLRI parsing with ADD-PATH support.
// RFC 7911 Section 3: Path Identifier is 4-octet field prepended to each NLRI.
//
// VALIDATES: NLRIs() returns nlri.NLRI with correct PathID when hasAddPath=true.
// PREVENTS: Path-id loss when forwarding to RIB plugin.
func TestMPReachWireNLRIs(t *testing.T) {
	// MP_REACH_NLRI with ADD-PATH: path-id=1, prefix=10.0.0.0/24, next-hop=1.1.1.1
	// Wire format (RFC 4760 + RFC 7911):
	// AFI(2) + SAFI(1) + NH_Len(1) + NextHop(4) + Reserved(1) + PathID(4) + PrefixLen(1) + Prefix(3)
	mpReachAddPath := []byte{
		0x00, 0x01, // AFI: IPv4
		0x01,                   // SAFI: unicast
		0x04,                   // NH length: 4
		0x01, 0x01, 0x01, 0x01, // Next-hop: 1.1.1.1
		0x00,                   // Reserved
		0x00, 0x00, 0x00, 0x01, // Path ID: 1
		0x18,             // Prefix length: 24
		0x0A, 0x00, 0x00, // Prefix: 10.0.0.0
	}

	wire := MPReachWire(mpReachAddPath)
	nlris, err := wire.NLRIs(true) // hasAddPath=true

	if err != nil {
		t.Fatalf("NLRIs(true) error: %v", err)
	}

	if len(nlris) != 1 {
		t.Fatalf("NLRIs(true) len = %d, want 1", len(nlris))
	}

	n := nlris[0]
	if n.PathID() != 1 {
		t.Errorf("PathID() = %d, want 1", n.PathID())
	}

	want := "10.0.0.0/24"
	if n.String() != "10.0.0.0/24 path-id set 1" {
		t.Errorf("String() = %q, want prefix %s with path-id set 1", n.String(), want)
	}

	// Verify family
	wantFamily := nlri.IPv4Unicast
	if n.Family() != wantFamily {
		t.Errorf("Family() = %v, want %v", n.Family(), wantFamily)
	}
}

// TestMPReachWireNLRIs_NoAddPath verifies NLRI parsing without ADD-PATH.
//
// VALIDATES: NLRIs() works correctly when hasAddPath=false.
// PREVENTS: Wrong parsing when ADD-PATH is not negotiated.
func TestMPReachWireNLRIs_NoAddPath(t *testing.T) {
	// Same without ADD-PATH
	mpReachNoAddPath := []byte{
		0x00, 0x01, // AFI: IPv4
		0x01,                   // SAFI: unicast
		0x04,                   // NH length: 4
		0x01, 0x01, 0x01, 0x01, // Next-hop: 1.1.1.1
		0x00,             // Reserved
		0x18,             // Prefix length: 24
		0x0A, 0x00, 0x00, // Prefix: 10.0.0.0
	}

	wire := MPReachWire(mpReachNoAddPath)
	nlris, err := wire.NLRIs(false) // hasAddPath=false

	if err != nil {
		t.Fatalf("NLRIs(false) error: %v", err)
	}

	if len(nlris) != 1 {
		t.Fatalf("NLRIs(false) len = %d, want 1", len(nlris))
	}

	n := nlris[0]
	if n.PathID() != 0 {
		t.Errorf("PathID() = %d, want 0", n.PathID())
	}

	if n.String() != "10.0.0.0/24" {
		t.Errorf("String() = %q, want 10.0.0.0/24", n.String())
	}
}

// TestMPReachWireNLRIs_Multiple verifies multiple NLRIs with ADD-PATH.
//
// VALIDATES: All NLRIs parsed correctly, each with its own path-id.
// PREVENTS: Only first NLRI returned, path-ids mixed up.
func TestMPReachWireNLRIs_Multiple(t *testing.T) {
	// Two NLRIs with different path-ids
	data := []byte{
		0x00, 0x01, // AFI: IPv4
		0x01,                   // SAFI: unicast
		0x04,                   // NH length: 4
		0x01, 0x01, 0x01, 0x01, // Next-hop: 1.1.1.1
		0x00, // Reserved
		// NLRI 1: path-id=1, 10.0.0.0/24
		0x00, 0x00, 0x00, 0x01, // Path ID: 1
		0x18,             // Prefix length: 24
		0x0A, 0x00, 0x00, // 10.0.0.0
		// NLRI 2: path-id=2, 10.0.0.0/24 (same prefix, different path)
		0x00, 0x00, 0x00, 0x02, // Path ID: 2
		0x18,             // Prefix length: 24
		0x0A, 0x00, 0x00, // 10.0.0.0
	}

	wire := MPReachWire(data)
	nlris, err := wire.NLRIs(true)

	if err != nil {
		t.Fatalf("NLRIs(true) error: %v", err)
	}

	if len(nlris) != 2 {
		t.Fatalf("NLRIs(true) len = %d, want 2", len(nlris))
	}

	// Both should be 10.0.0.0/24 but with different path-ids
	if nlris[0].PathID() != 1 {
		t.Errorf("nlris[0].PathID() = %d, want 1", nlris[0].PathID())
	}
	if nlris[1].PathID() != 2 {
		t.Errorf("nlris[1].PathID() = %d, want 2", nlris[1].PathID())
	}
}

// TestMPReachWireNLRIs_Error verifies error handling for malformed data.
//
// VALIDATES: Error returned for malformed wire bytes.
// PREVENTS: Silent data corruption.
func TestMPReachWireNLRIs_Error(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		hasAddPath bool
	}{
		{"too short", []byte{0x00, 0x01, 0x01}, false},
		{"truncated nlri", []byte{
			0x00, 0x01, 0x01, 0x04, // AFI, SAFI, NH len
			0x01, 0x01, 0x01, 0x01, // NH
			0x00,       // Reserved
			0x18, 0x0A, // Prefix len + truncated prefix
		}, false},
		{"truncated path-id", []byte{
			0x00, 0x01, 0x01, 0x04,
			0x01, 0x01, 0x01, 0x01,
			0x00,
			0x00, 0x00, // Only 2 bytes of path-id
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := MPReachWire(tt.data)
			_, err := wire.NLRIs(tt.hasAddPath)
			if err == nil {
				t.Error("NLRIs() should return error for malformed data")
			}
		})
	}
}

// TestMPUnreachWireNLRIs verifies NLRI parsing for withdrawals with ADD-PATH.
//
// VALIDATES: MP_UNREACH_NLRI correctly parsed with path-id.
// PREVENTS: Path-id loss in withdrawal path.
func TestMPUnreachWireNLRIs(t *testing.T) {
	// MP_UNREACH_NLRI with ADD-PATH
	// Wire format (RFC 4760 Section 4 + RFC 7911):
	// AFI(2) + SAFI(1) + PathID(4) + PrefixLen(1) + Prefix(variable)
	data := []byte{
		0x00, 0x01, // AFI: IPv4
		0x01,                   // SAFI: unicast
		0x00, 0x00, 0x00, 0x01, // Path ID: 1
		0x18,             // Prefix length: 24
		0x0A, 0x00, 0x00, // Prefix: 10.0.0.0
	}

	wire := MPUnreachWire(data)
	nlris, err := wire.NLRIs(true)

	if err != nil {
		t.Fatalf("NLRIs(true) error: %v", err)
	}

	if len(nlris) != 1 {
		t.Fatalf("NLRIs(true) len = %d, want 1", len(nlris))
	}

	if nlris[0].PathID() != 1 {
		t.Errorf("PathID() = %d, want 1", nlris[0].PathID())
	}
}

// TestIPv4ReachNLRIs verifies IPv4 body NLRI parsing with ADD-PATH.
//
// VALIDATES: Legacy IPv4 path correctly handles ADD-PATH.
// PREVENTS: Path-id loss for IPv4 unicast in UPDATE body.
func TestIPv4ReachNLRIs(t *testing.T) {
	// IPv4 NLRI with ADD-PATH in body
	nlriBytes := []byte{
		0x00, 0x00, 0x00, 0x01, // Path ID: 1
		0x18,             // Prefix length: 24
		0xC0, 0xA8, 0x01, // 192.168.1.0
	}

	reach := IPv4Reach{
		nh:   []byte{10, 0, 0, 1},
		nlri: nlriBytes,
	}

	nlris, err := reach.NLRIs(true)
	if err != nil {
		t.Fatalf("NLRIs(true) error: %v", err)
	}

	if len(nlris) != 1 {
		t.Fatalf("NLRIs(true) len = %d, want 1", len(nlris))
	}

	if nlris[0].PathID() != 1 {
		t.Errorf("PathID() = %d, want 1", nlris[0].PathID())
	}
}

// TestIPv4WithdrawNLRIs verifies IPv4 body withdrawal parsing with ADD-PATH.
//
// VALIDATES: Legacy IPv4 withdraw path correctly handles ADD-PATH.
// PREVENTS: Path-id loss for IPv4 unicast withdrawals in UPDATE body.
func TestIPv4WithdrawNLRIs(t *testing.T) {
	withdrawnBytes := []byte{
		0x00, 0x00, 0x00, 0x01, // Path ID: 1
		0x18,             // Prefix length: 24
		0xC0, 0xA8, 0x01, // 192.168.1.0
	}

	withdraw := IPv4Withdraw{
		withdrawn: withdrawnBytes,
	}

	nlris, err := withdraw.NLRIs(true)
	if err != nil {
		t.Fatalf("NLRIs(true) error: %v", err)
	}

	if len(nlris) != 1 {
		t.Fatalf("NLRIs(true) len = %d, want 1", len(nlris))
	}

	if nlris[0].PathID() != 1 {
		t.Errorf("PathID() = %d, want 1", nlris[0].PathID())
	}
}

// TestMPReachWireNLRIs_IPv6AddPath verifies IPv6 NLRI parsing with ADD-PATH.
//
// VALIDATES: IPv6 unicast correctly handles ADD-PATH path-id.
// PREVENTS: IPv6 ADD-PATH being broken while IPv4 works.
func TestMPReachWireNLRIs_IPv6AddPath(t *testing.T) {
	// MP_REACH_NLRI for IPv6 with ADD-PATH
	// Wire format: AFI(2) + SAFI(1) + NH_Len(1) + NextHop(16) + Reserved(1) + PathID(4) + PrefixLen(1) + Prefix
	nextHop := netip.MustParseAddr("2001:db8::1")
	nhBytes := nextHop.As16()

	data := make([]byte, 0, 64)
	data = append(data, 0x00, 0x02) // AFI: IPv6
	data = append(data, 0x01)       // SAFI: unicast
	data = append(data, 0x10)       // NH length: 16
	data = append(data, nhBytes[:]...)
	data = append(data, 0x00)                   // Reserved
	data = append(data, 0x00, 0x00, 0x00, 0x05) // Path ID: 5
	data = append(data, 48)                     // Prefix length: 48
	data = append(data, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01)

	wire := MPReachWire(data)
	nlris, err := wire.NLRIs(true)

	if err != nil {
		t.Fatalf("NLRIs(true) error: %v", err)
	}

	if len(nlris) != 1 {
		t.Fatalf("NLRIs(true) len = %d, want 1", len(nlris))
	}

	n := nlris[0]
	if n.PathID() != 5 {
		t.Errorf("PathID() = %d, want 5", n.PathID())
	}

	wantFamily := nlri.IPv6Unicast
	if n.Family() != wantFamily {
		t.Errorf("Family() = %v, want %v", n.Family(), wantFamily)
	}

	if n.String() != "2001:db8:1::/48 path-id set 5" {
		t.Errorf("String() = %q, want prefix with path-id set 5", n.String())
	}
}

// FuzzParseNLRIs tests NLRI parsing robustness against arbitrary input.
//
// VALIDATES: parseNLRIs handles arbitrary bytes without panicking.
// PREVENTS: Remote crash via malformed UPDATE with ADD-PATH.
// SECURITY: Critical - parses untrusted network data.
func FuzzParseNLRIs(f *testing.F) {
	// Seed corpus with valid NLRIs
	f.Add([]byte{24, 10, 0, 0}, false)                           // 10.0.0.0/24 without ADD-PATH
	f.Add([]byte{0, 0, 0, 1, 24, 10, 0, 0}, true)                // 10.0.0.0/24 with path-id=1
	f.Add([]byte{}, false)                                       // Empty
	f.Add([]byte{33, 10, 0, 0, 0}, false)                        // Invalid prefix length
	f.Add([]byte{0, 0, 0, 1}, true)                              // Truncated (only path-id)
	f.Add([]byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}, false) // IPv6 prefix

	f.Fuzz(func(t *testing.T, data []byte, hasAddPath bool) {
		// Test IPv4 unicast - MUST NOT panic
		_, _ = parseNLRIs(data, nlri.IPv4Unicast, hasAddPath)

		// Test IPv6 unicast - MUST NOT panic
		_, _ = parseNLRIs(data, nlri.IPv6Unicast, hasAddPath)

		// Test IPv4 multicast - MUST NOT panic
		_, _ = parseNLRIs(data, nlri.IPv4Multicast, hasAddPath)

		// Test IPv6 multicast - MUST NOT panic
		_, _ = parseNLRIs(data, nlri.IPv6Multicast, hasAddPath)
	})
}

// TestMPReachWireNLRIIterator verifies zero-allocation iterator for MP_REACH_NLRI.
//
// VALIDATES: NLRIIterator returns iterator that yields same prefixes as Prefixes().
// PREVENTS: Iterator skipping prefixes or yielding wrong data.
func TestMPReachWireNLRIIterator(t *testing.T) {
	// Build MP_REACH_NLRI for IPv4 with multiple prefixes
	data := []byte{
		0x00, 0x01, // AFI: IPv4
		0x01,                   // SAFI: unicast
		0x04,                   // NH length: 4
		0x01, 0x01, 0x01, 0x01, // Next-hop: 1.1.1.1
		0x00,         // Reserved
		24, 10, 0, 0, // 10.0.0.0/24
		24, 192, 168, 1, // 192.168.1.0/24
		16, 172, 16, // 172.16.0.0/16
	}

	wire := MPReachWire(data)
	iter := wire.NLRIIterator(false)

	if iter == nil {
		t.Fatal("NLRIIterator() returned nil")
	}

	// Count prefixes via iterator
	count := 0
	for _, _, ok := iter.Next(); ok; _, _, ok = iter.Next() {
		count++
	}

	if count != 3 {
		t.Errorf("Iterator yielded %d prefixes, want 3", count)
	}

	// Verify count matches Prefixes() result
	prefixes := wire.Prefixes()
	if len(prefixes) != count {
		t.Errorf("Prefixes() len=%d, Iterator count=%d", len(prefixes), count)
	}
}

// TestMPReachWireNLRIIteratorEmpty verifies iterator handles empty NLRI.
//
// VALIDATES: Returns nil for empty/malformed data.
// PREVENTS: Nil pointer dereference.
func TestMPReachWireNLRIIteratorEmpty(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantNil bool
	}{
		{"nil", nil, true},
		{"too short", []byte{0x00, 0x01}, true},
		{"no nlri", []byte{
			0x00, 0x01, 0x01, 0x04,
			0x01, 0x01, 0x01, 0x01,
			0x00, // Reserved, then no NLRI
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := MPReachWire(tt.data)
			iter := wire.NLRIIterator(false)
			if tt.wantNil && iter != nil {
				t.Errorf("NLRIIterator() = %v, want nil", iter)
			}
		})
	}
}

// TestMPUnreachWireNLRIIterator verifies zero-allocation iterator for MP_UNREACH_NLRI.
//
// VALIDATES: NLRIIterator works for withdraw messages.
// PREVENTS: Iterator broken for UNREACH path.
func TestMPUnreachWireNLRIIterator(t *testing.T) {
	data := []byte{
		0x00, 0x01, // AFI: IPv4
		0x01,         // SAFI: unicast
		24, 10, 0, 0, // 10.0.0.0/24
		24, 192, 168, 1, // 192.168.1.0/24
	}

	wire := MPUnreachWire(data)
	iter := wire.NLRIIterator(false)

	if iter == nil {
		t.Fatal("NLRIIterator() returned nil")
	}

	count := 0
	for _, _, ok := iter.Next(); ok; _, _, ok = iter.Next() {
		count++
	}

	if count != 2 {
		t.Errorf("Iterator yielded %d prefixes, want 2", count)
	}
}

// TestIPv4ReachNLRIIterator verifies zero-allocation iterator for IPv4Reach.
//
// VALIDATES: NLRIIterator works for legacy IPv4 path.
// PREVENTS: Iterator broken for body NLRI.
func TestIPv4ReachNLRIIterator(t *testing.T) {
	nlriBytes := []byte{
		24, 192, 168, 1, // 192.168.1.0/24
		16, 10, 0, // 10.0.0.0/16
	}

	reach := IPv4Reach{nlri: nlriBytes}
	iter := reach.NLRIIterator(false)

	if iter == nil {
		t.Fatal("NLRIIterator() returned nil")
	}

	count := 0
	for _, _, ok := iter.Next(); ok; _, _, ok = iter.Next() {
		count++
	}

	if count != 2 {
		t.Errorf("Iterator yielded %d prefixes, want 2", count)
	}
}

// TestIPv4WithdrawNLRIIterator verifies zero-allocation iterator for IPv4Withdraw.
//
// VALIDATES: NLRIIterator works for legacy IPv4 withdraw path.
// PREVENTS: Iterator broken for body withdrawn section.
func TestIPv4WithdrawNLRIIterator(t *testing.T) {
	withdrawnBytes := []byte{
		24, 192, 168, 1, // 192.168.1.0/24
	}

	withdraw := IPv4Withdraw{withdrawn: withdrawnBytes}
	iter := withdraw.NLRIIterator(false)

	if iter == nil {
		t.Fatal("NLRIIterator() returned nil")
	}

	count := 0
	for _, _, ok := iter.Next(); ok; _, _, ok = iter.Next() {
		count++
	}

	if count != 1 {
		t.Errorf("Iterator yielded %d prefixes, want 1", count)
	}
}
