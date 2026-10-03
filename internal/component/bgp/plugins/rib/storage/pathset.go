// Design: docs/architecture/rib/unified-locrib.md -- ADD-PATH in the value layer
// Related: familyrib_bart.go -- uses pathSet as the Store value type under ADD-PATH
// Related: familyrib_map.go -- same, under -tags maprib
// Related: routeentry.go -- the per-path route data wrapped here

package storage

import (
	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
)

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

// pathEntry is one path of a prefix inside a pathSet: its path id, its route
// data, and its MPLS label binding.
//
// labels is the path's own label handle (labeled unicast, SAFI 4), or
// attrpool.InvalidHandle when the path carries none. It sits beside the
// RouteEntry rather than in it, because sizeof_test guards RouteEntry and a
// non-labeled family would pay for it on every route. The pair packs into the
// eight bytes pathID alone would pad to, so it costs no memory.
type pathEntry struct {
	pathID uint32
	labels attrpool.Handle
	entry  RouteEntry
}

// upsert inserts or replaces the entry for pathID. Returns the replaced
// RouteEntry (released by the caller) plus a bool indicating whether a
// replacement happened. On new insert, (zero, false) is returned. A new path
// starts with no label binding.
//
// RFC 8277 Section 2.5: "If I1 is the same as I2, UPDATE U2 MUST be
// interpreted as meaning that L2 is now bound to P at N1 and that L1 is no
// longer bound to P at N1." The replacement ends L1's binding in setLabels,
// which the ingest calls for every labeled UPDATE right after the insert and
// which releases the handle it replaces. upsert keeps the binding in place,
// on purpose: the insert and the rebind take the RIB lock separately, and
// releasing here would leave the path with no label between the two, where a
// concurrent election installs the route with no label stack. The binding is
// replaced, never removed, the same way the parallel label store of a family
// without ADD-PATH replaces it (FamilyRIB.SetLabels).
func (s *pathSet) upsert(pathID uint32, entry RouteEntry) (RouteEntry, bool) {
	for i := range s.entries {
		if s.entries[i].pathID == pathID {
			old := s.entries[i].entry
			s.entries[i].entry = entry
			return old, true
		}
	}
	s.entries = append(s.entries, pathEntry{pathID: pathID, labels: attrpool.InvalidHandle, entry: entry})
	return RouteEntry{}, false
}

// refresh marks the route of pathID fresh and owned by messageID, keeping its
// route data and its label binding: the UPDATE re-announced the same route.
// Every caller looked the path up first, so an absent pathID is not reported.
func (s *pathSet) refresh(pathID uint32, messageID uint64) {
	for i := range s.entries {
		if s.entries[i].pathID == pathID {
			s.entries[i].entry.StaleLevel = StaleLevelFresh
			s.entries[i].entry.MsgID = messageID
			return
		}
	}
}

// setLabels binds h to pathID, releasing the handle the path held before.
// Returns false if the pathID is absent, so the caller still owns h.
//
// RFC 8277 Section 2.5: "If I1 is not the same as I2, U2 MUST be interpreted
// as meaning that L2 is now bound to P at N1, but U2 MUST NOT be interpreted
// as meaning that L1 is no longer bound to P at N1." Each path holds its own
// handle, so binding one path's labels leaves every other path's in place.
func (s *pathSet) setLabels(pathID uint32, h attrpool.Handle) bool {
	for i := range s.entries {
		if s.entries[i].pathID == pathID {
			releaseLabels(&s.entries[i])
			s.entries[i].labels = h
			return true
		}
	}
	return false
}

// lookupLabels returns the label handle bound to pathID, or
// attrpool.InvalidHandle when the path is absent or carries none.
func (s *pathSet) lookupLabels(pathID uint32) attrpool.Handle {
	for i := range s.entries {
		if s.entries[i].pathID == pathID {
			return s.entries[i].labels
		}
	}
	return attrpool.InvalidHandle
}

// releaseLabels releases e's label handle and leaves e with none.
func releaseLabels(e *pathEntry) {
	if e.labels.IsValid() {
		_ = pool.Labels.Release(e.labels)
	}
	e.labels = attrpool.InvalidHandle
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
		releaseLabels(&s.entries[i])
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

// releaseAll calls Release on every stored RouteEntry, releases every label
// handle, and empties the set.
func (s *pathSet) releaseAll() {
	for i := range s.entries {
		s.entries[i].entry.Release()
		releaseLabels(&s.entries[i])
	}
	s.entries = s.entries[:0]
}
