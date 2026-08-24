// Design: docs/architecture/appliance/installer-initrd.md -- PID-1 bootstrap: mount, console

//go:build linux && ze_installer

package disk

import (
	"log/slog"
	"os"

	"golang.org/x/sys/unix"
)

func bootstrap() {
	// slog is not available yet (console fan-out comes later), so use raw writes.
	os.Stdout.WriteString("[ze-install] mounting /proc\n") //nolint:errcheck // pre-slog bootstrap
	os.MkdirAll("/proc", 0o755)                            //nolint:errcheck,gosec // PID-1 init; a mount point, and the mount below replaces its mode
	if err := unix.Mount("none", "/proc", "proc", 0, ""); err != nil {
		os.Stdout.WriteString("[ze-install] WARNING: mount /proc failed\n") //nolint:errcheck // pre-slog
	}

	os.Stdout.WriteString("[ze-install] mounting /sys\n") //nolint:errcheck // pre-slog bootstrap
	os.MkdirAll("/sys", 0o755)                            //nolint:errcheck,gosec // PID-1 init; a mount point, and the mount below replaces its mode
	if err := unix.Mount("none", "/sys", "sysfs", 0, ""); err != nil {
		os.Stdout.WriteString("[ze-install] WARNING: mount /sys failed\n") //nolint:errcheck // pre-slog
	}

	os.Stdout.WriteString("[ze-install] mounting /dev\n") //nolint:errcheck // pre-slog bootstrap
	os.MkdirAll("/dev", 0o755)                            //nolint:errcheck,gosec // PID-1 init; a mount point, and the mount below replaces its mode
	if err := unix.Mount("none", "/dev", "devtmpfs", 0, ""); err != nil {
		os.Stdout.WriteString("[ze-install] WARNING: mount /dev failed\n") //nolint:errcheck // pre-slog
	}

	os.MkdirAll("/tmp", 0o1777)     //nolint:errcheck,gosec // PID-1 init; /tmp is sticky and world-writable by convention
	os.MkdirAll("/mnt/perm", 0o750) //nolint:errcheck // PID-1 init; rootfs is writable
	os.MkdirAll("/mnt/iso", 0o750)  //nolint:errcheck // PID-1 init; rootfs is writable

	cw := setupConsoles()
	handler := slog.NewTextHandler(cw, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	slog.Info("bootstrap complete", "consoles", len(cw.writers))
}
