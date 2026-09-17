// Design: docs/architecture/storage-backends.md -- no-replace publication.
package storage

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The vendored x/sys predates FreeBSD's renameat2 binding. These ABI values
// come from FreeBSD sys/sys/syscall.h and sys/sys/fcntl.h:
// https://github.com/freebsd/freebsd-src/blob/main/sys/sys/syscall.h
// https://github.com/freebsd/freebsd-src/blob/main/sys/sys/fcntl.h
const (
	freebsdRenameat2       = 602
	freebsdRenameNoReplace = 0x0001
)

func renameNoReplace(folder *os.File, source, target string) error {
	from, err := unix.BytePtrFromString(source)
	if err != nil {
		return err
	}
	to, err := unix.BytePtrFromString(target)
	if err != nil {
		return err
	}
	fd := folder.Fd()
	_, _, errno := unix.Syscall6(freebsdRenameat2, fd, uintptr(unsafe.Pointer(from)), fd, uintptr(unsafe.Pointer(to)), freebsdRenameNoReplace, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
