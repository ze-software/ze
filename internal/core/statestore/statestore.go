// Design: docs/architecture/zefs-format.md -- daemon-owned runtime state.
// Package statestore shares the config system's lifetime-owned Storage handle.
// Put, Get and Remove retain best-effort semantics for existing callers. Strict
// callers use Read, Write, Delete and Increment, which refuse unavailable storage.
package statestore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"sync"
)

// ErrUnavailable means the daemon has no persistent store.
var ErrUnavailable = errors.New("runtime state unavailable: initialize a persistent store with ze init")

// ErrCorrupt means a counter cannot be decoded without losing its history.
var ErrCorrupt = errors.New("runtime state is corrupt")

// shared serializes state mutations, including read-modify-write counters. Store
// replacement waits for pending operations; callers MUST unregister before Close.
var shared struct {
	sync.Mutex
	store Storage
}

// SetStore registers the daemon's owned handle. The owner MUST SetStore(nil)
// before closing it. Safe for concurrent use.
func SetStore(store Storage) {
	shared.Lock()
	defer shared.Unlock()
	shared.store = store
}

// Store returns the registered handle, or nil. The daemon retains ownership.
func Store() Storage {
	shared.Lock()
	defer shared.Unlock()
	return shared.store
}

// Put persists data, or returns false, nil when persistence is unavailable.
func Put(key string, data []byte) (bool, error) {
	err := Write(key, data)
	if errors.Is(err, ErrUnavailable) {
		return false, nil
	}
	return err == nil, err
}

// Get retains the best-effort contract: unreadable and absent keys return false.
func Get(key string) ([]byte, bool) {
	data, found, err := Read(key)
	if err != nil {
		return nil, false
	}
	return data, found
}

// Remove deletes a key, with absence and unavailable storage treated as no-ops.
func Remove(key string) error {
	err := Delete(key)
	if errors.Is(err, ErrUnavailable) {
		return nil
	}
	return err
}

// Read distinguishes an absent key (false, nil) from every read failure.
func Read(key string) ([]byte, bool, error) {
	shared.Lock()
	defer shared.Unlock()
	if shared.store == nil {
		return nil, false, ErrUnavailable
	}
	data, err := shared.store.ReadKey(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Write acknowledges only a successful durable write through the owned handle.
func Write(key string, data []byte) error {
	shared.Lock()
	defer shared.Unlock()
	if shared.store == nil {
		return ErrUnavailable
	}
	return shared.store.WriteKey(key, data)
}

// Delete strictly requires storage. An already absent key is successful.
func Delete(key string) error {
	shared.Lock()
	defer shared.Unlock()
	if shared.store == nil {
		return ErrUnavailable
	}
	err := shared.store.RemoveKey(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// List returns raw state keys beneath prefix, refusing unavailable storage.
func List(prefix string) ([]string, error) {
	shared.Lock()
	defer shared.Unlock()
	if shared.store == nil {
		return nil, ErrUnavailable
	}
	return shared.store.ListKeys(prefix)
}

// Increment atomically advances a big-endian uint32 counter and returns it only
// after persistence succeeds. Absent counters start at one. Corrupt or exhausted
// counters are refused so a restart can never reuse an earlier sequence space.
func Increment(key string) (value uint32, err error) {
	shared.Lock()
	defer shared.Unlock()
	if shared.store == nil {
		return 0, ErrUnavailable
	}
	guard, err := shared.store.AcquireLock(key)
	if err != nil {
		return 0, err
	}
	defer func() {
		if releaseErr := guard.Release(); releaseErr != nil {
			value = 0
			err = errors.Join(err, releaseErr)
		}
	}()
	data, err := guard.ReadFile(key)
	var previous uint32
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return 0, err
	case len(data) != 4:
		return 0, fmt.Errorf("%w: counter %s needs four bytes", ErrCorrupt, key)
	default:
		previous = binary.BigEndian.Uint32(data)
	}
	if previous == math.MaxUint32 {
		return 0, fmt.Errorf("counter %s exhausted", key)
	}
	var next [4]byte
	binary.BigEndian.PutUint32(next[:], previous+1)
	if err := guard.WriteFile(key, next[:], 0); err != nil {
		return 0, err
	}
	return previous + 1, nil
}
