package nlri

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	data := []byte{byte(EVPNRouteType2)}
	data = append(data, byte(8+10+4+1+6+1+3)) // Length
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ethTag...)
	data = append(data, macLen)
	data = append(data, mac...)
	data = append(data, ipLen)
	data = append(data, label...)

	nlri, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := nlri.(*EVPNType2)
	require.True(t, ok)

	assert.Equal(t, EVPNRouteType2, evpn.RouteType())
	assert.Equal(t, "65000:100", evpn.RD().String())
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

	data := []byte{byte(EVPNRouteType2)}
	data = append(data, byte(8+10+4+1+6+1+4+3)) // Length
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ethTag...)
	data = append(data, macLen)
	data = append(data, mac...)
	data = append(data, ipLen)
	data = append(data, ip...)
	data = append(data, label...)

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

	data := []byte{byte(EVPNRouteType2)}
	data = append(data, byte(8+10+4+1+6+1+16+3)) // Length
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ethTag...)
	data = append(data, macLen)
	data = append(data, mac...)
	data = append(data, ipLen)
	data = append(data, ip...)
	data = append(data, label...)

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

	data := []byte{byte(EVPNRouteType3)}
	data = append(data, byte(8+4+1+4)) // Length
	data = append(data, rd...)
	data = append(data, ethTag...)
	data = append(data, ipLen)
	data = append(data, ip...)

	nlri, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType3)
	require.True(t, ok)

	assert.Equal(t, EVPNRouteType3, evpn.RouteType())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), evpn.OriginatorIP())
}

// TestEVPNType5 verifies Type 5 IP Prefix route parsing.
//
// VALIDATES: IP Prefix route (used for L3VPN over EVPN).
//
// PREVENTS: IP Prefix route parsing failures.
func TestEVPNType5(t *testing.T) {
	// Type 5: IP Prefix
	// RD (8) + ESI (10) + EthTag (4) + IPLen (1) + IP (prefix) + GW (IP) + Label (3)
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(24)        // /24 prefix
	ip := []byte{10, 1, 2}   // 10.1.2.0/24
	gw := []byte{0, 0, 0, 0} // No gateway
	label := []byte{0x00, 0x01, 0x01}

	data := []byte{byte(EVPNRouteType5)}
	data = append(data, byte(8+10+4+1+3+4+3)) // Length
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ethTag...)
	data = append(data, ipLen)
	data = append(data, ip...)
	data = append(data, gw...)
	data = append(data, label...)

	nlri, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := nlri.(*EVPNType5)
	require.True(t, ok)

	assert.Equal(t, EVPNRouteType5, evpn.RouteType())
	assert.Equal(t, netip.MustParsePrefix("10.1.2.0/24"), evpn.Prefix())
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

	data := []byte{byte(EVPNRouteType1)}
	data = append(data, byte(8+10+4+3)) // Length = 25
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ethTag...)
	data = append(data, label...)

	nlri, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := nlri.(*EVPNType1)
	require.True(t, ok, "expected EVPNType1, got %T", nlri)

	assert.Equal(t, EVPNRouteType1, evpn.RouteType())
	assert.Equal(t, "65000:100", evpn.RD().String())
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
	assert.True(t, evpn.HasPathID())
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

	data := []byte{byte(EVPNRouteType4)}
	data = append(data, byte(8+10+1+4)) // Length = 23
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ipLen)
	data = append(data, ip...)

	nlri, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := nlri.(*EVPNType4)
	require.True(t, ok, "expected EVPNType4, got %T", nlri)

	assert.Equal(t, EVPNRouteType4, evpn.RouteType())
	assert.Equal(t, "65000:100", evpn.RD().String())
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

	data := []byte{byte(EVPNRouteType4)}
	data = append(data, byte(8+10+1+16)) // Length = 35
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ipLen)
	data = append(data, ip...)

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

	data := []byte{byte(EVPNRouteType4)}
	data = append(data, byte(8+10+1+8))
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ipLen)
	data = append(data, ip...)

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
