# Spec: EventBus backpressure symmetry, async fan-out, and ordering (DESIGN-REVIEW finding 4)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-unify-buffer-lifetime (HARD ordering: async fan-out must not land first) (closed, learned 1077 -- ordering constraint satisfied) |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. ~~`DESIGN-REVIEW.md` finding 4 ("The bus") and its verification notes~~ (2026-07-22: that file was an ephemeral session artifact, never committed -- it exists nowhere in the repo or git history. The finding is restated in full in this spec's Task section; nothing is lost)
4. `internal/component/plugin/process/delivery.go`, `pkg/plugin/rpc/mux.go`,
   `internal/component/plugin/server/engine_event.go`,
   `internal/component/plugin/server/dispatch.go`,
   `internal/component/bgp/redistribute/consumer.go`

## Task

Close DESIGN-REVIEW finding 4. The event mechanism itself is sound (one typed in-process
pub/sub in `internal/core/events/typed.go`, with JSON copies for external plugins). The
problems are around it: the delivery policy is asymmetric, the fan-out is synchronous and
couples producer latency to the slowest consumer, delivery ordering between engine and
external subscribers is an emergent `defer`, and the docs/naming for "bus" are stale and
overloaded.

Four verified defects in scope:

1. **Asymmetric backpressure.** Engine-to-plugin delivery does a blocking send on a 64-slot
   channel, so a slow plugin stalls the emitter (`process/delivery.go`, comment:
   "backpressure propagates naturally"). Plugin-to-engine requests are dropped after a 1-second
   timeout with only a log line (`pkg/plugin/rpc/mux.go`, `sendRequest`). Two opposite
   policies on one logical channel.
2. **Synchronous fan-out couples producer latency to the slowest consumer.** Engine
   subscribers run inline on the emitting goroutine (`server/engine_event.go`,
   dispatched via `defer` in `server/dispatch.go`). Combined with the redistribution
   consumer doing one blocking `UpdateRoute` RPC per entry with a 10-second timeout
   (`redistribute/consumer.go,39-55`), a producer emitting a 64-entry batch can block in
   `Emit` for N sequential round trips. The batch pool amortizes allocation
   (`core/redistevents/pool.go`), but allocation was never the bottleneck; the serialized
   synchronous I/O downstream is.
