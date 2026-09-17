// Design: docs/architecture/storage-backends.md -- encoding-independent operations.
package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

var (
	ErrNoStore     = errors.New("no configuration store")
	ErrPermissions = fmt.Errorf("unsafe store permissions or node: %w", fs.ErrPermission)
	ErrReadOnly    = errors.New("storage is read-only")
	ErrBusy        = errors.New("store is owned by another process")
	// ErrImportPending reports an import intent beside an absent tree: an import
	// crashed after moving the old tree to database.replaced-* and before
	// publishing its stage. It is never ErrNoStore, so nothing auto-creates an
	// empty tree over the operator's store; the repair is ze init --from.
	ErrImportPending = errors.New("unfinished import")
)

type keyAccess interface {
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, fs.FileMode) error
	Remove(string) error
}

type store struct {
	mu       sync.RWMutex
	tree     *treeEncoding
	blob     *zefs.BlobStore
	owner    *os.File
	readonly bool
	closed   bool
	metas    map[string]FileMeta
	observer func(string)
}

func newStore(tree *treeEncoding, blob *zefs.BlobStore, owner *os.File, readonly bool) *store {
	return &store{tree: tree, blob: blob, owner: owner, readonly: readonly, metas: make(map[string]FileMeta)}
}

func (s *store) access() keyAccess {
	if s.tree != nil {
		return s.tree
	}
	return s.blob
}

func validKey(key string) error {
	if key == "." {
		return fmt.Errorf("invalid storage key %q", key)
	}
	if !fs.ValidPath(key) {
		return fmt.Errorf("invalid storage key %q", key)
	}
	return nil
}

// CheckName refuses a config name whose directory is not this store's folder,
// so a path from elsewhere never silently resolves to a same-named stored config.
func (s *store) CheckName(name string) error {
	if isNamespaced(name) {
		return validKey(name)
	}
	base := filepath.Base(name)
	if base == "." {
		return fmt.Errorf("invalid config name %q", name)
	}
	if strings.Contains(base, "..") {
		return fmt.Errorf("invalid config name %q", name)
	}
	if s.tree == nil {
		return nil
	}
	if filepath.Dir(name) == "." {
		return nil
	}
	folder, err := filepath.Abs(s.tree.folder.Name())
	if err != nil {
		return err
	}
	parent, err := filepath.Abs(filepath.Dir(name))
	if err != nil {
		return err
	}
	if folder != parent {
		return fmt.Errorf("config %s is outside store folder %s", name, folder)
	}
	return nil
}

func resolveKey(name string) string {
	if isNamespaced(name) {
		return name
	}
	return zefs.KeyFileActive.Key(filepath.Base(name))
}
func isNamespaced(name string) bool {
	return strings.HasPrefix(name, "meta/") || strings.HasPrefix(name, "file/")
}
func resolveDirKey(name string) string {
	if isNamespaced(name) {
		return strings.TrimSuffix(name, "/")
	}
	if name == "meta" {
		return name
	}
	if name == "file" {
		return name
	}
	return zefs.KeyFileActive.Dir()
}

func (s *store) ReadFile(name string) ([]byte, error) {
	if err := s.CheckName(name); err != nil {
		return nil, err
	}
	return s.ReadKey(resolveKey(name))
}
func (s *store) ReadKey(key string) ([]byte, error) {
	if err := validKey(key); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fs.ErrClosed
	}
	return s.access().ReadFile(key)
}
func (s *store) WriteFile(name string, data []byte, _ fs.FileMode) error {
	if err := s.CheckName(name); err != nil {
		return err
	}
	return s.WriteKey(resolveKey(name), data)
}
func (s *store) WriteKey(key string, data []byte) (err error) {
	g, err := s.acquire()
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(g, err) }()
	return g.write(key, data, time.Now())
}
func (s *store) Remove(name string) error {
	if err := s.CheckName(name); err != nil {
		return err
	}
	return s.RemoveKey(resolveKey(name))
}
func (s *store) RemoveKey(key string) (err error) {
	g, err := s.acquire()
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(g, err) }()
	return g.remove(key)
}
func (s *store) Exists(name string) bool {
	if err := s.CheckName(name); err != nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return false
	}
	key := resolveKey(name)
	if s.tree != nil {
		return s.tree.has(key)
	}
	return s.blob.Has(key)
}
func (s *store) Stat(name string) (FileMeta, error) {
	if err := s.CheckName(name); err != nil {
		return FileMeta{}, err
	}
	key := resolveKey(name)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return FileMeta{}, fs.ErrClosed
	}
	if _, err := s.access().ReadFile(key); err != nil {
		return FileMeta{}, err
	}
	return s.metas[key], nil
}
func (s *store) SetWriteObserver(fn func(string)) { s.mu.Lock(); s.observer = fn; s.mu.Unlock() }

