package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestCheckCorruptHistoryFrameIsCorruption verifies `ze data check` grades a
// corrupt history frame as corruption on both encodings. Goal: a store whose
// config object frame fails its CRC exits 1, as every corrupt frame does.
// Method: write one version, flip one bit of the object's bytes on disk, run
// cmdCheck. On the tree the corrupt frame stays listed and does not read, and
// the history walk refuses a key it cannot read, so walking history there
// would turn the corruption verdict into exit 2, "the check could not run".
// VALIDATES: spec-storage-3 AC-10 exit contract (0 clean, 1 corrupt, 2 unreadable).
// PREVENTS: a disk error in history reported as a failure to run the check.
func TestCheckCorruptHistoryFrameIsCorruption(t *testing.T) {
	data := []byte("corrupt-history-object-bytes\n")
	stamp := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	sum := sha256.Sum256(data)
	objectFile := filepath.FromSlash(zefs.KeyObject.Key(hex.EncodeToString(sum[:])))

	flip := func(t *testing.T, path string) {
		t.Helper()
		raw, err := os.ReadFile(path) //nolint:gosec // the test's own temporary file
		require.NoError(t, err)
		index := bytes.Index(raw, data)
		require.GreaterOrEqual(t, index, 0, "object bytes not found in %s", path)
		raw[index] ^= 1
		require.NoError(t, os.WriteFile(path, raw, 0o600))
	}

	t.Run("tree", func(t *testing.T) {
		dir := t.TempDir()
		store, err := storage.Create(dir)
		require.NoError(t, err)
		require.NoError(t, store.WriteVersion("ze.conf", data, stamp))
		require.NoError(t, store.Close())
		tree := filepath.Join(dir, storage.TreeName)
		flip(t, filepath.Join(tree, objectFile))
		require.Equal(t, 1, cmdCheck(tree, nil))
	})

	t.Run("blob", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.zefs")
		blob, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
			return seed.WriteVersion("ze.conf", data, stamp)
		}, false, zefs.Spare(0))
		require.NoError(t, err)
		require.NoError(t, blob.Close())
		flip(t, path)
		require.Equal(t, 1, cmdCheck(path, nil))
	})
}
