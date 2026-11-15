// Design: docs/contributing/github-integration.md -- private receipt and credential descriptors never follow leaf aliases.
package github

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// notificationRootFile accepts one fixed basename in an already-open root.
// os.Root.OpenFile can resolve a symlink after O_NOFOLLOW fails, so the actual
// descriptor acquisition must call openat directly. O_NONBLOCK prevents a FIFO
// from waiting before the caller checks its descriptor's regular-file mode.
func notificationRootFile(root *os.Root, name string, flags int, mode uint32) (*os.File, error) {
	if filepath.Base(name) != name {
		return nil, errors.New("notification rooted file name must be a basename")
	}
	if name == "." {
		return nil, errors.New("notification rooted file name cannot name its directory")
	}
	if name == ".." {
		return nil, errors.New("notification rooted file name cannot name its parent directory")
	}
	directory, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	fd, openErr := unix.Openat(int(directory.Fd()), name, flags|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, mode)
	closeErr := directory.Close()
	if openErr != nil {
		return nil, errors.Join(openErr, closeErr)
	}
	if closeErr != nil {
		return nil, errors.Join(closeErr, unix.Close(fd))
	}
	return os.NewFile(uintptr(fd), name), nil
}
