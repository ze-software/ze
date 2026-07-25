// Design: docs/architecture/pool-architecture.md — per-attribute pool instances

package pool

import (
	"github.com/ze-software/ze/internal/component/bgp/attrpool"
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

// mustPool creates a pool with the given index and shard count, panicking on
// error. Indices 2-14 are hardcoded valid constants and the shard counts below
// are valid powers of two, so errors cannot occur in practice.
//
// Shard-count guidance: content-hash sharding only relieves Intern lock
// contention when a pool's *hot* values are diverse enough to spread across
// shards. A pool dominated by a handful of values (ORIGIN has 3; ATOMIC_AGGREGATE
// is a single zero-length flag) has its hot value monopolize one shard, so
// sharding buys nothing and only adds fixed per-shard overhead — those use 1
// shard (the pre-sharding single-lock pool). High-cardinality, per-route
// attributes (AS_PATH, communities, MED, next-hop, unknown attrs) shard fully.
func mustPool(idx uint8, initialCapacity, shards int) *attrpool.Pool {
	p, err := attrpool.NewWithShards(idx, initialCapacity, shards)
	if err != nil {
		panic("BUG: attrpool.NewWithShards failed for known-good index/shard count")
	}
	return p
}

const (
	// shardsHot is the full shard count for high-cardinality, frequently-interned
	// attributes whose diverse values spread across shards and relieve contention.
	shardsHot = 16
	// shardsCold is the single-shard (pre-sharding) pool for low-cardinality
	// attributes whose few hot values cannot benefit from sharding.
	shardsCold = 1
)

func init() {
	Origin = mustPool(2, 64, shardsCold)        // 3 values (IGP/EGP/INCOMPLETE)
	ASPath = mustPool(3, 1<<18, shardsHot)      // high cardinality, hot path
	LocalPref = mustPool(4, 1<<12, shardsCold)  // few policy values
	MED = mustPool(5, 1<<14, shardsCold)        // few distinct metric values in practice
	NextHop = mustPool(6, 1<<14, shardsHot)     // many distinct next-hops
	Communities = mustPool(7, 1<<16, shardsHot) // high cardinality
	LargeCommunities = mustPool(8, 1<<14, shardsHot)
	ExtCommunities = mustPool(9, 1<<14, shardsHot)
	ClusterList = mustPool(10, 1<<12, shardsCold)  // small (RR cluster ids)
	OriginatorID = mustPool(11, 1<<12, shardsCold) // small (# of RRs)
	AtomicAggregate = mustPool(12, 64, shardsCold) // single flag value
	Aggregator = mustPool(13, 1<<12, shardsCold)   // few aggregation points
	OtherAttrs = mustPool(14, 1<<16, shardsHot)    // unknown attrs, diverse
}

// AllPools returns all attribute pools for scheduler construction.
func AllPools() []*attrpool.Pool {
	return []*attrpool.Pool{
		Origin, ASPath, LocalPref, MED, NextHop,
		Communities, LargeCommunities, ExtCommunities,
		ClusterList, OriginatorID, AtomicAggregate,
		Aggregator, OtherAttrs, RibOut,
	}
}
