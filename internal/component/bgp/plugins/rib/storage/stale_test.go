package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestRouteEntry_StaleLevelDefault verifies new RouteEntry has StaleLevel=0 (fresh).
//
// VALIDATES: New routes are fresh (not stale at any level).
// PREVENTS: New routes incorrectly starting as stale.
func TestRouteEntry_StaleLevelDefault(t *testing.T) {
	t.Parallel()

	entry := NewRouteEntry()
	assert.Equal(t, StaleLevelFresh, entry.StaleLevel, "new RouteEntry should have StaleLevel=0")
}

// TestFamilyRIB_MarkStale verifies MarkStale sets StaleLevel on all routes.
//
// VALIDATES: All routes marked at specified stale level after MarkStale.
// PREVENTS: Routes remaining fresh after mark-stale command.
func TestFamilyRIB_MarkStale(t *testing.T) {
	t.Parallel()

	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	nlri1 := []byte{24, 10, 0, 0}
	nlri2 := []byte{24, 10, 0, 1}
	nlri3 := []byte{24, 10, 0, 2}

	rib.Insert(attrs, nlri1)
	rib.Insert(attrs, nlri2)
	rib.Insert(attrs, nlri3)

	// Before marking: all fresh
	entry1, _ := rib.LookupEntry(nlri1)
	assert.Equal(t, StaleLevelFresh, entry1.StaleLevel)

	rib.MarkStale(1) // level 1 (GR-stale equivalent)

	// After marking: all at level 1
	entry1, _ = rib.LookupEntry(nlri1)
	entry2, _ := rib.LookupEntry(nlri2)
	entry3, _ := rib.LookupEntry(nlri3)
	assert.Equal(t, uint8(1), entry1.StaleLevel, "route 1 should be at level 1")
	assert.Equal(t, uint8(1), entry2.StaleLevel, "route 2 should be at level 1")
	assert.Equal(t, uint8(1), entry3.StaleLevel, "route 3 should be at level 1")

	// Total count unchanged
	assert.Equal(t, 3, rib.Len())
}

// TestFamilyRIB_MarkStaleHigherLevel verifies MarkStale at higher level (LLGR).
//
// VALIDATES: MarkStale can set level 2 (LLGR-stale equivalent).
// PREVENTS: Only level 1 working.
func TestFamilyRIB_MarkStaleHigherLevel(t *testing.T) {
	t.Parallel()

	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	rib.Insert(attrs, []byte{24, 10, 0, 0})

	rib.MarkStale(2) // level 2 (LLGR-stale equivalent)

	entry, _ := rib.LookupEntry([]byte{24, 10, 0, 0})
	assert.Equal(t, uint8(2), entry.StaleLevel)
}

// TestFamilyRIB_PurgeStale verifies PurgeStale deletes only stale routes.
//
// VALIDATES: Only stale routes (level > 0) deleted, fresh routes kept.
// PREVENTS: Fresh routes being incorrectly purged.
func TestFamilyRIB_PurgeStale(t *testing.T) {
	t.Parallel()

	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	staleNLRI := []byte{24, 10, 0, 0}
	freshNLRI := []byte{24, 10, 0, 1}

	// Insert two routes
	rib.Insert(attrs, staleNLRI)
	rib.Insert(attrs, freshNLRI)

	// Mark all as stale
	rib.MarkStale(1)

	// Insert fresh route (replaces stale for freshNLRI -- implicit unstale)
	wireMED20 := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x14}
	freshAttrs := concat(wireOriginIGP, wireLocalPref100, wireMED20)
	rib.Insert(freshAttrs, freshNLRI)

	// freshNLRI should be fresh (new entry replaces stale one)
	freshEntry, ok := rib.LookupEntry(freshNLRI)
	require.True(t, ok)
	assert.Equal(t, StaleLevelFresh, freshEntry.StaleLevel, "replaced route should be fresh")

	// Purge should only remove staleNLRI
	purged := rib.PurgeStale()
	assert.Equal(t, 1, purged, "should purge 1 stale route")
	assert.Equal(t, 1, rib.Len(), "should have 1 route remaining")

	// staleNLRI gone, freshNLRI remains
	_, ok = rib.LookupEntry(staleNLRI)
	assert.False(t, ok, "stale route should be gone")

	_, ok = rib.LookupEntry(freshNLRI)
	assert.True(t, ok, "fresh route should remain")
}

