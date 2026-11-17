// Design: docs/architecture/container/image-store.md -- publication: the root reaches the disk before its mark
// Related: sync_other.go -- the same step on the other systems the tree compiles for

//go:build linux

package imagestore

import (
	"os"

	"golang.org/x/sys/unix"
)

// syncFilesystem writes every file of the filesystem that holds dir to the
// disk, and returns once it is there. Publication calls it once per image,
// before the completion mark: syncing the filesystem once costs less than
// syncing each of the tens of thousands of files an image root holds, and
// the mark MUST NOT reach the disk before the root it vouches for.
func syncFilesystem(dir *os.File) error {
	return unix.Syncfs(int(dir.Fd()))
}
