// Design: plan/spec-appliance-install-robust.md -- Linux-specific install operations

package disk

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// mountInjectDB mounts partition p4, downloads and writes database.zefs,
// then unmounts. On a real device this uses mount/umount; the partition
// is a real block device so standard filesystem I/O works (no ext4 library).
func mountInjectDB(part4, baseURL string) error {
	mountPoint := "/mnt/perm"
	if err := os.MkdirAll(mountPoint, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", mountPoint, err)
	}

	if err := runCmd("mount", "-t", "ext4", part4, mountPoint); err != nil {
		return fmt.Errorf("mount %s: %w", part4, err)
	}
	defer func() {
		if umountErr := runCmd("umount", mountPoint); umountErr != nil {
			slog.Warn("umount failed", "path", mountPoint, "error", umountErr)
		}
	}()

	var tb textbuf.Buffer
	zeDir := tb.Str(mountPoint).Str("/ze").String()
	if err := os.MkdirAll(zeDir, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", zeDir, err)
	}

	dbDest := tb.Reset().Str(zeDir).Str("/database.zefs").String()
	dbURL := tb.Reset().Str(baseURL).Str("/install/database.zefs").String()
	if err := downloadToFile(dbURL, dbDest); err != nil {
		return fmt.Errorf("download database.zefs: %w", err)
	}

	slog.Info("database injected", "path", dbDest)
	return runCmd("sync")
}

func doReboot() {
	slog.Info("rebooting")
	if err := runCmd("reboot", "-f"); err != nil {
		slog.Warn("reboot failed, trying sysrq", "error", err)
		os.WriteFile("/proc/sysrq-trigger", []byte("b"), 0o200) //nolint:gosec,errcheck // last resort
	}
}

func doPoweroff() {
	slog.Info("powering off")
	if err := runCmd("poweroff", "-f"); err != nil {
		slog.Warn("poweroff failed, trying sysrq", "error", err)
		os.WriteFile("/proc/sysrq-trigger", []byte("o"), 0o200) //nolint:gosec,errcheck // last resort
	}
}

func runCmd(name string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), name, args...) //nolint:gosec // controlled invocation
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
