// Design: docs/architecture/wire/nlri-bgpls.md -- BGP-LS receive-path fault management
// RFC: rfc/short/rfc9552.md -- Section 8.2.2, semantic errors are not malformations
// Overview: session_validation.go -- enforceRFC7606 applies the verdict on the receive path
// Related: rfc9552_nlri_test.go -- the Section 8.2.2 syntactic walk of the Link-State NLRI
// Related: rfc9552_test.go -- the Section 8.2.2 syntactic walk of the BGP-LS Attribute
//
// RFC 9552 Section 8.2.2 splits validation in two. A BGP-LS Speaker "MUST perform the
// following syntactic validation", and a Link-State NLRI or BGP-LS Attribute "MUST NOT be
// considered malformed or invalid based on the inclusion/exclusion of TLVs or contents of
// the TLV fields (i.e., semantic errors)". These tests drive enforceRFC7606, the function a
// received UPDATE reaches, with inputs that are wrong only in content, and pair each with a
// twin that differs in one length octet, so the pair shows the receive path judges framing
// and nothing else.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// rfc9552SemanticCase is one input that is wrong only in content, and its twin whose one
// changed length octet breaks the framing.
type rfc9552SemanticCase struct {
	name     string
	semantic []byte
	framing  []byte
}

// rfc9552SemanticNLRIs are three Link-State NLRIs (RFC 9552 Section 5.2) that a semantic
// check would refuse, each with a framing twin.
func rfc9552SemanticNLRIs() []rfc9552SemanticCase {
	return []rfc9552SemanticCase{
		{
			name: "private Protocol-ID and an empty Local Node Descriptors TLV",
			semantic: lsWireNLRI(1,
				0xc8,                                           // Protocol-ID 200: no registered value
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
				0x01, 0x00, 0x00, 0x00, // TLV 256 Local Node Descriptors, length 0: no AS, no IGP Router-ID
			),
			framing: lsWireNLRI(1,
				0xc8,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x04, // TLV 256 declaring 4 octets with none present
			),
		},
		{
			name: "Autonomous System sub-TLV shorter than its fixed length",
			semantic: lsWireNLRI(1,
				0x02,                                           // Protocol-ID: IS-IS Level 2
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
				0x01, 0x00, 0x00, 0x06, // TLV 256 Local Node Descriptors, length 6
				0x02, 0x00, 0x00, 0x02, 0xfd, 0xe9, // sub-TLV 512 Autonomous System, length 2 (Table 3 says 4)
			),
			framing: lsWireNLRI(1,
				0x02,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x06,
				0x02, 0x00, 0x00, 0x04, 0xfd, 0xe9, // sub-TLV 512 declaring 4 octets with 2 present
			),
		},
		{
			name: "Link NLRI with no Remote Node Descriptors TLV",
			semantic: lsWireNLRI(2,
				0x02,                                           // Protocol-ID: IS-IS Level 2
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
				0x01, 0x00, 0x00, 0x08, // TLV 256 Local Node Descriptors, length 8
				0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9, // sub-TLV 512 Autonomous System 65001
			),
			framing: lsWireNLRI(2,
				0x02,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x28, // TLV 256 declaring 40 octets with 8 present
				0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9,
			),
		},
	}
}

