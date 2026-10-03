// Design: docs/architecture/bgp/structural-forwarding.md -- withheld routes are withdrawn
// RFC: rfc/short/draft-ietf-idr-linklocal-capability.md
// Related: forward_next_hop.go -- egressNextHopWithheld, egressNextHopLinkLocalOnlyOffLink
// Related: forward_build.go -- buildWithdrawalPayload
//
// draft-ietf-idr-linklocal-capability Section 4: "If, after completing these
// procedures, there are no IPv6 next hop addresses included in the next hop, the
// BGP route MUST not be advertised to its peer. Instead, treat-as-withdraw
// (Section 2 of [RFC7606]) is used." These tests relay a route through both
// forward rails and read what each destination was written: the MP_REACH_NLRI
// next hop it was announced, or the MP_UNREACH_NLRI prefixes it was withdrawn.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// llnhWritten is what one destination was written: the MP_REACH_NLRI next-hop
// field it was announced (nil for none) and the MP_UNREACH_NLRI value it was
// withdrawn (nil for none).
type llnhWritten struct {
	nextHop []byte
	unreach []byte
}

// llnhItemWritten reads both off a dispatched item, in whichever form the rail
// handed it.
func llnhItemWritten(t *testing.T, items []fwdItem) llnhWritten {
	t.Helper()
	w := llnhWritten{nextHop: llnhItemNextHopField(t, items)}
	read := func(attrs []byte) {
		if _, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPUnreachNLRI); found {
			w.unreach = append([]byte(nil), value...)
		}
	}
	for i := range items {
		for _, body := range items[i].rawBodies {
			u, err := message.UnpackUpdate(body)
			require.NoError(t, err)
			read(u.PathAttributes)
		}
		for _, u := range items[i].updates {
			read(u.PathAttributes)
		}
	}
	return w
}

// llnhLinkLocalOnlyPayload is an external announcement of 2001:db8:7::/64 whose
// MP_REACH_NLRI next hop is the 16-octet Link-Local-only fe80::1, and the
// MP_UNREACH_NLRI value that withdraws the same prefix.
func llnhLinkLocalOnlyPayload(t *testing.T) ([]byte, []byte) {
	t.Helper()
	update, err := message.UnpackUpdate(llnhReflectedPayload("fe80::1"))
	require.NoError(t, err)
	_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	require.True(t, found, "fixture has no MP_REACH_NLRI")
	// AFI(2) SAFI(1) NH-length(1) NH(16) Reserved(1), then the NLRI.
	withdrawal := append([]byte{0, 2, 1}, mp[21:]...)
	return llnhExternalPayload(t, "fe80::1"), withdrawal
}

// TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer relays a received
// Link-Local-only next hop through the general rail under next hop unchanged to
// an internal and an external peer that no connected subnet holds, and to an
// external peer on the advertiser's subnet.
//
// VALIDATES: each multihop peer is announced nothing and written the withdrawal
// of 2001:db8:7::/64; the directly attached peer receives the Link-Local-only
// next hop unchanged.
// PREVENTS: a Link-Local next hop reaching a peer more than one hop away, and
// that peer keeping an earlier generation of the route.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1 positive -- on the general rail, a received Link-Local-only next hop
// passed unchanged toward an internal and an external peer more than one hop away
// leaves no IPv6 next hop to include, and each such peer is written no MP_REACH_NLRI
// next hop and the MP_UNREACH_NLRI withdrawing the prefix.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1 negative -- the refusal is keyed on the hop count: an external peer on
// the advertiser's subnet, in the same fan-out, is written the Link-Local-only next hop.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4 positive -- the internal multihop peer is written no MP_REACH_NLRI next hop
// and the withdrawal of the prefix.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9 positive -- the external multihop peer, with no Global next hop to include, is
// written no MP_REACH_NLRI next hop and the withdrawal of the prefix.
func TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer(t *testing.T) {
	payload, withdrawal := llnhLinkLocalOnlyPayload(t)
	for _, internal := range []bool{true, false} {
		multihop := llnhExternalPeer(t, llnhMultihopAddr, llnhSegment, NextHopUnchanged)
		if internal {
			multihop.settings.PeerAS = 65000
			multihop.refreshForwardFacts()
			// refreshForwardFacts re-reads the link scope from the host; the
			// fixture's interface table is the one under test.
			multihop.llScope.Store(newLinkScopeFrom(llnhSegment, multihop.settings.Address))
		}
		attached := llnhExternalPeer(t, llnhOnSegmentAddr, llnhSegment, NextHopUnchanged)

		got := llnhForwardWritten(t, payload, llnhExternalSource, multihop, attached)

		far, written := got[netip.MustParseAddr(llnhMultihopAddr)]
		require.True(t, written, "internal=%v: the multihop peer is written the withdrawal", internal)
		assert.Nil(t, far.nextHop, "internal=%v: nothing is announced to it", internal)
		assert.Equal(t, withdrawal, far.unreach, "internal=%v: the route is withdrawn", internal)

		near, written := got[netip.MustParseAddr(llnhOnSegmentAddr)]
		require.True(t, written, "internal=%v: the attached peer is owed the route", internal)
		assert.Equal(t, netip.MustParseAddr("fe80::1").AsSlice(), near.nextHop,
			"internal=%v: the attached peer keeps the received Link-Local-only next hop", internal)
	}
}

