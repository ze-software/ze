// Design: docs/architecture/zefs-format.md -- in-place write via pwrite
// Overview: store.go -- flush() uses pwriteRegions for in-place updates

//go:build unix

package zefs

import (
	"fmt"
	"os"
	"syscall"
)

type writeRegion struct {
	offset int
	data   []byte
}

func pwriteRegions(path string, regions []writeRegion) error {
	fd, err := os.OpenFile(path, os.O_WRONLY, 0) // #nosec G304 -- path is the store's own file
	if err != nil {
		return fmt.Errorf("zefs: pwrite open: %w", err)
	}
	defer fd.Close() //nolint:errcheck // best-effort close after sync

	for _, r := range regions {
		if _, err := syscall.Pwrite(int(fd.Fd()), r.data, int64(r.offset)); err != nil {
			return fmt.Errorf("zefs: pwrite at %d: %w", r.offset, err)
		}
	}

	if err := fd.Sync(); err != nil {
		return fmt.Errorf("zefs: pwrite fsync: %w", err)
	}
	return nil
}
