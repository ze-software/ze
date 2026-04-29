# Spec: rs-gap-0-umbrella

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-04-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `ai/rules/planning.md`
3. `docs/architecture/core-design.md`
4. `docs/architecture/update-cache.md`
5. `docs/architecture/forward-congestion-pool.md`
6. `docs/research/bird-bgp-reference.md`
7. `plan/design-rib-rs-fastpath.md`
8. `internal/component/bgp/plugins/rs/server.go`
9. `internal/component/bgp/plugins/rs/server_forward.go`
10. `internal/component/bgp/plugins/rs/server_withdrawal.go`
11. `internal/component/bgp/reactor/reactor_api_forward.go`
12. `internal/component/bgp/reactor/forward_pool.go`

## Task
Close the remaining grouped-input route-server performance gap to BIRD by changing Ze's structure, not by tuning constants. The intended changes are: remove peer-down inventory maintenance from the route-server forwarding critical path, remove the structured-event `sync.Map` hop inside `bgp-rs`, extract a real reactor batch-forward core that uses batched cache retains, and add a reactor-owned outbound attribute-bucket path so grouped routes are sent from a bucketed transmit structure rather than as one independently-forwarded UPDATE at a time.

This spec is about the grouped-input benchmark shape used by `ze-perf` today. The sender already packs many NLRIs into each UPDATE, so the receive-side IPv4 coalescer is not the primary lever here. The remaining gap is structural forwarding cost after grouped UPDATEs have already arrived.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - canonical BGP subsystem design, EventDispatcher, DirectBridge, update groups, two-trigger receive-path model
  → Decision: keep the receive-path `StructuredEvent` trigger as the forwarding entry point
  → Constraint: ingress filters run before caching and before either forwarders or state trackers consume the UPDATE

- [ ] `docs/architecture/update-cache.md` - cache-consumer lifecycle, `ForwardCached`, `ReleaseCached`, activation and ack semantics
  → Decision: preserve the message-id cache contract and immediate-eviction semantics
  → Constraint: any fast path must still respect cache ownership, retain/release, and consumer ack rules

- [ ] `docs/architecture/forward-congestion-pool.md` - forward pool, no-drop rule, copy-on-modify, overflow/backpressure
  → Decision: outbound grouping must live above the existing congestion and ownership model, not bypass it
  → Constraint: silent route drop is forbidden; unchanged peers share source buffers, modified peers copy-on-modify

- [ ] `docs/research/bird-bgp-reference.md` - BIRD's TX bucket system and why it is structurally flatter than Ze's current route-server path
  → Decision: if Ze adopts BIRD-like grouping, the bucket owner must be the reactor TX path, not the plugin
  → Constraint: attrs are encoded once per bucket and prefixes pack until the negotiated message limit is reached

- [ ] `plan/design-rib-rs-fastpath.md` - current two-trigger model and the explicit rejection of retiring the receive-path trigger
  → Decision: do not move forwarding onto `locrib.OnChange`
  → Constraint: forwarders still operate per received UPDATE, state trackers still operate per best-change

- [ ] `plan/learned/630-rs-fastpath-3-passthrough.md` - current `ForwardCached` / `ReleaseCached` fast path and what it intentionally preserved
  → Decision: extend the existing fast path rather than reintroduce text RPC or invent a plugin-specific transport
  → Constraint: egress filters, copy-on-modify, update groups, and zero-modify wire passthrough remain load-bearing

### Rules / Supporting Docs
- [ ] `ai/rules/plugin-design.md` - plugin boundary rules, SDK genericity, DirectBridge expectations
  → Decision: no rs-specific transport or side channel; any engine boundary change must fit the existing generic plugin model
  → Constraint: infrastructure must not import plugin implementations directly, and the fast path must stay on DirectBridge / SDK primitives rather than ad hoc calls

