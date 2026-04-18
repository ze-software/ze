# Spec: rs-fastpath-3-passthrough -- zero-copy pass-through for bgp-rs forwarding

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-rs-fastpath-2-adjrib |
| Phase | - |
| Updated | 2026-04-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec (especially Design Insights -> Decision Log)
2. Umbrella: `plan/spec-rs-fastpath-0-umbrella.md`
3. Completed siblings: `plan/learned/625-rs-fastpath-1-profile.md`, `plan/learned/626-rs-fastpath-2-adjrib.md`
4. `.claude/rules/no-layering.md`, `.claude/rules/enum-over-string.md`
5. `internal/component/bgp/reactor/reactor_api_forward.go` -- `ForwardUpdate` (per-destination loop, `ebgpWireCache`, `fwdBodyCache`, `fwdSupersedeKey`, `fwdIsWithdrawal`)
6. `internal/component/bgp/reactor/reactor_notify.go` -- `notifyMessageReceiver` cache Retain/Activate + DirectBridge event
7. `internal/component/bgp/reactor/forward_pool.go` -- `fwdPool`, `fwdItem.done`, `TryDispatch` / `DispatchOverflow`
8. `internal/component/bgp/reactor/forward_build.go` -- `buildModifiedPayload` (Outgoing Peer Pool copy-on-modify)
9. `internal/component/bgp/reactor/recent_cache.go` -- `CacheConsumer` + `CacheConsumerUnordered` pending-never-expires contract
10. `internal/component/bgp/plugins/rs/server.go` -- `dispatchStructured`, rs plumbing to be deleted (`forwardCh`/`forwardLoop`/`asyncForward`/`releaseCh`/`releaseLoop`)
11. `internal/component/bgp/plugins/rs/server_forward.go` -- `selectForwardTargets`, `batchForwardUpdate`, `flushBatch`
12. `internal/component/bgp/plugins/rs/server_handlers.go` -- `handleStateUp` / `replayForPeer` / Replaying gate

## Task

Third child of the `rs-fastpath` umbrella. Goal: remove the per-UPDATE text-RPC round-trip from bgp-rs's forwarding hot path. Phase 1 profile (`plan/learned/625-rs-fastpath-1-profile.md`) measured `plugin/server.tokenize` at 19.4 % of 2.5 GB allocations and `CommandRegistry.All` adding another ~5 %; both are driven by rs sending batched text commands (`cache N,N,N forward A,B,C`) that the engine re-tokenises per call.

This child replaces that text RPC with a reactor-owned forwarding primitive exposed through the SDK: `p.ForwardCached(ctx, updateIDs, destinations)` routes to `reactorAPIAdapter.ForwardUpdateDirect(updateIDs, destinations)` without text parsing. Reactor's existing per-destination pipeline (egress filters, EBGP wire cache, copy-on-modify via `buildModifiedPayload`) is reused unchanged -- one path, not a parallel dispatcher (`rules/no-layering`). The existing `fwdBodyCache` is extended to also memoise `supersedeKey` and `withdrawal` so per-destination work shared across peers (FNV hash, attribute scan) is computed once per unique rawBody per UPDATE. Symmetric `p.ReleaseCached(ctx, updateIDs)` replaces the `cache N release` text RPC for the "rs decided not to forward" path.

Preserves: `rules/design-principles.md` "zero-copy, copy-on-modify" contract; per-source ordering via rs worker pool; Replaying gate (AC-5b); withdrawal tracking in rs's `processForward`; per-destination egress filters; flow control via `workers.BackpressureDetected` + pause-source; replay-on-new-peer (child 2). All existing `.ci` tests unchanged. The fast-path primitive is reactor-owned and usable by any plugin; not bgp-rs specific in naming or coupling.

Depends on child 2 (`plan/learned/626-rs-fastpath-2-adjrib.md`) -- async adj-rib-in is the soft-dep mechanism that lets the fast path and side-subscriber storage not contend on the delivery goroutine.

## Required Reading

### Architecture Docs

- [ ] `.claude/rules/design-principles.md`
  -> Constraint: "Zero-copy, copy-on-modify." Source buffer shared read-only across destinations; copy only when egress filters modify; release when every destination has sent or copied. Fast-path must honour this end-to-end.
  -> Constraint: "No layering." ForwardUpdateDirect is a NEW adapter entry reusing the same per-destination loop as ForwardUpdate; we do NOT build a parallel lean dispatcher. The fwdBodyCache extension is loop-invariant code motion inside the shared loop, not a second path.
- [ ] `.claude/rules/buffer-first.md`
  -> Constraint: No new `make([]byte)` on the forward hot path. The new SDK method and reactor adapter use existing pools; no per-call allocations above the pre-existing contract.
- [ ] `.claude/rules/no-layering.md`
  -> Constraint: rs's `forwardCh` / `forwardLoop` / `asyncForward` / `releaseCh` / `releaseLoop` are DELETED when ForwardCached + ReleaseCached land -- keeping both is layering.
- [ ] `.claude/rules/enum-over-string.md`
  -> Constraint: Destinations travel as value types. `[]string` at the SDK surface matches existing `UpdateRoute(peerSelector string)` convention and parses to `netip.AddrPort` once at the reactor adapter boundary. msgIDs are uint64.
- [ ] `plan/learned/275-spec-forward-pool.md`
  -> Constraint: `fwdPool` is the single dispatcher. TryDispatch / DispatchOverflow / done() Release contract is unchanged by this spec.
- [ ] `plan/learned/277-rr-ebgp-forward.md`, `269-rr-serial-forward.md`, `289-rr-per-family-forward.md`
  -> Decision: Per-source ordering stays in rs's worker pool (`workers.Dispatch(workerKey{sourcePeer})`). Not replicated in reactor; reactor receives pre-ordered batches from a single per-source worker.
- [ ] `plan/learned/434-apply-mods.md`
  -> Constraint: `buildModifiedPayload` is the copy-on-modify point (Outgoing Peer Pool). Fast-path does not bypass it; it fires per-destination exactly when today.
- [ ] `plan/learned/625-rs-fastpath-1-profile.md`
  -> Decision: Text-RPC boundary is the measured cost (tokenise 19.4 %, CommandRegistry.All ~5 %). ForwardUpdate internals are NOT in the top-20; therefore this child does not profile them ahead of time. Phase 4 bench tells us if a follow-up child is needed.
