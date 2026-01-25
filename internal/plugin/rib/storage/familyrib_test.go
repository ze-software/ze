package storage

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFamilyRIB_Insert verifies basic route insertion.
//
// VALIDATES: Routes stored with attr handle → NLRI mapping.
// PREVENTS: Lost routes during insertion.
func TestFamilyRIB_Insert(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)
	defer rib.Release()

	// Simple attributes (ORIGIN IGP)
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	prefix := []byte{24, 10, 0, 0} // 10.0.0.0/24

	rib.Insert(attrs, prefix)

	// Verify stored
	assert.Equal(t, 1, rib.Len())
	handle, exists := rib.Lookup(prefix)
	assert.True(t, exists)
	assert.True(t, handle.IsValid())
}

// TestFamilyRIB_MultipleNLRI verifies multiple NLRIs share attrs.
//
// VALIDATES: Same attr handle used for multiple prefixes.
// PREVENTS: Memory waste from duplicate attr storage.
func TestFamilyRIB_MultipleNLRI(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	prefix1 := []byte{24, 10, 0, 0}
	prefix2 := []byte{24, 10, 0, 1}
	prefix3 := []byte{24, 10, 0, 2}

	rib.Insert(attrs, prefix1)
	rib.Insert(attrs, prefix2)
	rib.Insert(attrs, prefix3)

	assert.Equal(t, 3, rib.Len())

	// All should have same attr handle
	h1, ok1 := rib.Lookup(prefix1)
	h2, ok2 := rib.Lookup(prefix2)
	h3, ok3 := rib.Lookup(prefix3)

	require.True(t, ok1)
	require.True(t, ok2)
	require.True(t, ok3)

	// Same attributes = same handle (deduplication)
	assert.Equal(t, h1, h2)
	assert.Equal(t, h2, h3)
}

// TestFamilyRIB_ImplicitWithdraw verifies route replacement.
//
// VALIDATES: Same prefix with new attrs replaces old entry.
// PREVENTS: Stale routes remaining after implicit withdraw.
func TestFamilyRIB_ImplicitWithdraw(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)
	defer rib.Release()

	prefix := []byte{24, 10, 0, 0}
	attrs1 := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	attrs2 := []byte{0x40, 0x01, 0x01, 0x01} // ORIGIN EGP

	// Insert with first attrs
	rib.Insert(attrs1, prefix)
	h1, _ := rib.Lookup(prefix)

	// Insert same prefix with different attrs (implicit withdraw)
	rib.Insert(attrs2, prefix)
	h2, exists := rib.Lookup(prefix)

	assert.True(t, exists)
	assert.NotEqual(t, h1, h2) // Different attrs = different handle
	assert.Equal(t, 1, rib.Len())
}

// TestFamilyRIB_NoOpUpdate verifies duplicate insert is no-op.
//
// VALIDATES: Same prefix + same attrs doesn't change state.
// PREVENTS: Pool refcount leaks on duplicate inserts.
func TestFamilyRIB_NoOpUpdate(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	prefix := []byte{24, 10, 0, 0}

	rib.Insert(attrs, prefix)
	h1, _ := rib.Lookup(prefix)

	// Same prefix, same attrs = no-op
	rib.Insert(attrs, prefix)
	h2, _ := rib.Lookup(prefix)

	assert.Equal(t, h1, h2)
	assert.Equal(t, 1, rib.Len())
}

// TestFamilyRIB_Remove verifies route withdrawal.
//
// VALIDATES: Prefix removed from RIB.
// PREVENTS: Memory leaks from unreleased handles.
func TestFamilyRIB_Remove(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	prefix1 := []byte{24, 10, 0, 0}
	prefix2 := []byte{24, 10, 0, 1}

	rib.Insert(attrs, prefix1)
	rib.Insert(attrs, prefix2)

	// Remove first
	removed := rib.Remove(prefix1)
	assert.True(t, removed)
	assert.Equal(t, 1, rib.Len())

	// Verify prefix1 gone
	_, exists := rib.Lookup(prefix1)
	assert.False(t, exists)

	// prefix2 still present
	_, exists = rib.Lookup(prefix2)
	assert.True(t, exists)
}

// TestFamilyRIB_RemoveNonExistent verifies removing unknown prefix.
//
// VALIDATES: Remove returns false for unknown prefix.
// PREVENTS: Panic or incorrect state on invalid remove.
func TestFamilyRIB_RemoveNonExistent(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)
	defer rib.Release()

	prefix := []byte{24, 10, 0, 0}
	removed := rib.Remove(prefix)

	assert.False(t, removed)
	assert.Equal(t, 0, rib.Len())
}

