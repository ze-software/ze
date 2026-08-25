// RFC: rfc/short/rfc9552.md — BGP-LS obligations gated by make ze-rfc-check
//
// RFC 9552 obsoletes RFC 7752 with the same wire format and a stricter split
// between syntactic and semantic validation: a Propagator validates syntax
// only, and neither an NLRI nor the BGP-LS Attribute may be called malformed
// because of which TLVs it carries or what those TLVs say. The tests here pin
// that boundary on ze's decode side, plus the one BGP-LS encoder whose value
// RFC 9552 constrains bit-for-bit (the 1-octet IS-IS small metric).
//
// Requirements shared verbatim with RFC 7752 (unknown TLV preservation, family
// registration, capability negotiation, node descriptor sub-TLV ordering, TE
// Default Metric padding, RFC 4760 next-hop) carry an RFC9552 tag alongside the
// RFC7752 tag on the existing test rather than a duplicate test here.

package ls

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// descendingAttr builds a BGP-LS Attribute whose TLVs run in DESCENDING type
// order and mix contexts on purpose: a link attribute TLV (1088 Administrative
// Group) sits beside node attribute TLVs (1026 Node Name, 1024 Node Flag Bits),
// and an unknown type is thrown in. Every one of those is "unordered",
// "unexpected" or "unknown" under RFC 9552 Section 5.1, and none of them may
// make the attribute malformed.
func descendingAttr() []byte {
	var attr []byte
	attr = buildAttrTLV(attr, unknownAttrTLVType, []byte{0x01, 0x02}) // 0x7FFF, highest
	attr = buildAttrTLV(attr, TLVAdminGroup, []byte{0, 0, 0, 7})      // 1088, link TLV
	attr = buildAttrTLV(attr, TLVNodeName, []byte("core1"))           // 1026
	attr = buildAttrTLV(attr, TLVNodeFlagBits, []byte{0xFF})          // 1024, all bits set
	return attr
}

// TestRFC9552UnorderedUnexpectedAttributeTLVs proves the BGP-LS Attribute walk
// carries no ordering rule and no context rule: IterateAttrTLVs (attr.go:77)
// advances on the length field alone, and DecodeAttrTLV (attr.go:107) dispatches
// on the type code with no knowledge of the NLRI the attribute rides with.
//
// RFC 9552 relaxed RFC 7752 here: attribute TLV ordering is a SHOULD, and an
// unordered attribute MUST NOT be treated as malformed.
//
// VALIDATES: descending, cross-context and unknown TLVs all decode.
// PREVENTS: an ordering or context check being added to the attribute walk and
// silently dropping a valid peer's topology.
func TestRFC9552UnorderedUnexpectedAttributeTLVs(t *testing.T) {
	// RFC requirement: RFC9552-5.1-6 positive -- a BGP-LS Attribute whose TLVs descend by type decodes completely instead of being treated as malformed (§5.1)
	// RFC requirement: RFC9552-5.1-4 positive -- an unknown TLV type and a link-attribute TLV in a node-attribute set neither error nor stop the walk (§5.1)
	// RFC requirement: RFC9552-8.2.2-2 positive -- the attribute is not malformed on the strength of which TLVs it includes or the values they carry (§8.2.2)
	// RFC requirement: RFC9552-8.2.2-3 positive -- no TLV value is range-checked or cross-checked against the NLRI type, so nothing semantic is validated (§8.2.2)
	attr := descendingAttr()

	var types []uint16
	err := iterateAttrTLVs(attr, func(e attrTLVEntry) bool {
		types = append(types, e.Type)
		return true
	})
	require.NoError(t, err, "descending TLV order is not an error")
	assert.Equal(t, []uint16{unknownAttrTLVType, TLVAdminGroup, TLVNodeName, TLVNodeFlagBits}, types,
		"every TLV is yielded in wire order, descending included")

	tlvs, err := decodeAllAttrTLVs(attr)
	require.NoError(t, err, "an unordered, cross-context, partly unknown attribute decodes")
	require.Len(t, tlvs, 3, "the three recognized TLVs are typed; the unknown one is skipped")

	js := AttrTLVsToJSON(attr)
	assert.Equal(t, []string{"0x0102"}, js[genericTLVKey(unknownAttrTLVType)],
		"the unknown TLV keeps its bytes")
	assert.Equal(t, "core1", js["node-name"], "a TLV sitting after the unknown one is still reached")

	// Node Flag Bits 0xFF sets every reserved bit RFC 9552 leaves undefined.
	// A semantic check would reject it; ze decodes it.
	flags, err := decodeAttrTLV(attrTLVEntry{Type: TLVNodeFlagBits, Value: []byte{0xFF}})
	require.NoError(t, err, "reserved flag bits set is not a decode error")
	assert.Equal(t, TLVNodeFlagBits, flags.Code())
}

