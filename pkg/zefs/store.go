// Design: docs/architecture/zefs-format.md -- ZeFS file format and netcapstring framing
// Detail: guard.go -- WriteGuard and ReadGuard interfaces
// Detail: lock.go -- in-process locking (ReadLock, WriteLock)
// Detail: netcapstring.go -- netcapstring encoding/decoding
// Detail: tree.go -- in-memory tree for ReadDir
// Detail: file.go -- fs.File and fs.DirEntry wrappers
// Detail: mmap_unix.go -- mmap/munmap for zero-copy reads
// Detail: mmap_other.go -- heap fallback for non-unix
// Detail: registry.go -- key registry for centralized key definitions
// Related: ../../cmd/ze/internal/ssh/client/client.go — reads SSH credentials from meta/ssh/*

package zefs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const magic = "ZeFS"

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
	mu       sync.RWMutex
	path     string
	folder   *os.Root
	name     string
	alias    *blobAlias // Non-nil until a path constructor replaces an accepted symlink.
	confined bool       // Rooted constructors require an unaliased inode before in-place writes.
	root     *node
	keys     []string            // ordered keys for deterministic serialization
	slots    map[string]slotInfo // per-key slot capacities
	backing  []byte              // mmap'd region or heap buffer; tree nodes reference this
	fd       *os.File            // non-nil when backing is mmap'd (must stay open)

	dirty         map[string]bool // entries modified since last flush
	added         []string        // new entries since last flush (ordered)
	layoutChanged bool            // set on remove or capacity overflow
	policy        writePolicy     // spare capacity for new and grown slots
}

// Create creates a new empty store at the given path. Without a Spare option
// the store pads new and grown slots by SpareDefault percent.
func Create(path string, opts ...Option) (*BlobStore, error) {
	folder, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer folder.Close() //nolint:errcheck // The store retains its own handle.
	s, err := newBlobIn(folder, filepath.Base(path), opts)
	if err != nil {
		return nil, err
	}
	if err := s.flush(); err != nil {
		return nil, errors.Join(fmt.Errorf("zefs: create %s: %w", path, err), s.Close())
	}
	return s, nil
}

// CreateIn creates an artifact in an already opened directory. name MUST be one
// filename. The caller retains folder and MUST Close the returned store.
// The store retains its own directory handle, including across directory moves.
func CreateIn(folder *os.Root, name string, opts ...Option) (*BlobStore, error) {
	s, err := newBlobIn(folder, name, opts)
	if err != nil {
		return nil, err
	}
	s.confined = true
	previous, err := s.openBacking()
	if err == nil {
		if err := previous.Close(); err != nil {
			return nil, errors.Join(err, s.Close())
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, errors.Join(err, s.Close())
	}
	if err := s.flush(); err != nil {
		return nil, errors.Join(fmt.Errorf("zefs: create %s: %w", s.path, err), s.Close())
	}
	return s, nil
}

func newBlobIn(folder *os.Root, name string, opts []Option) (*BlobStore, error) {
	if folder == nil {
		return nil, fmt.Errorf("zefs: no artifact directory: open the directory before opening a blob")
	}
	if !fs.ValidPath(name) {
		return nil, fmt.Errorf("zefs: invalid artifact name %q: use one filename", name)
	}
	if name == "." {
		return nil, fmt.Errorf("zefs: artifact name is a directory: use one filename")
	}
	if filepath.Base(name) != name {
		return nil, fmt.Errorf("zefs: artifact name %q contains a directory: use one filename", name)
	}
	policy, err := newWritePolicy(opts)
	if err != nil {
		return nil, err
	}
	held, err := folder.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	return &BlobStore{
		path:   filepath.Join(folder.Name(), name),
		folder: held,
		name:   name,
		root:   newDirNode(),
		slots:  make(map[string]slotInfo),
		dirty:  make(map[string]bool),
		policy: policy,
	}, nil
}

// Open opens an existing store, memory-mapping the file for zero-copy reads.
// The policy is not read from the file: without a Spare option the store pads
// the slots it writes by SpareDefault percent, whatever the file was written with.
func Open(path string, opts ...Option) (*BlobStore, error) {
	folder, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer folder.Close() //nolint:errcheck // The store retains its own handle.
	s, err := newBlobPath(folder, filepath.Base(path), opts)
	if err != nil {
		return nil, err
	}
	if err := s.load(); err != nil {
		return nil, errors.Join(fmt.Errorf("zefs: open %s: %w", path, err), s.Close())
	}
	return s, nil
}

// OpenIn opens an artifact in an already opened directory. name MUST be one
// filename. The caller retains folder and MUST Close the returned store.
func OpenIn(folder *os.Root, name string, opts ...Option) (*BlobStore, error) {
	s, err := newBlobIn(folder, name, opts)
	if err != nil {
		return nil, err
	}
	s.confined = true
	if err := s.load(); err != nil {
		return nil, errors.Join(fmt.Errorf("zefs: open %s: %w", s.path, err), s.Close())
	}
	return s, nil
}

// OpenFileIn loads an acquired artifact without reopening its name. The caller
// MUST validate any ownership/mode contract on file before calling. This method
// takes ownership of file on success AND error; the caller retains folder.
// name is the publication leaf for later writes, which use rooted confinement.
// Loading is bounded by the import limit, not an immutable-content snapshot.
func OpenFileIn(folder *os.Root, name string, file *os.File, opts ...Option) (*BlobStore, error) {
	if file == nil {
		return nil, fmt.Errorf("zefs: no artifact descriptor: open the artifact before loading it")
	}
	s, err := newBlobIn(folder, name, opts)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	s.confined = true
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close(), s.Close())
	}
	if !info.Mode().IsRegular() {
		return nil, errors.Join(fmt.Errorf("zefs: artifact is not regular: use a regular file"), file.Close(), s.Close())
	}
	if info.Size() > maxImportSize {
		return nil, errors.Join(fmt.Errorf("zefs: artifact exceeds maximum size %d bytes", maxImportSize), file.Close(), s.Close())
	}
	if err := s.loadWritten(file); err != nil {
		return nil, errors.Join(err, s.Close())
	}
	return s, nil
}