func (s *store) ListKeys(prefix string) ([]string, error) {
	if prefix != "" {
		if err := validKey(strings.TrimSuffix(prefix, "/")); err != nil {
			return nil, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fs.ErrClosed
	}
	if s.tree != nil {
		return s.tree.list(prefix)
	}
	keys := s.blob.List("")
	result := keys[:0]
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			result = append(result, key)
		}
	}
	slices.Sort(result)
	return result, nil
}
func (s *store) List(prefix string) ([]string, error) {
	key := resolveDirKey(prefix)
	keys, err := s.ListKeys(key + "/")
	if err != nil {
		return nil, err
	}
	return immediateChildren(keys, key), nil
}

func immediateChildren(keys []string, prefix string) []string {
	result := keys[:0]
	for _, name := range keys {
		if !strings.Contains(strings.TrimPrefix(name, prefix+"/"), "/") {
			result = append(result, name)
		}
	}
	return result
}
func (s *store) ListVersions(name string) ([]VersionInfo, error) {
	if err := s.CheckName(name); err != nil {
		return nil, err
	}
	keys, err := s.ListKeys("file/")
	if err != nil {
		return nil, err
	}
	base := filepath.Base(name)
	var result []VersionInfo
	for _, key := range keys {
		parts := strings.Split(key, "/")
		if len(parts) != 3 {
			continue
		}
		if parts[2] != base {
			continue
		}
		stamp, err := parseVersionStamp(parts[1])
		if err != nil {
			continue
		}
		result = append(result, VersionInfo{Stamp: parts[1], Date: stamp, Path: key})
	}
	slices.SortFunc(result, func(a, b VersionInfo) int { return b.Date.Compare(a.Date) })
	return result, nil
}
func (s *store) WriteVersion(name string, data []byte, stamp time.Time) (err error) {
	g, err := s.acquire()
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(g, err) }()
	return g.WriteVersion(name, data, stamp)
}
func (s *store) Rename(oldName, newName string) (err error) {
	if err := s.CheckName(newName); err != nil {
		return err
	}
	if err := s.CheckName(oldName); err != nil {
		return err
	}
	g, err := s.acquire()
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(g, err) }()
	oldKey, newKey := resolveKey(oldName), resolveKey(newName)
	if oldKey == newKey {
		_, err = g.ReadFile(oldName)
		return err
	}
	data, err := g.ReadFile(oldName)
	if err != nil {
		return err
	}
	meta := s.metas[oldKey]
	if err := g.write(newKey, data, time.Now()); err != nil {
		return err
	}
	if err := g.remove(oldKey); err != nil {
		return err
	}
	g.pending[newKey] = meta
	return nil
}
func (s *store) AcquireLock(name string) (WriteGuard, error) {
	if name != "" {
		if err := s.CheckName(name); err != nil {
			return nil, err
		}
	}
	return s.acquire()
}
func (s *store) acquire() (*guard, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fs.ErrClosed
	}
	if s.readonly {
		s.mu.Unlock()
		return nil, ErrReadOnly
	}
	g := &guard{parent: s, access: s.access(), pending: make(map[string]FileMeta), removed: make(map[string]bool)}
	if s.blob != nil {
		lock, err := s.blob.Lock()
		if err != nil {
			s.mu.Unlock()
			return nil, err
		}
		g.blobLock = lock
		g.access = lock
	}
	return g, nil
}
func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if s.tree != nil {
		err = s.tree.close()
	}
	if s.blob != nil {
		err = s.blob.Close()
	}
	if s.owner != nil {
		err = errors.Join(err, s.owner.Close())
	}
	return err
}

