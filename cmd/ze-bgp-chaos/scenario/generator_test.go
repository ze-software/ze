package scenario

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSeedDeterminism verifies that the same seed and parameters
// produce identical PeerProfile slices.
//
// VALIDATES: Reproducibility — same seed → same scenario.
// PREVENTS: Non-deterministic RNG usage breaking reproducibility.
func TestSeedDeterminism(t *testing.T) {
	params := GeneratorParams{
		Seed:        42,
		Peers:       4,
		IBGPRatio:   0.3,
		LocalAS:     65000,
		Routes:      100,
		HeavyPeers:  1,
		HeavyRoutes: 2000,
		BasePort:    1790,
		ListenBase:  1890,
	}

	profiles1, err := Generate(params)
	require.NoError(t, err)

	profiles2, err := Generate(params)
	require.NoError(t, err)

	require.Equal(t, len(profiles1), len(profiles2))
	for i := range profiles1 {
		assert.Equal(t, profiles1[i], profiles2[i], "profile %d differs", i)
	}
}

// TestSeedDifferent verifies that different seeds produce different profiles.
//
// VALIDATES: Seed actually controls RNG.
// PREVENTS: Seed being ignored (hardcoded values).
func TestSeedDifferent(t *testing.T) {
	params1 := GeneratorParams{
		Seed: 42, Peers: 4, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 1, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}
	params2 := params1
	params2.Seed = 99

	profiles1, err := Generate(params1)
	require.NoError(t, err)

	profiles2, err := Generate(params2)
	require.NoError(t, err)

	differ := false
	for i := range profiles1 {
		if profiles1[i].ASN != profiles2[i].ASN || profiles1[i].RouterID != profiles2[i].RouterID {
			differ = true
			break
		}
	}
	assert.True(t, differ, "different seeds should produce different profiles")
}

// TestPeerCount verifies that --peers N produces exactly N profiles.
//
// VALIDATES: Correct peer count generation.
// PREVENTS: Off-by-one in profile slice allocation.
func TestPeerCount(t *testing.T) {
	tests := []struct {
		name  string
		peers int
	}{
		{"one_peer", 1},
		{"four_peers", 4},
		{"ten_peers", 10},
		{"fifty_peers", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := GeneratorParams{
				Seed: 42, Peers: tt.peers, IBGPRatio: 0.3, LocalAS: 65000,
				Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
				BasePort: 1790, ListenBase: 1890,
			}
			profiles, err := Generate(params)
			require.NoError(t, err)
			assert.Len(t, profiles, tt.peers)
		})
	}
}

// TestPeerCountBounds verifies boundary validation for --peers flag.
//
// VALIDATES: Peers must be 1-50.
// PREVENTS: Out-of-range peer counts causing resource exhaustion or empty scenarios.
// BOUNDARY: 0 (invalid below), 1 (min valid), 50 (max valid), 51 (invalid above).
func TestPeerCountBounds(t *testing.T) {
	tests := []struct {
		name    string
		peers   int
		wantErr bool
	}{
		{"invalid_below_0", 0, true},
		{"min_valid_1", 1, false},
		{"max_valid_50", 50, false},
		{"invalid_above_51", 51, true},
		{"negative", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := GeneratorParams{
				Seed: 42, Peers: tt.peers, IBGPRatio: 0.3, LocalAS: 65000,
				Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
				BasePort: 1790, ListenBase: 1890,
			}
			profiles, err := Generate(params)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, profiles)
			} else {
				require.NoError(t, err)
				assert.Len(t, profiles, tt.peers)
			}
		})
	}
}

// TestIBGPRatio verifies that ibgp-ratio controls the iBGP/eBGP split.
//
// VALIDATES: Ratio determines number of iBGP peers (rounded).
// PREVENTS: All peers being eBGP regardless of ratio.
func TestIBGPRatio(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 10, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	ibgpCount := 0
	for _, p := range profiles {
		if p.IsIBGP {
			ibgpCount++
		}
	}
	// 10 peers * 0.3 = 3 iBGP peers
	assert.Equal(t, 3, ibgpCount, "expected 3 iBGP peers with ratio 0.3 and 10 peers")
}

