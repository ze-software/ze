// Design: docs/architecture/config/change-file-structural-ops.md -- owned entry creation.
// Related: editor_create_entry.go -- replay, conflict and discard boundaries.

package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestSessionCreationReviewCopyRenameReplay preserves every dependency of a
// create/copy/rename sequence when a materialized draft is replayed again.
func TestSessionCreationReviewCopyRenameReplay(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		name := "direct"
		if candidate {
			name = "candidate"
		}
		t.Run(name, func(t *testing.T) {
			ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
			require.NoError(t, ed.CreateEntry([]string{"interface", "dummy", "first"}))
			require.NoError(t, ed.CopyListEntry([]string{"interface"}, "dummy", "first", []string{"second"}))
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "first", []string{"third"}))
			require.NoError(t, ed.SaveDraft())
			require.NoError(t, ed.SaveDraft(), "replaying a saved sequence must preserve the copy's source dependency")
			tree := creationReviewCommit(t, ed, candidate)
			assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "first"}))
			assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "second"}))
			assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "third"}))
		})
	}
}

// TestSessionCreationReviewStaleAncestor refuses a rename whose committed source
// another session removed; creating a child does not authorize recreating it.
func TestSessionCreationReviewStaleAncestor(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		name := "direct"
		if candidate {
			name = "candidate"
		}
		t.Run(name, func(t *testing.T) {
			ed, _, configPath := newLoadSessionEditor(t, validBGPConfig+"\ninterface { dummy first { description original } }\n")
			other := creationReviewOther(t, ed)
			require.NoError(t, other.DeleteListEntry([]string{"interface"}, "dummy", "first"))
			deleted, err := other.CommitSession()
			require.NoError(t, err)
			require.Empty(t, deleted.Conflicts)
			require.NoError(t, ed.CreateEntry([]string{"interface", "dummy", "first", "unit", "zero"}))
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "first", []string{"second"}))
			var result *CommitResult
			if candidate {
				result, _, err = ed.CommitSessionCandidate(time.Now())
			} else {
				result, err = ed.CommitSession()
			}
			require.NoError(t, err)
			require.Len(t, result.Conflicts, 1, "implicit ancestor reconstruction must not hide the stale rename source")
			assert.Equal(t, ConflictStale, result.Conflicts[0].Type)
			tree := committedLoadTree(t, ed, configPath)
			assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "first"}))
			assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "second"}))
		})
	}
}

// TestSessionCreationReviewDiscardRetry retains ownership when shared-draft
// pruning fails, so a subsequent discard can finish instead of orphaning a node.
func TestSessionCreationReviewDiscardRetry(t *testing.T) {
	ed, session, configPath := newLoadSessionEditor(t, validBGPConfig)
	path := []string{"interface", "dummy", "first"}
	require.NoError(t, ed.CreateEntry(path))
	require.NoError(t, ed.SaveDraft())
	foreign := ChangePath(configPath, "alice")
	// A malformed foreign change file is a real, deterministic prune failure.
	require.NoError(t, ed.store.WriteFile(foreign, []byte("#alice @ssh %2026-10-10T01:02:03Z create-entry\n"), 0o600))
	require.Error(t, ed.DiscardSessionPath(path))
	assert.Len(t, ed.PendingChanges(session.ID), 1, "failed pruning must not consume the only creation record")
	require.NoError(t, ed.store.Remove(foreign))
	require.NoError(t, ed.DiscardSessionPath(path))
	assert.Empty(t, ed.PendingChanges(session.ID))
	assert.Nil(t, walkPath(ed.tree, ed.schema, path), "retry must remove the materialized creation")
}

// TestSessionCreationReviewCopiedDiscard removes both uncommitted entries and
// their ownership when the complete create/copy sequence is discarded.
func TestSessionCreationReviewCopiedDiscard(t *testing.T) {
	ed, session, _ := newLoadSessionEditor(t, validBGPConfig)
	require.NoError(t, ed.CreateEntry([]string{"interface", "dummy", "first"}))
	require.NoError(t, ed.CopyListEntry([]string{"interface"}, "dummy", "first", []string{"second"}))
	require.NoError(t, ed.SaveDraft())
	require.NoError(t, ed.DiscardSessionPath(nil))
	assert.Empty(t, ed.PendingChanges(session.ID))
	for _, name := range []string{"first", "second"} {
		assert.Nil(t, walkPath(ed.tree, ed.schema, []string{"interface", "dummy", name}), "discarded %s must not survive in the draft", name)
	}
}

// TestSessionCreationReviewRenameDestinationDiscard discards only the rename:
// the original creation remains owned, while its saved destination disappears.
func TestSessionCreationReviewRenameDestinationDiscard(t *testing.T) {
	ed, session, _ := newLoadSessionEditor(t, validBGPConfig)
	first := []string{"interface", "dummy", "first"}
	second := []string{"interface", "dummy", "second"}
	require.NoError(t, ed.CreateEntry(first))
	require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "first", []string{"second"}))
	require.NoError(t, ed.SaveDraft())
	require.NoError(t, ed.DiscardSessionPath(second))
	assert.Len(t, ed.PendingChanges(session.ID), 1, "the original creation remains pending")
	assert.NotNil(t, walkPath(ed.tree, ed.schema, first))
	assert.Nil(t, walkPath(ed.tree, ed.schema, second), "discarded rename destination must not survive")
	tree := creationReviewCommit(t, ed, false)
	assert.NotNil(t, walkPath(tree, ed.schema, first))
	assert.Nil(t, walkPath(tree, ed.schema, second))
}

// TestSessionCreationReviewQuotedKey preserves a string list key as one token
// through change-file parsing, saved discard and both commit doors.
func TestSessionCreationReviewQuotedKey(t *testing.T) {
	for _, action := range []string{"direct", "candidate", "discard"} {
		t.Run(action, func(t *testing.T) {
			ed, session, _ := newLoadSessionEditor(t, validBGPConfig)
			path := []string{"bgp", "group", "edge west"}
			require.NoError(t, ed.CreateEntry(path))
			require.NotNil(t, walkPath(ed.tree, ed.schema, path), "the quoted key is accepted by the entry API")
			require.NoError(t, ed.SaveDraft(), "change-file replay must preserve key token boundaries")
			if action == "discard" {
				require.NoError(t, ed.DiscardSessionPath(path))
				assert.Empty(t, ed.PendingChanges(session.ID))
				assert.Nil(t, walkPath(ed.tree, ed.schema, path))
				return
			}
			tree := creationReviewCommit(t, ed, action == "candidate")
			assert.NotNil(t, walkPath(tree, ed.schema, path))
			assert.Nil(t, walkPath(tree, ed.schema, []string{"bgp", "group", "edge"}))
		})
	}
}

