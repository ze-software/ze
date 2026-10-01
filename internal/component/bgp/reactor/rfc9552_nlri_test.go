// RFC: rfc/short/rfc9552.md -- Section 8.2.2 fault management for BGP-LS
// Overview: session_validation.go -- enforceRFC7606 applies the verdict on the receive path
// Related: ../message/rfc7606_bgpls_nlri.go -- the Section 8.2.2 Link-State NLRI walk
// Related: rfc9552_test.go -- the same section's obligations for the BGP-LS Attribute
//
// RFC 9552 §8.2.2 makes the Link-State NLRI's syntax a session-path obligation: "A BGP-LS
// Speaker MUST perform the following syntactic validation of the Link-State NLRI to
// determine if it is malformed." These tests therefore drive enforceRFC7606, the function a
// received UPDATE reaches (session_read.go), rather than the walk alone: a validator
// nothing routes to is the defect this file exists to keep fixed, and §8.2.2 assigns a
// different action to each class of malformedness, which only the receive path applies.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// lsNodeNLRI is one well-formed Node NLRI (RFC 9552 §5.2, Figure 7): Protocol-ID 2 (IS-IS
// Level 2), a zero Identifier, and a Local Node Descriptors TLV (256) holding one
// Autonomous System sub-TLV (512) for the AS the caller names.
func lsNodeNLRI(autonomousSystem uint16) []byte {
	return lsWireNLRI(1,
		0x02,                                           // Protocol-ID: IS-IS Level 2
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
		0x01, 0x00, 0x00, 0x08, // TLV 256 Local Node Descriptors, length 8
		0x02, 0x00, 0x00, 0x04, // sub-TLV 512 Autonomous System, length 4
		0x00, 0x00, byte(autonomousSystem>>8), byte(autonomousSystem),
	)
}

// TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated drives a Link-State NLRI whose TLV
// lengths sum exactly to its Total NLRI Length while carrying a TLV type and a Node
// Descriptor sub-TLV type ze knows nothing about.
//
// VALIDATES: RFC 9552 §8.2.2 -- the syntactic walk discriminates. §5.1: "The presence of
// unknown or unexpected TLVs MUST NOT result in the NLRI or the BGP-LS Attribute being
// considered malformed." The NLRI is not malformed, so no fault-management action fires and
// it reaches the RIB and the propagation path with its bytes untouched.
// PREVENTS: a walk that reads a TLV type it does not recognize as a reason to discard,
// which would make ze a BGP-LS Propagator that drops every NLRI carrying a TLV registered
// after ze was built. That is the failure §5.1 and §8.2.2 both exist to stop.
//
// RFC requirement: RFC9552-8.2.2-9 negative -- an NLRI whose TLV lengths sum to its Total NLRI Length is found well formed by the syntactic validation, whatever TLV and sub-TLV types it carries.
// RFC requirement: RFC9552-8.2.2-4 negative -- 'NLRI discard' is reserved for the malformed NLRI: a well-formed one survives the receive path byte-identically.
// RFC requirement: RFC9552-8.2.2-5 negative -- the session reset is reserved for the non-skipable error: a well-formed NLRI section leaves the session up.
// RFC requirement: RFC9552-5.1-5 negative -- an NLRI whose TLV types ascend follows the ordering rule, so it is not considered malformed.
// RFC requirement: RFC9552-5.2.1.4-1 negative -- a Node Descriptor carrying two sub-TLV types once each satisfies the uniqueness rule, so it is not considered malformed.
// RFC requirement: RFC9552-5.1-4 positive -- the NLRI half: an NLRI carrying an unknown top-level TLV type (1234) and an unexpected Node Descriptor sub-TLV type (999) is not considered malformed on the receive path (enforceRFC7606 answers no action) and survives byte-identical.
func TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated(t *testing.T) {
	nlri := lsWireNLRI(1,
		0x02,                                           // Protocol-ID: IS-IS Level 2
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
		0x01, 0x00, 0x00, 0x10, // TLV 256 Local Node Descriptors, length 16
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9, // sub-TLV 512 Autonomous System 65001
		0x03, 0xe7, 0x00, 0x04, 0xde, 0xad, 0xbe, 0xef, // sub-TLV 999: unexpected, and not malformed
		0x04, 0xd2, 0x00, 0x03, 0xaa, 0xbb, 0xcc, // TLV 1234: unknown, and not malformed
	)
	s := nlriTypeTestSession()
	body := makeUpdateBody(nil, mpReachAttrs(lsFam, nlri), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "a well-formed Link-State NLRI is never a session reset")
	assert.Equal(t, message.RFC7606ActionNone, action,
		"an NLRI whose TLV lengths sum to its Total NLRI Length is not malformed")

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found, "the MP_REACH attribute must survive")
	assert.Equal(t, nlri, got, "the NLRI must carry the bytes the peer sent")
}