// TestRFC9552AttributeSyntaxStillRejected is the counter-case. RFC 9552 forbids
// calling an attribute malformed for its CONTENT, and requires the syntactic
// checks to stay: a TLV length that overruns the attribute, or a fixed-length
// TLV at the wrong size, is still refused.
//
// VALIDATES: IterateAttrTLVs returns ErrBGPLSTruncated on a length overrun and
// the fixed-length decoders reject a wrong-sized value.
// PREVENTS: "unordered and unexpected are fine" degenerating into "everything
// is fine", which would let a length-overrun TLV through.
func TestRFC9552AttributeSyntaxStillRejected(t *testing.T) {
	// RFC requirement: RFC9552-5.1-6 negative -- ordering leniency does not extend to syntax: a descending sequence ending in a length overrun is refused (§5.1)
	// RFC requirement: RFC9552-5.1-4 negative -- an unknown TLV type is tolerated only while its declared length fits the attribute (§5.1)
	// RFC requirement: RFC9552-8.2.2-2 negative -- a TLV whose declared length is inconsistent with the attribute length IS malformed (§8.2.2)
	// RFC requirement: RFC9552-8.2.2-3 negative -- syntactic validation is still performed, so the absence of semantic validation is not a blanket accept (§8.2.2)
	overrun := append(descendingAttr(), 0x7F, 0xFE, 0x00, 0x10, 0xAA) // claims 16 octets, 1 present

	err := iterateAttrTLVs(overrun, func(attrTLVEntry) bool { return true })
	assert.ErrorIs(t, err, ErrBGPLSTruncated, "a TLV length past the attribute end is refused")

	_, err = decodeAllAttrTLVs(overrun)
	assert.ErrorIs(t, err, ErrBGPLSTruncated)

	// Fixed-length TLV 1088 (Administrative Group) is 4 octets; 3 is malformed.
	_, err = decodeAttrTLV(attrTLVEntry{Type: TLVAdminGroup, Value: []byte{0, 0, 7}})
	assert.ErrorIs(t, err, ErrBGPLSTruncated, "a fixed-length TLV at the wrong size is refused")
}

// nodeNLRIOddButLegal builds a Node NLRI that is semantically questionable and
// syntactically perfect: a Private Use Protocol-ID (200), an Identifier that no
// well-known registry names, no IGP Router-ID sub-TLV at all (the descriptor a
// Consumer would call mandatory), and a sub-TLV type ze has no decoder for.
func nodeNLRIOddButLegal() []byte {
	var descs []byte
	descs = buildAttrTLV(descs, unknownAttrTLVType, []byte{0xDE, 0xAD, 0xBE, 0xEF})

	var container []byte
	container = buildAttrTLV(container, TLVLocalNodeDesc, descs)

	body := make([]byte, 9, 9+len(container))
	body[0] = 200 // RFC 9552 Section 5.2: Protocol-ID 200-255 is Private Use
	binary.BigEndian.PutUint64(body[1:], 0xFFFFFFFFFFFFFFFF)
	body = append(body, container...)

	nlri := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint16(nlri[0:], uint16(BGPLSNodeNLRI))
	binary.BigEndian.PutUint16(nlri[2:], uint16(len(body)))
	return append(nlri, body...)
}

