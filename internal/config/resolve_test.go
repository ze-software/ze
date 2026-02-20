package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolvedPeer extracts a peer's map from a ResolveBGPTree result, failing the test if missing.
func resolvedPeer(t *testing.T, result map[string]any, addr string) map[string]any { //nolint:unparam // addr varies by test scenario
	t.Helper()
	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok, "result[\"peer\"] should be a map")
	peer, ok := peerMap[addr].(map[string]any)
	require.True(t, ok, "peer %s should be a map", addr)
	return peer
}

// TestDeepMergeMaps verifies deep map merging for template resolution.
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

// TestResolveBGPTreeSimple verifies basic peer resolution without templates.
//
// VALIDATES: Peers are correctly extracted from bgp block into resolved map.
// PREVENTS: Resolution breaking when no templates exist.
func TestResolveBGPTreeSimple(t *testing.T) {
	// Build tree: bgp { local-as 65000; router-id 1.2.3.4; peer 10.0.0.1 { peer-as 65001; } }
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")
	bgp.Set("router-id", "1.2.3.4")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("hold-time", "180")

	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	// Check global values.
	assert.Equal(t, "65000", result["local-as"])
	assert.Equal(t, "1.2.3.4", result["router-id"])

	// Check peer.
	peerMap, ok := result["peer"].(map[string]any)
	require.True(t, ok, "peer should be a map")
	peer, ok := peerMap["10.0.0.1"].(map[string]any)
	require.True(t, ok, "peer 10.0.0.1 should be a map")
	assert.Equal(t, "65001", peer["peer-as"])
	assert.Equal(t, "180", peer["hold-time"])
}

// TestResolveBGPTreeTemplateInherit verifies template inheritance via 'inherit'.
//
// VALIDATES: Named template values are merged into the peer's resolved tree.
// PREVENTS: Template inheritance being lost during resolution.
func TestResolveBGPTreeTemplateInherit(t *testing.T) {
	// Build tree with template and peer that inherits it.
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")
	bgp.Set("router-id", "1.2.3.4")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("inherit", "my-template")

	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	// Template: template { group my-template { hold-time 300; capability { route-refresh true; } } }
	tmpl := NewTree()
	tmplGroup := NewTree()
	tmplGroup.Set("hold-time", "300")
	capTree := NewTree()
	capTree.Set("route-refresh", "true")
	tmplGroup.SetContainer("capability", capTree)
	tmpl.AddListEntry("group", "my-template", tmplGroup)
	tree.SetContainer("template", tmpl)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")

	// Peer should have template's hold-time.
	assert.Equal(t, "300", peer["hold-time"])
	// Peer should have template's capability.
	capMap, ok := peer["capability"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "true", capMap["route-refresh"])
	// Peer should still have its own peer-as.
	assert.Equal(t, "65001", peer["peer-as"])
	// inherit key should be removed from resolved map.
	_, hasInherit := peer["inherit"]
	assert.False(t, hasInherit, "inherit directive should be removed after resolution")
}

// TestResolveBGPTreePeerOverridesTemplate verifies peer values override template values.
//
// VALIDATES: Peer-level values take precedence over template values.
// PREVENTS: Template values incorrectly winning over peer's own configuration.
func TestResolveBGPTreePeerOverridesTemplate(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("hold-time", "90")
	peerTree.Set("inherit", "my-template")

	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	tmpl := NewTree()
	tmplGroup := NewTree()
	tmplGroup.Set("hold-time", "300")      // Template sets 300, peer sets 90 — peer wins.
	tmplGroup.Set("connection", "passive") // Only in template — should appear in resolved.
	tmpl.AddListEntry("group", "my-template", tmplGroup)
	tree.SetContainer("template", tmpl)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "90", peer["hold-time"], "peer's own hold-time should win")
	assert.Equal(t, "passive", peer["connection"], "template's passive should be inherited")
}

