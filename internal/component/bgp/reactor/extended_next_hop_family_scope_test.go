// Design: docs/architecture/wire/capabilities.md -- RFC 8950 extended next hop
// Related: peer.go -- rfc8950Family, Peer.resolveNextHop
// Related: forward_next_hop.go -- egressNextHopLacksExtendedNextHop
// Related: rfc8950_reactor_a2_forward_test.go -- the forward rail helpers a2Dest and a2Forward

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// rfc8950ScopeCases lists every AFI 1 family RFC 8950 Section 3 extends, and
// IPv4 SR Policy, whose own RFC 9830 Section 2.1 allows an IPv6 next hop "independent
// of the SR Policy AFI".
var rfc8950ScopeCases = []struct {
	name     string
	family   family.Family
	extended bool // RFC 8950 governs the family, so an IPv6 next hop needs the pair.
}{
	{"ipv4 unicast", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}, true},
	{"ipv4 multicast", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMulticast}, true},
	{"ipv4 labeled unicast", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}, true},
	{"vpn-ipv4 unicast", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, true},
	{"vpn-ipv4 multicast", family.Family{AFI: family.AFIIPv4, SAFI: safiVPNMulticast}, true},
	{"ipv4 sr policy", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}, false},
}

// TestRFC8950GatesCoverOnlyTheRFC8950Families checks the scope of the two RFC 8950
// Section 4 gates: the explicit next hop on the announce rails
// (Peer.resolveNextHop) and the forwarded next hop (egressNextHopLacksExtendedNextHop).
//
// Method: a peer that negotiated no Extended Next Hop pair is asked, for each AFI 1
// family, whether it may be sent the IPv6 next hop 2001:db8::1. The five RFC 8950
// families are refused. IPv4 SR Policy (1/73) is accepted, because RFC 9830 Section
// 2.1 lets its next hop be IPv6 whatever the AFI, with no Extended Next Hop pair.
//
// PREVENTS: an IPv4 SR Policy route with an IPv6 next hop being refused on the
// announce rail or withheld on the forward rails.
func TestRFC8950GatesCoverOnlyTheRFC8950Families(t *testing.T) {
	nextHop := netip.MustParseAddr("2001:db8::1")

	peer := NewPeer(NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65002, 0x01020301))
	peer.sendCtx.Store(bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{}, bgpctx.DirectionSend))

	for _, tc := range rfc8950ScopeCases {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := peer.resolveNextHop(nil, bgptypes.NewNextHopExplicit(nextHop), tc.family)
			var mods filterapi.ModAccumulator
			withheld := egressNextHopLacksExtendedNextHop(peer, &mods, nextHopValue{mp: nextHop, mpFamily: tc.family})
			if tc.extended {
				require.ErrorIs(t, err, ErrNextHopIncompatible, "an RFC 8950 family needs the pair")
				require.False(t, addr.IsValid(), "a refused next hop carries no address")
				require.True(t, withheld, "the forward rails withhold an RFC 8950 family without the pair")
				return
			}
			require.NoError(t, err, "RFC 9830 Section 2.1 allows an IPv6 next hop for IPv4 SR Policy")
			require.Equal(t, nextHop, addr)
			require.False(t, withheld, "the forward rails send IPv4 SR Policy with an IPv6 next hop")
		})
	}
}

