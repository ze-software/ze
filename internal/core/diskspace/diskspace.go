// Package diskspace answers how much room a filesystem has left.
//
// One declaration, because the question has been asked in two places for two
// reasons. `le scratch cache-clean` REPORTS the space a cache returned, and the
// kernel build GUARDS on the space it is about to need
// (internal/appliance/kernelbuilder). A second copy of statfs would be a second
// answer to one question.
//
// The cost of not asking is recorded: a full disk was read as a code defect six
// times (plan/journal/full-disk-false-red.md), and on the sixth it also
// corrupted the container runtime's image store, because the sparse file
// backing that runtime could no longer grow.
package diskspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// BytesPerGiB is the unit a disk is discussed in.
const BytesPerGiB = 1024 * 1024 * 1024

// Free answers the bytes available on the filesystem that holds path.
//
// It walks up to the first path that EXISTS, so a directory the caller has not
// created yet still reports its device rather than an error. That matters for a
// cache the toolchain creates on first use, and for a build output directory
// created after the guard that protects it.
//
// Safe for concurrent use.
func Free(path string) (uint64, error) {
	probe := path
	for {
		var stat unix.Statfs_t
		err := unix.Statfs(probe, &stat)
		if err == nil {
			return stat.Bavail * uint64(stat.Bsize), nil //nolint:gosec // Statfs_t.Bsize is uint32 on darwin and int64 on linux, and a block size is positive on both
		}
		if !errors.Is(err, os.ErrNotExist) {
			return 0, fmt.Errorf("statfs %s: %w", probe, err)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return 0, fmt.Errorf("statfs %s: %w", probe, err)
		}
		probe = parent
	}
}

// GiB renders a byte count in whole tenths of a gibibyte, for a message an
// operator reads next to their own `df` output.
func GiB(bytes uint64) string {
	return fmt.Sprintf("%.1fG", float64(bytes)/BytesPerGiB)
}