// TestRFC9552LinkStateNLRISemanticErrorsAreNotMalformed drives each semantically wrong
// Link-State NLRI through the receive path beside a well-formed one.
//
// VALIDATES: RFC 9552 Section 8.2.2 -- a missing descriptor, an unregistered Protocol-ID
// and a fixed-length sub-TLV of the wrong length are semantic errors, so enforceRFC7606
// takes no action and the NLRI survives byte-identical.
// PREVENTS: a receive path that discards a Link-State NLRI for its content, which would make
// ze a Propagator that silently drops what a newer or different Producer sends.
//
// RFC requirement: RFC9552-8.2.2-1 positive -- a Link-State NLRI with an unregistered Protocol-ID and an empty Local Node Descriptors TLV, one whose Autonomous System sub-TLV is 2 octets long, and a Link NLRI with no Remote Node Descriptors TLV are each kept byte-identical by the receive path, beside a well-formed NLRI.
// RFC requirement: RFC9552-8.2.2-3 positive -- the NLRI half: the receive path, which relays what it keeps, refuses none of those three semantically wrong NLRIs.
func TestRFC9552LinkStateNLRISemanticErrorsAreNotMalformed(t *testing.T) {
	survivor := lsNodeNLRI(65002)
	for _, c := range rfc9552SemanticNLRIs() {
		t.Run(c.name, func(t *testing.T) {
			got := lsReceive(t, c.semantic)
			assert.Equal(t, append(append([]byte{}, c.semantic...), survivor...), got,
				"a semantic error must not make the NLRI malformed")
		})
	}
}

// TestRFC9552LinkStateNLRIFramingTwinsAreDiscarded drives the framing twin of each NLRI in
// the unit above: one length octet changed so a TLV or sub-TLV runs past its container.
//
// VALIDATES: the walk that tolerates the content still judges the framing, so each twin is
// discarded and the well-formed NLRI beside it survives.
// PREVENTS: a receive path that passes the unit above by validating nothing at all.
//
// RFC requirement: RFC9552-8.2.2-1 negative -- each of the three NLRIs with one TLV or sub-TLV length declared past its container is discarded by the receive path, while the well-formed NLRI beside it survives.
// RFC requirement: RFC9552-8.2.2-3 negative -- the NLRI half: the receive path still performs the syntactic validation, so those three framing twins are discarded.
func TestRFC9552LinkStateNLRIFramingTwinsAreDiscarded(t *testing.T) {
	survivor := lsNodeNLRI(65002)
	for _, c := range rfc9552SemanticNLRIs() {
		t.Run(c.name, func(t *testing.T) {
			got := lsReceive(t, c.framing)
			assert.Equal(t, survivor, got, "a framing error must discard that NLRI alone")
		})
	}
}

// rfc9552SemanticAttributes are BGP-LS Attribute values (RFC 9552 Section 5.3) that a
// semantic check would refuse, each with a framing twin. The UPDATE carries a Node NLRI, so
// a Link Attribute TLV is out of its context as well.
func rfc9552SemanticAttributes() []rfc9552SemanticCase {
	return []rfc9552SemanticCase{
		{
			name: "TLVs in descending type order",
			semantic: []byte{
				0x04, 0x02, 0x00, 0x03, 0x7a, 0x65, 0x31, // TLV 1026 Node Name "ze1"
				0x04, 0x00, 0x00, 0x01, 0x40, // TLV 1024 Node Flag Bits
			},
			framing: []byte{
				0x04, 0x02, 0x00, 0x03, 0x7a, 0x65, 0x31,
				0x04, 0x00, 0x00, 0x10, 0x40, // TLV 1024 declaring 16 octets with 1 present
			},
		},
		{
			name: "one TLV type twice",
			semantic: []byte{
				0x04, 0x00, 0x00, 0x01, 0x40, // TLV 1024 Node Flag Bits
				0x04, 0x00, 0x00, 0x01, 0x80, // TLV 1024 again
			},
			framing: []byte{
				0x04, 0x00, 0x00, 0x01, 0x40,
				0x04, 0x00, 0x00, 0x10, 0x80,
			},
		},
		{
			name: "fixed-length Link Attribute TLV of the wrong length beside a Node NLRI",
			semantic: []byte{
				0x04, 0x40, 0x00, 0x03, 0xaa, 0xbb, 0xcc, // TLV 1088 Administrative Group, 3 octets (fixed length 4)
			},
			framing: []byte{
				0x04, 0x40, 0x00, 0x10, 0xaa, 0xbb, 0xcc,
			},
		},
		{
			name: "unregistered TLV type and reserved flag bits set",
			semantic: []byte{
				0x04, 0x00, 0x00, 0x01, 0xff, // TLV 1024 Node Flag Bits with the reserved bits set
				0xfd, 0xe8, 0x00, 0x02, 0x12, 0x34, // TLV 65000: no registered meaning
			},
			framing: []byte{
				0x04, 0x00, 0x00, 0x01, 0xff,
				0xfd, 0xe8, 0x00, 0x10, 0x12, 0x34,
			},
		},
	}
}

