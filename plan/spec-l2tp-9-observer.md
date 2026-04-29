# Spec: l2tp-9 -- Session Observer, Event Namespace, and CQM Sampler

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-l2tp-6c-ncp, spec-l2tp-7-subsystem |
| Phase | 5/5 (YANG schema + verification) |
| Updated | 2026-04-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-l2tp-0-umbrella.md` -- parent umbrella, "Event namespace" in-scope bullet
4. `internal/core/events/typed.go` -- typed event bus primitive
5. `internal/component/l2tp/session_fsm.go`, `tunnel_fsm.go`, `session.go`, `tunnel.go`
6. `internal/plugins/bfd/metrics.go` (precedent for subsystem-adjacent telemetry)

## Task

Provide the per-L2TP observability foundation: a typed event namespace for L2TP,
per-session event ring buffers, per-login CQM sample ring buffers, and an
inline EventBus observer that routes published events into those ring buffers.
CQM data comes from modifying the PPP session to report Echo-Reply RTT via a
new `ppp.EventEchoRTT` event; when CQM is enabled, the echo interval is
overridden to 1s (default 10s). The observer aggregates RTT reports into
100-second min/avg/max/loss buckets tagged with session-state (established,
negotiating, down). All ring buffer memory is pre-allocated at subsystem start.

This spec executes the "Event namespace" in-scope bullet already promised by
`spec-l2tp-0-umbrella.md`. Consumers downstream are `spec-l2tp-10-metrics` and
`spec-l2tp-11-web`.

## Design Decisions (agreed with user, 2026-04-17)

| # | Decision |
|---|----------|
| D3 | Storage, sampler, and observer live in `internal/component/l2tp/` (adjacent to emitting FSM code). Precedent: `internal/plugins/bfd/metrics.go`, `internal/component/vpp/telemetry.go`. |
| D4 | Event transport: typed events via `internal/core/events/`. Observer subscribes via `Event[T].Subscribe(bus, handler)` -- standard EventBus in-process delivery is already zero-copy. (Skeleton said "DirectBridge" but that's the plugin RPC transport, not the EventBus.) |
| D5 | CQM aggregation happens in inline EventBus handler, not via a channel-buffered goroutine. Ring append is O(1), ~50ns -- matches BFD metrics hook precedent. |
| D6 | CQM echo data comes from `ppp.EventEchoRTT` (new PPP event type), NOT from a separate echo generator. PPP session goroutine owns /dev/ppp fd exclusively. |
| D7 | When CQM is enabled, PPP echo interval overridden to 1s (default 10s) via `StartSession.EchoInterval`. Gives ~100 samples per 100s bucket. |
| D8 | `ppp.EventEchoRTT` requires adding `lastEchoSentAt time.Time` to pppSession. RTT = time.Since(lastEchoSentAt) on Echo-Reply. |
| D9 | Per-ring mutex for concurrent reactor access. Multiple reactors share one observer; each reactor's EventBus emit runs the handler in the reactor's goroutine. Lock hold ~50ns (slot overwrite). |
| D10 | Sample ring keyed by login (PPP username). Event ring keyed by session ID. |
| D11 | Bucket state enum distinguishes `established`, `negotiating`, `down`. Tx-limit and loss render as overlays, not states. |
| D12 | Retention: 24h at 100s resolution per login (864 buckets, ~34 KB/login). |
| D13 | Login identity: PPP username. |
| X | Cross-cutting: pre-allocate a pool of identically-sized ring buffers at subsystem start. Event ring pool: `max-sessions` buffers of `event-ring-size-per-session` slots. Sample ring pool: `max-logins` buffers of `retention/100` slots. On session/login creation, take from pool; on teardown/eviction, return to pool. Pool is a simple free list (slice of pre-allocated buffers), not sync.Pool. LRU eviction when sample pool exhausted. Zero runtime allocation after Start. |

## Scope

### In Scope

| Area | Description |
|------|-------------|
| Event namespace | Define `l2tp.*` event types: `tunnel-up`, `tunnel-down`, `session-up`, `session-down`, `lcp-up`, `lcp-down`, `ipcp-up`, `ipv6cp-up`, `auth-success`, `auth-failure`, `echo-timeout`, `tx-limit-hit`, `disconnect-requested`. Stable field set per type. |
| Event publisher wiring | Tunnel FSM, session FSM, PPP layer, RADIUS plugin, disconnect handler all publish via `core/events/` emit API. |
| Per-session event ring | Bounded ring buffer keyed by session ID. Pre-allocated. Size: TBD during DESIGN (target order of magnitude 200 events). |
| Per-login sample ring | Bounded ring buffer keyed by PPP username. 864 buckets. Pre-allocated. LRU eviction when `max-logins` reached. |
| CQM aggregation | Inline EventBus handler subscribes to `l2tp.echo-rtt`. Aggregates RTT values into current 100s bucket (running min/max/sum/count). Closes bucket on boundary, advances to next ring slot. State tagged from session lifecycle events. |
| Observer | Inline EventBus handlers subscribing to all `l2tp.*` events. Route each event record to per-session event ring by sessionID. No separate goroutine. |
| PPP echo RTT | New `ppp.EventEchoRTT` event type. PPP session records `lastEchoSentAt` on each Echo-Request, computes RTT on Echo-Reply, emits event on Manager.EventsOut(). Reactor relays to EventBus. |
| CQM echo interval | When CQM is enabled (`cqm-enabled` config leaf), `StartSession.EchoInterval` set to 1s. Configurable via `ze.l2tp.cqm.echo-interval` env var. |
| YANG config | `max-logins`, `sample-retention-seconds` (default 86400), `event-ring-size-per-session`. Env var registration per `rules/go-standards.md`. |
| Subsystem lifecycle | Observer and sampler start/stop tied to L2TP subsystem Start/Stop. |

### Out of Scope (other specs / deferred)

| Area | Location |
|------|----------|
| Prometheus exposure of observer state | `spec-l2tp-10-metrics` |
| Web UI, JSON/CSV/SSE feeds, uPlot chart | `spec-l2tp-11-web` |
| Disconnect action | `spec-l2tp-11-web` |
| Generic `internal/core/cqm/` engine | Deferred until second non-L2TP probe consumer exists (3+ use-case rule) |
| Persistent archive to disk | Out. All state in-memory; restart clears. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- registration pattern, subsystem lifecycle
  -> Constraint: subsystem implements `ze.Subsystem` (Start/Stop/Reload); workers started in Start, stopped in Stop
- [ ] `internal/core/events/typed.go` -- typed event bus: `events.Register[T](ns, et)` returns `*Event[T]` with `Emit(bus, T)` / `Subscribe(bus, func(T))`
  -> Decision: EventBus in-process delivery is already zero-copy (payload passed as `any`, no JSON for engine subscribers). No separate "DirectBridge" mechanism needed for observer -- standard EventBus.Subscribe suffices.
  -> Constraint: `events.Register[T]` must be called at package init; panics on duplicate (ns, et) with different T
- [ ] `plan/spec-l2tp-0-umbrella.md` -- parent umbrella scope and event namespace commitment
  -> Constraint: umbrella promises "Event namespace" as in-scope; this spec delivers it
- [ ] `plan/learned/606-eventbus-typed.md` -- typed EventBus history: string->any migration, lazy JSON marshal only for external plugin subs
  -> Decision: in-process subscribers receive Go values directly; zero allocation on hot path is already guaranteed by the bus design

### RFC Summaries
- [ ] `rfc/short/rfc1661.md` -- PPP LCP Echo-Request/Reply format and semantics
  -> Constraint: Echo-Request Code=9, Echo-Reply Code=10; Magic-Number in first 4 bytes of Data field
- [ ] `rfc/short/rfc2661.md` -- L2TPv2 session state transitions
  -> Constraint: session states: idle, wait-tunnel, wait-reply, wait-connect, wait-cs-answer, established

**Key insights:**
- "DirectBridge" in the skeleton was a misnomer. The actual EventBus already delivers in-process payloads as Go values without JSON serialization. DirectBridge is a separate concept: the plugin RPC transport for bridge-mode internal plugins. The observer subscribes via standard `Event[T].Subscribe(bus, handler)`.
- PPP sessions already run LCP Echo-Request at configurable interval (default 10s) inside the per-session goroutine. The session goroutine owns the /dev/ppp fd exclusively. A separate CQM sampler CANNOT inject its own echo requests -- it must consume RTT reports from the existing echo mechanism.
- L2TP events namespace already exists (`internal/component/l2tp/events/events.go`) with 5 typed events: `session-down`, `session-up`, `session-rate-change`, `session-ip-assigned`, `route-change`. New observer events extend this file.
- BFD metrics precedent: `atomic.Pointer[bfdMetrics]` with `bindMetricsRegistry`, `metricsHook` interface on engine loop. The L2TP observer follows a similar pattern but with ring buffers instead of Prometheus counters.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/session_fsm.go` (1086L) -- ICRQ/ICRP/ICCN/OCRQ/OCRP/OCCN/CDN/WEN/SLI handlers. Session state transitions happen in handleICCN (->established), handleOCCN (->established), handleCDN (->removed), teardownSession (->removed).
  -> Constraint: FSM handlers run under reactor's tunnelsMu. Event emit must not block (EventBus.Emit is synchronous for in-process subs -- handler must be fast).
  -> Decision: tunnel-up/tunnel-down event publish sites are in tunnel_fsm.go (handleSCCCN for up, handleStopCCN/teardownStopCCN for down). Session-up/down already emitted via EventBus in reactor.go handlePPPEvent/handleSessionUp.
