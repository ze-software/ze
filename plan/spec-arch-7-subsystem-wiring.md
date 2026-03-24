# Spec: arch-7-subsystem-wiring

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-03-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/subsystem-wiring.md` — target architecture with Mermaid diagrams
4. `pkg/ze/subsystem.go` — Subsystem interface
5. `internal/component/engine/engine.go` — Engine implementation
6. `internal/component/bus/bus.go` — Bus implementation
7. `cmd/ze/hub/main.go` — current startup path (to be changed)
8. `internal/component/bgp/reactor/reactor.go` — Reactor struct and Start/Stop
9. `internal/component/bgp/server/event_dispatcher.go` — EventDispatcher (to become Bus Consumer)
10. `internal/component/bgp/server/events.go` — event delivery functions

## Task

Wire the BGP reactor through the Engine as a `ze.Subsystem`, completing the arch-0 component boundary work. Three deliverables:

1. **BGPSubsystem adapter** — wraps `reactor.Reactor`, implements `ze.Subsystem`
2. **Startup migration** — `cmd/ze/hub/main.go` uses Engine instead of direct `reactor.Start()`
3. **Bus notification layer** — reactor publishes lightweight notifications to Bus topics in parallel with existing EventDispatcher data delivery

This unblocks `spec-iface-bus` (interface lifecycle management on the Bus) and any future cross-component communication.

### Design Decision: Bus is Notification, Not Data Transport

The Bus publishes lightweight signals ("an UPDATE arrived from peer X", "peer Y went down"). Plugin data delivery stays on the existing EventDispatcher direct path. The EventDispatcher returns cache consumer counts, handles per-subscriber format negotiation, manages DirectBridge zero-copy, and enforces dependency-ordered delivery — all of which require synchronous calling conventions that the fire-and-forget Bus cannot provide.

Cross-component consumers (e.g., interface plugin) subscribe to Bus notifications and react to signals. Plugins that need actual UPDATE data already have direct access via `pluginserver.Server` and DirectBridge.

### Design Decision: EventDispatcher Unchanged

The EventDispatcher keeps its current calling convention. The reactor calls it directly (not via Bus). No changes to format negotiation, cache consumer count tracking, or delivery semantics.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/subsystem-wiring.md` — target architecture diagrams
  → Decision: reference-based payloads for UPDATEs, full JSON for non-UPDATEs
  → Decision: EventDispatcher becomes Bus Consumer, format negotiation unchanged
- [ ] `docs/architecture/core-design.md` — current engine + plugin architecture
  → Constraint: WireUpdate is lazy-parsed, zero-copy — Bus must not force eager parsing
  → Constraint: engine is stateless for routes (no RIB in reactor)
- [ ] `plan/spec-arch-0-system-boundaries.md` — umbrella arch spec
  → Decision: 5 components: Engine, Bus, ConfigProvider, PluginManager, Subsystem
  → Decision: Bus is content-agnostic, payload always `[]byte`, topics hierarchical with `/`
  → Constraint: Subsystem receives Bus + ConfigProvider at startup
- [ ] `docs/architecture/update-cache.md` — RecentUpdateCache design
  → Constraint: cache entries are keyed by update-id, accessible while pending

### Completed Phases (learned summaries)
- [ ] `plan/learned/327-arch-5-engine.md` — Engine built, wiring deferred
  → Decision: `NewEngine(bus, config, plugins)` — caller owns construction
  → Decision: Hub.Orchestrator replacement deferred — this spec completes it
- [ ] `plan/learned/324-arch-2-bus.md` — Bus built, server integration deferred
  → Decision: server integration deferred — this spec completes it
- [ ] `plan/learned/328-arch-6-eliminate-hooks.md` — EventDispatcher replaces BGPHooks
  → Decision: EventDispatcher chosen over Bus for format negotiation
  → Constraint: EventDispatcher lives in `bgp/server/`, imports `plugin`

