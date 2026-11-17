// Design: docs/architecture/container/image-store.md -- publication: the root reaches the disk before its mark
// Related: sync_linux.go -- the per-filesystem sync on Linux, where containers run

//go:build !linux

package imagestore

import (
	"os"

	"golang.org/x/sys/unix"
)

// syncFilesystem writes every file to the disk. The systems other than Linux
// that the tree compiles for have no per-filesystem sync, so the whole
// system is synced: containers run on Linux only, and this keeps the
// publication order the same where the package is compiled.
func syncFilesystem(_ *os.File) error {
	return unix.Sync()
}
