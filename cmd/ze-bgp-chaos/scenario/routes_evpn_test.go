package scenario

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateEVPNRoutes_Unique verifies that different peer indices produce
// non-overlapping EVPN Type-2 MAC/IP routes.
//
// VALIDATES: Each peer gets unique MAC addresses.
// PREVENTS: MAC collision between EVPN peers.
func TestGenerateEVPNRoutes_Unique(t *testing.T) {
	seed := uint64(42)

	routes0 := GenerateEVPNRoutes(seed, 0, 30)
	routes1 := GenerateEVPNRoutes(seed, 1, 30)

	require.Len(t, routes0, 30)
	require.Len(t, routes1, 30)

	// MACs should differ between peers (first byte encodes peer index).
	assert.NotEqual(t, routes0[0].MAC, routes1[0].MAC,
		"different peers should have different MACs")
}

// TestGenerateEVPNRoutes_Deterministic verifies deterministic output.
//
// VALIDATES: Same seed + index → same routes.
// PREVENTS: Non-reproducible chaos runs.
func TestGenerateEVPNRoutes_Deterministic(t *testing.T) {
	routes1 := GenerateEVPNRoutes(12345, 2, 40)
	routes2 := GenerateEVPNRoutes(12345, 2, 40)

	require.Equal(t, routes1, routes2, "same seed should produce identical EVPN routes")
}

// TestGenerateEVPNRoutes_MACFormat verifies MAC address format.
//
// VALIDATES: MACs use locally-administered bit (byte[0] bit 1 set).
// PREVENTS: Conflicting with real hardware MACs.
func TestGenerateEVPNRoutes_MACFormat(t *testing.T) {
	routes := GenerateEVPNRoutes(42, 3, 10)
	require.NotEmpty(t, routes)

	for _, r := range routes {
		// Locally-administered bit: byte[0] & 0x02 should be set.
		assert.True(t, r.MAC[0]&0x02 != 0,
			"MAC should have locally-administered bit set: %02x", r.MAC[0])
		// Unicast: byte[0] bit 0 should be clear.
		assert.True(t, r.MAC[0]&0x01 == 0,
			"MAC should be unicast (bit 0 clear): %02x", r.MAC[0])
	}
}

// TestGenerateEVPNRoutes_HasIP verifies that Type-2 routes include an IP.
//
// VALIDATES: Each EVPN route has an associated IP address.
// PREVENTS: Type-2 routes without IP (MAC-only not useful for chaos testing).
func TestGenerateEVPNRoutes_HasIP(t *testing.T) {
	routes := GenerateEVPNRoutes(42, 0, 20)
	require.NotEmpty(t, routes)

	for _, r := range routes {
		assert.True(t, r.IP.IsValid(), "EVPN route should have a valid IP: %v", r.IP)
	}
}

// TestGenerateEVPNRoutes_RDFormat verifies Route Distinguisher format.
//
// VALIDATES: RD is type 0 with peer-derived values.
// PREVENTS: Invalid RD encoding.
func TestGenerateEVPNRoutes_RDFormat(t *testing.T) {
	routes := GenerateEVPNRoutes(42, 5, 10)
	require.NotEmpty(t, routes)

	rd := routes[0].RDBytes
	// Type 0: first 2 bytes are 0x0000.
	assert.Equal(t, byte(0), rd[0], "RD type high byte should be 0")
	assert.Equal(t, byte(0), rd[1], "RD type low byte should be 0")
}

// TestGenerateEVPNRoutes_Key verifies unique string key for validation.
//
// VALIDATES: Each route has a unique key.
// PREVENTS: Key collisions in validation model.
func TestGenerateEVPNRoutes_Key(t *testing.T) {
	routes := GenerateEVPNRoutes(42, 0, 50)
	require.Len(t, routes, 50)

	seen := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		assert.NotEmpty(t, r.Key, "route key should not be empty")
		_, dup := seen[r.Key]
		assert.False(t, dup, "duplicate key: %s", r.Key)
		seen[r.Key] = struct{}{}
	}
}
