// Design: docs/architecture/appliance/on-device-installer.md -- ze install disk entry point

package disk

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/fetch"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	sourceHTTP          = "http"
	sourceISO           = "iso"
	defaultPartPollWait = 10 * time.Second
)

// codeStorageNotGrown is the doctor code of persistent storage smaller than
// its disk, and fixStorageGrow the command that grows it on the booted
// system. The installer is PID 1 of the initrd and links no component, so it
// spells the code that the storage doctor check declares instead of importing
// it.
const (
	codeStorageNotGrown = "doctor-storage-not-grown"
	fixStorageGrow      = "request system storage grow"
)

var partPollInterval = 200 * time.Millisecond

// Run is the entry point for `ze install disk`. It reads the kernel cmdline,
// validates inputs, and performs the install (HTTP download or ISO write).
// Returns 0 on success, 1 on error.
func Run(_ []string) int {
	slog.Info("ze install disk starting")

	cfg := parseCmdline()
	if err := validateConfig(cfg); err != nil {
		slog.Error("invalid cmdline", "error", err)
		return 1
	}

	switch cfg.Source {
	case sourceHTTP:
		return runHTTP(cfg)
	case sourceISO:
		return runISO(cfg)
	default:
		slog.Error("unsupported source", "source", cfg.Source)
		return 1
	}
}

func validateConfig(cfg installConfig) error {
	if err := validateSource(cfg.Source); err != nil {
		return err
	}

	if cfg.Source == sourceHTTP {
		if cfg.Server == "" {
			return fmt.Errorf("ze.server not set on kernel cmdline")
		}
		if err := validateIPv4(cfg.Server); err != nil {
			return err
		}
		if err := validatePort(cfg.Port); err != nil {
			return err
		}
	}

	if err := validateImageName(cfg.Image); err != nil {
		return err
	}

	if err := validateDecimal(cfg.Wait); err != nil {
		return fmt.Errorf("ze.wait: %w", err)
	}

	if cfg.Target != "" {
		if err := validateTargetPath(cfg.Target); err != nil {
			return err
		}
	}

	if cfg.Source == sourceISO {
		if cfg.MediaID == "" {
			return fmt.Errorf("ze.media-id required for ISO source mode")
		}
		if err := validateMediaID(cfg.MediaID); err != nil {
			return err
		}
	}

	if cfg.RescueAuth != "" {
		if err := validateRescueAuth(cfg.RescueAuth); err != nil {
			return fmt.Errorf("ze.rescue-auth: %w", err)
		}
	}

	return nil
}

func runHTTP(cfg installConfig) int {
	var tb textbuf.Buffer
	baseURL := tb.Str("http://").Str(cfg.Server).Byte(':').Str(cfg.Port).String()

	if err := ensureNetwork(cfg.Server, cfg.Port, cfg.Mac, parseWait(cfg.Wait)); err != nil {
		slog.Error("network setup failed", "error", err)
		return 1
	}

	disk, err := selectTargetDisk(cfg.Target, nil, "")
	if err != nil {
		slog.Error("disk detection", "error", err)
		return 1
	}
	slog.Info("target disk", "disk", disk)

	shaURL := tb.Reset().Str(baseURL).Str("/install/image/").Str(cfg.Image).Str(".sha256").String()
	expectedSHA := ""
	shaFile, tmpErr := os.CreateTemp("", "ze-image-*.sha256")
	if tmpErr == nil {
		shaPath := shaFile.Name()
		shaFile.Close() //nolint:errcheck // will be overwritten by download
		defer os.Remove(shaPath)
		if dlErr := fetch.ToFile(shaURL, shaPath, ""); dlErr == nil {
			data, readErr := os.ReadFile(shaPath) //nolint:gosec // temp file we just created
			if readErr == nil {
				expectedSHA = extractSHA(string(data))
			}
		}
	}
	if expectedSHA == "" {
		slog.Warn("no image checksum available, skipping verification")
	} else {
		slog.Info("expected image sha256", "sha", expectedSHA)
	}

	imageURL := tb.Reset().Str(baseURL).Str("/install/image/").Str(cfg.Image).String()
	slog.Info("streaming image to disk", "url", redact.URL(imageURL), "disk", disk)
	if err := writeHTTPImage(imageURL, disk, expectedSHA); err != nil {
		slog.Error("image write failed", "error", err)
		return 1
	}
	syncFS()
	// The table is extended before the kernel rereads it, so the node of
	// partition 4 the install waits for below already has the grown size.
	growInstalledTable(disk)
	if err := blkRereadPart(disk); err != nil {
		slog.Warn("BLKRRPART failed", "error", err)
	}
	slog.Info("image written, partition table re-read")

	part4 := partitionPath(disk, 4)
	slog.Info("waiting for partition node", "partition", part4, "timeout", defaultPartPollWait)
	if err := waitForPartitionNode(part4, defaultPartPollWait); err != nil {
		slog.Error("partition not found", "partition", part4, "error", err)
		return 1
	}
	slog.Info("injecting database", "partition", part4)
	if err := mountInjectDB(part4, tb.Reset().Str(baseURL).String(), permMount); err != nil {
		slog.Error("database injection failed", "error", err)
		return 1
	}

	slog.Info("installation complete, rebooting")
	doReboot()
	return 0
}

