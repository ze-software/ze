// Design: docs/architecture/doctor-and-health-checks.md -- usable disk accounting.

package diskspace_test

import (
	"math"
	"testing"

	"github.com/ze-software/ze/internal/core/diskspace"
)

// TestUsableBlocks exercises signed kernel counters before their unsigned conversion.
func TestUsableBlocks(t *testing.T) {
	for _, blocks := range []int64{math.MinInt64, -1, 0, 1, 50, math.MaxInt64} {
		want := uint64(0)
		if blocks > 0 {
			want = uint64(blocks)
		}
		if got := diskspace.UsableBlocks(blocks); got != want {
			t.Errorf("UsableBlocks(%d) = %d, want %d", blocks, got, want)
		}
	}
	if got := diskspace.UsableBlocks(uint64(math.MaxUint64)); got != math.MaxUint64 {
		t.Errorf("unsigned maximum = %d", got)
	}
}

// TestPercent pins floor rounding and multiplication overflow boundaries.
func TestPercent(t *testing.T) {
	for _, tc := range []struct {
		part, total, want uint64
	}{
		{0, 0, 0}, {0, 100, 0}, {4, 100, 4}, {5, 100, 5},
		{49, 1000, 4}, {50, 1000, 5}, {75, 100, 75},
		{101, 100, 100}, {math.MaxUint64, math.MaxUint64, 100},
		{math.MaxUint64 - 1, math.MaxUint64, 99},
		{math.MaxUint64 / 20, math.MaxUint64, 4},
		{math.MaxUint64/20 + 1, math.MaxUint64, 5},
	} {
		if got := diskspace.Percent(tc.part, tc.total); got != tc.want {
			t.Errorf("Percent(%d, %d) = %d, want %d", tc.part, tc.total, got, tc.want)
		}
	}
}

// TestBytes rejects unrepresentable capacity rather than returning wrapped free space.
func TestBytes(t *testing.T) {
	for _, tc := range []struct {
		blocks, blockSize, want uint64
		wantErr                 bool
	}{
		{0, 4096, 0, false}, {50, 4096, 204800, false},
		{math.MaxUint64, 1, math.MaxUint64, false},
		{math.MaxUint64 / 4096, 4096, math.MaxUint64 - 4095, false},
		{math.MaxUint64/4096 + 1, 4096, 0, true},
		{math.MaxUint64, 2, 0, true}, {1, 0, 0, true},
	} {
		got, err := diskspace.Bytes(tc.blocks, tc.blockSize)
		if (err != nil) != tc.wantErr {
			t.Errorf("Bytes(%d, %d) error = %v", tc.blocks, tc.blockSize, err)
		}
		if got != tc.want {
			t.Errorf("Bytes(%d, %d) = %d, want %d", tc.blocks, tc.blockSize, got, tc.want)
		}
	}
}
