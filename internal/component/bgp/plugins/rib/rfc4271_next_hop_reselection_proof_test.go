// Design: docs/architecture/route-selection.md -- routing changes repeat selection
// Related: rfc4271_igp_cost_reselect_test.go -- cost-change counterpart and fixtures
// Related: rib_aigp.go -- production Loc-RIB change subscriber
// RFC: rfc/short/rfc4271.md -- Section 9.1.2, immediate next-hop changes

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

// Observe real distance lookups from candidate extraction, not a direct call
// to the selector. Only the resolver's immediate next hop changes: the BGP
// NEXT_HOP, IGP cost, attributes and received UPDATE count remain unchanged.
// RFC requirement: RFC4271-9.1.2-4 positive -- changing the immediate next hop
// of the interior route, with its metric unchanged and without another BGP
// UPDATE, makes the production change subscriber run candidate selection
// again against the new routing state.
// RFC requirement: RFC4271-9.1.2-4 negative -- the still-reachable BGP route
// survives that reselection with its original received NEXT_HOP and no AIGP;
// the immediate-hop change is not a reason to discard or rewrite every path.
func TestRFC4271ImmediateNextHopChangeRepeatsSelection(t *testing.T) {
	loc := locrib.NewRIB()
	r := newRIBManager(nil)
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	learned := netip.MustParseAddr("198.51.100.1")
	cover := netip.MustParsePrefix("198.51.100.0/24")
	prefix := netip.MustParsePrefix("10.77.0.0/24")
	initial := igpRoute(10)
	loc.Insert(family.IPv4Unicast, cover, initial)

	type observation struct {
		immediate netip.Addr
		cost      uint64
		resolved  bool
	}
	queried := make(chan observation, 16)
	igpcost.Set(func(addr netip.Addr) igpcost.Distance {
		distance := igpcost.Resolve(loc, addr)
		if addr == learned {
			path, _, found := loc.LPM(family.IPv4Unicast, addr)
			if found {
				// A bounded observer never stalls a production selection worker.
				select {
				case queried <- observation{path.NextHop, distance.Cost, distance.Resolved}:
				default:
				}
			}
		}
		return distance
	})
	t.Cleanup(func() { igpcost.Set(nil) })
	peer := netip.MustParseAddr("192.0.2.1")
	r.peerMeta[1] = &peerMetadata{Address: peer, PeerASN: 65001, LocalASN: 65001}
	routes := storage.NewPeerRIB(peer.String())
	r.bgpPeers[1] = routes
	t.Cleanup(routes.Release)
	routes.Insert(family.IPv4Unicast,
		aigpSelectionAttrs(learned.As4(), nil, 100, 1), ipv4Prefix(24, 10, 77, 0))

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		r.runAIGPSelection(ctx)
	}()
	t.Cleanup(func() { cancel(); <-stopped })
	next := func() observation {
		select {
		case result := <-queried:
			return result
		case <-time.After(5 * time.Second):
			t.Fatal("routing change did not reach BGP candidate selection")
			return observation{}
		}
	}

	// Initial selection installs its path, which itself emits one Loc-RIB
	// change. Drain the resulting no-op reselection before changing the IGP
	// route, so an old pending notification cannot satisfy the new assertion.
	for range 2 {
		seen := next()
		require.Equal(t, initial.NextHop, seen.immediate)
		require.Equal(t, uint64(10), seen.cost)
		require.True(t, seen.resolved)
	}
	selected, _, found := loc.LPM(family.IPv4Unicast, prefix.Addr())
	require.True(t, found)
	require.True(t, selected.IsBGP)
	require.Equal(t, learned, selected.NextHop)
	require.False(t, selected.AIGPPresent)

	changed := initial
	changed.NextHop = netip.MustParseAddr("203.0.113.2")
	loc.Insert(family.IPv4Unicast, cover, changed)
	seen := next()
	require.Equal(t, changed.NextHop, seen.immediate)
	require.Equal(t, uint64(10), seen.cost, "the witness changes no IGP cost")
	require.True(t, seen.resolved)
	selected, _, found = loc.LPM(family.IPv4Unicast, prefix.Addr())
	require.True(t, found, "reselection keeps the reachable route")
	require.True(t, selected.IsBGP)
	require.Equal(t, learned, selected.NextHop, "received BGP NEXT_HOP is not the resolved immediate hop")
	require.False(t, selected.AIGPPresent)
}
