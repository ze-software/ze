//go:build linux

// Design: docs/architecture/storage-backends.md -- installer maintenance lock.

package storage

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// AcquireMaintenance holds the same exclusive lifetime lock as Open without
// opening any store data. The caller MUST Close it before starting the daemon
// or transferring ownership. Root may lock a service-owned folder; other users
// MUST own it. Ancestors and lock nodes MUST be real directories/regular files.
func AcquireMaintenance(dir string) (io.Closer, error) {
	folder, err := ownershipOpenFolder(dir)
	if err != nil {
		return nil, err
	}
	var info unix.Stat_t
	if err := unix.Fstat(int(folder.Fd()), &info); err != nil {
		return nil, errors.Join(err, folder.Close())
	}
	if os.Geteuid() != 0 {
		if info.Uid != uint32(os.Geteuid()) {
			return nil, errors.Join(fmt.Errorf("%w: maintenance of %s requires its owner or root", ErrPermissions, dir), folder.Close())
		}
	}
	owner, _, err := ownershipLock(folder)
	closeErr := folder.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return nil, errors.Join(closeErr, owner.Close())
	}
	return owner, nil
}
