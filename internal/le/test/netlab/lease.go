// Design: docs/guide/netlab.md -- a live run owns its directory until teardown.
package testnetlab

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// liveDirectory is admitted only after obtaining its exclusive process lease
// and proving that no deployment already exists there. Closing the descriptor
// releases the lease; the file remains so another opener cannot lock a new inode.
type liveDirectory struct {
	path  string
	lease *os.File
}

func acquireLiveDirectory(directory string) (*liveDirectory, error) {
	canonical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve live lab directory: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, err
	}
	leasePath := filepath.Join(canonical, ".ze-netlab-run.lock")
	file, err := os.OpenFile(leasePath, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open live lab lease: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.Join(fmt.Errorf("lab directory %s is already owned by another run", canonical), file.Close())
		}
		return nil, errors.Join(fmt.Errorf("lock live lab directory: %w", err), file.Close())
	}
	for _, marker := range []string{"netlab.snapshot.yml", "netlab.snapshot.pickle", "clab.yml", "netlab.lock", ".netlab.lock"} {
		_, err := os.Lstat(filepath.Join(canonical, marker))
		if err == nil {
			return nil, errors.Join(fmt.Errorf("lab directory %s already holds deployment state %s; use its owning netlab session to stop it", canonical, marker), file.Close())
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Join(fmt.Errorf("inspect live lab deployment state: %w", err), file.Close())
		}
	}
	return &liveDirectory{path: canonical, lease: file}, nil
}

func (d *liveDirectory) close() error { return d.lease.Close() }
