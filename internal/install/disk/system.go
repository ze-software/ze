// Design: docs/architecture/appliance/on-device-installer.md -- Linux-specific install operations

package disk

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/fetch"
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

// growPermTable and growPermFS are the two growth steps of an install. On
// Linux init() in grow_linux.go replaces them with the calls into the
// partition-table and ext4 leaves; these defaults exist so the package
// compiles elsewhere, and they refuse rather than report a growth.
//
// growPermTable extends the partition table of disk so that its fourth
// partition ends at the last usable sector. growPermFS grows the ext4
// filesystem mounted on mountPoint to the size of partition. The caller MUST
// call growPermFS only while partition is mounted on mountPoint.
var (
	growPermTable = func(string) error {
		return fmt.Errorf("grow partition table: not supported on this platform")
	}
	growPermFS = func(_, _ string) error {
		return fmt.Errorf("grow filesystem: not supported on this platform")
	}
)

// The steps an install runs before the growth are package variables, so a
// test runs runHTTP and runISO whole and writes to no device. Production code
// never assigns them.
var (
	selectTargetDisk = findTargetDisk
	writeHTTPImage   = fetch.ToDisk
	writeISOImage    = localImageToDisk
	locateISOMedia   = findInstallerISO
)

// permMount is where the installer mounts the persistent partition of the
// disk it has just written. A variable so a test mounts it in its own folder.
var permMount = "/mnt/perm"

// mountInjectDB mounts partition p4 and preserves its live tree or baked seed.
// Only a seedless partition receives the image server's bootstrap blob. With
// the seed in place it grows the filesystem to the partition, inside the same
// mount, and a growth that fails is logged and survived (growInstalledFS).
func mountInjectDB(part4, baseURL, mountPoint string) error {
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

	if err := injectDB(baseURL, mountPoint); err != nil {
		return err
	}
	growInstalledFS(mountPoint, part4)
	return nil
}

// injectDB puts the seed database on the persistent partition mounted on
// mountPoint, unless the partition already holds a live store or a baked seed.
func injectDB(baseURL, mountPoint string) error {
	var tb textbuf.Buffer
	zeDir := tb.Str(mountPoint).Str("/ze").String()
	if err := os.MkdirAll(zeDir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", zeDir, err)
	}

	treePath := filepath.Join(zeDir, "database")
	if info, err := os.Lstat(treePath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("preserve store %s: expected a directory, found %s", treePath, info.Mode())
		}
		slog.Info("keeping existing live store, skipping bootstrap database", "path", treePath)
		syncFS()
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect store %s: %w", treePath, err)
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
	if err := fetch.ToFile(dbURL, dbDest, ""); err != nil {
		return fmt.Errorf("download database.zefs: %w", err)
	}

	slog.Info("bootstrap database injected", "path", dbDest)
	syncFS()
	return nil
}

// bakedSeedPresent accepts the live database directory or a non-empty regular
// seed blob. An empty blob remains eligible for bootstrap replacement.
func bakedSeedPresent(dbPath string) bool {
	info, err := os.Lstat(dbPath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return filepath.Base(dbPath) == "database"
	}
	return info.Mode().IsRegular() && info.Size() > 0
}

func doReboot() {
	slog.Info("rebooting")
	rebootFS()
}

func doPoweroff() {
	slog.Info("powering off")
	poweroffFS()
}
