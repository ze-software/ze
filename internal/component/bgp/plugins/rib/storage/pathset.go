// Design: plan/design-rib-unified.md -- Phase 2 (ADD-PATH moves to value layer)
// Related: familyrib_bart.go -- uses pathSet as the Store value type under ADD-PATH
// Related: familyrib_map.go -- same, under -tags maprib
// Related: routeentry.go -- the per-path route data wrapped here

package storage

// pathSet holds the per-path-id RouteEntry list for a single prefix under
// RFC 7911 ADD-PATH. A non-ADD-PATH session does not use pathSet at all --
// FamilyRIB picks between the `direct` store (values are RouteEntry) and the
// `multi` store (values are pathSet) at construction. Path-id is stored here
// in the value layer rather than conflated into the store's key, so the BART
// trie remains the prefix index in every case.
//
// Typical size: 1-4 entries per prefix even under ADD-PATH. A linear scan
// over `entries` beats a map for that cardinality (no hash, no pointer chase)
// and keeps the memory footprint small. Flip to a map if profiling shows a
// peer that advertises hundreds of paths per prefix.
//
// Callers are expected to hold FamilyRIB's outer synchronization; pathSet
// itself is NOT safe for concurrent use.
type pathSet struct {
	entries []pathEntry
}

// pathEntry is one (path-id, RouteEntry) pair inside a pathSet.
type pathEntry struct {
	pathID uint32
	entry  RouteEntry
}

// upsert inserts or replaces the entry for pathID. Returns the replaced
// RouteEntry (released by the caller) plus a bool indicating whether a
// replacement happened. On new insert, (zero, false) is returned.
func (s *pathSet) upsert(pathID uint32, entry RouteEntry) (RouteEntry, bool) {
	for i := range s.entries {
		if s.entries[i].pathID == pathID {
			old := s.entries[i].entry
			s.entries[i].entry = entry
			return old, true
		}
	}
	s.entries = append(s.entries, pathEntry{pathID: pathID, entry: entry})
	return RouteEntry{}, false
}

// lookup returns the RouteEntry for pathID. Returns (zero, false) if absent.
func (s *pathSet) lookup(pathID uint32) (RouteEntry, bool) {
	for i := range s.entries {
		if s.entries[i].pathID == pathID {
			return s.entries[i].entry, true
		}
	}
	return RouteEntry{}, false
}

// remove deletes the entry for pathID. Returns (entry, true) when a removal
// happened so the caller can Release() the pool handles; (zero, false)
// otherwise.
func (s *pathSet) remove(pathID uint32) (RouteEntry, bool) {
	for i := range s.entries {
		if s.entries[i].pathID != pathID {
			continue
		}
		removed := s.entries[i].entry
		// Swap-delete: order is not observable to callers.
		last := len(s.entries) - 1
		s.entries[i] = s.entries[last]
		s.entries = s.entries[:last]
		return removed, true
	}
	return RouteEntry{}, false
}

// modify calls fn with a pointer to the entry for pathID. Returns false if
// the pathID is absent.
func (s *pathSet) modify(pathID uint32, fn func(*RouteEntry)) bool {
	for i := range s.entries {
		if s.entries[i].pathID == pathID {
			fn(&s.entries[i].entry)
			return true
		}
	}
	return false
}

// len returns the number of path-id entries in the set.
func (s *pathSet) len() int { return len(s.entries) }

// releaseAll calls Release on every stored RouteEntry and empties the set.
func (s *pathSet) releaseAll() {
	for i := range s.entries {
		s.entries[i].entry.Release()
	}
	s.entries = s.entries[:0]
}