// TestLinkLocalOnlyRouteServerWithdrawnFromMultihopClient is the route-server
// twin: reactorForwardRS asks the same gate.
//
// VALIDATES: a multihop client is written the withdrawal, an attached client the
// unchanged Link-Local-only next hop.
// PREVENTS: the two rails answering differently for the deployment that runs
// rs-fast-path.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9 positive -- on the route-server rail, the external client more than one hop
// away, with no Global next hop to include, is written no MP_REACH_NLRI next hop and the
// MP_UNREACH_NLRI withdrawing the prefix.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9 negative -- the external client on the advertiser's subnet, in the same
// fan-out, is written the Link-Local-only next hop.
func TestLinkLocalOnlyRouteServerWithdrawnFromMultihopClient(t *testing.T) {
	payload, withdrawal := llnhLinkLocalOnlyPayload(t)
	multihop := llnhExternalPeer(t, llnhMultihopAddr, llnhSegment, NextHopUnchanged)
	attached := llnhExternalPeer(t, llnhOnSegmentAddr, llnhSegment, NextHopUnchanged)

	got := llnhForwardRSWritten(t, payload, multihop, attached)

	far, written := got[netip.MustParseAddr(llnhMultihopAddr)]
	require.True(t, written, "the multihop client is written the withdrawal")
	assert.Nil(t, far.nextHop)
	assert.Equal(t, withdrawal, far.unreach)

	near, written := got[netip.MustParseAddr(llnhOnSegmentAddr)]
	require.True(t, written)
	assert.Equal(t, netip.MustParseAddr("fe80::1").AsSlice(), near.nextHop)
}

// TestLinkLocalReceivedPairStrippedForMultihopInternalPeer relays the 32-octet
// Global plus Link-Local pair to an internal peer no connected subnet holds.
//
// VALIDATES: the field written is the 16-octet Global 2001:db8:1::1 alone, under
// next hop unchanged and auto, while an internal peer on the subnet keeps the
// pair. Draft Section 4: "If the internal peer is more than one IP hop away, the
// BGP speaker MUST NOT include a Link-Local IPv6 next hop."
// PREVENTS: the internal half of the cut going untested: the 4-8 units relay to
// external peers only.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2 positive -- on the general rail, under next hop unchanged and auto, the
// internal peer no connected subnet holds is written the 16-octet Global next hop alone,
// without the received Link-Local half.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2 negative -- the internal peer on the advertiser's subnet, in the same fan-out,
// is written the 32-octet Global plus Link-Local pair.
func TestLinkLocalReceivedPairStrippedForMultihopInternalPeer(t *testing.T) {
	global := netip.MustParseAddr(llnhAdvertiserAddr).As16()
	linkLocal := netip.MustParseAddr("fe80::9").As16()
	for _, mode := range []uint8{NextHopUnchanged, NextHopAuto} {
		multihop := llnhExternalPeer(t, llnhMultihopAddr, llnhSegment, mode)
		attached := llnhExternalPeer(t, llnhOnSegmentAddr, llnhSegment, mode)
		for _, p := range []*Peer{multihop, attached} {
			p.settings.PeerAS = 65000
			p.refreshForwardFacts()
			// refreshForwardFacts re-reads the link scope from the host; the
			// fixture's interface table is the one under test.
			p.llScope.Store(newLinkScopeFrom(llnhSegment, p.settings.Address))
		}

		got := llnhForwardWritten(t, llnhReceivedPairPayload(), llnhExternalSource, multihop, attached)

		assert.Equal(t, global[:], got[netip.MustParseAddr(llnhMultihopAddr)].nextHop,
			"mode %d: the multihop internal peer is written the Global alone", mode)
		assert.Equal(t, append(global[:], linkLocal[:]...), got[netip.MustParseAddr(llnhOnSegmentAddr)].nextHop,
			"mode %d: the attached internal peer keeps the pair", mode)
	}
}

// TestNextHopSelfWithheldRouteServerWithdraws is the route-server twin of
// TestDraftLinkLocalOneHopLostNextHopWithdraws: a client configured next-hop
// self with no address of this speaker to write.
//
// VALIDATES: the client is announced nothing and written the withdrawal; the
// third-party next hop is never sent in its place. Draft Section 4, one-hop
// external default procedure: "If no next hops are included, the route MUST NOT
// be announced (treat-as-withdraw)."
// PREVENTS: the client keeping the route it was announced before the speaker
// lost its usable addresses.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7 positive -- on the route-server rail, a client configured next-hop self with
// no address of this speaker to write is written no MP_REACH_NLRI next hop and the
// MP_UNREACH_NLRI withdrawing the prefix.
func TestNextHopSelfWithheldRouteServerWithdraws(t *testing.T) {
	payload, withdrawal := llnhLinkLocalOnlyPayload(t)
	client := llnhExternalPeer(t, llnhOnSegmentAddr, llnhSegment, NextHopSelf)
	require.True(t, client.forwardFacts().nhSelfWithheld, "fixture did not reach the no-next-hop procedure")

	got := llnhForwardRSWritten(t, payload, client)

	w, written := got[netip.MustParseAddr(llnhOnSegmentAddr)]
	require.True(t, written, "the client is written the withdrawal")
	assert.Nil(t, w.nextHop)
	assert.Equal(t, withdrawal, w.unreach)
}
