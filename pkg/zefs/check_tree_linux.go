// Design: docs/architecture/zefs-format.md -- exclusive tree repair publication.
// Related: check_tree_unix.go -- shared Linux and Darwin traversal.

package zefs

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameFrameTree(parent *os.File, source, target string) error {
	return unix.Renameat2(int(parent.Fd()), source, int(parent.Fd()), target, unix.RENAME_NOREPLACE)
}

func trustedFramePath(path string) string {
	return path
}
