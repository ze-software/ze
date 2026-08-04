// RFC: rfc/short/rfc7606.md -- Section 5.4 typed NLRI
// Overview: session_validation_nlritype.go -- the ingress filter these tests drive
//
// RFC 7606 Section 5.4 names three typed address families in its own text: MCAST-VPN
// (RFC 6514), MCAST-VPLS (RFC 7117) and EVPN (RFC 7432). session_validation_nlritype_test.go
// proves the EVPN half. This file proves the other two families ze advertises, so the filter
// is shown to read a route type through each family's own framing rather than through one
// hardcoded offset.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/family"
)

// The two further typed families Section 5.4 binds in this build.
var (
	mvpnFam = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN}
	mupFam  = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMUP}
)

// mupWireNLRI frames one BGP-MUP NLRI under architecture type 1:
// [arch:1][route-type:2][length:1][body] (draft-ietf-bess-mup-safi Section 3.1).
func mupWireNLRI(routeType uint16, body ...byte) []byte {
	out := []byte{1, byte(routeType >> 8), byte(routeType), byte(len(body))}
	return append(out, body...)
}

// registerTypeLenRecognizer installs the ruling the MVPN plugin's init() installs: the route
// type is the first octet after any RFC 7911 path identifier, and lo..hi is the implemented
// range. The reactor package does not import the plugin, so the test states the ruling
// itself, exactly as registerEVPNRecognizer does.
//
// The stand-in is the RULING only. Everything under it is the real thing: the family's
// registered nlrisplit splitter carves the section, and enforceRFC7606 applies the filter.
// Each real recognizer is pinned in its own package (TestRecognizerIsRegisteredForBoth*,
// TestImplementedMatchesRouteTypeNames), so a ruling that drifted would go red there rather
// than here. These tests prove the filter, which is what they claim.
//
// SnapshotForTest, never ResetForTest: this binary links real NLRI plugins through other
// test files, and a teardown that left the registry empty would make a later test find no
// recognizer for a family the daemon really rules on, so its filter would do nothing and
// the test would pass proving nothing.
func registerTypeLenRecognizer(t *testing.T, fam family.Family, lo, hi byte) {
	t.Helper()
	t.Cleanup(nlritype.SnapshotForTest())
	require.NoError(t, nlritype.Register(fam, func(nlriBytes []byte, addPath bool) bool {
		off := 0
		if addPath {
			off = 4
		}
		if off >= len(nlriBytes) {
			return false
		}
		return nlriBytes[off] >= lo && nlriBytes[off] <= hi
	}))
}

// registerMUPRecognizer installs the BGP-MUP ruling the plugin's init() installs:
// draft-ietf-bess-mup-safi Section 3.1 defines route types 1..4 under architecture type 1,
// and the Route Type specific encoding depends on both, so the PAIR names the type. Same
// stand-in reasoning and same snapshot teardown as registerTypeLenRecognizer.
func registerMUPRecognizer(t *testing.T) {
	t.Helper()
	t.Cleanup(nlritype.SnapshotForTest())
	require.NoError(t, nlritype.Register(mupFam, func(nlriBytes []byte, addPath bool) bool {
		off := 0
		if addPath {
			off = 4
		}
		if off+3 > len(nlriBytes) {
			return false
		}
		rt := uint16(nlriBytes[off+1])<<8 | uint16(nlriBytes[off+2])
		return nlriBytes[off] == 1 && rt >= 1 && rt <= 4
	}))
}

// TestRFC7606Section54DiscardsUnrecognizedMVPNType covers the family RFC 7606 Section 5.4
// names FIRST among its examples of typed NLRI.
//
// VALIDATES: RFC 7606 Section 5.4 -- "Certain address families, for example, MCAST-VPN
// [RFC6514], MCAST-VPLS [RFC7117], and EVPN [RFC7432] have NLRI that are typed" -- applied
// to ipv4/mvpn, whose NLRI RFC 6514 Section 4 frames as [route-type:1][length:1][body] and
// whose route types are 1..7.
// PREVENTS: proving the MUST for EVPN alone while MCAST-VPN, the section's own first
// example, still retains and relays a route type ze cannot read.
//
// RFC requirement: RFC7606-5.4-1 positive -- an MCAST-VPN route whose type ze does not implement is discarded at ingress, exactly as an EVPN one is.
func TestRFC7606Section54DiscardsUnrecognizedMVPNType(t *testing.T) {
	// RFC 6514 Section 4 defines route types 1..7, so 8 and above are unrecognized.
	registerTypeLenRecognizer(t, mvpnFam, 1, 7)
	s := nlriTypeTestSession()

	keep := evpnWireNLRI(7, 0xaa) // Source Tree Join; same [type][len][body] envelope
	drop := evpnWireNLRI(8, 0xbb) // no such MCAST-VPN route type
	nlri := append(append([]byte{}, keep...), drop...)

	body := makeUpdateBody(nil, mpReachAttrs(mvpnFam, nlri), nil)
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found, "the implemented route survives, so the attribute must remain")
	assert.Equal(t, keep, got, "the undefined MCAST-VPN route type must be gone")
}

