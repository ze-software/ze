// Design: docs/architecture/route-selection.md -- next-hop admission and reselection
// Related: rib_commands.go, rib_aigp.go, rfc4271_igp_cost_reselect_test.go
// RFC: rfc/short/rfc4271.md -- Sections 9.1.2 and 9.1.2.1
package rib

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// nextHopReachabilityRIB resolves through the same Loc-RIB the manager publishes
// into, as sysrib does in production. No fabricated resolved bit admits a path.
func nextHopReachabilityRIB(t *testing.T) (*RIBManager, *locrib.RIB) {
	t.Helper()
	r := newTestRIBManagerWithBus(newTestEventBus())
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	igpcost.Set(func(addr netip.Addr) igpcost.Distance { return igpcost.Resolve(loc, addr) })
	t.Cleanup(func() { igpcost.Set(nil) })
	return r, loc
}

// nextHopReachabilityPath stores a received iBGP path before selection starts.
// The caller MUST join any selection worker before the cleanup releases it.
func nextHopReachabilityPath(t *testing.T, r *RIBManager, peerOwner uint64, peer, nextHop netip.Addr, preference uint32, nlri []byte) *storage.PeerRIB {
	t.Helper()
	routes := storage.NewPeerRIB(peer.String())
	r.bgpPeers[peerOwner] = routes
	r.peerMeta[peerOwner] = &peerMetadata{Address: peer, PeerASN: 65001, LocalASN: 65001}
	t.Cleanup(routes.Release)
	routes.Insert(family.IPv4Unicast, aigpSelectionAttrs(nextHop.As4(), nil, preference, 1), nlri)
	return routes
}

// startNextHopReachabilitySelection owns one production selection worker. Its
// cleanup MUST cancel and join it before any stored path or resolver is released.
func startNextHopReachabilitySelection(t *testing.T, r *RIBManager) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		// RFC 4271 Section 9.1.2: routing changes re-run Phase 2 without UPDATEs.
		r.runAIGPSelection(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-stopped
	})
}

// VALIDATES: the unreachable path cannot shadow a reachable alternative even
// when its higher LOCAL_PREF would decide the election before interior cost.
// The reachable zero-cost path must still be installed, rather than both dropped.
// PREVENTS: using maximum IGP cost as the only penalty for an unresolved NEXT_HOP.
// MUTATION: make candidateNextHopResolved admit every path.
// RFC requirement: RFC4271-9.1.2-1 positive -- an unresolved higher-LOCAL_PREF path is excluded before ranking; the reachable alternative reaches the Loc-RIB.
func TestRFC4271UnresolvedNextHopCannotWinOnLocalPref(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	prefix := netip.MustParsePrefix("10.40.0.0/24")
	wirePrefix := ipv4Prefix(24, 10, 40, 0)
	reachable := netip.MustParseAddr("198.51.100.1")
	unreachable := netip.MustParseAddr("198.51.100.129")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/25"), igpRoute(0))
	require.True(t, igpcost.Resolve(loc, reachable).Resolved)
	require.False(t, igpcost.Resolve(loc, unreachable).Resolved)
	nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), unreachable, 200, wirePrefix)
	nextHopReachabilityPath(t, r, 2, netip.MustParseAddr("192.0.2.2"), reachable, 100, wirePrefix)

	// RFC 4271 Section 9.1.2: exclusion precedes the Phase 2 ranking.
	_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
	require.True(t, changed, "the reachable alternative must enter service")
	best, found := loc.Best(family.IPv4Unicast, prefix)
	require.True(t, found, "rejecting the unresolved path must preserve the sound alternative")
	require.True(t, best.IsBGP)
	require.Equal(t, reachable, best.NextHop)
}

