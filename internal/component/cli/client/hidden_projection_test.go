// Design: docs/architecture/cli/command-completion.md -- attached command visibility.
package client

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// TestRuntimeTreeOmitsHiddenBuiltinRows builds the actual attached tree from
// command-list rows. A real builtin's path survives while discovery hides it.
func TestRuntimeTreeOmitsHiddenBuiltinRows(t *testing.T) {
	paths, err := WireToPath()
	require.NoError(t, err)
	path := paths["ze-plugin:system-version-software"]
	require.NotEmpty(t, path)
	type discoveryRow struct {
		Value       string `json:"value"`
		ShortHelp   string `json:"short-help"`
		Description string `json:"description"`
		Hidden      bool   `json:"hidden"`
	}
	rows := struct {
		Commands []discoveryRow `json:"commands"`
	}{Commands: []discoveryRow{
		{Value: path, Hidden: true, Description: "Hidden builtin explanation."},
		{Value: "show zz-visibility-public", ShortHelp: "Public dynamic answer"},
	}}
	encoded, err := json.Marshal(rows)
	require.NoError(t, err)
	tree, err := BuildRuntimeTreeFromDispatch(commandListDispatch(string(encoded)))
	require.NoError(t, err)
	node := command.FindNode(tree, strings.Fields(path))
	require.NotNil(t, node, "hidden discovery must not erase a typed execution path")
	completer := command.NewTreeCompleter(tree)
	_, found := completer.Explain(path)
	require.False(t, found)
	words := strings.Fields(path)
	for _, suggestion := range completer.Complete(strings.Join(words[:len(words)-1], " ") + " ") {
		require.NotEqual(t, words[len(words)-1], suggestion.Text)
	}
	public := completer.Complete("show zz-visibility-")
	require.Len(t, public, 1)
	require.Equal(t, "zz-visibility-public", public[0].Text)
}
