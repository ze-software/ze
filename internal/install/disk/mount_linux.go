// Design: docs/architecture/appliance/installer-initrd.md -- mount/umount via unix syscalls

//go:build linux

package disk

import (
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/sys/unix"
)

func init() {
	mountFS = func(source, target, fstype string, readOnly bool) error {
		slog.Debug("mount", "source", source, "target", target, "fstype", fstype, "ro", readOnly)
		if err := os.MkdirAll(target, 0o750); err != nil {
			return fmt.Errorf("mkdir %s: %w", target, err)
		}
		var flags uintptr
		if readOnly {
			flags = unix.MS_RDONLY
		}
		if err := unix.Mount(source, target, fstype, flags, ""); err != nil {
			return fmt.Errorf("mount %s on %s (%s): %w", source, target, fstype, err)
		}
		return nil
	}
	umountFS = func(target string) error {
		slog.Debug("umount", "target", target)
		if err := unix.Unmount(target, 0); err != nil {
			return fmt.Errorf("umount %s: %w", target, err)
		}
		return nil
	}
}
