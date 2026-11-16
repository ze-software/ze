// Design: docs/guide/config-editor.md -- presence flags share activation and saved intent.
package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
)

func TestSessionSetPresenceFlagActivationSurvivesDraft(t *testing.T) {
	seed := validBGPConfig + "\n" + setFlagFirewall
	ed, _, path := newLoadSessionEditor(t, seed)
	require.True(t, ed.treeValid, "the test seed must parse before any command")
	_, err := config.NewParser(ed.schema).Parse(seed)
	require.NoError(t, err, "the actual registered schema must accept the complete seed")
	flagPath := []string{"firewall", "table", "flags", "chain", "input", "term", "allow", "then"}
	flag, ok := ed.schema.LookupTokenPath(append(append([]string{}, flagPath...), "accept")).(*config.ContainerNode)
	require.True(t, ok, "the command must reach the actual presence declaration")
	require.True(t, flag.Presence)
	command := "firewall table flags chain input term allow then accept"
	_, err = ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: "set " + command})
	require.NoError(t, err)
	_, err = ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: "deactivate " + command})
	require.NoError(t, err)
	assert.True(t, ed.WalkPath(flagPath).IsLeafInactive("accept"))
	require.NoError(t, ed.SaveDraft())
	require.NoError(t, ed.SaveDraft())
	assert.True(t, ed.WalkPath(flagPath).IsLeafInactive("accept"))
	assert.Contains(t, ed.WorkingContent(), "nop firewall table flags chain input term allow then accept\n")
	before, err := ed.store.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(before), "accept", "saving an inactive candidate cannot publish it")
	_, err = ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: "activate " + command})
	require.NoError(t, err)
	assert.False(t, ed.WalkPath(flagPath).IsLeafInactive("accept"))
	require.NoError(t, ed.SaveDraft())
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	published := walkPath(committedLoadTree(t, ed, path), ed.schema, flagPath)
	require.NotNil(t, published)
	value, found := published.Get("accept")
	assert.True(t, found)
	assert.Equal(t, "true", value)
	assert.False(t, published.IsLeafInactive("accept"))
}
