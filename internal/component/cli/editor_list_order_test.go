// Design: docs/guide/config-editor.md -- load, structural edits and rollback.
// VALIDATES: spec-config-list-entries-operator-order A-3, AC-7, AC-9 and AC-10.

package cli

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

const editorListOrderConfig = `interface {
	dummy zulu { description first; }
	dummy alpha { description second; }
	dummy mike { description third; }
}
`

// TestEditorLoadRecordsEntryOrder reads the recorded keys, not printed text, so
// a printer cannot hide a load that creates entries in map order. Both modes
// retain existing entry positions and append new entries in input order.
func TestEditorLoadRecordsEntryOrder(t *testing.T) {
	for _, session := range []bool{false, true} {
		for _, replace := range []bool{false, true} {
			for _, form := range []string{"hierarchical", "set", "relative"} {
				name := "file/merge/" + form
				if session {
					name = "session/merge/" + form
				}
				if replace {
					name += "/replace"
				}
				t.Run(name, func(t *testing.T) {
					ed, configPath := newListOrderEditor(t, session, `interface {
						dummy zulu { description original; }
						dummy obsolete { description retained-by-merge; }
					}`)
					body := `dummy mike { description third; }
						dummy alpha { description second; }
						dummy bravo { description fourth; }
						dummy zulu { description updated; }`
					input := "interface { " + body + " }"
					var path []string
					if form == "relative" {
						path = []string{"interface"}
						input = body
					}
					if form == "set" {
						input = "set interface dummy mike description third\n" +
							"set interface dummy alpha description second\n" +
							"set interface dummy bravo description fourth\n" +
							"set interface dummy zulu description updated\n"
					}
					loaded, err := ed.ParseLoad(input, path)
					if err != nil {
						t.Fatal(err)
					}
					if replace {
						err = ed.LoadReplace(path, loaded)
					} else {
						err = ed.LoadMerge(path, loaded)
					}
					if err != nil {
						t.Fatal(err)
					}
					want := []string{"zulu", "obsolete", "mike", "alpha", "bravo"}
					if replace {
						want = []string{"zulu", "mike", "alpha", "bravo"}
					}
					requireEditorListOrder(t, ed.tree, want)
					zulu := ed.tree.GetContainer("interface").GetList("dummy")["zulu"]
					if got, _ := zulu.Get("description"); got != "updated" {
						t.Fatalf("loaded description = %q, want updated", got)
					}
					commitListOrderEditor(t, ed)
					reopened := reopenListOrderEditor(t, ed, configPath, session)
					requireEditorListOrder(t, reopened.tree, want)
				})
			}
		}
	}
}

// TestSessionTypedEntriesCommitRecordedOrder creates enough entries that a
// coincidentally matching map iteration cannot credibly hide an unordered replay.
// The candidate must retain typed order, and a fresh editor must read that same
// order from the committed file rather than from the original editor's cache.
func TestSessionTypedEntriesCommitRecordedOrder(t *testing.T) {
	ed, path := newListOrderEditor(t, true, "interface { dummy existing { description baseline; } }")
	want := []string{"existing", "zulu", "alpha", "mike", "bravo", "yankee", "charlie", "xray", "delta", "whiskey", "echo"}
	for _, key := range want[1:] {
		if err := ed.SetValue([]string{"interface", "dummy", key}, "description", key); err != nil {
			t.Fatal(err)
		}
	}
	requireEditorListOrder(t, ed.tree, want)
	commitListOrderEditor(t, ed)
	reopened := reopenListOrderEditor(t, ed, path, true)
	requireEditorListOrder(t, reopened.tree, want)
	for _, key := range want[1:] {
		if got, _ := reopened.tree.GetContainer("interface").GetList("dummy")[key].Get("description"); got != key {
			t.Fatalf("description of %s = %q, want %q", key, got, key)
		}
	}
}

// TestSessionTypedEntriesReplayOrder covers explicit draft save and a partial
// discard's view rebuild, then the candidate-commit path used by the daemon.
func TestSessionTypedEntriesReplayOrder(t *testing.T) {
	for _, action := range []string{"save", "discard"} {
		t.Run(action, func(t *testing.T) {
			ed, _ := newListOrderEditor(t, true, "interface { dummy existing { description baseline; } }")
			want := []string{"existing", "zulu", "alpha", "mike", "bravo", "yankee", "charlie", "xray", "delta", "whiskey", "echo"}
			for _, key := range want[1:] {
				if err := ed.SetValue([]string{"interface", "dummy", key}, "description", key); err != nil {
					t.Fatal(err)
				}
			}
			if action == "save" {
				if err := ed.SaveDraft(); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := ed.SetValue([]string{"interface", "dummy", "existing"}, "description", "trial"); err != nil {
					t.Fatal(err)
				}
				if err := ed.DiscardSessionPath([]string{"interface", "dummy", "existing", "description"}); err != nil {
					t.Fatal(err)
				}
				if got, _ := ed.tree.GetContainer("interface").GetList("dummy")["existing"].Get("description"); got != "baseline" {
					t.Fatalf("discarded description = %q, want baseline", got)
				}
			}
			requireEditorListOrder(t, ed.tree, want)
			candidate := creationReviewCommit(t, ed, true)
			requireEditorListOrder(t, candidate, want)
		})
	}
}