// TestSessionCreationReviewCanonicalAddress records entry identity independently
// of explicit default key leaves or the order in which key pairs were typed.
func TestSessionCreationReviewCanonicalAddress(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   []string
		oldKey  string
		newPath []string
	}{
		{"explicit default", []string{"bfd", "single-hop-session", "fe80::b", "vrf", "default", "interface", "zbl0"}, "fe80::b interface zbl0", []string{"bfd", "single-hop-session", "fe80::c", "interface", "zbl0"}},
		{"key order", []string{"bfd", "single-hop-session", "fe80::b", "interface", "zbl0", "vrf", "blue"}, "fe80::b vrf blue interface zbl0", []string{"bfd", "single-hop-session", "fe80::c", "vrf", "blue", "interface", "zbl0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
			require.NoError(t, ed.CreateEntry(tc.input))
			require.NoError(t, ed.RenameListEntry([]string{"bfd"}, "single-hop-session", tc.oldKey, []string{"fe80::c"}))
			require.NoError(t, ed.SaveDraft())
			require.NoError(t, ed.SaveDraft(), "canonical rename and creation must identify the same source")
			tree := creationReviewCommit(t, ed, false)
			assert.Nil(t, walkPath(tree, ed.schema, tc.input))
			assert.NotNil(t, walkPath(tree, ed.schema, tc.newPath))
		})
	}
}

// TestSessionCreationReviewSiblingOwnership treats complete multi-key addresses
// as sibling entries, even when one address is a textual prefix of the other.
func TestSessionCreationReviewSiblingOwnership(t *testing.T) {
	for _, force := range []bool{false, true} {
		name := "normal"
		if force {
			name = "forced"
		}
		t.Run(name, func(t *testing.T) {
			ed, session, configPath := newLoadSessionEditor(t, validBGPConfig+"\nbfd { single-hop-session 203.0.113.9 {} }\n")
			other := creationReviewOther(t, ed)
			path := []string{"bfd", "single-hop-session", "203.0.113.9", "interface", "zbl0"}
			require.NoError(t, ed.CreateEntry(path))
			require.NoError(t, other.DeleteListEntry([]string{"bfd"}, "single-hop-session", "203.0.113.9"))
			var result *CommitResult
			var err error
			if force {
				result, err = other.CommitSessionForce()
			} else {
				result, err = other.CommitSession()
			}
			require.NoError(t, err)
			assert.Empty(t, result.Conflicts, "deleting a sibling does not conflict with creation")
			assert.Empty(t, result.Warnings)
			assert.Len(t, ed.PendingChanges(session.ID), 1, "a forced sibling deletion must not discard another entry's owner")
			assert.Nil(t, walkPath(committedLoadTree(t, ed, configPath), ed.schema, path), "the other session must not publish this creation")
			tree := creationReviewCommit(t, ed, false)
			assert.NotNil(t, walkPath(tree, ed.schema, path), "the creation remains independently committable")
			assert.Nil(t, walkPath(tree, ed.schema, []string{"bfd", "single-hop-session", "203.0.113.9"}))
		})
	}
}

func creationReviewOther(t *testing.T, ed *Editor) *Editor {
	t.Helper()
	other, err := NewEditorWithStorage(ed.store, ed.originalPath)
	require.NoError(t, err)
	t.Cleanup(func() { other.Close() }) //nolint:errcheck // Best effort cleanup.
	other.SetSession(NewEditSession("alice", "ssh"))
	return other
}

func creationReviewCommit(t *testing.T, ed *Editor, candidate bool) *config.Tree {
	t.Helper()
	if candidate {
		result, content, err := ed.CommitSessionCandidate(time.Now())
		require.NoError(t, err)
		require.Empty(t, result.Conflicts)
		tree, _, err := parseConfigWithFormat(content, ed.schema)
		require.NoError(t, err)
		return tree
	}
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	return committedLoadTree(t, ed, ed.originalPath)
}

// TestSessionCreationReviewUnresolvableOwner refuses to prune while another
// pending owner's address cannot be resolved, retaining intent for a retry.
func TestSessionCreationReviewUnresolvableOwner(t *testing.T) {
	ed, session, configPath := newLoadSessionEditor(t, validBGPConfig)
	path := []string{"interface", "dummy", "first"}
	require.NoError(t, ed.CreateEntry(path))
	require.NoError(t, ed.SaveDraft())
	foreign := ChangePath(configPath, "alice")
	require.NoError(t, ed.store.WriteFile(foreign, []byte("#alice @ssh %2026-10-10T01:02:03Z create-entry obsolete entry\n"), 0o600))
	conflicts, err := ed.detectConflicts()
	require.Error(t, err)
	assert.Empty(t, conflicts, "resolution failure is not a fabricated overlap")
	var status textbuf.Buffer
	ed.appendConflictStatus(&status)
	assert.Contains(t, status.String(), "conflict check failed:")
	assert.NotContains(t, status.String(), "conflict with ")
	_, err = ed.CommitSession()
	require.Error(t, err)
	_, err = ed.CommitSessionForce()
	require.Error(t, err, "force cannot override unknown ownership")
	_, _, err = ed.CommitSessionCandidate(time.Now())
	require.Error(t, err)
	assert.Nil(t, walkPath(committedLoadTree(t, ed, configPath), ed.schema, path))
	assert.Error(t, ed.DiscardSessionPath(path), "an unresolved owner is not proof that the entry is unowned")
	assert.Len(t, ed.PendingChanges(session.ID), 1, "resolution failure must preserve creation intent")
	assert.NotNil(t, walkPath(ed.tree, ed.schema, path))
	require.NoError(t, ed.store.Remove(foreign))
	require.NoError(t, ed.DiscardSessionPath(path))
	assert.Empty(t, ed.PendingChanges(session.ID))
	assert.Nil(t, walkPath(ed.tree, ed.schema, path))
}

