// Design: docs/architecture/zefs-format.md -- config storage abstraction
// Detail: blob.go -- blob storage implementation wrapping zefs
//
// Package storage provides a file I/O abstraction for ze's configuration system.
// Two implementations: filesystemStorage (wraps os calls, current behavior) and
// blobStorage (wraps zefs BlobStore). All callers use absolute filesystem paths
// as names; the blob implementation strips the leading "/" to form the key.
package storage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Storage provides abstracted file operations for config, draft, and backup files.
// For zero-copy reads from blob storage, use AcquireLock -- the WriteGuard's
// ReadFile returns lock-scoped slices without copying. The unlocked ReadFile
// always returns caller-owned copies.
type Storage interface {
	// ReadFile reads the named file and returns a caller-owned copy.
	ReadFile(name string) ([]byte, error)

	// WriteFile writes data to the named file atomically.
	// For filesystem: temp file + rename. For blob: batched via WriteLock.
	WriteFile(name string, data []byte, perm fs.FileMode) error

	// Remove removes the named file.
	Remove(name string) error

	// Exists returns true if the named file exists.
	Exists(name string) bool

	// List returns all file names under the given directory prefix.
	// Returns immediate children only (not recursive).
	List(prefix string) ([]string, error)

	// AcquireLock acquires exclusive write access for the named config.
	// Returns a WriteGuard that provides locked read/write/remove operations.
	// Release must be called to release the lock.
	AcquireLock(name string) (WriteGuard, error)

	// Stat returns metadata about the named file.
	// For filesystem: uses os.Stat for ModTime.
	// For blob: reads from per-key metadata tracked internally.
	// Returns zero FileMeta and an error if the file does not exist.
	Stat(name string) (FileMeta, error)

	// Close releases resources held by the storage.
	// For filesystem: no-op. For blob: closes the BlobStore.
	Close() error
}

// FileMeta holds metadata about a file in storage.
type FileMeta struct {
	ModTime    time.Time // Last modification time.
	ModifiedBy string    // Session ID of last modifier (empty if unknown).
}

// WriteGuard provides locked read/write/remove operations.
// All I/O within a locked section goes through the guard.
// Release must be called exactly once.
type WriteGuard interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	Remove(name string) error
	Release() error

	// SetModifier records which session is performing writes through this guard.
	// Subsequent WriteFile calls through this guard will associate the modifier
	// with each written file for Stat().ModifiedBy. No-op for filesystem storage
	// (OS does not track modifier identity).
	SetModifier(sessionID string)
}

// IsBlobStorage returns true if the given storage is backed by a zefs blob store.
// Used by callers that need mode-specific behavior (host keys).
func IsBlobStorage(s Storage) bool {
	_, ok := s.(*blobStorage)
	return ok
}

// filesystemStorage wraps os calls for direct filesystem I/O.
// The mu field serializes AcquireLock callers within the same process.
// Cross-process serialization is not provided (filesystem mode is used
// with the -f flag which explicitly bypasses the daemon).
type filesystemStorage struct {
	mu sync.Mutex
}

// NewFilesystem returns a Storage backed by the real filesystem.
func NewFilesystem() Storage {
	return &filesystemStorage{}
}

func (s *filesystemStorage) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name) //nolint:gosec // paths are resolved by caller
}

func (s *filesystemStorage) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return atomicWriteFile(name, data, perm)
}

func (s *filesystemStorage) Remove(name string) error {
	return os.Remove(name)
}

func (s *filesystemStorage) Stat(name string) (FileMeta, error) {
	info, err := os.Stat(name)
	if err != nil {
		return FileMeta{}, err
	}
	return FileMeta{ModTime: info.ModTime()}, nil
}

func (s *filesystemStorage) Close() error {
	return nil
}

func (s *filesystemStorage) Exists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func (s *filesystemStorage) List(prefix string) ([]string, error) {
	entries, err := os.ReadDir(prefix)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			result = append(result, filepath.Join(prefix, e.Name()))
		}
	}
	return result, nil
}

// AcquireLock acquires the in-process mutex for exclusive write access.
// Serializes concurrent goroutines within the same process.
func (s *filesystemStorage) AcquireLock(_ string) (WriteGuard, error) {
	s.mu.Lock()
	return &filesystemGuard{mu: &s.mu}, nil
}

// filesystemGuard holds the mutex and delegates I/O to os calls.
type filesystemGuard struct {
	mu *sync.Mutex
}

func (g *filesystemGuard) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name) //nolint:gosec // paths are resolved by caller
}

func (g *filesystemGuard) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return atomicWriteFile(name, data, perm)
}

func (g *filesystemGuard) Remove(name string) error {
	return os.Remove(name)
}

func (g *filesystemGuard) Release() error {
	if g.mu != nil {
		g.mu.Unlock()
		g.mu = nil
	}
	return nil
}

func (g *filesystemGuard) SetModifier(_ string) {
	// No-op for filesystem: OS tracks mtime via WriteFile; modifier identity not available.
}

// atomicWriteFile writes data to path via a temp file and rename.
// Ensures the file is never partially written.
func atomicWriteFile(path string, data []byte, perm fs.FileMode) error {
	if perm == 0 {
		perm = 0o600
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("storage: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".ze-storage-*")
	if err != nil {
		return fmt.Errorf("storage: create temp: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close() //nolint:errcheck // closing after write error
		return fmt.Errorf("storage: write temp: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close() //nolint:errcheck // closing after chmod error
		return fmt.Errorf("storage: chmod temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close() //nolint:errcheck // closing after sync error
		return fmt.Errorf("storage: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("storage: close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("storage: rename temp: %w", err)
	}
	committed = true
	return nil
}
