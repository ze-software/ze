package reactor

import (
	"runtime"
	"sync"
	"testing"
)

// VALIDATES: AC-26 — sync.Pool replaced by pool multiplexer with block-backed handles.
// PREVENTS: Unbounded per-buffer make() allocation, GC evicting pool entries unpredictably.

func TestBufMux_GetReturn(t *testing.T) {
	// Get returns a valid handle, Return makes it available again.
	mux := newBufMux(64, 10) // 64 bytes per buffer, 10 buffers per block
	h := mux.Get()
	if h.Buf == nil {
		t.Fatal("Get() returned nil Buf")
	}
	if len(h.Buf) != 64 {
		t.Fatalf("Get() returned buf len %d, want 64", len(h.Buf))
	}

	mux.Return(h)

	// After return, Get() should reuse the same buffer.
	h2 := mux.Get()
	if h2.Buf == nil {
		t.Fatal("Get() after Return() returned nil Buf")
	}
	// Should be from the same block.
	if h2.ID != h.ID {
		t.Fatalf("Get() after Return() returned block ID %d, want %d", h2.ID, h.ID)
	}
}

func TestBufMux_BlockID(t *testing.T) {
	// Handle.ID matches the block it came from. Second block gets ID 1.
	mux := newBufMux(64, 2) // 2 buffers per block — fill fast
	h0a := mux.Get()
	h0b := mux.Get()
	// Block 0 is now full (2/2 in use). Next Get grows block 1.
	h1a := mux.Get()

	if h0a.ID != 0 || h0b.ID != 0 {
		t.Fatalf("first two handles should be block 0, got %d and %d", h0a.ID, h0b.ID)
	}
	if h1a.ID != 1 {
		t.Fatalf("third handle should be block 1, got %d", h1a.ID)
	}

	mux.Return(h0a)
	mux.Return(h0b)
	mux.Return(h1a)
}

func TestBufMux_Grow(t *testing.T) {
	// New block allocated when all existing buffers are in use.
	mux := newBufMux(64, 3)
	handles := make([]BufHandle, 3)
	for i := range handles {
		handles[i] = mux.Get()
	}
	// All 3 from block 0. Block 0 is full.
	if mux.blockCount() != 1 {
		t.Fatalf("after filling block 0: blockCount=%d, want 1", mux.blockCount())
	}

	// This Get() should trigger growth.
	h := mux.Get()
	if mux.blockCount() != 2 {
		t.Fatalf("after growth: blockCount=%d, want 2", mux.blockCount())
	}
	if h.ID != 1 {
		t.Fatalf("new handle should be from block 1, got %d", h.ID)
	}

	mux.Return(h)
	for _, hh := range handles {
		mux.Return(hh)
	}
}

func TestBufMux_AllocatesFromLowest(t *testing.T) {
	// Get() prefers the lowest block with free buffers.
	// This keeps steady-state in low blocks, letting high blocks drain for collapse.
	mux := newBufMux(64, 2) // 2 per block
	h0a := mux.Get()        // block 0
	h0b := mux.Get()        // block 0, now full
	h1a := mux.Get()        // grows block 1, allocates from 1

	// Return one from block 0 — block 0 now has 1 free.
	mux.Return(h0a)

	// Return one from block 1 — block 1 now has 1 free.
	mux.Return(h1a)

	// Both blocks have free buffers. Get() should prefer block 0 (lowest).
	h := mux.Get()
	if h.ID != 0 {
		t.Fatalf("Get() should prefer lowest block, got block %d, want 0", h.ID)
	}

	mux.Return(h)
	mux.Return(h0b)
}

func TestBufMux_GrowAtMaximum(t *testing.T) {
	// When at maximum capacity, Get() returns zero handle.
	mux := newBufMux(64, 2)
	mux.SetMaxBlocks(1) // Only allow 1 block (2 buffers total)
	h0 := mux.Get()
	h1 := mux.Get()

	// Block 0 full, cannot grow. Should return zero handle.
	h := mux.Get()
	if h.Buf != nil {
		t.Fatal("Get() at max should return zero handle (nil Buf)")
	}

	mux.Return(h0)
	mux.Return(h1)
}