// TestRFC9552NLRIContentsNeverMakeItMalformed proves ParseBGPLS (types.go:295)
// judges an NLRI on framing alone. It reads the type, the total length, the
// Protocol-ID and the Identifier, hands the rest to parseNodeDescriptorTLVs
// (types.go:391) -- which switches on known sub-TLV types and ignores the rest
// -- and caches the wire bytes so WriteTo (types_nlri.go:63) replays them.
//
// That is exactly the Propagator behavior RFC 9552 Section 8.2.2 demands:
// mandatory-TLV presence, value ranges and per-NLRI-type TLV applicability are
// Consumer concerns, and ze checks none of them.
//
// VALIDATES: a Private Use Protocol-ID, a missing IGP Router-ID and an unknown
// sub-TLV all parse, and the NLRI propagates byte-identically.
// PREVENTS: a semantic guard being added to the parser and dropping topology a
// downstream Consumer would have understood.
func TestRFC9552NLRIContentsNeverMakeItMalformed(t *testing.T) {
	// RFC requirement: RFC9552-8.2.2-1 positive -- a Link-State NLRI missing the descriptors a Consumer treats as mandatory, carrying an unknown sub-TLV and a Private Use Protocol-ID, is not malformed (§8.2.2)
	// RFC requirement: RFC9552-8.2.2-3 positive -- ze performs no semantic validation on the propagation path: the NLRI parses and re-emits byte-identically (§8.2.2)
	wire := nodeNLRIOddButLegal()

	parsed, err := ParseBGPLS(wire)
	require.NoError(t, err, "content, not framing, is a Consumer concern")
	node, ok := parsed.(*BGPLSNode)
	require.True(t, ok)
	assert.Equal(t, BGPLSProtocolID(200), node.ProtocolID(), "a Private Use Protocol-ID is kept")
	assert.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), node.Identifier())
	assert.Empty(t, node.LocalNode.IGPRouterID, "no IGP Router-ID sub-TLV is present, and none is required")

	out := make([]byte, parsed.Len())
	n := parsed.WriteTo(out, 0)
	require.Equal(t, len(wire), n)
	assert.Equal(t, wire, out[:n], "the NLRI propagates unchanged, unknown sub-TLV included")

	// An empty descriptor set is likewise accepted: body is Protocol-ID plus
	// Identifier and nothing else.
	bare := []byte{0x00, 0x01, 0x00, 0x09, 0x03, 0, 0, 0, 0, 0, 0, 0, 0}
	bareParsed, err := ParseBGPLS(bare)
	require.NoError(t, err, "an NLRI with no Local Node Descriptors TLV is not malformed")
	assert.Equal(t, BGPLSNodeNLRI, bareParsed.NLRIType())
}

// TestRFC9552NLRIFramingErrorsRejected is the counter-case: the syntactic floor
// RFC 9552 Section 8.2.2 keeps. A Total NLRI Length that runs past the buffer,
// a body too short to hold the Protocol-ID and Identifier, and a sub-TLV length
// that overruns the descriptor are all refused.
//
// VALIDATES: ParseBGPLS length checks (types.go:297, :306, :311) and
// parseNodeDescriptorTLVs (types.go:396) reject framing errors.
// PREVENTS: "no semantic validation" being read as "no validation".
func TestRFC9552NLRIFramingErrorsRejected(t *testing.T) {
	// RFC requirement: RFC9552-8.2.2-1 negative -- framing errors (Total NLRI Length past the buffer, body shorter than Protocol-ID plus Identifier, sub-TLV length overrun) ARE malformed (§8.2.2)
	// RFC requirement: RFC9552-8.2.2-3 negative -- ze still validates syntax, so tolerating content is not tolerating everything (§8.2.2)
	overrun := []byte{0x00, 0x01, 0x00, 0x40, 0x03, 0, 0, 0, 0, 0, 0, 0, 0}
	_, err := ParseBGPLS(overrun)
	assert.ErrorIs(t, err, ErrBGPLSTruncated, "Total NLRI Length past the buffer is malformed")

	short := []byte{0x00, 0x01, 0x00, 0x04, 0x03, 0, 0, 0}
	_, err = ParseBGPLS(short)
	assert.ErrorIs(t, err, ErrBGPLSTruncated, "a body shorter than Protocol-ID plus Identifier is malformed")

	var descs []byte
	descs = append(descs, 0x02, 0x00, 0x00, 0x20, 0x01) // sub-TLV 512 claims 32 octets, 1 present
	var container []byte
	container = buildAttrTLV(container, TLVLocalNodeDesc, descs)
	body := make([]byte, 9, 9+len(container))
	body[0] = byte(ProtoISISL2)
	body = append(body, container...)
	badSub := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint16(badSub[0:], uint16(BGPLSNodeNLRI))
	binary.BigEndian.PutUint16(badSub[2:], uint16(len(body)))
	badSub = append(badSub, body...)

	_, err = ParseBGPLS(badSub)
	assert.ErrorIs(t, err, ErrBGPLSTruncated, "a sub-TLV length past the descriptor is malformed")
}

