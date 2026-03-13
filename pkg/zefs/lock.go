// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore concurrent access via ReadLock and WriteLock
// Related: guard.go -- WriteGuard and ReadGuard interfaces

package zefs

import (
	"fmt"
	"io/fs"
	"sync/atomic"
)

// ReadLock is a shared lock guard for read-only operations.
// Multiple ReadLocks can be held simultaneously.
// Release must be called exactly once; subsequent calls are no-ops.
// The released flag uses atomic.Bool so Release is safe from concurrent goroutines.
type ReadLock struct {
	s        *BlobStore
	released atomic.Bool
}

// Release releases the shared lock. Safe to call multiple times and from
// concurrent goroutines.
func (rl *ReadLock) Release() {
	if !rl.released.CompareAndSwap(false, true) {
		return
	}
	rl.s.mu.RUnlock()
}

// ReadFile returns the contents of the named file.
func (rl *ReadLock) ReadFile(name string) ([]byte, error) {
	return rl.s.readFile(name)
}

// Has returns true if the named file exists.
func (rl *ReadLock) Has(name string) bool {
	return rl.s.root.has(name)
}

// List returns all file keys under the given prefix.
func (rl *ReadLock) List(prefix string) []string {
	return rl.s.list(prefix)
}

// ReadDir returns directory entries for the named directory.
func (rl *ReadLock) ReadDir(name string) ([]fs.DirEntry, error) {
	return rl.s.readDir(name)
}

// WriteLock is an exclusive lock guard for read-write operations.
// Only one WriteLock can be held at a time, and it blocks all ReadLocks.
// Writes are batched in memory; a single flush occurs on Release.
// Release must be called exactly once; subsequent calls return nil.
type WriteLock struct {
	s        *BlobStore
	dirty    bool
	released atomic.Bool
}

// Release flushes any pending writes to disk and releases both
// the in-process mutex and the cross-process flock.
// Safe to call multiple times and from concurrent goroutines;
// subsequent calls return nil.
func (wl *WriteLock) Release() error {
	if !wl.released.CompareAndSwap(false, true) {
		return nil
	}
	defer flockUnlock(wl.s.lockFd) //nolint:errcheck // best-effort flock release
	defer wl.s.mu.Unlock()
	if wl.dirty {
		return wl.s.flush()
	}
	return nil
}

// ReadFile returns the contents of the named file, including pending writes.
func (wl *WriteLock) ReadFile(name string) ([]byte, error) {
	return wl.s.readFile(name)
}

// Has returns true if the named file exists.
func (wl *WriteLock) Has(name string) bool {
	return wl.s.root.has(name)
}

// List returns all file keys under the given prefix.
func (wl *WriteLock) List(prefix string) []string {
	return wl.s.list(prefix)
}

// ReadDir returns directory entries for the named directory.
func (wl *WriteLock) ReadDir(name string) ([]fs.DirEntry, error) {
	return wl.s.readDir(name)
}

// WriteFile creates or updates the named file without flushing to disk.
// The flush happens when Release is called.
func (wl *WriteLock) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := wl.s.writeFileNoFlush(name, data, perm); err != nil {
		return err
	}
	wl.dirty = true
	return nil
}

// Remove deletes the named file without flushing to disk.
func (wl *WriteLock) Remove(name string) error {
	err := wl.s.removeNoFlush(name)
	if err == nil {
		wl.dirty = true
	}
	return err
}

// RLock acquires a shared read lock and returns a ReadLock guard.
func (s *BlobStore) RLock() *ReadLock {
	s.mu.RLock()
	return &ReadLock{s: s}
}

// Lock acquires a cross-process flock and an exclusive in-process write lock,
// then returns a WriteLock guard. Release must be called to release both locks.
func (s *BlobStore) Lock() (*WriteLock, error) {
	if err := flockExclusive(s.lockFd); err != nil {
		return nil, fmt.Errorf("zefs: flock: %w", err)
	}
	s.mu.Lock()
	return &WriteLock{s: s}, nil
}