// VALIDATES: a lone unresolved path stays available in Adj-RIB-In but cannot
// become the Loc-RIB winner merely because no alternative exists.
// PREVENTS: the single-candidate fast path bypassing reachability admission.
// MUTATION: make candidateNextHopResolved admit every path.
// RFC requirement: RFC4271-9.1.2-1 positive -- the only received path is unresolved and is not selected or installed in the Loc-RIB.
func TestRFC4271OnlyUnresolvedNextHopIsNotInstalled(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	nextHop := netip.MustParseAddr("198.51.100.129")
	wirePrefix := ipv4Prefix(24, 10, 41, 0)
	routes := nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 200, wirePrefix)
	require.False(t, igpcost.Resolve(loc, nextHop).Resolved)

	// RFC 4271 Section 9.1.2: being the only path does not waive admission.
	_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
	require.False(t, changed, "an unresolved path cannot create a selected route")
	_, found := loc.Best(family.IPv4Unicast, netip.MustParsePrefix("10.41.0.0/24"))
	require.False(t, found, "the unresolved route must not enter the Loc-RIB")
	require.Equal(t, 1, routes.Len(), "the received path remains available for later resolution")
}

// VALIDATES: a genuinely reachable next hop with interior cost zero is admitted.
// PREVENTS: treating zero cost as a failed lookup, or rejecting every BGP path.
// MUTATION: make candidateNextHopResolved refuse every path.
// RFC requirement: RFC4271-9.1.2-1 negative -- a resolved zero-cost NEXT_HOP is not excluded and its BGP route is installed.
func TestRFC4271ReachableZeroCostNextHopIsInstalled(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	nextHop := netip.MustParseAddr("198.51.100.1")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/25"), igpRoute(0))
	distance := igpcost.Resolve(loc, nextHop)
	require.True(t, distance.Resolved)
	require.Zero(t, distance.Cost)
	wirePrefix := ipv4Prefix(24, 10, 42, 0)
	nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 100, wirePrefix)

	// RFC 4271 Section 9.1.2: the only feasible path is installed.
	_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
	require.True(t, changed)
	best, found := loc.Best(family.IPv4Unicast, netip.MustParsePrefix("10.42.0.0/24"))
	require.True(t, found)
	require.True(t, best.IsBGP)
	require.Equal(t, nextHop, best.NextHop)
}

// VALIDATES: a next hop covered by a self-recursive or mutually recursive static
// route cannot make a received path feasible; finding a covering prefix is not enough.
// PREVENTS: a longest-prefix match being mistaken for complete resolution.
// MUTATION: make candidateNextHopResolved admit every path.
// RFC requirement: RFC4271-9.1.2-1 positive -- paths whose next hops resolve only through a self or mutual recursion are not installed in the Loc-RIB.
func TestRFC4271RecursiveNextHopLoopIsNotInstalled(t *testing.T) {
	for _, mutual := range []bool{false, true} {
		name := "self"
		if mutual {
			name = "mutual"
		}
		t.Run(name, func(t *testing.T) {
			r, loc := nextHopReachabilityRIB(t)
			nextHop := netip.MustParseAddr("198.51.100.1")
			recursive := igpRoute(0)
			recursive.MetricRecursive = true
			recursive.NextHop = nextHop
			if mutual {
				recursive.NextHop = netip.MustParseAddr("203.0.113.1")
				back := recursive
				back.NextHop = nextHop
				loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("203.0.113.0/24"), back)
			}
			loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), recursive)
			require.False(t, igpcost.Resolve(loc, nextHop).Resolved)
			wirePrefix := ipv4Prefix(24, 10, 44, 0)
			nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 100, wirePrefix)

			// RFC 4271 Section 9.1.2.1: mutual and self recursion fail resolution.
			_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
			require.False(t, changed)
			_, found := loc.Best(family.IPv4Unicast, netip.MustParsePrefix("10.44.0.0/24"))
			require.False(t, found, "a loop in next-hop resolution is not a feasible BGP route")
		})
	}
}