// TestRFC9552ISISSmallMetricTwoMSBsZero pins the one BGP-LS value RFC 9552
// constrains bit-for-bit in an encoder ze owns: the 1-octet IGP Metric carries
// an IS-IS small metric, whose two most significant bits the originator MUST
// set to zero. writeIGPMetricValue masks with 0x3F for the 1-octet form
// (attr_link.go:286), and decodeIGPMetric masks the same way on receipt
// (attr_link.go:310), which is the "ignored by the receiver" half.
//
// VALIDATES: the 1-octet IGP Metric always leaves bits 7 and 6 clear.
// PREVENTS: a metric whose low bits happen to be small carrying stray high bits
// into the IS-IS small-metric field.
func TestRFC9552ISISSmallMetricTwoMSBsZero(t *testing.T) {
	// RFC requirement: RFC9552-5.3.2.3-2 positive -- the 1-octet IS-IS small metric is emitted with its two most significant bits zero (§5.3.2.3)
	for _, metric := range []uint32{0, 1, 0x2A, 0x3F} {
		tlv := &lsIGPMetric{Metric: metric}
		buf := make([]byte, tlv.Len())
		n := tlv.WriteTo(buf, 0)
		require.Equal(t, 5, n, "4 octet header + 1 octet IS-IS small metric")
		assert.Equal(t, TLVIGPMetric, binary.BigEndian.Uint16(buf[0:]))
		assert.Equal(t, uint16(1), binary.BigEndian.Uint16(buf[2:]))
		assert.Zero(t, buf[4]&0xC0, "the two most significant bits are zero")
		assert.Equal(t, byte(metric), buf[4])
	}

	// A 1-octet metric received with the top bits set round-trips with them
	// cleared, so re-advertisement cannot reintroduce them.
	decoded, err := decodeIGPMetric([]byte{0xFF})
	require.NoError(t, err)
	small, ok := decoded.(*lsIGPMetric)
	require.True(t, ok)
	assert.Equal(t, uint32(0x3F), small.Metric, "the receiver ignores the two high bits")
	buf := make([]byte, decoded.Len())
	written := decoded.WriteTo(buf, 0)
	require.Equal(t, 5, written)
	assert.Zero(t, buf[4]&0xC0, "the re-encoded small metric still has the two high bits zero")
}

// TestRFC9552IGPMetricWidthGrowsInsteadOfTruncating is the counter-case: the
// two-MSB rule is honored by choosing a wider encoding, never by discarding
// value bits. igpMetricValueLen (attr_link.go:265) moves to 2 octets above 0x3F
// and 3 octets above 0xFFFF, and those widths are not IS-IS small metrics, so
// the 0x3F mask does not apply to them.
//
// VALIDATES: metrics above 63 are never emitted in the 1-octet form.
// PREVENTS: a masking encoder silently turning metric 64 into metric 0.
func TestRFC9552IGPMetricWidthGrowsInsteadOfTruncating(t *testing.T) {
	// RFC requirement: RFC9552-5.3.2.3-2 negative -- a metric wider than 6 bits is never emitted in the 1-octet IS-IS small-metric form, so the two-MSB rule never truncates a value (§5.3.2.3)
	for _, tc := range []struct {
		metric   uint32
		valueLen int
	}{
		{0x40, 2}, {0xFF, 2}, {0xFFFF, 2}, {0x10000, 3}, {0xFFFFFF, 3},
	} {
		tlv := &lsIGPMetric{Metric: tc.metric}
		buf := make([]byte, tlv.Len())
		n := tlv.WriteTo(buf, 0)
		require.Equal(t, 4+tc.valueLen, n, "metric %#x uses a %d octet value", tc.metric, tc.valueLen)
		assert.Equal(t, uint16(tc.valueLen), binary.BigEndian.Uint16(buf[2:]))

		var got uint32
		for _, b := range buf[4:n] {
			got = got<<8 | uint32(b)
		}
		assert.Equal(t, tc.metric, got, "no value bits are lost to the small-metric mask")
	}
}

// isisTestMTID is the topology the R-bit test carries. Any value inside the
// low 12 bits works; 14 is one an IS-IS deployment would plausibly use.
const isisTestMTID uint16 = 0x00E

