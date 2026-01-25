// Package nlri tests for wire format verification.
package nlri

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"testing"
)

// TestWireFormat_AddPath verifies actual wire bytes match expected format.
//
// RFC 7911 Section 3: ADD-PATH prepends 4-byte path identifier.
//
// VALIDATES: Wire format is [pathID][payload] when AddPath=true.
// PREVENTS: Path ID in wrong position or missing entirely.
func TestWireFormat_AddPath(t *testing.T) {
	tests := []struct {
		name    string
		nlri    NLRI
		addPath bool
		wantHex string // Expected wire format in hex
	}{
		// INET - IPv4
		{
			name:    "INET_10.0.0.0/24_noAddPath",
			nlri:    NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
			addPath: false,
			wantHex: "180a0000", // [prefixLen=24][10.0.0]
		},
		{
			name:    "INET_10.0.0.0/24_withAddPath_pathID0",
			nlri:    NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
			addPath: true,
			wantHex: "00000000180a0000", // [pathID=0][prefixLen=24][10.0.0]
		},
		{
			name:    "INET_10.0.0.0/24_withAddPath_pathID42",
			nlri:    NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 42),
			addPath: true,
			wantHex: "0000002a180a0000", // [pathID=42][prefixLen=24][10.0.0]
		},
		// INET - IPv6
		{
			name:    "INET_2001:db8::/32_noAddPath",
			nlri:    NewINET(IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0),
			addPath: false,
			wantHex: "2020010db8", // [prefixLen=32][2001:0db8]
		},
		{
			name:    "INET_2001:db8::/32_withAddPath_pathID100",
			nlri:    NewINET(IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 100),
			addPath: true,
			wantHex: "000000642020010db8", // [pathID=100][prefixLen=32][2001:0db8]
		},
		// INET edge cases
		{
			name:    "INET_0.0.0.0/0_noAddPath",
			nlri:    NewINET(IPv4Unicast, netip.MustParsePrefix("0.0.0.0/0"), 0),
			addPath: false,
			wantHex: "00", // [prefixLen=0]
		},
		{
			name:    "INET_192.168.1.128/32_withAddPath",
			nlri:    NewINET(IPv4Unicast, netip.MustParsePrefix("192.168.1.128/32"), 1),
			addPath: true,
			wantHex: "0000000120c0a80180", // [pathID=1][prefixLen=32][192.168.1.128]
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 100)
			n := WriteNLRI(tt.nlri, buf, 0, tt.addPath)
			got := hex.EncodeToString(buf[:n])
			if got != tt.wantHex {
				t.Errorf("wire format:\n  got:  %s\n  want: %s", got, tt.wantHex)
			}
		})
	}
}

// TestWireFormat_IPVPN verifies IPVPN wire format with ADD-PATH.
//
// RFC 4364/4659: VPN NLRI = [length][labels][RD][prefix]
// RFC 7911: ADD-PATH prepends 4-byte path ID.
// Labels: 3 bytes each, label 16000 = 0x3E80 << 4 | BOS = 0x3E8001
//
// VALIDATES: VPN wire format is [pathID][length][labels][RD][prefix].
// PREVENTS: Label/RD/prefix in wrong order.
func TestWireFormat_IPVPN(t *testing.T) {
	rd, _ := ParseRDString("0:1")

	tests := []struct {
		name    string
		nlri    NLRI
		addPath bool
		wantHex string
	}{
		{
			name:    "IPVPN_10.0.0.0/24_noAddPath",
			nlri:    NewIPVPN(IPv4VPN, rd, []uint32{16000}, netip.MustParsePrefix("10.0.0.0/24"), 0),
			addPath: false,
			wantHex: "70" + "03e801" + "0000000000000001" + "0a0000", // [len=112][label 3B][RD 8B][prefix 3B]
		},
		{
			name:    "IPVPN_10.0.0.0/24_withAddPath",
			nlri:    NewIPVPN(IPv4VPN, rd, []uint32{16000}, netip.MustParsePrefix("10.0.0.0/24"), 42),
			addPath: true,
			wantHex: "0000002a" + "70" + "03e801" + "0000000000000001" + "0a0000", // [pathID 4B][len][label][RD][prefix]
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 100)
			n := WriteNLRI(tt.nlri, buf, 0, tt.addPath)
			got := hex.EncodeToString(buf[:n])
			if got != tt.wantHex {
				t.Errorf("wire format:\n  got:  %s\n  want: %s", got, tt.wantHex)
			}
		})
	}
}

