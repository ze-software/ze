// Design: docs/architecture/update-building.md -- VPN next-hop link-local condition.
// Related: egress_inject_filter.go -- normalizeOrdinaryNextHop, the session egress gate.
// Related: forward_next_hop.go -- applyEgressNextHopScope, the decision it reuses.
// Related: filter_delta_handlers.go -- familyAdmitsNextHopOctets, the width table.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// vpnLLAPINLRI is one VPN-IPv6 NLRI: 120 bits, label 1000 bottom-of-stack,
// RD 65001:100, prefix 2001:db8::/32 (RFC 4659 Section 3.2; RFC 4364 Section 4.3.4).
const vpnLLAPINLRI = "78" + "003e81" + "0000fde900000064" + "20010db8"

// vpnLLAPIAnnounce announces one VPN-IPv6 route with the explicit next hop
// through the operator batch API to a recipient whose session egress is the
// production gate (peer_run.go installs exportFilterForBody the same way), and
// returns the Next Hop field the recipient read off the wire, as hex.
func vpnLLAPIAnnounce(t *testing.T, recipient string, nextHop netip.Addr) string {
	t.Helper()
	vpnv6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
	peer := vpnLLPeer(t, recipient)
	session, messages := newEstablishedSessionForPeer(t, peer)
	caps := []capability.Capability{
		&capability.ASN4{ASN: 65000},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIVPN},
	}
	neg := capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65002})
	session.negotiated = neg
	peer.negotiated.Store(NewNegotiatedCapabilities(neg))
	peer.sendCtx.Store(bgpctx.NewEncodingContext(neg.Identity, neg.Encoding, bgpctx.DirectionSend))
	peer.fwdFacts.Store(peer.buildForwardFacts())

	api := groupUpdatesReactor([]*Peer{peer}, false)
	session.egressRouteFilter = func(body []byte) (bool, []byte) {
		return api.r.exportFilterForBody(peer, body)
	}
	routeNLRI, err := nlri.NewWireNLRI(vpnv6, mustHex(t, vpnLLAPINLRI), false)
	require.NoError(t, err)
	batch := bgptypes.NLRIBatch{Family: vpnv6, NLRIs: []nlri.NLRI{routeNLRI}, NextHop: bgptypes.NewNextHopExplicit(nextHop)}
	require.NoError(t, api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()))

	var msg []byte
	select {
	case msg = <-messages:
	case <-time.After(2 * time.Second):
		t.Fatal("the VPN-IPv6 announce sent no UPDATE")
	}
	attrs, _ := rfc8950UpdateSections(t, msg)
	return vpnLLNextHopField(t, attrs)
}

// TestRFC4659APIVPNv6LinkLocalCommonSubnet announces a VPN-IPv6 route through
// the operator batch API (AnnounceNLRIBatch) and reads the recipient's wire.
//
// VALIDATES: the API rail reaches the same common-subnet decision as the
// configured and forwarding rails: the on-link recipient gets the 48-octet
// RD+global, RD+link-local field, the off-link one the 24-octet field.
// PREVENTS: the session egress normalizer skipping SAFI 128, which sent the
// 24-octet field (length 0x18) to a recipient on the speaker's own subnet.
//
// RFC requirement: RFC4659-3.2.1.1-3 positive -- a VPN-IPv6 route announced through the operator batch API with the speaker's own global 2001:db8:1::2 as next hop reaches a recipient sharing the connected 2001:db8:1::/64 with the 48-octet Next Hop field: zero RD, global 2001:db8:1::2, zero RD, link-local fe80::2.
// RFC requirement: RFC4659-3.2.1.1-3 negative -- the same API announce to a recipient on no connected subnet carries the 24-octet field, zero RD plus the global alone.
func TestRFC4659APIVPNv6LinkLocalCommonSubnet(t *testing.T) {
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
			assert.Equal(t, tc.want, vpnLLAPIAnnounce(t, tc.recipient, nextHop))
		})
	}
}

