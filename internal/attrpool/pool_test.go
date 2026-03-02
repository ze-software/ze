package attrpool

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers for cleaner error handling

func mustGet(t *testing.T, p *Pool, h Handle) []byte {
	t.Helper()
	data, err := p.Get(h)
	require.NoError(t, err)
	return data
}

func mustLength(t *testing.T, p *Pool, h Handle) int {
	t.Helper()
	length, err := p.Length(h)
	require.NoError(t, err)
	return length
}

func mustRelease(t *testing.T, p *Pool, h Handle) {
	t.Helper()
	err := p.Release(h)
	require.NoError(t, err)
}

// TestInternDeduplication verifies that interning identical data returns
// the same handle and increments reference count.
//
// VALIDATES: Memory efficiency through deduplication.
//
// PREVENTS: Memory bloat - without deduplication, 1M routes sharing the
// same AS_PATH would store 1M copies instead of 1 with refCount=1M.
func TestInternDeduplication(t *testing.T) {
	p := New(1024)

	h1 := p.Intern([]byte("hello"))
	h2 := p.Intern([]byte("hello"))

	require.Equal(t, h1, h2, "identical data must return same handle")
	require.True(t, h1.IsValid(), "handle must be valid")
}

// TestInternUnique verifies that different data gets different handles.
//
// VALIDATES: Correct storage of distinct entries.
//
// PREVENTS: Data corruption where different data incorrectly shares
// the same handle, returning wrong data on Get().
func TestInternUnique(t *testing.T) {
	p := New(1024)

	h1 := p.Intern([]byte("hello"))
	h2 := p.Intern([]byte("world"))

	require.NotEqual(t, h1, h2, "different data must return different handles")
}

// TestGetReturnsCorrectData verifies Get() returns the interned data.
//
// VALIDATES: Data integrity through intern/get cycle.
//
// PREVENTS: Data corruption or loss during storage.
func TestGetReturnsCorrectData(t *testing.T) {
	p := New(1024)
	data := []byte("test data 12345")

	h := p.Intern(data)
	got := mustGet(t, p, h)

	require.Equal(t, data, got, "Get must return original data")
}

// TestGetMultipleEntries verifies multiple entries can be retrieved correctly.
//
// VALIDATES: Multiple entries stored independently.
//
// PREVENTS: Entries overwriting each other in storage.
func TestGetMultipleEntries(t *testing.T) {
	p := New(1024)

	h1 := p.Intern([]byte("first"))
	h2 := p.Intern([]byte("second"))
	h3 := p.Intern([]byte("third"))

	require.Equal(t, []byte("first"), mustGet(t, p, h1))
	require.Equal(t, []byte("second"), mustGet(t, p, h2))
	require.Equal(t, []byte("third"), mustGet(t, p, h3))
}

// TestReleaseDecrementsRefCount verifies Release() decrements reference count.
//
// VALIDATES: Reference counting correctness.
//
// PREVENTS: Memory leaks from entries never being freed, or
// use-after-free from premature deletion.
func TestReleaseDecrementsRefCount(t *testing.T) {
	p := New(1024)

	// Intern twice (refCount = 2)
	h := p.Intern([]byte("data"))
	_ = p.Intern([]byte("data"))

	// Release once (refCount = 1)
	mustRelease(t, p, h)

	// Data should still be accessible
	got := mustGet(t, p, h)
	require.Equal(t, []byte("data"), got, "data must survive partial release")
}

// TestReleaseToZeroMarksDead verifies that releasing to refCount=0 marks dead.
//
// VALIDATES: Entry lifecycle management.
//
// PREVENTS: Dead entries remaining live (memory leak) or live entries
// being marked dead (use-after-free).
func TestReleaseToZeroMarksDead(t *testing.T) {
	p := New(1024)

	h := p.Intern([]byte("data"))
	mustRelease(t, p, h)

	// After release to zero, entry should be dead
	// New intern of same data should get new handle (or reuse slot)
	h2 := p.Intern([]byte("data"))
	// Either same slot reused or new slot - both are valid
	require.True(t, h2.IsValid())

	// New handle should still work
	require.Equal(t, []byte("data"), mustGet(t, p, h2))
}