**Key insights:**
- Engine, Bus, Subsystem interface all exist and are tested — this is a wiring spec, not a design spec
- EventDispatcher handles format negotiation per subscriber (text/json, parsed/raw, DirectBridge)
- RecentUpdateCache stores UPDATE messages by update-id — enables reference-based Bus payloads
- State and EOR events need dependency-ordered delivery (reverse topological tier)
- `cmd/ze/hub/main.go` has two paths: `runBGPInProcess()` (YANG config) and `runOrchestratorWithData()` (hub config) — only the BGP path changes

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/main.go` — `runBGPInProcess()` calls `bgpconfig.LoadReactorWithPlugins()` then `reactor.Start()` directly. Signal handling (SIGINT/SIGTERM), stdin EOF monitoring, privilege dropping, GR marker reading. Hub orchestrator path is separate and unchanged.
  → Constraint: privilege drop happens AFTER reactor.Start() (port binding) — must preserve this ordering
  → Constraint: GR marker read/remove happens BEFORE reactor.Start() — must preserve
  → Constraint: chaos wrappers (SetClock, SetDialer, SetListenerFactory) injected between load and start
  → Constraint: ze.ready.file signal written after reactor.Start() — test infrastructure depends on this
- [ ] `internal/component/bgp/reactor/reactor.go` — `Reactor` struct holds `*pluginserver.Server` as `api` and `*EventDispatcher` as `eventDispatcher`. `StartWithContext()` starts listeners, cache, metrics. `Stop()` cancels context. `Wait()` waits for wg.
  → Constraint: `New(config)` constructor takes `*Config` — adapter must not change this
  → Constraint: `SetClock/SetDialer/SetListenerFactory` called between construction and Start
- [ ] `internal/component/bgp/server/event_dispatcher.go` — `EventDispatcher` wraps `*pluginserver.Server` + `*format.JSONEncoder`. Methods: `OnMessageReceived`, `OnMessageBatchReceived`, `OnMessageSent`, `OnPeerStateChange`, `OnEORReceived`, `OnPeerNegotiated`, `OnPeerCongestionChange`, `BroadcastValidateOpen`.
  → Constraint: `BroadcastValidateOpen` is synchronous request/response — does NOT fit Bus pub/sub pattern
  → Constraint: `OnMessageReceived` returns `int` (cache consumer count) — Bus delivery is fire-and-forget
- [ ] `internal/component/bgp/server/events.go` — per-event delivery functions. Format caching via stack-allocated `formatCache`. StructuredUpdate pooling for DirectBridge. Dependency-ordered delivery for state/EOR events. Monitor delivery for CLI `subscribe` command.
  → Constraint: `onMessageReceived` returns cache consumer count used for `RecentUpdateCache.Activate()`
  → Constraint: state events delivered sequentially in reverse dependency tier order
  → Constraint: monitor delivery uses json+parsed format regardless of subscriber preferences
- [ ] `internal/component/bgp/reactor/reactor_notify.go` — `notifyMessageReceiver()` builds `PeerInfo`, runs ingress filters, caches UPDATE, routes to `messageReceiver.OnMessage*`. Returns `bool` (whether buf was kept by cache).
  → Constraint: ingress filters run BEFORE caching and dispatch — must preserve this ordering
  → Constraint: per-peer delivery channel for UPDATEs (async via `deliverChan`)
  → Constraint: non-UPDATE messages delivered synchronously (FSM-critical)
- [ ] `internal/component/engine/engine.go` — `Engine` struct with `NewEngine(bus, config, plugins)`. `Start()`: plugins first, subsystems in order. `Stop()`: subsystems reverse, then plugins. `Reload()`: calls `sub.Reload()` for each.
  → Constraint: `Start()` takes `context.Context`, subsystem `Start(ctx, bus, config)` signature
- [ ] `internal/component/bus/bus.go` — `Bus` with `NewBus()`, `CreateTopic`, `Publish`, `Subscribe`, `Unsubscribe`, `Stop`. Per-consumer delivery goroutine with batched drain. Prefix matching. Metadata filtering.
  → Constraint: `Publish()` returns nothing — fire-and-forget, no return value for cache consumer count
- [ ] `pkg/ze/subsystem.go` — `Subsystem` interface: `Name()`, `Start(ctx, Bus, ConfigProvider)`, `Stop(ctx)`, `Reload(ctx, ConfigProvider)`
- [ ] `pkg/ze/bus.go` — `Bus` interface, `Consumer` interface (`Deliver([]Event) error`), `Event` struct (Topic, Payload, Metadata)

**Behavior to preserve:**
- Format negotiation per subscriber (text/json, parsed/raw, DirectBridge)
- DirectBridge zero-copy for in-process plugins (StructuredUpdate pooling)
- Dependency-ordered sequential delivery for state and EOR events
- Cache consumer count tracking for RecentUpdateCache lifecycle
- Ingress filter chain before caching/dispatch
- Per-peer async delivery channel for received UPDATEs
- Synchronous delivery for non-UPDATE messages (FSM-critical)
- BroadcastValidateOpen synchronous request/response
- Chaos wrapper injection between construction and start
- Privilege drop after port binding
- GR marker read before start
- Signal handling (SIGINT/SIGTERM for shutdown, SIGHUP deferred)
- Stdin EOF monitoring for pipe mode
- ze.ready.file signal for test infrastructure
- Monitor delivery for CLI `subscribe` command
- Hub orchestrator path (runOrchestratorWithData) unchanged

**Behavior to change:**
- `cmd/ze/hub/main.go` runBGPInProcess: use Engine.Start() instead of reactor.Start() directly
- Reactor publishes lightweight Bus notifications in parallel with existing EventDispatcher calls
- Reactor stores Bus reference for future cross-component consumers (iface-bus)

## Data Flow (MANDATORY)

### Entry Points

Three entry points affected by this change:

| Source | Entry | Format |
|--------|-------|--------|
| TCP wire | Peer session receives UPDATE | Raw bytes in pool buffer |
| Peer FSM | State transitions (up/down) | PeerInfo + state string |
| CLI/startup | `cmd/ze/hub/main.go` | Config file path |

### Transformation Path

**Startup path (changed):**
1. `cmd/ze/hub/main.go` calls `bgpconfig.LoadReactorWithPlugins()` — unchanged
2. Creates `bus.NewBus()`, wraps reactor in `BGPSubsystem` adapter
3. Creates `engine.NewEngine(bus, nilConfig, nilPlugins)` — ConfigProvider and PluginManager are nil stubs for now (full integration in future specs)
4. `engine.Start(ctx)` calls `BGPSubsystem.Start(ctx, bus, config)`
5. BGPSubsystem stores bus, creates topics, subscribes EventDispatcher, calls `reactor.StartWithContext(ctx)`

**UPDATE event path (dual-path):**
1. TCP session → `notifyMessageReceiver()` — unchanged: builds PeerInfo, runs ingress filters, caches UPDATE
2. EventDispatcher data path unchanged: `receiver.OnMessageReceived(peerInfo, msg)` → returns consumerCount → `Activate(id, count)`
3. In parallel: reactor publishes lightweight notification to `bus.Publish("bgp/update", notification, metadata)`
4. Cross-component Bus consumers receive notification and react (e.g., future interface plugin)

**State event path (dual-path):**
1. Peer FSM → reactor `notifyPeerClosed()` / `notifyPeerEstablished()` — unchanged
2. EventDispatcher data path unchanged: `eventDispatcher.OnPeerStateChange()` with dependency ordering
3. In parallel: reactor publishes notification to `bus.Publish("bgp/state", notification, metadata)`
4. Cross-component Bus consumers receive notification and react

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI → Engine | `engine.Start(ctx)` | [ ] |
| Engine → BGPSubsystem | `sub.Start(ctx, bus, config)` | [ ] |
| Reactor → Bus (notification) | `bus.Publish(topic, notification, metadata)` | [ ] |
| Reactor → EventDispatcher (data) | direct method calls (unchanged) | [ ] |
| EventDispatcher → Plugins | per-process delivery (unchanged) | [ ] |

### Integration Points
- `internal/component/engine/engine.go` — Engine.Start/Stop/Reload orchestrates BGPSubsystem
- `internal/component/bus/bus.go` — Bus.Publish/Subscribe connects reactor to EventDispatcher
- `internal/component/bgp/reactor/reactor.go` — Reactor stores Bus reference, publishes events
- `internal/component/bgp/server/event_dispatcher.go` — EventDispatcher implements ze.Consumer
- `cmd/ze/hub/main.go` — startup path uses Engine

### Architectural Verification
- [ ] No bypassed layers (startup goes through Engine, not direct reactor.Start())
- [ ] No unintended coupling (Engine has no BGP knowledge, Bus has no BGP knowledge)
- [ ] No duplicated functionality (reuses existing Bus and Engine implementations)
- [ ] Dual-path clean (Bus = notification only, EventDispatcher = data only, no overlap)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Engine.Start() with BGPSubsystem registered | → | BGPSubsystem.Start() calls reactor.StartWithContext() | `TestEngineStartsBGPSubsystem` |
| Engine.Stop() | → | BGPSubsystem.Stop() calls reactor.Stop() + Wait() | `TestEngineStopsBGPSubsystem` |
| Reactor event | → | Bus.Publish("bgp/...") notification reaches subscriber | `TestBusNotificationPublished` |
| Config with BGP section | → | Engine starts reactor via BGPSubsystem | `test/parse/engine-startup.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Engine.Start() with BGPSubsystem registered | BGPSubsystem.Start() called, reactor starts, Bus topics created |
| AC-2 | Engine.Stop() | BGPSubsystem.Stop() called, reactor stops gracefully, Bus stops |
| AC-3 | BGPSubsystem.Start() receives Bus | Bus topics `bgp/update`, `bgp/state`, `bgp/negotiated`, `bgp/eor`, `bgp/congestion` created |
| AC-4 | Received UPDATE message | Reactor publishes notification to `bgp/update` Bus topic; EventDispatcher data path unchanged |
| AC-5 | Peer state change (up/down) | Reactor publishes notification to `bgp/state` Bus topic; EventDispatcher data path unchanged |
| AC-6 | EOR marker detected | Reactor publishes notification to `bgp/eor` Bus topic |
| AC-7 | Capability negotiation complete | Reactor publishes notification to `bgp/negotiated` Bus topic |
| AC-8 | Congestion state change | Reactor publishes notification to `bgp/congestion` Bus topic |
| AC-9 | Bus subscriber receives notification | A Bus consumer subscribed to `bgp/` prefix receives published events |
| AC-10 | EventDispatcher data path | Cache consumer count, format negotiation, DirectBridge, dependency ordering all unchanged |
| AC-11 | Chaos wrappers | SetClock/SetDialer/SetListenerFactory still injectable between construction and start |
| AC-12 | Privilege drop | Happens after reactor.Start() (port binding), before accepting connections |
| AC-13 | `make ze-verify` | All existing tests pass — no behavior change |
| AC-14 | `cmd/ze/hub/main.go` BGP path | Uses Engine.Start() instead of reactor.Start() |
| AC-15 | `cmd/ze/hub/main.go` hub path | Unchanged (runOrchestratorWithData) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPSubsystemName` | `internal/component/bgp/subsystem/subsystem_test.go` | Name() returns "bgp" | |
| `TestBGPSubsystemStartStop` | `internal/component/bgp/subsystem/subsystem_test.go` | Start calls reactor.StartWithContext, Stop calls reactor.Stop+Wait | |
| `TestBGPSubsystemCreatesBusTopics` | `internal/component/bgp/subsystem/subsystem_test.go` | Start creates bgp/update, bgp/state, bgp/negotiated, bgp/eor, bgp/congestion topics | |
| `TestEngineStartsBGPSubsystem` | `internal/component/bgp/subsystem/subsystem_test.go` | Engine.Start() reaches BGPSubsystem.Start() | |
| `TestEngineStopsBGPSubsystem` | `internal/component/bgp/subsystem/subsystem_test.go` | Engine.Stop() reaches BGPSubsystem.Stop() | |
| `TestBusNotificationPublished` | `internal/component/bgp/subsystem/subsystem_test.go` | Reactor publishes notification to Bus, subscriber receives it | |
| `TestBusNotificationMetadata` | `internal/component/bgp/subsystem/subsystem_test.go` | Bus notification has correct metadata (peer, direction) | |

### Boundary Tests (MANDATORY for numeric inputs)

No new numeric inputs — this spec is pure wiring.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-engine-startup` | `test/parse/engine-startup.ci` | BGP config parsed, reactor starts via Engine, ze.ready.file written | |

