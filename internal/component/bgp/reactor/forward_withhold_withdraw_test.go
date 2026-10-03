// Design: docs/architecture/bgp/structural-forwarding.md -- withheld routes are withdrawn
// Related: forward_next_hop.go -- egressNextHopWithheld, the next-hop gates both rails ask
// Related: forward_build.go -- buildWithdrawalPayload
// Related: draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go -- llnhWritten, llnhItemWritten
//
// A destination a withhold gate refuses may hold the previous generation of the
// route, and neither forward rail keeps a per-peer Adj-RIB-Out that could say it
// never did. So every gate sends the refused destination the withdrawal of the
// route (RFC 7606 Section 2, treat-as-withdraw) instead of nothing. These tests
// read the MP_UNREACH_NLRI, or the Withdrawn Routes field, each refused
// destination was written, one gate at a time, beside a destination the same
// gate passes in the same fan-out.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// llnhReflectedWithdrawal is the MP_UNREACH_NLRI value that withdraws the prefix
// llnhReflectedPayload announces: AFI, SAFI, then the NLRI of its MP_REACH_NLRI.
func llnhReflectedWithdrawal(t *testing.T, payload []byte) []byte {
	t.Helper()
	update, err := message.UnpackUpdate(payload)
	require.NoError(t, err)
	_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	require.True(t, found, "fixture has no MP_REACH_NLRI")
	nhLen := int(mp[3])
	// AFI(2) SAFI(1) NH-length(1) NH(nhLen) Reserved(1), then the NLRI.
	return append([]byte{mp[0], mp[1], mp[2]}, mp[4+nhLen+1:]...)
}

// TestReflectedLinkLocalOnlyRouteWithdrawnFromClientOffTheSegment reflects a
// Link-Local-only next hop to a client off the advertiser's segment and to one
// on it.
//
// VALIDATES: the client off the segment is announced nothing and written the
// MP_UNREACH_NLRI that withdraws 2001:db8:7::/64; the client on the segment is
// announced the unchanged fe80::1. Draft Section 4: "A Route Reflector (RR)
// reflecting a route with a link-local-only next hop MUST NOT advertise that
// route to a client unless the client shares the same link-layer segment as the
// original advertiser."
// PREVENTS: the off-segment client keeping an earlier generation of the route,
// whose next hop it may still be told to use.
func TestReflectedLinkLocalOnlyRouteWithdrawnFromClientOffTheSegment(t *testing.T) {
	payload := llnhReflectedPayload("fe80::1")
	offSegment := llnhClient(t, llnhOffSegmentAddr, llnhSegment, false /*nextHopSelf*/)
	onSegment := llnhClient(t, llnhOnSegmentAddr, llnhSegment, false /*nextHopSelf*/)

	got := llnhForwardWritten(t, payload, forwardSourceInfo{
		resolved: true, isIBGP: true, isRRClient: true, globalLocalAS: 65000,
	}, offSegment, onSegment)

	off, written := got[netip.MustParseAddr(llnhOffSegmentAddr)]
	require.True(t, written, "the client off the segment is written the withdrawal")
	assert.Nil(t, off.nextHop, "nothing is announced to it")
	assert.Equal(t, llnhReflectedWithdrawal(t, payload), off.unreach, "the route is withdrawn")

	on, written := got[netip.MustParseAddr(llnhOnSegmentAddr)]
	require.True(t, written, "the client on the segment is owed the route")
	assert.Equal(t, netip.MustParseAddr("fe80::1").AsSlice(), on.nextHop)
	assert.Nil(t, on.unreach, "the client on the segment is withdrawn nothing")
}

// TestRFC8950WithheldRouteWithdrawnFromPeerLackingThePair forwards IPv4 unicast
// with the IPv6 next hop 2001:db8::1 on both rails, to a destination that
// negotiated <1/1, IPv6> and to one that did not.
//
// VALIDATES: the unpaired destination is announced nothing and written the
// MP_UNREACH_NLRI for 1/1 that withdraws 192.0.2.0/24; the paired one is
// announced the route. RFC 8950 Section 4: "A BGP speaker MUST only advertise
// the IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a BGP peer if the BGP
// speaker has first ascertained via the BGP Capability Advertisement that the BGP
// peer supports the Extended Next Hop Encoding capability for the relevant
// AFI/SAFI pair."
// PREVENTS: the unpaired destination keeping the IPv4 next-hop generation of the
// route after the source moved it to an IPv6 next hop.
func TestRFC8950WithheldRouteWithdrawnFromPeerLackingThePair(t *testing.T) {
	payload := a2Payload(netip.MustParseAddr("2001:db8::1").AsSlice())
	withdrawal := append([]byte{0x00, 0x01, 0x01}, a2Announced...)
	for _, rs := range []bool{false, true} {
		paired := a2Dest(t, "192.0.2.21", 65021, netip.Addr{}, true)
		unpaired := a2Dest(t, "192.0.2.22", 65022, netip.Addr{}, false)

		var got map[netip.Addr]llnhWritten
		if rs {
			got = llnhForwardRSWritten(t, payload, paired, unpaired)
		} else {
			got = llnhForwardWritten(t, payload, forwardSourceInfo{resolved: true}, paired, unpaired)
		}

		withheld, written := got[netip.MustParseAddr("192.0.2.22")]
		require.True(t, written, "rs=%v: the unpaired destination is written the withdrawal", rs)
		assert.Nil(t, withheld.nextHop, "rs=%v: no IPv6 next hop reaches it", rs)
		assert.Equal(t, withdrawal, withheld.unreach, "rs=%v: 192.0.2.0/24 is withdrawn", rs)

		sent, written := got[netip.MustParseAddr("192.0.2.21")]
		require.True(t, written, "rs=%v: the paired destination is owed the route", rs)
		assert.Equal(t, netip.MustParseAddr("2001:db8::1").AsSlice(), sent.nextHop, "rs=%v", rs)
		assert.Nil(t, sent.unreach, "rs=%v: the paired destination is withdrawn nothing", rs)
	}
}

