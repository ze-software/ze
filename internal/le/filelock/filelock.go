// Design: docs/architecture/core-design.md -- a derived artifact declares its inputs and its rebuild
//
// Package filelock takes the exclusive advisory lock of a named file that
// another process may remove, for the le tools whose sessions share one
// checkout: the derived-artifact lock (internal/le/derived) and the verify
// parts record (internal/le/verify) both hold their lock through Open.
package filelock

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
)

// ErrBusy is the answer of Open when another holder kept the lock past the
// caller's deadline.
var ErrBusy = errors.New("another process holds it")

// ErrRemoved is the answer of Open when the file was removed attempts times
// in a row between the open and the lock. Each removal is a process acting
// on the file, so meeting this many means something removes it in a loop,
// and an error says so rather than spinning.
var ErrRemoved = errors.New("it was removed repeatedly while waiting for its lock")

// attempts bounds how often Open reopens a file removed under it.
const attempts = 8

// retry is how often a bounded wait asks for the lock again. A holder
// releases the lock by closing a descriptor, which signals no waiter, so a
// wait with a deadline asks again rather than blocking in flock(2).
const retry = 50 * time.Millisecond

// Open opens path, creating it with mode 0600, and answers it with its
// exclusive advisory lock held. The caller MUST close the file, which
// releases the lock.
//
// A zero deadline waits for the lock without a bound. Any other deadline
// waits until then and answers ErrBusy. waiting, when not nil, is called at
// most once per Open: the first time the lock is found held, on whichever
// attempt that is, before the wait starts. A test learns from it that the
// caller is about to block, without a sleep.
//
// The lock is on the open file, not on the path. A process that opened the
// file and then waited while something removed it would hold a lock on a file
// nobody can find, so the lock counts only while path still names the locked
// file; otherwise the file is closed and opened again, attempts times.
func Open(path string, deadline time.Time, waiting func()) (*os.File, error) {
	// An attempt that found the lock free and the file removed never called
	// waiting, so it is not cleared per attempt: OnceFunc keeps it for the
	// attempt that first meets a held lock, and silent on every one after.
	if waiting != nil {
		waiting = sync.OnceFunc(waiting)
	}
	for range attempts {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // a lock path the caller derived from its own checkout
		if err != nil {
			return nil, err
		}
		named, err := Hold(file, path, deadline, waiting)
		if err != nil {
			file.Close() //nolint:errcheck // the lock error owns the verdict
			return nil, err
		}
		if named {
			return file, nil
		}
		file.Close() //nolint:errcheck // a removed file holds nothing to keep; closing it drops its lock
	}
	return nil, ErrRemoved
}

// Hold takes the exclusive lock of an open file, waiting as Open does, and
// reports whether path still names file. When it does not, or when the check
// fails, the lock is released again and file holds nothing a reader of path
// can see.
func Hold(file *os.File, path string, deadline time.Time, waiting func()) (bool, error) {
	if err := take(file, deadline, waiting); err != nil {
		return false, err
	}
	named, err := stillNamed(file, path)
	if err != nil {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN) //nolint:errcheck // the stat error owns the verdict
		return false, err
	}
	if !named {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN) //nolint:errcheck // the caller closes the file
		return false, nil
	}
	return true, nil
}

// take acquires file's exclusive lock: at once when nobody holds it, then
// without a bound for a zero deadline, or by asking again every retry until
// the deadline.
func take(file *os.File, deadline time.Time, waiting func()) error {
	descriptor := int(file.Fd())
	err := syscall.Flock(descriptor, syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		return fmt.Errorf("flock: %w", err)
	}
	if waiting != nil {
		waiting()
	}
	if deadline.IsZero() {
		if err := syscall.Flock(descriptor, syscall.LOCK_EX); err != nil {
			return fmt.Errorf("flock: %w", err)
		}
		return nil
	}
	// Bounded by the deadline: each pass either takes the lock, fails, or
	// moves closer to the deadline by at least one pause.
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ErrBusy
		}
		time.Sleep(min(retry, remaining))
		err := syscall.Flock(descriptor, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("flock: %w", err)
		}
	}
}

// stillNamed reports whether path still names file, and releases file's lock
// when it does not, so a removed file holds nothing another process can find.
func stillNamed(file *os.File, path string) (bool, error) {
	opened, err := file.Stat()
	if err != nil {
		return false, err
	}
	named, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(opened, named), nil
}
