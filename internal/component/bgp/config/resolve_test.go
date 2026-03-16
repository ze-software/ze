package bgpconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// resolvedPeer extracts a peer's map from a ResolveBGPTree result, failing the test if missing.
func resolvedPeer(t *testing.T, result map[string]any, addr string) map[string]any {
	t.Helper()
	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok, "result[\"peer\"] should be a map")
	peer, ok := peerMap[addr].(map[string]any)
	require.True(t, ok, "peer %s should be a map", addr)
	return peer
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
			dst:  map[string]any{"hold-time": "90"},
			src:  map[string]any{"hold-time": "180"},
			want: map[string]any{"hold-time": "180"},
		},
		{
			name: "add_new_key",
			dst:  map[string]any{"peer-as": "65001"},
			src:  map[string]any{"hold-time": "180"},
			want: map[string]any{"peer-as": "65001", "hold-time": "180"},
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
			dst:  map[string]any{"peer-as": "65001"},
			src:  map[string]any{},
			want: map[string]any{"peer-as": "65001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deepMergeMaps(tt.dst, tt.src)
			assert.Equal(t, tt.want, tt.dst)
		})
	}
}

// TestResolveBGPTree_GroupDefaults verifies that group-level fields merge into peers.
//
// VALIDATES: AC-1, AC-2 — group defaults are inherited by peers.
// PREVENTS: Groups being ignored during resolution.
func TestResolveBGPTree_GroupDefaults(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")
	bgp.Set("router-id", "1.2.3.4")

	groupTree := config.NewTree()
	groupTree.Set("hold-time", "180")
	groupTree.Set("connection", "passive")

	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "180", peer["hold-time"], "group hold-time should be inherited")
	assert.Equal(t, "passive", peer["connection"], "group connection should be inherited")
	assert.Equal(t, "65001", peer["peer-as"], "peer's own peer-as should be present")
}

// TestResolveBGPTree_PeerOverridesGroup verifies peer values take precedence over group defaults.
//
// VALIDATES: AC-3 — peer-level config overrides group.
// PREVENTS: Group values incorrectly winning over peer values.
func TestResolveBGPTree_PeerOverridesGroup(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	groupTree := config.NewTree()
	groupTree.Set("hold-time", "180")
	groupTree.Set("connection", "passive")

	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("hold-time", "90") // Override group's 180.
	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "90", peer["hold-time"], "peer's hold-time should override group's")
	assert.Equal(t, "passive", peer["connection"], "group's connection should be inherited")
}

// TestResolveBGPTree_DeepMergeCapabilities verifies capability containers deep-merge.
//
// VALIDATES: AC-4 — capabilities from group and peer are combined.
// PREVENTS: Peer capability container replacing group capabilities instead of merging.
func TestResolveBGPTree_DeepMergeCapabilities(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	groupTree := config.NewTree()
	groupCap := config.NewTree()
	groupCap.Set("route-refresh", "true")
	groupTree.SetContainer("capability", groupCap)

	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	peerCap := config.NewTree()
	peerCap.Set("extended-message", "enable")
	peerTree.SetContainer("capability", peerCap)
	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	capMap, ok := peer["capability"].(map[string]any)
	require.True(t, ok, "capability should be a map")
	assert.Equal(t, "true", capMap["route-refresh"], "group capability merged")
	assert.Equal(t, "enable", capMap["extended-message"], "peer capability merged")
}

// TestResolveBGPTree_BGPGlobalInheritance verifies bgp-level globals reach peers through groups.
//
// VALIDATES: AC-5 — bgp-level local-as flows to peers.
// PREVENTS: Group layer blocking bgp globals from reaching peers.
func TestResolveBGPTree_BGPGlobalInheritance(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")
	bgp.Set("router-id", "1.2.3.4")

	groupTree := config.NewTree()
	// Group does NOT set local-as — bgp global should flow through.
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	// Verify bgp globals are in the result (not in peer map — they're at top level).
	assert.Equal(t, "65000", result["local-as"])
	assert.Equal(t, "1.2.3.4", result["router-id"])

	// Peer should exist and have its own fields.
	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "65001", peer["peer-as"])
}

// TestResolveBGPTree_GroupOverridesBGPGlobal verifies group local-as overrides bgp global.
//
// VALIDATES: AC-6 — group-level local-as takes precedence over bgp-level.
// PREVENTS: BGP global values incorrectly winning when group explicitly sets them.
func TestResolveBGPTree_GroupOverridesBGPGlobal(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000") // BGP global.

	groupTree := config.NewTree()
	groupTree.Set("local-as", "65001") // Group overrides.
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65002")
	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)

	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "65001", peer["local-as"], "group local-as should override bgp global")
}