// TestSessionCreationReviewQuotedMetadataOwner keeps an unrelated user's valid
// quoted-key leaf edit independent of creation, commit and saved discard.
func TestSessionCreationReviewQuotedMetadataOwner(t *testing.T) {
	for _, action := range []string{"direct", "force", "candidate", "candidate force", "discard"} {
		t.Run(action, func(t *testing.T) {
			ed, _, configPath := newLoadSessionEditor(t, validBGPConfig)
			other := creationReviewOther(t, ed)
			otherSession := other.session.ID
			group := []string{"bgp", "group", "edge west"}
			require.NoError(t, other.SetValue(group, "description", "other user's pending leaf"))
			path := []string{"interface", "dummy", "first"}
			require.NoError(t, ed.CreateEntry(path))
			var tree *config.Tree
			if action == "discard" {
				require.NoError(t, ed.SaveDraft())
				require.NoError(t, ed.DiscardSessionPath(path), "a valid quoted metadata owner must not make pruning inconclusive")
				assert.Nil(t, walkPath(ed.tree, ed.schema, path))
				tree = committedLoadTree(t, ed, configPath)
			} else {
				var result *CommitResult
				var content string
				var err error
				switch action {
				case "direct":
					result, err = ed.CommitSession()
				case "force":
					result, err = ed.CommitSessionForce()
				case "candidate":
					result, content, err = ed.CommitSessionCandidate(time.Now())
				case "candidate force":
					result, content, err = ed.CommitSessionCandidateForce(time.Now())
				}
				require.NoError(t, err, "valid quoted metadata must resolve without blocking an independent creation")
				require.Empty(t, result.Conflicts)
				require.Empty(t, result.Warnings)
				if content == "" {
					tree = committedLoadTree(t, ed, configPath)
				} else {
					tree, _, err = parseConfigWithFormat(content, ed.schema)
					require.NoError(t, err)
					assert.Nil(t, walkPath(committedLoadTree(t, ed, configPath), ed.schema, path))
				}
				assert.NotNil(t, walkPath(tree, ed.schema, path))
			}
			assert.Nil(t, walkPath(tree, ed.schema, group), "another user's quoted-key edit must not be published")
			assert.Len(t, other.PendingChanges(otherSession), 1, "another user's ownership must survive")
		})
	}
}

// TestSessionCreationReviewMetadataOnlyDiscard uses no creation operation:
// quoted discard prefixes must still remove ordinary scalar/member metadata.
func TestSessionCreationReviewMetadataOnlyDiscard(t *testing.T) {
	for _, member := range []bool{false, true} {
		name := "scalar"
		if member {
			name = "member"
		}
		t.Run(name, func(t *testing.T) {
			ed, session, _ := newLoadSessionEditor(t, validBGPConfig)
			group := []string{"bgp", "group", "edge#west"}
			if member {
				require.NoError(t, ed.SetValue([]string{"bgp", "group", "edge#west", "session", "community"}, "send", "standard"))
			} else {
				require.NoError(t, ed.SetValue(group, "description", "pending"))
			}
			require.Len(t, ed.PendingChanges(session.ID), 1)
			require.NoError(t, ed.DiscardSessionPath(group))
			assert.Empty(t, ed.PendingChanges(session.ID), "metadata-only discard must consume the addressed change")
			assert.Nil(t, walkPath(ed.tree, ed.schema, group), "discard must not replay the removed metadata")
		})
	}
}

// TestSessionCreationReviewQuotedStructuralPaths preserves valid existing-entry
// rename addresses when discarding or comparing them with another creation.
func TestSessionCreationReviewQuotedStructuralPaths(t *testing.T) {
	seed := strings.Replace(validBGPConfig, "bgp {", "bgp {\n group \"edge west\" { description original }\n", 1)
	for _, action := range []string{"discard", "coexistence"} {
		t.Run(action, func(t *testing.T) {
			ed, session, configPath := newLoadSessionEditor(t, seed)
			var other *Editor
			if action == "coexistence" {
				other = creationReviewOther(t, ed)
				require.NoError(t, other.CreateEntry([]string{"interface", "dummy", "first"}))
			}
			require.NoError(t, ed.RenameListEntry([]string{"bgp"}, "group", "edge west", []string{"edge east"}))
			if action == "discard" {
				require.NoError(t, ed.DiscardSessionPath([]string{"bgp"}), "valid quoted structural paths must resolve during discard")
				assert.Empty(t, ed.PendingChanges(session.ID))
				assert.NotNil(t, walkPath(ed.tree, ed.schema, []string{"bgp", "group", "edge west"}))
				assert.Nil(t, walkPath(ed.tree, ed.schema, []string{"bgp", "group", "edge east"}))
				return
			}
			result, err := ed.CommitSession()
			require.NoError(t, err)
			require.Empty(t, result.Conflicts)
			tree := committedLoadTree(t, ed, configPath)
			assert.Nil(t, walkPath(tree, ed.schema, []string{"bgp", "group", "edge west"}))
			assert.NotNil(t, walkPath(tree, ed.schema, []string{"bgp", "group", "edge east"}))
			assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "first"}))
			assert.Len(t, other.PendingChanges(other.session.ID), 1)
		})
	}
}

// TestSessionCreationReviewRebasedQuotedParent preserves leaf and member
// deactivation through a rename whose destination needs quoting.
func TestSessionCreationReviewRebasedQuotedParent(t *testing.T) {
	for _, member := range []bool{false, true} {
		for _, candidate := range []bool{false, true} {
			name := "leaf/direct"
			if member {
				name = "member/direct"
			}
			if candidate {
				name = strings.TrimSuffix(name, "direct") + "candidate"
			}
			t.Run(name, func(t *testing.T) {
				ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
				group := []string{"bgp", "group", "first"}
				require.NoError(t, ed.CreateEntry(group))
				if member {
					parent := []string{"bgp", "group", "first", "session", "community"}
					require.NoError(t, ed.SetValue(parent, "send", "standard"))
					require.NoError(t, ed.DeactivateLeafListValue(parent, "send", "standard"))
				} else {
					require.NoError(t, ed.SetValue(group, "description", "protected group"))
					require.NoError(t, ed.DeactivateLeaf(group, "description"))
				}
				require.NoError(t, ed.RenameListEntry([]string{"bgp"}, "group", "first", []string{"edge west"}))
				require.NoError(t, ed.SaveDraft())
				assertState := func(tree *config.Tree) {
					t.Helper()
					path := []string{"bgp", "group", "edge west"}
					if member {
						path = append(path, "session", "community")
					}
					entry := walkPath(tree, ed.schema, path)
					require.NotNil(t, entry)
					if member {
						assert.Equal(t, []config.MemberState{{Value: "standard", Inactive: true}}, entry.GetMultiValuesState("send"))
					} else {
						assert.True(t, entry.IsLeafInactive("description"), "rebased parent decoding must not silently skip deactivation")
						value, exists := entry.Get("description")
						assert.True(t, exists, "deactivation must preserve the configured leaf")
						assert.Equal(t, "protected group", value)
					}
				}
				assertState(ed.tree)
				assertState(creationReviewCommit(t, ed, candidate))
			})
		}
	}
}