// TestIBGPRatioExtreme verifies ratio extremes: 0.0 → all eBGP, 1.0 → all iBGP.
//
// VALIDATES: Extreme ratios produce correct splits.
// PREVENTS: Off-by-one at ratio boundaries.
func TestIBGPRatioExtreme(t *testing.T) {
	tests := []struct {
		name     string
		ratio    float64
		peers    int
		wantIBGP int
	}{
		{"all_ebgp", 0.0, 5, 0},
		{"all_ibgp", 1.0, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := GeneratorParams{
				Seed: 42, Peers: tt.peers, IBGPRatio: tt.ratio, LocalAS: 65000,
				Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
				BasePort: 1790, ListenBase: 1890,
			}
			profiles, err := Generate(params)
			require.NoError(t, err)

			ibgpCount := 0
			for _, p := range profiles {
				if p.IsIBGP {
					ibgpCount++
				}
			}
			assert.Equal(t, tt.wantIBGP, ibgpCount)
		})
	}
}

// TestProfileASN verifies ASN assignment: iBGP peers share local-as,
// eBGP peers get unique ASNs.
//
// VALIDATES: iBGP peers have LocalAS; eBGP peers have distinct ASNs.
// PREVENTS: ASN collision between eBGP peers or wrong ASN for iBGP.
func TestProfileASN(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 10, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	ebgpASNs := make(map[uint32]bool)
	for _, p := range profiles {
		if p.IsIBGP {
			assert.Equal(t, uint32(65000), p.ASN, "iBGP peer should have local AS")
		} else {
			assert.NotEqual(t, uint32(65000), p.ASN, "eBGP peer should not have local AS")
			assert.False(t, ebgpASNs[p.ASN], "eBGP ASNs must be unique, duplicate: %d", p.ASN)
			ebgpASNs[p.ASN] = true
		}
	}
}

// TestProfileRouterID verifies that each peer has a unique router ID.
//
// VALIDATES: Router IDs are unique across all peers.
// PREVENTS: Router ID collision causing BGP session conflicts.
func TestProfileRouterID(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 50, IBGPRatio: 0.5, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	seen := make(map[string]bool)
	for _, p := range profiles {
		assert.True(t, p.RouterID.Is4(), "router ID should be IPv4")
		rid := p.RouterID.String()
		assert.False(t, seen[rid], "duplicate router ID: %s", rid)
		seen[rid] = true
	}
}

// TestProfileConnectionMode verifies a mix of active and passive peers.
//
// VALIDATES: Connection modes are assigned deterministically with variety.
// PREVENTS: All peers being the same mode.
func TestProfileConnectionMode(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 10, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	activeCount := 0
	passiveCount := 0
	for _, p := range profiles {
		switch p.Mode {
		case ModeActive:
			activeCount++
		case ModePassive:
			passiveCount++
		}
	}
	// With 10 peers, we expect at least 1 of each mode
	assert.Greater(t, activeCount, 0, "should have at least one active peer")
	assert.Greater(t, passiveCount, 0, "should have at least one passive peer")
}

// TestProfilePort verifies that each peer gets a unique port assignment.
//
// VALIDATES: Ports are assigned sequentially from listen-base for passive peers.
// PREVENTS: Port collision between peers.
func TestProfilePort(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 10, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	ports := make(map[int]bool)
	for _, p := range profiles {
		assert.False(t, ports[p.Port], "duplicate port: %d", p.Port)
		ports[p.Port] = true
	}
}