- [ ] `docs/contributing/rfc-implementation-guide.md` - RFC work checklist and documentation expectations
  → Decision: any code that directly enforces RFC 4271 or RFC 7947 constraints gets explicit RFC comments and targeted tests
  → Constraint: performance work cannot quietly change wire semantics without documenting the RFC-preserving behavior it relies on

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - base UPDATE format, IPv4 NLRI layout, message size limits
  → Constraint: outbound regrouping must preserve UPDATE ordering and never exceed the negotiated max message length

- [ ] `rfc/short/rfc7947.md` - route-server forwarding semantics
  → Constraint: performance changes must not turn the RS into a transit rewrite point; AS_PATH non-prepend, NEXT_HOP transparency, MED transparency, and per-client policy remain intact

**Key insights:**
- The default `ze-perf` benchmark already sends grouped multi-NLRI UPDATEs. The sender path in `internal/perf/benchmark.go` auto-packs prefixes close to the 4096-byte limit, so the new receive-side IPv4 coalescer has little left to merge in the grouped-input benchmark.
- Focused 20k tests confirmed this: grouped input (`--batch-size 0`) changed little with coalescing on/off, while one-prefix-per-UPDATE (`--batch-size 1`) changed by roughly 10x. The coalescer works, but it is solving the wrong shape for the default benchmark.
- BIRD is faster here because its route/attr bucket queue is already its transmit engine. Ze is still UPDATE/event driven and pays plugin, cache, worker, and per-destination dispatch overhead around each grouped UPDATE.
- `bgp-rs` currently does per-prefix route-inventory work before forward batching, even though the grouped-input benchmark only measures how fast the grouped UPDATE gets forwarded.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rs/server.go` - `dispatchStructured()` stores `forwardCtx` in `fwdCtx sync.Map`, keyed by message-id, then dispatches a worker item that carries only the id
- [ ] `internal/component/bgp/plugins/rs/server_withdrawal.go` - `processForward()` loads and deletes the `forwardCtx`, extracts families, updates the per-source `withdrawals` map before forward batching, then calls `batchForwardUpdate()`
- [ ] `internal/component/bgp/plugins/rs/server_forward.go` - batches only by identical destination selector; flushes accumulated ids through `Plugin.ForwardCached()`
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `ForwardUpdatesDirect()` parses selectors once, but still forwards one cached UPDATE at a time through the existing per-destination loop; `Retain()` is still called once per peer even though `RetainN()` already exists
- [ ] `internal/component/bgp/reactor/forward_pool.go` - destination workers batch write already-built `fwdItem`s and flush once per batch, but they do not regroup multiple forwarded UPDATEs by post-egress attribute identity
- [ ] `internal/component/bgp/reactor/recent_cache.go` - cache supports `RetainN()` and cache-consumer lifecycle, but the forward loop does not yet exploit batched retain/release in the grouped-input path
- [ ] `internal/perf/benchmark.go` - default benchmark input uses `batch-size <= 0`, which packs as many NLRIs as fit into one UPDATE; the grouped-input benchmark therefore stresses forwarding structure rather than sender-side packet grouping
- [ ] `~/Code/gitlab.nic.cz/labs/bird/proto/bgp/bgp.c` - BIRD keeps a queue of outbound work keyed by shared attrs and sends when the socket is writable
- [ ] `~/Code/gitlab.nic.cz/labs/bird/proto/bgp/packets.c` - `bgp_create_update()` drains withdrawal buckets and reachable-route buckets; `bgp_fire_tx()` sends directly from that bucket queue

**Behavior to preserve:**
- The receive-path `StructuredEvent` trigger remains the forwarder entry point. `locrib.OnChange` remains for state trackers only.
- `ForwardCached()` / `ReleaseCached()` remain the canonical forwarder SDK surface. No regression to the legacy text command path.
- Cache ownership, cache-consumer acks, immediate eviction when fully released, and safety-valve semantics remain unchanged.
- No-drop behavior under backpressure remains unchanged. Slow peers must still buffer or tear down, never silently lose routes.
- Copy-on-modify remains unchanged: unchanged peers share source buffers, modified peers get destination-owned buffers.
- RFC 7947 route-server transparency remains unchanged: no RS AS_PATH prepend, no NEXT_HOP rewrite, MED preserved, per-client export policy still applies.
- Existing fast-path identity, modified-copy, replaying-peer, withdrawal, and backpressure functional tests remain valid.

**Behavior to change:**
- Both the structured (DirectBridge) and text (fork-mode) `bgp-rs` paths will stop round-tripping UPDATE context through `fwdCtx sync.Map`; `workItem` will carry `sourcePeer`, `msg`, and `textPayload` directly, eliminating the `sync.Map` entirely.
- Peer-down route inventory maintenance will move off the forward critical path. Forwarding grouped UPDATEs must not wait on per-prefix string-keyed withdrawal-map maintenance.
- Reactor batch forwarding will stop treating each grouped UPDATE as an isolated send unit once destinations are known. The batch core will amortize selector, retain, and post-egress grouping work across ids.
- Unchanged grouped routes with identical post-egress attribute identity will be eligible for reactor-owned outbound buckets, so Ze can send from a bucketed TX structure closer to BIRD's model.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Wire bytes enter on the source peer's TCP session as BGP UPDATEs.
- For this benchmark shape, each UPDATE already contains many IPv4 unicast NLRIs sharing one attr set.

### Transformation Path
1. Session read path parses the BGP header and UPDATE body into a `WireUpdate`.
2. Reactor caches the received UPDATE in `RecentUpdateCache` and emits a `StructuredEvent` to internal plugins.
3. `bgp-rs` structured dispatch receives `*RawMessage` plus source peer address.
4. Current design stores `{sourcePeer, msg}` in `fwdCtx`, keyed by `msgID`, then dispatches a worker item carrying only the `msgID`. **Target design:** `workItem` carries `sourcePeer`, `msg`, and `textPayload` directly; `fwdCtx` is eliminated.
5. Worker reads context from the `workItem`, extracts families, extracts compact NLRI summary (extract-then-forward), forwards via batch, then passes NLRI summary to the inventory path off the critical path.
6. Batch flush calls `Plugin.ForwardCached(ids, destinations)` back into the reactor through DirectBridge.
7. Reactor resolves ids from cache, resolves destinations, applies per-peer egress decisions, then dispatches `fwdItem`s to per-destination forward workers.
8. Forward workers write raw bodies or rebuilt updates to each destination TCP socket and flush once per batch.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Reactor | Session parse -> `WireUpdate` -> `RecentUpdateCache` | [ ] |
| Reactor -> Plugin | DirectBridge structured event delivery (`StructuredEvent` carrying `*RawMessage`) | [ ] |
| Plugin worker -> Reactor | DirectBridge typed `ForwardCached()` / `ReleaseCached()` call | [ ] |
| Reactor cache -> Forward workers | cache lookup by message-id plus retain/release lifecycle | [ ] |
| Reactor -> TCP writer | per-destination forward worker batches raw bodies / rebuilt updates to `bufio.Writer` | [ ] |

### Integration Points
- `bgp-rs` worker pool stays the source-ordering boundary, but structured work items can carry context directly.
- `RecentUpdateCache` remains the owner of received source buffers. Any new batch core or bucket path must retain and release through the cache contract, not around it.
- `updateGroups` remain the capability-sharing abstraction for destination peers. Outbound buckets must sit above or within this grouping, not duplicate it elsewhere.
- Egress filters and copy-on-modify stay in `reactor_api_forward.go` / forward-build path. Outbound buckets can only group updates after the post-egress attr identity is known.
- Peer-down withdrawal replay remains a `bgp-rs` responsibility unless this spec explicitly rehomes it; the new design must still produce the same observable withdrawals when a source peer drops.

### Architectural Verification
- [ ] No bypassed layers, the receive-path trigger, cache, and forward pool remain the authoritative path
- [ ] No unintended coupling, plugin boundary stays DirectBridge / SDK based
- [ ] No duplicated functionality, outbound buckets extend the forward path instead of creating a second route server
- [ ] Zero-copy preserved where applicable, unchanged grouped routes still reuse source buffers until a copy is actually required

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Source peer sends grouped IPv4 UPDATE to RS | -> | `dispatchStructured()` -> structured work item path -> reactor batch core -> destination write | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` plus new `test/plugin/bgp-rs-grouped-bucket.ci` |
| Export policy modifies one destination only | -> | egress filter path -> copy-on-modify bypasses outbound bucket sharing for that peer | `test/plugin/bgp-rs-mod-copy.ci` |
| Source peer goes down after previously announced routes | -> | peer-down route inventory -> withdrawal send path | `test/plugin/rs-ipv4-withdrawal.ci` |
| Peer-up replay runs while forwarding continues | -> | replay/EOR path remains correct with new inventory/forward structure | `test/plugin/bgp-rs-replaying-gate.ci` |
| Slow destination peer causes queue buildup | -> | forward pool overflow/backpressure still protects no-drop invariant | `test/plugin/rs-backpressure.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `python3 test/perf/run.py --build --test ze` on the grouped-input 100k IPv4 benchmark after this spec lands | Ze reaches at least 500k routes/sec average throughput and at most 220 ms convergence on the reference M4/Colima harness, with 0 lost routes |
| AC-2 | Both structured and text `bgp-rs` UPDATE paths under test inspection | UPDATE work items no longer round-trip through `fwdCtx sync.Map` on either path; `workItem` carries `sourcePeer`, `msg`, and `textPayload` directly; `grep -rn fwdCtx internal/component/bgp/plugins/rs/` returns zero hits |
| AC-3 | Grouped UPDATE arrives on the structured path | Forward batching is not blocked on per-prefix string-keyed withdrawal-map maintenance in the same worker critical path |
| AC-4 | `ForwardUpdatesDirect` / batch core handles multiple ids for the same destination set | Reactor resolves destinations once per batch, uses `RetainN()` for the batch, and preserves cache ownership semantics |
| AC-5 | Multiple grouped UPDATEs with identical post-egress attrs target the same destination or update-group | The reactor emits outbound packets from a shared attr bucket, packing as many NLRIs as fit within negotiated message size |
| AC-6 | One destination requires egress modification while another does not | Modified peer takes the copy-on-modify path; unmodified peer still shares source bytes; no attr leakage across peers |
| AC-7 | Source peer teardown after forwarded announcements | The peer-down path still emits correct withdrawals for all previously-announced routes |
| AC-8 | Existing fast-path RS identity tests run after the refactor | RFC 7947 semantics remain unchanged: no RS AS_PATH prepend, NEXT_HOP transparency preserved, MED preserved, per-client policy still applies |
| AC-9 | Slow destination peer fills peer channel / overflow pool | No route drops occur; existing congestion and teardown behavior remains intact |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRSDispatchStructuredCarriesContextDirect` | `internal/component/bgp/plugins/rs/server_test.go` | Structured work items carry source peer plus raw message directly, no `fwdCtx` map lookup | |
| `TestRSDispatchTextCarriesContextDirect` | `internal/component/bgp/plugins/rs/server_test.go` | Text work items carry source peer plus text payload directly, no `fwdCtx` map lookup | |
| `TestRSInventoryWorkerKeepsPeerDownState` | `internal/component/bgp/plugins/rs/server_test.go` | Route inventory moved off the forward critical path still preserves peer-down withdrawal state | |
| `TestForwardUpdatesDirectRetainN` | `internal/component/bgp/reactor/forward_update_test.go` | Batch core uses one batched retain for a batch rather than one retain per peer per id | |
| `TestForwardBucketGroupsIdenticalAttrs` | `internal/component/bgp/reactor/forward_bucket_test.go` | Outbound bucket groups grouped UPDATEs sharing post-egress attr identity | |
| `TestForwardBucketFlushesAtMessageLimit` | `internal/component/bgp/reactor/forward_bucket_test.go` | Bucket drain respects 4096-byte or extended-message limit | |
| `TestForwardBucketBypassesModifiedPeer` | `internal/component/bgp/reactor/forward_bucket_test.go` | Modified peers do not incorrectly share a bucket with unchanged peers | |
| `BenchmarkRSForwardGroupedInput` | `internal/component/bgp/plugins/rs/server_bench_test.go` | Grouped-input forward throughput baseline and improvement | |
| `BenchmarkForwardBucketDrain` | `internal/component/bgp/reactor/forward_bucket_bench_test.go` | Cost of bucket drain versus current per-item write path | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Standard UPDATE body length | 0 to 4077 bytes | 4077 | N/A | 4078 |
| Extended UPDATE body length | 0 to 65516 bytes | 65516 | N/A | 65517 |
| Batch retain count | 1 to `len(batch.ids)` | `len(batch.ids)` | 0 | N/A |
| Bucketed IPv4 `/24` prefixes in one standard UPDATE | computed from attr overhead and remaining body | computed max | max - 1 remains same bucket | max + 1 forces split |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-fastpath-ebgp-shared` | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | RS still forwards unchanged shared UPDATEs correctly through the fast path | |
| `bgp-rs-grouped-bucket` | `test/plugin/bgp-rs-grouped-bucket.ci` | Multiple grouped UPDATEs with identical post-egress attrs leave Ze as fewer bucketed outbound UPDATEs | |
| `bgp-rs-mod-copy` | `test/plugin/bgp-rs-mod-copy.ci` | Modified export path still copies only the modified peer | |
| `rs-ipv4-withdrawal` | `test/plugin/rs-ipv4-withdrawal.ci` | Peer-down still withdraws every route that had been announced from that source | |
| `bgp-rs-replaying-gate` | `test/plugin/bgp-rs-replaying-gate.ci` | Replay and EOR semantics survive the structural refactor | |
| `rs-backpressure` | `test/plugin/rs-backpressure.ci` | Slow-peer congestion behavior remains unchanged | |

### Future (if deferring any tests)
- None. The grouped-bucket functional test is mandatory because this spec is about end-to-end grouped-input behavior, not just micro-benchmarks.

## Files to Modify
- `internal/component/bgp/plugins/rs/server.go` - remove `fwdCtx sync.Map` entirely; update both `dispatchStructured` and `dispatchText` to populate `workItem` fields directly; wire separate inventory path
- `internal/component/bgp/plugins/rs/worker.go` - extend `workItem` struct with `sourcePeer string`, `msg *bgptypes.RawMessage`, `textPayload string`; update `onItemDrop` signature (no more `fwdCtx.Delete`)
- `internal/component/bgp/plugins/rs/server_forward.go` - adapt batching to new worker item shape; keep destination-set batching while delegating bucketing to the reactor
- `internal/component/bgp/plugins/rs/server_withdrawal.go` - split peer-down route inventory maintenance from forward critical path; `processForward` reads from `workItem` instead of `fwdCtx`; extract-then-forward design for buffer lifetime safety
- `internal/component/bgp/plugins/rs/server_handlers.go` - coordinate `handleStateDown` with the new inventory path; ensure inventory worker is drained before withdrawal entries are extracted
- `internal/component/bgp/reactor/reactor_api_forward.go` - extract a real batch-forward core, use `RetainN()`, and feed outbound bucket grouping
- `internal/component/bgp/reactor/forward_pool.go` - integrate outbound bucket drain with existing write/flush and congestion model
- `internal/component/bgp/reactor/recent_cache.go` - only if additional batch-release helpers are needed to support the new forward core cleanly
- `docs/architecture/core-design.md` - record the new RS forward structure and bucketed TX path
- `docs/architecture/update-cache.md` - document any batch-retain / batch-forward lifecycle changes
- `docs/architecture/forward-congestion-pool.md` - document how outbound buckets sit on top of the existing no-drop pool model
- `docs/comparison.md` - update the route-server comparison note if the structural difference section changes materially

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | `internal/yang/modules/*.yang` |
| CLI commands/flags | [ ] | `cmd/ze/*/main.go` or subcommand files |
| Editor autocomplete | [ ] | YANG-driven |
| Functional test for new RPC/API | [ ] | `test/plugin/bgp-rs-grouped-bucket.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfc4271.md`, `rfc/short/rfc7947.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md`, `docs/performance.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md`, `docs/architecture/update-cache.md`, `docs/architecture/forward-congestion-pool.md` |

## Files to Create
- `internal/component/bgp/plugins/rs/server_inventory.go` - dedicated peer-down route inventory path, separate from the forward critical path
- `internal/component/bgp/reactor/forward_bucket.go` - outbound attribute-bucket model and drain logic
- `internal/component/bgp/reactor/forward_bucket_test.go` - unit coverage for grouping, split, and modified-peer bypass
- `internal/component/bgp/plugins/rs/server_bench_test.go` - grouped-input RS hot-path benchmark
- `internal/component/bgp/reactor/forward_bucket_bench_test.go` - bucket drain micro-benchmark
- `test/plugin/bgp-rs-grouped-bucket.ci` - end-to-end grouped-input bucket wiring proof

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | Functional tests + targeted perf run + `make ze-verify` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Relevant implementation phase |
| 8. Re-verify | Stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Stage 5 |
| 13. Present summary | Executive Summary Report per `ai/rules/planning.md` |

### Implementation Phases
1. **Phase: remove fwdCtx indirection from both dispatch paths** - replace `fwdCtx sync.Map` with direct work items carrying forward context for both the structured (DirectBridge) and text (fork-mode) paths; `workItem` gains `sourcePeer string`, `msg *bgptypes.RawMessage`, and `textPayload string` fields so `processForward` reads from the item, never from a side map
   - Tests: `TestRSDispatchStructuredCarriesContextDirect`, `TestRSDispatchTextCarriesContextDirect`, `BenchmarkRSForwardGroupedInput`
   - Files: `server.go` (remove `fwdCtx` field, update both `dispatchStructured` and `dispatchText`), `server_forward.go`, `server_withdrawal.go` (update `processForward` signature to accept `workItem`), `worker.go` (extend `workItem` struct)
   - Verify: `grep -rn fwdCtx internal/component/bgp/plugins/rs/` returns zero hits; existing functional fast-path tests still pass
   - Design: both `dispatchStructured` and `dispatchText` populate `workItem` fields directly. The `onItemDrop` callback no longer needs `fwdCtx.Delete`. The text path is the legacy fallback (fork-mode, rarely used in production), but it shares the same `processForward` entry point, so eliminating `fwdCtx` for both paths is a small incremental cost that removes the entire `sync.Map`

2. **Phase: move peer-down route inventory off the forward critical path** - maintain peer-down withdrawal state through a dedicated inventory path that does not delay forward batching; preserve peer-down semantics and replay interactions
   - Tests: `TestRSInventoryWorkerKeepsPeerDownState`, `rs-ipv4-withdrawal`, `bgp-rs-replaying-gate`
   - Files: `server_inventory.go`, `server_withdrawal.go`, `server_handlers.go`
   - Verify: peer-down emits the same withdrawals as today; grouped forward latency no longer includes string-heavy inventory maintenance
   - **Design decision (buffer lifetime):** Today `processForward` updates the withdrawal map BEFORE calling `batchForwardUpdate` because forwarding can trigger cache eviction (`ForwardUpdate -> Ack -> evictLocked`) which frees the pool buffer backing `ctx.msg.WireUpdate` (see `server_withdrawal.go:77-80`). Reading `WireUpdate` after forward would be use-after-free. To decouple inventory from forwarding, Phase 2 uses **extract-then-forward**: before calling `batchForwardUpdate`, extract families (already done today) and a compact NLRI summary (`[]netip.Prefix`, 16 bytes each, from a pooled slice) using `netip.PrefixFrom` (fast, no string allocation). The summary is passed to the inventory path as a value. The `WireUpdate` is not accessed after `batchForwardUpdate` is called. String keys (`Masked().String()`) are produced only in the inventory path, off the forward critical path. This moves the per-prefix cost (200x string alloc + map insert for a typical grouped UPDATE) out of the forwarding latency while preserving the use-after-free safety by not touching `WireUpdate` after the forward call

3. **Phase: extract reactor batch-forward core and batched cache retains** - turn `ForwardUpdatesDirect()` into a real batch engine rather than a thin wrapper over one-id-at-a-time forwarding; use `RetainN()` and batch-level destination resolution
   - Tests: `TestForwardUpdatesDirectRetainN`, existing `forward_update_test.go` coverage, `bgp-rs-fastpath-ebgp-shared`
   - Files: `reactor_api_forward.go`, optionally `recent_cache.go`
   - Verify: selector parsed once per batch, retain batched, no legacy text forward path reappears

4. **Phase: add reactor-owned outbound attribute buckets** - group unchanged grouped UPDATEs by post-egress attr identity per destination or update-group, drain buckets to TCP within negotiated message-size limits, bypass for modified peers
   - Tests: `TestForwardBucketGroupsIdenticalAttrs`, `TestForwardBucketFlushesAtMessageLimit`, `TestForwardBucketBypassesModifiedPeer`, `bgp-rs-grouped-bucket.ci`, `bgp-rs-mod-copy.ci`
   - Files: new `forward_bucket.go`, `reactor_api_forward.go`, `forward_pool.go`
   - Verify: grouped-input benchmark improves materially and egress correctness is unchanged

5. **Phase: verification and docs** - regenerate grouped-input benchmark evidence, update architecture docs, comparison notes, and performance narrative
   - Tests: `python3 test/perf/run.py --build --test ze`, `make ze-verify`
   - Files: docs listed above, perf result files as evidence if intentionally refreshed
   - Verify: AC-1 through AC-9 evidenced directly

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC row has direct evidence, including the grouped-input perf target |
| Correctness | Peer-down withdrawals remain correct even though inventory moved off the forward critical path |
| Naming | New reactor bucket and inventory types use existing Ze vocabulary, not new transport or RIB terms that imply the wrong ownership |
| Data flow | Receive-path trigger remains load-bearing, cache ownership remains authoritative, and outbound buckets sit above egress decision logic rather than beside it |
| Rule: no-layering | No new rs-specific transport or side channel is introduced; existing DirectBridge / `ForwardCached` path is extended, not paralleled |
| Rule: exact-or-reject | Bucket packing splits at exact negotiated message-size boundaries, never silently truncates |
| Rule: buffer-first | Grouping preserves zero-copy for unchanged paths and only allocates when copy-on-modify or bucket materialization truly requires it |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `server_inventory.go` exists and owns peer-down route inventory work | `ls internal/component/bgp/plugins/rs/server_inventory.go` |
| Neither structured nor text path uses `fwdCtx` | `grep -rn "fwdCtx" internal/component/bgp/plugins/rs/` returns zero hits (excluding test files asserting its absence) |
| Reactor batch core uses `RetainN()` | `grep -n "RetainN" internal/component/bgp/reactor/reactor_api_forward.go` |
| Outbound bucket implementation exists | `ls internal/component/bgp/reactor/forward_bucket.go` |
| Grouped bucket functional test exists | `ls test/plugin/bgp-rs-grouped-bucket.ci` |
| Grouped-input perf target evidenced | `bin/ze-perf report test/perf/results/bird.json test/perf/results/ze.json` plus archived ze result JSON |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Route leak isolation | No outbound bucket may mix routes whose post-egress attr identity differs for a destination peer |
| Buffer ownership | Bucketing and direct work-item carry must not outlive source-buffer ownership or bypass cache retain/release |
| Resource exhaustion | Inventory and bucket structures must stay bounded by actual received routes and negotiated message sizes, not unbounded string growth |
| Backpressure safety | New batching cannot bypass slow-peer protection or hold unbounded buffers outside the forward pool budget |
| Replay / teardown correctness | Moving inventory work off the forward critical path must not create a race where peer-down misses withdrawals or replay/EOR state |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `workItem` fields insufficient for some edge case (e.g. extremely large text payloads) | Profile the edge case; if `workItem` copy cost exceeds `sync.Map` store/load, add a pooled allocation for text payloads only |
| Inventory move breaks peer-down withdrawals | Return to Phase 2 and make ordering explicit before forwarding or teardown |
| Bucket path regresses modified-peer correctness | Return to Phase 4 and tighten attr-identity boundary so modified peers never share |
| Grouped-input perf target missed but correctness holds | Re-profile and decide whether the remaining cost is inside bucket drain, forward pool flush, or cache lifecycle before widening scope |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The new IPv4 receive-side coalescer should improve the default bird-vs-ze benchmark | The default perf harness already sends grouped multi-NLRI UPDATEs, so grouped-input forwarding, not receive-side coalescing, dominates | Focused `ze-perf` runs with `--batch-size 0` vs `--batch-size 1` and coalescing on/off | Refocused the spec from RX coalescing to forwarding structure |
| BIRD was probably winning because it waits on a timer before aggregating | In the BIRD path examined here, the socket-writable loop drains a bucket queue immediately; the key difference is bucket ownership, not a timer delay | Reading `proto/bgp/bgp.c` and `proto/bgp/packets.c` in the local BIRD tree | Spec targets bucket-owned TX structure rather than timer tuning |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Retire the receive-path forward trigger and move forwarding onto Loc-RIB change events | Already rejected by `plan/design-rib-rs-fastpath.md`; it breaks the ingress filter pipeline and loses duplicate-perceived UPDATE behavior | Keep `StructuredEvent` trigger and optimize the hot path behind it |
| Chase only micro-optimizations such as timer reset or constants | Useful but not the structural reason grouped-input Ze still trails BIRD | Prioritize inventory removal, batch-forward core, and outbound buckets |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Assuming benchmark shape from the optimization name instead of reading `internal/perf/benchmark.go` | 1 | For perf work, always inspect benchmark input shape before attributing wins or losses | keep in spec and learned summary |

## Design Insights
- The current grouped-input benchmark stresses Ze after packet grouping, not before it.
- `RetainN()` already exists, which makes per-peer one-by-one retain calls an avoidable design lag rather than a missing primitive.
- The right place for BIRD-like grouping in Ze is reactor-owned outbound buckets, because the plugin does not own post-egress attr identity or buffer lifetime.
- The receive-path trigger remains load-bearing because it is where ingress filtering happens before caching.
- The current `withdrawals` map does useful peer-down work, but doing it inline in `processForward()` makes grouped UPDATE forwarding pay per-prefix bookkeeping cost before any bytes leave the box.

## RFC Documentation

Add `// RFC NNNN Section X.Y` comments above any new code that enforces:
- RFC 4271 UPDATE size and packing limits
- RFC 7947 route-server transparency constraints when attr buckets are introduced

