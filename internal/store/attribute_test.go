package store

import (
	"sync"
	"testing"
)

// testValue is a simple hashable value for testing.
type testValue struct {
	data string
}

func (t testValue) Hash() uint64 {
	return HashString(t.data)
}

func (t testValue) Equal(other any) bool {
	o, ok := other.(testValue)
	if !ok {
		return false
	}
	return t.data == o.data
}

func TestAttributeStore_InternBasic(t *testing.T) {
	store := NewAttributeStore[testValue](10)
	defer store.Stop()

	v1 := testValue{data: "hello"}
	v2 := testValue{data: "hello"}
	v3 := testValue{data: "world"}

	// Intern v1
	r1 := store.Intern(v1)
	if r1.data != "hello" {
		t.Errorf("expected 'hello', got %q", r1.data)
	}

	// Intern v2 (same as v1) - should return same instance
	r2 := store.Intern(v2)
	if r2.data != "hello" {
		t.Errorf("expected 'hello', got %q", r2.data)
	}

	// Intern v3 (different)
	r3 := store.Intern(v3)
	if r3.data != "world" {
		t.Errorf("expected 'world', got %q", r3.data)
	}

	// Check store length
	if store.Len() != 2 {
		t.Errorf("expected 2 entries, got %d", store.Len())
	}

	// Check stats
	hits, misses := store.Stats()
	if misses != 2 { // v1 and v3 are new
		t.Errorf("expected 2 misses, got %d", misses)
	}
	if hits != 1 { // v2 is a hit
		t.Errorf("expected 1 hit, got %d", hits)
	}
}

func TestAttributeStore_Lookup(t *testing.T) {
	store := NewAttributeStore[testValue](10)
	defer store.Stop()

	v1 := testValue{data: "test"}

	// Lookup before interning - should not find
	_, found := store.Lookup(v1)
	if found {
		t.Error("should not find value before interning")
	}

	// Intern the value
	store.Intern(v1)

	// Lookup after interning - should find
	result, found := store.Lookup(v1)
	if !found {
		t.Error("should find value after interning")
	}
	if result.data != "test" {
		t.Errorf("expected 'test', got %q", result.data)
	}
}

func TestAttributeStore_Release(t *testing.T) {
	store := NewAttributeStore[testValue](10)
	defer store.Stop()

	v1 := testValue{data: "release-test"}

	// Intern twice to get refCount=2
	store.Intern(v1)
	store.Intern(v1)

	if store.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", store.Len())
	}

	// Release once - should still exist
	released := store.Release(v1)
	if !released {
		t.Error("expected release to return true")
	}
	if store.Len() != 1 {
		t.Errorf("expected 1 entry after first release, got %d", store.Len())
	}

	// Release again - should be removed
	released = store.Release(v1)
	if !released {
		t.Error("expected release to return true")
	}
	if store.Len() != 0 {
		t.Errorf("expected 0 entries after second release, got %d", store.Len())
	}

	// Release non-existent - should return false
	released = store.Release(v1)
	if released {
		t.Error("expected release to return false for non-existent value")
	}
}

func TestAttributeStore_Concurrent(t *testing.T) {
	store := NewAttributeStore[testValue](100)
	defer store.Stop()

	// Intern the same value from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Intern(testValue{data: "concurrent"})
		}()
	}
	wg.Wait()

	// Should only have 1 entry
	if store.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", store.Len())
	}

	// Stats should show 1 miss and 99 hits
	hits, misses := store.Stats()
	if misses != 1 {
		t.Errorf("expected 1 miss, got %d", misses)
	}
	if hits != 99 {
		t.Errorf("expected 99 hits, got %d", hits)
	}
}

func TestAttributeStore_InternDirect(t *testing.T) {
	store := NewAttributeStore[testValue](10)
	defer store.Stop()

	// Use InternDirect for synchronous interning
	v1 := testValue{data: "direct"}
	r1 := store.InternDirect(v1)
	if r1.data != "direct" {
		t.Errorf("expected 'direct', got %q", r1.data)
	}

	// Second intern should return same
	v2 := testValue{data: "direct"}
	r2 := store.InternDirect(v2)
	if r2.data != "direct" {
		t.Errorf("expected 'direct', got %q", r2.data)
	}

	if store.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", store.Len())
	}
}

func TestHashHelpers(t *testing.T) {
	// Test HashBytes
	h1 := HashBytes([]byte("test"))
	h2 := HashBytes([]byte("test"))
	h3 := HashBytes([]byte("other"))

	if h1 != h2 {
		t.Error("same bytes should produce same hash")
	}
	if h1 == h3 {
		t.Error("different bytes should produce different hash")
	}

	// Test HashUint32
	h4 := HashUint32(12345)
	h5 := HashUint32(12345)
	h6 := HashUint32(54321)

	if h4 != h5 {
		t.Error("same uint32 should produce same hash")
	}
	if h4 == h6 {
		t.Error("different uint32 should produce different hash")
	}

	// Test HashString
	h7 := HashString("hello")
	h8 := HashString("hello")
	h9 := HashString("world")

	if h7 != h8 {
		t.Error("same string should produce same hash")
	}
	if h7 == h9 {
		t.Error("different string should produce different hash")
	}

	// Test CombineHashes
	c1 := CombineHashes(h1, h4, h7)
	c2 := CombineHashes(h1, h4, h7)
	c3 := CombineHashes(h1, h4, h9)

	if c1 != c2 {
		t.Error("same combined hashes should be equal")
	}
	if c1 == c3 {
		t.Error("different combined hashes should differ")
	}
}

func BenchmarkAttributeStore_Intern(b *testing.B) {
	store := NewAttributeStore[testValue](1000)
	defer store.Stop()

	value := testValue{data: "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Intern(value)
	}
}

func BenchmarkAttributeStore_InternDirect(b *testing.B) {
	store := NewAttributeStore[testValue](1000)
	defer store.Stop()

	value := testValue{data: "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.InternDirect(value)
	}
}

func BenchmarkAttributeStore_ConcurrentIntern(b *testing.B) {
	store := NewAttributeStore[testValue](1000)
	defer store.Stop()

	value := testValue{data: "concurrent-bench"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			store.Intern(value)
		}
	})
}