type guard struct {
	parent   *store
	access   keyAccess
	blobLock *zefs.WriteLock
	modifier string
	pending  map[string]FileMeta
	removed  map[string]bool
	released bool
}

func (g *guard) ReadFile(name string) ([]byte, error) {
	if err := g.parent.CheckName(name); err != nil {
		return nil, err
	}
	if g.released {
		return nil, fs.ErrClosed
	}
	key := resolveKey(name)
	if err := validKey(key); err != nil {
		return nil, err
	}
	return g.access.ReadFile(key)
}

func (g *guard) List(prefix string) ([]string, error) {
	if g.released {
		return nil, fs.ErrClosed
	}
	key := resolveDirKey(prefix)
	if err := validKey(key); err != nil {
		return nil, err
	}
	var keys []string
	if g.blobLock != nil {
		keys = g.blobLock.List(key)
		slices.Sort(keys)
	} else {
		var err error
		keys, err = g.parent.tree.list(key + "/")
		if err != nil {
			return nil, err
		}
	}
	return immediateChildren(keys, key), nil
}
func (g *guard) WriteFile(name string, data []byte, _ fs.FileMode) error {
	if err := g.parent.CheckName(name); err != nil {
		return err
	}
	return g.write(resolveKey(name), data, time.Now())
}
func (g *guard) write(key string, data []byte, stamp time.Time) error {
	if g.released {
		return fs.ErrClosed
	}
	if err := validKey(key); err != nil {
		return err
	}
	if err := g.access.WriteFile(key, data, 0o600); err != nil {
		return err
	}
	g.pending[key] = FileMeta{ModTime: stamp, ModifiedBy: g.modifier}
	delete(g.removed, key)
	return nil
}
func (g *guard) Remove(name string) error {
	if err := g.parent.CheckName(name); err != nil {
		return err
	}
	return g.remove(resolveKey(name))
}
func (g *guard) remove(key string) error {
	if g.released {
		return fs.ErrClosed
	}
	if err := validKey(key); err != nil {
		return err
	}
	if err := g.access.Remove(key); err != nil {
		return err
	}
	delete(g.pending, key)
	g.removed[key] = true
	return nil
}
func (g *guard) Has(name string) bool {
	if g.released {
		return false
	}
	if err := g.parent.CheckName(name); err != nil {
		return false
	}
	key := resolveKey(name)
	if g.blobLock != nil {
		return g.blobLock.Has(key)
	}
	return g.parent.tree.has(key)
}
func (g *guard) SetModifier(modifier string) { g.modifier = modifier }
func (g *guard) WriteVersion(name string, data []byte, stamp time.Time) error {
	if err := g.parent.CheckName(name); err != nil {
		return err
	}
	return g.write(zefs.KeyFileVersion.Key(FormatVersionStamp(stamp), filepath.Base(name)), data, stamp)
}

// Release MUST follow AcquireLock. Observers run only after durability and unlock.
func (g *guard) Release() error {
	if g.released {
		return nil
	}
	g.released = true
	var err error
	if g.blobLock != nil {
		err = g.blobLock.Release()
	}
	if err == nil {
		maps.Copy(g.parent.metas, g.pending)
		for key := range g.removed {
			delete(g.parent.metas, key)
		}
	}
	observer := g.parent.observer
	g.parent.mu.Unlock()
	if err != nil {
		return err
	}
	if observer != nil {
		for key := range g.pending {
			observer(key)
		}
	}
	return nil
}
