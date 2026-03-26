package reactor

import (
	"sync"
	"testing"
)

// VALIDATES: AC-27 — combined capacity across 4K + 64K multiplexer instances.
// PREVENTS: One pool exhausting memory while the other has headroom, with no
// shared awareness triggering backpressure.

func TestBufMux_Stats(t *testing.T) {
	// Stats returns correct allocated and inUse counts.
	mux := newBufMux(64, 4)

	alloc, used := mux.Stats()
	if alloc != 0 || used != 0 {
		t.Fatalf("empty mux: allocated=%d, used=%d; want 0, 0", alloc, used)
	}

	// Get one buffer — creates block 0 (4 slots), 1 in use.
	h1 := mux.Get()
	alloc, used = mux.Stats()
	if alloc != 4 || used != 1 {
		t.Fatalf("after 1 Get: allocated=%d, used=%d; want 4, 1", alloc, used)
	}

	// Get three more — fills block 0.
	h2 := mux.Get()
	h3 := mux.Get()
	h4 := mux.Get()
	alloc, used = mux.Stats()
	if alloc != 4 || used != 4 {
		t.Fatalf("after 4 Gets: allocated=%d, used=%d; want 4, 4", alloc, used)
	}

	// Get one more — grows block 1.
	h5 := mux.Get()
	alloc, used = mux.Stats()
	if alloc != 8 || used != 5 {
		t.Fatalf("after growth: allocated=%d, used=%d; want 8, 5", alloc, used)
	}

	// Return some.
	mux.Return(h1)
	mux.Return(h2)
	alloc, used = mux.Stats()
	if alloc != 8 || used != 3 {
		t.Fatalf("after 2 returns: allocated=%d, used=%d; want 8, 3", alloc, used)
	}

	mux.Return(h3)
	mux.Return(h4)
	mux.Return(h5)
}

func TestBufMux_StatsAfterCollapse(t *testing.T) {
	// Stats reflects collapsed blocks.
	mux := newBufMux(64, 4)
	b0 := make([]BufHandle, 4)
	for i := range b0 {
		b0[i] = mux.Get()
	}
	b1 := make([]BufHandle, 2)
	for i := range b1 {
		b1[i] = mux.Get()
	}

	// Return all of block 1, 3/4 of block 0.
	for _, h := range b1 {
		mux.Return(h)
	}
	for i := range 3 {
		mux.Return(b0[i])
	}

	mux.tryCollapse()

	alloc, used := mux.Stats()
	if alloc != 4 || used != 1 {
		t.Fatalf("after collapse: allocated=%d, used=%d; want 4, 1", alloc, used)
	}
	mux.Return(b0[3])
}

func TestProbedPool_Stats(t *testing.T) {
	// probedPool.Stats delegates to underlying BufMux.
	pp := newProbedPool(64, 4)
	alloc, used := pp.Stats()
	if alloc != 0 || used != 0 {
		t.Fatalf("empty pool: allocated=%d, used=%d; want 0, 0", alloc, used)
	}

	h := pp.Get()
	alloc, used = pp.Stats()
	if alloc != 4 || used != 1 {
		t.Fatalf("after 1 Get: allocated=%d, used=%d; want 4, 1", alloc, used)
	}
	pp.Return(h)
}

func TestProbedPool_AddProbe(t *testing.T) {
	// VALIDATES: AC-27 — probes are chainable (collapse + overflow both fire).
	// PREVENTS: SetProbe replacing collapse probe when reactor adds overflow probe.
	pp := newProbedPool(64, 4)

	var calls1, calls2 int
	pp.SetProbe(func() { calls1++ })
	pp.AddProbe(func() { calls2++ })

	h := pp.Get()
	pp.Return(h)

	if calls1 != 1 {
		t.Fatalf("first probe called %d times, want 1", calls1)
	}
	if calls2 != 1 {
		t.Fatalf("second probe called %d times, want 1", calls2)
	}

	// Chain a third.
	var calls3 int
	pp.AddProbe(func() { calls3++ })

	h = pp.Get()
	pp.Return(h)

	if calls1 != 2 || calls2 != 2 || calls3 != 1 {
		t.Fatalf("after third probe: calls=%d,%d,%d; want 2,2,1", calls1, calls2, calls3)
	}
}

