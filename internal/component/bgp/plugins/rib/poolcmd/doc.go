// Design: docs/architecture/pool-architecture.md -- pool-stats, the operator view of the attribute pools
//
// Package poolcmd answers `pool-stats` (ze-bgp:pool-stats): per-attribute-pool
// occupancy and deduplication rates for the BGP RIB attribute pools. It is an
// in-process reader of the pools the rib plugin owns, so it lives inside the
// rib plugin, and removing the plugin removes the command with it. The ze-bgp:
// WireMethod prefix is a legacy label, not an ownership claim.
package poolcmd
