package rib

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestBestPrevShardCountClamps exercises the env-driven shard count for the
// BGP plugin's bestPrev sharded store.
func TestBestPrevShardCountClamps(t *testing.T) {
	defer env.ResetCache()

	require.NoError(t, env.Set("ze.bgp.rib.bestprev.shards", "0"))
	env.ResetCache()
	assert.Equal(t, bestPrevMinShards, bestPrevShardCount(), "0 must clamp up to bestPrevMinShards")

	require.NoError(t, env.Set("ze.bgp.rib.bestprev.shards", "999"))
	env.ResetCache()
	assert.Equal(t, bestPrevMaxShards, bestPrevShardCount(), "999 must clamp down to bestPrevMaxShards")

	require.NoError(t, env.Set("ze.bgp.rib.bestprev.shards", "4"))
	env.ResetCache()
	assert.Equal(t, 4, bestPrevShardCount(), "in-range value passes through")

	require.NoError(t, env.Set("ze.bgp.rib.bestprev.shards", ""))
	env.ResetCache()
}

// TestBestPrevShardIndexDeterministic confirms repeat hashing of the same
// prefix lands on the same shard.
func TestBestPrevShardIndexDeterministic(t *testing.T) {
	pfx := netip.MustParsePrefix("172.16.0.0/16")
	first := bestPrevShardIndex(pfx, 16)
	for range 1000 {
		assert.Equal(t, first, bestPrevShardIndex(pfx, 16))
	}
}

// TestParallelCheckBestPathChangeNoLostWrites stresses checkBestPathChange
// from N goroutines, each driving M distinct prefixes through a single
// peer's PeerRIB. The test asserts total recorded best entries equal the
// number of distinct prefixes inserted: per-shard locks and the now-
// concurrent bestPathInterner survive parallel writes.
//
// VALIDATES: bestPrevShards + bestPrevInterner are safe under concurrent
// callers, no records dropped due to lock-ordering or interner races.
// PREVENTS: a regression where the sharded bestPrev or the per-table-locked
// interner silently drops a record under concurrent first-sightings.
//
// Note on locking: this test intentionally does NOT hold r.mu across the
// checkBestPathChange call, even though the production caller in
// rib_structured.go does. The point is to stress the NEW per-shard
// locking and the interner's own mutexes under real concurrency. The
// peer-keyed maps (ribInPool, peerMeta) are populated before the
// goroutines launch and never mutated afterwards, so gatherCandidates /
// bestCandidateNextHopAddr's lockless reads on r.ribInPool are safe
// here even without r.mu. Do NOT copy this pattern into production --
// rib_structured.go must continue to hold r.mu around the surrounding
// peerRIB.Insert / peerRIB.Remove work.
func TestParallelCheckBestPathChangeNoLostWrites(t *testing.T) {
	const (
		goroutines = 4
		perRoutine = 64
	)
	r := newTestRIBManager(t)
	fam := family.Family{AFI: 1, SAFI: 1}
	peerAddr := "10.0.0.1"
	peerRIB := storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr] = peerRIB
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range perRoutine {
				prefix := []byte{32, 10, byte(g), byte(i >> 8), byte(i)}
				attrs := makeAttrBytes([4]byte{10, byte(g), byte(i >> 8), byte(i)})
				r.mu.Lock()
				peerRIB.Insert(fam, attrs, prefix)
				r.mu.Unlock()
				_, ok := r.checkBestPathChange(fam, prefix, false, nil)
				if !ok {
					t.Errorf("checkBestPathChange returned (zero, false) for g=%d i=%d", g, i)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	depths := r.bestPrev.shardDepth(fam)
	total := 0
	for _, d := range depths {
		total += d
	}
	assert.Equal(t, goroutines*perRoutine, total, "every distinct prefix must record one bestPathRecord")
}
