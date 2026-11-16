// Design: docs/architecture/route-selection.md -- ordinary and prospective next-hop resolution
// Related: addpath_best_per_prefix_test.go, bestchange_alloc_test.go, bestprev_shard_test.go, config_tx_reselect_test.go, config_tx_test.go

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// electionFixtureLocRIB supplies independent terminal next-hop routes for an
// election fixture. Ordinary cost lookup and prospective admission use the same
// real table. A connected host route also remains preferred when an offered BGP
// destination equals its next hop, preserving the concurrency fixtures' inputs.
// Tests using this process-wide cost hook MUST NOT run in parallel.
func electionFixtureLocRIB(t testing.TB, r *RIBManager, hops ...netip.Addr) *locrib.RIB {
	t.Helper()
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	igpcost.Set(func(addr netip.Addr) igpcost.Distance { return igpcost.Resolve(loc, addr) })
	t.Cleanup(func() {
		// SetLocRIB stops its observability workers before the resolver is cleared.
		r.SetLocRIB(nil)
		igpcost.Set(nil)
	})
	connected := redistevents.RegisterProtocol("connected")
	for _, hop := range hops {
		fam := family.IPv4Unicast
		if hop.Is6() {
			fam = family.IPv6Unicast
		}
		loc.Insert(fam, netip.PrefixFrom(hop, hop.BitLen()), locrib.Path{
			Source: connected, Interface: "election-fixture0",
		})
		distance := igpcost.Resolve(loc, hop)
		require.True(t, distance.Resolved, "independent connected next-hop prerequisite: %s", hop)
		require.Zero(t, distance.Cost)
	}
	return loc
}
