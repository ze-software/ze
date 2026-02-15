package peer

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/scenario"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateBuild verifies building a valid ipv4/unicast UPDATE
// from a generated route prefix.
//
// VALIDATES: UPDATE construction with correct attributes and NLRI.
// PREVENTS: Malformed UPDATE causing Ze to reject the route.
func TestUpdateBuild(t *testing.T) {
	cfg := SenderConfig{
		ASN:     65001,
		IsIBGP:  false,
		NextHop: netip.MustParseAddr("10.255.0.1"),
	}
	sender := NewSender(cfg)

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	data := sender.BuildRoute(prefix)

	require.NotNil(t, data)
	require.Greater(t, len(data), 19, "UPDATE must be larger than header")
	// Message type should be UPDATE (2).
	assert.Equal(t, byte(2), data[18])
}

// TestUpdateBuildIBGP verifies iBGP UPDATE includes LOCAL_PREF.
//
// VALIDATES: iBGP updates set LOCAL_PREF attribute.
// PREVENTS: Missing LOCAL_PREF causing iBGP route to be ignored.
func TestUpdateBuildIBGP(t *testing.T) {
	cfg := SenderConfig{
		ASN:     65000,
		IsIBGP:  true,
		NextHop: netip.MustParseAddr("10.255.0.1"),
	}
	sender := NewSender(cfg)

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	data := sender.BuildRoute(prefix)

	require.NotNil(t, data)
	// Should be a valid UPDATE.
	assert.Equal(t, byte(2), data[18])
	// iBGP UPDATE should be larger (has LOCAL_PREF attribute).
	assert.Greater(t, len(data), 40, "iBGP UPDATE should include LOCAL_PREF")
}

// TestEORBuild verifies building an End-of-RIB marker for ipv4/unicast.
//
// VALIDATES: EOR is an empty UPDATE per RFC 4724.
// PREVENTS: Wrong EOR format causing Ze to misinterpret the marker.
func TestEORBuild(t *testing.T) {
	data := BuildEOR("ipv4/unicast")

	require.NotNil(t, data)
	// Message type should be UPDATE (2).
	assert.Equal(t, byte(2), data[18])
	// IPv4 unicast EOR is an empty UPDATE: header (19) + withdrawn len (2) + attr len (2) = 23.
	assert.Equal(t, 23, len(data), "IPv4 unicast EOR should be 23 bytes")
}

// TestMultipleRoutesDifferent verifies that building routes for different
// prefixes produces different wire bytes.
//
// VALIDATES: Each prefix produces unique UPDATE.
// PREVENTS: Builder reusing stale state between calls.
func TestMultipleRoutesDifferent(t *testing.T) {
	cfg := SenderConfig{
		ASN:     65001,
		IsIBGP:  false,
		NextHop: netip.MustParseAddr("10.255.0.1"),
	}
	sender := NewSender(cfg)

	data1 := sender.BuildRoute(netip.MustParsePrefix("10.0.0.0/24"))
	data2 := sender.BuildRoute(netip.MustParsePrefix("10.0.1.0/24"))

	require.NotNil(t, data1)
	require.NotNil(t, data2)
	assert.NotEqual(t, data1, data2, "different prefixes should produce different UPDATEs")
}

// TestBuildWithdrawalSingle verifies a single-prefix withdrawal UPDATE.
//
// VALIDATES: Withdrawal UPDATE contains the prefix in WithdrawnRoutes section.
// PREVENTS: Withdrawal encoded as announcement instead of withdrawal.
func TestBuildWithdrawalSingle(t *testing.T) {
	data := BuildWithdrawal([]netip.Prefix{netip.MustParsePrefix("10.0.1.0/24")})

	require.NotNil(t, data)
	assert.Equal(t, byte(2), data[18], "message type should be UPDATE")

	// After header (19 bytes): withdrawn-len (2) + withdrawn data + attr-len (2, value 0).
	// Withdrawn: /24 = 1 (len) + 3 (bytes) = 4 bytes.
	// Total: 19 + 2 + 4 + 2 = 27.
	assert.Equal(t, 27, len(data), "single /24 withdrawal should be 27 bytes")

	// Verify withdrawn routes length field.
	withdrawnLen := int(data[19])<<8 | int(data[20])
	assert.Equal(t, 4, withdrawnLen, "withdrawn routes length should be 4")

	// Verify path attributes length is 0.
	attrOff := 19 + 2 + withdrawnLen
	attrLen := int(data[attrOff])<<8 | int(data[attrOff+1])
	assert.Equal(t, 0, attrLen, "path attributes length should be 0")
}

