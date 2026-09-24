package ls

// RFC 9552 receive-side handling of the undefined bits in the two one-octet flag
// TLVs: Node Flag Bits (TLV 1024, Section 5.3.1.1) and IGP Flags (TLV 1152,
// Section 5.3.3.1). Both sections end with the same sentence: "The bits that are
// not defined MUST be set to 0 by the originator and MUST be ignored by the
// receiver."
//
// The public attribute decoder serves the offline CLI. The propagation path
// retains attribute bytes and does not call these semantic decoders. The tests
// below cover the receive half of the requirement, not originator policy.
//
// "Ignored" has two halves, and each test asserts both. The undefined bits do
// not change what the defined bits decode to, and they do not stop the walk: the
// TLV after the flag TLV still decodes.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flagAttrWithTrailer builds a BGP-LS Attribute holding one one-octet flag TLV
// followed by a Node Name TLV, so a test can see whether the walk continued past
// the flags.
func flagAttrWithTrailer(flagType uint16, flags byte) []byte {
	var attr []byte
	attr = buildAttrTLV(attr, flagType, []byte{flags})
	attr = buildAttrTLV(attr, TLVNodeName, []byte("core1"))
	return attr
}

// definedFlagBits returns the defined bits of a decoded flag TLV's JSON, minus
// the reserved key, so two decodes can be compared on the bits the RFC names.
func definedFlagBits(t *testing.T, tlv lsAttrTLV, jsonKey string) map[string]any {
	t.Helper()
	js, ok := tlv.ToJSON()[jsonKey].(map[string]any)
	require.True(t, ok, "flag TLV JSON is keyed %q", jsonKey)
	defined := make(map[string]any, len(js))
	for k, v := range js {
		if k == jsonKeyReserved {
			continue
		}
		defined[k] = v
	}
	return defined
}

// TestRFC9552NodeFlagBitsUndefinedBitsIgnored proves the receiver ignores the
// two low bits of the Node Flag Bits TLV, which RFC 9552 Table 7 leaves
// undefined. decodeNodeFlagBits returns the octet whole and nothing in the walk
// branches on bits 6-7, so a peer that sets them yields the same O, A, E, B, R
// and V bits as one that clears them, and the TLV after it still decodes.
//
// VALIDATES: undefined bits set and clear give one set of defined bits, no
// decode error, and the walk reaches the next TLV.
// PREVENTS: a check on the undefined bits being added to the receive path and
// dropping a node a conforming peer sent.
func TestRFC9552NodeFlagBitsUndefinedBitsIgnored(t *testing.T) {
	// RFC requirement: RFC9552-5.3.1.1-1 positive -- the undefined low two bits of the Node Flag Bits TLV are ignored on receipt: set and clear decode to the same O, A, E, B, R and V bits, without error, and the TLV after it still decodes (§5.3.1.1)
	const definedOnly byte = 0xA8   // O=1 A=0 E=1 B=0 R=1 V=0, undefined bits 0.
	const withUndefined byte = 0xAB // the same six defined bits, undefined bits 1.

	clear, err := decodeAllAttrTLVs(flagAttrWithTrailer(TLVNodeFlagBits, definedOnly))
	require.NoError(t, err, "undefined bits clear: the attribute decodes")
	set, err := decodeAllAttrTLVs(flagAttrWithTrailer(TLVNodeFlagBits, withUndefined))
	require.NoError(t, err, "undefined bits set: the attribute still decodes")

	require.Len(t, clear, 2, "flag TLV and the Node Name after it")
	require.Len(t, set, 2, "the walk does not stop at undefined bits")

	clearFlags, ok := clear[0].(*lsNodeFlagBits)
	require.True(t, ok, "TLV 1024 decodes to lsNodeFlagBits")
	setFlags, ok := set[0].(*lsNodeFlagBits)
	require.True(t, ok, "TLV 1024 with undefined bits set decodes to lsNodeFlagBits")

	assert.Equal(t, definedOnly, clearFlags.Flags, "the octet sent is the octet decoded")
	assert.Equal(t, definedFlagBits(t, clearFlags, "node-flags"), definedFlagBits(t, setFlags, "node-flags"),
		"the undefined bits carry no meaning: every defined bit reads the same")

	name, ok := set[1].(*lsNodeName)
	require.True(t, ok, "the TLV after the flags still decodes")
	assert.Equal(t, "core1", name.Name)
}

