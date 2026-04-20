package rib

import (
	"fmt"
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
// Locking mirrors production: no outer lock across the hot path.
// peerRIB.Insert uses PeerRIB's own lock; checkBestPathChange acquires
// r.peerMu.RLock internally for its brief map read of ribInPool.
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
				peerRIB.Insert(fam, attrs, prefix)
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

// TestConcurrentDownVsUpdate races a concurrent peer-down against an
// in-flight UPDATE to assert the new peerMu split survives the window
// between Phase 1 release and Phase 3 completion. The DOWN path
// (peerRIB.Release + delete(ribInPool, peer)) may interleave between the
// UPDATE's Phase 1 and Phase 3; writes in Phase 2 to the local peerRIB
// pointer then land on an orphan PeerRIB.
//
// VALIDATES: neither side crashes, no deadlock, no data race detected by
// -race. Final state is consistent: either the peer exists with its RIB
// non-empty, or it is absent (cleared by DOWN).
// PREVENTS: a regression that re-introduces a coarse lock serializing
// DOWN behind in-flight UPDATEs, or an unsafe access to a released
// PeerRIB's internal state.
func TestConcurrentDownVsUpdate(t *testing.T) {
	const iterations = 128
	r := newTestRIBManager(t)
	fam := family.Family{AFI: 1, SAFI: 1}
	peerAddr := "10.0.0.42"

	var wg sync.WaitGroup
	wg.Add(2)

	// UPDATE-side: mimic handleReceivedStructured's Phase 1 -> Phase 2 flow
	// without the full structured-event plumbing. Each iteration lazily
	// creates the peer slot, writes a route, then triggers best-path.
	go func() {
		defer wg.Done()
		for i := range iterations {
			r.peerMu.Lock()
			peerRIB := r.ribInPool[peerAddr]
			if peerRIB == nil {
				peerRIB = storage.NewPeerRIB(peerAddr)
				r.ribInPool[peerAddr] = peerRIB
			}
			r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}
			r.peerMu.Unlock()

			prefix := []byte{32, 10, 0, 0, byte(i)}
			attrs := makeAttrBytes([4]byte{10, 0, 0, byte(i)})
			peerRIB.Insert(fam, attrs, prefix)
			r.checkBestPathChange(fam, prefix, false, nil)
		}
	}()

	// DOWN-side: mimic handleStructuredState's non-retained DOWN path.
	go func() {
		defer wg.Done()
		for range iterations {
			r.peerMu.Lock()
			if peerRIB := r.ribInPool[peerAddr]; peerRIB != nil {
				peerRIB.Release()
				delete(r.ribInPool, peerAddr)
			}
			delete(r.peerMeta, peerAddr)
			r.peerMu.Unlock()
		}
	}()

	wg.Wait()

	// The race's semantics are "eventually consistent", so exact prefix
	// counts cannot be asserted. What MUST hold: the peer-keyed maps
	// agree with each other -- both present (peer up) or both absent
	// (peer cleared). An orphan peerMeta without a ribInPool entry (or
	// vice versa) would indicate a forgotten map update in one of the
	// paths.
	r.peerMu.RLock()
	_, hasRIB := r.ribInPool[peerAddr]
	_, hasMeta := r.peerMeta[peerAddr]
	r.peerMu.RUnlock()
	assert.Equal(t, hasRIB, hasMeta, "ribInPool and peerMeta must be consistent for this peer")
}

// TestParallelMultiPeerNoLostWrites is the stress test the design's step 5
// calls for: N peer goroutines each processing their own UPDATE stream in
// parallel. Each goroutine lazily creates its own peer slot (brief
// peerMu.Lock), then runs the UPDATE processing flow with no outer lock.
// The test asserts every (peer, prefix) pair was recorded in bestPrev
// exactly once.
//
// VALIDATES: the r.peerMu split lets multiple peer goroutines run the
// UPDATE hot path concurrently. Race detector catches any forgotten
// lock on peer-keyed maps.
// PREVENTS: a regression that re-introduces a coarse outer lock across
// the whole UPDATE handler, collapsing the sharding benefit.
func TestParallelMultiPeerNoLostWrites(t *testing.T) {
	const (
		peers      = 4
		perRoutine = 128
	)
	r := newTestRIBManager(t)
	fam := family.Family{AFI: 1, SAFI: 1}

	var wg sync.WaitGroup
	for p := range peers {
		wg.Add(1)
		peerAddr := fmt.Sprintf("10.0.0.%d", p+1)
		go func(p int, peerAddr string) {
			defer wg.Done()

			// Phase 1: brief lock to create this peer's slot. Matches
			// rib_structured.go::handleReceivedStructured.
			r.peerMu.Lock()
			peerRIB := storage.NewPeerRIB(peerAddr)
			r.ribInPool[peerAddr] = peerRIB
			r.peerMeta[peerAddr] = &PeerMeta{PeerASN: uint32(65000 + p + 1), LocalASN: 65000}
			r.peerMu.Unlock()

			// Phase 2+3: PeerRIB.Insert uses its own lock; checkBestPathChange
			// takes peerMu.RLock internally for map reads.
			for i := range perRoutine {
				prefix := []byte{32, 172, byte(p), byte(i >> 8), byte(i)}
				attrs := makeAttrBytes([4]byte{172, byte(p), byte(i >> 8), byte(i)})
				peerRIB.Insert(fam, attrs, prefix)
				_, ok := r.checkBestPathChange(fam, prefix, false, nil)
				if !ok {
					t.Errorf("checkBestPathChange returned (zero, false) for peer=%s i=%d", peerAddr, i)
					return
				}
			}
		}(p, peerAddr)
	}
	wg.Wait()

	depths := r.bestPrev.shardDepth(fam)
	total := 0
	for _, d := range depths {
		total += d
	}
	assert.Equal(t, peers*perRoutine, total, "every (peer, prefix) pair must record one bestPathRecord")
}
