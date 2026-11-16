// Design: docs/architecture/config/change-file-structural-ops.md -- activation waits only for a genuinely new target.
package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
)

func TestSessionNewFlagActivationWaitsForAssignment(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		for _, inactive := range []bool{false, true} {
			name := "direct/active"
			if candidate {
				name = "candidate/active"
			}
			if inactive {
				name += "/inactive"
			}
			t.Run(name, func(t *testing.T) {
				ed, _, path := newLoadSessionEditor(t, validBGPConfig)
				node, ok := ed.schema.LookupTokenPath([]string{"system", "conntrack", "accounting"}).(*config.LeafNode)
				require.True(t, ok)
				require.Equal(t, config.TypeEmpty, node.Type)
				require.Nil(t, ed.tree.GetContainer("system"), "the parent is created only by the pending assignment")
				before, err := ed.store.ReadFile(path)
				require.NoError(t, err)
				commands := []string{"set system conntrack accounting", "deactivate system conntrack accounting"}
				if !inactive {
					commands = append(commands, "activate system conntrack accounting")
				}
				for _, line := range commands {
					_, err := ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: line})
					require.NoError(t, err, line)
				}
				require.NoError(t, ed.SaveDraft())
				saved := ed.WalkPath([]string{"system", "conntrack"})
				require.NotNil(t, saved)
				assert.Equal(t, inactive, saved.IsLeafInactive("accounting"), "the first save must preserve activation")
				require.NoError(t, ed.SaveDraft())
				var tree *config.Tree
				if candidate {
					result, text, err := ed.CommitSessionCandidate(time.Now())
					require.NoError(t, err)
					require.Empty(t, result.Conflicts)
					tree, _, err = parseConfigWithFormat(text, ed.schema)
					require.NoError(t, err)
					unchanged, err := ed.store.ReadFile(path)
					require.NoError(t, err)
					assert.Equal(t, before, unchanged)
				} else {
					result, err := ed.CommitSession()
					require.NoError(t, err)
					require.Empty(t, result.Conflicts)
					tree = committedLoadTree(t, ed, path)
				}
				parent := walkPath(tree, ed.schema, []string{"system", "conntrack"})
				require.NotNil(t, parent)
				value, found := parent.Get("accounting")
				assert.True(t, found)
				assert.Equal(t, "true", value)
				assert.Equal(t, inactive, parent.IsLeafInactive("accounting"))
				assert.Contains(t, config.Serialize(tree, ed.schema), "router-id 1.2.3.4")
			})
		}
	}
}

func TestSessionActivationRefusesRemovedCommittedAncestor(t *testing.T) {
	for _, lost := range []string{"parent", "list ancestor"} {
		t.Run(lost, func(t *testing.T) {
			originalFirewall := setFlagFirewall
			foreignFirewall := setFlagFirewall
			if lost == "parent" {
				originalFirewall = strings.Replace(setFlagFirewall, "term allow { from { protocol tcp; destination-port 179; } }", "term allow { from { protocol tcp; destination-port 179; } then { accept; } }", 1)
				require.NotEqual(t, setFlagFirewall, originalFirewall)
			} else {
				foreignFirewall = strings.Replace(setFlagFirewall, "term allow { from { protocol tcp; destination-port 179; } }", "", 1)
				require.NotEqual(t, setFlagFirewall, foreignFirewall)
			}
			seed := validBGPConfig + "\n" + originalFirewall
			ed, session, path := newLoadSessionEditor(t, seed)
			require.True(t, ed.treeValid)
			_, err := config.NewParser(ed.schema).Parse(seed)
			require.NoError(t, err)
			for _, line := range []string{
				"set firewall table flags chain input term allow then counter",
				"deactivate firewall table flags chain input term allow then counter",
			} {
				_, err := ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: line})
				require.NoError(t, err, line)
			}
			require.NoError(t, ed.SaveDraft())
			changePath := ChangePath(path, session.User)
			pending, err := ed.store.ReadFile(changePath)
			require.NoError(t, err)
			draft, err := ed.store.ReadFile(DraftPath(path))
			require.NoError(t, err)
			foreign := validBGPConfig + "\n" + foreignFirewall
			_, err = config.NewParser(ed.schema).Parse(foreign)
			require.NoError(t, err, "the competing commit must be a real schema-valid tree")
			require.NoError(t, ed.store.WriteFile(path, []byte(foreign), 0o600))
			_, err = ed.CommitSession()
			require.ErrorContains(t, err, "path not found")
			after, readErr := ed.store.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, foreign, string(after), "activation cannot resurrect a removed committed ancestor")
			after, readErr = ed.store.ReadFile(changePath)
			require.NoError(t, readErr)
			assert.Equal(t, pending, after)
			after, readErr = ed.store.ReadFile(DraftPath(path))
			require.NoError(t, readErr)
			assert.Equal(t, draft, after, "a refusal retains the saved assignments")
		})
	}
}
