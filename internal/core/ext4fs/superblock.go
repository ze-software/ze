// Design: docs/architecture/appliance/on-device-installer.md -- the ext4 leaf: superblock properties and online resize
// Detail: resize_linux.go -- the online resize of a mounted filesystem
// Detail: resize_other.go -- the refusal on a system that is not Linux

// Package ext4fs reads the properties of an ext4 superblock and grows a
// mounted ext4 filesystem.
//
// It is a core leaf with no registry and no config lifecycle. The superblock
// read is pure Go, so the build host checks an image with it; the resize is a
// kernel call in a Linux-only file.
//
// The package is a wiring stub. Every operation answers ErrNotImplemented, and
// the kernel-facts and build-format phases of
// plan/post-release/features/spec-appliance-perm-grows-at-install.md replace
// each one.
package ext4fs

import (
	"errors"
	"io"
)

// ErrNotImplemented is what every operation of this package answers until the
// ext4 code is written. A caller MUST treat it as a failure: no superblock was
// read and no filesystem was resized.
var ErrNotImplemented = errors.New("ext4fs: the ext4 code is not implemented yet")

// ErrUnsupported is what Resize answers on a system that is not Linux.
var ErrUnsupported = errors.New("ext4fs: an ext4 filesystem is resized on Linux only")

// Superblock holds the properties of a filesystem that the persistent
// partition's format fixes. The build-format phase gives it its fields.
type Superblock struct{}

// ReadSuperblock reads the superblock of the ext4 filesystem that starts at
// the first byte of filesystem.
//
// It is a wiring stub: it reads nothing and answers ErrNotImplemented.
func ReadSuperblock(_ io.ReaderAt) (Superblock, error) {
	return Superblock{}, ErrNotImplemented
}
