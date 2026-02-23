// Design: docs/architecture/pool-architecture.md — per-attribute pool instances

package pool

import (
	basepool "codeberg.org/thomas-mangin/ze/internal/pool"
)

// Per-attribute-type pools for fine-grained deduplication.
// Routes with identical ORIGIN, AS_PATH, LOCAL_PREF but different MED
// now share the common attributes instead of duplicating the entire blob.
//
// Pool indices 2-14 are assigned to per-attribute pools.
// See docs/architecture/core-design.md Section 4 for design rationale.

// Origin pool for ORIGIN attribute (RFC 4271 Section 4.3a).
// Only 3 possible values: IGP (0), EGP (1), INCOMPLETE (2).
var Origin = basepool.NewWithIdx(2, 64) // idx=2, 64B initial (tiny)

// ASPath pool for AS_PATH attribute (RFC 4271 Section 4.3b).
// Many unique paths but shared across routes to same destination.
var ASPath = basepool.NewWithIdx(3, 1<<18) // idx=3, 256KB initial

// LocalPref pool for LOCAL_PREF attribute (RFC 4271 Section 4.3e).
// Typically few unique values (100, 200, 300, etc.).
var LocalPref = basepool.NewWithIdx(4, 1<<12) // idx=4, 4KB initial

// MED pool for MULTI_EXIT_DISC attribute (RFC 4271 Section 4.3d).
// More variance than LOCAL_PREF but still limited unique values.
var MED = basepool.NewWithIdx(5, 1<<14) // idx=5, 16KB initial

// NextHop pool for NEXT_HOP attribute (RFC 4271 Section 4.3c).
// One per peer typically, but can vary for route servers.
var NextHop = basepool.NewWithIdx(6, 1<<14) // idx=6, 16KB initial

// Communities pool for COMMUNITIES attribute (RFC 1997).
// Moderate sharing across routes with same community set.
var Communities = basepool.NewWithIdx(7, 1<<16) // idx=7, 64KB initial

// LargeCommunities pool for LARGE_COMMUNITIES attribute (RFC 8092).
// Less common than standard communities.
var LargeCommunities = basepool.NewWithIdx(8, 1<<14) // idx=8, 16KB initial

// ExtCommunities pool for EXTENDED_COMMUNITIES attribute (RFC 4360).
// RT/RD values, moderate sharing in VPN scenarios.
var ExtCommunities = basepool.NewWithIdx(9, 1<<14) // idx=9, 16KB initial

// ClusterList pool for CLUSTER_LIST attribute (RFC 4456).
// Route reflector only, typically few unique values.
var ClusterList = basepool.NewWithIdx(10, 1<<12) // idx=10, 4KB initial

// OriginatorID pool for ORIGINATOR_ID attribute (RFC 4456).
// Route reflector only, one per original router.
var OriginatorID = basepool.NewWithIdx(11, 1<<12) // idx=11, 4KB initial

// AtomicAggregate pool for ATOMIC_AGGREGATE attribute (RFC 4271 Section 5.1.6).
// Well-known discretionary, zero-length value.
var AtomicAggregate = basepool.NewWithIdx(12, 64) // idx=12, 64B initial (tiny)

// Aggregator pool for AGGREGATOR attribute (RFC 4271 Section 5.1.7).
// Optional transitive, contains ASN + IP address.
var Aggregator = basepool.NewWithIdx(13, 1<<12) // idx=13, 4KB initial

// OtherAttrs pool for unknown/unhandled attributes.
// Stores complete attribute wire bytes for attributes not individually pooled.
// Each entry is prefixed with type code for sorting on reconstruction.
var OtherAttrs = basepool.NewWithIdx(14, 1<<16) // idx=14, 64KB initial