func runISO(cfg installConfig) int {
	media, err := locateISOMedia(cfg.Image, cfg.MediaID)
	if err != nil {
		slog.Error("ISO media not found", "error", err)
		return 1
	}
	slog.Info("ISO media found", "device", media.sourceDev)

	sourceDisks := []string{media.sourceDisk}
	if media.ventoyDisk != "" {
		sourceDisks = append(sourceDisks, media.ventoyDisk)
	}

	disk, err := selectTargetDisk(cfg.Target, sourceDisks, "")
	if err != nil {
		slog.Error("disk detection", "error", err)
		return 1
	}
	slog.Info("target disk", "disk", disk)

	var tb textbuf.Buffer
	imagePath := tb.Str(isoMount).Str("/ze-install/images/").Str(cfg.Image).String()
	checksumPath := tb.Reset().Str(imagePath).Str(".sha256").String()

	if err := writeISOImage(imagePath, checksumPath, disk); err != nil {
		slog.Error("image write failed", "error", err)
		_ = umountFS(isoMount)
		return 1
	}

	// The image is flushed before the table is extended, and the table is
	// extended before finishISOWrite asks the kernel to reread it.
	syncFS()
	growInstalledTable(disk)
	if err := finishISOWrite(disk); err != nil {
		slog.Error("ISO image finalization failed", "disk", disk, "error", err)
		_ = umountFS(isoMount)
		return 1
	}
	// The ISO install has no seed to write, so this mount of the persistent
	// partition exists only to grow its filesystem.
	if err := growISOPerm(disk); err != nil {
		reportStorageNotGrown("filesystem", disk, err)
	}
	_ = umountFS(isoMount)
	slog.Info("ISO installation complete, powering off")
	doPoweroff()
	return 0
}

// findInstallerISO finds the installer medium: the ISO itself first, then an
// ISO file on a Ventoy drive.
func findInstallerISO(image, mediaID string) (*isoMedia, error) {
	media, err := findISOMedia(image, mediaID)
	if err == nil {
		return media, nil
	}
	slog.Info("direct ISO not found, scanning Ventoy partitions")
	return tryVentoyISO(image, mediaID)
}

// growInstalledTable extends the partition table of the disk the install has
// just written, so that the persistent partition reaches the end of the disk.
// A failure is logged and survived: the written system is complete on its
// image-sized partition, and stopping here would leave a machine that holds a
// whole system in the recovery console.
func growInstalledTable(disk string) {
	if err := growPermTable(disk); err != nil {
		reportStorageNotGrown("partition table", disk, err)
	}
}

// growInstalledFS grows the filesystem of the persistent partition, which the
// caller MUST have mounted on mountPoint. A failure is logged and survived,
// for the reason growInstalledTable gives.
func growInstalledFS(mountPoint, partition string) {
	if err := growPermFS(mountPoint, partition); err != nil {
		reportStorageNotGrown("filesystem", partition, err)
		return
	}
	syncFS()
}

// growISOPerm mounts the persistent partition of disk, grows its filesystem
// and unmounts it. It answers the first step that failed; the unmount runs
// whenever the mount succeeded.
func growISOPerm(disk string) error {
	part4 := partitionPath(disk, 4)
	if err := waitForPartitionNode(part4, defaultPartPollWait); err != nil {
		return err
	}
	if err := os.MkdirAll(permMount, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", permMount, err)
	}
	if err := mountFS(part4, permMount, "ext4", false); err != nil {
		return fmt.Errorf("mount %s: %w", part4, err)
	}
	growErr := growPermFS(permMount, part4)
	if growErr == nil {
		syncFS()
	}
	if err := umountFS(permMount); err != nil {
		return errors.Join(growErr, fmt.Errorf("unmount %s after the resize: %w", permMount, err))
	}
	return growErr
}

// reportStorageNotGrown logs one growth step that failed. step names the step
// (the partition table or the filesystem) and device the disk or partition it
// worked on.
func reportStorageNotGrown(step, device string, err error) {
	slog.Error("persistent storage was not grown to the disk: the installed system is complete and keeps the partition size of the image",
		"step", step, "device", device, "error", err, "code", codeStorageNotGrown, "fix", fixStorageGrow)
}

// finishISOWrite reports the shared write-completion marker only after the
// bytes are synchronized and the kernel has accepted the new partition table.
func finishISOWrite(disk string) error {
	syncFS()
	if err := blkRereadPart(disk); err != nil {
		return fmt.Errorf("reread installed ISO partition table on %s: %w", disk, err)
	}
	slog.Info("image written, partition table re-read")
	return nil
}

// waitForPartitionNode polls for a block device node to appear after
// BLKRRPART. devtmpfs creates nodes asynchronously; mounting immediately
// after re-reading the partition table races (ISSUE-2, mirroring
// tools/installer-initrd/init:1122-1127).
func waitForPartitionNode(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("partition %s not found after %v", path, timeout)
		}
		time.Sleep(partPollInterval)
	}
}

func extractSHA(s string) string {
	for i, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			s = s[:i]
			break
		}
	}
	if validateSHA256(s) != nil {
		return ""
	}
	return s
}