- [ ] `plan/learned/626-rs-fastpath-2-adjrib.md`
  -> Constraint: adj-rib-in is `OptionalDependencies` now. Fast path must continue to work whether adj-rib-in is loaded or not. Replay path (rs `replayForPeer`) uses a different RPC (`adj-rib-in replay ...`) and is OUT OF SCOPE for this child.

### RFC Summaries

- [ ] `rfc/short/rfc4271.md` — UPDATE message format, attribute rules.
  -> Constraint: AS-PATH prepend on eBGP egress is per RFC 4271 §9.1.2. Already cached in `ebgpWireCache` per `(localAS, secondaryAS, asn4)` variant; one compute per UPDATE, reused across destinations in that variant. AC-6b verifies this.

**Key insights (RESEARCH 2026-04-18):**

- The cache-consumer contract (`CacheConsumer: true` + `CacheConsumerUnordered: true` on the rs plugin registration) already guarantees that a msgID accepted via `Activate(id, consumerCount)` cannot expire from `recentUpdates` until the consumer acks. This is a load-bearing invariant for rs's batch accumulator: msgIDs can sit in a 500-deep batch before flush without risk of cache eviction. Phase 1 pins this invariant with `TestPendingCacheNeverExpires` so future cache refactors cannot silently regress it.
- The source buffer refcount already exists: `recentUpdates.Retain(updateID)` per destination (line 672 of `reactor_api_forward.go`) paired with `Release` via `fwdItem.done` in the fwdPool worker. AC-2 "refcount equals destinations" is an existing invariant; this child verifies it holds through the new call path, it does not introduce the mechanism.
- Copy-on-modify via Outgoing Peer Pool (`buildModifiedPayload`, learned/434) is the only copy allowed and already correct. AC-3 verifies it through the new path.
- The zero-copy rawBodies append (line 593, `item.rawBodies = append(item.rawBodies, peerWire.Payload())` when `srcCtxID == destCtxID`) already yields "no new buffer per destination" in the iBGP→iBGP-same-ctx case. AC-6a verifies it unchanged.
- EBGP wire prepend is amortised once per `(localAS, secondaryAS, asn4)` variant via `ebgpWireCache`. All destinations in the same variant share the same bytes. AC-6b verifies "A-receives-identical-bytes-to-B" plus deterministic equality to source-with-prepend.
- The fast-path primitive is reactor-owned. `p.ForwardCached` is not rs-specific; any plugin that holds cached msgIDs and a resolved destination list may use it. bgp-redistribute does NOT use it (it synthesises fresh announces; separate shape).

## Current Behavior

**Source files read (2026-04-18):**
- [x] `internal/component/bgp/reactor/reactor_notify.go` -- `notifyMessageReceiver` builds `WireUpdate`, caches via `recentUpdates.Add` (owns the pool buf), emits DirectBridge event. rs receives `*RawMessage` with zero-copy `WireUpdate`.
  -> Constraint: DirectBridge pointer delivery is WITHIN the BGP component (reactor -> bgp-rs plugin, both under `internal/component/bgp/`). Not a component/engine seam. `rules/enum-over-string.md` cross-boundary rule does NOT apply here; retain/release discipline does.
- [x] `internal/component/bgp/reactor/reactor_api_forward.go` -- `ForwardUpdate(selector, updateID, pluginName)` at line 156. Per-destination loop (lines 369-686): state check, RR source filter, egress filter chain, policy export filter, RR attribute injection, `applyNextHopMod`/`applySendCommunityFilter`/`applyASOverride`, `getEBGPWire` (cached in local `ebgpWireCache`), mods via `buildModifiedPayload`, group-aware body cache, rawBodies build, `fwdSupersedeKey` (line 662, FNV-1a over rawBodies), `fwdIsWithdrawal` (line 665), `recentUpdates.Retain` + `item.done = Release`, `fwdPool.TryDispatch` / `DispatchOverflow`.
  -> Decision: `ForwardUpdateDirect(updateIDs, destinations)` is a NEW adapter entry that iterates updateIDs, looks up each cached entry, then runs the SAME per-destination loop with `destinations` replacing the selector-match iteration. No parallel dispatcher.
  -> Decision: `fwdBodyCache` (lines 336-349, 558-564) is extended to also memoise `supersedeKey` and `withdrawal` so destinations sharing the same rawBodies skip redundant FNV hashes and attribute scans. Benefits ALL callers of the per-destination loop, not just the fast path.
- [x] `internal/component/bgp/reactor/forward_pool.go` -- `fwdPool`, per-destination worker goroutine + bounded channel, `fwdItem` with `done func()`, `TryDispatch` / `DispatchOverflow` / `safeBatchHandle` / `releaseItem`.
  -> Constraint: Unchanged by this spec. Fast path dispatches to the same pool via the same `TryDispatch` contract.
- [x] `internal/component/bgp/reactor/forward_build.go` -- `buildModifiedPayload` (progressive build into Outgoing Peer Pool buffer); `acquireModBuf` fallback to `modBufPool` sync.Pool; `buildWithdrawalPayload` (RFC 9494 LLGR announce->withdrawal).
  -> Constraint: Copy-on-modify point. Unchanged. AC-3 verifies it through the new call path.
- [x] `internal/component/bgp/plugins/rs/server.go` -- `dispatchStructured(peerAddr, msg *bgptypes.RawMessage)` at line 616 stores `forwardCtx` in `fwdCtx`, dispatches `workItem{msgID}` to per-source worker via `workers.Dispatch(workerKey{sourcePeer: peerAddr}, ...)`. Flow control via `workers.BackpressureDetected` (line 638). `startForwardLoop` / `startReleaseLoop` spawn fire-and-forget sender goroutines draining `forwardCh` / `releaseCh`.
  -> Decision: Per-source worker keeps its role. `flushBatch` calls `p.ForwardCached(ctx, batch.ids, batch.targets)` instead of `asyncForward("*", "cache <IDs> forward <selectors>")`. `releaseCache(msgID)` calls `p.ReleaseCached(ctx, []uint64{msgID})`.
  -> Decision: `forwardCh` + `forwardStop` + `forwardDone` + `startForwardLoop` + `stopForwardLoop` + `asyncForward` + `rsForwardChDepth` + `fwdSendersDefault` + `env.MustRegister("ze.rs.fwd.senders", ...)` + `releaseCh` + `releaseStop` + `releaseDone` + `startReleaseLoop` + `stopReleaseLoop` are DELETED. `rules/no-layering` -- the buffering existed to absorb text-RPC cost that no longer exists.
