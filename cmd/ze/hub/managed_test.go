// Design: docs/architecture/fleet-config.md -- managed client runtime commit wiring

package hub

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/pkg/zefs"
)

func TestWireManagedCommitStagesCandidateAndPromotes(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	configPath := filepath.Join(dir, "ze.conf")
	activeStamp := "20260524-090000.000"
	reloadCalled := false

	require.NoError(t, store.WriteFile(configPath, []byte("old"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), mustParseManagedStamp(t, activeStamp)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, activeStamp)

	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		reloadCalled = true
		data, _, ok, err := storage.ReadCandidateConfig(store, configPath)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "new", string(data))
		return storage.PromoteCandidate(store, configPath)
	}, nil, nil)

	require.NotNil(t, client.OnCommit)
	require.NoError(t, client.OnCommit([]byte("new")))
	assert.True(t, reloadCalled)

	rollback, ok := configPointer(t, store, configPath, zefs.KeyConfigRollback)
	require.True(t, ok)
	assert.Equal(t, activeStamp, rollback)

	_, ok = configPointer(t, store, configPath, zefs.KeyConfigCandidate)
	assert.False(t, ok)

	activeData, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "new", string(activeData))
}

func TestWireManagedCommitClearsCandidateOnReloadFailure(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	configPath := filepath.Join(dir, "ze.conf")
	activeStamp := "20260524-090000.000"

	require.NoError(t, store.WriteFile(configPath, []byte("old"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), mustParseManagedStamp(t, activeStamp)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, activeStamp)

	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		return errors.New("verify rejected")
	}, nil, nil)

	err := client.OnCommit([]byte("new"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verify rejected")

	active, ok := configPointer(t, store, configPath, zefs.KeyConfigActive)
	require.True(t, ok)
	assert.Equal(t, activeStamp, active)

	_, ok = configPointer(t, store, configPath, zefs.KeyConfigCandidate)
	assert.False(t, ok)
}

// VALIDATES: a managed push that lands while the daemon's confirmed-commit
// window is open is refused with the window's refusal naming its owner, so the
// managed server reads an error, and nothing is staged or reloaded: the
// window's revert would wipe the push.
// PREVENTS: a hub push applied inside a window and silently undone at its
// deadline.
func TestWireManagedCommitRefusedDuringConfirmWindow(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	configPath := filepath.Join(dir, "ze.conf")
	activeStamp := "20260524-090000.000"
	require.NoError(t, store.WriteFile(configPath, []byte("old"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), mustParseManagedStamp(t, activeStamp)))
	setConfigPointer(t, store, configPath, zefs.KeyConfigActive, activeStamp)

	window := confirm.NewWindow(func([]byte) error { return nil }, confirmNopRecorder{})
	t.Cleanup(window.Stop)
	opened := confirm.Commit{
		Snapshot: func() ([]byte, error) { return []byte("old"), nil },
		Apply:    func() error { return nil },
	}
	require.NoError(t, window.Confirmed("alice", time.Minute, false, opened))

	reloaded := false
	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		reloaded = true
		return storage.PromoteCandidate(store, configPath)
	}, nil, func() *confirm.Window { return window })

	err := client.OnCommit([]byte("new"))
	var other *confirm.OtherUserError
	require.ErrorAs(t, err, &other, "a push during alice's window is refused with the window's refusal")
	assert.Equal(t, "alice", other.Owner)
	assert.False(t, reloaded, "the refused push reloaded the daemon")
	_, ok := configPointer(t, store, configPath, zefs.KeyConfigCandidate)
	assert.False(t, ok, "the refused push staged a candidate")
	activeData, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "old", string(activeData))
}

func mustParseManagedStamp(t *testing.T, stamp string) time.Time {
	t.Helper()
	parsed := mustParseReloadStamp(t, stamp)
	return parsed
}