// TestRFC7606Section54DiscardsUnrecognizedMUPType proves the filter reads a route type
// through a DIFFERENT envelope, not only the two-octet one EVPN and MCAST-VPN share.
//
// VALIDATES: RFC 7606 Section 5.4 applied to ipv4/mup, whose NLRI
// draft-ietf-bess-mup-safi Section 3.1 frames as [arch:1][route-type:2][length:1][body], so
// the length octet sits at offset 3 and the type spans two octets.
// PREVENTS: a filter that carves every typed family on EVPN's offsets, which would misread
// every BGP-MUP boundary and then discard or keep routes at random.
//
// RFC requirement: RFC7606-5.4-1 positive -- a BGP-MUP route whose type ze does not implement is discarded at ingress, through the family's own four-octet envelope.
func TestRFC7606Section54DiscardsUnrecognizedMUPType(t *testing.T) {
	registerMUPRecognizer(t)
	s := nlriTypeTestSession()

	keep := mupWireNLRI(4, 0xaa)
	drop := mupWireNLRI(5, 0xbb) // route type 5 is not defined
	nlri := append(append([]byte{}, keep...), drop...)

	body := makeUpdateBody(nil, mpReachAttrs(mupFam, nlri), nil)
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action)

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found)
	assert.Equal(t, keep, got, "the undefined BGP-MUP route type must be gone")
}

// TestRFC7606Section54FiltersTreatAsWithdrawSynthesis closes the one path where the filter
// used to be skipped.
//
// VALIDATES: RFC 7606 Section 5.4 on an UPDATE that ALSO earns treat-as-withdraw. Section 2
// makes treat-as-withdraw keep the routes and withdraw them, and
// message.SynthesizeWithdrawFamilies turns this UPDATE's MP_REACH into an MP_UNREACH
// carrying the SAME NLRI bytes, which processMessage then dispatches to every eligible peer.
// PREVENTS: an unrecognized route type reaching a peer inside a synthesized withdrawal. The
// filter used to skip this action entirely, on the reading that a withdrawn route needs no
// discard; the withdrawal carries the NLRI, so it does.
//
// The malformed attribute here is ORIGIN with a value of 2 octets, which RFC 7606 Section
// 7.1 makes treat-as-withdraw.
//
// RFC requirement: RFC7606-5.4-1 positive -- an unrecognized NLRI type is discarded even when the same UPDATE is treated as a withdrawal, so it is never relayed inside the synthesized MP_UNREACH.
func TestRFC7606Section54FiltersTreatAsWithdrawSynthesis(t *testing.T) {
	registerTypeLenRecognizer(t, evpnFam, 1, 5)
	s := nlriTypeTestSession()

	keep := evpnWireNLRI(2, 0xaa)
	drop := evpnWireNLRI(99, 0xbb)
	nlri := append(append([]byte{}, keep...), drop...)

	attrs := mpReachAttrs(evpnFam, nlri)
	// RFC 7606 Section 7.1: ORIGIN is malformed when its length is not 1.
	attrs = append([]byte{0x40, 0x01, 0x02, 0x00, 0x00}, attrs...)

	body := makeUpdateBody(nil, attrs, nil)
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
		"a two-octet ORIGIN is treat-as-withdraw, which is the path under test")

	bodies := message.SynthesizeWithdrawFamilies(wu.Payload(), acceptEveryFamily)
	require.Len(t, bodies, 1, "the surviving route must still be withdrawn")
	assert.NotContains(t, string(bodies[0]), string(drop),
		"the unrecognized route type must not ride out inside the synthesized withdrawal")
	assert.Contains(t, string(bodies[0]), string(keep),
		"the implemented route must still be withdrawn, so this is not a vacuous absence")
}
