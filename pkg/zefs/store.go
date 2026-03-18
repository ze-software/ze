// Design: docs/architecture/zefs-format.md -- ZeFS file format and netcapstring framing
// Detail: guard.go -- WriteGuard and ReadGuard interfaces
// Detail: lock.go -- in-process locking (ReadLock, WriteLock)
// Detail: netcapstring.go -- netcapstring encoding/decoding
// Detail: tree.go -- in-memory tree for ReadDir
// Detail: file.go -- fs.File and fs.DirEntry wrappers
// Detail: mmap_unix.go -- mmap/munmap for zero-copy reads
// Detail: mmap_other.go -- heap fallback for non-unix

package zefs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	magic          = "ZeFS"
	initialNameCap = 64
	initialDataCap = 256
)

// slotInfo tracks a key-value entry's disk layout (two netcapstrings).
type slotInfo struct {
	name netcapSlot
	data netcapSlot
}

// BlobStore is a netcapstring-framed blob store with hierarchical keys.
// It implements fs.FS, fs.ReadFileFS, and fs.ReadDirFS.
//
// ReadFile and Open return caller-owned copies. For zero-copy access,
// use RLock()/Lock() to acquire a ReadLock/WriteLock guard -- slices
// returned by the guard's ReadFile are valid for the lock duration.
// Do not retain lock-scoped slices past Release().
type BlobStore struct {
	mu      sync.RWMutex
	path    string
	root    *node
	keys    []string            // ordered keys for deterministic serialization
	slots   map[string]slotInfo // per-key slot capacities
	backing []byte              // mmap'd region or heap buffer; tree nodes reference this
	fd      *os.File            // non-nil when backing is mmap'd (must stay open)
}

// Create creates a new empty store at the given path.
func Create(path string) (*BlobStore, error) {
	s := &BlobStore{
		path:  path,
		root:  newDirNode(),
		slots: make(map[string]slotInfo),
	}
	if err := s.flush(); err != nil {
		return nil, fmt.Errorf("zefs: create %s: %w", path, err)
	}
	return s, nil
}

// Open opens an existing store, memory-mapping the file for zero-copy reads.
func Open(path string) (*BlobStore, error) {
	s := &BlobStore{
		path:  path,
		root:  newDirNode(),
		slots: make(map[string]slotInfo),
	}
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("zefs: open %s: %w", path, err)
	}
	return s, nil
}

// Close releases the memory mapping and any associated resources.
// After Close, slices returned by ReadFile or Open are no longer valid.
func (s *BlobStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unload()
}

