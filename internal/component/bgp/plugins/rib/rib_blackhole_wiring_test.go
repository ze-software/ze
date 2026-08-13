// VALIDATES: the whole chain, from the operator's per-peer leaves to a stamped
// best-path change on BOTH rails out of the RIB. A received UPDATE carrying
// 65535:666 becomes a discard route only when RFC 7999 Section 3.3's two
// conditions hold.
// PREVENTS: the honoring decision existing as a function nothing calls, which
// is what the route type was before this: consumed by two FIB backends and
// produced by no protocol.

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// blackholeAttrBytes builds ORIGIN + NEXT_HOP plus a COMMUNITIES attribute
// carrying the listed 4-octet values. Passing none omits the attribute, which
// is the untagged control case.
func blackholeAttrBytes(nhIP [4]byte, communities ...[4]byte) []byte {
	attrs := makeAttrBytes(nhIP)
	if len(communities) == 0 {
		return attrs
	}
	// COMMUNITIES: flags 0xC0 (optional transitive), type 8, then the values.
	comm := []byte{0xC0, 0x08, byte(len(communities) * 4)}
	for _, c := range communities {
		comm = append(comm, c[0], c[1], c[2], c[3])
	}
	return append(attrs, comm...)
}

var (
	blackholeValue = [4]byte{0xFF, 0xFF, 0x02, 0x9A} // RFC 7999, 65535:666
	noExportValue  = [4]byte{0xFF, 0xFF, 0xFF, 0x01} // RFC 1997 NO_EXPORT
)

// blackholeRIB builds a RIBManager with one peer, a Loc-RIB wired in, and the
// given blackhole configuration for that peer.
func blackholeRIB(t *testing.T, peer netip.Addr, cfg blackholeConfig) (*RIBManager, *locrib.RIB) {
	t.Helper()
	r := newTestRIBManagerWithBus(newTestEventBus())
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	r.bgpPeers[peer] = storage.NewPeerRIB(peer.String())
	if cfg.hasAnyRule() {
		m := map[netip.Addr]blackholeConfig{peer: cfg}
		r.blackholeCfg.Store(&m)
	}
	return r, loc
}

// announce inserts one route from the peer and returns the best-path change.
func announce(t *testing.T, r *RIBManager, peer netip.Addr, nlri, attrs []byte) (bestChangeEntry, bool) {
	t.Helper()
	fam := family.Family{AFI: 1, SAFI: 1}
	r.bgpPeers[peer].Insert(fam, attrs, nlri, true)
	return r.checkBestPathChange(fam, nlri, false, nil)
}

func blackholeLocRIBType(t *testing.T, loc *locrib.RIB, pfx netip.Prefix) routetype.Type {
	t.Helper()
	best, ok := loc.Best(family.Family{AFI: 1, SAFI: 1}, pfx)
	if !ok {
		t.Fatalf("no Loc-RIB best for %v", pfx)
	}
	return best.RouteType
}

// AC-2: both RFC 7999 Section 3.3 conditions hold, so the route becomes a
// discard on both rails.
//
// RFC requirement: RFC7999-3.3-1 positive -- the announced prefix is covered by
// an equal or shorter prefix the neighbor is authorized to advertise, so the
// announcement is accepted and honored.
// RFC requirement: RFC7999-3.3-2 positive -- the receiving party agreed to
// honor BLACKHOLE on that particular BGP session, so the announcement is
// accepted and honored.
func TestBlackholeRouteTypeStampedOnBestPath(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		honor:      true,
		authorized: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})

	change, ok := announce(t, r, peer,
		ipv4Prefix(32, 10, 0, 0, 1),
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))

	require.True(t, ok, "best-path change not detected")
	assert.Equal(t, routetype.Blackhole, change.RouteType,
		"event-bus rail: the forked-plugin deployment would install a forwarding route")
	assert.Equal(t, routetype.Blackhole,
		blackholeLocRIBType(t, loc, netip.MustParsePrefix("10.0.0.1/32")),
		"Loc-RIB rail: the default in-process deployment would install a forwarding route")
}

