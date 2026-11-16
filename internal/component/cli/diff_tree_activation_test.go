// Design: docs/guide/config-editor.md -- pending views preserve activation state.

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
)

// TestTreeDiffWholeLeafActivationMatchesSerializer checks that state-only
// changes remain visible for every leaf kind the editor can deactivate.
func TestTreeDiffWholeLeafActivationMatchesSerializer(t *testing.T) {
	for _, tc := range []struct {
		name string
		node config.Node
		text string
	}{
		{"leaf", config.Leaf(config.TypeString), "item retained\n"},
		{"multi", &config.MultiLeafNode{Type: config.TypeString}, "item first second\n"},
		{"bracket", &config.BracketLeafListNode{Type: config.TypeString}, "item [ first second ]\n"},
		{"array", &config.ValueOrArrayNode{Type: config.TypeString}, "item [ first second ]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := config.NewSchema()
			schema.Define("item", tc.node)
			active := treeDiffWorking(t, schema, tc.text)
			inactive := treeDiffWorking(t, schema, "inactive: "+tc.text)
			for _, state := range []struct {
				name   string
				before string
				after  string
				marker diffMarker
			}{
				{"deactivate", active, inactive, diffModified},
				{"activate", inactive, active, diffModified},
				{"unchanged inactive", inactive, inactive, diffUnchanged},
				{"added inactive", "", inactive, diffAdded},
				{"removed inactive", inactive, "", diffRemoved},
			} {
				t.Run(state.name, func(t *testing.T) {
					lines, err := computeTreeAnnotatedDiff(state.before, state.after, schema)
					require.NoError(t, err)
					require.Len(t, lines, 1)
					want := state.after
					if want == "" {
						want = state.before
					}
					assert.Equal(t, strings.TrimSpace(want), lines[0].Text)
					assert.Equal(t, state.marker, lines[0].Marker)
					assertTreeDiffWorkingMapping(t, lines, state.after)
				})
			}
		})
	}
}

// TestTreeDiffContainerActivationMatchesSerializer keeps both inline and block
// container openings faithful to the candidate, including added/removed nodes.
func TestTreeDiffContainerActivationMatchesSerializer(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"inline", "item { first retained; }\n"},
		{"block", "item { first retained; second also; }\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := config.NewSchema()
			schema.Define("item", config.Container(
				config.Field("first", config.Leaf(config.TypeString)),
				config.Field("second", config.Leaf(config.TypeString)),
			))
			active := treeDiffWorking(t, schema, tc.text)
			inactive := treeDiffWorking(t, schema, "inactive: "+tc.text)
			for _, state := range []struct {
				name, before, after string
				marker              diffMarker
			}{
				{"deactivate", active, inactive, diffModified},
				{"activate", inactive, active, diffModified},
				{"unchanged inactive", inactive, inactive, diffUnchanged},
				{"added inactive", "", inactive, diffAdded},
				{"removed inactive", inactive, "", diffRemoved},
			} {
				t.Run(state.name, func(t *testing.T) {
					lines, err := computeTreeAnnotatedDiff(state.before, state.after, schema)
					require.NoError(t, err)
					require.NotEmpty(t, lines)
					want := state.after
					if want == "" {
						want = state.before
					}
					assert.Equal(t, strings.Split(strings.TrimSpace(want), "\n")[0], lines[0].Text)
					assert.Equal(t, state.marker, lines[0].Marker)
					assertTreeDiffWorkingMapping(t, lines, state.after)
				})
			}
		})
	}
}

// TestSessionChangesDisplaysInactiveCandidateUntilCommit follows actual editor
// commands, proving state is staged correctly before the display layer sees it.
func TestSessionChangesDisplaysInactiveCandidateUntilCommit(t *testing.T) {
	ed, _, configPath := newLoadSessionEditor(t, validBGPConfig)
	before, err := ed.store.ReadFile(configPath)
	require.NoError(t, err)
	for _, line := range []string{"deactivate bgp router-id", "deactivate bgp peer peer1"} {
		_, err := ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: line})
		require.NoError(t, err)
	}
	bgp := ed.tree.GetContainer("bgp")
	require.True(t, bgp.IsLeafInactive("router-id"))
	require.True(t, bgp.GetList("peer")["peer1"].IsInactive())
	require.Contains(t, ed.DisplayContentAtPath(nil), "inactive: router-id 1.2.3.4")
	changes, err := ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: "show | changes"})
	require.NoError(t, err)
	assert.Contains(t, changes.Feedback, "2 pending changes")
	assert.Contains(t, changes.Output, "inactive: router-id 1.2.3.4")
	assert.Contains(t, changes.Output, "inactive: peer peer1")
	confirmed, err := ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: "show confirmed"})
	require.NoError(t, err)
	assert.NotContains(t, confirmed.Output, "inactive: router-id")
	assert.NotContains(t, confirmed.Output, "inactive: peer peer1")
	unchanged, err := ed.store.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, before, unchanged)
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	published, err := ed.store.ReadFile(configPath)
	require.NoError(t, err)
	assert.NotEqual(t, before, published)
	confirmed, err = ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: "show confirmed"})
	require.NoError(t, err)
	assert.Contains(t, confirmed.Output, "inactive: router-id 1.2.3.4")
	assert.Contains(t, confirmed.Output, "inactive: peer peer1")
}
