// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore heap-backed fallback
// Related: mmap_unix.go -- mmap for zero-copy reads on unix

//go:build !unix

package zefs

import (
	"fmt"
	"io"
	"os"
)

// loadBacking reads the file into a heap-allocated buffer.
// Fallback for platforms without mmap support.
func loadBacking(path string) ([]byte, *os.File, error) {
	f, err := os.Open(path) // #nosec G304 -- path is the store's sidecar file
	if err != nil {
		return nil, nil, fmt.Errorf("zefs: read: %w", err)
	}
	return loadBackingFile(f)
}

// loadBackingFile reads an already-open file into a heap-allocated buffer. It
// takes the descriptor rather than the name so a reload reads the inode the
// caller wrote, not whatever the name resolves to now. It closes f.
func loadBackingFile(f *os.File) ([]byte, *os.File, error) {
	defer f.Close() //nolint:errcheck // heap-backed: the descriptor has no further use

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, fmt.Errorf("zefs: read: %w", err)
	}
	if len(data) == 0 {
		return nil, nil, fmt.Errorf("zefs: empty file")
	}
	return data, nil, nil // fd is nil = heap-backed
}

// unloadBacking is a no-op for heap-backed stores (GC handles it).
func unloadBacking(_ []byte, _ *os.File) error { return nil }
