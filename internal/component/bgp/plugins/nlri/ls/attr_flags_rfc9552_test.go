package ls

// RFC 9552 receive-side handling of the undefined bits in the two one-octet flag
// TLVs: Node Flag Bits (TLV 1024, Section 5.3.1.1) and IGP Flags (TLV 1152,
// Section 5.3.3.1). Both sections end with the same sentence: "The bits that are
// not defined MUST be set to 0 by the originator and MUST be ignored by the
// receiver."
//
// ze is a BGP-LS Consumer decoder and Propagator, never a Producer: the plugin
// registers both families with Mode "decode" (plugin.go, familyModeDecode), so
// the originator half of the sentence has no encoder to exercise. The receiver
// half is what these tests pin. The producers are decodeNodeFlagBits
// (attr_node.go) and decodeIGPFlags (attr_prefix.go), reached through
// decodeAllAttrTLVs (attr.go), the walk every received BGP-LS Attribute takes.
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

	clearFlags, ok := clear[0].(*LsIGPFlags)
	require.True(t, ok, "TLV 1152 decodes to LsIGPFlags")
	setFlags, ok := set[0].(*LsIGPFlags)
	require.True(t, ok, "TLV 1152 with undefined bits set decodes to LsIGPFlags")

	assert.Equal(t, definedOnly, clearFlags.Flags, "the octet sent is the octet decoded")
	assert.Equal(t, definedFlagBits(t, clearFlags, "igp-flags"), definedFlagBits(t, setFlags, "igp-flags"),
		"the undefined bits carry no meaning: every defined bit reads the same")

	name, ok := set[1].(*lsNodeName)
	require.True(t, ok, "the TLV after the flags still decodes")
	assert.Equal(t, "core1", name.Name)
}