- [ ] `internal/component/l2tp/tunnel_fsm.go` (677L) -- SCCRQ/SCCCN/StopCCN/Hello handlers. Tunnel established in handleSCCCN, torn down in handleStopCCN/teardownStopCCN.
  -> Constraint: tunnel state = idle/wait-ctl-conn/established/closed. Only established->closed and idle->established are interesting for events.
- [ ] `internal/core/events/events.go` (241L) -- namespace registry (`ValidEvents`), `RegisterNamespace`, `RegisterEventType`. Protected by eventsMu.
  -> Constraint: namespace registration via `RegisterNamespace(ns, eventTypes...)` is idempotent. Typed `events.Register[T]` calls `RegisterNamespace` internally.
- [ ] `internal/core/events/typed.go` (313L) -- `Event[T]`, `SignalEvent`, `Register[T]`, `RegisterSignal`. Type registry maps (ns, et) -> reflect.Type.
  -> Constraint: `Register[T]` panics if same (ns, et) registered with different T. L2TP events already registered in `l2tp/events/events.go` -- extend, don't replace.
- [ ] `internal/plugins/bfd/metrics.go` (194L) -- `bfdMetrics` struct with atomic.Pointer, `metricsHook` interface, `bindMetricsRegistry`, `refreshSessionsGauge`. Metrics hook attached to each engine Loop via `attachMetricsHook`.
  -> Decision: BFD hook pattern is good for metrics (counters, histograms), but observer needs ring buffers, not Prometheus. Different pattern: observer is a long-lived worker that subscribes to EventBus, not a hook on the engine.