func TestProbedPool_AddProbeOnNil(t *testing.T) {
	// AddProbe with no existing probe works.
	pp := newProbedPool(64, 4)

	var called int
	pp.AddProbe(func() { called++ })

	h := pp.Get()
	pp.Return(h)

	if called != 1 {
		t.Fatalf("probe called %d times, want 1", called)
	}
}

func TestBufMux_BudgetDeniesGrowth(t *testing.T) {
	// VALIDATES: AC-27 — budget prevents growth when combined limit exceeded.
	// PREVENTS: One pool growing unbounded while the other is under pressure.
	budget := newCombinedBudget(64 * 2) // budget for exactly 1 block (2 * 64 bytes)
	mux := newBufMux(64, 2)
	mux.SetBudget(budget)

	// First block grows normally (within budget).
	h1 := mux.Get()
	h2 := mux.Get()
	if mux.blockCount() != 1 {
		t.Fatalf("after filling block 0: blockCount=%d, want 1", mux.blockCount())
	}

	// Second block denied — would exceed budget.
	h3 := mux.Get()
	if h3.Buf != nil {
		t.Fatal("Get() should return zero handle when budget exceeded")
	}
	if mux.blockCount() != 1 {
		t.Fatalf("after denied grow: blockCount=%d, want 1", mux.blockCount())
	}

	mux.Return(h1)
	mux.Return(h2)
}

func TestBufMux_NoBudget(t *testing.T) {
	// Nil budget means unlimited growth (existing behavior).
	mux := newBufMux(64, 2)
	h1 := mux.Get()
	h2 := mux.Get()
	h3 := mux.Get() // grows block 1
	if h3.Buf == nil {
		t.Fatal("nil budget should allow unlimited growth")
	}
	mux.Return(h1)
	mux.Return(h2)
	mux.Return(h3)
}

func TestBufMux_BudgetTracksCollapse(t *testing.T) {
	// Budget counter decreases when blocks are collapsed.
	budget := newCombinedBudget(64 * 4) // budget for 2 blocks
	mux := newBufMux(64, 2)
	mux.SetBudget(budget)

	// Fill block 0, grow block 1.
	h0a := mux.Get()
	h0b := mux.Get()
	h1a := mux.Get()

	if budget.AllocatedBytes() != int64(64*2*2) {
		t.Fatalf("after 2 blocks: allocated=%d, want %d", budget.AllocatedBytes(), 64*2*2)
	}

	// Return all of block 1, enough of block 0 for collapse.
	mux.Return(h1a)
	h1b := mux.Get() // still in block 1
	mux.Return(h1b)
	mux.Return(h0a)

	// Collapse block 1.
	mux.tryCollapse()

	if budget.AllocatedBytes() != int64(64*2) {
		t.Fatalf("after collapse: allocated=%d, want %d", budget.AllocatedBytes(), 64*2)
	}

	mux.Return(h0b)
}

func TestBufMux_CombinedStats(t *testing.T) {
	// VALIDATES: AC-27 — combined usage across two multiplexer instances.
	// PREVENTS: Growth/backpressure decisions that only see one pool.
	mux4K := newBufMux(4096, 4)
	mux64K := newBufMux(65535, 4)

	totalBytes, usedBytes := combinedMuxStats(mux4K, mux64K)
	if totalBytes != 0 || usedBytes != 0 {
		t.Fatalf("empty: totalBytes=%d, usedBytes=%d; want 0, 0", totalBytes, usedBytes)
	}

	// Allocate from 4K pool.
	h4K := mux4K.Get()
	totalBytes, usedBytes = combinedMuxStats(mux4K, mux64K)
	wantTotal := int64(4 * 4096) // one block of 4 * 4096
	wantUsed := int64(1 * 4096)
	if totalBytes != wantTotal || usedBytes != wantUsed {
		t.Fatalf("after 4K Get: total=%d, used=%d; want %d, %d",
			totalBytes, usedBytes, wantTotal, wantUsed)
	}

	// Allocate from 64K pool.
	h64K := mux64K.Get()
	totalBytes, usedBytes = combinedMuxStats(mux4K, mux64K)
	wantTotal = int64(4*4096) + int64(4*65535)
	wantUsed = int64(1*4096) + int64(1*65535)
	if totalBytes != wantTotal || usedBytes != wantUsed {
		t.Fatalf("after both Gets: total=%d, used=%d; want %d, %d",
			totalBytes, usedBytes, wantTotal, wantUsed)
	}

	mux4K.Return(h4K)
	mux64K.Return(h64K)
}

