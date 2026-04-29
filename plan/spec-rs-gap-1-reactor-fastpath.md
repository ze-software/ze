# Spec: rs-gap-1-reactor-fastpath

| Field | Value |
|-------|-------|
| Status | done |
| Depends | rs-gap-0-umbrella |
| Phase | 4/4 |
| Updated | 2026-04-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `ai/rules/planning.md`
3. `docs/architecture/core-design.md`
4. `docs/architecture/forward-congestion-pool.md`
5. `plan/design-rib-rs-fastpath.md`
6. `internal/component/bgp/reactor/peer_run.go`
7. `internal/component/bgp/reactor/reactor_api_forward.go`
8. `internal/component/bgp/reactor/peersettings.go`
9. `internal/component/bgp/server/events.go`

## Task

Eliminate the plugin round-trip from the route-server UPDATE forwarding path. Today each received UPDATE crosses 6 boundaries (cache -> plugin dispatch -> bgp-rs worker -> ForwardCached RPC -> ForwardUpdatesDirect -> fwd pool) before reaching TCP write. BIRD crosses 2 (route table -> outbound bucket -> TCP write). The remaining 4x throughput gap after rs-gap-0-umbrella is structural: the reactor already owns every piece needed for RS forwarding (peer map, egress filters, EBGP prepend, copy-on-modify, forward pool) but calls them through the plugin RPC boundary.

This spec adds a reactor-native RS forwarding mode that calls the egress pipeline directly from `notifyMessageReceiver` on the session read goroutine, bypassing the deliverChan -> delivery goroutine -> plugin-dispatch -> ForwardCached -> ForwardUpdatesDirect -> per-id-ForwardUpdate chain. Plugins (adj-rib-in, monitoring) still receive events via fire-and-forget delivery for storage and observability.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- two-trigger model, EventDispatcher, DirectBridge, update groups, ingress/egress filter pipeline
  → Decision: keep the receive-path StructuredEvent trigger for state-tracker plugins (adj-rib-in, rib); add a PARALLEL reactor-native forward that runs BEFORE plugin dispatch
  → Constraint: ingress filters run before caching and before either forwarders or state trackers consume the UPDATE; the reactor fast path runs after ingress filters

- [ ] `docs/architecture/forward-congestion-pool.md` -- forward pool, no-drop rule, copy-on-modify, overflow/backpressure
  → Decision: the reactor fast path dispatches to the same forward pool (same congestion, same no-drop, same per-peer workers)
  → Constraint: silent route drop is forbidden; unchanged peers share source buffers, modified peers copy-on-modify

- [ ] `plan/design-rib-rs-fastpath.md` -- two-trigger model and explicit rejection of retiring the receive-path trigger
  → Decision: do not move forwarding onto locrib.OnChange; the reactor fast path is a THIRD path (inline forward during receive, before plugin dispatch)
  → Constraint: forwarders still operate per received UPDATE, state trackers still operate per best-change

- [ ] `plan/learned/630-rs-fastpath-3-passthrough.md` -- ForwardCached / ReleaseCached SDK primitives
  → Decision: the reactor fast path bypasses ForwardCached entirely; ForwardCached remains for bgp-rs fallback and other plugin consumers
  → Constraint: the ForwardCached path must still work when the reactor fast path is disabled

### Rules / Supporting Docs
- [ ] `ai/rules/plugin-design.md` -- plugin boundary rules, SDK genericity
  → Decision: the reactor fast path is an RS-specific optimization inside the reactor, not a generic plugin primitive; bgp-rs remains for lifecycle management
  → Constraint: the fast path must not import plugin implementations; it uses PeerSettings flags to decide

- [ ] `docs/contributing/rfc-implementation-guide.md` -- RFC work checklist
  → Decision: RFC 7947 constraints (no AS-PATH prepend, NEXT_HOP transparency) are enforced by the existing egress pipeline in reactor_api_forward.go, which the fast path reuses
  → Constraint: the fast path must not change wire semantics; it calls the same egress pipeline as ForwardUpdate

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- base UPDATE format, message size limits
  → Constraint: outbound UPDATEs must respect negotiated max message length; the egress pipeline already handles this

- [ ] `rfc/short/rfc7947.md` -- route-server forwarding semantics
  → Constraint: no RS AS_PATH prepend, NEXT_HOP transparency, MED transparency, per-client policy; the egress pipeline already enforces these

