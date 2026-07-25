// Design: docs/architecture/fleet-config.md -- managed client runtime commit wiring

package hub

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
)

func TestWireManagedCommitStagesCandidateAndPromotes(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "ze.conf")
	activeStamp := "20260524-090000.000"
	reloadCalled := false

	require.NoError(t, store.WriteFile(configPath, []byte("old"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), mustParseManagedStamp(t, activeStamp)))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerActive, activeStamp))

	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		reloadCalled = true
		data, _, ok, err := storage.ReadCandidateConfig(store, configPath)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "new", string(data))
		return storage.PromoteCandidate(store, configPath)
	}, nil)

	require.NotNil(t, client.OnCommit)
	require.NoError(t, client.OnCommit([]byte("new")))
	assert.True(t, reloadCalled)

	rollback, ok, err := storage.ReadPointer(store, configPath, storage.PointerRollback)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, activeStamp, rollback)

	_, ok, err = storage.ReadPointer(store, configPath, storage.PointerCandidate)
	require.NoError(t, err)
	assert.False(t, ok)

	activeData, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Equal(t, "new", string(activeData))
}

func TestWireManagedCommitClearsCandidateOnReloadFailure(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "ze.conf")
	activeStamp := "20260524-090000.000"

	require.NoError(t, store.WriteFile(configPath, []byte("old"), 0o600))
	require.NoError(t, store.WriteVersion(configPath, []byte("old"), mustParseManagedStamp(t, activeStamp)))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerActive, activeStamp))

	client := &managed.ClientConfig{}
	wireManagedCommit(client, store, configPath, func() error {
		return errors.New("verify rejected")
	}, nil)

	err := client.OnCommit([]byte("new"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verify rejected")

	active, ok, err := storage.ReadPointer(store, configPath, storage.PointerActive)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, activeStamp, active)

	_, ok, err = storage.ReadPointer(store, configPath, storage.PointerCandidate)
	require.NoError(t, err)
	assert.False(t, ok)
}

func mustParseManagedStamp(t *testing.T, stamp string) time.Time {
	t.Helper()
	parsed, err := storage.ParseVersionStamp(stamp)
	require.NoError(t, err)
	return parsed
}