### Future (if deferring any tests)
- Cross-component Bus test (interface plugin publishes, BGP subscribes) — deferred to spec-iface-bus
- Engine.Reload() integration test — deferred until ConfigProvider is wired (spec-arch-0 tracks this)
- PluginManager integration — currently nil stub, deferred until PluginManager is wired to Engine
- Bus notification content verification for each event type — deferred to when first cross-component consumer exists

## Files to Modify

- `cmd/ze/hub/main.go` — `runBGPInProcess()` uses Engine for startup instead of direct reactor.Start()
- `internal/component/bgp/reactor/reactor.go` — add Bus field, SetBus method
- `internal/component/bgp/reactor/reactor_notify.go` — publish Bus notifications in parallel with existing EventDispatcher calls
- `docs/architecture/core-design.md` — update to reflect Engine-supervised startup

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A — no new RPCs |
| CLI commands/flags | No | N/A — startup path changes only |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — Engine-supervised startup, Bus event flow |

## Files to Create

- `internal/component/bgp/subsystem/subsystem.go` — BGPSubsystem adapter implementing ze.Subsystem
- `internal/component/bgp/subsystem/subsystem_test.go` — unit tests for BGPSubsystem
- `test/parse/engine-startup.ci` — functional test: config → Engine → reactor starts

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what exists |
| 3. Implement (TDD) | Phases 1-4 below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: BGPSubsystem adapter** — Create `internal/component/bgp/subsystem/` package with adapter wrapping Reactor
   - Tests: `TestBGPSubsystemName`, `TestBGPSubsystemStartStop`, `TestBGPSubsystemCreatesBusTopics`, `TestEngineStartsBGPSubsystem`, `TestEngineStopsBGPSubsystem`
   - Files: `subsystem.go`, `subsystem_test.go`
   - Verify: tests fail → implement → tests pass