// TestRFC9552BGPLSAttributeSemanticErrorsAreNotMalformed drives each semantically wrong
// BGP-LS Attribute through the receive path.
//
// VALIDATES: RFC 9552 Section 8.2.2 -- TLV order, a repeated TLV, a TLV out of its NLRI's
// context, a fixed-length TLV of the wrong length, an unregistered type and reserved flag
// bits are semantic errors, so enforceRFC7606 takes no action and the attribute survives
// byte-identical.
// PREVENTS: an 'Attribute Discard' taken for content, which strips every link-state
// attribute a Producer encodes differently from ze's decoders.
//
// RFC requirement: RFC9552-8.2.2-2 positive -- a BGP-LS Attribute with TLVs in descending order, with one TLV type twice, with a 3-octet Administrative Group beside a Node NLRI, or with an unregistered TLV type and reserved Node Flag bits is kept byte-identical by the receive path.
// RFC requirement: RFC9552-8.2.2-3 positive -- the attribute half: the receive path, which relays what it keeps, refuses none of those four semantically wrong attributes.
func TestRFC9552BGPLSAttributeSemanticErrorsAreNotMalformed(t *testing.T) {
	for _, c := range rfc9552SemanticAttributes() {
		t.Run(c.name, func(t *testing.T) {
			s := rfc9552Session()
			body := makeUpdateBody(nil, rfc9552BGPLSUpdate(c.semantic), nil)

			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err)
			assert.Equal(t, message.RFC7606ActionNone, action,
				"a semantic error must not make the BGP-LS Attribute malformed")

			count, kept := countAttrCode(rfc8669PathAttrs(t, wu.Payload()), bgplsAttrCode)
			require.Equal(t, 1, count, "the BGP-LS Attribute must still be on the wire")
			assert.Equal(t, c.semantic, kept, "the attribute must carry the bytes the peer sent")
		})
	}
}

// TestRFC9552BGPLSAttributeFramingTwinsAreDiscarded drives the framing twin of each
// attribute in the unit above: its last TLV declares 16 octets with fewer present.
//
// VALIDATES: the walk that tolerates the content still judges the framing, so each twin is
// handled as 'Attribute Discard'.
// PREVENTS: a receive path that passes the unit above by validating nothing at all.
//
// RFC requirement: RFC9552-8.2.2-2 negative -- each of the four attributes with its last TLV length declared past the attribute is removed from the UPDATE by the receive path ('Attribute Discard').
// RFC requirement: RFC9552-8.2.2-3 negative -- the attribute half: the receive path still performs the syntactic validation, so those four framing twins are discarded.
func TestRFC9552BGPLSAttributeFramingTwinsAreDiscarded(t *testing.T) {
	for _, c := range rfc9552SemanticAttributes() {
		t.Run(c.name, func(t *testing.T) {
			s := rfc9552Session()
			body := makeUpdateBody(nil, rfc9552BGPLSUpdate(c.framing), nil)

			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err, "a skipable BGP-LS Attribute error is never a session reset")
			assert.Equal(t, message.RFC7606ActionAttributeDiscard, action)

			count, _ := countAttrCode(rfc8669PathAttrs(t, wu.Payload()), bgplsAttrCode)
			assert.Equal(t, 0, count, "the malformed BGP-LS Attribute must be gone from the UPDATE")
		})
	}
}