// TestResolveBGPTreeGlobMatch verifies auto-matching glob templates.
//
// VALIDATES: Glob patterns auto-apply to matching peers.
// PREVENTS: Glob templates being silently skipped.
func TestResolveBGPTreeGlobMatch(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	// Template: template { match 10.0.0.* { hold-time 300; } }
	tmpl := NewTree()
	matchTree := NewTree()
	matchTree.Set("hold-time", "300")
	tmpl.AddListEntry("match", "10.0.0.*", matchTree)
	tree.SetContainer("template", tmpl)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "300", peer["hold-time"], "glob template should apply")
}

// TestResolveBGPTreeGlobPlusPeerOverride verifies glob + peer precedence.
//
// VALIDATES: Peer values override glob-matched template values.
// PREVENTS: Glob templates incorrectly taking precedence over peer config.
func TestResolveBGPTreeGlobPlusPeerOverride(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("hold-time", "60") // Peer overrides glob.
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	tmpl := NewTree()
	matchTree := NewTree()
	matchTree.Set("hold-time", "300")
	matchTree.Set("connection", "passive") // Only in glob — should appear.
	tmpl.AddListEntry("match", "10.0.0.*", matchTree)
	tree.SetContainer("template", tmpl)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "60", peer["hold-time"], "peer value should override glob")
	assert.Equal(t, "passive", peer["connection"], "glob's passive should be inherited")
}

// TestResolveBGPTreeAllThreeLayers verifies glob → template → peer precedence.
//
// VALIDATES: Three-layer template resolution produces correct merged output.
// PREVENTS: Layer ordering bugs where wrong layer wins.
func TestResolveBGPTreeAllThreeLayers(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("hold-time", "45") // Layer 3: peer value wins for hold-time.
	peerTree.Set("inherit", "my-tmpl")
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	tmpl := NewTree()

	// Glob match: layer 1.
	matchTree := NewTree()
	matchTree.Set("hold-time", "300")      // Overridden by template, then peer.
	matchTree.Set("connection", "passive") // Not overridden by template or peer.
	matchTree.Set("description", "from-glob")
	tmpl.AddListEntry("match", "10.0.0.*", matchTree)

	// Named template: layer 2.
	groupTree := NewTree()
	groupTree.Set("description", "from-template") // Overrides glob's description.
	groupTree.Set("hold-time", "180")             // Overridden by peer.
	groupTree.Set("group-updates", "false")       // Not overridden by peer.
	tmpl.AddListEntry("group", "my-tmpl", groupTree)

	tree.SetContainer("template", tmpl)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "45", peer["hold-time"], "peer value should win (layer 3)")
	assert.Equal(t, "from-template", peer["description"], "template should override glob (layer 2)")
	assert.Equal(t, "passive", peer["connection"], "glob value should survive (layer 1)")
	assert.Equal(t, "false", peer["group-updates"], "template value should survive (layer 2)")
}

// TestResolveBGPTreeMissingBGP verifies error when bgp block is missing.
//
// VALIDATES: Clear error for missing bgp block.
// PREVENTS: Panic on nil bgp container.
func TestResolveBGPTreeMissingBGP(t *testing.T) {
	tree := NewTree()
	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bgp")
}

// TestResolveBGPTreeInheritNotFound verifies error for missing template.
//
// VALIDATES: Clear error when inherited template doesn't exist.
// PREVENTS: Silent failure when template name is misspelled.
func TestResolveBGPTreeInheritNotFound(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("inherit", "nonexistent")
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestResolveBGPTreeNoPeers verifies empty peer map is not an error.
//
// VALIDATES: Config with no peers returns valid map with no peer key.
// PREVENTS: Error on configs used for validation only (no peers).
func TestResolveBGPTreeNoPeers(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")
	tree.SetContainer("bgp", bgp)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)
	assert.Equal(t, "65000", result["local-as"])
}