// TestSessionCreationReviewSavedThenRenamed removes a previously materialized
// creation source, but keeps that source when another pending owner needs it.
func TestSessionCreationReviewSavedThenRenamed(t *testing.T) {
	for _, otherOwner := range []bool{false, true} {
		name := "unowned source"
		if otherOwner {
			name = "other owner"
		}
		t.Run(name, func(t *testing.T) {
			ed, session, _ := newLoadSessionEditor(t, validBGPConfig+"\ninterface { dummy stable {} }\n")
			first := []string{"interface", "dummy", "first"}
			second := []string{"interface", "dummy", "second"}
			require.NoError(t, ed.CreateEntry(first))
			require.NoError(t, ed.SaveDraft())
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "first", []string{"second"}))
			var other *Editor
			if otherOwner {
				// The rename was already recorded; the other session now owns
				// the old materialization independently of this creation.
				other = creationReviewOther(t, ed)
				require.NoError(t, other.SetValue(first, "description", "other owner"))
				require.NoError(t, other.SaveDraft())
			}
			require.NoError(t, ed.SaveDraft())
			assert.Equal(t, otherOwner, walkPath(ed.tree, ed.schema, first) != nil, "saved replay must remove only an unowned superseded source")
			assert.NotNil(t, walkPath(ed.tree, ed.schema, second))
			require.NoError(t, ed.DiscardSessionPath(nil))
			assert.Empty(t, ed.PendingChanges(session.ID))
			assert.Equal(t, otherOwner, walkPath(ed.tree, ed.schema, first) != nil)
			assert.Nil(t, walkPath(ed.tree, ed.schema, second))
			assert.NotNil(t, walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "stable"}), "committed unrelated entry must survive")
			if otherOwner {
				entry := walkPath(ed.tree, ed.schema, first)
				require.NotNil(t, entry)
				value, _ := entry.Get("description")
				assert.Equal(t, "other owner", value)
				assert.Len(t, other.PendingChanges(other.session.ID), 1)
			}
		})
	}
}

// TestSessionCreationReviewDeletedDescendantMoves keeps a created-then-deleted
// child absent when a committed ancestor is renamed or copied.
func TestSessionCreationReviewDeletedDescendantMoves(t *testing.T) {
	for _, action := range []string{"rename", "copy"} {
		for _, candidate := range []bool{false, true} {
			name := action + "/direct"
			if candidate {
				name = action + "/candidate"
			}
			t.Run(name, func(t *testing.T) {
				seed := validBGPConfig + "\ninterface { dummy first { description original; unit kept {} } }\n"
				ed, _, _ := newLoadSessionEditor(t, seed)
				require.NoError(t, ed.CreateEntry([]string{"interface", "dummy", "first", "unit", "zero"}))
				require.NoError(t, ed.DeleteListEntry([]string{"interface", "dummy", "first"}, "unit", "zero"))
				if action == "rename" {
					require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "first", []string{"second"}))
				} else {
					require.NoError(t, ed.CopyListEntry([]string{"interface"}, "dummy", "first", []string{"second"}))
				}
				require.NoError(t, ed.SaveDraft())
				require.NoError(t, ed.SaveDraft())
				tree := creationReviewCommit(t, ed, candidate)
				assert.Equal(t, action == "copy", walkPath(tree, ed.schema, []string{"interface", "dummy", "first"}) != nil)
				target := walkPath(tree, ed.schema, []string{"interface", "dummy", "second"})
				require.NotNil(t, target)
				value, _ := target.Get("description")
				assert.Equal(t, "original", value, "committed source content must survive the move/copy")
				assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "second", "unit", "kept"}))
				assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "second", "unit", "zero"}), "deleted descendant must not be resurrected")
				assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "first", "unit", "zero"}))
			})
		}
	}
}

// TestSessionCreationReviewImplicitAncestorDiscard prunes newly materialized
// list-entry ancestors while retaining committed and independently owned ones.
func TestSessionCreationReviewImplicitAncestorDiscard(t *testing.T) {
	for _, owner := range []string{"none", "committed", "pending"} {
		t.Run(owner, func(t *testing.T) {
			seed := validBGPConfig
			if owner == "committed" {
				seed += "\ninterface { dummy first { description committed } }\n"
			}
			ed, session, _ := newLoadSessionEditor(t, seed)
			parent := []string{"interface", "dummy", "first"}
			child := []string{"interface", "dummy", "first", "unit", "zero"}
			require.NoError(t, ed.CreateEntry(child))
			require.NoError(t, ed.SaveDraft())
			var other *Editor
			if owner == "pending" {
				other = creationReviewOther(t, ed)
				require.NoError(t, other.SetValue(parent, "description", "pending owner"))
				require.NoError(t, other.SaveDraft())
			}
			require.NoError(t, ed.DiscardSessionPath(nil))
			assert.Empty(t, ed.PendingChanges(session.ID))
			assert.Nil(t, walkPath(ed.tree, ed.schema, child))
			entry := walkPath(ed.tree, ed.schema, parent)
			if owner == "none" {
				assert.Nil(t, entry, "implicit list ancestor must not remain without an owner")
				return
			}
			require.NotNil(t, entry, "owned ancestor must survive")
			value, _ := entry.Get("description")
			if owner == "committed" {
				assert.Equal(t, "committed", value)
			} else {
				assert.Equal(t, "pending owner", value)
				assert.Len(t, other.PendingChanges(other.session.ID), 1)
			}
		})
	}
}

