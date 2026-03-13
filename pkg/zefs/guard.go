// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore guard interfaces for concurrent access
// Related: lock.go -- WriteLock and ReadLock implement these interfaces

package zefs

import "io/fs"

// WriteGuard provides exclusive access with read/write operations.
// Both blob-backed and filesystem-backed implementations satisfy this
// interface, providing a consistent locking API regardless of storage mode.
type WriteGuard interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	Remove(name string) error
	Has(name string) bool
	List(prefix string) []string
	ReadDir(name string) ([]fs.DirEntry, error)
	Release() error
}

// ReadGuard provides shared access with read-only operations.
type ReadGuard interface {
	ReadFile(name string) ([]byte, error)
	Has(name string) bool
	List(prefix string) []string
	ReadDir(name string) ([]fs.DirEntry, error)
	Release()
}

// Compile-time interface satisfaction checks.
var (
	_ WriteGuard = (*WriteLock)(nil)
	_ ReadGuard  = (*ReadLock)(nil)
)
