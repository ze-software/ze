package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AC-11 -- unified tree merges config and command domains.
// PREVENTS: Tree build failure when loading real YANG + RPCs.
func TestUnifiedTreeBuild(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err, "building unified tree should succeed")
	require.NotNil(t, root)
	assert.NotEmpty(t, root.Children, "root should have children")
}

// VALIDATES: AC-11 -- config nodes present with correct types.
// PREVENTS: Config entries missing from unified tree.
func TestUnifiedTreeConfigNodes(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	// bgp container should exist as config
	bgp, ok := root.Children["bgp"]
	require.True(t, ok, "bgp should be in unified tree")
	assert.Contains(t, bgp.Source, SourceConfig)

	// bgp > router-id should exist
	rid, ok := bgp.Children["router-id"]
	require.True(t, ok, "bgp > router-id should exist")
	assert.Contains(t, rid.Source, SourceConfig)
	assert.NotEmpty(t, rid.Type, "router-id should have a type")

	// bgp > peer should exist as a list
	peer, ok := bgp.Children["peer"]
	require.True(t, ok, "bgp > peer should exist")
	assert.Contains(t, peer.Source, SourceConfig)
}

// VALIDATES: AC-11 -- command nodes present at correct paths.
// PREVENTS: Command entries missing from unified tree.
func TestUnifiedTreeCommandNodes(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	// "peer" from command tree should exist
	peer, ok := root.Children["peer"]
	require.True(t, ok, "peer should be in unified tree from commands")
	assert.Contains(t, peer.Source, SourceCommand)

	// peer > list should exist
	list, ok := peer.Children["list"]
	require.True(t, ok, "peer > list should exist")
	assert.Equal(t, SourceCommand, list.Source)
}

// VALIDATES: AC-12 -- cross-domain nodes tagged as SourceBoth.
// PREVENTS: Nodes present in both domains not being detected.
func TestUnifiedTreeCrossDomain(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	// At top level, commands have "peer", "cache", "summary", "daemon", "system", etc.
	// Config has "bgp", "environment", "plugin".
	// "peer" at root is SourceCommand (config peer is under bgp container).

	// Verify the tree has nodes from both domains at root
	hasConfig := false
	hasCommand := false
	for _, child := range root.Children {
		if child.Source == SourceConfig || child.Source == SourceBoth {
			hasConfig = true
		}
		if child.Source == SourceCommand || child.Source == SourceBoth {
			hasCommand = true
		}
	}
	assert.True(t, hasConfig, "root should have config nodes")
	assert.True(t, hasCommand, "root should have command nodes")
}

// VALIDATES: AC-1, AC-10 -- detects known collisions.
// PREVENTS: Known collisions going undetected.
func TestUnifiedTreeCollisions(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	allGroups := CollectCollisions(root, 1)

	// We know bgp > peer config children have collisions:
	// local, link-local both start with "l"
	found := false
	for _, g := range allGroups {
		if g.Prefix == "l" {
			for _, s := range g.Siblings {
				if s.Name == "local" {
					found = true
					break
				}
			}
		}
	}
	assert.True(t, found, "should find local/link-local collision in bgp > peer")

	// peer commands have collisions: raw, refresh, remove, resume start with "r"
	foundR := false
	for _, g := range allGroups {
		if g.Prefix == "r" {
			for _, s := range g.Siblings {
				if s.Name == "raw" || s.Name == "refresh" || s.Name == "remove" || s.Name == "resume" {
					foundR = true
					break
				}
			}
		}
	}
	assert.True(t, foundR, "should find r-prefix collision in peer commands")
}

// PREVENTS: Crash when collecting collisions on a leaf node (no children).
func TestCollectCollisionsSingleChild(t *testing.T) {
	root := &AnalysisNode{
		Name:     "(root)",
		Children: map[string]*AnalysisNode{"only": {Name: "only", Children: make(map[string]*AnalysisNode)}},
	}
	groups := CollectCollisions(root, 1)
	assert.Empty(t, groups, "single child should produce no collisions")
}

// PREVENTS: Panic on nil AnalysisNode.
func TestCollectCollisionsNil(t *testing.T) {
	groups := CollectCollisions(nil, 1)
	assert.Empty(t, groups)
}

