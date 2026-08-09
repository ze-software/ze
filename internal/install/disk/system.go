// Design: docs/architecture/appliance/on-device-installer.md -- Linux-specific install operations

package disk

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// mountFS, umountFS, syncFS, rebootFS, poweroffFS are the syscall-level
// operations. On Linux they are replaced by init() in blockdev_linux.go
// and mount_linux.go with direct syscalls; these defaults exist only so
// the package compiles on non-Linux (development) targets.
var (
	mountFS = func(source, target, fstype string, readOnly bool) error {
		return fmt.Errorf("mount: not supported on this platform")
	}
	umountFS          = func(target string) error { return fmt.Errorf("umount: not supported on this platform") }
	syncFS            = func() {}
	rebootFS          = func() { os.WriteFile("/proc/sysrq-trigger", []byte("b"), 0o200) } //nolint:gosec,errcheck // fallback
	poweroffFS        = func() { os.WriteFile("/proc/sysrq-trigger", []byte("o"), 0o200) } //nolint:gosec,errcheck // fallback
	blkRereadPart     = func(string) error { return fmt.Errorf("BLKRRPART: not supported on this platform") }
	loopAttach        = func(string) (string, error) { return "", fmt.Errorf("loop: not supported on this platform") }
	loopDetach        = func(string) {}
	ensureLoopDevices = func() {}
	linkUp            = func(string) error { return fmt.Errorf("linkUp: not supported on this platform") }
	dhcpAcquireApply  = func(string) error { return fmt.Errorf("dhcp: not supported on this platform") }
	flushIface        = func(string) error { return fmt.Errorf("flush: not supported on this platform") }
)

// mountInjectDB mounts partition p4, downloads and writes database.zefs,
// then unmounts.
func mountInjectDB(part4, baseURL string) error {
	mountPoint := "/mnt/perm"
	if err := os.MkdirAll(mountPoint, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", mountPoint, err)
	}

	if err := mountFS(part4, mountPoint, "ext4", false); err != nil {
		return fmt.Errorf("mount %s: %w", part4, err)
	}
	defer func() {
		if umountErr := umountFS(mountPoint); umountErr != nil {
			slog.Warn("umount failed", "path", mountPoint, "error", umountErr)
		}
	}()

	var tb textbuf.Buffer
	zeDir := tb.Str(mountPoint).Str("/ze").String()
	if err := os.MkdirAll(zeDir, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", zeDir, err)
	}

	dbDest := tb.Reset().Str(zeDir).Str("/database.zefs").String()

	// Preserve a seed the image already carries. `ze appliance build` bakes the
	// full appliance seed -- the SSH listener on the configured host/port, the
	// ze.conf template, web certificates -- into /perm/ze/database.zefs. The
	// image server's /install/database.zefs is a first-boot BOOTSTRAP database
	// (SSH on 127.0.0.1:2222, no config) meant for a seedless image. Downloading
	// it over a provisioned seed replaces the appliance's real configuration
	// with a localhost-only login, stranding the box unreachable on the network
	// -- the regression that made a provisioned box unSSHable after install.
	// Only fetch the bootstrap database when the image shipped without a seed.
	if bakedSeedPresent(dbDest) {
		slog.Info("keeping seed baked into image, skipping bootstrap database", "path", dbDest)
		syncFS()
		return nil
	}

	dbURL := tb.Reset().Str(baseURL).Str("/install/database.zefs").String()
	if err := downloadToFile(dbURL, dbDest); err != nil {
		return fmt.Errorf("download database.zefs: %w", err)
	}

	slog.Info("bootstrap database injected", "path", dbDest)
	syncFS()
	return nil
}

// bakedSeedPresent reports whether the image already shipped a non-empty
// ze/database.zefs seed in its /perm partition. A zero-length file counts as
// absent so a truncated or failed bake still falls back to the bootstrap
// database rather than leaving the box with an unusable seed.
func bakedSeedPresent(dbPath string) bool {
	info, err := os.Stat(dbPath)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func doReboot() {
	slog.Info("rebooting")
	rebootFS()
}

func doPoweroff() {
	slog.Info("powering off")
	poweroffFS()
}
