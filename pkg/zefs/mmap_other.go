// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore heap-backed fallback
// Related: mmap_unix.go -- mmap for zero-copy reads on unix

//go:build !unix

package zefs

import (
	"fmt"
	"os"
)

// loadBacking reads the file into a heap-allocated buffer.
// Fallback for platforms without mmap support.
func loadBacking(path string) ([]byte, *os.File, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is the store's sidecar file
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
