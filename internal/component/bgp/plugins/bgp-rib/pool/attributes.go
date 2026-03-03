// Design: docs/architecture/pool-architecture.md — per-attribute pool instances

package pool

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
)

// Per-attribute-type pools for fine-grained deduplication.
// Routes with identical ORIGIN, AS_PATH, LOCAL_PREF but different MED
// now share the common attributes instead of duplicating the entire blob.
//
// Pool indices 2-14 are assigned to per-attribute pools.
// See docs/architecture/core-design.md Section 4 for design rationale.

var (
	Origin           *attrpool.Pool // ORIGIN (RFC 4271 Section 4.3a), idx=2
	ASPath           *attrpool.Pool // AS_PATH (RFC 4271 Section 4.3b), idx=3
	LocalPref        *attrpool.Pool // LOCAL_PREF (RFC 4271 Section 4.3e), idx=4
	MED              *attrpool.Pool // MULTI_EXIT_DISC (RFC 4271 Section 4.3d), idx=5
	NextHop          *attrpool.Pool // NEXT_HOP (RFC 4271 Section 4.3c), idx=6
	Communities      *attrpool.Pool // COMMUNITIES (RFC 1997), idx=7
	LargeCommunities *attrpool.Pool // LARGE_COMMUNITIES (RFC 8092), idx=8
	ExtCommunities   *attrpool.Pool // EXTENDED_COMMUNITIES (RFC 4360), idx=9
	ClusterList      *attrpool.Pool // CLUSTER_LIST (RFC 4456), idx=10
	OriginatorID     *attrpool.Pool // ORIGINATOR_ID (RFC 4456), idx=11
	AtomicAggregate  *attrpool.Pool // ATOMIC_AGGREGATE (RFC 4271 Section 5.1.6), idx=12
	Aggregator       *attrpool.Pool // AGGREGATOR (RFC 4271 Section 5.1.7), idx=13
	OtherAttrs       *attrpool.Pool // unknown/unhandled attributes, idx=14
)

// mustPool creates a pool with the given index, panicking on error.
// Indices 2-14 are hardcoded valid constants, so errors cannot occur in practice.
func mustPool(idx uint8, initialCapacity int) *attrpool.Pool {
	p, err := attrpool.NewWithIdx(idx, initialCapacity)
	if err != nil {
		panic("BUG: attrpool.NewWithIdx failed for known-good index")
	}
	return p
}

func init() {
	Origin = mustPool(2, 64)
	ASPath = mustPool(3, 1<<18)
	LocalPref = mustPool(4, 1<<12)
	MED = mustPool(5, 1<<14)
	NextHop = mustPool(6, 1<<14)
	Communities = mustPool(7, 1<<16)
	LargeCommunities = mustPool(8, 1<<14)
	ExtCommunities = mustPool(9, 1<<14)
	ClusterList = mustPool(10, 1<<12)
	OriginatorID = mustPool(11, 1<<12)
	AtomicAggregate = mustPool(12, 64)
	Aggregator = mustPool(13, 1<<12)
	OtherAttrs = mustPool(14, 1<<16)
}

// AllPools returns all 13 attribute pools for scheduler construction.
func AllPools() []*attrpool.Pool {
	return []*attrpool.Pool{
		Origin, ASPath, LocalPref, MED, NextHop,
		Communities, LargeCommunities, ExtCommunities,
		ClusterList, OriginatorID, AtomicAggregate,
		Aggregator, OtherAttrs,
	}
}
