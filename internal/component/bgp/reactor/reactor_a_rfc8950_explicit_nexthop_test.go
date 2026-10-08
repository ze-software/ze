// Design: docs/architecture/wire/capabilities.md -- RFC 8950 extended next hop
// Related: rfc8950_nexthop_pair_test.go -- the same rule for next-hop self

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestRFC8950ExplicitIPv6NextHopFollowsTheNegotiatedPair drives RFC 8950 Section 4
// for a next hop the operator names, over the announce entry point a plugin or the
// CLI enters (reactorAPIAdapter.AnnounceNLRIBatch): "A BGP speaker MUST only
// advertise the IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a BGP peer if the
// BGP speaker has first ascertained via the BGP Capability Advertisement that the
// BGP peer supports the Extended Next Hop Encoding capability for the relevant
// AFI/SAFI pair."
//
// Method: one Established peer with a live session over a pipe receives an IPv4
// unicast announce with the explicit next hop 2001:db8::1. The negotiated Extended
// Next Hop pairs vary per case. With <1/1, IPv6> negotiated the announce is
// accepted and an UPDATE is sent (its encoding is
// TestRFC8950BatchRailIPv4UnicastIPv6NextHopUsesMPReach). With the pair absent,
// or with only <1/128, IPv6> negotiated, the announce is refused with
// ErrNextHopIncompatible and no UPDATE reaches the wire. The VPN-IPv4 half is
// asserted at Peer.resolveNextHop, which every announce rail calls: an explicit IPv6
// next hop for 1/128 is licensed by <1/128, IPv6> alone.
//
// RFC requirement: RFC8950-4-1 positive -- an explicit IPv6 next hop is accepted for IPv4 unicast (announce sent) only when <1/1, IPv6> is negotiated, and for VPN-IPv4 only when <1/128, IPv6> is negotiated.
// RFC requirement: RFC8950-4-1 negative -- with the relevant pair not negotiated (none, or only the other pair) the explicit IPv6 next hop is refused with ErrNextHopIncompatible and no UPDATE is sent.
// RFC requirement: RFC5549-4-1 positive -- an explicit IPv6 next hop is accepted for IPv4 unicast (announce sent) only when <1/1, IPv6> is negotiated, and for VPN-IPv4 only when <1/128, IPv6> is negotiated.
// RFC requirement: RFC5549-4-1 negative -- with the relevant pair not negotiated (none, or only the other pair) the explicit IPv6 next hop is refused with ErrNextHopIncompatible and no UPDATE is sent.
// RFC requirement: RFC5549-4-4 positive -- an explicit IPv6 next hop is accepted for IPv4 unicast (announce sent) only when <1/1, IPv6> is negotiated, and for VPN-IPv4 only when <1/128, IPv6> is negotiated.
// RFC requirement: RFC5549-4-4 negative -- with the relevant pair not negotiated (none, or only the other pair) the explicit IPv6 next hop is refused with ErrNextHopIncompatible and no UPDATE is sent.
func TestRFC8950ExplicitIPv6NextHopFollowsTheNegotiatedPair(t *testing.T) {
	nextHop := netip.MustParseAddr("2001:db8::1")
	ipv4VPN := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	unicastPair := capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}
	vpnPair := capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIVPN}

	for _, tc := range []struct {
		name       string
		negotiated []capability.Family
		unicast    bool // <1/1, IPv6> licenses the IPv4 unicast announce.
		vpn        bool // <1/128, IPv6> licenses the VPN-IPv4 next hop.
	}{
		{"unicast pair", []capability.Family{unicastPair}, true, false},
		{"vpn pair only", []capability.Family{vpnPair}, false, true},
		{"no pair", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65002, 0x01020301)
			peer := NewPeer(settings)
			_, messages := newEstablishedSessionForPeer(t, peer)
			peer.state.Store(int32(PeerStateEstablished))
			caps := []capability.Capability{
				&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
				&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIVPN},
			}
			var pairs []capability.ExtendedNextHopFamily
			for _, pair := range tc.negotiated {
				pairs = append(pairs, capability.ExtendedNextHopFamily{
					NLRIAFI: pair.AFI, NLRISAFI: pair.SAFI, NextHopAFI: capability.AFIIPv6,
				})
			}
			caps = append(caps, &capability.ExtendedNextHop{Families: pairs})
			neg := capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65002})
			peer.currentSession().negotiated = neg
			peer.negotiated.Store(NewNegotiatedCapabilities(neg))
			peer.sendCtx.Store(bgpctx.NewEncodingContext(neg.Identity, neg.Encoding, bgpctx.DirectionSend))

			addr, err := peer.resolveNextHop(peer.currentSession(), bgptypes.NewNextHopExplicit(nextHop), ipv4VPN)
			if tc.vpn {
				require.NoError(t, err, "<1/128, IPv6> must license the VPN-IPv4 next hop")
				require.Equal(t, nextHop, addr)
			} else {
				require.ErrorIs(t, err, ErrNextHopIncompatible, "VPN-IPv4 must refuse the IPv6 next hop")
				require.False(t, addr.IsValid(), "a refused next hop must carry no address")
			}

			a := &reactorAPIAdapter{r: &Reactor{
				attrModHandlers: attrModHandlersWithDefaults(),
				config:          &Config{LocalAS: 65000},
				peers:           map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
			}}
			batch := bgptypes.NLRIBatch{
				Family:  family.IPv4Unicast,
				NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("203.0.113.0/24"), 0)},
				NextHop: bgptypes.NewNextHopExplicit(nextHop),
			}
			err = a.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())

			if !tc.unicast {
				require.ErrorIs(t, err, ErrNextHopIncompatible, "IPv4 unicast must refuse the IPv6 next hop")
				select {
				case msg := <-messages:
					t.Fatalf("no UPDATE may reach a peer lacking <1/1, IPv6>, got %x", msg)
				case <-time.After(100 * time.Millisecond):
				}
				return
			}
			require.NoError(t, err, "<1/1, IPv6> must license the IPv4 unicast announce")
			select {
			case <-messages:
			case <-time.After(2 * time.Second):
				t.Fatal("the licensed announce sent no UPDATE")
			}
		})
	}
}

