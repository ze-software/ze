// Design: docs/architecture/web-interface.md -- pending count and review ownership.
package web

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// pendingProgramBytes snapshots the committed source and present candidate
// files for the named fixture owners. Missing draft files are valid before a
// save; presence in this map also detects an unexpected new file afterward.
func pendingProgramBytes(t *testing.T, mgr *EditorManager, users ...string) map[string]string {
	t.Helper()
	paths := []string{mgr.configPath, cli.DraftPath(mgr.configPath)}
	for _, user := range users {
		paths = append(paths, cli.ChangePath(mgr.configPath, user))
	}
	files := make(map[string]string)
	for _, path := range paths {
		if !mgr.store.Exists(path) {
			continue
		}
		data, err := mgr.store.ReadFile(path)
		require.NoError(t, err)
		files[path] = string(data)
	}
	return files
}

// pendingCountStore refuses the unguarded directory census only after setup.
// Reading this user's known change file under its guard remains available.
type pendingCountStore struct {
	storage.Storage
	failListing bool
	refusals    int
}

func (s *pendingCountStore) List(path string) ([]string, error) {
	if s.failListing {
		s.refusals++
		return nil, errors.New("pending directory census unavailable")
	}
	return s.Storage.List(path)
}

// TestPendingCountUsesTheReviewedStructuralProgram keeps the useful review
// and its exact logical count when an unrelated directory census is unavailable.
// Two empty entries have no scalar metadata: both exist only as structural intent.
func TestPendingCountUsesTheReviewedStructuralProgram(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	store := &pendingCountStore{Storage: mgr.store}
	mgr.store = store
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "group", "first"}))
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "group", "second"}))
	require.NoError(t, mgr.CreateEntry("bob", []string{"bgp", "group", "foreign"}))
	before := pendingProgramBytes(t, mgr, "alice", "bob")
	require.Equal(t, 2, mgr.ChangeCount("alice"))
	want := "+ bgp group first \n+ bgp group second \n"
	review, err := mgr.Diff("alice")
	require.NoError(t, err)
	require.Equal(t, want, review)

	store.failListing = true
	assert.Equal(t, 2, mgr.ChangeCount("alice"), "count must describe the two reviewed operations, not a dirty fallback")
	review, err = mgr.Diff("alice")
	require.NoError(t, err)
	assert.Equal(t, want, review)
	store.failListing = false
	assert.Equal(t, before, pendingProgramBytes(t, mgr, "alice", "bob"), "inspection must preserve every owner's stored intent and committed config")
}

// TestPendingCountRetainsSavedAndUnsavedOwnership compares the logical count
// with the explicit reviewed records across a save, a later edit and another
// owner's saved intent. A replacement scalar remains one change despite its
// two presentation lines; an empty entry remains one change without any scalar.
func TestPendingCountRetainsSavedAndUnsavedOwnership(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "192.0.2.10"))
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "group", "saved-empty"}))
	require.NoError(t, mgr.SaveDraft("alice"))
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "session", "asn"}, "local", "65010"))
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "group", "unsaved-empty"}))
	require.NoError(t, mgr.CreateEntry("bob", []string{"bgp", "group", "foreign-empty"}))
	require.NoError(t, mgr.SaveDraft("bob"))
	before := pendingProgramBytes(t, mgr, "alice", "bob")
	assert.Equal(t, 4, mgr.ChangeCount("alice"))
	review, err := mgr.Diff("alice")
	require.NoError(t, err)
	assert.Contains(t, review, "- bgp router-id 10.0.0.250\n+ bgp router-id 192.0.2.10\n")
	assert.Contains(t, review, "- bgp session asn local 64999\n+ bgp session asn local 65010\n")
	assert.Contains(t, review, "+ bgp group saved-empty \n")
	assert.Contains(t, review, "+ bgp group unsaved-empty \n")
	assert.NotContains(t, review, "foreign-empty")
	assert.Equal(t, before, pendingProgramBytes(t, mgr, "alice", "bob"))
}
