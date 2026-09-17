// Design: docs/architecture/storage-backends.md -- no-replace publication.
package storage

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameNoReplace(folder *os.File, source, target string) error {
	return unix.RenameatxNp(int(folder.Fd()), source, int(folder.Fd()), target, unix.RENAME_EXCL)
}
