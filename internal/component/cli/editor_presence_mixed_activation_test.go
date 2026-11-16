// Design: docs/guide/config-editor.md -- whole presence activation covers its value and block.
package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
)

// TestSessionPresenceMixedActivationSurvivesCommit exercises an actual YANG
// presence flag followed by a child edit, without publishing an active alias.
func TestSessionPresenceMixedActivationSurvivesCommit(t *testing.T) {
	seed := validBGPConfig + `
firewall { backend nft; table flags { family inet; chain postrouting {
        type nat; hook postrouting; priority 100; policy accept;
        term rewrite { then { counter; } }
    } } }`
	ed, session, path := newLoadSessionEditor(t, seed)
	require.True(t, ed.treeValid)
	_, err := config.NewParser(ed.schema).Parse(seed)
	require.NoError(t, err)
	parentPath := []string{"firewall", "table", "flags", "chain", "postrouting", "term", "rewrite", "then"}
	fullPath := append(append([]string{}, parentPath...), "masquerade")
	node, ok := ed.schema.LookupTokenPath(fullPath).(*config.ContainerNode)
	require.True(t, ok)
	require.True(t, node.Presence)
	require.NotNil(t, node.Get("port-range"))
	command := "firewall table flags chain postrouting term rewrite then masquerade"
	for _, line := range []string{"set " + command, "set " + command + " port-range 1024-65535", "deactivate " + command} {
		_, err := ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: line})
		require.NoError(t, err, line)
	}
	check := func(tree *config.Tree, inactive bool) {
		t.Helper()
		parent := walkPath(tree, ed.schema, parentPath)
		require.NotNil(t, parent)
		value, found := parent.Get("masquerade")
		assert.True(t, found)
		assert.Equal(t, "true", value)
		assert.Equal(t, inactive, parent.IsLeafInactive("masquerade"))
		block := parent.GetContainer("masquerade")
		require.NotNil(t, block)
		assert.Equal(t, inactive, block.IsInactive())
		value, found = block.Get("port-range")
		assert.True(t, found)
		assert.Equal(t, "1024-65535", value)
		active := tree.Clone()
		config.PruneInactive(active, ed.schema)
		activeParent := walkPath(active, ed.schema, parentPath)
		require.NotNil(t, activeParent)
		_, present := activeParent.Get("masquerade")
		assert.Equal(t, !inactive, present)
		assert.Equal(t, !inactive, activeParent.GetContainer("masquerade") != nil)
		if inactive {
			_, present = activeParent.ToPluginMap()["masquerade"]
			assert.False(t, present, "an inactive block cannot leave an active scalar alias for its consumer")
		}
	}
	check(ed.tree, true)
	require.NoError(t, ed.SaveDraft())
	require.NoError(t, ed.SaveDraft())
	check(ed.tree, true)
	sparse, err := ed.store.ReadFile(ChangePath(path, session.User))
	require.NoError(t, err)
	_, _, operations, err := config.ParseChangeFile(string(sparse), config.NewSetParser(ed.schema))
	require.NoError(t, err)
	require.Len(t, operations, 1, "save retains the whole-path activation intent")
	assert.Equal(t, config.StructuralOpDeactivatePath, operations[0].Type)
	draft, err := ed.store.ReadFile(DraftPath(path))
	require.NoError(t, err)
	_, ownership, err := config.NewSetParser(ed.schema).ParseWithMeta(string(draft))
	require.NoError(t, err)
	entries := ownership.SessionEntries(ed.schema, session.ID)
	require.Len(t, entries, 2, "the flag and child retain their own assignment records")
	before, err := ed.store.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(before), "masquerade")
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	check(committedLoadTree(t, ed, path), true)
	_, err = ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: "activate " + command})
	require.NoError(t, err)
	check(ed.tree, false)
	result, err = ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	check(committedLoadTree(t, ed, path), false)
	assert.Contains(t, config.Serialize(committedLoadTree(t, ed, path), ed.schema), "router-id 1.2.3.4")
}