// TestBuildWithdrawalMultiple verifies multiple prefixes in one withdrawal.
//
// VALIDATES: All prefixes encoded in withdrawn section.
// PREVENTS: Only first prefix being withdrawn.
func TestBuildWithdrawalMultiple(t *testing.T) {
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("10.0.1.0/24"),
		netip.MustParsePrefix("10.0.2.0/24"),
		netip.MustParsePrefix("10.0.3.0/24"),
	}
	data := BuildWithdrawal(prefixes)

	require.NotNil(t, data)
	// 3 x /24 = 3 x 4 bytes = 12 bytes withdrawn.
	// Total: 19 + 2 + 12 + 2 = 35.
	assert.Equal(t, 35, len(data))

	withdrawnLen := int(data[19])<<8 | int(data[20])
	assert.Equal(t, 12, withdrawnLen)
}

// TestBuildWithdrawalEmpty verifies empty prefix list returns nil.
//
// VALIDATES: No UPDATE produced for zero withdrawals.
// PREVENTS: Sending empty UPDATEs that confuse Ze.
func TestBuildWithdrawalEmpty(t *testing.T) {
	data := BuildWithdrawal(nil)
	assert.Nil(t, data)

	data = BuildWithdrawal([]netip.Prefix{})
	assert.Nil(t, data)
}

// TestBuildMalformedUpdate verifies malformed UPDATE construction.
//
// VALIDATES: Malformed UPDATE has valid BGP framing but invalid ORIGIN value.
// PREVENTS: Malformed message rejected by Ze before reaching error handling.
func TestBuildMalformedUpdate(t *testing.T) {
	data := BuildMalformedUpdate()

	require.NotNil(t, data)
	require.Greater(t, len(data), 19, "must be larger than header")

	// Valid BGP marker (16 bytes of 0xFF).
	for i := range 16 {
		assert.Equal(t, byte(0xFF), data[i], "marker byte %d", i)
	}

	// Message type should be UPDATE (2).
	assert.Equal(t, byte(2), data[18])

	// Length field should match actual length.
	msgLen := int(data[16])<<8 | int(data[17])
	assert.Equal(t, len(data), msgLen)

	// After header: withdrawn-len = 0.
	assert.Equal(t, byte(0), data[19])
	assert.Equal(t, byte(0), data[20])

	// Total path attribute length > 0 (has the malformed attribute).
	attrLen := int(data[21])<<8 | int(data[22])
	assert.Greater(t, attrLen, 0, "should have path attributes")
}

// TestBuildVPNRoute verifies building a VPN UPDATE.
//
// VALIDATES: VPN UPDATE produces valid wire bytes with NLRI in MP_REACH.
// PREVENTS: VPN route construction failure breaking multi-family support.
func TestBuildVPNRoute(t *testing.T) {
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsIBGP:  true,
		NextHop: netip.MustParseAddr("10.255.0.1"),
	})

	route := scenario.VPNRoute{
		RDBytes: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
		Labels:  []uint32{100},
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		Key:     "vpn-test",
	}

	data := sender.BuildVPNRoute(route)

	require.NotNil(t, data)
	assert.Greater(t, len(data), 19, "VPN UPDATE must be larger than header")
	assert.Equal(t, byte(2), data[18], "message type should be UPDATE")
}