- [x] `internal/component/bgp/plugins/rs/server_forward.go` -- `selectForwardTargets(buf, sourcePeer, families)` filters peers by up-state, source exclusion, family support. `batchForwardUpdate(key, src, msgID, families)` selects targets and accumulates into per-worker `forwardBatch`. Flush at selector-change OR `maxBatchSize=500` OR `onDrained` (partial-batch flush).
  -> Decision: Unchanged in structure; `flushBatch` substitutes the RPC call only. Batching evidence (learned/625): maxBatchSize 50 -> 500 = +9 % throughput.
- [x] `internal/component/bgp/plugins/rs/server_handlers.go` -- `handleStateUp` sets `Replaying=true` + spawns `replayForPeer`; `Replaying=false` after full+delta replay + sendEOR (unified across success, missing-dep, IPC timeout, engine error paths per child 2).
  -> Constraint: AC-5b. `selectForwardTargets` must continue to exclude `Replaying=true` peers so live fast-path forwarding does not duplicate replay output. rs's filter runs BEFORE calling ForwardCached; reactor receives only Replaying=false destinations.
- [x] `internal/component/bgp/reactor/recent_cache.go` (referenced by learned/275) -- `recentUpdates` cache with `Add/Get/Retain/Release/Ack/Activate`. `CacheConsumer: true` + `CacheConsumerUnordered: true` on rs's registration means pending entries are not evicted until acked.
  -> Constraint: Pending-never-expires invariant. Phase 1 pins it with `TestPendingCacheNeverExpires`.
- [x] `internal/component/plugin/server/dispatch.go` + SDK MuxConn (via learned/625 context) -- text-RPC `UpdateRoute` goes through `tokenize` (19.4 % alloc) and `CommandRegistry.All` (~5 %). Fast path uses a new opcode that bypasses tokenisation.
  -> Decision: New RPC opcodes `ForwardCached` and `ReleaseCached` on the same MuxConn. Binary-encoded (uint64 IDs + []string destinations). Engine dispatch is direct to `reactorAPIAdapter.ForwardUpdateDirect` / `ReleaseUpdates`, no tokenise.

**Behavior to preserve:**
- Per-source ordering across all destinations (via rs worker pool serialisation per `workerKey`).
- Egress filter pipeline per destination (redistribution, community, prefix, AS-PATH, NEXT_HOP, role/OTC, strip-private) -- runs inside reactor's unchanged per-destination loop.
- Copy-on-modify via Outgoing Peer Pool (`buildModifiedPayload`) when egress filter / next-hop / AS-override fires.
- Flow control via `workers.BackpressureDetected` + pause-source. With `forwardCh` deleted, backpressure now flows directly from `p.ForwardCached` return latency to the rs per-source worker; `workers.BackpressureDetected` triggers peer pause exactly as today.
- Replay-on-new-peer (child 2 soft-dep path unchanged).
- Replaying=true destination gate (AC-5b): rs's `selectForwardTargets` filters BEFORE calling ForwardCached; reactor receives a pre-filtered list.
- Withdrawal tracking: rs's `processForward` continues to populate `rs.withdrawals` from NLRI parse so `handleStateDown` can emit withdrawals on source-peer-down.
- Cache pending-never-expires invariant (pinned by `TestPendingCacheNeverExpires` in Phase 1).

**Behavior to change:**
- rs worker: `flushBatch` calls `p.ForwardCached(ctx, ids, destinations)` instead of `asyncForward(...)`. `releaseCache` calls `p.ReleaseCached(ctx, []uint64{id})`.
- SDK: new methods `ForwardCached(ctx, updateIDs []uint64, destinations []string) error` and `ReleaseCached(ctx, updateIDs []uint64) error`. Reactor-owned primitive; not rs-specific.
- Reactor adapter: new methods `ForwardUpdateDirect(updateIDs []uint64, destinations []netip.AddrPort, pluginName string)` and `ReleaseUpdates(updateIDs []uint64, pluginName string)`. ForwardUpdateDirect reuses ForwardUpdate's per-destination loop via a shared helper; no parallel path.
- `fwdBodyCacheEntry` grows two fields (`supersedeKey`, `withdrawal`) so destinations sharing the same rawBodies compute them once per unique rawBody per UPDATE.
- rs plumbing deleted (see Source files read entry for `rs/server.go`).
- Engine RPC dispatch: new opcodes for ForwardCached / ReleaseCached bypassing `tokenize` + `CommandRegistry.All`.

## Data Flow

### Entry Point

- Inbound UPDATE bytes on TCP arrive at a peer session.

### Transformation Path

1. Session `Run()` reads wire bytes into its Incoming Peer Pool buffer.
2. Reactor framing produces a `WireUpdate` reference (zero-copy).
3. `notifyMessageReceiver` caches the UPDATE via `recentUpdates.Add` (cache owns the pool buffer) and emits a DirectBridge event.
4. rs plugin's `OnStructuredEvent` receives `*RawMessage`; `dispatchStructured` stores `forwardCtx` and dispatches `workItem{msgID}` to the per-source worker (`workers.Dispatch(workerKey{sourcePeer}, ...)`). Per-source serialisation preserved.
5. Per-source worker drains `workItem`, calls `processForward` (parses families, updates `rs.withdrawals`), then `batchForwardUpdate` which selects destinations via `selectForwardTargets` (filters Replaying=true peers, source exclusion, family support) and accumulates into per-worker `forwardBatch`.
6. Flush triggers: selector change OR batch reaches `maxBatchSize=500` OR worker channel drains (`onDrained`).
7. Flush calls `p.ForwardCached(ctx, batch.ids, batch.targets)`. No rs-side buffering goroutines; call blocks the per-source worker directly (backpressure flows through `workers.BackpressureDetected`).
8. SDK encodes a binary RPC (new opcode) over the existing MuxConn. No tokenise, no `CommandRegistry.All` lookup.
9. Engine RPC handler dispatches to `reactorAPIAdapter.ForwardUpdateDirect(updateIDs, destinations, pluginName)`.
10. `ForwardUpdateDirect` iterates updateIDs. For each: look up cached `ReceivedUpdate`; `defer Ack(id, pluginName)`; loop over destinations running the shared per-destination helper (egress filter chain, EBGP wire cache, `buildModifiedPayload` on mods, fwdItem build). The `fwdBodyCache` now memoises `supersedeKey` + `withdrawal` so destinations sharing the same rawBodies avoid redundant FNV hash + attribute scan.
11. Per destination: `recentUpdates.Retain(id)` + `fwdItem.done = Release(id)` + `fwdPool.TryDispatch(item)` (fall through to `DispatchOverflow` if channel full).
12. Per-destination forward_pool worker writes the rawBodies (or modified copy from Outgoing Peer Pool) to TCP, then `item.done()` releases the cache retain. When the last retain drops, the cache entry is evictable.
13. rs's `releaseCache(msgID)` (called when `selectForwardTargets` returns empty) calls `p.ReleaseCached(ctx, []uint64{id})` which on the engine side acks the entry via `Release(id) / Ack(id, pluginName)`.

