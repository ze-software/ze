package storage

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/family"
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

	rib.Insert(family.IPv4Unicast, attrs, prefix)

	assert.Equal(t, 1, rib.Len())
	assert.Equal(t, 1, rib.FamilyLen(family.IPv4Unicast))
	assert.Equal(t, 0, rib.FamilyLen(family.IPv6Unicast))
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

	rib.Insert(family.IPv4Unicast, attrs, v4prefix)
	rib.Insert(family.IPv6Unicast, attrs, v6prefix)

	assert.Equal(t, 2, rib.Len())
	assert.Equal(t, 1, rib.FamilyLen(family.IPv4Unicast))
	assert.Equal(t, 1, rib.FamilyLen(family.IPv6Unicast))

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

	rib.Insert(family.IPv4Unicast, attrs, prefix1)
	rib.Insert(family.IPv4Unicast, attrs, prefix2)

	removed := rib.Remove(family.IPv4Unicast, prefix1)
	assert.True(t, removed)
	assert.Equal(t, 1, rib.Len())

	// Remove non-existent
	removed = rib.Remove(family.IPv4Unicast, []byte{24, 10, 0, 2})
	assert.False(t, removed)

	// Remove from non-existent family
	removed = rib.Remove(family.IPv6Unicast, prefix2)
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

	rib.Insert(family.IPv4Unicast, attrs, prefix)

	entry, found := rib.Lookup(family.IPv4Unicast, prefix)
	require.True(t, found)
	require.NotNil(t, entry)
	assert.True(t, entry.GetBundle().HasOrigin(), "should have ORIGIN attribute")

	// Non-existent.
	_, found = rib.Lookup(family.IPv4Unicast, []byte{24, 10, 0, 1})
	assert.False(t, found)

	// Wrong family.
	_, found = rib.Lookup(family.IPv6Unicast, prefix)
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
	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 1})
	rib.Insert(family.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	count := 0
	rib.Iterate(func(fam family.Family, nlriBytes []byte, entry RouteEntry) bool {
		count++
		assert.True(t, entry.GetBundle().HasOrigin())
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

	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 1})
	rib.Insert(family.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	v4count := 0
	rib.IterateFamily(family.IPv4Unicast, func(nlriBytes []byte, entry RouteEntry) bool {
		v4count++
		return true
	})

	v6count := 0
	rib.IterateFamily(family.IPv6Unicast, func(nlriBytes []byte, entry RouteEntry) bool {
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
	rib.Insert(family.IPv4Unicast, attrs, []byte{24, 10, 0, 0})
	rib.Insert(family.IPv6Unicast, attrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})

	assert.Equal(t, 2, rib.Len())

	rib.Clear()

	assert.Equal(t, 0, rib.Len())
	assert.Len(t, rib.Families(), 0)
}

// TestPeerRIB_AddPath verifies ADD-PATH support.
//
// VALIDATES: ADD-PATH state passed to family RIB.
// PREVENTS: ADD-PATH routes treated as duplicate.
//
// This is receive-side keying of the Adj-RIB-In. The RFC 7911 Section 2 assignment of
// identifiers ze advertises is proven by the reactor forward path-id units
// (internal/component/bgp/reactor/rfc7911_forward_path_id_test.go).
func TestPeerRIB_AddPath(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	rib.SetAddPath(family.IPv4Unicast, true)

	attrs := []byte{0x40, 0x01, 0x01, 0x00}

	// Same IP prefix, different path-IDs
	nlri1 := []byte{0, 0, 0, 1, 24, 10, 0, 0}
	nlri2 := []byte{0, 0, 0, 2, 24, 10, 0, 0}

	rib.Insert(family.IPv4Unicast, attrs, nlri1)
	rib.Insert(family.IPv4Unicast, attrs, nlri2)

	assert.Equal(t, 2, rib.Len())
}

// TestPeerRIB_InsertEntry verifies parse-once insertion across families.
func TestPeerRIB_InsertEntry(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	entry, fp, attrLen, err := ParseRouteEntry(attrs)
	require.NoError(t, err)

	nlri1 := []byte{24, 10, 0, 0}
	nlri2 := []byte{24, 10, 0, 1}
	nlri3 := []byte{24, 10, 0, 2}

	rib.InsertEntry(family.IPv4Unicast, entry, fp, attrLen, nlri1)
	rib.InsertEntry(family.IPv4Unicast, entry, fp, attrLen, nlri2)
	rib.InsertEntry(family.IPv4Unicast, entry, fp, attrLen, nlri3)
	entry.Release()

	assert.Equal(t, 3, rib.Len())

	e1, ok := rib.Lookup(family.IPv4Unicast, nlri1)
	require.True(t, ok)
	e2, ok := rib.Lookup(family.IPv4Unicast, nlri2)
	require.True(t, ok)
	assert.True(t, entriesEqual(e1, e2), "shared attrs should produce same handles")
}

// TestPeerRIBRetainedPathsSurviveReplacement covers both append operations,
// ADD-PATH framing, and label side-data. Two independent snapshots survive
// replacement, and the final release returns every old unique pool handle.
func TestPeerRIBRetainedPathsSurviveReplacement(t *testing.T) {
	prefix := netip.MustParsePrefix("198.18.247.0/24")
	for _, tc := range []struct {
		name    string
		fam     family.Family
		key     []byte
		labeled bool
	}{
		{"prefix", family.IPv4Unicast, []byte{24, 198, 18, 247}, false},
		{"labeled", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}, []byte{24, 198, 18, 247}, true},
		{"opaque", family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}, bgpLSNodeNLRI(91, 65091), false},
	} {
		for _, addPath := range []bool{false, true} {
			name := tc.name
			if addPath {
				name += "/add-path"
			}
			t.Run(name, func(t *testing.T) {
				rib := NewPeerRIB("192.0.2.247")
				defer rib.Release()
				rib.SetAddPath(tc.fam, addPath)
				wire := tc.key
				if addPath {
					wire = append([]byte{0, 0, 0, 91}, tc.key...)
				}
				attrs := []byte{
					0x40, 1, 1, 0,
					0x40, 2, 6, 2, 1, 0xFE, 0xDC, 0xBA, 0x98,
					0x40, 3, 4, 198, 18, 247, 246,
				}
				rib.Insert(tc.fam, attrs, wire)
				oldLabels := []uint32{1048570, 1048569}
				if tc.labeled {
					h := pool.InternLabels(oldLabels)
					if !rib.SetLabelsIfRouteExists(tc.fam, wire, h) {
						_ = pool.Labels.Release(h)
						t.Fatal("could not attach labels")
					}
				}
				appendPaths := func(dst []PrefixPath) ([]PrefixPath, bool) {
					if IsCIDRFamily(tc.fam) {
						return rib.AppendPrefixPathsRetained(tc.fam, prefix, dst)
					}
					return rib.AppendKeyPathsRetained(tc.fam, tc.key, dst)
				}
				var scratch [2]PrefixPath
				paths, storedAddPath := appendPaths(scratch[:0])
				defer func() {
					for i := range paths {
						paths[i].Release()
					}
				}()
				require.Len(t, paths, 1)
				require.Equal(t, addPath, storedAddPath)
				paths, _ = appendPaths(paths)
				require.Len(t, paths, 2)
				if addPath {
					require.Equal(t, uint32(91), paths[0].PathID)
				}
				if !IsCIDRFamily(tc.fam) {
					require.Equal(t, string(tc.key), paths[0].Route)
				}
				asPath := paths[0].Entry.ASPath
				nextHop := paths[0].Entry.GetBundle().NextHop
				labels := paths[0].Labels

				require.True(t, rib.Remove(tc.fam, wire))
				rib.Insert(tc.fam, []byte{0x40, 1, 1, 2}, wire)
				if tc.labeled {
					h := pool.InternLabels([]uint32{1048568})
					if !rib.SetLabelsIfRouteExists(tc.fam, wire, h) {
						_ = pool.Labels.Release(h)
						t.Fatal("could not attach replacement labels")
					}
				}
				data, err := pool.ASPath.Get(asPath)
				require.NoError(t, err)
				require.Equal(t, []byte{2, 1, 0xFE, 0xDC, 0xBA, 0x98}, data)
				data, err = pool.NextHop.Get(nextHop)
				require.NoError(t, err)
				require.Equal(t, []byte{198, 18, 247, 246}, data)
				if tc.labeled {
					require.Equal(t, oldLabels, pool.ResolveLabels(labels))
				}
				paths[0].Release()
				_, err = pool.ASPath.Get(asPath)
				require.NoError(t, err, "second snapshot still owns AS_PATH")
				paths[1].Release()
				_, err = pool.ASPath.Get(asPath)
				require.Error(t, err, "final snapshot must release AS_PATH")
				_, err = pool.NextHop.Get(nextHop)
				require.Error(t, err, "final snapshot must release bundle attributes")
				if tc.labeled {
					_, err = pool.Labels.Get(labels)
					require.Error(t, err, "final snapshot must release label side-data")
				}
			})
		}
	}
}

