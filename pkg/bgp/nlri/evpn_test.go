package nlri

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildEVPNData builds EVPN test data with pre-allocated capacity.
func buildEVPNData(routeType EVPNRouteType, length byte, components ...[]byte) []byte {
	total := 2 // type + length bytes
	for _, c := range components {
		total += len(c)
	}
	data := make([]byte, 0, total)
	data = append(data, byte(routeType), length)
	for _, c := range components {
		data = append(data, c...)
	}
	return data
}

// TestEVPNType2MACOnly verifies Type 2 MAC-only route parsing.
//
// VALIDATES: Basic MAC advertisement without IP.
//
// PREVENTS: MAC-only route parsing failures.
func TestEVPNType2MACOnly(t *testing.T) {
	// Type 2: MAC/IP Advertisement
	// RD (8) + ESI (10) + EthTag (4) + MACLen (1) + MAC (6) + IPLen (1) + Labels (3)
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // 65000:100
	esi := make([]byte, 10)                                      // All zeros
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}                     // Tag 0
	macLen := byte(48)                                           // 48 bits
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(0)                  // No IP
	label := []byte{0x00, 0x01, 0x01} // Label 16

	data := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, label)

	nlri, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := nlri.(*EVPNType2)
	require.True(t, ok)

	assert.Equal(t, EVPNRouteType2, evpn.RouteType())
	assert.Equal(t, "0:65000:100", evpn.RD().String())
	assert.Equal(t, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, evpn.MAC())
	assert.False(t, evpn.IP().IsValid())
	assert.Equal(t, L2VPNEVPN, evpn.Family())
}

// TestEVPNType2MACIPv4 verifies Type 2 MAC+IPv4 route parsing.
//
// VALIDATES: MAC+IPv4 advertisement.
//
// PREVENTS: IPv4 parsing errors in MAC/IP routes.
func TestEVPNType2MACIPv4(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(32) // IPv4
	ip := []byte{10, 0, 0, 1}
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+4+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, ip, label)

	nlri, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType2)
	require.True(t, ok, "expected EVPNType2")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), evpn.IP())
}

// TestEVPNType2MACIPv6 verifies Type 2 MAC+IPv6 route parsing.
//
// VALIDATES: MAC+IPv6 advertisement.
//
// PREVENTS: IPv6 parsing errors in MAC/IP routes.
func TestEVPNType2MACIPv6(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(128)                                                       // IPv6
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1} // 2001:db8::1
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+16+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, ip, label)

	nlri, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType2)
	require.True(t, ok, "expected EVPNType2")
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), evpn.IP())
}

// TestEVPNType3 verifies Type 3 Inclusive Multicast route parsing.
//
// VALIDATES: IMET route for BUM traffic.
//
// PREVENTS: Multicast route parsing failures.
func TestEVPNType3(t *testing.T) {
	// Type 3: Inclusive Multicast Ethernet Tag
	// RD (8) + EthTag (4) + IPLen (1) + IP (4 or 16)
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}

	data := buildEVPNData(EVPNRouteType3, byte(8+4+1+4),
		rd, ethTag, []byte{ipLen}, ip)

	nlri, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType3)
	require.True(t, ok)

	assert.Equal(t, EVPNRouteType3, evpn.RouteType())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), evpn.OriginatorIP())
}

// TestEVPNType5IPv4 verifies Type 5 IP Prefix route parsing for IPv4.
//
// VALIDATES: RFC 9136 Section 3.1 - IPv4 IP Prefix route with fixed 4-byte
// prefix field and 4-byte gateway field. Total NLRI length = 34 bytes.
//
// PREVENTS: Incorrect variable-length prefix parsing that violates RFC 9136.
func TestEVPNType5IPv4(t *testing.T) {
	// Type 5: IP Prefix per RFC 9136 Section 3.1
	// RD (8) + ESI (10) + EthTag (4) + IPLen (1) + IP (4 fixed) + GW (4 fixed) + Label (3) = 34
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // 65000:100
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(24)                 // /24 prefix
	ip := []byte{10, 1, 2, 0}         // 10.1.2.0/24 - FIXED 4 bytes per RFC 9136
	gw := []byte{0, 0, 0, 0}          // No gateway - FIXED 4 bytes
	label := []byte{0x00, 0x01, 0x01} // Label 16

	data := buildEVPNData(EVPNRouteType5, byte(34), // Length = 34 per RFC 9136 for IPv4
		rd, esi, ethTag, []byte{ipLen}, ip, gw, label)

	nlri, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := nlri.(*EVPNType5)
	require.True(t, ok, "expected EVPNType5, got %T", nlri)

	assert.Equal(t, EVPNRouteType5, evpn.RouteType())
	assert.Equal(t, "0:65000:100", evpn.RD().String())
	assert.Equal(t, netip.MustParsePrefix("10.1.2.0/24"), evpn.Prefix())
	assert.Equal(t, netip.MustParseAddr("0.0.0.0"), evpn.Gateway())
	assert.Equal(t, L2VPNEVPN, evpn.Family())
}