// TestFamilyRIB_PurgeStaleEmpty verifies PurgeStale with no stale routes.
//
// VALIDATES: PurgeStale is a no-op when no routes are stale.
// PREVENTS: Incorrect deletion of fresh routes.
func TestFamilyRIB_PurgeStaleEmpty(t *testing.T) {
	t.Parallel()

	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	rib.Insert(attrs, []byte{24, 10, 0, 0})
	rib.Insert(attrs, []byte{24, 10, 0, 1})

	purged := rib.PurgeStale()
	assert.Equal(t, 0, purged, "no routes should be purged")
	assert.Equal(t, 2, rib.Len(), "all routes should remain")
}

// TestFamilyRIB_InsertClearsStale verifies Insert replaces stale route with fresh.
//
// VALIDATES: INSERT after mark-stale clears StaleLevel to fresh.
// PREVENTS: Updated routes remaining marked as stale.
func TestFamilyRIB_InsertClearsStale(t *testing.T) {
	t.Parallel()

	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0}

	rib.Insert(attrs, nlriBytes)
	rib.MarkStale(1)

	entry, _ := rib.LookupEntry(nlriBytes)
	assert.Equal(t, uint8(1), entry.StaleLevel, "should be stale after mark")

	// Re-insert with different attrs (implicit withdraw + fresh insert)
	wireMED20 := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x14}
	newAttrs := concat(wireOriginIGP, wireLocalPref100, wireMED20)
	rib.Insert(newAttrs, nlriBytes)

	entry, _ = rib.LookupEntry(nlriBytes)
	assert.Equal(t, StaleLevelFresh, entry.StaleLevel, "should be fresh after re-insert")
}

// TestFamilyRIB_InsertNewDuringStale verifies new NLRI during GR window is fresh.
//
// VALIDATES: New routes during GR window are not stale.
// PREVENTS: Brand new routes being incorrectly marked stale.
func TestFamilyRIB_InsertNewDuringStale(t *testing.T) {
	t.Parallel()

	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)

	// Insert initial route, then mark stale
	rib.Insert(attrs, []byte{24, 10, 0, 0})
	rib.MarkStale(1)

	// Insert a NEW route (not replacing an existing one)
	rib.Insert(attrs, []byte{24, 10, 0, 1})

	newEntry, ok := rib.LookupEntry([]byte{24, 10, 0, 1})
	require.True(t, ok)
	assert.Equal(t, StaleLevelFresh, newEntry.StaleLevel, "new route should be fresh")

	oldEntry, _ := rib.LookupEntry([]byte{24, 10, 0, 0})
	assert.Equal(t, uint8(1), oldEntry.StaleLevel, "old route should still be stale")
}

// TestFamilyRIB_StaleCount verifies stale route counting.
//
// VALIDATES: StaleCount returns correct count of routes with StaleLevel > 0.
// PREVENTS: Incorrect count in bgp rib status output.
func TestFamilyRIB_StaleCount(t *testing.T) {
	t.Parallel()

	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	rib.Insert(attrs, []byte{24, 10, 0, 0})
	rib.Insert(attrs, []byte{24, 10, 0, 1})
	rib.Insert(attrs, []byte{24, 10, 0, 2})

	assert.Equal(t, 0, rib.StaleCount(), "no stale routes initially")

	rib.MarkStale(1)
	assert.Equal(t, 3, rib.StaleCount(), "all routes stale after mark")

	// Insert fresh replacement for one
	wireMED20 := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x14}
	rib.Insert(concat(wireOriginIGP, wireLocalPref100, wireMED20), []byte{24, 10, 0, 1})
	assert.Equal(t, 2, rib.StaleCount(), "2 stale after one refreshed")

	rib.PurgeStale()
	assert.Equal(t, 0, rib.StaleCount(), "0 stale after purge")
	assert.Equal(t, 1, rib.Len(), "1 fresh route remains")
}

// TestPeerRIB_MarkFamilyStale verifies per-family stale marking.
//
// VALIDATES: MarkFamilyStale only affects specified family.
// PREVENTS: Cross-family stale marking.
func TestPeerRIB_MarkFamilyStale(t *testing.T) {
	t.Parallel()

	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	v4prefix := []byte{24, 10, 0, 0}
	v6prefix := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}

	rib.Insert(family.IPv4Unicast, attrs, v4prefix)
	rib.Insert(family.IPv6Unicast, attrs, v6prefix)

	rib.MarkFamilyStale(family.IPv4Unicast, 1)

	// IPv4 should be stale
	v4entry, ok := rib.Lookup(family.IPv4Unicast, v4prefix)
	require.True(t, ok)
	assert.Equal(t, uint8(1), v4entry.StaleLevel, "IPv4 route should be stale")

	// IPv6 should be fresh
	v6entry, ok := rib.Lookup(family.IPv6Unicast, v6prefix)
	require.True(t, ok)
	assert.Equal(t, StaleLevelFresh, v6entry.StaleLevel, "IPv6 route should be fresh")
}