// TestRFC8950BatchRailIPv4UnicastIPv6NextHopUsesMPReach is the encoding half of
// the licensed case above.
//
// VALIDATES: with <1/1, IPv6> negotiated, an IPv4 unicast announce with the
// explicit next hop 2001:db8::1 reaches the wire as MP_REACH_NLRI for AFI 1 /
// SAFI 1 carrying that next hop and the prefix, with no NEXT_HOP attribute and an
// empty NLRI field. RFC 8950 Section 3: "this document allows advertising the
// MP_REACH_NLRI attribute [RFC4760] with this content: AFI = 1, SAFI = 1, 2, or
// 4, Length of Next Hop Address = 16 or 32".
// PREVENTS: buildBatchAnnounceUpdate treating every IPv4 unicast batch as
// inline NLRI with a NEXT_HOP attribute, which wrote a 16-octet NEXT_HOP.
//
// RFC requirement: RFC8950-4-1 positive -- with <1/1, IPv6> negotiated, the IPv4 unicast route with the IPv6 next hop is advertised as MP_REACH_NLRI AFI 1 / SAFI 1 carrying that next hop and the prefix, with no NEXT_HOP attribute and an empty NLRI field.
func TestRFC8950BatchRailIPv4UnicastIPv6NextHopUsesMPReach(t *testing.T) {
	nextHop := netip.MustParseAddr("2001:db8::1")
	settings := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65002, 0x01020301)
	peer := NewPeer(settings)
	_, messages := newEstablishedSessionForPeer(t, peer)
	peer.state.Store(int32(PeerStateEstablished))
	caps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ExtendedNextHop{Families: []capability.ExtendedNextHopFamily{{
			NLRIAFI: capability.AFIIPv4, NLRISAFI: capability.SAFIUnicast, NextHopAFI: capability.AFIIPv6,
		}}},
	}
	neg := capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65002})
	peer.currentSession().negotiated = neg
	peer.negotiated.Store(NewNegotiatedCapabilities(neg))
	peer.sendCtx.Store(bgpctx.NewEncodingContext(neg.Identity, neg.Encoding, bgpctx.DirectionSend))

	a := &reactorAPIAdapter{r: &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}}
	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("203.0.113.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(nextHop),
	}
	require.NoError(t, a.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()))
	var msg []byte
	select {
	case msg = <-messages:
	case <-time.After(2 * time.Second):
		t.Fatal("the licensed announce sent no UPDATE")
	}
	mpReach := rfc8950UpdateMPReach(t, msg)
	parsed, err := attribute.ParseMPReachNLRI(mpReach)
	require.NoError(t, err)
	require.Equal(t, attribute.AFIIPv4, parsed.AFI)
	require.Equal(t, attribute.SAFIUnicast, parsed.SAFI)
	require.Equal(t, nextHop.AsSlice(), mpReach[4:4+int(mpReach[3])], "MP_REACH_NLRI carries the IPv6 next hop")
	// The reserved octet follows the next hop, then the NLRI: 203.0.113.0/24.
	require.Equal(t, []byte{0, 24, 203, 0, 113}, mpReach[4+int(mpReach[3]):], "MP_REACH_NLRI carries the prefix")

	attrs, inline := rfc8950UpdateSections(t, msg)
	_, _, _, hasNextHop := attribute.AttrFind(attrs, attribute.AttrNextHop)
	require.False(t, hasNextHop, "the NEXT_HOP attribute holds an IPv4 address only")
	require.Empty(t, inline, "the prefix travels in MP_REACH_NLRI, not the NLRI field")
}

// rfc8950UpdateMPReach returns the MP_REACH_NLRI value of one BGP UPDATE message
// as read off the wire, header included.
func rfc8950UpdateMPReach(t *testing.T, msg []byte) []byte {
	t.Helper()
	attrs, _ := rfc8950UpdateSections(t, msg)
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	require.True(t, found, "the UPDATE must carry MP_REACH_NLRI")
	return value
}

// rfc8950UpdateSections splits one BGP UPDATE message, header included, into its
// Path Attributes field and its NLRI field.
func rfc8950UpdateSections(t *testing.T, msg []byte) (attrs, inline []byte) {
	t.Helper()
	const headerOctets = 19
	require.Greater(t, len(msg), headerOctets+4)
	require.Equal(t, byte(2), msg[18], "the message must be an UPDATE")
	body := msg[headerOctets:]
	withdrawnOctets := int(binary.BigEndian.Uint16(body))
	attrsStart := 2 + withdrawnOctets + 2
	require.LessOrEqual(t, attrsStart, len(body))
	attrOctets := int(binary.BigEndian.Uint16(body[2+withdrawnOctets:]))
	require.LessOrEqual(t, attrsStart+attrOctets, len(body))
	return body[attrsStart : attrsStart+attrOctets], body[attrsStart+attrOctets:]
}
