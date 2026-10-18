// Design: docs/architecture/update-building.md -- VPN next-hop link-local condition.
// Related: peer_forward_facts.go -- applyFactsNextHop, the configured next-hop rewrite.
// Related: rfc4659_vpn_link_local_test.go -- the speaker, segment and hex fixtures reused here.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// vpnNHRewritePeer builds an established destination with a configured
// next-hop mode. A recipient inside vpnLLSegment gets its link scope settled
// against that segment; an IPv4 recipient gets none.
func vpnNHRewritePeer(t *testing.T, recipient, local string, mode uint8, explicit string) *Peer {
	t.Helper()
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       netip.MustParseAddr(recipient),
		LocalAddress:  netip.MustParseAddr(local),
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65002,
		RouterID:      0x0a000001,
		NextHopMode:   mode,
	}
	if explicit != "" {
		settings.NextHopAddress = netip.MustParseAddr(explicit)
	}
	if settings.Address.Is6() {
		settings.LinkLocal = netip.MustParseAddr(vpnLLLinkLocal)
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.sendCtx.Store(bgpctx.EncodingContextForASN4(true))
	if settings.Address.Is6() {
		peer.llScope.Store(newLinkScopeFrom(vpnLLSegment, settings.Address))
	}
	peer.fwdFacts.Store(peer.buildForwardFacts())
	return peer
}

// vpnNHRelay runs a received MP_REACH through the configured next-hop rewrite
// and the egress normalizer, in the order both forwarding rails call them, and
// returns the emitted Next Hop field as hex.
func vpnNHRelay(t *testing.T, dest *Peer, fam family.Family, field, nlri string) string {
	t.Helper()
	fieldBytes, err := hex.DecodeString(field)
	require.NoError(t, err)
	nlriBytes, err := hex.DecodeString(nlri)
	require.NoError(t, err)
	src := buildMPReachSource(uint16(fam.AFI), byte(fam.SAFI), fieldBytes, nlriBytes)
	var mods filterapi.ModAccumulator
	applyFactsNextHop(dest.forwardFacts(), &mods, fam)
	applyEgressNextHopScope(dest, dest.forwardFacts(), &mods, buildUpdatePayload(src, nil), fam)
	out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, mods.Ops())
	require.True(t, ok, "the rewrite emits the MP_REACH attribute")
	return vpnLLNextHopField(t, out)
}

const (
	// VPN-IPv6 NLRI: 120 bits (label 1000 bottom-of-stack, RD 65001:100, 2001:db8::/32).
	vpnNHNLRIv6 = "78" + "003e81" + "0000fde900000064" + "20010db8"
	// VPN-IPv4 NLRI: 112 bits (label 1000 bottom-of-stack, RD 65001:100, 10.1.0.0/24).
	vpnNHNLRIv4 = "70" + "003e81" + "0000fde900000064" + "0a0100"
	// A received third-party next hop: zero RD plus 2001:db8:99::1.
	vpnNHReceived = vpnLLRDZero + "20010db8009900000000000000000001"
)

// TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD relays a VPN-IPv6 route to a
// destination configured with next-hop self or an explicit IPv6 next hop.
//
// VALIDATES: the configured rewrite writes the VPN-IPv6 form, zero RD before
// each address: 48 octets with the speaker's link-local toward a destination on
// the common subnet, 24 octets toward one off it and for an explicit address.
// PREVENTS: next-hop self or explicit writing the plain 16- or 32-octet RFC 2545
// field into a SAFI 128 MP_REACH, whose only legal lengths are 24 and 48.
//
// RFC requirement: RFC4659-3.2.1.1-3 positive -- a relayed VPN-IPv6 route under next-hop self toward a destination sharing 2001:db8:1::/64 with the speaker leaves with the 48-octet field: zero RD, global 2001:db8:1::2, zero RD, link-local fe80::2.
// RFC requirement: RFC4659-3.2.1.1-3 negative -- the same route under next-hop self toward a destination on no connected subnet leaves with the 24-octet field, zero RD plus the global alone.
func TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD(t *testing.T) {
	vpnv6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
	for _, tc := range []struct {
		name      string
		recipient string
		mode      uint8
		explicit  string
		want      string
	}{
		{name: "self on-link", recipient: vpnLLOnLink, mode: NextHopSelf, want: vpnLLRDZero + vpnLLGlobal + vpnLLRDZero + vpnLLLL},
		{name: "self off-link", recipient: vpnLLOffLink, mode: NextHopSelf, want: vpnLLRDZero + vpnLLGlobal},
		{name: "explicit off-link", recipient: vpnLLOffLink, mode: NextHopExplicit, explicit: "2001:db8:77::1", want: vpnLLRDZero + "20010db8007700000000000000000001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dest := vpnNHRewritePeer(t, tc.recipient, vpnLLSpeaker, tc.mode, tc.explicit)
			assert.Equal(t, tc.want, vpnNHRelay(t, dest, vpnv6, vpnNHReceived, vpnNHNLRIv6))
		})
	}
}

// TestRFC4364ForwardVPNIPv4TransportNextHopRewrite relays VPN routes to a
// destination whose next-hop self is an IPv4 address.
//
// VALIDATES: VPN-IPv4 gets the 12-octet zero RD plus IPv4 field (RFC 4364
// Section 4.3.2); VPN-IPv6 gets the 24-octet zero RD plus IPv4-mapped IPv6
// field (RFC 4659 Section 3.2.1.2).
// PREVENTS: the rewrite writing the bare 16-octet mapped address, which no
// SAFI 128 length admits, or the materializer refusing the 12-octet field.
func TestRFC4364ForwardVPNIPv4TransportNextHopRewrite(t *testing.T) {
	dest := vpnNHRewritePeer(t, "10.0.0.3", "10.0.0.2", NextHopSelf, "")
	vpnv4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	vpnv6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
	assert.Equal(t, vpnLLRDZero+"0a000002", vpnNHRelay(t, dest, vpnv4, vpnLLRDZero+"0a090909", vpnNHNLRIv4))
	assert.Equal(t, vpnLLRDZero+"00000000000000000000ffff0a000002", vpnNHRelay(t, dest, vpnv6, vpnNHReceived, vpnNHNLRIv6))
}

// TestRFC2545StaticLabeledIPv6LinkLocalCommonSubnet originates a configured
// IPv6 labeled-unicast route to a recipient on the speaker's subnet and to one
// off it.
//
// VALIDATES: the link-local Peer.linkLocalNextHopFor decides reaches the SAFI 4
// builder: 32 octets (global then link-local) on-link, 16 octets off-link.
// PREVENTS: the labeled-unicast branch of buildStaticRouteUpdateNew dropping the
// decided link-local, as the VPN branch did before 43d84584f8.
//
// RFC requirement: RFC2545-3-3 positive -- a configured IPv6 labeled-unicast route sent to a recipient sharing 2001:db8:1::/64 with the speaker, whose next hop is the speaker's global 2001:db8:1::2, carries the 32-octet field: global 2001:db8:1::2 then link-local fe80::2.
// RFC requirement: RFC2545-3-3 negative -- the same route sent to a recipient on no connected subnet carries the 16-octet global alone.
func TestRFC2545StaticLabeledIPv6LinkLocalCommonSubnet(t *testing.T) {
	route := &StaticRoute{
		Prefix: netip.MustParsePrefix("2001:db8:5601::/48"),
		Labels: []uint32{1000},
	}
	nextHop := netip.MustParseAddr(vpnLLSpeaker)
	for _, tc := range []struct {
		name      string
		recipient string
		want      string
	}{
		{name: "common subnet", recipient: vpnLLOnLink, want: vpnLLGlobal + vpnLLLL},
		{name: "no common subnet", recipient: vpnLLOffLink, want: vpnLLGlobal},
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
