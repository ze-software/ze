// Design: docs/architecture/core-design.md — disk usage for support bundle

//go:build !windows

package support

import (
	"syscall"

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

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		var pct int
		if total > 0 {
			pct = int(used * 100 / total)
		}
		results = append(results, diskUsage{
			Path:       path,
			TotalBytes: total,
			FreeBytes:  free,
			UsedBytes:  used,
			UsedPct:    pct,
		})
	}

	return map[string]any{"filesystems": results}, nil
}
