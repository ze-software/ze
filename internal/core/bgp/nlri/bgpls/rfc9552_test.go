// Design: docs/architecture/wire/nlri-bgpls.md — RFC 9552 NLRI framing tests
// RFC: rfc/short/rfc9552.md
// Related: types.go — parseBGPLS, the NLRI parser under test

package bgpls

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unknownAttrTLVType is a TLV type code with no registered decoder. A copy of
// the BGP-LS plugin's rfc7752_test.go constant: a core test cannot import the
// plugin's test files, and both packages' tests use it.
const unknownAttrTLVType uint16 = 0x7FFF

// buildAttrTLV appends a TLV (type, length, value) to dst. A copy of the
// BGP-LS plugin's rfc7752_test.go helper, for the same reason.
func buildAttrTLV(dst []byte, tlvType uint16, value []byte) []byte {
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint16(hdr[0:], tlvType)
	binary.BigEndian.PutUint16(hdr[2:], uint16(len(value)))
	dst = append(dst, hdr...)
	return append(dst, value...)
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

// TestRFC9552NLRIContentsNeverMakeItMalformed proves parseBGPLS (types.go:295)
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

	parsed, err := parseBGPLS(wire)
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
	bareParsed, err := parseBGPLS(bare)
	require.NoError(t, err, "an NLRI with no Local Node Descriptors TLV is not malformed")
	assert.Equal(t, BGPLSNodeNLRI, bareParsed.NLRIType())
}

// TestRFC9552NLRIFramingErrorsRejected is the counter-case: the syntactic floor
// RFC 9552 Section 8.2.2 keeps. A Total NLRI Length that runs past the buffer,
// a body too short to hold the Protocol-ID and Identifier, and a sub-TLV length
// that overruns the descriptor are all refused.
//
// VALIDATES: parseBGPLS length checks (types.go:297, :306, :311) and
// parseNodeDescriptorTLVs (types.go:396) reject framing errors.
// PREVENTS: "no semantic validation" being read as "no validation".
func TestRFC9552NLRIFramingErrorsRejected(t *testing.T) {
	// RFC requirement: RFC9552-8.2.2-1 negative -- framing errors (Total NLRI Length past the buffer, body shorter than Protocol-ID plus Identifier, sub-TLV length overrun) ARE malformed (§8.2.2)
	// RFC requirement: RFC9552-8.2.2-3 negative -- ze still validates syntax, so tolerating content is not tolerating everything (§8.2.2)
	overrun := []byte{0x00, 0x01, 0x00, 0x40, 0x03, 0, 0, 0, 0, 0, 0, 0, 0}
	_, err := parseBGPLS(overrun)
	assert.ErrorIs(t, err, ErrBGPLSTruncated, "Total NLRI Length past the buffer is malformed")

	short := []byte{0x00, 0x01, 0x00, 0x04, 0x03, 0, 0, 0}
	_, err = parseBGPLS(short)
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

	_, err = parseBGPLS(badSub)
	assert.ErrorIs(t, err, ErrBGPLSTruncated, "a sub-TLV length past the descriptor is malformed")
}
