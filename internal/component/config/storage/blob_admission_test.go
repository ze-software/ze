// Design: docs/architecture/storage-backends.md -- artifact admission and ownership.
package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/zefs"
)

func TestOpenBlobUnsafeEntryBeforeLock(t *testing.T) {
	for _, writable := range []bool{false, true} {
		for _, kind := range []string{"missing", "symlink", "directory", "fifo", "public", "executable", "setuid", "setgid", "sticky", "foreign-owner"} {
			label := "reader/"
			if writable {
				label = "writer/"
			}
			t.Run(label+kind, func(t *testing.T) {
				if kind == "foreign-owner" && os.Geteuid() != 0 {
					t.Skip("changing file ownership requires root; no sudo")
				}
				dir := t.TempDir()
				path := filepath.Join(dir, "artifact")
				sentinel := filepath.Join(dir, "sentinel")
				require.NoError(t, os.WriteFile(sentinel, []byte("preserve"), 0o600))
				switch kind {
				case "missing":
				case "symlink":
					require.NoError(t, os.Symlink(sentinel, path))
				case "directory":
					require.NoError(t, os.Mkdir(path, 0o700))
				case "fifo":
					require.NoError(t, unix.Mkfifo(path, 0o600))
				default:
					require.NoError(t, os.WriteFile(path, []byte("preserve"), 0o600))
					mode := os.FileMode(0o600)
					switch kind {
					case "public":
						mode = 0o644
					case "executable":
						mode = 0o700
					case "setuid":
						mode |= os.ModeSetuid
					case "setgid":
						mode |= os.ModeSetgid
					case "sticky":
						mode |= os.ModeSticky
					case "foreign-owner":
						require.NoError(t, os.Chown(path, 1, -1))
					}
					require.NoError(t, os.Chmod(path, mode))
				}
				opened, err := OpenBlob(path, writable)
				if opened != nil {
					require.NoError(t, opened.Close())
				}
				require.Error(t, err)
				if kind == "missing" {
					require.ErrorIs(t, err, os.ErrNotExist)
				} else {
					require.ErrorIs(t, err, ErrPermissions)
				}
				require.NoFileExists(t, path+".lock")
				data, err := os.ReadFile(sentinel)
				require.NoError(t, err)
				require.Equal(t, "preserve", string(data))
			})
		}
	}
}

func TestOpenBlobConsumedDescriptorUnderLock(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		label := "accepted-descriptor"
		if unsafe {
			label = "unsafe-replacement"
		}
		t.Run(label, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "artifact")
			seed, err := CreateBlob(path)
			require.NoError(t, err)
			require.NoError(t, seed.WriteFile("router.conf", []byte("accepted"), 0))
			require.NoError(t, seed.Close())
			var observed, consumed *os.File
			calls := 0
			acquire := func(folder *os.File, name string, directory bool) (*os.File, error) {
				calls++
				if calls == 1 {
					var err error
					observed, err = openNode(folder, name, directory)
					return observed, err
				}
				_, err := observed.Stat()
				require.ErrorIs(t, err, os.ErrClosed)
				competitor, err := lockStoreFile(folder, name+".lock", unix.LOCK_EX)
				if competitor != nil {
					require.NoError(t, competitor.Close())
				}
				require.ErrorIs(t, err, ErrBusy, "consumption must acquire under flock")
				if unsafe {
					require.NoError(t, os.Chmod(path, 0o644))
				}
				consumed, err = openNode(folder, name, directory)
				if err != nil {
					return nil, err
				}
				// Replace the name after descriptor validation: loading must still
				// consume the acquired descriptor, not reopen the invalid name.
				require.NoError(t, os.Rename(path, path+".held"))
				require.NoError(t, os.WriteFile(path, []byte("invalid replacement"), 0o644))
				return consumed, nil
			}
			opened, err := openBlob(path, false, acquire)
			require.Equal(t, 2, calls)
			if unsafe {
				require.ErrorIs(t, err, ErrPermissions)
				require.Nil(t, opened)
				require.NoError(t, os.Chmod(path, 0o600))
				// Failed admission must release the lock.
				retry, err := OpenBlob(path, true)
				require.NoError(t, err)
				require.NoError(t, retry.Close())
				return
			}
			require.NoError(t, err)
			data, err := opened.ReadFile("router.conf")
			require.NoError(t, err)
			require.Equal(t, "accepted", string(data))
			require.NoError(t, opened.Close())
			_, err = consumed.Stat()
			require.ErrorIs(t, err, os.ErrClosed)
			data, err = os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, "invalid replacement", string(data))
			report, err := zefs.Check(path + ".held")
			require.NoError(t, err)
			require.True(t, report.ContainerOK)
		})
	}
}
