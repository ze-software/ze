// Design: plan/design-rib-unified.md -- Phase 3c (Loc-RIB change notifications)
// Design: plan/design-rib-rs-fastpath.md -- Change.Forward handle for zero-copy forwarding
// Related: manager.go -- RIB.Insert / RIB.Remove dispatch to subscribed ChangeHandlers
// Related: forward_handle.go -- ForwardHandle interface carried by Change.Forward

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

// String returns a diagnostic form for logs. Not used on the
// production hot path (warn/info); diagnostic subscribers that enable
// bgp.rib=debug explicitly opt into per-Change formatting cost.
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
	// Forward is an optional handle to the producer's wire buffer. When
	// non-nil, a subscriber that wants to forward the Change without
	// rebuilding from Best may AddRef the handle and retain it past the
	// handler. Populated on ChangeAdd / ChangeUpdate emitted by
	// InsertForward. Nil for non-BGP producers, always nil for
	// ChangeRemove, and also nil for a ChangeUpdate synthesized by
	// Remove (fallback to next-best) because PathGroup paths do not
	// retain per-path buffers today.
	Forward ForwardHandle
}

// ChangeHandler is invoked synchronously from Insert/Remove when the best
// path for a prefix changes. Handlers MUST NOT call RIB mutators on the
// same RIB during their invocation -- the RIB's write lock is held. Cheap,
// non-blocking handlers only; offload heavy work to a goroutine.
//
// When Change.Forward is non-nil, the handler runs while the producer
// still holds its reference on the backing buffer. A handler that wants
// to forward the buffer MUST call Forward.AddRef before returning;
// Release happens later (typically off-lock from the handler's own
// worker). See ForwardHandle for the full lifetime contract.
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

// appendEntry installs e as the last subscriber. Idempotent: an entry whose
// id is already present is not re-appended. Used by RIB.OnChange to replicate
// a registration across every shard's subscriber list.
func (s *subscriberList) appendEntry(e subEntry) {
	s.muSubs.Lock()
	defer s.muSubs.Unlock()
	cur := s.load()
	for _, c := range cur {
		if c.id == e.id {
			return
		}
	}
	next := make([]subEntry, len(cur), len(cur)+1)
	copy(next, cur)
	next = append(next, e)
	s.list.Store(&next)
}

// removeID drops the entry whose id matches. No-op when absent.
func (s *subscriberList) removeID(id uint64) {
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

// replace seeds the list with entries (used when a new shard inherits the
// RIB's current subscriber template). Must be called before the shard is
// reachable from any concurrent dispatch path.
func (s *subscriberList) replace(entries []subEntry) {
	next := make([]subEntry, len(entries))
	copy(next, entries)
	s.list.Store(&next)
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