// TestWireFormat_LabeledUnicast verifies labeled unicast wire format.
//
// RFC 8277 Section 2.2: Labeled NLRI = [length][labels][prefix]
// RFC 7911: ADD-PATH prepends 4-byte path ID.
// Labels: 3 bytes each, label 16000 = 0x3E80 << 4 | BOS = 0x3E8001
//
// VALIDATES: Labeled wire format is [pathID][length][labels][prefix].
// PREVENTS: Label bytes in wrong position.
func TestWireFormat_LabeledUnicast(t *testing.T) {
	tests := []struct {
		name    string
		nlri    NLRI
		addPath bool
		wantHex string
	}{
		{
			name:    "LabeledUnicast_10.0.0.0/24_noAddPath",
			nlri:    NewLabeledUnicast(IPv4LabeledUnicast, netip.MustParsePrefix("10.0.0.0/24"), []uint32{16000}, 0),
			addPath: false,
			wantHex: "30" + "03e801" + "0a0000", // [len=48][label 3B][prefix 3B]
		},
		{
			name:    "LabeledUnicast_10.0.0.0/24_withAddPath",
			nlri:    NewLabeledUnicast(IPv4LabeledUnicast, netip.MustParsePrefix("10.0.0.0/24"), []uint32{16000}, 77),
			addPath: true,
			wantHex: "0000004d" + "30" + "03e801" + "0a0000", // [pathID=77 4B][len][label][prefix]
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 100)
			n := WriteNLRI(tt.nlri, buf, 0, tt.addPath)
			got := hex.EncodeToString(buf[:n])
			if got != tt.wantHex {
				t.Errorf("wire format:\n  got:  %s\n  want: %s", got, tt.wantHex)
			}
		})
	}
}

