// RFC: rfc/short/rfc7606.md -- Section 5.4 typed NLRI; rfc/short/rfc9552.md -- Section 5.2
// Overview: session_validation_nlritype.go -- the ingress filter these tests drive
//
// RFC 7606 Section 5.4 is enforced at ingress, not at the RIB, because the RIB is not the
// propagation gate: reactorForwardRS fans the RECEIVED wire straight to every eligible peer
// and buildFwdBody appends the payload verbatim on the same-context path. So these tests
// drive enforceRFC7606, the one point upstream of both the RIB and the forward rails.

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/family"
)

// evpnFam and lsFam are the two families Section 5.4's ruling divides.
var (
	evpnFam = family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	lsFam   = family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
)

// nlriTypeTestSession builds an EBGP session with no encoding context, so no family
// negotiates RFC 7911 ADD-PATH and the NLRI sections carry no path identifier.
func nlriTypeTestSession() *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	return NewSession(settings)
}

// registerEVPNRecognizer installs the same ruling the EVPN plugin's init() installs:
// route types 1..5 are implemented, everything else is not. The reactor package does not
// import the plugin, so the test states the ruling itself.
//
// SnapshotForTest, never ResetForTest: this binary links real NLRI plugins through other
// test files, and a teardown that left the registry empty would make a later test find no
// recognizer for a family the daemon really rules on, so its Section 5.4 filter would do
// nothing and the test would pass proving nothing.
func registerEVPNRecognizer(t *testing.T) {
	t.Helper()
	t.Cleanup(nlritype.SnapshotForTest())
	err := nlritype.Register(evpnFam, func(nlriBytes []byte, addPath bool) bool {
		off := 0
		if addPath {
			off = 4
		}
		if off >= len(nlriBytes) {
			return false
		}
		return nlriBytes[off] >= 1 && nlriBytes[off] <= 5
	})
	require.NoError(t, err)
}

// evpnWireNLRI frames one EVPN NLRI: [route-type][length][body] (RFC 7432 Section 7.1).
func evpnWireNLRI(routeType byte, body ...byte) []byte {
	return append([]byte{routeType, byte(len(body))}, body...)
}

// lsWireNLRI frames one BGP-LS NLRI: [type:2][total length:2][body] (RFC 9552 Section 5.2).
func lsWireNLRI(nlriType uint16, body ...byte) []byte {
	out := []byte{byte(nlriType >> 8), byte(nlriType), byte(len(body) >> 8), byte(len(body))}
	return append(out, body...)
}

// mpReachAttrs returns the well-known mandatory attributes plus an MP_REACH_NLRI attribute
// carrying nlri for the given family. Next-hop is a single 4-byte address.
func mpReachAttrs(fam family.Family, nlri []byte) []byte {
	value := []byte{
		byte(uint16(fam.AFI) >> 8), byte(uint16(fam.AFI)), byte(fam.SAFI),
		0x04, 0xc0, 0x00, 0x02, 0x01, // next-hop length 4, 192.0.2.1
		0x00, // reserved
	}
	value = append(value, nlri...)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
	}
	attrs = append(attrs, 0x80, byte(attribute.AttrMPReachNLRI), byte(len(value)))
	return append(attrs, value...)
}

// mpReachAttrsExtLen is mpReachAttrs with the MP_REACH attribute carrying the RFC 4271
// Section 4.3 Extended Length flag and a two-octet length, the form a real speaker uses
// once the NLRI section passes 255 octets.
func mpReachAttrsExtLen(fam family.Family, nlri []byte) []byte {
	value := []byte{
		byte(uint16(fam.AFI) >> 8), byte(uint16(fam.AFI)), byte(fam.SAFI),
		0x04, 0xc0, 0x00, 0x02, 0x01, // next-hop length 4, 192.0.2.1
		0x00, // reserved
	}
	value = append(value, nlri...)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
	}
	attrs = append(attrs,
		0x90, byte(attribute.AttrMPReachNLRI), byte(len(value)>>8), byte(len(value)))
	return append(attrs, value...)
}