// TestDoubleReleaseError verifies that double-release returns error.
//
// VALIDATES: Double-release is detected and rejected.
//
// PREVENTS: freeSlots corruption from adding same slot twice,
// which would cause data corruption on subsequent Intern calls.
func TestDoubleReleaseError(t *testing.T) {
	p := New(1024)

	h := p.Intern([]byte("data"))

	// First release should succeed
	err := p.Release(h)
	require.NoError(t, err, "first release should succeed")

	// Second release should return ErrSlotDead
	err = p.Release(h)
	require.ErrorIs(t, err, ErrSlotDead, "double release must return ErrSlotDead")

	// Get on dead slot should also fail
	_, err = p.Get(h)
	require.ErrorIs(t, err, ErrSlotDead, "Get on released handle must return ErrSlotDead")
}

// TestInternWithErrorDataTooLarge verifies InternWithError returns error for large data.
//
// VALIDATES: InternWithError doesn't panic on large data.
//
// PREVENTS: Panic in error-returning function (API inconsistency).
func TestInternWithErrorDataTooLarge(t *testing.T) {
	p := New(1024)

	// Data exceeding MaxDataLength should return error, not panic
	tooLarge := make([]byte, MaxDataLength+1)
	h, err := p.InternWithError(tooLarge)
	require.ErrorIs(t, err, ErrDataTooLarge)
	require.Equal(t, InvalidHandle, h)

	// Max length should still work
	maxData := make([]byte, MaxDataLength)
	h, err = p.InternWithError(maxData)
	require.NoError(t, err)
	require.True(t, h.IsValid())
}

// TestInternEmpty verifies empty byte slice handling.
//
// VALIDATES: Edge case - empty data is valid input.
//
// PREVENTS: Panic or corruption on empty input.
func TestInternEmpty(t *testing.T) {
	p := New(1024)

	h := p.Intern([]byte{})
	require.True(t, h.IsValid())

	got := mustGet(t, p, h)
	require.Equal(t, []byte{}, got)
}

// TestInternNil verifies nil byte slice handling.
//
// VALIDATES: Edge case - nil data treated as empty.
//
// PREVENTS: Panic on nil input.
func TestInternNil(t *testing.T) {
	p := New(1024)

	h := p.Intern(nil)
	require.True(t, h.IsValid())

	got := mustGet(t, p, h)
	require.Equal(t, []byte{}, got)
}

// TestConcurrentIntern verifies thread-safety of concurrent Intern calls.
//
// VALIDATES: Thread-safety under concurrent access.
//
// PREVENTS: Data races, corruption, panics under load from multiple
// BGP peers interning routes simultaneously.
func TestConcurrentIntern(t *testing.T) {
	p := New(1024 * 1024)
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 1000 {
				data := fmt.Appendf(nil, "data-%d-%d", id, j)
				h := p.Intern(data)
				got := mustGet(t, p, h)
				assert.Equal(t, data, got)
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentInternDedup verifies deduplication works under concurrent access.
//
// VALIDATES: Thread-safe deduplication.
//
// PREVENTS: Race conditions causing duplicate storage of same data.
func TestConcurrentInternDedup(t *testing.T) {
	p := New(1024)
	var wg sync.WaitGroup
	handles := make([]Handle, 100)

	// All goroutines intern the same data
	for i := range 100 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			handles[idx] = p.Intern([]byte("shared-data"))
		}(i)
	}

	wg.Wait()

	// All handles should be the same (deduplication)
	first := handles[0]
	for i, h := range handles {
		require.Equal(t, first, h, "handle %d should match first handle", i)
	}
}

