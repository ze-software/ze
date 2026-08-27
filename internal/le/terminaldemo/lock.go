// Design: docs/architecture/core-design.md -- terminal-demo state has one process owner
// Overview: types.go -- the Engine that owns the lock interval

package terminaldemo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func (e *Engine) withLock(work func() error) error {
	if err := os.MkdirAll(filepath.Dir(e.lockPath), 0o750); err != nil {
		return err
	}
	handle, err := os.OpenFile(e.lockPath, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer handle.Close() //nolint:errcheck // The lock operation owns the verdict.

	deadline := time.Now().Add(e.lockWait)
	for {
		err = syscall.Flock(int(handle.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			if !errors.Is(err, syscall.EAGAIN) {
				return err
			}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("another demo run held %s for %g seconds", e.lockPath, e.lockWait.Seconds())
		}
		time.Sleep(e.lockPoll)
	}
	defer syscall.Flock(int(handle.Fd()), syscall.LOCK_UN) //nolint:errcheck // Process exit also releases this advisory lock.
	return work()
}
