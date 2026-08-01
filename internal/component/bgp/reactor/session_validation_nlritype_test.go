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
func registerEVPNRecognizer(t *testing.T) {
	t.Helper()
	nlritype.ResetForTest()
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
	t.Cleanup(nlritype.ResetForTest)
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
// entirely, so nothing is relayed and no empty attribute goes out.
// PREVENTS: relaying an UPDATE that announces nothing, and relaying an MP_REACH whose NLRI
// section is empty, which a conforming receiver has no reason to accept.
func TestRFC7606Section54DropsMPReachWhenNothingSurvives(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	nlri := append(evpnWireNLRI(99, 0xaa), evpnWireNLRI(200, 0xbb)...)
	body := makeUpdateBody(nil, mpReachAttrs(evpnFam, nlri), nil)

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)

	_, found := mpReachNLRIOf(t, wu.Payload())
	assert.False(t, found,
		"no route survives, so the MP_REACH attribute must be removed rather than emptied")
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

// VALIDATES: a family nobody has ruled on is left exactly as received.
// PREVENTS: risk R-1, an over-broad discard reaching a family whose specification was never
// read. The registry's default is the mitigation, so it is asserted on the real path.
func TestRFC7606Section54LeavesUnruledFamilyUntouched(t *testing.T) {
	nlritype.ResetForTest()
	t.Cleanup(nlritype.ResetForTest)
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