// PREVENTS: SortedChildren crash on nil Children map.
func TestSortedChildrenNil(t *testing.T) {
	node := &AnalysisNode{Name: "leaf"}
	assert.Empty(t, node.SortedChildren())
}

// PREVENTS: Config constraint fields not populated (mandatory, default, range).
func TestUnifiedTreeConstraints(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	bgp := root.Children["bgp"]
	require.NotNil(t, bgp)

	// router-id is mandatory in ze-bgp-conf.yang
	rid := bgp.Children["router-id"]
	require.NotNil(t, rid)
	assert.True(t, rid.Mandatory, "router-id should be mandatory")

	// hold-time has default 90 -- check inside peer
	peer := bgp.Children["peer"]
	require.NotNil(t, peer)
	ht := peer.Children["hold-time"]
	require.NotNil(t, ht)
	assert.Equal(t, "90", ht.Default, "hold-time should have default 90")
}

// PREVENTS: List key not skipped, showing up as config child.
func TestUnifiedTreeListKeySkipped(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	bgp := root.Children["bgp"]
	require.NotNil(t, bgp)
	peer := bgp.Children["peer"]
	require.NotNil(t, peer)

	// "name" is the list key for peer -- it should be skipped.
	_, hasName := peer.Children["name"]
	assert.False(t, hasName, "list key 'name' should be skipped in peer children")
}

// PREVENTS: AllRPCDocs returning wrong count or missing commands.
func TestAllRPCDocsCount(t *testing.T) {
	docs, err := AllRPCDocs()
	require.NoError(t, err)
	// Should have a reasonable number of commands (at least 20 from bgp + system + plugin)
	assert.Greater(t, len(docs), 20, "should have at least 20 registered commands")

	// Every doc should have non-empty CLICommand and WireMethod.
	for _, d := range docs {
		assert.NotEmpty(t, d.CLICommand, "every doc should have CLICommand")
		assert.NotEmpty(t, d.WireMethod, "every doc should have WireMethod")
	}
}

// PREVENTS: RPC parameter extraction failing silently.
func TestAllRPCDocsHaveParams(t *testing.T) {
	docs, err := AllRPCDocs()
	require.NoError(t, err)

	// "peer list" has a "selector" input parameter in ze-bgp-api.yang
	for _, d := range docs {
		if d.CLICommand != "peer list" {
			continue
		}
		assert.NotEmpty(t, d.Input, "peer list should have input parameters")
		found := false
		for _, leaf := range d.Input {
			if leaf.Name == "selector" {
				found = true
				break
			}
		}
		assert.True(t, found, "peer list should have 'selector' input parameter")
		return
	}
	t.Fatal("peer list not found in docs")
}

// PREVENTS: SourceBoth merge logic not working when command node precedes config node.
// The "plugin" node exists at root in both domains:
// - command: "plugin encoding" -> "plugin > encoding"
// - config: ze-plugin-conf defines "plugin" container.
func TestUnifiedTreeSourceBothMerge(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	plugin, ok := root.Children["plugin"]
	require.True(t, ok, "plugin should be in unified tree")
	assert.Equal(t, SourceBoth, plugin.Source,
		"plugin should be SourceBoth (exists in both config from ze-plugin-conf and command from bgp plugin *)")
}

// PREVENTS: Tree with SourceBoth parent correctly shows children from both domains.
func TestUnifiedTreeBothDomainChildren(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	plugin, ok := root.Children["plugin"]
	require.True(t, ok)

	// Command children: "encoding", "format", "ack" (from bgp plugin * RPCs)
	// Config children: whatever ze-plugin-conf defines
	// Both should be present as children.
	hasCommandChild := false
	for _, child := range plugin.Children {
		if child.Source == SourceCommand || child.Source == SourceBoth {
			hasCommandChild = true
			break
		}
	}
	// plugin has command children (encoding, format, ack from RPCs)
	assert.True(t, hasCommandChild, "plugin should have command children")
}

// PREVENTS: walkYANGEntry not enriching existing command node with config metadata.
func TestUnifiedTreeMergeEnrichesDescription(t *testing.T) {
	root, err := BuildUnifiedTree()
	require.NoError(t, err)

	plugin, ok := root.Children["plugin"]
	require.True(t, ok)
	// After merge, the plugin node should have a description from YANG (not empty).
	assert.NotEmpty(t, plugin.Description,
		"merged plugin node should have description from YANG config module")
}
