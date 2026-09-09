package context

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRegistryRegister_NewContext verifies that Register returns a new ID.
//
// VALIDATES: Register assigns unique IDs to new contexts.
//
// PREVENTS: ID collisions between distinct contexts.
func TestRegistryRegister_NewContext(t *testing.T) {
	r := NewRegistry()

	ctx1 := NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true},
		DirectionRecv,
	)

	ctx2 := NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65002, PeerASN: 65003},
		&capability.EncodingCaps{ASN4: false},
		DirectionRecv,
	)

	id1, err1 := r.Register(ctx1)
	id2, err2 := r.Register(ctx2)

	if err1 != nil || err2 != nil {
		t.Fatalf("Register failed: err1=%v, err2=%v", err1, err2)
	}
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

	identity := &capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001}
	encoding := &capability.EncodingCaps{
		ASN4: true,
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: capability.AddPathBoth,
		},
	}

	ctx1 := NewEncodingContext(identity, encoding, DirectionRecv)
	ctx2 := NewEncodingContext(identity, encoding, DirectionRecv)

	id1, err1 := r.Register(ctx1)
	id2, err2 := r.Register(ctx2)

	if err1 != nil || err2 != nil {
		t.Fatalf("Register failed: err1=%v, err2=%v", err1, err2)
	}
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

	ctx := NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true},
		DirectionRecv,
	)

	id, err := r.Register(ctx)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	retrieved := r.Get(id)

	if retrieved == nil {
		t.Fatal("Get returned nil for registered context")
		return
	}

	if retrieved.ASN4() != ctx.ASN4() {
		t.Errorf("ASN4 mismatch: got %v, want %v", retrieved.ASN4(), ctx.ASN4())
	}
	if retrieved.LocalASN() != ctx.LocalASN() {
		t.Errorf("LocalASN mismatch: got %d, want %d", retrieved.LocalASN(), ctx.LocalASN())
	}
	if retrieved.PeerASN() != ctx.PeerASN() {
		t.Errorf("PeerASN mismatch: got %d, want %d", retrieved.PeerASN(), ctx.PeerASN())
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
	for i := range numGoroutines {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for j := range numOps {
				ctx := NewEncodingContext(
					&capability.PeerIdentity{
						LocalASN: uint32(seed % 65536),     //nolint:gosec // test-only
						PeerASN:  uint32((j % 10) + 65000), //nolint:gosec // test-only
					},
					&capability.EncodingCaps{ASN4: (seed+j)%2 == 0},
					DirectionRecv,
				)
				id, err := r.Register(ctx)
				if err != nil {
					t.Errorf("Register failed: %v", err)
					return
				}
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

	ctx1 := NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		nil,
		DirectionRecv,
	)
	ctx2 := NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002},
		nil,
		DirectionRecv,
	)
	// ctx3 has same params as ctx1, should deduplicate
	ctx3 := NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		nil,
		DirectionRecv,
	)

	if _, err := r.Register(ctx1); err != nil {
		t.Fatalf("Register ctx1: %v", err)
	}
	if r.Count() != 1 {
		t.Errorf("after 1 unique context, count should be 1, got %d", r.Count())
	}

	if _, err := r.Register(ctx2); err != nil {
		t.Fatalf("Register ctx2: %v", err)
	}
	if r.Count() != 2 {
		t.Errorf("after 2 unique contexts, count should be 2, got %d", r.Count())
	}

	if _, err := r.Register(ctx3); err != nil {
		t.Fatalf("Register ctx3: %v", err)
	}
	if r.Count() != 2 {
		t.Errorf("after duplicate, count should still be 2, got %d", r.Count())
	}
}

// TestEncodingContextIDDiffersOnASN4Alone holds the property the receive-side
// ASN4 relabel rests on: two contexts that agree on everything except the AS
// number width are two contexts, never one.
//
// The RFC 6793 ingest reconciliation rewrites a received UPDATE to four octets
// and relabels it with fwdContextIDWithASN4(recvCtxID, true)
// (internal/component/bgp/reactor/forward_context.go). If the registry
// deduplicated the pair, that relabel would hand the collapsed payload the
// SAME id the two-octet session reads, and every consumer would decode a
// four-octet AS_PATH at two octets.
//
// VALIDATES: A-2 of spec-forwarded-as-path-obeys-rfc6793-for-every-destination.
// PREVENTS: a silent width collision. Nothing downstream compares widths, so
// this failure would surface as wrong AS numbers rather than as an error.
func TestEncodingContextIDDiffersOnASN4Alone(t *testing.T) {
	r := NewRegistry()

	identity := &capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001}
	addPath := map[capability.Family]capability.AddPathMode{
		{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: capability.AddPathBoth,
	}

	narrow := NewEncodingContext(identity,
		&capability.EncodingCaps{ASN4: false, AddPathMode: addPath}, DirectionRecv)
	wide := NewEncodingContext(identity,
		&capability.EncodingCaps{ASN4: true, AddPathMode: addPath}, DirectionRecv)

	narrowID, err := r.Register(narrow)
	if err != nil {
		t.Fatalf("register the two-octet context: %v", err)
	}
	wideID, err := r.Register(wide)
	if err != nil {
		t.Fatalf("register the four-octet context: %v", err)
	}

	if narrowID == wideID {
		t.Fatalf("contexts differing only in ASN4 share id %d, so the relabel cannot distinguish them", narrowID)
	}
	if got := r.Get(narrowID); got == nil || got.ASN4() {
		t.Errorf("the two-octet id must resolve to a two-octet context")
	}
	if got := r.Get(wideID); got == nil || !got.ASN4() {
		t.Errorf("the four-octet id must resolve to a four-octet context")
	}
}
