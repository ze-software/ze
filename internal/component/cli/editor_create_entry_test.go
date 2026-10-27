// Design: docs/guide/config-editor.md -- session edits survive commit.
// Related: editor_draft.go -- writeThroughCreate records empty list entries.

package cli

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestSessionEmptyEntryCommit drives both session commit doors with the exact
// leafless static route used by the IPsec showcase and a directly created dummy.
// The candidate tree must keep the entry, not just its nonempty sibling's leaves.
func TestSessionEmptyEntryCommit(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		for _, merge := range []bool{false, true} {
			name := "direct"
			if merge {
				name = "merge"
			}
			if candidate {
				name += "/candidate"
			} else {
				name += "/commit"
			}
			t.Run(name, func(t *testing.T) {
				ed, _, configPath := newLoadSessionEditor(t, validBGPConfig)
				path := []string{"interface", "dummy", "zetrk1"}
				if merge {
					loaded, err := ed.ParseLoad(`interface {
    xfrm vpn-xfrm { if-id 42; mtu 1400; }
}
static {
    table default {
        route 10.78.2.0/24 {
            next { interface vpn-xfrm {} }
        }
    }
}`, nil)
					if err != nil {
						t.Fatal(err)
					}
					if err := ed.LoadMerge(nil, loaded); err != nil {
						t.Fatal(err)
					}
					path = []string{"static", "table", "default", "route", "10.78.2.0/24", "next", "interface", "vpn-xfrm"}
				} else if err := ed.CreateEntry(path); err != nil {
					t.Fatal(err)
				}
				if walkPath(ed.tree, ed.schema, path) == nil {
					t.Fatalf("entry %v was not created in the editing tree", path)
				}
				var tree *config.Tree
				if candidate {
					result, content, err := ed.CommitSessionCandidate(time.Now())
					if err != nil {
						t.Fatal(err)
					}
					if len(result.Conflicts) != 0 {
						t.Fatalf("commit conflicts: %v", result.Conflicts)
					}
					tree, _, err = parseConfigWithFormat(content, ed.schema)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					result, err := ed.CommitSession()
					if err != nil {
						t.Fatal(err)
					}
					if len(result.Conflicts) != 0 {
						t.Fatalf("commit conflicts: %v", result.Conflicts)
					}
					tree = committedLoadTree(t, ed, configPath)
				}
				if walkPath(tree, ed.schema, path) == nil {
					t.Fatalf("session commit lost leafless entry %v", path)
				}
			})
		}
	}
}

// TestSessionEmptyEntryOwnership keeps creation in its owner's change file:
// another user's commit cannot publish it, and discarding it removes the view
// even after SaveDraft materialized it.
func TestSessionEmptyEntryOwnership(t *testing.T) {
	for _, saved := range []bool{false, true} {
		name := "unsaved"
		if saved {
			name = "saved"
		}
		t.Run(name, func(t *testing.T) {
			ed, session, configPath := newLoadSessionEditor(t, validBGPConfig)
			path := []string{"interface", "dummy", "zetrk1"}
			if err := ed.CreateEntry(path); err != nil {
				t.Fatal(err)
			}
			if err := ed.CreateEntry(path); err != nil {
				t.Fatal(err)
			}
			if got := ed.PendingChanges(session.ID); len(got) != 1 {
				t.Fatalf("idempotent creation records %v, want one owned change", got)
			}
			if saved {
				if err := ed.SaveDraft(); err != nil {
					t.Fatal(err)
				}
			}
			other, err := NewEditorWithStorage(ed.store, configPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { other.Close() }) //nolint:errcheck // Best effort cleanup.
			other.SetSession(NewEditSession("alice", "ssh"))
			if err := other.SetValue([]string{"bgp"}, "router-id", "5.6.7.8"); err != nil {
				t.Fatal(err)
			}
			result, err := other.CommitSession()
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Conflicts) != 0 {
				t.Fatalf("unrelated commit conflicts: %v", result.Conflicts)
			}
			if walkPath(committedLoadTree(t, ed, configPath), ed.schema, path) != nil {
				t.Fatal("another session committed the pending empty entry")
			}
			if err := ed.DiscardSessionPath(path); err != nil {
				t.Fatal(err)
			}
			if got := ed.PendingChanges(session.ID); len(got) != 0 {
				t.Fatalf("discard left creation pending: %v", got)
			}
			if walkPath(ed.tree, ed.schema, path) != nil {
				t.Fatal("discard left an unowned empty entry in the editing tree")
			}
		})
	}
}

