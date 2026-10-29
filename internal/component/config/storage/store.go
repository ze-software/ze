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
	// ErrImportPending reports an unfinished or unreadable import intent,
	// beside an absent tree or an existing one. It is never ErrNoStore, so
	// nothing auto-creates an empty tree over the operator's store; the repair
	// is the command the error names: ze init --from or ze data restore full.
	ErrImportPending = errors.New("unfinished import")
)

type keyAccess interface {
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, fs.FileMode) error
	Remove(string) error
}

// keyEncoding is a key space the store reaches directly: the live tree, or the
// memory encoding of the editor test harness. Each MUST be safe for a read
// concurrent with a guard's writes, because unguarded reads do not wait for
// the guard, and for a configuration guard's writes concurrent with a runtime
// guard's writes, because the two never wait for each other (store). A blob is
// reached through its BlobStore outside a guard and its WriteLock inside one.
type keyEncoding interface {
	keyAccess
	has(string) bool
	list(string) ([]string, error)
}

// store owns one encoding. Safe for concurrent use, under three locks taken
// in the order writer, then runtime, then mu:
//
//   - writer is the configuration lock. A guard holds it from acquire to
//     Release unless it was acquired for a runtime-state key, so configuration
//     writers run one at a time. A guard can hold it across many syncs: the
//     boot record does on a slow disk. ReadVersion takes it shared, because its two reads (entry,
//     then object) must not straddle a removal; Stat, because its bytes and
//     its meta must come from the same side of a guard; and writeConfigFile,
//     which keeps Close away while it syncs a loose config.
//   - runtime is the runtime-state lock, over every key runtimeKey names. A
//     guard acquired for a runtime-state key holds only it, and refuses any
//     other key. A configuration guard takes it the first time it reaches a
//     runtime-state key, a listing that could hold one included, and keeps it
//     to Release, so a backup still sees the whole store at one moment. So a
//     plugin's runtime-state write never waits for the syncs of a
//     configuration write it has nothing to do with: OSPF's planned graceful
//     restart fact passed its five second deadline behind the boot record when
//     one lock covered both. A
//     blob store takes both locks for every guard, because its one WriteLock
//     serializes it anyway.
//   - mu is held briefly. Its read lock covers one unguarded read, so Close
//     cannot release the encoding under it; Release takes it to publish metas.
//
// An unguarded read takes mu only, never writer: a read of one key needs no
// writer to finish, because the tree replaces a key's bytes with one rename
// and the memory and blob encodings lock themselves. A read that waited for
// writer waited for every sync a guard made, of keys it never asked for.
// closed is written under all three locks and read under any one.
type store struct {
	writer   sync.RWMutex
	runtime  sync.Mutex
	mu       sync.RWMutex
	tree     *treeEncoding
	keys     keyEncoding
	blob     *zefs.BlobStore
	owner    *os.File
	readonly bool
	closed   bool
	metas    map[string]FileMeta
	observer func(string)
}

func newStore(tree *treeEncoding, blob *zefs.BlobStore, owner *os.File, readonly bool) *store {
	result := &store{tree: tree, blob: blob, owner: owner, readonly: readonly, metas: make(map[string]FileMeta)}
	// A nil *treeEncoding stored in keys would be a non-nil interface, and every
	// keys branch would then call through a nil tree on a blob store.
	if tree != nil {
		result.keys = tree
	}
	return result
}

func (s *store) access() keyAccess {
	if s.keys != nil {
		return s.keys
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
	// The two directories are compared by IDENTITY, never by spelling. One
	// directory reaches this guard under two names whenever a symlink stands in
	// the path: filepath.Abs does not resolve one, and the framed-tree opener
	// canonicalizes its own folder (on darwin trustedFramePath rewrites /var to
	// /private/var, pkg/zefs/check_tree_darwin.go). A text comparison then
	// refuses a config that sits in the store folder, and says "outside" about
	// a file that is inside it.
	folderInfo, err := s.tree.folder.Stat()
	if err != nil {
		return err
	}
	folder := s.tree.folder.Name()
	parentInfo, err := os.Stat(filepath.Dir(name))
	if err != nil {
		// A directory that cannot be stat'ed is not shown to be the store
		// folder, so the guard refuses: it fails closed, and the error says
		// which read failed.
		return fmt.Errorf("config %s is outside store folder %s: %w", name, folder, err)
	}
	if !os.SameFile(folderInfo, parentInfo) {
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
	g, err := s.acquire(runtimeKey(key))
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
	g, err := s.acquire(runtimeKey(key))
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
	if s.keys != nil {
		return s.keys.has(key)
	}
	return s.blob.Has(key)
}
func (s *store) Stat(name string) (FileMeta, error) {
	if err := s.CheckName(name); err != nil {
		return FileMeta{}, err
	}
	key := resolveKey(name)
	// writer, held shared, keeps the answer whole: a guard writes a key's bytes
	// at once but publishes its meta only at Release, so without it Stat could
	// pair the new bytes with the old ModTime, and the editor would later take
	// its own draft for another session's change. Stat may then wait for a
	// slow guard, which only the CLI editor's draft checks pay: no runtime
	// state read calls it.
	s.writer.RLock()
	defer s.writer.RUnlock()
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
	if err := validPrefix(prefix); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fs.ErrClosed
	}
	if s.keys != nil {
		return s.keys.list(prefix)
	}
	return keysWithPrefix(s.blob.List(""), prefix), nil
}

// validPrefix refuses a ListKeys prefix that no key can start with. The empty
// prefix names the whole store, and a trailing slash names a directory.
func validPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	return validKey(strings.TrimSuffix(prefix, "/"))
}