// TestRFC9552IGPFlagsUndefinedBitsIgnored proves the receiver ignores the four
// low bits of the IGP Flags TLV, which RFC 9552 Table 11 leaves undefined.
// decodeIGPFlags returns the octet whole and nothing in the walk branches on
// bits 4-7, so a peer that sets them yields the same D, N, L and P bits as one
// that clears them, and the TLV after it still decodes.
//
// VALIDATES: undefined bits set and clear give one set of defined bits, no
// decode error, and the walk reaches the next TLV.
// PREVENTS: a check on the undefined bits being added to the receive path and
// dropping a prefix a conforming peer sent.
func TestRFC9552IGPFlagsUndefinedBitsIgnored(t *testing.T) {
	// RFC requirement: RFC9552-5.3.3.1-1 positive -- the undefined low four bits of the IGP Flags TLV are ignored on receipt: set and clear decode to the same D, N, L and P bits, without error, and the TLV after it still decodes (§5.3.3.1)
	const definedOnly byte = 0xA0   // D=1 N=0 L=1 P=0, undefined bits 0.
	const withUndefined byte = 0xAF // the same four defined bits, undefined bits 1.

	clear, err := decodeAllAttrTLVs(flagAttrWithTrailer(TLVIGPFlags, definedOnly))
	require.NoError(t, err, "undefined bits clear: the attribute decodes")
	set, err := decodeAllAttrTLVs(flagAttrWithTrailer(TLVIGPFlags, withUndefined))
	require.NoError(t, err, "undefined bits set: the attribute still decodes")

	require.Len(t, clear, 2, "flag TLV and the Node Name after it")
	require.Len(t, set, 2, "the walk does not stop at undefined bits")

	clearFlags, ok := clear[0].(*lsIGPFlags)
	require.True(t, ok, "TLV 1152 decodes to lsIGPFlags")
	setFlags, ok := set[0].(*lsIGPFlags)
	require.True(t, ok, "TLV 1152 with undefined bits set decodes to lsIGPFlags")

	assert.Equal(t, definedOnly, clearFlags.Flags, "the octet sent is the octet decoded")
	assert.Equal(t, definedFlagBits(t, clearFlags, "igp-flags"), definedFlagBits(t, setFlags, "igp-flags"),
		"the undefined bits carry no meaning: every defined bit reads the same")

	name, ok := set[1].(*lsNodeName)
	require.True(t, ok, "the TLV after the flags still decodes")
	assert.Equal(t, "core1", name.Name)
}

// TestRFC9552UndefinedNodeBitsDoNotSetDefinedFlags checks that reserved-only
// flags cannot turn on a defined node capability in the consumer's JSON view.
// RFC requirement: RFC9552-5.3.1.1-1 negative -- setting only the undefined node flag bits sets no defined O, T, E, B, R or V bit and does not suppress the following Node Name.
func TestRFC9552UndefinedNodeBitsDoNotSetDefinedFlags(t *testing.T) {
	got := AttrTLVsToJSON(flagAttrWithTrailer(TLVNodeFlagBits, 0x03))
	flags, ok := got["node-flags"].(map[string]any)
	require.True(t, ok)
	delete(flags, jsonKeyReserved) // The CLI may display the raw reserved bits.
	assert.Equal(t, map[string]any{
		"node-flags": map[string]any{"O": 0, "T": 0, "E": 0, "B": 0, "R": 0, "V": 0},
		"node-name":  "core1",
	}, got)
}

// TestRFC9552UndefinedIGPBitsDoNotSetDefinedFlags checks that reserved-only
// flags cannot become a routing flag in the consumer's JSON view.
// RFC requirement: RFC9552-5.3.3.1-1 negative -- setting only the undefined IGP flag bits sets no defined D, N, L or P bit and does not suppress the following Node Name.
func TestRFC9552UndefinedIGPBitsDoNotSetDefinedFlags(t *testing.T) {
	got := AttrTLVsToJSON(flagAttrWithTrailer(TLVIGPFlags, 0x0f))
	flags, ok := got["igp-flags"].(map[string]any)
	require.True(t, ok)
	delete(flags, jsonKeyReserved)
	assert.Equal(t, map[string]any{
		"igp-flags": map[string]any{"D": 0, "N": 0, "L": 0, "P": 0},
		"node-name": "core1",
	}, got)
}

// TestRFC9552MPLSProtocolMaskDecode checks both defined protocols through the
// consumer registry rather than a direct call to the new decoder.
// RFC requirement: RFC9552-5.3.2.2-3 positive -- LDP and RSVP-TE flags decode when the reserved bits are zero.
func TestRFC9552MPLSProtocolMaskDecode(t *testing.T) {
	attr := buildAttrTLV(nil, 1094, []byte{0xc0})
	assert.Equal(t, map[string]any{
		"mpls-protocol-mask": map[string]any{"L": 1, "R": 1},
	}, AttrTLVsToJSON(attr))
}

// TestRFC9552MPLSProtocolMaskReservedIgnored checks that reserved-only bits
// neither report an enabled MPLS protocol nor stop the next TLV.
// RFC requirement: RFC9552-5.3.2.2-3 negative -- nonzero reserved bits set no defined MPLS protocol flag and do not suppress the following Node Name.
func TestRFC9552MPLSProtocolMaskReservedIgnored(t *testing.T) {
	attr := buildAttrTLV(nil, 1094, []byte{0x3f})
	attr = buildAttrTLV(attr, TLVNodeName, []byte("core1"))
	assert.Equal(t, map[string]any{
		"mpls-protocol-mask": map[string]any{"L": 0, "R": 0},
		"node-name":          "core1",
	}, AttrTLVsToJSON(attr))
}