// TestResolveBGPTree_MultipleGroups verifies peers from different groups resolve independently.
//
// VALIDATES: Multiple groups with different defaults produce correct per-peer resolution.
// PREVENTS: Cross-contamination between groups.
func TestResolveBGPTree_MultipleGroups(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	// Group 1: fast peers.
	group1 := config.NewTree()
	group1.Set("hold-time", "30")
	peer1 := config.NewTree()
	peer1.Set("peer-as", "65001")
	group1.AddListEntry("peer", "10.0.0.1", peer1)
	bgp.AddListEntry("group", "fast-peers", group1)

	// Group 2: slow peers.
	group2 := config.NewTree()
	group2.Set("hold-time", "300")
	peer2 := config.NewTree()
	peer2.Set("peer-as", "65002")
	group2.AddListEntry("peer", "10.0.0.2", peer2)
	bgp.AddListEntry("group", "slow-peers", group2)

	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	p1 := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "30", p1["hold-time"], "fast-peers group hold-time")

	p2 := resolvedPeer(t, result, "10.0.0.2")
	assert.Equal(t, "300", p2["hold-time"], "slow-peers group hold-time")
}

// TestResolveBGPTree_DuplicatePeerName verifies error on duplicate peer names across groups.
//
// VALIDATES: AC-8 — duplicate peer names produce config validation error.
// PREVENTS: Two peers with the same name causing ambiguous CLI selection.
func TestResolveBGPTree_DuplicatePeerName(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	group1 := config.NewTree()
	peer1 := config.NewTree()
	peer1.Set("peer-as", "65001")
	peer1.Set("name", "router-east")
	group1.AddListEntry("peer", "10.0.0.1", peer1)
	bgp.AddListEntry("group", "group1", group1)

	group2 := config.NewTree()
	peer2 := config.NewTree()
	peer2.Set("peer-as", "65002")
	peer2.Set("name", "router-east") // Duplicate name.
	group2.AddListEntry("peer", "10.0.0.2", peer2)
	bgp.AddListEntry("group", "group2", group2)

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "router-east")
	assert.Contains(t, err.Error(), "duplicate")
}

// TestResolveBGPTree_EmptyGroup verifies an empty group (no peers) is valid.
//
// VALIDATES: AC-16 — empty groups parse without error.
// PREVENTS: Error on groups used for future peer additions.
func TestResolveBGPTree_EmptyGroup(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	groupTree := config.NewTree()
	groupTree.Set("hold-time", "180")
	// No peers added.
	bgp.AddListEntry("group", "empty-group", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)
	assert.Equal(t, "65000", result["local-as"])
}

// TestResolveBGPTree_PeerNameValidation verifies invalid peer names are rejected.
//
// VALIDATES: AC-14, AC-15 — names that look like IPs or contain invalid chars are rejected.
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
			name:     "contains_dots",
			peerName: "router.east",
			wantErr:  "invalid peer name",
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
			bgp.Set("local-as", "65000")

			groupTree := config.NewTree()
			peerTree := config.NewTree()
			peerTree.Set("peer-as", "65001")
			peerTree.Set("name", tt.peerName)
			groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
			bgp.AddListEntry("group", "test-group", groupTree)
			tree.SetContainer("bgp", bgp)

			_, err := ResolveBGPTree(tree)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestResolveBGPTree_ValidPeerNames verifies valid peer names are accepted.
//
// VALIDATES: AC-7 — valid names parse without error and appear in resolved map.
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
			bgp.Set("local-as", "65000")

			groupTree := config.NewTree()
			peerTree := config.NewTree()
			peerTree.Set("peer-as", "65001")
			peerTree.Set("name", tt.peerName)
			groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
			bgp.AddListEntry("group", "test-group", groupTree)
			tree.SetContainer("bgp", bgp)

			result, err := ResolveBGPTree(tree)
			require.NoError(t, err)

			peer := resolvedPeer(t, result, "10.0.0.1")
			assert.Equal(t, tt.peerName, peer["name"])
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

// TestResolveBGPTree_PeerNamePreserved verifies peer name is kept in the resolved map.
//
// VALIDATES: AC-7 — name field survives resolution.
// PREVENTS: Name being stripped during merge (like inherit was).
func TestResolveBGPTree_PeerNamePreserved(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("name", "google")
	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
	bgp.AddListEntry("group", "peering", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "google", peer["name"])
}