// AC-1: RFC 7999 Section 4. A peer that never agreed discards nothing, and the
// same UPDATE that becomes a blackhole above stays an ordinary route here.
//
// This is the pair that makes the test above discriminate. An assertion that a
// blackhole route was installed passes when EVERY route is installed as one.
//
// RFC requirement: RFC7999-4-1 positive -- a network element with no blackhole
// configuration at all does not discard traffic toward a tagged prefix.
func TestBlackholeNotStampedWithoutAgreement(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{})

	change, ok := announce(t, r, peer,
		ipv4Prefix(32, 10, 0, 0, 1),
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))

	require.True(t, ok)
	assert.Equal(t, routetype.Type(0), change.RouteType,
		"an unconfigured peer made Ze discard traffic")
	assert.Equal(t, routetype.Type(0),
		blackholeLocRIBType(t, loc, netip.MustParsePrefix("10.0.0.1/32")))
}

// AC-1 again, and this is the case that discriminates. The peer above has NO
// entry in the config map at all, so it is refused by the map miss and the
// honor leaf is never consulted. Here the peer IS configured, with a covering
// authorization listed, and honor is false. RFC 7999 Section 3.3 needs both
// conditions, and the agreement is the one missing.
//
// RFC requirement: RFC7999-3.3-2 negative -- the receiving party did NOT agree
// to honor BLACKHOLE on this session, so the announcement is not honored even
// though its prefix is covered.
// RFC requirement: RFC7999-4-1 positive -- absent an explicit configuration
// directive, the network element does not discard traffic toward the tagged
// prefix.
func TestBlackholeNotStampedWhenHonorIsOffButAuthorizationExists(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		honor:      false,
		authorized: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})
	require.NotNil(t, r.blackholeCfg.Load(), "the peer must be IN the config map for this case to bite")

	change, ok := announce(t, r, peer,
		ipv4Prefix(32, 10, 0, 0, 1),
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))

	require.True(t, ok)
	assert.Equal(t, routetype.Type(0), change.RouteType,
		"a peer that never agreed to honor BLACKHOLE made Ze discard traffic")
	assert.Equal(t, routetype.Type(0),
		blackholeLocRIBType(t, loc, netip.MustParsePrefix("10.0.0.1/32")))
}

// AC-3: RFC 7999 Section 3.3, first condition. The agreement is in force and
// the community is present, and the prefix sits outside every block the peer is
// authorized for.
//
// RFC requirement: RFC7999-3.3-1 negative -- the announced prefix is covered by
// no equal or shorter prefix the neighbor is authorized to advertise, so the
// announcement is not honored even though the session agreed to honor it.
func TestBlackholeNotStampedOutsideAuthorization(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		honor:      true,
		authorized: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})

	change, ok := announce(t, r, peer,
		ipv4Prefix(32, 198, 51, 100, 1),
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))

	require.True(t, ok)
	assert.Equal(t, routetype.Type(0), change.RouteType,
		"the peer blackholed a prefix it was never authorized for")
	assert.Equal(t, routetype.Type(0),
		blackholeLocRIBType(t, loc, netip.MustParsePrefix("198.51.100.1/32")))
}

// AC-4: an untagged route from a fully authorized, opted-in peer forwards. The
// community is what asks for the discard, not the authorization.
func TestBlackholeNotStampedWithoutTheCommunity(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		honor:      true,
		authorized: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})

	change, ok := announce(t, r, peer,
		ipv4Prefix(32, 10, 0, 0, 1),
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, noExportValue))

	require.True(t, ok)
	assert.Equal(t, routetype.Type(0), change.RouteType,
		"an untagged route was discarded")
	assert.Equal(t, routetype.Type(0),
		blackholeLocRIBType(t, loc, netip.MustParsePrefix("10.0.0.1/32")))
}