// ReadFile returns a copy of the named file's contents.
// The caller owns the returned slice and may modify it freely.
// This satisfies the fs.ReadFileFS contract (caller-owned bytes).
func (s *BlobStore) ReadFile(name string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.readFile(name)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func (s *BlobStore) readFile(name string) ([]byte, error) {
	data, ok := s.root.get(name)
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return data, nil
}

// Open opens the named file, implementing fs.FS.
func (s *BlobStore) Open(name string) (fs.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name == "." {
		return &storeDir{node: s.root, name: "."}, nil
	}

	nd := s.root.walk(name)
	if nd == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	if nd.data != nil {
		parts := strings.Split(name, "/")
		// Copy data so the returned fs.File is not backed by mmap memory
		// that may be invalidated by a subsequent flush or Close.
		dataCopy := make([]byte, len(nd.data))
		copy(dataCopy, nd.data)
		return &storeFile{name: parts[len(parts)-1], data: dataCopy}, nil
	}

	parts := strings.Split(name, "/")
	return &storeDir{node: nd, name: parts[len(parts)-1]}, nil
}

// ReadDir returns directory entries for the named directory.
func (s *BlobStore) ReadDir(name string) ([]fs.DirEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readDir(name)
}

func (s *BlobStore) readDir(name string) ([]fs.DirEntry, error) {
	nd := s.root.walk(name)
	if nd == nil || nd.children == nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}

	entries := make([]fs.DirEntry, 0, len(nd.children))
	for childName, child := range nd.children {
		entries = append(entries, &storeDirEntry{
			entryName: childName,
			isDir:     child.children != nil,
			size:      len(child.data),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}

// WriteFile creates or updates the named file.
// The perm argument is accepted for Go proposal #67002 compatibility but ignored.
// For goroutine safety, use Lock() to acquire a WriteLock for batched writes.
func (s *BlobStore) WriteFile(name string, data []byte, perm fs.FileMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writeFileNoFlush(name, data, perm); err != nil {
		return err
	}
	return s.flush()
}

func (s *BlobStore) writeFileNoFlush(name string, data []byte, _ fs.FileMode) error {
	if !fs.ValidPath(name) || name == "." {
		return &fs.PathError{Op: "write", Path: name, Err: fs.ErrInvalid}
	}
	stored := make([]byte, len(data))
	copy(stored, data)

	if !s.root.has(name) {
		s.keys = append(s.keys, name)
		s.slots[name] = slotInfo{
			name: netcapSlot{capacity: growCapacity(len(name), initialNameCap)},
			data: netcapSlot{capacity: growCapacity(len(data), initialDataCap)},
		}
	} else {
		sl := s.slots[name]
		if len(data) > sl.data.capacity {
			sl.data.capacity = growCapacity(len(data), sl.data.capacity)
			s.slots[name] = sl
		}
	}

	return s.root.set(name, stored)
}

// Remove deletes the named file.
// For goroutine safety, use Lock() to acquire a WriteLock for batched writes.
func (s *BlobStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.removeNoFlush(name); err != nil {
		return err
	}
	return s.flush()
}

func (s *BlobStore) removeNoFlush(name string) error {
	if !s.root.remove(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	for i, k := range s.keys {
		if k == name {
			s.keys = append(s.keys[:i], s.keys[i+1:]...)
			break
		}
	}
	delete(s.slots, name)
	return nil
}

// Has returns true if the named file exists.
func (s *BlobStore) Has(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root.has(name)
}

// List returns all file keys under the given prefix.
// Use "" to list all keys.
func (s *BlobStore) List(prefix string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.list(prefix)
}

func (s *BlobStore) list(prefix string) []string {
	nd := s.root
	if prefix != "" {
		nd = s.root.walk(prefix)
		if nd == nil {
			return nil
		}
	}
	var result []string
	nd.collect(prefix, &result)
	return result
}

// Export writes the raw store bytes to w.
func (s *BlobStore) Export(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := s.encode()
	_, err := w.Write(data)
	return err
}

// maxImportSize is the maximum allowed import size (256 MB).
// Prevents memory exhaustion from untrusted or corrupted input.
const maxImportSize = 256 * 1024 * 1024

// maxEntryCount is the maximum number of entries allowed in a store.
// Prevents memory exhaustion from crafted blobs with millions of tiny entries.
// 100,000 entries is far beyond any real config storage use case.
const maxEntryCount = 100_000

// Import replaces the store contents from r.
// The input is validated into temporary structures before committing,
// so the store is unchanged if the input is malformed.
// For goroutine safety, use Lock() to acquire a WriteLock for batched writes.
func (s *BlobStore) Import(r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := io.ReadAll(io.LimitReader(r, maxImportSize+1))
	if err != nil {
		return fmt.Errorf("zefs: import: %w", err)
	}
	if len(data) > maxImportSize {
		return fmt.Errorf("zefs: import exceeds maximum size %d bytes", maxImportSize)
	}

	// Decode into temporary structures to validate before committing.
	tmpRoot, tmpKeys, tmpSlots, err := decodeInto(data)
	if err != nil {
		return fmt.Errorf("zefs: import: %w", err)
	}

	if err := s.unload(); err != nil {
		return fmt.Errorf("zefs: import unload: %w", err)
	}

	s.root = tmpRoot
	s.keys = tmpKeys
	s.slots = tmpSlots

	return s.flush()
}

// slot returns the slotInfo for a key (used by tests).
func (s *BlobStore) slot(name string) (slotInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sl, ok := s.slots[name]
	return sl, ok
}

// load memory-maps the file and decodes the tree with zero-copy references.
func (s *BlobStore) load() error {
	data, fd, err := loadBacking(s.path)
	if err != nil {
		return err
	}

	if err := s.unload(); err != nil {
		// Best effort: release newly loaded backing if old unload fails
		unloadErr := unloadBacking(data, fd)
		if unloadErr != nil {
			return fmt.Errorf("zefs: load: %w (also: %w)", err, unloadErr)
		}
		return err
	}

	s.backing = data
	s.fd = fd
	return s.decode(s.backing)
}

// unload releases the backing buffer (munmap on unix, no-op on heap).
func (s *BlobStore) unload() error {
	if s.backing == nil {
		return nil
	}
	err := unloadBacking(s.backing, s.fd)
	s.backing = nil
	s.fd = nil
	return err
}

// encode serializes the store to a new byte buffer using single-allocation WriteTo.
// Safe to call while backed by mmap: data is copied out during writing.
func (s *BlobStore) encode() []byte {
	// Phase 1: compute entries total size
	entriesSize := 0
	for _, key := range s.keys {
		if !s.root.has(key) {
			continue
		}
		sl := s.slots[key]
		entriesSize += sl.name.totalLen()
		entriesSize += sl.data.totalLen()
	}
	entriesSize++ // trailing '\n'

	// Phase 2: compute container + total size
	containerCap := growCapacity(entriesSize, entriesSize)
	containerHdrLen := netcapstringHeaderLen(containerCap)
	totalSize := len(magic) + containerHdrLen + containerCap

	// Phase 3: single allocation, write everything in place
	result := make([]byte, totalSize)
	off := copy(result, magic)

	// Container header
	off += writeNetcapstringHeader(result, off, containerCap, entriesSize)

	// Entries: key-value pairs written directly into the container data region
	for _, key := range s.keys {
		data, ok := s.root.get(key)
		if !ok {
			continue
		}
		sl := s.slots[key]
		off += writeNetcapstring(result, off, []byte(key), sl.name.capacity)
		off += writeNetcapstring(result, off, data, sl.data.capacity)
	}
	result[off] = '\n'
	off++
	// Space-fill container padding (human-readable)
	for i := off; i < len(result); i++ {
		result[i] = ' '
	}

	return result
}

// decodeInto parses store bytes into fresh temporary structures.
// Returns the tree, keys, and slots without modifying any BlobStore.
// Used by Import to validate before committing, and by decode for normal loading.
func decodeInto(data []byte) (*node, []string, map[string]slotInfo, error) {
	if len(data) < len(magic) || string(data[:len(magic)]) != magic {
		return nil, nil, nil, fmt.Errorf("zefs: invalid magic: %q", data[:min(len(data), len(magic))])
	}

	containerData, containerCap, _, err := decodeNetcapstringRef(data, len(magic))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("zefs: container: %w", err)
	}

	// Compute where container data starts in the backing buffer.
	// Entry offsets within containerData are relative; add this base to get backing-absolute offsets.
	containerDataBase := len(magic) + netcapstringHeaderLen(containerCap)

	root := newDirNode()
	var keys []string
	slots := make(map[string]slotInfo)

	off := 0
	for off < len(containerData) {
		if containerData[off] == '\n' || containerData[off] == 0 || containerData[off] == ' ' {
			break
		}

		nameOff := off
		nameData, nameCap, next, err := decodeNetcapstringRef(containerData, off)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("zefs: entry name at %d: %w", off, err)
		}
		off = next

		dataOff := off
		fileData, dataCap, next, err := decodeNetcapstringRef(containerData, off)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("zefs: entry data at %d: %w", off, err)
		}
		off = next

		key := string(nameData)
		if !fs.ValidPath(key) || key == "." {
			return nil, nil, nil, fmt.Errorf("zefs: invalid key in store: %q", key)
		}
		if err := root.set(key, fileData); err != nil {
			return nil, nil, nil, fmt.Errorf("zefs: decode: %w", err)
		}
		keys = append(keys, key)
		if len(keys) > maxEntryCount {
			return nil, nil, nil, fmt.Errorf("zefs: entry count exceeds maximum %d", maxEntryCount)
		}
		slots[key] = slotInfo{
			name: netcapSlot{offset: containerDataBase + nameOff, capacity: nameCap, used: len(nameData)},
			data: netcapSlot{offset: containerDataBase + dataOff, capacity: dataCap, used: len(fileData)},
		}
	}

	return root, keys, slots, nil
}

// decode parses store bytes using zero-copy references into the backing buffer.
// Tree nodes will hold sub-slices of data (no allocation per entry).
func (s *BlobStore) decode(data []byte) error {
	root, keys, slots, err := decodeInto(data)
	if err != nil {
		return err
	}
	s.root = root
	s.keys = keys
	s.slots = slots
	return nil
}

// flush encodes, writes to disk, then reloads via mmap.
// After flush, all tree nodes reference the new backing.
// If the write or reload fails, the tree is rebuilt from the encoded
// copy as a heap-backed buffer so the store remains usable.
func (s *BlobStore) flush() error {
	// encode() copies data out of current backing (safe before unload)
	encoded := s.encode()

	// Release old backing before writing (avoids inode conflict with mmap)
	if err := s.unload(); err != nil {
		return fmt.Errorf("zefs: flush unload: %w", err)
	}

	// Atomic write: temp file in same directory, then rename.
	// os.Rename is atomic on POSIX when source and target are on the same filesystem.
	if err := s.atomicWrite(encoded); err != nil {
		s.recoverFromEncoded(encoded)
		return err
	}

	// Re-map new file; tree nodes now reference new backing
	if err := s.load(); err != nil {
		s.recoverFromEncoded(encoded)
		return fmt.Errorf("zefs: flush reload: %w", err)
	}
	return nil
}

// atomicWrite writes data to s.path via a temp file and rename.
// os.Rename is atomic on POSIX when source and target share a filesystem.
func (s *BlobStore) atomicWrite(data []byte) error {
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".zefs-*.tmp")
	if err != nil {
		return fmt.Errorf("zefs: flush create temp: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			os.Remove(tmpName) //nolint:errcheck // best-effort cleanup of temp file
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return fmt.Errorf("zefs: flush write: %w (close: %w)", err, closeErr)
		}
		return fmt.Errorf("zefs: flush write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("zefs: flush close temp: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("zefs: flush rename: %w", err)
	}
	committed = true
	return nil
}

// recoverFromEncoded rebuilds the tree from an encoded copy after a failed
// write or reload. The tree nodes reference the encoded buffer (heap-backed,
// fd=nil so unloadBacking skips munmap).
func (s *BlobStore) recoverFromEncoded(encoded []byte) {
	s.backing = encoded
	s.fd = nil
	// encoded was just produced by encode(), decode cannot fail
	if err := s.decode(encoded); err != nil {
		s.root = newDirNode()
		s.keys = nil
		s.slots = make(map[string]slotInfo)
	}
}
