// Design: docs/architecture/core-design.md — disk usage for support bundle

//go:build !windows

package support

import (
	"fmt"
	"syscall"

	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/paths"
)

type diskUsage struct {
	Path       string `json:"path"`
	TotalBytes uint64 `json:"total-bytes"`
	FreeBytes  uint64 `json:"free-bytes"`
	UsedBytes  uint64 `json:"used-bytes"`
	UsedPct    int    `json:"used-pct"`
}

func collectDiskInfo() (any, error) {
	targets := []string{"/"}
	if dir := paths.DefaultConfigDir(); dir != "" {
		targets = append(targets, dir)
	}

	results := make([]diskUsage, 0, len(targets))
	seen := make(map[string]bool)

	for _, path := range targets {
		if seen[path] {
			continue
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			continue
		}
		seen[path] = true

		usage, err := diskUsageFromBlocks(path, diskspace.UsableBlocks(stat.Blocks),
			diskspace.UsableBlocks(stat.Bavail), diskspace.UsableBlocks(stat.Bsize))
		if err != nil {
			return nil, fmt.Errorf("disk usage %s: %w", path, err)
		}
		results = append(results, usage)
	}

	return map[string]any{"filesystems": results}, nil
}

func diskUsageFromBlocks(path string, blocks, available, blockSize uint64) (diskUsage, error) {
	total, err := diskspace.Bytes(blocks, blockSize)
	if err != nil {
		return diskUsage{}, err
	}
	// Available blocks exclude reserved space; a signed deficit has already
	// become zero. Cap at the total before subtracting to prevent underflow.
	// Total bytes fit, so this product bounded by the same total fits too.
	free := min(available, blocks) * blockSize
	used := total - free
	return diskUsage{
		Path:       path,
		TotalBytes: total,
		FreeBytes:  free,
		UsedBytes:  used,
		UsedPct:    int(diskspace.Percent(used, total)),
	}, nil
}
