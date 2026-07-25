// Design: plan/learned/907-appliance-install-robust.md -- disk detection for on-device installer

package disk

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// diskNameFromPath strips /dev/ prefix and partition suffix to get the
// whole-disk name. Handles NVMe (nvme0n1p4), eMMC (mmcblk0p1), and
// SCSI/virtio (sda1, vda3).
func diskNameFromPath(path string) string {
	name := strings.TrimPrefix(path, "/dev/")

	switch {
	case strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "mmcblk"):
		if idx := strings.LastIndex(name, "p"); idx > 0 {
			suffix := name[idx+1:]
			allDigits := suffix != ""
			for _, c := range suffix {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return name[:idx]
			}
		}
		return name
	default:
		for name != "" && name[len(name)-1] >= '0' && name[len(name)-1] <= '9' {
			name = name[:len(name)-1]
		}
		return name
	}
}

// partitionPath returns the device path for partition num on disk.
// NVMe and eMMC use the "p" separator (nvme0n1p4); others append directly (sda4).
func partitionPath(disk string, num int) string {
	name := strings.TrimPrefix(disk, "/dev/")
	var tb textbuf.Buffer
	if strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "mmcblk") {
		return tb.Str(disk).Byte('p').Int(int64(num)).String()
	}
	return tb.Str(disk).Int(int64(num)).String()
}

// findTargetDisk scans /sys/block for a suitable non-removable target disk.
// When rejectMultiple is true (ISO mode), errors on ambiguity.
// When false (HTTP mode), picks the first match (parity with shell).
func findTargetDisk(explicit string, sourceDisks []string, rejectMultiple bool) (string, error) {
	if explicit != "" {
		if err := validateTargetPath(explicit); err != nil {
			return "", err
		}
		name := diskNameFromPath(explicit)
		if slices.Contains(sourceDisks, name) {
			return "", fmt.Errorf("ze.target %q is the ISO source media", explicit)
		}
		return explicit, nil
	}

	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return "", fmt.Errorf("read /sys/block: %w", err)
	}

	var candidates []string
	for _, entry := range entries {
		name := entry.Name()
		if isSkippedDisk(name) {
			continue
		}
		if slices.Contains(sourceDisks, name) {
			continue
		}
		var tbR textbuf.Buffer
		removablePath := tbR.Str("/sys/block/").Str(name).Str("/removable").String()
		if data, readErr := os.ReadFile(removablePath); readErr == nil { //nolint:gosec // sysfs path
			if strings.TrimSpace(string(data)) == "1" {
				continue
			}
		}
		candidates = append(candidates, name)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no non-removable block device found")
	}

	if rejectMultiple && len(candidates) > 1 {
		return "", fmt.Errorf("multiple target disks found (%v); set ze.target on the kernel cmdline", candidates)
	}

	var tb textbuf.Buffer
	return tb.Str("/dev/").Str(candidates[0]).String(), nil
}
