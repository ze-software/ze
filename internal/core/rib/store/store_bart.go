//go:build !maprib

// Design: docs/architecture/plugin/rib-storage-design.md -- generic prefix-keyed store
// Related: store_map.go -- map-only fallback under `-tags maprib`
// Related: nlrikey.go -- NLRIToPrefix / PrefixToNLRI helpers for wire-byte callers

package store

import (
	"net/netip"

	"github.com/gaissmai/bart"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Store is a generic prefix-keyed store backed by a BART trie. Keys are
// netip.Prefix; values are held inline by BART. Callers who need per-path-id
// semantics (RFC 7911 ADD-PATH) put a path-id -> T map in the value layer
// (see internal/component/bgp/plugins/rib/storage pathSet for the BGP
// example). This keeps Store itself generic and non-branching.
//
// Concurrency: NOT safe for concurrent use. Callers own synchronization
// (typically an outer sync.RWMutex, matching RIBManager.mu).
//
// Iteration contract: the pfx passed to Iterate / ModifyAll callbacks is a
// value; callbacks may retain it. Callbacks MUST NOT call Insert, Delete, or
// Modify on the same Store during iteration -- collect keys first and mutate
// after iteration returns.
type Store[T any] struct {
	fam  family.Family
	trie *bart.Table[T]
}

// NewStore creates a Store for the given family.
func NewStore[T any](fam family.Family) *Store[T] {
	return &Store[T]{fam: fam, trie: new(bart.Table[T])}
}

// Insert stores v under pfx. An invalid Prefix is silently ignored.
func (s *Store[T]) Insert(pfx netip.Prefix, v T) {
	if !pfx.IsValid() {
		return
	}
	s.trie.Insert(pfx, v)
}

// Lookup returns the value stored for pfx. Returns (zero, false) if absent or
// if pfx is invalid.
func (s *Store[T]) Lookup(pfx netip.Prefix) (T, bool) {
	if !pfx.IsValid() {
		var zero T
		return zero, false
	}
	return s.trie.Get(pfx)
}

// Delete removes the entry for pfx. Returns true when an entry existed and
// was removed; false otherwise.
func (s *Store[T]) Delete(pfx netip.Prefix) bool {
	if !pfx.IsValid() {
		return false
	}
	if _, exists := s.trie.Get(pfx); !exists {
		return false
	}
	s.trie.Delete(pfx)
	return true
}

// Len returns the number of stored entries.
func (s *Store[T]) Len() int { return s.trie.Size() }

// Iterate visits every entry. A callback return of false stops iteration.
func (s *Store[T]) Iterate(fn func(pfx netip.Prefix, v T) bool) {
	for pfx, v := range s.trie.All() {
		if !fn(pfx, v) {
			return
		}
	}
}

// Modify calls fn with a pointer to the entry for pfx. The mutated value is
// written back on return. Returns false if the entry is absent or pfx is
// invalid.
func (s *Store[T]) Modify(pfx netip.Prefix, fn func(*T)) bool {
	if !pfx.IsValid() {
		return false
	}
	v, exists := s.trie.Get(pfx)
	if !exists {
		return false
	}
	fn(&v)
	s.trie.Insert(pfx, v)
	return true
}

// ModifyAll visits every entry. Mutations written through the pointer are
// persisted back into the store.
func (s *Store[T]) ModifyAll(fn func(*T)) {
	for pfx, v := range s.trie.All() {
		fn(&v)
		s.trie.Insert(pfx, v)
	}
}

// Reset clears every entry. The backing trie is replaced with an empty one.
// Callers that need per-entry cleanup (e.g. releasing pool handles on
// RouteEntry) must run ModifyAll first.
func (s *Store[T]) Reset() {
	s.trie = new(bart.Table[T])
}

// Family returns the family this store was constructed for.
func (s *Store[T]) Family() family.Family { return s.fam }
