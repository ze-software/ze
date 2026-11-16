// Design: docs/architecture/appliance/on-device-installer.md -- telling the kernel a grown partition's size
// Related: kernel_linux.go -- the kernel call on Linux

//go:build !linux

package gptdisk

import (
	"errors"
	"os"
)

// ErrUnsupported is what ResizeInKernel answers on a system that is not
// Linux: the appliance's disks are grown on Linux only.
var ErrUnsupported = errors.New("a partition's size is handed to the kernel on Linux only")

// ResizeInKernel answers ErrUnsupported: the kernel call exists on Linux
// only.
func ResizeInKernel(_ *os.File, _ Growth) error {
	return ErrUnsupported
}