// TestRFC9830SRPolicyNextHopSelfIgnoresTheAFI checks next-hop self for SR Policy.
// RFC 9830 Section 2.1: the next hop "may be either a 4-octet IPv4 address or a
// 16-octet IPv6 address, independent of the SR Policy AFI".
//
// Method: a peer that negotiated no Extended Next Hop pair resolves next-hop self
// over an IPv6 session for IPv4 SR Policy (1/73), and over an IPv4 session for
// IPv6 SR Policy (2/73). Both resolve to the session's local address. IPv4
// unicast over the IPv6 session is still refused, so the SR Policy answer is not
// a gate that accepts everything.
func TestRFC9830SRPolicyNextHopSelfIgnoresTheAFI(t *testing.T) {
	for _, tc := range []struct {
		name     string
		local    netip.Addr
		family   family.Family
		accepted bool
	}{
		{"ipv4 sr policy, ipv6 session", netip.MustParseAddr("2001:db8::1"), family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}, true},
		{"ipv6 sr policy, ipv4 session", netip.MustParseAddr("192.0.2.1"), family.Family{AFI: family.AFIIPv6, SAFI: family.SAFISRPolicy}, true},
		{"ipv4 unicast, ipv6 session", netip.MustParseAddr("2001:db8::1"), family.IPv4Unicast, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("10.0.0.2"), 65000, 65002, 0x01020301)
			peer := NewPeer(settings)
			peer.sendCtx.Store(bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{}, bgpctx.DirectionSend))
			session := NewSession(settings)
			session.transport.Store(&sessionTransport{local: tc.local})

			addr, err := peer.resolveNextHop(session, bgptypes.NewNextHopSelf(), tc.family)
			if !tc.accepted {
				require.ErrorIs(t, err, ErrNextHopIncompatible)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.local, addr)
		})
	}
}

// TestRFC9830IPv4SRPolicyIPv6NextHopIsForwardedWithoutThePair drives the forward
// rails with an IPv4 SR Policy route whose next hop is IPv6.
//
// Method: one UPDATE withdraws 198.51.100.0/24 and announces an IPv4 SR Policy
// (1/73) NLRI in MP_REACH_NLRI with the 16-octet next hop 2001:db8::1. It goes to
// a destination that negotiated 1/73 and no Extended Next Hop pair, on the general
// rail and on the route-server rail.
//
// VALIDATES: the destination is sent the MP_REACH_NLRI for 1/73 with the IPv6
// next hop unchanged. RFC 9830 Section 2.1: the next hop "may be either a
// 4-octet IPv4 address or a 16-octet IPv6 address, independent of the SR Policy
// AFI".
// PREVENTS: the RFC 8950 Section 4 gate, which covers IPv4 and VPN-IPv4 NLRI
// only, withholding an SR Policy route.
func TestRFC9830IPv4SRPolicyIPv6NextHopIsForwardedWithoutThePair(t *testing.T) {
	nextHop := netip.MustParseAddr("2001:db8::1")
	srPolicy := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}

	// RFC 9830 Section 2.1 NLRI: Length 96 bits, Distinguisher 1, Color 100,
	// Endpoint 192.0.2.1.
	policy := []byte{96, 0, 0, 0, 1, 0, 0, 0, 100, 192, 0, 2, 1}
	mpReach := []byte{0x00, 0x01, byte(family.SAFISRPolicy), 16}
	mpReach = append(mpReach, nextHop.AsSlice()...)
	mpReach = append(mpReach, 0x00)
	mpReach = append(mpReach, policy...)
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x06, 0x02, 0x01, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x80, 0x0E, byte(len(mpReach)), // MP_REACH_NLRI header
	}
	attrs = append(attrs, mpReach...)
	payload := []byte{0x00, byte(len(a2Withdrawn))}
	payload = append(payload, a2Withdrawn...)
	payload = append(payload, 0x00, byte(len(attrs)))
	payload = append(payload, attrs...)

	for _, rs := range []bool{false, true} {
		t.Run(map[bool]string{false: "general rail", true: "route-server rail"}[rs], func(t *testing.T) {
			dest := a2Dest(t, "192.0.2.23", 65023, netip.Addr{}, false)
			dest.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{
				family.IPv4Unicast: true,
				srPolicy:           true,
			}})
			dest.refreshForwardFacts()

			got := a2Forward(t, rs, payload, dest)

			sent, ok := got[netip.MustParseAddr("192.0.2.23")]
			require.True(t, ok, "the destination is owed the update")
			require.Equal(t, a2Withdrawn, sent.withdrawn)
			require.GreaterOrEqual(t, len(sent.mpReach), 4+16)
			require.Equal(t, []byte{0x00, 0x01, byte(family.SAFISRPolicy), 16}, sent.mpReach[:4],
				"IPv4 SR Policy with a 16-octet next hop")
			require.Equal(t, nextHop.AsSlice(), sent.mpReach[4:20])
		})
	}
}
