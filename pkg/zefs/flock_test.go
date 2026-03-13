//go:build unix

package zefs

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// VALIDATES: Lock() acquires cross-process flock on the blob file
// PREVENTS: concurrent processes corrupting the blob during writes

func TestWriteLockAcquiresCrossProcessFlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl, err := s.Lock()
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Open a second fd on the .lock file (simulating another process)
	probe, err := os.OpenFile(path+".lock", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open probe fd: %v", err)
	}
	defer probe.Close() //nolint:errcheck // test cleanup

	// Non-blocking flock should fail (EWOULDBLOCK) because Lock() holds it
	err = syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("expected EWOULDBLOCK while WriteLock held, got: %v", err)
	}

	// Release the WriteLock
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Now the probe should succeed
	err = syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		t.Fatalf("flock should succeed after Release, got: %v", err)
	}
	_ = syscall.Flock(int(probe.Fd()), syscall.LOCK_UN)

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: cross-process flock survives flush (the key invariant)
// PREVENTS: lock released mid-write when mmap fd is closed/reopened

func TestFlockSurvivesFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "initial", []byte("data"))

	wl, err := s.Lock()
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Write triggers flush on Release -- the mmap fd will be closed/reopened
	writeLockedOrFatal(t, wl, "new-key", []byte("new-value"))

	// Open probe fd on the .lock file (flock is held there, not on the store file)
	probe, err := os.OpenFile(path+".lock", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open probe fd: %v", err)
	}
	defer probe.Close() //nolint:errcheck // test cleanup

	// Probe should still be blocked (flock held on persistent fd, not mmap fd)
	err = syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("expected EWOULDBLOCK during dirty lock, got: %v", err)
	}

	// Release flushes (close/reopen mmap fd) then unlocks flock
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Now probe should succeed
	err = syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		t.Fatalf("flock should succeed after Release, got: %v", err)
	}
	_ = syscall.Flock(int(probe.Fd()), syscall.LOCK_UN)

	// Verify data persisted correctly
	got, err := s.ReadFile("new-key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-value" {
		t.Errorf("got %q, want %q", got, "new-value")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: flock functions are nil-safe (required for non-unix fallback)
// PREVENTS: panic when lockFd is nil

func TestFlockNilSafety(t *testing.T) {
	// closeLockFd(nil) should not panic
	if err := closeLockFd(nil); err != nil {
		t.Errorf("closeLockFd(nil): %v", err)
	}

	// flockExclusive(nil) should not panic
	if err := flockExclusive(nil); err != nil {
		t.Errorf("flockExclusive(nil): %v", err)
	}

	// flockUnlock(nil) should not panic
	if err := flockUnlock(nil); err != nil {
		t.Errorf("flockUnlock(nil): %v", err)
	}
}

// VALIDATES: openLockFd opens a usable fd for flock
// PREVENTS: fd opened with wrong flags for advisory locking

func TestOpenLockFdUsable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Open a lock fd independently
	fd, err := openLockFd(path)
	if err != nil {
		t.Fatalf("openLockFd: %v", err)
	}

	// Should be able to flock it
	err = syscall.Flock(int(fd.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		t.Fatalf("flock on openLockFd fd: %v", err)
	}
	_ = syscall.Flock(int(fd.Fd()), syscall.LOCK_UN)

	if err := closeLockFd(fd); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: openLockFd fails when directory doesn't exist
// PREVENTS: creating lock files in nonexistent directories

func TestOpenLockFdNonexistentDir(t *testing.T) {
	_, err := openLockFd(filepath.Join(t.TempDir(), "no-such-dir", "does-not-exist"))
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// VALIDATES: openLockFd creates .lock file for valid directory
// PREVENTS: regression if O_CREATE flag is removed

func TestOpenLockFdCreatesLockFile(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "test.zefs")
	fd, err := openLockFd(storePath)
	if err != nil {
		t.Fatalf("openLockFd: %v", err)
	}
	defer closeLockFd(fd) //nolint:errcheck // test cleanup

	// Verify the .lock file was created
	if _, err := os.Stat(storePath + ".lock"); err != nil {
		t.Errorf("expected .lock file to exist: %v", err)
	}
}

// VALIDATES: Close releases the flock fd
// PREVENTS: leaked file descriptors after store is closed

func TestCloseReleasesFlockFd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Acquire and release a lock to confirm flock works
	wl, err := s.Lock()
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Close the store (should close lockFd)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Open a probe on the .lock file -- flock should succeed (no one holding it)
	probe, err := os.OpenFile(path+".lock", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open probe: %v", err)
	}
	defer probe.Close() //nolint:errcheck // test cleanup

	err = syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		t.Fatalf("flock should succeed after Close, got: %v", err)
	}
	_ = syscall.Flock(int(probe.Fd()), syscall.LOCK_UN)
}
