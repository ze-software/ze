package attribute

import (
	"net/netip"
	"testing"
)

func TestMPReachNLRI_Pack(t *testing.T) {
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
			got := tt.attr.Pack()
			if len(got) != len(tt.expected) {
				t.Errorf("Pack() len = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("Pack()[%d] = 0x%02x, want 0x%02x", i, got[i], tt.expected[i])
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

func TestMPUnreachNLRI_Pack(t *testing.T) {
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
			got := tt.attr.Pack()
			if len(got) != len(tt.expected) {
				t.Errorf("Pack() len = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("Pack()[%d] = 0x%02x, want 0x%02x", i, got[i], tt.expected[i])
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

func TestMPReachNLRI_RoundTrip(t *testing.T) {
	original := &MPReachNLRI{
		AFI:      AFIIPv6,
		SAFI:     SAFIUnicast,
		NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
		NLRI:     []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x01},
	}

	packed := original.Pack()
	parsed, err := ParseMPReachNLRI(packed)
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

	packed := original.Pack()
	parsed, err := ParseMPUnreachNLRI(packed)
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
