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
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// igpTestSource stands for an interior protocol in the Loc-RIB. A non-BGP path
// that is not MetricRecursive ends the igpcost walk, so its Metric is the
// interior cost of every BGP next hop it covers.
const igpTestSource = 250

// igpRoute is the shape an interior protocol installs: a next hop on an
// interface, so the metric is terminal rather than recursive.
func igpRoute(metric uint32) locrib.Path {
	return locrib.Path{
		Source:    igpTestSource,
		Instance:  1,
		Metric:    metric,
		NextHop:   netip.MustParseAddr("203.0.113.1"),
		Interface: "eth0",
		RouteType: routetype.Unicast,
	}
}

// VALIDATES: spec-fib-depth AC-1 and AC-13. Two iBGP paths without AIGP are
// ranked by the interior cost the Loc-RIB holds for their next hops, and a
// change of that cost alone, with no BGP UPDATE and no direct call into the
// selector, moves the best path through the Loc-RIB change subscription.
// PREVENTS: an interior cost read once at UPDATE time, a reselection wired only
// for AIGP routes, and a test that stubs the distance instead of reading the RIB.
//
// The lookup registered here is the one sysrib registers in production:
// nhResolver.IGPMetric is igpcost.Resolve over the same Loc-RIB.
func TestIGPCostChangeReselectsWithoutAIGP(t *testing.T) {
	loc := locrib.NewRIB()
	r := newRIBManager(nil)
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	igpcost.Set(func(addr netip.Addr) igpcost.Distance { return igpcost.Resolve(loc, addr) })
	t.Cleanup(func() { igpcost.Set(nil) })

	coverA := netip.MustParsePrefix("198.51.100.0/25")
	coverB := netip.MustParsePrefix("198.51.100.128/25")
	nextHopA := netip.MustParseAddr("198.51.100.1")
	nextHopB := netip.MustParseAddr("198.51.100.129")
	loc.Insert(family.IPv4Unicast, coverA, igpRoute(10))
	loc.Insert(family.IPv4Unicast, coverB, igpRoute(20))

	wirePrefix := ipv4Prefix(24, 10, 20, 0)
	for i, peerText := range []string{"192.0.2.1", "192.0.2.2"} {
		peer := netip.MustParseAddr(peerText)
		r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65001}
		routes := storage.NewPeerRIB(peerText)
		r.bgpPeers[peer] = routes
		t.Cleanup(routes.Release)
		// The cheaper next hop rides on the peer the later tie-breakers
		// (BGP Identifier, peer address) would reject, so a winner that
		// ignored interior cost would pick next hop B.
		nextHop := nextHopB.As4()
		if i == 1 {
			nextHop = nextHopA.As4()
		}
		// Same LOCAL_PREF, AS_PATH and ORIGIN, and no AIGP: interior cost is
		// the first criterion that separates them (RFC 4271 Section 9.1.2.2(e)).
		routes.Insert(family.IPv4Unicast, aigpSelectionAttrs(nextHop, nil, 100, 1), wirePrefix)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); r.runAIGPSelection(ctx) }()
	t.Cleanup(func() { cancel(); <-stopped })

	selected := func() netip.Addr {
		path, _, found := loc.LPM(family.IPv4Unicast, netip.MustParseAddr("10.20.0.1"))
		if !found {
			return netip.Addr{}
		}
		if !path.IsBGP {
			return netip.Addr{}
		}
		return path.NextHop
	}
	require.Eventually(t, func() bool { return selected() == nextHopA }, 5*time.Second, 10*time.Millisecond,
		"AC-1: interior cost 10 via %s must beat 20 via %s", nextHopA, nextHopB)

	// AC-13: only the interior route changes, and nothing calls the selector.
	loc.Insert(family.IPv4Unicast, coverA, igpRoute(30))
	require.Eventually(t, func() bool { return selected() == nextHopB }, 5*time.Second, 10*time.Millisecond,
		"AC-13: interior cost to %s rose to 30, the best path must move to %s", nextHopA, nextHopB)

	// The subscription keeps reselecting: it does not fire once and stop.
	loc.Insert(family.IPv4Unicast, coverA, igpRoute(5))
	require.Eventually(t, func() bool { return selected() == nextHopA }, 5*time.Second, 10*time.Millisecond,
		"AC-13: interior cost to %s fell to 5, the best path must return to it", nextHopA)
}