// TestEVPNType5IPv6 verifies Type 5 IP Prefix route parsing for IPv6.
//
// VALIDATES: RFC 9136 Section 3.1 - IPv6 IP Prefix route with fixed 16-byte
// prefix field and 16-byte gateway field. Total NLRI length = 58 bytes.
//
// PREVENTS: Incorrect variable-length prefix parsing for IPv6.
func TestEVPNType5IPv6(t *testing.T) {
	// Type 5: IP Prefix per RFC 9136 Section 3.1
	// RD (8) + ESI (10) + EthTag (4) + IPLen (1) + IP (16 fixed) + GW (16 fixed) + Label (3) = 58
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(64) // /64 prefix
	// 2001:db8::/64 - FIXED 16 bytes per RFC 9136
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	gw := make([]byte, 16) // No gateway - FIXED 16 bytes
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType5, byte(58), // Length = 58 per RFC 9136 for IPv6
		rd, esi, ethTag, []byte{ipLen}, ip, gw, label)

	nlri, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType5)
	require.True(t, ok, "expected EVPNType5")

	assert.Equal(t, netip.MustParsePrefix("2001:db8::/64"), evpn.Prefix())
}

// TestEVPNType5InvalidLength verifies rejection of invalid NLRI lengths.
//
// VALIDATES: RFC 9136 requirement that length MUST be 34 (IPv4) or 58 (IPv6).
//
// PREVENTS: Accepting malformed Type 5 routes with incorrect lengths.
func TestEVPNType5InvalidLength(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(24)
	// Variable-length prefix (3 bytes) - WRONG per RFC 9136
	ip := []byte{10, 1, 2}
	gw := []byte{0, 0, 0, 0}
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType5, byte(8+10+4+1+3+4+3), // Length = 33, not 34
		rd, esi, ethTag, []byte{ipLen}, ip, gw, label)

	_, _, err := ParseEVPN(data, false)
	require.Error(t, err, "should reject non-standard Type 5 length")
}

// TestEVPNRouteTypeString verifies route type string representation.
func TestEVPNRouteTypeString(t *testing.T) {
	assert.Equal(t, "ethernet-auto-discovery", EVPNRouteType1.String())
	assert.Equal(t, "mac-ip-advertisement", EVPNRouteType2.String())
	assert.Equal(t, "inclusive-multicast", EVPNRouteType3.String())
	assert.Equal(t, "ethernet-segment", EVPNRouteType4.String())
	assert.Equal(t, "ip-prefix", EVPNRouteType5.String())
}

// TestEVPNType1 verifies Type 1 Ethernet Auto-Discovery route parsing.
//
// VALIDATES: RFC 7432 Section 7.1 - Ethernet A-D route format with
// RD (8) + ESI (10) + EthernetTag (4) + Label (3) = 25 bytes.
//
// PREVENTS: Parsing failures for Ethernet Auto-Discovery routes used
// in multihoming scenarios for fast convergence and aliasing.
func TestEVPNType1(t *testing.T) {
	// Type 1: Ethernet Auto-Discovery
	// RFC 7432 Section 7.1: RD (8) + ESI (10) + EthTag (4) + Label (3)
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // 65000:100
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ethTag := []byte{0x00, 0x00, 0x00, 0x0A} // Tag 10
	label := []byte{0x00, 0x01, 0x01}        // Label 16

	data := buildEVPNData(EVPNRouteType1, byte(8+10+4+3), // Length = 25
		rd, esi, ethTag, label)

	nlri, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := nlri.(*EVPNType1)
	require.True(t, ok, "expected EVPNType1, got %T", nlri)

	assert.Equal(t, EVPNRouteType1, evpn.RouteType())
	assert.Equal(t, "0:65000:100", evpn.RD().String())
	assert.Equal(t, ESI{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}, evpn.ESI())
	assert.Equal(t, uint32(10), evpn.EthernetTag())
	assert.Equal(t, []uint32{16}, evpn.Labels())
	assert.Equal(t, L2VPNEVPN, evpn.Family())
}

