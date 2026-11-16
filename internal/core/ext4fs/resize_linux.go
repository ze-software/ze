// Design: docs/architecture/appliance/on-device-installer.md -- online resize of a mounted ext4 filesystem
// Overview: superblock.go -- the package and its superblock read
// Related: resize_other.go -- the refusal on a system that is not Linux

//go:build linux

package ext4fs

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ext4IocResizeFS is EXT4_IOC_RESIZE_FS, read from the kernel's UAPI header
// linux/ext4.h: "#define EXT4_IOC_RESIZE_FS _IOW('f', 16, __u64)". The
// encoding is the generic one of linux/asm-generic/ioctl.h, which amd64 and
// arm64 use: the write direction (1) in bits 30-31, the argument's size (8) in
// bits 16-29, the type in bits 8-15 and the number in bits 0-7.
const ext4IocResizeFS = 1<<30 | 8<<16 | 'f'<<8 | 16

// Resize grows the ext4 filesystem mounted on mountPoint to blockCount blocks
// of the filesystem's own block size. The caller MUST be root (the kernel asks
// for CAP_SYS_RESOURCE) and MUST compute blockCount from the size of the
// partition, never from the disk.
//
// The kernel refuses a count below the current one, so the filesystem never
// shrinks; the guest test TestOnlineResizeGrowsMountedFilesystem holds that.
func Resize(mountPoint string, blockCount uint64) error {
	if blockCount == 0 {
		return ErrZeroBlockCount
	}
	dir, err := os.Open(filepath.Clean(mountPoint))
	if err != nil {
		return fmt.Errorf("ext4fs: %w", err)
	}
	defer dir.Close() //nolint:errcheck // a read-only descriptor opened only for the ioctl
	count := blockCount
	// x/sys/unix has no typed helper for a pointer to a __u64, so the pointer
	// is passed raw; count stays on this frame until the call returns.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, dir.Fd(), ext4IocResizeFS,
		uintptr(unsafe.Pointer(&count))) //nolint:gosec // EXT4_IOC_RESIZE_FS reads one __u64 through this pointer and keeps no reference
	if errno != 0 {
		return fmt.Errorf("ext4fs: the kernel refused to resize the filesystem on %s to %d blocks: %w",
			mountPoint, blockCount, errno)
	}
	return nil
}
