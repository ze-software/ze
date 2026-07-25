package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestCheckBestPathChange_BGPMultipathECMP verifies that a BGP multipath best
// change (two equal-cost routes for one prefix, distinct next-hops) carries the
// full equal-cost next-hop set on the Loc-RIB Change.ECMP (rib-arch-4). Before
// the fix the producer called SelectBest and mirrored a single Path, so the ECMP
// siblings reached only the `show bgp rib best` display and the FIB installed a
// single next-hop.
//
// VALIDATES: checkBestPathChange runs SelectMultipath and mirrors the sibling
// next-hops onto the Loc-RIB Path.ECMP (surfaced as Change.ECMP), including when
// the best next-hop is unchanged (the same-best short-circuit refreshes it).
// PREVENTS: BGP ECMP collapsing to a single kernel next-hop.
func TestCheckBestPathChange_BGPMultipathECMP(t *testing.T) {
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	r.maximumPaths.Store(4)

	var last locrib.Change
	loc.OnChange(func(c locrib.Change) {
		if c.Kind != locrib.ChangeRemove {
			last = c
		}
	})

	const prefix = "192.168.5.0/24"
	// Two equal-cost routes (matching origin/localpref/aspath) from two peers,
	// each with a distinct next-hop, so SelectMultipath ties them.
	_, _, err := r.handleCommand("request bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", prefix, "origin", "igp", "localpref", "100", "aspath", "65001", "nexthop", "10.0.0.11"})
	require.NoError(t, err)
	_, _, err = r.handleCommand("request bgp rib inject", "", []string{
		"10.0.0.2", "ipv4/unicast", prefix, "origin", "igp", "localpref", "100", "aspath", "65001", "nexthop", "10.0.0.12"})
	require.NoError(t, err)

	// The best next-hop plus the ECMP set must together cover both next-hops.
	all := append([]netip.Addr{last.Best.NextHop}, last.ECMP...)
	assert.ElementsMatch(t, []netip.Addr{
		netip.MustParseAddr("10.0.0.11"), netip.MustParseAddr("10.0.0.12"),
	}, all, "Loc-RIB Change must carry the full BGP ECMP next-hop set")
	assert.Len(t, last.ECMP, 1, "one equal-cost sibling next-hop")
}
