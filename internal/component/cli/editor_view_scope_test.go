// Design: docs/architecture/config/change-file-structural-ops.md -- visible editor scopes.
// Related: editor_view.go -- the constructor decides shared draft or own candidate.
// Related: editor_session_cleanup_test.go -- publication consumes only its owner.

package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

type viewScopeFixture struct {
	current *Editor
	foreign string
	entries []config.SessionEntry
}

func newViewScopeFixture(t *testing.T, isolated bool) *viewScopeFixture {
	t.Helper()
	path := writeTestConfig(t, validBGPConfig+"\ninterface { dummy stable { description baseline; } }\n")
	store := newTestTreeStore(t, path)
	other, err := NewEditorWithStorage(store, path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, other.Close()) })
	other.SetSession(NewEditSession("bob", "ssh"))
	require.NoError(t, other.SetValue([]string{"interface", "dummy", "foreign"}, "description", "foreign-saved"))
	require.NoError(t, other.SaveDraft())
	var current *Editor
	if isolated {
		current, err = NewIsolatedEditorWithStorage(store, path)
	} else {
		current, err = NewEditorWithStorage(store, path)
	}
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, current.Close()) })
	// Both policies use the same origin: isolation comes from construction.
	current.SetSession(NewEditSession("alice", "ssh"))
	require.NoError(t, current.ResumeSession())
	return &viewScopeFixture{current: current, foreign: other.SessionID(),
		entries: other.meta.SessionEntries(other.schema, other.SessionID())}
}

func (f *viewScopeFixture) checkView(t *testing.T, shared, own bool) {
	t.Helper()
	foreign := walkPath(f.current.tree, f.current.schema, []string{"interface", "dummy", "foreign"})
	assert.Equal(t, shared, foreign != nil)
	assert.Equal(t, shared, strings.Contains(f.current.WorkingContent(), "foreign-saved"))
	assert.Equal(t, own, walkPath(f.current.tree, f.current.schema, []string{"interface", "dummy", "mine"}) != nil)
	data, err := f.current.store.ReadFile(DraftPath(f.current.originalPath))
	require.NoError(t, err)
	_, meta, err := config.NewSetParser(f.current.schema).ParseWithMeta(string(data))
	require.NoError(t, err)
	assert.ElementsMatch(t, f.entries, meta.SessionEntries(f.current.schema, f.foreign))
}

// TestStoredEditorSaveViewScope preserves the existing shared CLI materialized
// view while the explicitly isolated constructor keeps another owner's saved
// values out of its live tree and serialized candidate after SaveDraft.
func TestStoredEditorSaveViewScope(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		name := "shared"
		if isolated {
			name = "isolated"
		}
		t.Run(name, func(t *testing.T) {
			f := newViewScopeFixture(t, isolated)
			require.NoError(t, f.current.SetValue([]string{"interface", "dummy", "mine"}, "description", "own-pending"))
			require.NoError(t, f.current.SaveDraft())
			f.checkView(t, !isolated, true)
		})
	}
}

// TestStoredEditorRebuildViewScope drives the actual path-discard and committed
// refresh producers; each rebuild must retain the constructor's visible scope
// and preserve the other owner's durable metadata.
func TestStoredEditorRebuildViewScope(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		name := "shared"
		if isolated {
			name = "isolated"
		}
		t.Run(name, func(t *testing.T) {
			f := newViewScopeFixture(t, isolated)
			require.NoError(t, f.current.SetValue([]string{"interface", "dummy", "mine"}, "description", "own-pending"))
			require.NoError(t, f.current.DiscardSessionPath([]string{"interface", "dummy", "mine"}))
			f.checkView(t, !isolated, false)
			require.NoError(t, f.current.RefreshCommittedView())
			f.checkView(t, !isolated, false)
		})
	}
}

var errViewScopeCandidateRefused = errors.New("synthetic isolated candidate refusal")

// TestIsolatedCandidateRefusalKeepsOwnView reaches SaveDraft inside candidate
// commit, then refuses the actual private staged tree. Another owner's global
// draft values must not appear in the retained candidate or be published.
func TestIsolatedCandidateRefusalKeepsOwnView(t *testing.T) {
	f := newViewScopeFixture(t, true)
	editor := f.current
	require.NoError(t, editor.SetValue([]string{"interface", "dummy", "mine"}, "description", "own-pending"))
	before, err := editor.store.ReadFile(editor.originalPath)
	require.NoError(t, err)
	globalChecked := false
	privateChecked := false
	editor.SetPreCommitValidate(func(candidate string) error {
		if strings.Contains(candidate, "foreign-saved") {
			globalChecked = true
			return nil
		}
		privateChecked = true
		return errViewScopeCandidateRefused
	})
	_, _, err = editor.CommitSessionCandidate(time.Now())
	require.ErrorIs(t, err, errViewScopeCandidateRefused)
	require.True(t, globalChecked, "the real shared draft validation must run")
	require.True(t, privateChecked, "the staged own candidate must be refused")
	f.checkView(t, false, true)
	after, err := editor.store.ReadFile(editor.originalPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.NotEmpty(t, editor.PendingChanges(editor.SessionID()))
}
