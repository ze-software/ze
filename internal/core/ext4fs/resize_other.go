// Design: docs/architecture/appliance/on-device-installer.md -- no ext4 resize off Linux
// Overview: superblock.go -- the package
// Related: resize_linux.go -- the online resize this file stands in for

//go:build !linux

package ext4fs

// Resize answers ErrUnsupported: the online resize is a Linux kernel call.
func Resize(_ string, _ uint64) error {
	return ErrUnsupported
}