- [ ] `internal/component/l2tp/reactor.go` (1000+L) -- reactor run loop with select on listener, kernelErrCh, kernelSuccessCh, pppDriver.EventsOut(), tickCh, updateCh, stop. PPP events dispatched via handlePPPEvent.
  -> Constraint: reactor already emits `l2tpevents.SessionDown`, `l2tpevents.SessionUp`, `l2tpevents.SessionIPAssigned` via EventBus. Observer subscribes to these same events.
  -> Decision: sampler worker is NOT part of the reactor select loop. It's a separate goroutine started by the subsystem, like the timer worker.
- [ ] `internal/component/l2tp/events/events.go` (100L) -- existing typed events: RouteChange, SessionDown, SessionUp, SessionRateChange, SessionIPAssigned. Namespace = "l2tp".
  -> Constraint: new observer events (tunnel-up, tunnel-down, lcp-up, lcp-down, echo-timeout, auth-success, auth-failure) EXTEND this file, same namespace.
- [ ] `internal/component/ppp/session_run.go` -- PPP session goroutine owns echo: sends Echo-Request on echoTicker (default 10s), tracks echoOutstanding, tears down after echoMax failures.
  -> Constraint: only the session goroutine can write to /dev/ppp chanFD. External CQM sampler cannot inject echo requests. Must piggyback on existing echo mechanism.
  -> Decision: CQM RTT data comes from modifying PPP session to report echo RTT on each reply via a new `ppp.Event` type (e.g., `EventEchoRTT`). Sampler aggregates these reports, does NOT generate its own echoes.
