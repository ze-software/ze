// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore cross-process flock on persistent fd
// Related: flock_other.go -- no-op fallback for non-unix

//go:build unix

package zefs

import (
	"fmt"
	"os"
	"syscall"
)

// openLockFd opens (or creates) a dedicated .lock file for cross-process flock.
// A separate file is used because atomic flush (temp+rename) replaces the store
// inode, which would invalidate a flock held on the store file itself.
func openLockFd(path string) (*os.File, error) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // store path from caller
	if err != nil {
		return nil, fmt.Errorf("zefs: lock fd: %w", err)
	}
	return f, nil
}

// closeLockFd closes the persistent lock file descriptor.
func closeLockFd(f *os.File) error {
	if f == nil {
		return nil
	}
	return f.Close()
}

// flockExclusive acquires an exclusive advisory lock (blocks until acquired).
// Advisory only: non-cooperative processes can bypass this lock.
// Retries on EINTR.
func flockExclusive(f *os.File) error {
	if f == nil {
		return nil
	}
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if err == syscall.EINTR {
			continue
		}
		return err
	}
}

// flockUnlock releases the advisory lock.
func flockUnlock(f *os.File) error {
	if f == nil {
		return nil
	}
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
