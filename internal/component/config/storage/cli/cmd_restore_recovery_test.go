// Design: docs/architecture/storage-backends.md -- explicit private recovery resolution.
package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestOfflineFullRestoreResolvesPrivateRecovery proves the actual standalone
// full-restore adapter consumes a proven source through explicit resume and
// never asks a daemon after the original owner has already closed.
func TestOfflineFullRestoreResolvesPrivateRecovery(t *testing.T) {
	live, storePath := ownedStore(t, "meta/original")
	dir := filepath.Dir(storePath)
	owner, err := storage.NewDaemonRecovery(storage.BindConfigSource(live, "ze.conf", storage.ConfigSourceStored))
	require.NoError(t, err)
	original := dataArtifact(t, "router.conf", "replacement")
	_, err = storage.RecoveryFor(owner.Borrow()).StagePath(context.Background(), original)
	require.NoError(t, err)
	closed, err := owner.Close()
	require.NoError(t, err)
	competing, err := storage.Open(dir)
	require.NoError(t, err)
	require.ErrorIs(t, closed.Apply(), storage.ErrBusy)
	stage, err := storage.PendingRecoveryStage(competing)
	require.NoError(t, err)
	require.NoError(t, competing.Close())
	code := cmdRestoreFullThrough(storePath, restoreArgs{file: filepath.Join(stage, "snapshot.zefs"), mode: restoreModeFull}, func(_, _ string) ([]byte, error) {
		t.Fatal("offline owned recovery must not ask a daemon")
		return nil, nil
	})
	require.Zero(t, code)
	_, err = os.Stat(stage)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.FileExists(t, original)
}

// TestOfflineFullRestoreRefusesUnownedRecoveryPrefix proves a missing ownership
// record cannot fall through to generic restore and mutate the target store.
func TestOfflineFullRestoreRefusesUnownedRecoveryPrefix(t *testing.T) {
	live, storePath := ownedStore(t, "meta/original")
	require.NoError(t, live.Close())
	stage := filepath.Join(filepath.Dir(storePath), "database.recovery-tmp-user-owned")
	require.NoError(t, os.Mkdir(stage, 0o700))
	bytes, err := os.ReadFile(dataArtifact(t, "router.conf", "replacement"))
	require.NoError(t, err)
	snapshot := filepath.Join(stage, "snapshot.zefs")
	require.NoError(t, os.WriteFile(snapshot, bytes, 0o600))
	marker := filepath.Join(stage, "user-notes")
	require.NoError(t, os.WriteFile(marker, []byte("preserve this"), 0o600))
	code := cmdRestoreFullThrough(storePath, restoreArgs{file: snapshot, mode: restoreModeFull}, func(_, _ string) ([]byte, error) {
		t.Fatal("unowned source refusal must not ask a daemon")
		return nil, nil
	})
	require.Equal(t, 1, code)
	require.FileExists(t, snapshot)
	require.FileExists(t, marker)
	current, err := storage.Open(filepath.Dir(storePath))
	require.NoError(t, err)
	defer func() { require.NoError(t, current.Close()) }()
	raw, err := current.ReadKey("meta/original")
	require.NoError(t, err)
	require.Equal(t, "keep", string(raw))
}