- [ ] `internal/component/l2tp/subsystem.go` (415L) -- Start creates listeners, reactors, timers, kernelWorkers, pppDrivers, drainDones. Stop in reverse order. Observer/sampler workers would be added alongside these.
  -> Constraint: Start/Stop ordering matters. Observer must start before reactor (so EventBus subscriber is ready). Sampler starts after PPP driver (needs echo events).
- [ ] `internal/component/l2tp/session.go` (242L) -- L2TPSession struct with localSID, remoteSID, state, username, assignedAddr, pppInterface. Accessed under reactor tunnelsMu.
  -> Constraint: session identity for event ring = localSID (uint16). Login identity for sample ring = username (string).
- [ ] `internal/component/l2tp/snapshot.go` (241L) -- SessionSnapshot with Username, AssignedAddr, State. Snapshot() copies under tunnelsMu.
  -> Decision: observer ring read API can be exposed via a similar snapshot pattern for spec-l2tp-11-web.

**Behavior to preserve:**
- Existing L2TP EventBus events (session-down, session-up, session-rate-change, session-ip-assigned, route-change) unchanged
- PPP echo keepalive behavior unchanged (default 10s interval, 3 failures = teardown)
- Reactor select loop structure unchanged (observer is a separate goroutine, not a new select arm)
- Subsystem Start/Stop ordering for existing workers unchanged

**Behavior to change:**
- Add new typed events to `l2tp/events/events.go`: tunnel-up, tunnel-down, lcp-up, lcp-down, echo-rtt, echo-timeout, auth-success, auth-failure
- Add EventBus emit calls in tunnel_fsm.go (tunnel established/closed) and propagate from PPP events
- Add `ppp.EventEchoRTT` to PPP session goroutine: emitted on each Echo-Reply received, carries RTT duration
- Add observer worker (EventBus subscriber -> ring buffer router) to subsystem lifecycle
- Add CQM sampler worker (aggregates EventEchoRTT into 100s buckets) to subsystem lifecycle
- Add `max-logins`, `sample-retention-seconds`, `event-ring-size-per-session` YANG leaves

## Data Flow (MANDATORY)

### Entry Points
- FSM state transitions (session FSM, tunnel FSM)
- PPP negotiation completions (LCP, IPCP, IPv6CP)
- RADIUS plugin outcomes
- CQM sampler (LCP echo loop)

### Transformation Path
1. FSM/reactor emits typed event via `l2tpevents.TunnelUp.Emit(bus, payload)` (standard EventBus, in-process delivery is zero-copy)
2. Observer worker's `Event[T].Subscribe(bus, handler)` callback receives Go value directly (no JSON, no allocation)
3. Observer routes event to per-session event ring (keyed by sessionID) or per-login sample ring (keyed by username)
4. Ring buffers read by metrics exporter (`spec-l2tp-10`) and web feeds (`spec-l2tp-11`) via snapshot API