// TestEVPNType1WithAddPath verifies Type 1 with Add-Path ID.
//
// VALIDATES: Ethernet Auto-Discovery with path ID (RFC 7911 support).
//
// PREVENTS: Add-path parsing errors for Type 1 routes.
func TestEVPNType1WithAddPath(t *testing.T) {
	pathID := []byte{0x00, 0x00, 0x00, 0x05} // Path ID 5
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	label := []byte{0x00, 0x01, 0x01}

	data := pathID
	data = append(data, byte(EVPNRouteType1))
	data = append(data, byte(8+10+4+3))
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ethTag...)
	data = append(data, label...)

	nlri, _, err := ParseEVPN(data, true)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType1)
	require.True(t, ok, "expected EVPNType1")
	assert.True(t, evpn.PathID() != 0)
	assert.Equal(t, uint32(5), evpn.PathID())
}

// TestEVPNType4IPv4 verifies Type 4 Ethernet Segment route with IPv4.
//
// VALIDATES: RFC 7432 Section 7.4 - Ethernet Segment route format with
// RD (8) + ESI (10) + IPLen (1) + IP (4) = 23 bytes for IPv4.
//
// PREVENTS: Parsing failures for Ethernet Segment routes used in
// Designated Forwarder election.
func TestEVPNType4IPv4(t *testing.T) {
	// Type 4: Ethernet Segment
	// RFC 7432 Section 7.4: RD (8) + ESI (10) + IPLen (1) + IP (4/16)
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // 65000:100
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ipLen := byte(32) // IPv4 = 32 bits
	ip := []byte{10, 0, 0, 1}

	data := buildEVPNData(EVPNRouteType4, byte(8+10+1+4), // Length = 23
		rd, esi, []byte{ipLen}, ip)

	nlri, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := nlri.(*EVPNType4)
	require.True(t, ok, "expected EVPNType4, got %T", nlri)

	assert.Equal(t, EVPNRouteType4, evpn.RouteType())
	assert.Equal(t, "0:65000:100", evpn.RD().String())
	assert.Equal(t, ESI{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}, evpn.ESI())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), evpn.OriginatorIP())
	assert.Equal(t, L2VPNEVPN, evpn.Family())
}

// TestEVPNType4IPv6 verifies Type 4 Ethernet Segment route with IPv6.
//
// VALIDATES: RFC 7432 Section 7.4 - Ethernet Segment route with IPv6
// originator address (35 bytes total).
//
// PREVENTS: IPv6 parsing errors in Ethernet Segment routes.
func TestEVPNType4IPv6(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ipLen := byte(128)                                                       // IPv6 = 128 bits
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1} // 2001:db8::1

	data := buildEVPNData(EVPNRouteType4, byte(8+10+1+16), // Length = 35
		rd, esi, []byte{ipLen}, ip)

	nlri, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType4)
	require.True(t, ok, "expected EVPNType4")
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), evpn.OriginatorIP())
}

// TestEVPNType4InvalidIPLen verifies error handling for invalid IP length.
//
// VALIDATES: Rejection of invalid IP address lengths per RFC 7432 Section 7.4.
//
// PREVENTS: Accepting malformed Ethernet Segment routes.
func TestEVPNType4InvalidIPLen(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ipLen := byte(64) // Invalid: must be 32 or 128
	ip := make([]byte, 8)

	data := buildEVPNData(EVPNRouteType4, byte(8+10+1+8),
		rd, esi, []byte{ipLen}, ip)

	_, _, err := ParseEVPN(data, false)
	require.Error(t, err, "should reject invalid IP length")
}

// TestEVPNErrors verifies error handling.
func TestEVPNErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"no length", []byte{byte(EVPNRouteType2)}},
		{"truncated", []byte{byte(EVPNRouteType2), 50, 0x00}}, // says 50 bytes but only 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseEVPN(tt.data, false)
			require.Error(t, err)
		})
	}
}

