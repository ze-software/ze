// RFC: rfc/short/rfc4760.md — Multiprotocol Extensions for BGP-4
// Overview: rfc7606.go — UPDATE validation (the mpReachCount>0 mandatory-attribute branch)
// Detail: update_build.go — BuildUnicast (LOCAL_PREF for iBGP, MP_REACH for non-IPv4-unicast)
//
// RFC 4760 Section 3 obligations that bind an MP_REACH_NLRI UPDATE to the well-known
// attributes it must accompany: ORIGIN and AS_PATH always, LOCAL_PREF additionally in IBGP.
// These drive the mpReachCount>0 code path in ValidateUpdateRFC7606 (rfc7606.go:366-380),
// distinct from the inline-IPv4-NLRI path, and the IBGP LOCAL_PREF emission in BuildUnicast
// (update_build.go:262-268 alongside the MP_REACH at :322). Helpers sOrigin/sASPath/sJoin are
// declared in rfc7606_structural_test.go (same package) and reused here.

package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// mpReachIPv6Attr returns a complete, well-formed MP_REACH_NLRI path attribute (flags 0x80,
// code 14) for 2001:db8::/64 with a 16-byte IPv6 next hop. AFI=2/SAFI=1, NH_Len=16 (valid for
// IPv6 unicast), and the /64 NLRI fits the attribute exactly, so the attribute passes the
// MP_REACH next-hop and NLRI-syntax checks and is counted by mpReachCount.
func mpReachIPv6Attr() []byte {
	value := []byte{
		0x00, 0x02, // AFI = IPv6
		0x01, // SAFI = unicast
		0x10, // Length of Next Hop = 16
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // next hop 2001:db8::1
		0x00,                                               // Reserved
		64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // NLRI 2001:db8::/64
	}
	return append([]byte{attrFlagOptional, attrCodeMPReachNLRI, byte(len(value))}, value...)
}

// attrCodesInBlob walks a packed path-attributes blob using the RFC 4271 Section 4.3
// flags/code/length framing (extended-length bit 0x10 included) and returns the set of
// attribute type codes present.
func attrCodesInBlob(pathAttrs []byte) map[uint8]bool {
	codes := map[uint8]bool{}
	pos := 0
	for pos+2 <= len(pathAttrs) {
		flags := pathAttrs[pos]
		code := pathAttrs[pos+1]
		pos += 2
		var attrLen int
		if flags&0x10 != 0 { // extended length
			if pos+2 > len(pathAttrs) {
				break
			}
			attrLen = int(pathAttrs[pos])<<8 | int(pathAttrs[pos+1])
			pos += 2
		} else {
			if pos+1 > len(pathAttrs) {
				break
			}
			attrLen = int(pathAttrs[pos])
			pos++
		}
		codes[code] = true
		pos += attrLen
	}
	return codes
}

// TestRFC4760MPReachRequiresOriginAndASPath enforces that an UPDATE carrying MP_REACH_NLRI
// also carries ORIGIN and AS_PATH.
//
// VALIDATES: RFC 4760 Section 3 -- "An UPDATE message that carries the MP_REACH_NLRI MUST also
// carry the ORIGIN and the AS_PATH attributes (both in EBGP and in IBGP exchanges)."
//
// PREVENTS: accepting a multiprotocol announcement that omits a mandatory well-known
// attribute, which would install a route with no origin or no AS path.
//
// This drives the mpReachCount>0 branch in ValidateUpdateRFC7606 (rfc7606.go:366-380), reached
// solely because MP_REACH_NLRI is present with no inline IPv4 NLRI (hasNLRI=false). That branch
// is distinct from the inline-IPv4-NLRI branch at rfc7606.go:341-363. Because mpReachCount>0,
// the §5.2 no-reachable-NLRI escalation at rfc7606.go:387 does not fire, so the action stays
// treat-as-withdraw rather than escalating to session reset.
//
// RFC requirement: RFC4760-3-3 positive -- path attributes ORIGIN + AS_PATH + MP_REACH_NLRI are
// accepted with RFC7606ActionNone: both mandatory attributes are present, so the mpReachCount>0
// branch records no missing-attribute error (internal/component/bgp/message/rfc7606.go:366-380).
// RFC requirement: RFC4760-3-3 negative -- an MP_REACH_NLRI UPDATE that omits ORIGIN, or
// separately omits AS_PATH, is treat-as-withdraw with the absent attribute named: the
// mpReachCount>0 branch records a treat-as-withdraw naming ORIGIN/AS_PATH
// (internal/component/bgp/message/rfc7606.go:366-380).
func TestRFC4760MPReachRequiresOriginAndASPath(t *testing.T) {
	t.Parallel()
	mp := mpReachIPv6Attr()

	t.Run("origin and as_path present accepted", func(t *testing.T) {
		t.Parallel()
		pathAttrs := sJoin(sOrigin, sASPath, mp)

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action,
			"MP_REACH_NLRI accompanied by ORIGIN and AS_PATH must be accepted: %s", result.Description)
	})

	t.Run("origin missing withdraws", func(t *testing.T) {
		t.Parallel()
		// AS_PATH present, ORIGIN absent.
		pathAttrs := sJoin(sASPath, mp)

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action,
			"MP_REACH_NLRI without ORIGIN must be treat-as-withdraw, not accepted: %s", result.Description)
		require.Equal(t, attrCodeOrigin, result.AttrCode,
			"the missing-attribute error must name ORIGIN")
		require.Contains(t, result.Description, "ORIGIN",
			"the description must name the missing ORIGIN attribute")
	})

	t.Run("as_path missing withdraws", func(t *testing.T) {
		t.Parallel()
		// ORIGIN present, AS_PATH absent.
		pathAttrs := sJoin(sOrigin, mp)

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action,
			"MP_REACH_NLRI without AS_PATH must be treat-as-withdraw, not accepted: %s", result.Description)
		require.Equal(t, attrCodeASPath, result.AttrCode,
			"the missing-attribute error must name AS_PATH")
		require.Contains(t, result.Description, "AS_PATH",
			"the description must name the missing AS_PATH attribute")
	})
}

