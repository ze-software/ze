package storage

import (
	"codeberg.org/thomas-mangin/ze/internal/pool"
)

// RouteEntry stores per-attribute handles for a single route.
// Each attribute type has its own pool, enabling fine-grained deduplication.
// Routes with identical ORIGIN but different MED share the ORIGIN pool entry.
//
// Use InvalidHandle for attributes not present in the route.
// Use Release() to decrement refcounts when removing from RIB.
//
// Limitation: Attribute flags (especially Partial bit 0x20) are not preserved.
// For exact wire reproduction, use msg-id cache forwarding instead.
type RouteEntry struct {
	// Well-known mandatory (RFC 4271 Section 5.1)
	Origin  pool.Handle // ORIGIN (type 1) - IGP, EGP, INCOMPLETE
	ASPath  pool.Handle // AS_PATH (type 2)
	NextHop pool.Handle // NEXT_HOP (type 3)

	// Well-known discretionary / optional
	LocalPref       pool.Handle // LOCAL_PREF (type 5) - IBGP only
	MED             pool.Handle // MULTI_EXIT_DISC (type 4)
	AtomicAggregate pool.Handle // ATOMIC_AGGREGATE (type 6)
	Aggregator      pool.Handle // AGGREGATOR (type 7)

	// Communities
	Communities      pool.Handle // COMMUNITIES (type 8, RFC 1997)
	LargeCommunities pool.Handle // LARGE_COMMUNITIES (type 32, RFC 8092)
	ExtCommunities   pool.Handle // EXTENDED_COMMUNITIES (type 16, RFC 4360)

	// Route reflection (RFC 4456)
	ClusterList  pool.Handle // CLUSTER_LIST (type 10)
	OriginatorID pool.Handle // ORIGINATOR_ID (type 9)

	// Other attributes stored as blob (type-code prefixed for sorting)
	OtherAttrs pool.Handle // Unknown/unhandled attributes
}

// NewRouteEntry creates a RouteEntry with all handles set to InvalidHandle.
func NewRouteEntry() *RouteEntry {
	return &RouteEntry{
		Origin:           pool.InvalidHandle,
		ASPath:           pool.InvalidHandle,
		NextHop:          pool.InvalidHandle,
		LocalPref:        pool.InvalidHandle,
		MED:              pool.InvalidHandle,
		AtomicAggregate:  pool.InvalidHandle,
		Aggregator:       pool.InvalidHandle,
		Communities:      pool.InvalidHandle,
		LargeCommunities: pool.InvalidHandle,
		ExtCommunities:   pool.InvalidHandle,
		ClusterList:      pool.InvalidHandle,
		OriginatorID:     pool.InvalidHandle,
		OtherAttrs:       pool.InvalidHandle,
	}
}

// HasOrigin returns true if ORIGIN attribute is present.
func (e *RouteEntry) HasOrigin() bool { return e.Origin.IsValid() }

// HasASPath returns true if AS_PATH attribute is present.
func (e *RouteEntry) HasASPath() bool { return e.ASPath.IsValid() }

// HasNextHop returns true if NEXT_HOP attribute is present.
func (e *RouteEntry) HasNextHop() bool { return e.NextHop.IsValid() }

// HasLocalPref returns true if LOCAL_PREF attribute is present.
func (e *RouteEntry) HasLocalPref() bool { return e.LocalPref.IsValid() }

// HasMED returns true if MED attribute is present.
func (e *RouteEntry) HasMED() bool { return e.MED.IsValid() }

// HasAtomicAggregate returns true if ATOMIC_AGGREGATE attribute is present.
func (e *RouteEntry) HasAtomicAggregate() bool { return e.AtomicAggregate.IsValid() }

// HasAggregator returns true if AGGREGATOR attribute is present.
func (e *RouteEntry) HasAggregator() bool { return e.Aggregator.IsValid() }

// HasCommunities returns true if COMMUNITIES attribute is present.
func (e *RouteEntry) HasCommunities() bool { return e.Communities.IsValid() }

// HasLargeCommunities returns true if LARGE_COMMUNITIES attribute is present.
func (e *RouteEntry) HasLargeCommunities() bool { return e.LargeCommunities.IsValid() }

// HasExtCommunities returns true if EXTENDED_COMMUNITIES attribute is present.
func (e *RouteEntry) HasExtCommunities() bool { return e.ExtCommunities.IsValid() }

// HasClusterList returns true if CLUSTER_LIST attribute is present.
func (e *RouteEntry) HasClusterList() bool { return e.ClusterList.IsValid() }

// HasOriginatorID returns true if ORIGINATOR_ID attribute is present.
func (e *RouteEntry) HasOriginatorID() bool { return e.OriginatorID.IsValid() }

// HasOtherAttrs returns true if unknown attributes are present.
func (e *RouteEntry) HasOtherAttrs() bool { return e.OtherAttrs.IsValid() }