// TestConcurrentRelease verifies thread-safety of concurrent Release calls.
//
// VALIDATES: Thread-safe reference counting.
//
// PREVENTS: Race conditions corrupting reference counts.
func TestConcurrentRelease(t *testing.T) {
	p := New(1024)

	// Intern same data 100 times (refCount = 100)
	var handles []Handle
	for range 100 {
		handles = append(handles, p.Intern([]byte("shared")))
	}

	// Release from multiple goroutines
	var wg sync.WaitGroup
	for _, h := range handles {
		wg.Add(1)
		go func(handle Handle) {
			defer wg.Done()
			_ = p.Release(handle)
		}(h)
	}

	wg.Wait()

	// After all releases, data should be dead
	// Re-interning should work
	h := p.Intern([]byte("shared"))
	require.True(t, h.IsValid())
}

// TestLength verifies Length() returns correct data length.
//
// VALIDATES: Length query without data copy.
//
// PREVENTS: Incorrect length reporting for wire format construction.
func TestLength(t *testing.T) {
	p := New(1024)

	h := p.Intern([]byte("hello world"))
	require.Equal(t, 11, mustLength(t, p, h))

	h2 := p.Intern([]byte{})
	require.Equal(t, 0, mustLength(t, p, h2))
}

// TestLargeData verifies handling of larger data chunks.
//
// VALIDATES: Variable-size data storage.
//
// PREVENTS: Buffer overflow or truncation on large inputs.
func TestLargeData(t *testing.T) {
	p := New(1024 * 1024)

	// Create 10KB of data
	large := make([]byte, 10*1024)
	for i := range large {
		large[i] = byte(i % 256)
	}

	h := p.Intern(large)
	got := mustGet(t, p, h)

	require.Equal(t, large, got)
	require.Equal(t, 10*1024, mustLength(t, p, h))
}

// TestPoolIdxEncoding verifies Pool embeds idx in returned handles.
//
// VALIDATES: Intern returns handles with correct poolIdx encoded.
//
// PREVENTS: Wrong pool lookup when multiple pools exist.
func TestPoolIdxEncoding(t *testing.T) {
	p := NewWithIdx(5, 1024) // idx=5
	h := p.Intern([]byte("test"))

	require.Equal(t, uint8(5), h.PoolIdx(), "handle must encode pool idx")
	require.True(t, h.IsValid(), "handle must be valid")
}

// TestPoolExtractsSlot verifies Pool methods use slot portion of handle.
//
// VALIDATES: Get/Length/Release work with encoded handles.
//
// PREVENTS: Using full handle as slot index (would be wrong offset).
func TestPoolExtractsSlot(t *testing.T) {
	p := NewWithIdx(5, 1024)
	h := p.Intern([]byte("hello"))

	// Get works with encoded handle
	require.Equal(t, []byte("hello"), mustGet(t, p, h), "Get must extract slot correctly")

	// Length works
	require.Equal(t, 5, mustLength(t, p, h), "Length must extract slot correctly")

	// Verify pool handles encode poolIdx correctly
	require.Equal(t, uint8(5), h.PoolIdx(), "PoolIdx must be preserved in handle")
}

// TestPoolIdxValidation verifies pool rejects invalid idx.
//
// VALIDATES: Pool creation validates idx range.
//
// PREVENTS: Creating pool with reserved idx=31.
func TestPoolIdxValidation(t *testing.T) {
	require.Panics(t, func() {
		NewWithIdx(31, 1024) // Reserved idx
	}, "pool must reject idx=31")
}

