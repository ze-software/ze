// Design: docs/architecture/doctor-and-health-checks.md -- usable disk accounting.

//go:build !windows

package support

import (
	"math"
	"testing"

	"github.com/ze-software/ze/internal/core/diskspace"
)

// TestDiskUsageFromBlocks checks the JSON producer's signed availability and
// both byte and percentage multiplication boundaries without a real full disk.
func TestDiskUsageFromBlocks(t *testing.T) {
	for _, tc := range []struct {
		name        string
		blocks      uint64
		available   int64
		blockSize   uint64
		total, free uint64
		usedPct     int
		wantErr     bool
	}{
		{"negative", 100, -1, 4096, 409600, 0, 100, false},
		{"zero", 100, 0, 4096, 409600, 0, 100, false},
		{"five percent", 100, 5, 4096, 409600, 20480, 95, false},
		{"normal", 100, 75, 4096, 409600, 307200, 25, false},
		{"empty", 0, 0, 4096, 0, 0, 0, false},
		{"above total", 100, 101, 4096, 409600, 409600, 0, false},
		{"percentage overflow", math.MaxUint64, 0, 1, math.MaxUint64, 0, 100, false},
		{"byte boundary", math.MaxUint64 / 4096, 0, 4096, math.MaxUint64 - 4095, 0, 100, false},
		{"byte overflow", math.MaxUint64/4096 + 1, 0, 4096, 0, 0, 0, true},
		{"invalid block size", 100, 75, 0, 0, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := diskUsageFromBlocks("disk", tc.blocks, diskspace.UsableBlocks(tc.available), tc.blockSize)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantErr {
				if got != (diskUsage{}) {
					t.Errorf("failed accounting returned capacity: %+v", got)
				}
				return
			}
			want := diskUsage{Path: "disk", TotalBytes: tc.total, FreeBytes: tc.free,
				UsedBytes: tc.total - tc.free, UsedPct: tc.usedPct}
			if got != want {
				t.Errorf("usage = %+v, want %+v", got, want)
			}
		})
	}
}

// TestCollectDiskInfoCapacity exercises the production statfs-to-JSON path.
func TestCollectDiskInfoCapacity(t *testing.T) {
	got, err := collectDiskInfo()
	if err != nil {
		t.Fatal(err)
	}
	result, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("disk info = %T", got)
	}
	filesystems, ok := result["filesystems"].([]diskUsage)
	if !ok {
		t.Fatalf("filesystems = %T", result["filesystems"])
	}
	if len(filesystems) == 0 {
		t.Fatal("root filesystem not measured")
	}
	for _, usage := range filesystems {
		if usage.FreeBytes > usage.TotalBytes || usage.UsedBytes != usage.TotalBytes-usage.FreeBytes {
			t.Errorf("inconsistent capacity: %+v", usage)
		}
		if usage.UsedPct < 0 || usage.UsedPct > 100 {
			t.Errorf("invalid percentage: %+v", usage)
		}
	}
}
