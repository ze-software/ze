// Design: docs/architecture/plugin/rib-storage-design.md -- exact winner identity.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// TestBestOwnerTieIsStable preserves every protocol criterion and then makes
// otherwise equal same-address owners independent of candidate arrival order.
func TestBestOwnerTieIsStable(t *testing.T) {
	address := netip.MustParseAddr("192.0.2.188")
	first := &Candidate{PeerAddr: address.String(), PeerIP: address, PeerOwner: 1701}
	second := &Candidate{PeerAddr: address.String(), PeerIP: address, PeerOwner: 1702}
	for _, candidates := range [][]*Candidate{{first, second}, {second, first}} {
		require.Same(t, first, SelectBest(candidates))
		order := ComparePair(first, second)
		require.Negative(t, order)
		reasonOrder, step, reason := comparePairWithReason(first, second)
		require.Negative(t, reasonOrder)
		require.Equal(t, order, reasonOrder)
		require.Equal(t, BestStepPeerOwner, step)
		require.Equal(t, "peer-owner 1701 vs 1702", reason)
	}
	// Owner identity never outranks a protocol preference or a path identifier.
	second.LocalPref = 200
	require.Same(t, second, SelectBest([]*Candidate{first, second}))
	second.LocalPref = 0
	first.PathID = 1
	require.Same(t, second, SelectBest([]*Candidate{first, second}))
}

// TestCandidateKeepsRetainedOwner checks the comparison fact at its producing
// boundary, rather than looking the address up after the source was retained.
func TestCandidateKeepsRetainedOwner(t *testing.T) {
	r := newTestRIBManager(t)
	source := storage.NewSourceReference(1701, 41, nil)
	defer source.Release()
	var entry storage.RouteEntry
	entry.SetSourceReference(source)
	defer entry.Release()
	address := netip.MustParseAddr("192.0.2.188")
	candidate := r.extractCandidate(family.IPv4Unicast, 1701, address, address.String(), entry)
	require.Equal(t, uint64(1701), candidate.PeerOwner)
}

// TestBestOwnerSlotsAndExactPurge keeps a same-address sibling's referenced
// winner while purging only one owner, and reclaims slots after their last use.
func TestBestOwnerSlotsAndExactPurge(t *testing.T) {
	r := newTestRIBManager(t)
	first := bestPeer{owner: 1701, address: "192.0.2.188"}
	second := bestPeer{owner: 1702, address: first.address}
	firstIndex, ok := r.bestPathInterner.internPeer(first)
	require.True(t, ok)
	secondIndex, ok := r.bestPathInterner.internPeer(second)
	require.True(t, ok)
	require.NotEqual(t, firstIndex, secondIndex)
	require.Equal(t, first.address, r.bestPathInterner.peerAt(firstIndex))
	require.Equal(t, second.address, r.bestPathInterner.peerAt(secondIndex))
	require.Equal(t, first.owner, r.bestPathInterner.ownerAt(firstIndex))
	require.Equal(t, second.owner, r.bestPathInterner.ownerAt(secondIndex))
	firstPrefix := netip.MustParsePrefix("198.18.1.0/24")
	secondPrefix := netip.MustParsePrefix("198.18.2.0/24")
	shards := r.bestPrev.familyShards(family.IPv4Unicast, true)
	for _, winner := range []struct {
		prefix netip.Prefix
		index  uint16
	}{{firstPrefix, firstIndex}, {secondPrefix, secondIndex}} {
		shard := shards.shardFor(winner.prefix)
		shard.mu.Lock()
		shard.store.insert(winner.prefix, nil, bestPrevRecord{rec: packBestPath(0, winner.index, 0, 0)}, "")
		shard.mu.Unlock()
	}
	removed := r.purgeBestPrevForOwner(first)
	require.Len(t, removed[family.IPv4Unicast], 1)
	require.Equal(t, firstPrefix, removed[family.IPv4Unicast][0].Prefix)
	_, present := r.bestPathInterner.peerIdxOf(first)
	require.False(t, present)
	_, present = r.bestPathInterner.peerIdxOf(second)
	require.True(t, present)
	shard := shards.shardFor(secondPrefix)
	shard.mu.RLock()
	remaining, _, present := shard.store.lookup(secondPrefix, nil)
	shard.mu.RUnlock()
	require.True(t, present)
	require.Equal(t, secondIndex, remaining.rec.peerIdx())
	require.Equal(t, second.owner, r.bestPathInterner.ownerAt(secondIndex))
	removed = r.purgeBestPrevForPeer(first.address)
	require.Len(t, removed[family.IPv4Unicast], 1)
	require.Equal(t, secondPrefix, removed[family.IPv4Unicast][0].Prefix)
	require.Empty(t, r.bestPathInterner.peerIdx)
	require.Len(t, r.bestPathInterner.peersFree, 2)
}
