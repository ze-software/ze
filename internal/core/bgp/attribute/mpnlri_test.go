package attribute

import (
	"errors"
	"net/netip"
	"slices"
	"testing"
)

// TestMPReachNLRI_WriteTo checks that WriteTo emits the RFC 4760 Section 3 wire form.
//
// RFC requirement: RFC4760-3-1 positive -- WriteTo emits the Reserved octet as 0x00 at wire
// offset 4+NH_len; every case asserts that byte is 0x00, and the producer writes it as 0
// unconditionally (internal/core/bgp/attribute/mpnlri.go, MPReachNLRI.WriteTo).
//
// RFC requirement: RFC4760-3-2 positive -- WriteTo encodes the Network Address of Next Hop with
// a length that counts the octets it writes (NH_Len 0x10 for a 16-byte IPv6 hop, 0x0c/0x18 for VPN
// RD+IPv4/IPv6, 0x20 for the 32-byte global+link-local pair), the field that lets a receiver
// determine the next hop's network-layer protocol; each case asserts the NH_Len byte and the
// next-hop bytes (internal/core/bgp/attribute/mpnlri.go, MPReachNLRI.nextHopOctets and
// MPReachNLRI.WriteTo).
func TestMPReachNLRI_WriteTo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		attr     *MPReachNLRI
		expected []byte
	}{
		{
			name: "IPv6 unicast single next-hop",
			attr: &MPReachNLRI{
				AFI:      AFIIPv6,
				SAFI:     SAFIUnicast,
				NextHops: NewNextHopAddrs([]netip.Addr{netip.MustParseAddr("2001:db8::1")}),
				NLRI:     []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x01}, // 2001:db8:0:1::/64
			},
			expected: []byte{
				0x00, 0x02, // AFI IPv6
				0x01,                                                                                           // SAFI unicast
				0x10,                                                                                           // NH len = 16
				0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // next-hop
				0x00,                                               // reserved
				64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x01, // NLRI
			},
		},
		{
			// RFC 4364 Section 4.3.4: VPN next-hop is RD(8) + IPv4(4) = 12 bytes.
			// The RD is set to all zeros per RFC 4364.
			name: "IPv4 VPN",
			attr: &MPReachNLRI{
				AFI:      AFIIPv4,
				SAFI:     SAFIVPN,
				NextHops: NewNextHopAddrs([]netip.Addr{netip.MustParseAddr("10.0.0.1")}),
				NLRI:     []byte{0x01, 0x02, 0x03},
			},
			expected: []byte{
				0x00, 0x01, // AFI IPv4
				0x80,                                           // SAFI VPN (128)
				0x0c,                                           // NH len = 12 (8 RD + 4 IPv4)
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RD = 0 (per RFC 4364)
				0x0a, 0x00, 0x00, 0x01, // next-hop 10.0.0.1
				0x00,             // reserved
				0x01, 0x02, 0x03, // NLRI
			},
		},
		{
			// RFC 4659: VPN-IPv6 next-hop is RD(8) + IPv6(16) = 24 bytes.
			name: "IPv6 VPN",
			attr: &MPReachNLRI{
				AFI:      AFIIPv6,
				SAFI:     SAFIVPN,
				NextHops: NewNextHopAddrs([]netip.Addr{netip.MustParseAddr("2001:db8::1")}),
				NLRI:     []byte{0x01, 0x02},
			},
			expected: []byte{
				0x00, 0x02, // AFI IPv6
				0x80,                                           // SAFI VPN (128)
				0x18,                                           // NH len = 24 (8 RD + 16 IPv6)
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RD = 0 (per RFC 4364)
				0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // next-hop
				0x00,       // reserved
				0x01, 0x02, // NLRI
			},
		},
		{
			name: "IPv6 dual next-hop (global + link-local)",
			attr: &MPReachNLRI{
				AFI:  AFIIPv6,
				SAFI: SAFIUnicast,
				NextHops: NewNextHopAddrs([]netip.Addr{
					netip.MustParseAddr("2001:db8::1"),
					netip.MustParseAddr("fe80::1"),
				}),
				NLRI: nil,
			},
			expected: []byte{
				0x00, 0x02, // AFI IPv6
				0x01,                                                                                           // SAFI unicast
				0x20,                                                                                           // NH len = 32
				0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // global
				0xfe, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // link-local
				0x00, // reserved
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := make([]byte, 256)
			n := tt.attr.WriteTo(buf, 0)
			got := buf[:n]
			if len(got) != len(tt.expected) {
				t.Errorf("WriteTo() len = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("WriteTo()[%d] = 0x%02x, want 0x%02x", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

// TestParseMPReachNLRI checks that ParseMPReachNLRI reads the RFC 4760 Section 3 wire form.
//
// RFC requirement: RFC4760-3-2 positive -- ParseMPReachNLRI derives the next-hop count and family
// from the Length of Next Hop Address field (wantNHLen 1 for a 16-byte IPv6 hop, 2 for a 32-byte
// global+link-local pair), reading the length to determine the network-layer protocol of the next
// hop rather than assuming it from the NLRI AFI (internal/core/bgp/attribute/mpnlri.go, parseNextHops).
func TestParseMPReachNLRI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		data      []byte
		wantAFI   AFI
		wantSAFI  SAFI
		wantNHLen int
		wantNLRI  int
		wantErr   bool
	}{
		{
			name: "IPv6 unicast",
			data: []byte{
				0x00, 0x02, // AFI IPv6
				0x01,                                                                                           // SAFI unicast
				0x10,                                                                                           // NH len = 16
				0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // next-hop
				0x00,                       // reserved
				64, 0x20, 0x01, 0x0d, 0xb8, // NLRI start
			},
			wantAFI:   AFIIPv6,
			wantSAFI:  SAFIUnicast,
			wantNHLen: 1,
			wantNLRI:  5,
		},
		{
			name: "IPv6 dual next-hop",
			data: []byte{
				0x00, 0x02, // AFI IPv6
				0x01,                                                                                           // SAFI unicast
				0x20,                                                                                           // NH len = 32
				0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // global
				0xfe, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // link-local
				0x00, // reserved
			},
			wantAFI:   AFIIPv6,
			wantSAFI:  SAFIUnicast,
			wantNHLen: 2,
			wantNLRI:  0,
		},
		{
			name:    "too short",
			data:    []byte{0x00, 0x02, 0x01}, // Missing NH len
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, err := ParseMPReachNLRI(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMPReachNLRI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if m.AFI != tt.wantAFI {
				t.Errorf("AFI = %d, want %d", m.AFI, tt.wantAFI)
			}
			if m.SAFI != tt.wantSAFI {
				t.Errorf("SAFI = %d, want %d", m.SAFI, tt.wantSAFI)
			}
			if m.NextHops.Len() != tt.wantNHLen {
				t.Errorf("NextHops len = %d, want %d", m.NextHops.Len(), tt.wantNHLen)
			}
			if len(m.NLRI) != tt.wantNLRI {
				t.Errorf("NLRI len = %d, want %d", len(m.NLRI), tt.wantNLRI)
			}
		})
	}
}

func TestMPUnreachNLRI_WriteTo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		attr     *MPUnreachNLRI
		expected []byte
	}{
		{
			name: "IPv6 unicast withdraw",
			attr: &MPUnreachNLRI{
				AFI:  AFIIPv6,
				SAFI: SAFIUnicast,
				NLRI: []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x01},
			},
			expected: []byte{
				0x00, 0x02, // AFI IPv6
				0x01,                                               // SAFI unicast
				64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x01, // NLRI
			},
		},
		{
			name: "End-of-RIB marker",
			attr: &MPUnreachNLRI{
				AFI:  AFIIPv6,
				SAFI: SAFIUnicast,
				NLRI: nil,
			},
			expected: []byte{
				0x00, 0x02, // AFI IPv6
				0x01, // SAFI unicast
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := make([]byte, 256)
			n := tt.attr.WriteTo(buf, 0)
			got := buf[:n]
			if len(got) != len(tt.expected) {
				t.Errorf("WriteTo() len = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("WriteTo()[%d] = 0x%02x, want 0x%02x", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestParseMPUnreachNLRI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		data     []byte
		wantAFI  AFI
		wantSAFI SAFI
		wantNLRI int
		wantEOR  bool
		wantErr  bool
	}{
		{
			name: "IPv6 withdraw",
			data: []byte{
				0x00, 0x02, // AFI IPv6
				0x01,           // SAFI unicast
				64, 0x20, 0x01, // NLRI
			},
			wantAFI:  AFIIPv6,
			wantSAFI: SAFIUnicast,
			wantNLRI: 3,
			wantEOR:  false,
		},
		{
			name: "End-of-RIB",
			data: []byte{
				0x00, 0x02, // AFI IPv6
				0x01, // SAFI unicast
			},
			wantAFI:  AFIIPv6,
			wantSAFI: SAFIUnicast,
			wantNLRI: 0,
			wantEOR:  true,
		},
		{
			name:    "too short",
			data:    []byte{0x00, 0x02},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, err := ParseMPUnreachNLRI(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMPUnreachNLRI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if m.AFI != tt.wantAFI {
				t.Errorf("AFI = %d, want %d", m.AFI, tt.wantAFI)
			}
			if m.SAFI != tt.wantSAFI {
				t.Errorf("SAFI = %d, want %d", m.SAFI, tt.wantSAFI)
			}
			if len(m.NLRI) != tt.wantNLRI {
				t.Errorf("NLRI len = %d, want %d", len(m.NLRI), tt.wantNLRI)
			}
			if m.IsEndOfRIB() != tt.wantEOR {
				t.Errorf("IsEndOfRIB() = %v, want %v", m.IsEndOfRIB(), tt.wantEOR)
			}
		})
	}
}

// TestParseMPReachNLRI_ExtendedNextHop tests RFC 5549/8950 support.
//
// VALIDATES: IPv4 NLRI with IPv6 next-hop parses correctly when the
// Extended Next Hop capability is negotiated.
//
// PREVENTS: Parsing failures when receiving IPv4 routes with IPv6 next-hops
// over IPv6-only infrastructure.
//
// RFC requirement: RFC8950-3-1 positive -- an IPv4 NLRI (AFI=1) advertisement with a 16-byte
// next-hop is parsed as an IPv6 next-hop: parseNextHops keys off the Next Hop Length (16 -> IPv6)
// regardless of the NLRI AFI (internal/core/bgp/attribute/mpnlri.go, parseNextHops).
//
// RFC requirement: RFC5549-3-1 positive -- a receiver uses the Length of Next Hop Address field to
// determine the next-hop protocol: an IPv4 NLRI (AFI=1) with a 16-byte next-hop is decoded as an IPv6
// next-hop, parseNextHops keying off the length regardless of NLRI AFI (internal/core/bgp/attribute/mpnlri.go, parseNextHops).
func TestParseMPReachNLRI_ExtendedNextHop(t *testing.T) {
	t.Parallel()
	// RFC 5549 Section 3: IPv4 NLRI with 16-byte (IPv6) next-hop
	// This is used when advertising IPv4 routes over IPv6-only networks.
	//
	// RFC 5549 Section 3: "The BGP speaker receiving the advertisement MUST
	// use the Length of Next Hop Address field to determine which network-layer
	// protocol the next hop address belongs to."
	data := []byte{
		0x00, 0x01, // AFI IPv4
		0x01, // SAFI unicast
		0x10, // NH len = 16 (IPv6)
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // 2001:db8::1
		0x00,                   // reserved
		0x18, 0x0a, 0x00, 0x01, // 10.0.1.0/24
	}

	m, err := ParseMPReachNLRI(data)
	if err != nil {
		t.Fatalf("ParseMPReachNLRI() error = %v", err)
	}

	// Verify AFI/SAFI (NLRI family)
	if m.AFI != AFIIPv4 {
		t.Errorf("AFI = %d, want %d (IPv4)", m.AFI, AFIIPv4)
	}
	if m.SAFI != SAFIUnicast {
		t.Errorf("SAFI = %d, want %d (Unicast)", m.SAFI, SAFIUnicast)
	}

	// Verify IPv6 next-hop was parsed correctly
	if m.NextHops.Len() != 1 {
		t.Fatalf("NextHops len = %d, want 1", m.NextHops.Len())
	}
	if !m.NextHops.Slice()[0].Is6() {
		t.Errorf("NextHops[0] is not IPv6: %v", m.NextHops.Slice()[0])
	}
	expected := netip.MustParseAddr("2001:db8::1")
	if m.NextHops.Slice()[0] != expected {
		t.Errorf("NextHops[0] = %v, want %v", m.NextHops.Slice()[0], expected)
	}

	// Verify NLRI
	if len(m.NLRI) != 4 {
		t.Errorf("NLRI len = %d, want 4", len(m.NLRI))
	}
}

// TestParseMPReachNLRI_ExtendedNextHop_VPN tests RFC 5549 with VPN SAFI.
//
// VALIDATES: VPN-IPv4 NLRI with IPv6 next-hop parses correctly.
//
// PREVENTS: Parsing failures for VPN routes over IPv6 infrastructure.
//
// RFC requirement: RFC5549-3-1 positive -- a VPN-IPv4 NLRI (SAFI=128) with a legacy 16-byte next-hop
// (no Route Distinguisher) is decoded as an IPv6 next-hop by length, per RFC 5549's obsolete VPN
// encoding accepted for backwards compatibility (internal/core/bgp/attribute/mpnlri.go, parseVPNNextHops).
func TestParseMPReachNLRI_ExtendedNextHop_VPN(t *testing.T) {
	t.Parallel()
	// RFC 5549 Section 6.2: VPN-IPv4 NLRI with IPv6 next-hop
	data := []byte{
		0x00, 0x01, // AFI IPv4
		0x80, // SAFI VPN (128)
		0x10, // NH len = 16 (IPv6)
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, // 2001:db8::2
		0x00,             // reserved
		0x01, 0x02, 0x03, // VPN NLRI (simplified)
	}

	m, err := ParseMPReachNLRI(data)
	if err != nil {
		t.Fatalf("ParseMPReachNLRI() error = %v", err)
	}

	if m.AFI != AFIIPv4 {
		t.Errorf("AFI = %d, want %d", m.AFI, AFIIPv4)
	}
	if m.SAFI != SAFIVPN {
		t.Errorf("SAFI = %d, want %d", m.SAFI, SAFIVPN)
	}
	if m.NextHops.Len() != 1 {
		t.Fatalf("NextHops len = %d, want 1", m.NextHops.Len())
	}
	if !m.NextHops.Slice()[0].Is6() {
		t.Errorf("NextHops[0] is not IPv6: %v", m.NextHops.Slice()[0])
	}
}

// TestParseMPReachNLRI_ExtendedNextHop_DualStack tests IPv4 NLRI with
// global+link-local IPv6 next-hop per RFC 2545.
//
// VALIDATES: 32-byte next-hop (global+link-local) parses as two IPv6 addresses.
//
// PREVENTS: Incorrect parsing of dual-stack next-hop announcements.
//
// RFC requirement: RFC8950-3-1 positive -- an IPv4 NLRI advertisement with a 32-byte next-hop is
// parsed as two IPv6 addresses (global + link-local): parseNextHops selects the family from the
// Next Hop Length (32 -> dual IPv6) regardless of NLRI AFI (internal/core/bgp/attribute/mpnlri.go, parseNextHops).
//
// RFC requirement: RFC5549-3-1 positive -- the receiver uses the Length of Next Hop Address field: an
// IPv4 NLRI with a 32-byte next-hop is decoded as two IPv6 addresses (global + link-local), parseNextHops
// selecting the family from the length regardless of NLRI AFI (internal/core/bgp/attribute/mpnlri.go, parseNextHops).
func TestParseMPReachNLRI_ExtendedNextHop_DualStack(t *testing.T) {
	t.Parallel()
	// RFC 5549 Section 3 + RFC 2545: 32-byte next-hop = global + link-local
	data := []byte{
		0x00, 0x01, // AFI IPv4
		0x01, // SAFI unicast
		0x20, // NH len = 32 (global + link-local)
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // global 2001:db8::1
		0xfe, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // link-local fe80::1
		0x00,                   // reserved
		0x18, 0xc0, 0xa8, 0x01, // 192.168.1.0/24
	}

	m, err := ParseMPReachNLRI(data)
	if err != nil {
		t.Fatalf("ParseMPReachNLRI() error = %v", err)
	}

	if m.NextHops.Len() != 2 {
		t.Fatalf("NextHops len = %d, want 2", m.NextHops.Len())
	}
	if !m.NextHops.Slice()[0].Is6() {
		t.Errorf("NextHops[0] is not IPv6: %v", m.NextHops.Slice()[0])
	}
	if !m.NextHops.Slice()[1].Is6() {
		t.Errorf("NextHops[1] is not IPv6: %v", m.NextHops.Slice()[1])
	}
}

// TestParseMPReachNLRI_InvalidNextHopLength verifies that a next-hop length which maps to no
// valid network-layer protocol encoding is rejected.
//
// VALIDATES: RFC 8950 Section 3 -- the Next Hop Length field is authoritative and validated.
//
// PREVENTS: Silently mis-parsing an IPv4 NLRI next-hop whose length is neither a valid IPv4
// (multiple of 4) nor IPv6 (16 or 32) length.
//
// RFC requirement: RFC8950-3-1 negative -- for IPv4 NLRI a 5-byte next-hop is neither a valid
// IPv6 length (16/32) nor a multiple of 4, so parseNextHops rejects it with ErrInvalidNextHopLen
// (internal/core/bgp/attribute/mpnlri.go, parseNextHops). The length field determines the protocol and
// an unsupported length is not defaulted or ignored.
//
// RFC requirement: RFC5549-3-1 negative -- the Length of Next Hop Address field is authoritative: a
// 5-byte next-hop for IPv4 NLRI is neither a valid IPv6 length (16/32) nor a multiple of 4, so
// parseNextHops rejects it with ErrInvalidNextHopLen rather than defaulting by AFI
// (internal/core/bgp/attribute/mpnlri.go, parseNextHops).
//
// RFC requirement: RFC4760-3-2 negative -- a 5-byte next-hop for IPv4 NLRI maps to no valid
// network-layer protocol encoding (not an IPv6 16/32 length, not a multiple of 4), so parseNextHops
// rejects it with ErrInvalidNextHopLen rather than guessing the next hop's protocol
// (internal/core/bgp/attribute/mpnlri.go, parseNextHops).
func TestParseMPReachNLRI_InvalidNextHopLength(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x00, 0x01, // AFI IPv4
		0x01,                         // SAFI unicast
		0x05,                         // NH len = 5 (invalid: not 16/32, not a multiple of 4)
		0xAA, 0xBB, 0xCC, 0xDD, 0xEE, // 5 next-hop bytes
		0x00,                   // reserved
		0x18, 0xc0, 0x00, 0x02, // NLRI 192.0.2.0/24
	}

	m, err := ParseMPReachNLRI(data)
	if !errors.Is(err, ErrInvalidNextHopLen) {
		t.Fatalf("ParseMPReachNLRI() error = %v, want ErrInvalidNextHopLen", err)
	}
	if m != nil {
		t.Errorf("expected nil MPReachNLRI on invalid next-hop length, got %v", m)
	}
}

// TestParseMPReachNLRI_VPNIPv4NextHop tests VPN-IPv4 next-hop with RD prefix.
//
// VALIDATES: RFC 4364 Section 4.3.4 - VPN next-hop includes 8-byte RD prefix.
// For VPN-IPv4, next-hop is 12 bytes: RD(8) + IPv4(4).
//
// PREVENTS: Incorrect parsing of VPN routes, treating RD as part of IP address.
func TestParseMPReachNLRI_VPNIPv4NextHop(t *testing.T) {
	t.Parallel()
	// VPN-IPv4: AFI=1, SAFI=128
	// Next-hop: 8-byte RD (all zeros per RFC 4364) + 4-byte IPv4
	data := []byte{
		0x00, 0x01, // AFI IPv4
		0x80,                                           // SAFI VPN (128)
		0x0c,                                           // NH len = 12 (8 RD + 4 IPv4)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RD = 0 (per RFC 4364)
		0x0a, 0x00, 0x00, 0x01, // 10.0.0.1
		0x00,             // reserved
		0x01, 0x02, 0x03, // VPN NLRI (simplified)
	}

	m, err := ParseMPReachNLRI(data)
	if err != nil {
		t.Fatalf("ParseMPReachNLRI() error = %v", err)
	}

	if m.AFI != AFIIPv4 {
		t.Errorf("AFI = %d, want %d", m.AFI, AFIIPv4)
	}
	if m.SAFI != SAFIVPN {
		t.Errorf("SAFI = %d, want %d", m.SAFI, SAFIVPN)
	}

	// Should have exactly one next-hop (the IPv4 address, not the RD)
	if m.NextHops.Len() != 1 {
		t.Fatalf("NextHops len = %d, want 1", m.NextHops.Len())
	}

	// The next-hop should be the IPv4 address, not including the RD
	expected := netip.MustParseAddr("10.0.0.1")
	if m.NextHops.Slice()[0] != expected {
		t.Errorf("NextHops[0] = %v, want %v", m.NextHops.Slice()[0], expected)
	}
	if !m.NextHops.Slice()[0].Is4() {
		t.Errorf("NextHops[0] should be IPv4, got: %v", m.NextHops.Slice()[0])
	}
}

// TestParseMPReachNLRI_VPNWithIPv6NextHop tests VPN next-hop parsing with 24-byte format.
//
// VALIDATES: RFC 4659 (VPN-IPv6) and RFC 8950 (VPN-IPv4 with IPv6 NH) 24-byte formats.
// Both use RD(8) + IPv6(16) = 24 bytes.
//
// PREVENTS: Incorrect parsing of VPN routes with IPv6 next-hop.
//
// RFC requirement: RFC8950-3-2 positive -- a VPN next-hop of 24 bytes is decoded as an 8-byte
// Route Distinguisher (all zeros here) followed by a 16-byte IPv6 address; parseVPNNextHops skips
// the RD prefix and returns the IPv6 next-hop (internal/core/bgp/attribute/mpnlri.go, parseVPNNextHops).
func TestParseMPReachNLRI_VPNWithIPv6NextHop(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		afi      AFI
		afiBytes []byte
		wantNH   string
	}{
		{
			name:     "VPN-IPv6_RFC4659",
			afi:      AFIIPv6,
			afiBytes: []byte{0x00, 0x02},
			wantNH:   "2001:db8::1",
		},
		{
			name:     "VPN-IPv4_RFC8950",
			afi:      AFIIPv4,
			afiBytes: []byte{0x00, 0x01},
			wantNH:   "2001:db8::2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := netip.MustParseAddr(tt.wantNH)
			nhBytes := expected.AsSlice()

			// Build data: AFI(2) + SAFI(1) + NH_LEN(1) + RD(8) + IPv6(16) + Reserved(1) + NLRI
			data := make([]byte, 0, 32)
			data = append(data, tt.afiBytes...)                                             // AFI
			data = append(data, 0x80, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00) // SAFI VPN (128), NH len = 24, RD = 0
			data = append(data, nhBytes...)                                                 // IPv6 next-hop
			data = append(data, 0x00, 0x01, 0x02, 0x03)                                     // reserved + VPN NLRI (simplified)

			m, err := ParseMPReachNLRI(data)
			if err != nil {
				t.Fatalf("ParseMPReachNLRI() error = %v", err)
			}

			if m.AFI != tt.afi {
				t.Errorf("AFI = %d, want %d", m.AFI, tt.afi)
			}
			if m.SAFI != SAFIVPN {
				t.Errorf("SAFI = %d, want %d", m.SAFI, SAFIVPN)
			}
			if m.NextHops.Len() != 1 {
				t.Fatalf("NextHops len = %d, want 1", m.NextHops.Len())
			}
			if m.NextHops.Slice()[0] != expected {
				t.Errorf("NextHops[0] = %v, want %v", m.NextHops.Slice()[0], expected)
			}
			if !m.NextHops.Slice()[0].Is6() {
				t.Errorf("NextHops[0] should be IPv6, got: %v", m.NextHops.Slice()[0])
			}
		})
	}
}

