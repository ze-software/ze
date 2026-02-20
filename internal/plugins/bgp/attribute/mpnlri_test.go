package attribute

import (
	"net/netip"
	"testing"
)

func TestMPReachNLRI_WriteTo(t *testing.T) {
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
				NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
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
			name: "IPv4 VPN",
			attr: &MPReachNLRI{
				AFI:      AFIIPv4,
				SAFI:     SAFIVPN,
				NextHops: []netip.Addr{netip.MustParseAddr("10.0.0.1")},
				NLRI:     []byte{0x01, 0x02, 0x03},
			},
			expected: []byte{
				0x00, 0x01, // AFI IPv4
				0x80,                   // SAFI VPN (128)
				0x04,                   // NH len = 4
				0x0a, 0x00, 0x00, 0x01, // next-hop 10.0.0.1
				0x00,             // reserved
				0x01, 0x02, 0x03, // NLRI
			},
		},
		{
			name: "IPv6 dual next-hop (global + link-local)",
			attr: &MPReachNLRI{
				AFI:  AFIIPv6,
				SAFI: SAFIUnicast,
				NextHops: []netip.Addr{
					netip.MustParseAddr("2001:db8::1"),
					netip.MustParseAddr("fe80::1"),
				},
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

func TestParseMPReachNLRI(t *testing.T) {
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
			if len(m.NextHops) != tt.wantNHLen {
				t.Errorf("NextHops len = %d, want %d", len(m.NextHops), tt.wantNHLen)
			}
			if len(m.NLRI) != tt.wantNLRI {
				t.Errorf("NLRI len = %d, want %d", len(m.NLRI), tt.wantNLRI)
			}
		})
	}
}

func TestMPUnreachNLRI_WriteTo(t *testing.T) {
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
func TestParseMPReachNLRI_ExtendedNextHop(t *testing.T) {
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
	if len(m.NextHops) != 1 {
		t.Fatalf("NextHops len = %d, want 1", len(m.NextHops))
	}
	if !m.NextHops[0].Is6() {
		t.Errorf("NextHops[0] is not IPv6: %v", m.NextHops[0])
	}
	expected := netip.MustParseAddr("2001:db8::1")
	if m.NextHops[0] != expected {
		t.Errorf("NextHops[0] = %v, want %v", m.NextHops[0], expected)
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
func TestParseMPReachNLRI_ExtendedNextHop_VPN(t *testing.T) {
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
	if len(m.NextHops) != 1 {
		t.Fatalf("NextHops len = %d, want 1", len(m.NextHops))
	}
	if !m.NextHops[0].Is6() {
		t.Errorf("NextHops[0] is not IPv6: %v", m.NextHops[0])
	}
}

// TestParseMPReachNLRI_ExtendedNextHop_DualStack tests IPv4 NLRI with
// global+link-local IPv6 next-hop per RFC 2545.
//
// VALIDATES: 32-byte next-hop (global+link-local) parses as two IPv6 addresses.
//
// PREVENTS: Incorrect parsing of dual-stack next-hop announcements.
func TestParseMPReachNLRI_ExtendedNextHop_DualStack(t *testing.T) {
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

	if len(m.NextHops) != 2 {
		t.Fatalf("NextHops len = %d, want 2", len(m.NextHops))
	}
	if !m.NextHops[0].Is6() {
		t.Errorf("NextHops[0] is not IPv6: %v", m.NextHops[0])
	}
	if !m.NextHops[1].Is6() {
		t.Errorf("NextHops[1] is not IPv6: %v", m.NextHops[1])
	}
}

// TestParseMPReachNLRI_VPNIPv4NextHop tests VPN-IPv4 next-hop with RD prefix.
//
// VALIDATES: RFC 4364 Section 4.3.4 - VPN next-hop includes 8-byte RD prefix.
// For VPN-IPv4, next-hop is 12 bytes: RD(8) + IPv4(4).
//
// PREVENTS: Incorrect parsing of VPN routes, treating RD as part of IP address.
func TestParseMPReachNLRI_VPNIPv4NextHop(t *testing.T) {
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
	if len(m.NextHops) != 1 {
		t.Fatalf("NextHops len = %d, want 1", len(m.NextHops))
	}

	// The next-hop should be the IPv4 address, not including the RD
	expected := netip.MustParseAddr("10.0.0.1")
	if m.NextHops[0] != expected {
		t.Errorf("NextHops[0] = %v, want %v", m.NextHops[0], expected)
	}
	if !m.NextHops[0].Is4() {
		t.Errorf("NextHops[0] should be IPv4, got: %v", m.NextHops[0])
	}
}

// TestParseMPReachNLRI_VPNWithIPv6NextHop tests VPN next-hop parsing with 24-byte format.
//
// VALIDATES: RFC 4659 (VPN-IPv6) and RFC 8950 (VPN-IPv4 with IPv6 NH) 24-byte formats.
// Both use RD(8) + IPv6(16) = 24 bytes.
//
// PREVENTS: Incorrect parsing of VPN routes with IPv6 next-hop.
func TestParseMPReachNLRI_VPNWithIPv6NextHop(t *testing.T) {
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
			if len(m.NextHops) != 1 {
				t.Fatalf("NextHops len = %d, want 1", len(m.NextHops))
			}
			if m.NextHops[0] != expected {
				t.Errorf("NextHops[0] = %v, want %v", m.NextHops[0], expected)
			}
			if !m.NextHops[0].Is6() {
				t.Errorf("NextHops[0] should be IPv6, got: %v", m.NextHops[0])
			}
		})
	}
}

func TestMPReachNLRI_RoundTrip(t *testing.T) {
	original := &MPReachNLRI{
		AFI:      AFIIPv6,
		SAFI:     SAFIUnicast,
		NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
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
	if len(parsed.NextHops) != len(original.NextHops) {
		t.Errorf("NextHops len = %d, want %d", len(parsed.NextHops), len(original.NextHops))
	}
	if len(parsed.NLRI) != len(original.NLRI) {
		t.Errorf("NLRI len = %d, want %d", len(parsed.NLRI), len(original.NLRI))
	}
}

func TestMPUnreachNLRI_RoundTrip(t *testing.T) {
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