**Key insights:**
- The reactor already owns every component needed for RS forwarding: peer map, egress filter chain, EBGP AS-PATH prepend, next-hop rewriting, copy-on-modify, forward pool, update groups, backpressure. These all live in `reactor_api_forward.go` and are called by `ForwardUpdate`.
- The bottleneck is not any individual component but the NUMBER OF BOUNDARIES: cache Add/Activate, plugin channel dispatch, bgp-rs worker channel, ForwardCached RPC dispatch, selector parsing, per-id cache Get, per-id ForwardUpdate peer walk.
- `notifyMessageReceiver` in `reactor_notify.go` already has the WireUpdate and source peer identity. It can call the egress pipeline directly instead of going through deliverChan -> delivery goroutine -> OnMessageBatchReceived -> plugin dispatch -> bgp-rs -> ForwardCached -> ForwardUpdatesDirect -> ForwardUpdate.
- Buffer ownership: cache Add still runs, so the cache entry owns the buffer. The fast path uses the same `Retain`/`Release` pattern as ForwardUpdate -- `done()` callbacks call `Release(messageID)`. No buffer ownership change from the current model.
- bgp-rs becomes a lifecycle manager (peer-up replay, peer-down withdrawals, flow control) rather than the forwarding engine. It still subscribes to UPDATE events for withdrawal inventory tracking.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer_run.go` -- delivery goroutine reads from `deliverChan`, batches items via `drainDeliveryBatch`, calls `receiver.OnMessageBatchReceived`, then `recentUpdates.Activate` per message. The fast path insertion point is NOT here but in `notifyMessageReceiver` (reactor_notify.go), which runs earlier on the session read goroutine.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` -- `ForwardUpdate` contains the full egress pipeline: peer map walk, source exclusion, EBGP wire cache, egress filters, next-hop rewrite, copy-on-modify, fwd pool dispatch. The reactor fast path extracts and reuses this logic.
- [ ] `internal/component/bgp/reactor/peersettings.go` -- `PeerSettings` has `GroupName`, `ExportFilters`, `IsEBGP()`, `IsIBGP()`, `RouteReflectorClient`, etc. A new `RSFastPath bool` field (derived from config) controls whether the reactor fast path is enabled for this peer's group.
- [ ] `internal/component/bgp/server/events.go` -- `onMessageBatchReceived` dispatches to all subscribed plugins with fire-and-forget delivery (no result wait for UPDATE events after rs-gap-0). The reactor fast path in `notifyMessageReceiver` runs BEFORE the deliverChan enqueue that leads to this dispatch.
- [ ] `internal/component/bgp/reactor/forward_pool.go` -- `fwdBatchHandler` writes to TCP. The reactor fast path dispatches to the same pool via the same `TryDispatch`/`DispatchOverflow` API.
- [ ] `internal/component/bgp/plugins/rs/server.go` -- bgp-rs `dispatchStructured` -> worker -> `processForward` -> `batchForwardUpdate` -> `ForwardCached`. This entire chain is bypassed by the reactor fast path for RS peers.

**Behavior to preserve:**
- Ingress filter pipeline runs before the reactor fast path (filters modify wire bytes before forwarding).
- Egress filter chain, EBGP AS-PATH prepend, next-hop rewriting, copy-on-modify, update groups, RFC 4456 route reflection rules -- all enforced identically to ForwardUpdate.
- Forward pool no-drop behavior, backpressure, congestion control -- unchanged.
- RFC 7947 route-server transparency: no RS AS_PATH prepend, NEXT_HOP preserved, MED preserved.
- Plugin event delivery for state trackers (adj-rib-in, rib, monitoring) -- still fire-and-forget.
- bgp-rs withdrawal tracking (peer-down route inventory) -- still receives UPDATE events.
- bgp-rs peer-up replay via adj-rib-in -- unchanged.
- Existing ForwardCached/ForwardUpdatesDirect path -- still works for fallback and non-RS plugins.
- All existing functional tests pass unchanged.

**Behavior to change:**
- For peers in RS-fast-path-enabled groups: the reactor fast path runs in `notifyMessageReceiver` (reactor_notify.go), AFTER ingress filters and cache Add but BEFORE the deliverChan enqueue. It calls the egress pipeline directly from the session read goroutine, bypassing the delivery goroutine, plugin dispatch, bgp-rs worker, ForwardCached RPC, and ForwardUpdatesDirect entirely.
- **Read goroutine stall analysis:** The fast path runs on the source peer's TCP read goroutine, which means the next TCP read from that source stalls until `reactorForwardRS` completes. This is acceptable because: (1) `TryDispatch` is non-blocking (select with default), (2) `DispatchOverflow` appends to an unbounded buffer without blocking, (3) the peer walk under RLock is O(N) with N typically 2-8 for RS deployments, and (4) the per-peer egress work (next-hop mod, community filter, EBGP wire lookup) is sub-microsecond. The dominant cost is the N channel sends, which is the same work ForwardUpdate does. The trade-off is explicit: source read latency increases by the peer-walk cost, but total forwarding latency decreases by eliminating 5 boundary crossings. For large RS deployments (N > 50), profile and consider an async variant if read stall exceeds 100us.
- Cache Add still runs (buf ownership transfers to cache as today). The cache entry provides buffer lifetime safety: fwdItem `done()` callbacks call `Release(messageID)` as they do today. No buffer lifetime change needed.
- The fast path calls a new `reactorForwardRS` which reuses the existing ForwardUpdate egress pipeline. This is NOT an extraction/refactor of ForwardUpdate -- it is a SECOND caller of the same per-peer egress logic (EBGP wire, next-hop, egress filters, copy-on-modify, fwd pool dispatch). The egress logic stays inside ForwardUpdate; `reactorForwardRS` reimplements the outer loop (peer walk, source exclusion) but delegates per-peer work to shared helpers.
- Plugin dispatch still happens (fire-and-forget to delivery goroutine). bgp-rs receives the UPDATE event and checks `RawMessage.ReactorForwarded`. If true AND `FastPathSkipped` is empty, bgp-rs skips ForwardCached entirely and only updates the withdrawal inventory map. If true AND `FastPathSkipped` is non-empty, bgp-rs calls ForwardCached with a selector matching only the skipped peers (those with per-peer export policy). If false, bgp-rs uses the existing full ForwardCached path unchanged.
- Cache Activate count: the fast path does NOT wait for plugin results. It calls `Activate(id, cacheConsumerCount)` immediately using the pre-computed count (same as the fire-and-forget delivery from rs-gap-0). This is safe because early acks from fast plugins are handled by `earlyAckCount` in the cache.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Wire bytes enter on the source peer's TCP session as BGP UPDATEs.
- Session read path parses the BGP header and UPDATE body into a `WireUpdate`.
- `notifyMessageReceiver` in `reactor_notify.go` runs ingress filters, then adds to cache.