// TestSessionCopyNestedEntriesKeepsOrder records pending child entries before a
// copy, then checks both source and snapshot through direct and candidate commit.
func TestSessionCopyNestedEntriesKeepsOrder(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		name := "direct"
		if candidate {
			name = "candidate"
		}
		t.Run(name, func(t *testing.T) {
			ed, _ := newListOrderEditor(t, true, editorListOrderConfig)
			want := []string{"9", "1", "7", "2", "8", "3", "6", "4", "5", "0"}
			for _, key := range want {
				if err := ed.SetValue([]string{"interface", "dummy", "zulu", "unit", key}, "description", key); err != nil {
					t.Fatal(err)
				}
			}
			if err := ed.CopyListEntry([]string{"interface"}, "dummy", "zulu", []string{"yankee"}); err != nil {
				t.Fatal(err)
			}
			for _, tree := range []*config.Tree{ed.tree, creationReviewCommit(t, ed, candidate)} {
				requireEditorListOrder(t, tree, []string{"zulu", "yankee", "alpha", "mike"})
				for _, parent := range []string{"zulu", "yankee"} {
					units := tree.GetContainer("interface").GetList("dummy")[parent].GetListOrdered("unit")
					got := make([]string, 0, len(units))
					for _, entry := range units {
						got = append(got, entry.Key)
						if value, _ := entry.Value.Get("description"); value != entry.Key {
							t.Fatalf("%s unit %s description = %q", parent, entry.Key, value)
						}
					}
					if !slices.Equal(got, want) {
						t.Fatalf("%s child order = %v, want %v", parent, got, want)
					}
				}
			}
		})
	}
}

// TestSessionCreatedEntryCopyKeepsOrder checks that replay of explicit creation
// operations retains copy adjacency, not only the immediately edited candidate.
func TestSessionCreatedEntryCopyKeepsOrder(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		name := "direct"
		if candidate {
			name = "candidate"
		}
		t.Run(name, func(t *testing.T) {
			ed, _ := newListOrderEditor(t, true, "interface {}")
			for _, key := range []string{"zulu", "alpha", "mike"} {
				path := []string{"interface", "dummy", key}
				if err := ed.CreateEntry(path); err != nil {
					t.Fatal(err)
				}
				if err := ed.SetValue(path, "description", key); err != nil {
					t.Fatal(err)
				}
			}
			if err := ed.CopyListEntry([]string{"interface"}, "dummy", "zulu", []string{"yankee"}); err != nil {
				t.Fatal(err)
			}
			want := []string{"zulu", "yankee", "alpha", "mike"}
			requireEditorListOrder(t, ed.tree, want)
			committed := creationReviewCommit(t, ed, candidate)
			requireEditorListOrder(t, committed, want)
			if got, _ := committed.GetContainer("interface").GetList("dummy")["yankee"].Get("description"); got != "zulu" {
				t.Fatalf("copied description = %q, want zulu", got)
			}
		})
	}
}

// TestSessionCreatedEntryRenameKeepsOrder checks that lowering a new entry's
// rename to its final creation address leaves that creation in the same slot.
func TestSessionCreatedEntryRenameKeepsOrder(t *testing.T) {
	for _, saved := range []bool{false, true} {
		name := "unsaved"
		if saved {
			name = "saved"
		}
		t.Run(name, func(t *testing.T) {
			ed, _ := newListOrderEditor(t, true, "interface {}")
			for _, key := range []string{"zulu", "alpha", "mike"} {
				path := []string{"interface", "dummy", key}
				if err := ed.CreateEntry(path); err != nil {
					t.Fatal(err)
				}
				if err := ed.SetValue(path, "description", key); err != nil {
					t.Fatal(err)
				}
			}
			if saved {
				if err := ed.SaveDraft(); err != nil {
					t.Fatal(err)
				}
			}
			if err := ed.RenameListEntry([]string{"interface"}, "dummy", "alpha", []string{"bravo"}); err != nil {
				t.Fatal(err)
			}
			want := []string{"zulu", "bravo", "mike"}
			requireEditorListOrder(t, ed.tree, want)
			committed := creationReviewCommit(t, ed, false)
			requireEditorListOrder(t, committed, want)
			if got, _ := committed.GetContainer("interface").GetList("dummy")["bravo"].Get("description"); got != "alpha" {
				t.Fatalf("renamed description = %q, want alpha", got)
			}
		})
	}
}