// VALIDATES: resolution through a currently usable less-specific route cannot
// admit a candidate whose own installation would capture that resolution chain.
// PREVENTS: checking only the current routing table, then installing a loop.
// MUTATION: omit the prospective destination check from next-hop admission.
// RFC requirement: RFC4271-9.1.2-1 positive -- a candidate currently resolvable through a less-specific route is excluded when installing it would create direct or mutual recursion.
func TestRFC4271NextHopCannotDependOnItsProspectiveRoute(t *testing.T) {
	for _, indirect := range []bool{false, true} {
		name := "direct"
		if indirect {
			name = "indirect"
		}
		t.Run(name, func(t *testing.T) {
			r, loc := nextHopReachabilityRIB(t)
			loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), igpRoute(0))
			nextHop := netip.MustParseAddr("10.45.0.1")
			if indirect {
				recursive := igpRoute(0)
				recursive.MetricRecursive = true
				recursive.NextHop = nextHop
				loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), recursive)
				nextHop = netip.MustParseAddr("198.51.100.1")
			}
			require.True(t, igpcost.Resolve(loc, nextHop).Resolved,
				"the existing less-specific route resolves the next hop before this candidate is installed")
			wirePrefix := ipv4Prefix(24, 10, 45, 0)
			nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 100, wirePrefix)

			// RFC 4271 Section 9.1.2: installation must not make NEXT_HOP unresolvable.
			_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
			require.False(t, changed, "a route that would make its next hop depend on itself must not be selected")
			_, found := loc.Best(family.IPv4Unicast, netip.MustParsePrefix("10.45.0.0/24"))
			require.False(t, found, "the prospective recursive route must not enter the Loc-RIB")
		})
	}
}

// VALIDATES: recursive static resolution ending at a zero-cost interior route,
// and a more-specific terminal route inside the advertised prefix, remain usable.
// PREVENTS: excluding every recursive next hop or every next hop inside its NLRI.
// MUTATION: refuse recursive resolution or next hops contained in the candidate prefix unconditionally.
// RFC requirement: RFC4271-9.1.2-1 negative -- an independently resolved recursive path and a next hop protected by a longer matching interior route are both installed.
func TestRFC4271IndependentRecursiveNextHopIsInstalled(t *testing.T) {
	for _, longer := range []bool{false, true} {
		name := "recursive-static"
		if longer {
			name = "longer-interior-prefix"
		}
		t.Run(name, func(t *testing.T) {
			r, loc := nextHopReachabilityRIB(t)
			nextHop := netip.MustParseAddr("198.51.100.1")
			if longer {
				nextHop = netip.MustParseAddr("10.46.0.1")
				loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("10.46.0.0/25"), igpRoute(0))
			} else {
				loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("203.0.113.0/24"), igpRoute(0))
				recursive := igpRoute(0)
				recursive.MetricRecursive = true
				loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), recursive)
			}
			require.True(t, igpcost.Resolve(loc, nextHop).Resolved)
			wirePrefix := ipv4Prefix(24, 10, 46, 0)
			nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 100, wirePrefix)

			// RFC 4271 Section 9.1.2.1: independently resolved routes remain feasible.
			_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
			require.True(t, changed)
			best, found := loc.Best(family.IPv4Unicast, netip.MustParsePrefix("10.46.0.0/24"))
			require.True(t, found)
			require.True(t, best.IsBGP)
			require.Equal(t, nextHop, best.NextHop)
		})
	}
}

// VALIDATES: a connected route at the same prefix stays selected in the routing
// table, so admitting the BGP contribution cannot capture its own next hop.
// PREVENTS: rejecting every next hop contained in the received route's prefix.
// MUTATION: treat the proposed BGP path as selected regardless of Loc-RIB ranking.
// RFC requirement: RFC4271-9.1.2-1 negative -- a same-prefix connected winner independently resolves the BGP next hop, so the received BGP contribution is admitted without displacing the connected path.
func TestRFC4271SamePrefixConnectedNextHopRemainsUsable(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	prefix := netip.MustParsePrefix("10.47.0.0/24")
	nextHop := netip.MustParseAddr("10.47.0.1")
	connected := igpRoute(0)
	connected.NextHop = netip.Addr{}
	connected.HasDistanceOverride, connected.DistanceOverride = true, 0
	loc.Insert(family.IPv4Unicast, prefix, connected)
	wirePrefix := ipv4Prefix(24, 10, 47, 0)
	nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHop, 100, wirePrefix)

	// RFC 4271 Section 9.1.2.1: independent direct reachability survives installation.
	_, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
	require.True(t, changed, "the feasible BGP contribution must enter the Loc-RIB")
	admitted := false
	loc.Inspect(family.IPv4Unicast, prefix, func(group locrib.PathGroup) {
		for _, path := range group.Paths {
			if path.Source == bgpProtocolID {
				admitted = true
				require.Equal(t, nextHop, path.NextHop)
			}
		}
	})
	require.True(t, admitted)
	best, found := loc.Best(family.IPv4Unicast, prefix)
	require.True(t, found)
	require.False(t, best.IsBGP, "the connected path still wins the routing table")
	require.Equal(t, connected.Source, best.Source)
}