func TestBufMux_CollapseHighest(t *testing.T) {
	// Collapse: highest block fully returned + block below has >=50% free.
	mux := newBufMux(64, 4) // 4 per block
	// Fill block 0 → grow block 1.
	b0 := make([]BufHandle, 4)
	for i := range b0 {
		b0[i] = mux.Get()
	}
	b1 := make([]BufHandle, 2)
	for i := range b1 {
		b1[i] = mux.Get()
	}
	if mux.blockCount() != 2 {
		t.Fatalf("should have 2 blocks, got %d", mux.blockCount())
	}

	// Return all of block 1 (highest, fully returned).
	for _, h := range b1 {
		mux.Return(h)
	}
	// Return 3 of 4 from block 0 (75% free >= 50%).
	for i := range 3 {
		mux.Return(b0[i])
	}

	// Force collapse check.
	mux.tryCollapse()

	if mux.blockCount() != 1 {
		t.Fatalf("after collapse: blockCount=%d, want 1", mux.blockCount())
	}

	mux.Return(b0[3])
}

func TestBufMux_CollapseBlockedByLowFree(t *testing.T) {
	// No collapse if block below has less than 50% free.
	mux := newBufMux(64, 4)
	b0 := make([]BufHandle, 4)
	for i := range b0 {
		b0[i] = mux.Get()
	}
	b1 := make([]BufHandle, 2)
	for i := range b1 {
		b1[i] = mux.Get()
	}

	// Return all of block 1 (fully returned).
	for _, h := range b1 {
		mux.Return(h)
	}
	// Return only 1 of 4 from block 0 (25% free < 50%).
	mux.Return(b0[0])

	mux.tryCollapse()

	if mux.blockCount() != 2 {
		t.Fatalf("collapse should be blocked: blockCount=%d, want 2", mux.blockCount())
	}

	for i := 1; i < 4; i++ {
		mux.Return(b0[i])
	}
}

func TestBufMux_CollapseStraggler(t *testing.T) {
	// Block with 1 outstanding buffer is NOT collapsed.
	mux := newBufMux(64, 4)
	b0 := make([]BufHandle, 4)
	for i := range b0 {
		b0[i] = mux.Get()
	}
	b1 := make([]BufHandle, 4)
	for i := range b1 {
		b1[i] = mux.Get()
	}

	// Return 3 of 4 from block 1 (not fully returned).
	for i := range 3 {
		mux.Return(b1[i])
	}
	// Return all of block 0 (100% free).
	for _, h := range b0 {
		mux.Return(h)
	}

	mux.tryCollapse()

	// Block 1 is highest but has 1 outstanding — no collapse.
	if mux.blockCount() != 2 {
		t.Fatalf("straggler should prevent collapse: blockCount=%d, want 2", mux.blockCount())
	}

	mux.Return(b1[3])
}

func TestBufMux_CollapseCascade(t *testing.T) {
	// Collapse cascades through multiple fully-returned blocks.
	mux := newBufMux(64, 2)
	// Create 3 blocks.
	b0 := make([]BufHandle, 2)
	for i := range b0 {
		b0[i] = mux.Get()
	}
	b1 := make([]BufHandle, 2)
	for i := range b1 {
		b1[i] = mux.Get()
	}
	b2 := make([]BufHandle, 2)
	for i := range b2 {
		b2[i] = mux.Get()
	}
	if mux.blockCount() != 3 {
		t.Fatalf("should have 3 blocks, got %d", mux.blockCount())
	}

	// Return everything.
	for _, h := range b2 {
		mux.Return(h)
	}
	for _, h := range b1 {
		mux.Return(h)
	}
	for _, h := range b0 {
		mux.Return(h)
	}

	mux.tryCollapse()

	// All blocks fully returned. Cascade should delete all except the last
	// one standing (which becomes the only block). Actually, if all are
	// fully returned, they can all be deleted since the check requires
	// "block below exists AND has >=50% free". When only one block remains,
	// there is no block below, so collapse stops.
	// With 3 blocks all returned: block 2 (highest) fully returned, block 1
	// has 100% free >= 50% → delete 2. Block 1 (now highest) fully returned,
	// block 0 has 100% free >= 50% → delete 1. Block 0 (now highest) fully
	// returned, no block below → stop.
	if mux.blockCount() != 1 {
		t.Fatalf("cascade should leave 1 block: blockCount=%d, want 1", mux.blockCount())
	}
}

