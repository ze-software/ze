package attrpool

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompactionReclaimsDeadSpace verifies compaction frees dead entry space.
//
// VALIDATES: Memory reclamation through compaction.
//
// PREVENTS: Unbounded memory growth as routes are withdrawn and
// re-announced, leaving dead entries consuming space forever.
func TestCompactionReclaimsDeadSpace(t *testing.T) {
	p := New(1024)

	// Create entries
	h1 := mustIntern(t, p, []byte("AAAA"))
	h2 := mustIntern(t, p, []byte("BBBB"))
	h3 := mustIntern(t, p, []byte("CCCC"))

	_ = h1
	_ = h3

	// Release middle entry (creates dead space)
	_ = p.Release(h2)

	// Record metrics before compaction
	before := p.Metrics()
	require.Greater(t, before.DeadSlots, int32(0), "should have dead slots before compaction")

	// Force compaction
	p.Compact()

	// Record metrics after
	after := p.Metrics()

	require.Less(t, after.DeadSlots, before.DeadSlots,
		"compaction must reduce dead slots")
}

// TestCompactionPreservesLiveData verifies handles remain valid after compaction.
//
// VALIDATES: Handle stability guarantee across compaction.
//
// PREVENTS: Data corruption where compaction moves data but handles
// still point to old offsets, causing Get() to return garbage.
func TestCompactionPreservesLiveData(t *testing.T) {
	p := New(1024)

	h1 := mustIntern(t, p, []byte("AAAA"))
	h2 := mustIntern(t, p, []byte("BBBB"))
	h3 := mustIntern(t, p, []byte("CCCC"))

	// Release middle entry
	_ = p.Release(h2)

	// Compact
	p.Compact()

	// Remaining handles must still work
	d1, err := p.Get(h1)
	require.NoError(t, err)
	require.Equal(t, []byte("AAAA"), d1, "h1 must survive compaction")
	d3, err := p.Get(h3)
	require.NoError(t, err)
	require.Equal(t, []byte("CCCC"), d3, "h3 must survive compaction")
}

// TestCompactionMultipleRounds verifies repeated compaction works correctly.
//
// VALIDATES: Repeated compaction doesn't corrupt state.
//
// PREVENTS: State corruption from multiple compaction cycles.
func TestCompactionMultipleRounds(t *testing.T) {
	p := New(1024)

	// Round 1: create and release
	h1 := mustIntern(t, p, []byte("round1"))
	_ = p.Release(h1)
	p.Compact()

	// Round 2: create and release
	h2 := mustIntern(t, p, []byte("round2"))
	d2, err := p.Get(h2)
	require.NoError(t, err)
	require.Equal(t, []byte("round2"), d2)
	_ = p.Release(h2)
	p.Compact()

	// Round 3: create and keep
	h3 := mustIntern(t, p, []byte("round3"))
	p.Compact()
	d3, err := p.Get(h3)
	require.NoError(t, err)
	require.Equal(t, []byte("round3"), d3)
}

// TestCompactionWithNoDeadEntries verifies compaction on clean pool is safe.
//
// VALIDATES: No-op compaction doesn't corrupt state.
//
// PREVENTS: Panic or corruption when compacting pool with no dead entries.
func TestCompactionWithNoDeadEntries(t *testing.T) {
	p := New(1024)

	h1 := mustIntern(t, p, []byte("live1"))
	h2 := mustIntern(t, p, []byte("live2"))

	// Compact with no dead entries
	p.Compact()

	d1, err := p.Get(h1)
	require.NoError(t, err)
	require.Equal(t, []byte("live1"), d1)
	d2, err := p.Get(h2)
	require.NoError(t, err)
	require.Equal(t, []byte("live2"), d2)
}

// TestCompactionEmptyPool verifies compaction on empty pool is safe.
//
// VALIDATES: Compaction handles edge case of empty pool.
//
// PREVENTS: Panic on compacting empty pool.
func TestCompactionEmptyPool(t *testing.T) {
	p := New(1024)

	// Compact empty pool
	require.NotPanics(t, func() {
		p.Compact()
	})

	// Pool should still be usable
	h := mustIntern(t, p, []byte("after-compact"))
	d, err := p.Get(h)
	require.NoError(t, err)
	require.Equal(t, []byte("after-compact"), d)
}

