// Design: ai/rules/architecture.md -- runtime state persists in the managed
// zefs store, never as loose files.
//
// Package statestore is the sanctioned way for in-core plugins and components to
// persist best-effort runtime STATE (rolling baselines, snapshots, sequence
// numbers, last-known time) into the shared zefs store (database.zefs) under a
// registered pkg/zefs key, instead of writing loose files with raw os.WriteFile.
// On the gokrazy appliance the store lives on the writable /perm partition and is
// the one integrity-checked, backed-up artifact; loose state files escape that.
//
// CRITICAL -- one shared BlobStore, not per-call opens. The config system opens
// database.zefs ONCE at startup and holds that single *zefs.BlobStore for the
// process lifetime; it never reloads from disk. A flush re-encodes the whole file
// from that instance's in-memory tree. If state were written through a SEPARATE
// transient BlobStore of the same file, the config store's next flush would
// re-encode from its stale tree and DROP every state key (and a state write could
// revert a concurrent config commit). So statestore writes through the config
// system's OWN handle, registered at startup via SetStore: every writer then
// shares one in-memory tree, serialized by that store's own lock. There is no
// second instance and therefore no lost-update window.
//
// Best-effort: when no store is registered (filesystem-fallback mode, or before
// startup wiring) Put/Get/Remove are no-ops, exactly as the old loose-file path
// was non-fatal.
package statestore

import (
	"sync/atomic"

	"github.com/ze-software/ze/pkg/zefs"
)

// current holds the process-wide shared store (the config system's handle), or
// nil in filesystem-fallback mode. atomic so SetStore (startup/tests) and the
// persister reads need no external lock; the store's own mutex serializes I/O.
var current atomic.Pointer[zefs.BlobStore]

// SetStore registers the process-wide zefs store so runtime-state persisters write
// into the SAME in-memory tree as config (see package doc). bs is the config
// system's own *zefs.BlobStore -- obtain it in cmd/ze with
// storage.BlobStoreFrom(resolve.Storage()). bs may be nil (filesystem-fallback
// mode): persistence is then a best-effort no-op. Call once at daemon startup;
// tests set a temp store and reset to nil on cleanup.
func SetStore(bs *zefs.BlobStore) { current.Store(bs) }

// Store returns the registered shared store, or nil in filesystem-fallback mode.
func Store() *zefs.BlobStore { return current.Load() }

// Put persists data under key in the shared store. Best-effort: a no-op
// (false, nil) when no store is registered. The write goes through the shared
// handle's WriteFile, so it is serialized with config writes and never dropped.
func Put(key string, data []byte) (bool, error) {
	bs := current.Load()
	if bs == nil {
		return false, nil
	}
	if err := bs.WriteFile(key, data, 0); err != nil {
		return false, err
	}
	return true, nil
}

// Get reads key from the shared store. ok is false when no store is registered or
// the key is absent.
func Get(key string) ([]byte, bool) {
	bs := current.Load()
	if bs == nil {
		return nil, false
	}
	data, err := bs.ReadFile(key)
	if err != nil {
		return nil, false
	}
	return data, true
}

// Remove deletes key from the shared store, best-effort (no-op when absent).
func Remove(key string) error {
	bs := current.Load()
	if bs == nil || !bs.Has(key) {
		return nil
	}
	return bs.Remove(key)
}