func TestMPReachNLRI_RoundTrip(t *testing.T) {
	t.Parallel()
	original := &MPReachNLRI{
		AFI:      AFIIPv6,
		SAFI:     SAFIUnicast,
		NextHops: NewNextHopAddrs([]netip.Addr{netip.MustParseAddr("2001:db8::1")}),
		NLRI:     []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x01},
	}

	buf := make([]byte, 256)
	n := original.WriteTo(buf, 0)
	parsed, err := ParseMPReachNLRI(buf[:n])
	if err != nil {
		t.Fatalf("ParseMPReachNLRI() error = %v", err)
	}

	if parsed.AFI != original.AFI {
		t.Errorf("AFI = %d, want %d", parsed.AFI, original.AFI)
	}
	if parsed.SAFI != original.SAFI {
		t.Errorf("SAFI = %d, want %d", parsed.SAFI, original.SAFI)
	}
	if parsed.NextHops.Len() != original.NextHops.Len() {
		t.Errorf("NextHops len = %d, want %d", parsed.NextHops.Len(), original.NextHops.Len())
	}
	if len(parsed.NLRI) != len(original.NLRI) {
		t.Errorf("NLRI len = %d, want %d", len(parsed.NLRI), len(original.NLRI))
	}
}

// TestMPReachNLRI_RoundTrip_VPN verifies VPN next-hop round-trip: WriteTo adds
// the 8-byte RD prefix per RFC 4364, ParseMPReachNLRI strips it back to the IP.
//
// VALIDATES: RFC 4364 Section 4.3.4 VPN next-hop encoding round-trips correctly.
// PREVENTS: VPN routes rejected by GoBGP due to incorrect next-hop length.
//
// RFC requirement: RFC8950-3-2 positive -- WriteTo prefixes a VPN next-hop with an 8-byte
// all-zero Route Distinguisher (wire NH_Len = 12 = RD(8) + IPv4(4)), and ParseMPReachNLRI strips
// the RD back to the address; the RD is always written as zero (internal/core/bgp/attribute/mpnlri.go, MPReachNLRI.WriteTo).
func TestMPReachNLRI_RoundTrip_VPN(t *testing.T) {
	t.Parallel()
	original := &MPReachNLRI{
		AFI:      AFIIPv4,
		SAFI:     SAFIVPN,
		NextHops: NewNextHopAddrs([]netip.Addr{netip.MustParseAddr("10.0.0.1")}),
		NLRI:     []byte{0x01, 0x02, 0x03},
	}

	buf := make([]byte, 256)
	n := original.WriteTo(buf, 0)

	// Wire bytes must have NH_Len=12 (RD prefix included)
	if buf[3] != 12 {
		t.Fatalf("wire NH_Len = %d, want 12 (8 RD + 4 IPv4)", buf[3])
	}

	parsed, err := ParseMPReachNLRI(buf[:n])
	if err != nil {
		t.Fatalf("ParseMPReachNLRI() error = %v", err)
	}

	if parsed.AFI != original.AFI {
		t.Errorf("AFI = %d, want %d", parsed.AFI, original.AFI)
	}
	if parsed.SAFI != original.SAFI {
		t.Errorf("SAFI = %d, want %d", parsed.SAFI, original.SAFI)
	}
	if parsed.NextHops.Len() != 1 {
		t.Fatalf("NextHops len = %d, want 1", parsed.NextHops.Len())
	}
	if parsed.NextHops.Slice()[0] != original.NextHops.Slice()[0] {
		t.Errorf("NextHops[0] = %v, want %v", parsed.NextHops.Slice()[0], original.NextHops.Slice()[0])
	}
	if len(parsed.NLRI) != len(original.NLRI) {
		t.Errorf("NLRI len = %d, want %d", len(parsed.NLRI), len(original.NLRI))
	}
}