3. **Ordering quirk.** External plugin subscribers are delivered before in-process engine
   subscribers because engine dispatch is deferred (`server/dispatch.go`, comment: "engine
   handlers run AFTER plugin process delivery"). Correctness reasoning that assumes synchronous
   in-process re-entrancy holds only for the in-process deployment shape.
4. **Overloaded naming and stale docs.** "Bus" and "hub" each name several unrelated things
   (`ze.EventBus`, the RPC delivery pipeline, `pluginserver.Hub`, `hub.Orchestrator`,
   `cmd/ze/hub`, plus the separate Operational Report Bus). `docs/architecture/core-design.md`
   section 1 still shows a standalone "Bus (notification pub/sub)" component
   (`core-design.md,72`) that `plan/learned/DESIGN-HISTORY.md` records as absorbed into
   the plugin server ("Plugin system: architecture" -> "Abandoned approaches", the standalone
   Bus entry: "`ze.EventBus` is now backed by `Server.Emit`").

Goal: (1) one documented backpressure policy on both directions with a drop/overflow metric,
no silent loss; (2) decouple `Emit` latency from slow consumers by draining the batch off the
emit path; (3) make engine-vs-external delivery ordering explicit and documented rather than an
emergent `defer`; (4) fix the stale core-design.md diagram and add a naming glossary.

**Explicitly out of scope (referenced, not duplicated):**
- The route-change event bridge lossiness: owned by `spec-unify-route-events.md` (which rides
  this bus but explicitly leaves the fan-out concern to this spec).
- Replay-request vocabulary unification: owned by `spec-unify-replay.md` (same note).
- The Operational Report Bus and its harness shutdown hang: owned by
  `spec-finish-report-bus.md` (a different "bus"; watch the naming collision).
- A repo-wide file-level rename of bus/hub symbols: deferred (rename is a known minefield per
  the review; this spec adds a glossary and fixes the diagram, not a mass rename).

### Post-wave corrections (2026-07-10)

**Decision (2026-07-10, user): reconcile.** The backpressure redesign must reconcile THREE
existing policies on the plugin-RPC path, not the two the Task's defect 1 describes. The
third landed in the 2026-07 implementation wave. All three producers verified in current
code:

| # | Policy | Producer | Citation |
|---|--------|----------|----------|
| 1 | Blocking send, engine to plugin | `Process.Deliver` blocks on the 64-cap `eventChan` (select with only a ctx-cancel escape, no timeout) | `internal/component/plugin/process/delivery.go`; capacity `eventDeliveryCapacity = 64` at :60 |
| 2 | Timed drop, plugin to engine | `MuxConn.sendRequest` waits 1s for `requestCh` then drops; readLoop logs "request channel full, dropping inbound request" | `pkg/plugin/rpc/mux.go`; drop log at :258-261 |
| 3 | Fail-fast close on stalled write (NEW, 2026-07 wave) | `writeAppended` applies a default 30s write deadline on deadline-capable transports and arms a reusable watchdog timer otherwise; on a stall `fireWatchdog` logs, fires the metric hook, and closes both ends | `pkg/plugin/rpc/conn.go` (window), :286-334 (write path, transport selection at :307-315), :191-200 (`fireWatchdog`), :139 (`SetWriteWatchdogHook`); counter `ze_plugin_write_watchdog_total` wired in `internal/component/plugin/server/server.go` |

The unified policy AC-1 demands must explicitly cover or supersede the watchdog: either the
watchdog becomes the documented stall backstop under the unified policy (with its metric
folded into the drop/overflow metric family), or the unified policy replaces it, but it may
not be left as an undocumented fourth behavior. AC-1's original wording predates the
watchdog and is struck and superseded in the Acceptance Criteria table below.

Metrics-doc merge note: `docs/plugin-development/metrics.md` now documents
`ze_plugin_write_watchdog_total`. The drop/overflow metrics this spec adds (Integration
Checklist "Prometheus counters" row; Documentation Update Checklist row 14) must merge into
that existing metrics table and its watchdog prose, not open a parallel section.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/core-design.md` sections 1 and 15 - the stale standalone-Bus diagram
  vs the real EventBus and the separate Operational Report Bus
  → Decision: the standalone Bus was absorbed into the plugin server; `ze.EventBus` is backed by `Server.Emit`.
  → Constraint: any diagram/prose change must keep the Operational Report Bus (§15) distinct from the EventBus.
- [ ] `docs/architecture/api/process-protocol.md` - plugin event delivery and RPC framing
  → Constraint: external-plugin delivery is JSON over the process protocol; backpressure changes must not break the wire framing.
- [ ] `plan/learned/DESIGN-HISTORY.md` (bus-absorption entries) - why the standalone bus was removed
  → Constraint: do not reintroduce a standalone bus component; work within `Server.Emit`.

**Key insights:** the mechanism is good; the fixes are policy (backpressure), scheduling
(async drain), determinism (ordering), and documentation (naming/diagram), not a rewrite.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/plugin/process/delivery.go` - `Deliver` blocks on a 64-cap
  `eventChan` (`eventDeliveryCapacity = 64`, lines 56-83).
- [ ] `pkg/plugin/rpc/mux.go` - `sendRequest` non-blocking-ish send that drops after
  `time.After(time.Second)` and logs "dropping inbound request" (lines 258-276).
- [ ] `internal/component/plugin/server/engine_event.go` - `dispatch` snapshots handlers and
  runs them inline in the caller's goroutine (lines 95-112).
- [ ] `internal/component/plugin/server/dispatch.go` - engine dispatch is `defer`red so it
  runs after external plugin `Deliver` (lines 428-472, defer at 441).
- [ ] `internal/component/bgp/redistribute/consumer.go` - `InjectRoute` does one blocking
  `UpdateRoute` RPC per entry with `updateRouteTimeout = 10 * time.Second` (lines 18,39-55).
- [ ] `internal/core/redistevents/pool.go` - batch `sync.Pool` amortizes allocation only
  (lines 5-13).
- [ ] `docs/architecture/core-design.md` - standalone Bus in diagram (line 28) and prose
  (line 72).

**Behavior to preserve:** (unless user explicitly said to change)
- Typed pub/sub semantics: `events.Register[T]` / `Emit` / `Subscribe` API unchanged for callers.
- External plugins keep receiving JSON copies over the process protocol; wire framing unchanged.
- No event is lost that is not lost today, and none is double-delivered.
- The Operational Report Bus (§15) stays a separate mechanism.
- Existing subscribers that today observe events (rib, sysrib, flowexport, redistribute) keep
  observing the same events with the same payloads.

**Behavior to change:** (only if user explicitly requested)
- Backpressure policy becomes symmetric and explicit, with a metric on drops/overflow (no
  silent plugin-to-engine drop).
- `Emit` no longer blocks for the duration of a slow consumer's N sequential RPCs.
- Engine-vs-external delivery ordering becomes an explicit, documented contract.
- core-design.md no longer shows a standalone Bus; a naming glossary is added.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A producer calls `Emit` on `ze.EventBus` (backed by `Server.Emit`) with a typed payload,
  e.g. a redistribution `RouteChangeBatch` of up to 64 entries.
- Format at entry: a typed Go payload in-process; JSON is produced lazily only if an external
  subscriber exists.

### Transformation Path
1. `Server.dispatch` computes namespace/eventType, optionally decodes the typed payload for
   engine subscribers (`server/dispatch.go`).
2. It `defer`s `dispatchEngineEvent` so engine handlers run after external delivery
   (`dispatch.go`).
3. It marshals JSON once and calls `p.Deliver` for each matching external process
   (`dispatch.go`); `Deliver` blocks if that process's 64-slot channel is full
   (`process/delivery.go`).
4. After the function body, the deferred `dispatchEngineEvent` runs engine handlers inline on
   the emitting goroutine (`engine_event.go`).
5. One engine handler is the redistribution consumer, which issues N blocking `UpdateRoute`
   RPCs, each up to 10s (`redistribute/consumer.go`), so `Emit` can block for N round
   trips before returning to the producer.
6. In the reverse direction, a plugin-initiated request enters `mux.readLoop` and is offered to
   `requestCh`; if full it is dropped after 1s (`mux.go`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Emitter ↔ external plugin | JSON over process protocol; blocking 64-slot channel | [ ] |
| Emitter ↔ engine subscriber | inline call on emit goroutine via `defer` | [ ] |
| Plugin ↔ engine (reverse) | `requestCh` send, dropped after 1s | [ ] |
| Consumer ↔ BGP reactor | one `UpdateRoute` RPC per entry, 10s timeout | [ ] |

### Integration Points
- `Server.Emit` / `dispatch` - the single fan-out point to change scheduling and ordering.
- `Process.Deliver` and `eventDeliveryCapacity` - the engine-to-plugin backpressure seam.
- `MuxConn.sendRequest` - the plugin-to-engine drop seam.
- `redistribute.BGPConsumer.InjectRoute` - the slow consumer that must move off the emit path.
- Telemetry registry - for the new drop/overflow/latency metrics.

### Architectural Verification
- [ ] No bypassed layers (delivery still flows through `Server.Emit`; no standalone bus reborn)
- [ ] No unintended coupling (async drain does not leak payload lifetime; see `spec-unify-buffer-lifetime.md`)
- [ ] No duplicated functionality (one backpressure policy, not a third channel)
- [ ] Zero-copy preserved where applicable (async drain must respect batch pool release rules)
- [ ] Registration over hardcoding — new metrics register via the telemetry registry; no
  per-consumer scheduling special-case added to a core package (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Draining the redistribution batch asynchronously does not violate the batch pool lifetime contract | `core/redistevents/pool.go` recycles entries on `ReleaseBatch`; see `spec-unify-buffer-lifetime.md` | Async drain reads recycled/zeroed data | Snapshot/copy before handing to the async drainer; race test with `-race` | unvalidated |
| A-2 | The replay coordinator's correctness does not actually depend on external-before-engine ordering | `dispatch.go` defer; DESIGN-REVIEW section 4 ordering note | Making ordering explicit changes replay behavior | Trace the replay coordinator; targeted ordering `.ci` | unvalidated |
| A-3 | A bounded overflow policy (drop-oldest or block-with-timeout) is acceptable for both directions | `delivery.go`/`mux.go` current opposite policies | Chosen policy loses events some subscriber needs | Per-direction metric + burst `.ci`; confirm with user which policy | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Async fan-out introduces event reordering a subscriber relies on | rib/sysrib `.ci` diff | Preserve per-subscriber ordering; only decouple across subscribers |
| R-2 | Payload lifetime bug when draining off the emit goroutine | `-race` failures, zeroed fields | Copy/snapshot at hand-off; coordinate with buffer-lifetime spec |
| R-3 | Metric/policy change masks a real slow-plugin problem | drops climb silently | Alert-worthy metric + log at first drop, not per drop |
| R-4 | Async fan-out lands before `spec-unify-buffer-lifetime` enforces pool/handle/RawBytes lifetime, silently corrupting retained batches | `-race` failures, zeroed/wrong-route fields only under async delivery | HARD gate: block Phase 3 until buffer-lifetime enforcement exists; snapshot pooled payloads at hand-off |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| burst producer emits a 64-entry batch while a consumer is slow | → | `Emit` returns in bounded time (drain is async) | `test/plugin/bus-slow-consumer-nonblocking.ci` |
| plugin sends more inbound requests than `requestCh` holds | → | overflow counted via metric, not silently dropped | `test/plugin/bus-inbound-overflow-metric.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Slow external plugin (channel would fill) and slow inbound path | ~~One documented backpressure policy applies to both directions; every drop/overflow increments a metric; no silent drop~~ (struck 2026-07-10: wording predates the write watchdog, a third policy on the same path; see Post-wave corrections) One documented backpressure policy covers all three mechanisms on the plugin-RPC path (blocking engine-to-plugin send, 1s inbound drop, write-watchdog / write-deadline fail-fast close), each either subsumed by the unified policy or explicitly documented as superseded by it; every drop, overflow, or watchdog close increments a metric; no silent loss |
| AC-2 | Producer emits a 64-entry redistribution batch while the consumer RPCs are slow | `Emit` returns within a bounded time independent of downstream RPC latency (drain runs off the emit goroutine) |
| AC-3 | Event with both engine and external subscribers | Delivery ordering is an explicit, documented contract; the replay coordinator no longer depends on an emergent `defer` |
| AC-4 | Read `docs/architecture/core-design.md` | No standalone Bus component in the §1 diagram/prose; a naming glossary disambiguates EventBus / delivery pipeline / Hub / Orchestrator / cmd/ze/hub / Report Bus |
| AC-5 | Burst producer + slow consumer, run under `-race` | No event lost beyond the documented policy, none double-delivered, no data race |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs a redistribution burst against a slow reactor | producer -> `Emit` -> async drain -> N RPCs | `test/plugin/bus-slow-consumer-nonblocking.ci` |
| 2 | a plugin floods inbound requests | `mux.readLoop` -> bounded `requestCh` -> metric | `test/plugin/bus-inbound-overflow-metric.ci` |
| 3 | an operator reads the architecture doc | core-design.md §1 + glossary | `make ze-doc-verify` on the updated doc |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEmitNonBlockingWithSlowConsumer` | `internal/component/plugin/server/dispatch_test.go` | Emit returns bounded while consumer is slow | |
| `TestInboundOverflowCounted` | `pkg/plugin/rpc/mux_test.go` | dropped inbound request increments metric, not silent | |
| `TestDeliveryOrderingContract` | `internal/component/plugin/server/dispatch_test.go` | engine vs external ordering is explicit | |
| `TestAsyncDrainNoRace` | `internal/component/bgp/redistribute/consumer_test.go` | async drain safe under `-race` | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| delivery channel capacity | 1 .. N | 64 (current) | 0 rejected | N/A |
| batch size drained per emit | 1 .. 64 | 64 | 0 no-op | growth logged |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-slow-consumer-nonblocking` | `test/plugin/bus-slow-consumer-nonblocking.ci` | slow consumer does not stall the producer | |
| `bus-inbound-overflow-metric` | `test/plugin/bus-inbound-overflow-metric.ci` | inbound overflow is observable, not silent | |

### Interop Tests (MANDATORY for protocol features)
Not applicable: this changes internal event scheduling/backpressure, not any peer-facing wire
protocol. Existing BGP/redistribution interop scenarios are the regression gate.

### Future (if deferring any tests)
- Chaos test injecting sustained slow-plugin backpressure is a strong follow-on; note if deferred.

## Files to Modify
- `internal/component/plugin/server/dispatch.go` - fan-out scheduling and explicit ordering
  contract; hand batches to an async drainer.
- `internal/component/plugin/server/engine_event.go` - engine subscriber invocation off the
  emit goroutine (bounded), preserving per-subscriber order.
- `internal/component/plugin/process/delivery.go` - explicit backpressure policy + metric on
  the engine-to-plugin path.
- `pkg/plugin/rpc/mux.go` - symmetric policy + metric on the plugin-to-engine drop path
  (`sendRequest`).
- `internal/component/bgp/redistribute/consumer.go` - drain the batch asynchronously so
  per-entry RPC latency does not block `Emit` (respect pool lifetime).
- `docs/architecture/core-design.md` - remove standalone Bus from §1; add a naming glossary.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Prometheus counters (drop/overflow, emit latency) | [ ] | telemetry registry + `docs/plugin-development/metrics.md` |
| Doctor check | [ ] | not expected (no new runtime dependency) |
| Functional tests for changed delivery | [ ] | `test/plugin/bus-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` §1 diagram + glossary |
| 14 | Prometheus counters added/changed? | [ ] | `docs/plugin-development/metrics.md` |
| 16 | Changed source referenced by doc anchors? | [ ] | grep `docs/` for `source: .../dispatch.go`, `delivery.go`, `mux.go` |

## Files to Create
- `test/plugin/bus-slow-consumer-nonblocking.ci` - proves Emit is non-blocking under a slow consumer.
- `test/plugin/bus-inbound-overflow-metric.ci` - proves inbound overflow is observable.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | `make ze-precommit-verify` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — add the drop/overflow metric and a test seam that
   measures Emit latency; write both `.ci` tests expecting bounded latency and counted overflow
   (fail now).
   - Files: `delivery.go`, `mux.go`, `dispatch.go`, the two `.ci` files.
   - Verify: tests fail because Emit still blocks and inbound drops are silent.
2. **Phase: Symmetric backpressure + metrics** — one documented policy on both directions; no
   silent drop; metrics registered.
   - Tests: `TestInboundOverflowCounted`, `bus-inbound-overflow-metric.ci`.
3. **Phase: Async fan-out** — engine handlers and the redistribution drain run off the emit
   goroutine with a bounded hand-off; snapshot payloads to respect pool lifetime.
   - Tests: `TestEmitNonBlockingWithSlowConsumer`, `TestAsyncDrainNoRace`, `bus-slow-consumer-nonblocking.ci`.
4. **Phase: Explicit ordering contract** — replace the emergent `defer` ordering with a
   documented, tested contract; verify the replay coordinator.
   - Tests: `TestDeliveryOrderingContract`.
5. **Phase: Docs + glossary** — fix core-design.md §1; add the naming glossary; `make ze-doc-verify`.
6. **Full verification** → `make ze-precommit-verify` (with `-race` on the new tests).
7. **Complete spec** → learned summary `plan/learned/NNN-bus-async-fanout.md`; two commits.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | No event lost beyond policy; none double-delivered; per-subscriber order kept |
| Data flow | Delivery still flows through `Server.Emit`; no standalone bus reborn |
| Concurrency | Async drain safe under `-race`; payload lifetime respected |
| Registration over hardcoding | Metrics register via telemetry; no per-consumer scheduling special-case in a core package |
| Rule: no-layering | Old blocking/defer paths removed, not left dormant beside the new one |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Symmetric backpressure | grep both `delivery.go` and `mux.go` reference the same policy + metric |
| Non-blocking Emit | `bus-slow-consumer-nonblocking.ci` passes; latency assertion holds |
| Ordering contract | `TestDeliveryOrderingContract` passes; doc states the contract |
| Doc fixed | `docs/architecture/core-design.md` has no standalone Bus; glossary present |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A flooding plugin cannot OOM the engine via unbounded queues (bounded + metric) |
| Error leakage | Drop/overflow logs do not leak sensitive payload contents |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Reordering breaks a subscriber | Preserve per-subscriber order (Phase 3) |
| `-race` failure on drain | Snapshot before hand-off (Phase 3, A-1) |
| Replay behavior changes | Re-check A-2; may need explicit ordering the coordinator expects |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Drain slow consumers off the emit goroutine | Bigger channel / more allocation pooling | The bottleneck is serialized synchronous I/O, not allocation; pooling solved the cheap problem |
| Glossary + diagram fix, not a mass rename | Rename bus/hub symbols repo-wide | Rename is a known minefield (review notes silent breakage); docs fix is the high-value low-risk part |
| One backpressure policy both directions | Keep block one way, drop the other | Asymmetry is the defect; symmetry + metric makes behavior predictable and observable |

## Known Limitations
- Repo-wide rename of overloaded `bus`/`hub` symbols is deferred; this spec fixes the diagram
  and adds a glossary only.
- Route-event bridge lossiness and replay vocabulary are owned by `spec-unify-route-events.md`
  and `spec-unify-replay.md`; this spec only changes how those events are scheduled/delivered.
- **Ordering constraint (hard, verified).** Async fan-out (AC-2) breaks the exact invariant
  the redistevents batch pool depends on: `ReleaseBatch` has no refcount and is safe today
  "only because every subscriber has returned by the time Emit returns"
  (`internal/core/redistevents/pool.go,68-87`, confirmed in the DESIGN-REVIEW finding 6
  review). The same holds for `attrpool.Handle` (no generation tag) and WireUpdate `RawBytes`
  (invalid after handler return). Therefore this spec MUST NOT land before
  `spec-unify-buffer-lifetime.md` enforces those lifetimes (poison/canary + snapshot-on-retain);
  the async drainer must snapshot/copy any pooled payload before it outlives `Emit`. If
  buffer-lifetime is not yet landed, the async-drain phases (3) are blocked.

## Implementation Summary

### What Was Implemented
- [filled during /implement]

### Bugs Found/Fixed
- [silent inbound drop, if it masked a real loss]

### Documentation Updates
- [core-design.md §1 + glossary; metrics doc, or "None" with grep evidence]

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes (lint + all ze tests, `-race` on new concurrency tests)
- [ ] Feature code integrated (`internal/*`, `pkg/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (work within `Server.Emit`)
- [ ] No speculative features (four named defects only)
- [ ] Explicit > implicit behavior (documented ordering + policy)
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (channel capacity, batch size)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests N/A (internal scheduling change; existing interop is the regression gate)

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Write learned summary to `plan/learned/NNN-bus-async-fanout.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-review-bus-async-fanout.md`
