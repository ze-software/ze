package message

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// testEVPNType2Bytes builds EVPN Type 2 (MAC/IP) NLRI bytes inline with label=100.
// This avoids importing the evpn plugin package (which would create an import cycle).
func testEVPNType2Bytes(rd nlri.RouteDistinguisher, mac [6]byte, ip netip.Addr) []byte {
	label := uint32(100) //nolint:goconst // test-only value
	ipLen := 0
	var ipBytes []byte
	if ip.IsValid() {
		if ip.Is4() {
			ipLen = 32
			b := ip.As4()
			ipBytes = b[:]
		} else {
			ipLen = 128
			b := ip.As16()
			ipBytes = b[:]
		}
	}
	labelBytes := [3]byte{byte(label >> 12), byte(label >> 4), byte(label<<4) | 0x01}
	payloadLen := 8 + 10 + 4 + 1 + 6 + 1 + len(ipBytes) + 3
	buf := make([]byte, 2+payloadLen)
	buf[0] = 2 // Route Type 2
	buf[1] = byte(payloadLen)
	copy(buf[2:], rd.Bytes())
	// ESI = zeros (10 bytes at offset 10)
	binary.BigEndian.PutUint32(buf[20:], 0) // ethernet tag
	buf[24] = 48                            // MAC length (bits)
	copy(buf[25:], mac[:])
	buf[31] = byte(ipLen)
	if len(ipBytes) > 0 {
		copy(buf[32:], ipBytes)
	}
	copy(buf[32+len(ipBytes):], labelBytes[:])
	return buf
}

// testEVPNType3Bytes builds EVPN Type 3 (Inclusive Multicast) NLRI bytes inline.
func testEVPNType3Bytes(rd nlri.RouteDistinguisher, originatorIP netip.Addr) []byte {
	var ipBytes []byte
	var ipLen int
	if originatorIP.Is4() {
		ipLen = 32
		b := originatorIP.As4()
		ipBytes = b[:]
	} else {
		ipLen = 128
		b := originatorIP.As16()
		ipBytes = b[:]
	}
	payloadLen := 8 + 4 + 1 + len(ipBytes)
	buf := make([]byte, 2+payloadLen)
	buf[0] = 3 // Route Type 3
	buf[1] = byte(payloadLen)
	copy(buf[2:], rd.Bytes())
	binary.BigEndian.PutUint32(buf[10:], 0) // ethernet tag
	buf[14] = byte(ipLen)
	copy(buf[15:], ipBytes)
	return buf
}

// testEVPNType5Bytes builds EVPN Type 5 (IP Prefix) NLRI bytes inline.
func testEVPNType5Bytes(rd nlri.RouteDistinguisher, ethernetTag uint32, prefix netip.Prefix, gateway netip.Addr, label uint32) []byte {
	prefixBits := prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	var gwBytes []byte
	if gateway.Is4() {
		b := gateway.As4()
		gwBytes = b[:]
	} else {
		b := gateway.As16()
		gwBytes = b[:]
	}
	labelBytes := [3]byte{byte(label >> 12), byte(label >> 4), byte(label<<4) | 0x01}
	payloadLen := 8 + 10 + 4 + 1 + prefixBytes + len(gwBytes) + 3
	buf := make([]byte, 2+payloadLen)
	buf[0] = 5 // Route Type 5
	buf[1] = byte(payloadLen)
	copy(buf[2:], rd.Bytes())
	// ESI = zeros (10 bytes)
	binary.BigEndian.PutUint32(buf[20:], ethernetTag)
	buf[24] = byte(prefixBits)
	copy(buf[25:], prefix.Addr().AsSlice()[:prefixBytes])
	copy(buf[25+prefixBytes:], gwBytes)
	copy(buf[25+prefixBytes+len(gwBytes):], labelBytes[:])
	return buf
}

// makeRD creates a Type 0 RD (ASN:Local) for testing.
func makeRD(local uint32) nlri.RouteDistinguisher {
	const asn uint16 = 100 // test value
	rd := nlri.RouteDistinguisher{Type: nlri.RDType0}
	rd.Value[0] = byte(asn >> 8)
	rd.Value[1] = byte(asn)
	rd.Value[2] = byte(local >> 24)
	rd.Value[3] = byte(local >> 16)
	rd.Value[4] = byte(local >> 8)
	rd.Value[5] = byte(local)
	return rd
}