// TestMPReachNextHopHandlerRefusesOffTableWidth sets a Next Hop field whose
// width is one the MP_REACH handler's generic set knows (4, 12, 16, 24, 32, 48)
// but the family's own length table (attribute.ValidNextHopLens) does not.
//
// VALIDATES: familyAdmitsNextHopOctets keeps the original attribute for every
// off-table width, per family, and still applies an on-table width.
// PREVENTS: a 16-octet field in SAFI 128 or a 24-octet field in IPv6 unicast
// leaving the speaker because the generic set alone admitted it. RFC 4659
// Section 3.2.1.1 admits 24 and 48 octets for VPN-IPv6; RFC 2545 Section 3
// admits 16 and 32 for IPv6 unicast.
func TestMPReachNextHopHandlerRefusesOffTableWidth(t *testing.T) {
	const (
		v4      = "c0000201"
		v6      = "20010db8000100000000000000000002"
		ll      = "fe800000000000000000000000000002"
		rd      = "0000000000000000"
		ipv4NLR = "18c63364"           // 198.51.100.0/24
		ipv6NLR = "4020010db800070000" // 2001:db8:7::/64
	)
	for _, tc := range []struct {
		name     string
		afi      uint16
		safi     byte
		original string
		nlri     string
		set      string
		applied  bool
	}{
		{name: "ipv4-unicast/12", afi: 1, safi: 1, original: v4, nlri: ipv4NLR, set: rd + v4},
		{name: "ipv4-unicast/24", afi: 1, safi: 1, original: v4, nlri: ipv4NLR, set: rd + v6},
		{name: "ipv4-unicast/16", afi: 1, safi: 1, original: v4, nlri: ipv4NLR, set: v6, applied: true},
		{name: "ipv6-unicast/4", afi: 2, safi: 1, original: v6, nlri: ipv6NLR, set: v4},
		{name: "ipv6-unicast/24", afi: 2, safi: 1, original: v6, nlri: ipv6NLR, set: rd + v6},
		{name: "ipv6-unicast/32", afi: 2, safi: 1, original: v6, nlri: ipv6NLR, set: v6 + ll, applied: true},
		{name: "vpn-ipv4/4", afi: 1, safi: 128, original: rd + v4, nlri: vpnLLAPINLRI, set: v4},
		{name: "vpn-ipv4/16", afi: 1, safi: 128, original: rd + v4, nlri: vpnLLAPINLRI, set: v6},
		{name: "vpn-ipv4/24", afi: 1, safi: 128, original: rd + v4, nlri: vpnLLAPINLRI, set: rd + v6, applied: true},
		{name: "vpn-ipv6/12", afi: 2, safi: 128, original: rd + v6, nlri: vpnLLAPINLRI, set: rd + v4},
		{name: "vpn-ipv6/16", afi: 2, safi: 128, original: rd + v6, nlri: vpnLLAPINLRI, set: v6},
		{name: "vpn-ipv6/32", afi: 2, safi: 128, original: rd + v6, nlri: vpnLLAPINLRI, set: v6 + ll},
		{name: "vpn-ipv6/48", afi: 2, safi: 128, original: rd + v6, nlri: vpnLLAPINLRI, set: rd + v6 + rd + ll, applied: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := buildMPReachSource(tc.afi, tc.safi, mustHex(t, tc.original), mustHex(t, tc.nlri))
			var mods filterapi.ModAccumulator
			mods.OpCopy(14, filterapi.AttrModSet, mustHex(t, tc.set))
			out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, mods.Ops())
			require.True(t, ok, "the handler emits the attribute")
			if !tc.applied {
				assert.Equal(t, hex.EncodeToString(src), hex.EncodeToString(out), "an off-table width leaves the attribute unchanged")
				return
			}
			assert.Equal(t, tc.set, vpnLLNextHopField(t, out), "an on-table width is written")
		})
	}
}

// TestRFC4659StaticVPNv6IPv4NextHopIsMapped originates a configured VPN-IPv6
// route whose next hop is an IPv4 address.
//
// VALIDATES: the configured rail writes the 24-octet zero RD plus IPv4-mapped
// IPv6 form RFC 4659 Section 3.2.1.2 defines ("whose 16-octet IPv6 address is
// encoded as an IPv4-mapped IPv6 address"), the form next-hop self already
// writes on the forwarding rails (precomputeVPNNextHop).
// PREVENTS: the 12-octet zero RD plus IPv4 field, a VPN-IPv4 length that RFC
// 4659 Section 3.2.1.1's VPN-IPv6 table (24 or 48) does not hold.
func TestRFC4659StaticVPNv6IPv4NextHopIsMapped(t *testing.T) {
	route := &StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8:5601::/48"),
		RD:      "65001:100",
		RDBytes: [8]byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 100},
		Labels:  []uint32{1000},
	}
	nextHop := netip.MustParseAddr("192.0.2.1")
	peer := vpnLLPeer(t, vpnLLOnLink)
	ub := message.NewUpdateBuilder(65000, true, true, false)
	update, err := buildStaticRouteUpdateNew(ub, route, nextHop, peer.linkLocalNextHopFor(nil, nextHop), peer.sendCtx.Load(), false)
	require.NoError(t, err)
	assert.Equal(t, vpnLLRDZero+"00000000000000000000ffffc0000201", vpnLLNextHopField(t, update.PathAttributes))
}

// TestRFC4659APIVPNv6IPv4NextHopIsMapped announces a VPN-IPv6 route with an
// IPv4 next hop through the operator batch API.
//
// VALIDATES: the API rail writes the same 24-octet zero RD plus IPv4-mapped
// IPv6 form as the configured rail (RFC 4659 Section 3.2.1.2).
func TestRFC4659APIVPNv6IPv4NextHopIsMapped(t *testing.T) {
	got := vpnLLAPIAnnounce(t, vpnLLOnLink, netip.MustParseAddr("192.0.2.1"))
	assert.Equal(t, vpnLLRDZero+"00000000000000000000ffffc0000201", got)
}