// keysWithPrefix filters every blob key to the ones that string-start with
// prefix, and sorts them, so a blob answers ListKeys in the order the tree
// encoding answers it. It reuses keys' backing array.
func keysWithPrefix(keys []string, prefix string) []string {
	result := keys[:0]
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			result = append(result, key)
		}
	}
	slices.Sort(result)
	return result
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
		stamp, entryName, ok := historyEntry(key)
		if !ok {
			continue
		}
		if entryName != base {
			continue
		}
		date, err := parseVersionStamp(stamp)
		if err != nil {
			return nil, fmt.Errorf("list versions: %s: %w", key, err)
		}
		result = append(result, VersionInfo{Stamp: stamp, Date: date, Path: key})
	}
	slices.SortFunc(result, func(a, b VersionInfo) int { return b.Date.Compare(a.Date) })
	return result, nil
}

// ReadVersion resolves the history entry of name at stamp to its object and
// verifies the hash. It holds writer shared for both reads, so no removal can
// delete the object between them, and mu, so Close waits for them.
func (s *store) ReadVersion(name, stamp string) ([]byte, error) {
	key, err := versionPath(s, name, stamp)
	if err != nil {
		return nil, err
	}
	s.writer.RLock()
	defer s.writer.RUnlock()
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fs.ErrClosed
	}
	return readVersionEntry(s.access().ReadFile, key)
}
func (s *store) WriteVersion(name string, data []byte, stamp time.Time) (err error) {
	g, err := s.acquire(false)
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
	g, err := s.acquire(false)
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
	// A runtime guard's Release writes metas under mu alone, beside this
	// configuration guard, so the read takes mu too.
	s.mu.RLock()
	meta := s.metas[oldKey]
	s.mu.RUnlock()
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
	return s.acquire(runtimeKey(name))
}

// runtimeKey reports whether key is runtime state: a metadata key outside the
// configuration pointers. Every other key (config files, history, objects,
// pointers) is configuration, under writer.
func runtimeKey(key string) bool {
	if !strings.HasPrefix(key, "meta/") {
		return false
	}
	return !strings.HasPrefix(key, "meta/config/")
}

// runtimeOverlap reports whether a listing of prefix can hold a runtime-state key.
func runtimeOverlap(prefix string) bool {
	if strings.HasPrefix("meta/", prefix) { //nolint:gocritic // argOrder: deliberate, asks whether prefix is itself a prefix of "meta/".
		return true
	}
	return runtimeKey(prefix)
}

// runtimeOnly reports whether every key a listing of prefix can hold is
// runtime state, so a runtime guard may answer it.
func runtimeOnly(prefix string) bool {
	if !runtimeKey(prefix) {
		return false
	}
	return !strings.HasPrefix("meta/config/", prefix) //nolint:gocritic // argOrder: deliberate, asks whether prefix is itself a prefix of "meta/config/".
}

// acquire takes the locks a new guard holds: runtime alone for a runtime-state
// guard, writer for any other, and both on a blob (store).
func (s *store) acquire(runtime bool) (*guard, error) {
	g := &guard{parent: s, access: s.access(), pending: make(map[string]FileMeta), removed: make(map[string]bool)}
	if s.blob != nil || !runtime {
		s.writer.Lock()
		g.holdsConfig = true
	}
	if s.blob != nil || runtime {
		s.runtime.Lock()
		g.holdsRuntime = true
	}
	if s.closed {
		g.unlock()
		return nil, fs.ErrClosed
	}
	if s.readonly {
		g.unlock()
		return nil, ErrReadOnly
	}
	if s.blob != nil {
		lock, err := s.blob.Lock()
		if err != nil {
			g.unlock()
			return nil, err
		}
		g.blobLock = lock
		g.access = lock
	}
	return g, nil
}
func (s *store) Close() error {
	s.writer.Lock()
	defer s.writer.Unlock()
	s.runtime.Lock()
	defer s.runtime.Unlock()
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
	parent       *store
	access       keyAccess
	blobLock     *zefs.WriteLock
	modifier     string
	pending      map[string]FileMeta
	removed      map[string]bool
	released     bool
	holdsConfig  bool
	holdsRuntime bool
}