// TestSessionCreationReviewRepeatedSavedRename retains every materialized
// location and the creator's saved leaves across more than one saved rename.
func TestSessionCreationReviewRepeatedSavedRename(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		name := "direct"
		if candidate {
			name = "candidate"
		}
		t.Run(name, func(t *testing.T) {
			ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
			require.NoError(t, ed.CreateEntry([]string{"interface", "dummy", "first"}))
			require.NoError(t, ed.SetValue([]string{"interface", "dummy", "first"}, "description", "creator"))
			require.NoError(t, ed.SaveDraft())
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "first", []string{"second"}))
			require.NoError(t, ed.SaveDraft())
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "second", []string{"third"}))
			require.NoError(t, ed.SaveDraft())
			for _, old := range []string{"first", "second"} {
				assert.Nil(t, walkPath(ed.tree, ed.schema, []string{"interface", "dummy", old}), "superseded %s must not remain in the saved draft", old)
			}
			third := walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "third"})
			require.NotNil(t, third)
			value, _ := third.Get("description")
			assert.Equal(t, "creator", value)
			tree := creationReviewCommit(t, ed, candidate)
			for _, old := range []string{"first", "second"} {
				assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", old}))
			}
			third = walkPath(tree, ed.schema, []string{"interface", "dummy", "third"})
			require.NotNil(t, third)
			value, _ = third.Get("description")
			assert.Equal(t, "creator", value, "saved creator metadata must reach the final name")
		})
	}
}

// TestSessionCreationReviewSavedCopyPayload copies the creator's saved leaves,
// not merely the empty creation program, before moving the original entry.
func TestSessionCreationReviewSavedCopyPayload(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		name := "direct"
		if candidate {
			name = "candidate"
		}
		t.Run(name, func(t *testing.T) {
			ed, _, configPath := newLoadSessionEditor(t, validBGPConfig)
			first := []string{"interface", "dummy", "first"}
			require.NoError(t, ed.CreateEntry(first))
			require.NoError(t, ed.SetValue(first, "description", "creator"))
			require.NoError(t, ed.SaveDraft())
			other := creationReviewOther(t, ed)
			otherPath := []string{"interface", "dummy", "independent"}
			require.NoError(t, other.SetValue(otherPath, "description", "other owner"))
			require.NoError(t, other.SaveDraft())
			require.NoError(t, ed.CopyListEntry([]string{"interface"}, "dummy", "first", []string{"second"}))
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "first", []string{"third"}))
			require.NoError(t, ed.SaveDraft())
			require.NoError(t, ed.SaveDraft())
			assertPayload := func(tree *config.Tree) {
				t.Helper()
				assert.Nil(t, walkPath(tree, ed.schema, first))
				for _, entryName := range []string{"second", "third"} {
					entry := walkPath(tree, ed.schema, []string{"interface", "dummy", entryName})
					require.NotNil(t, entry)
					value, exists := entry.Get("description")
					assert.True(t, exists, "%s must retain the saved creator leaf", entryName)
					assert.Equal(t, "creator", value)
				}
			}
			assertPayload(ed.tree)
			tree := creationReviewCommit(t, ed, candidate)
			assertPayload(tree)
			assert.Nil(t, walkPath(tree, ed.schema, otherPath), "copy must not publish another owner's entry")
			assert.Len(t, other.PendingChanges(other.session.ID), 1)
			if candidate {
				assert.Nil(t, walkPath(committedLoadTree(t, ed, configPath), ed.schema, []string{"interface", "dummy", "third"}))
			}
		})
	}
}

// TestSessionCreationReviewDeletedCreationDiscard removes earlier saved
// materializations even when a later unsaved operation deleted the creation.
func TestSessionCreationReviewDeletedCreationDiscard(t *testing.T) {
	for _, nested := range []bool{false, true} {
		for _, discard := range []string{"all", "path"} {
			for _, owner := range []string{"none", "committed", "pending"} {
				name := "entry/" + discard + "/" + owner
				if nested {
					name = "nested/" + discard + "/" + owner
				}
				t.Run(name, func(t *testing.T) {
					seed := validBGPConfig
					if owner == "committed" {
						seed += "\ninterface { dummy first { description committed } }\n"
					}
					ed, session, configPath := newLoadSessionEditor(t, seed)
					root := []string{"interface", "dummy", "first"}
					path := root
					if nested {
						path = []string{"interface", "dummy", "first", "unit", "zero"}
					}
					require.NoError(t, ed.CreateEntry(path))
					require.NoError(t, ed.SaveDraft())
					require.NoError(t, ed.DeleteListEntry(path[:len(path)-2], path[len(path)-2], path[len(path)-1]))
					assert.Nil(t, walkPath(ed.tree, ed.schema, path), "the unsaved delete must be visible before discard")
					var other *Editor
					if owner == "pending" {
						other = creationReviewOther(t, ed)
						require.NoError(t, other.SetValue(root, "description", "pending owner"))
						require.NoError(t, other.SaveDraft())
					}
					var discardPath []string
					if discard == "path" {
						discardPath = path
					}
					require.NoError(t, ed.DiscardSessionPath(discardPath))
					assert.Empty(t, ed.PendingChanges(session.ID))
					if nested || owner == "none" {
						assert.Nil(t, walkPath(ed.tree, ed.schema, path), "discard must not resurrect an unowned saved creation")
					}
					rootEntry := walkPath(ed.tree, ed.schema, root)
					if owner == "none" {
						assert.Nil(t, rootEntry, "an unowned implicit ancestor must not remain")
					} else {
						require.NotNil(t, rootEntry, "committed or independently owned materializations must survive")
						value, _ := rootEntry.Get("description")
						if owner == "committed" {
							assert.Equal(t, "committed", value)
						} else {
							assert.Equal(t, "pending owner", value)
							assert.Len(t, other.PendingChanges(other.session.ID), 1)
						}
					}
					assert.Equal(t, owner == "committed", walkPath(committedLoadTree(t, ed, configPath), ed.schema, root) != nil, "discard must not publish pending entries")
				})
			}
		}
	}
}