// A prefix already installed as a forwarding route, then re-announced by the
// same peer with BLACKHOLE added and nothing else changed. Peer, next-hop and
// MED are identical, so the same-best short circuit is exactly what this hits.
// Both rails must still carry the change.
func TestBlackholeStampedOnCommunityOnlyReannounce(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		honor:      true,
		authorized: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})
	nlri := ipv4Prefix(32, 10, 0, 0, 1)
	pfx := netip.MustParsePrefix("10.0.0.1/32")

	first, ok := announce(t, r, peer, nlri, blackholeAttrBytes([4]byte{192, 168, 1, 1}))
	require.True(t, ok, "first announce not detected")
	require.Equal(t, routetype.Type(0), first.RouteType)

	second, ok := announce(t, r, peer, nlri,
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))

	assert.True(t, ok,
		"the community-only re-announce was suppressed: the forked-plugin deployment keeps forwarding")
	assert.Equal(t, routetype.Blackhole, second.RouteType)
	assert.Equal(t, routetype.Blackhole, blackholeLocRIBType(t, loc, pfx),
		"the Loc-RIB kept the forwarding path after the peer blackholed it")
}

// The reverse transition. A peer that withdraws the community must get its
// traffic forwarded again, or a blackhole is permanent once applied.
func TestBlackholeClearedWhenCommunityRemoved(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		honor:      true,
		authorized: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})
	nlri := ipv4Prefix(32, 10, 0, 0, 1)
	pfx := netip.MustParsePrefix("10.0.0.1/32")

	_, ok := announce(t, r, peer, nlri,
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))
	require.True(t, ok)
	require.Equal(t, routetype.Blackhole, blackholeLocRIBType(t, loc, pfx))

	change, ok := announce(t, r, peer, nlri, blackholeAttrBytes([4]byte{192, 168, 1, 1}))

	assert.True(t, ok, "removing the community was suppressed: the discard is permanent")
	assert.Equal(t, routetype.Type(0), change.RouteType)
	assert.Equal(t, routetype.Type(0), blackholeLocRIBType(t, loc, pfx),
		"traffic is still discarded after the peer stopped asking for it")
}

// AC-8: the peer WITHDRAWS the prefix instead of re-announcing it untagged.
//
// This is not the test above with a different verb. Removing the community
// leaves a best path to re-stamp. The Loc-RIB entry survives, and only its
// route type changes. A withdraw leaves NO path, so the entry must leave both
// rails. The event-bus rail must carry BestChangeWithdraw, and the Loc-RIB must
// hold no best at all.
//
// A discard route the FIB keeps after its last path is gone is a permanent
// black hole for a prefix nobody announces. Nothing upstream would ever
// revisit it.
func TestBlackholeRemovedWhenPrefixWithdrawn(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r, loc := blackholeRIB(t, peer, blackholeConfig{
		honor:      true,
		authorized: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})
	fam := family.Family{AFI: 1, SAFI: 1}
	nlri := ipv4Prefix(32, 10, 0, 0, 1)
	pfx := netip.MustParsePrefix("10.0.0.1/32")

	_, ok := announce(t, r, peer, nlri,
		blackholeAttrBytes([4]byte{192, 168, 1, 1}, blackholeValue))
	require.True(t, ok)
	require.Equal(t, routetype.Blackhole, blackholeLocRIBType(t, loc, pfx))

	// The withdraw: the peer's route is gone, and the best path is recomputed
	// with no candidate left.
	require.True(t, r.bgpPeers[peer].Remove(fam, nlri), "the peer route was not there to withdraw")
	change, ok := r.checkBestPathChange(fam, nlri, false, nil)

	require.True(t, ok, "the withdraw produced no best-path change: the FIB is never told")
	assert.Equal(t, ribevents.BestChangeWithdraw, change.Action,
		"event-bus rail: the forked-plugin deployment keeps discarding a withdrawn prefix")
	_, stillBest := loc.Best(fam, pfx)
	assert.False(t, stillBest,
		"Loc-RIB rail: the discard route outlived the announcement that asked for it")
}