2. **Phase: Bus notifications + startup wiring** — Reactor publishes Bus notifications in parallel with existing EventDispatcher calls. Wire `cmd/ze/hub/main.go` through Engine.
   - Tests: `TestBusNotificationPublished`, `TestBusNotificationMetadata`, functional test `test/parse/engine-startup.ci`
   - Files: `reactor.go`, `reactor_notify.go`, `cmd/ze/hub/main.go`
   - Verify: tests fail → implement → tests pass

5. **Functional tests** → Create after feature works. Cover user-visible behavior.
6. **Full verification** → `make ze-verify`
7. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/NNN-arch-7-subsystem-wiring.md`, delete spec from `plan/`.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | EventDispatcher data path completely unchanged (no regressions) |
| Correctness | Bus notifications fire for all event types (update, state, eor, negotiated, congestion) |
| Startup ordering | GR marker → chaos wrappers → reactor.Start → privilege drop → ready file |
| No performance regression | Bus notification is fire-and-forget, does not block EventDispatcher path |
| Rule: no-layering | No duplicate event dispatch — Bus is notification only, EventDispatcher is data only |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/component/bgp/subsystem/subsystem.go` exists | `ls -la internal/component/bgp/subsystem/subsystem.go` |
| BGPSubsystem implements ze.Subsystem | `grep 'var.*Subsystem.*BGPSubsystem' internal/component/bgp/subsystem/` |
| Engine used in cmd/ze/hub/main.go | `grep 'engine\.' cmd/ze/hub/main.go` |
| Bus topics created | `grep 'CreateTopic' internal/component/bgp/subsystem/subsystem.go` |
| Bus notifications published | `grep 'bus.*Publish\|Publish.*bgp/' internal/component/bgp/reactor/reactor_notify.go` |
| Functional test exists | `ls -la test/parse/engine-startup.ci` |
| `make ze-verify` passes | Run and paste output |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Bus payload decode: validate update-id and peer-address format |
| Resource exhaustion | Bus delivery channel (cap 64) — same as existing, no new risk |
| Privilege | Privilege drop ordering preserved after Engine migration |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| Cache consumer count wrong | Phase 3 — verify Activate() call path after Bus indirection |
| Existing tests fail | Investigate — likely startup ordering or nil pointer from missing wiring |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## RFC Documentation

No RFC work — this is internal architecture wiring.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-arch-7-subsystem-wiring.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
