package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	attrpool "github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/family"
)

// TestFamilyRIB_OpaqueNonCIDR exercises the non-CIDR opaque-map backend:
// FamilyRIB accepts arbitrary NLRI wire bytes (EVPN in this test), stores
// them keyed by the full byte sequence, and survives Insert/Lookup/Remove/
// Iterate/MarkStale/PurgeStale without a netip.Prefix round-trip.
//
// VALIDATES: non-CIDR families (EVPN, flowspec, VPN, MVPN, RTC, bgp-ls)
// have functional storage in FamilyRIB.
// PREVENTS: the Phase-2 regression where any non-CIDR NLRI was silently
// dropped because NLRIToPrefix returned ok=false.
func TestFamilyRIB_OpaqueNonCIDR(t *testing.T) {
	// AFI=L2VPN, SAFI=EVPN is the canonical non-CIDR family.
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	rib := newFamilyRIB(fam, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	// Two distinct EVPN NLRIs. Bytes are arbitrary; the test doesn't care
	// about EVPN route-type semantics, only that different byte sequences
	// become different keys.
	nlri1 := []byte{0x02, 0x19, 0x01, 0x02, 0x03}
	nlri2 := []byte{0x02, 0x19, 0x01, 0x02, 0x04}

	rib.Insert(attrs, nlri1, true)
	rib.Insert(attrs, nlri2, true)
	assert.Equal(t, 2, rib.Len(), "two distinct opaque NLRIs stored")

	_, ok := rib.lookupEntry(nlri1)
	assert.True(t, ok)
	_, ok = rib.lookupEntry(nlri2)
	assert.True(t, ok)

	// Iterate yields both entries.
	seen := map[string]bool{}
	rib.IterateEntry(func(n []byte, _ RouteEntry) bool {
		seen[string(n)] = true
		return true
	})
	assert.Len(t, seen, 2, "Iterate must yield every opaque entry")

	// MarkStale + StaleCount + PurgeStale.
	rib.MarkStale(2)
	assert.Equal(t, 2, rib.StaleCount())
	assert.Equal(t, 2, rib.PurgeStale(), "both entries are stale")
	assert.Equal(t, 0, rib.Len())

	// Remove reports absence correctly.
	assert.False(t, rib.Remove(nlri1))
}

// TestFamilyRIB_OpaqueImplicitWithdraw verifies the same-attrs short-circuit
// and implicit withdraw on the opaque backend.
//
// VALIDATES: re-insert with identical attrs reuses the stored handles and
// clears the stale flag; different attrs release the old entry.
func TestFamilyRIB_OpaqueImplicitWithdraw(t *testing.T) {
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	rib := newFamilyRIB(fam, false)
	defer rib.Release()

	attrs1 := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	nlri := []byte{0x02, 0x19, 0x01, 0x02, 0x03}

	rib.Insert(attrs1, nlri, true)
	entry1, _ := rib.lookupEntry(nlri)
	originSlot := entry1.GetBundle().Origin.Slot()

	// Mark stale, then re-insert with identical attrs -- stale flag clears,
	// handles stay the same.
	rib.MarkStale(1)
	rib.Insert(attrs1, nlri, true)
	entry2, ok := rib.lookupEntry(nlri)
	require.True(t, ok)
	assert.Equal(t, StaleLevelFresh, entry2.StaleLevel, "stale flag cleared on re-insert")
	assert.Equal(t, originSlot, entry2.GetBundle().Origin.Slot(), "handles reused on identical attrs")
	assert.Equal(t, 1, rib.Len())
}

// TestFamilyRIB_PerAttrDedup verifies per-attribute deduplication.
//
// VALIDATES: Routes with same ORIGIN/LOCAL_PREF but different MED share common attrs.
// PREVENTS: Full blob duplication when only one attribute differs.
func TestFamilyRIB_PerAttrDedup(t *testing.T) {
	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	// Two routes with same ORIGIN and LOCAL_PREF but different MED.
	// ORIGIN=IGP, LOCAL_PREF=100, MED=10.
	attrs1 := concat(wireOriginIGP, wireLocalPref100, wireMED100)
	// ORIGIN=IGP, LOCAL_PREF=100, MED=20.
	wireMED20 := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x14}
	attrs2 := concat(wireOriginIGP, wireLocalPref100, wireMED20)

	nlri1 := []byte{24, 10, 0, 0} // 10.0.0.0/24
	nlri2 := []byte{24, 10, 0, 1} // 10.0.1.0/24

	rib.Insert(attrs1, nlri1, true)
	rib.Insert(attrs2, nlri2, true)

	// Lookup both routes.
	entry1, ok := rib.lookupEntry(nlri1)
	require.True(t, ok, "route 1 should exist")

	entry2, ok := rib.lookupEntry(nlri2)
	require.True(t, ok, "route 2 should exist")

	// ORIGIN and LOCAL_PREF should share pool slots (same values).
	assert.Equal(t, entry1.GetBundle().Origin.Slot(), entry2.GetBundle().Origin.Slot(),
		"ORIGIN should share pool slot")
	assert.Equal(t, entry1.GetBundle().LocalPref.Slot(), entry2.GetBundle().LocalPref.Slot(),
		"LOCAL_PREF should share pool slot")

	// MED should have different slots (different values).
	assert.NotEqual(t, entry1.GetBundle().MED.Slot(), entry2.GetBundle().MED.Slot(),
		"MED should have different pool slots")
}

