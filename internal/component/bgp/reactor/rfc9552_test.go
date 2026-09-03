// RFC: rfc/short/rfc9552.md — Section 8.2.2 fault management for BGP-LS
// Overview: session_validation.go — enforceRFC7606 applies the verdict on the receive path
// Related: ../message/rfc7606_bgpls.go — validateBGPLSAttr, the Section 8.2.2 syntactic walk
//
// RFC 9552 §8.2.2 makes the BGP-LS Attribute's syntax a session-path obligation: a BGP-LS
// Speaker MUST check that the TLV lengths sum to the attribute length and that each TLV
// length is valid, and MUST handle a malformed attribute it can skip as 'Attribute Discard'.
// These tests therefore drive enforceRFC7606, the function a received UPDATE reaches
// (session_read.go), rather than the validator alone: a validator nothing routes to is the
// defect this file exists to keep fixed.

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"

	// The BGP-LS Attribute is RECOGNIZED because the ls plugin registers it:
	// attribute.RegisterName(29, "BGP_LS") in its init (plugins/nlri/ls/register.go).
	// Recognition is what stops publishBase stripping it under RFC 4271 Section 5,
	// which requires an unrecognized non-transitive attribute to be dropped rather
	// than propagated. A shipped ze links the plugin through the generated
	// composition root (plugin/all/all_ze_bgp.go); a reactor test binary links
	// nothing, so without this import the test asks the receive path to keep an
	// attribute that, in THAT binary, ze correctly holds no meaning for.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
)

// bgplsAttrCode is the BGP-LS Attribute, RFC 9552 §5.3 ("assigned value 29 by IANA").
const bgplsAttrCode uint8 = 29

// attrTombstoneCode is the marker that replaces a discarded attribute in place
// (draft-mangin-idr-attr-tombstone-00 §5.1, message.ApplyAttrDiscard).
const attrTombstoneCode uint8 = 252

// rfc9552NodeNLRI is one Link-State Node NLRI (RFC 9552 §5.2): NLRI Type 1, Total NLRI
// Length 21, Protocol-ID 2 (IS-IS Level 2), a zero Identifier, and a Local Node Descriptors
// TLV (256) holding one Autonomous System sub-TLV (512) for AS 65001.
var rfc9552NodeNLRI = []byte{
	0x00, 0x01, 0x00, 0x15, // NLRI Type 1, Total NLRI Length 21
	0x02,                                           // Protocol-ID: IS-IS Level 2
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
	0x01, 0x00, 0x00, 0x08, // TLV 256 Local Node Descriptors, length 8
	0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9, // sub-TLV 512 Autonomous System 65001
}

// rfc9552BGPLSUpdate builds the path attributes of a BGP-LS UPDATE: the two attributes
// RFC 7606 §3.d makes mandatory beside an MP_REACH_NLRI, an MP_REACH_NLRI for
// (AFI 16388, SAFI 71) carrying rfc9552NodeNLRI, and a BGP-LS Attribute holding attrValue.
func rfc9552BGPLSUpdate(attrValue []byte) []byte {
	mpReach := []byte{0x40, 0x04, 0x47, 0x04, 0xc0, 0x00, 0x02, 0x01, 0x00} // AFI 16388, SAFI 71, next-hop 192.0.2.1, Reserved
	mpReach = append(mpReach, rfc9552NodeNLRI...)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
	}
	attrs = append(attrs, 0x80, 14, byte(len(mpReach))) // optional non-transitive MP_REACH_NLRI
	attrs = append(attrs, mpReach...)
	attrs = append(attrs, 0x80, bgplsAttrCode, byte(len(attrValue))) // optional non-transitive, RFC 9552 §5.3
	return append(attrs, attrValue...)
}

// rfc9552Session builds an EBGP session (local AS 65001, peer AS 65002) to receive on.
func rfc9552Session() *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	return NewSession(settings)
}

// TestRFC9552BGPLSAttributeWellFormedIsKept drives a BGP-LS UPDATE whose attribute TLV
// lengths sum exactly to the attribute length.
//
// VALIDATES: RFC 9552 §8.2.2 — the syntactic walk discriminates. A well-formed BGP-LS
// Attribute is not malformed, so no fault-management action fires and the attribute reaches
// the RIB and the propagation path with its bytes untouched.
// PREVENTS: a blanket discard of attribute 29, which would make ze a BGP-LS Propagator that
// silently strips every link-state attribute it relays.
//
// RFC requirement: RFC9552-8.2.2-10 negative -- an attribute whose TLV lengths sum to the attribute length passes the syntactic validation instead of being called malformed.
// RFC requirement: RFC9552-8.2.2-6 negative -- 'Attribute Discard' is reserved for the malformed attribute: a well-formed one survives the receive path byte-identically.
func TestRFC9552BGPLSAttributeWellFormedIsKept(t *testing.T) {
	// TLV 1024 (Node Flag Bits), length 1, then TLV 1026 (Node Name), length 3.
	value := []byte{
		0x04, 0x00, 0x00, 0x01, 0x40,
		0x04, 0x02, 0x00, 0x03, 0x7a, 0x65, 0x31,
	}
	s := rfc9552Session()
	body := makeUpdateBody(nil, rfc9552BGPLSUpdate(value), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action,
		"a BGP-LS Attribute whose TLV lengths sum to its length is not malformed")

	attrs := rfc8669PathAttrs(t, wu.Payload())
	count, kept := countAttrCode(attrs, bgplsAttrCode)
	require.Equal(t, 1, count, "the BGP-LS Attribute must still be on the wire")
	assert.Equal(t, value, kept, "the surviving attribute must carry the bytes the peer sent")
}

