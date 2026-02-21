package scenario

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteGenIPv4Unique verifies that N routes from the same seed
// produce N distinct /24 prefixes.
//
// VALIDATES: Route uniqueness within a single peer.
// PREVENTS: Duplicate prefix generation causing UPDATE collisions.
func TestRouteGenIPv4Unique(t *testing.T) {
	routes := GenerateIPv4Routes(42, 0, 100, 50)
	require.Len(t, routes, 100)

	seen := make(map[netip.Prefix]bool)
	for _, r := range routes {
		assert.False(t, seen[r], "duplicate route: %s", r)
		seen[r] = true
	}
}

// TestRouteGenIPv4Deterministic verifies that the same seed and peer index
// always produce the same route set.
//
// VALIDATES: Reproducibility of route generation.
// PREVENTS: Non-deterministic routes breaking chaos scenario replay.
func TestRouteGenIPv4Deterministic(t *testing.T) {
	routes1 := GenerateIPv4Routes(42, 0, 100, 50)
	routes2 := GenerateIPv4Routes(42, 0, 100, 50)

	require.Equal(t, len(routes1), len(routes2))
	for i := range routes1 {
		assert.Equal(t, routes1[i], routes2[i], "route %d differs", i)
	}
}

// TestRouteGenIPv4NoPeerOverlap verifies that routes from different peers
// do not overlap.
//
// VALIDATES: Per-peer route isolation.
// PREVENTS: Two peers announcing the same prefix, confusing validation.
func TestRouteGenIPv4NoPeerOverlap(t *testing.T) {
	routes0 := GenerateIPv4Routes(42, 0, 100, 50)
	routes1 := GenerateIPv4Routes(42, 1, 100, 50)

	seen := make(map[netip.Prefix]bool)
	for _, r := range routes0 {
		seen[r] = true
	}
	for _, r := range routes1 {
		assert.False(t, seen[r], "peer 1 route %s overlaps with peer 0", r)
	}
}

// TestRouteGenIPv4Count verifies route count matches request.
//
// VALIDATES: Correct number of routes generated.
// PREVENTS: Off-by-one in route generation loop.
func TestRouteGenIPv4Count(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{"one_route", 1},
		{"hundred_routes", 100},
		{"two_thousand_routes", 2000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := GenerateIPv4Routes(42, 0, tt.count, 50)
			assert.Len(t, routes, tt.count)
		})
	}
}

// TestRouteGenIPv4ValidPrefixes verifies all generated prefixes are valid
// IPv4 /24s and not in reserved ranges (0.0.0.0/8, 127.0.0.0/8, multicast).
//
// VALIDATES: Generated prefixes are routable-looking.
// PREVENTS: Generating loopback, multicast, or zero-network prefixes.
func TestRouteGenIPv4ValidPrefixes(t *testing.T) {
	routes := GenerateIPv4Routes(42, 0, 500, 50)

	for _, r := range routes {
		assert.True(t, r.Addr().Is4(), "must be IPv4: %s", r)
		assert.Equal(t, 24, r.Bits(), "must be /24: %s", r)

		first := r.Addr().As4()[0]
		assert.NotEqual(t, byte(0), first, "must not be 0.x.x.x: %s", r)
		assert.NotEqual(t, byte(127), first, "must not be 127.x.x.x: %s", r)
		assert.Less(t, first, byte(224), "must not be multicast: %s", r)
	}
}

// TestRouteGenIPv4LargeCount verifies that requesting more than the /24 pool
// (262,144 per 4 first-octets) returns the full requested count by using
// more specific prefix lengths (/25, /26, etc.).
//
// VALIDATES: Dynamic prefix length scaling for large route counts.
// PREVENTS: Silent truncation when --heavy-routes exceeds /24 pool capacity.
func TestRouteGenIPv4LargeCount(t *testing.T) {
	routes := GenerateIPv4Routes(42, 0, 500000, 50)
	require.Len(t, routes, 500000)

	// All routes must still be unique.
	seen := make(map[netip.Prefix]bool, len(routes))
	for _, r := range routes {
		assert.False(t, seen[r], "duplicate route: %s", r)
		seen[r] = true
	}
}

// TestRouteGenIPv4DifferentSeeds verifies different seeds produce different routes.
//
// VALIDATES: Seed controls route generation.
// PREVENTS: Seed being ignored in route generation.
func TestRouteGenIPv4DifferentSeeds(t *testing.T) {
	routes1 := GenerateIPv4Routes(42, 0, 10, 50)
	routes2 := GenerateIPv4Routes(99, 0, 10, 50)

	differ := false
	for i := range routes1 {
		if routes1[i] != routes2[i] {
			differ = true
			break
		}
	}
	assert.True(t, differ, "different seeds should produce different routes")
}