// TestFamilyRIB_Insert verifies basic insert with per-attr storage.
//
// VALIDATES: Insert parses attributes and stores RouteEntry.
// PREVENTS: Insert failing or not using per-attr pools.
func TestFamilyRIB_Insert(t *testing.T) {
	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	nlriBytes := []byte{24, 192, 168, 1} // 192.168.1.0/24

	rib.Insert(attrs, nlriBytes, true)

	assert.Equal(t, 1, rib.Len(), "should have 1 route")

	entry, ok := rib.lookupEntry(nlriBytes)
	require.True(t, ok)
	assert.True(t, entry.GetBundle().HasOrigin())
	assert.True(t, entry.HasASPath())
	assert.True(t, entry.GetBundle().HasNextHop())
}

// TestFamilyRIB_InsertRejectsMalformedAttributes verifies malformed attributes are not installed.
//
// VALIDATES: Insert drops updates whose path attribute list cannot be parsed completely.
// PREVENTS: Remote malformed UPDATE bytes from creating routes with missing attributes.
func TestFamilyRIB_InsertRejectsMalformedAttributes(t *testing.T) {
	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x64, 0x00, 0x00} // ORIGIN length says 100, only 2 bytes present.
	nlriBytes := []byte{24, 192, 168, 1}

	rib.Insert(attrs, nlriBytes, true)

	assert.Equal(t, 0, rib.Len())
	_, ok := rib.lookupEntry(nlriBytes)
	assert.False(t, ok)
}

// TestFamilyRIB_ImplicitWithdraw verifies implicit withdraw behavior.
//
// VALIDATES: Same NLRI with new attrs releases old entry.
// PREVENTS: Memory leak from unreleased old RouteEntry.
func TestFamilyRIB_ImplicitWithdraw(t *testing.T) {
	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	nlriBytes := []byte{24, 10, 0, 0} // 10.0.0.0/24

	// First insert with MED=10.
	attrs1 := concat(wireOriginIGP, wireMED100)
	rib.Insert(attrs1, nlriBytes, true)

	entry1, ok := rib.lookupEntry(nlriBytes)
	require.True(t, ok)
	// Save slot values before implicit withdraw releases the entry.
	origin1Slot := entry1.GetBundle().Origin.Slot()
	med1Slot := entry1.GetBundle().MED.Slot()

	// Second insert with MED=20 (implicit withdraw).
	wireMED20 := []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x14}
	attrs2 := concat(wireOriginIGP, wireMED20)
	rib.Insert(attrs2, nlriBytes, true)

	entry2, ok := rib.lookupEntry(nlriBytes)
	require.True(t, ok)

	// ORIGIN should share pool slot (same value interned twice).
	assert.Equal(t, origin1Slot, entry2.GetBundle().Origin.Slot(),
		"ORIGIN should share pool slot after implicit withdraw")

	// MED should be different (different values).
	assert.NotEqual(t, med1Slot, entry2.GetBundle().MED.Slot(),
		"MED should have different slot after implicit withdraw")

	// Still only 1 route.
	assert.Equal(t, 1, rib.Len())
}