func TestProbedPool_CollapseEveryInterval(t *testing.T) {
	// VALIDATES: AC-27 — collapse piggybacked on normal Get() traffic.
	// PREVENTS: Needing a dedicated timer to reclaim overflow blocks.
	const interval = 10
	pp := withCollapseProbe(newProbedPool(64, 4), interval)

	// Create 2 blocks: fill block 0 (4 buffers), grow block 1 (2 buffers).
	// Counter (in probe closure) after setup: 6.
	b0 := make([]BufHandle, 4)
	for i := range b0 {
		b0[i] = pp.Get()
	}
	b1 := make([]BufHandle, 2)
	for i := range b1 {
		b1[i] = pp.Get()
	}

	// Return all of block 1 + 3 of 4 from block 0.
	// Block 1 fully returned, block 0 has 75% free. Collapse-ready.
	for _, h := range b1 {
		pp.Return(h)
	}
	for i := range 3 {
		pp.Return(b0[i])
	}

	if pp.blockCount() != 2 {
		t.Fatalf("setup: blockCount=%d, want 2", pp.blockCount())
	}

	// 3 more Gets (counter 7-9): no collapse yet.
	for range 3 {
		h := pp.Get()
		pp.Return(h)
	}
	if pp.blockCount() != 2 {
		t.Fatalf("before interval: blockCount=%d, want 2", pp.blockCount())
	}

	// 10th Get triggers collapse via probe's counter.
	h := pp.Get()
	if pp.blockCount() != 1 {
		t.Fatalf("after interval: blockCount=%d, want 1", pp.blockCount())
	}

	pp.Return(h)
	pp.Return(b0[3])
}

func TestProbedPool_ProbeFiresEveryGet(t *testing.T) {
	// VALIDATES: AC-27 — probe fires on every Get, target owns the counter.
	// PREVENTS: Counter living in the wrapper instead of the target.
	pp := newProbedPool(64, 4)

	var probeCount int
	pp.SetProbe(func() { probeCount++ })

	// Probe fires on every Get.
	for range 5 {
		h := pp.Get()
		pp.Return(h)
	}
	if probeCount != 5 {
		t.Fatalf("probe count after 5 Gets: %d, want 5", probeCount)
	}

	// No probe when nil.
	pp.SetProbe(nil)
	h := pp.Get()
	pp.Return(h)
	if probeCount != 5 {
		t.Fatalf("probe count after nil probe: %d, want 5", probeCount)
	}
}

func TestBufMux_ZeroHandleSentinel(t *testing.T) {
	// Zero-value BufHandle has nil Buf — used as "no buffer" sentinel.
	var h BufHandle
	if h.Buf != nil {
		t.Fatal("zero BufHandle should have nil Buf")
	}
	if h.ID != 0 {
		t.Fatal("zero BufHandle should have ID 0")
	}
}

func TestBufMux_ReturnToCorrectBlock(t *testing.T) {
	// Return routes to the correct block by ID, not by buffer size.
	mux := newBufMux(64, 2)

	// Fill block 0, grow block 1.
	h0a := mux.Get()
	h0b := mux.Get()
	h1a := mux.Get()
	h1b := mux.Get()

	// Return from block 0.
	mux.Return(h0a)

	// Return from block 1.
	mux.Return(h1a)

	// Get should prefer lowest (block 0) which has 1 free.
	h := mux.Get()
	if h.ID != 0 {
		t.Fatalf("should get from block 0 (lowest), got block %d", h.ID)
	}

	mux.Return(h)
	mux.Return(h0b)
	mux.Return(h1b)
}