// TestRFC9552LinkStateNLRITLVOverrunIsDiscarded drives a Node NLRI whose Local Node
// Descriptors TLV declares 200 octets of value with 8 present. The NLRI's own Total NLRI
// Length is correct, so ze knows where the NLRI ends and can skip it.
//
// VALIDATES: RFC 9552 §8.2.2 -- "The length of the TLVs and, when the TLV is recognized
// then, the length of its sub-TLVs in the NLRI are valid" is checked on the session path,
// and the prescribed handling is the one the same section gives for an error it can skip:
// "it MUST handle such malformed NLRIs as 'NLRI discard'". The malformed NLRI is removed,
// the well-formed one beside it survives, and the session lives.
// PREVENTS: a malformed Link-State NLRI reaching every BGP-LS Consumer downstream, and the
// opposite failure of tearing down a session over an error the RFC calls skipable.
//
// RFC requirement: RFC9552-8.2.2-9 positive -- a TLV whose declared length exceeds the NLRI that carries it is found malformed by the syntactic validation ze runs on the receive path.
// RFC requirement: RFC9552-8.2.2-4 positive -- that malformed NLRI is handled as 'NLRI discard': it alone is removed and the rest of the UPDATE keeps being processed.
func TestRFC9552LinkStateNLRITLVOverrunIsDiscarded(t *testing.T) {
	malformed := lsWireNLRI(1,
		0x02,                                           // Protocol-ID: IS-IS Level 2
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
		0x01, 0x00, 0x00, 0xc8, // TLV 256 claiming 200 octets of value
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9, // 8 octets are all that follow
	)
	survivor := lsNodeNLRI(65002)

	s := nlriTypeTestSession()
	nlri := append(append([]byte{}, malformed...), survivor...)
	body := makeUpdateBody(nil, mpReachAttrs(lsFam, nlri), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "a skipable Link-State NLRI error is never a session reset")
	assert.Equal(t, message.RFC7606ActionNone, action,
		"§8.2.2 discards the NLRI; it is not an attribute discard or a treat-as-withdraw")

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found, "the well-formed NLRI survives, so the attribute must remain")
	assert.Equal(t, survivor, got, "only the malformed NLRI may be removed")
}

// TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded drives a Node NLRI whose
// Local Node Descriptors TLV carries the Autonomous System sub-TLV twice. Every length in
// it frames correctly, so nothing but the uniqueness rule can condemn it.
//
// VALIDATES: RFC 9552 §8.2.2's seventh bullet -- "For NLRIs carrying either a Local or
// Remote Node Descriptor TLV, there is not more than one instance of a sub-TLV present",
// which §5.2.1.4 states as "At most, there MUST be one instance of each sub-TLV type
// present in any Node Descriptor". The NLRI is discarded and the session lives.
// PREVENTS: two BGP-LS Producers deriving different keys for one node, which is what
// §5.2.1.4's uniqueness and ordering rules exist to stop. It also proves the walk is not
// merely a length check with a longer comment.
//
// RFC requirement: RFC9552-8.2.2-9 positive -- a Node Descriptor carrying one sub-TLV type twice is found malformed although every length in the NLRI frames correctly.
// RFC requirement: RFC9552-5.2.1.4-1 positive -- the uniqueness rule is enforced on the receive path, so a second instance of a sub-TLV type is not accepted.
// RFC requirement: RFC9552-8.2.2-4 positive -- the error is skipable, so the prescribed 'NLRI discard' removes that NLRI alone.
func TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded(t *testing.T) {
	malformed := lsWireNLRI(1,
		0x02,                                           // Protocol-ID: IS-IS Level 2
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
		0x01, 0x00, 0x00, 0x10, // TLV 256 Local Node Descriptors, length 16
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9, // sub-TLV 512 Autonomous System 65001
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xea, // sub-TLV 512 again: forbidden
	)
	survivor := lsNodeNLRI(65002)

	s := nlriTypeTestSession()
	nlri := append(append([]byte{}, malformed...), survivor...)
	body := makeUpdateBody(nil, mpReachAttrs(lsFam, nlri), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "a skipable Link-State NLRI error is never a session reset")
	assert.Equal(t, message.RFC7606ActionNone, action)

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found, "the well-formed NLRI survives, so the attribute must remain")
	assert.Equal(t, survivor, got,
		"the NLRI whose Node Descriptor repeats a sub-TLV type must be the one removed")
}

// TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded drives a Link NLRI carrying its Remote
// Node Descriptors TLV (257) before its Local one (256).
//
// VALIDATES: RFC 9552 §8.2.2 -- "The rule regarding the ordering of TLVs has been followed
// as described in Section 5.1", and §5.1 makes the consequence explicit: "NLRIs having TLVs
// that do not follow the above ordering rules MUST be considered as malformed by a BGP-LS
// Propagator." §8.2.2 names this case as its own example of an error ze can skip, so the
// action is the discard rather than the session reset.
// PREVENTS: two byte-different encodings of one link-state object propagating as two
// objects, which §5.1's canonical ordering exists to prevent.
//
// RFC requirement: RFC9552-8.2.2-9 positive -- an NLRI whose TLVs descend by type is found malformed by the ordering rule §8.2.2 sends the walk to §5.1 for.
// RFC requirement: RFC9552-5.1-5 positive -- ze is the BGP-LS Propagator §5.1 addresses, and it considers an NLRI with descending TLV types malformed.
func TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded(t *testing.T) {
	malformed := lsWireNLRI(2,
		0x02,                                           // Protocol-ID: IS-IS Level 2
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
		0x01, 0x01, 0x00, 0x08, // TLV 257 Remote Node Descriptors, out of order
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9,
		0x01, 0x00, 0x00, 0x08, // TLV 256 Local Node Descriptors, after its successor
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9,
	)
	survivor := lsNodeNLRI(65002)

	s := nlriTypeTestSession()
	nlri := append(append([]byte{}, malformed...), survivor...)
	body := makeUpdateBody(nil, mpReachAttrs(lsFam, nlri), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "the TLV ordering rule is §8.2.2's own example of a skipable error")
	assert.Equal(t, message.RFC7606ActionNone, action)

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found, "the well-formed NLRI survives, so the attribute must remain")
	assert.Equal(t, survivor, got, "the NLRI with descending TLV types must be the one removed")
}

// TestRFC9552LinkStateNLRILengthOverrunResetsSession drives a Link-State NLRI whose Total
// NLRI Length declares 255 octets with 21 present, so the NLRI lengths do not sum to the
// MP_REACH_NLRI length and no boundary after it can be located.
//
// VALIDATES: RFC 9552 §8.2.2's first bullet -- "The sum of all TLV lengths found in the BGP
// MP_REACH_NLRI attribute corresponds to the BGP MP_REACH_NLRI length" -- and the action the
// same section gives when the error "results in the inability to process the BGP UPDATE
// message": ze implements no 'AFI/SAFI disable', so its "Alternately, the router MUST
// perform a 'session reset'" applies. RFC 7606 §3(j) reaches the same verdict, because
// treat-as-withdraw needs the NLRI field parsed.
// PREVENTS: guessing where the next NLRI starts and rewriting the wire from that guess, and
// the milder failure of relaying an NLRI section ze could not parse.
//
// RFC requirement: RFC9552-8.2.2-9 positive -- an NLRI whose Total NLRI Length overruns the MP_REACH_NLRI attribute is found malformed, and takes the action §8.2.2 prescribes when the UPDATE cannot be processed.
// RFC requirement: RFC9552-8.2.2-5 positive -- the error is non-skipable and ze implements no 'AFI/SAFI disable', so the session reset §8.2.2 makes the alternative is what fires.
func TestRFC9552LinkStateNLRILengthOverrunResetsSession(t *testing.T) {
	nlri := lsNodeNLRI(65001)
	require.Len(t, nlri, 25)
	nlri[2], nlri[3] = 0x00, 0xff // Total NLRI Length 255, with 21 octets of body present

	s := nlriTypeTestSession()
	body := makeUpdateBody(nil, mpReachAttrs(lsFam, nlri), nil)

	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	assert.Equal(t, message.RFC7606ActionSessionReset, action,
		"a Link-State NLRI length that overruns its attribute is not skipable")
	require.Error(t, err, "the session reset is reported to the caller as an error")
}