func TestBufMux_CombinedUsedRatio(t *testing.T) {
	mux4K := newBufMux(4096, 2)
	mux64K := newBufMux(65535, 2)

	ratio := combinedMuxUsedRatio(mux4K, mux64K)
	if ratio != 0.0 {
		t.Fatalf("empty: ratio=%f, want 0.0", ratio)
	}

	// Fill all 4K slots (2).
	h4Ka := mux4K.Get()
	h4Kb := mux4K.Get()

	// Fill 1 of 2 64K slots.
	h64K := mux64K.Get()

	// Total allocated: 2*4096 + 2*65535 = 139262
	// Total used: 2*4096 + 1*65535 = 73727
	ratio = combinedMuxUsedRatio(mux4K, mux64K)
	want := float64(2*4096+1*65535) / float64(2*4096+2*65535)
	if ratio < want-0.001 || ratio > want+0.001 {
		t.Fatalf("ratio=%f, want ~%f", ratio, want)
	}

	mux4K.Return(h4Ka)
	mux4K.Return(h4Kb)
	mux64K.Return(h64K)
}

func TestBufMux_SharedBudget(t *testing.T) {
	// VALIDATES: AC-27 — shared budget prevents one pool from starving the other.
	// Two muxes share a byte budget. Growing one affects the other.
	mux4K := newBufMux(4096, 2)
	mux64K := newBufMux(65535, 2)

	// Budget: enough for 1 block of 4K (8192) + 1 block of 64K (131070) = 139262.
	// A second block from either would exceed the budget.
	cb := newCombinedBudget(139262)
	mux4K.SetBudget(cb)
	mux64K.SetBudget(cb)

	// Fill both first blocks.
	h4Ka := mux4K.Get()
	h4Kb := mux4K.Get()
	h64Ka := mux64K.Get()
	h64Kb := mux64K.Get()

	// Both blocks full. Growing either would exceed budget.
	h := mux4K.Get()
	if h.Buf != nil {
		t.Fatal("4K grow should be denied: combined budget exceeded")
	}
	h = mux64K.Get()
	if h.Buf != nil {
		t.Fatal("64K grow should be denied: combined budget exceeded")
	}

	// Return from 4K pool — frees space in existing block, but doesn't change allocated total.
	mux4K.Return(h4Ka)
	// Can get from 4K (existing free slot) but still can't grow.
	h4Ka = mux4K.Get()
	if h4Ka.Buf == nil {
		t.Fatal("should be able to get from existing 4K block")
	}

	mux4K.Return(h4Ka)
	mux4K.Return(h4Kb)
	mux64K.Return(h64Ka)
	mux64K.Return(h64Kb)
}

func TestBufMux_StatsConcurrent(t *testing.T) {
	// Stats is safe under concurrent Get/Return.
	mux := newBufMux(64, 4)
	const goroutines = 8
	const ops = 100

	var wg sync.WaitGroup
	wg.Add(goroutines + 1) // workers + stats reader

	// Stats reader.
	go func() {
		defer wg.Done()
		for range ops * goroutines {
			alloc, used := mux.Stats()
			if used > alloc {
				t.Errorf("used (%d) > allocated (%d)", used, alloc)
			}
		}
	}()

	// Workers.
	for range goroutines {
		go func() {
			defer wg.Done()
			for range ops {
				h := mux.Get()
				if h.Buf != nil {
					mux.Return(h)
				}
			}
		}()
	}

	wg.Wait()
}

