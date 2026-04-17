//go:build !maprib

// Design: docs/architecture/plugin/rib-storage-design.md -- generic NLRI-keyed store
// Related: store_map.go -- map-only fallback under `-tags maprib`
// Related: familyrib_bart.go -- wraps *Store[RouteEntry] plus pool-handle lifecycle
// Related: nlrikey.go -- NLRIToPrefix / PrefixToNLRI / NLRIKey used as backend keys

package storage

import (
	"github.com/gaissmai/bart"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Store is a generic NLRI-keyed store dispatching between a BART trie
// (non-ADD-PATH) and a Go map keyed by NLRIKey (ADD-PATH). The mode is fixed
// at construction from the `addPath` argument -- non-ADD-PATH sessions never
// allocate the map; ADD-PATH sessions never allocate the trie.
//
// Stored values are held inline by BART for the trie case; the map case holds
// values by map. Lookups return a copy of the value; Modify uses
// get-mutate-insert to write back.
//
// Callers requiring both backends simultaneously (e.g. best-path state that
// spans peers with mixed ADD-PATH capability) construct two Store instances,
// one per mode, and dispatch at the call site.
//
// Concurrency: NOT safe for concurrent use. Callers own synchronization
// (typically an outer sync.RWMutex, matching RIBManager.mu).
//
// Iteration contract: the nlriBytes slice passed to Iterate / ModifyAll
// callbacks is valid only for the duration of that single callback. Callbacks
// MUST copy if the slice needs to outlive the call (see FamilyRIB.PurgeStale).
// Callbacks MUST NOT call Insert, Delete, or Modify on the same Store during
// iteration -- collect keys first and mutate after iteration returns.
type Store[T any] struct {
	fam     family.Family
	addPath bool

	trie   *bart.Table[T] // used when !addPath
	routes map[NLRIKey]T  // used when addPath
}

// NewStore creates a Store for the given family. When addPath is true the
// store uses a map keyed by NLRIKey (the 24-byte copy of the NLRI wire bytes,
// which for ADD-PATH includes a 4-byte path-id prefix). Otherwise it uses a
// BART trie keyed by netip.Prefix.
func NewStore[T any](fam family.Family, addPath bool) *Store[T] {
	s := &Store[T]{fam: fam, addPath: addPath}
	if addPath {
		s.routes = make(map[NLRIKey]T)
	} else {
		s.trie = new(bart.Table[T])
	}
	return s
}

// Insert stores v under nlriBytes. For the trie backend, malformed NLRI bytes
// (wrong prefix length for the family) are silently ignored -- the same
// behavior as FamilyRIB's trie path.
func (s *Store[T]) Insert(nlriBytes []byte, v T) {
	if s.addPath {
		s.routes[NewNLRIKey(nlriBytes)] = v
		return
	}
	pfx, ok := NLRIToPrefix(s.fam, nlriBytes)
	if !ok {
		return
	}
	s.trie.Insert(pfx, v)
}

// Lookup returns the value stored for nlriBytes. Returns (zero, false) if
// absent or, for the trie backend, if nlriBytes is malformed.
func (s *Store[T]) Lookup(nlriBytes []byte) (T, bool) {
	if s.addPath {
		v, ok := s.routes[NewNLRIKey(nlriBytes)]
		return v, ok
	}
	pfx, ok := NLRIToPrefix(s.fam, nlriBytes)
	if !ok {
		var zero T
		return zero, false
	}
	return s.trie.Get(pfx)
}

// Delete removes the entry for nlriBytes. Returns true when an entry existed
// and was removed; false otherwise.
func (s *Store[T]) Delete(nlriBytes []byte) bool {
	if s.addPath {
		key := NewNLRIKey(nlriBytes)
		if _, ok := s.routes[key]; !ok {
			return false
		}
		delete(s.routes, key)
		return true
	}
	pfx, ok := NLRIToPrefix(s.fam, nlriBytes)
	if !ok {
		return false
	}
	if _, exists := s.trie.Get(pfx); !exists {
		return false
	}
	s.trie.Delete(pfx)
	return true
}

// Len returns the number of stored entries.
func (s *Store[T]) Len() int {
	if s.addPath {
		return len(s.routes)
	}
	return s.trie.Size()
}

// Iterate visits every entry. A callback return of false stops iteration.
// For the trie backend, nlriBytes is written into a single buffer reused
// across iterations -- zero heap allocation per entry. See the type doc for
// the "valid only during callback" contract.
func (s *Store[T]) Iterate(fn func(nlriBytes []byte, v T) bool) {
	if s.addPath {
		for key, v := range s.routes {
			if !fn(key.Bytes(), v) {
				return
			}
		}
		return
	}
	var buf [17]byte // 1 prefix-len + max 16 (IPv6)
	for pfx, v := range s.trie.All() {
		nlri := PrefixToNLRIInto(pfx, buf[:])
		if nlri == nil {
			// Defensive: PrefixToNLRIInto only returns nil on an invalid
			// (bits<0) Prefix, which cannot arise via this Store's Insert
			// path. Skip rather than hand the callback a nil slice.
			continue
		}
		if !fn(nlri, v) {
			return
		}
	}
}

// Modify calls fn with a pointer to the entry for nlriBytes. The mutated value
// is written back on return. Returns false if the entry is absent or the NLRI
// is malformed.
func (s *Store[T]) Modify(nlriBytes []byte, fn func(*T)) bool {
	if s.addPath {
		key := NewNLRIKey(nlriBytes)
		v, ok := s.routes[key]
		if !ok {
			return false
		}
		fn(&v)
		s.routes[key] = v
		return true
	}
	pfx, ok := NLRIToPrefix(s.fam, nlriBytes)
	if !ok {
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
	if s.addPath {
		for key, v := range s.routes {
			fn(&v)
			s.routes[key] = v
		}
		return
	}
	for pfx, v := range s.trie.All() {
		fn(&v)
		s.trie.Insert(pfx, v)
	}
}

// Reset clears every entry. The active backend is replaced with an empty
// instance of the same kind (new trie under !maprib, fresh map under
// maprib-compatible path). Callers that need per-entry cleanup (e.g. pool
// handle release on FamilyRIB.RouteEntry) must run ModifyAll first.
func (s *Store[T]) Reset() {
	if s.addPath {
		s.routes = make(map[NLRIKey]T)
		return
	}
	s.trie = new(bart.Table[T])
}

// Family returns the family this store was constructed for.
func (s *Store[T]) Family() family.Family { return s.fam }

// AddPath reports whether this store uses the ADD-PATH map backend.
func (s *Store[T]) AddPath() bool { return s.addPath }