// TestPeerRIB_MarkAllStale verifies all-family stale marking.
//
// VALIDATES: MarkAllStale marks all families at specified level.
// PREVENTS: Some families remaining fresh.
func TestPeerRIB_MarkAllStale(t *testing.T) {
	t.Parallel()

	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}

	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(family.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	rib.MarkAllStale(1)

	v4entry, _ := rib.Lookup(family.IPv4Unicast, []byte{24, 10, 0, 0})
	v6entry, _ := rib.Lookup(family.IPv6Unicast, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	assert.Equal(t, uint8(1), v4entry.StaleLevel, "IPv4 should be stale")
	assert.Equal(t, uint8(1), v6entry.StaleLevel, "IPv6 should be stale")
}

// TestPeerRIB_PurgeFamilyStale verifies per-family stale purge.
//
// VALIDATES: Only stale routes in specified family deleted.
// PREVENTS: Cross-family purge.
func TestPeerRIB_PurgeFamilyStale(t *testing.T) {
	t.Parallel()

	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}

	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 1})
	rib.Insert(family.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	// Mark all stale, then refresh one IPv4 route
	rib.MarkAllStale(1)
	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0}) // refresh

	// Purge only IPv4 stale -- should remove 10.0.1.0/24 but keep 10.0.0.0/24
	purged := rib.PurgeFamilyStale(family.IPv4Unicast)
	assert.Equal(t, 1, purged, "should purge 1 stale IPv4 route")

	// IPv4: 1 route remains (refreshed one)
	assert.Equal(t, 1, rib.FamilyLen(family.IPv4Unicast))

	// IPv6: still stale, untouched
	assert.Equal(t, 1, rib.FamilyLen(family.IPv6Unicast))
	v6entry, _ := rib.Lookup(family.IPv6Unicast, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})
	assert.Equal(t, uint8(1), v6entry.StaleLevel, "IPv6 should still be stale")
}

// TestPeerRIB_PurgeAllStale verifies all-family stale purge.
//
// VALIDATES: Purge all stale routes across families.
// PREVENTS: Stale routes remaining after full purge.
func TestPeerRIB_PurgeAllStale(t *testing.T) {
	t.Parallel()

	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}

	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 1})
	rib.Insert(family.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	rib.MarkAllStale(1)

	// Refresh one IPv4 route
	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0})

	purged := rib.PurgeAllStale()
	assert.Equal(t, 2, purged, "should purge 2 stale routes")
	assert.Equal(t, 1, rib.Len(), "1 fresh route remains")

	// The refreshed one survives
	_, ok := rib.Lookup(family.IPv4Unicast, []byte{24, 10, 0, 0})
	assert.True(t, ok, "refreshed route should remain")
}

// TestPeerRIB_StaleCount verifies stale count across families.
//
// VALIDATES: Total stale count correct.
// PREVENTS: Incorrect count in bgp rib status.
func TestPeerRIB_StaleCount(t *testing.T) {
	t.Parallel()

	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}

	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 1})
	rib.Insert(family.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	assert.Equal(t, 0, rib.StaleCount())

	rib.MarkAllStale(1)
	assert.Equal(t, 3, rib.StaleCount())

	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0}) // refresh
	assert.Equal(t, 2, rib.StaleCount())
}

// TestPeerRIB_MarkFamilyStaleNonExistent verifies marking non-existent family is no-op.
//
// VALIDATES: MarkFamilyStale on absent family doesn't crash.
// PREVENTS: Crash on mark-stale for family with no routes.
func TestPeerRIB_MarkFamilyStaleNonExistent(t *testing.T) {
	t.Parallel()

	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	// Should not crash
	rib.MarkFamilyStale(family.IPv4Unicast, 1)
	assert.Equal(t, 0, rib.StaleCount())
}

// TestPeerRIB_PurgeFamilyStaleNonExistent verifies purging non-existent family is no-op.
//
// VALIDATES: PurgeFamilyStale on absent family returns 0.
// PREVENTS: Crash on purge-stale for family with no routes.
func TestPeerRIB_PurgeFamilyStaleNonExistent(t *testing.T) {
	t.Parallel()

	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	purged := rib.PurgeFamilyStale(family.IPv4Unicast)
	assert.Equal(t, 0, purged)
}