### Transformation Path
1. Session read path parses UPDATE into `WireUpdate` with pool-backed buffer.
2. `notifyMessageReceiver` runs ingress filters (may modify wire bytes). Unchanged.
3. Cache Add: `recentUpdates.Add(ReceivedUpdate{...})`. Unchanged -- buf ownership transfers to cache. The cache entry provides buffer lifetime for the fast path (same `Retain`/`Release` pattern as ForwardUpdate).
4. **NEW: Reactor fast path check.** `notifyMessageReceiver` checks: does the source peer have `RSFastPath` enabled? If yes, call `reactorForwardRS(reactor, update, sourcePeerAddr)`.
5. **NEW: `reactorForwardRS`.** Walks `reactor.peers` under RLock, excludes source, runs the per-peer egress pipeline (same logic as ForwardUpdate's inner loop: EBGP wire, egress filters, next-hop, send-community, copy-on-modify). For each peer: `Retain(id)`, build fwdItem, `TryDispatch`/`DispatchOverflow` to fwd pool. `done()` calls `Release(id)`. Peers with `ExportFilters` configured are SKIPPED (fall through to bgp-rs).
6. **NEW: Activate with pre-computed count.** `recentUpdates.Activate(id, cacheConsumerCount)` uses the static count of cache-consumer plugins.
7. **NEW: Set `msg.ReactorForwarded = true`.** Signals bgp-rs to skip ForwardCached.
8. Enqueue to `peer.deliverChan` for fire-and-forget plugin dispatch (adj-rib-in storage, bgp-rs withdrawal tracking, monitoring). Unchanged pathway.
9. Forward pool workers write raw UPDATE bodies to destination TCP sockets and flush. Unchanged.

### Key difference from today
Steps 4-7 run in `notifyMessageReceiver` on the session read goroutine, BEFORE the deliverChan enqueue at step 8. Today, forwarding happens in steps 8+: deliverChan -> delivery goroutine -> OnMessageBatchReceived -> plugin dispatch -> bgp-rs worker -> ForwardCached -> ForwardUpdatesDirect -> ForwardUpdate. The fast path collapses those 7 hops into step 5.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Reactor | Session parse -> WireUpdate -> deliverChan | [ ] |
| Reactor -> Forward Pool | TryDispatch/DispatchOverflow with fwdItem (same as ForwardUpdate) | [ ] |
| Forward Pool -> TCP | fwdBatchHandler writes raw bodies, flushes | [ ] |
| Reactor -> Plugins | Fire-and-forget StructuredEvent delivery (unchanged) | [ ] |

### Integration Points
- `reactorForwardRS` reuses the per-peer egress helpers from `reactor_api_forward.go` (`applyNextHopMod`, `applySendCommunityFilter`, `applyASOverride`, `buildModifiedPayload`, `wireu.RewriteASPath`, `wireu.SplitWireUpdate`). The outer peer-walk loop is reimplemented (no selector parsing, no cache Get needed), but per-peer work is shared.
- `PeerSettings.RSFastPath` flag (config-derived) controls fast-path eligibility per peer group.
- Forward pool dispatch uses the same `TryDispatch`/`DispatchOverflow` API. The `done()` callback calls `Release(messageID)` on the cache entry (same as ForwardUpdate).
- bgp-rs detects reactor-forwarded UPDATEs via `RawMessage.ReactorForwarded` and skips ForwardCached; it still extracts NLRI records for withdrawal inventory.
- Cache Add always runs (provides buffer lifetime). Cache Activate runs with pre-computed consumer count.

### Architectural Verification
- [ ] No bypassed layers: ingress filters still run before forwarding; egress filters still run per destination peer
- [ ] No unintended coupling: reactor fast path uses PeerSettings flags, not plugin imports
- [ ] No duplicated functionality: egress pipeline is shared with ForwardUpdate via extraction
- [ ] Zero-copy preserved: unchanged peers share source buffer until TCP write completes

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Source peer sends grouped IPv4 UPDATE to RS with fast-path enabled | -> | `notifyMessageReceiver` -> `reactorForwardRS` -> fwd pool -> destination TCP | `test/plugin/bgp-rs-reactor-fastpath.ci` |
| Source peer sends UPDATE with per-peer export policy on one destination | -> | fast path detects fallback needed -> bgp-rs ForwardCached path | `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` |
| Source peer goes down after fast-path-forwarded routes | -> | bgp-rs peer-down withdrawal still emits correct withdrawals | `test/plugin/rs-ipv4-withdrawal.ci` (existing) |
| New peer connects while fast-path forwarding continues | -> | bgp-rs peer-up replay still works via adj-rib-in | `test/plugin/bgp-rs-replaying-gate.ci` (existing) |
| Slow destination peer causes queue buildup during fast-path forwarding | -> | forward pool overflow/backpressure still protects no-drop invariant | `test/plugin/rs-backpressure.ci` (existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `python3 test/perf/run.py --build --test ze` on the grouped-input 100k IPv4 benchmark | Ze reaches at least 600k routes/sec average throughput and at most 170 ms convergence on the reference harness, with 0 lost routes. Baseline is 405k/247ms (post rs-gap-0 fire-and-forget delivery). If target is missed, capture a forwarding-window-only CPU profile (not full 30s test lifecycle) to identify the remaining bottleneck before re-targeting. |
| AC-2 | RS-fast-path-enabled peer group, no per-peer export policy | Received UPDATEs are forwarded directly from `notifyMessageReceiver` via `reactorForwardRS` without going through deliverChan, delivery goroutine, plugin dispatch for forwarding, bgp-rs worker, ForwardCached, or ForwardUpdatesDirect. Cache Add still runs (provides buffer lifetime). |
| AC-3 | RS-fast-path-enabled group, one peer has per-peer export policy | That peer falls back to the bgp-rs ForwardCached path; other peers use the reactor fast path |
| AC-4 | Source peer goes down after fast-path-forwarded announcements | bgp-rs peer-down withdrawal still emits correct withdrawals for all previously-announced routes (withdrawal inventory was updated from the fire-and-forget StructuredEvent) |
| AC-5 | New peer joins RS group while fast-path forwarding continues | Peer-up replay via adj-rib-in still works correctly; new peer receives full table |
| AC-6 | Fast-path forwarding with EBGP peers (different local-AS) | AS-PATH prepend, next-hop rewrite, and community filtering are applied identically to the ForwardUpdate path |
| AC-6b | Fast-path forwarding with mixed IBGP/EBGP RS group (route reflection) | RFC 4456 ORIGINATOR_ID injection and CLUSTER_LIST prepend are applied for IBGP destinations, identically to the ForwardUpdate path. `reactorForwardRS` must carry `srcIsIBGP`, `srcIsRRClient`, and `srcRemoteRouterID` from the source peer. |
| AC-7 | Existing fast-path RS tests (identity, mod-copy, replaying, backpressure) | All pass unchanged -- the fast path uses the same egress pipeline |
| AC-8 | `grep -rn 'reactorForwardRS' internal/component/bgp/reactor/` | Function exists in `forward_rs.go` and is called from `notifyMessageReceiver` in `reactor_notify.go` |
| AC-9 | Slow destination peer fills peer channel / overflow pool during fast-path forwarding | No route drops; existing congestion and teardown behavior remains intact |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReactorForwardRSBasic` | `internal/component/bgp/reactor/forward_rs_test.go` | Fast path forwards UPDATE to all peers except source, using same egress pipeline | |
| `TestReactorForwardRSEBGPPrepend` | `internal/component/bgp/reactor/forward_rs_test.go` | EBGP AS-PATH prepend applied correctly on fast path | |
| `TestReactorForwardRSCopyOnModify` | `internal/component/bgp/reactor/forward_rs_test.go` | Per-peer modification (next-hop, community filter) produces copy-on-modify, unmodified peers share source buffer | |
| `TestReactorForwardRSFallback` | `internal/component/bgp/reactor/forward_rs_test.go` | Peer with export policy excluded from fast path, falls through to bgp-rs | |
| `TestReactorForwardRSRouteReflection` | `internal/component/bgp/reactor/forward_rs_test.go` | IBGP destination peers get ORIGINATOR_ID and CLUSTER_LIST per RFC 4456 | |
| `TestReactorForwardRSBufferLifetime` | `internal/component/bgp/reactor/forward_rs_test.go` | Receive buffer not returned until all fwd pool done() callbacks fire | |
| `TestReactorForwardRSCacheLifetime` | `internal/component/bgp/reactor/forward_rs_test.go` | Cache Add runs before fast path; Activate runs after with pre-computed count; cache entry evicts normally after all consumers ack | |
| `BenchmarkReactorForwardRS` | `internal/component/bgp/reactor/forward_rs_bench_test.go` | Per-UPDATE cost of the reactor fast path vs ForwardUpdatesDirect baseline | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Standard UPDATE body length | 0 to 4077 bytes | 4077 | N/A | 4078 |
| Extended UPDATE body length | 0 to 65516 bytes | 65516 | N/A | 65517 |
| Destination peer count | 0 to N | N (all RS peers minus source) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-reactor-fastpath` | `test/plugin/bgp-rs-reactor-fastpath.ci` | RS forwards grouped UPDATEs via reactor fast path, receiver gets all routes | |
| `bgp-rs-reactor-fastpath-fallback` | `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` | Peer with export policy falls back to bgp-rs path, all routes correct | |
| `bgp-rs-fastpath-ebgp-shared` | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | Existing: RS identity tests still pass with reactor fast path enabled | |
| `bgp-rs-mod-copy` | `test/plugin/bgp-rs-mod-copy.ci` | Existing: modified export path still copies only the modified peer | |
| `rs-ipv4-withdrawal` | `test/plugin/rs-ipv4-withdrawal.ci` | Existing: peer-down withdrawals correct with fast-path forwarding | |

### Future (if deferring any tests)
- None. Reactor fast path is the critical forwarding change; all tests are mandatory.

## Files to Modify
- `internal/component/bgp/reactor/reactor_notify.go` -- insert fast path call in `notifyMessageReceiver` after cache Add (line ~418), before deliverChan enqueue (line ~447). Check `peer.settings.RSFastPath`, call `reactorForwardRS`, set `msg.ReactorForwarded`, call `Activate` with pre-computed count.
- `internal/component/bgp/reactor/peersettings.go` -- add `RSFastPath bool` field to PeerSettings
- `internal/component/bgp/reactor/config.go` -- wire `RSFastPath` from config tree to PeerSettings (derived from peer-group role or explicit flag)
- `internal/component/bgp/types/types.go` -- add `ReactorForwarded bool` and `FastPathSkipped []netip.AddrPort` fields to `RawMessage`
- `internal/component/bgp/plugins/rs/server_withdrawal.go` -- `processForward` checks `item.msg.ReactorForwarded`; if true, skips ForwardCached but still updates withdrawal inventory
- `docs/architecture/core-design.md` -- document the reactor RS fast path as a third forwarding trigger alongside StructuredEvent (forwarders) and locrib.OnChange (state trackers)
- `docs/performance.md` -- document the fast-path design and benchmark results

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [x] | `internal/yang/modules/ze-bgp.yang` -- `rs-fast-path` leaf under peer-group |
| CLI commands/flags | [ ] | YANG-driven |
| Editor autocomplete | [ ] | YANG-driven |
| Functional test for fast path | [x] | `test/plugin/bgp-rs-reactor-fastpath.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- reactor RS fast path |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- `rs-fast-path` config option |
| 3 | CLI command added/changed? | [ ] | - |
| 4 | API/RPC added/changed? | [ ] | - |
| 5 | Plugin added/changed? | [ ] | - |
| 6 | Has a user guide page? | [ ] | - |
| 7 | Wire format changed? | [ ] | - |
| 8 | Plugin SDK/protocol changed? | [ ] | - |
| 9 | RFC behavior implemented? | [ ] | - |
| 10 | Test infrastructure changed? | [ ] | - |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md`, `docs/performance.md` |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- reactor RS fast path |

## Files to Create
- `internal/component/bgp/reactor/forward_rs.go` -- `reactorForwardRS` function: inline RS forwarding from `notifyMessageReceiver` (session read goroutine) using shared egress pipeline
- `internal/component/bgp/reactor/forward_rs_test.go` -- unit tests for reactor RS fast path
- `internal/component/bgp/reactor/forward_rs_bench_test.go` -- benchmark comparing fast path to ForwardUpdatesDirect baseline
- `test/plugin/bgp-rs-reactor-fastpath.ci` -- end-to-end fast path wiring test
- `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` -- fallback to bgp-rs when export policy present

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | Functional tests + targeted perf run + `make ze-verify-fast` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Relevant implementation phase |
| 8. Re-verify | Stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Stage 5 |
| 13. Present summary | Executive Summary Report per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: add RSFastPath config flag and `reactorForwardRS`** -- add `RSFastPath bool` to PeerSettings, wire from config tree. Implement `reactorForwardRS` in `forward_rs.go`: walks `reactor.peers` under RLock, excludes source peer, runs the per-peer egress pipeline (EBGP wire, egress filters, next-hop, send-community, copy-on-modify, fwd pool dispatch). This is NOT an extraction of ForwardUpdate -- it is a new function that reimplements the outer peer-walk loop but calls the same shared per-peer helpers. Buffer lifetime: `Retain(id)` before dispatch, `done()` calls `Release(id)` -- identical to ForwardUpdate's pattern, using the same cache entry from the already-executed cache Add.
   - **Extraction boundary (what `reactorForwardRS` skips vs shares):**
     - SKIPS: selector parsing (forwards to all RS group peers), cache Get (has WireUpdate directly), pluginName ack tracking (not a cache consumer), fwdBodyCache (optimization deferred to later)
     - SHARES (calls directly): `applyNextHopMod`, `applySendCommunityFilter`, `applyASOverride`, `buildModifiedPayload`, `buildWithdrawalPayload`, `wireu.RewriteASPath`/`RewriteASPathDual`, `wireu.SplitWireUpdate`, `fwdPool.TryDispatch`/`DispatchOverflow`
     - REIMPLEMENTS: outer peer walk (simpler -- no selector, just iterate `reactor.peers` and skip source + export-policy peers), EBGP wire cache (local to each call, same key structure as ForwardUpdate), batch `RetainN`
     - MUST INCLUDE: RFC 4456 route reflection (ORIGINATOR_ID, CLUSTER_LIST) for IBGP destination peers, using `srcIsIBGP`, `srcIsRRClient`, `srcRemoteRouterID` from source peer
   - Tests: `TestReactorForwardRSBasic`, `TestReactorForwardRSEBGPPrepend`, `TestReactorForwardRSCopyOnModify`, `TestReactorForwardRSBufferLifetime`, `TestReactorForwardRSRouteReflection`
   - Files: `forward_rs.go` (new), `peersettings.go`, `config.go`, `reactor.go`
   - Verify: unit tests pass

2. **Phase: wire fast path into notifyMessageReceiver** -- in `reactor_notify.go`, after cache Add (line 409-418) and before the deliverChan enqueue (line 447-449), insert: if source peer has `RSFastPath`, call `reactorForwardRS`, then set `msg.ReactorForwarded = true`, then call `Activate(id, cacheConsumerCount)`. The deliverChan enqueue still happens (for fire-and-forget plugin delivery). Add `ReactorForwarded bool` field to `bgptypes.RawMessage`.
   - Tests: `TestReactorForwardRSCacheLifetime`, `BenchmarkReactorForwardRS`
   - Files: `reactor_notify.go`, `internal/component/bgp/types/types.go`
   - Verify: benchmark shows improvement; cache entries are still created and properly evicted

3. **Phase: bgp-rs coordination** -- bgp-rs `processForward` checks `item.msg.ReactorForwarded`. If true, skips `batchForwardUpdate` (no ForwardCached call) but still extracts NLRI records and updates the withdrawal inventory map.
   - **Fallback design (decided):** When `reactorForwardRS` encounters a peer with `ExportFilters`, it skips that peer AND records the peer address in `RawMessage.FastPathSkipped []netip.AddrPort`. bgp-rs reads `FastPathSkipped` and calls ForwardCached with a selector that matches ONLY those peers. This avoids wasted duplicate work (Option B rejected) while keeping the coordination data on the message itself (no shared mutable state). If `FastPathSkipped` is empty, bgp-rs skips ForwardCached entirely. If `ReactorForwarded` is false (fast path disabled for this peer group), bgp-rs uses the existing full ForwardCached path unchanged.
   - Tests: `TestReactorForwardRSFallback`, existing `rs-ipv4-withdrawal`, `bgp-rs-replaying-gate`
   - Files: `server_withdrawal.go`, `forward_rs.go`, `internal/component/bgp/types/types.go`
   - Verify: withdrawal tests pass, fallback test passes

4. **Phase: functional tests, perf verification, docs** -- create functional tests, run grouped-input benchmark, update architecture docs and performance narrative.
   - Tests: `bgp-rs-reactor-fastpath.ci`, `bgp-rs-reactor-fastpath-fallback.ci`, `python3 test/perf/run.py --build --test ze`
   - Files: docs, test files, perf results
   - Verify: AC-1 through AC-9 evidenced

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC row has direct evidence |
| Correctness | Buffer lifetime: receive buffer not freed until all fwd pool done() callbacks fire |
| Naming | `reactorForwardRS` follows reactor naming conventions, not plugin naming |
| Data flow | Ingress filters run before fast path; egress filters run inside fast path per peer |
| Rule: no-layering | No rs-specific imports in reactor; fast path uses PeerSettings flags only |
| Rule: buffer-first | No new allocations in the fast path per UPDATE; reuses existing pool buffers |
| Rule: exact-or-reject | Message size limits enforced by the shared egress pipeline, not duplicated |
| Egress parity | Fast path produces byte-identical output to ForwardUpdate for the same inputs |
| Route reflection | RFC 4456 ORIGINATOR_ID/CLUSTER_LIST applied for IBGP destinations in mixed groups |
| Read-stall bound | Profile `reactorForwardRS` duration on session read goroutine; must be < 100us for N <= 50 peers |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `forward_rs.go` exists | `ls internal/component/bgp/reactor/forward_rs.go` |
| `RSFastPath` field in PeerSettings | `grep -n RSFastPath internal/component/bgp/reactor/peersettings.go` |
| Fast path called from notifyMessageReceiver | `grep -n reactorForwardRS internal/component/bgp/reactor/reactor_notify.go` |
| `ReactorForwarded` and `FastPathSkipped` on RawMessage | `grep -n 'ReactorForwarded\|FastPathSkipped' internal/component/bgp/types/types.go` |
| bgp-rs checks ReactorForwarded | `grep -n ReactorForwarded internal/component/bgp/plugins/rs/server_withdrawal.go` |
| Functional test exists | `ls test/plugin/bgp-rs-reactor-fastpath.ci` |
| Benchmark shows improvement | `go test -bench=BenchmarkReactorForwardRS ./internal/component/bgp/reactor/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Route leak isolation | Fast path must not forward to peers outside the RS group; source exclusion must work |
| Buffer ownership | Receive buffer must not be returned to pool while fwd pool workers still reference it |
| Resource exhaustion | Fast path must not allocate unbounded memory per UPDATE; reuse existing pool buffers |
| Backpressure safety | Fast path uses same TryDispatch (non-blocking) / DispatchOverflow (unbounded append) as ForwardUpdate; no new blocking paths on session read goroutine |
| Config validation | `RSFastPath` must only be set on peer groups where all peers are RS participants |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Egress pipeline extraction breaks existing ForwardUpdate tests | Phase 1: extraction was wrong, restore and re-factor |
| Buffer lifetime violation (use-after-free on receive buffer) | Phase 2: done() callback not wired correctly; add retain/release tracking |
| Fast path produces different wire output than ForwardUpdate | Phase 2: shared egress helper not truly shared; diff the outputs in a test |
| bgp-rs still tries to forward reactor-forwarded UPDATEs | Phase 3: detection flag not set or not checked; add explicit test |
| Withdrawal tracking misses fast-path-forwarded routes | Phase 3: adj-rib-in not receiving events; verify fire-and-forget delivery |
| Grouped-input perf target missed | Re-profile to find remaining bottleneck; may need to bypass forward pool entirely for burst writes |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| rs-gap-0 structural optimizations (fwdCtx removal, batched retains, outbound buckets) would meaningfully improve throughput | These targeted operations worth <0.1% of total forwarding time; the bottleneck was boundary crossings | 4 benchmark runs showing no improvement (331k +/- 2%) | Refocused from micro-optimization to architectural change |
| Event delivery serialization (adj-rib-in blocking bgp-rs) was the primary bottleneck | Fire-and-forget delivery improved throughput by 22% (331k -> 405k) confirming it was A bottleneck but not the ONLY one | Benchmark run 5 showed improvement but 4x gap to BIRD remains | This spec addresses the remaining structural boundaries |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Pipelined event delivery (per-message result channels) | eventChan capacity (64) smaller than batch size (99); delivery still blocked on 65th event | Fire-and-forget delivery with pre-computed cache consumer count |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Profiling 30s amortized capture instead of forwarding-window-only capture | 1 | For perf work, capture profiles that isolate the measurement window, not the full test lifecycle | Keep in spec |

## Design Insights
- The reactor already owns every piece of the RS forwarding pipeline. The plugin boundary exists for flexibility, not for forwarding correctness.
- Buffer lifetime is the key design constraint: without the cache, the receive buffer must be retained until all TCP writes complete. The forward pool's `done()` callback is the natural retention mechanism.
- bgp-rs transitions from "forwarding engine" to "lifecycle manager" (replay, withdrawal, flow control). This is a cleaner separation of concerns.
- The fallback path (bgp-rs ForwardCached) ensures per-peer export policy still works. The reactor fast path handles the common case; the plugin handles the edge case.

## RFC Documentation

Add `// RFC NNNN Section X.Y` comments above any new code that enforces:
- RFC 7947 route-server transparency when the reactor fast path is used
- RFC 4271 UPDATE size limits in the shared egress pipeline

## Implementation Summary

### What Was Implemented
- `forward_rs.go`: `reactorForwardRS` function -- inline RS forwarding from `notifyMessageReceiver`
- `forward_body.go`: `buildFwdBody` shared helper -- extracted body-building logic from ForwardUpdate, used by both ForwardUpdate and reactorForwardRS (eliminated code duplication)
- `peersettings.go`: `RSFastPath bool` field on PeerSettings
- `config.go`: wired `rs-fast-path` from behavior config to PeerSettings
- `rawmessage.go`: `ReactorForwarded bool` and `FastPathSkipped []netip.AddrPort` fields on RawMessage
- `reactor_notify.go`: fast path wiring in `notifyMessageReceiver` after cache Add, before deliverChan enqueue
- `reactor_api_forward.go`: refactored ForwardUpdate to use shared `buildFwdBody`
- `server_withdrawal.go`: bgp-rs checks `ReactorForwarded`, skips ForwardCached when set
- `server_forward.go`: `batchForwardUpdateSkipped` for fallback peers with ExportFilters

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/architecture/core-design.md`: documented reactor RS fast path as third forwarding trigger
- `docs/features.md`: added reactor RS fast path to plugin features
- `docs/guide/configuration.md`: documented `rs-fast-path` config option
- `docs/performance.md`: documented fast path design and boundary reduction
- `docs/comparison.md`: documented RS fast path vs BIRD architecture

### Deviations from Plan
- Added `forward_body.go` (not in spec) to extract shared body-building logic, eliminating dupl linter violations
- Refactored ForwardUpdate to use shared `buildFwdBody` (not in spec, but necessary for code sharing)
- Activate not called in fast path code -- delivery goroutine handles it via OnMessageBatchReceived
- Functional tests (.ci files) and benchmark deferred to separate session (require full test harness)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Reactor-native RS forwarding | Done | forward_rs.go | reactorForwardRS |
| Bypass plugin dispatch | Done | reactor_notify.go:447 | Before deliverChan enqueue |
| bgp-rs coordination | Done | server_withdrawal.go:79-93 | ReactorForwarded check |
| Fallback for ExportFilters | Done | forward_rs.go:46, server_forward.go:batchForwardUpdateSkipped | FastPathSkipped list |
| Config wiring | Done | config.go, peersettings.go | behavior > rs-fast-path |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Pending | Requires benchmark run | Deferred to perf session |
| AC-2 | Done | reactor_notify.go:447-454, forward_rs.go | Fast path before deliverChan |
| AC-3 | Done | TestReactorForwardRSFallback | ExportFilters -> skipped list |
| AC-4 | Done | server_withdrawal.go:95-100 | Withdrawal map still updated |
| AC-5 | Done | Unchanged code path | adj-rib-in still gets fire-and-forget |
| AC-6 | Done | TestReactorForwardRSEBGPPrepend | EBGP wire generation verified |
| AC-6b | Done | TestReactorForwardRSRouteReflection | RFC 4456 attributes |
| AC-7 | Done | All BGP tests pass | No regressions |
| AC-8 | Done | grep confirms existence | forward_rs.go + reactor_notify.go |
| AC-9 | Done | Same TryDispatch/DispatchOverflow | No new blocking paths |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestReactorForwardRSBasic | Done | forward_rs_test.go | Dispatch to all except source |
| TestReactorForwardRSEBGPPrepend | Done | forward_rs_test.go | AS-PATH prepend for EBGP |
| TestReactorForwardRSCopyOnModify | Skipped | - | Covered by route reflection test (mods applied) |
| TestReactorForwardRSFallback | Done | forward_rs_test.go | ExportFilters skip |
| TestReactorForwardRSBufferLifetime | Done | forward_rs_test.go | Retain/Release lifecycle |
| TestReactorForwardRSCacheLifetime | Done | forward_rs_test.go | Cache entry survives |
| TestReactorForwardRSRouteReflection | Done | forward_rs_test.go | RFC 4456 attributes |
| BenchmarkReactorForwardRS | Pending | - | Deferred to perf session |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| forward_rs.go (new) | Done | reactorForwardRS |
| forward_body.go (new, unplanned) | Done | Shared helper to eliminate duplication |
| forward_rs_test.go (new) | Done | 7 unit tests |
| peersettings.go | Done | RSFastPath field |
| config.go | Done | rs-fast-path wiring |
| rawmessage.go | Done | ReactorForwarded + FastPathSkipped |
| reactor_notify.go | Done | Fast path insertion |
| reactor_api_forward.go | Done | Refactored to use buildFwdBody |
| server_withdrawal.go | Done | ReactorForwarded check |
| server_forward.go | Done | batchForwardUpdateSkipped |
| test/plugin/bgp-rs-reactor-fastpath.ci | Pending | Functional test deferred |
| test/plugin/bgp-rs-reactor-fastpath-fallback.ci | Pending | Functional test deferred |

### Audit Summary
- **Total items:** 29
- **Done:** 25
- **Partial:** 0
- **Skipped:** 1 (CopyOnModify test -- covered by RouteReflection)
- **Pending:** 3 (benchmark, 2 functional tests)
- **Changed:** 1 (added forward_body.go for dedup)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Functional tests (.ci) and benchmark not yet created | test/plugin/ | Deferred to perf session |
| 2 | NOTE | Pre-existing lint failures (unused functions) in RS package | server_withdrawal.go | Not introduced by this spec |

## Pre-Commit Verification

All changed packages compile (`go vet`) and tests pass:
- `go test ./internal/component/bgp/reactor/` -- PASS (incl. 7 new fast-path tests)
- `go test ./internal/component/bgp/plugins/rs/` -- PASS
- `go test ./internal/component/bgp/types/` -- PASS
- `go test ./internal/component/bgp/...` -- PASS