// TestResolveBGPTree_GroupNameInPeer verifies group name is stored in resolved peer map.
//
// VALIDATES: GroupName flows through resolution for PeerSettings.
// PREVENTS: Group membership info being lost during resolution.
func TestResolveBGPTree_GroupNameInPeer(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
	bgp.AddListEntry("group", "rr-clients", groupTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "rr-clients", peer["group-name"])
}

// TestResolveBGPTree_NoGroups verifies config with no groups returns valid map.
//
// VALIDATES: BGP block with no groups is valid (just globals).
// PREVENTS: Error when no groups are defined.
func TestResolveBGPTree_NoGroups(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)
	assert.Equal(t, "65000", result["local-as"])
}

// TestResolveBGPTree_StandalonePeer verifies peers directly under bgp work without groups.
//
// VALIDATES: AC-12 -- standalone peers (no group) parse correctly.
// PREVENTS: Regression where removing template support breaks standalone peers.
func TestResolveBGPTree_StandalonePeer(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")
	bgp.Set("router-id", "1.2.3.4")

	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("hold-time", "180")
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "65001", peer["peer-as"])
	assert.Equal(t, "180", peer["hold-time"])
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
	bgp.Set("local-as", "65000")

	// Group with one peer.
	groupTree := config.NewTree()
	groupTree.Set("hold-time", "180")
	groupPeer := config.NewTree()
	groupPeer.Set("peer-as", "65001")
	groupTree.AddListEntry("peer", "10.0.0.1", groupPeer)
	bgp.AddListEntry("group", "fast", groupTree)

	// Standalone peer.
	standalonePeer := config.NewTree()
	standalonePeer.Set("peer-as", "65002")
	standalonePeer.Set("hold-time", "90")
	bgp.AddListEntry("peer", "10.0.0.2", standalonePeer)

	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	// Grouped peer inherits group defaults.
	p1 := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "180", p1["hold-time"])
	assert.Equal(t, "fast", p1["group-name"])

	// Standalone peer uses its own values.
	p2 := resolvedPeer(t, result, "10.0.0.2")
	assert.Equal(t, "90", p2["hold-time"])
	_, hasGroupName := p2["group-name"]
	assert.False(t, hasGroupName)
}

// TestResolveBGPTree_DuplicatePeerIPAcrossGroups verifies error on same IP in two groups.
//
// VALIDATES: Duplicate peer IP across groups produces config validation error.
// PREVENTS: Two groups defining the same peer IP causing silent override.
func TestResolveBGPTree_DuplicatePeerIPAcrossGroups(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	group1 := config.NewTree()
	peer1 := config.NewTree()
	peer1.Set("peer-as", "65001")
	group1.AddListEntry("peer", "10.0.0.1", peer1)
	bgp.AddListEntry("group", "group1", group1)

	group2 := config.NewTree()
	peer2 := config.NewTree()
	peer2.Set("peer-as", "65002")
	group2.AddListEntry("peer", "10.0.0.1", peer2) // Same IP as group1.
	bgp.AddListEntry("group", "group2", group2)

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.0.0.1")
	assert.Contains(t, err.Error(), "duplicate")
}

// TestResolveBGPTree_DuplicatePeerIPGroupAndStandalone verifies error on same IP in group and standalone.
//
// VALIDATES: Duplicate peer IP between group and standalone produces error.
// PREVENTS: Group peer and standalone peer with same IP silently overwriting each other.
func TestResolveBGPTree_DuplicatePeerIPGroupAndStandalone(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	groupTree := config.NewTree()
	groupPeer := config.NewTree()
	groupPeer.Set("peer-as", "65001")
	groupTree.AddListEntry("peer", "10.0.0.1", groupPeer)
	bgp.AddListEntry("group", "grp", groupTree)

	standalonePeer := config.NewTree()
	standalonePeer.Set("peer-as", "65002")
	bgp.AddListEntry("peer", "10.0.0.1", standalonePeer) // Same IP as group peer.

	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.0.0.1")
	assert.Contains(t, err.Error(), "duplicate")
}