// TestEVPNType1RoundTrip verifies that parsing and encoding are symmetric.
//
// VALIDATES: Bytes() produces wire format that ParseEVPN() can read back identically.
//
// PREVENTS: Asymmetric encoding that would corrupt routes on re-advertisement,
// causing EVPN Type 1 routes to be malformed when sent to peers.
func TestEVPNType1RoundTrip(t *testing.T) {
	// Type 1: Ethernet Auto-Discovery
	// Wire: [type:1][len:1][RD:8][ESI:10][EthTag:4][Labels:3]
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // 65000:100
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ethTag := []byte{0x00, 0x00, 0x00, 0x0A} // Tag 10
	label := []byte{0x00, 0x01, 0x01}        // Label 16 with BOS

	original := buildEVPNData(EVPNRouteType1, byte(8+10+4+3),
		rd, esi, ethTag, label)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType1)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType2RoundTripMACOnly verifies Type 2 MAC-only round-trip.
//
// VALIDATES: Bytes() correctly encodes MAC-only routes (no IP).
//
// PREVENTS: MAC-only routes being corrupted, breaking L2 EVPN.
func TestEVPNType2RoundTripMACOnly(t *testing.T) {
	// Type 2: MAC/IP Advertisement (MAC only)
	// Wire: [type:1][len:1][RD:8][ESI:10][EthTag:4][MACLen:1][MAC:6][IPLen:1][Labels:3]
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(0)
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, label)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType2)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType2RoundTripWithIPv4 verifies Type 2 with IPv4 round-trip.
//
// VALIDATES: Bytes() correctly encodes MAC+IPv4 routes.
//
// PREVENTS: IPv4 address corruption in MAC/IP advertisement routes.
func TestEVPNType2RoundTripWithIPv4(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+4+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, ip, label)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType2)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType3RoundTripIPv4 verifies Type 3 with IPv4 round-trip.
//
// VALIDATES: Bytes() correctly encodes Inclusive Multicast routes.
//
// PREVENTS: BUM traffic flooding issues from malformed Type 3 routes.
func TestEVPNType3RoundTripIPv4(t *testing.T) {
	// Type 3: Inclusive Multicast Ethernet Tag
	// Wire: [type:1][len:1][RD:8][EthTag:4][IPLen:1][IP:4]
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}

	original := buildEVPNData(EVPNRouteType3, byte(8+4+1+4),
		rd, ethTag, []byte{ipLen}, ip)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType3)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType4RoundTripIPv4 verifies Type 4 with IPv4 round-trip.
//
// VALIDATES: Bytes() correctly encodes Ethernet Segment routes.
//
// PREVENTS: DF election failures from malformed Type 4 routes.
func TestEVPNType4RoundTripIPv4(t *testing.T) {
	// Type 4: Ethernet Segment
	// Wire: [type:1][len:1][RD:8][ESI:10][IPLen:1][IP:4]
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}

	original := buildEVPNData(EVPNRouteType4, byte(8+10+1+4),
		rd, esi, []byte{ipLen}, ip)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType4)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType5RoundTripIPv4 verifies Type 5 IPv4 round-trip.
//
// VALIDATES: Bytes() correctly encodes IP Prefix routes per RFC 9136.
//
// PREVENTS: IP prefix routes being malformed, breaking L3VPN over EVPN.
func TestEVPNType5RoundTripIPv4(t *testing.T) {
	// Type 5: IP Prefix per RFC 9136
	// Wire: [type:1][len:1][RD:8][ESI:10][EthTag:4][PrefixLen:1][Prefix:4][GW:4][Labels:3]
	// Total length = 34 for IPv4
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	prefixLen := byte(24)
	prefix := []byte{10, 1, 2, 0}
	gw := []byte{0, 0, 0, 0}
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType5, byte(34),
		rd, esi, ethTag, []byte{prefixLen}, prefix, gw, label)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType5)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType5RoundTripIPv6 verifies Type 5 IPv6 round-trip.
//
// VALIDATES: Bytes() correctly encodes IPv6 prefix routes per RFC 9136.
//
// PREVENTS: IPv6 prefix routes being malformed.
func TestEVPNType5RoundTripIPv6(t *testing.T) {
	// Type 5: IP Prefix per RFC 9136
	// Total length = 58 for IPv6
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	prefixLen := byte(64)
	prefix := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	gw := make([]byte, 16)
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType5, byte(58),
		rd, esi, ethTag, []byte{prefixLen}, prefix, gw, label)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType5)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType2RoundTripWithIPv6 verifies Type 2 with IPv6 round-trip.