// TestSessionCreationReviewMixedRenameChain keeps existing-entry rename chains
// replayable when the same change program also owns a new entry.
func TestSessionCreationReviewMixedRenameChain(t *testing.T) {
	for _, creation := range []string{"unrelated", "descendant"} {
		for _, action := range []string{"direct", "candidate", "force", "candidate force"} {
			for _, saved := range []bool{false, true} {
				name := creation + "/" + action + "/commit"
				if saved {
					name = creation + "/" + action + "/repeated save"
				}
				t.Run(name, func(t *testing.T) {
					seed := validBGPConfig + "\ninterface { dummy stable { description original; unit kept {} } }\n"
					ed, _, configPath := newLoadSessionEditor(t, seed)
					created := []string{"interface", "dummy", "fresh"}
					if creation == "descendant" {
						created = []string{"interface", "dummy", "stable", "unit", "zero"}
					}
					require.NoError(t, ed.CreateEntry(created))
					require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "stable", []string{"middle"}))
					require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "middle", []string{"final"}))
					assertState := func(tree *config.Tree) {
						t.Helper()
						for _, old := range []string{"stable", "middle"} {
							assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", old}))
						}
						final := walkPath(tree, ed.schema, []string{"interface", "dummy", "final"})
						require.NotNil(t, final)
						value, exists := final.Get("description")
						assert.True(t, exists)
						assert.Equal(t, "original", value, "the committed source payload must survive the effective rename")
						assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "final", "unit", "kept"}))
						if creation == "descendant" {
							assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "final", "unit", "zero"}))
						} else {
							assert.NotNil(t, walkPath(tree, ed.schema, created))
						}
					}
					if saved {
						require.NoError(t, ed.SaveDraft())
						require.NoError(t, ed.SaveDraft(), "replay must not require a consumed intermediate source")
						assertState(ed.tree)
					}
					var result *CommitResult
					var content string
					var err error
					switch action {
					case "direct":
						result, err = ed.CommitSession()
					case "candidate":
						result, content, err = ed.CommitSessionCandidate(time.Now())
					case "force":
						result, err = ed.CommitSessionForce()
					case "candidate force":
						result, content, err = ed.CommitSessionCandidateForce(time.Now())
					}
					require.NoError(t, err)
					require.Empty(t, result.Conflicts, "an intermediate name is produced by the preceding rename, not missing committed state")
					require.Empty(t, result.Warnings)
					var tree *config.Tree
					if strings.HasPrefix(action, "candidate") {
						tree, _, err = parseConfigWithFormat(content, ed.schema)
						require.NoError(t, err)
						committed := committedLoadTree(t, ed, configPath)
						assert.NotNil(t, walkPath(committed, ed.schema, []string{"interface", "dummy", "stable"}))
						assert.Nil(t, walkPath(committed, ed.schema, []string{"interface", "dummy", "final"}))
					} else {
						tree = committedLoadTree(t, ed, configPath)
					}
					assertState(tree)
				})
			}
		}
	}
}

// TestSessionCreationReviewMixedRenameChainStaleSource still validates the
// original committed source, not an intermediate node reconstructed by replay.
func TestSessionCreationReviewMixedRenameChainStaleSource(t *testing.T) {
	for _, creation := range []string{"unrelated", "descendant"} {
		for _, candidate := range []bool{false, true} {
			name := creation + "/direct"
			if candidate {
				name = creation + "/candidate"
			}
			t.Run(name, func(t *testing.T) {
				seed := validBGPConfig + "\ninterface { dummy stable { description original } }\n"
				ed, session, configPath := newLoadSessionEditor(t, seed)
				other := creationReviewOther(t, ed)
				require.NoError(t, other.DeleteListEntry([]string{"interface"}, "dummy", "stable"))
				deleted, err := other.CommitSession()
				require.NoError(t, err)
				require.Empty(t, deleted.Conflicts)
				created := []string{"interface", "dummy", "fresh"}
				if creation == "descendant" {
					created = []string{"interface", "dummy", "stable", "unit", "zero"}
				}
				require.NoError(t, ed.CreateEntry(created))
				require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "stable", []string{"middle"}))
				require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "middle", []string{"final"}))
				var result *CommitResult
				if candidate {
					result, _, err = ed.CommitSessionCandidate(time.Now())
				} else {
					result, err = ed.CommitSession()
				}
				require.NoError(t, err)
				require.Len(t, result.Conflicts, 1)
				assert.Equal(t, ConflictStale, result.Conflicts[0].Type)
				assert.Equal(t, "interface dummy stable", result.Conflicts[0].Path)
				assert.NotEmpty(t, ed.PendingChanges(session.ID), "refusal must retain the creation and rename intent")
				committed := committedLoadTree(t, ed, configPath)
				for _, absent := range []string{"stable", "middle", "final", "fresh"} {
					assert.Nil(t, walkPath(committed, ed.schema, []string{"interface", "dummy", absent}), "stale source must not be reconstructed")
				}
			})
		}
	}
}

// TestSessionCreationReviewIntermediateDraft distinguishes replay from a saved
// intermediate draft from conflict checking against the committed source.
func TestSessionCreationReviewIntermediateDraft(t *testing.T) {
	for _, creation := range []string{"unrelated", "descendant"} {
		for _, candidate := range []bool{false, true} {
			name := creation + "/direct"
			if candidate {
				name = creation + "/candidate"
			}
			t.Run(name, func(t *testing.T) {
				seed := validBGPConfig + "\ninterface { dummy stable { description original; unit kept {} } }\n"
				ed, _, configPath := newLoadSessionEditor(t, seed)
				created := []string{"interface", "dummy", "fresh"}
				if creation == "descendant" {
					created = []string{"interface", "dummy", "stable", "unit", "zero"}
				}
				require.NoError(t, ed.CreateEntry(created))
				require.NoError(t, ed.SetValue([]string{"interface", "dummy", "stable"}, "description", "saved creator"))
				require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "stable", []string{"middle"}))
				require.NoError(t, ed.SaveDraft())
				middle := walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "middle"})
				require.NotNil(t, middle)
				value, exists := middle.Get("description")
				require.True(t, exists)
				require.Equal(t, "saved creator", value)
				other := creationReviewOther(t, ed)
				otherPath := []string{"interface", "dummy", "independent"}
				require.NoError(t, other.SetValue(otherPath, "description", "other owner"))
				require.NoError(t, other.SaveDraft())
				require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "middle", []string{"final"}))
				assertState := func(tree *config.Tree) {
					t.Helper()
					for _, old := range []string{"stable", "middle"} {
						assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", old}))
					}
					final := walkPath(tree, ed.schema, []string{"interface", "dummy", "final"})
					require.NotNil(t, final)
					value, exists := final.Get("description")
					assert.True(t, exists)
					assert.Equal(t, "saved creator", value, "saved leaf ownership must follow the second rename")
					assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "final", "unit", "kept"}))
					if creation == "descendant" {
						assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "final", "unit", "zero"}))
					} else {
						assert.NotNil(t, walkPath(tree, ed.schema, created))
					}
				}
				require.NoError(t, ed.SaveDraft(), "the saved source is middle, while the committed source remains stable")
				assertState(ed.tree)
				require.NoError(t, ed.SaveDraft())
				assertState(ed.tree)
				independent := walkPath(ed.tree, ed.schema, otherPath)
				require.NotNil(t, independent, "replay must preserve another owner's materialized draft edit")
				value, exists = independent.Get("description")
				assert.True(t, exists)
				assert.Equal(t, "other owner", value)
				tree := creationReviewCommit(t, ed, candidate)
				assertState(tree)
				assert.Nil(t, walkPath(tree, ed.schema, otherPath), "commit must not publish another owner's edit")
				assert.Len(t, other.PendingChanges(other.session.ID), 1)
				if candidate {
					committed := committedLoadTree(t, ed, configPath)
					assert.NotNil(t, walkPath(committed, ed.schema, []string{"interface", "dummy", "stable"}))
					assert.Nil(t, walkPath(committed, ed.schema, []string{"interface", "dummy", "final"}))
				}
			})
		}
	}
}