// errOutsideRuntimeGuard refuses a configuration key to a guard acquired for
// runtime state. It is a caller defect: such a guard holds no configuration
// lock, so it cannot write or list that space consistently.
func errOutsideRuntimeGuard(key string) error {
	return fmt.Errorf("store guard acquired for runtime state cannot reach %q: acquire the guard with a configuration name to reach configuration keys", key)
}

// reach makes the guard hold the lock that covers key, taking runtime the
// first time a configuration guard reaches runtime state (store).
func (g *guard) reach(key string) error {
	if !runtimeKey(key) {
		if g.holdsConfig {
			return nil
		}
		return errOutsideRuntimeGuard(key)
	}
	g.holdRuntime()
	return nil
}

// reachPrefix is reach for a listing, which can hold any key under prefix.
func (g *guard) reachPrefix(prefix string) error {
	if !g.holdsConfig {
		if runtimeOnly(prefix) {
			return nil
		}
		return errOutsideRuntimeGuard(prefix)
	}
	if runtimeOverlap(prefix) {
		g.holdRuntime()
	}
	return nil
}

// holdRuntime takes the runtime lock after writer, which keeps the lock order.
func (g *guard) holdRuntime() {
	if g.holdsRuntime {
		return
	}
	g.parent.runtime.Lock()
	g.holdsRuntime = true
}

// unlock releases what the guard holds, runtime before writer.
func (g *guard) unlock() {
	if g.holdsRuntime {
		g.parent.runtime.Unlock()
	}
	if g.holdsConfig {
		g.parent.writer.Unlock()
	}
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
	if err := g.reach(key); err != nil {
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
	if err := g.reachPrefix(key + "/"); err != nil {
		return nil, err
	}
	var keys []string
	if g.blobLock != nil {
		keys = g.blobLock.List(key)
		slices.Sort(keys)
	} else {
		var err error
		keys, err = g.parent.keys.list(key + "/")
		if err != nil {
			return nil, err
		}
	}
	return immediateChildren(keys, key), nil
}

// ReadKey reads one raw key through the held guard. It reaches the encoding
// directly, because on a blob Storage.ReadKey would wait forever for the blob
// mutex the guard's WriteLock holds. On a blob the bytes are valid only until Release.
func (g *guard) ReadKey(key string) ([]byte, error) {
	if g.released {
		return nil, fs.ErrClosed
	}
	if err := validKey(key); err != nil {
		return nil, err
	}
	if err := g.reach(key); err != nil {
		return nil, err
	}
	return g.access.ReadFile(key)
}

// ListKeys answers what Storage.ListKeys answers, through the held guard: every
// raw key that string-starts with prefix, at any depth, sorted. The blob route
// lists through the held WriteLock, because BlobStore.List takes the blob's
// mutex again.
func (g *guard) ListKeys(prefix string) ([]string, error) {
	if g.released {
		return nil, fs.ErrClosed
	}
	if err := validPrefix(prefix); err != nil {
		return nil, err
	}
	if err := g.reachPrefix(prefix); err != nil {
		return nil, err
	}
	if g.blobLock != nil {
		return keysWithPrefix(g.blobLock.List(""), prefix), nil
	}
	return g.parent.keys.list(prefix)
}

// WriteKey writes one raw key through the held guard, as Storage.WriteKey
// does outside one. Storage.WriteKey would acquire the store again and hang.
func (g *guard) WriteKey(key string, data []byte) error {
	return g.write(key, data, time.Now())
}

// RemoveKey removes one raw key through the held guard, as Storage.RemoveKey
// does outside one. Storage.RemoveKey would acquire the store again and hang.
func (g *guard) RemoveKey(key string) error {
	return g.remove(key)
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
	if err := g.reach(key); err != nil {
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
	if err := g.reach(key); err != nil {
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
	if err := g.reach(key); err != nil {
		// Has cannot return the refusal, and false would read as an absent
		// key: a runtime guard asked for a configuration key is a caller defect.
		panic("BUG: " + err.Error())
	}
	if g.blobLock != nil {
		return g.blobLock.Has(key)
	}
	return g.parent.keys.has(key)
}
func (g *guard) SetModifier(modifier string) { g.modifier = modifier }
func (g *guard) WriteVersion(name string, data []byte, stamp time.Time) error {
	if err := g.parent.CheckName(name); err != nil {
		return err
	}
	return writeVersionObject(g, zefs.KeyFileVersion.Key(FormatVersionStamp(stamp), filepath.Base(name)), data, stamp)
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
	g.parent.mu.Lock()
	if err == nil {
		maps.Copy(g.parent.metas, g.pending)
		for key := range g.removed {
			delete(g.parent.metas, key)
		}
	}
	observer := g.parent.observer
	g.parent.mu.Unlock()
	g.unlock()
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