// TestRFC9552BGPLSAttributeTLVOverrunDiscarded drives a BGP-LS Attribute whose second TLV
// declares 16 octets of value with 3 present. The attribute's own length is correct, so the
// UPDATE is processable and only the attribute is at fault.
//
// VALIDATES: RFC 9552 §8.2.2 — "the length of each TLV ... are valid" is checked on the
// session path, and the prescribed handling is 'Attribute Discard': the attribute is lost
// whole (a tombstone naming code 29 replaces it) while the rest of the UPDATE, MP_REACH_NLRI
// included, continues to be processed and the session survives.
// PREVENTS: a malformed BGP-LS Attribute being relayed to every BGP-LS Consumer downstream,
// and the opposite failure of tearing the session down for an error the RFC calls skipable.
//
// RFC requirement: RFC9552-8.2.2-10 positive -- a TLV whose declared length exceeds the BGP-LS Attribute length is found malformed by the syntactic validation ze runs on the receive path.
// RFC requirement: RFC9552-8.2.2-6 positive -- that malformed BGP-LS Attribute is handled as 'Attribute Discard': it is removed from the UPDATE and the rest of the UPDATE keeps being processed.
func TestRFC9552BGPLSAttributeTLVOverrunDiscarded(t *testing.T) {
	// TLV 1024 (Node Flag Bits) length 1, then TLV 1026 (Node Name) claiming 16 octets
	// while 3 remain.
	value := []byte{
		0x04, 0x00, 0x00, 0x01, 0x40,
		0x04, 0x02, 0x00, 0x10, 0x7a, 0x65, 0x31,
	}
	s := rfc9552Session()
	body := makeUpdateBody(nil, rfc9552BGPLSUpdate(value), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "a skipable BGP-LS Attribute error is never a session reset")
	assert.Equal(t, message.RFC7606ActionAttributeDiscard, action)

	attrs := rfc8669PathAttrs(t, wu.Payload())
	count, _ := countAttrCode(attrs, bgplsAttrCode)
	assert.Equal(t, 0, count, "the malformed BGP-LS Attribute must be gone from the UPDATE")

	markers, marker := countAttrCode(attrs, attrTombstoneCode)
	require.Equal(t, 1, markers, "the discard must leave one ATTR_TOMBSTONE behind")
	require.GreaterOrEqual(t, len(marker), 2)
	assert.Equal(t, bgplsAttrCode, marker[0], "the tombstone must name the BGP-LS Attribute")
	assert.Equal(t, message.DiscardReasonInvalidLength, marker[1])

	mpReach, _ := countAttrCode(attrs, 14)
	assert.Equal(t, 1, mpReach,
		"the rest of the UPDATE continues to be processed: the Link-State NLRI is still carried")
}

// TestRFC9552BGPLSAttributeTrailingOctetsDiscarded drives a BGP-LS Attribute that ends two
// octets into a TLV header, so the TLV lengths inside it sum to less than the attribute
// length.
//
// VALIDATES: RFC 9552 §8.2.2 first bullet — "the sum of all TLV lengths found in the BGP-LS
// Attribute corresponds to the BGP-LS Attribute length". A tail too short to hold a TLV
// header is the other way that sum can fail, and it is caught on the session path.
// PREVENTS: a walk that only bounds-checks a TLV it can read, and so accepts padding or a
// truncated trailing TLV that a downstream Consumer would parse differently.
//
// RFC requirement: RFC9552-8.2.2-10 positive -- an attribute whose TLV lengths do not sum to the attribute length is found malformed on the receive path.
// RFC requirement: RFC9552-8.2.2-6 positive -- the same 'Attribute Discard' handling applies, with the session and the rest of the UPDATE untouched.
func TestRFC9552BGPLSAttributeTrailingOctetsDiscarded(t *testing.T) {
	// TLV 1024 (Node Flag Bits) length 1, then two octets: half a TLV header.
	value := []byte{
		0x04, 0x00, 0x00, 0x01, 0x40,
		0x04, 0x02,
	}
	s := rfc9552Session()
	body := makeUpdateBody(nil, rfc9552BGPLSUpdate(value), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "a skipable BGP-LS Attribute error is never a session reset")
	assert.Equal(t, message.RFC7606ActionAttributeDiscard, action)

	attrs := rfc8669PathAttrs(t, wu.Payload())
	count, _ := countAttrCode(attrs, bgplsAttrCode)
	assert.Equal(t, 0, count, "the malformed BGP-LS Attribute must be gone from the UPDATE")

	markers, marker := countAttrCode(attrs, attrTombstoneCode)
	require.Equal(t, 1, markers, "the discard must leave one ATTR_TOMBSTONE behind")
	require.GreaterOrEqual(t, len(marker), 2)
	assert.Equal(t, bgplsAttrCode, marker[0], "the tombstone must name the BGP-LS Attribute")
}