func TestBufMux_DoubleReturnCorruption(t *testing.T) {
	// VALIDATES: AC-26 -- double return must not corrupt the free list.
	// PREVENTS: Two Get() calls returning the same buffer (memory corruption).
	mux := newBufMux(64, 2)
	h := mux.Get()

	mux.Return(h)
	// Second return adds duplicate idx to free list.
	mux.Return(h)

	// Two Gets should NOT return the same buffer pointer.
	h1 := mux.Get()
	h2 := mux.Get()

	if h1.Buf != nil && h2.Buf != nil && &h1.Buf[0] == &h2.Buf[0] {
		t.Fatal("double return allowed two Gets to return the same buffer")
	}

	if h1.Buf != nil {
		mux.Return(h1)
	}
	if h2.Buf != nil {
		mux.Return(h2)
	}
}

func TestBufMux_CollapseBoundary50Percent(t *testing.T) {
	// VALIDATES: AC-26 -- collapse triggers at exactly 50% free (not 49%).
	// PREVENTS: Off-by-one in freeRatio threshold.
	mux := newBufMux(64, 4)
	b0 := make([]BufHandle, 4)
	for i := range b0 {
		b0[i] = mux.Get()
	}
	b1 := make([]BufHandle, 4)
	for i := range b1 {
		b1[i] = mux.Get()
	}

	// Return all of block 1 (fully returned).
	for _, h := range b1 {
		mux.Return(h)
	}
	// Return exactly 2 of 4 from block 0 (50% free, not < 50%).
	mux.Return(b0[0])
	mux.Return(b0[1])

	// 50% free: freeRatio=0.5, condition is < 0.5, so collapse should proceed.
	mux.tryCollapse()
	if mux.blockCount() != 1 {
		t.Fatalf("50%% free should trigger collapse: blockCount=%d, want 1", mux.blockCount())
	}

	mux.Return(b0[2])
	mux.Return(b0[3])
}

// --- Mixed-size BufMux tests (fwd-auto-sizing Phase 2) ---

func TestMixedBufMux_Get4K(t *testing.T) {
	// 4K slice allocated from a 64K block (subdivision).
	m := newMixedBufMux()
	h := m.Get4K()
	if h.Buf == nil {
		t.Fatal("Get4K() returned nil Buf")
	}
	if len(h.Buf) != 4096 {
		t.Fatalf("Get4K() buf len = %d, want 4096", len(h.Buf))
	}
	m.Return(h)
}

func TestMixedBufMux_Get64K(t *testing.T) {
	// Full 64K block allocated for ExtMsg peer.
	m := newMixedBufMux()
	h := m.Get64K()
	if h.Buf == nil {
		t.Fatal("Get64K() returned nil Buf")
	}
	if len(h.Buf) != 65535 {
		t.Fatalf("Get64K() buf len = %d, want 65535", len(h.Buf))
	}
	m.Return(h)
}

func TestMixedBufMux_Mixed(t *testing.T) {
	// 4K and 64K allocations coexist in the same pool.
	m := newMixedBufMux()
	h4 := m.Get4K()
	h64 := m.Get64K()
	if h4.Buf == nil || h64.Buf == nil {
		t.Fatal("mixed allocation returned nil")
	}
	if len(h4.Buf) != 4096 {
		t.Fatalf("4K buf len = %d", len(h4.Buf))
	}
	if len(h64.Buf) != 65535 {
		t.Fatalf("64K buf len = %d", len(h64.Buf))
	}
	m.Return(h4)
	m.Return(h64)
}

func TestMixedBufMux_Return(t *testing.T) {
	// Return releases buffer, subsequent Get succeeds.
	m := newMixedBufMux()
	m.SetByteBudget(4096 * 16) // one block's worth of 4K slices
	// Exhaust the pool.
	handles := make([]BufHandle, 16)
	for i := range handles {
		handles[i] = m.Get4K()
		if handles[i].Buf == nil {
			t.Fatalf("Get4K() #%d returned nil before exhaustion", i)
		}
	}
	// Pool should be exhausted now.
	h := m.Get4K()
	if h.Buf != nil {
		t.Fatal("expected nil after exhaustion")
	}
	// Return one, should be able to get again.
	m.Return(handles[0])
	h = m.Get4K()
	if h.Buf == nil {
		t.Fatal("Get4K() after Return() returned nil")
	}
	m.Return(h)
	for i := 1; i < len(handles); i++ {
		m.Return(handles[i])
	}
}