//
// VALIDATES: Bytes() correctly encodes MAC+IPv6 routes.
//
// PREVENTS: IPv6 address corruption in MAC/IP advertisement routes.
func TestEVPNType2RoundTripWithIPv6(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(128)
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+16+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, ip, label)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType2)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType3RoundTripIPv6 verifies Type 3 with IPv6 round-trip.
//
// VALIDATES: Bytes() correctly encodes Inclusive Multicast routes with IPv6.
//
// PREVENTS: IPv6 originator address corruption in IMET routes.
func TestEVPNType3RoundTripIPv6(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(128)
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	original := buildEVPNData(EVPNRouteType3, byte(8+4+1+16),
		rd, ethTag, []byte{ipLen}, ip)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType3)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType4RoundTripIPv6 verifies Type 4 with IPv6 round-trip.
//
// VALIDATES: Bytes() correctly encodes Ethernet Segment routes with IPv6.
//
// PREVENTS: IPv6 originator address corruption in ES routes.
func TestEVPNType4RoundTripIPv6(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ipLen := byte(128)
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	original := buildEVPNData(EVPNRouteType4, byte(8+10+1+16),
		rd, esi, []byte{ipLen}, ip)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType4)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType1RoundTripMultiLabel verifies Type 1 with label stack.