// Close releases the memory mapping and retained directory. Callers of Create,
// CreateIn, Open and OpenIn MUST call Close. Repeated Close calls are safe.
func (s *BlobStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.unload()
	err = errors.Join(err, s.closeAlias())
	if s.folder != nil {
		err = errors.Join(err, s.folder.Close())
		s.folder = nil
	}
	return err
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

	if s.root.has(name) {
		sl := s.slots[name]
		if len(data) > sl.data.capacity {
			sl.data.capacity = s.policy.capacity(len(data))
			s.slots[name] = sl
			s.layoutChanged = true
		}
		s.dirty[name] = true
	} else {
		s.keys = append(s.keys, name)
		s.slots[name] = slotInfo{
			name: netcapSlot{capacity: s.policy.capacity(len(name))},
			data: netcapSlot{capacity: s.policy.capacity(len(data))},
		}
		s.added = append(s.added, name)
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
	delete(s.dirty, name)
	s.layoutChanged = true
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
	s.resetDirty()

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
	file, err := s.openBacking()
	if err != nil {
		return err
	}
	return s.loadWritten(file)
}

func (s *BlobStore) openBacking() (*os.File, error) {
	if s.folder == nil {
		return nil, fs.ErrClosed
	}
	folder, name := s.backingName()
	return openBlobFile(folder, name, os.O_RDONLY)
}

func (s *BlobStore) readBacking() ([]byte, error) {
	file, err := s.openBacking()
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // Read-only snapshot.
	return io.ReadAll(file)
}

// loadWritten memory-maps the descriptor atomicWrite returned, and decodes the
// tree from it. It takes ownership of written: the mapping keeps the inode
// alive, so the reload cannot fail on a name another process moved or removed
// in the meantime.
func (s *BlobStore) loadWritten(written *os.File) error {
	data, fd, err := loadBackingFile(written)
	if err != nil {
		return err
	}
	return s.install(data, fd)
}

// install replaces the backing buffer with data and decodes the tree from it.
func (s *BlobStore) install(data []byte, fd *os.File) error {
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

	// Phase 2: compute magic + container total size
	containerCap := entriesSize
	magicLen := netcapstringTotalLen(len(magic))
	totalSize := magicLen + netcapstringTotalLen(containerCap)

	// Phase 3: single allocation, write everything in place
	result := make([]byte, totalSize)
	off := writeNetcapstring(result, 0, []byte(magic), len(magic))

	// Container header (placeholder CRC, patched after entries are written)
	containerHdrOff := off
	off += writeNetcapstringHeader(result, off, containerCap, entriesSize, 0)
	containerDataOff := off

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
	// Space-fill container padding (human-readable), then trailing '\n' for container
	containerEnd := len(result) - 1
	for i := off; i < containerEnd; i++ {
		result[i] = ' '
	}
	result[containerEnd] = '\n'

	// Patch container CRC over the used data region
	containerCRC := crc32.Checksum(result[containerDataOff:containerDataOff+entriesSize], CRC32cTable)
	writeNetcapstringHeader(result, containerHdrOff, containerCap, entriesSize, containerCRC)

	return result
}

// decodeInto parses store bytes into fresh temporary structures.
// Returns the tree, keys, and slots without modifying any BlobStore.
// Used by Import to validate before committing, and by decode for normal loading.
func decodeInto(data []byte) (*node, []string, map[string]slotInfo, error) {
	// Decode magic netcapstring
	magicData, _, magicNext, err := DecodeNetcapstringRef(data, 0)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("zefs: magic: %w", err)
	}
	if string(magicData) != magic {
		return nil, nil, nil, fmt.Errorf("zefs: invalid magic: %q", magicData)
	}

	containerData, containerCap, _, err := DecodeNetcapstringRef(data, magicNext)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("zefs: container: %w", err)
	}

	// Compute where container data starts in the backing buffer.
	// Entry offsets within containerData are relative; add this base to get backing-absolute offsets.
	containerDataBase := magicNext + netcapstringHeaderLen(containerCap)

	root := newDirNode()
	var keys []string
	slots := make(map[string]slotInfo)

	off := 0
	for off < len(containerData) {
		if containerData[off] == '\n' || containerData[off] == 0 || containerData[off] == ' ' || containerData[off] == ',' {
			break
		}

		nameOff := off
		nameData, nameCap, next, err := DecodeNetcapstringRef(containerData, off)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("zefs: entry name at %d: %w", off, err)
		}
		off = next

		dataOff := off
		fileData, dataCap, next, err := DecodeNetcapstringRef(containerData, off)
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
// If the write or reload fails, the tree is rebuilt from the previous
// on-disk state so in-memory never diverges from persisted data.
func (s *BlobStore) flush() error {
	if !s.layoutChanged && len(s.added) == 0 && len(s.dirty) > 0 && s.backing != nil {
		// Snapshot before pwrite so flushFull fallback has clean recovery data
		preSnapshot, snapshotErr := s.readBacking()
		if snapshotErr != nil {
			return snapshotErr
		}
		if err := s.flushInPlace(); err == nil {
			s.resetDirty()
			return nil
		} else if errors.Is(err, fs.ErrPermission) {
			return err // A refused write target must not enter recovery publication.
		}
		// In-place failed (may have partially written). Restore clean file before fallback.
		//
		// atomicWrite, NOT os.WriteFile: os.WriteFile opens O_TRUNC on the SAME
		// inode, and other ze processes hold this file mapped PROT_READ|MAP_PRIVATE
		// (mmap_unix.go) with config nodes as zero-copy slices into it. zefs's lock
		// is an in-process sync.RWMutex (lock.go), so nothing excludes them.
		// Truncating under a live mapping invalidates its pages and the next read
		// takes SIGBUS -- "unexpected fault address / fatal error: fault", which is
		// how test/ospf/ospf-ldp-sync-restore.ci died in the 2026-07-29 verify.
		// os.Rename installs a NEW inode, so existing mappings stay valid.
		if len(preSnapshot) > 0 {
			// best-effort restore before the flushFull fallback, which reloads
			// the store itself, so the returned handle has no use here
			if restored, restoreErr := s.atomicWrite(preSnapshot); restoreErr == nil {
				restored.Close() //nolint:errcheck // best-effort restore path
			}
		}
	}
	return s.flushFull()
}