// TestProfileRouteCount verifies route counts are assigned correctly,
// including heavy peers.
//
// VALIDATES: Normal peers get --routes count, heavy peers get --heavy-routes.
// PREVENTS: All peers getting the same route count.
func TestProfileRouteCount(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 5, IBGPRatio: 0.0, LocalAS: 65000,
		Routes: 100, HeavyPeers: 2, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	heavyCount := 0
	normalCount := 0
	for _, p := range profiles {
		switch p.RouteCount {
		case 2000:
			heavyCount++
		case 100:
			normalCount++
		}
	}
	assert.Equal(t, 2, heavyCount, "expected 2 heavy peers")
	assert.Equal(t, 3, normalCount, "expected 3 normal peers")
}

// TestProfileHoldTime verifies that hold time is set to a valid value.
//
// VALIDATES: Hold time is set (non-zero, >= 3 per RFC 4271).
// PREVENTS: Zero hold time causing immediate session expiry.
func TestProfileHoldTime(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 5, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	for i, p := range profiles {
		assert.GreaterOrEqual(t, p.HoldTime, uint16(3),
			"peer %d hold time must be >= 3 (RFC 4271)", i)
	}
}

// TestFamilyAssignment verifies that each peer is assigned a non-empty family
// set including ipv4/unicast, deterministically from the seed.
//
// VALIDATES: Peers get family assignments including mandatory ipv4/unicast.
// PREVENTS: Empty family sets or missing mandatory family.
func TestFamilyAssignment(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 10, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	for i, p := range profiles {
		require.NotEmpty(t, p.Families, "peer %d should have families assigned", i)

		// ipv4/unicast is always included.
		assert.True(t, slices.Contains(p.Families, "ipv4/unicast"), "peer %d must include ipv4/unicast", i)
	}

	// At least one peer should have more than just ipv4/unicast (with 10 peers
	// and default probabilities, this is statistically certain).
	hasMulti := false
	for _, p := range profiles {
		if len(p.Families) > 1 {
			hasMulti = true
			break
		}
	}
	assert.True(t, hasMulti, "at least one peer should have multiple families")
}

// TestFamilyAssignmentDeterministic verifies family assignment is deterministic.
//
// VALIDATES: Same seed → same family assignments.
// PREVENTS: Non-deterministic family assignment.
func TestFamilyAssignmentDeterministic(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 10, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
	}

	profiles1, err := Generate(params)
	require.NoError(t, err)
	profiles2, err := Generate(params)
	require.NoError(t, err)

	for i := range profiles1 {
		assert.Equal(t, profiles1[i].Families, profiles2[i].Families,
			"peer %d families should be deterministic", i)
	}
}

// TestFamilyFilterInclude verifies --families limits which families are assigned.
//
// VALIDATES: Only specified families can appear in profiles.
// PREVENTS: Families outside the include list being assigned.
func TestFamilyFilterInclude(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 10, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
		Families: []string{"ipv4/unicast", "ipv6/unicast"},
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	allowed := map[string]bool{"ipv4/unicast": true, "ipv6/unicast": true}
	for i, p := range profiles {
		for _, f := range p.Families {
			assert.True(t, allowed[f],
				"peer %d has family %s not in include list", i, f)
		}
	}
}

// TestFamilyFilterExclude verifies --exclude-families removes families.
//
// VALIDATES: Excluded families never appear in profiles.
// PREVENTS: Excluded families being assigned despite filter.
func TestFamilyFilterExclude(t *testing.T) {
	params := GeneratorParams{
		Seed: 42, Peers: 10, IBGPRatio: 0.3, LocalAS: 65000,
		Routes: 100, HeavyPeers: 0, HeavyRoutes: 2000,
		BasePort: 1790, ListenBase: 1890,
		ExcludeFamilies: []string{"l2vpn/evpn", "ipv4/flow", "ipv6/flow"},
	}

	profiles, err := Generate(params)
	require.NoError(t, err)

	excluded := map[string]bool{"l2vpn/evpn": true, "ipv4/flow": true, "ipv6/flow": true}
	for i, p := range profiles {
		for _, f := range p.Families {
			assert.False(t, excluded[f],
				"peer %d has excluded family %s", i, f)
		}
	}
}