// VALIDATES: changing only interior reachability replaces an installed winner,
// removes it when neither next hop resolves, and restores it from retained input.
// PREVENTS: stale selected routes after next-hop loss and recovery needing a new UPDATE.
// MUTATION: make candidateNextHopResolved admit every path, or stop reselectAIGPRoutes from processing retained prefixes.
// RFC requirement: RFC4271-9.1.2-4 positive -- removal and restoration of the routes resolving NEXT_HOP re-run selection without any new BGP UPDATE.
// RFC requirement: RFC4271-9.1.2.1-2 positive -- when all received next hops become unresolved, the previously installed BGP path leaves the Loc-RIB.
// RFC requirement: RFC4271-9.1.2.1-2 negative -- a reachable alternative remains installed after the preferred next hop is lost, and restoring reachability installs the retained preferred path again.
func TestRFC4271NextHopLossReselectsAndRestorationRecovers(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)
	prefix := netip.MustParsePrefix("10.43.0.0/24")
	wirePrefix := ipv4Prefix(24, 10, 43, 0)
	coverA := netip.MustParsePrefix("198.51.100.0/25")
	coverB := netip.MustParsePrefix("198.51.100.128/25")
	nextHopA := netip.MustParseAddr("198.51.100.1")
	nextHopB := netip.MustParseAddr("198.51.100.129")
	loc.Insert(family.IPv4Unicast, coverA, igpRoute(0))
	loc.Insert(family.IPv4Unicast, coverB, igpRoute(10))
	pathA := nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), nextHopA, 200, wirePrefix)
	pathB := nextHopReachabilityPath(t, r, 2, netip.MustParseAddr("192.0.2.2"), nextHopB, 100, wirePrefix)
	startNextHopReachabilitySelection(t, r)

	selected := func(want netip.Addr) bool {
		path, found := loc.Best(family.IPv4Unicast, prefix)
		if !found {
			return false
		}
		if !path.IsBGP {
			return false
		}
		return path.NextHop == want
	}
	require.Eventually(t, func() bool { return selected(nextHopA) }, 5*time.Second, 10*time.Millisecond,
		"the reachable higher-LOCAL_PREF path must initially be installed")

	_, removed := loc.Remove(family.IPv4Unicast, coverA, igpTestSource, 1)
	require.True(t, removed)
	require.Eventually(t, func() bool { return selected(nextHopB) }, 5*time.Second, 10*time.Millisecond,
		"losing the preferred next hop must install the reachable alternative without a BGP UPDATE")

	_, removed = loc.Remove(family.IPv4Unicast, coverB, igpTestSource, 1)
	require.True(t, removed)
	require.Eventually(t, func() bool {
		_, found := loc.Best(family.IPv4Unicast, prefix)
		return !found
	}, 5*time.Second, 10*time.Millisecond, "an installed route with no resolving next hop must leave the Loc-RIB")
	require.Equal(t, 1, pathA.Len(), "reachability loss does not discard the preferred received path")
	require.Equal(t, 1, pathB.Len(), "reachability loss does not discard the alternative received path")

	loc.Insert(family.IPv4Unicast, coverA, igpRoute(0))
	require.Eventually(t, func() bool { return selected(nextHopA) }, 5*time.Second, 10*time.Millisecond,
		"restoring next-hop reachability must recover the retained path without a BGP UPDATE")
}