Side path (subscriber from child 2): adj-rib-in, when loaded, stores asynchronously off the hot path. Unchanged by this spec.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Session <-> Reactor | `WireUpdate` reference into cache buffer (within BGP component) | [ ] |
| Reactor -> rs plugin | DirectBridge `*RawMessage` (within BGP component) | [ ] |
| rs plugin -> Engine RPC | SDK `ForwardCached` / `ReleaseCached` opcodes over MuxConn (no tokenise) | [ ] |
| Engine -> Reactor adapter | `ForwardUpdateDirect` / `ReleaseUpdates` direct calls | [ ] |
| Reactor -> forward_pool | `fwdItem` via `TryDispatch` / `DispatchOverflow` (unchanged) | [ ] |
| forward_pool -> TCP | direct write of pool buffer (unchanged) | [ ] |

### Integration Points

- `pkg/plugin/sdk/` (SDK) -- new `Plugin.ForwardCached(ctx, updateIDs, destinations) error` + `Plugin.ReleaseCached(ctx, updateIDs) error` methods.
- `internal/component/plugin/server/dispatch.go` -- new RPC opcodes `ForwardCached` / `ReleaseCached`, binary-encoded; dispatch direct to reactor adapter.
- `internal/component/bgp/reactor/reactor_api_forward.go` -- new `ForwardUpdateDirect(updateIDs, destinations, pluginName)`; existing `ForwardUpdate(selector, updateID, pluginName)` refactored to share the per-destination helper (single path; `rules/no-layering`).
- `internal/component/bgp/reactor/reactor_api_forward.go` (cache extension) -- `fwdBodyCacheEntry` grows `supersedeKey` + `withdrawal` fields; hoisted on cache miss, reused on hit.
- `internal/component/bgp/reactor/recent_cache.go` -- pin pending-never-expires invariant with test.
- `internal/component/bgp/plugins/rs/server_forward.go` -- `flushBatch` calls `p.ForwardCached` directly.
- `internal/component/bgp/plugins/rs/server.go` -- DELETE `forwardCh`/`forwardLoop`/`asyncForward`/`releaseCh`/`releaseLoop` plumbing + associated env registrations.

### Architectural Verification