func TestMixedBufMux_Exhausted(t *testing.T) {
	// Get returns nil when byte budget is reached.
	m := newMixedBufMux()
	m.SetByteBudget(4096 * 16) // exactly one 64K block (16 x 4K slices)
	for range 16 {
		h := m.Get4K()
		if h.Buf == nil {
			t.Fatal("should not exhaust before 16 allocations")
		}
	}
	h := m.Get4K()
	if h.Buf != nil {
		t.Fatal("should be exhausted after 16 x 4K allocations on 64K budget")
	}
}

func TestMixedBufMux_Collapse(t *testing.T) {
	// Fully-returned block collapses, freeing memory.
	m := newMixedBufMux()
	// Allocate enough to grow 2 blocks.
	handles1 := make([]BufHandle, 16)
	for i := range handles1 {
		handles1[i] = m.Get4K()
	}
	// This forces a second block.
	handles2 := make([]BufHandle, 16)
	for i := range handles2 {
		handles2[i] = m.Get4K()
	}
	if m.blockCount() != 2 {
		t.Fatalf("expected 2 blocks, got %d", m.blockCount())
	}
	// Return all of block 2 and most of block 1.
	for _, h := range handles2 {
		m.Return(h)
	}
	for i := range 12 { // 12 of 16 = 75% free
		m.Return(handles1[i])
	}
	m.tryCollapse()
	if m.blockCount() != 1 {
		t.Fatalf("collapse should leave 1 block, got %d", m.blockCount())
	}
	for i := 12; i < 16; i++ {
		m.Return(handles1[i])
	}
}

func TestMixedBufMux_Stats(t *testing.T) {
	// Stats returns block-level byte counts.
	// Get4K subdivides one block, Get64K takes one whole block = 2 active blocks.
	m := newMixedBufMux()
	h4 := m.Get4K()
	h64 := m.Get64K()
	totalBytes, usedBytes := m.Stats()
	// 2 active blocks * 64K = 128K used. Total includes free blocks from chunk growth.
	wantUsed := int64(2 * overflowBlockSize)
	if usedBytes != wantUsed {
		t.Fatalf("usedBytes = %d, want %d", usedBytes, wantUsed)
	}
	// Total >= used (includes free blocks from chunk allocation).
	if totalBytes < usedBytes {
		t.Fatalf("totalBytes %d < usedBytes %d", totalBytes, usedBytes)
	}
	m.Return(h4)
	m.Return(h64)
}

func TestMixedBufMux_CollapseTombstoneReuse(t *testing.T) {
	// Collapse nils blocks, tombstones their slots. Next growth reuses the slots.
	// Verifies block IDs remain stable through the cycle.
	m := newMixedBufMux()
	m.SetByteBudget(overflowChunkBlocks * overflowBlockSize) // exactly 1 chunk

	// Allocate and return a 4K slice to create an active then free block.
	h := m.Get4K()
	if h.Buf == nil {
		t.Fatal("Get4K should succeed")
	}
	blockID := h.ID
	m.Return(h)

	// Block is now free. liveBlocks = chunk size (16).
	m.mu.Lock()
	liveBefore := m.liveBlocks
	freeBefore := len(m.free)
	m.mu.Unlock()

	// Collapse: should nil free blocks, creating tombstones.
	m.tryCollapse()

	m.mu.Lock()
	liveAfter := m.liveBlocks
	tombAfter := len(m.tombstones)
	m.mu.Unlock()

	if liveAfter >= liveBefore {
		t.Fatalf("collapse should reduce liveBlocks: before=%d after=%d", liveBefore, liveAfter)
	}
	if tombAfter < freeBefore {
		t.Fatalf("collapse should create tombstones: free=%d tombstones=%d", freeBefore, tombAfter)
	}

	// Now allocate again -- should reuse tombstoned slots, not grow beyond budget.
	h2 := m.Get4K()
	if h2.Buf == nil {
		t.Fatal("Get4K after collapse should succeed (reuse tombstoned slots)")
	}

	// The new block should have a valid ID within the original slice range.
	m.mu.Lock()
	if int(h2.ID) >= len(m.blocks) || m.blocks[h2.ID] == nil {
		t.Fatalf("new block ID %d should map to a valid slot", h2.ID)
	}
	m.mu.Unlock()

	// Original blockID slot should have been reused or another tombstone was.
	// Either way, the pool is functional.
	_ = blockID
	m.Return(h2)
}