// extLenMPReachOffset is where mpReachAttrsExtLen puts the MP_REACH header: after the
// four-octet ORIGIN and the three-octet empty AS_PATH.
const extLenMPReachOffset = 7

// mpUnreachAttrs returns an MP_UNREACH_NLRI attribute carrying nlri for the given family.
func mpUnreachAttrs(fam family.Family, nlri []byte) []byte {
	value := []byte{byte(uint16(fam.AFI) >> 8), byte(uint16(fam.AFI)), byte(fam.SAFI)}
	value = append(value, nlri...)

	attrs := []byte{0x80, byte(attribute.AttrMPUnreachNLRI), byte(len(value))}
	return append(attrs, value...)
}

// mpReachNLRIOf slices the NLRI bytes back out of the MP_REACH attribute of an UPDATE body,
// so a test asserts on what the peer would actually receive.
func mpReachNLRIOf(t *testing.T, body []byte) ([]byte, bool) {
	t.Helper()
	attrs := rfc8669PathAttrs(t, body)
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if !found {
		return nil, false
	}
	start, ok := message.MPNLRIStart(uint8(attribute.AttrMPReachNLRI), value)
	require.True(t, ok, "the MP_REACH attribute must still be well formed after the rewrite")
	return value[start:], true
}

// TestRFC7606Section54DiscardsUnrecognizedEVPNType is the proof behind the compliance claim.
//
// VALIDATES: RFC 7606 Section 5.4 -- "A BGP speaker advertising support for such a typed
// address family MUST handle routes with unrecognized NLRI types within that address family
// by discarding them". The unrecognized route type is gone from the UPDATE that ze goes on
// to install and relay; the recognized ones on either side of it survive, in wire order.
// PREVENTS: an EVPN route type ze does not implement being retained as an opaque RIB entry
// and re-advertised verbatim, which is what ze did before this.
//
// RFC requirement: RFC7606-5.4-1 positive -- an EVPN route whose type ze does not implement is discarded at ingress, so it reaches neither the RIB nor the forward path.
func TestRFC7606Section54DiscardsUnrecognizedEVPNType(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	keepA := evpnWireNLRI(2, 0xaa) // MAC/IP Advertisement, implemented
	drop := evpnWireNLRI(99, 0xde) // no such route type in ze
	keepB := evpnWireNLRI(3, 0xbb) // Inclusive Multicast, implemented

	var nlri []byte
	nlri = append(nlri, keepA...)
	nlri = append(nlri, drop...)
	nlri = append(nlri, keepB...)

	body := makeUpdateBody(nil, mpReachAttrs(evpnFam, nlri), nil)
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "an unrecognized NLRI type is discarded, never a session reset")
	assert.Equal(t, message.RFC7606ActionNone, action,
		"Section 5.4 discards a route; it is not an attribute discard or a treat-as-withdraw")

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found, "routes ze does implement survive, so the attribute must remain")

	want := append(append([]byte{}, keepA...), keepB...)
	assert.Equal(t, want, got,
		"the unrecognized type must be gone and the implemented ones must survive in order")
}

