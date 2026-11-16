// Design: docs/architecture/appliance/on-device-installer.md -- online resize of a mounted ext4 filesystem
// Overview: superblock.go -- the package and its not-implemented answer
// Related: resize_other.go -- the refusal on a system that is not Linux

//go:build linux

package ext4fs

// Resize grows the ext4 filesystem mounted on mountPoint to blockCount blocks
// of the filesystem's own block size. The filesystem never shrinks.
//
// It is a wiring stub: it makes no kernel call and answers ErrNotImplemented.
func Resize(_ string, _ uint64) error {
	return ErrNotImplemented
}
