// Design: docs/architecture/wire/nlri.md -- Extended Next Hop Encoding (RFC 8950)
// Related: peer_test.go -- the IPv4/unicast canUseNextHopFor cases

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC8950IPv6NextHopFollowsTheNegotiatedPair proves that an IPv6 next hop for IPv4
// and VPN-IPv4 NLRI is licensed per AFI/SAFI pair, on the resolve step every
// origination rail uses for next-hop self.
//
// RFC 8950 Section 4: "A BGP speaker MUST only advertise the IPv4 or VPN-IPv4 NLRI with
// an IPv6 next hop to a BGP peer if the BGP speaker has first ascertained via the BGP
// Capability Advertisement that the BGP peer supports the Extended Next Hop Encoding
// capability for the relevant AFI/SAFI pair."
//
// Method: a session whose local transport address is IPv6 resolves next-hop self for
// IPv4/unicast (1/1) and VPN-IPv4 (1/128) under three send contexts: Extended Next Hop
// negotiated for 1/1 only, for 1/128 only, and for neither. Resolution succeeds, with the
// session's IPv6 address, only for the pair the context carries; every other pair is
// refused with ErrNextHopIncompatible, which each rail treats as "do not advertise".
//
// VALIDATES: the Extended Next Hop license is per AFI/SAFI pair, for VPN-IPv4 as well as IPv4.
// PREVENTS: an IPv6 next hop for VPN-IPv4 riding on an IPv4/unicast-only negotiation.
//
// RFC requirement: RFC8950-4-1 positive -- with Extended Next Hop negotiated for exactly one of IPv4/unicast and VPN-IPv4, next-hop self resolves to the session's IPv6 address for that pair's NLRI.
// RFC requirement: RFC8950-4-1 negative -- an IPv6 next hop is refused with ErrNextHopIncompatible for VPN-IPv4 when only IPv4/unicast is negotiated, for IPv4/unicast when only VPN-IPv4 is negotiated, and for both when neither is.
// RFC requirement: RFC5549-4-1 positive -- with Extended Next Hop negotiated for exactly one of IPv4/unicast and VPN-IPv4, next-hop self resolves to the session's IPv6 address for that pair's NLRI.
// RFC requirement: RFC5549-4-1 negative -- an IPv6 next hop is refused with ErrNextHopIncompatible for VPN-IPv4 when only IPv4/unicast is negotiated, for IPv4/unicast when only VPN-IPv4 is negotiated, and for both when neither is.
// RFC requirement: RFC5549-4-4 positive -- with Extended Next Hop negotiated for exactly one of IPv4/unicast and VPN-IPv4, next-hop self resolves to the session's IPv6 address for that pair's NLRI.
// RFC requirement: RFC5549-4-4 negative -- an IPv6 next hop is refused with ErrNextHopIncompatible for VPN-IPv4 when only IPv4/unicast is negotiated, for IPv4/unicast when only VPN-IPv4 is negotiated, and for both when neither is.
func TestRFC8950IPv6NextHopFollowsTheNegotiatedPair(t *testing.T) {
	local := netip.MustParseAddr("2001:db8::1")
	ipv4Unicast := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	ipv4VPN := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	unicastPair := capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}
	vpnPair := capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIVPN}

	for _, tc := range []struct {
		name       string
		negotiated []capability.Family
		licensed   map[family.Family]bool
	}{
		{"unicast only", []capability.Family{unicastPair}, map[family.Family]bool{ipv4Unicast: true, ipv4VPN: false}},
		{"vpn only", []capability.Family{vpnPair}, map[family.Family]bool{ipv4Unicast: false, ipv4VPN: true}},
		{"neither", nil, map[family.Family]bool{ipv4Unicast: false, ipv4VPN: false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("2001:db8::2"), 65000, 65001, 0x01010101)
			peer := NewPeer(settings)
			extNH := map[capability.Family]capability.AFI{}
			for _, pair := range tc.negotiated {
				extNH[pair] = capability.AFIIPv6
			}
			peer.sendCtx.Store(bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
				ExtendedNextHop: extNH,
			}, bgpctx.DirectionSend))

			session := NewSession(settings)
			session.transport.Store(&sessionTransport{local: local})

			for fam, licensed := range tc.licensed {
				addr, err := peer.resolveNextHop(session, bgptypes.NewNextHopSelf(), fam)
				if licensed {
					require.NoError(t, err, "%s must accept the IPv6 next hop", fam)
					require.Equal(t, local, addr)
					continue
				}
				require.ErrorIs(t, err, ErrNextHopIncompatible, "%s must refuse the IPv6 next hop", fam)
				require.False(t, addr.IsValid(), "a refused next hop must carry no address")
			}
		})
	}
}
