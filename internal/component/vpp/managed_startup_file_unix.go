//go:build linux || darwin

// Design: docs/guide/vpp.md -- original startup CLI file readiness
package vpp

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func openStartupCLIFile(path string) (startupCLIFile, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		closeErr := unix.Close(fd)
		return nil, errors.Join(errors.New("VPP startup CLI file descriptor could not be retained"), closeErr)
	}
	return file, nil
}