// TestPeerRIBRemoveFamilyMatchingOwnsStorageTransaction pins the same lock used
// by received inserts across selection and removal, and reports only removed
// generations. The fresh and stale paths initially came from one UPDATE.
func TestPeerRIBRemoveFamilyMatchingOwnsStorageTransaction(t *testing.T) {
	rib := NewPeerRIB("192.0.2.1")
	defer rib.Release()
	fam := family.IPv4Unicast
	rib.SetAddPath(fam, true)
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	fresh := []byte{0, 0, 0, 0, 24, 10, 0, 0}
	stale := []byte{0, 0, 0, 17, 24, 10, 0, 0}
	for _, raw := range [][]byte{fresh, stale} {
		rib.Insert(fam, attrs, raw)
		rib.ModifyFamilyEntry(fam, raw, func(entry *RouteEntry) {
			entry.MsgID = 91
			entry.StaleLevel = 2
		})
	}
	rib.Insert(fam, attrs, fresh)
	rib.ModifyFamilyEntry(fam, fresh, func(entry *RouteEntry) { entry.MsgID = 92 })
	requireStorageLock := func() {
		t.Helper()
		acquired := rib.mu.TryLock()
		if acquired {
			rib.mu.Unlock()
		}
		require.False(t, acquired, "received inserts must be excluded by the storage lock")
	}
	var removed [][]byte
	count := rib.RemoveFamilyMatching(fam, func(entry RouteEntry) bool {
		requireStorageLock()
		return entry.StaleLevel != StaleLevelFresh
	}, func(raw []byte, message uint64, addPath bool) {
		requireStorageLock()
		require.Equal(t, uint64(91), message)
		require.True(t, addPath)
		require.Equal(t, stale, raw)
		_, exists := rib.families[fam].lookupEntry(raw)
		require.False(t, exists, "callback must describe a successful removal, not a candidate")
		removed = append(removed, raw)
	})
	require.Equal(t, 1, count)
	require.Len(t, removed, 1)
	entry, exists := rib.Lookup(fam, fresh)
	require.True(t, exists)
	require.Equal(t, uint64(92), entry.MsgID)
	require.Equal(t, StaleLevelFresh, entry.StaleLevel)
	rib.Insert(fam, attrs, stale)
	require.Equal(t, stale, removed[0], "callback bytes must survive later storage mutation")
	require.Equal(t, 2, rib.Len())
}