// TestFamilyRIB_RemoveLastNLRI verifies attr handle release.
//
// VALIDATES: Attr handle released when last NLRI removed.
// PREVENTS: Handle leaks leaving orphaned pool data.
func TestFamilyRIB_RemoveLastNLRI(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	prefix := []byte{24, 10, 0, 0}

	rib.Insert(attrs, prefix)
	assert.Equal(t, 1, rib.EntryCount()) // One attr entry

	rib.Remove(prefix)
	assert.Equal(t, 0, rib.Len())
	assert.Equal(t, 0, rib.EntryCount()) // Attr entry also removed
}

// TestFamilyRIB_Iterate verifies iteration over routes.
//
// VALIDATES: All routes visited during iteration.
// PREVENTS: Missing routes during route replay.
func TestFamilyRIB_Iterate(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs1 := []byte{0x40, 0x01, 0x01, 0x00}
	attrs2 := []byte{0x40, 0x01, 0x01, 0x01}

	// Two attr sets, multiple NLRIs each
	rib.Insert(attrs1, []byte{24, 10, 0, 0})
	rib.Insert(attrs1, []byte{24, 10, 0, 1})
	rib.Insert(attrs2, []byte{24, 10, 0, 2})
	rib.Insert(attrs2, []byte{24, 10, 0, 3})

	// Collect via iteration
	count := 0
	rib.Iterate(func(attrBytes []byte, nlriBytes []byte) bool {
		count++
		return true
	})

	assert.Equal(t, 4, count)
}

// TestFamilyRIB_IterateEarlyExit verifies early termination.
//
// VALIDATES: Iterate stops when callback returns false.
// PREVENTS: Unnecessary iteration.
func TestFamilyRIB_IterateEarlyExit(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	for i := 0; i < 10; i++ {
		rib.Insert(attrs, []byte{24, 10, 0, byte(i)})
	}

	count := 0
	rib.Iterate(func(attrBytes []byte, nlriBytes []byte) bool {
		count++
		return count < 3
	})

	assert.Equal(t, 3, count)
}

// TestFamilyRIB_IPv6 verifies pooled storage for IPv6.
//
// VALIDATES: IPv6 NLRIs stored with pool handles.
// PREVENTS: Memory waste from large NLRI copies.
func TestFamilyRIB_IPv6(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv6Unicast, false)
	defer rib.Release()

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	prefix1 := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}
	prefix2 := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02}

	rib.Insert(attrs, prefix1)
	rib.Insert(attrs, prefix2)

	assert.Equal(t, 2, rib.Len())

	h1, ok1 := rib.Lookup(prefix1)
	h2, ok2 := rib.Lookup(prefix2)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.Equal(t, h1, h2) // Same attrs
}

// TestFamilyRIB_AddPath verifies ADD-PATH support.
//
// VALIDATES: Same prefix with different path-IDs are distinct.
// PREVENTS: Path-ID collision corrupting RIB.
func TestFamilyRIB_AddPath(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, true) // ADD-PATH enabled
	defer rib.Release()

	attrs1 := []byte{0x40, 0x01, 0x01, 0x00}
	attrs2 := []byte{0x40, 0x01, 0x01, 0x01}

	// Same IP prefix, different path-IDs
	nlri1 := []byte{0, 0, 0, 1, 24, 10, 0, 0} // path-id=1
	nlri2 := []byte{0, 0, 0, 2, 24, 10, 0, 0} // path-id=2

	rib.Insert(attrs1, nlri1)
	rib.Insert(attrs2, nlri2)

	assert.Equal(t, 2, rib.Len())

	h1, ok1 := rib.Lookup(nlri1)
	h2, ok2 := rib.Lookup(nlri2)

	require.True(t, ok1)
	require.True(t, ok2)
	assert.NotEqual(t, h1, h2) // Different paths
}

// TestFamilyRIB_Release verifies cleanup.
//
// VALIDATES: All pool handles released on cleanup.
// PREVENTS: Memory leaks from orphaned handles.
func TestFamilyRIB_Release(t *testing.T) {
	rib := NewFamilyRIB(nlri.IPv4Unicast, false)

	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	for i := 0; i < 100; i++ {
		rib.Insert(attrs, []byte{24, 10, 0, byte(i)})
	}

	assert.Equal(t, 100, rib.Len())

	rib.Release()

	assert.Equal(t, 0, rib.Len())
	assert.Equal(t, 0, rib.EntryCount())
}