// TestPoolIdxBoundary verifies boundary values for pool idx.
//
// VALIDATES: Pool accepts idx 0-30, rejects 31.
// BOUNDARY: idx 0-30 valid, 31 reserved.
//
// PREVENTS: Off-by-one errors in idx validation.
func TestPoolIdxBoundary(t *testing.T) {
	t.Run("idx_0_valid", func(t *testing.T) {
		p := NewWithIdx(0, 64)
		h := p.Intern([]byte("a"))
		require.Equal(t, uint8(0), h.PoolIdx())
	})

	t.Run("idx_30_last_valid", func(t *testing.T) {
		p := NewWithIdx(30, 64)
		h := p.Intern([]byte("b"))
		require.Equal(t, uint8(30), h.PoolIdx())
	})

	t.Run("idx_31_reserved", func(t *testing.T) {
		require.Panics(t, func() {
			NewWithIdx(31, 64)
		})
	})
}

// TestPoolMultiplePools verifies handles from different pools have different poolIdx.
//
// VALIDATES: Multiple pools coexist with distinct poolIdx.
//
// PREVENTS: Pool confusion when routing NLRI handles to correct pool.
func TestPoolMultiplePools(t *testing.T) {
	p0 := NewWithIdx(0, 64)
	p1 := NewWithIdx(1, 64)
	p5 := NewWithIdx(5, 64)

	h0 := p0.Intern([]byte("data"))
	h1 := p1.Intern([]byte("data"))
	h5 := p5.Intern([]byte("data"))

	require.Equal(t, uint8(0), h0.PoolIdx())
	require.Equal(t, uint8(1), h1.PoolIdx())
	require.Equal(t, uint8(5), h5.PoolIdx())

	// Each pool returns its own data
	require.Equal(t, []byte("data"), mustGet(t, p0, h0))
	require.Equal(t, []byte("data"), mustGet(t, p1, h1))
	require.Equal(t, []byte("data"), mustGet(t, p5, h5))
}

// TestPoolWrongPoolError verifies using handle with wrong pool returns error.
//
// VALIDATES: Cross-pool handle misuse is detected and rejected.
//
// PREVENTS: Silent data corruption from using handle with wrong pool.
func TestPoolWrongPoolError(t *testing.T) {
	p0 := NewWithIdx(0, 64)
	p1 := NewWithIdx(1, 64)

	h0 := p0.Intern([]byte("data"))

	// Using h0 (from p0) with p1 should return ErrWrongPool
	_, err := p1.Get(h0)
	require.ErrorIs(t, err, ErrWrongPool, "Get with wrong pool must return ErrWrongPool")

	_, err = p1.Length(h0)
	require.ErrorIs(t, err, ErrWrongPool, "Length with wrong pool must return ErrWrongPool")

	err = p1.Release(h0)
	require.ErrorIs(t, err, ErrWrongPool, "Release with wrong pool must return ErrWrongPool")
}

// TestMaxSlotsConstant verifies MaxSlots matches 24-bit handle slot field.
//
// VALIDATES: Slot limit constant is correctly defined.
//
// PREVENTS: Mismatch between constant and handle bit width.
func TestMaxSlotsConstant(t *testing.T) {
	require.Equal(t, uint32(0xFFFFFF), uint32(MaxSlots), "MaxSlots must be 24-bit max (0xFFFFFF)")
}

// TestPoolIdxDeduplication verifies dedup works correctly with non-zero idx.
//
// VALIDATES: Deduplication uses h.Slot() not raw handle as slot index.
//
// PREVENTS: Panic when second Intern triggers dedup path with idx>0.
// REPRODUCES: Bug found during code review - p.slots[h] instead of p.slots[h.Slot()].
func TestPoolIdxDeduplication(t *testing.T) {
	p := NewWithIdx(5, 1024)

	// First intern - creates new entry
	h1 := p.Intern([]byte("test"))
	require.Equal(t, uint8(5), h1.PoolIdx())
	require.Equal(t, uint32(0), h1.Slot())

	// Second intern of same data - triggers dedup path
	h2 := p.Intern([]byte("test"))
	require.Equal(t, h1, h2, "dedup should return same handle")

	// Verify data is correct
	require.Equal(t, []byte("test"), mustGet(t, p, h1))
}