// TestSessionCreationReviewIntermediateCopy preserves the snapshot read from
// an intermediate name, including effects that differ after the copy.
func TestSessionCreationReviewIntermediateCopy(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		name := "direct"
		if candidate {
			name = "candidate"
		}
		t.Run(name, func(t *testing.T) {
			seed := validBGPConfig + "\ninterface { dummy stable { description original; unit kept {} } }\n"
			ed, _, configPath := newLoadSessionEditor(t, seed)
			require.NoError(t, ed.CreateEntry([]string{"interface", "dummy", "fresh"}))
			require.NoError(t, ed.CreateEntry([]string{"interface", "dummy", "stable", "unit", "zero"}))
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "stable", []string{"middle"}))
			middle := []string{"interface", "dummy", "middle"}
			require.NoError(t, ed.SetValue(middle, "description", "copy time"))
			require.NoError(t, ed.CopyListEntry([]string{"interface"}, "dummy", "middle", []string{"copied"}))
			require.NoError(t, ed.SetValue(middle, "description", "after copy"))
			require.NoError(t, ed.DeleteListEntry(middle, "unit", "zero"))
			require.NoError(t, ed.CreateEntry([]string{"interface", "dummy", "middle", "unit", "late"}))
			require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "middle", []string{"final"}))
			assertState := func(tree *config.Tree) {
				t.Helper()
				for _, old := range []string{"stable", "middle"} {
					assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", old}))
				}
				for _, entryName := range []string{"copied", "final"} {
					path := []string{"interface", "dummy", entryName}
					entry := walkPath(tree, ed.schema, path)
					require.NotNil(t, entry)
					value, exists := entry.Get("description")
					assert.True(t, exists)
					expected := "after copy"
					if entryName == "copied" {
						expected = "copy time"
					}
					assert.Equal(t, expected, value)
					assert.NotNil(t, walkPath(tree, ed.schema, append(path, "unit", "kept")), "the committed source payload must survive")
					assert.Equal(t, entryName == "copied", walkPath(tree, ed.schema, append(path, "unit", "zero")) != nil, "a later source deletion must not alter the copy")
					assert.Equal(t, entryName == "final", walkPath(tree, ed.schema, append(path, "unit", "late")) != nil, "a later source creation must not enter the copy")
				}
				assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "fresh"}))
			}
			assertState(ed.tree)
			require.NoError(t, ed.SaveDraft())
			assertState(ed.tree)
			require.NoError(t, ed.SaveDraft())
			assertState(ed.tree)
			assertState(creationReviewCommit(t, ed, candidate))
			if candidate {
				committed := committedLoadTree(t, ed, configPath)
				assert.NotNil(t, walkPath(committed, ed.schema, []string{"interface", "dummy", "stable"}))
				assert.Nil(t, walkPath(committed, ed.schema, []string{"interface", "dummy", "copied"}))
				assert.Nil(t, walkPath(committed, ed.schema, []string{"interface", "dummy", "final"}))
			}
		})
	}
}

// TestSessionCreationReviewIdentityRename preserves descendant and leaf effects
// when the entry returns to its committed name, with or without an earlier save.
func TestSessionCreationReviewIdentityRename(t *testing.T) {
	for _, creation := range []string{"unrelated", "descendant"} {
		for _, saved := range []bool{false, true} {
			for _, candidate := range []bool{false, true} {
				name := creation + "/unsaved/direct"
				if saved {
					name = creation + "/saved/direct"
				}
				if candidate {
					name = strings.TrimSuffix(name, "direct") + "candidate"
				}
				t.Run(name, func(t *testing.T) {
					seed := validBGPConfig + "\ninterface { dummy stable { description original; unit kept {} } }\n"
					ed, _, configPath := newLoadSessionEditor(t, seed)
					created := []string{"interface", "dummy", "fresh"}
					if creation == "descendant" {
						created = []string{"interface", "dummy", "stable", "unit", "zero"}
					}
					require.NoError(t, ed.CreateEntry(created))
					require.NoError(t, ed.SetValue([]string{"interface", "dummy", "stable"}, "description", "creator"))
					require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "stable", []string{"middle"}))
					if saved {
						require.NoError(t, ed.SaveDraft())
					}
					require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "middle", []string{"stable"}))
					assertState := func(tree *config.Tree) {
						t.Helper()
						assert.Nil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "middle"}))
						entry := walkPath(tree, ed.schema, []string{"interface", "dummy", "stable"})
						require.NotNil(t, entry)
						value, exists := entry.Get("description")
						assert.True(t, exists)
						assert.Equal(t, "creator", value)
						assert.NotNil(t, walkPath(tree, ed.schema, []string{"interface", "dummy", "stable", "unit", "kept"}))
						assert.NotNil(t, walkPath(tree, ed.schema, created))
					}
					require.NoError(t, ed.SaveDraft(), "an identity rename must not conflict with its own source")
					assertState(ed.tree)
					require.NoError(t, ed.SaveDraft())
					assertState(ed.tree)
					assertState(creationReviewCommit(t, ed, candidate))
					if candidate {
						committed := committedLoadTree(t, ed, configPath)
						entry := walkPath(committed, ed.schema, []string{"interface", "dummy", "stable"})
						require.NotNil(t, entry)
						value, _ := entry.Get("description")
						assert.Equal(t, "original", value)
						assert.Nil(t, walkPath(committed, ed.schema, created))
					}
				})
			}
		}
	}
}

