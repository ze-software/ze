// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore mmap lifecycle
// Related: mmap_other.go -- heap fallback for non-unix

//go:build unix

package zefs

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// loadBacking memory-maps the file for zero-copy reads.
// Returns the mapped region and the open file descriptor (must stay open for mmap lifetime).
func loadBacking(path string) (data []byte, fd *os.File, retErr error) {
	f, err := os.Open(path) // #nosec G304 -- path is the store's sidecar file
	if err != nil {
		return nil, nil, fmt.Errorf("zefs: mmap open: %w", err)
	}
	defer func() {
		if retErr != nil {
			if closeErr := f.Close(); closeErr != nil {
				retErr = errors.Join(retErr, closeErr)
			}
		}
	}()

	fi, err := f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("zefs: mmap stat: %w", err)
	}

	size := fi.Size()
	if size == 0 {
		return nil, nil, fmt.Errorf("zefs: mmap: empty file")
	}

	data, err = syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		return nil, nil, fmt.Errorf("zefs: mmap: %w", err)
	}

	return data, f, nil
}

// unloadBacking unmaps the memory and closes the file descriptor.
func unloadBacking(data []byte, fd *os.File) error {
	var errs []error
	if data != nil && fd != nil {
		if err := syscall.Munmap(data); err != nil {
			errs = append(errs, fmt.Errorf("zefs: munmap: %w", err))
		}
	}
	if fd != nil {
		if err := fd.Close(); err != nil {
			errs = append(errs, fmt.Errorf("zefs: close: %w", err))
		}
	}
	return errors.Join(errs...)
}