// TestInternMaxLength verifies maximum data length handling.
//
// VALIDATES: Data up to MaxDataLength (65535) can be interned.
//
// PREVENTS: Silent truncation of large data.
func TestInternMaxLength(t *testing.T) {
	p := New(1024 * 1024)

	// Max length should work
	maxData := make([]byte, MaxDataLength)
	for i := range maxData {
		maxData[i] = byte(i % 256)
	}
	h := p.Intern(maxData)
	require.True(t, h.IsValid())
	require.Equal(t, MaxDataLength, mustLength(t, p, h))

	// Verify data integrity
	got := mustGet(t, p, h)
	require.Equal(t, maxData, got)
}

// TestInternTooLarge verifies data exceeding MaxDataLength panics.
//
// VALIDATES: Large data is rejected, not silently truncated.
//
// PREVENTS: Data corruption from uint16 length overflow.
func TestInternTooLarge(t *testing.T) {
	p := New(1024 * 1024)

	// One byte over limit should panic
	tooLarge := make([]byte, MaxDataLength+1)
	require.Panics(t, func() {
		p.Intern(tooLarge)
	}, "data exceeding MaxDataLength must panic")
}

// TestPoolIdxRebuildIndex verifies rebuildIndex includes poolIdx in handles.
//
// VALIDATES: After buffer growth, index contains handles with correct poolIdx.
//
// PREVENTS: rebuildIndex creating handles with poolIdx=0 instead of pool's idx.
func TestPoolIdxRebuildIndex(t *testing.T) {
	// Start with tiny capacity to force buffer growth
	p := NewWithIdx(7, 64)

	// Intern enough data to trigger buffer growth
	var handles []Handle
	for i := range 100 {
		h := p.Intern(fmt.Appendf(nil, "data-%d-padding-to-make-it-longer", i))
		handles = append(handles, h)
		require.Equal(t, uint8(7), h.PoolIdx(), "handle %d should have poolIdx=7", i)
	}

	// Now dedup should still work - this exercises rebuilt index
	h := p.Intern([]byte("data-0-padding-to-make-it-longer"))
	require.Equal(t, handles[0], h, "dedup should return original handle after buffer growth")
	require.Equal(t, uint8(7), h.PoolIdx(), "deduped handle should have poolIdx=7")
}

// TestPoolAddRef verifies AddRef increments reference count.
//
// VALIDATES: AddRef allows sharing handles between owners.
//
// PREVENTS: Use-after-free when multiple owners release same handle.
func TestPoolAddRef(t *testing.T) {
	p := NewWithIdx(0, 1024)

	h := p.Intern([]byte("shared-data"))

	// Add reference
	require.NoError(t, p.AddRef(h))

	// First release
	require.NoError(t, p.Release(h))

	// Data still accessible (second owner still holds ref)
	data, err := p.Get(h)
	require.NoError(t, err)
	require.Equal(t, []byte("shared-data"), data)

	// Second release
	require.NoError(t, p.Release(h))

	// Now data should be dead
	_, err = p.Get(h)
	require.Error(t, err)
}

// TestPoolGetBySlot verifies GetBySlot retrieves data by slot index.
//
// VALIDATES: Normalized slot access works correctly.
//
// PREVENTS: Data corruption when handles stored without bufferBit.
func TestPoolGetBySlot(t *testing.T) {
	p := NewWithIdx(5, 1024)

	h := p.Intern([]byte("slot-data"))
	slotIdx := h.Slot()

	// Get by slot
	data, err := p.GetBySlot(slotIdx)
	require.NoError(t, err)
	require.Equal(t, []byte("slot-data"), data)

	// Get by handle should return same data
	data2, err := p.Get(h)
	require.NoError(t, err)
	require.Equal(t, data, data2)
}