CQM path:
1. PPP session goroutine receives Echo-Reply, computes RTT, emits `ppp.EventEchoRTT` on Manager.EventsOut()
2. Reactor reads EventEchoRTT from pppDriver.EventsOut(), emits `l2tpevents.EchoRTT.Emit(bus, payload)`
3. CQM sampler (EventBus subscriber) aggregates RTT into current 100s bucket
4. On bucket boundary, sampler closes bucket (min/avg/max/loss/state) and appends to per-login sample ring

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| PPP session -> reactor | `ppp.EventEchoRTT` on `Manager.EventsOut()` channel |
| Reactor -> observer | `l2tpevents.X.Emit(bus, payload)` -- EventBus in-process delivery |
| PPP session -> CQM sampler | `l2tpevents.EchoRTT.Emit(bus, payload)` via reactor relay |
| Observer/sampler -> downstream | Ring buffer read API (snapshot, no re-emission) |

### Integration Points
- `subsystem.go` Start/Stop wires observer and sampler worker lifecycle
- `reactor.go` handlePPPEvent relays new `ppp.EventEchoRTT` to EventBus
- `ppp/session_run.go` emits `EventEchoRTT` on Echo-Reply received (new event type in ppp/events.go)
- `l2tp/events/events.go` defines new typed event handles (tunnel-up, tunnel-down, echo-rtt, etc.)
- `l2tp-8b-radius` plugin is already an EventBus publisher (session-ip-assigned); auth-success/failure events added here

### Architectural Verification
- [ ] Zero-copy preserved: EventBus in-process delivery passes Go values, no JSON marshal for engine subscribers
- [ ] Pre-allocation verified at Start (no runtime `make` in hot path)
- [ ] LRU eviction tested at `max-logins`
- [ ] PPP echo mechanism unchanged (observer piggybacks on RTT reports, does not inject its own echoes)

## Wiring Test (MANDATORY)

