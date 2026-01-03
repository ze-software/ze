package pool

import (
	"bytes"
	"sync"
	"testing"
)

// TestPoolInternAndGet verifies basic intern and retrieval.
//
// VALIDATES: Interned data can be retrieved via handle
// PREVENTS: Data loss or corruption during storage.
func TestPoolInternAndGet(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 1024,
		ExpectedEntries:   10,
	})

	data := []byte("test data")
	h := p.Intern(data)

	if h == InvalidHandle {
		t.Fatal("Intern returned InvalidHandle")
	}

	got := p.Get(h)
	if !bytes.Equal(got, data) {
		t.Errorf("Get() = %q, want %q", got, data)
	}
}

// TestPoolDeduplication verifies identical data returns same handle.
//
// VALIDATES: Same data returns same handle (deduplication works)
// PREVENTS: Memory waste from storing duplicate data.
func TestPoolDeduplication(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 1024,
		ExpectedEntries:   10,
	})

	data := []byte("deduplicate me")

	h1 := p.Intern(data)
	h2 := p.Intern(data)

	if h1 != h2 {
		t.Errorf("Same data should return same handle: %#x vs %#x", h1, h2)
	}

	// Also test with a copy (different slice, same content)
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	h3 := p.Intern(dataCopy)

	if h1 != h3 {
		t.Errorf("Equal data should return same handle: %#x vs %#x", h1, h3)
	}
}

// TestPoolDifferentData verifies different data returns different handles.
//
// VALIDATES: Different data returns different handles
// PREVENTS: Incorrect deduplication of different data.
func TestPoolDifferentData(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 1024,
		ExpectedEntries:   10,
	})

	h1 := p.Intern([]byte("data one"))
	h2 := p.Intern([]byte("data two"))

	if h1 == h2 {
		t.Error("Different data should return different handles")
	}

	// Verify each retrieves correctly
	if got := p.Get(h1); !bytes.Equal(got, []byte("data one")) {
		t.Errorf("Get(h1) = %q, want %q", got, "data one")
	}
	if got := p.Get(h2); !bytes.Equal(got, []byte("data two")) {
		t.Errorf("Get(h2) = %q, want %q", got, "data two")
	}
}

// TestPoolLength verifies Length returns correct size.
//
// VALIDATES: Length returns correct byte count
// PREVENTS: Incorrect size reporting.
func TestPoolLength(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 1024,
		ExpectedEntries:   10,
	})

	tests := []struct {
		data []byte
		want int
	}{
		{[]byte(""), 0},
		{[]byte("x"), 1},
		{[]byte("hello world"), 11},
		{make([]byte, 100), 100},
	}

	for _, tt := range tests {
		h := p.Intern(tt.data)
		if got := p.Length(h); got != tt.want {
			t.Errorf("Length(%q) = %d, want %d", tt.data, got, tt.want)
		}
	}
}

// TestPoolAddRefRelease verifies reference counting lifecycle.
//
// VALIDATES: AddRef/Release correctly manage reference count
// PREVENTS: Premature data reclamation or memory leaks.
func TestPoolAddRefRelease(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 1024,
		ExpectedEntries:   10,
	})

	data := []byte("refcounted data")
	h := p.Intern(data)

	// Initial refcount should be 1
	// Add two more references
	p.AddRef(h)
	p.AddRef(h)

	// Data should still be retrievable
	if got := p.Get(h); !bytes.Equal(got, data) {
		t.Errorf("Get() after AddRef = %q, want %q", got, data)
	}

	// Release three times (back to 0)
	p.Release(h)
	p.Release(h)
	p.Release(h)

	// After all releases, re-interning should work
	// (slot may be reused or new slot allocated)
	h2 := p.Intern(data)
	if got := p.Get(h2); !bytes.Equal(got, data) {
		t.Errorf("Get() after re-intern = %q, want %q", got, data)
	}
}