// TestFamilyRIB_Remove verifies route removal.
//
// VALIDATES: Remove releases RouteEntry handles.
// PREVENTS: Memory leak from unreleased handles on remove.
func TestFamilyRIB_Remove(t *testing.T) {
	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0}

	rib.Insert(attrs, nlriBytes, true)
	assert.Equal(t, 1, rib.Len())

	removed := rib.Remove(nlriBytes)
	assert.True(t, removed)
	assert.Equal(t, 0, rib.Len())

	_, ok := rib.lookupEntry(nlriBytes)
	assert.False(t, ok, "route should not exist after remove")
}

// TestFamilyRIB_IterateEntry verifies iteration over entries.
//
// VALIDATES: IterateEntry visits all routes with their RouteEntry.
// PREVENTS: Missing routes during iteration.
func TestFamilyRIB_IterateEntry(t *testing.T) {
	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	nlri1 := []byte{24, 10, 0, 0}
	nlri2 := []byte{24, 10, 0, 1}

	rib.Insert(attrs, nlri1, true)
	rib.Insert(attrs, nlri2, true)

	var count int
	rib.IterateEntry(func(nlriBytes []byte, entry RouteEntry) bool {
		count++
		assert.True(t, entry.GetBundle().HasOrigin())
		assert.True(t, entry.GetBundle().HasLocalPref())
		return true
	})

	assert.Equal(t, 2, count, "should iterate 2 routes")
}

// TestFamilyRIB_NoOpUpdate verifies same attrs don't create duplicates.
//
// VALIDATES: Same NLRI+attrs = no-op (no extra pool refs).
// PREVENTS: Pool ref leaks from redundant updates.
func TestFamilyRIB_NoOpUpdate(t *testing.T) {
	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0}

	// Insert twice with same data.
	rib.Insert(attrs, nlriBytes, true)
	entry1, _ := rib.lookupEntry(nlriBytes)
	originSlot1 := entry1.GetBundle().Origin.Slot()

	rib.Insert(attrs, nlriBytes, true)
	entry2, _ := rib.lookupEntry(nlriBytes)

	// Should be same entry (or at least same slots).
	assert.Equal(t, originSlot1, entry2.GetBundle().Origin.Slot())
	assert.Equal(t, 1, rib.Len())
}

// TestFamilyRIB_ToWireBytes verifies wire reconstruction.
//
// VALIDATES: RouteEntry can be reconstructed to valid wire format.
// PREVENTS: Data loss during storage/reconstruction cycle.
func TestFamilyRIB_ToWireBytes(t *testing.T) {
	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	// Insert with known attributes.
	attrs := concat(wireOriginIGP, wireLocalPref100, wireMED100)
	nlriBytes := []byte{24, 10, 0, 0}

	rib.Insert(attrs, nlriBytes, true)

	entry, ok := rib.lookupEntry(nlriBytes)
	require.True(t, ok)

	// Reconstruct wire bytes.
	reconstructed, err := entry.ToWireBytes()
	require.NoError(t, err)

	// Should contain ORIGIN, LOCAL_PREF, MED.
	// Parse reconstructed to verify.
	entry2, err := ParseAttributes(reconstructed, true)
	require.NoError(t, err)
	defer entry2.Release()

	// Verify values match by comparing pool data.
	origData1, _ := attrpool.Origin.Get(entry.GetBundle().Origin)
	origData2, _ := attrpool.Origin.Get(entry2.GetBundle().Origin)
	assert.Equal(t, origData1, origData2, "ORIGIN should match")

	lpData1, _ := attrpool.LocalPref.Get(entry.GetBundle().LocalPref)
	lpData2, _ := attrpool.LocalPref.Get(entry2.GetBundle().LocalPref)
	assert.Equal(t, lpData1, lpData2, "LOCAL_PREF should match")

	medData1, _ := attrpool.MED.Get(entry.GetBundle().MED)
	medData2, _ := attrpool.MED.Get(entry2.GetBundle().MED)
	assert.Equal(t, medData1, medData2, "MED should match")
}

