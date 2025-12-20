package store

import (
	"sync"
	"testing"
)

// testNLRI is a simple NLRIHashable for testing.
type testNLRI struct {
	family uint32
	data   []byte
}

func (n testNLRI) Bytes() []byte {
	return n.data
}

func (n testNLRI) FamilyKey() uint32 {
	return n.family
}

func TestFamilyStore_InternBasic(t *testing.T) {
	store := NewFamilyStore[testNLRI](10)
	defer store.Stop()

	n1 := testNLRI{family: 1, data: []byte{10, 0, 0, 0, 24}}
	n2 := testNLRI{family: 1, data: []byte{10, 0, 0, 0, 24}}
	n3 := testNLRI{family: 1, data: []byte{192, 168, 1, 0, 24}}

	// Intern n1
	r1 := store.Intern(n1)
	if string(r1.data) != string(n1.data) {
		t.Error("interned data mismatch")
	}

	// Intern n2 (same as n1)
	r2 := store.Intern(n2)
	if string(r2.data) != string(n1.data) {
		t.Error("interned data mismatch")
	}

	// Intern n3 (different)
	r3 := store.Intern(n3)
	if string(r3.data) != string(n3.data) {
		t.Error("interned data mismatch")
	}

	if store.Len() != 2 {
		t.Errorf("expected 2 entries, got %d", store.Len())
	}

	hits, misses := store.Stats()
	if misses != 2 {
		t.Errorf("expected 2 misses, got %d", misses)
	}
	if hits != 1 {
		t.Errorf("expected 1 hit, got %d", hits)
	}
}

func TestFamilyStore_Release(t *testing.T) {
	store := NewFamilyStore[testNLRI](10)
	defer store.Stop()

	n1 := testNLRI{family: 1, data: []byte{10, 0, 0, 0}}

	// Intern twice
	store.Intern(n1)
	store.Intern(n1)

	if store.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", store.Len())
	}

	// Release once
	if !store.Release(n1) {
		t.Error("release should return true")
	}
	if store.Len() != 1 {
		t.Errorf("expected 1 entry after first release, got %d", store.Len())
	}

	// Release again
	if !store.Release(n1) {
		t.Error("release should return true")
	}
	if store.Len() != 0 {
		t.Errorf("expected 0 entries after second release, got %d", store.Len())
	}
}

func TestFamilyStore_Concurrent(t *testing.T) {
	store := NewFamilyStore[testNLRI](100)
	defer store.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Intern(testNLRI{family: 1, data: []byte{1, 2, 3, 4}})
		}()
	}
	wg.Wait()

	if store.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", store.Len())
	}

	hits, misses := store.Stats()
	if misses != 1 {
		t.Errorf("expected 1 miss, got %d", misses)
	}
	if hits != 99 {
		t.Errorf("expected 99 hits, got %d", hits)
	}
}

func TestNLRIStore_MultipleFamilies(t *testing.T) {
	store := NewNLRIStore[testNLRI](10)
	defer store.Stop()

	// IPv4 unicast (family key 0x00010001)
	n1 := testNLRI{family: 0x00010001, data: []byte{10, 0, 0, 0}}
	// IPv6 unicast (family key 0x00020001)
	n2 := testNLRI{family: 0x00020001, data: []byte{0x20, 0x01, 0x0d, 0xb8}}
	// Same family as n1, different prefix
	n3 := testNLRI{family: 0x00010001, data: []byte{192, 168, 0, 0}}

	store.Intern(n1)
	store.Intern(n2)
	store.Intern(n3)

	if store.FamilyCount() != 2 {
		t.Errorf("expected 2 families, got %d", store.FamilyCount())
	}

	if store.TotalLen() != 3 {
		t.Errorf("expected 3 total entries, got %d", store.TotalLen())
	}
}

func TestNLRIStore_GetOrCreate(t *testing.T) {
	store := NewNLRIStore[testNLRI](10)
	defer store.Stop()

	// First call creates
	s1 := store.GetOrCreate(1)
	if s1 == nil {
		t.Fatal("GetOrCreate should return non-nil")
	}

	// Second call returns same
	s2 := store.GetOrCreate(1)
	if s1 != s2 {
		t.Error("GetOrCreate should return same store for same key")
	}

	// Different key creates new store
	s3 := store.GetOrCreate(2)
	if s1 == s3 {
		t.Error("GetOrCreate should return different store for different key")
	}
}

func TestNLRIStore_Release(t *testing.T) {
	store := NewNLRIStore[testNLRI](10)
	defer store.Stop()

	n1 := testNLRI{family: 1, data: []byte{1, 2, 3}}

	// Intern
	store.Intern(n1)
	if store.TotalLen() != 1 {
		t.Errorf("expected 1 entry, got %d", store.TotalLen())
	}

	// Release
	if !store.Release(n1) {
		t.Error("release should return true")
	}
	if store.TotalLen() != 0 {
		t.Errorf("expected 0 entries, got %d", store.TotalLen())
	}

	// Release non-existent family
	n2 := testNLRI{family: 999, data: []byte{1, 2, 3}}
	if store.Release(n2) {
		t.Error("release of non-existent family should return false")
	}
}

func TestNLRIStore_ConcurrentFamilies(t *testing.T) {
	store := NewNLRIStore[testNLRI](100)
	defer store.Stop()

	var wg sync.WaitGroup

	// Concurrent access to multiple families
	for family := uint32(1); family <= 10; family++ {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(f uint32) {
				defer wg.Done()
				store.Intern(testNLRI{family: f, data: []byte{1, 2, 3, 4}})
			}(family)
		}
	}
	wg.Wait()

	if store.FamilyCount() != 10 {
		t.Errorf("expected 10 families, got %d", store.FamilyCount())
	}

	// Each family should have exactly 1 unique entry (same data)
	if store.TotalLen() != 10 {
		t.Errorf("expected 10 total entries, got %d", store.TotalLen())
	}
}

func BenchmarkFamilyStore_Intern(b *testing.B) {
	store := NewFamilyStore[testNLRI](1000)
	defer store.Stop()

	nlri := testNLRI{family: 1, data: []byte{10, 0, 0, 0, 24}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Intern(nlri)
	}
}

func BenchmarkNLRIStore_Intern(b *testing.B) {
	store := NewNLRIStore[testNLRI](1000)
	defer store.Stop()

	nlri := testNLRI{family: 0x00010001, data: []byte{10, 0, 0, 0, 24}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Intern(nlri)
	}
}