// TestPoolReleaseBySlot verifies ReleaseBySlot decrements reference count.
//
// VALIDATES: Normalized slot release works correctly.
//
// PREVENTS: Memory leaks when handles stored without bufferBit.
func TestPoolReleaseBySlot(t *testing.T) {
	p := NewWithIdx(5, 1024)

	h := p.Intern([]byte("release-by-slot"))
	slotIdx := h.Slot()

	// Release by slot
	require.NoError(t, p.ReleaseBySlot(slotIdx))

	// Data should be dead
	_, err := p.GetBySlot(slotIdx)
	require.Error(t, err)
}

// TestPoolIncrementalCompaction verifies MigrateBatch works correctly.
//
// VALIDATES: Non-blocking incremental compaction migrates data.
//
// PREVENTS: Data loss or corruption during background compaction.
func TestPoolIncrementalCompaction(t *testing.T) {
	p := NewWithIdx(0, 1024)

	// Create several entries
	h1 := p.Intern([]byte("entry-1"))
	h2 := p.Intern([]byte("entry-2"))
	h3 := p.Intern([]byte("entry-3"))

	// Release one to create dead space
	require.NoError(t, p.Release(h2))

	// Start compaction
	p.StartCompaction()
	require.Equal(t, PoolCompacting, p.State())

	// Migrate in batches
	for !p.MigrateBatch(1) {
		// Keep migrating
	}

	// Data still accessible via original handles
	data1, err := p.Get(h1)
	require.NoError(t, err)
	require.Equal(t, []byte("entry-1"), data1)

	data3, err := p.Get(h3)
	require.NoError(t, err)
	require.Equal(t, []byte("entry-3"), data3)

	// Wait for old buffer release (no more refs)
	p.CheckOldBufferRelease()
}

// TestPoolBothHandlesValidDuringCompaction verifies old and new handles work.
//
// VALIDATES: Both buffer bits valid during compaction.
//
// PREVENTS: Handle invalidation during background compaction.
func TestPoolBothHandlesValidDuringCompaction(t *testing.T) {
	p := NewWithIdx(0, 1024)

	// Create entry before compaction
	h1 := p.Intern([]byte("before-compact"))
	originalBit := h1.BufferBit()

	// Start compaction (flips currentBit)
	p.StartCompaction()

	// Old handle still works
	data, err := p.Get(h1)
	require.NoError(t, err)
	require.Equal(t, []byte("before-compact"), data)

	// New intern creates handle with new bufferBit
	h2 := p.Intern([]byte("during-compact"))
	require.NotEqual(t, originalBit, h2.BufferBit(), "new handle should have different bufferBit")

	// Both handles work
	data1, err := p.Get(h1)
	require.NoError(t, err)
	require.Equal(t, []byte("before-compact"), data1)

	data2, err := p.Get(h2)
	require.NoError(t, err)
	require.Equal(t, []byte("during-compact"), data2)

	// Complete migration
	for !p.MigrateBatch(100) {
	}
}

// TestCompactDuringIncrementalCompaction verifies Compact() is no-op during incremental.
//
// VALIDATES: Stop-the-world Compact doesn't interfere with incremental compaction.
//
// PREVENTS: Data corruption from concurrent compaction methods.
func TestCompactDuringIncrementalCompaction(t *testing.T) {
	p := NewWithIdx(0, 1024)

	// Create entries
	h1 := p.Intern([]byte("entry-1"))
	h2 := p.Intern([]byte("entry-2"))

	// Release one to create dead space
	require.NoError(t, p.Release(h1))

	// Start incremental compaction
	p.StartCompaction()
	require.Equal(t, PoolCompacting, p.State())

	// Calling Compact() during incremental should be no-op
	p.Compact()

	// Should still be in compacting state
	require.Equal(t, PoolCompacting, p.State())

	// Data should still be accessible
	data, err := p.Get(h2)
	require.NoError(t, err)
	require.Equal(t, []byte("entry-2"), data)

	// Complete incremental compaction
	for !p.MigrateBatch(100) {
	}
	p.CheckOldBufferRelease()
}