func TestBufMux_BudgetExactBoundary(t *testing.T) {
	// VALIDATES: AC-27 — canGrow allows growth when allocated + blockBytes == maxBytes.
	// PREVENTS: Off-by-one in <= vs < comparison.
	mux := newBufMux(64, 2)
	blockBytes := 64 * 2 // 128 bytes per block

	// Budget for exactly 2 blocks (256 bytes).
	cb := newCombinedBudget(int64(blockBytes * 2))
	mux.SetBudget(cb)

	// Fill block 0.
	h0a := mux.Get()
	h0b := mux.Get()

	// Block 1 should succeed: 128 + 128 = 256 <= 256.
	h1a := mux.Get()
	if h1a.Buf == nil {
		t.Fatal("Get() should succeed at exact budget boundary (allocated + blockBytes == maxBytes)")
	}

	// Block 2 should be denied: 256 + 128 = 384 > 256.
	h1b := mux.Get()
	h2 := mux.Get()
	if h2.Buf != nil {
		t.Fatal("Get() should return zero handle above budget")
	}

	mux.Return(h0a)
	mux.Return(h0b)
	mux.Return(h1a)
	if h1b.Buf != nil {
		mux.Return(h1b)
	}
}

func TestCombinedBudget_UnlimitedAlwaysAllows(t *testing.T) {
	// VALIDATES: AC-27 — maxBytes <= 0 means unlimited.
	// PREVENTS: Zero or negative budget accidentally blocking all growth.
	for _, max := range []int64{0, -1, -100} {
		cb := newCombinedBudget(max)
		if !cb.tryReserve(1_000_000) {
			t.Errorf("maxBytes=%d: tryReserve should always succeed for unlimited budget", max)
		}
	}
}

func TestBufMux_SetBudgetSeedsExistingBlocks(t *testing.T) {
	// VALIDATES: AC-27 — SetBudget accounts for pre-existing blocks.
	// PREVENTS: Negative budget counter when blocks grown before budget exist are later collapsed.
	mux := newBufMux(64, 2)

	// Grow a block before budget is set.
	h := mux.Get()
	if mux.blockCount() != 1 {
		t.Fatalf("should have 1 block, got %d", mux.blockCount())
	}

	// Set budget — should seed the counter with the existing block's bytes.
	cb := newCombinedBudget(64 * 4)
	mux.SetBudget(cb)

	if cb.AllocatedBytes() != int64(64*2) {
		t.Fatalf("budget should be seeded with existing block: allocated=%d, want %d",
			cb.AllocatedBytes(), 64*2)
	}

	// Return and collapse — counter should reach 0, not go negative.
	mux.Return(h)
	// Need another block for collapse to trigger (needs highest fully returned + below >=50% free).
	h2a := mux.Get()
	h2b := mux.Get()
	h3a := mux.Get() // grows block 1

	mux.Return(h3a)
	mux.Return(h2a)
	mux.tryCollapse()

	if cb.AllocatedBytes() < 0 {
		t.Fatalf("budget counter went negative: %d", cb.AllocatedBytes())
	}

	mux.Return(h2b)
}

func TestCombinedBufMuxGlobalStats(t *testing.T) {
	// VALIDATES: AC-27 — CombinedBufMuxStats and CombinedBufMuxUsedRatio
	// read from the global bufMux4K/bufMux64K pools.
	// PREVENTS: Global wiring disconnect.

	// The global pools start empty or have steady-state traffic.
	// Just verify the functions are callable and return consistent values.
	totalBytes, usedBytes := CombinedBufMuxStats()
	if usedBytes > totalBytes {
		t.Fatalf("usedBytes (%d) > totalBytes (%d)", usedBytes, totalBytes)
	}

	ratio := CombinedBufMuxUsedRatio()
	if ratio < 0 || ratio > 1.0 {
		t.Fatalf("ratio %f out of [0, 1] range", ratio)
	}

	// Allocate from global 4K pool, verify stats change.
	h := bufMux4K.Get()
	if h.Buf == nil {
		t.Fatal("global bufMux4K.Get() returned nil")
	}

	totalAfter, usedAfter := CombinedBufMuxStats()
	if totalAfter < totalBytes {
		t.Fatalf("totalBytes should not decrease after Get: before=%d, after=%d", totalBytes, totalAfter)
	}
	if usedAfter <= usedBytes && totalAfter > 0 {
		t.Fatalf("usedBytes should increase after Get: before=%d, after=%d", usedBytes, usedAfter)
	}

	bufMux4K.Return(h)
}

