package rib

import (
	"bytes"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// VALIDATES: Identical attributes return the same interned value; different attributes differ.
// PREVENTS: Deduplication failure causing memory bloat.
func TestRouteStore_InternAttribute(t *testing.T) {
	store := NewRouteStore(10)
	defer store.Stop()

	// Create two identical attributes
	a1 := attribute.LocalPref(100)
	a2 := attribute.LocalPref(100)

	// Intern both
	r1 := store.InternAttribute(a1)
	r2 := store.InternAttribute(a2)

	// Should return same value
	if r1 != r2 {
		t.Error("identical attributes should return same interned value")
	}

	// Different attribute
	a3 := attribute.LocalPref(200)
	r3 := store.InternAttribute(a3)

	if r1 == r3 {
		t.Error("different attributes should return different values")
	}
}

// VALIDATES: Identical NLRIs return equal interned bytes.
// PREVENTS: NLRI deduplication failure in route store.
func TestRouteStore_InternNLRI(t *testing.T) {
	store := NewRouteStore(10)
	defer store.Stop()

	// Create two identical NLRIs
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	n1 := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
	n2 := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)

	// Intern both
	r1 := store.InternNLRI(n1)
	r2 := store.InternNLRI(n2)

	// Should return equal NLRIs (same bytes)
	if !bytes.Equal(r1.Bytes(), r2.Bytes()) {
		t.Error("identical NLRIs should return same bytes")
	}
}

// VALIDATES: Identical routes return same interned route with correct reference count.
// PREVENTS: Route deduplication failure or wrong reference counting.
func TestRouteStore_InternRoute(t *testing.T) {
	store := NewRouteStore(10)
	defer store.Stop()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nextHop := netip.MustParseAddr("192.168.1.1")

	n := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
	attrs := []attribute.Attribute{
		attribute.LocalPref(100),
	}

	r1 := NewRoute(n, nextHop, attrs)
	r2 := NewRoute(n, nextHop, attrs)

	// Intern both routes
	ir1 := store.InternRoute(r1)
	ir2 := store.InternRoute(r2)

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
	store := NewRouteStore(10)
	defer store.Stop()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nextHop := netip.MustParseAddr("192.168.1.1")

	n := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
	r := NewRoute(n, nextHop, nil)

	// Intern twice
	ir1 := store.InternRoute(r)
	ir2 := store.InternRoute(r)

	stats := store.Stats()
	if stats.Routes != 1 {
		t.Errorf("expected 1 route, got %d", stats.Routes)
	}

	// Release once
	store.ReleaseRoute(ir1)
	stats = store.Stats()
	if stats.Routes != 1 {
		t.Errorf("expected 1 route after first release, got %d", stats.Routes)
	}

	// Release again (should remove)
	store.ReleaseRoute(ir2)
	stats = store.Stats()
	if stats.Routes != 0 {
		t.Errorf("expected 0 routes after second release, got %d", stats.Routes)
	}
}

// VALIDATES: Stats correctly report route count, NLRI families, and attribute types.
// PREVENTS: Wrong statistics in monitoring or diagnostics output.
func TestRouteStore_Stats(t *testing.T) {
	store := NewRouteStore(10)
	defer store.Stop()

	// Add some routes
	for i := range 5 {
		prefix := netip.MustParsePrefix("10.0.0.0/24")
		nextHop := netip.MustParseAddr("192.168.1.1")
		n := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, uint32(i)) //nolint:gosec // Test data
		attrs := []attribute.Attribute{
			attribute.LocalPref(uint32(100 + i)), //nolint:gosec // Test data
		}
		r := NewRoute(n, nextHop, attrs)
		store.InternRoute(r)
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
	store := NewRouteStore(1000)
	defer store.Stop()

	attr := attribute.LocalPref(100)

	for b.Loop() {
		store.InternAttribute(attr)
	}
}

func BenchmarkRouteStore_InternRoute(b *testing.B) {
	store := NewRouteStore(1000)
	defer store.Stop()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nextHop := netip.MustParseAddr("192.168.1.1")
	n := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
	attrs := []attribute.Attribute{
		attribute.LocalPref(100),
	}
	route := NewRoute(n, nextHop, attrs)

	for b.Loop() {
		store.InternRoute(route)
	}
}
