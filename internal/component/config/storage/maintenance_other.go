//go:build !linux

// Design: docs/architecture/storage-backends.md -- installer maintenance lock.

package storage

import (
	"fmt"
	"io"
)

// AcquireMaintenance requires Linux's descriptor-safe installer locking.
func AcquireMaintenance(dir string) (io.Closer, error) {
	return nil, fmt.Errorf("maintenance of %s requires Linux; use a Linux host", dir)
}
