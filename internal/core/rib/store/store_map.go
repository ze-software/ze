//go:build maprib

// Design: docs/architecture/plugin/rib-storage-design.md -- generic prefix-keyed store (map-only fallback)
// Related: store_bart.go -- default BART backend under `!maprib`

package store

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Store is the map-only Store variant enabled via `go build -tags maprib`.
// Keys are netip.Prefix; the public surface matches the default (BART-backed)
// variant.
//
// Concurrency: NOT safe for concurrent use. Callers own synchronization.
//
// Iteration contract: the pfx passed to Iterate / ModifyAll callbacks is a
// value; callbacks may retain it. Callbacks MUST NOT call Insert, Delete, or
// Modify on the same Store during iteration.
type Store[T any] struct {
	fam    family.Family
	routes map[netip.Prefix]T
}

// NewStore creates a Store for the given family using map-only storage.
func NewStore[T any](fam family.Family) *Store[T] {
	return &Store[T]{fam: fam, routes: make(map[netip.Prefix]T)}
}

// Insert stores v under pfx. An invalid Prefix is silently ignored.
func (s *Store[T]) Insert(pfx netip.Prefix, v T) {
	if !pfx.IsValid() {
		return
	}
	s.routes[pfx] = v
}

// Lookup returns the value stored for pfx. Returns (zero, false) if absent.
func (s *Store[T]) Lookup(pfx netip.Prefix) (T, bool) {
	v, ok := s.routes[pfx]
	return v, ok
}

// Delete removes the entry for pfx. Returns true when an entry existed.
func (s *Store[T]) Delete(pfx netip.Prefix) bool {
	if _, ok := s.routes[pfx]; !ok {
		return false
	}
	delete(s.routes, pfx)
	return true
}

// Len returns the number of stored entries.
func (s *Store[T]) Len() int { return len(s.routes) }

// Iterate visits every entry. A callback return of false stops iteration.
func (s *Store[T]) Iterate(fn func(pfx netip.Prefix, v T) bool) {
	for pfx, v := range s.routes {
		if !fn(pfx, v) {
			return
		}
	}
}

// Modify calls fn with a pointer to the entry for pfx. Mutations persist.
// Returns false if the entry is absent.
func (s *Store[T]) Modify(pfx netip.Prefix, fn func(*T)) bool {
	v, ok := s.routes[pfx]
	if !ok {
		return false
	}
	fn(&v)
	s.routes[pfx] = v
	return true
}

// ModifyAll visits every entry with pointer access; mutations persist.
func (s *Store[T]) ModifyAll(fn func(*T)) {
	for pfx, v := range s.routes {
		fn(&v)
		s.routes[pfx] = v
	}
}

func (s *Store[T]) ModifyAllKeyed(fn func(pfx netip.Prefix, v *T)) {
	for pfx, v := range s.routes {
		fn(pfx, &v)
		s.routes[pfx] = v
	}
}

// Reset clears every entry. Callers that need per-entry cleanup must run
// ModifyAll first.
func (s *Store[T]) Reset() {
	s.routes = make(map[netip.Prefix]T)
}

// Family returns the family this store was constructed for.
func (s *Store[T]) Family() family.Family { return s.fam }