// Release decrements refcount for all valid handles and resets to InvalidHandle.
// Safe to call multiple times.
func (e *RouteEntry) Release() {
	if e.Origin.IsValid() {
		_ = pool.Origin.Release(e.Origin)
		e.Origin = pool.InvalidHandle
	}
	if e.ASPath.IsValid() {
		_ = pool.ASPath.Release(e.ASPath)
		e.ASPath = pool.InvalidHandle
	}
	if e.NextHop.IsValid() {
		_ = pool.NextHop.Release(e.NextHop)
		e.NextHop = pool.InvalidHandle
	}
	if e.LocalPref.IsValid() {
		_ = pool.LocalPref.Release(e.LocalPref)
		e.LocalPref = pool.InvalidHandle
	}
	if e.MED.IsValid() {
		_ = pool.MED.Release(e.MED)
		e.MED = pool.InvalidHandle
	}
	if e.AtomicAggregate.IsValid() {
		_ = pool.AtomicAggregate.Release(e.AtomicAggregate)
		e.AtomicAggregate = pool.InvalidHandle
	}
	if e.Aggregator.IsValid() {
		_ = pool.Aggregator.Release(e.Aggregator)
		e.Aggregator = pool.InvalidHandle
	}
	if e.Communities.IsValid() {
		_ = pool.Communities.Release(e.Communities)
		e.Communities = pool.InvalidHandle
	}
	if e.LargeCommunities.IsValid() {
		_ = pool.LargeCommunities.Release(e.LargeCommunities)
		e.LargeCommunities = pool.InvalidHandle
	}
	if e.ExtCommunities.IsValid() {
		_ = pool.ExtCommunities.Release(e.ExtCommunities)
		e.ExtCommunities = pool.InvalidHandle
	}
	if e.ClusterList.IsValid() {
		_ = pool.ClusterList.Release(e.ClusterList)
		e.ClusterList = pool.InvalidHandle
	}
	if e.OriginatorID.IsValid() {
		_ = pool.OriginatorID.Release(e.OriginatorID)
		e.OriginatorID = pool.InvalidHandle
	}
	if e.OtherAttrs.IsValid() {
		_ = pool.OtherAttrs.Release(e.OtherAttrs)
		e.OtherAttrs = pool.InvalidHandle
	}
}

// AddRef increments refcount for all valid handles.
// Use when sharing a RouteEntry between multiple owners.
// On error, rolls back any increments already made.
func (e *RouteEntry) AddRef() error {
	// Track which handles we've incremented for rollback.
	var incremented []struct {
		pool   *pool.Pool
		handle pool.Handle
	}

	addRef := func(p *pool.Pool, h pool.Handle) error {
		if !h.IsValid() {
			return nil
		}
		if err := p.AddRef(h); err != nil {
			return err
		}
		incremented = append(incremented, struct {
			pool   *pool.Pool
			handle pool.Handle
		}{p, h})
		return nil
	}

	// Try to increment all handles.
	if err := addRef(pool.Origin, e.Origin); err != nil {
		goto rollback
	}
	if err := addRef(pool.ASPath, e.ASPath); err != nil {
		goto rollback
	}
	if err := addRef(pool.NextHop, e.NextHop); err != nil {
		goto rollback
	}
	if err := addRef(pool.LocalPref, e.LocalPref); err != nil {
		goto rollback
	}
	if err := addRef(pool.MED, e.MED); err != nil {
		goto rollback
	}
	if err := addRef(pool.AtomicAggregate, e.AtomicAggregate); err != nil {
		goto rollback
	}
	if err := addRef(pool.Aggregator, e.Aggregator); err != nil {
		goto rollback
	}
	if err := addRef(pool.Communities, e.Communities); err != nil {
		goto rollback
	}
	if err := addRef(pool.LargeCommunities, e.LargeCommunities); err != nil {
		goto rollback
	}
	if err := addRef(pool.ExtCommunities, e.ExtCommunities); err != nil {
		goto rollback
	}
	if err := addRef(pool.ClusterList, e.ClusterList); err != nil {
		goto rollback
	}
	if err := addRef(pool.OriginatorID, e.OriginatorID); err != nil {
		goto rollback
	}
	if err := addRef(pool.OtherAttrs, e.OtherAttrs); err != nil {
		goto rollback
	}

	return nil

rollback:
	// Release all handles we've already incremented.
	for _, inc := range incremented {
		_ = inc.pool.Release(inc.handle)
	}
	return pool.ErrPoolShutdown // Most likely cause of AddRef failure.
}

// Clone creates a copy of the RouteEntry with AddRef called on all handles.
// Caller must call Release() on the clone when done.
// Returns nil if AddRef fails (e.g., pool shutdown).
func (e *RouteEntry) Clone() *RouteEntry {
	clone := &RouteEntry{
		Origin:           e.Origin,
		ASPath:           e.ASPath,
		NextHop:          e.NextHop,
		LocalPref:        e.LocalPref,
		MED:              e.MED,
		AtomicAggregate:  e.AtomicAggregate,
		Aggregator:       e.Aggregator,
		Communities:      e.Communities,
		LargeCommunities: e.LargeCommunities,
		ExtCommunities:   e.ExtCommunities,
		ClusterList:      e.ClusterList,
		OriginatorID:     e.OriginatorID,
		OtherAttrs:       e.OtherAttrs,
	}
	if err := clone.AddRef(); err != nil {
		return nil
	}
	return clone
}
