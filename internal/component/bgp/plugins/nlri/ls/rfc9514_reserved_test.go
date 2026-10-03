package ls

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 9514 marks a Reserved octet or reserved flag bits in each SRv6 TLV as "MUST be
// ignored on receipt". The decoders below never read those bits, so a sender that sets
// them changes nothing ze decodes and is never refused.

// srv6EndXValue builds an SRv6 End.X SID TLV value: Behavior(2) Flags(1) Algorithm(1)
// Weight(1) Reserved(1) [Neighbor ID] SID(16).
func srv6EndXValue(reserved byte, neighborID []byte) []byte {
	v := []byte{0x00, 0x05, 0x80, 0x00, 0x0a, reserved}
	v = append(v, neighborID...)
	sid := make([]byte, 16)
	sid[0], sid[1], sid[15] = 0x20, 0x01, 0x01
	return append(v, sid...)
}

// TestRFC9514EndXSIDReservedIgnored pins the End.X and LAN End.X Reserved octet.
//
// VALIDATES: RFC9514-4.1-2 and RFC9514-4.2-2, the Reserved octet decodes to nothing and
// refuses nothing.
// PREVENTS: a decoder that rejects or exposes a Reserved octet a sender left non-zero.
func TestRFC9514EndXSIDReservedIgnored(t *testing.T) {
	cases := []struct {
		name       string
		code       uint16
		neighborID []byte
	}{
		{"end.x 1106", TLVSRv6EndXSID, nil},
		{"lan end.x is-is 1107", TLVSRv6LANEndXISIS, []byte{0x19, 0x20, 0x00, 0x00, 0x00, 0x01}},
		{"lan end.x ospfv3 1108", TLVSRv6LANEndXOSPF, []byte{0x0a, 0x00, 0x00, 0x01}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// RFC requirement: RFC9514-4.1-2 positive -- an End.X SID TLV with Reserved 0xFF decodes to the same TLV as one with Reserved 0 (§4.1).
			// RFC requirement: RFC9514-4.1-2 negative -- a non-zero Reserved octet is never refused: no error, and no decoded field carries it (§4.1).
			// RFC requirement: RFC9514-4.2-2 positive -- a LAN End.X SID TLV (IS-IS 1107, OSPFv3 1108) with Reserved 0xFF decodes to the same TLV as one with Reserved 0 (§4.2).
			// RFC requirement: RFC9514-4.2-2 negative -- a non-zero Reserved octet in a LAN End.X SID TLV is never refused: no error, and no decoded field carries it (§4.2).
			decode := decodeSRv6EndXSID(tc.code, len(tc.neighborID))

			zero, err := decode(srv6EndXValue(0x00, tc.neighborID))
			require.NoError(t, err)
			set, err := decode(srv6EndXValue(0xFF, tc.neighborID))
			require.NoError(t, err, "a non-zero Reserved octet is not refused")

			assert.Equal(t, zero, set, "the Reserved octet reaches no decoded field")
			got, ok := set.(*lsSRv6EndXSID)
			require.True(t, ok)
			assert.Equal(t, uint16(5), got.EndpointBehavior)
			assert.Equal(t, uint8(0x80), got.Flags)
			assert.Equal(t, uint8(0x0a), got.Weight)
			assert.Equal(t, tc.neighborID, got.NeighborID)
		})
	}
}

// TestRFC9514EndpointBehaviorUndefinedFlagsIgnored pins the Endpoint Behavior TLV flags.
//
// VALIDATES: RFC9514-7.1-4, undefined flag bits refuse nothing and alter no other field.
// PREVENTS: rejecting a TLV over flag bits a later document may define.
//
// RFC requirement: RFC9514-7.1-4 positive -- an SRv6 Endpoint Behavior TLV with every flag bit set decodes with the same Behavior and Algorithm as one with no flag set (§7.1).
// RFC requirement: RFC9514-7.1-4 negative -- undefined flag bits never cause the TLV to be refused (§7.1).
func TestRFC9514EndpointBehaviorUndefinedFlagsIgnored(t *testing.T) {
	clear, err := decodeSRv6EndpointBehavior([]byte{0x00, 0x13, 0x00, 0x80})
	require.NoError(t, err)
	set, err := decodeSRv6EndpointBehavior([]byte{0x00, 0x13, 0xFF, 0x80})
	require.NoError(t, err, "undefined flags are not refused")

	want, ok := clear.(*LsSRv6EndpointBehavior)
	require.True(t, ok)
	got, ok := set.(*LsSRv6EndpointBehavior)
	require.True(t, ok)
	assert.Equal(t, want.EndpointBehavior, got.EndpointBehavior)
	assert.Equal(t, want.Algorithm, got.Algorithm)
	assert.Equal(t, uint16(0x13), got.EndpointBehavior)
	assert.Equal(t, uint8(0x80), got.Algorithm)
}

// srv6PeerNodeValue builds an SRv6 BGP PeerNode SID TLV value: Flags(1) Weight(1)
// Reserved(2) Peer AS(4) Peer BGP-ID(4).
func srv6PeerNodeValue(flags byte, reserved [2]byte) []byte {
	return []byte{flags, 0x07, reserved[0], reserved[1], 0x00, 0x00, 0xfd, 0xe8, 0xc0, 0x00, 0x02, 0x01}
}