// TestInternDuringCompactionUnmigratedData verifies dedup works for unmigrated slots.
//
// VALIDATES: Intern finds existing data in unmigrated slots during compaction.
//
// PREVENTS: Duplicate storage when interning data that exists but hasn't migrated.
func TestInternDuringCompactionUnmigratedData(t *testing.T) {
	p := NewWithIdx(0, 1024)

	// Create entry before compaction
	h1 := p.Intern([]byte("existing-data"))
	originalSlot := h1.Slot()

	// Start compaction but don't migrate anything yet
	p.StartCompaction()
	require.Equal(t, PoolCompacting, p.State())

	// Intern same data - should find existing entry (unmigrated)
	h2 := p.Intern([]byte("existing-data"))

	// Should return same slot (dedup worked)
	require.Equal(t, originalSlot, h2.Slot(), "should dedup to same slot")

	// Both handles should work
	data1, err := p.Get(h1)
	require.NoError(t, err)
	require.Equal(t, []byte("existing-data"), data1)

	data2, err := p.Get(h2)
	require.NoError(t, err)
	require.Equal(t, []byte("existing-data"), data2)

	// Complete migration
	for !p.MigrateBatch(100) {
	}
}

// TestReleaseOldHandleDuringCompaction verifies Release with old bufferBit works.
//
// VALIDATES: Release correctly decrements old buffer refCount during compaction.
//
// PREVENTS: Buffer refCount mismatch causing premature or delayed buffer release.
func TestReleaseOldHandleDuringCompaction(t *testing.T) {
	p := NewWithIdx(0, 1024)

	// Create entry before compaction
	h := p.Intern([]byte("release-test"))
	originalBit := h.BufferBit()

	// Start compaction (flips currentBit)
	p.StartCompaction()
	require.NotEqual(t, originalBit, p.currentBit, "compaction should flip buffer")

	// Release using old handle (old bufferBit)
	require.NoError(t, p.Release(h))

	// Data should be dead
	_, err := p.Get(h)
	require.Error(t, err)

	// Complete migration and verify no issues
	for !p.MigrateBatch(100) {
	}
	p.CheckOldBufferRelease()
	require.Equal(t, PoolNormal, p.State())
}

// TestBufferGrowthDuringCompaction verifies rebuildIndex skips unmigrated slots.
//
// VALIDATES: Index rebuild during compaction doesn't corrupt unmigrated entries.
//
// PREVENTS: Index corruption from using uninitialized offsets for unmigrated slots.
func TestBufferGrowthDuringCompaction(t *testing.T) {
	// Start with tiny buffer to force growth
	p := NewWithIdx(0, 64)

	// Create several entries before compaction
	h1 := p.Intern([]byte("pre-compact-1"))
	h2 := p.Intern([]byte("pre-compact-2"))
	h3 := p.Intern([]byte("pre-compact-3"))

	// Start compaction
	p.StartCompaction()

	// Migrate only first slot
	p.MigrateBatch(1)

	// Now intern enough data to trigger buffer growth
	// This forces ensureCapacity -> rebuildIndex
	var newHandles []Handle
	for i := range 50 {
		h := p.Intern(fmt.Appendf(nil, "new-data-during-compact-%d-padding", i))
		newHandles = append(newHandles, h)
	}

	// Original handles should still work
	data1, err := p.Get(h1)
	require.NoError(t, err)
	require.Equal(t, []byte("pre-compact-1"), data1)

	data2, err := p.Get(h2)
	require.NoError(t, err)
	require.Equal(t, []byte("pre-compact-2"), data2)

	data3, err := p.Get(h3)
	require.NoError(t, err)
	require.Equal(t, []byte("pre-compact-3"), data3)

	// New handles should work
	for i, h := range newHandles {
		data, err := p.Get(h)
		require.NoError(t, err)
		expected := fmt.Sprintf("new-data-during-compact-%d-padding", i)
		require.Equal(t, []byte(expected), data)
	}

	// Complete migration
	for !p.MigrateBatch(100) {
	}
	p.CheckOldBufferRelease()
}