// TestMPReachNLRI_EncodeDecodeSymmetry verifies that WriteTo produces wire bytes
// whose next-hop length matches ValidNextHopLens for every supported AFI/SAFI,
// and that ParseMPReachNLRI round-trips them back to the original next-hop.
//
// VALIDATES: Encode/decode symmetry -- the encoder must produce formats the
// decoder accepts, and the valid-length table must agree with both sides.
// PREVENTS: Bugs like the VPN next-hop encoding (4 bytes instead of 12) where
// the write side silently produced an invalid format the parse side would reject.
func TestMPReachNLRI_EncodeDecodeSymmetry(t *testing.T) {
	t.Parallel()

	ipv4 := netip.MustParseAddr("10.0.0.1")
	ipv6 := netip.MustParseAddr("2001:db8::1")
	nlriData := []byte{0x18, 0x0a, 0x00, 0x01} // 10.0.1.0/24

	tests := []struct {
		name    string
		afi     AFI
		safi    SAFI
		nextHop netip.Addr
		wantNH  netip.Addr // expected after round-trip (same as nextHop)
	}{
		{"IPv4/unicast/v4nh", AFIIPv4, SAFIUnicast, ipv4, ipv4},
		{"IPv6/unicast/v6nh", AFIIPv6, SAFIUnicast, ipv6, ipv6},
		{"IPv4/VPN/v4nh", AFIIPv4, SAFIVPN, ipv4, ipv4},
		{"IPv6/VPN/v6nh", AFIIPv6, SAFIVPN, ipv6, ipv6},
		{"L2VPN/EVPN/v4nh", AFIL2VPN, SAFIEVPN, ipv4, ipv4},
		{"L2VPN/EVPN/v6nh", AFIL2VPN, SAFIEVPN, ipv6, ipv6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			original := &MPReachNLRI{
				AFI:      tt.afi,
				SAFI:     tt.safi,
				NextHops: NewNextHopAddrs([]netip.Addr{tt.nextHop}),
				NLRI:     nlriData,
			}

			// Encode
			buf := make([]byte, 256)
			n := original.WriteTo(buf, 0)

			// Check wire next-hop length against ValidNextHopLens
			wireNHLen := int(buf[3])
			validLens := ValidNextHopLens(tt.afi, tt.safi)
			if validLens != nil && !slices.Contains(validLens, wireNHLen) {
				t.Errorf("wire NH_Len=%d not in ValidNextHopLens(%d,%d)=%v",
					wireNHLen, tt.afi, tt.safi, validLens)
			}

			// Decode
			parsed, err := ParseMPReachNLRI(buf[:n])
			if err != nil {
				t.Fatalf("ParseMPReachNLRI() error = %v", err)
			}

			// Verify round-trip
			if parsed.AFI != tt.afi {
				t.Errorf("AFI = %d, want %d", parsed.AFI, tt.afi)
			}
			if parsed.SAFI != tt.safi {
				t.Errorf("SAFI = %d, want %d", parsed.SAFI, tt.safi)
			}
			if parsed.NextHops.Len() != 1 {
				t.Fatalf("NextHops len = %d, want 1", parsed.NextHops.Len())
			}
			if parsed.NextHops.Slice()[0] != tt.wantNH {
				t.Errorf("NextHops[0] = %v, want %v", parsed.NextHops.Slice()[0], tt.wantNH)
			}
		})
	}
}