// TestRFC9552LinkStateUnreachLengthOverrunResetsSession drives §8.2.2's second bullet, the
// MP_UNREACH_NLRI twin of the length-sum check above.
//
// VALIDATES: a withdrawn Link-State NLRI whose Total NLRI Length declares 255 octets with
// 21 present makes the NLRI lengths disagree with the MP_UNREACH_NLRI length, and the
// receive path answers session reset, reported as an error.
// PREVENTS: the withdraw side escaping the check the announce side has.
//
// RFC requirement: RFC9552-8.2.2-9 positive -- the MP_UNREACH_NLRI bullet of the syntactic validation: a Link-State NLRI in MP_UNREACH_NLRI whose Total NLRI Length overruns the attribute is found malformed on the receive path, which answers session reset.
func TestRFC9552LinkStateUnreachLengthOverrunResetsSession(t *testing.T) {
	nlri := lsNodeNLRI(65001)
	nlri[3] = 0xff // Total NLRI Length 255; 21 octets follow
	s := nlriTypeTestSession()
	body := makeUpdateBody(nil, mpUnreachAttrs(lsFam, nlri), nil)

	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))

	assert.Equal(t, message.RFC7606ActionSessionReset, action,
		"a withdrawn Link-State NLRI length that overruns MP_UNREACH_NLRI is not skipable")
	require.Error(t, err, "the session reset is reported to the caller as an error")
}

// TestRFC9552LinkStateRecognizedTLVSubTLVOverrunIsDiscarded drives §8.2.2's sub-TLV length
// bullet: inside a recognized TLV (256 Local Node Descriptors, itself correctly framed), the
// Autonomous System sub-TLV declares 8 octets with 4 present.
//
// VALIDATES: the NLRI is malformed and discarded; the well-formed NLRI beside it survives.
// PREVENTS: a walk that frames top-level TLVs only.
//
// RFC requirement: RFC9552-8.2.2-9 positive -- "when the TLV is recognized then, the length of its sub-TLVs": a sub-TLV whose declared length overruns the recognized TLV 256 carrying it is found malformed on the receive path, which discards that NLRI and keeps the survivor.
func TestRFC9552LinkStateRecognizedTLVSubTLVOverrunIsDiscarded(t *testing.T) {
	malformed := lsWireNLRI(1,
		0x02,                                           // Protocol-ID: IS-IS Level 2
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
		0x01, 0x00, 0x00, 0x08, // TLV 256 Local Node Descriptors, length 8: frames correctly
		0x02, 0x00, 0x00, 0x08, 0x00, 0x00, 0xfd, 0xe9, // sub-TLV 512 claiming 8, 4 present
	)

	got := lsReceive(t, malformed)

	assert.Equal(t, lsNodeNLRI(65002), got, "only the NLRI whose sub-TLV overruns is removed")
}

// lsNodeNLRIWithTrailing is a Node NLRI (Protocol-ID 2, zero Identifier, Local Node
// Descriptors holding AS 65001) followed by the top-level TLVs the caller frames.
func lsNodeNLRIWithTrailing(trailing ...byte) []byte {
	body := []byte{
		0x02,                                           // Protocol-ID: IS-IS Level 2
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Identifier
		0x01, 0x00, 0x00, 0x08, // TLV 256 Local Node Descriptors, length 8
		0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9, // sub-TLV 512 Autonomous System 65001
	}
	return lsWireNLRI(1, append(body, trailing...)...)
}

// lsReceive runs one MP_REACH carrying nlri and a well-formed survivor through
// enforceRFC7606 and returns the NLRI bytes the attribute still carries.
func lsReceive(t *testing.T, nlri []byte) []byte {
	t.Helper()
	survivor := lsNodeNLRI(65002)
	s := nlriTypeTestSession()
	body := makeUpdateBody(nil, mpReachAttrs(lsFam, append(append([]byte{}, nlri...), survivor...)), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "no case here is a session reset")
	assert.Equal(t, message.RFC7606ActionNone, action, "a Link-State NLRI fault is an NLRI discard")
	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found, "the survivor keeps the attribute alive")
	return got
}