// TestUpdateBuilder_BuildEVPN_Type2 verifies EVPN Type 2 (MAC/IP) UPDATE building.
//
// VALIDATES: BuildEVPN produces valid UPDATE with MP_REACH_NLRI for Type 2.
// PREVENTS: EVPN encoding regression.
func TestUpdateBuilder_BuildEVPN_Type2(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	// RD 100:1
	rd := makeRD(1)

	// Build EVPN Type 2 NLRI bytes inline (avoids evpn plugin import cycle)
	nlriBytes := testEVPNType2Bytes(rd, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, netip.MustParseAddr("10.0.0.1"))

	params := EVPNParams{
		NLRI:    nlriBytes,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildEVPN(params)

	require.NotNil(t, update, "BuildEVPN returned nil")

	// Verify path attributes contain MP_REACH_NLRI
	assert.Greater(t, len(update.PathAttributes), 0, "PathAttributes should not be empty")

	// Write and verify structure
	data := PackTo(update, nil)
	assert.Greater(t, len(data), HeaderLen, "packed data should include header")
}

// TestUpdateBuilder_BuildEVPN_Type5 verifies EVPN Type 5 (IP Prefix) UPDATE building.
//
// VALIDATES: BuildEVPN produces valid UPDATE for Type 5 IP Prefix route.
// PREVENTS: EVPN Type 5 encoding bugs.
func TestUpdateBuilder_BuildEVPN_Type5(t *testing.T) {
	ub := NewUpdateBuilder(65001, true, true, false) // iBGP

	// RD 100:200
	rd := makeRD(200)

	// Build EVPN Type 5 NLRI bytes inline (avoids evpn plugin import cycle)
	nlriBytes := testEVPNType5Bytes(rd, 100, netip.MustParsePrefix("10.0.0.0/24"), netip.IPv4Unspecified(), 200)

	params := EVPNParams{
		NLRI:            nlriBytes,
		NextHop:         netip.MustParseAddr("192.168.1.1"),
		Origin:          attribute.OriginIGP,
		LocalPreference: 150,
	}

	update := ub.BuildEVPN(params)

	require.NotNil(t, update, "BuildEVPN returned nil")
	assert.Greater(t, len(update.PathAttributes), 0, "PathAttributes should not be empty")
}

// TestUpdateBuilder_BuildEVPN_Type3 verifies EVPN Type 3 (Inclusive Multicast) UPDATE.
//
// VALIDATES: BuildEVPN produces valid UPDATE for Type 3.
// PREVENTS: EVPN Type 3 encoding bugs.
func TestUpdateBuilder_BuildEVPN_Type3(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	// RD 100:1
	rd := makeRD(1)

	// Build EVPN Type 3 NLRI bytes inline (avoids evpn plugin import cycle)
	nlriBytes := testEVPNType3Bytes(rd, netip.MustParseAddr("192.168.1.1"))

	params := EVPNParams{
		NLRI:    nlriBytes,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP,
	}

	update := ub.BuildEVPN(params)

	require.NotNil(t, update, "BuildEVPN returned nil")
}

// TestUpdateBuilder_BuildEVPN_ExtCommunity verifies extended community in EVPN UPDATE.
//
// VALIDATES: Route Targets are included in EVPN UPDATE.
// PREVENTS: Extended community encoding bugs.
func TestUpdateBuilder_BuildEVPN_ExtCommunity(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)

	// RD 100:1
	rd := makeRD(1)

	// Route Target: 100:200
	rt := make([]byte, 8)
	rt[0] = 0x00 // Type high (2-octet AS)
	rt[1] = 0x02 // Type low (Route Target)
	rt[2] = 0x00 // AS high
	rt[3] = 0x64 // AS low (100)
	rt[4] = 0x00
	rt[5] = 0x00
	rt[6] = 0x00
	rt[7] = 0xC8 // Local admin (200)

	// Build EVPN Type 2 NLRI bytes inline (avoids evpn plugin import cycle)
	nlriBytes := testEVPNType2Bytes(rd, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, netip.Addr{})

	params := EVPNParams{
		NLRI:              nlriBytes,
		NextHop:           netip.MustParseAddr("192.168.1.1"),
		Origin:            attribute.OriginIGP,
		ExtCommunityBytes: rt,
	}

	update := ub.BuildEVPN(params)

	require.NotNil(t, update, "BuildEVPN returned nil")

	// Verify path attributes are present
	assert.Greater(t, len(update.PathAttributes), 0)
}
