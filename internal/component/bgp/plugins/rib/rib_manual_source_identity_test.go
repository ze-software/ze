// Design: docs/architecture/plugin/rib-storage-design.md -- manual and learned source isolation.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestManualRIBInjectionKeepsForwardingWithoutAuthority exercises the operator
// command and real system routing table for an address with no configured Peer.
// Its useful manual route must not create authenticated learned-source state.
func TestManualRIBInjectionKeepsForwardingWithoutAuthority(t *testing.T) {
	r := newTestRIBManager(t)
	manualSourceCleanup(t, r)
	nextHop := netip.MustParseAddr("10.0.0.1")
	loc := electionFixtureLocRIB(t, r, nextHop)
	prefix := netip.MustParsePrefix("198.18.171.0/24")
	peer := "192.0.2.171"
	status, _, err := r.handleCommand("request bgp rib inject", "", []string{
		peer, family.IPv4Unicast.String(), prefix.String(), "nhop", nextHop.String(),
	})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	best, present := loc.Best(family.IPv4Unicast, prefix)
	require.True(t, present, "a manual source still supplies useful forwarding")
	require.Equal(t, nextHop, best.NextHop)
	require.Empty(t, r.bgpPeers, "a synthetic manual source must not become an authenticated Peer owner")
	require.Nil(t, r.sourceLeases, "passive manual input creates no source-lease service")
	rows := requirePeerRoutes(t, anyToJSONStr(t, r.showPipeline("*", []string{"received"})), peer)
	require.Len(t, rows, 1, "the manual route remains visible through the existing show command")
	status, _, err = r.handleCommand("request bgp rib withdraw", "", []string{peer, family.IPv4Unicast.String(), prefix.String()})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	_, present = loc.Best(family.IPv4Unicast, prefix)
	require.False(t, present, "manual withdrawal removes its forwarding contribution")
}

// TestManualRIBInjectionDoesNotReplaceLearnedOwner inserts and withdraws a
// manual route at the same address/prefix as an actually leased learned route.
// The original owner, receipt and useful route must survive both operations.
func TestManualRIBInjectionDoesNotReplaceLearnedOwner(t *testing.T) {
	r, calls := sourceLeaseTestRIB(t, true)
	// Stop table observers before either fixture releases its own route stores.
	defer r.SetLocRIB(nil)
	t.Cleanup(func() {
		for _, routes := range r.ribInPool[bgpProtocolID] {
			routes.Release()
		}
	})
	peer := netip.MustParseAddr("192.0.2.172")
	const owner, receipt uint64 = 1701, 172
	fam, raw := sourceLeaseReceive(t, r, peer, owner, receipt, false, false)
	assertLearned := func() {
		t.Helper()
		count := 0
		for _, routes := range r.bgpPeers {
			entry, found := routes.Lookup(fam, raw)
			if !found {
				continue
			}
			if entry.SourcePeerOwner() != owner {
				continue
			}
			require.Equal(t, receipt, entry.SourceReceipt)
			count++
		}
		require.Equal(t, 1, count, "the original learned candidate remains independently stored")
		require.Zero(t, calls.count(rpc.SourceLeaseRelease, receipt), "a manual operation cannot retire the learned source")
	}
	assertLearned()
	status, _, err := r.handleCommand("request bgp rib inject", "", []string{
		peer.String(), fam.String(), "10.0.0.0/8", "nhop", "10.0.0.1", "localpref", "200",
	})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	assertLearned()
	status, _, err = r.handleCommand("request bgp rib withdraw", "", []string{peer.String(), fam.String(), "10.0.0.0/8"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	assertLearned()
	loc := r.locRIB.Load()
	require.NotNil(t, loc, "the source fixture's system routing table remains attached")
	best, present := loc.Best(fam, netip.MustParsePrefix("10.0.0.0/8"))
	require.True(t, present, "withdrawing the manual contribution preserves useful learned forwarding")
	require.Equal(t, netip.MustParseAddr("10.0.0.1"), best.NextHop)
}

// manualSourceCleanup releases only the route stores this fixture creates.
// The finite current store population bounds the deduplication and both loops.
func manualSourceCleanup(t *testing.T, r *RIBManager) {
	t.Helper()
	t.Cleanup(func() {
		seen := make(map[*storage.PeerRIB]struct{})
		for _, routes := range r.bgpPeers {
			seen[routes] = struct{}{}
		}
		for _, routes := range r.ribInPool[bgpProtocolID] {
			seen[routes] = struct{}{}
		}
		for routes := range seen {
			routes.Release()
		}
	})
}