func TestMixedBufMux_ReturnNonActive(t *testing.T) {
	// Return to a free block (double return) should log error, not corrupt state.
	m := newMixedBufMux()
	h := m.Get64K()
	m.Return(h)
	// Block is now free. Second return should be caught.
	m.Return(h) // should log "return to non-active block", not panic or corrupt
	// Pool should still be functional.
	h2 := m.Get4K()
	if h2.Buf == nil {
		t.Fatal("pool should be functional after double return")
	}
	m.Return(h2)
}

func TestMixedBufMux_SubdivTransition(t *testing.T) {
	// Fill one subdivided block (16 slices), then next Get4K opens a new block.
	m := newMixedBufMux()
	handles := make([]BufHandle, 17)
	for i := range 16 {
		handles[i] = m.Get4K()
		if handles[i].Buf == nil {
			t.Fatalf("Get4K #%d returned nil", i)
		}
	}
	// All 16 slices from block 0 are out. Next Get4K must subdivide a new block.
	handles[16] = m.Get4K()
	if handles[16].Buf == nil {
		t.Fatal("Get4K #17 should subdivide a new block")
	}
	if handles[16].ID == handles[0].ID {
		t.Fatal("17th slice should come from a different block than the first 16")
	}
	if m.blockCount() != 2 {
		t.Fatalf("expected 2 active blocks, got %d", m.blockCount())
	}
	for _, h := range handles {
		m.Return(h)
	}
}

func TestMixedBufMux_WholeAndSubdivCoexist(t *testing.T) {
	// Get64K and Get4K from the same pool -- whole and subdivided blocks coexist.
	m := newMixedBufMux()
	h64 := m.Get64K()
	if h64.Buf == nil {
		t.Fatal("Get64K returned nil")
	}
	if len(h64.Buf) != 65535 {
		t.Fatalf("Get64K buf len = %d, want 65535", len(h64.Buf))
	}
	h4 := m.Get4K()
	if h4.Buf == nil {
		t.Fatal("Get4K returned nil")
	}
	if len(h4.Buf) != 4096 {
		t.Fatalf("Get4K buf len = %d, want 4096", len(h4.Buf))
	}
	// Different blocks.
	if h64.ID == h4.ID {
		t.Fatal("whole and subdivided should use different blocks")
	}
	if m.blockCount() != 2 {
		t.Fatalf("expected 2 active blocks, got %d", m.blockCount())
	}
	m.Return(h64)
	m.Return(h4)
}

func TestMixedBufMux_NonSequentialSliceReturn(t *testing.T) {
	// Return 16 slices in reverse order -- bitmask must handle any order.
	m := newMixedBufMux()
	handles := make([]BufHandle, 16)
	for i := range handles {
		handles[i] = m.Get4K()
	}
	if m.blockCount() != 1 {
		t.Fatalf("expected 1 active block, got %d", m.blockCount())
	}
	// Return in reverse.
	for i := 15; i >= 0; i-- {
		m.Return(handles[i])
	}
	// Block should be fully returned and in free list.
	if m.blockCount() != 0 {
		t.Fatalf("expected 0 active blocks after full return, got %d", m.blockCount())
	}
	// Pool should still work.
	h := m.Get4K()
	if h.Buf == nil {
		t.Fatal("Get4K after full return should succeed (reuses free block)")
	}
	m.Return(h)
}

func TestMixedBufMux_BlockReuseCrossMode(t *testing.T) {
	// Key spec property: a block used as 64K, returned, then reused as 4K subdivided.
	m := newMixedBufMux()
	m.SetByteBudget(overflowBlockSize) // 1 block max

	// Use as 64K whole.
	h64 := m.Get64K()
	if h64.Buf == nil {
		t.Fatal("Get64K returned nil")
	}
	blockID := h64.ID
	m.Return(h64)

	// Same block should now be reusable as subdivided 4K.
	h4 := m.Get4K()
	if h4.Buf == nil {
		t.Fatal("Get4K should reuse the freed block")
	}
	if h4.ID != blockID {
		t.Fatalf("expected reuse of block %d, got block %d", blockID, h4.ID)
	}
	if len(h4.Buf) != 4096 {
		t.Fatalf("reused block should give 4K slice, got len %d", len(h4.Buf))
	}
	m.Return(h4)

	// And back to 64K.
	h64b := m.Get64K()
	if h64b.Buf == nil {
		t.Fatal("Get64K should reuse the freed block again")
	}
	if h64b.ID != blockID {
		t.Fatalf("expected reuse of block %d, got block %d", blockID, h64b.ID)
	}
	m.Return(h64b)
}