func (s *BlobStore) flushFull() error {
	var oldEncoded []byte
	// An initial Create has no loaded state to recover after a failed write.
	if s.backing != nil {
		var snapshotErr error
		oldEncoded, snapshotErr = s.readBacking()
		if snapshotErr != nil && !errors.Is(snapshotErr, fs.ErrNotExist) {
			return snapshotErr
		}
	}

	encoded := s.encode()

	if err := s.unload(); err != nil {
		return fmt.Errorf("zefs: flush unload: %w", err)
	}

	written, err := s.atomicWrite(encoded)
	if err != nil {
		if len(oldEncoded) > 0 {
			s.recoverFromEncoded(oldEncoded)
		} else {
			s.recoverFromEncoded(encoded)
		}
		return err
	}

	if err := s.loadWritten(written); err != nil {
		s.recoverFromEncoded(encoded)
		return fmt.Errorf("zefs: flush reload: %w", err)
	}
	s.resetDirty()
	return nil
}

func (s *BlobStore) flushInPlace() error {
	var regions []writeRegion

	for name := range s.dirty {
		data, ok := s.root.get(name)
		if !ok {
			continue
		}
		sl := s.slots[name]

		// Build the full netcapstring for this entry's data slot
		buf := make([]byte, sl.data.totalLen())
		writeNetcapstring(buf, 0, data, sl.data.capacity)
		regions = append(regions, writeRegion{offset: sl.data.offset, data: buf})
	}

	folder, name := s.backingName()
	if err := pwriteRegions(folder, name, regions, s.confined); err != nil {
		return err
	}

	// Re-read the file to compute container CRC, then patch the container header
	fileData, err := s.readBacking()
	if err != nil {
		return fmt.Errorf("zefs: flush in-place re-read: %w", err)
	}

	containerHdrOff := netcapstringTotalLen(len(magic))
	containerCap := s.containerCapacity()
	containerHdrLen := netcapstringHeaderLen(containerCap)
	containerDataOff := containerHdrOff + containerHdrLen

	// Bound the slice against what was actually read back. The container capacity
	// is in-memory state and the file is whatever is on disk right now; when they
	// disagree the old unchecked slice panicked out of the plugin goroutine
	// ("slice bounds out of range [:2876] with capacity 512", seen from
	// loadOSPFBootCount under a contended run). An error here is not a lost
	// write: flush() restores the pre-pwrite snapshot and falls back to
	// flushFull, which re-encodes the whole file from memory
	// (ai/rules/evidence.md -- fail closed, and say which two numbers
	// disagreed).
	if end := containerDataOff + containerCap; end > len(fileData) {
		return fmt.Errorf("zefs: flush in-place: %s is %d bytes, container needs %d (offset %d + capacity %d)",
			s.path, len(fileData), end, containerDataOff, containerCap)
	}
	containerCRC := crc32.Checksum(fileData[containerDataOff:containerDataOff+containerCap], CRC32cTable)
	hdrBuf := make([]byte, containerHdrLen)
	writeNetcapstringHeader(hdrBuf, 0, containerCap, containerCap, containerCRC)

	if err := pwriteRegions(folder, name, []writeRegion{{offset: containerHdrOff, data: hdrBuf}}, s.confined); err != nil {
		return err
	}

	// Re-mmap so read path sees the changes
	if err := s.unload(); err != nil {
		return fmt.Errorf("zefs: flush in-place unload: %w", err)
	}
	if err := s.load(); err != nil {
		return fmt.Errorf("zefs: flush in-place reload: %w", err)
	}
	return nil
}

