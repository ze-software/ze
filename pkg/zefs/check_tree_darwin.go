// Design: docs/architecture/zefs-format.md -- exclusive tree repair publication.
// Related: check_tree_unix.go -- shared Linux and Darwin traversal.

package zefs

import (
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func renameFrameTree(parent *os.File, source, target string) error {
	return unix.RenameatxNp(int(parent.Fd()), source, int(parent.Fd()), target, unix.RENAME_EXCL)
}

// trustedFramePath selects Darwin's fixed system bases without following their
// symlink entries. /tmp and /var refer to /private/tmp and /private/var; every
// component of those canonical bases and the supplied suffix is then checked.
func trustedFramePath(path string) string {
	for _, base := range []string{"/tmp", "/var"} {
		if path == base {
			return "/private" + path
		}
		if strings.HasPrefix(path, base+"/") {
			return "/private" + path
		}
	}
	return path
}