// TestLinkLocalOnlyNextHopSelfWithdrawnWithoutTheCapability configures next-hop
// self over a session whose local endpoint is fe80::77, toward one destination
// that negotiated the Link-Local Next Hop Capability and one that did not, on
// both rails.
//
// VALIDATES: the destination without code 77 is announced nothing and written
// the MP_UNREACH_NLRI that withdraws 2001:db8:7::/64; the one with it is
// announced fe80::77. Draft Section 2: "When the capability has not been
// negotiated, the procedures in this document do not apply."
// PREVENTS: the refused destination keeping the route announced before its
// session moved to a link-local endpoint.
func TestLinkLocalOnlyNextHopSelfWithdrawnWithoutTheCapability(t *testing.T) {
	payload := nhSelfLinkLocalPayload()
	withdrawal := llnhReflectedWithdrawal(t, payload)
	permitted := netip.MustParseAddr("2001:db8:1::31")
	refused := netip.MustParseAddr("2001:db8:1::32")
	for _, rs := range []bool{false, true} {
		dests := []*Peer{
			nhSelfLinkLocalDest(t, permitted.String(), true /*nextHopSelf*/, true /*llnh*/),
			nhSelfLinkLocalDest(t, refused.String(), true /*nextHopSelf*/, false /*llnh*/),
		}

		var got map[netip.Addr]llnhWritten
		if rs {
			got = llnhForwardRSWritten(t, payload, dests...)
		} else {
			got = llnhForwardWritten(t, payload, forwardSourceInfo{
				resolved: true, isIBGP: true, isRRClient: true, globalLocalAS: 65000,
			}, dests...)
		}

		withheld, written := got[refused]
		require.True(t, written, "rs=%v: the destination without code 77 is written the withdrawal", rs)
		assert.Nil(t, withheld.nextHop, "rs=%v: nothing is announced to it", rs)
		assert.Equal(t, withdrawal, withheld.unreach, "rs=%v: the route is withdrawn", rs)

		sent, written := got[permitted]
		require.True(t, written, "rs=%v: the destination with code 77 is owed the route", rs)
		assert.Equal(t, nhSelfLinkLocalEndpoint.AsSlice(), sent.nextHop, "rs=%v", rs)
	}
}

// TestEgressFilterRejectWithdrawsOnBothRails installs an egress filter that
// refuses one destination and passes another, and forwards one UPDATE that
// withdraws 198.51.100.0/24 and announces 10.0.0.0/24 on each rail.
//
// VALIDATES: the refused destination is announced nothing and written both
// prefixes in the Withdrawn Routes field; the passed one is announced 10.0.0.0/24
// and withdrawn 198.51.100.0/24 alone.
// PREVENTS: a destination an operator filter now refuses keeping the route it was
// announced before the filter changed, on either rail.
func TestEgressFilterRejectWithdrawsOnBothRails(t *testing.T) {
	refusedAddr := netip.MustParseAddr("10.0.0.3")
	passedAddr := netip.MustParseAddr("10.0.0.4")
	payload := wkMixedPayload(attribute.Community(0xFDE90001))
	for _, rs := range []bool{false, true} {
		ctx := bgpctx.EncodingContextForASN4(true)
		ctxID, err := bgpctx.Registry.Register(ctx)
		require.NoError(t, err)
		refused := wkPeer(t, refusedAddr.String(), 65003, ctx, ctxID)
		passed := wkPeer(t, passedAddr.String(), 65004, ctx, ctxID)

		refuse := func(_, dest filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
			return dest.Address != refusedAddr
		}
		// The general rail runs the stage-ordered pipeline, the route-server
		// rail the in-process filters directly; each is given the same filter.
		got := a2ForwardWith(t, rs, func(r *Reactor, _ *forwardSourceInfo) {
			r.orderedEgressSteps = orderedEgressStepsFromFuncs(refuse)
			r.egressFilters = []filterapi.EgressFilterFunc{refuse}
		}, payload, refused, passed)

		withheld, written := got[refusedAddr]
		require.True(t, written, "rs=%v: the refused destination is written the withdrawal", rs)
		assert.Empty(t, withheld.nlri, "rs=%v: nothing is announced to it", rs)
		assert.Equal(t, append(append([]byte(nil), wkWithdrawnPrefix...), wkAnnouncedPrefix...), withheld.withdrawn,
			"rs=%v: the refused announcement is withdrawn beside the source's withdrawal", rs)

		sent, written := got[passedAddr]
		require.True(t, written, "rs=%v: the passed destination is owed the route", rs)
		assert.Equal(t, wkAnnouncedPrefix, sent.nlri, "rs=%v", rs)
		assert.Equal(t, wkWithdrawnPrefix, sent.withdrawn, "rs=%v", rs)
	}
}
