// Design: docs/architecture/appliance/on-device-installer.md -- the shared GPT partition-table leaf
// Detail: grow.go -- extending the table of an installed disk to the disk's size

// Package gptdisk reads, validates and extends a GPT partition table.
//
// It is a core leaf: pure Go over a reader and a writer, with no syscall, so
// the installer initrd, the build host and the daemon import the same table
// arithmetic. The caller holds the block device and tells the kernel about the
// result (a partition-table reread, or a partition resize on a live disk).
//
// The package is a wiring stub. Every operation answers ErrNotImplemented, and
// the partition-table phase of
// plan/post-release/features/spec-appliance-perm-grows-at-install.md replaces
// each one.
package gptdisk

import "errors"

// ErrNotImplemented is what every operation of this package answers until the
// partition-table code is written. A caller MUST treat it as a failure: no
// table was read and no sector was written.
var ErrNotImplemented = errors.New("gptdisk: the partition-table code is not implemented yet")
