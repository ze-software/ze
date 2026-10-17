package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// syncCounter records every barrier a write asks for, by file name, and then
// performs the real sync so the write stays as durable as in production.
type syncCounter struct {
	names []string
}

func (c *syncCounter) sync(file *os.File) error {
	c.names = append(c.names, file.Name())
	return file.Sync()
}

// TestInstallBytesSyncsFrameThenDestination counts the barriers of one
// publication. Goal: the frame and the destination directory are synced, in
// that order, and nothing else is: neither a repeat of the destination when it
// is also the stage, nor the stage when it is distinct. Method: install into
// each shape with a counting sync and read the value back.
func TestInstallBytesSyncsFrameThenDestination(t *testing.T) {
	for _, shape := range []string{"stage is destination", "stage is distinct"} {
		t.Run(shape, func(t *testing.T) {
			dir := t.TempDir()
			destination := dir
			if shape == "stage is distinct" {
				destination = filepath.Join(dir, "keys")
				require.NoError(t, os.Mkdir(destination, 0o700))
			}
			stage, err := openFolder(dir)
			require.NoError(t, err)
			defer stage.Close() //nolint:errcheck // test cleanup.
			parent, err := openFolder(destination)
			require.NoError(t, err)
			defer parent.Close() //nolint:errcheck // test cleanup.

			var counter syncCounter
			require.NoError(t, installBytes(stage, parent, "value", []byte("bytes"), counter.sync))

			require.Len(t, counter.names, 2, "frame and destination only: %v", counter.names)
			assert.Equal(t, ".ze-storage-", filepath.Base(counter.names[0])[:len(".ze-storage-")], "the frame first")
			assert.Equal(t, parent.Name(), counter.names[1], "the destination directory second")
			got, err := os.ReadFile(filepath.Join(destination, "value"))
			require.NoError(t, err)
			assert.Equal(t, "bytes", string(got))
		})
	}
}

// TestTreeRewriteSyncCount counts the barriers of a tree key write whose
// directories already exist. Goal: two barriers, the frame and the key's
// directory; the containing folder that stages the frame is not synced.
// Method: write the key once so its ancestors exist, then count the rewrite.
func TestTreeRewriteSyncCount(t *testing.T) {
	s := newTreeStorage(t, t.TempDir())
	concrete, ok := s.(*store)
	require.True(t, ok)
	require.NoError(t, s.WriteFile("file/active/router.conf", []byte("one"), 0o600))

	var counter syncCounter
	concrete.tree.sync = counter.sync
	t.Cleanup(func() { concrete.tree.sync = nil })
	require.NoError(t, s.WriteFile("file/active/router.conf", []byte("two"), 0o600))

	require.Len(t, counter.names, 2, "frame and key directory only: %v", counter.names)
	assert.Equal(t, "active", filepath.Base(counter.names[1]))
	got, err := s.ReadFile("file/active/router.conf")
	require.NoError(t, err)
	assert.Equal(t, "two", string(got))
}

// TestLockStoreFileSyncsFolderOnlyWhenCreated counts the folder barriers of
// taking a store lock. Goal: the first lock, whose exclusive create adds the
// file, syncs the folder once; a later lock of the existing file syncs
// nothing, because a flock is kernel state and the folder did not change.
// Method: lock a fresh name, release it, lock it again, counting each time.
func TestLockStoreFileSyncsFolderOnlyWhenCreated(t *testing.T) {
	folder, err := openFolder(t.TempDir())
	require.NoError(t, err)
	defer folder.Close() //nolint:errcheck // test cleanup.

	var created syncCounter
	first, err := lockStoreFile(folder, "database.lock", unix.LOCK_EX, created.sync)
	require.NoError(t, err)
	require.NoError(t, first.Close())
	assert.Equal(t, []string{folder.Name()}, created.names, "a created lock syncs its folder once")

	var existing syncCounter
	second, err := lockStoreFile(folder, "database.lock", unix.LOCK_EX, existing.sync)
	require.NoError(t, err)
	require.NoError(t, second.Close())
	assert.Empty(t, existing.names, "an existing lock syncs nothing")
}