// TestWireFormat_EVPN verifies EVPN wire format with ADD-PATH.
//
// RFC 7432: EVPN NLRI = [type][length][payload]
// RFC 7911: ADD-PATH prepends 4-byte path ID.
//
// EVPN Type 2 (MAC/IP) payload:
//
//	RD(8) + ESI(10) + EthTag(4) + MACLen(1) + MAC(6) + IPLen(1) + IP(4) + Label(3) = 37
//	+ type(1) + len(1) = 39 bytes
//
// EVPN Type 5 (IP Prefix) payload:
//
//	RD(8) + ESI(10) + EthTag(4) + PrefixLen(1) + Prefix(4) + GW(4) + Label(3) = 34
//	+ type(1) + len(1) = 36 bytes
//
// VALIDATES: EVPN wire format is [pathID][type][length][payload].
// PREVENTS: ADD-PATH encoding bugs that led to spec 070.
func TestWireFormat_EVPN(t *testing.T) {
	rd := RouteDistinguisher{Type: 0, Value: [6]byte{0, 0, 0, 0, 0, 1}}
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	tests := []struct {
		name    string
		nlri    NLRI
		addPath bool
		wantLen int // Verify length matches
	}{
		{
			name:    "EVPNType2_noAddPath",
			nlri:    NewEVPNType2(rd, ESI{}, 0, mac, netip.MustParseAddr("10.0.0.1"), []uint32{100}),
			addPath: false,
			wantLen: 39, // type(1) + len(1) + payload(37)
		},
		{
			name:    "EVPNType2_withAddPath",
			nlri:    NewEVPNType2(rd, ESI{}, 0, mac, netip.MustParseAddr("10.0.0.1"), []uint32{100}),
			addPath: true,
			wantLen: 43, // pathID(4) + type(1) + len(1) + payload(37)
		},
		{
			name:    "EVPNType5_noAddPath",
			nlri:    NewEVPNType5(rd, ESI{}, 0, netip.MustParsePrefix("10.0.0.0/24"), netip.Addr{}, []uint32{100}),
			addPath: false,
			wantLen: 36, // type(1) + len(1) + payload(34)
		},
		{
			name:    "EVPNType5_withAddPath",
			nlri:    NewEVPNType5(rd, ESI{}, 0, netip.MustParsePrefix("10.0.0.0/24"), netip.Addr{}, []uint32{100}),
			addPath: true,
			wantLen: 40, // pathID(4) + type(1) + len(1) + payload(34)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 100)
			n := WriteNLRI(tt.nlri, buf, 0, tt.addPath)

			if n != tt.wantLen {
				t.Errorf("length = %d, want %d", n, tt.wantLen)
			}

			// Verify path ID position when ADD-PATH enabled
			if tt.addPath {
				// First 4 bytes should be path ID (0 for these test cases)
				pathID := buf[0:4]
				if !bytes.Equal(pathID, []byte{0, 0, 0, 0}) {
					t.Errorf("path ID = %x, want 00000000", pathID)
				}
				// Byte 4 should be EVPN type
				if buf[4] != 2 && buf[4] != 5 {
					t.Errorf("EVPN type at wrong position, got %d", buf[4])
				}
			} else if buf[0] != 2 && buf[0] != 5 {
				// First byte should be EVPN type
				t.Errorf("EVPN type = %d, want 2 or 5", buf[0])
			}
		})
	}
}

// TestRoundTrip_INET verifies encode → decode → encode produces identical bytes.
//
// VALIDATES: Parsing preserves all NLRI data including path ID.
// PREVENTS: Data loss during parse/pack cycle.
func TestRoundTrip_INET(t *testing.T) {
	tests := []struct {
		name    string
		nlri    *INET
		addPath bool
	}{
		{"IPv4_noPath", NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0), false},
		{"IPv4_withPath", NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 42), true},
		{"IPv4/32_withPath", NewINET(IPv4Unicast, netip.MustParsePrefix("192.168.1.1/32"), 100), true},
		{"IPv4/0_withPath", NewINET(IPv4Unicast, netip.MustParsePrefix("0.0.0.0/0"), 1), true},
		{"IPv6_noPath", NewINET(IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0), false},
		{"IPv6_withPath", NewINET(IPv6Unicast, netip.MustParsePrefix("2001:db8::1/128"), 77), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// addPath is already a bool
			family := tt.nlri.Family()

			// Encode
			buf := make([]byte, 100)
			n := WriteNLRI(tt.nlri, buf, 0, tt.addPath)
			wire := buf[:n]

			// Decode - ParseINET returns (NLRI, remaining, error)
			parsed, remaining, err := ParseINET(family.AFI, family.SAFI, wire, tt.addPath)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			consumed := n - len(remaining)
			if consumed != n {
				t.Fatalf("consumed %d bytes, wrote %d", consumed, n)
			}

			// Re-encode
			buf2 := make([]byte, 100)
			n2 := WriteNLRI(parsed, buf2, 0, tt.addPath)
			wire2 := buf2[:n2]

			// Compare
			if !bytes.Equal(wire, wire2) {
				t.Errorf("round-trip mismatch:\n  orig: %x\n  trip: %x", wire, wire2)
			}

			// Verify path ID preserved
			if tt.addPath && parsed.PathID() != tt.nlri.PathID() {
				t.Errorf("path ID = %d, want %d", parsed.PathID(), tt.nlri.PathID())
			}
		})
	}
}