// TestSessionEmptyEntryCandidateKeepsIntent checks the failed-reload boundary:
// staging never publishes or consumes creation; rejecting the first candidate
// preserves intent for a second stage, and only acceptance consumes that intent.
func TestSessionEmptyEntryCandidateKeepsIntent(t *testing.T) {
	ed, session, configPath := newLoadSessionEditor(t, validBGPConfig)
	path := []string{"interface", "dummy", "zetrk1"}
	if err := ed.CreateEntry(path); err != nil {
		t.Fatal(err)
	}
	for attempt := range 2 {
		result, content, err := ed.CommitSessionCandidate(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Conflicts) != 0 {
			t.Fatalf("candidate conflicts: %v", result.Conflicts)
		}
		tree, _, err := parseConfigWithFormat(content, ed.schema)
		if err != nil {
			t.Fatal(err)
		}
		if walkPath(tree, ed.schema, path) == nil {
			t.Fatal("candidate lost the empty entry")
		}
		if walkPath(committedLoadTree(t, ed, configPath), ed.schema, path) != nil {
			t.Fatal("staging published the empty entry before reload acceptance")
		}
		if got := ed.PendingChanges(session.ID); len(got) != 1 {
			t.Fatalf("staging consumed creation intent: %v", got)
		}
		if attempt == 0 {
			// This is the failed-reload cleanup used by the commit command,
			// not a second stage over an outstanding candidate.
			if err := storage.ClearCandidate(ed.store, configPath); err != nil {
				t.Fatal(err)
			}
			if got := ed.PendingChanges(session.ID); len(got) != 1 {
				t.Fatalf("candidate rejection consumed creation intent: %v", got)
			}
			if walkPath(committedLoadTree(t, ed, configPath), ed.schema, path) != nil {
				t.Fatal("candidate rejection published the empty entry")
			}
			continue
		}
		if err := storage.PromoteCandidate(ed.store, configPath); err != nil {
			t.Fatal(err)
		}
		if err := ed.MarkCommittedContent(content); err != nil {
			t.Fatal(err)
		}
		if walkPath(committedLoadTree(t, ed, configPath), ed.schema, path) == nil {
			t.Fatal("accepted candidate lost the empty entry")
		}
		if got := ed.PendingChanges(session.ID); len(got) != 0 {
			t.Fatalf("accepted candidate left creation pending: %v", got)
		}
	}
}

// TestSessionEmptyEntryStructuralEdits preserves subsequent delete, rename
// and copy intent instead of resurrecting the original created entry.
func TestSessionEmptyEntryStructuralEdits(t *testing.T) {
	for _, action := range []string{"delete", "rename", "copy"} {
		t.Run(action, func(t *testing.T) {
			ed, _, configPath := newLoadSessionEditor(t, validBGPConfig)
			path := []string{"interface", "dummy", "first"}
			if err := ed.CreateEntry(path); err != nil {
				t.Fatal(err)
			}
			var err error
			switch action {
			case "delete":
				err = ed.DeleteListEntry([]string{"interface"}, "dummy", "first")
			case "rename":
				err = ed.RenameListEntry([]string{"interface"}, "dummy", "first", []string{"second"})
			case "copy":
				err = ed.CopyListEntry([]string{"interface"}, "dummy", "first", []string{"second"})
			}
			if err != nil {
				t.Fatal(err)
			}
			// Save twice: the second replay sees the structural result already
			// materialized, but MUST NOT recreate a renamed source.
			for range 2 {
				if err := ed.SaveDraft(); err != nil {
					t.Fatal(err)
				}
			}
			result, err := ed.CommitSession()
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Conflicts) != 0 {
				t.Fatalf("commit conflicts: %v", result.Conflicts)
			}
			tree := committedLoadTree(t, ed, configPath)
			first := walkPath(tree, ed.schema, path) != nil
			second := walkPath(tree, ed.schema, []string{"interface", "dummy", "second"}) != nil
			if first != (action == "copy") {
				t.Fatalf("%s leaves first=%v", action, first)
			}
			if second != (action != "delete") {
				t.Fatalf("%s leaves second=%v", action, second)
			}
		})
	}
}

// TestSessionEmptyEntrySavedDiscard removes the materialized entry even when no
// other commit happened to clean the shared draft first.
func TestSessionEmptyEntrySavedDiscard(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "path", true: "all"}[all], func(t *testing.T) {
			ed, session, _ := newLoadSessionEditor(t, validBGPConfig)
			path := []string{"interface", "dummy", "zetrk1"}
			if err := ed.CreateEntry(path); err != nil {
				t.Fatal(err)
			}
			if err := ed.SaveDraft(); err != nil {
				t.Fatal(err)
			}
			discard := path
			if all {
				discard = nil
			}
			if err := ed.DiscardSessionPath(discard); err != nil {
				t.Fatal(err)
			}
			if walkPath(ed.tree, ed.schema, path) != nil {
				t.Fatal("discard left the saved empty entry visible")
			}
			if got := ed.PendingChanges(session.ID); len(got) != 0 {
				t.Fatalf("discard left pending creation: %v", got)
			}
		})
	}
}

