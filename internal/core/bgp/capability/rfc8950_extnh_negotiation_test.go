// Design: docs/architecture/wire/capabilities.md -- Extended Next Hop Encoding negotiation
// RFC: rfc/short/rfc8950.md -- peer support decided per NLRI AFI/SAFI pair (Section 4)
// RFC: rfc/short/rfc5549.md -- the same obligation in the document RFC 8950 obsoletes (Section 4)
//
// RFC 8950 Section 4: "A BGP speaker that wishes to advertise an IPv6 next hop
// for IPv4 NLRI or for VPN-IPv4 NLRI to a BGP peer as per this specification
// MUST use the Capability Advertisement procedures defined in [RFC5492] with
// the Extended Next Hop Encoding capability to determine whether its peer
// supports this for the NLRI AFI/SAFI pair(s) of interest." These tests read
// the per-pair answer Negotiate gives, both through Negotiated and through the
// EncodingCaps the UPDATE encoders consult.
//
// VALIDATES: an IPv6 next hop is allowed for exactly the pairs both Extended
// Next Hop capabilities list with the same Next Hop AFI.
// PREVENTS: one pair in the peer's capability enabling every pair.

package capability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	extNHIPv4Unicast = Family{AFI: AFIIPv4, SAFI: SAFIUnicast}
	extNHIPv4VPN     = Family{AFI: AFIIPv4, SAFI: SAFIVPN}
)

// extNHOpen answers an OPEN's capabilities advertising both IPv4 pairs as
// Multiprotocol, plus an Extended Next Hop capability listing the given tuples
// (none when tuples is nil).
func extNHOpen(tuples []ExtendedNextHopFamily) []Capability {
	caps := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIVPN},
	}
	if tuples != nil {
		caps = append(caps, &ExtendedNextHop{Families: tuples})
	}
	return caps
}

// extNHAnswer answers the negotiated next-hop AFI for f, requiring the
// Negotiated view and the EncodingCaps view to agree.
func extNHAnswer(t *testing.T, neg *Negotiated, f Family) AFI {
	t.Helper()
	afi := neg.ExtendedNextHopAFI(f)
	require.Equal(t, afi, neg.Encoding.ExtendedNextHopAFI(f), "Negotiated and EncodingCaps disagree for %s", f)
	return afi
}

var bothIPv4PairsOverIPv6 = []ExtendedNextHopFamily{
	{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
	{NLRIAFI: AFIIPv4, NLRISAFI: SAFIVPN, NextHopAFI: AFIIPv6},
}

// TestRFC8950PeerSupportDecidedPerPair: both OPENs list IPv4 unicast and
// VPN-IPv4 with Next Hop AFI 2, so both pairs may carry an IPv6 next hop.
//
// RFC requirement: RFC8950-4-2 positive -- when both Extended Next Hop capabilities list <1,1,2> and <1,128,2>, Negotiate answers Next Hop AFI 2 for IPv4 unicast and for VPN-IPv4, through Negotiated and EncodingCaps alike.
// RFC requirement: RFC5549-4-2 positive -- when both Extended Next Hop capabilities list <1,1,2> and <1,128,2>, Negotiate answers Next Hop AFI 2 for IPv4 unicast and for VPN-IPv4, through Negotiated and EncodingCaps alike.
func TestRFC8950PeerSupportDecidedPerPair(t *testing.T) {
	t.Parallel()
	neg := Negotiate(extNHOpen(bothIPv4PairsOverIPv6), extNHOpen(bothIPv4PairsOverIPv6), PeerIdentity{LocalASN: 65001, PeerASN: 65002})
	require.Equal(t, AFIIPv6, extNHAnswer(t, neg, extNHIPv4Unicast))
	require.Equal(t, AFIIPv6, extNHAnswer(t, neg, extNHIPv4VPN))
}

// TestRFC8950PeerWithoutThePairGetsNoIPv6NextHop: Ze advertises both pairs,
// and the peer's capability decides. A peer listing only IPv4 unicast, a peer
// listing VPN-IPv4 with the wrong Next Hop AFI, and a peer sending no Extended
// Next Hop capability each leave the uncovered pair with no IPv6 next hop.
//
// RFC requirement: RFC8950-4-2 negative -- Ze advertising <1,1,2> and <1,128,2>: a peer listing only <1,1,2> leaves VPN-IPv4 at 0 while IPv4 unicast is 2; a peer listing <1,128,1> leaves VPN-IPv4 at 0; a peer with no Extended Next Hop capability leaves both pairs at 0.
func TestRFC8950PeerWithoutThePairGetsNoIPv6NextHop(t *testing.T) {
	t.Parallel()
	local := extNHOpen(bothIPv4PairsOverIPv6)
	identity := PeerIdentity{LocalASN: 65001, PeerASN: 65002}

	neg := Negotiate(local, extNHOpen(bothIPv4PairsOverIPv6[:1]), identity)
	require.Equal(t, AFIIPv6, extNHAnswer(t, neg, extNHIPv4Unicast), "the pair the peer listed")
	require.Equal(t, AFI(0), extNHAnswer(t, neg, extNHIPv4VPN), "the pair the peer did not list")

	neg = Negotiate(local, extNHOpen([]ExtendedNextHopFamily{
		{NLRIAFI: AFIIPv4, NLRISAFI: SAFIVPN, NextHopAFI: AFIIPv4},
	}), identity)
	require.Equal(t, AFI(0), extNHAnswer(t, neg, extNHIPv4VPN), "the peer listed another Next Hop AFI")
	require.Equal(t, AFI(0), extNHAnswer(t, neg, extNHIPv4Unicast), "the peer did not list IPv4 unicast")

	neg = Negotiate(local, extNHOpen(nil), identity)
	require.Equal(t, AFI(0), extNHAnswer(t, neg, extNHIPv4Unicast), "no capability from the peer")
	require.Equal(t, AFI(0), extNHAnswer(t, neg, extNHIPv4VPN), "no capability from the peer")
}
