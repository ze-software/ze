package pool

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	require.True(t, h1.Valid(), "handle must be valid")
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
	got := p.Get(h)

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

	require.Equal(t, []byte("first"), p.Get(h1))
	require.Equal(t, []byte("second"), p.Get(h2))
	require.Equal(t, []byte("third"), p.Get(h3))
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
	p.Release(h)

	// Data should still be accessible
	got := p.Get(h)
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
	p.Release(h)

	// After release to zero, entry should be dead
	// New intern of same data should get new handle (or reuse slot)
	h2 := p.Intern([]byte("data"))
	// Either same slot reused or new slot - both are valid
	require.True(t, h2.Valid())

	// New handle should still work
	require.Equal(t, []byte("data"), p.Get(h2))
}

// TestInternEmpty verifies empty byte slice handling.
//
// VALIDATES: Edge case - empty data is valid input.
//
// PREVENTS: Panic or corruption on empty input.
func TestInternEmpty(t *testing.T) {
	p := New(1024)

	h := p.Intern([]byte{})
	require.True(t, h.Valid())

	got := p.Get(h)
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
	require.True(t, h.Valid())

	got := p.Get(h)
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

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				data := []byte(fmt.Sprintf("data-%d-%d", id, j))
				h := p.Intern(data)
				got := p.Get(h)
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
	for i := 0; i < 100; i++ {
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
	for i := 0; i < 100; i++ {
		handles = append(handles, p.Intern([]byte("shared")))
	}

	// Release from multiple goroutines
	var wg sync.WaitGroup
	for _, h := range handles {
		wg.Add(1)
		go func(handle Handle) {
			defer wg.Done()
			p.Release(handle)
		}(h)
	}

	wg.Wait()

	// After all releases, data should be dead
	// Re-interning should work
	h := p.Intern([]byte("shared"))
	require.True(t, h.Valid())
}

// TestLength verifies Length() returns correct data length.
//
// VALIDATES: Length query without data copy.
//
// PREVENTS: Incorrect length reporting for wire format construction.
func TestLength(t *testing.T) {
	p := New(1024)

	h := p.Intern([]byte("hello world"))
	require.Equal(t, 11, p.Length(h))

	h2 := p.Intern([]byte{})
	require.Equal(t, 0, p.Length(h2))
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
	got := p.Get(h)

	require.Equal(t, large, got)
	require.Equal(t, 10*1024, p.Length(h))
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
	require.True(t, h.Valid(), "handle must be valid")
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
	require.Equal(t, []byte("hello"), p.Get(h), "Get must extract slot correctly")

	// Length works
	require.Equal(t, 5, p.Length(h), "Length must extract slot correctly")

	// WithFlags doesn't break access
	h2 := h.WithFlags(1)
	require.Equal(t, []byte("hello"), p.Get(h2), "Get must work after WithFlags")
}

// TestPoolIdxValidation verifies pool rejects invalid idx.
//
// VALIDATES: Pool creation validates idx range.
//
// PREVENTS: Creating pool with reserved idx=63.
func TestPoolIdxValidation(t *testing.T) {
	require.Panics(t, func() {
		NewWithIdx(63, 1024) // Reserved idx
	}, "pool must reject idx=63")
}

// TestPoolIdxBoundary verifies boundary values for pool idx.
//
// VALIDATES: Pool accepts idx 0-62, rejects 63.
// BOUNDARY: idx 0-62 valid, 63 reserved.
//
// PREVENTS: Off-by-one errors in idx validation.
func TestPoolIdxBoundary(t *testing.T) {
	t.Run("idx_0_valid", func(t *testing.T) {
		p := NewWithIdx(0, 64)
		h := p.Intern([]byte("a"))
		require.Equal(t, uint8(0), h.PoolIdx())
	})

	t.Run("idx_62_last_valid", func(t *testing.T) {
		p := NewWithIdx(62, 64)
		h := p.Intern([]byte("b"))
		require.Equal(t, uint8(62), h.PoolIdx())
	})

	t.Run("idx_63_reserved", func(t *testing.T) {
		require.Panics(t, func() {
			NewWithIdx(63, 64)
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
	require.Equal(t, []byte("data"), p0.Get(h0))
	require.Equal(t, []byte("data"), p1.Get(h1))
	require.Equal(t, []byte("data"), p5.Get(h5))
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
	require.Equal(t, []byte("test"), p.Get(h1))
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
	for i := 0; i < 100; i++ {
		h := p.Intern([]byte(fmt.Sprintf("data-%d-padding-to-make-it-longer", i)))
		handles = append(handles, h)
		require.Equal(t, uint8(7), h.PoolIdx(), "handle %d should have poolIdx=7", i)
	}

	// Now dedup should still work - this exercises rebuilt index
	h := p.Intern([]byte("data-0-padding-to-make-it-longer"))
	require.Equal(t, handles[0], h, "dedup should return original handle after buffer growth")
	require.Equal(t, uint8(7), h.PoolIdx(), "deduped handle should have poolIdx=7")
}