func TestInitBufMuxBudget_Zero(t *testing.T) {
	// VALIDATES: AC-27 — initBufMuxBudget(0) is a no-op.
	// PREVENTS: Zero maxBytes accidentally creating a budget that blocks growth.

	// Save current budget state.
	old4K := bufMux4K.mux.budget
	old64K := bufMux64K.mux.budget
	defer func() {
		bufMux4K.mux.budget = old4K
		bufMux64K.mux.budget = old64K
	}()

	// Clear budgets.
	bufMux4K.mux.budget = nil
	bufMux64K.mux.budget = nil

	initBufMuxBudget(0)

	// Budget is always created (even with 0 = unlimited) so
	// updateBufMuxBudget never hits the create-path concurrently.
	if bufMux4K.mux.budget == nil {
		t.Fatal("initBufMuxBudget(0) should create a budget (unlimited)")
	}
	if bufMux4K.mux.budget.maxBytes.Load() != 0 {
		t.Fatal("initBufMuxBudget(0) should set maxBytes=0 (unlimited)")
	}
	if bufMux4K.mux.budget != bufMux64K.mux.budget {
		t.Fatal("both pools should share the same budget instance")
	}
}

func TestInitBufMuxBudget_Positive(t *testing.T) {
	// VALIDATES: AC-27 — initBufMuxBudget(N) wires shared budget to both pools.
	// PREVENTS: Budget not actually shared between 4K and 64K pools.

	old4K := bufMux4K.mux.budget
	old64K := bufMux64K.mux.budget
	defer func() {
		bufMux4K.mux.budget = old4K
		bufMux64K.mux.budget = old64K
	}()

	bufMux4K.mux.budget = nil
	bufMux64K.mux.budget = nil

	initBufMuxBudget(1_000_000_000) // 1GB — large enough to not interfere

	if bufMux4K.mux.budget == nil {
		t.Fatal("initBufMuxBudget should set budget on bufMux4K")
	}
	if bufMux64K.mux.budget == nil {
		t.Fatal("initBufMuxBudget should set budget on bufMux64K")
	}
	if bufMux4K.mux.budget != bufMux64K.mux.budget {
		t.Fatal("both pools should share the same budget instance")
	}
}

func TestUpdateBufMuxBudget_UpdatesExistingLimit(t *testing.T) {
	// VALIDATES: AC-28 -- updateBufMuxBudget atomically updates the shared budget limit.
	// PREVENTS: Data race on combinedBudget.maxBytes (finding #1).

	old4K := bufMux4K.mux.budget
	old64K := bufMux64K.mux.budget
	defer func() {
		bufMux4K.mux.budget = old4K
		bufMux64K.mux.budget = old64K
	}()

	initBufMuxBudget(1_000_000)
	if bufMux4K.mux.budget.maxBytes.Load() != 1_000_000 {
		t.Fatal("initial maxBytes should be 1000000")
	}

	updateBufMuxBudget(5_000_000)
	if bufMux4K.mux.budget.maxBytes.Load() != 5_000_000 {
		t.Errorf("maxBytes after update = %d, want 5000000", bufMux4K.mux.budget.maxBytes.Load())
	}
	if bufMux64K.mux.budget.maxBytes.Load() != 5_000_000 {
		t.Errorf("64K maxBytes = %d, want 5000000 (should share budget)", bufMux64K.mux.budget.maxBytes.Load())
	}
}

func TestUpdateBufMuxBudget_ZeroMeansUnlimited(t *testing.T) {
	// VALIDATES: AC-28 -- updateBufMuxBudget(0) sets unlimited (no cap).
	// PREVENTS: Stale budget when all peers removed (finding #5).

	old4K := bufMux4K.mux.budget
	old64K := bufMux64K.mux.budget
	defer func() {
		bufMux4K.mux.budget = old4K
		bufMux64K.mux.budget = old64K
	}()

	initBufMuxBudget(1_000_000)
	updateBufMuxBudget(0)

	if bufMux4K.mux.budget.maxBytes.Load() != 0 {
		t.Errorf("maxBytes after zero update = %d, want 0 (unlimited)", bufMux4K.mux.budget.maxBytes.Load())
	}
}
