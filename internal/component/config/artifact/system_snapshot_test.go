// Design: docs/architecture/web-interface.md -- account-bound system snapshot lifetime.
package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

func systemSnapshotFixture(t *testing.T) (*Owner, storage.Storage) {
	t.Helper()
	owner := New()
	owner.parentTemp = t.TempDir()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteKey("test/system-snapshot/opaque", []byte{0, 255, 128}))
	return owner, store
}

func systemSnapshotProducer(ctx context.Context, store storage.Storage) func(string, int64) error {
	return func(path string, limit int64) error {
		_, err := storage.BackupBounded(ctx, store, path, limit)
		return err
	}
}

// TestArtifactSystemSnapshotWithoutConfiguration proves a complete store with
// no configuration remains downloadable, account-bound, immutable and removable.
func TestArtifactSystemSnapshotWithoutConfiguration(t *testing.T) {
	owner, store := systemSnapshotFixture(t)
	transfer, err := owner.SnapshotSystem(t.Context(), "alice", systemSnapshotProducer(t.Context(), store))
	require.NoError(t, err)
	require.Len(t, owner.sources, 1)
	require.Empty(t, owner.items, "a system download must not create an editor workspace")
	path := owner.sources[transfer.ID].path
	require.NoError(t, storage.CheckArtifact(path))
	_, _, err = owner.Read(t.Context(), "bob", transfer.ID, 0)
	var refusal *Refusal
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, 404, refusal.Status)
	_, before, err := owner.Read(t.Context(), "alice", transfer.ID, 0)
	require.NoError(t, err)
	require.NoError(t, store.WriteKey("test/system-snapshot/opaque", []byte("new live value")))
	_, after, err := owner.Read(t.Context(), "alice", transfer.ID, 0)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.NoError(t, owner.Abort(t.Context(), "alice", transfer.ID))
	require.Empty(t, owner.sources)
	require.Zero(t, owner.bytes)
	_, err = os.Stat(filepath.Dir(path))
	require.ErrorIs(t, err, os.ErrNotExist)
}

// TestArtifactSystemSnapshotAccountAllowance refuses before calling the store
// producer once this account's retained snapshot inventory reaches its bound.
func TestArtifactSystemSnapshotAccountAllowance(t *testing.T) {
	owner, store := systemSnapshotFixture(t)
	for range AccountMax {
		_, err := owner.SnapshotSystem(t.Context(), "alice", systemSnapshotProducer(t.Context(), store))
		require.NoError(t, err)
	}
	called := false
	_, err := owner.SnapshotSystem(t.Context(), "alice", func(string, int64) error {
		called = true
		return errors.New("quota refusal must precede production")
	})
	var refusal *Refusal
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, 503, refusal.Status)
	require.False(t, called)
	require.Len(t, owner.sources, AccountMax)
}

// TestArtifactSystemSnapshotFailureRemovesOutput checks partial production and
// cancellation after a complete write against real private directories.
func TestArtifactSystemSnapshotFailureRemovesOutput(t *testing.T) {
	for _, kind := range []string{"partial", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			owner, store := systemSnapshotFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			_, err := owner.SnapshotSystem(ctx, "alice", func(path string, limit int64) error {
				if kind == "partial" {
					if err := os.WriteFile(path, []byte("partial private backup"), 0o600); err != nil {
						return err
					}
					return errors.New("injected snapshot production failure")
				}
				_, err := storage.BackupBounded(ctx, store, path, limit)
				cancel()
				return err
			})
			require.Error(t, err)
			if kind == "canceled" {
				require.ErrorIs(t, err, context.Canceled)
			}
			require.Empty(t, owner.sources)
			require.Zero(t, owner.bytes)
			entries, err := os.ReadDir(owner.root)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}

// TestArtifactSystemSnapshotCleanupDebtBlocksProduction retains a failed
// removal as explicit debt and prevents another source write until it clears.
func TestArtifactSystemSnapshotCleanupDebtBlocksProduction(t *testing.T) {
	owner, store := systemSnapshotFixture(t)
	owner.removeAll = func(string) error { return errors.New("injected snapshot cleanup failure") }
	_, err := owner.SnapshotSystem(t.Context(), "alice", func(path string, _ int64) error {
		if err := os.WriteFile(path, []byte("partial private backup"), 0o600); err != nil {
			return err
		}
		return errors.New("injected snapshot production failure")
	})
	require.Error(t, err)
	require.NotEmpty(t, owner.cleanupDebt)
	require.Positive(t, owner.debtBytes)
	called := false
	_, err = owner.SnapshotSystem(t.Context(), "bob", func(string, int64) error {
		called = true
		return errors.New("debt refusal must precede production")
	})
	var refusal *Refusal
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, 503, refusal.Status)
	require.False(t, called)
	owner.removeAll = os.RemoveAll
	_, err = owner.SnapshotSystem(t.Context(), "bob", systemSnapshotProducer(t.Context(), store))
	require.NoError(t, err)
	require.Empty(t, owner.cleanupDebt)
	require.Zero(t, owner.debtBytes)
}
