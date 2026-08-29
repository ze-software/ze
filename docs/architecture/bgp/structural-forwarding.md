# Structural Forwarding: what left the critical path

The route-server forwarding gap against BIRD on grouped input was closed by
changing structure, not by tuning constants. `docs/architecture/core-design.md`
Section 9 describes the mechanisms that resulted. This page records what they
replaced and the ordering constraint that keeps them correct.

<!-- source: internal/component/bgp/reactor/forward_body.go -- shared body building -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactor-native RS forwarding -->

## Four costs, and what replaced them

| Old cost | Replacement |
|----------|-------------|
| Forward context stored in a `sync.Map` that every structured and text dispatch path loaded from, one map hop per UPDATE | a value-carrying `workItem` passed through dispatch, holding the source peer, the message and the text payload |
| Peer-down withdrawal inventory built inline in the forward path, allocating strings and writing maps before any byte left the box | NLRI records extracted as `netip.Prefix` before forwarding, applied to the withdrawal map after forwarding |
| One `Retain(id)` per destination peer, so N destinations meant N entry points | one `RetainN(id, peerCount)` per update id, fed by a pending dispatch buffer |
| Identical path attributes written as separate TCP writes | `fwdBucketMerge` at the batch-handler level merges NLRIs into fewer outbound bodies, inside the negotiated message size limit |

The critical path now touches no `sync.Map`, allocates no string for an NLRI
key, and issues one cache retain per UPDATE.

## The ordering constraint

**`extractWireNLRIRecords` MUST run before forwarding.** Cache eviction can free
the pool buffer backing `msg.WireUpdate` once `ForwardCached` has run, so
extraction after forwarding reads freed memory. Application to the withdrawal
map happens after forwarding, when the keys are already materialized. The pooled
`nlriRecord` slice is returned once the map update completes.

Correctness of the withdrawal map depends on this split: extract while the cache
buffer is alive, apply once the bytes are gone.

## How long a Path Identifier lives

Ze's own Path Identifier for a re-advertised path is held in one table, which
both rails read, so a replayed route and a live forward of one path carry the
same value (`forward_path_id.go`).

The key mirrors the key the SOURCE uses to name a path. A source that negotiated
no ADD-PATH names a path by its prefix and frames no identifier, so ze holds one
identifier for that source's whole session and gives every prefix of it the same
one. A source that negotiated ADD-PATH names a path by (prefix, identifier), so
ze holds one entry per pair.

Only the second kind is freed before the peer is removed, and the free runs at
ONE point: the recent-update cache evicting the UPDATE that withdrew the path
(`recent_cache.go` `evictLocked`). It cannot run at the end of either rail,
because one UPDATE reaches both: `reactorForwardRS` serves the destinations it
can and hands the rest to the rs plugin as `FastPathSkipped`, which forwards them
through `forwardUpdateCore`. Freeing after the first rail would mint a fresh
identifier for the second rail's destinations, and each of those would then hold
a route ze can never withdraw.

## What bucketing excludes, and why

Bucket merge handles an item with exactly one `rawBodies` entry, no parsed
`updates`, and `peerBufIdx == 0`. Those three conditions exclude every
parsed-update path, and every copy-on-modify path whose copy differs per
destination: such bytes must not be merged.

One copy is not excluded, and must not be. The RFC 7911 Path Identifier rewrite
(`fwdRegenerateRawPathIDs`) writes into a pooled copy whose handle travels on
`fwdBodyResult.transcodeBuf` rather than on `peerBufIdx`, so the item still meets
all three conditions. It is correct to merge it, because ze's identifiers are
chosen per ingress path and not per destination: the copy carries the same bytes
for every destination that reads Path Identifiers.

Merging uses pooled scratch buffers and FNV-64a for attribute grouping. A hash
collision is caught by a `bytes.Equal` check against the actual attribute bytes
before any merge happens.
