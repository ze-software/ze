package rib

import (
	"bytes"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// VALIDATES: Identical attributes return the same interned value; different attributes differ.
// PREVENTS: Deduplication failure causing memory bloat.
func TestRouteStore_InternAttribute(t *testing.T) {
	store := newRouteStore(10)
	defer store.Stop()

	// Create two identical attributes
	a1 := attribute.LocalPref(100)
	a2 := attribute.LocalPref(100)

	// Intern both
	r1 := store.internAttribute(a1)
	r2 := store.internAttribute(a2)

	// Should return same value
	if r1 != r2 {
		t.Error("identical attributes should return same interned value")
	}

	// Different attribute
	a3 := attribute.LocalPref(200)
	r3 := store.internAttribute(a3)

	if r1 == r3 {
		t.Error("different attributes should return different values")
	}
}

// VALIDATES: Identical NLRIs return equal interned bytes.
// PREVENTS: NLRI deduplication failure in route store.
func TestRouteStore_InternNLRI(t *testing.T) {
	store := newRouteStore(10)
	defer store.Stop()

	// Create two identical NLRIs
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	n1 := nlri.NewINET(family.IPv4Unicast, prefix, 0)
	n2 := nlri.NewINET(family.IPv4Unicast, prefix, 0)

	// Intern both
	r1 := store.internNLRI(n1)
	r2 := store.internNLRI(n2)

	// Should return equal NLRIs (same bytes)
	if !bytes.Equal(r1.Bytes(), r2.Bytes()) {
		t.Error("identical NLRIs should return same bytes")
	}
}

// VALIDATES: Identical routes return same interned route with correct reference count.
// PREVENTS: Route deduplication failure or wrong reference counting.
func TestRouteStore_InternRoute(t *testing.T) {
	store := newRouteStore(10)
	defer store.Stop()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nextHop := netip.MustParseAddr("192.168.1.1")

	n := nlri.NewINET(family.IPv4Unicast, prefix, 0)
	attrs := []attribute.Attribute{
		attribute.LocalPref(100),
	}

	r1 := NewRoute(n, nextHop, attrs)
	r2 := NewRoute(n, nextHop, attrs)

	// Intern both routes
	ir1 := store.internRoute(r1)
	ir2 := store.internRoute(r2)

	// Should return same route (reference counted)
	if ir1 != ir2 {
		t.Error("identical routes should return same interned route")
	}

	// Reference count should be 2
	if ir1.RefCount() != 2 {
		t.Errorf("expected refCount=2, got %d", ir1.RefCount())
	}
}

// VALIDATES: Route is removed only after all references are released.
// PREVENTS: Premature route eviction or leaked routes after release.
func TestRouteStore_ReleaseRoute(t *testing.T) {
	store := newRouteStore(10)
	defer store.Stop()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nextHop := netip.MustParseAddr("192.168.1.1")

	n := nlri.NewINET(family.IPv4Unicast, prefix, 0)
	r := NewRoute(n, nextHop, nil)

	// Intern twice
	ir1 := store.internRoute(r)
	ir2 := store.internRoute(r)

	stats := store.Stats()
	if stats.Routes != 1 {
		t.Errorf("expected 1 route, got %d", stats.Routes)
	}

	// Release once
	store.releaseRoute(ir1)
	stats = store.Stats()
	if stats.Routes != 1 {
		t.Errorf("expected 1 route after first release, got %d", stats.Routes)
	}

	// Release again (should remove)
	store.releaseRoute(ir2)
	stats = store.Stats()
	if stats.Routes != 0 {
		t.Errorf("expected 0 routes after second release, got %d", stats.Routes)
	}
}

// VALIDATES: Stats correctly report route count, NLRI families, and attribute types.
// PREVENTS: Wrong statistics in monitoring or diagnostics output.
func TestRouteStore_Stats(t *testing.T) {
	store := newRouteStore(10)
	defer store.Stop()

	// Add some routes
	for i := range 5 {
		prefix := netip.MustParsePrefix("10.0.0.0/24")
		nextHop := netip.MustParseAddr("192.168.1.1")
		n := nlri.NewINET(family.IPv4Unicast, prefix, uint32(i)) //nolint:gosec // Test data
		attrs := []attribute.Attribute{
			attribute.LocalPref(uint32(100 + i)), //nolint:gosec // Test data
		}
		r := NewRoute(n, nextHop, attrs)
		store.internRoute(r)
	}

	stats := store.Stats()

	if stats.Routes != 5 {
		t.Errorf("expected 5 routes, got %d", stats.Routes)
	}
	if stats.NLRIFamilies < 1 {
		t.Errorf("expected at least 1 NLRI family, got %d", stats.NLRIFamilies)
	}
	if stats.AttributeTypes < 1 {
		t.Errorf("expected at least 1 attribute type, got %d", stats.AttributeTypes)
	}
}

func BenchmarkRouteStore_InternAttribute(b *testing.B) {
	store := newRouteStore(1000)
	defer store.Stop()

	attr := attribute.LocalPref(100)

	for b.Loop() {
		store.internAttribute(attr)
	}
}

func BenchmarkRouteStore_InternRoute(b *testing.B) {
	store := newRouteStore(1000)
	defer store.Stop()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nextHop := netip.MustParseAddr("192.168.1.1")
	n := nlri.NewINET(family.IPv4Unicast, prefix, 0)
	attrs := []attribute.Attribute{
		attribute.LocalPref(100),
	}
	route := NewRoute(n, nextHop, attrs)

	for b.Loop() {
		store.internRoute(route)
	}
}

// VALIDATES: a route another caller still holds is never removed from the store.
// internRoute finds an entry and acquires it under routesMu, so releaseRoute must
// decide under that same lock. Deciding outside it lets an intern take the count
// back to 1 on a route already condemned, and the map entry is then deleted under
// a live holder.
// PREVENTS: interned components returning to their stores while a caller holds a
// route that still points at them.
// Producer: releaseRoute (store.go).
func TestReleaseRouteCannotDropARouteAnotherInternHolds(t *testing.T) {
	store := newRouteStore(10)
	defer store.Stop()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nextHop := netip.MustParseAddr("192.168.1.1")
	n := nlri.NewINET(family.IPv4Unicast, prefix, 0)
	r := NewRoute(n, nextHop, []attribute.Attribute{attribute.LocalPref(100)})

	// Resolve the key once. Index() fills a lazy cache on the Route it is called
	// on, so reading it from the interned route inside the goroutines would race
	// with releaseRoute's own call.
	idx := string(r.Index())

	const goroutines = 4
	const iterations = 3000

	var wg sync.WaitGroup
	missing := make(chan struct{}, 1)

	for range goroutines {
		wg.Go(func() {
			for range iterations {
				held := store.internRoute(r)

				// This goroutine holds a reference, so the entry cannot legally
				// be gone: only a decrement to zero may remove it.
				store.routesMu.RLock()
				_, ok := store.routes[idx]
				store.routesMu.RUnlock()
				if !ok {
					select {
					case missing <- struct{}{}:
					default:
					}
				}

				store.releaseRoute(held)
			}
		})
	}
	wg.Wait()

	select {
	case <-missing:
		t.Fatal("a route was removed from the store while a caller held a reference to it")
	default:
	}

	if stats := store.Stats(); stats.Routes != 0 {
		t.Errorf("after balanced intern and release: %d routes remain, want 0", stats.Routes)
	}
}
