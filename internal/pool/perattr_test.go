package pool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPerAttributePools_Existence verifies per-attribute pools are defined.
//
// VALIDATES: All per-attribute pools exist with correct pool indices.
// PREVENTS: Missing pool definitions causing nil pointer panics.
func TestPerAttributePools_Existence(t *testing.T) {
	pools := []struct {
		name string
		pool *Pool
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
			assert.Equal(t, tc.idx, tc.pool.idx, "pool %s should have idx %d", tc.name, tc.idx)
		})
	}
}

// TestPerAttributePools_UniqueIndices verifies all pools have unique indices.
//
// VALIDATES: No pool index collisions that would cause cross-pool handle confusion.
// PREVENTS: Handle from one pool being used with another pool.
func TestPerAttributePools_UniqueIndices(t *testing.T) {
	allPools := []*Pool{
		Attributes,       // idx=0
		NLRI,             // idx=1
		Origin,           // idx=2
		ASPath,           // idx=3
		LocalPref,        // idx=4
		MED,              // idx=5
		NextHop,          // idx=6
		Communities,      // idx=7
		LargeCommunities, // idx=8
		ExtCommunities,   // idx=9
		ClusterList,      // idx=10
		OriginatorID,     // idx=11
		AtomicAggregate,  // idx=12
		Aggregator,       // idx=13
		OtherAttrs,       // idx=14
	}

	seen := make(map[uint8]string)
	for i, p := range allPools {
		names := []string{
			"Attributes", "NLRI", "Origin", "ASPath", "LocalPref", "MED",
			"NextHop", "Communities", "LargeCommunities", "ExtCommunities",
			"ClusterList", "OriginatorID", "AtomicAggregate", "Aggregator", "OtherAttrs",
		}
		name := names[i]

		if existing, ok := seen[p.idx]; ok {
			t.Errorf("pool %s has same idx %d as pool %s", name, p.idx, existing)
		}
		seen[p.idx] = name
	}
}

// TestPerAttributePools_InternAndGet verifies basic intern/get operations.
//
// VALIDATES: Per-attribute pools correctly store and retrieve data.
// PREVENTS: Broken pool initialization causing data loss.
func TestPerAttributePools_InternAndGet(t *testing.T) {
	// Use Origin pool as representative (smallest expected entries)
	testData := []byte{0x00} // IGP

	h := Origin.Intern(testData)
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

	h1 := LocalPref.Intern(testData)
	h2 := LocalPref.Intern(testData)

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

	h := Origin.Intern(testData)
	defer func() { _ = Origin.Release(h) }()

	// Try to use Origin handle with ASPath pool
	_, err := ASPath.Get(h)
	assert.ErrorIs(t, err, ErrWrongPool, "using Origin handle with ASPath should fail")
}