// TestResolveBGPTreeNewSyntaxTemplate verifies template { bgp { peer <pattern> { ... } } }.
//
// VALIDATES: New-syntax templates with inherit-name are resolved correctly.
// PREVENTS: New syntax templates being ignored.
func TestResolveBGPTreeNewSyntaxTemplate(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("inherit", "fast-peer")
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	// New syntax: template { bgp { peer 10.0.0.* { inherit-name fast-peer; hold-time 30; } } }
	tmpl := NewTree()
	bgpTmpl := NewTree()
	peerTmplTree := NewTree()
	peerTmplTree.Set("inherit-name", "fast-peer")
	peerTmplTree.Set("hold-time", "30")
	bgpTmpl.AddListEntry("peer", "10.0.0.*", peerTmplTree)
	tmpl.SetContainer("bgp", bgpTmpl)
	tree.SetContainer("template", tmpl)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "30", peer["hold-time"])
	// inherit-name should NOT leak into resolved map (it's config metadata).
	_, hasInheritName := peer["inherit-name"]
	assert.False(t, hasInheritName)
}

// TestResolveBGPTreeNewSyntaxAutoGlob verifies template { bgp { peer <pattern> { ... } } }
// without inherit-name auto-applies as a glob.
//
// VALIDATES: Unnamed new-syntax templates auto-match peers like globs.
// PREVENTS: New syntax auto-matching being broken.
func TestResolveBGPTreeNewSyntaxAutoGlob(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")

	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	// template { bgp { peer 10.0.0.* { hold-time 300; } } } — no inherit-name, auto-glob.
	tmpl := NewTree()
	bgpTmpl := NewTree()
	globTree := NewTree()
	globTree.Set("hold-time", "300")
	bgpTmpl.AddListEntry("peer", "10.0.0.*", globTree)
	tmpl.SetContainer("bgp", bgpTmpl)
	tree.SetContainer("template", tmpl)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	assert.Equal(t, "300", peer["hold-time"])
}

// TestResolveBGPTreeDeepContainerMerge verifies containers are deep-merged across layers.
//
// VALIDATES: Capability containers from glob, template, and peer are deep-merged.
// PREVENTS: Container replacement instead of key-level merge.
func TestResolveBGPTreeDeepContainerMerge(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")

	// Peer has extended-message capability.
	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("inherit", "base")
	peerCap := NewTree()
	peerCap.Set("extended-message", "enable")
	peerTree.SetContainer("capability", peerCap)
	bgp.AddListEntry("peer", "10.0.0.1", peerTree)
	tree.SetContainer("bgp", bgp)

	// Template has route-refresh.
	tmpl := NewTree()
	groupTree := NewTree()
	tmplCap := NewTree()
	tmplCap.Set("route-refresh", "true")
	groupTree.SetContainer("capability", tmplCap)
	tmpl.AddListEntry("group", "base", groupTree)
	tree.SetContainer("template", tmpl)

	result, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peer := resolvedPeer(t, result, "10.0.0.1")
	capMap, ok := peer["capability"].(map[string]any)
	require.True(t, ok, "capability should be a map")
	assert.Equal(t, "true", capMap["route-refresh"], "template capability should be merged")
	assert.Equal(t, "enable", capMap["extended-message"], "peer capability should be merged")
}

// TestResolveBGPTreePatternValidation verifies inherit-name pattern check.
//
// VALIDATES: Peer address must match template pattern when pattern exists.
// PREVENTS: Peer inheriting from template intended for different address range.
func TestResolveBGPTreePatternValidation(t *testing.T) {
	tree := NewTree()
	bgp := NewTree()
	bgp.Set("local-as", "65000")

	// Peer 192.168.1.1 tries to inherit from template with pattern 10.0.0.*.
	peerTree := NewTree()
	peerTree.Set("peer-as", "65001")
	peerTree.Set("inherit", "ten-net")
	bgp.AddListEntry("peer", "192.168.1.1", peerTree)
	tree.SetContainer("bgp", bgp)

	tmpl := NewTree()
	bgpTmpl := NewTree()
	peerTmplTree := NewTree()
	peerTmplTree.Set("inherit-name", "ten-net")
	peerTmplTree.Set("hold-time", "30")
	bgpTmpl.AddListEntry("peer", "10.0.0.*", peerTmplTree)
	tmpl.SetContainer("bgp", bgpTmpl)
	tree.SetContainer("template", tmpl)

	_, err := ResolveBGPTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}
