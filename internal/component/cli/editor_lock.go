// Design: docs/architecture/config/yang-config-design.md — file locking for concurrent editing
// Related: editor.go — config editor (uses lock during write-through)

package cli

import (
	"fmt"
	"os"
	"syscall"
)

// acquireLock opens and exclusively locks a file using flock.
// The lock is advisory -- other editors must also use flock to coordinate.
// Returns the open file handle; caller must release with releaseLock.
func acquireLock(lockPath string) (*os.File, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDONLY, 0o600) //nolint:gosec // Lock file path is derived from config
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close() //nolint:errcheck,gosec // Best effort on error path
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	return f, nil
}

// releaseLock releases the flock and closes the file.
func releaseLock(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		f.Close() //nolint:errcheck,gosec // Best effort on error path
		return fmt.Errorf("release lock: %w", err)
	}
	return f.Close()
}
