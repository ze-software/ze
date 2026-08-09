// Design: docs/architecture/appliance/on-device-installer.md -- ISO media detection and local image write

package disk

import (
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	isoMount        = "/mnt/iso"
	ventoyDataMount = "/mnt/ventoy"
)

type isoMedia struct {
	sourceDev  string
	sourceDisk string
	ventoyDisk string
	mountPoint string
}

// findISOMedia scans block devices for a mounted ISO9660 filesystem
// containing the ze-install directory with a matching media-id.
func findISOMedia(image, mediaID string) (*isoMedia, error) {
	if err := os.MkdirAll(isoMount, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", isoMount, err)
	}

	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, fmt.Errorf("read /sys/block: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if isSkippedDisk(name) {
			continue
		}

		nodes := blockDevNodes(name)
		for _, node := range nodes {
			if !isBlockDevice(node) {
				continue
			}
			if err := mountFS(node, isoMount, "iso9660", true); err != nil {
				continue
			}

			if m := checkISOContent(node, name, image, mediaID); m != nil {
				return m, nil
			}
			_ = umountFS(isoMount)
		}
	}

	return nil, fmt.Errorf("no ze installer ISO with media-id %s found", mediaID)
}

// tryVentoyISO scans FAT/exFAT partitions for ISO files on Ventoy-style
// multi-boot USB drives, loop-mounting each candidate.
func tryVentoyISO(image, mediaID string) (*isoMedia, error) {
	ensureLoopDevices()
	if err := os.MkdirAll(ventoyDataMount, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", ventoyDataMount, err)
	}

	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, fmt.Errorf("read /sys/block: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if isSkippedDisk(name) {
			continue
		}

		nodes := blockDevNodes(name)
		for _, node := range nodes {
			if !isBlockDevice(node) {
				continue
			}

			mounted := false
			for _, fs := range []string{"vfat", "exfat"} {
				if mountFS(node, ventoyDataMount, fs, true) == nil {
					mounted = true
					break
				}
			}
			if !mounted {
				continue
			}

			if m := scanVentoyISOs(node, name, image, mediaID); m != nil {
				return m, nil
			}
			_ = umountFS(ventoyDataMount)
		}
	}

	return nil, fmt.Errorf("no Ventoy ISO with media-id %s found", mediaID)
}

func scanVentoyISOs(node, diskName, image, mediaID string) *isoMedia {
	patterns := []string{
		filepath.Join(ventoyDataMount, "*.iso"),
		filepath.Join(ventoyDataMount, "*", "*.iso"),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, iso := range matches {
			loopDev, loopErr := loopAttach(iso)
			if loopErr != nil {
				continue
			}
			if mountFS(loopDev, isoMount, "iso9660", true) != nil {
				loopDetach(loopDev)
				continue
			}
			if m := checkISOContent(loopDev, diskName, image, mediaID); m != nil {
				m.ventoyDisk = diskName
				slog.Info("found installer ISO via Ventoy", "partition", node, "loop", loopDev)
				return m
			}
			_ = umountFS(isoMount)
			loopDetach(loopDev)
		}
	}
	return nil
}

func checkISOContent(dev, _, image, mediaID string) *isoMedia {
	var tb textbuf.Buffer
	manifestPath := tb.Str(isoMount).Str("/ze-install/manifest.json").String()
	imagePath := tb.Reset().Str(isoMount).Str("/ze-install/images/").Str(image).String()
	mediaIDPath := tb.Reset().Str(isoMount).Str("/ze-install/media-id").String()

	if !fileExists(manifestPath) || !fileExists(imagePath) || !fileExists(mediaIDPath) {
		return nil
	}

	data, err := os.ReadFile(mediaIDPath) //nolint:gosec // ISO mount path
	if err != nil {
		return nil
	}
	candidate := strings.TrimSpace(string(data))
	if candidate != mediaID {
		return nil
	}

	return &isoMedia{
		sourceDev:  dev,
		sourceDisk: diskNameFromPath(dev),
		mountPoint: isoMount,
	}
}

// localImageToDisk verifies the checksum of a local image and writes it
// to disk, handling both raw and gzip-compressed images.
func localImageToDisk(imagePath, checksumPath, disk string) error {
	expectedSHA, err := readExpectedSHA(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksum: %w", err)
	}

	actualSHA, err := fileSHA256(imagePath)
	if err != nil {
		return fmt.Errorf("hash image: %w", err)
	}
	if actualSHA != expectedSHA {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actualSHA, expectedSHA)
	}
	slog.Info("local image checksum verified", "sha256", expectedSHA)

	if strings.HasSuffix(imagePath, ".gz") {
		return decompressGzToDisk(imagePath, disk)
	}
	return copyFileToDisk(imagePath, disk)
}

func readExpectedSHA(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // build-controlled path
	if err != nil {
		return "", err
	}
	sha := extractSHA(string(data))
	if sha == "" {
		return "", fmt.Errorf("no valid SHA-256 in %s", path)
	}
	return sha, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // build-controlled path
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return textbuf.StringHex(h.Sum(nil)), nil
}

func decompressGzToDisk(src, disk string) error {
	slog.Info("decompressing gzip image to disk", "src", src, "disk", disk)
	in, err := os.Open(src) //nolint:gosec // build-controlled path
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close() //nolint:errcheck // read-only

	gz, err := gzip.NewReader(in)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close() //nolint:errcheck // read-only

	out, err := os.OpenFile(disk, os.O_WRONLY, 0) //nolint:gosec // validated disk path
	if err != nil {
		return fmt.Errorf("open %s: %w", disk, err)
	}
	outClosed := false
	defer func() {
		if !outClosed {
			out.Close() //nolint:errcheck // cleanup on error path
		}
	}()

	if _, err := io.Copy(out, gz); err != nil { //nolint:gosec // installer writes multi-GB disk images; no bomb risk
		return fmt.Errorf("decompress to %s: %w", disk, err)
	}
	outClosed = true
	return out.Close()
}

func copyFileToDisk(src, disk string) error {
	in, err := os.Open(src) //nolint:gosec // build-controlled path
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close() //nolint:errcheck // read-only

	out, err := os.OpenFile(disk, os.O_WRONLY, 0) //nolint:gosec // validated disk path
	if err != nil {
		return fmt.Errorf("open %s: %w", disk, err)
	}
	outClosed := false
	defer func() {
		if !outClosed {
			out.Close() //nolint:errcheck // cleanup on error path
		}
	}()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy to %s: %w", disk, err)
	}
	outClosed = true
	return out.Close()
}

func blockDevNodes(name string) []string {
	var tb textbuf.Buffer
	nodes := []string{tb.Str("/dev/").Str(name).String()}

	pattern := tb.Reset().Str("/dev/").Str(name).Str("[0-9]*").String()
	if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
		nodes = append(nodes, matches...)
	}
	pattern = tb.Reset().Str("/dev/").Str(name).Str("p[0-9]*").String()
	if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
		nodes = append(nodes, matches...)
	}
	return nodes
}

func isBlockDevice(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeDevice != 0
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