// TestRoundTrip_IPVPN verifies IPVPN encode → decode → encode.
//
// VALIDATES: VPN routes preserve RD, labels, and prefix.
// PREVENTS: Label or RD corruption during round-trip.
func TestRoundTrip_IPVPN(t *testing.T) {
	rd, _ := ParseRDString("65000:100")

	tests := []struct {
		name    string
		nlri    *IPVPN
		addPath bool
	}{
		{"VPNv4_noPath", NewIPVPN(IPv4VPN, rd, []uint32{16000}, netip.MustParsePrefix("10.0.0.0/24"), 0), false},
		{"VPNv4_withPath", NewIPVPN(IPv4VPN, rd, []uint32{16000}, netip.MustParsePrefix("10.0.0.0/24"), 42), true},
		{"VPNv4_twoLabels", NewIPVPN(IPv4VPN, rd, []uint32{16000, 17000}, netip.MustParsePrefix("10.0.0.0/24"), 1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// addPath is already a bool
			family := tt.nlri.Family()

			// Encode
			buf := make([]byte, 100)
			n := WriteNLRI(tt.nlri, buf, 0, tt.addPath)
			wire := buf[:n]

			// Decode - ParseIPVPN returns (NLRI, remaining, error)
			parsed, remaining, err := ParseIPVPN(family.AFI, family.SAFI, wire, tt.addPath)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			consumed := n - len(remaining)
			if consumed != n {
				t.Fatalf("consumed %d bytes, wrote %d", consumed, n)
			}

			// Re-encode
			buf2 := make([]byte, 100)
			n2 := WriteNLRI(parsed, buf2, 0, tt.addPath)
			wire2 := buf2[:n2]

			// Compare
			if !bytes.Equal(wire, wire2) {
				t.Errorf("round-trip mismatch:\n  orig: %x\n  trip: %x", wire, wire2)
			}
		})
	}
}

// TestRoundTrip_EVPN verifies EVPN encode → decode → encode.
//
// VALIDATES: EVPN routes preserve RD, MAC, IP, labels.
// PREVENTS: EVPN encoding corruption that led to spec 070.
func TestRoundTrip_EVPN(t *testing.T) {
	rd := RouteDistinguisher{Type: 0, Value: [6]byte{0, 0, 0, 0, 0, 1}}
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	tests := []struct {
		name    string
		nlri    NLRI
		addPath bool
	}{
		{"Type2_noPath", NewEVPNType2(rd, ESI{}, 0, mac, netip.MustParseAddr("10.0.0.1"), []uint32{100}), false},
		{"Type2_withPath", NewEVPNType2(rd, ESI{}, 0, mac, netip.MustParseAddr("10.0.0.1"), []uint32{100}), true},
		{"Type5_noPath", NewEVPNType5(rd, ESI{}, 0, netip.MustParsePrefix("10.0.0.0/24"), netip.Addr{}, []uint32{100}), false},
		{"Type5_withPath", NewEVPNType5(rd, ESI{}, 0, netip.MustParsePrefix("10.0.0.0/24"), netip.Addr{}, []uint32{100}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// addPath is already a bool

			// Encode
			buf := make([]byte, 100)
			n := WriteNLRI(tt.nlri, buf, 0, tt.addPath)
			wire := buf[:n]

			// Decode - ParseEVPN returns (NLRI, remaining, error)
			parsed, remaining, err := ParseEVPN(wire, tt.addPath)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			consumed := n - len(remaining)
			if consumed != n {
				t.Fatalf("consumed %d bytes, wrote %d", consumed, n)
			}

			// Re-encode
			buf2 := make([]byte, 100)
			n2 := WriteNLRI(parsed, buf2, 0, tt.addPath)
			wire2 := buf2[:n2]

			// Compare
			if !bytes.Equal(wire, wire2) {
				t.Errorf("round-trip mismatch:\n  orig: %x\n  trip: %x", wire, wire2)
			}
		})
	}
}
