// Design: docs/architecture/appliance/on-device-installer.md -- ze install disk entry point

package disk

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	sourceHTTP          = "http"
	sourceISO           = "iso"
	defaultPartPollWait = 10 * time.Second
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

	disk, err := findTargetDisk(cfg.Target, nil, "")
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
		if dlErr := downloadToFile(shaURL, shaPath); dlErr == nil {
			data, readErr := os.ReadFile(shaPath) //nolint:gosec // temp file we just created
			if readErr == nil {
				expectedSHA = extractSHA(string(data))
			}
		}
	}
	if expectedSHA != "" {
		slog.Info("expected image sha256", "sha", expectedSHA)
	} else {
		slog.Warn("no image checksum available, skipping verification")
	}

	imageURL := tb.Reset().Str(baseURL).Str("/install/image/").Str(cfg.Image).String()
	slog.Info("streaming image to disk", "url", imageURL, "disk", disk)
	if err := downloadToDisk(imageURL, disk, expectedSHA); err != nil {
		slog.Error("image write failed", "error", err)
		return 1
	}
	syncFS()
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
	if err := mountInjectDB(part4, tb.Reset().Str(baseURL).String()); err != nil {
		slog.Error("database injection failed", "error", err)
		return 1
	}

	slog.Info("installation complete, rebooting")
	doReboot()
	return 0
}

func runISO(cfg installConfig) int {
	media, err := findISOMedia(cfg.Image, cfg.MediaID)
	if err != nil {
		slog.Info("direct ISO not found, scanning Ventoy partitions")
		media, err = tryVentoyISO(cfg.Image, cfg.MediaID)
	}
	if err != nil {
		slog.Error("ISO media not found", "error", err)
		return 1
	}
	slog.Info("ISO media found", "device", media.sourceDev)

	sourceDisks := []string{media.sourceDisk}
	if media.ventoyDisk != "" {
		sourceDisks = append(sourceDisks, media.ventoyDisk)
	}

	disk, err := findTargetDisk(cfg.Target, sourceDisks, "")
	if err != nil {
		slog.Error("disk detection", "error", err)
		return 1
	}
	slog.Info("target disk", "disk", disk)

	var tb textbuf.Buffer
	imagePath := tb.Str(isoMount).Str("/ze-install/images/").Str(cfg.Image).String()
	checksumPath := tb.Reset().Str(imagePath).Str(".sha256").String()

	if err := localImageToDisk(imagePath, checksumPath, disk); err != nil {
		slog.Error("image write failed", "error", err)
		_ = umountFS(isoMount)
		return 1
	}

	syncFS()
	_ = umountFS(isoMount)
	slog.Info("ISO installation complete, powering off")
	doPoweroff()
	return 0
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