// TestRFC9552LinkStateNLRIUnknownTLVOverrunIsDiscarded is the other side of the unknown-TLV
// tolerance: an unknown top-level TLV type whose declared length runs past its NLRI.
//
// VALIDATES: the unknown type does not shelter broken framing: the NLRI is discarded and
// the well-formed NLRI beside it survives.
// PREVENTS: a walk that skips the length check for a type it does not recognize, which
// would pass the positive unit above.
//
// RFC requirement: RFC9552-5.1-4 negative -- the NLRI half: tolerance covers the TYPE only; an unknown top-level TLV (1234) declaring 200 octets with 3 present makes its NLRI malformed on the receive path, which discards it while the well-formed NLRI beside it survives.
func TestRFC9552LinkStateNLRIUnknownTLVOverrunIsDiscarded(t *testing.T) {
	malformed := lsNodeNLRIWithTrailing(0x04, 0xd2, 0x00, 0xc8, 0xaa, 0xbb, 0xcc) // TLV 1234 claiming 200

	got := lsReceive(t, malformed)

	assert.Equal(t, lsNodeNLRI(65002), got, "only the NLRI whose unknown TLV overruns is removed")
}

// TestRFC9552LinkStateSameTypeTLVsOutOfOrderAreDiscarded drives the same-type half of the
// Section 5.1 ordering rule, which the descending-type unit above does not reach.
//
// VALIDATES: two TLVs of one type (1234) whose Length fields descend, and two of equal
// Length whose Values descend, each make their NLRI malformed: discarded, survivor kept.
// PREVENTS: a walk that compares types only.
//
// RFC requirement: RFC9552-5.1-5 positive -- the same-type clauses of "the above ordering rules": an NLRI repeating TLV 1234 with Lengths 3 then 2, and one repeating it with equal Lengths and Values bb bb then aa aa, are each considered malformed by ze as Propagator and discarded on the receive path.
func TestRFC9552LinkStateSameTypeTLVsOutOfOrderAreDiscarded(t *testing.T) {
	cases := map[string][]byte{
		"length descends": lsNodeNLRIWithTrailing(
			0x04, 0xd2, 0x00, 0x03, 0xaa, 0xaa, 0xaa,
			0x04, 0xd2, 0x00, 0x02, 0xaa, 0xaa),
		"value descends": lsNodeNLRIWithTrailing(
			0x04, 0xd2, 0x00, 0x02, 0xbb, 0xbb,
			0x04, 0xd2, 0x00, 0x02, 0xaa, 0xaa),
	}
	for name, malformed := range cases {
		got := lsReceive(t, malformed)

		assert.Equal(t, lsNodeNLRI(65002), got, "%s: the out-of-order NLRI is removed", name)
	}
}

// TestRFC9552LinkStateSameTypeTLVsInOrderArePropagated is the compliant side of the same
// clauses.
//
// VALIDATES: TLV 1234 repeated with ascending Lengths, and with equal Lengths and
// ascending Values, is not malformed: the NLRI survives byte-identical beside the survivor.
// PREVENTS: a walk that refuses every repeated TLV type, which would pass the unit above.
//
// RFC requirement: RFC9552-5.1-5 negative -- an NLRI repeating TLV 1234 with Lengths 2 then 3, and one repeating it with equal Lengths and Values aa aa then bb bb, follows the ordering rules and is not considered malformed: both survive the receive path byte-identical.
func TestRFC9552LinkStateSameTypeTLVsInOrderArePropagated(t *testing.T) {
	cases := map[string][]byte{
		"length ascends": lsNodeNLRIWithTrailing(
			0x04, 0xd2, 0x00, 0x02, 0xaa, 0xaa,
			0x04, 0xd2, 0x00, 0x03, 0xaa, 0xaa, 0xaa),
		"value ascends": lsNodeNLRIWithTrailing(
			0x04, 0xd2, 0x00, 0x02, 0xaa, 0xaa,
			0x04, 0xd2, 0x00, 0x02, 0xbb, 0xbb),
	}
	for name, ordered := range cases {
		got := lsReceive(t, ordered)

		want := append(append([]byte{}, ordered...), lsNodeNLRI(65002)...)
		assert.Equal(t, want, got, "%s: the ordered NLRI survives", name)
	}
}