// TestRFC4760IBGPMPReachCarriesLocalPref enforces that an IBGP UPDATE carrying MP_REACH_NLRI
// also carries LOCAL_PREF.
//
// VALIDATES: RFC 4760 Section 3 -- "Moreover, in IBGP exchanges such a message MUST also carry
// the LOCAL_PREF attribute."
//
// PREVENTS: emitting an IBGP multiprotocol UPDATE with no LOCAL_PREF, which would leave the
// receiving IBGP peer without the degree-of-preference RFC 4271 route selection requires.
//
// Driven through the UPDATE builder: an IPv6 unicast route always uses MP_REACH_NLRI (code 14)
// rather than inline NLRI (update_build.go:320-323), and LOCAL_PREF (code 5) is appended for
// every IBGP build (update_build.go:262-268). The eBGP build of the identical route is the
// natural negative: the same MP_REACH path with LOCAL_PREF omitted because the session is not
// IBGP.
//
// RFC requirement: RFC4760-3-4 positive -- an IBGP IPv6/MP_REACH UPDATE built by BuildUnicast
// carries LOCAL_PREF (code 5) alongside MP_REACH_NLRI (code 14): the IBGP branch appends
// LOCAL_PREF (internal/component/bgp/message/update_build.go:262-268) and the IPv6 route uses
// MP_REACH (internal/component/bgp/message/update_build.go:320-323).
// RFC requirement: RFC4760-3-4 negative -- the same route built for an eBGP session carries
// MP_REACH_NLRI (code 14) but NOT LOCAL_PREF (code 5): the LOCAL_PREF append is guarded by
// ub.IsIBGP, so a non-IBGP MP_REACH build has no LOCAL_PREF
// (internal/component/bgp/message/update_build.go:262-268).
func TestRFC4760IBGPMPReachCarriesLocalPref(t *testing.T) {
	t.Parallel()
	params := &UnicastParams{
		Prefix:  netip.MustParsePrefix("2001:db8::/64"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
	}

	t.Run("ibgp includes local_pref", func(t *testing.T) {
		t.Parallel()
		ub := NewUpdateBuilder(65001, true /*isIBGP*/, true /*asn4*/, false /*addPath*/)
		upd := ub.BuildUnicast(params)
		codes := attrCodesInBlob(upd.PathAttributes)

		require.True(t, codes[attrCodeMPReachNLRI],
			"an IPv6 route must carry MP_REACH_NLRI (code 14)")
		require.True(t, codes[attrCodeLocalPref],
			"an IBGP UPDATE carrying MP_REACH_NLRI must also carry LOCAL_PREF (code 5)")
	})

	t.Run("ebgp omits local_pref", func(t *testing.T) {
		t.Parallel()
		ub := NewUpdateBuilder(65001, false /*isIBGP*/, true /*asn4*/, false /*addPath*/)
		upd := ub.BuildUnicast(params)
		codes := attrCodesInBlob(upd.PathAttributes)

		require.True(t, codes[attrCodeMPReachNLRI],
			"the eBGP route must still carry MP_REACH_NLRI (code 14)")
		require.False(t, codes[attrCodeLocalPref],
			"LOCAL_PREF (code 5) is IBGP-only, so an eBGP MP_REACH build must omit it")
	})
}

// mpReachBadNextHopAttr returns an MP_REACH_NLRI path attribute for IPv6 unicast whose Length
// of Next Hop Network Address is 8. attribute.ValidNextHopLens admits only 16 and 32 for
// AFI 2 / SAFI 1, so the attribute is incorrect within the meaning of RFC 4760 Section 7 and
// RFC 7606 Section 7.11. The attribute value is 13 octets, past the Section 5.3 minimum of 5,
// so the refusal comes from the next-hop check rather than from the length floor.
func mpReachBadNextHopAttr() []byte {
	value := []byte{
		0x00, 0x02, // AFI = IPv6
		0x01,                                           // SAFI = unicast
		0x08,                                           // Length of Next Hop = 8, valid for no IPv6 unicast encoding
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // 8 next-hop octets
		0x00, // Reserved
	}
	return append([]byte{attrFlagOptional, attrCodeMPReachNLRI, byte(len(value))}, value...)
}

// mpUnreachShortAttr returns an MP_UNREACH_NLRI path attribute whose value is 2 octets. RFC 7606
// Section 5.3 makes an MP_UNREACH_NLRI shorter than 3 incorrect, because the AFI and SAFI do not
// both fit.
func mpUnreachShortAttr() []byte {
	value := []byte{0x00, 0x02} // AFI = IPv6, no room for the SAFI octet
	return append([]byte{attrFlagOptional, attrCodeMPUnreachNLRI, byte(len(value))}, value...)
}

// TestRFC4760IncorrectMPAttributeResetsTheSession enforces the RFC 4760 Section 7 response to an
// incorrect MP_REACH_NLRI or MP_UNREACH_NLRI.
//
// VALIDATES: RFC 4760 Section 7 -- "If a BGP speaker receives from a neighbor an UPDATE message
// that contains the MP_REACH_NLRI or MP_UNREACH_NLRI attribute, and if the speaker determines
// that the attribute is incorrect, the speaker MUST delete all the BGP routes received from that
// neighbor whose AFI/SAFI is the same as the one carried in the incorrect MP_REACH_NLRI or
// MP_UNREACH_NLRI attribute."
//
// RFC 7606 updates that handling. Its Section 2 names the RFC 4760 Section 7 behavior "AFI/SAFI
// disable", and Sections 3(j) and 7.11 require either that approach or "session reset" whenever
// the NLRI cannot be located: "Since the next hop precedes the NLRI field in the attribute, in
// this case it will not be possible to reliably locate the NLRI; thus, the 'session reset' or
// 'AFI/SAFI disable' approach MUST be used." Ze takes the session-reset branch, which is the
// stronger of the two: the session drops, and RIBManager.handleState
// (internal/component/bgp/plugins/rib/rib.go) releases the neighbor's whole Adj-RIB-In on
// peer-down, so every route of the offending AFI/SAFI is deleted along with the rest.
//
// This test owns the trigger half of that chain: an incorrect MP attribute must produce
// RFC7606ActionSessionReset, and a correct one must not. The deletion half is
// TestHandleState_PeerDown in internal/component/bgp/plugins/rib.
//
// PREVENTS: an incorrect MP attribute being downgraded to treat-as-withdraw or accepted, which
// would leave the neighbor's routes for that AFI/SAFI installed after the speaker has already
// decided it cannot parse what the neighbor sent.
//
// RFC requirement: RFC4760-7-1 positive -- an MP_REACH_NLRI with a next-hop length its AFI/SAFI
// does not admit, and an MP_UNREACH_NLRI shorter than three octets, each return
// RFC7606ActionSessionReset, the action that deletes the neighbor's routes for that AFI/SAFI
// (internal/component/bgp/message/rfc7606.go validateMPReachNextHop, validateMPUnreachAttr).
// RFC requirement: RFC4760-7-1 negative -- a well-formed MP_REACH_NLRI carrying ORIGIN and
// AS_PATH returns RFC7606ActionNone, so a correct attribute never triggers the deletion
// (internal/component/bgp/message/rfc7606.go validateMPReachAttr).
func TestRFC4760IncorrectMPAttributeResetsTheSession(t *testing.T) {
	t.Parallel()

	t.Run("incorrect mp_reach next-hop length resets", func(t *testing.T) {
		t.Parallel()
		pathAttrs := sJoin(sOrigin, sASPath, mpReachBadNextHopAttr())

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionSessionReset, result.Action,
			"an MP_REACH_NLRI whose next-hop length is invalid for its AFI/SAFI must reset the session: %s",
			result.Description)
		require.Equal(t, attrCodeMPReachNLRI, result.AttrCode,
			"the refusal must name MP_REACH_NLRI (code 14)")
	})

	t.Run("incorrect mp_unreach length resets", func(t *testing.T) {
		t.Parallel()
		pathAttrs := sJoin(mpUnreachShortAttr())

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionSessionReset, result.Action,
			"an MP_UNREACH_NLRI shorter than three octets must reset the session: %s",
			result.Description)
		require.Equal(t, attrCodeMPUnreachNLRI, result.AttrCode,
			"the refusal must name MP_UNREACH_NLRI (code 15)")
	})

	t.Run("correct mp_reach does not reset", func(t *testing.T) {
		t.Parallel()
		pathAttrs := sJoin(sOrigin, sASPath, mpReachIPv6Attr())

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action,
			"a well-formed MP_REACH_NLRI must not reset the session: %s", result.Description)
	})
}