// TestSessionCreatedEntrySavedRenameDraftOrder checks the materialized draft
// itself: a later commit must not be the first operation to restore its order.
func TestSessionCreatedEntrySavedRenameDraftOrder(t *testing.T) {
	ed, _ := newListOrderEditor(t, true, "interface {}")
	for _, key := range []string{"zulu", "alpha", "mike"} {
		path := []string{"interface", "dummy", key}
		if err := ed.CreateEntry(path); err != nil {
			t.Fatal(err)
		}
		if err := ed.SetValue(path, "description", key); err != nil {
			t.Fatal(err)
		}
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	if err := ed.RenameListEntry([]string{"interface"}, "dummy", "alpha", []string{"bravo"}); err != nil {
		t.Fatal(err)
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	requireEditorListOrder(t, ed.tree, []string{"zulu", "bravo", "mike"})
}

// TestSessionCreatedEntrySavedCopyDraftOrder checks copy adjacency when replay
// starts from a draft that already materialized all of the original siblings.
func TestSessionCreatedEntrySavedCopyDraftOrder(t *testing.T) {
	ed, _ := newListOrderEditor(t, true, "interface {}")
	for _, key := range []string{"zulu", "alpha", "mike"} {
		path := []string{"interface", "dummy", key}
		if err := ed.CreateEntry(path); err != nil {
			t.Fatal(err)
		}
		if err := ed.SetValue(path, "description", key); err != nil {
			t.Fatal(err)
		}
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	if err := ed.CopyListEntry([]string{"interface"}, "dummy", "zulu", []string{"yankee"}); err != nil {
		t.Fatal(err)
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	requireEditorListOrder(t, ed.tree, []string{"zulu", "yankee", "alpha", "mike"})
}

// TestSessionCreatedEntrySavedMoveChainDraftOrder rebases placement destinations
// through later renames, preserving multiple copies' order and copy-time values.
func TestSessionCreatedEntrySavedMoveChainDraftOrder(t *testing.T) {
	ed, _ := newListOrderEditor(t, true, "interface {}")
	for _, key := range []string{"zulu", "alpha", "mike"} {
		path := []string{"interface", "dummy", key}
		if err := ed.CreateEntry(path); err != nil {
			t.Fatal(err)
		}
		if err := ed.SetValue(path, "description", key); err != nil {
			t.Fatal(err)
		}
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"yankee", "xray"} {
		if err := ed.CopyListEntry([]string{"interface"}, "dummy", "zulu", []string{key}); err != nil {
			t.Fatal(err)
		}
	}
	if err := ed.RenameListEntry([]string{"interface"}, "dummy", "yankee", []string{"bravo"}); err != nil {
		t.Fatal(err)
	}
	if err := ed.RenameListEntry([]string{"interface"}, "dummy", "zulu", []string{"november"}); err != nil {
		t.Fatal(err)
	}
	if err := ed.SetValue([]string{"interface", "dummy", "november"}, "description", "changed"); err != nil {
		t.Fatal(err)
	}
	want := []string{"november", "xray", "bravo", "alpha", "mike"}
	for range 2 {
		if err := ed.SaveDraft(); err != nil {
			t.Fatal(err)
		}
		requireEditorListOrder(t, ed.tree, want)
	}
	committed := creationReviewCommit(t, ed, false)
	requireEditorListOrder(t, committed, want)
	for _, key := range []string{"xray", "bravo"} {
		if got, _ := committed.GetContainer("interface").GetList("dummy")[key].Get("description"); got != "zulu" {
			t.Fatalf("%s copy-time description = %q, want zulu", key, got)
		}
	}
}

// TestSessionCreatedEntrySavedMovesPreserveOtherOwner keeps another user's
// independently saved entries, their relative order, values and ownership while
// positioning this session's copied and renamed creation destinations.
func TestSessionCreatedEntrySavedMovesPreserveOtherOwner(t *testing.T) {
	ed, _ := newListOrderEditor(t, true, "interface { dummy existing { description baseline; } }")
	for _, key := range []string{"zulu", "alpha", "mike"} {
		path := []string{"interface", "dummy", key}
		if err := ed.CreateEntry(path); err != nil {
			t.Fatal(err)
		}
		if err := ed.SetValue(path, "description", key); err != nil {
			t.Fatal(err)
		}
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	other := creationReviewOther(t, ed)
	for _, key := range []string{"existing", "foreign"} {
		if err := other.SetValue([]string{"interface", "dummy", key}, "description", "alice-owned"); err != nil {
			t.Fatal(err)
		}
	}
	if err := other.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	before := other.meta.SessionEntries(other.schema, other.session.ID)
	if len(before) != 2 {
		t.Fatalf("other owner's setup metadata = %v, want two entries", before)
	}
	if err := ed.CopyListEntry([]string{"interface"}, "dummy", "zulu", []string{"yankee"}); err != nil {
		t.Fatal(err)
	}
	if err := ed.RenameListEntry([]string{"interface"}, "dummy", "alpha", []string{"bravo"}); err != nil {
		t.Fatal(err)
	}
	if err := ed.SaveDraft(); err != nil {
		t.Fatal(err)
	}
	requireEditorListOrder(t, ed.tree, []string{"existing", "zulu", "yankee", "bravo", "mike", "foreign"})
	for _, key := range []string{"existing", "foreign"} {
		if got, _ := ed.tree.GetContainer("interface").GetList("dummy")[key].Get("description"); got != "alice-owned" {
			t.Fatalf("other owner's %s payload changed to %q", key, got)
		}
	}
	after := ed.meta.SessionEntries(ed.schema, other.session.ID)
	if len(after) != len(before) {
		t.Fatalf("other owner's metadata = %v, want %v", after, before)
	}
	for _, entry := range before {
		found := slices.ContainsFunc(after, func(candidate config.SessionEntry) bool {
			return slices.Equal(candidate.Path, entry.Path) && candidate.Entry == entry.Entry
		})
		if !found {
			t.Fatalf("other owner's metadata changed: missing %+v", entry)
		}
	}
}

// TestSessionCreatedEntryReusesDeletedDestination preserves chronological
// deletion barriers when a copy or rename reuses a previously deleted key.
// Empty entries ensure later leaf replay cannot disguise a lost creation.
func TestSessionCreatedEntryReusesDeletedDestination(t *testing.T) {
	for _, operation := range []string{"copy", "rename"} {
		for _, mode := range []string{"save", "direct", "candidate"} {
			for _, saved := range []bool{false, true} {
				name := operation + "/" + mode + "/unsaved"
				if saved {
					name = operation + "/" + mode + "/saved"
				}
				t.Run(name, func(t *testing.T) {
					ed, _ := newListOrderEditor(t, true, "interface {}")
					for _, key := range []string{"a", "b", "c"} {
						if err := ed.CreateEntry([]string{"interface", "dummy", key}); err != nil {
							t.Fatal(err)
						}
					}
					if saved {
						if err := ed.SaveDraft(); err != nil {
							t.Fatal(err)
						}
					}
					if err := ed.DeleteByPath([]string{"interface", "dummy", "b"}); err != nil {
						t.Fatal(err)
					}
					want := []string{"a", "b", "c"}
					if operation == "copy" {
						if err := ed.CopyListEntry([]string{"interface"}, "dummy", "a", []string{"b"}); err != nil {
							t.Fatal(err)
						}
					} else {
						if err := ed.RenameListEntry([]string{"interface"}, "dummy", "a", []string{"b"}); err != nil {
							t.Fatal(err)
						}
						want = []string{"b", "c"}
					}
					requireEditorListOrder(t, ed.tree, want)
					if mode == "save" {
						if err := ed.SaveDraft(); err != nil {
							t.Fatal(err)
						}
						requireEditorListOrder(t, ed.tree, want)
						return
					}
					committed := creationReviewCommit(t, ed, mode == "candidate")
					requireEditorListOrder(t, committed, want)
				})
			}
		}
	}
}

// TestEditorListOrderLifecycle checks copy adjacency and rename position before
// printing, then crosses commit storage, a fresh editor and a rollback archive.
// The in-memory assertions isolate producer order from serializer regressions.
func TestEditorListOrderLifecycle(t *testing.T) {
	for _, session := range []bool{false, true} {
		name := "file"
		if session {
			name = "session"
		}
		t.Run(name, func(t *testing.T) {
			ed, path := newListOrderEditor(t, session, editorListOrderConfig)
			requireEditorListOrder(t, ed.tree, []string{"zulu", "alpha", "mike"})
			if err := ed.CopyListEntry([]string{"interface"}, "dummy", "zulu", []string{"yankee"}); err != nil {
				t.Fatal(err)
			}
			requireEditorListOrder(t, ed.tree, []string{"zulu", "yankee", "alpha", "mike"})
			if err := ed.RenameListEntry([]string{"interface"}, "dummy", "alpha", []string{"bravo"}); err != nil {
				t.Fatal(err)
			}
			requireEditorListOrder(t, ed.tree, []string{"zulu", "yankee", "bravo", "mike"})
			commitListOrderEditor(t, ed)
			reopened := reopenListOrderEditor(t, ed, path, session)
			requireEditorListOrder(t, reopened.tree, []string{"zulu", "yankee", "bravo", "mike"})
			backups, err := reopened.ListBackups()
			if err != nil {
				t.Fatal(err)
			}
			if len(backups) == 0 {
				t.Fatal("commit did not record a rollback revision")
			}
			if err := reopened.Rollback(backups[0].Path); err != nil {
				t.Fatal(err)
			}
			requireEditorListOrder(t, reopened.tree, []string{"zulu", "alpha", "mike"})
			restored := reopenListOrderEditor(t, reopened, path, session)
			requireEditorListOrder(t, restored.tree, []string{"zulu", "alpha", "mike"})
		})
	}
}

// TestEditorReorderOnlyCommitRecordsOrder proves AC-7's storage half without
// consulting the diff. Removing and restoring identical content changes only
// the list's recorded order; commit and a fresh load must keep that order.
func TestEditorReorderOnlyCommitRecordsOrder(t *testing.T) {
	for _, session := range []bool{false, true} {
		name := "file"
		if session {
			name = "session"
		}
		t.Run(name, func(t *testing.T) {
			ed, path := newListOrderEditor(t, session, editorListOrderConfig)
			if err := ed.DeleteByPath([]string{"interface", "dummy", "zulu"}); err != nil {
				t.Fatal(err)
			}
			if err := ed.SetValue([]string{"interface", "dummy", "zulu"}, "description", "first"); err != nil {
				t.Fatal(err)
			}
			requireEditorListOrder(t, ed.tree, []string{"alpha", "mike", "zulu"})
			commitListOrderEditor(t, ed)
			reopened := reopenListOrderEditor(t, ed, path, session)
			requireEditorListOrder(t, reopened.tree, []string{"alpha", "mike", "zulu"})
			if got, _ := reopened.tree.GetContainer("interface").GetList("dummy")["zulu"].Get("description"); got != "first" {
				t.Fatalf("reordered entry description = %q, want first", got)
			}
		})
	}
}

// newListOrderEditor uses the same tree store for file and session paths.
func newListOrderEditor(t *testing.T, session bool, content string) (*Editor, string) {
	t.Helper()
	path := writeTestConfig(t, content)
	store := newTestTreeStore(t, path)
	var ed *Editor
	var err error
	if session {
		ed, err = NewEditorWithStorage(store, path)
	} else {
		ed, err = NewLooseFileEditor(store, path)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // Best effort cleanup.
	if session {
		ed.SetSession(NewEditSession("thomas", "ssh"))
	}
	return ed, path
}

// reopenListOrderEditor reads persisted content rather than the candidate tree.
func reopenListOrderEditor(t *testing.T, ed *Editor, path string, session bool) *Editor {
	t.Helper()
	var reopened *Editor
	var err error
	if session {
		reopened, err = NewEditorWithStorage(ed.store, path)
	} else {
		reopened, err = NewLooseFileEditor(ed.store, path)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() }) //nolint:errcheck,gosec // Best effort cleanup.
	return reopened
}

// commitListOrderEditor uses the commit primitive belonging to the editor mode.
func commitListOrderEditor(t *testing.T, ed *Editor) {
	t.Helper()
	if ed.session == nil {
		if _, err := ed.Save(); err != nil {
			t.Fatal(err)
		}
		return
	}
	result, err := ed.CommitSession()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conflicts) != 0 {
		t.Fatalf("commit conflicts: %v", result.Conflicts)
	}
}

// requireEditorListOrder compares every recorded key and entry count.
func requireEditorListOrder(t *testing.T, tree *config.Tree, want []string) {
	t.Helper()
	interfaces := tree.GetContainer("interface")
	if interfaces == nil {
		t.Fatal("interface container missing")
	}
	entries := interfaces.GetListOrdered("dummy")
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Key)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("recorded entry order = %v, want %v", got, want)
	}
	if count := len(interfaces.GetList("dummy")); count != len(want) {
		t.Fatalf("entry count = %d, want %d", count, len(want))
	}
}