// TestDedupAfterBufferGrowthDuringCompaction verifies dedup works for unmigrated slots.
//
// VALIDATES: Index preserves old-buffer entries during rebuildIndex.
//
// PREVENTS: Deduplication failure causing duplicate storage after buffer growth.
func TestDedupAfterBufferGrowthDuringCompaction(t *testing.T) {
	// Start with tiny buffer to force growth
	p := NewWithIdx(0, 64)

	// Create entry before compaction
	h1 := p.Intern([]byte("dedup-test-data"))
	originalSlot := h1.Slot()

	// Start compaction but don't migrate
	p.StartCompaction()

	// Trigger buffer growth by interning lots of new data
	for i := range 50 {
		p.Intern(fmt.Appendf(nil, "grow-buffer-data-%d-padding-to-make-longer", i))
	}

	// Now try to dedup with the unmigrated entry
	// This MUST return the same slot (dedup should work)
	h2 := p.Intern([]byte("dedup-test-data"))

	require.Equal(t, originalSlot, h2.Slot(),
		"dedup must work for unmigrated slots after buffer growth")

	// Verify both handles return same data
	data1, err := p.Get(h1)
	require.NoError(t, err)
	data2, err := p.Get(h2)
	require.NoError(t, err)
	require.Equal(t, data1, data2)

	// Complete migration
	for !p.MigrateBatch(100) {
	}
}

// TestNewSlotsDuringCompaction verifies new slots added during compaction work correctly.
//
// VALIDATES: New slots during compaction are not incorrectly migrated.
//
// PREVENTS: MigrateBatch reading garbage offsets for slots created after compaction started.
func TestNewSlotsDuringCompaction(t *testing.T) {
	p := NewWithIdx(0, 1024)

	// Create initial entries
	h1 := p.Intern([]byte("before-compact"))

	// Start compaction
	p.StartCompaction()

	// Add new slots DURING compaction
	h2 := p.Intern([]byte("during-compact-1"))
	h3 := p.Intern([]byte("during-compact-2"))

	// Migrate all - should NOT corrupt new slots
	for !p.MigrateBatch(100) {
	}

	// All handles should return correct data
	data1, err := p.Get(h1)
	require.NoError(t, err)
	require.Equal(t, []byte("before-compact"), data1)

	data2, err := p.Get(h2)
	require.NoError(t, err)
	require.Equal(t, []byte("during-compact-1"), data2, "new slot during compaction corrupted")

	data3, err := p.Get(h3)
	require.NoError(t, err)
	require.Equal(t, []byte("during-compact-2"), data3, "new slot during compaction corrupted")
}

// TestSlotReuseStaleIndexEntry verifies stale index entries don't cause wrong data.
//
// VALIDATES: Releasing during compaction cleans up index properly.
//
// PREVENTS: Dedup returning stale handle that reads wrong data after slot reuse.
func TestSlotReuseStaleIndexEntry(t *testing.T) {
	p := NewWithIdx(0, 1024)

	// Create entry before compaction
	h1 := p.Intern([]byte("original-data"))
	slot1 := h1.Slot()

	// Start compaction
	p.StartCompaction()

	// Release during compaction (using old-buffer handle)
	require.NoError(t, p.Release(h1))

	// Reuse the slot with different data
	h2 := p.Intern([]byte("different-data"))

	// The slot should be reused
	require.Equal(t, slot1, h2.Slot(), "slot should be reused from free list")

	// Now try to dedup with original data
	// This should NOT return the old stale handle
	h3 := p.Intern([]byte("original-data"))

	// Get should return correct data
	data, err := p.Get(h3)
	require.NoError(t, err)
	require.Equal(t, []byte("original-data"), data,
		"dedup must not return stale handle that reads wrong data")
}