<!-- Filled during DESIGN phase with concrete .ci test names. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| LCP Echo-Request scheduled on established session | → | CQM sampler 100s bucket aggregation | `test/l2tp/observer-cqm-bucket.ci` |
| Session FSM enters Established | → | `l2tp.session-up` event published, observer appends to ring | `test/l2tp/observer-event-routing.ci` |
| `max-logins` reached | → | LRU eviction on new login arrival | `test/l2tp/observer-lru-eviction.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Tunnel reaches established (SCCCN accepted) | `l2tp.tunnel-up` typed event emitted with TunnelID, PeerAddr, PeerHostName; observer appends to per-session event ring |
| AC-2 | PPP session reaches Established (EventSessionUp) | Observer appends `session-up` event to per-session event ring |
| AC-3 | Echo-Reply received on established session with CQM enabled | `ppp.EventEchoRTT` emitted with RTT; reactor relays to `l2tp.echo-rtt`; observer updates current CQM bucket min/max/sum/count |
| AC-4 | 100s elapses with echo traffic on established session | CQM bucket closed with state=`established`, loss=0, min/avg/max RTT populated. Appended to per-login sample ring. |
| AC-5 | Echo times out (3 consecutive failures at 1s CQM interval) | `session-down` event with echo-timeout reason in event ring; CQM bucket covering the down window has state=`down` |
| AC-6 | Same login reconnects on new session ID | Sample ring continues (login-keyed continuity); event ring starts fresh for new session ID |
| AC-7 | `max-logins` reached and a new login arrives | LRU login's sample ring reclaimed; new login uses pre-allocated slot; no runtime allocation |
| AC-8 | Subsystem Start with CQM enabled | All rings pre-allocated based on `max-logins`; PPP echo interval overridden to 1s |
| AC-9 | Subsystem Stop | Observer unsubscribes from EventBus; no goroutine leak; ring memory available for GC |
| AC-10 | `event-ring-size-per-session` set to 64 | Per-session event ring holds 64 entries; 65th overwrites oldest |
| AC-11 | `sample-retention-seconds` set to 3600 (1h) | Per-login sample ring holds 36 buckets (3600/100); 37th overwrites oldest |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestEventRingAppendAndWrap | `observer_test.go` | Circular buffer overwrites oldest when full | |
| TestEventRingRoutesBySessionID | `observer_test.go` | Events with different SIDs land in correct rings | |
| TestSampleRingBucketClose | `cqm_test.go` | After 100s, bucket closed with correct min/avg/max/loss | |
| TestSampleRingBucketStateTag | `cqm_test.go` | Bucket state reflects established/negotiating/down from lifecycle events | |
| TestSampleRingLossCount | `cqm_test.go` | Missing echoes in a 100s window reflected in loss field | |
| TestObserverLRUEviction | `observer_test.go` | At max-logins, new login evicts LRU; reclaimed buffer reused from pool | |
| TestObserverPoolPreallocation | `observer_test.go` | After NewObserver, pool has max-logins free buffers; no alloc on Acquire | |
| TestObserverPoolReturnAndReuse | `observer_test.go` | Released buffer returned to pool; next Acquire returns same buffer | |
| TestLoginContinuity | `observer_test.go` | Same username reconnection: sample ring preserved, event ring fresh | |
| TestEventEchoRTTEmitted | `ppp/session_run_test.go` | Echo-Reply produces EventEchoRTT with correct RTT duration | |
| TestTunnelUpEventEmitted | `tunnel_fsm_test.go` | handleSCCCN emits tunnel-up event via EventBus | |
| TestTunnelDownEventEmitted | `tunnel_fsm_test.go` | handleStopCCN emits tunnel-down event via EventBus | |
| TestEventRegistration | `l2tp/events/events_test.go` | All new event types registered with correct payload types | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `max-logins` | 1-1000000 | 1000000 | 0 | 1000001 |
| `sample-retention-seconds` | 100-86400 | 86400 | 99 | 86401 |
| `event-ring-size-per-session` | 16-4096 | 4096 | 15 | 4097 |
| CQM bucket count | retention/100 | 864 (at 86400) | 1 (at 100) | N/A |
| Echo RTT | 0-MaxInt64 (Duration) | N/A | negative (clamp to 0) | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| TBD | `test/l2tp/observer-cqm-bucket.ci` | Session establishes, 100s elapses, one bucket lands with expected fields | |
| TBD | `test/l2tp/observer-event-routing.ci` | FSM transitions generate events in per-session ring | |
| TBD | `test/l2tp/observer-lru-eviction.ci` | Exceeding `max-logins` evicts LRU | |

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ppp/events.go` | Add `EventEchoRTT` struct (TunnelID, SessionID, RTT time.Duration) |
| `internal/component/ppp/session.go` | Add `lastEchoSentAt time.Time` field (goroutine-owned, no lock) |
| `internal/component/ppp/session_run.go` | Record `lastEchoSentAt = time.Now()` in sendEchoRequest; compute RTT and emit EventEchoRTT in handleLCPPacket on Echo-Reply |
| `internal/component/l2tp/events/events.go` | Add TunnelUp, TunnelDown, EchoRTT typed event handles with payload structs |
| `internal/component/l2tp/reactor.go` | Add EventEchoRTT to handlePPPEvent switch; relay to EventBus; bump pppEventTypeFreeze from [5] to [6]; set CQM echo interval on StartSession |
| `internal/component/l2tp/tunnel_fsm.go` | No direct changes. Reactor emits tunnel-up/down AFTER FSM returns, matching existing session-down pattern (reactor checks tunnel state change and emits). |
| `internal/component/l2tp/reactor.go` (tunnel events) | After calling handleMessage, reactor checks if tunnel transitioned to established (emit tunnel-up) or closed (emit tunnel-down). Same pattern as handlePPPEvent -> session-down emit. |
| `internal/component/l2tp/subsystem.go` | Create observer at Start, wire EventBus, pass config params; unsubscribe at Stop; pass CQM echo interval config to reactor |
| `internal/component/l2tp/config.go` | Add MaxLogins, SampleRetentionSeconds, EventRingSizePerSession, CQMEnabled parameters |
| `internal/component/l2tp/schema/ze-l2tp-conf.yang` | Add `max-logins`, `sample-retention-seconds`, `event-ring-size-per-session`, `cqm-enabled` leaves |

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | `internal/component/l2tp/schema/ze-l2tp-conf.yang` |
| Env vars for new leaves | [ ] | Per `rules/go-standards.md` env section |
| Functional test for end-to-end routing | [ ] | `test/l2tp/observer-*.ci` |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [ ] | N/A (no CLI in this spec) |
| 4 | API/RPC added/changed? | [ ] | Event namespace docs in `docs/architecture/api/events.md` |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | `docs/guide/l2tp.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/l2tp.md` (new observer + CQM section) |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/l2tp/observer.go` | Observer struct, ring buffer pool (pre-allocated free list), event ring type (circular buffer of ObserverEvent), sample ring type (circular buffer of CQMBucket), LRU eviction map, EventBus subscribe/unsubscribe, ring read snapshot API |
| `internal/component/l2tp/cqm.go` | CQMBucket struct (~40 bytes: timestamp, state, echo count, loss, min/max/sum RTT), BucketState enum (established/negotiating/down), aggregation logic (update running min/max/sum on each echo-rtt), bucket boundary detection (100s wall-clock), state tagging from session lifecycle |
| `internal/component/l2tp/observer_test.go` | Ring append/wrap, routing by SID, LRU eviction, pool pre-allocation/return/reuse, login continuity |
| `internal/component/l2tp/cqm_test.go` | Bucket aggregation math, state transitions, loss counting, boundary detection |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This spec + session state |
| 2. Audit | Files to Modify/Create tables |
| 3. Implement (TDD) | Implementation Phases 1-5 below |
| 4. Full verification | `make ze-verify` |
| 5-12 | Per standard /implement flow |

### Implementation Phases

**Phase 1: PPP EventEchoRTT** (TDD: TestEventEchoRTTEmitted)
- Add `EventEchoRTT` to `ppp/events.go`
- Add `lastEchoSentAt time.Time` to `ppp/session.go`
- Modify `sendEchoRequest` to record `lastEchoSentAt = time.Now()`
- Modify `handleLCPPacket` for `LCPEchoReply`: compute RTT, emit EventEchoRTT
- Bump `pppEventTypeFreeze` in `reactor.go` from [5] to [6]
- Add EventEchoRTT case in reactor `handlePPPEvent`: relay to EventBus

**Phase 2: L2TP event types** (TDD: TestEventRegistration, TestTunnelUpEventEmitted, TestTunnelDownEventEmitted)
- Add `TunnelUp`, `TunnelDown`, `EchoRTT` to `l2tp/events/events.go` with payload structs
- Wire tunnel-up emit in `tunnel_fsm.go` handleSCCCN (needs EventBus ref; pass via tunnel struct from reactor)
- Wire tunnel-down emit in `tunnel_fsm.go` handleStopCCN/teardownStopCCN

**Phase 3: Ring buffer pool and event ring** (TDD: TestEventRingAppendAndWrap, TestEventRingRoutesBySessionID, TestObserverPoolPreallocation, TestObserverPoolReturnAndReuse)
- Implement `observer.go`: eventRing (circular buffer), ringPool (pre-allocated free list), Observer struct
- EventBus subscribe: session-up, session-down, tunnel-up, tunnel-down, session-ip-assigned, echo-rtt
- Route events to per-session event ring by sessionID
- Allocate event ring from pool on first event for a session; return on session-down

**Phase 4: CQM sample ring and aggregation** (TDD: TestSampleRingBucketClose, TestSampleRingBucketStateTag, TestSampleRingLossCount, TestObserverLRUEviction, TestLoginContinuity)
- Implement `cqm.go`: CQMBucket struct, BucketState enum, sampleRing (circular buffer of buckets)
- CQM aggregation in echo-rtt handler: update running min/max/sum/count in current bucket
- Bucket boundary detection: compare wall-clock time against bucket start; close on 100s boundary
- State tagging: session-up -> established, lcp-up -> negotiating, session-down -> down
- Per-login sample ring pool with LRU eviction
- Login continuity: same username reconnection preserves sample ring

**Phase 5: Subsystem wiring and config** (TDD: integration tests)
- Add config params to `config.go` and YANG schema
- Wire Observer creation in `subsystem.go` Start; pass EventBus, config params
- Set CQM echo interval on StartSession when cqm-enabled
- Unsubscribe observer in Stop
- Env var registration per go-standards.md

### Critical Review Checklist

| Check | What to verify |
|-------|---------------|
| Completeness | AC-1..AC-11 all demonstrated |
| Correctness | CQM bucket math: avg = sum/count, not sum/(count+loss) |
| Naming | JSON keys kebab-case, YANG kebab-case, Go packages no hyphens |
| Data flow | echo-rtt flows PPP -> reactor -> EventBus -> observer -> ring |
| Rule: no runtime alloc | Ring pool exhaustion returns nil, not make; handler no-ops |
| Rule: goroutine-lifecycle | No new goroutines; inline handlers only |
| Concurrency | Per-ring mutex; multiple reactors share one observer |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| EventEchoRTT emitted on Echo-Reply | `ppp/session_run_test.go` |
| tunnel-up/tunnel-down events | `tunnel_fsm_test.go` |
| Event ring append and wrap | `observer_test.go` |
| CQM bucket aggregation | `cqm_test.go` |
| LRU eviction | `observer_test.go` |
| Pool pre-allocation | `observer_test.go` |
| Subsystem Start/Stop wiring | `subsystem_test.go` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | max-logins bounds memory; pool never grows past pre-allocated size |
| Information leak | Ring buffers contain session metadata (username, IP); access only via snapshot API, not exported |
| Denial of service | Echo-rtt events from a flood of fake sessions: bounded by pool size |
| Memory safety | Ring buffer index arithmetic uses modulo; no out-of-bounds possible |

### Failure Routing (inherited from template)

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source, back to RESEARCH |
| Lint failure | Fix inline; if architectural, back to DESIGN |
| Functional test fails | Check AC; back to DESIGN or IMPLEMENT |
| 3 fix attempts fail | STOP. Report. Ask user. |

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

- "DirectBridge" was a misnomer in the skeleton. The EventBus in-process delivery is already zero-copy. DirectBridge is the plugin RPC transport for bridge-mode plugins.
- PPP session goroutine owns /dev/ppp fd exclusively. CQM data comes from piggybacking on the existing echo mechanism via EventEchoRTT, not from a separate echo generator.
- Inline EventBus handlers (Approach A) chosen over channel-buffered goroutine (Approach B). Ring append is O(1), ~50ns. Adding a channel is overhead with no benefit at this work level. Matches BFD metrics hook precedent.
- Ring buffer pool (free list of pre-allocated buffers) chosen because all buffers within a pool are identically sized. On session create: take from pool. On teardown: return to pool. Zero runtime allocation.
- Per-ring mutex needed because multiple reactors share one observer. Lock hold time ~50ns (slot overwrite). Acceptable contention.

## RFC Documentation

Add `// RFC 1661 Section 5.8` (LCP Echo-Request/Reply) near sampler code when implemented.

## Implementation Summary (filled during IMPLEMENT)

## Implementation Audit (filled during IMPLEMENT)

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

## Review Gate (filled during IMPLEMENT)

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

## Pre-Commit Verification (filled during IMPLEMENT)

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table filled with concrete test names
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated in subsystem Start/Stop
- [ ] Documentation updated

### Quality Gates
- [ ] RFC 1661 Section 5.8 reference comment near sampler
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Pre-allocation verified at Start
- [ ] No runtime `make` on observer or sampler hot path
- [ ] EventBus in-process zero-copy preserved (no JSON marshal for engine subs)
- [ ] Single responsibility per file (observer, cqm, events)

### TDD
- [ ] Tests written
- [ ] Tests FAIL first
- [ ] Tests PASS after implementation
- [ ] Boundary tests for `max-logins`, `sample-retention-seconds`, `event-ring-size-per-session`
- [ ] Functional tests exercise end-to-end event routing

### Completion (BLOCKING before commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary at `plan/learned/NNN-l2tp-9-observer.md`
- [ ] Summary in same commit as code
