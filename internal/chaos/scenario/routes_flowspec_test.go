package scenario

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateFlowSpecV4Routes_Unique verifies unique IPv4 FlowSpec rules.
//
// VALIDATES: Each peer gets distinct FlowSpec rules.
// PREVENTS: Rule collision between peers.
func TestGenerateFlowSpecV4Routes_Unique(t *testing.T) {
	seed := uint64(42)

	routes0 := GenerateFlowSpecRoutes(seed, 0, 20, 50, false)
	routes1 := GenerateFlowSpecRoutes(seed, 1, 20, 50, false)

	require.Len(t, routes0, 20)
	require.Len(t, routes1, 20)

	// Keys should not overlap.
	set0 := make(map[string]struct{}, len(routes0))
	for _, r := range routes0 {
		set0[r.Key] = struct{}{}
	}
	for _, r := range routes1 {
		_, overlap := set0[r.Key]
		assert.False(t, overlap, "peers should have non-overlapping FlowSpec keys: %s", r.Key)
	}
}

// TestGenerateFlowSpecV6Routes verifies IPv6 FlowSpec generation.
//
// VALIDATES: IPv6 FlowSpec routes use IPv6 source/dest prefixes.
// PREVENTS: Using IPv4 prefixes for IPv6 FlowSpec.
func TestGenerateFlowSpecV6Routes(t *testing.T) {
	routes := GenerateFlowSpecRoutes(42, 0, 10, 50, true)
	require.Len(t, routes, 10)

	for _, r := range routes {
		assert.True(t, r.IsIPv6, "should be IPv6 FlowSpec")
	}
}

// TestGenerateFlowSpecRoutes_Deterministic verifies deterministic output.
//
// VALIDATES: Same seed + index → same routes.
// PREVENTS: Non-reproducible chaos runs.
func TestGenerateFlowSpecRoutes_Deterministic(t *testing.T) {
	routes1 := GenerateFlowSpecRoutes(12345, 2, 30, 50, false)
	routes2 := GenerateFlowSpecRoutes(12345, 2, 30, 50, false)

	require.Equal(t, routes1, routes2, "same seed should produce identical FlowSpec routes")
}

// TestGenerateFlowSpecRoutes_HasComponents verifies rules have components.
//
// VALIDATES: Each FlowSpec rule has at least a destination prefix component.
// PREVENTS: Empty FlowSpec rules that would be rejected.
func TestGenerateFlowSpecRoutes_HasComponents(t *testing.T) {
	routes := GenerateFlowSpecRoutes(42, 0, 10, 50, false)
	require.NotEmpty(t, routes)

	for _, r := range routes {
		assert.True(t, r.DestPrefix.IsValid(), "FlowSpec rule should have destination prefix")
	}
}

// TestGenerateFlowSpecRoutes_Key verifies unique string key for validation.
//
// VALIDATES: Each route has a unique key.
// PREVENTS: Key collisions in validation model.
func TestGenerateFlowSpecRoutes_Key(t *testing.T) {
	routes := GenerateFlowSpecRoutes(42, 0, 30, 50, false)
	require.Len(t, routes, 30)

	seen := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		assert.NotEmpty(t, r.Key, "route key should not be empty")
		_, dup := seen[r.Key]
		assert.False(t, dup, "duplicate key: %s", r.Key)
		seen[r.Key] = struct{}{}
	}
}