// TestRFC7606Section54PropagatesUnknownBGPLSType is the other half of the ruling.
//
// VALIDATES: RFC 9552 Section 5.2 -- "this document deviates from the default handling
// behavior specified by Section 5.4 (paragraph 2) of [RFC7606] for Link-State address
// family. An implementation MUST handle unknown Link-State NLRI types as opaque objects and
// MUST preserve and propagate them."
// PREVENTS: a blanket Section 5.4 discard breaking BGP-LS, where the family's own
// specification requires the opposite. Ze is a propagator on that path, so dropping an
// unknown Link-State type would stop new types reaching consumers behind it.
//
// RFC requirement: RFC7606-5.4-1 negative -- a family whose own specification overrides Section 5.4 keeps propagating its unknown NLRI types unchanged.
func TestRFC7606Section54PropagatesUnknownBGPLSType(t *testing.T) {
	registerEVPNRecognizer(t) // only evpn is ruled on; bgp-ls deliberately is not
	s := nlriTypeTestSession()

	known := lsWireNLRI(1, 0x02, 0x00)    // Node NLRI
	unknown := lsWireNLRI(99, 0xde, 0xad) // a type ze does not parse

	nlri := append(append([]byte{}, known...), unknown...)
	body := makeUpdateBody(nil, mpReachAttrs(lsFam, nlri), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found, "the MP_REACH attribute must survive untouched")
	assert.Equal(t, nlri, got,
		"RFC 9552 Section 5.2: an unknown Link-State NLRI type is preserved and propagated")
}

// VALIDATES: an UPDATE whose every NLRI is unrecognized loses the MP_REACH attribute
// entirely AND is dropped rather than relayed, because what is left conveys nothing.
// PREVENTS: relaying an UPDATE that announces nothing, and relaying an MP_REACH whose NLRI
// section is empty, which a conforming receiver has no reason to accept.
func TestRFC7606Section54DropsMPReachWhenNothingSurvives(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	nlri := append(evpnWireNLRI(99, 0xaa), evpnWireNLRI(200, 0xbb)...)
	body := makeUpdateBody(nil, mpReachAttrs(evpnFam, nlri), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
		"the UPDATE now conveys nothing, so it rides the drop rail rather than the relay")

	_, found := mpReachNLRIOf(t, wu.Payload())
	assert.False(t, found,
		"no route survives, so the MP_REACH attribute must be removed rather than emptied")
	assert.Empty(t, message.SynthesizeWithdrawFamilies(wu.Payload(), acceptEveryFamily),
		"nothing is left to withdraw, which is what makes processMessage drop the UPDATE")
}

// acceptEveryFamily stands in for Session.mpFamilyDispatchable, so a test can ask the
// same question processMessage asks: does this body still carry anything to withdraw?
func acceptEveryFamily(uint16, uint8) bool { return true }

// TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR pins the shape the
// MP_REACH test cannot reach.
//
// VALIDATES: an MP_UNREACH-only UPDATE (RFC 4760 Section 4 makes it conforming input, and
// RFC 7606 Section 5.1 encourages MP_UNREACH as the first and only attribute) whose every
// withdrawal names an unrecognized type is DROPPED, not relayed.
// PREVENTS: a forged End-of-RIB. With the only attribute gone, RebuildUpdateBody emits
// withdrawn-length 0, attribute-length 0 and no NLRI -- four zero octets, which is exactly
// RFC 4724 Section 2's legacy End-of-RIB marker. Relaying it would make ze-build tell every peer
// that this neighbor finished its initial routing update, ending a restarting peer's RFC
// 4724 route deferral early on a withdrawal the peer never meant as an EoR.
//
// The MP_REACH tests miss this because mpReachAttrs prepends ORIGIN and AS_PATH, so their
// attribute section never empties.
//
// RFC requirement: RFC7606-5.4-1 positive -- an MP_UNREACH-only UPDATE whose withdrawals are all unrecognized types is discarded whole, and never rebuilt into an End-of-RIB marker.
func TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	nlri := append(evpnWireNLRI(6, 0xaa), evpnWireNLRI(99, 0xbb)...)
	body := makeUpdateBody(nil, mpUnreachAttrs(evpnFam, nlri), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "an unrecognized type is discarded, never a session reset")
	assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
		"every route is gone, so the UPDATE must ride the drop rail")

	// This is the hazard, stated as an assertion rather than a comment: the rebuilt body
	// READS as an End-of-RIB. That is why it must not be relayed.
	_, isEOR := wireu.NewWireUpdate(wu.Payload(), 0).IsEOR()
	assert.True(t, isEOR,
		"the rebuilt body is byte-identical to an EoR, which is what makes relaying it a forgery")

	// And this is the drop: processMessage synthesizes withdrawals from this body, gets
	// none, and consumes the UPDATE without dispatching or forwarding it.
	assert.Empty(t, message.SynthesizeWithdrawFamilies(wu.Payload(), acceptEveryFamily),
		"no family is left to withdraw, so processMessage drops the UPDATE and counts no EoR")
}

