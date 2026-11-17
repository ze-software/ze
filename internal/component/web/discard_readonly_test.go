// Design: docs/architecture/web-interface.md -- read-only discard snapshots.
// Related: discard_snapshot_test.go -- evicted ownership and transport refusals.

package web

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestDiscardAuditSnapshotDoesNotStartAnEditingSession reads saved assignments,
// unsaved assignments and an empty-entry operation after eviction. Inspection
// must preserve all bytes and must not construct a new session or cache entry.
func TestDiscardAuditSnapshotDoesNotStartAnEditingSession(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "group", "saved"}, "description", "saved-owner"))
	us, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	require.NoError(t, us.editor.SaveDraft())
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "group", "unsaved"}, "description", "unsaved-owner"))
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "group", "empty-owner"}))
	require.NoError(t, mgr.SetValue("other", []string{"bgp", "group", "theirs"}, "description", "foreign-owner"))
	before := discardSnapshotBytes(t, mgr.store, mgr.configPath)
	mgr.mu.Lock()
	delete(mgr.sessions, "alice")
	mgr.mu.Unlock()
	sessionFactory := mgr.editSessionFactory
	started := 0
	mgr.editSessionFactory = func(username, origin string) contract.EditSession {
		started++
		return sessionFactory(username, origin)
	}
	detail, err := mgr.discardDiff("alice")
	require.NoError(t, err)
	assert.Contains(t, detail, "saved-owner")
	assert.Contains(t, detail, "unsaved-owner")
	assert.Contains(t, detail, "empty-owner")
	assert.NotContains(t, detail, "foreign-owner")
	assert.Zero(t, started, "a snapshot does not enter active editing recovery")
	mgr.mu.RLock()
	_, cached := mgr.sessions["alice"]
	mgr.mu.RUnlock()
	assert.False(t, cached, "a private snapshot must not cache an unresumed editor")
	assert.Equal(t, before, discardSnapshotBytes(t, mgr.store, mgr.configPath))
}

// TestDiscardAuditSnapshotUsesLatestUnsavedAssignment keeps a saved value
// from shadowing a later assignment to that same session and leaf.
func TestDiscardAuditSnapshotUsesLatestUnsavedAssignment(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "group", "mine"}, "description", "saved-before-edit"))
	us, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	require.NoError(t, us.editor.SaveDraft())
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "group", "mine"}, "description", "latest-unsaved-value"))
	detail, err := mgr.discardDiff("alice")
	require.NoError(t, err)
	assert.Contains(t, detail, "latest-unsaved-value")
	assert.NotContains(t, detail, "saved-before-edit")
}

// TestDiscardAuditSnapshotFollowsSavedRename reads the shared planner's final
// address for saved metadata after a later unsaved rename, without moving the
// stored metadata or changing either durable file.
func TestDiscardAuditSnapshotFollowsSavedRename(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "group", "saved"}, "description", "saved-before-rename"))
	us, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	require.NoError(t, us.editor.SaveDraft())
	require.NoError(t, mgr.RenameListEntry("alice", []string{"bgp"}, "group", "saved", []string{"moved"}))
	draftBefore, err := mgr.store.ReadFile(cli.DraftPath(mgr.configPath))
	require.NoError(t, err)
	changeBefore, err := mgr.store.ReadFile(cli.ChangePath(mgr.configPath, "alice"))
	require.NoError(t, err)
	detail, err := mgr.discardDiff("alice")
	require.NoError(t, err)
	assert.Contains(t, detail, "+ bgp group moved description saved-before-rename")
	assert.NotContains(t, detail, "+ bgp group saved description")
	assert.Contains(t, detail, "~ rename bgp group saved to bgp group moved")
	draftAfter, err := mgr.store.ReadFile(cli.DraftPath(mgr.configPath))
	require.NoError(t, err)
	assert.Equal(t, draftBefore, draftAfter)
	changeAfter, err := mgr.store.ReadFile(cli.ChangePath(mgr.configPath, "alice"))
	require.NoError(t, err)
	assert.Equal(t, changeBefore, changeAfter)
}

// TestDiscardAuditSnapshotStorageFailuresRemainErrors injects failures only in
// the guarded snapshot, including its final release, while a cached editor is
// available. Each failure must refuse the detail, preserve all bytes, and leave
// the actual guard released so a repaired snapshot can complete.
func TestDiscardAuditSnapshotStorageFailuresRemainErrors(t *testing.T) {
	for _, stage := range []string{"draft", "change", "list", "release"} {
		t.Run(stage, func(t *testing.T) {
			mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
			require.NoError(t, mgr.SetValue("alice", []string{"bgp", "group", "saved"}, "description", "saved-owner"))
			us, err := mgr.GetOrCreate("alice")
			require.NoError(t, err)
			require.NoError(t, us.editor.SaveDraft())
			require.NoError(t, mgr.SetValue("alice", []string{"bgp", "group", "unsaved"}, "description", "unsaved-owner"))
			require.NoError(t, mgr.SetValue("other", []string{"bgp", "group", "theirs"}, "description", "foreign-owner"))
			require.NoError(t, mgr.RenameListEntry("alice", []string{"bgp"}, "group", "saved", []string{"moved"}))
			original := mgr.store
			before := discardSnapshotBytes(t, original, mgr.configPath)
			fault := &discardSnapshotFaultStore{Storage: original, failure: errors.New("injected snapshot " + stage + " failure")}
			switch stage {
			case "draft":
				fault.failPath = cli.DraftPath(mgr.configPath)
			case "change":
				fault.failPath = cli.ChangePath(mgr.configPath, "alice")
			case "list":
				fault.failList = true
			case "release":
				fault.failRelease = true
			}
			mgr.store = fault
			detail, err := mgr.discardDiff("alice")
			require.ErrorIs(t, err, fault.failure)
			assert.Empty(t, detail)
			assert.Equal(t, 1, fault.refused, "the intended guarded boundary must execute")
			assert.Equal(t, before, discardSnapshotBytes(t, original, mgr.configPath))
			mgr.store = original
			detail, err = mgr.discardDiff("alice")
			require.NoError(t, err, "failure must release the actual storage guard")
			assert.Contains(t, detail, "saved-owner")
			assert.Contains(t, detail, "unsaved-owner")
			assert.NotContains(t, detail, "foreign-owner")
		})
	}
}