// TestRFC9514PeerNodeSIDReservedIgnored pins the PeerNode SID reserved flag bits and
// Reserved field.
//
// VALIDATES: RFC9514-7.2-5 and RFC9514-7.2-6, reserved flag bits and the Reserved field
// refuse nothing and alter no other field.
// PREVENTS: a decoder that reads meaning into bits the RFC leaves for the future.
func TestRFC9514PeerNodeSIDReservedIgnored(t *testing.T) {
	// B, S and P are the defined flags (0xE0); the low five bits are reserved.
	base, err := decodeSRv6BGPPeerNodeSID(srv6PeerNodeValue(0xE0, [2]byte{0, 0}))
	require.NoError(t, err)
	want, ok := base.(*LsSRv6BGPPeerNodeSID)
	require.True(t, ok)

	t.Run("reserved flag bits", func(t *testing.T) {
		// RFC requirement: RFC9514-7.2-5 positive -- a PeerNode SID TLV with the reserved flag bits set decodes the same Weight, Peer AS and Peer BGP-ID as one with them clear (§7.2).
		// RFC requirement: RFC9514-7.2-5 negative -- the reserved flag bits never cause the TLV to be refused (§7.2).
		set, err := decodeSRv6BGPPeerNodeSID(srv6PeerNodeValue(0xFF, [2]byte{0, 0}))
		require.NoError(t, err, "reserved flag bits are not refused")
		got, ok := set.(*LsSRv6BGPPeerNodeSID)
		require.True(t, ok)
		assert.Equal(t, want.Weight, got.Weight)
		assert.Equal(t, want.PeerAS, got.PeerAS)
		assert.Equal(t, want.PeerBGPID, got.PeerBGPID)
		assert.Equal(t, uint32(65000), got.PeerAS)
	})

	t.Run("reserved field", func(t *testing.T) {
		// RFC requirement: RFC9514-7.2-6 positive -- a PeerNode SID TLV with Reserved 0xFFFF decodes to the same TLV as one with Reserved 0 (§7.2).
		// RFC requirement: RFC9514-7.2-6 negative -- a non-zero Reserved field is never refused, and no decoded field carries it (§7.2).
		set, err := decodeSRv6BGPPeerNodeSID(srv6PeerNodeValue(0xE0, [2]byte{0xFF, 0xFF}))
		require.NoError(t, err, "a non-zero Reserved field is not refused")
		assert.Equal(t, base, set, "the Reserved field reaches no decoded field")
	})
}

// TestRFC9514CapabilitiesDecode checks the operator-facing attribute decoder
// with a nonzero capability flag word and a following attribute.
// RFC requirement: RFC9514-3.1-3 positive -- with Reserved zero, the consumer decodes the capability flags and the following Node Name.
func TestRFC9514CapabilitiesDecode(t *testing.T) {
	attr := buildAttrTLV(nil, 1038, []byte{0x80, 0x01, 0, 0})
	attr = buildAttrTLV(attr, TLVNodeName, []byte("core1"))
	assert.Equal(t, map[string]any{
		"srv6-capabilities": map[string]any{"flags": 32769},
		"node-name":         "core1",
	}, AttrTLVsToJSON(attr))
}

// TestRFC9514CapabilitiesReservedIgnored excludes the reserved word from the
// decoded capability flags, including when every reserved bit is set.
// RFC requirement: RFC9514-3.1-3 negative -- nonzero Reserved does not change the decoded capability flags or suppress the following Node Name.
func TestRFC9514CapabilitiesReservedIgnored(t *testing.T) {
	attr := buildAttrTLV(nil, 1038, []byte{0x40, 0x02, 0xff, 0xff})
	attr = buildAttrTLV(attr, TLVNodeName, []byte("core2"))
	assert.Equal(t, map[string]any{
		"srv6-capabilities": map[string]any{"flags": 16386},
		"node-name":         "core2",
	}, AttrTLVsToJSON(attr))
}

// TestRFC9514LocatorDecode checks all defined fields and retains unknown
// sub-TLV bytes in the operator-facing view.
// RFC requirement: RFC9514-5.1-2 positive -- with Reserved zero, the consumer decodes locator flags, algorithm and metric without treating the reserved word as data.
func TestRFC9514LocatorDecode(t *testing.T) {
	attr := buildAttrTLV(nil, 1162, []byte{
		0x80, 128, 0, 0, 0x01, 0x02, 0x03, 0x04,
		0xfe, 0x01, 0, 1, 0xa5,
	})
	attr = buildAttrTLV(attr, TLVNodeName, []byte("core1"))
	got := AttrTLVsToJSON(attr)
	locator, ok := got["srv6-locator"].(map[string]any)
	require.True(t, ok)
	text, ok := locator["sub-tlvs"].(string)
	require.True(t, ok)
	value, prefixed := strings.CutPrefix(text, "0x")
	require.True(t, prefixed)
	subTLVs, err := hex.DecodeString(value)
	require.NoError(t, err)
	locator["sub-tlvs"] = subTLVs
	assert.Equal(t, map[string]any{
		"srv6-locator": map[string]any{
			"flags": 128, "algorithm": 128, "metric": uint32(0x01020304),
			"sub-tlvs": []byte{0xfe, 0x01, 0, 1, 0xa5},
		},
		"node-name": "core1",
	}, got)
}

// TestRFC9514LocatorReservedIgnored checks the nonzero-reserved case through
// the same attribute registry used by the CLI decoder.
// RFC requirement: RFC9514-5.1-2 negative -- nonzero Reserved neither changes locator flags, algorithm and metric nor suppresses the following Node Name.
func TestRFC9514LocatorReservedIgnored(t *testing.T) {
	attr := buildAttrTLV(nil, 1162, []byte{0, 129, 0xff, 0xff, 0, 0, 0, 42})
	attr = buildAttrTLV(attr, TLVNodeName, []byte("core2"))
	assert.Equal(t, map[string]any{
		"srv6-locator": map[string]any{
			"flags": 0, "algorithm": 129, "metric": uint32(42),
		},
		"node-name": "core2",
	}, AttrTLVsToJSON(attr))
}
