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
	t.Parallel()
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
			t.Parallel()
			buf := make([]byte, 100)
			n := WriteNLRI(tt.nlri, buf, 0, tt.addPath)
			got := hex.EncodeToString(buf[:n])
			if got != tt.wantHex {
				t.Errorf("wire format:\n  got:  %s\n  want: %s", got, tt.wantHex)
			}
		})
	}
}

// Note: TestWireFormat_IPVPN moved to internal/plugin/vpn/vpn_test.go
// Note: TestWireFormat_LabeledUnicast moved to internal/plugins/bgp-nlri-labeled/
// Note: TestWireFormat_EVPN moved to internal/plugin/evpn/types_test.go

// TestRoundTrip_INET verifies encode → decode → encode produces identical bytes.
//
// VALIDATES: Parsing preserves all NLRI data including path ID.
// PREVENTS: Data loss during parse/pack cycle.
func TestRoundTrip_INET(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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

// Note: TestRoundTrip_IPVPN moved to internal/plugin/vpn/vpn_test.go

// Note: TestRoundTrip_EVPN moved to internal/plugin/evpn/types_test.go