//
// VALIDATES: Bytes() correctly encodes multiple MPLS labels with BOS bit.
//
// PREVENTS: Label stack corruption breaking EVPN-MPLS forwarding.
func TestEVPNType1RoundTripMultiLabel(t *testing.T) {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ethTag := []byte{0x00, 0x00, 0x00, 0x0A}
	// Label stack: 100, 200 (BOS on last)
	// Label 100 = 0x000640 (no BOS), Label 200 = 0x000C81 (with BOS)
	labels := []byte{0x00, 0x06, 0x40, 0x00, 0x0C, 0x81}

	original := buildEVPNData(EVPNRouteType1, byte(8+10+4+6),
		rd, esi, ethTag, labels)

	nlri, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType1)
	require.True(t, ok)
	assert.Equal(t, []uint32{100, 200}, evpn.Labels())

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestParseESIString verifies ESI string parsing.
//
// VALIDATES: ParseESIString handles all formats (0, hex, colon-separated).
// PREVENTS: ESI string parsing bugs.
func TestParseESIString(t *testing.T) {
	tests := []struct {
		input    string
		expected ESI
		wantErr  bool
	}{
		// Zero ESI
		{"0", ESI{}, false},
		{"", ESI{}, false},
		// Plain hex (20 chars)
		{"00112233445566778899", ESI{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}, false},
		// Colon-separated
		{"00:11:22:33:44:55:66:77:88:99", ESI{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}, false},
		// All zeros colon-separated
		{"00:00:00:00:00:00:00:00:00:00", ESI{}, false},
		// All FF
		{"ff:ff:ff:ff:ff:ff:ff:ff:ff:ff", ESI{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, false},
		{"FFFFFFFFFFFFFFFFFFFF", ESI{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, false},
		// Invalid
		{"invalid", ESI{}, true},
		{"00:11:22", ESI{}, true}, // Too short
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := ParseESIString(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestEVPNType2StringCommandStyle verifies Type 2 String() outputs command-style format.
//
// VALIDATES: Type 2 (MAC/IP) output uses "mac-ip rd set ... mac set ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType2StringCommandStyle(t *testing.T) {
	rd, _ := ParseRDString("65000:100")
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	// MAC-only
	e := NewEVPNType2(rd, [10]byte{}, 0, mac, netip.Addr{}, []uint32{1000})
	s := e.String()
	assert.Contains(t, s, "mac-ip", "should start with route type")
	assert.Contains(t, s, "rd set 0:65000:100", "rd should use 'set' keyword")
	assert.Contains(t, s, "mac set 00:11:22:33:44:55", "mac should use 'set' keyword")
}

// TestEVPNType2WithIPStringCommandStyle verifies Type 2 with IP uses command-style.
//
// VALIDATES: Type 2 with IP outputs "mac-ip rd set ... mac set ... ip set ..." format.
// PREVENTS: Missing IP in output when present.
func TestEVPNType2WithIPStringCommandStyle(t *testing.T) {
	rd, _ := ParseRDString("65000:100")
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ip := netip.MustParseAddr("192.168.1.1")

	e := NewEVPNType2(rd, [10]byte{}, 100, mac, ip, []uint32{1000})
	s := e.String()
	assert.Contains(t, s, "mac-ip", "should start with route type")
	assert.Contains(t, s, "rd set 0:65000:100", "rd should use 'set' keyword")
	assert.Contains(t, s, "mac set 00:11:22:33:44:55", "mac should use 'set' keyword")
	assert.Contains(t, s, "ip set 192.168.1.1", "ip should use 'set' keyword")
	assert.Contains(t, s, "etag set 100", "etag should use 'set' keyword")
	assert.Contains(t, s, "label set 1000", "label should use 'set' keyword")
}

// TestEVPNType3StringCommandStyle verifies Type 3 String() outputs command-style format.
//
// VALIDATES: Type 3 (Multicast) output uses "multicast rd set ... ip set ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType3StringCommandStyle(t *testing.T) {
	rd, _ := ParseRDString("65000:100")
	ip := netip.MustParseAddr("10.0.0.1")

	e := NewEVPNType3(rd, 200, ip)
	s := e.String()
	assert.Contains(t, s, "multicast", "should start with route type")
	assert.Contains(t, s, "rd set 0:65000:100", "rd should use 'set' keyword")
	assert.Contains(t, s, "ip set 10.0.0.1", "ip should use 'set' keyword")
	assert.Contains(t, s, "etag set 200", "etag should use 'set' keyword")
}

// TestEVPNType5StringCommandStyle verifies Type 5 String() outputs command-style format.
//
// VALIDATES: Type 5 (IP Prefix) output uses "ip-prefix rd set ... prefix set ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType5StringCommandStyle(t *testing.T) {
	rd, _ := ParseRDString("65000:100")
	prefix := netip.MustParsePrefix("10.0.0.0/24")

	e := NewEVPNType5(rd, [10]byte{}, 0, prefix, netip.Addr{}, []uint32{1000})
	s := e.String()
	assert.Contains(t, s, "ip-prefix", "should start with route type")
	assert.Contains(t, s, "rd set 0:65000:100", "rd should use 'set' keyword")
	assert.Contains(t, s, "prefix set 10.0.0.0/24", "prefix should use 'set' keyword")
	assert.Contains(t, s, "label set 1000", "label should use 'set' keyword")
}

// TestEVPNType1StringCommandStyle verifies Type 1 String() outputs command-style format.
//
// VALIDATES: Type 1 (Ethernet A-D) output uses "ethernet-ad rd set ... esi set ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType1StringCommandStyle(t *testing.T) {
	rd, _ := ParseRDString("65000:100")
	esi := [10]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}

	e := NewEVPNType1(rd, esi, 10, []uint32{1000})
	s := e.String()
	assert.Contains(t, s, "ethernet-ad", "should start with route type")
	assert.Contains(t, s, "rd set 0:65000:100", "rd should use 'set' keyword")
	assert.Contains(t, s, "esi set 00:01:02:03:04:05:06:07:08:09", "esi should use 'set' keyword")
	assert.Contains(t, s, "etag set 10", "etag should use 'set' keyword")
	assert.Contains(t, s, "label set 1000", "label should use 'set' keyword")
}

// TestEVPNType4StringCommandStyle verifies Type 4 String() outputs command-style format.
//
// VALIDATES: Type 4 (Ethernet Segment) output uses "ethernet-segment rd set ... esi set ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType4StringCommandStyle(t *testing.T) {
	rd, _ := ParseRDString("65000:100")
	esi := [10]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ip := netip.MustParseAddr("10.0.0.1")

	e := NewEVPNType4(rd, esi, ip)
	s := e.String()
	assert.Contains(t, s, "ethernet-segment", "should start with route type")
	assert.Contains(t, s, "rd set 0:65000:100", "rd should use 'set' keyword")
	assert.Contains(t, s, "esi set 00:01:02:03:04:05:06:07:08:09", "esi should use 'set' keyword")
	assert.Contains(t, s, "ip set 10.0.0.1", "ip should use 'set' keyword")
}