- [ ] No bypassed layers -- rs still owns policy; reactor still owns wire handling.
- [ ] No parallel dispatcher -- `ForwardUpdate` and `ForwardUpdateDirect` share the per-destination helper and the extended `fwdBodyCache`.
- [ ] `rules/no-layering` -- rs plumbing deleted in same commit as ForwardCached introduction.
- [ ] Zero-copy preserved in the iBGP-same-ctx case (AC-6a); shared-prepend determinism preserved in the eBGP case (AC-6b); one copy per modifying destination on mods (AC-3).
- [ ] Cross-boundary pointer rule (`rules/enum-over-string.md` + `rules/memory.md`) -- SDK surface carries value types only (uint64 + []string + value context). DirectBridge `*WireUpdate` is within the BGP component and out of scope for the cross-component rule.

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Two iBGP receivers with matching ASN4/ADD-PATH; sender streams UPDATEs | -> | `ForwardUpdateDirect` zero-copy iBGP path | `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` |
| Two eBGP receivers with matching localAS; sender streams UPDATEs | -> | `ForwardUpdateDirect` shared EBGP-wire prepend | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |
| Two peers + bgp-rs with AS-PATH override on one destination | -> | `ForwardUpdateDirect` + copy-on-modify via Outgoing Peer Pool | `test/plugin/bgp-rs-mod-copy.ci` |
| Two peers + bgp-rs, destination B mid-replay (Replaying=true) when live UPDATE arrives | -> | rs-side `selectForwardTargets` Replaying gate | `test/plugin/bgp-rs-replaying-gate.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 100k IPv4 route bench, ze vs bird | ze throughput >= umbrella AC-1 target (400k rps / 200k floor); first-route <= umbrella target; convergence <= umbrella target. |
| AC-2 | Two receivers, no per-destination mods; unit test with instrumented cache | Source cache refcount equals number of destinations during flight (one Retain per destination, one Release per `fwdItem.done`); no new buffer allocated per destination. Already-existing invariant; this AC verifies it holds through `ForwardUpdateDirect`. |
| AC-3 | Copy-on-modify: sender -> ze -> two receivers, AS-PATH rewrite on receiver A | Receiver A gets rewritten UPDATE via Outgoing Peer Pool buffer; receiver B gets unchanged UPDATE; exactly one Outgoing Peer Pool buffer allocated for A. Verified through `ForwardUpdateDirect` path. |
| AC-4 | Per-source ordering | With N=10000 UPDATEs from one source, receiver sees them in sender order. Preserved by rs's per-source worker (`workers.Dispatch(workerKey{sourcePeer})`). |
| AC-5 | Backpressure: one destination TCP stalls | `workers.BackpressureDetected` fires; source peer paused; other destinations continue. With `forwardCh` deleted, backpressure flows directly from `p.ForwardCached` return latency into rs worker channel occupancy. |
| AC-5b | Destination B is `Replaying=true` when a live UPDATE arrives at rs | rs's `selectForwardTargets` excludes B from the destination list passed to `ForwardCached`. B receives the prefix exactly once (via replay), never twice. |
| AC-6a | Pure iBGP pass-through: sender + two iBGP receivers with matching ASN4 / ADD-PATH | Hex of outbound payload to each receiver == hex of sender's payload (modulo framing, modulo RFC 7606 sanitisation). Strict byte identity. |
| AC-6b | eBGP shared determinism: sender + two eBGP receivers with same localAS | Hex of outbound to A == hex of outbound to B (shared EBGP-wire prepend). Both equal source payload with deterministic AS-PATH prepend (`localAS` single-prepend per RFC 4271 §9.1.2). |
| AC-7 | All existing `.ci` tests | Pass unchanged. |
| AC-8 | `make ze-test`, `make ze-verify-fast`, `make ze-race-reactor` | All clean. |
| AC-9 | `BenchmarkForwardDirect` | Per-UPDATE in-process hot path >= 500k UPDATE/s/core against Phase 1 profile baseline. |
| AC-10 | Pending-never-expires invariant | `TestPendingCacheNeverExpires`: seed N entries with pending consumer count > 0; force TTL-driven eviction attempts; assert entries survive until consumer acks. Pins the `CacheConsumer`/`CacheConsumerUnordered` contract that rs's batch accumulator depends on. |
| AC-11 | rs forward-path plumbing deleted | After implementation, grep returns zero matches for `forwardCh`, `forwardLoop`, `asyncForward`, `releaseCh`, `releaseLoop`, `rsForwardChDepth`, `fwdSendersDefault`, `ze.rs.fwd.senders` in `internal/component/bgp/plugins/rs/`. `rules/no-layering` satisfied. |
| AC-12 | SDK surface value-typed | `p.ForwardCached(ctx, []uint64, []string) error` and `p.ReleaseCached(ctx, []uint64) error` -- no pointers across the SDK boundary (`rules/enum-over-string.md`). |
| AC-13 | ForwardUpdate unchanged behaviour | Existing callers of `ForwardUpdate(selector, updateID, pluginName)` (e.g. any text-RPC consumer that survives this change) observe identical wire output. The shared per-destination helper preserves semantics. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPendingCacheNeverExpires` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-10: seed N entries via `Add` + `Activate(id, 1)`; trigger TTL eviction attempts; assert entries survive until `Ack(id, consumer)` or `Release`. | |
| `TestForwardUpdateDirectRefcount` | `internal/component/bgp/reactor/forward_update_test.go` | AC-2: ForwardUpdateDirect retains once per destination and each `fwdItem.done` releases once; no leak, no double-release. | |
| `TestForwardUpdateDirectCopyOnModify` | `internal/component/bgp/reactor/forward_build_test.go` | AC-3: exactly one Outgoing Peer Pool buffer allocated per modifying destination, through ForwardUpdateDirect. | |
| `TestForwardUpdateDirectOrdering` | `internal/component/bgp/reactor/forward_update_test.go` | AC-4: N=10000 across two destinations via a single ForwardUpdateDirect call preserve order. | |
| `TestForwardBackpressureThroughFastPath` | `internal/component/bgp/reactor/forward_pool_test.go` | AC-5: BackpressureDetected fires on TryDispatch miss; DispatchOverflow fallback path triggered. | |
| `TestFwdBodyCacheHoistsSupersedeAndWithdrawal` | `internal/component/bgp/reactor/forward_update_test.go` | Decision 2 hoisting: two destinations sharing the same rawBodies compute `supersedeKey` + `withdrawal` once, not twice. | |
| `TestForwardUpdateDirectMissingMsgIDIsLogged` | `internal/component/bgp/reactor/forward_update_test.go` | 7a guard: when a msgID is missing (injected via test hook that forces eviction), logs ERROR "BUG:" and continues the rest of the batch. | |
| `TestRSFlushCallsForwardCached` | `internal/component/bgp/plugins/rs/server_forward_test.go` | rs `flushBatch` invokes `p.ForwardCached(ctx, ids, targets)` (via hook), not the deleted `asyncForward`. | |
| `TestRSReplayingGateFilters` | `internal/component/bgp/plugins/rs/server_forward_test.go` | AC-5b: `selectForwardTargets` excludes `Replaying=true` peers before the destination list is passed to ForwardCached. | |
| `TestRSForwardPlumbingDeleted` | `internal/component/bgp/plugins/rs/server_test.go` | AC-11: asserts `forwardCh`, `releaseCh`, `asyncForward`, and related fields do not exist on `RouteServer`. Compile-time enforcement via grep in a Go test. | |
| `BenchmarkForwardDirect` | `internal/component/bgp/reactor/forward_update_bench_test.go` | AC-9: per-UPDATE in-process hot path >= 500k/s/core. | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Source cache refcount | 0..N destinations | N | `-` | N+1 (indicates leak; caught by `TestForwardUpdateDirectRefcount`) |
| Destination fwdItem channel depth | inherited from fwdPool sizing | `-` | `-` | overflow path (verified by `TestForwardBackpressureThroughFastPath`) |
| ForwardCached batch size | 0..maxBatchSize (500) | 500 | `-` | `-` (rs caps at maxBatchSize in `batchForwardUpdate`) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-fastpath-ibgp-identity` | `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` | Sender + two iBGP receivers (matching ASN4 / ADD-PATH). Sender streams 1000 UPDATEs. AC-6a: hex of outbound payload to each receiver == hex of sender's payload. All 1000 delivered. | |
| `bgp-rs-fastpath-ebgp-shared` | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | Sender + two eBGP receivers (same localAS). Sender streams 1000 UPDATEs. AC-6b: hex of outbound to A == hex of outbound to B; both equal source-with-deterministic-prepend. | |
| `bgp-rs-mod-copy` | `test/plugin/bgp-rs-mod-copy.ci` | Two peers + bgp-rs with AS-PATH override on receiver A. AC-3: A's payload is rewritten; B's is unchanged. | |
| `bgp-rs-replaying-gate` | `test/plugin/bgp-rs-replaying-gate.ci` | Two peers + bgp-rs; peer B joins mid-stream triggering replay. While Replaying=true, live UPDATEs are excluded from B. B receives each prefix exactly once (via replay). AC-5b. | |

### Future

- None. All scope fits in this child.

## Files to Modify

- `pkg/plugin/sdk/sdk.go` (or the specific SDK file that owns `Plugin.UpdateRoute`) -- add `Plugin.ForwardCached(ctx, updateIDs, destinations) error` + `Plugin.ReleaseCached(ctx, updateIDs) error`.
- `pkg/plugin/rpc/*` -- new RPC opcodes (binary-encoded: `[]uint64` + `[]string` + context metadata).
- `internal/component/plugin/server/dispatch.go` -- register opcode handlers; dispatch directly to reactor adapter without tokenise / CommandRegistry.All.
- `internal/component/bgp/reactor/reactor_api_forward.go` -- add `ForwardUpdateDirect(updateIDs, destinations, pluginName) error` and `ReleaseUpdates(updateIDs, pluginName) error`. Extract the per-destination loop from existing `ForwardUpdate` into a shared helper both entries call. Extend `fwdBodyCacheEntry` with `supersedeKey` + `withdrawal` fields; hoist the computation to cache miss.
- `internal/component/bgp/reactor/recent_cache.go` -- no code change expected beyond a comment pinning the pending-never-expires invariant; verified by `TestPendingCacheNeverExpires`.
- `internal/component/bgp/plugins/rs/server.go` -- DELETE fields: `forwardCh`, `forwardStop`, `forwardDone`, `releaseCh`, `releaseStop`, `releaseDone`. DELETE functions: `startForwardLoop`, `stopForwardLoop`, `asyncForward`, `startReleaseLoop`, `stopReleaseLoop`. DELETE constants: `rsForwardChDepth`, `fwdSendersDefault`. DELETE env registration: `env.MustRegister("ze.rs.fwd.senders", ...)`. Replace `releaseCache(msgID)` body with a direct `p.ReleaseCached(ctx, []uint64{msgID})` call.
- `internal/component/bgp/plugins/rs/server_forward.go` -- `flushBatch` calls `rs.plugin.ForwardCached(ctx, batch.ids, batch.targets)` instead of `asyncForward(...)`.
- `internal/component/config/environment.go` -- REMOVE the `ze.rs.fwd.senders` env entry if centralised there.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] no | - |
| CLI commands | [ ] no | - |
| Editor autocomplete | [ ] no | - |
| New SDK method | [x] | `pkg/plugin/sdk/`, `pkg/plugin/rpc/`, `internal/component/plugin/server/dispatch.go` |
| New reactor adapter method | [x] | `internal/component/bgp/reactor/reactor_api_forward.go` |
| Deleted rs plumbing (grep guard) | [x] | `internal/component/bgp/plugins/rs/server.go`, `internal/config/environment.go` |
| Functional tests for new behaviour | [x] | four `.ci` files under "Functional Tests" |
| Invariant unit test | [x] | `TestPendingCacheNeverExpires` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] yes | `docs/features.md` -- fast-path forwarding for cache-consumer plugins |
| 2 | Config syntax changed? | [ ] no | - |
| 3 | CLI command added/changed? | [ ] no | - |
| 4 | API/RPC added/changed? | [x] yes | `docs/architecture/api/commands.md` -- new `ForwardCached` + `ReleaseCached` SDK methods; text `cache N forward` / `cache N release` commands remain for fork-mode external plugins |
| 5 | Plugin added/changed? | [x] yes | `docs/guide/plugins.md` -- rs plumbing simplified |
| 6 | Has a user guide page? | [ ] no | - |
| 7 | Wire format changed? | [ ] no (byte-identical on the wire) | - |
| 8 | Plugin SDK/protocol changed? | [x] yes | `.claude/rules/plugin-design.md` -- new SDK methods + opcode added to the 5-stage protocol; `docs/plugin-development/` SDK reference |
| 9 | RFC behavior implemented? | [ ] no | - |
| 10 | Test infrastructure changed? | [x] yes | `docs/functional-tests.md` -- benchmark `BenchmarkForwardDirect` replaces `BenchmarkForwardPassThrough` naming |
| 11 | Affects daemon comparison? | [x] yes | `docs/comparison.md`, `docs/performance.md` -- umbrella updates these on close |
| 12 | Internal architecture changed? | [x] yes | `docs/architecture/core-design.md` -- forwarding fast path section, `fwdBodyCache` hoisting, SDK ForwardCached primitive |

## Files to Create

- `test/plugin/bgp-rs-fastpath-ibgp-identity.ci`
- `test/plugin/bgp-rs-fastpath-ebgp-shared.ci`
- `test/plugin/bgp-rs-mod-copy.ci`
- `test/plugin/bgp-rs-replaying-gate.ci`
- `internal/component/bgp/reactor/forward_update_bench_test.go` (hosts `BenchmarkForwardDirect`)
- `plan/learned/NNN-rs-fastpath-3-passthrough.md` (on completion)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. `/ze-review` gate | Review Gate |
| 5. Full verification | `make ze-test`, `make ze-verify-fast`, `make ze-race-reactor`, `test/perf/run.py --test ze bird` |
| 6–9. Critical review + fixes | Critical Review Checklist |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Executive Summary | Per `rules/planning.md` |

### Implementation Phases

1. **Phase 1 -- Invariant pin.** No behaviour change. Add `TestPendingCacheNeverExpires` to pin the `CacheConsumer`/`CacheConsumerUnordered` contract that the rest of the spec relies on. Add `TestForwardUpdateDirectRefcount` as a scaffold (initially asserting against existing `ForwardUpdate` call path; Phase 2 retargets it).
   - Tests: `TestPendingCacheNeverExpires`.
   - Files: `internal/component/bgp/reactor/recent_cache_test.go` (new or extended).
   - Verify: invariant holds; `make ze-verify-fast` clean; `make ze-race-reactor` clean.

2. **Phase 2 -- SDK primitive + reactor adapter.** Add `Plugin.ForwardCached(ctx, updateIDs, destinations) error` + `Plugin.ReleaseCached(ctx, updateIDs) error` in the SDK. Add binary RPC opcodes wired through `internal/component/plugin/server/dispatch.go` without tokenise. Add `reactorAPIAdapter.ForwardUpdateDirect(updateIDs, destinations, pluginName)` + `ReleaseUpdates`. Extract the per-destination loop from `ForwardUpdate` into a shared helper both `ForwardUpdate` and `ForwardUpdateDirect` call. Extend `fwdBodyCacheEntry` with `supersedeKey` + `withdrawal` fields; hoist computation to cache miss.
   - Tests: `TestForwardUpdateDirectRefcount`, `TestForwardUpdateDirectCopyOnModify`, `TestForwardUpdateDirectOrdering`, `TestForwardBackpressureThroughFastPath`, `TestFwdBodyCacheHoistsSupersedeAndWithdrawal`, `TestForwardUpdateDirectMissingMsgIDIsLogged`.
   - Files: `pkg/plugin/sdk/*`, `pkg/plugin/rpc/*`, `internal/component/plugin/server/dispatch.go`, `internal/component/bgp/reactor/reactor_api_forward.go`.
   - Verify: unit tests pass; `ForwardUpdate` behaviour unchanged (AC-13); `make ze-race-reactor` clean on any touched concurrency paths.

3. **Phase 3 -- rs switch + plumbing delete.** rs `flushBatch` calls `p.ForwardCached(ctx, batch.ids, batch.targets)`. rs `releaseCache` calls `p.ReleaseCached(ctx, []uint64{id})`. DELETE `forwardCh`/`forwardLoop`/`asyncForward`/`releaseCh`/`releaseLoop`/`rsForwardChDepth`/`fwdSendersDefault`/`ze.rs.fwd.senders` (see Files to Modify for full list).
   - Tests: `TestRSFlushCallsForwardCached`, `TestRSReplayingGateFilters`, `TestRSForwardPlumbingDeleted`, functional tests `bgp-rs-fastpath-ibgp-identity.ci`, `bgp-rs-fastpath-ebgp-shared.ci`, `bgp-rs-mod-copy.ci`, `bgp-rs-replaying-gate.ci`.
   - Files: `internal/component/bgp/plugins/rs/server.go`, `internal/component/bgp/plugins/rs/server_forward.go`, `internal/config/environment.go`.
   - Verify: functional tests pass; wire bytes correct (AC-6a + AC-6b); `make ze-race-reactor` clean.

4. **Phase 4 -- Benchmark + soak + umbrella AC-1.** `BenchmarkForwardDirect` captures per-UPDATE hot-path cost. `test/perf/run.py --test ze bird` captures 100k-route throughput against bird. Record results in spec + umbrella.
   - Tests: `BenchmarkForwardDirect`; `test/perf/run.py`.
   - Files: `internal/component/bgp/reactor/forward_update_bench_test.go`.
   - Verify: AC-1 met (>= 400k rps; floor 200k); AC-9 met (>= 500k UPDATE/s/core); `make ze-race-reactor` clean.

5. **Docs + learned summary.** Update `docs/architecture/core-design.md`, `docs/architecture/api/commands.md`, `docs/features.md`, `docs/guide/plugins.md`, `docs/plugin-development/` per Documentation Update Checklist. Write `plan/learned/NNN-rs-fastpath-3-passthrough.md`.

6. **Full verification.** `make ze-verify-fast`, `make ze-race-reactor`, `test/perf/run.py --test ze bird`.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1 through AC-13) has test + file:line. AC-1 and AC-9 benchmark results pasted in spec. |
| Correctness | AC-6a hex identity for iBGP same-ctx. AC-6b A==B + source-with-prepend for eBGP shared localAS. AC-3 copy-on-modify through ForwardUpdateDirect. |
| Rule: no-layering | `forwardCh`/`forwardLoop`/`asyncForward`/`releaseCh`/`releaseLoop` DELETED (not left beside ForwardCached). Single shared per-destination helper, not a parallel lean dispatcher. |
| Rule: goroutine-lifecycle | No new per-event goroutines. rs worker count unchanged. Forward-sender goroutines removed. |
| Rule: buffer-first | No new `append` or `make([]byte)` on wire paths. New SDK opcodes use existing frame buffers. |
| Rule: design-principles | Copy-on-modify via Outgoing Peer Pool is the only copy allowed. Per-source ordering preserved. |
| Rule: enum-over-string | SDK surface uses `[]uint64` + `[]string` + value-typed context. No pointers across the SDK boundary. Destination parsing at adapter boundary once per batch. |
| Rule: integration-completeness | rs must be wired to the new SDK methods; no feature left in isolation. Grep-guard AC-11 enforces deletion. |
| Reactor concurrency | `make ze-race-reactor` clean. ForwardUpdateDirect shares `fwdBodyCache` map with ForwardUpdate; both callers must respect its scope (per-ForwardUpdate-call, not shared across calls). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `BenchmarkForwardDirect` meets AC-9 | `go test -run=^$ -bench=BenchmarkForwardDirect ./internal/component/bgp/reactor/...` |
| `.ci` tests pass | `bin/ze-test plugin -p bgp-rs-fastpath-ibgp-identity`, `-p bgp-rs-fastpath-ebgp-shared`, `-p bgp-rs-mod-copy`, `-p bgp-rs-replaying-gate` |
| ze-perf 100k meets umbrella AC-1 | `python3 test/perf/run.py --test ze bird`, read `test/perf/results/ze.json` |
| AC-6a hex identity | `bgp-rs-fastpath-ibgp-identity.ci` asserts hex(outbound) == hex(sender payload) |
| AC-6b shared determinism | `bgp-rs-fastpath-ebgp-shared.ci` asserts hex(outbound A) == hex(outbound B) and equals source with deterministic localAS prepend |
| AC-10 pending-never-expires | `go test -run TestPendingCacheNeverExpires ./internal/component/bgp/reactor/...` passes |
| AC-11 rs plumbing deleted | `grep -rn 'forwardCh\|asyncForward\|releaseCh\|rsForwardChDepth\|fwdSendersDefault\|ze.rs.fwd.senders' internal/component/bgp/plugins/rs/` returns zero |
| Learned summary | `ls plan/learned/*rs-fastpath-3-passthrough*.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | No change to wire parsing; bounds checks unchanged. |
| Resource exhaustion | Refcount cannot underflow; fwdItem channel bounded. |
| Error leakage | Unreachable-state guard (`TestForwardUpdateDirectMissingMsgIDIsLogged`) logs ERROR "BUG:" and continues; does not panic the daemon. Transport errors on ForwardCached log per existing `updateRoute` severity taxonomy. No state dumps in log messages. |
| Concurrency | `make ze-race-reactor` clean; refcount uses atomic. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| AC-6a hex diff (iBGP-identity) | Fix in Phase 2; zero-copy rawBodies path regressed. Check `srcCtxID == destCtxID` short-circuit (line 592 of `reactor_api_forward.go`). |
| AC-6b bytes differ between A and B (eBGP-shared) | Fix in Phase 2; EBGP wire cache key or fwdBodyCache entry differs when it should not. |
| AC-4 ordering test fails | Fix in Phase 3; rs worker serialisation violated, or ForwardUpdateDirect reordered updateIDs within the batch. |
| AC-5 backpressure test fails | Fix in Phase 2/3; direct blocking on ForwardCached must surface through `workers.BackpressureDetected`. |
| AC-5b Replaying gate fails | Fix in Phase 3; rs's `selectForwardTargets` should filter Replaying peers before batch.targets accumulates. |
| AC-10 pending-never-expires fails | Fix in Phase 1 before proceeding. If the cache contract is broken, the rest of the spec cannot be trusted. |
| AC-11 plumbing grep returns matches | Delete the residual plumbing in Phase 3; `rules/no-layering` violated. |
| AC-1 benchmark below target | Identify next hop with pprof (engine-side or reactor-side). If `fwdBodyCache` hoisting missed a hotspot, file a follow-up child. Do not claim done. |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

<!-- LIVE -->

### Decision Log (2026-04-18, captured during `/ze-design` session)

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | Mechanism for rs -> reactor fast path | Reactor forwarding primitive exposed via SDK: `p.ForwardCached(ctx, updateIDs, destinations)` + `p.ReleaseCached(ctx, updateIDs)`. Not rs-specific; any plugin may use it. | Preserves rs worker + Replaying filter + withdrawal tracking; skips tokenise (19.4 % alloc) + CommandRegistry.All (~5 %); same SDK shape for in-process and fork-mode plugins. |
| 2 | Fast-path depth: swap-call-only vs swap-call-and-hoist | Swap the call AND hoist shared per-destination work by extending `fwdBodyCache` to memoise `supersedeKey` + `withdrawal`. One path, one cache; not a parallel dispatcher. | Benefits all forwarders (ForwardUpdate + ForwardUpdateDirect). Small structural change. `feedback_no_deferral` says don't split hard work. |
| 3a | Destination encoding on SDK method | `[]string` on SDK surface (matches existing `UpdateRoute(peerSelector string)` convention); parsed to `netip.AddrPort` once at the reactor adapter boundary. | Consistency with existing SDK RPC shape. Parse cost amortised across batch. |
| 3b | Cross-boundary pointer rule scope | Not applicable: bgp-rs <-> reactor is WITHIN the BGP component. `rules/memory.md` / cross-boundary-pointer rule targets component/engine seams. | User clarified: pointer is fine within the same component/engine. "Plugin" was a bad wording choice in the earlier memory entry. |
| 4 | rs batch accumulator | Keep. `ForwardCached` takes `[]uint64`. maxBatchSize=500 preserved (learned/625: +9 % throughput). | Amortises MuxConn frame + engine RPC handler overhead even after tokenise is removed. rs worker already has the batching state. |
| 5 | rs's forward/release plumbing | Delete: `forwardCh`, `forwardLoop`, `asyncForward`, `releaseCh`, `releaseLoop`, `rsForwardChDepth`, `fwdSendersDefault`, `ze.rs.fwd.senders`. Per-source rs worker calls SDK methods directly. | `rules/no-layering`: the plumbing existed to absorb text-RPC cost that no longer exists. Backpressure becomes cleaner (single signal: `workers.BackpressureDetected`). |
| 6 | AC-6 wire-bytes semantic | Split into AC-6a (strict iBGP-identity hex match) and AC-6b (eBGP shared determinism: A=B bytes, equals source with deterministic localAS prepend). | Each invariant is independently testable; failures localise to the specific optimisation that broke. Covers both the common RS-eBGP case and the pure iBGP zero-copy case. |
| 7a | Missing msgID in batch | Impossible by `CacheConsumer` + `CacheConsumerUnordered` pending-count contract. `TestPendingCacheNeverExpires` pins it. If it ever fires, `ForwardUpdateDirect` logs ERROR "BUG:" and continues (no panic -- 24/7 daemon stability). | User: "if we queue we should ensure expiry is impossible." The contract already guarantees this; test pins it. |
| 7b | Destination peer gone | Skip, continue. | Matches existing per-destination loop semantics. |
| 7c | fwdPool.TryDispatch full | TryDispatch -> DispatchOverflow -> `done()` on stopped pool. Unchanged. | Existing contract; no new behaviour. |
| 7d | Transport error on ForwardCached | DEBUG (if stopping) / WARN (connection error) / ERROR otherwise. No retry. Cache entries expire via TTL. | Matches current `updateRoute` severity taxonomy (server.go:476-485). Tighter lifecycle (ack protocol + retry) is a separate spec if soak testing surfaces leakage. |
| 7e | Retain/Release/Ack contract | Unchanged. `ForwardUpdateDirect` does `defer Ack(id, pluginName)` per looked-up id; per-destination `Retain` paired with `fwdItem.done` `Release`. | Existing contract; this child does not change cache-consumer semantics. |

### Key architectural observations

- **The fast-path primitive is reactor-owned, not rs-owned.** Naming (`ForwardCached`, `ReleaseCached`) and placement in the SDK reflects that. Any future plugin with cached msgIDs and a resolved destination list uses the same primitive.
- **bgp-redistribute (separate spec) does NOT use this primitive.** It synthesises fresh announces via `UpdateRoute(ctx, "*", "update text ...")`, a different hot path at much lower volume. Orthogonal design.
- **DirectBridge `*WireUpdate` pointer delivery is within BGP component.** Not a cross-component seam. Retain/Release discipline keeps it sound. `rules/memory.md` not violated.
- **Reading "enum" widely:** the SDK surface carries `[]uint64` + `[]string` (value types). Destination strings are parsed once at the adapter boundary. See `rules/enum-over-string.md` "Where Strings Are OK" -- the destination string is the existing SDK contract (matches `UpdateRoute(peerSelector string)`).

## RFC Documentation

- RFC 4271 — UPDATE format; pass-through must reproduce wire bytes verbatim.
- RFC 7606 — attribute sanitisation on receive (ze already applies this pre-forward; no change here).

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan

| File | Status | Notes |
|------|--------|-------|

### Audit Summary

- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes
- [ ] ze-perf 100k meets umbrella AC-1
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)

- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rs-fastpath-3-passthrough.md`
- [ ] Summary included in commit