// isisMTIDDescriptorNLRI builds a Link-State NLRI of nlriType carrying a
// Multi-Topology Identifier TLV (263) followed by trailer. The MT-ID is written
// with its four leading R bits taken from rBits, so the same topology value can
// be sent with the reserved bits clear and with all four set.
func isisMTIDDescriptorNLRI(nlriType BGPLSNLRIType, rBits uint16, trailer []byte) []byte {
	value := make([]byte, 2)
	binary.BigEndian.PutUint16(value, rBits<<12|isisTestMTID)

	var descs []byte
	descs = buildAttrTLV(descs, TLVMultiTopologyID, value)
	descs = append(descs, trailer...)

	body := make([]byte, 9, 9+len(descs))
	body[0] = byte(ProtoISISL2) // an IS-IS Protocol-ID: the polarity this clause governs
	body = append(body, descs...)

	nlri := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint16(nlri[0:], uint16(nlriType))
	binary.BigEndian.PutUint16(nlri[2:], uint16(len(body)))
	return append(nlri, body...)
}

// TestRFC9552ISISMTIDReservedBitsIgnoredOnReceipt proves the R bits of an IS-IS
// MT-ID carry no meaning for a receiver. Both descriptor walks mask the MT-ID
// field to its low 12 bits (plugin.go:405 for a Prefix Descriptor, plugin.go:519
// for a Link Descriptor), so a peer that leaves the reserved bits set yields the
// same topology as one that clears them.
//
// "Ignored" is the load-bearing word: the walk does not stop either. The TLV
// after the MT-ID still decodes, which is what separates ignoring the bits from
// refusing the NLRI over them.
//
// VALIDATES: R bits set and R bits clear give one MT-ID, and the descriptor
// after the MT-ID TLV survives both.
// PREVENTS: a range check on the MT-ID field being added to the receive path and
// dropping an IS-IS topology a conforming peer sent.
func TestRFC9552ISISMTIDReservedBitsIgnoredOnReceipt(t *testing.T) {
	// RFC requirement: RFC9552-5.2.2.1-1 positive -- the reserved Bits R of an IS-IS Link or Prefix Descriptor MT-ID are ignored on receipt: all four set decodes to the same MT-ID as all four clear (§5.2.2.1)
	var linkTrailer []byte
	linkTrailer = buildAttrTLV(linkTrailer, TLVIPv4InterfaceAddr, []byte{192, 0, 2, 1})

	_, _, linkClear := parseBGPLSLinkTLVs(isisMTIDDescriptorNLRI(BGPLSLinkNLRI, 0x0, linkTrailer))
	_, _, linkSet := parseBGPLSLinkTLVs(isisMTIDDescriptorNLRI(BGPLSLinkNLRI, 0xF, linkTrailer))

	require.Equal(t, []any{int(isisTestMTID)}, linkClear.mtIDs, "reserved bits clear: the MT-ID is the value sent")
	assert.Equal(t, linkClear.mtIDs, linkSet.mtIDs, "reserved bits set: the four R bits carry no topology meaning")
	assert.Equal(t, []any{"192.0.2.1"}, linkSet.ifAddrs, "the descriptor after the MT-ID TLV still decodes")

	var prefixTrailer []byte
	prefixTrailer = buildAttrTLV(prefixTrailer, TLVIPReachabilityInfo, []byte{24, 192, 0, 2})

	_, prefixClear := parseBGPLSPrefixTLVs(isisMTIDDescriptorNLRI(BGPLSPrefixV4NLRI, 0x0, prefixTrailer), BGPLSPrefixV4NLRI)
	_, prefixSet := parseBGPLSPrefixTLVs(isisMTIDDescriptorNLRI(BGPLSPrefixV4NLRI, 0xF, prefixTrailer), BGPLSPrefixV4NLRI)

	require.Equal(t, []any{int(isisTestMTID)}, prefixClear.mtIDs, "reserved bits clear: the MT-ID is the value sent")
	assert.Equal(t, prefixClear.mtIDs, prefixSet.mtIDs, "reserved bits set: the four R bits carry no topology meaning")
	assert.Equal(t, "192.0.2.0/24", prefixSet.prefix, "the descriptor after the MT-ID TLV still decodes")
}