func TestMixedBufMux_ConcurrentMixed(t *testing.T) {
	// Concurrent Get4K, Get64K, and Return from multiple goroutines.
	m := newMixedBufMux()
	m.SetByteBudget(64 * overflowBlockSize) // 64 blocks = 4MB

	const goroutines = 8
	const opsPerGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for range opsPerGoroutine {
				if id%2 == 0 {
					h := m.Get4K()
					if h.Buf != nil {
						h.Buf[0] = byte(id)
						runtime.Gosched()
						if h.Buf[0] != byte(id) {
							t.Errorf("goroutine %d: 4K buffer corrupted", id)
						}
						m.Return(h)
					}
				} else {
					h := m.Get64K()
					if h.Buf != nil {
						h.Buf[0] = byte(id)
						runtime.Gosched()
						if h.Buf[0] != byte(id) {
							t.Errorf("goroutine %d: 64K buffer corrupted", id)
						}
						m.Return(h)
					}
				}
			}
		}(g)
	}
	wg.Wait()
}

func TestMixedBufMux_GrowthAndCollapseCycle(t *testing.T) {
	// Verify: pool grows under pressure, UsedRatio climbs, collapse returns memory,
	// ratio drops, regrowth reuses tombstoned slots.
	m := newMixedBufMux()
	m.SetByteBudget(4 * overflowChunkBlocks * overflowBlockSize) // 4 chunks = 64 blocks

	// Phase 1: empty pool, ratio = 0.
	r := m.UsedRatio()
	if r != 0.0 {
		t.Fatalf("empty pool ratio = %f, want 0.0", r)
	}

	// Phase 2: allocate 32 x 4K slices -> 2 subdivided blocks -> triggers 1 chunk (16 blocks).
	handles := make([]BufHandle, 32)
	for i := range handles {
		handles[i] = m.Get4K()
		if handles[i].Buf == nil {
			t.Fatalf("Get4K #%d returned nil", i)
		}
	}
	// 1 chunk = 16 live blocks, max = 64 -> ratio = 0.25.
	r = m.UsedRatio()
	if r < 0.2 || r > 0.3 {
		t.Fatalf("after 1 chunk: ratio = %f, want ~0.25", r)
	}

	// Phase 3: allocate 64K handles to exhaust the first chunk's free blocks (14)
	// and force a second chunk growth.
	big := make([]BufHandle, 16) // 14 free + need 2 more -> triggers second chunk
	for i := range big {
		big[i] = m.Get64K()
		if big[i].Buf == nil {
			t.Fatalf("Get64K #%d returned nil", i)
		}
	}
	// 2 chunks = 32 live blocks, max = 64 -> ratio = 0.5.
	r2 := m.UsedRatio()
	if r2 <= r {
		t.Fatalf("ratio should increase after second chunk growth: was %f, now %f", r, r2)
	}

	// Phase 4: return everything.
	for _, h := range handles {
		m.Return(h)
	}
	for _, h := range big {
		m.Return(h)
	}
	// All blocks are free, but still live (memory held). Ratio unchanged.
	rAfterReturn := m.UsedRatio()
	if rAfterReturn != r2 {
		t.Fatalf("return doesn't change ratio (memory still held): was %f, now %f", r2, rAfterReturn)
	}
	if m.blockCount() != 0 {
		t.Fatalf("no active blocks after full return, got %d", m.blockCount())
	}

	// Phase 5: collapse. Should nil free blocks, drop liveBlocks, ratio drops.
	m.tryCollapse()
	rAfterCollapse := m.UsedRatio()
	if rAfterCollapse >= rAfterReturn {
		t.Fatalf("collapse should reduce ratio: was %f, now %f", rAfterReturn, rAfterCollapse)
	}

	// Phase 6: regrow. Should reuse tombstoned slots.
	m.mu.Lock()
	tombsBefore := len(m.tombstones)
	m.mu.Unlock()
	if tombsBefore == 0 {
		t.Fatal("collapse should have created tombstones")
	}

	h := m.Get4K()
	if h.Buf == nil {
		t.Fatal("Get4K after collapse should succeed via tombstone reuse")
	}

	m.mu.Lock()
	tombsAfter := len(m.tombstones)
	m.mu.Unlock()
	if tombsAfter >= tombsBefore {
		t.Fatalf("growth should consume tombstones: before=%d after=%d", tombsBefore, tombsAfter)
	}
	m.Return(h)
}