// TestSessionCreationReviewIdentityRenameStaleSource preserves the original
// source safeguard for a real move following an effect-free round trip.
func TestSessionCreationReviewIdentityRenameStaleSource(t *testing.T) {
	for _, creation := range []string{"unrelated", "descendant"} {
		for _, candidate := range []bool{false, true} {
			name := creation + "/direct"
			if candidate {
				name = creation + "/candidate"
			}
			t.Run(name, func(t *testing.T) {
				seed := validBGPConfig + "\ninterface { dummy stable { description original } }\n"
				ed, session, configPath := newLoadSessionEditor(t, seed)
				other := creationReviewOther(t, ed)
				require.NoError(t, other.DeleteListEntry([]string{"interface"}, "dummy", "stable"))
				deleted, err := other.CommitSession()
				require.NoError(t, err)
				require.Empty(t, deleted.Conflicts)
				created := []string{"interface", "dummy", "fresh"}
				if creation == "descendant" {
					created = []string{"interface", "dummy", "stable", "unit", "zero"}
				}
				require.NoError(t, ed.CreateEntry(created))
				require.NoError(t, ed.SetValue([]string{"interface", "dummy", "stable"}, "description", "creator"))
				require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "stable", []string{"middle"}))
				require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "middle", []string{"stable"}))
				require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "stable", []string{"final"}))
				var result *CommitResult
				if candidate {
					result, _, err = ed.CommitSessionCandidate(time.Now())
				} else {
					result, err = ed.CommitSession()
				}
				require.NoError(t, err)
				require.NotEmpty(t, result.Conflicts, "an identity chain must not authorize resurrection of a stale edited source")
				for _, conflict := range result.Conflicts {
					assert.Equal(t, ConflictStale, conflict.Type)
				}
				assert.NotEmpty(t, ed.PendingChanges(session.ID))
				committed := committedLoadTree(t, ed, configPath)
				for _, absent := range []string{"stable", "middle", "final", "fresh"} {
					assert.Nil(t, walkPath(committed, ed.schema, []string{"interface", "dummy", absent}))
				}
			})
		}
	}
}

// TestSessionCreationReviewCopiedMemberState keeps the copy-time member program
// independent of subsequent rebasing of the original entry.
func TestSessionCreationReviewCopiedMemberState(t *testing.T) {
	for _, saved := range []bool{false, true} {
		for _, candidate := range []bool{false, true} {
			name := "unsaved/direct"
			if saved {
				name = "saved/direct"
			}
			if candidate {
				name = strings.TrimSuffix(name, "direct") + "candidate"
			}
			t.Run(name, func(t *testing.T) {
				ed, _, configPath := newLoadSessionEditor(t, validBGPConfig)
				require.NoError(t, ed.CreateEntry([]string{"bgp", "group", "first"}))
				parent := []string{"bgp", "group", "first", "session", "community"}
				require.NoError(t, ed.SetValue(parent, "send", "standard"))
				require.NoError(t, ed.DeactivateLeafListValue(parent, "send", "standard"))
				if saved {
					require.NoError(t, ed.SaveDraft())
				}
				require.NoError(t, ed.CopyListEntry([]string{"bgp"}, "group", "first", []string{"second"}))
				require.NoError(t, ed.RenameListEntry([]string{"bgp"}, "group", "first", []string{"third"}))
				assertState := func(tree *config.Tree) {
					t.Helper()
					assert.Nil(t, walkPath(tree, ed.schema, []string{"bgp", "group", "first"}))
					for _, entryName := range []string{"second", "third"} {
						entry := walkPath(tree, ed.schema, []string{"bgp", "group", entryName, "session", "community"})
						require.NotNil(t, entry)
						assert.Equal(t, []config.MemberState{{Value: "standard", Inactive: true}}, entry.GetMultiValuesState("send"), "%s must retain the copy-time member state", entryName)
					}
				}
				assertState(ed.tree)
				require.NoError(t, ed.SaveDraft())
				assertState(ed.tree)
				require.NoError(t, ed.SaveDraft())
				assertState(ed.tree)
				assertState(creationReviewCommit(t, ed, candidate))
				if candidate {
					committed := committedLoadTree(t, ed, configPath)
					for _, absent := range []string{"first", "second", "third"} {
						assert.Nil(t, walkPath(committed, ed.schema, []string{"bgp", "group", absent}))
					}
				}
			})
		}
	}
}

// TestSessionCreationReviewReusedHistoricalSource preserves an independent
// owner's new entry at a name vacated by an earlier saved rename.
func TestSessionCreationReviewReusedHistoricalSource(t *testing.T) {
	seed := validBGPConfig + "\ninterface { dummy stable { description original } }\n"
	ed, _, _ := newLoadSessionEditor(t, seed)
	require.NoError(t, ed.CreateEntry([]string{"interface", "dummy", "fresh"}))
	require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "stable", []string{"middle"}))
	require.NoError(t, ed.SaveDraft())
	other := creationReviewOther(t, ed)
	require.True(t, other.LoadDraft(), "resume the shared draft as the real edit session does")
	stable := []string{"interface", "dummy", "stable"}
	require.Nil(t, walkPath(other.tree, other.schema, stable), "the saved rename must have vacated this name")
	require.NoError(t, other.CreateEntry(stable))
	require.NoError(t, other.SetValue(stable, "description", "other owner"))
	require.Len(t, other.PendingChanges(other.session.ID), 2, "record both the new entry and its leaf before saving")
	require.NoError(t, other.SaveDraft())
	require.NoError(t, ed.RenameListEntry([]string{"interface"}, "dummy", "middle", []string{"final"}))
	for range 2 {
		require.NoError(t, ed.SaveDraft(), "replay must not rewind through an independently reused source name")
		reused := walkPath(ed.tree, ed.schema, stable)
		require.NotNil(t, reused)
		value, exists := reused.Get("description")
		assert.True(t, exists)
		assert.Equal(t, "other owner", value)
		final := walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "final"})
		require.NotNil(t, final)
		value, exists = final.Get("description")
		assert.True(t, exists)
		assert.Equal(t, "original", value)
		assert.Nil(t, walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "middle"}))
		assert.NotNil(t, walkPath(ed.tree, ed.schema, []string{"interface", "dummy", "fresh"}))
		assert.Len(t, other.PendingChanges(other.session.ID), 2, "both independent creation and leaf ownership must survive")
	}
}