// VALIDATES: an UPDATE where every route type is implemented is returned byte-identical,
// on the same backing array.
// PREVENTS: paying an allocation and losing the zero-copy relay for every conforming EVPN
// peer, which would make the compliance fix a throughput regression.
func TestRFC7606Section54LeavesConformingUpdateZeroCopy(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	nlri := append(evpnWireNLRI(2, 0xaa), evpnWireNLRI(5, 0xbb)...)
	body := makeUpdateBody(nil, mpReachAttrs(evpnFam, nlri), nil)

	in := wireu.NewWireUpdate(body, 0)
	wu, action, err := s.enforceRFC7606(in)
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)
	assert.Same(t, in, wu, "nothing was discarded, so the received WireUpdate must be reused")
	assert.Equal(t, &body[0], &wu.Payload()[0],
		"nothing was discarded, so the wire bytes must not be copied")
}

// VALIDATES: a withdrawal naming an unrecognized route type is discarded on the same terms
// as an announcement.
// PREVENTS: relaying a withdrawal for a route ze never relayed, and the asymmetry of
// filtering one direction only.
func TestRFC7606Section54DiscardsUnrecognizedEVPNWithdrawal(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	keep := evpnWireNLRI(2, 0xaa)
	drop := evpnWireNLRI(99, 0xbb)
	body := makeUpdateBody(nil, mpUnreachAttrs(evpnFam, append(append([]byte{}, keep...), drop...)), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)

	attrs := rfc8669PathAttrs(t, wu.Payload())
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPUnreachNLRI)
	require.True(t, found, "the recognized withdrawal survives, so the attribute must remain")
	start, ok := message.MPNLRIStart(uint8(attribute.AttrMPUnreachNLRI), value)
	require.True(t, ok)
	assert.Equal(t, keep, value[start:],
		"the withdrawal of an unrecognized type must be dropped with the route it names")
}

// TestRFC7606Section54SessionResetsUnparseableTypedNLRI closes a peer-controlled bypass.
//
// VALIDATES: RFC 7606 Section 5.3 -- an MP attribute is incorrect when "the length of the
// last NLRI found exceeds the amount of unconsumed data remaining in the attribute" -- and
// Section 3(j), which requires session reset when the NLRI field cannot be parsed, because
// treat-as-withdraw needs it parsed.
// PREVENTS: a one-octet bypass of the Section 5.4 MUST. message.validateMPNLRISyntax runs
// the Section 5.3 walk only for IPv4/IPv6 unicast and multicast and returns nil for every
// typed family, so nothing upstream had checked this framing. A peer that appended one
// truncated NLRI made the splitter fail, and the filter used to relay the whole section
// untouched -- unrecognized types included, on demand.
//
// RFC requirement: RFC7606-5.4-1 positive -- a typed NLRI section whose last route overruns the attribute is session-reset per Sections 5.3 and 3(j), so no unrecognized type is relayed by making the split fail.
func TestRFC7606Section54SessionResetsUnparseableTypedNLRI(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	// One implemented route, then a header declaring a 4-octet body with 1 octet left.
	nlri := append(evpnWireNLRI(2, 0xaa), 99, 0x04, 0xde)
	body := makeUpdateBody(nil, mpReachAttrs(evpnFam, nlri), nil)

	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.Error(t, err, "Section 3(j): an NLRI field that cannot be parsed resets the session")
	assert.Equal(t, message.RFC7606ActionSessionReset, action)
}

