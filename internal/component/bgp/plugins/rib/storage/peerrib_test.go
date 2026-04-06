package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// TestPeerRIB_Insert verifies basic route insertion.
//
// VALIDATES: Routes stored per family with shared attrs.
// PREVENTS: Routes lost during insertion.
func TestPeerRIB_Insert(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	prefix := []byte{24, 10, 0, 0}

	rib.Insert(nlri.IPv4Unicast, attrs, prefix)

	assert.Equal(t, 1, rib.Len())
	assert.Equal(t, 1, rib.FamilyLen(nlri.IPv4Unicast))
	assert.Equal(t, 0, rib.FamilyLen(nlri.IPv6Unicast))
}

// TestPeerRIB_MultipleFamilies verifies multi-family support.
//
// VALIDATES: Routes stored correctly per family.
// PREVENTS: Cross-family route confusion.
func TestPeerRIB_MultipleFamilies(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	v4prefix := []byte{24, 10, 0, 0}
	v6prefix := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}

	rib.Insert(nlri.IPv4Unicast, attrs, v4prefix)
	rib.Insert(nlri.IPv6Unicast, attrs, v6prefix)

	assert.Equal(t, 2, rib.Len())
	assert.Equal(t, 1, rib.FamilyLen(nlri.IPv4Unicast))
	assert.Equal(t, 1, rib.FamilyLen(nlri.IPv6Unicast))

	// Verify families
	families := rib.Families()
	assert.Len(t, families, 2)
}

// TestPeerRIB_Remove verifies route withdrawal.
//
// VALIDATES: Routes removed correctly.
// PREVENTS: Memory leaks from orphaned routes.
func TestPeerRIB_Remove(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	prefix1 := []byte{24, 10, 0, 0}
	prefix2 := []byte{24, 10, 0, 1}

	rib.Insert(nlri.IPv4Unicast, attrs, prefix1)
	rib.Insert(nlri.IPv4Unicast, attrs, prefix2)

	removed := rib.Remove(nlri.IPv4Unicast, prefix1)
	assert.True(t, removed)
	assert.Equal(t, 1, rib.Len())

	// Remove non-existent
	removed = rib.Remove(nlri.IPv4Unicast, []byte{24, 10, 0, 2})
	assert.False(t, removed)

	// Remove from non-existent family
	removed = rib.Remove(nlri.IPv6Unicast, prefix2)
	assert.False(t, removed)
}

// TestPeerRIB_Lookup verifies route lookup.
//
// VALIDATES: Route attributes can be retrieved.
// PREVENTS: Lost attribute data.
func TestPeerRIB_Lookup(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN=IGP
	prefix := []byte{24, 10, 0, 0}

	rib.Insert(nlri.IPv4Unicast, attrs, prefix)

	entry, found := rib.Lookup(nlri.IPv4Unicast, prefix)
	require.True(t, found)
	require.NotNil(t, entry)
	assert.True(t, entry.HasOrigin(), "should have ORIGIN attribute")

	// Non-existent.
	_, found = rib.Lookup(nlri.IPv4Unicast, []byte{24, 10, 0, 1})
	assert.False(t, found)

	// Wrong family.
	_, found = rib.Lookup(nlri.IPv6Unicast, prefix)
	assert.False(t, found)
}

// TestPeerRIB_Iterate verifies iteration.
//
// VALIDATES: All routes visited during iteration.
// PREVENTS: Missing routes during route replay.
func TestPeerRIB_Iterate(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}

	// Add routes to multiple families.
	rib.Insert(nlri.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(nlri.IPv4Unicast, attrs, []byte{24, 10, 0, 1})
	rib.Insert(nlri.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	count := 0
	rib.Iterate(func(family nlri.Family, nlriBytes []byte, entry RouteEntry) bool {
		count++
		assert.True(t, entry.HasOrigin())
		return true
	})

	assert.Equal(t, 3, count)
}

// TestPeerRIB_IterateFamily verifies family-specific iteration.
//
// VALIDATES: Only routes from specific family visited.
// PREVENTS: Cross-family route leakage.
func TestPeerRIB_IterateFamily(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}

	rib.Insert(nlri.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(nlri.IPv4Unicast, attrs, []byte{24, 10, 0, 1})
	rib.Insert(nlri.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	v4count := 0
	rib.IterateFamily(nlri.IPv4Unicast, func(nlriBytes []byte, entry RouteEntry) bool {
		v4count++
		return true
	})

	v6count := 0
	rib.IterateFamily(nlri.IPv6Unicast, func(nlriBytes []byte, entry RouteEntry) bool {
		v6count++
		return true
	})

	assert.Equal(t, 2, v4count)
	assert.Equal(t, 1, v6count)
}

// TestPeerRIB_Clear verifies RIB clearing.
//
// VALIDATES: All routes removed on clear.
// PREVENTS: Memory leaks from orphaned pool handles.
func TestPeerRIB_Clear(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	rib.Insert(nlri.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(nlri.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	assert.Equal(t, 2, rib.Len())

	rib.Clear()

	assert.Equal(t, 0, rib.Len())
	assert.Len(t, rib.Families(), 0)
}

// TestPeerRIB_AddPath verifies ADD-PATH support.
//
// VALIDATES: ADD-PATH state passed to family RIB.
// PREVENTS: ADD-PATH routes treated as duplicate.
func TestPeerRIB_AddPath(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	rib.SetAddPath(nlri.IPv4Unicast, true)

	attrs := []byte{0x40, 0x01, 0x01, 0x00}

	// Same IP prefix, different path-IDs
	nlri1 := []byte{0, 0, 0, 1, 24, 10, 0, 0}
	nlri2 := []byte{0, 0, 0, 2, 24, 10, 0, 0}

	rib.Insert(nlri.IPv4Unicast, attrs, nlri1)
	rib.Insert(nlri.IPv4Unicast, attrs, nlri2)

	assert.Equal(t, 2, rib.Len())
}
