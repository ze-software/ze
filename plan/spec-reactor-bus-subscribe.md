# Spec: reactor-bus-subscribe — Reactor Bus Subscription

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2025-03-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `pkg/ze/bus.go` — Bus, Consumer, Event, Subscription interfaces
4. `internal/component/bus/bus.go` — Bus implementation (Subscribe, worker, deliveryLoop)
5. `internal/component/bgp/reactor/reactor.go` — Reactor struct, SetBus, publishBusNotification
6. `internal/component/bgp/subsystem/subsystem.go` — BGPSubsystem adapter (Start creates topics)

## Task

Make the BGP reactor a **bidirectional Bus participant** — it can already publish, but cannot subscribe to or receive Bus events. This unblocks `spec-iface-bus`, which needs BGP to react to `interface/addr/added` and `interface/addr/removed` events.

The change is small and surgical: implement `ze.Consumer` on the reactor, subscribe during startup, and dispatch received events to registered handlers via channel + worker (consistent with `rules/goroutine-lifecycle.md`).

### Scope

| In scope | Out of scope |
|----------|-------------|
| Reactor implements `ze.Consumer` | Any specific event handler logic (that's `spec-iface-bus`) |
| Subscribe to configurable prefixes at startup | Interface plugin code |
| Channel + worker event dispatch | DHCP, mirroring, migration |
| Unsubscribe on Stop | Changes to Bus implementation |
| Handler registration API | Changes to EventDispatcher |

## Required Reading

### Architecture Docs
- [ ] `pkg/ze/bus.go` — Bus, Consumer, Event, Subscription interfaces
  → Constraint: Consumer has single method `Deliver(events []ze.Event) error`
  → Constraint: Bus.Subscribe takes prefix + filter + Consumer, returns Subscription
- [ ] `internal/component/bus/bus.go` — Bus creates per-consumer delivery goroutine
  → Constraint: Bus already manages one worker goroutine per Consumer — reactor must NOT create its own delivery goroutine for Bus events (that would double-buffer)
  → Decision: Reactor's Deliver() dispatches directly to handlers (Bus worker is the goroutine)
- [ ] `internal/component/bgp/reactor/reactor.go` — Reactor struct, SetBus, StartWithContext
  → Constraint: Bus field already exists (line 249), SetBus already exists (line 385)
  → Constraint: StartWithContext acquires mu, starts listeners, peers, signals — subscription goes here
- [ ] `internal/component/bgp/subsystem/subsystem.go` — BGPSubsystem.Start creates topics then calls reactor.StartWithContext
  → Constraint: Topics are created before reactor starts — subscription happens during StartWithContext, topics already exist

**Key insights:**
- Bus already creates a per-consumer worker goroutine with buffered channel (cap 64). The reactor's `Deliver()` method runs inside that goroutine — no need for the reactor to create another.
- Reactor holds `ze.Bus` already. It publishes via `publishBusNotification()`. Subscribing is the missing half.
- Handler dispatch should use a map of topic prefix → handler function, registered before startup.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/ze/bus.go` — Consumer interface: `Deliver(events []ze.Event) error`. Event has Topic, Payload, Metadata.
- [ ] `internal/component/bus/bus.go` — `Subscribe()` creates worker goroutine per consumer. `deliveryLoop()` drains channel, calls `Deliver()` in batches. Panic recovery via `safeDeliver()`.
- [ ] `internal/component/bgp/reactor/reactor.go` — `bus ze.Bus` field (line 249). `SetBus(b)` (line 385). `publishBusNotification()` (line 399). No Subscribe/Consumer/Deliver anywhere.
- [ ] `internal/component/bgp/subsystem/subsystem.go` — `Start()` creates 5 topics (`bgp/update`, `bgp/state`, `bgp/negotiated`, `bgp/eor`, `bgp/congestion`), stores Bus, calls `reactor.StartWithContext(ctx)`.

**Behavior to preserve:**
- `publishBusNotification()` — unchanged, publish path stays fire-and-forget
- Bus per-consumer worker goroutine — reactor does NOT create its own dispatch goroutine
- `SetBus()` called before `StartWithContext()` — subscription happens inside Start, not SetBus
- Reactor lock protocol in `StartWithContext()` — subscription must not deadlock

**Behavior to change:**
- Reactor does not implement `ze.Consumer` → it will
- Reactor does not call `bus.Subscribe()` → it will, during StartWithContext
- Reactor has no way to register event handlers → it will, via `OnBusEvent(prefix, handler)`
- Reactor does not unsubscribe on Stop → it will

## Data Flow (MANDATORY)

### Entry Point
- Bus delivers events to reactor's `Deliver()` method via the per-consumer worker goroutine
- Events arrive as `[]ze.Event` (batched by `drainBatch()` in `bus.go`)

### Transformation Path
1. **Bus worker** — drains channel, calls `reactor.Deliver(events)` with batch
2. **Deliver** — iterates events, for each: find matching handler(s) by topic prefix
3. **Handler** — registered function processes the event (e.g., future: start listener on addr/added)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Bus → Reactor | `ze.Consumer.Deliver()` called by Bus worker goroutine | [ ] |
| Reactor → Handlers | Direct function call within Deliver, same goroutine | [ ] |

### Integration Points
- `internal/component/bus/bus.go` — Bus.Subscribe, Bus.Unsubscribe (existing, unchanged)
- `internal/component/bgp/reactor/reactor.go` — Reactor struct gains Consumer impl + handler map
- `internal/component/bgp/subsystem/subsystem.go` — unchanged (topics already created before reactor starts)

### Architectural Verification
- [ ] No bypassed layers (events flow through Bus → Consumer.Deliver → handler)
- [ ] No unintended coupling (handlers registered by caller, reactor doesn't import handler packages)
- [ ] No duplicated functionality (uses Bus delivery goroutine, doesn't create another)
- [ ] Zero-copy preserved where applicable (Event.Payload is `[]byte`, passed by reference)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Bus.Publish(topic) | → | Reactor.Deliver() dispatches to handler | `TestReactorReceivesBusEvent` |
| Reactor.Stop() | → | Bus.Unsubscribe() called, no more deliveries | `TestReactorUnsubscribesOnStop` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Handler registered via `OnBusEvent("interface/", fn)` before Start | Handler stored in reactor, callable after subscription |
| AC-2 | Bus publishes event matching registered prefix | Reactor's Deliver receives event, matching handler called with event |
| AC-3 | Bus publishes event NOT matching any registered prefix | Event received by Deliver, no handler called, no error |
| AC-4 | Multiple handlers registered for overlapping prefixes | All matching handlers called for each event |
| AC-5 | Reactor.Stop() called | All subscriptions unsubscribed, no further deliveries |
| AC-6 | No handlers registered, no subscriptions | Reactor starts and stops cleanly with no Bus interaction beyond existing publish |
| AC-7 | Handler registered after Start | Returns error — handlers must be registered before Start |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReactorReceivesBusEvent` | `internal/component/bgp/reactor/reactor_bus_test.go` | AC-2: event published → handler called with correct topic/payload/metadata | |
| `TestReactorNoMatchingHandler` | `internal/component/bgp/reactor/reactor_bus_test.go` | AC-3: unmatched event → no handler called, no error | |
| `TestReactorMultipleHandlers` | `internal/component/bgp/reactor/reactor_bus_test.go` | AC-4: overlapping prefixes → all matching handlers called | |
| `TestReactorUnsubscribesOnStop` | `internal/component/bgp/reactor/reactor_bus_test.go` | AC-5: after Stop, no more deliveries | |
| `TestReactorNoHandlersNoop` | `internal/component/bgp/reactor/reactor_bus_test.go` | AC-6: no handlers → no subscription, clean start/stop | |
| `TestReactorOnBusEventAfterStart` | `internal/component/bgp/reactor/reactor_bus_test.go` | AC-7: registering after Start returns error | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no numeric inputs in this spec.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-reactor-bus-subscribe` | `test/plugin/reactor-bus-subscribe.ci` | Config starts ze, external Bus publish → reactor handler fires | |

### Future (if deferring any tests)
- Concurrent handler registration stress test — defer to chaos framework

## Files to Modify

- `internal/component/bgp/reactor/reactor.go` — add `busHandlers`, `busSubs` fields, `OnBusEvent()` method, subscription in `StartWithContext`, unsubscription in `stopInternal`/`monitor`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | — |
| RPC count in architecture docs | No | — |
| CLI commands/flags | No | — |
| CLI usage/help text | No | — |
| API commands doc | No | — |
| Plugin SDK docs | No | — |
| Editor autocomplete | No | — |
| Functional test for new RPC/API | No | — |

## Files to Create

- `internal/component/bgp/reactor/reactor_bus.go` — `ze.Consumer` implementation, `OnBusEvent`, handler dispatch
- `internal/component/bgp/reactor/reactor_bus_test.go` — all unit tests
- `test/plugin/reactor-bus-subscribe.ci` — functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Consumer + handler registry** — implement `ze.Consumer` on reactor, handler map, `OnBusEvent()`
   - Tests: `TestReactorReceivesBusEvent`, `TestReactorNoMatchingHandler`, `TestReactorMultipleHandlers`, `TestReactorOnBusEventAfterStart`, `TestReactorNoHandlersNoop`
   - Files: `reactor_bus.go`, `reactor.go` (add fields)
   - Verify: tests fail → implement → tests pass

2. **Phase: Subscribe/Unsubscribe lifecycle** — subscribe during StartWithContext, unsubscribe during Stop
   - Tests: `TestReactorUnsubscribesOnStop`
   - Files: `reactor.go` (StartWithContext, stop path)
   - Verify: tests fail → implement → tests pass

3. **Phase: Functional test** — `.ci` test proving end-to-end subscription
   - Tests: `test/plugin/reactor-bus-subscribe.ci`
   - Files: test file
   - Verify: functional test passes

4. **Full verification** → `make ze-verify` (lint + all ze tests except fuzz)

5. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Deliver does not hold reactor.mu (would deadlock with publishBusNotification or Stop) |
| Naming | `OnBusEvent` follows existing `Set*` pattern for pre-Start configuration |
| Data flow | Events flow Bus worker → Deliver → handler. No extra goroutine created. |
| Rule: goroutine-lifecycle | No new goroutine created — Bus worker goroutine is the delivery context |
| Rule: no-layering | No wrapper around Bus.Subscribe — direct call |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `reactor_bus.go` exists | `ls internal/component/bgp/reactor/reactor_bus.go` |
| `reactor_bus_test.go` exists | `ls internal/component/bgp/reactor/reactor_bus_test.go` |
| Reactor implements `ze.Consumer` | `grep 'func.*Reactor.*Deliver' internal/component/bgp/reactor/reactor_bus.go` |
| `OnBusEvent` method exists | `grep 'func.*Reactor.*OnBusEvent' internal/component/bgp/reactor/reactor_bus.go` |
| Subscribe called in Start | `grep 'Subscribe' internal/component/bgp/reactor/reactor.go` |
| Unsubscribe called in Stop | `grep 'Unsubscribe' internal/component/bgp/reactor/reactor.go` |
| Functional test exists | `ls test/plugin/reactor-bus-subscribe.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Deliver receives arbitrary events — handler prefix matching must not panic on nil Metadata |
| Resource exhaustion | Handler map is fixed at startup (AC-7) — no unbounded growth |
| Concurrency | Deliver called from Bus worker goroutine. Handlers must not hold reactor.mu to avoid deadlock with publish path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

### Why no extra goroutine in reactor

The Bus already creates a per-consumer worker goroutine (`deliveryLoop` in `bus.go`). Adding a channel + goroutine in the reactor would double-buffer events. Instead, `Deliver()` runs directly in the Bus worker goroutine and calls handlers synchronously. Handlers that need async processing can enqueue work themselves.

### Why handlers must be registered before Start

Handlers are stored in a slice that is read during `Deliver()` (called from Bus worker goroutine). Allowing registration after Start would require synchronization on every Deliver call. Since all consumers of Bus events are known at compile time (interface plugin, future subsystems), pre-Start registration is sufficient and avoids runtime locking overhead.

### Why OnBusEvent takes a prefix, not exact topic

Bus subscriptions are prefix-based. A handler for `"interface/"` receives both `"interface/addr/added"` and `"interface/addr/removed"`. This matches how the Bus works and how `spec-iface-bus` will use it (BGP subscribes to `"interface/"` prefix).

### Handler function signature

`func(ze.Event)` — simple, matches Bus Event type. The handler owns decoding Payload (Bus is content-agnostic). No error return — handlers log errors internally, consistent with fire-and-forget Bus design.

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

N/A — no protocol changes.

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