// TestResolveBGPTree_StandalonePeerWithName verifies peer name works on standalone peers.
//
// VALIDATES: AC-7 for standalone peers -- name is preserved.
// PREVENTS: Name validation only working for grouped peers.
func TestResolveBGPTree_StandalonePeerWithName(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("name", "google")
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "google", peer["name"])
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
			bgp.Set("local-as", "65000")

			peerTree := config.NewTree()
			peerTree.Set("peer-as", "65001")
			peerTree.Set("name", tt.peerName)
			bgp.AddListEntry("peer", "10.0.0.1", peerTree)
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
	}{
		{"hyphens_only", "---"},
		{"underscores_only", "___"},
		{"mixed_punctuation", "_-_-_"},
		{"single_hyphen", "-"},
		{"single_underscore", "_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			bgp := config.NewTree()
			bgp.Set("local-as", "65000")

			peerTree := config.NewTree()
			peerTree.Set("peer-as", "65001")
			peerTree.Set("name", tt.peerName)
			bgp.AddListEntry("peer", "10.0.0.1", peerTree)
			tree.SetContainer("bgp", bgp)

			_, err := ResolveBGPTree(tree)
			require.Error(t, err, "punctuation-only name %q should be rejected", tt.peerName)
			assert.Contains(t, err.Error(), "at least one letter or digit")
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
	bgp.Set("local-as", "65000")

	longName := strings.Repeat("a", maxPeerNameLen+1) // 256 chars

	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("name", longName)
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
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
	bgp.Set("local-as", "65000")

	exactName := strings.Repeat("x", maxPeerNameLen) // exactly 255 chars

	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("name", exactName)
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, exactName, peer["name"])
}

// TestResolveBGPTree_EmptyPeerNameIgnored verifies that `name ""` is silently ignored.
//
// VALIDATES: Empty peer name is treated as absent (no validation error).
// PREVENTS: False positive validation error on empty name string.
func TestResolveBGPTree_EmptyPeerNameIgnored(t *testing.T) {
	tree := config.NewTree()
	bgp := config.NewTree()
	bgp.Set("local-as", "65000")

	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("name", "") // Explicitly empty.
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
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
		{"contains_dots", "group.one", "invalid group name"},
		{"contains_spaces", "group one", "invalid group name"},
		{"contains_colon", "group:one", "invalid group name"},
		{"punctuation_only", "---", "at least one letter or digit"},
		{"unicode", "\u00e9quipe", "invalid group name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			bgp := config.NewTree()
			bgp.Set("local-as", "65000")

			groupTree := config.NewTree()
			peerTree := config.NewTree()
			peerTree.Set("peer-as", "65001")
			groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
			bgp.AddListEntry("group", tt.groupName, groupTree)
			tree.SetContainer("bgp", bgp)

			_, err := ResolveBGPTree(tree)
			require.Error(t, err, "group name %q should be rejected", tt.groupName)
			assert.Contains(t, err.Error(), tt.wantErr)
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
	bgp.Set("local-as", "65000")

	longName := strings.Repeat("g", maxPeerNameLen+1) // 256 chars

	groupTree := config.NewTree()
	peerTree := config.NewTree()
	peerTree.Set("peer-as", "65001")
	groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
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
			bgp.Set("local-as", "65000")

			groupTree := config.NewTree()
			peerTree := config.NewTree()
			peerTree.Set("peer-as", "65001")
			groupTree.AddListEntry("peer", "10.0.0.1", peerTree)
			bgp.AddListEntry("group", groupName, groupTree)
			tree.SetContainer("bgp", bgp)

			result, err := ResolveBGPTree(tree)
			require.NoError(t, err)

			peer := resolvedPeer(t, result, "10.0.0.1")
			assert.Equal(t, groupName, peer["group-name"])
		})
	}
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
	deepMergeMaps(dst, src)
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
	deepMergeMaps(dst, src)

	l1, ok := dst["level1"].(map[string]any)
	require.True(t, ok, "level1 should be a map")
	l2, ok := l1["level2"].(map[string]any)
	require.True(t, ok, "level2 should be a map")
	assert.Equal(t, "dst-val", l2["dst-key"], "dst key should be preserved")
	assert.Equal(t, "src-val", l2["src-key"], "src key should be merged")
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
		{"empty", "", true},
		{"dot", "group.one", true},
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
		{"wildcard", "*", true},
		{"ip_address", "10.0.0.1", true},
		{"ipv6_address", "2001:db8::1", true},
		{"dots", "router.east", true},
		{"spaces", "router east", true},
		{"comma", "router,east", true},
		{"colon", "router:east", true},
		{"unicode_accent", "\u00e9quipe", true},
		{"cjk", "\u8def\u7531\u5668", true},
		{"punctuation_only_hyphens", "---", true},
		{"punctuation_only_underscores", "___", true},
		{"too_long", strings.Repeat("a", 256), true},
		{"at_limit", strings.Repeat("a", 255), false},
		{"hyphen_prefix_with_alpha", "-router", false},
		{"reserved_list", "list", true},
		{"reserved_detail", "detail", true},
		{"reserved_add", "add", true},
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