func (s *BlobStore) containerCapacity() int {
	cap_ := 0
	for _, key := range s.keys {
		if !s.root.has(key) {
			continue
		}
		sl := s.slots[key]
		cap_ += sl.name.totalLen()
		cap_ += sl.data.totalLen()
	}
	cap_++ // trailing '\n'
	return cap_
}

func (s *BlobStore) resetDirty() {
	for k := range s.dirty {
		delete(s.dirty, k)
	}
	s.added = s.added[:0]
	s.layoutChanged = false
}

// atomicWrite publishes through the retained directory and returns the staged
// inode's handle. The caller MUST close it or pass it to loadBackingFile.
// Neither publication nor reload resolves the directory's former pathname.
func (s *BlobStore) atomicWrite(data []byte) (handle *os.File, retErr error) {
	if s.folder == nil {
		return nil, fs.ErrClosed
	}
	tmpName := ".zefs-" + rand.Text() + ".tmp"
	tmp, err := s.folder.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("zefs: flush create temp: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			s.folder.Remove(tmpName) //nolint:errcheck // Best-effort cleanup of unpublished data.
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return nil, fmt.Errorf("zefs: flush write: %w (close: %w)", err, closeErr)
		}
		return nil, fmt.Errorf("zefs: flush write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close() //nolint:errcheck // already failing on sync path
		return nil, fmt.Errorf("zefs: flush fsync temp: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Join(err, tmp.Close())
	}
	handle = tmp
	defer func() {
		if retErr != nil {
			tmp.Close() //nolint:errcheck // Cleanup on a failed commit.
		}
	}()
	if err := s.folder.Rename(tmpName, s.name); err != nil {
		return nil, fmt.Errorf("zefs: flush rename: %w", err)
	}
	committed = true
	if err := s.closeAlias(); err != nil {
		return nil, err
	}
	dirFd, err := s.folder.Open(".")
	if err != nil {
		return nil, err
	}
	if err := errors.Join(dirFd.Sync(), dirFd.Close()); err != nil {
		return nil, err
	}
	return handle, nil
}

// recoverFromEncoded rebuilds the tree from an encoded copy after a failed
// write or reload. The tree nodes reference the encoded buffer (heap-backed,
// fd=nil so unloadBacking skips munmap).
func (s *BlobStore) recoverFromEncoded(encoded []byte) {
	s.backing = encoded
	s.fd = nil
	if err := s.decode(encoded); err != nil {
		s.root = newDirNode()
		s.keys = nil
		s.slots = make(map[string]slotInfo)
	}
	s.resetDirty()
}
