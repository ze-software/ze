//go:build integration && linux

// Design: docs/architecture/storage-backends.md -- coordination and durable publication.
package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestStoreLockNeedsNoDirectorySync proves that coordination works through a
// directory descriptor that permits openat but cannot be synced. Both ownership
// modes retain their exclusion, private stable inode and close/reopen lifecycle.
func TestStoreLockNeedsNoDirectorySync(t *testing.T) {
	for _, mode := range []struct {
		name string
		lock int
	}{
		{name: "exclusive", lock: unix.LOCK_EX},
		{name: "shared", lock: unix.LOCK_SH},
	} {
		t.Run(mode.name, func(t *testing.T) {
			dir := t.TempDir()
			fd, err := unix.Open(dir, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
			require.NoError(t, err)
			folder := os.NewFile(uintptr(fd), dir)
			defer folder.Close() //nolint:errcheck // Test cleanup.
			require.ErrorIs(t, folder.Sync(), unix.EBADF, "positive control: this descriptor cannot sync")

			first, err := lockStoreFile(folder, "database.lock", mode.lock)
			require.NoError(t, err, "a coordination lock needs naming and flock, not directory durability")
			defer first.Close() //nolint:errcheck // Also releases the owner if an assertion fails before explicit close.
			before, err := first.Stat()
			require.NoError(t, err)
			require.True(t, before.Mode().IsRegular())
			require.Equal(t, os.FileMode(0o600), before.Mode().Perm())

			reader, err := lockStoreFile(folder, "database.lock", unix.LOCK_SH)
			if mode.lock == unix.LOCK_SH {
				require.NoError(t, err, "shared readers must coexist")
				defer reader.Close() //nolint:errcheck // Test cleanup after an early assertion failure.
			} else {
				if reader != nil {
					require.NoError(t, reader.Close())
				}
				require.ErrorIs(t, err, ErrBusy, "an exclusive owner must exclude readers")
			}
			writer, err := lockStoreFile(folder, "database.lock", unix.LOCK_EX)
			if writer != nil {
				require.NoError(t, writer.Close())
			}
			require.ErrorIs(t, err, ErrBusy, "both ownership modes must exclude writers")
			require.NoError(t, first.Close())
			if mode.lock == unix.LOCK_SH {
				writer, err = lockStoreFile(folder, "database.lock", unix.LOCK_EX)
				if writer != nil {
					require.NoError(t, writer.Close())
				}
				require.ErrorIs(t, err, ErrBusy, "the remaining reader must retain ownership")
				require.NoError(t, reader.Close())
			}

			again, err := lockStoreFile(folder, "database.lock", unix.LOCK_EX)
			require.NoError(t, err, "closing every owner must release the lock")
			defer again.Close() //nolint:errcheck // Test cleanup.
			after, err := again.Stat()
			require.NoError(t, err)
			require.True(t, os.SameFile(before, after), "reacquisition must preserve the stable lock inode")
			require.Equal(t, os.FileMode(0o600), after.Mode().Perm())
		})
	}
}