// TestPoolDeduplicationWithRefCount verifies dedup increments refcount.
//
// VALIDATES: Interning existing data increments refcount
// PREVENTS: Data loss when multiple users share handle.
func TestPoolDeduplicationWithRefCount(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 1024,
		ExpectedEntries:   10,
	})

	data := []byte("shared data")

	// First intern: refcount = 1
	h1 := p.Intern(data)

	// Second intern (dedup): refcount = 2
	h2 := p.Intern(data)

	if h1 != h2 {
		t.Fatal("Expected same handle for dedup")
	}

	// Release once: refcount = 1 (still alive)
	p.Release(h1)

	// Data should still be available
	if got := p.Get(h1); !bytes.Equal(got, data) {
		t.Errorf("Get() after first release = %q, want %q", got, data)
	}

	// Release again: refcount = 0 (dead)
	p.Release(h1)

	// Third intern should still work (may reuse or create new)
	h3 := p.Intern(data)
	if got := p.Get(h3); !bytes.Equal(got, data) {
		t.Errorf("Get() after re-intern = %q, want %q", got, data)
	}
}

// TestPoolConcurrentIntern verifies thread-safe interning.
//
// VALIDATES: Concurrent Intern calls are safe
// PREVENTS: Data races or corruption under load.
func TestPoolConcurrentIntern(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 4096,
		ExpectedEntries:   100,
	})

	const numGoroutines = 10
	const numOps = 100

	// Pre-generate test data
	testData := make([][]byte, numOps)
	for i := range testData {
		testData[i] = []byte("data-" + string(rune('A'+i%26)))
	}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				data := testData[i%len(testData)]
				h := p.Intern(data)
				got := p.Get(h)
				if !bytes.Equal(got, data) {
					t.Errorf("Concurrent Get mismatch: got %q, want %q", got, data)
				}
			}
		}()
	}

	wg.Wait()
}

// TestPoolConcurrentAddRefRelease verifies thread-safe reference counting.
//
// VALIDATES: Concurrent AddRef/Release are safe
// PREVENTS: Reference count races.
func TestPoolConcurrentAddRefRelease(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 1024,
		ExpectedEntries:   10,
	})

	data := []byte("concurrent refs")
	h := p.Intern(data)

	const numGoroutines = 10
	const numOps = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Half goroutines do AddRef
	for g := 0; g < numGoroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				p.AddRef(h)
			}
		}()
	}

	// Half goroutines do Release (but fewer to avoid going negative)
	for g := 0; g < numGoroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < numOps/2; i++ {
				p.Release(h)
			}
		}()
	}

	wg.Wait()

	// Data should still be retrievable (net positive refcount)
	if got := p.Get(h); !bytes.Equal(got, data) {
		t.Errorf("Get() after concurrent ops = %q, want %q", got, data)
	}
}

// TestPoolBufferGrowth verifies buffer grows correctly.
//
// VALIDATES: Pool handles data larger than initial buffer
// PREVENTS: Buffer overflow or data truncation.
func TestPoolBufferGrowth(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 64, // Small initial size
		ExpectedEntries:   10,
	})

	// Store data that will require growth
	var handles []Handle
	for i := 0; i < 20; i++ {
		data := make([]byte, 32) // Each 32 bytes, total 640 bytes
		for j := range data {
			data[j] = byte(i)
		}
		h := p.Intern(data)
		handles = append(handles, h)
	}

	// Verify all data is still correct
	for i, h := range handles {
		got := p.Get(h)
		if len(got) != 32 {
			t.Errorf("Entry %d: length = %d, want 32", i, len(got))
		}
		for j, b := range got {
			if b != byte(i) {
				t.Errorf("Entry %d byte %d: got %d, want %d", i, j, b, i)
			}
		}
	}
}

// TestPoolEmptyData verifies empty byte slices work.
//
// VALIDATES: Empty data can be interned and retrieved
// PREVENTS: Edge case handling issues.
func TestPoolEmptyData(t *testing.T) {
	p := NewPool(PoolConfig{
		InitialBufferSize: 1024,
		ExpectedEntries:   10,
	})

	h := p.Intern([]byte{})

	got := p.Get(h)
	if len(got) != 0 {
		t.Errorf("Get(empty) length = %d, want 0", len(got))
	}

	if p.Length(h) != 0 {
		t.Errorf("Length(empty) = %d, want 0", p.Length(h))
	}
}