// TestValidNextHopLens_Coverage ensures ValidNextHopLens returns non-nil for
// all AFI/SAFI combinations that Ze supports in encode/decode paths.
//
// VALIDATES: The valid-length table covers all families Ze handles.
// PREVENTS: Adding a new SAFI to encode/decode without updating the table.
func TestValidNextHopLens_Coverage(t *testing.T) {
	t.Parallel()

	expected := []struct {
		afi  AFI
		safi SAFI
		desc string
	}{
		{AFIIPv4, SAFIUnicast, "IPv4 unicast"},
		{AFIIPv4, SAFIMulticast, "IPv4 multicast"},
		{AFIIPv4, SAFIVPN, "IPv4 VPN"},
		{AFIIPv4, SAFIMPLSLabel, "IPv4 MPLS"},
		{AFIIPv6, SAFIUnicast, "IPv6 unicast"},
		{AFIIPv6, SAFIMulticast, "IPv6 multicast"},
		{AFIIPv6, SAFIVPN, "IPv6 VPN"},
		{AFIIPv6, SAFIMPLSLabel, "IPv6 MPLS"},
		{AFIL2VPN, SAFIEVPN, "L2VPN EVPN"},
		{AFIIPv4, SAFISRPolicy, "IPv4 SR-Policy"},
		{AFIIPv6, SAFISRPolicy, "IPv6 SR-Policy"},
	}

	for _, e := range expected {
		lens := ValidNextHopLens(e.afi, e.safi)
		if lens == nil {
			t.Errorf("ValidNextHopLens(%d, %d) [%s] = nil, want non-nil", e.afi, e.safi, e.desc)
		}
		if len(lens) == 0 {
			t.Errorf("ValidNextHopLens(%d, %d) [%s] is empty", e.afi, e.safi, e.desc)
		}
	}

	// Invalid combinations must return nil
	if lens := ValidNextHopLens(AFIIPv4, SAFIEVPN); lens != nil {
		t.Errorf("ValidNextHopLens(IPv4, EVPN) = %v, want nil (EVPN is L2VPN only)", lens)
	}
	if lens := ValidNextHopLens(AFIL2VPN, SAFIVPN); lens != nil {
		t.Errorf("ValidNextHopLens(L2VPN, VPN) = %v, want nil", lens)
	}
}

func TestMPUnreachNLRI_RoundTrip(t *testing.T) {
	t.Parallel()
	original := &MPUnreachNLRI{
		AFI:  AFIIPv6,
		SAFI: SAFIUnicast,
		NLRI: []byte{64, 0x20, 0x01, 0x0d, 0xb8},
	}

	buf := make([]byte, 256)
	n := original.WriteTo(buf, 0)
	parsed, err := ParseMPUnreachNLRI(buf[:n])
	if err != nil {
		t.Fatalf("ParseMPUnreachNLRI() error = %v", err)
	}

	if parsed.AFI != original.AFI {
		t.Errorf("AFI = %d, want %d", parsed.AFI, original.AFI)
	}
	if parsed.SAFI != original.SAFI {
		t.Errorf("SAFI = %d, want %d", parsed.SAFI, original.SAFI)
	}
	if len(parsed.NLRI) != len(original.NLRI) {
		t.Errorf("NLRI len = %d, want %d", len(parsed.NLRI), len(original.NLRI))
	}
}