// TestFamilyRIB_InsertEntry_CIDR verifies that InsertEntry produces the same
// RIB state as Insert for CIDR families.
func TestFamilyRIB_InsertEntry_CIDR(t *testing.T) {
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100)
	nlri1 := []byte{24, 10, 0, 0}
	nlri2 := []byte{24, 10, 0, 1}
	nlri3 := []byte{24, 10, 0, 2}

	// Insert via old path.
	ribOld := newFamilyRIB(family.IPv4Unicast, false)
	defer ribOld.Release()
	ribOld.Insert(attrs, nlri1, true)
	ribOld.Insert(attrs, nlri2, true)
	ribOld.Insert(attrs, nlri3, true)

	// Insert via new parse-once path.
	ribNew := newFamilyRIB(family.IPv4Unicast, false)
	defer ribNew.Release()
	entry, fp, attrLen, err := ParseRouteEntry(attrs, true)
	require.NoError(t, err)
	ribNew.InsertEntry(nlri1, entry, fp, attrLen)
	ribNew.InsertEntry(nlri2, entry, fp, attrLen)
	ribNew.InsertEntry(nlri3, entry, fp, attrLen)
	entry.Release()

	assert.Equal(t, ribOld.Len(), ribNew.Len())

	// Both RIBs should have identical entries.
	ribOld.IterateEntry(func(nlri []byte, oldE RouteEntry) bool {
		newE, ok := ribNew.lookupEntry(nlri)
		require.True(t, ok, "NLRI %x missing from InsertEntry RIB", nlri)
		assert.True(t, entriesEqual(oldE, newE), "entries should have same handles for NLRI %x", nlri)
		assert.Equal(t, oldE.AttrFingerprint, newE.AttrFingerprint)
		assert.Equal(t, oldE.AttrLen, newE.AttrLen)
		return true
	})
}

// TestFamilyRIB_InsertEntry_NoOpFingerprint verifies that re-inserting the
// same attributes via InsertEntry hits the fingerprint short-circuit.
func TestFamilyRIB_InsertEntry_NoOpFingerprint(t *testing.T) {
	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	nlri := []byte{24, 10, 0, 0}

	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	// First insert.
	entry1, fp1, al1, err := ParseRouteEntry(attrs, true)
	require.NoError(t, err)
	rib.InsertEntry(nlri, entry1, fp1, al1)
	entry1.Release()

	e1, ok := rib.lookupEntry(nlri)
	require.True(t, ok)
	bundle1 := e1.Bundle

	// Second insert with same attrs should be a no-op.
	entry2, fp2, al2, err := ParseRouteEntry(attrs, true)
	require.NoError(t, err)
	rib.InsertEntry(nlri, entry2, fp2, al2)
	entry2.Release()

	e2, ok := rib.lookupEntry(nlri)
	require.True(t, ok)
	assert.Equal(t, bundle1, e2.Bundle, "no-op insert should preserve original entry")
}

// TestFamilyRIB_InsertEntry_Replace verifies that InsertEntry with different
// attributes replaces the existing entry.
func TestFamilyRIB_InsertEntry_Replace(t *testing.T) {
	attrsA := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireLocalPref100)
	attrsB := concat(wireOriginIGP, wireASPath65001, wireNextHop, wireMED100)
	nlri := []byte{24, 10, 0, 0}

	rib := newFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	entryA, fpA, alA, err := ParseRouteEntry(attrsA, true)
	require.NoError(t, err)
	rib.InsertEntry(nlri, entryA, fpA, alA)
	entryA.Release()

	e1, _ := rib.lookupEntry(nlri)
	bundleA := e1.Bundle

	entryB, fpB, alB, err := ParseRouteEntry(attrsB, true)
	require.NoError(t, err)
	rib.InsertEntry(nlri, entryB, fpB, alB)
	entryB.Release()

	e2, ok := rib.lookupEntry(nlri)
	require.True(t, ok)
	assert.NotEqual(t, bundleA, e2.Bundle, "different attrs should replace entry")
}

// TestFamilyRIB_InsertEntry_Opaque verifies InsertEntry works for non-CIDR families.
func TestFamilyRIB_InsertEntry_Opaque(t *testing.T) {
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	rib := newFamilyRIB(fam, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	nlri1 := []byte{0x02, 0x19, 0x01, 0x02, 0x03}
	nlri2 := []byte{0x02, 0x19, 0x01, 0x02, 0x04}

	entry, fp, attrLen, err := ParseRouteEntry(attrs, true)
	require.NoError(t, err)
	rib.InsertEntry(nlri1, entry, fp, attrLen)
	rib.InsertEntry(nlri2, entry, fp, attrLen)
	entry.Release()

	assert.Equal(t, 2, rib.Len())
	_, ok := rib.lookupEntry(nlri1)
	assert.True(t, ok)
	_, ok = rib.lookupEntry(nlri2)
	assert.True(t, ok)
}
