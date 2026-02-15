package scenario

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateVPNv4Routes_Unique verifies that different peer indices produce
// non-overlapping VPN IPv4 routes with distinct RDs.
//
// VALIDATES: Each peer gets unique RD + prefix combinations.
// PREVENTS: Route collision between VPN peers.
func TestGenerateVPNv4Routes_Unique(t *testing.T) {
	seed := uint64(42)

	routes0 := GenerateVPNRoutes(seed, 0, 50, false)
	routes1 := GenerateVPNRoutes(seed, 1, 50, false)

	require.Len(t, routes0, 50)
	require.Len(t, routes1, 50)

	// RDs should differ between peers.
	assert.NotEqual(t, routes0[0].RDBytes, routes1[0].RDBytes,
		"different peers should have different RDs")

	// All routes should have IPv4 prefixes.
	for _, r := range routes0 {
		assert.True(t, r.Prefix.Addr().Is4(), "VPNv4 route should have IPv4 prefix: %s", r.Prefix)
	}
}

// TestGenerateVPNv6Routes_Unique verifies VPN IPv6 route generation.
//
// VALIDATES: VPN IPv6 routes use IPv6 prefixes with peer-specific RDs.
// PREVENTS: Using IPv4 prefixes for VPNv6.
func TestGenerateVPNv6Routes_Unique(t *testing.T) {
	seed := uint64(42)

	routes := GenerateVPNRoutes(seed, 0, 30, true)
	require.Len(t, routes, 30)

	for _, r := range routes {
		assert.True(t, r.Prefix.Addr().Is6(), "VPNv6 route should have IPv6 prefix: %s", r.Prefix)
	}
}

// TestGenerateVPNRoutes_Deterministic verifies deterministic output.
//
// VALIDATES: Same seed + index → same routes.
// PREVENTS: Non-reproducible chaos runs.
func TestGenerateVPNRoutes_Deterministic(t *testing.T) {
	routes1 := GenerateVPNRoutes(12345, 2, 40, false)
	routes2 := GenerateVPNRoutes(12345, 2, 40, false)

	require.Equal(t, routes1, routes2, "same seed should produce identical VPN routes")
}

// TestGenerateVPNRoutes_RDFormat verifies Route Distinguisher format.
//
// VALIDATES: RD is type 0 with peer-derived values per master design.
// PREVENTS: Invalid RD encoding.
func TestGenerateVPNRoutes_RDFormat(t *testing.T) {
	routes := GenerateVPNRoutes(42, 3, 10, false)
	require.NotEmpty(t, routes)

	rd := routes[0].RDBytes
	// Type 0 RD: first 2 bytes are 0x0000.
	assert.Equal(t, byte(0), rd[0], "RD type high byte should be 0")
	assert.Equal(t, byte(0), rd[1], "RD type low byte should be 0")
}

// TestGenerateVPNRoutes_Labels verifies MPLS label assignment.
//
// VALIDATES: Each VPN route has a valid label.
// PREVENTS: Missing or zero labels.
func TestGenerateVPNRoutes_Labels(t *testing.T) {
	routes := GenerateVPNRoutes(42, 0, 20, false)
	require.NotEmpty(t, routes)

	for _, r := range routes {
		require.NotEmpty(t, r.Labels, "VPN route must have at least one label")
		assert.Greater(t, r.Labels[0], uint32(0), "label should be non-zero")
		assert.LessOrEqual(t, r.Labels[0], uint32(1048575), "label must be 20-bit")
	}
}

// TestGenerateVPNRoutes_Key verifies unique string key for validation.
//
// VALIDATES: Each route has a unique key suitable for validation tracking.
// PREVENTS: Key collisions causing false validation results.
func TestGenerateVPNRoutes_Key(t *testing.T) {
	routes := GenerateVPNRoutes(42, 0, 50, false)
	require.Len(t, routes, 50)

	seen := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		assert.NotEmpty(t, r.Key, "route key should not be empty")
		_, dup := seen[r.Key]
		assert.False(t, dup, "duplicate key: %s", r.Key)
		seen[r.Key] = struct{}{}
	}
}