## Implementation Summary

### What Was Implemented
- Phase 1: Removed `fwdCtx sync.Map` indirection from both structured and text dispatch paths. `workItem` now carries `sourcePeer`, `msg`, and `textPayload` directly.
- Phase 2: Created `server_inventory.go` with extract-then-forward design. NLRI records extracted as `netip.Prefix` values (zero string allocation) before forwarding. Withdrawal map updated after forwarding with string keys produced off the forward critical path.
- Phase 3: Replaced per-peer `Retain()` calls in `ForwardUpdate` with a single `RetainN(id, peerCount)` per id using a pending dispatch buffer.
- Phase 4: Created `forward_bucket.go` with outbound attribute bucket grouping. `fwdBatchHandler` merges queued items with identical path attributes into fewer outbound UPDATEs before TCP writes. Respects negotiated message-size limits. Items with copy-on-modify or parsed-update path bypass bucketing.

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/architecture/core-design.md`: Added rs-gap-0 sections documenting batched retains, outbound buckets, and direct dispatch

### Deviations from Plan
- Phase 4 bucket merge operates at the batch handler level (fwdBatchHandler) rather than between ForwardUpdate and dispatch. This is simpler and avoids duplicating the complex egress filter chain.

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

## Pre-Commit Verification

Not started. This section is completed only after implementation.