// TestSessionEmptyEntryConflicts protects creation from a concurrent ancestor
// deletion without making independent child leaf changes conflict.
func TestSessionEmptyEntryConflicts(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	create := config.PendingChange{Kind: config.PendingChangeSet, Path: "interface dummy first"}
	for _, tc := range []struct {
		name     string
		other    config.PendingChange
		conflict bool
	}{
		{"same creation", create, false},
		{"ancestor delete", config.PendingChange{Kind: config.PendingChangeDelete, Path: "interface"}, true},
		{"same delete", config.PendingChange{Kind: config.PendingChangeDelete, Path: create.Path}, true},
		{"sibling delete", config.PendingChange{Kind: config.PendingChangeDelete, Path: "interface dummy second"}, false},
		{"child set", config.PendingChange{Kind: config.PendingChangeSet, Path: create.Path + " description", Value: "other"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pendingChangesConflict(ed.schema, create, tc.other)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.conflict {
				t.Fatalf("forward conflict=%v, want %v", got, tc.conflict)
			}
			got, err = pendingChangesConflict(ed.schema, tc.other, create)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.conflict {
				t.Fatalf("reverse conflict=%v, want %v", got, tc.conflict)
			}
		})
	}
}

// TestSessionEmptyEntryMultiKey keeps each full entry address distinct through
// saved discard and commit, rather than treating a key leaf as an ordinary edit.
func TestSessionEmptyEntryMultiKey(t *testing.T) {
	ed, _, configPath := newLoadSessionEditor(t, validBGPConfig)
	first := []string{"bfd", "single-hop-session", "fe80::b", "interface", "zbl0"}
	second := []string{"bfd", "single-hop-session", "fe80::b", "interface", "zbl1"}
	for _, path := range [][]string{first, second} {
		if err := ed.CreateEntry(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	if err := ed.DiscardSessionPath(first); err != nil {
		t.Fatal(err)
	}
	result, err := ed.CommitSession()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conflicts) != 0 {
		t.Fatalf("commit conflicts: %v", result.Conflicts)
	}
	tree := committedLoadTree(t, ed, configPath)
	if walkPath(tree, ed.schema, first) != nil {
		t.Fatal("discarded multi-key entry was committed")
	}
	if walkPath(tree, ed.schema, second) == nil {
		t.Fatal("surviving multi-key entry was lost")
	}
}

// TestSessionEmptyEntryForcedDelete drops another user's creation ownership
// along with the saved entry; their next session view cannot resurrect it.
func TestSessionEmptyEntryForcedDelete(t *testing.T) {
	ed, session, configPath := newLoadSessionEditor(t, validBGPConfig+"\ninterface { dummy stable {} }\n")
	path := []string{"interface", "dummy", "zetrk1"}
	if err := ed.CreateEntry(path); err != nil {
		t.Fatal(err)
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	other, err := NewEditorWithStorage(ed.store, configPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { other.Close() }) //nolint:errcheck // Best effort cleanup.
	other.SetSession(NewEditSession("alice", "ssh"))
	if err := other.DeleteListEntry([]string{"interface"}, "dummy", "zetrk1"); err != nil {
		t.Fatal(err)
	}
	refused, err := other.CommitSession()
	if err != nil {
		t.Fatal(err)
	}
	if len(refused.Conflicts) != 1 || refused.Conflicts[0].Type != ConflictLive {
		t.Fatalf("delete did not report the other owner's creation: %v", refused.Conflicts)
	}
	result, err := other.CommitSessionForce()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conflicts) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("forced delete: %+v", result)
	}
	if got := ed.PendingChanges(session.ID); len(got) != 0 {
		t.Fatalf("forced delete left creation ownership: %v", got)
	}
	if err := ed.DiscardSessionPath(nil); err != nil {
		t.Fatal(err)
	}
	if walkPath(ed.tree, ed.schema, path) != nil {
		t.Fatal("forced delete left a materialized empty entry")
	}
	if walkPath(committedLoadTree(t, ed, configPath), ed.schema, path) != nil {
		t.Fatal("forced delete committed the discarded entry")
	}
}

// TestSessionEmptyEntryRenamedDiscard removes the saved destination rather
// than leaving an unowned entry after the original creation was renamed.
func TestSessionEmptyEntryRenamedDiscard(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	path := []string{"interface", "dummy", "first"}
	if err := ed.CreateEntry(path); err != nil {
		t.Fatal(err)
	}
	if err := ed.RenameListEntry([]string{"interface"}, "dummy", "first", []string{"second"}); err != nil {
		t.Fatal(err)
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	if err := ed.DiscardSessionPath(nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first", "second"} {
		if walkPath(ed.tree, ed.schema, []string{"interface", "dummy", name}) != nil {
			t.Fatalf("discard left the newly created entry %s", name)
		}
	}
}
