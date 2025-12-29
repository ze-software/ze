package context

import (
	"sync"
	"testing"
)

// TestRegistryRegister_NewContext verifies that Register returns a new ID.
//
// VALIDATES: Register assigns unique IDs to new contexts.
//
// PREVENTS: ID collisions between distinct contexts.
func TestRegistryRegister_NewContext(t *testing.T) {
	r := NewRegistry()

	ctx1 := &EncodingContext{
		ASN4:    true,
		LocalAS: 65000,
		PeerAS:  65001,
	}

	ctx2 := &EncodingContext{
		ASN4:    false,
		LocalAS: 65002,
		PeerAS:  65003,
	}

	id1 := r.Register(ctx1)
	id2 := r.Register(ctx2)

	if id1 == id2 {
		t.Errorf("different contexts got same ID: %d", id1)
	}
}

// TestRegistryRegister_Dedup verifies that identical contexts get the same ID.
//
// VALIDATES: Register returns same ID for structurally identical contexts.
//
// PREVENTS: Memory waste from storing duplicate contexts.
func TestRegistryRegister_Dedup(t *testing.T) {
	r := NewRegistry()

	ctx1 := &EncodingContext{
		ASN4:    true,
		LocalAS: 65000,
		PeerAS:  65001,
		AddPath: map[Family]bool{
			{AFI: 1, SAFI: 1}: true,
		},
	}

	ctx2 := &EncodingContext{
		ASN4:    true,
		LocalAS: 65000,
		PeerAS:  65001,
		AddPath: map[Family]bool{
			{AFI: 1, SAFI: 1}: true,
		},
	}

	id1 := r.Register(ctx1)
	id2 := r.Register(ctx2)

	if id1 != id2 {
		t.Errorf("identical contexts got different IDs: %d != %d", id1, id2)
	}
}

// TestRegistryGet_Exists verifies that Get returns the registered context.
//
// VALIDATES: Get retrieves the correct context by ID.
//
// PREVENTS: Wrong context lookup or data corruption.
func TestRegistryGet_Exists(t *testing.T) {
	r := NewRegistry()

	ctx := &EncodingContext{
		ASN4:    true,
		LocalAS: 65000,
		PeerAS:  65001,
	}

	id := r.Register(ctx)
	retrieved := r.Get(id)

	if retrieved == nil {
		t.Fatal("Get returned nil for registered context")
	}

	if retrieved.ASN4 != ctx.ASN4 {
		t.Errorf("ASN4 mismatch: got %v, want %v", retrieved.ASN4, ctx.ASN4)
	}
	if retrieved.LocalAS != ctx.LocalAS {
		t.Errorf("LocalAS mismatch: got %d, want %d", retrieved.LocalAS, ctx.LocalAS)
	}
	if retrieved.PeerAS != ctx.PeerAS {
		t.Errorf("PeerAS mismatch: got %d, want %d", retrieved.PeerAS, ctx.PeerAS)
	}
}

// TestRegistryGet_NotExists verifies that Get returns nil for unknown ID.
//
// VALIDATES: Get returns nil for unregistered IDs.
//
// PREVENTS: Panic or undefined behavior on invalid lookup.
func TestRegistryGet_NotExists(t *testing.T) {
	r := NewRegistry()

	retrieved := r.Get(12345)
	if retrieved != nil {
		t.Errorf("Get returned non-nil for unknown ID: %+v", retrieved)
	}
}

// TestRegistryConcurrent verifies thread safety of Register and Get.
//
// VALIDATES: Concurrent Register/Get operations don't race.
//
// PREVENTS: Data corruption under concurrent access.
func TestRegistryConcurrent(t *testing.T) {
	r := NewRegistry()

	var wg sync.WaitGroup
	const numGoroutines = 100
	const numOps = 100

	// Spawn goroutines that register and get contexts concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				ctx := &EncodingContext{
					ASN4:    (seed+j)%2 == 0,
					LocalAS: uint32(seed % 65536),     //nolint:gosec // test-only, no security risk
					PeerAS:  uint32((j % 10) + 65000), //nolint:gosec // test-only, no security risk
				}
				id := r.Register(ctx)
				retrieved := r.Get(id)
				if retrieved == nil {
					t.Errorf("Get returned nil for just-registered context")
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestRegistryCount verifies the registry tracks context count correctly.
//
// VALIDATES: Count returns correct number of unique contexts.
//
// PREVENTS: Memory leaks from untracked contexts.
func TestRegistryCount(t *testing.T) {
	r := NewRegistry()

	if r.Count() != 0 {
		t.Errorf("new registry should have 0 contexts, got %d", r.Count())
	}

	ctx1 := &EncodingContext{LocalAS: 65000}
	ctx2 := &EncodingContext{LocalAS: 65001}
	ctx3 := &EncodingContext{LocalAS: 65000} // Same as ctx1

	r.Register(ctx1)
	if r.Count() != 1 {
		t.Errorf("after 1 unique context, count should be 1, got %d", r.Count())
	}

	r.Register(ctx2)
	if r.Count() != 2 {
		t.Errorf("after 2 unique contexts, count should be 2, got %d", r.Count())
	}

	r.Register(ctx3) // Duplicate
	if r.Count() != 2 {
		t.Errorf("after duplicate, count should still be 2, got %d", r.Count())
	}
}
