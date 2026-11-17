//go:build linux || darwin || freebsd

// Design: docs/guide/config-editor.md -- bind a confined import to its opened file.

package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/zefs"
)

// openLoadFile opens the validated canonical path without following a replaced
// component. The caller MUST close the returned file.
func openLoadFile(path string) (*os.File, error) {
	parent, err := zefs.OpenDirectory(filepath.Dir(path), false)
	if err != nil {
		return nil, err
	}
	fd, openErr := unix.Openat(int(parent.Fd()), filepath.Base(path),
		unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	closeErr := parent.Close()
	if openErr != nil {
		return nil, errors.Join(openErr, closeErr)
	}
	file := os.NewFile(uintptr(fd), path)
	info, statErr := file.Stat()
	if err := errors.Join(statErr, closeErr); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if !info.Mode().IsRegular() {
		return nil, errors.Join(fs.ErrPermission, file.Close())
	}
	return file, nil
}
