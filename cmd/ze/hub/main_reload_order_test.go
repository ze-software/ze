package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/configorder"
)

// twoTermChainTree builds a tree holding one firewall chain whose two terms are
// written out of alphabetical order, so a lowering that dropped the order would
// be caught by a reader that recovered one by sorting, as well as by a reader
// that refuses a list with none.
func twoTermChainTree() *zeconfig.Tree {
	drop := zeconfig.NewTree()
	drop.Set("action", "drop")
	accept := zeconfig.NewTree()
	accept.Set("action", "accept")

	chain := zeconfig.NewTree()
	chain.AddListEntry("term", "zz-drop", drop)
	chain.AddListEntry("term", "aa-accept", accept)

	firewall := zeconfig.NewTree()
	firewall.SetContainer("filter", chain)

	tree := zeconfig.NewTree()
	tree.SetContainer("firewall", firewall)
	return tree
}

func chainOf(t *testing.T, root map[string]any) map[string]any {
	t.Helper()
	firewall, ok := root["firewall"].(map[string]any)
	require.True(t, ok, "firewall root is %T, want a map", root["firewall"])
	chain, ok := firewall["filter"].(map[string]any)
	require.True(t, ok, "filter container is %T, want a map", firewall["filter"])
	return chain
}

// VALIDATES: lowerForPlugins gives the ConfigProvider the SAME plugin-facing
// lowering it returns, so the snapshot a failed reload replays into the plugin
// server still carries every list's entry order.
// PREVENTS: boot seeding the provider with ToMap while the coordinator's tree
// held ToPluginMap. The two maps only ever meet on the rollback path, where
// snapshotToLoadedTree turns the provider snapshot back into a config tree, so
// the divergence was invisible until a reload FAILED. configorder.Entries then
// refused every multi-entry list in the replayed config, and the recovery from
// a bad config failed with it.
func TestLowerForPluginsGivesTheProviderTheOrderARollbackReplays(t *testing.T) {
	cp := zeconfig.NewProvider()
	configTree := lowerForPlugins(twoTermChainTree(), cp)

	// The coordinator's own tree carries the order.
	terms, err := configorder.Entries(chainOf(t, configTree), "term", "name")
	require.NoError(t, err, "the map handed to the coordinator carries no entry order")
	require.Len(t, terms, 2)
	assert.Equal(t, "zz-drop", terms[0].Key)
	assert.Equal(t, "aa-accept", terms[1].Key)

	// So does the map a rollback replays into the plugin server.
	prior, err := snapshotProvider(cp)
	require.NoError(t, err)
	replayed, err := configorder.Entries(chainOf(t, snapshotToLoadedTree(prior)), "term", "name")
	require.NoError(t, err, "the map rollbackReload replays carries no entry order")
	require.Len(t, replayed, 2)
	assert.Equal(t, "zz-drop", replayed[0].Key)
	assert.Equal(t, "aa-accept", replayed[1].Key)
}
