package scenario

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateIPv6Routes_Unique verifies that different peer indices produce
// non-overlapping IPv6 /48 prefixes within the 2001:db8::/32 documentation range.
//
// VALIDATES: Each peer gets a unique slice of IPv6 address space.
// PREVENTS: Overlapping routes between peers causing false validation failures.
func TestGenerateIPv6Routes_Unique(t *testing.T) {
	seed := uint64(42)

	routes0 := GenerateIPv6Routes(seed, 0, 50)
	routes1 := GenerateIPv6Routes(seed, 1, 50)

	require.Len(t, routes0, 50)
	require.Len(t, routes1, 50)

	// No overlaps between peers.
	set0 := make(map[string]struct{}, len(routes0))
	for _, r := range routes0 {
		set0[r.Addr().String()] = struct{}{}
	}
	for _, r := range routes1 {
		_, overlap := set0[r.Addr().String()]
		assert.False(t, overlap, "peer 0 and peer 1 have overlapping route: %s", r)
	}

	// All routes are within 2001:db8::/32.
	for _, r := range routes0 {
		addr := r.Addr()
		assert.True(t, addr.Is6(), "route should be IPv6: %s", r)
		assert.Equal(t, 48, r.Bits(), "route should be /48: %s", r)
		b := addr.As16()
		assert.Equal(t, byte(0x20), b[0], "first byte should be 0x20")
		assert.Equal(t, byte(0x01), b[1], "second byte should be 0x01")
		assert.Equal(t, byte(0x0d), b[2], "third byte should be 0x0d")
		assert.Equal(t, byte(0xb8), b[3], "fourth byte should be 0xb8")
	}
}

// TestGenerateIPv6Routes_Deterministic verifies that the same seed and peer
// index always produces the same routes.
//
// VALIDATES: Deterministic output from seed.
// PREVENTS: Non-reproducible test runs.
func TestGenerateIPv6Routes_Deterministic(t *testing.T) {
	seed := uint64(12345)
	routes1 := GenerateIPv6Routes(seed, 3, 100)
	routes2 := GenerateIPv6Routes(seed, 3, 100)

	require.Len(t, routes1, 100)
	require.Equal(t, routes1, routes2, "same seed + index should produce identical routes")
}

// TestGenerateIPv6Routes_DifferentSeeds verifies that different seeds produce
// different route orderings.
//
// VALIDATES: Seed affects route generation.
// PREVENTS: Ignoring seed parameter.
func TestGenerateIPv6Routes_DifferentSeeds(t *testing.T) {
	routes1 := GenerateIPv6Routes(100, 0, 20)
	routes2 := GenerateIPv6Routes(200, 0, 20)

	// Same count.
	require.Len(t, routes1, 20)
	require.Len(t, routes2, 20)

	// At least one route should differ (extremely unlikely to be identical with different seeds).
	differ := false
	for i := range routes1 {
		if routes1[i] != routes2[i] {
			differ = true
			break
		}
	}
	assert.True(t, differ, "different seeds should produce different route sets")
}

// TestGenerateIPv6Routes_UniqueWithinPeer verifies that routes within a single
// peer are unique (no duplicates).
//
// VALIDATES: No duplicate prefixes within a peer's route set.
// PREVENTS: Double-announcing the same route.
func TestGenerateIPv6Routes_UniqueWithinPeer(t *testing.T) {
	routes := GenerateIPv6Routes(42, 0, 200)
	require.Len(t, routes, 200)

	seen := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		key := r.String()
		_, dup := seen[key]
		assert.False(t, dup, "duplicate route: %s", key)
		seen[key] = struct{}{}
	}
}

// TestGenerateIPv6Routes_LargeCount verifies that requesting more than the
// /48 pool (1,280 per peer) returns the full requested count by using
// more specific prefix lengths (/49, /50, etc.).
//
// VALIDATES: Dynamic prefix length scaling for large route counts.
// PREVENTS: Silent truncation when route count exceeds /48 pool capacity.
func TestGenerateIPv6Routes_LargeCount(t *testing.T) {
	routes := GenerateIPv6Routes(42, 0, 5000)
	require.Len(t, routes, 5000)

	seen := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		key := r.String()
		_, dup := seen[key]
		assert.False(t, dup, "duplicate route: %s", key)
		seen[key] = struct{}{}
	}
}
