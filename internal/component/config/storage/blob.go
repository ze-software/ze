// Design: docs/architecture/zefs-format.md -- blob storage implementation
// Overview: storage.go -- Storage interface and filesystem implementation

package storage

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// blobStorage wraps a zefs BlobStore for config file I/O.
// All paths are absolute filesystem paths; the leading "/" is stripped to form blob keys.
type blobStorage struct {
	store     *zefs.BlobStore
	blobPath  string
	configDir string
}

// NewBlob returns a Storage backed by a zefs blob store at blobPath.
// If the blob does not exist, it is created and existing config files are migrated.
// If creation fails, returns an error (caller decides whether to fall back to filesystem).
func NewBlob(blobPath, configDir string) (Storage, error) {
	var store *zefs.BlobStore
	var err error

	if _, statErr := os.Stat(blobPath); statErr == nil {
		store, err = zefs.Open(blobPath)
	} else {
		store, err = zefs.Create(blobPath)
		if err == nil {
			migrateExistingFiles(store, configDir)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("storage: blob %s: %w", blobPath, err)
	}

	return &blobStorage{store: store, blobPath: blobPath, configDir: configDir}, nil
}

// Close closes the underlying blob store.
func (s *blobStorage) Close() error {
	return s.store.Close()
}

func (s *blobStorage) ReadFile(name string) ([]byte, error) {
	return s.store.ReadFile(resolveKey(name, s.configDir))
}

func (s *blobStorage) WriteFile(name string, data []byte, _ fs.FileMode) error {
	return s.store.WriteFile(resolveKey(name, s.configDir), data, 0)
}

func (s *blobStorage) Remove(name string) error {
	return s.store.Remove(resolveKey(name, s.configDir))
}

func (s *blobStorage) Exists(name string) bool {
	return s.store.Has(resolveKey(name, s.configDir))
}

func (s *blobStorage) List(prefix string) ([]string, error) {
	key := resolveKey(prefix, s.configDir)
	// Use ReadDir for immediate children only (matches filesystem semantics)
	entries, err := s.store.ReadDir(key)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			result = append(result, string(filepath.Separator)+filepath.Join(key, e.Name()))
		}
	}
	return result, nil
}

func (s *blobStorage) AcquireLock(_ string) (WriteGuard, error) {
	wl, err := s.store.Lock()
	if err != nil {
		return nil, fmt.Errorf("storage: blob lock: %w", err)
	}
	return &blobGuard{wl: wl, configDir: s.configDir}, nil
}

// blobGuard wraps a zefs WriteLock as a WriteGuard.
type blobGuard struct {
	wl        *zefs.WriteLock
	configDir string
}

func (g *blobGuard) ReadFile(name string) ([]byte, error) {
	return g.wl.ReadFile(resolveKey(name, g.configDir))
}

func (g *blobGuard) WriteFile(name string, data []byte, _ fs.FileMode) error {
	return g.wl.WriteFile(resolveKey(name, g.configDir), data, 0)
}

func (g *blobGuard) Remove(name string) error {
	return g.wl.Remove(resolveKey(name, g.configDir))
}

func (g *blobGuard) Release() error {
	return g.wl.Release()
}

// pathToKey converts an absolute filesystem path to a blob key
// by stripping the leading "/".
func pathToKey(path string) string {
	return strings.TrimPrefix(path, "/")
}

// resolveKey converts a path to a blob key, resolving relative paths
// against configDir to match the keys created during migration.
func resolveKey(name, configDir string) string {
	if filepath.IsAbs(name) {
		return pathToKey(name)
	}
	if configDir == "" {
		return pathToKey(name)
	}
	abs, err := filepath.Abs(filepath.Join(configDir, name))
	if err != nil {
		return pathToKey(name)
	}
	return pathToKey(abs)
}

// migrateExistingFiles imports config files from configDir into a newly created blob.
func migrateExistingFiles(store *zefs.BlobStore, configDir string) {
	if configDir == "" {
		return
	}

	wl, err := store.Lock()
	if err != nil {
		slog.Warn("storage: migration lock failed", "error", err)
		return
	}

	imported := 0
	patterns := []string{
		filepath.Join(configDir, "*.conf"),
		filepath.Join(configDir, "*.conf.draft"),
		filepath.Join(configDir, "rollback", "*.conf"),
		filepath.Join(configDir, "ssh_host_*"),
	}

	for _, pattern := range patterns {
		matches, globErr := filepath.Glob(pattern)
		if globErr != nil {
			continue
		}
		for _, path := range matches {
			abs, absErr := filepath.Abs(path)
			if absErr != nil {
				continue
			}
			key := pathToKey(abs)
			if wl.Has(key) {
				continue // idempotent: skip if already in blob
			}
			data, readErr := os.ReadFile(abs) //nolint:gosec // migrating user's config files
			if readErr != nil {
				slog.Warn("storage: migration read failed", "path", abs, "error", readErr)
				continue
			}
			if writeErr := wl.WriteFile(key, data, 0); writeErr != nil {
				slog.Warn("storage: migration write failed", "key", key, "error", writeErr)
				continue
			}
			slog.Info("storage: migrated", "key", key, "bytes", len(data))
			imported++
		}
	}

	if err := wl.Release(); err != nil {
		slog.Warn("storage: migration flush failed", "error", err)
		return
	}

	if imported > 0 {
		slog.Info("storage: migration complete", "files", imported)
	}
}
