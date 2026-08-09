// Design: docs/architecture/appliance/installer-initrd.md -- block device ioctls + syscall wiring

//go:build linux

package disk

import (
	"fmt"
	"log/slog"

	"golang.org/x/sys/unix"
)

func init() {
	blkRereadPart = sysBlkRereadPart
	syncFS = func() { unix.Sync() }
	rebootFS = func() {
		unix.Sync()
		_ = unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
	}
	poweroffFS = func() {
		unix.Sync()
		_ = unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
	}
}

func sysBlkRereadPart(disk string) error {
	fd, err := unix.Open(disk, unix.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", disk, err)
	}
	defer func() {
		if cerr := unix.Close(fd); cerr != nil {
			slog.Warn("close block device failed", "disk", disk, "error", cerr)
		}
	}()
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.BLKRRPART, 0)
	if errno != 0 {
		return fmt.Errorf("BLKRRPART %s: %w", disk, errno)
	}
	return nil
}
