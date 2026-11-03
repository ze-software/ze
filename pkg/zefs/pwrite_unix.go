// Design: docs/architecture/zefs-format.md -- in-place write via pwrite
// Overview: store.go -- flush() uses pwriteRegions for in-place updates

//go:build unix

package zefs

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

type writeRegion struct {
	offset int
	data   []byte
}

// openBlobAlias preserves ordinary path constructors' final-symlink contract.
// It starts at their retained parent. Rooted constructors never call it.
func openBlobAlias(folder *os.Root, name string) (*os.File, error) {
	dir, err := folder.Open(".")
	if err != nil {
		return nil, err
	}
	defer dir.Close() //nolint:errcheck // Read-only directory handle.
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func openBlobFile(folder *os.Root, name string, flags int) (*os.File, error) {
	file, err := folder.OpenFile(name, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close() //nolint:errcheck // The stat failure is returned.
		return nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close() //nolint:errcheck // The unsafe file is refused.
		return nil, fmt.Errorf("zefs: %s is not a regular artifact: use a regular file", file.Name())
	}
	return file, nil
}

func pwriteRegions(folder *os.Root, name string, regions []writeRegion, confined bool) error {
	fd, err := openBlobFile(folder, name, os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("zefs: pwrite open: %w", err)
	}
	defer fd.Close() //nolint:errcheck // best-effort close after sync
	if confined {
		var stat unix.Stat_t
		if err := unix.Fstat(int(fd.Fd()), &stat); err != nil {
			return err
		}
		if stat.Nlink != 1 {
			return fmt.Errorf("%w: zefs artifact %s has %d links; remove the artifact alias before writing", fs.ErrPermission, name, stat.Nlink)
		}
	}

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
