// Design: docs/architecture/storage-backends.md -- the memory encoding.
// Related: store.go -- the encoding-independent operations it plugs into.

package storage

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"
)

// CreateMemory returns an empty store whose keys live in process memory and
// die with it. It is the editor test harness's store (`./le test editor`):
// the harness runs hundreds of editor sessions at once, and a commit on the
// live tree syncs every frame and directory it touches, so under load the
// tests waited on the disk rather than on the editor. It is never a daemon's
// store, because it keeps nothing across a restart.
//
// Every Storage operation is the store's own: name mapping, guards, versions,
// metadata and observers run the code the tree runs, over this key space. A
// config name is accepted in any directory, as on a blob, because the store
// has no folder to compare it with. Caller MUST Close the result.
func CreateMemory() Storage {
	result := newStore(nil, nil, nil, false)
	result.keys = &memoryEncoding{values: make(map[string][]byte)}
	return result
}

// memoryEncoding holds each key's bytes in a map. Safe for concurrent use:
// the store serializes writes under a guard, and an unguarded read runs beside
// that guard's writes, so mu covers the map.
type memoryEncoding struct {
	mu     sync.RWMutex
	values map[string][]byte
}

// ReadFile returns a copy, so an unlocked read stays caller-owned as the
// contract says, and a later write cannot change bytes a caller holds.
func (m *memoryEncoding) ReadFile(key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.values[key]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: key, Err: fs.ErrNotExist}
	}
	return slices.Clone(data), nil
}

// WriteFile refuses a key that would be both a value and a directory, as the
// tree refuses it: a key under an existing value, or a value over existing keys.
func (m *memoryEncoding) WriteFile(key string, data []byte, _ fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range len(key) {
		if key[i] != '/' {
			continue
		}
		if _, ok := m.values[key[:i]]; ok {
			return fmt.Errorf("write %s: %s holds a value, not keys: %w", key, key[:i], fs.ErrExist)
		}
	}
	directory := key + "/"
	for existing := range m.values {
		if strings.HasPrefix(existing, directory) {
			return fmt.Errorf("write %s: it holds keys, not a value: %w", key, fs.ErrExist)
		}
	}
	// Copy so the caller may reuse its buffer. Appending onto an empty literal
	// keeps an empty value non-nil, so it reads back as empty, never as nil.
	m.values[key] = append([]byte{}, data...)
	return nil
}

// Remove deletes one value. An absent key keeps fs.ErrNotExist, as the tree's
// does, so statestore.Delete can treat absence as success.
func (m *memoryEncoding) Remove(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.values[key]; !ok {
		return &fs.PathError{Op: "remove", Path: key, Err: fs.ErrNotExist}
	}
	delete(m.values, key)
	return nil
}

func (m *memoryEncoding) has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.values[key]
	return ok
}

// list answers what the tree's list answers: every key that string-starts
// with prefix, at any depth, sorted.
func (m *memoryEncoding) list(prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for key := range m.values {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys, nil
}
