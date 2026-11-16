// Design: docs/architecture/config/change-file-structural-ops.md -- structural replay conflicts.
// Related: editor_create_entry.go -- committed source checks.

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestSessionCopiedEntryRenameCommit exercises the saved graphical action
// sequence through the shared editor, retaining the original committed source.
func TestSessionCopiedEntryRenameCommit(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		name := "direct"
		if candidate {
			name = "candidate"
		}
		t.Run(name, func(t *testing.T) {
			seed := validBGPConfig + "\ninterface { dummy source { description original; unit kept {} } }\n"
			ed, session, configPath := newLoadSessionEditor(t, seed)
			require.True(t, ed.treeValid)
			_, err := config.NewParser(ed.schema).Parse(seed)
			require.NoError(t, err, "the seed must parse strictly before exercising structural replay")
			source := walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "source"})
			require.NotNil(t, source)
			value, exists := source.Get("description")
			require.True(t, exists)
			require.Equal(t, "original", value)
			require.NotNil(t, walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "source", "unit", "kept"}))
			require.NoError(t, ed.CopyListEntry([]string{"interface"}, "dummy", "source", []string{"copied"}))
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "copied", []string{"final"}))
			require.NoError(t, ed.SaveDraft())
			require.NoError(t, ed.SaveDraft(), "a saved copy and rename must remain replayable")
			require.NoError(t, ed.DeactivatePath([]string{"interface", "dummy", "final"}))
			require.NoError(t, ed.ActivatePath([]string{"interface", "dummy", "final"}))
			data, err := ed.store.ReadFile(ChangePath(configPath, session.User))
			require.NoError(t, err)
			_, _, ops, err := config.ParseChangeFile(string(data), config.NewSetParser(ed.schema))
			require.NoError(t, err)
			require.Len(t, ops, 4)
			assert.Equal(t, config.StructuralOpCopyEntry, ops[0].Type)
			assert.Equal(t, "source", ops[0].OldKey)
			assert.Equal(t, "copied", ops[0].NewKey)
			assert.Equal(t, config.StructuralOpRename, ops[1].Type)
			assert.Equal(t, "copied", ops[1].OldKey)
			assert.Equal(t, "final", ops[1].NewKey)
			assert.Equal(t, config.StructuralOpDeactivatePath, ops[2].Type)
			assert.Equal(t, config.StructuralOpActivatePath, ops[3].Type)
			tree := creationReviewCommit(t, ed, candidate)
			assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "copied"}))
			for _, key := range []string{"source", "final"} {
				entry := walkPath(tree, ed.schema, []string{"interface", "dummy", key})
				require.NotNil(t, entry)
				value, exists := entry.Get("description")
				assert.True(t, exists)
				assert.Equal(t, "original", value)
				assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", key, "unit", "kept"}))
			}
			if candidate {
				assert.Nil(t, walkPath(committedLoadTree(t, ed, configPath), ed.schema, []string{"interface", "dummy", "final"}), "staging must not publish the candidate")
			}
		})
	}
}

// TestSessionCopiedEntryRenameConcurrentCommit refuses foreign committed
// collisions and a removed original source without consuming saved intent.
func TestSessionCopiedEntryRenameConcurrentCommit(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		for _, foreign := range []string{"source removed", "copied", "final"} {
			name := "direct/" + foreign
			if candidate {
				name = "candidate/" + foreign
			}
			t.Run(name, func(t *testing.T) {
				seed := validBGPConfig + "\ninterface { dummy source { description original; unit kept {} } }\n"
				ed, session, configPath := newLoadSessionEditor(t, seed)
				require.True(t, ed.treeValid)
				_, err := config.NewParser(ed.schema).Parse(seed)
				require.NoError(t, err, "the seed must parse strictly before exercising structural replay")
				source := walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "source"})
				require.NotNil(t, source)
				value, exists := source.Get("description")
				require.True(t, exists)
				require.Equal(t, "original", value)
				require.NotNil(t, walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "source", "unit", "kept"}))
				require.NoError(t, ed.CopyListEntry([]string{"interface"}, "dummy", "source", []string{"copied"}))
				require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "copied", []string{"final"}))
				require.NoError(t, ed.SaveDraft())
				changePath := ChangePath(configPath, session.User)
				before, err := ed.store.ReadFile(changePath)
				require.NoError(t, err)
				committed := validBGPConfig
				if foreign != "source removed" {
					committed += "\ninterface { dummy source { description original; } dummy " + foreign + " { description foreign; } }\n"
				}
				require.NoError(t, ed.store.WriteFile(configPath, []byte(committed), 0o600))
				var result *CommitResult
				var content string
				if candidate {
					result, content, err = ed.CommitSessionCandidate(time.Now())
				} else {
					result, err = ed.CommitSession()
				}
				switch foreign {
				case "final":
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Len(t, result.Conflicts, 1)
					assert.Equal(t, ConflictStale, result.Conflicts[0].Type)
					assert.Equal(t, "rename destination already exists", result.Conflicts[0].OtherValue)
					assert.Zero(t, result.Applied)
				case "copied":
					require.ErrorContains(t, err, "already exists")
				default:
					require.ErrorContains(t, err, "not found")
				}
				assert.Empty(t, content, "a refusal cannot stage a candidate")
				after, readErr := ed.store.ReadFile(configPath)
				require.NoError(t, readErr)
				assert.Equal(t, committed, string(after), "a refusal cannot overwrite foreign committed state")
				after, readErr = ed.store.ReadFile(changePath)
				require.NoError(t, readErr)
				assert.Equal(t, string(before), string(after), "a refusal must retain the complete structural program")
			})
		}
	}
}
