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

## What bucketing excludes, and why

Bucket merge handles an item with exactly one `rawBodies` entry, no parsed
`updates`, and `peerBufIdx == 0`. Those three conditions exclude every
copy-on-modify path and every parsed-update path, which must not be merged
because their bytes differ per destination.

Merging uses pooled scratch buffers and FNV-64a for attribute grouping. A hash
collision is caught by a `bytes.Equal` check against the actual attribute bytes
before any merge happens.
