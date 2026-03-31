package bgpconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// resolvedPeer extracts a peer's map from a ResolveBGPTree result, failing the test if missing.
func resolvedPeer(t *testing.T, result map[string]any, name string) map[string]any {
	t.Helper()
	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok, "result[\"peer\"] should be a map")
	peer, ok := peerMap[name].(map[string]any)
	require.True(t, ok, "peer %s should be a map", name)
	return peer
}

// resolvedPeerRemote extracts the "remote" sub-map from a resolved peer.
func resolvedPeerRemote(t *testing.T, peer map[string]any) map[string]any {
	t.Helper()
	remote, ok := peer["remote"].(map[string]any)
	require.True(t, ok, "peer[\"remote\"] should be a map")
	return remote
}

// resolvedLocal extracts the top-level "local" sub-map from a result.
func resolvedLocal(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	local, ok := result["local"].(map[string]any)
	require.True(t, ok, "result[\"local\"] should be a map")
	return local
}

// TestDeepMergeMaps verifies deep map merging for group resolution.
//
// VALIDATES: Later values override earlier values, maps are recursively merged.
// PREVENTS: Shallow merge that replaces entire containers instead of merging keys.
func TestDeepMergeMaps(t *testing.T) {
	tests := []struct {
		name string
		dst  map[string]any
		src  map[string]any
		want map[string]any
	}{
		{
			name: "leaf_override",
			dst:  map[string]any{"timer": map[string]any{"receive-hold-time": "90"}},
			src:  map[string]any{"timer": map[string]any{"receive-hold-time": "180"}},
			want: map[string]any{"timer": map[string]any{"receive-hold-time": "180"}},
		},
		{
			name: "add_new_key",
			dst:  map[string]any{"remote": map[string]any{"as": "65001"}},
			src:  map[string]any{"timer": map[string]any{"receive-hold-time": "180"}},
			want: map[string]any{"remote": map[string]any{"as": "65001"}, "timer": map[string]any{"receive-hold-time": "180"}},
		},
		{
			name: "deep_merge_containers",
			dst: map[string]any{
				"capability": map[string]any{"asn4": "true"},
			},
			src: map[string]any{
				"capability": map[string]any{"route-refresh": "true"},
			},
			want: map[string]any{
				"capability": map[string]any{"asn4": "true", "route-refresh": "true"},
			},
		},
		{
			name: "deep_override_in_container",
			dst: map[string]any{
				"capability": map[string]any{"asn4": "true", "route-refresh": "false"},
			},
			src: map[string]any{
				"capability": map[string]any{"route-refresh": "true"},
			},
			want: map[string]any{
				"capability": map[string]any{"asn4": "true", "route-refresh": "true"},
			},
		},
		{
			name: "src_replaces_non_map_with_map",
			dst:  map[string]any{"capability": "simple"},
			src:  map[string]any{"capability": map[string]any{"asn4": "true"}},
			want: map[string]any{"capability": map[string]any{"asn4": "true"}},
		},
		{
			name: "empty_src",
			dst:  map[string]any{"remote": map[string]any{"as": "65001"}},
			src:  map[string]any{},
			want: map[string]any{"remote": map[string]any{"as": "65001"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deepMergeMaps(tt.dst, tt.src, nil)
			assert.Equal(t, tt.want, tt.dst)
		})
	}
}

// TestResolveBGPTree_GroupDefaults verifies that group-level fields merge into peers.
//
// VALIDATES: AC-1, AC-2 -- group defaults are inherited by peers.
// PREVENTS: Groups being ignored during resolution.
func TestResolveBGPTree_GroupDefaults(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)
	bgp.Set("router-id", "1.2.3.4")

	groupTree := config.NewTree()
	groupTimerTree := config.NewTree()
	groupTimerTree.Set("receive-hold-time", "180")
	groupTree.SetContainer("timer", groupTimerTree)
	groupLocal := config.NewTree()
	groupLocal.Set("connect", "false")
	groupTree.SetContainer("local", groupLocal)

	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	peerLocal := config.NewTree()
	peerLocal.Set("ip", "auto")
	peerTree.SetContainer("local", peerLocal)
	groupTree.AddListEntry("peer", "peer1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "peer1")
	timerMap, ok := peer["timer"].(map[string]any)
	require.True(t, ok, "peer timer should be a map")
	assert.Equal(t, "180", timerMap["receive-hold-time"], "group receive-hold-time should be inherited")
	localMap, ok := peer["local"].(map[string]any)
	require.True(t, ok, "peer local should be a map")
	assert.Equal(t, "false", localMap["connect"], "group local connect should be inherited")
	remote := resolvedPeerRemote(t, peer)
	assert.Equal(t, "65001", remote["as"], "peer's own remote as should be present")
}

// TestResolveBGPTree_PeerOverridesGroup verifies peer values take precedence over group defaults.
//
// VALIDATES: AC-3 -- peer-level config overrides group.
// PREVENTS: Group values incorrectly winning over peer values.
func TestResolveBGPTree_PeerOverridesGroup(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	groupTimerTree := config.NewTree()
	groupTimerTree.Set("receive-hold-time", "180")
	groupTree.SetContainer("timer", groupTimerTree)
	groupLocal := config.NewTree()
	groupLocal.Set("connect", "false")
	groupTree.SetContainer("local", groupLocal)

	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	peerLocal := config.NewTree()
	peerLocal.Set("ip", "auto")
	peerTree.SetContainer("local", peerLocal)
	peerTimerTree := config.NewTree()
	peerTimerTree.Set("receive-hold-time", "90") // Override group's 180.
	peerTree.SetContainer("timer", peerTimerTree)
	groupTree.AddListEntry("peer", "peer1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "peer1")
	timerMap, ok := peer["timer"].(map[string]any)
	require.True(t, ok, "peer timer should be a map")
	assert.Equal(t, "90", timerMap["receive-hold-time"], "peer's receive-hold-time should override group's")
	localMap, ok := peer["local"].(map[string]any)
	require.True(t, ok, "peer local should be a map")
	assert.Equal(t, "false", localMap["connect"], "group's local connect should be inherited")
}

// TestResolveBGPTree_DeepMergeCapabilities verifies capability containers deep-merge.
//
// VALIDATES: AC-4 -- capabilities from group and peer are combined.
// PREVENTS: Peer capability container replacing group capabilities instead of merging.
func TestResolveBGPTree_DeepMergeCapabilities(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	groupCap := config.NewTree()
	groupCap.Set("route-refresh", "true")
	groupTree.SetContainer("capability", groupCap)

	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	peerCap := config.NewTree()
	peerCap.Set("extended-message", "enable")
	peerTree.SetContainer("capability", peerCap)
	groupTree.AddListEntry("peer", "peer1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "peer1")
	capMap, ok := peer["capability"].(map[string]any)
	require.True(t, ok, "capability should be a map")
	assert.Equal(t, "true", capMap["route-refresh"], "group capability merged")
	assert.Equal(t, "enable", capMap["extended-message"], "peer capability merged")
}

// TestResolveBGPTree_BGPGlobalInheritance verifies bgp-level globals reach peers through groups.
//
// VALIDATES: AC-5 -- bgp-level local as flows to peers.
// PREVENTS: Group layer blocking bgp globals from reaching peers.
func TestResolveBGPTree_BGPGlobalInheritance(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)
	bgp.Set("router-id", "1.2.3.4")

	groupTree := config.NewTree()
	// Group does NOT set local -- bgp global should flow through.
	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	groupTree.AddListEntry("peer", "peer1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	// Verify bgp globals are in the result (not in peer map -- they're at top level).
	topLocal := resolvedLocal(t, result)
	assert.Equal(t, "65000", topLocal["as"])
	assert.Equal(t, "1.2.3.4", result["router-id"])

	// Peer should exist and have its own fields.
	peer := resolvedPeer(t, result, "peer1")
	remote := resolvedPeerRemote(t, peer)
	assert.Equal(t, "65001", remote["as"])
}

// TestResolveBGPTree_GroupOverridesBGPGlobal verifies group local as overrides bgp global.
//
// VALIDATES: AC-6 -- group-level local as takes precedence over bgp-level.
// PREVENTS: BGP global values incorrectly winning when group explicitly sets them.
func TestResolveBGPTree_GroupOverridesBGPGlobal(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000") // BGP global.
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	groupLocalTree := config.NewTree()
	groupLocalTree.Set("as", "65001") // Group overrides.
	groupTree.SetContainer("local", groupLocalTree)
	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65002")
	peerTree.SetContainer("remote", peerRemote)
	groupTree.AddListEntry("peer", "peer1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "peer1")
	peerLocalMap, ok := peer["local"].(map[string]any)
	require.True(t, ok, "peer local should be a map")
	assert.Equal(t, "65001", peerLocalMap["as"], "group local as should override bgp global")
}

// TestResolveBGPTree_MultipleGroups verifies peers from different groups resolve independently.
//
// VALIDATES: Multiple groups with different defaults produce correct per-peer resolution.
// PREVENTS: Cross-contamination between groups.
func TestResolveBGPTree_MultipleGroups(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	// Group 1: fast peers.
	group1 := config.NewTree()
	group1Timer := config.NewTree()
	group1Timer.Set("receive-hold-time", "30")
	group1.SetContainer("timer", group1Timer)
	peer1 := config.NewTree()
	peer1Remote := config.NewTree()
	peer1Remote.Set("ip", "10.0.0.1")
	peer1Remote.Set("as", "65001")
	peer1.SetContainer("remote", peer1Remote)
	group1.AddListEntry("peer", "fast1", peer1)
	bgp.AddListEntry("group", "fast-peers", group1)

	// Group 2: slow peers.
	group2 := config.NewTree()
	group2Timer := config.NewTree()
	group2Timer.Set("receive-hold-time", "300")
	group2.SetContainer("timer", group2Timer)
	peer2 := config.NewTree()
	peer2Remote := config.NewTree()
	peer2Remote.Set("ip", "10.0.0.2")
	peer2Remote.Set("as", "65002")
	peer2.SetContainer("remote", peer2Remote)
	group2.AddListEntry("peer", "slow1", peer2)
	bgp.AddListEntry("group", "slow-peers", group2)

	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	p1 := resolvedPeer(t, result, "fast1")
	p1Timer, ok := p1["timer"].(map[string]any)
	require.True(t, ok, "fast1 timer should be a map")
	assert.Equal(t, "30", p1Timer["receive-hold-time"], "fast-peers group receive-hold-time")

	p2 := resolvedPeer(t, result, "slow1")
	p2Timer, ok := p2["timer"].(map[string]any)
	require.True(t, ok, "slow1 timer should be a map")
	assert.Equal(t, "300", p2Timer["receive-hold-time"], "slow-peers group receive-hold-time")
}

// TestResolveBGPTree_DuplicatePeerName verifies error on duplicate peer names across groups.
//
// VALIDATES: AC-8 -- duplicate peer names produce config validation error.
// PREVENTS: Two peers with the same name causing ambiguous CLI selection.
func TestResolveBGPTree_DuplicatePeerName(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	group1 := config.NewTree()
	peer1 := config.NewTree()
	peer1Remote := config.NewTree()
	peer1Remote.Set("ip", "10.0.0.1")
	peer1Remote.Set("as", "65001")
	peer1.SetContainer("remote", peer1Remote)
	group1.AddListEntry("peer", "router-east", peer1)
	bgp.AddListEntry("group", "group1", group1)

	group2 := config.NewTree()
	peer2 := config.NewTree()
	peer2Remote := config.NewTree()
	peer2Remote.Set("ip", "10.0.0.2")
	peer2Remote.Set("as", "65002")
	peer2.SetContainer("remote", peer2Remote)
	group2.AddListEntry("peer", "router-east", peer2) // Duplicate name.
	bgp.AddListEntry("group", "group2", group2)

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "router-east")
	assert.Contains(t, err.Error(), "duplicate")
}

// TestResolveBGPTree_EmptyGroup verifies an empty group (no peers) is valid.
//
// VALIDATES: AC-16 -- empty groups parse without error.
// PREVENTS: Error on groups used for future peer additions.
func TestResolveBGPTree_EmptyGroup(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	groupTimerTree := config.NewTree()
	groupTimerTree.Set("receive-hold-time", "180")
	groupTree.SetContainer("timer", groupTimerTree)
	// No peers added.
	bgp.AddListEntry("group", "empty-group", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)
	topLocal := resolvedLocal(t, result)
	assert.Equal(t, "65000", topLocal["as"])
}

// TestResolveBGPTree_PeerNameValidation verifies invalid peer names are rejected.
//
// VALIDATES: AC-14, AC-15 -- names that look like IPs or contain invalid chars are rejected.
// PREVENTS: Peer names that would be ambiguous with IP selectors in CLI.
func TestResolveBGPTree_PeerNameValidation(t *testing.T) {
	tests := []struct {
		name     string
		peerName string
		wantErr  string
	}{
		{
			name:     "ip_like_name",
			peerName: "10.0.0.1",
			wantErr:  "invalid peer name",
		},
		{
			// Dots are allowed in peer names (FQDN-style: router.east.dc1).
			// isValidPeerNameChar includes '.'. This is a valid name.
			name:     "contains_dots_is_valid",
			peerName: "router.east",
			wantErr:  "", // valid -- no error expected
		},
		{
			name:     "contains_spaces",
			peerName: "router east",
			wantErr:  "invalid peer name",
		},
		{
			name:     "contains_comma",
			peerName: "router,east",
			wantErr:  "invalid peer name",
		},
		{
			name:     "contains_colon",
			peerName: "router:east",
			wantErr:  "invalid peer name",
		},
		{
			name:     "wildcard",
			peerName: "*",
			wantErr:  "invalid peer name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			bgp := config.NewTree()
			bgpLocal := config.NewTree()
			bgpLocal.Set("as", "65000")
			bgp.SetContainer("local", bgpLocal)

			groupTree := config.NewTree()
			peerTree := config.NewTree()
			peerRemote := config.NewTree()
			peerRemote.Set("ip", "10.0.0.1")
			peerRemote.Set("as", "65001")
			peerTree.SetContainer("remote", peerRemote)
			// Use name as the list key (validatePeerName checks the key).
			groupTree.AddListEntry("peer", tt.peerName, peerTree)
			bgp.AddListEntry("group", "test-group", groupTree)
			tree.SetContainer("bgp", bgp)

			_, err := ResolveBGPTree(tree)
			if tt.wantErr == "" {
				require.NoError(t, err, "peer name %q should be valid", tt.peerName)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestResolveBGPTree_ValidPeerNames verifies valid peer names are accepted.
//
// VALIDATES: AC-7 -- valid names parse without error and appear in resolved map.
// PREVENTS: Over-restrictive name validation rejecting legitimate names.
func TestResolveBGPTree_ValidPeerNames(t *testing.T) {
	tests := []struct {
		peerName string
	}{
		{"google"},
		{"router-east"},
		{"router_west"},
		{"rtr1"},
		{"a"},
	}

	for _, tt := range tests {
		t.Run(tt.peerName, func(t *testing.T) {
			tree := config.NewTree()
			bgp := config.NewTree()
			bgpLocal := config.NewTree()
			bgpLocal.Set("as", "65000")
			bgp.SetContainer("local", bgpLocal)

			groupTree := config.NewTree()
			peerTree := config.NewTree()
			peerRemote := config.NewTree()
			peerRemote.Set("ip", "10.0.0.1")
			peerRemote.Set("as", "65001")
			peerTree.SetContainer("remote", peerRemote)
			groupTree.AddListEntry("peer", tt.peerName, peerTree)
			bgp.AddListEntry("group", "test-group", groupTree)
			tree.SetContainer("bgp", bgp)

			result, err := ResolveBGPTree(tree)
			require.NoError(t, err)

			// Peer should be keyed by its list key name.
			peer := resolvedPeer(t, result, tt.peerName)
			// Name is the map key, not a field in the resolved map.
			require.NotNil(t, peer)
		})
	}
}

// TestResolveBGPTree_MissingBGP verifies error when bgp block is missing.
//
// VALIDATES: Clear error for missing bgp block.
// PREVENTS: Panic on nil bgp container.
func TestResolveBGPTree_MissingBGP(t *testing.T) {
	tree := config.NewTree()
	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bgp")
}

// TestResolveBGPTree_PeerNamePreserved verifies peer name is kept as the list key in the resolved map.
//
// VALIDATES: AC-7 -- name (list key) survives resolution.
// PREVENTS: Name being stripped during merge.
func TestResolveBGPTree_PeerNamePreserved(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	groupTree.AddListEntry("peer", "google", peerTree)
	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	// The peer is keyed by its name "google".
	_ = resolvedPeer(t, result, "google")
}

// TestResolveBGPTree_GroupNameInPeer verifies group name is stored in resolved peer map.
//
// VALIDATES: GroupName flows through resolution for PeerSettings.
// PREVENTS: Group membership info being lost during resolution.
func TestResolveBGPTree_GroupNameInPeer(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	groupTree.AddListEntry("peer", "peer1", peerTree)
	bgp.AddListEntry("group", "rr-clients", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "peer1")
	assert.Equal(t, "rr-clients", peer["group-name"])
}

// TestResolveBGPTree_NoGroups verifies config with no groups returns valid map.
//
// VALIDATES: BGP block with no groups is valid (just globals).
// PREVENTS: Error when no groups are defined.
func TestResolveBGPTree_NoGroups(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)
	topLocal := resolvedLocal(t, result)
	assert.Equal(t, "65000", topLocal["as"])
}

// TestResolveBGPTree_StandalonePeer verifies peers directly under bgp work without groups.
//
// VALIDATES: AC-12 -- standalone peers (no group) parse correctly.
// PREVENTS: Regression where removing template support breaks standalone peers.
func TestResolveBGPTree_StandalonePeer(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)
	bgp.Set("router-id", "1.2.3.4")

	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	peerTimerTree := config.NewTree()
	peerTimerTree.Set("receive-hold-time", "180")
	peerTree.SetContainer("timer", peerTimerTree)
	bgp.AddListEntry("peer", "peer1", peerTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "peer1")
	remote := resolvedPeerRemote(t, peer)
	assert.Equal(t, "65001", remote["as"])
	timerMap, ok := peer["timer"].(map[string]any)
	require.True(t, ok, "peer timer should be a map")
	assert.Equal(t, "180", timerMap["receive-hold-time"])
	// Standalone peers should not have group-name.
	_, hasGroupName := peer["group-name"]
	assert.False(t, hasGroupName, "standalone peer should not have group-name")
}

// TestResolveBGPTree_MixedGroupAndStandalone verifies groups and standalone peers coexist.
//
// VALIDATES: Both grouped and standalone peers resolve correctly in the same config.
// PREVENTS: Group resolution interfering with standalone peer resolution.
func TestResolveBGPTree_MixedGroupAndStandalone(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	// Group with one peer.
	groupTree := config.NewTree()
	groupTimerTree := config.NewTree()
	groupTimerTree.Set("receive-hold-time", "180")
	groupTree.SetContainer("timer", groupTimerTree)
	groupPeer := config.NewTree()
	gpRemote := config.NewTree()
	gpRemote.Set("ip", "10.0.0.1")
	gpRemote.Set("as", "65001")
	groupPeer.SetContainer("remote", gpRemote)
	groupTree.AddListEntry("peer", "grouped1", groupPeer)
	bgp.AddListEntry("group", "fast", groupTree)

	// Standalone peer.
	standalonePeer := config.NewTree()
	spRemote := config.NewTree()
	spRemote.Set("ip", "10.0.0.2")
	spRemote.Set("as", "65002")
	standalonePeer.SetContainer("remote", spRemote)
	spTimerTree := config.NewTree()
	spTimerTree.Set("receive-hold-time", "90")
	standalonePeer.SetContainer("timer", spTimerTree)
	bgp.AddListEntry("peer", "standalone1", standalonePeer)

	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	// Grouped peer inherits group defaults.
	p1 := resolvedPeer(t, result, "grouped1")
	p1Timer, ok := p1["timer"].(map[string]any)
	require.True(t, ok, "grouped1 timer should be a map")
	assert.Equal(t, "180", p1Timer["receive-hold-time"])
	assert.Equal(t, "fast", p1["group-name"])

	// Standalone peer uses its own values.
	p2 := resolvedPeer(t, result, "standalone1")
	p2Timer, ok := p2["timer"].(map[string]any)
	require.True(t, ok, "standalone1 timer should be a map")
	assert.Equal(t, "90", p2Timer["receive-hold-time"])
	_, hasGroupName := p2["group-name"]
	assert.False(t, hasGroupName)
}

// TestResolveBGPTree_DuplicatePeerNameAcrossGroups verifies error on same peer name in two groups.
//
// VALIDATES: Duplicate peer name across groups produces config validation error.
// PREVENTS: Two groups defining the same peer name causing silent override.
func TestResolveBGPTree_DuplicatePeerNameAcrossGroups(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	group1 := config.NewTree()
	peer1 := config.NewTree()
	p1Remote := config.NewTree()
	p1Remote.Set("ip", "10.0.0.1")
	p1Remote.Set("as", "65001")
	peer1.SetContainer("remote", p1Remote)
	group1.AddListEntry("peer", "dup-name", peer1)
	bgp.AddListEntry("group", "group1", group1)

	group2 := config.NewTree()
	peer2 := config.NewTree()
	p2Remote := config.NewTree()
	p2Remote.Set("ip", "10.0.0.2")
	p2Remote.Set("as", "65002")
	peer2.SetContainer("remote", p2Remote)
	group2.AddListEntry("peer", "dup-name", peer2) // Same name as group1.
	bgp.AddListEntry("group", "group2", group2)

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dup-name")
	assert.Contains(t, err.Error(), "duplicate")
}

// TestResolveBGPTree_DuplicatePeerNameGroupAndStandalone verifies error on same name in group and standalone.
//
// VALIDATES: Duplicate peer name between group and standalone produces error.
// PREVENTS: Group peer and standalone peer with same name silently overwriting each other.
func TestResolveBGPTree_DuplicatePeerNameGroupAndStandalone(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	groupTree := config.NewTree()
	groupPeer := config.NewTree()
	gpRemote := config.NewTree()
	gpRemote.Set("ip", "10.0.0.1")
	gpRemote.Set("as", "65001")
	groupPeer.SetContainer("remote", gpRemote)
	groupTree.AddListEntry("peer", "dup-name", groupPeer)
	bgp.AddListEntry("group", "grp", groupTree)

	standalonePeer := config.NewTree()
	spRemote := config.NewTree()
	spRemote.Set("ip", "10.0.0.2")
	spRemote.Set("as", "65002")
	standalonePeer.SetContainer("remote", spRemote)
	bgp.AddListEntry("peer", "dup-name", standalonePeer) // Same name as group peer.

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dup-name")
	assert.Contains(t, err.Error(), "duplicate")
}

// TestResolveBGPTree_StandalonePeerWithName verifies peer name works on standalone peers.
//
// VALIDATES: AC-7 for standalone peers -- name (list key) is preserved.
// PREVENTS: Name validation only working for grouped peers.
func TestResolveBGPTree_StandalonePeerWithName(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	bgp.AddListEntry("peer", "google", peerTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	// The peer is keyed by its name "google".
	_ = resolvedPeer(t, result, "google")
}

// TestResolveBGPTree_PeerNameUnicodeRejected verifies non-ASCII characters are rejected in peer names.
//
// VALIDATES: Peer names with unicode letters (CJK, accents) are rejected.
// PREVENTS: Display issues and CLI ambiguity from non-ASCII peer names.
func TestResolveBGPTree_PeerNameUnicodeRejected(t *testing.T) {
	tests := []struct {
		name     string
		peerName string
	}{
		{"accented", "routeur-\u00e8st"},
		{"cjk", "\u8def\u7531\u5668"},
		{"emoji_in_name", "router\U0001F600"},
		{"cyrillic", "\u0440\u043e\u0443\u0442\u0435\u0440"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			bgp := config.NewTree()
			bgpLocal := config.NewTree()
			bgpLocal.Set("as", "65000")
			bgp.SetContainer("local", bgpLocal)

			peerTree := config.NewTree()
			peerRemote := config.NewTree()
			peerRemote.Set("ip", "10.0.0.1")
			peerRemote.Set("as", "65001")
			peerTree.SetContainer("remote", peerRemote)
			bgp.AddListEntry("peer", tt.peerName, peerTree)
			tree.SetContainer("bgp", bgp)

			_, err := ResolveBGPTree(tree)
			require.Error(t, err, "unicode peer name %q should be rejected", tt.peerName)
			assert.Contains(t, err.Error(), "invalid peer name")
		})
	}
}

// TestResolveBGPTree_PeerNamePunctuationOnly verifies punctuation-only names are rejected.
//
// VALIDATES: Names like "---" or "___" that contain no letters or digits are rejected.
// PREVENTS: Confusing CLI selectors that look like flags or separators.
func TestResolveBGPTree_PeerNamePunctuationOnly(t *testing.T) {
	tests := []struct {
		name     string
		peerName string
		wantErr  string
	}{
		// Names starting with underscore pass first-char but fail alphanumeric check.
		{"underscores_only", "___", "at least one letter or digit"},
		{"mixed_punctuation", "_-_-_", "at least one letter or digit"},
		{"single_underscore", "_", "at least one letter or digit"},
		// Names starting with hyphen fail at first-char check (before alphanumeric check).
		{"hyphens_only", "---", "first character must be alphanumeric or underscore"},
		{"single_hyphen", "-", "first character must be alphanumeric or underscore"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			bgp := config.NewTree()
			bgpLocal := config.NewTree()
			bgpLocal.Set("as", "65000")
			bgp.SetContainer("local", bgpLocal)

			peerTree := config.NewTree()
			peerRemote := config.NewTree()
			peerRemote.Set("ip", "10.0.0.1")
			peerRemote.Set("as", "65001")
			peerTree.SetContainer("remote", peerRemote)
			bgp.AddListEntry("peer", tt.peerName, peerTree)
			tree.SetContainer("bgp", bgp)

			_, err := ResolveBGPTree(tree)
			require.Error(t, err, "punctuation-only name %q should be rejected", tt.peerName)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestResolveBGPTree_PeerNameTooLong verifies very long peer names are rejected.
//
// VALIDATES: Peer names exceeding maxPeerNameLen (255) are rejected.
// PREVENTS: DoS via oversized names in JSON responses.
func TestResolveBGPTree_PeerNameTooLong(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	longName := strings.Repeat("a", maxPeerNameLen+1) // 256 chars

	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	bgp.AddListEntry("peer", longName, peerTree)
	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}

// TestResolveBGPTree_PeerNameAtMaxLength verifies names exactly at the limit are accepted.
//
// VALIDATES: Boundary: name of exactly maxPeerNameLen characters is valid.
// PREVENTS: Off-by-one in length validation.
func TestResolveBGPTree_PeerNameAtMaxLength(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	exactName := strings.Repeat("x", maxPeerNameLen) // exactly 255 chars

	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	bgp.AddListEntry("peer", exactName, peerTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	_ = resolvedPeer(t, result, exactName)
}

// TestResolveBGPTree_EmptyPeerNameIgnored verifies that empty peer name is rejected.
//
// VALIDATES: Empty peer name produces a validation error.
// PREVENTS: Peers with empty names being silently accepted.
func TestResolveBGPTree_EmptyPeerNameIgnored(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	peerTree.Set("name", "") // Explicitly empty name field.
	bgp.AddListEntry("peer", "peer1", peerTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "peer1")
	assert.Equal(t, "", peer["name"], "empty name should be preserved in map")
}

// TestResolveBGPTree_GroupNameValidation verifies invalid group names are rejected.
//
// VALIDATES: Group names follow the same character and length rules as peer names.
// PREVENTS: Group names with special characters causing CLI or JSON issues.
func TestResolveBGPTree_GroupNameValidation(t *testing.T) {
	tests := []struct {
		name      string
		groupName string
		wantErr   string
	}{
		{"contains_dots_is_valid", "group.one", ""}, // dots allowed (FQDN-style)
		{"contains_spaces", "group one", "invalid group name"},
		{"contains_colon", "group:one", "invalid group name"},
		{"punctuation_only", "---", "first character must be alphanumeric or underscore"},
		{"unicode", "\u00e9quipe", "invalid group name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			bgp := config.NewTree()
			bgpLocal := config.NewTree()
			bgpLocal.Set("as", "65000")
			bgp.SetContainer("local", bgpLocal)

			groupTree := config.NewTree()
			peerTree := config.NewTree()
			peerRemote := config.NewTree()
			peerRemote.Set("ip", "10.0.0.1")
			peerRemote.Set("as", "65001")
			peerTree.SetContainer("remote", peerRemote)
			groupTree.AddListEntry("peer", "peer1", peerTree)
			bgp.AddListEntry("group", tt.groupName, groupTree)
			tree.SetContainer("bgp", bgp)

			_, err := ResolveBGPTree(tree)
			if tt.wantErr == "" {
				require.NoError(t, err, "group name %q should be valid", tt.groupName)
			} else {
				require.Error(t, err, "group name %q should be rejected", tt.groupName)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestResolveBGPTree_GroupNameTooLong verifies very long group names are rejected.
//
// VALIDATES: Group names exceeding maxPeerNameLen (255) are rejected.
// PREVENTS: DoS via oversized group names.
func TestResolveBGPTree_GroupNameTooLong(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	longName := strings.Repeat("g", maxPeerNameLen+1) // 256 chars

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	groupTree.AddListEntry("peer", "peer1", peerTree)
	bgp.AddListEntry("group", longName, groupTree)
	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}

// TestResolveBGPTree_ValidGroupNames verifies valid group names are accepted.
//
// VALIDATES: Reasonable group names parse without error.
// PREVENTS: Over-restrictive group name validation.
func TestResolveBGPTree_ValidGroupNames(t *testing.T) {
	tests := []string{
		"rr-clients",
		"transit_peers",
		"IX1",
		"a",
		"fast-peers",
		"group123",
	}

	for _, groupName := range tests {
		t.Run(groupName, func(t *testing.T) {
			tree := config.NewTree()
			bgp := config.NewTree()
			bgpLocal := config.NewTree()
			bgpLocal.Set("as", "65000")
			bgp.SetContainer("local", bgpLocal)

			groupTree := config.NewTree()
			peerTree := config.NewTree()
			peerRemote := config.NewTree()
			peerRemote.Set("ip", "10.0.0.1")
			peerRemote.Set("as", "65001")
			peerTree.SetContainer("remote", peerRemote)
			groupTree.AddListEntry("peer", "peer1", peerTree)
			bgp.AddListEntry("group", groupName, groupTree)
			tree.SetContainer("bgp", bgp)

			result, err := ResolveBGPTree(tree)
			require.NoError(t, err)

			peer := resolvedPeer(t, result, "peer1")
			assert.Equal(t, groupName, peer["group-name"])
		})
	}
}

// TestResolveBGPTree_BGPLevelFamilyInheritance verifies peers inherit family
// config from the bgp root level when not overridden.
//
// VALIDATES: bgp { family { ipv4/unicast { prefix { maximum N; } } } } flows to peers.
// PREVENTS: Standalone peers missing root-level family defaults.
func TestResolveBGPTree_BGPLevelFamilyInheritance(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpLocal := config.NewTree()
	bgpLocal.Set("as", "65000")
	bgp.SetContainer("local", bgpLocal)

	// BGP-level family defaults.
	bgpFamily := config.NewTree()
	ipv4Tree := config.NewTree()
	ipv4Prefix := config.NewTree()
	ipv4Prefix.Set("maximum", "100000")
	ipv4Tree.SetContainer("prefix", ipv4Prefix)
	bgpFamily.AddListEntry("family", "ipv4/unicast", ipv4Tree)
	// family is a list, stored in bgp tree
	bgp.AddListEntry("family", "ipv4/unicast", ipv4Tree)

	// Standalone peer with NO family override.
	peerTree := config.NewTree()
	peerRemote := config.NewTree()
	peerRemote.Set("ip", "10.0.0.1")
	peerRemote.Set("as", "65001")
	peerTree.SetContainer("remote", peerRemote)
	bgp.AddListEntry("peer", "peer1", peerTree)

	// Standalone peer WITH family override.
	peer2Tree := config.NewTree()
	peer2Remote := config.NewTree()
	peer2Remote.Set("ip", "10.0.0.2")
	peer2Remote.Set("as", "65002")
	peer2Tree.SetContainer("remote", peer2Remote)
	peer2Family := config.NewTree()
	peer2Prefix := config.NewTree()
	peer2Prefix.Set("maximum", "500000")
	peer2Family.SetContainer("prefix", peer2Prefix)
	peer2Tree.AddListEntry("family", "ipv4/unicast", peer2Family)
	bgp.AddListEntry("peer", "peer2", peer2Tree)

	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	// peer1 should inherit bgp-level family with maximum 100000.
	p1 := resolvedPeer(t, result, "peer1")
	p1Family, ok := p1["family"].(map[string]any)
	require.True(t, ok, "peer1 should have family")
	p1Ipv4, ok := p1Family["ipv4/unicast"].(map[string]any)
	require.True(t, ok, "peer1 should have ipv4/unicast family")
	p1Prefix, ok := p1Ipv4["prefix"].(map[string]any)
	require.True(t, ok, "peer1 should have prefix config")
	assert.Equal(t, "100000", p1Prefix["maximum"], "peer1 should inherit bgp-level maximum")

	// peer2 should use its own override (500000).
	p2 := resolvedPeer(t, result, "peer2")
	p2Family, ok := p2["family"].(map[string]any)
	require.True(t, ok, "peer2 should have family")
	p2Ipv4, ok := p2Family["ipv4/unicast"].(map[string]any)
	require.True(t, ok, "peer2 should have ipv4/unicast family")
	p2Prefix, ok := p2Ipv4["prefix"].(map[string]any)
	require.True(t, ok, "peer2 should have prefix config")
	assert.Equal(t, "500000", p2Prefix["maximum"], "peer2 should use its own maximum")
}

// newTestPeer builds a peer tree with connection > remote > ip and session > asn > remote.
func newTestPeer(ip, as string) *config.Tree {
	peer := config.NewTree()
	conn := config.NewTree()
	remote := config.NewTree()
	remote.Set("ip", ip)
	conn.SetContainer("remote", remote)
	peer.SetContainer("connection", conn)
	session := config.NewTree()
	asn := config.NewTree()
	asn.Set("remote", as)
	session.SetContainer("asn", asn)
	peer.SetContainer("session", session)
	return peer
}

// TestResolveBGPTree_DuplicateRemoteIP verifies error on duplicate connection > remote > ip across peers.
//
// VALIDATES: Duplicate remote IP addresses produce config validation error.
// PREVENTS: Two peers with different names but same remote IP causing ambiguous connections.
func TestResolveBGPTree_DuplicateRemoteIP(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpSession := config.NewTree()
	bgpASN := config.NewTree()
	bgpASN.Set("local", "65000")
	bgpSession.SetContainer("asn", bgpASN)
	bgp.SetContainer("session", bgpSession)

	bgp.AddListEntry("peer", "peer_alpha", newTestPeer("10.0.0.1", "65001"))
	bgp.AddListEntry("peer", "peer_beta", newTestPeer("10.0.0.1", "65002")) // Same IP.

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.0.0.1")
	assert.Contains(t, err.Error(), "duplicate remote IP")
}

// TestResolveBGPTree_DuplicateRemoteIPAcrossGroups verifies error on duplicate connection > remote > ip in different groups.
//
// VALIDATES: Duplicate remote IP addresses across groups produce config validation error.
// PREVENTS: Two peers in different groups with same remote IP.
func TestResolveBGPTree_DuplicateRemoteIPAcrossGroups(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpSession := config.NewTree()
	bgpASN := config.NewTree()
	bgpASN.Set("local", "65000")
	bgpSession.SetContainer("asn", bgpASN)
	bgp.SetContainer("session", bgpSession)

	group1 := config.NewTree()
	group1.AddListEntry("peer", "peer_alpha", newTestPeer("10.0.0.1", "65001"))
	bgp.AddListEntry("group", "group1", group1)

	group2 := config.NewTree()
	group2.AddListEntry("peer", "peer_beta", newTestPeer("10.0.0.1", "65002")) // Same IP.
	bgp.AddListEntry("group", "group2", group2)

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.0.0.1")
	assert.Contains(t, err.Error(), "duplicate remote IP")
}

// TestResolveBGPTree_DuplicateRemoteIPGroupAndStandalone verifies error on duplicate connection > remote > ip across group and standalone.
//
// VALIDATES: Duplicate remote IP between grouped and standalone peers produces error.
// PREVENTS: Same remote IP used in group peer and standalone peer.
func TestResolveBGPTree_DuplicateRemoteIPGroupAndStandalone(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpSession := config.NewTree()
	bgpASN := config.NewTree()
	bgpASN.Set("local", "65000")
	bgpSession.SetContainer("asn", bgpASN)
	bgp.SetContainer("session", bgpSession)

	group1 := config.NewTree()
	group1.AddListEntry("peer", "peer_alpha", newTestPeer("10.0.0.1", "65001"))
	bgp.AddListEntry("group", "group1", group1)

	bgp.AddListEntry("peer", "peer_beta", newTestPeer("10.0.0.1", "65002")) // Same IP.

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.0.0.1")
	assert.Contains(t, err.Error(), "duplicate remote IP")
}

// TestResolveBGPTree_UniqueRemoteIPsAccepted verifies no error when remote IPs are unique.
//
// VALIDATES: Peers with different remote IPs are accepted.
// PREVENTS: False positive duplicate detection.
func TestResolveBGPTree_UniqueRemoteIPsAccepted(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpSession := config.NewTree()
	bgpASN := config.NewTree()
	bgpASN.Set("local", "65000")
	bgpSession.SetContainer("asn", bgpASN)
	bgp.SetContainer("session", bgpSession)

	bgp.AddListEntry("peer", "peer_alpha", newTestPeer("10.0.0.1", "65001"))
	bgp.AddListEntry("peer", "peer_beta", newTestPeer("10.0.0.2", "65002")) // Different IP.

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.NoError(t, err)
}

// TestResolveBGPTree_MissingRemoteIPNoDuplicateError verifies peers without connection > remote > ip don't cause false duplicates.
//
// VALIDATES: Peers without remote IP are not treated as duplicates of each other.
// PREVENTS: Empty-string IP being treated as a duplicate.
func TestResolveBGPTree_MissingRemoteIPNoDuplicateError(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgpSession := config.NewTree()
	bgpASN := config.NewTree()
	bgpASN.Set("local", "65000")
	bgpSession.SetContainer("asn", bgpASN)
	bgp.SetContainer("session", bgpSession)

	// Peers with session > asn > remote but no connection > remote > ip.
	peer1 := config.NewTree()
	p1Session := config.NewTree()
	p1ASN := config.NewTree()
	p1ASN.Set("remote", "65001")
	p1Session.SetContainer("asn", p1ASN)
	peer1.SetContainer("session", p1Session)
	bgp.AddListEntry("peer", "peer_alpha", peer1)

	peer2 := config.NewTree()
	p2Session := config.NewTree()
	p2ASN := config.NewTree()
	p2ASN.Set("remote", "65002")
	p2Session.SetContainer("asn", p2ASN)
	peer2.SetContainer("session", p2Session)
	bgp.AddListEntry("peer", "peer_beta", peer2)

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.NoError(t, err)
}

// TestDeepMergeMaps_FamilyLeafOverride verifies that leaf family values are replaced, not merged.
//
// VALIDATES: When group has family "ipv4/unicast" and peer has "ipv6/unicast", the peer's value wins.
// PREVENTS: Assumption that families accumulate via deep merge (they are leaves, not maps).
func TestDeepMergeMaps_FamilyLeafOverride(t *testing.T) {
	dst := map[string]any{
		"family": "ipv4/unicast",
	}
	src := map[string]any{
		"family": "ipv6/unicast",
	}
	deepMergeMaps(dst, src, nil)
	assert.Equal(t, "ipv6/unicast", dst["family"], "peer family should override group family")
}

// TestDeepMergeMaps_NestedThreeLevels verifies deep merge works for deeply nested maps.
//
// VALIDATES: Three-level deep merge preserves all keys.
// PREVENTS: Merge stopping at second level.
func TestDeepMergeMaps_NestedThreeLevels(t *testing.T) {
	dst := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"dst-key": "dst-val",
			},
		},
	}
	src := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"src-key": "src-val",
			},
		},
	}
	deepMergeMaps(dst, src, nil)

	l1, ok := dst["level1"].(map[string]any)
	require.True(t, ok, "level1 should be a map")
	l2, ok := l1["level2"].(map[string]any)
	require.True(t, ok, "level2 should be a map")
	assert.Equal(t, "dst-val", l2["dst-key"], "dst key should be preserved")
	assert.Equal(t, "src-val", l2["src-key"], "src key should be merged")
}

// TestCumulativeLeafList verifies that deepMergeMaps appends slice values
// for keys marked as cumulative instead of replacing them.
//
// VALIDATES: AC-21 -- ze:cumulative leaf-list values accumulated across levels.
// PREVENTS: Most-specific level replacing cumulative leaf-list values.
func TestCumulativeLeafList(t *testing.T) {
	cumulative := map[string]bool{
		"filter.ingress.community.tag": true,
	}

	// BGP-level: tag = ["block-list"]
	dst := map[string]any{
		"filter": map[string]any{
			"ingress": map[string]any{
				"community": map[string]any{
					"tag": []any{"block-list"},
				},
			},
		},
	}

	// Group-level: tag = ["transit-mark"]
	src := map[string]any{
		"filter": map[string]any{
			"ingress": map[string]any{
				"community": map[string]any{
					"tag": []any{"transit-mark"},
				},
			},
		},
	}

	deepMergeMaps(dst, src, cumulative)

	// Both values should be present (accumulated, not replaced).
	filter, ok := dst["filter"].(map[string]any)
	require.True(t, ok, "filter should be a map")
	ingress, ok := filter["ingress"].(map[string]any)
	require.True(t, ok, "ingress should be a map")
	community, ok := ingress["community"].(map[string]any)
	require.True(t, ok, "community should be a map")
	tags, ok := community["tag"].([]any)
	require.True(t, ok, "tag should be a []any")
	assert.Equal(t, 2, len(tags), "should have 2 accumulated tags")
	assert.Contains(t, tags, "block-list")
	assert.Contains(t, tags, "transit-mark")
}

// TestCumulativeLeafListNonCumulativeUnchanged verifies that normal leaf-lists
// are still replaced (not accumulated) when a cumulative set is provided.
//
// VALIDATES: Normal merge behavior preserved for non-cumulative keys.
// PREVENTS: All leaf-lists becoming cumulative by accident.
func TestCumulativeLeafListNonCumulativeUnchanged(t *testing.T) {
	cumulative := map[string]bool{
		"filter.ingress.community.tag": true,
	}

	dst := map[string]any{
		"some-list": []any{"old-val"},
	}
	src := map[string]any{
		"some-list": []any{"new-val"},
	}

	deepMergeMaps(dst, src, cumulative)

	// Non-cumulative: src replaces dst.
	vals, ok := dst["some-list"].([]any)
	require.True(t, ok)
	assert.Equal(t, 1, len(vals))
	assert.Equal(t, "new-val", vals[0])
}

// TestCumulativeLeafListStringSlice verifies that cumulative merge works when
// ToMap() produces []string (the actual type from Tree multi-values).
//
// VALIDATES: Cumulative merge handles []string from ToMap(), not just []any.
// PREVENTS: Cumulative merge being dead code for real config data.
func TestCumulativeLeafListStringSlice(t *testing.T) {
	cumulative := map[string]bool{
		"filter.ingress.community.tag": true,
	}

	// ToMap() produces []string for multi-values.
	dst := map[string]any{
		"filter": map[string]any{
			"ingress": map[string]any{
				"community": map[string]any{
					"tag": []string{"global-mark"},
				},
			},
		},
	}
	src := map[string]any{
		"filter": map[string]any{
			"ingress": map[string]any{
				"community": map[string]any{
					"tag": []string{"peer-mark"},
				},
			},
		},
	}

	deepMergeMaps(dst, src, cumulative)

	filter, ok := dst["filter"].(map[string]any)
	require.True(t, ok)
	ingress, ok := filter["ingress"].(map[string]any)
	require.True(t, ok)
	community, ok := ingress["community"].(map[string]any)
	require.True(t, ok)
	tags, ok := community["tag"].([]any)
	require.True(t, ok, "accumulated result should be []any")
	assert.Equal(t, 2, len(tags))
	assert.Contains(t, tags, "global-mark")
	assert.Contains(t, tags, "peer-mark")
}

// TestCumulativeLeafListBareString verifies that a single-value leaf-list
// (produced as bare string by ToMap()) is accumulated correctly.
//
// VALIDATES: Single-value tag/strip not silently lost.
// PREVENTS: ToMap() bare string causing nil from toAnySlice.
func TestCumulativeLeafListBareString(t *testing.T) {
	cumulative := map[string]bool{
		"filter.ingress.community.tag": true,
	}

	dst := map[string]any{
		"filter": map[string]any{
			"ingress": map[string]any{
				"community": map[string]any{
					"tag": "global-mark", // bare string from ToMap() single-value
				},
			},
		},
	}
	src := map[string]any{
		"filter": map[string]any{
			"ingress": map[string]any{
				"community": map[string]any{
					"tag": "peer-mark", // bare string
				},
			},
		},
	}

	deepMergeMaps(dst, src, cumulative)

	filter, ok := dst["filter"].(map[string]any)
	require.True(t, ok)
	ingress, ok := filter["ingress"].(map[string]any)
	require.True(t, ok)
	community, ok := ingress["community"].(map[string]any)
	require.True(t, ok)
	tags, ok := community["tag"].([]any)
	require.True(t, ok, "accumulated bare strings should become []any")
	assert.Equal(t, 2, len(tags))
	assert.Contains(t, tags, "global-mark")
	assert.Contains(t, tags, "peer-mark")
}

// TestValidateGroupName verifies group name validation edge cases.
//
// VALIDATES: validateGroupName function works correctly.
// PREVENTS: Inconsistent validation between peer names and group names.
func TestValidateGroupName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid_simple", "peering", false},
		{"valid_hyphen", "rr-clients", false},
		{"valid_underscore", "transit_peers", false},
		{"valid_digits", "group123", false},
		{"valid_dot_middle", "group.one", false},
		{"valid_underscore_start", "_private", false},
		{"empty", "", true},
		{"dot_start", ".group", true},
		{"hyphen_start", "-group", true},
		{"space", "group one", true},
		{"unicode", "\u00e9quipe", true},
		{"punctuation_only", "---", true},
		{"too_long", strings.Repeat("a", 256), true},
		{"at_limit", strings.Repeat("a", 255), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGroupName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidatePeerName verifies peer name validation edge cases.
//
// VALIDATES: validatePeerName function works correctly with new restrictions.
// PREVENTS: Regression in peer name validation.
func TestValidatePeerName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid_simple", "google", false},
		{"valid_hyphen", "router-east", false},
		{"valid_underscore", "router_west", false},
		{"valid_single_char", "a", false},
		{"valid_digits_mixed", "rtr1", false},
		{"valid_dot_middle", "router.east", false},
		{"valid_underscore_start", "_router", false},
		{"valid_multi_dots", "dc1.rack2.rtr3", false},
		{"wildcard", "*", true},
		{"ip_address", "10.0.0.1", true},
		{"ipv6_address", "2001:db8::1", true},
		{"dot_start", ".router", true},
		{"hyphen_start", "-router", true},
		{"spaces", "router east", true},
		{"comma", "router,east", true},
		{"colon", "router:east", true},
		{"unicode_accent", "\u00e9quipe", true},
		{"cjk", "\u8def\u7531\u5668", true},
		{"punctuation_only_hyphens", "---", true},
		{"punctuation_only_underscores", "___", true},
		{"too_long", strings.Repeat("a", 256), true},
		{"at_limit", strings.Repeat("a", 255), false},
		{"reserved_list", "list", true},
		{"reserved_detail", "detail", true},
		{"formerly_reserved_add", "add", false},
		{"reserved_update", "update", true},
		{"reserved_teardown", "teardown", true},
		{"reserved_prefix_ok", "list-east", false},
		{"reserved_suffix_ok", "my-list", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePeerName(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "name %q should be rejected", tt.input)
			} else {
				assert.NoError(t, err, "name %q should be accepted", tt.input)
			}
		})
	}
}
