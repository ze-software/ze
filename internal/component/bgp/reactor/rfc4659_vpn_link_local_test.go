// Design: docs/architecture/update-building.md -- VPN next-hop link-local condition.
// Related: link_scope.go -- linkScope.linkLocalNextHop, the common-subnet decision.
// Related: forward_next_hop.go -- applyEgressNextHopScope, the forwarding rails.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// The speaker owns 2001:db8:1::2 and fe80::2 on the connected 2001:db8:1::/64.
const (
	vpnLLSpeaker   = "2001:db8:1::2"
	vpnLLLinkLocal = "fe80::2"
	vpnLLOnLink    = "2001:db8:1::3"  // recipient on the speaker's subnet
	vpnLLOffLink   = "2001:db8:ff::1" // recipient on no connected subnet
)

var vpnLLSegment = []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}

// vpnLLPeer builds an established destination whose link scope is settled
// against vpnLLSegment, with the speaker's global and link-local configured.
func vpnLLPeer(t *testing.T, recipient string) *Peer {
	t.Helper()
	peer := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       netip.MustParseAddr(recipient),
		LocalAddress:  netip.MustParseAddr(vpnLLSpeaker),
		LinkLocal:     netip.MustParseAddr(vpnLLLinkLocal),
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65002,
		RouterID:      0x0a000001,
		NextHopMode:   NextHopUnchanged,
	})
	peer.state.Store(int32(PeerStateEstablished))
	peer.sendCtx.Store(bgpctx.EncodingContextForASN4(true))
	peer.llScope.Store(newLinkScopeFrom(vpnLLSegment, peer.settings.Address))
	peer.fwdFacts.Store(peer.buildForwardFacts())
	return peer
}

// vpnLLNextHopField returns the Network Address of Next Hop field of the
// MP_REACH_NLRI in attrs, as hex.
func vpnLLNextHopField(t *testing.T, attrs []byte) string {
	t.Helper()
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	require.True(t, found, "UPDATE carries MP_REACH_NLRI")
	require.GreaterOrEqual(t, len(value), 4)
	return hex.EncodeToString(value[4 : 4+int(value[3])])
}

const (
	vpnLLRDZero = "0000000000000000"
	vpnLLGlobal = "20010db8000100000000000000000002"
	vpnLLLL     = "fe800000000000000000000000000002"
)

// TestRFC4659StaticVPNv6LinkLocalCommonSubnet originates a configured VPN-IPv6
// route to a recipient on the speaker's subnet and to one off it.
//
// VALIDATES: the decision Peer.linkLocalNextHopFor makes reaches the VPN
// builder: the on-link recipient gets the 48-octet field, the off-link one the
// 24-octet field, from the same route and the same `link-local` leaf.
// PREVENTS: the VPN branch of buildStaticRouteUpdateNew dropping the decided
// link-local (24 octets on a common subnet), or the leaf alone deciding it.
//
// RFC requirement: RFC4659-3.2.1.1-3 positive -- a configured VPN-IPv6 route sent to a recipient sharing the connected 2001:db8:1::/64 with the speaker, whose next hop is the speaker's own global on that subnet, carries the 48-octet Next Hop field: zero RD, global 2001:db8:1::2, zero RD, link-local fe80::2.
// RFC requirement: RFC4659-3.2.1.1-3 negative -- the same route and the same configured link-local sent to a recipient on no connected subnet carries the 24-octet field, zero RD plus the global alone.
func TestRFC4659StaticVPNv6LinkLocalCommonSubnet(t *testing.T) {
	route := &StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8:5601::/48"),
		RD:      "65001:100",
		RDBytes: [8]byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 100},
		Labels:  []uint32{1000},
	}
	nextHop := netip.MustParseAddr(vpnLLSpeaker)
	for _, tc := range []struct {
		name      string
		recipient string
		want      string
	}{
		{name: "common subnet", recipient: vpnLLOnLink, want: vpnLLRDZero + vpnLLGlobal + vpnLLRDZero + vpnLLLL},
		{name: "no common subnet", recipient: vpnLLOffLink, want: vpnLLRDZero + vpnLLGlobal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := vpnLLPeer(t, tc.recipient)
			ub := message.NewUpdateBuilder(65000, true, true, false)
			update, err := buildStaticRouteUpdateNew(ub, route, nextHop, peer.linkLocalNextHopFor(nil, nextHop), peer.sendCtx.Load(), false)
			require.NoError(t, err)
			assert.Equal(t, tc.want, vpnLLNextHopField(t, update.PathAttributes))
		})
	}
}

// TestRFC4659ForwardVPNv6LinkLocalCommonSubnet relays a VPN-IPv6 route through
// the egress normalizer both forwarding rails call (applyEgressNextHopScope).
//
// VALIDATES: a 24-octet RD+global field whose global is the speaker's own is
// raised to the 48-octet pair for an on-link destination and left at 24 octets
// for an off-link one; a received 48-octet pair is cut to 24 octets off-link.
// PREVENTS: the forwarding rails answering the condition differently from
// origination, or the materializer refusing the 48-octet operation.
//
// RFC requirement: RFC4659-3.2.1.1-3 positive -- a relayed VPN-IPv6 route whose 24-octet next hop is the speaker's global 2001:db8:1::2 leaves for a destination on the common 2001:db8:1::/64 with the 48-octet field ending in the speaker's link-local fe80::2.
// RFC requirement: RFC4659-3.2.1.1-3 negative -- the same route to a destination on no connected subnet keeps the 24-octet field, and a received 48-octet pair sent there loses its link-local half.
func TestRFC4659ForwardVPNv6LinkLocalCommonSubnet(t *testing.T) {
	vpnv6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
	single, err := hex.DecodeString(vpnLLRDZero + vpnLLGlobal)
	require.NoError(t, err)
	pair, err := hex.DecodeString(vpnLLRDZero + vpnLLGlobal + vpnLLRDZero + vpnLLLL)
	require.NoError(t, err)
	// VPN NLRI: 120 bits (label 1000 bottom-of-stack, RD 65001:100, 2001:db8::/32).
	nlri, err := hex.DecodeString("78" + "003e81" + "0000fde900000064" + "20010db8")
	require.NoError(t, err)
	normalize := func(dest *Peer, field []byte) string {
		t.Helper()
		src := buildMPReachSource(uint16(vpnv6.AFI), byte(vpnv6.SAFI), field, nlri)
		var mods filterapi.ModAccumulator
		applyEgressNextHopScope(dest, dest.forwardFacts(), &mods, buildUpdatePayload(src, nil), vpnv6)
		out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, mods.Ops())
		require.True(t, ok, "normalization emits the MP_REACH attribute")
		return vpnLLNextHopField(t, out)
	}
	onLink := vpnLLPeer(t, vpnLLOnLink)
	offLink := vpnLLPeer(t, vpnLLOffLink)

	assert.Equal(t, hex.EncodeToString(pair), normalize(onLink, single), "on-link: raised to the 48-octet pair")
	assert.Equal(t, hex.EncodeToString(single), normalize(offLink, single), "off-link: the global alone")
	assert.Equal(t, hex.EncodeToString(single), normalize(offLink, pair), "off-link: a received pair loses its link-local")
	assert.Equal(t, hex.EncodeToString(pair), normalize(onLink, pair), "on-link: a received pair is kept")
}