// TestDiscardAuditSnapshotValidatesBeforeFactory preserves the shared identity
// boundary for private readers, with an allowed empty snapshot as its control.
func TestDiscardAuditSnapshotValidatesBeforeFactory(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	factory := mgr.editorFactory
	opened := 0
	mgr.editorFactory = func(store any, path string) (contract.Editor, error) {
		opened++
		return factory(store, path)
	}
	detail, err := mgr.discardDiff("../other")
	require.Error(t, err)
	assert.Empty(t, detail)
	assert.Zero(t, opened)
	detail, err = mgr.discardDiff("reader")
	require.NoError(t, err)
	assert.Empty(t, detail)
	assert.Equal(t, 1, opened)
}

// TestDiscardAuditSnapshotExcludesForeignSavedOwner puts both accounts' saved
// assignments in the same shared draft. The snapshot must include its owner's
// value, exclude the foreign path and value, and leave that entire draft intact.
func TestDiscardAuditSnapshotExcludesForeignSavedOwner(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "group", "own-saved"}, "description", "owner-saved-detail"))
	alice, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	aliceSession := alice.editor.SessionID()
	require.NoError(t, mgr.SaveDraft("alice"))
	require.NoError(t, mgr.SetValue("bob", []string{"bgp", "group", "foreign-saved"}, "description", "foreign-saved-detail"))
	bob, err := mgr.GetOrCreate("bob")
	require.NoError(t, err)
	bobSession := bob.editor.SessionID()
	require.NoError(t, mgr.SaveDraft("bob"))
	draftPath := cli.DraftPath(mgr.configPath)
	before, err := mgr.store.ReadFile(draftPath)
	require.NoError(t, err)
	_, metadata, err := config.NewSetParser(schema).ParseWithMeta(string(before))
	require.NoError(t, err)
	require.Len(t, metadata.SessionEntries(schema, aliceSession), 1, "the own saved record must be in this draft")
	require.Len(t, metadata.SessionEntries(schema, bobSession), 1, "the foreign saved record must be in the same draft")

	detail, err := mgr.discardDiff("alice")
	require.NoError(t, err)
	assert.Contains(t, detail, "+ bgp group own-saved description owner-saved-detail")
	assert.NotContains(t, detail, "bgp group foreign-saved")
	assert.NotContains(t, detail, "foreign-saved-detail")
	after, err := mgr.store.ReadFile(draftPath)
	require.NoError(t, err)
	assert.Equal(t, before, after, "owner selection is read-only for the complete shared draft")
}

// discardSnapshotBytes records only the four owned fixture files; every caller
// creates both users' intent and a saved draft before taking its comparison.
func discardSnapshotBytes(t *testing.T, store storage.Storage, configPath string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	for _, path := range []string{configPath, cli.DraftPath(configPath), cli.ChangePath(configPath, "alice"), cli.ChangePath(configPath, "other")} {
		data, err := store.ReadFile(path)
		require.NoError(t, err)
		files[path] = string(data)
	}
	return files
}

// The fault wrapper belongs to one synchronous test and never owns the store.
// Its release failure follows the real release so no guard survives the test.
type discardSnapshotFaultStore struct {
	storage.Storage
	failure     error
	failPath    string
	failRelease bool
	failList    bool
	refused     int
}

func (s *discardSnapshotFaultStore) AcquireLock(path string) (storage.WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	return discardSnapshotFaultGuard{WriteGuard: guard, owner: s}, nil
}

type discardSnapshotFaultGuard struct {
	storage.WriteGuard
	owner *discardSnapshotFaultStore
}

func (g discardSnapshotFaultGuard) ReadFile(path string) ([]byte, error) {
	if path == g.owner.failPath {
		g.owner.refused++
		return nil, g.owner.failure
	}
	return g.WriteGuard.ReadFile(path)
}

func (g discardSnapshotFaultGuard) List(path string) ([]string, error) {
	if g.owner.failList {
		g.owner.refused++
		return nil, g.owner.failure
	}
	return g.WriteGuard.List(path)
}

func (g discardSnapshotFaultGuard) Release() error {
	err := g.WriteGuard.Release()
	if g.owner.failRelease {
		g.owner.refused++
		return errors.Join(err, g.owner.failure)
	}
	return err
}
