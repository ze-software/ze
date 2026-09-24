// Design: docs/architecture/diagnostics/crash-capture.md -- the lock that marks a pending crash file as live

//go:build !unix

package crashlog

import (
	"errors"
	"os"
)

// errNoLock is what lockFile answers where the platform has no flock.
var errNoLock = errors.New("crashlog: file locks are not supported on this platform")

// lockFile refuses where the platform has no flock. armCrashOutput then arms
// nothing, and a harvest leaves every pending file alone, because neither can
// tell a live process from a dead one.
func lockFile(_ *os.File) error {
	return errNoLock
}
