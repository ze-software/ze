// Design: docs/architecture/storage-backends.md -- a live salvage walk under the configuration guard.
package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/zefs"
)

// TestHoldWritesKeepsEveryWriterOffTheWalk verifies A-7 of the data store
// maintenance spec: a key the daemon removes while a salvage walks the live
// tree waits for the walk instead of failing it. Method: inside the walk, both
// locks a writer takes (configuration and runtime state) are shown held with
// TryLock, a removal of a runtime-state key is started, the real salvage runs
// over the tree, and the removal is shown still waiting when the walk ends. The
// salvage holds the key, and the removal completes once the guard is released.
func TestHoldWritesKeepsEveryWriterOffTheWalk(t *testing.T) {
	source := newTreeStorage(t, t.TempDir())
	const runtimeKey = "meta/test/removed-during-walk"
	require.NoError(t, source.WriteKey(runtimeKey, []byte("runtime state")))
	require.NoError(t, source.WriteKey("file/active/router.conf", []byte("config")))
	owned, ok := ownedStore(source)
	require.True(t, ok)

	output := filepath.Join(t.TempDir(), "salvage")
	removed := make(chan error, 1)
	err := HoldWrites(source, func(tree string) error {
		if owned.writer.TryLock() {
			owned.writer.Unlock()
			t.Error("the configuration lock was free during the walk")
		}
		if owned.runtime.TryLock() {
			owned.runtime.Unlock()
			t.Error("the runtime-state lock was free during the walk")
		}
		go func() { removed <- source.RemoveKey(runtimeKey) }()
		report, walkErr := zefs.RepairPath(tree, output)
		if walkErr != nil {
			return walkErr
		}
		select {
		case removeErr := <-removed:
			t.Errorf("the removal ran during the walk (err %v)", removeErr)
		default:
		}
		require.Contains(t, report.Recovered, runtimeKey)
		require.Contains(t, report.Recovered, "file/active/router.conf")
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, <-removed)
	_, err = source.ReadKey(runtimeKey)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// TestSalvageCopiesTheLiveTree verifies AC-11's salvage: Salvage writes a new
// tree holding every key of the live store, byte-equal, and leaves the live
// store as it was.
func TestSalvageCopiesTheLiveTree(t *testing.T) {
	source := newTreeStorage(t, t.TempDir())
	keys := map[string]string{
		"meta/test/state":         "runtime state",
		"file/active/router.conf": "config text",
	}
	for key, value := range keys {
		require.NoError(t, source.WriteKey(key, []byte(value)))
	}

	output := filepath.Join(t.TempDir(), "salvage")
	report, err := Salvage(source, output)
	require.NoError(t, err)
	require.Equal(t, len(keys), report.RecoveredCount)
	require.Zero(t, report.SkippedCount)

	salvaged, err := OpenTree(output, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, salvaged.Close()) })
	for key, value := range keys {
		data, readErr := salvaged.ReadKey(key)
		require.NoError(t, readErr, key)
		require.Equal(t, value, string(data), key)
		live, readErr := source.ReadKey(key)
		require.NoError(t, readErr, key)
		require.Equal(t, value, string(live), key)
	}
}

// TestSalvageRefusesAnOutputTheStoreOwns verifies AC-11's refusals below the
// path rules: an output naming a store-owned entry of the store folder, one
// inside the live tree, and one that exists are each refused with the rule
// named, and none of them writes anything.
func TestSalvageRefusesAnOutputTheStoreOwns(t *testing.T) {
	dir := t.TempDir()
	source := newTreeStorage(t, dir)
	require.NoError(t, source.WriteKey("meta/test/state", []byte("state")))
	existing := filepath.Join(t.TempDir(), "taken")
	require.NoError(t, os.Mkdir(existing, 0o700))

	for _, refused := range []struct {
		name   string
		output string
		rule   string
	}{
		{"lock sidecar", filepath.Join(dir, "salvage.lock"), "a lock sidecar"},
		{"inside the live tree", filepath.Join(dir, treeName, "salvage"), "inside the live tree"},
		{"existing output", existing, "already exists"},
	} {
		t.Run(refused.name, func(t *testing.T) {
			_, err := Salvage(source, refused.output)
			require.Error(t, err)
			require.Contains(t, err.Error(), refused.rule)
			if refused.output != existing {
				_, statErr := os.Lstat(refused.output)
				require.True(t, errors.Is(statErr, fs.ErrNotExist), "a refused output was written: %v", statErr)
			}
		})
	}
}
