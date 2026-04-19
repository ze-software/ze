//go:build maprib

// Design: docs/architecture/plugin/rib-storage-design.md -- generic NLRI-keyed store (map-only fallback)
// Related: store_bart.go -- default BART+map dispatch under `!maprib`
// Related: nlrikey.go -- NLRIKey is the sole key type under maprib

package store

import (
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Store is the map-only Store variant enabled via `go build -tags maprib`.
// Both ADD-PATH and non-ADD-PATH sessions route through the same map keyed by
// NLRIKey. The public surface matches the default (BART-backed) variant; only
// the backend differs.
//
// Concurrency: NOT safe for concurrent use. Callers own synchronization.
//
// Iteration contract: the nlriBytes slice passed to Iterate / ModifyAll
// callbacks is valid only for the duration of that single callback. Callbacks
// MUST copy if the slice needs to outlive the call. Callbacks MUST NOT call
// Insert, Delete, or Modify on the same Store during iteration.
type Store[T any] struct {
	fam     family.Family
	addPath bool
	routes  map[NLRIKey]T
}

// NewStore creates a Store for the given family using map-only storage.
// The addPath argument is retained for API symmetry with the default variant
// but does not change the backend.
func NewStore[T any](fam family.Family, addPath bool) *Store[T] {
	return &Store[T]{
		fam:     fam,
		addPath: addPath,
		routes:  make(map[NLRIKey]T),
	}
}

// Insert stores v under nlriBytes.
func (s *Store[T]) Insert(nlriBytes []byte, v T) {
	s.routes[NewNLRIKey(nlriBytes)] = v
}

// Lookup returns the value stored for nlriBytes. Returns (zero, false) if absent.
func (s *Store[T]) Lookup(nlriBytes []byte) (T, bool) {
	v, ok := s.routes[NewNLRIKey(nlriBytes)]
	return v, ok
}

// Delete removes the entry for nlriBytes. Returns true when an entry existed.
func (s *Store[T]) Delete(nlriBytes []byte) bool {
	key := NewNLRIKey(nlriBytes)
	if _, ok := s.routes[key]; !ok {
		return false
	}
	delete(s.routes, key)
	return true
}

// Len returns the number of stored entries.
func (s *Store[T]) Len() int { return len(s.routes) }

// Iterate visits every entry. A callback return of false stops iteration.
func (s *Store[T]) Iterate(fn func(nlriBytes []byte, v T) bool) {
	for key, v := range s.routes {
		if !fn(key.Bytes(), v) {
			return
		}
	}
}

// Modify calls fn with a pointer to the entry for nlriBytes. Mutations are
// persisted on return. Returns false if the entry is absent.
func (s *Store[T]) Modify(nlriBytes []byte, fn func(*T)) bool {
	key := NewNLRIKey(nlriBytes)
	v, ok := s.routes[key]
	if !ok {
		return false
	}
	fn(&v)
	s.routes[key] = v
	return true
}

// ModifyAll visits every entry with pointer access; mutations persist.
func (s *Store[T]) ModifyAll(fn func(*T)) {
	for key, v := range s.routes {
		fn(&v)
		s.routes[key] = v
	}
}

// Reset clears every entry. Under maprib the backend is always the same map,
// so this simply rebuilds an empty one. Callers that need per-entry cleanup
// must run ModifyAll first.
func (s *Store[T]) Reset() {
	s.routes = make(map[NLRIKey]T)
}

// Family returns the family this store was constructed for.
func (s *Store[T]) Family() family.Family { return s.fam }

// AddPath reports whether the caller requested ADD-PATH semantics. Under
// maprib the backend is always a map, so the flag is informational only.
func (s *Store[T]) AddPath() bool { return s.addPath }