// TestBuildEVPNRoute verifies building an EVPN Type-2 UPDATE.
//
// VALIDATES: EVPN UPDATE produces valid wire bytes with Type-2 NLRI.
// PREVENTS: EVPN NLRI construction failure from RD encoding errors.
func TestBuildEVPNRoute(t *testing.T) {
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsIBGP:  true,
		NextHop: netip.MustParseAddr("10.255.0.1"),
	})

	route := scenario.EVPNRoute{
		RDBytes:     [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
		MAC:         [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		IP:          netip.MustParseAddr("10.0.0.1"),
		EthernetTag: 0,
		Labels:      []uint32{200},
		Key:         "evpn-test",
	}

	data := sender.BuildEVPNRoute(route)

	require.NotNil(t, data)
	assert.Greater(t, len(data), 19, "EVPN UPDATE must be larger than header")
	assert.Equal(t, byte(2), data[18], "message type should be UPDATE")
}

// TestBuildFlowSpecRoute verifies building a FlowSpec UPDATE.
//
// VALIDATES: FlowSpec UPDATE produces valid wire bytes with flow components.
// PREVENTS: FlowSpec component encoding errors breaking the UPDATE.
func TestBuildFlowSpecRoute(t *testing.T) {
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsIBGP:  true,
		NextHop: netip.MustParseAddr("10.255.0.1"),
	})

	route := scenario.FlowSpecRoute{
		DestPrefix:   netip.MustParsePrefix("10.0.0.0/24"),
		SourcePrefix: netip.MustParsePrefix("192.168.1.0/24"),
		IsIPv6:       false,
		Key:          "flow-test",
	}

	data := sender.BuildFlowSpecRoute(route)

	require.NotNil(t, data)
	assert.Greater(t, len(data), 19, "FlowSpec UPDATE must be larger than header")
	assert.Equal(t, byte(2), data[18], "message type should be UPDATE")
}

// TestBuildEORGeneric verifies the generic BuildEOR for all families.
//
// VALIDATES: EOR construction works for all 7 supported families.
// PREVENTS: Missing family in familyToNLRI map causing nil EOR.
func TestBuildEORGeneric(t *testing.T) {
	families := []string{
		"ipv4/unicast", "ipv6/unicast",
		"ipv4/vpn", "ipv6/vpn",
		"l2vpn/evpn",
		"ipv4/flow", "ipv6/flow",
	}

	for _, f := range families {
		t.Run(f, func(t *testing.T) {
			data := BuildEOR(f)
			require.NotNil(t, data, "EOR should not be nil for %s", f)
			assert.Equal(t, byte(2), data[18], "message type should be UPDATE")
		})
	}
}

// TestBuildEORUnknownFamily verifies BuildEOR returns nil for unknown families.
//
// VALIDATES: Unknown family strings are handled gracefully.
// PREVENTS: Panic or invalid message from unsupported family.
func TestBuildEORUnknownFamily(t *testing.T) {
	data := BuildEOR("bogus/family")
	assert.Nil(t, data)
}

// TestBuildWithdrawalRoundTrip verifies withdrawn prefixes can be parsed back.
//
// VALIDATES: Wire encoding matches the format parseIPv4Prefix expects.
// PREVENTS: Incompatible encoding between sender and receiver.
func TestBuildWithdrawalRoundTrip(t *testing.T) {
	original := []netip.Prefix{
		netip.MustParsePrefix("10.0.1.0/24"),
		netip.MustParsePrefix("172.16.0.0/16"),
	}
	data := BuildWithdrawal(original)
	require.NotNil(t, data)

	// Parse the withdrawn routes section.
	withdrawnLen := int(data[19])<<8 | int(data[20])
	withdrawn := data[21 : 21+withdrawnLen]

	var parsed []netip.Prefix
	off := 0
	for off < len(withdrawn) {
		prefix, n := parseIPv4Prefix(withdrawn[off:])
		require.Greater(t, n, 0, "should parse a prefix")
		parsed = append(parsed, prefix)
		off += n
	}

	assert.Equal(t, original, parsed)
}
