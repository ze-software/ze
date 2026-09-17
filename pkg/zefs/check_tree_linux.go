// Design: docs/architecture/zefs-format.md -- exclusive tree repair publication.
// Related: check_tree_unix.go -- shared Linux, Darwin and FreeBSD traversal.

package zefs

import (
	"os"

	"golang.org/x/sys/unix"
)

// RenameNoReplace renames source to target beneath parent and fails when target
// exists, including an empty directory ordinary rename would overwrite.
func RenameNoReplace(parent *os.File, source, target string) error {
	return unix.Renameat2(int(parent.Fd()), source, int(parent.Fd()), target, unix.RENAME_NOREPLACE)
}

func trustedFramePath(path string) string {
	return path
}
