// Design: docs/architecture/api/commands.md -- hidden tree projections.
package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHiddenCommandOverlayRetainsAncestorVisibility merges a dynamic plugin
// below a hidden grouping and compares overlay and in-place discovery.
func TestHiddenCommandOverlayRetainsAncestorVisibility(t *testing.T) {
	base := &Node{Children: map[string]*Node{
		"public": {Name: "public", Description: "Public command."},
		"secret": {Name: "secret", Hidden: true, Children: make(map[string]*Node)},
	}}
	entries := []CommandEntry{{Name: "secret child", ShortHelp: "Secret child", Description: "Secret command."}}
	overlay := OverlayCommandPaths(base, entries)
	require.Empty(t, NewTreeCompleter(overlay).Complete("secret "))
	_, found := NewTreeCompleter(overlay).Explain("secret child")
	require.False(t, found)
	require.True(t, HiddenAtPath(overlay, []string{"secret", "child"}))
	require.True(t, HiddenAtPath(base, []string{"secret", "not-in-schema"}))
	require.False(t, HiddenAtPath(base, []string{"public", "dynamic"}))
	require.Empty(t, base.Children["secret"].Children, "overlay must leave the schema unchanged")
	MergeCommandPaths(base, entries)
	require.True(t, FindNode(base, []string{"secret", "child"}).Hidden)
	require.Empty(t, SubcommandNames(base.Children["secret"]))
	require.Equal(t, []string{"public"}, SubcommandNames(base))
}

// TestMergeYANGNodesRetainsHiddenMetadata exercises the projection used by
// standalone verb completion for existing RPC nodes and schema-only nodes.
func TestMergeYANGNodesRetainsHiddenMetadata(t *testing.T) {
	schema := &Node{Children: map[string]*Node{
		"existing": {Name: "existing", Hidden: true},
		"added":    {Name: "added", Hidden: true},
		"public":   {Name: "public"},
	}}
	tree := BuildTree([]RPCInfo{{CLICommand: "existing"}}, false)
	MergeYANGNodes(tree, schema)
	require.True(t, tree.Children["existing"].Hidden)
	require.True(t, tree.Children["added"].Hidden)
	require.Equal(t, []string{"public"}, SubcommandNames(tree))
	for _, suggestion := range NewTreeCompleter(tree).Complete("") {
		require.Equal(t, "public", suggestion.Text)
	}
}
