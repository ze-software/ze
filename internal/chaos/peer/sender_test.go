package peer

import (
	"bytes"
	"io"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/component/bgp/message"
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
	data := buildWithdrawal([]netip.Prefix{netip.MustParsePrefix("10.0.1.0/24")})

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
	data := buildWithdrawal(prefixes)

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
	data := buildWithdrawal(nil)
	assert.Nil(t, data)

	data = buildWithdrawal([]netip.Prefix{})
	assert.Nil(t, data)
}

// TestBuildMalformedUpdate verifies malformed UPDATE construction.
//
// VALIDATES: Malformed UPDATE has valid BGP framing but invalid ORIGIN value.
// PREVENTS: Malformed message rejected by Ze before reaching error handling.
func TestBuildMalformedUpdate(t *testing.T) {
	data := buildMalformedUpdate()

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

	// Trailing NLRI must be present so RFC 7606 Section 5.2 does not escalate to session-reset.
	nlriOff := 23 + attrLen
	require.Less(t, nlriOff, len(data), "malformed UPDATE must include an NLRI for treat-as-withdraw")
	result := message.ValidateUpdateRFC7606(data[23:nlriOff], true, false, true)
	require.NotNil(t, result)
	assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, result.Action)
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

	data := sender.buildVPNRoute(route)

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

	data := sender.buildEVPNRoute(route)

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

	data := sender.buildFlowSpecRoute(route)

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
		"ipv4/mpls-vpn", "ipv6/mpls-vpn",
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
	data := buildWithdrawal(original)
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

// TestSendWithdrawalBatches validates bounded UPDATEs and the wire data in their events.
//
// VALIDATES: The first UPDATE fits the legacy limit exactly and the second carries
// the remaining prefix. Both events describe the bytes written to the connection.
// PREVENTS: Oversized withdrawal UPDATEs or incomplete route and MRT observations.
func TestSendWithdrawalBatches(t *testing.T) {
	prefixes := withdrawalTestPrefixes()
	var wire bytes.Buffer
	var events []Event
	err := sendWithdrawal(&wire, prefixes, func(event Event) { events = append(events, event) })
	require.NoError(t, err)
	require.Len(t, events, 2)

	firstCount := len(prefixes) - 1
	assert.Equal(t, prefixes[:firstCount], events[0].WithdrawnPrefixes)
	assert.Equal(t, firstCount, events[0].Count)
	assert.Equal(t, prefixes[firstCount:], events[1].WithdrawnPrefixes)
	assert.Equal(t, 1, events[1].Count)
	assert.Len(t, events[0].BGPMessage, message.MaxMsgLen)
	assert.Len(t, events[1].BGPMessage, message.MinUpdateLen+4)

	for _, event := range events {
		header, parseErr := message.ParseHeader(event.BGPMessage)
		require.NoError(t, parseErr)
		assert.Equal(t, len(event.BGPMessage), int(header.Length))
		assert.Equal(t, int64(header.Length), event.BytesSent)
	}
	assert.Equal(t, wire.Bytes(), append(slices.Clone(events[0].BGPMessage), events[1].BGPMessage...))
}

// TestSendWithdrawalPreservesCompletedBatches validates the observation after a write fails.
//
// VALIDATES: A second-batch failure retains the first batch as committed evidence.
// PREVENTS: Losing the successful batch or recording a batch the writer rejected.
func TestSendWithdrawalPreservesCompletedBatches(t *testing.T) {
	prefixes := withdrawalTestPrefixes()
	var writes int
	writer := withdrawalTestWriter(func(data []byte) (int, error) {
		writes++
		if writes == 2 {
			return 0, io.ErrClosedPipe
		}
		return len(data), nil
	})
	var events []Event
	err := sendWithdrawal(writer, prefixes, func(event Event) { events = append(events, event) })

	require.ErrorIs(t, err, io.ErrClosedPipe)
	assert.Equal(t, 2, writes)
	require.Len(t, events, 1)
	assert.Equal(t, prefixes[:len(prefixes)-1], events[0].WithdrawnPrefixes)
}

func withdrawalTestPrefixes() []netip.Prefix {
	// 814 /32s plus one /16 occupy the 4073 route octets in a legacy UPDATE.
	prefixes := make([]netip.Prefix, 0, 816)
	for i := range 814 {
		addr := netip.AddrFrom4([4]byte{10, 0, byte(i >> 8), byte(i & 255)})
		prefixes = append(prefixes, netip.PrefixFrom(addr, 32))
	}
	return append(prefixes, netip.MustParsePrefix("172.16.0.0/16"), netip.MustParsePrefix("192.0.2.0/24"))
}

type withdrawalTestWriter func([]byte) (int, error)

func (w withdrawalTestWriter) Write(data []byte) (int, error) {
	return w(data)
}
