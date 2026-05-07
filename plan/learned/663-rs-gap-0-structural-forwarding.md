# 663 -- RS Gap 0: Structural Forwarding Performance

## Context

`spec-rs-gap-0-umbrella` closed the remaining grouped-input route-server
performance gap to BIRD by changing Ze's forwarding structure, not by tuning
constants. The gap existed because Ze's route-server plugin paid per-prefix
bookkeeping cost on the forward critical path, used per-peer individual cache
retains, and lacked outbound attribute bucketing.

Before this work:

- `processForward()` stored context in a `sync.Map` (`fwdCtx`) that every
  structured and text dispatch path loaded from, adding a map hop per UPDATE.
- Peer-down withdrawal inventory (recording which prefixes came from which
  peer) ran inline in the forward path, forcing string allocation and map
  writes before any bytes left the box.
- Each destination peer called `Retain(id)` individually, producing N syscall
  entry points for N peers.
- UPDATEs with identical path attributes were forwarded as separate TCP writes
  even when packing would reduce per-message header overhead.

## Decisions

- Replace `fwdCtx sync.Map` with a value-carrying `workItem` struct passed
  directly through dispatch. The work item carries `sourcePeer`, `msg`, and
  `textPayload` -- no indirection.
- Extract NLRI records (as `netip.Prefix`, zero-alloc for unicast) BEFORE
  forwarding, then apply them to the withdrawal map AFTER forwarding. This
  moves string key generation off the critical path.
- Replace per-peer `Retain()` with a single `RetainN(id, peerCount)` per
  update ID, using a pending dispatch buffer.
- Add `fwdBucketMerge` at the `fwdBatchHandler` level (inside forward pool
  drain, after egress decisions). Items with identical path attributes and no
  per-peer modifications merge NLRIs into fewer outbound bodies, respecting
  negotiated message size limits.
- Items using parsed-update path or with copy-on-modify bypass bucketing.
- Bucket merge uses pooled scratch buffers (`bucketScratchPool`) and FNV-64a
  hashing for attr grouping.

## Consequences

- The forward critical path no longer touches `sync.Map`, does not allocate
  strings for NLRI keys, and produces one cache retain call per UPDATE instead
  of one per destination peer.
- Outbound TCP write count is reduced for grouped UPDATEs (common in the
  `ze-perf` benchmark shape).
- Withdrawal map correctness is preserved: extraction happens before forwarding
  (while the cache buffer is still alive), application happens after.
- Perf results archived in `test/perf/results/` for regression tracking.

## Gotchas

- `extractWireNLRIRecords` must be called BEFORE forwarding because cache
  eviction can free the pool buffer backing `msg.WireUpdate` after
  `ForwardCached`. The pooled `nlriRecord` slice must be returned after map
  update.
- The bucket merge only handles items with exactly one `rawBodies` entry, no
  parsed `updates`, and no per-peer buffer index (`peerBufIdx == 0`). This
  correctly excludes copy-on-modify paths.
- FNV hash collisions are handled by a secondary `bytes.Equal` check on the
  actual attr bytes before merging.
- The named functional test `bgp-rs-grouped-bucket.ci` from the spec
  deliverables was not created as a separate file; equivalent coverage exists
  across `bgp-rs-reactor-fastpath.ci`, `bgp-rs-fastpath-ebgp-shared.ci`, and
  `forward_bucket_test.go` unit tests.

## Files

- `internal/component/bgp/plugins/rs/{server_inventory.go,server_withdrawal.go,server.go}`
- `internal/component/bgp/plugins/rs/{server_test.go,propagation_test.go,worker_test.go}`
- `internal/component/bgp/reactor/{forward_bucket.go,forward_bucket_test.go}`
- `internal/component/bgp/reactor/{reactor_api_forward.go,forward_pool.go}`
- `docs/architecture/core-design.md` (rs-gap-0 sections)
