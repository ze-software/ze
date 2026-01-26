package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/pool"
)

// TestFamilyRIB_PerAttrDedup verifies per-attribute deduplication.
//
// VALIDATES: Routes with same ORIGIN/LOCAL_PREF but different MED share common attrs.
// PREVENTS: Full blob duplication when only one attribute differs.
func TestFamilyRIB_PerAttrDedup(t *testing.T) {
	rib := NewFamilyRIBPerAttr(nlri.IPv4Unicast, false)
	defer rib.Release()

	// Two routes with same ORIGIN and LOCAL_PREF but different MED.
	// ORIGIN=IGP, LOCAL_PREF=100, MED=10.
	attrs1 := concat(wireOriginIGP, wireLocalPref100, wireMED100)
	// ORIGIN=IGP, LOCAL_PREF=100, MED=20.
	wireMED20 := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x14}
	attrs2 := concat(wireOriginIGP, wireLocalPref100, wireMED20)

	nlri1 := []byte{24, 10, 0, 0} // 10.0.0.0/24
	nlri2 := []byte{24, 10, 0, 1} // 10.0.1.0/24

	rib.Insert(attrs1, nlri1)
	rib.Insert(attrs2, nlri2)

	// Lookup both routes.
	entry1, ok := rib.LookupEntry(nlri1)
	require.True(t, ok, "route 1 should exist")

	entry2, ok := rib.LookupEntry(nlri2)
	require.True(t, ok, "route 2 should exist")

	// ORIGIN and LOCAL_PREF should share pool slots (same values).
	assert.Equal(t, entry1.Origin.Slot(), entry2.Origin.Slot(),
		"ORIGIN should share pool slot")
	assert.Equal(t, entry1.LocalPref.Slot(), entry2.LocalPref.Slot(),
		"LOCAL_PREF should share pool slot")

	// MED should have different slots (different values).
	assert.NotEqual(t, entry1.MED.Slot(), entry2.MED.Slot(),
		"MED should have different pool slots")
}

// TestFamilyRIBPerAttr_Insert verifies basic insert with per-attr storage.
//
// VALIDATES: Insert parses attributes and stores RouteEntry.
// PREVENTS: Insert failing or not using per-attr pools.
func TestFamilyRIBPerAttr_Insert(t *testing.T) {
	rib := NewFamilyRIBPerAttr(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	nlriBytes := []byte{24, 192, 168, 1} // 192.168.1.0/24

	rib.Insert(attrs, nlriBytes)

	assert.Equal(t, 1, rib.Len(), "should have 1 route")

	entry, ok := rib.LookupEntry(nlriBytes)
	require.True(t, ok)
	assert.True(t, entry.HasOrigin())
	assert.True(t, entry.HasASPath())
	assert.True(t, entry.HasNextHop())
}

// TestFamilyRIBPerAttr_ImplicitWithdraw verifies implicit withdraw behavior.
//
// VALIDATES: Same NLRI with new attrs releases old entry.
// PREVENTS: Memory leak from unreleased old RouteEntry.
func TestFamilyRIBPerAttr_ImplicitWithdraw(t *testing.T) {
	rib := NewFamilyRIBPerAttr(nlri.IPv4Unicast, false)
	defer rib.Release()

	nlriBytes := []byte{24, 10, 0, 0} // 10.0.0.0/24

	// First insert with MED=10.
	attrs1 := concat(wireOriginIGP, wireMED100)
	rib.Insert(attrs1, nlriBytes)

	entry1, ok := rib.LookupEntry(nlriBytes)
	require.True(t, ok)
	// Save slot values before implicit withdraw releases the entry.
	origin1Slot := entry1.Origin.Slot()
	med1Slot := entry1.MED.Slot()

	// Second insert with MED=20 (implicit withdraw).
	wireMED20 := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x14}
	attrs2 := concat(wireOriginIGP, wireMED20)
	rib.Insert(attrs2, nlriBytes)

	entry2, ok := rib.LookupEntry(nlriBytes)
	require.True(t, ok)

	// ORIGIN should share pool slot (same value interned twice).
	assert.Equal(t, origin1Slot, entry2.Origin.Slot(),
		"ORIGIN should share pool slot after implicit withdraw")

	// MED should be different (different values).
	assert.NotEqual(t, med1Slot, entry2.MED.Slot(),
		"MED should have different slot after implicit withdraw")

	// Still only 1 route.
	assert.Equal(t, 1, rib.Len())
}