// TestCompactionAllDead verifies compaction when all entries are dead.
//
// VALIDATES: Full cleanup works correctly.
//
// PREVENTS: Corruption when removing all entries.
func TestCompactionAllDead(t *testing.T) {
	p := New(1024)

	h1 := mustIntern(t, p, []byte("dead1"))
	h2 := mustIntern(t, p, []byte("dead2"))
	h3 := mustIntern(t, p, []byte("dead3"))

	_ = p.Release(h1)
	_ = p.Release(h2)
	_ = p.Release(h3)

	p.Compact()

	m := p.Metrics()
	require.Equal(t, int32(0), m.LiveSlots, "no live slots after full release")
	require.Equal(t, int32(0), m.DeadSlots, "no dead slots after compaction")

	// Pool should still be usable
	h := mustIntern(t, p, []byte("new-after-all-dead"))
	d, err := p.Get(h)
	require.NoError(t, err)
	require.Equal(t, []byte("new-after-all-dead"), d)
}

// TestCompactionPerShardValidHandles verifies that incremental compaction runs
// across every shard and that handles in all shards remain valid throughout the
// migration (AC-6).
//
// VALIDATES: AC-6 - per-shard incremental compaction keeps handles valid.
//
// PREVENTS: A handle in one shard being invalidated while another shard migrates.
func TestCompactionPerShardValidHandles(t *testing.T) {
	p := New(1024 * 1024)

	// Spread live and soon-dead entries across all shards.
	const perShard = 8
	live := make(map[Handle][]byte)
	for i := range numShards * perShard {
		data := fmt.Appendf(nil, "compact-shard-%05d", i)
		h := mustIntern(t, p, data)
		if i%2 == 0 {
			// Create dead space in roughly half the slots of every shard.
			require.NoError(t, p.Release(h))
		} else {
			live[h] = data
		}
	}

	before := p.Metrics()
	require.Greater(t, before.DeadSlots, int32(0), "dead slots should exist before compaction")

	// Drive incremental compaction across all shards.
	p.StartCompaction()
	require.Equal(t, PoolCompacting, p.State(), "every shard should be compacting")

	for !p.MigrateBatch(4) {
		// Live handles must stay valid mid-migration, in every shard. Their old
		// buffer stays pinned (refcount > 0), so reads remain correct.
		for h, want := range live {
			got, err := p.Get(h)
			require.NoError(t, err, "handle must stay valid during compaction")
			require.Equal(t, want, got)
		}
	}

	// Migration complete: live handles still resolve, dead space reclaimed.
	for h, want := range live {
		got, err := p.Get(h)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	mid := p.Metrics()
	require.Less(t, mid.DeadSlots, before.DeadSlots, "migration reclaimed dead slots in every shard")

	// Live handles pin their shard's old buffer, so each shard finishes only
	// once its handles drain. Release them and the shards complete.
	for h := range live {
		require.NoError(t, p.Release(h))
	}
	p.CheckOldBufferRelease()
	require.Equal(t, PoolNormal, p.State(), "compaction completes on every shard once handles drain")
}

// TestInternReuseDuringCompactionKeepsData drives the exact interleaving where a
// slot freed before compaction is reused by Intern WHILE compaction is mid-flight,
// at a slot index migration has not yet reached. The reused entry's bytes live in
// the new (current) buffer, but its stale old-buffer offset still points at the
// previous occupant. Migration must not copy from that stale old offset and clobber
// the live new-buffer offset.
//
// VALIDATES: free-list reuse during incremental compaction does not migrate a
// reused slot from stale old-buffer offset data.
//
// PREVENTS: Get(reusedHandle) returning the previous occupant's bytes (silent
// attribute corruption) after migration walks past the reused slot.
//
// The interleaving is driven deterministically via same-package access to the
// shard's compaction state and free list — no scheduler, no timing.
func TestInternReuseDuringCompactionKeepsData(t *testing.T) {
	// this test exercises the release-only reuse-during-compaction
	// path; debug builds disable slot reuse as the ABA guard
	// (spec-unify-buffer-lifetime), so the precondition is deliberately absent
	// there. It still runs and must pass in the default (release) build. See
	// docs/architecture/memory/lifetime-contracts.md.
	if !slotReuseEnabled {
		t.Skip("slot reuse disabled in debug builds (ABA guard)")
	}
	// Single shard so every payload lands in shard 0 and slot indices are
	// deterministic: A=0, B=1, C=2.
	p, err := NewWithShards(7, 1024, 1)
	require.NoError(t, err)
	s := &p.shards[0]

	hA := mustIntern(t, p, []byte("AAAA"))
	hB := mustIntern(t, p, []byte("BBBB"))
	hC := mustIntern(t, p, []byte("CCCC"))
	require.Equal(t, uint32(0), hA.shardSlot())
	require.Equal(t, uint32(1), hB.shardSlot())
	require.Equal(t, uint32(2), hC.shardSlot())

	// Release B: slot 1 becomes dead and is pushed onto the free list.
	require.NoError(t, p.Release(hB))
	require.Equal(t, []uint32{1}, s.freeSlots, "slot 1 must be free for reuse")

	// Begin incremental compaction. cursor=0, compactSlotCount=3, new buffer is
	// the flipped (currently empty) side. Migration has not advanced yet.
	p.StartCompaction()
	require.Equal(t, PoolCompacting, s.state)
	require.Equal(t, uint32(0), s.compactCursor)
	require.Equal(t, uint32(3), s.compactSlotCount)

	// Reuse slot 1 via Intern BEFORE migration reaches it. The new entry's bytes
	// are written into the new (current) buffer; slot 1's old-buffer offset is now
	// stale (it still describes where "BBBB" used to live).
	hD := mustIntern(t, p, []byte("DDDD"))
	require.Equal(t, uint32(1), hD.shardSlot(), "Intern must reuse freed slot 1")

	// Sanity: the reused handle reads correctly right now (before migration).
	got, err := p.Get(hD)
	require.NoError(t, err)
	require.Equal(t, []byte("DDDD"), got, "reused slot reads correctly before migration")

	// Drive migration to completion. As the cursor walks past slot 1, the buggy
	// path copies from slot 1's stale old-buffer offset and overwrites the live
	// new-buffer offset, corrupting D into B's old bytes.
	for !p.MigrateBatch(1) {
	}

	// After migration, the reused entry must still read its own data, not the
	// previous occupant's stale bytes.
	got, err = p.Get(hD)
	require.NoError(t, err)
	require.Equal(t, []byte("DDDD"), got, "reused slot must survive migration uncorrupted")

	// Surviving original live handles are unaffected.
	gotA, err := p.Get(hA)
	require.NoError(t, err)
	require.Equal(t, []byte("AAAA"), gotA)
	gotC, err := p.Get(hC)
	require.NoError(t, err)
	require.Equal(t, []byte("CCCC"), gotC)
}

// TestConcurrentAccessDuringCompaction verifies operations work during compaction.
//
// VALIDATES: Availability during maintenance operations.
//
// PREVENTS: BGP session stalls during compaction, causing holdtime
// expiry and session drops.
func TestConcurrentAccessDuringCompaction(t *testing.T) {
	p := New(1024 * 1024)

	// Pre-populate
	handles := make([]Handle, 1000)
	for i := range handles {
		handles[i] = mustIntern(t, p, fmt.Appendf(nil, "data-%04d", i))
	}

	// Release half to create dead space
	for i := 0; i < len(handles); i += 2 {
		_ = p.Release(handles[i])
	}

	var wg sync.WaitGroup

	// Start compaction in background
	wg.Go(func() {
		p.Compact()
	})

	// Concurrent reads during compaction
	for i := 1; i < len(handles); i += 2 {
		wg.Add(1)
		go func(h Handle, expected string) {
			defer wg.Done()
			got, err := p.Get(h)
			require.NoError(t, err)
			require.Equal(t, []byte(expected), got)
		}(handles[i], fmt.Sprintf("data-%04d", i))
	}

	wg.Wait()
}

// TestConcurrentInternDuringCompaction verifies Intern works during compaction.
//
// VALIDATES: New entries can be added during compaction.
//
// PREVENTS: Blocking new route announcements during maintenance.
func TestConcurrentInternDuringCompaction(t *testing.T) {
	p := New(1024 * 1024)

	// Pre-populate and create dead space
	for i := range 100 {
		h := mustIntern(t, p, fmt.Appendf(nil, "pre-%d", i))
		if i%2 == 0 {
			_ = p.Release(h)
		}
	}

	var wg sync.WaitGroup

	// Compaction in background
	wg.Go(func() {
		p.Compact()
	})

	// Concurrent Intern during compaction
	newHandles := make([]Handle, 50)
	for i := range 50 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			newHandles[idx] = mustIntern(t, p, fmt.Appendf(nil, "new-%d", idx))
		}(i)
	}

	wg.Wait()

	// Verify new entries are accessible
	for i, h := range newHandles {
		d, err := p.Get(h)
		require.NoError(t, err)
		require.Equal(t, fmt.Appendf(nil, "new-%d", i), d)
	}
}
