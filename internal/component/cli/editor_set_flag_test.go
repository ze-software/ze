// Design: docs/guide/config-editor.md -- schema flags use the shared set producer.
package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/firewall/yang" // Register the actual firewall schema exercised below.
)

const setFlagFirewall = `firewall {
    backend nft;
    table flags { family inet; chain input {
        type filter; hook input; priority 0; policy drop;
        term allow { from { protocol tcp; destination-port 179; } }
    } }
}`

func TestSessionSetSchemaFlags(t *testing.T) {
	for _, tc := range []struct {
		name, seed, command string
		parent              []string
		key                 string
	}{
		{"presence", setFlagFirewall, "set firewall table flags chain input term allow then accept", []string{"firewall", "table", "flags", "chain", "input", "term", "allow", "then"}, "accept"},
		{"empty", "", "set system conntrack accounting", []string{"system", "conntrack"}, "accounting"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seed := validBGPConfig + "\n" + tc.seed
			ed, _, path := newLoadSessionEditor(t, seed)
			require.True(t, ed.treeValid, "the test seed must parse before any command")
			_, err := config.NewParser(ed.schema).Parse(seed)
			require.NoError(t, err, "the actual registered schema must accept the complete seed")
			node := ed.schema.LookupTokenPath(append(append([]string{}, tc.parent...), tc.key))
			require.NotNil(t, node, "the test must reach the actual flag declaration")
			if tc.name == "presence" {
				flag, ok := node.(*config.ContainerNode)
				require.True(t, ok)
				require.True(t, flag.Presence)
			} else {
				flag, ok := node.(*config.LeafNode)
				require.True(t, ok)
				require.Equal(t, config.TypeEmpty, flag.Type)
			}
			before, err := ed.store.ReadFile(path)
			require.NoError(t, err)
			answer, err := ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: tc.command})
			require.NoError(t, err)
			assert.Contains(t, answer.Feedback, "set ")
			target := ed.WalkPath(tc.parent)
			require.NotNil(t, target)
			value, found := target.Get(tc.key)
			assert.True(t, found)
			assert.Equal(t, "true", value)
			require.NoError(t, ed.SaveDraft())
			require.NoError(t, ed.SaveDraft(), "the flag must survive sparse metadata replay")
			unchanged, err := ed.store.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, before, unchanged, "setting a flag changes only the candidate")
			result, err := ed.CommitSession()
			require.NoError(t, err)
			require.Empty(t, result.Conflicts)
			published := walkPath(committedLoadTree(t, ed, path), ed.schema, tc.parent)
			require.NotNil(t, published)
			value, found = published.Get(tc.key)
			assert.True(t, found)
			assert.Equal(t, "true", value)
			assert.Contains(t, config.Serialize(committedLoadTree(t, ed, path), ed.schema), "router-id 1.2.3.4")
		})
	}
}

func TestSessionSetFlagRefusalsKeepCandidate(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	require.True(t, ed.treeValid)
	presence, ok := ed.schema.LookupTokenPath([]string{"firewall", "table", "flags", "chain", "input", "term", "allow", "then", "accept"}).(*config.ContainerNode)
	require.True(t, ok, "an extra-value refusal must not be an unknown firewall schema refusal")
	require.True(t, presence.Presence)
	before := ed.WorkingContent()
	for _, line := range []string{
		"set system host", "set system conntrack accounting true",
		"set firewall table flags chain input term allow then accept true",
		"set system conntrack missing-flag",
	} {
		_, err := ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: line})
		require.Error(t, err, line)
		assert.Equal(t, before, ed.WorkingContent(), line)
	}
}

// TestSessionSetFlagInEditContext resolves a value-less setting before the
// ordinary command's minimum path/value arity is considered.
func TestSessionSetFlagInEditContext(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	answer, err := ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Path: []string{"system", "conntrack"}, Line: "set accounting"})
	require.NoError(t, err)
	assert.Contains(t, answer.Feedback, "accounting")
	value, found := ed.WalkPath([]string{"system", "conntrack"}).Get("accounting")
	assert.True(t, found)
	assert.Equal(t, "true", value)
}
