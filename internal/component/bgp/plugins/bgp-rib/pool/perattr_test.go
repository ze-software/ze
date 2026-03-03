package pool

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustIntern(t *testing.T, p *attrpool.Pool, data []byte) attrpool.Handle {
	t.Helper()
	h, err := p.Intern(data)
	require.NoError(t, err)
	return h
}

// TestPerAttributePools_Existence verifies per-attribute pools are defined.
//
// VALIDATES: All per-attribute pools exist with correct pool indices.
// PREVENTS: Missing pool definitions causing nil pointer panics.
func TestPerAttributePools_Existence(t *testing.T) {
	pools := []struct {
		name string
		pool *attrpool.Pool
		idx  uint8
	}{
		{"Origin", Origin, 2},
		{"ASPath", ASPath, 3},
		{"LocalPref", LocalPref, 4},
		{"MED", MED, 5},
		{"NextHop", NextHop, 6},
		{"Communities", Communities, 7},
		{"LargeCommunities", LargeCommunities, 8},
		{"ExtCommunities", ExtCommunities, 9},
		{"ClusterList", ClusterList, 10},
		{"OriginatorID", OriginatorID, 11},
		{"AtomicAggregate", AtomicAggregate, 12},
		{"Aggregator", Aggregator, 13},
		{"OtherAttrs", OtherAttrs, 14},
	}

	for _, tc := range pools {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.pool, "pool %s should not be nil", tc.name)

			// Verify pool index via handle (public API).
			h := mustIntern(t, tc.pool, []byte{0x00})
			assert.Equal(t, tc.idx, h.PoolIdx(), "pool %s should have idx %d", tc.name, tc.idx)
			require.NoError(t, tc.pool.Release(h))
		})
	}
}

// TestPerAttributePools_UniqueIndices verifies all pools have unique indices.
//
// VALIDATES: No pool index collisions that would cause cross-pool handle confusion.
// PREVENTS: Handle from one pool being used with another pool.
func TestPerAttributePools_UniqueIndices(t *testing.T) {
	type poolEntry struct {
		name string
		pool *attrpool.Pool
	}

	allPools := []poolEntry{
		{"Origin", Origin},
		{"ASPath", ASPath},
		{"LocalPref", LocalPref},
		{"MED", MED},
		{"NextHop", NextHop},
		{"Communities", Communities},
		{"LargeCommunities", LargeCommunities},
		{"ExtCommunities", ExtCommunities},
		{"ClusterList", ClusterList},
		{"OriginatorID", OriginatorID},
		{"AtomicAggregate", AtomicAggregate},
		{"Aggregator", Aggregator},
		{"OtherAttrs", OtherAttrs},
	}

	seen := make(map[uint8]string)
	for _, pe := range allPools {
		h := mustIntern(t, pe.pool, []byte{0x00})
		idx := h.PoolIdx()
		_ = pe.pool.Release(h)

		if existing, ok := seen[idx]; ok {
			t.Errorf("pool %s has same idx %d as pool %s", pe.name, idx, existing)
		}
		seen[idx] = pe.name
	}
}

// TestPerAttributePools_InternAndGet verifies basic intern/get operations.
//
// VALIDATES: Per-attribute pools correctly store and retrieve data.
// PREVENTS: Broken pool initialization causing data loss.
func TestPerAttributePools_InternAndGet(t *testing.T) {
	// Use Origin pool as representative (smallest expected entries)
	testData := []byte{0x00} // IGP

	h := mustIntern(t, Origin, testData)
	require.True(t, h.IsValid(), "handle should be valid")
	assert.Equal(t, uint8(2), h.PoolIdx(), "handle should have Origin pool idx")

	got, err := Origin.Get(h)
	require.NoError(t, err)
	assert.Equal(t, testData, got)

	// Cleanup
	require.NoError(t, Origin.Release(h))
}

// TestPerAttributePools_Deduplication verifies deduplication within pools.
//
// VALIDATES: Identical data returns same handle (deduplication works).
// PREVENTS: Memory waste from storing duplicates.
func TestPerAttributePools_Deduplication(t *testing.T) {
	// Use LocalPref pool - few unique values expected
	testData := []byte{0x00, 0x00, 0x00, 0x64} // LOCAL_PREF = 100

	h1 := mustIntern(t, LocalPref, testData)
	h2 := mustIntern(t, LocalPref, testData)

	// Same data should return same slot (deduplication)
	assert.Equal(t, h1.Slot(), h2.Slot(), "same data should return same slot")

	// Cleanup - release both refs
	require.NoError(t, LocalPref.Release(h1))
	require.NoError(t, LocalPref.Release(h2))
}

// TestPerAttributePools_CrossPoolRejection verifies handles are pool-specific.
//
// VALIDATES: Using a handle from pool A with pool B returns error.
// PREVENTS: Data corruption from cross-pool handle misuse.
func TestPerAttributePools_CrossPoolRejection(t *testing.T) {
	testData := []byte{0x01}

	h := mustIntern(t, Origin, testData)
	defer func() { _ = Origin.Release(h) }()

	// Try to use Origin handle with ASPath pool
	_, err := ASPath.Get(h)
	assert.ErrorIs(t, err, attrpool.ErrWrongPool, "using Origin handle with ASPath should fail")
}
