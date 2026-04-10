package configjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseBGPSubtree_Wrapped verifies extraction from {"bgp": {...}}.
//
// VALIDATES: ParseBGPSubtree returns the bgp subtree from a wrapped JSON object.
// PREVENTS: Misparse when bgp key is present.
func TestParseBGPSubtree_Wrapped(t *testing.T) {
	input := `{"bgp":{"router-id":"10.0.0.1","peer":{"192.0.2.1":{}}}}`

	bgp, ok := ParseBGPSubtree(input)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", bgp["router-id"])
	assert.NotNil(t, bgp["peer"])
}

// TestParseBGPSubtree_Bare verifies extraction from bare {...} JSON.
//
// VALIDATES: ParseBGPSubtree returns the root as-is when no "bgp" key.
// PREVENTS: Failure on bare config objects (e.g., peer-level subtree).
func TestParseBGPSubtree_Bare(t *testing.T) {
	input := `{"router-id":"10.0.0.1","peer":{"192.0.2.1":{}}}`

	bgp, ok := ParseBGPSubtree(input)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", bgp["router-id"])
}

// TestParseBGPSubtree_InvalidJSON verifies invalid JSON returns false.
//
// VALIDATES: ParseBGPSubtree fails gracefully on malformed input.
// PREVENTS: Panic on bad JSON.
func TestParseBGPSubtree_InvalidJSON(t *testing.T) {
	_, ok := ParseBGPSubtree(`{invalid`)
	assert.False(t, ok)
}

// TestParseBGPSubtree_Empty verifies empty object returns the empty map.
//
// VALIDATES: ParseBGPSubtree handles empty objects.
// PREVENTS: False negative on empty config.
func TestParseBGPSubtree_Empty(t *testing.T) {
	bgp, ok := ParseBGPSubtree(`{}`)
	require.True(t, ok)
	assert.Empty(t, bgp)
}

// TestForEachPeer_StandalonePeers verifies iteration over standalone peers.
//
// VALIDATES: ForEachPeer visits peers under bgpTree["peer"].
// PREVENTS: Standalone peers being skipped.
func TestForEachPeer_StandalonePeers(t *testing.T) {
	bgpTree := map[string]any{
		"peer": map[string]any{
			"192.0.2.1": map[string]any{"session": map[string]any{"asn": map[string]any{"remote": float64(65001)}}},
			"192.0.2.2": map[string]any{"session": map[string]any{"asn": map[string]any{"remote": float64(65002)}}},
		},
	}

	visited := make(map[string]map[string]any)
	ForEachPeer(bgpTree, func(addr string, peerMap, groupMap map[string]any) {
		assert.Nil(t, groupMap, "standalone peers should have nil groupMap")
		visited[addr] = peerMap
	})

	require.Len(t, visited, 2)
	assert.Contains(t, visited, "192.0.2.1")
	assert.Contains(t, visited, "192.0.2.2")
}

// TestForEachPeer_GroupedPeers verifies iteration over peers within groups.
//
// VALIDATES: ForEachPeer visits peers under bgpTree["group"][name]["peer"].
// PREVENTS: Grouped peers being missed.
func TestForEachPeer_GroupedPeers(t *testing.T) {
	bgpTree := map[string]any{
		"group": map[string]any{
			"transit": map[string]any{
				"session": map[string]any{"asn": map[string]any{"local": float64(65000)}},
				"peer": map[string]any{
					"192.0.2.1": map[string]any{},
				},
			},
			"peers": map[string]any{
				"peer": map[string]any{
					"192.0.2.2": map[string]any{},
				},
			},
		},
	}

	type visit struct {
		addr     string
		hasGroup bool
	}
	var visits []visit
	ForEachPeer(bgpTree, func(addr string, _, groupMap map[string]any) {
		visits = append(visits, visit{addr: addr, hasGroup: groupMap != nil})
	})

	require.Len(t, visits, 2)
	for _, v := range visits {
		assert.True(t, v.hasGroup, "grouped peers should have non-nil groupMap for %s", v.addr)
	}
}

// TestForEachPeer_MixedStandaloneAndGrouped verifies both paths in one tree.
//
// VALIDATES: ForEachPeer visits standalone and grouped peers together.
// PREVENTS: One path shadowing the other.
func TestForEachPeer_MixedStandaloneAndGrouped(t *testing.T) {
	bgpTree := map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{},
		},
		"group": map[string]any{
			"transit": map[string]any{
				"peer": map[string]any{
					"10.0.0.2": map[string]any{},
				},
			},
		},
	}

	addrs := make(map[string]bool)
	ForEachPeer(bgpTree, func(addr string, _, _ map[string]any) {
		addrs[addr] = true
	})

	assert.Len(t, addrs, 2)
	assert.True(t, addrs["10.0.0.1"])
	assert.True(t, addrs["10.0.0.2"])
}

// TestForEachPeer_NilPeerMap verifies peers with no config fields.
//
// VALIDATES: ForEachPeer handles nil/non-map peer entries gracefully.
// PREVENTS: Panic on peer entries that are not maps.
func TestForEachPeer_NilPeerMap(t *testing.T) {
	bgpTree := map[string]any{
		"peer": map[string]any{
			"192.0.2.1": nil, // No config
		},
	}

	var visited bool
	ForEachPeer(bgpTree, func(addr string, peerMap, _ map[string]any) {
		assert.Equal(t, "192.0.2.1", addr)
		assert.Nil(t, peerMap)
		visited = true
	})
	assert.True(t, visited)
}

// TestForEachPeer_EmptyTree verifies no visits on empty tree.
//
// VALIDATES: ForEachPeer handles empty/missing peer and group keys.
// PREVENTS: Panic on empty config.
func TestForEachPeer_EmptyTree(t *testing.T) {
	var count int
	ForEachPeer(map[string]any{}, func(_ string, _, _ map[string]any) {
		count++
	})
	assert.Zero(t, count)
}

// TestGetCapability_Present verifies capability extraction from session config.
//
// VALIDATES: GetCapability navigates session.capability correctly.
// PREVENTS: Wrong path for capability lookup.
func TestGetCapability_Present(t *testing.T) {
	m := map[string]any{
		"session": map[string]any{
			"capability": map[string]any{
				"route-refresh":    true,
				"graceful-restart": map[string]any{"time": float64(120)},
			},
		},
	}

	caps := GetCapability(m)
	require.NotNil(t, caps)
	assert.Equal(t, true, caps["route-refresh"])
	gr, ok := caps["graceful-restart"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(120), gr["time"])
}

// TestGetCapability_NoSession verifies nil return when session is absent.
//
// VALIDATES: GetCapability returns nil when session key is missing.
// PREVENTS: Panic on config without session block.
func TestGetCapability_NoSession(t *testing.T) {
	m := map[string]any{"connection": map[string]any{}}
	assert.Nil(t, GetCapability(m))
}

// TestGetCapability_NoCapability verifies nil return when capability is absent.
//
// VALIDATES: GetCapability returns nil when session exists but capability does not.
// PREVENTS: False positive on session-only config.
func TestGetCapability_NoCapability(t *testing.T) {
	m := map[string]any{
		"session": map[string]any{"asn": map[string]any{"local": float64(65000)}},
	}
	assert.Nil(t, GetCapability(m))
}

// TestGetCapability_NilInput verifies nil return on nil input.
//
// VALIDATES: GetCapability handles nil map gracefully.
// PREVENTS: Panic on nil.
func TestGetCapability_NilInput(t *testing.T) {
	assert.Nil(t, GetCapability(nil))
}