func TestMixedBufMux_PartialDrainCollapse(t *testing.T) {
	// 64K blocks return fast, subdivided blocks drain slowly.
	// Collapse should free the whole-returned blocks but not touch
	// the partially-drained subdivided block.
	m := newMixedBufMux()

	// Allocate: 16 x 4K (1 subdivided block) + 15 x 64K (whole blocks).
	// All from the same initial chunk of 16 blocks.
	slices := make([]BufHandle, 16)
	for i := range slices {
		slices[i] = m.Get4K()
	}
	wholes := make([]BufHandle, 15)
	for i := range wholes {
		wholes[i] = m.Get64K()
	}
	// 16 active blocks: 1 subdivided + 15 whole.
	if m.blockCount() != 16 {
		t.Fatalf("expected 16 active, got %d", m.blockCount())
	}

	// Return all 64K blocks. They go to free list immediately.
	for _, h := range wholes {
		m.Return(h)
	}
	// 1 active (subdivided, 16 slices still out), 15 free.
	if m.blockCount() != 1 {
		t.Fatalf("expected 1 active after whole returns, got %d", m.blockCount())
	}

	// Collapse: should nil the 15 free blocks, not the 1 active subdivided block.
	m.tryCollapse()

	m.mu.Lock()
	live := m.liveBlocks
	tombs := len(m.tombstones)
	m.mu.Unlock()

	if live != 1 {
		t.Fatalf("after collapse: liveBlocks = %d, want 1 (the subdivided block)", live)
	}
	if tombs != 15 {
		t.Fatalf("after collapse: tombstones = %d, want 15", tombs)
	}
	// The subdivided block is still functional.
	if m.blockCount() != 1 {
		t.Fatalf("subdivided block should still be active, got %d", m.blockCount())
	}

	// Return 8 of 16 slices. Block stays active (8 still out).
	for i := range 8 {
		m.Return(slices[i])
	}
	if m.blockCount() != 1 {
		t.Fatalf("partially drained block should stay active, got %d", m.blockCount())
	}

	// Return remaining 8. Block goes to free list.
	for i := 8; i < 16; i++ {
		m.Return(slices[i])
	}
	if m.blockCount() != 0 {
		t.Fatalf("fully drained block should be free, got %d active", m.blockCount())
	}

	// Pool still works: reuses the last free block.
	h := m.Get4K()
	if h.Buf == nil {
		t.Fatal("pool should work after partial drain cycle")
	}
	m.Return(h)
}

func TestBufMux_ConcurrentGetReturn(t *testing.T) {
	// VALIDATES: AC-26 -- concurrent access from multiple goroutines is safe.
	// PREVENTS: Race conditions in Get/Return under contention.
	mux := newBufMux(64, 4)
	const goroutines = 10
	const opsPerGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for range opsPerGoroutine {
				h := mux.Get()
				if h.Buf == nil {
					t.Errorf("goroutine %d: Get returned nil", id)
					return
				}
				// Write a marker to detect buffer sharing.
				h.Buf[0] = byte(id)
				// Yield to increase race window.
				runtime.Gosched()
				if h.Buf[0] != byte(id) {
					t.Errorf("goroutine %d: buffer corrupted (expected %d, got %d)", id, id, h.Buf[0])
					return
				}
				mux.Return(h)
			}
		}(g)
	}
	wg.Wait()
}