// VALIDATES: the rewrite preserves an Extended Length attribute header, re-encoding the new
// length in the same two octets rather than switching to the one-octet form.
// PREVENTS: a peer receiving an attribute whose flags say Extended Length while its length
// field is one octet, which every conforming parser reads as a malformed attribute list.
func TestRFC7606Section54RewritesExtendedLengthMPReach(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	keep := evpnWireNLRI(2, 0xaa)
	drop := evpnWireNLRI(99, 0xbb)
	body := makeUpdateBody(nil, mpReachAttrsExtLen(evpnFam, append(append([]byte{}, keep...), drop...)), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)

	attrs := rfc8669PathAttrs(t, wu.Payload())
	off := extLenMPReachOffset
	require.GreaterOrEqual(t, len(attrs), off+4)
	assert.Equal(t, byte(0x90), attrs[off], "the Extended Length flag must survive the rewrite")
	assert.Equal(t, byte(attribute.AttrMPReachNLRI), attrs[off+1])
	gotLen := int(attrs[off+2])<<8 | int(attrs[off+3])
	assert.Equal(t, len(attrs)-(off+4), gotLen,
		"the two-octet length must be re-encoded to the shortened value")

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found)
	assert.Equal(t, keep, got)
}

// VALIDATES: rewriteMPNLRISections copies through every octet of an attribute whose header
// it could not frame, including the flags and type code it had already stepped over.
// PREVENTS: silently truncating two to four octets out of the rebuilt attribute section on
// malformed input. This is asserted on the helper rather than through enforceRFC7606
// because the Section 4 bounds checks reject such a section before the rewrite runs; the
// copy-through is a belt on top of that brace, and an untested belt is a wire corruption
// waiting for the day the brace changes.
func TestRewriteMPNLRISectionsKeepsUnframeableTail(t *testing.T) {
	// A well-formed MP_UNREACH that IS edited, then a two-octet stump: flags and type code
	// with no length octet. The edit has to be applied for real, or the walk would never
	// reach the stump and the test would be identical with no edits at all.
	unreach := []byte{0x80, byte(attribute.AttrMPUnreachNLRI), 0x05, 0x00, 0x19, 0x46, 0x02, 0x00}
	stump := []byte{0x80, byte(attribute.AttrMPReachNLRI)}
	attrs := append(append([]byte{}, unreach...), stump...)
	edits := []mpNLRIEdit{{code: uint8(attribute.AttrMPUnreachNLRI), nlri: []byte{0x03, 0x00}, dropped: 1}}

	got := rewriteMPNLRISections(attrs, edits)

	// The edited attribute is rebuilt with the replacement NLRI and a corrected length.
	want := append([]byte{0x80, byte(attribute.AttrMPUnreachNLRI), 0x05, 0x00, 0x19, 0x46, 0x03, 0x00}, stump...)
	assert.Equal(t, want, got,
		"the edit is applied AND the unframeable tail is copied through whole, header included")
}

// VALIDATES: a family nobody has ruled on is left exactly as received.
// PREVENTS: risk R-1, an over-broad discard reaching a family whose specification was never
// read. The registry's default is the mitigation, so it is asserted on the real path.
func TestRFC7606Section54LeavesUnruledFamilyUntouched(t *testing.T) {
	// Snapshot rather than clear: this leaves the registry empty for the duration, which is
	// exactly the unruled state under test, and puts back what the linked plugins registered.
	t.Cleanup(nlritype.SnapshotForTest())
	s := nlriTypeTestSession()

	nlri := append(evpnWireNLRI(2, 0xaa), evpnWireNLRI(99, 0xbb)...)
	body := makeUpdateBody(nil, mpReachAttrs(evpnFam, nlri), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found)
	assert.Equal(t, nlri, got, "with no ruling registered, nothing may be discarded")
}