// TestFamilyRIBPerAttr_Remove verifies route removal.
//
// VALIDATES: Remove releases RouteEntry handles.
// PREVENTS: Memory leak from unreleased handles on remove.
func TestFamilyRIBPerAttr_Remove(t *testing.T) {
	rib := NewFamilyRIBPerAttr(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0}

	rib.Insert(attrs, nlriBytes)
	assert.Equal(t, 1, rib.Len())

	removed := rib.Remove(nlriBytes)
	assert.True(t, removed)
	assert.Equal(t, 0, rib.Len())

	_, ok := rib.LookupEntry(nlriBytes)
	assert.False(t, ok, "route should not exist after remove")
}

// TestFamilyRIBPerAttr_IterateEntry verifies iteration over entries.
//
// VALIDATES: IterateEntry visits all routes with their RouteEntry.
// PREVENTS: Missing routes during iteration.
func TestFamilyRIBPerAttr_IterateEntry(t *testing.T) {
	rib := NewFamilyRIBPerAttr(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	nlri1 := []byte{24, 10, 0, 0}
	nlri2 := []byte{24, 10, 0, 1}

	rib.Insert(attrs, nlri1)
	rib.Insert(attrs, nlri2)

	var count int
	rib.IterateEntry(func(nlriBytes []byte, entry *RouteEntry) bool {
		count++
		assert.True(t, entry.HasOrigin())
		assert.True(t, entry.HasLocalPref())
		return true
	})

	assert.Equal(t, 2, count, "should iterate 2 routes")
}

// TestFamilyRIBPerAttr_NoOpUpdate verifies same attrs don't create duplicates.
//
// VALIDATES: Same NLRI+attrs = no-op (no extra pool refs).
// PREVENTS: Pool ref leaks from redundant updates.
func TestFamilyRIBPerAttr_NoOpUpdate(t *testing.T) {
	rib := NewFamilyRIBPerAttr(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0}

	// Insert twice with same data.
	rib.Insert(attrs, nlriBytes)
	entry1, _ := rib.LookupEntry(nlriBytes)
	originSlot1 := entry1.Origin.Slot()

	rib.Insert(attrs, nlriBytes)
	entry2, _ := rib.LookupEntry(nlriBytes)

	// Should be same entry (or at least same slots).
	assert.Equal(t, originSlot1, entry2.Origin.Slot())
	assert.Equal(t, 1, rib.Len())
}

// TestFamilyRIBPerAttr_ToWireBytes verifies wire reconstruction.
//
// VALIDATES: RouteEntry can be reconstructed to valid wire format.
// PREVENTS: Data loss during storage/reconstruction cycle.
func TestFamilyRIBPerAttr_ToWireBytes(t *testing.T) {
	rib := NewFamilyRIBPerAttr(nlri.IPv4Unicast, false)
	defer rib.Release()

	// Insert with known attributes.
	attrs := concat(wireOriginIGP, wireLocalPref100, wireMED100)
	nlriBytes := []byte{24, 10, 0, 0}

	rib.Insert(attrs, nlriBytes)

	entry, ok := rib.LookupEntry(nlriBytes)
	require.True(t, ok)

	// Reconstruct wire bytes.
	reconstructed, err := entry.ToWireBytes()
	require.NoError(t, err)

	// Should contain ORIGIN, LOCAL_PREF, MED.
	// Parse reconstructed to verify.
	entry2, err := ParseAttributes(reconstructed)
	require.NoError(t, err)
	defer entry2.Release()

	// Verify values match by comparing pool data.
	origData1, _ := pool.Origin.Get(entry.Origin)
	origData2, _ := pool.Origin.Get(entry2.Origin)
	assert.Equal(t, origData1, origData2, "ORIGIN should match")

	lpData1, _ := pool.LocalPref.Get(entry.LocalPref)
	lpData2, _ := pool.LocalPref.Get(entry2.LocalPref)
	assert.Equal(t, lpData1, lpData2, "LOCAL_PREF should match")

	medData1, _ := pool.MED.Get(entry.MED)
	medData2, _ := pool.MED.Get(entry2.MED)
	assert.Equal(t, medData1, medData2, "MED should match")
}