// VALIDATES: RFC 4271 Section 9.1.2.2(e), the unknown-cost case. The RFC skips
// the step only when "the NEXT_HOP hop for a route is reachable, but no cost can
// be determined". The test pins the two facts Ze's ranking rests on: every
// Loc-RIB path that resolves a next hop carries a metric, so a reachable next
// hop always has a cost (zero included), and an unresolved distance therefore
// means the next hop is not reachable through the Loc-RIB, which RFC 4271
// Section 9.1.2 excludes from Phase 2. Such a path loses step (e) to a
// reachable one even when every later tie-breaker favors it.
// PREVENTS: an unresolvable next hop ranked as cost zero, and the skip rule
// applied to an unresolvable next hop, either of which lets the unreachable
// path win on BGP Identifier or peer address.
func TestIGPCostUnresolvedNextHopLosesToReachable(t *testing.T) {
	loc := locrib.NewRIB()
	r := newRIBManager(nil)
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	igpcost.Set(func(addr netip.Addr) igpcost.Distance { return igpcost.Resolve(loc, addr) })
	t.Cleanup(func() { igpcost.Set(nil) })

	reachable := netip.MustParseAddr("198.51.100.1")
	unreachable := netip.MustParseAddr("198.51.100.200")
	discarded := netip.MustParseAddr("198.51.100.129")
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/25"), igpRoute(0))
	blackhole := igpRoute(0)
	blackhole.RouteType = routetype.Blackhole
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.128/26"), blackhole)

	// A reachable next hop has a cost even when the metric is zero.
	got := igpcost.Resolve(loc, reachable)
	require.True(t, got.Resolved, "a next hop covered by a Loc-RIB path is reachable with a cost")
	require.Zero(t, got.Cost)
	require.False(t, igpcost.Resolve(loc, unreachable).Resolved, "no covering route: not reachable")
	require.False(t, igpcost.Resolve(loc, discarded).Resolved, "a discard route does not reach the next hop")

	wirePrefix := ipv4Prefix(24, 10, 30, 0)
	// The unresolvable next hops ride on the lower peer addresses, which the
	// final tie-breaker prefers, so only step (e) can make the reachable path win.
	for _, entry := range []struct {
		peer    string
		nextHop netip.Addr
	}{
		{"192.0.2.1", unreachable},
		{"192.0.2.2", discarded},
		{"192.0.2.3", reachable},
	} {
		peer := netip.MustParseAddr(entry.peer)
		r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65001}
		routes := storage.NewPeerRIB(entry.peer)
		r.bgpPeers[peer] = routes
		t.Cleanup(routes.Release)
		routes.Insert(family.IPv4Unicast, aigpSelectionAttrs(entry.nextHop.As4(), nil, 100, 1), wirePrefix)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); r.runAIGPSelection(ctx) }()
	t.Cleanup(func() { cancel(); <-stopped })

	selected := func() netip.Addr {
		path, _, found := loc.LPM(family.IPv4Unicast, netip.MustParseAddr("10.30.0.1"))
		if !found {
			return netip.Addr{}
		}
		if !path.IsBGP {
			return netip.Addr{}
		}
		return path.NextHop
	}
	require.Eventually(t, func() bool { return selected() == reachable }, 5*time.Second, 10*time.Millisecond,
		"the path via reachable %s must beat paths via unresolvable %s and %s", reachable, unreachable, discarded)
}
