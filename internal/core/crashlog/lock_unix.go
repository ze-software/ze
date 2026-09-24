// Design: docs/architecture/diagnostics/crash-capture.md -- the lock that marks a pending crash file as live

//go:build unix

package crashlog

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes an exclusive lock on f without waiting. It fails when another
// open file description holds the lock, which is how a harvest tells a running
// process's pending file from a dead one's. The kernel drops the lock when the
// holder exits, however it exits.
func lockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}
