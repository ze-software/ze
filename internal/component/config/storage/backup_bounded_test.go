// Design: docs/architecture/storage-backends.md -- complete backups inside admitted byte limits.
package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBackupBoundedPreservesCompleteStore requires the real native encoder to
// preserve binary, empty and credential values while staying inside its limit.
func TestBackupBoundedPreservesCompleteStore(t *testing.T) {
	source, err := Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, source.Close()) })
	want := map[string][]byte{
		"test/backup/empty":  {},
		"test/backup/binary": {0, 255, 128, 0, 1},
		"test/backup/secret": []byte("private-credential"),
	}
	for key, value := range want {
		require.NoError(t, source.WriteKey(key, value))
	}
	path := filepath.Join(t.TempDir(), "bounded.zefs")
	result, err := BackupBounded(t.Context(), source, path, 4096)
	require.NoError(t, err)
	require.LessOrEqual(t, result.Bytes, int64(4096))
	require.Equal(t, len(want), result.Keys)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	copy, err := OpenBlob(path, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, copy.Close()) })
	keys, err := copy.ListKeys("")
	require.NoError(t, err)
	require.Len(t, keys, len(want))
	for key, value := range want {
		got, err := copy.ReadKey(key)
		require.NoError(t, err)
		require.True(t, bytes.Equal(value, got), key)
		live, err := source.ReadKey(key)
		require.NoError(t, err)
		require.True(t, bytes.Equal(value, live), key)
	}
}

// TestBackupBoundedRefusesBeforeArtifactCreation uses a real oversized value:
// failure must precede even a temporary encoder output or output lock file.
func TestBackupBoundedRefusesBeforeArtifactCreation(t *testing.T) {
	source, err := Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, source.Close()) })
	content := bytes.Repeat([]byte{128, 255}, 2048)
	require.NoError(t, source.WriteKey("test/backup/oversized", content))
	for _, allowance := range []int64{-1, 0, 255, 300, 1024} {
		directory := t.TempDir()
		_, err := BackupBounded(t.Context(), source, filepath.Join(directory, "refused.zefs"), allowance)
		require.ErrorIs(t, err, ErrBackupAllowance)
		entries, err := os.ReadDir(directory)
		require.NoError(t, err)
		require.Empty(t, entries)
		kept, err := source.ReadKey("test/backup/oversized")
		require.NoError(t, err)
		require.Equal(t, content, kept)
	}
}

// TestBackupBoundedCancellationKeepsSource refuses before creating output when
// its request has already ended, while preserving useful source bytes.
func TestBackupBoundedCancellationKeepsSource(t *testing.T) {
	source, err := Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, source.Close()) })
	require.NoError(t, source.WriteKey("test/backup/kept", []byte("kept")))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	directory := t.TempDir()
	_, err = BackupBounded(ctx, source, filepath.Join(directory, "cancelled.zefs"), 4096)
	require.ErrorIs(t, err, context.Canceled)
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Empty(t, entries)
	kept, err := source.ReadKey("test/backup/kept")
	require.NoError(t, err)
	require.Equal(t, []byte("kept"), kept)
}
