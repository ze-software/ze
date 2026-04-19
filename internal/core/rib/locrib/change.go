// Design: plan/design-rib-unified.md -- Phase 3c (Loc-RIB change notifications)
// Related: manager.go -- RIB.Insert / RIB.Remove dispatch to subscribed ChangeHandlers

package locrib

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// ChangeKind classifies the effect of a RIB mutation on the best path for a
// prefix.
type ChangeKind uint8

const (
	// ChangeUnspecified is the zero value and never emitted; used to surface
	// uninitialised Change values on the consumer side.
	ChangeUnspecified ChangeKind = 0

	// ChangeAdd fires when a prefix that had no valid best acquires one.
	// Best carries the newly-selected path.
	ChangeAdd ChangeKind = 1

	// ChangeUpdate fires when a prefix already had a valid best but the best
	// identity changed (different source/instance/next-hop/metric). Best
	// carries the new path.
	ChangeUpdate ChangeKind = 2

	// ChangeRemove fires when the last valid path for a prefix goes away.
	// Best is the zero Path; consumers should drop the prefix from their view.
	ChangeRemove ChangeKind = 3
)

// String returns a diagnostic form for logs. Never used on the hot path.
func (k ChangeKind) String() string {
	switch k {
	case ChangeAdd:
		return "add"
	case ChangeUpdate:
		return "update"
	case ChangeRemove:
		return "remove"
	case ChangeUnspecified:
		return "unspecified"
	}
	return "unspecified"
}

// Change is the payload delivered to a ChangeHandler. Value-typed; carries
// enough state for a consumer to react without re-querying the RIB.
type Change struct {
	Family family.Family
	Prefix netip.Prefix
	Kind   ChangeKind
	// Best is the selected path after the mutation. Valid for Add and
	// Update; zero Path for Remove.
	Best Path
}

// ChangeHandler is invoked synchronously from Insert/Remove when the best
// path for a prefix changes. Handlers MUST NOT call RIB mutators on the
// same RIB during their invocation -- the RIB's write lock is held. Cheap,
// non-blocking handlers only; offload heavy work to a goroutine.
type ChangeHandler func(c Change)

// subscriberList stores ChangeHandlers in a copy-on-write slice so Insert /
// Remove can dispatch under the RIB write lock without an extra mutex on the
// fire path. Subscribe and Unsubscribe take the muSubs lock and install a new
// slice; the hot path does an atomic.Load and iterates.
type subscriberList struct {
	muSubs sync.Mutex
	list   atomic.Pointer[[]subEntry]
}

// subEntry pairs a handler with its subscription id so unsubscribe can match
// by identity instead of pointer equality (closures are not comparable).
type subEntry struct {
	id uint64
	fn ChangeHandler
}

// subscribe appends fn and returns a function that removes it.
func (s *subscriberList) subscribe(fn ChangeHandler, id uint64) func() {
	s.muSubs.Lock()
	defer s.muSubs.Unlock()

	cur := s.load()
	next := make([]subEntry, len(cur), len(cur)+1)
	copy(next, cur)
	next = append(next, subEntry{id: id, fn: fn})
	s.list.Store(&next)

	return func() {
		s.muSubs.Lock()
		defer s.muSubs.Unlock()
		cur := s.load()
		next := make([]subEntry, 0, len(cur))
		for _, e := range cur {
			if e.id == id {
				continue
			}
			next = append(next, e)
		}
		s.list.Store(&next)
	}
}

// dispatch fires every handler with c. Runs under the RIB's write lock.
func (s *subscriberList) dispatch(c Change) {
	for _, e := range s.load() {
		e.fn(c)
	}
}

// load returns the current slice of subscribers. Never returns nil.
func (s *subscriberList) load() []subEntry {
	p := s.list.Load()
	if p == nil {
		return nil
	}
	return *p
}
