# Spec: l2tp-7c -- L2TP route-change events for redistribute

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-bgp-redistribute, spec-l2tp-7-subsystem |
| Phase | - |
| Updated | 2026-04-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-bgp-redistribute.md` -- consumer plugin; the thing that actually produces UPDATEs
3. `plan/spec-l2tp-7-subsystem.md` + `plan/learned/620-l2tp-7-subsystem.md` -- parent
4. `plan/deferrals.md` 2026-04-17 row
5. `internal/component/l2tp/route_observer.go` -- current observer (state tracking only)
6. `internal/component/l2tp/redistribute.go` -- `l2tp` source registration
7. `pkg/ze/eventbus.go` + `internal/core/events/events.go` -- typed EventBus handles
8. `internal/plugins/sysrib/events/` -- `BestChangeBatch` / `BestChangeEntry` shape precedent

## Task

Publish L2TP subscriber route lifecycle on the EventBus as
`(l2tp, route-change)` batched events so the `bgp-redistribute`
plugin (spec-bgp-redistribute) can advertise subscriber `/32`
(IPv4) and `/128` (IPv6) prefixes to configured BGP peers.

L2TP is a **producer only** under this model. It has no BGP
knowledge, no evaluator lookup, no injector handoff. It emits
events on every IPCP / IPv6CP completion and every session
teardown, unconditionally. Redistribute gating lives entirely
in the consumer (bgp-redistribute) per the
subscription-filter model.

User-visible behaviour: operators configure
`redistribute { import l2tp { family ipv4/unicast ipv6/unicast; } }`
and BGP peers announce subscribers' /32s and /128s with
next-hop = the local session address to each peer (resolved by
the reactor's existing `resolveNextHop(Self, family)`). Without
the import rule, no routes are advertised.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp.md` -- subsystem overview
- [ ] `docs/architecture/core-design.md` -- EventBus + redistribute registry
- [ ] `plan/spec-bgp-redistribute.md` -- consumer contract (payload shape, event naming)

### RFC Summaries
- [ ] `rfc/short/rfc2661.md` -- L2TPv2 session lifecycle (already cited in l2tp-7)

**Key insights:** (filled during RESEARCH phase)

## Current Behavior (MANDATORY)

**Source files read:**
- `internal/component/l2tp/route_observer.go` -- `subscriberRouteObserver` tracks `records`, logs IPCP-up / session-down, bumps counters. No events emitted.
- `internal/component/l2tp/redistribute.go` -- registers `"l2tp"` source via `redistribute.RegisterSource`.
- `internal/component/l2tp/subsystem.go` -- `Start` constructs the observer and attaches to every reactor.
- `internal/component/l2tp/reactor.go` -- dispatches `EventSessionIPAssigned` / session-down into the observer.
- `internal/plugins/sysrib/events/` -- reference for event payload types; bgp-rib emits `BestChangeBatch`.

**Behavior to preserve:**
- Observer in-memory state tracking (`records` map), counters, `show l2tp statistics` output.
- Reactor dispatch points (`EventSessionIPAssigned`, session-down) -- no change.
- `l2tp` source registration.
- All existing `.ci` tests.

**Behavior to change:**
- Observer emits `(l2tp, route-change)` batched events:
  - On IPCP-up / IPv6CP-up: batch with one `{Action: add, Prefix: <addr>/32 or /128, ...}` entry, family-specific.
  - On session-down: batch with one `{Action: remove, ...}` entry per family the session had up.
- Observer gains an EventBus handle at construction (nil-tolerant: no bus -> events silently dropped, state still tracked).
- No evaluator consultation. No cross-component injector.

## Data Flow (MANDATORY)

### Entry Point
- Reactor dispatches `EventSessionIPAssigned` (IPCP / IPv6CP success) to `observer.OnSessionIPUp`.
- Reactor dispatches session teardown to `observer.OnSessionDown`.

### Transformation Path
1. Observer updates in-memory record (unchanged).
2. Observer builds a `RouteChangeBatch{Protocol:"l2tp", Family:<fam>, Entries:[{Action:add, Prefix:<addr>/N}]}`.
3. Observer emits via the typed handle `Event[RouteChangeBatch].Emit(bus, batch)`.
4. Subscribers receive asynchronously. This plugin does NOT wait, does NOT know who subscribed.
5. On session-down: observer consults its own `routeRecord` for per-family state, emits one remove-batch per previously-up family. Record is cleared after emit.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| L2TP observer -> EventBus | typed `Event[RouteChangeBatch]` handle | [ ] unit test asserts emission |
| EventBus -> bgp-redistribute | existing subscribe pattern | [ ] functional test (chain) |

### Integration Points
- Typed event handle for `(l2tp, route-change)` registered at package init in `internal/component/l2tp/events/` (new small package, follows `bgp-rib/events/` precedent).
- Consumer spec (`spec-bgp-redistribute`) iterates `redistribute.SourceNames()` and binds to each non-BGP handle; L2TP's handle is picked up automatically once registered.

### Architectural Verification
- [ ] L2TP does NOT import `bgp/plugins/redistribute` or any BGP package beyond shared core
- [ ] Event handle package has no upward dependency on consumers
- [ ] Observer remains nil-bus-tolerant so unit tests don't require a full bus
- [ ] No busy-wait / goroutine-per-event in observer (emit is a typed-handle method call)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IPCP completes on LNS with `redistribute { import l2tp }` configured | -> | observer emits event -> bgp-redistribute subscribes + dispatches announce -> reactor emits UPDATE | `test/plugin/redistribute-l2tp-announce.ci` |
| Same session, no `redistribute l2tp` import rule | -> | observer emits event -> bgp-redistribute filter rejects -> no UPDATE | `test/plugin/redistribute-l2tp-not-configured.ci` |
| Session torn down (CDN / StopCCN) with a previously announced route | -> | observer emits remove-batch -> bgp-redistribute dispatches withdraw -> reactor emits WITHDRAWN_ROUTES | `test/plugin/redistribute-l2tp-withdraw.ci` |
| Dual-stack subscriber (IPv4 + IPv6) | -> | two separate batches on up; two on down | folded into `redistribute-l2tp-announce.ci` |

Tests rely on the test harness's L2TP kernel-probe gate (`ze.l2tp.skip-kernel-probe`, introduced in spec-l2tp-7) to boot without CAP_NET_ADMIN, and on a test producer that drives synthetic session up/down events -- same scaffolding used by spec-l2tp-7b's deferred redistribute tests.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPCP completes (IPv4 assigned) with bus available | Observer emits one `(l2tp, route-change)` batch: `Protocol=l2tp, Family=ipv4/unicast, Entries=[{Action:add, Prefix:<addr>/32}]` |
| AC-2 | IPv6CP completes (IPv6 assigned) on the same session | Second separate batch: `Family=ipv6/unicast, Entries=[{Action:add, Prefix:<addr>/128}]` |
| AC-3 | Session teardown with both families up | Two remove-batches, one per family that was up; counters incremented |
| AC-4 | Session teardown with only IPv4 up | One remove-batch for ipv4/unicast; no IPv6 emission |
| AC-5 | EventBus nil (tests / partial subsystem init) | Observer records state, logs, does NOT panic, does NOT crash |
| AC-6 | With `redistribute { import l2tp { family ipv4/unicast; } }` config + configured BGP peer | Peer receives BGP UPDATE with `/32` NLRI + NEXT_HOP = local session addr + `origin=incomplete` + empty AS-path |
| AC-7 | Without any `redistribute l2tp` rule | No BGP UPDATE (event emitted but consumer filter rejects) |
| AC-8 | Two BGP peers with distinct local session addresses | Each peer's UPDATE carries its own NEXT_HOP |
| AC-9 | Session torn down after AC-6 | Peer receives WITHDRAWN_ROUTES for the /32 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestObserver_OnSessionIPUp_EmitsBatch_IPv4` | `internal/component/l2tp/route_observer_test.go` | IPCP-up emits correct batch shape | |
| `TestObserver_OnSessionIPUp_EmitsBatch_IPv6` | same | IPv6CP-up emits correct batch shape | |
| `TestObserver_OnSessionDown_EmitsRemoveBatches_PerFamily` | same | Down with v4+v6 up: two remove batches | |
| `TestObserver_OnSessionDown_NoEmission_IfNothingUp` | same | Down before any IP came up: no emission | |
| `TestObserver_NilBus_StillTracksState` | same | No bus: records updated, counters bumped, no panic | |
| `TestRouteChangeHandle_Registered` | `internal/component/l2tp/events/events_test.go` | Typed handle exists with expected namespace/type | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| n/a (no numeric surface) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-l2tp-announce` | `test/plugin/redistribute-l2tp-announce.ci` | Config with import + simulated session -> peer receives /32 UPDATE with self-nhop | |
| `redistribute-l2tp-not-configured` | `test/plugin/redistribute-l2tp-not-configured.ci` | Same session, no import rule -> peer receives nothing | |
| `redistribute-l2tp-withdraw` | `test/plugin/redistribute-l2tp-withdraw.ci` | Teardown after announce -> WITHDRAWN_ROUTES | |

### Future (if deferring any tests)
- None. Functional tests are mandatory per `rules/integration-completeness.md`. The fixtures used by l2tp-7b for session state apply here.

## Files to Modify
- `internal/component/l2tp/route_observer.go` -- store bus handle; emit batches; track per-family up/down for symmetric withdraw
- `internal/component/l2tp/subsystem.go` -- pass bus handle to observer constructor

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Typed EventBus handle | [x] | `internal/component/l2tp/events/events.go` (new) |
| Observer emits events | [x] | `internal/component/l2tp/route_observer.go` |
| Subsystem wires the bus | [x] | `internal/component/l2tp/subsystem.go` |
| Functional tests | [x] | `test/plugin/redistribute-l2tp-*.ci` |
| Consumer (bgp-redistribute) | [x] dep | spec-bgp-redistribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- `redistribute l2tp` now live |
| 2 | Config syntax changed? | [ ] no | -- |
| 3 | CLI command added/changed? | [ ] no | -- |
| 4 | API/RPC added/changed? | [ ] no | -- |
| 5 | Plugin added/changed? | [ ] no (producer is the subsystem, not a plugin) | -- |
| 6 | Has a user guide page? | [x] | `docs/guide/l2tp.md` -- replace "future work" note with live behaviour and an example redistribute config |
| 7 | Wire format changed? | [ ] no | -- |
| 8 | Plugin SDK/protocol changed? | [ ] no | -- |
| 9 | RFC behavior implemented? | [ ] no | -- |
| 10 | Test infrastructure changed? | [ ] no | -- |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- L2TP redistribute parity |
| 12 | Internal architecture changed? | [x] | `docs/architecture/l2tp.md` -- observer now emits route-change events |

## Files to Create
- `internal/component/l2tp/events/events.go` -- typed `Event[RouteChangeBatch]` handle for `(l2tp, route-change)`
- `internal/component/l2tp/events/events_test.go`
- `test/plugin/redistribute-l2tp-announce.ci`
- `test/plugin/redistribute-l2tp-not-configured.ci`
- `test/plugin/redistribute-l2tp-withdraw.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + spec-bgp-redistribute |
| 2. Audit | Files to Modify / Create |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` |
| 6-12 | Standard flow |

### Implementation Phases
1. **Phase: event handle** -- new `internal/component/l2tp/events/` package with typed handle; unit test handle registration.
2. **Phase: observer emission** -- add bus handle field to `subscriberRouteObserver`; emit add-batch on IPCP-up / IPv6CP-up; unit tests with fake bus.
3. **Phase: observer symmetric withdraw** -- per-family tracking in `routeRecord`; emit remove-batch on session-down only for families that were up; unit tests.
4. **Phase: subsystem wiring** -- `Subsystem.Start` passes its EventBus to `newSubscriberRouteObserver`; nil-tolerant path preserved.
5. **Phase: functional `.ci` tests** -- three tests above; reuse l2tp-7b scaffolding for simulated session events.
6. **Phase: docs + learned summary** -- update listed doc files; write `plan/learned/NNN-l2tp-7c.md`.
7. **Full verification** -- `make ze-verify-fast`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation + test |
| Correctness | Per-family emission symmetry: only withdraw what was announced |
| Naming | Handle namespace `"l2tp"`, event type `"route-change"` -- matches bgp-redistribute's expected binding |
| Data flow | Observer knows only about EventBus; no BGP types touched |
| Rule: no-layering | No direct import of bgp or bgp-redistribute from l2tp |
| Rule: integration-completeness | All three `.ci` tests present |
| Rule: sibling-audit | Every caller of `newSubscriberRouteObserver` updated to pass the bus handle |
| Rule: plugin-design | Typed handle follows EventBus conventions (compile-time type, handle-based subscribe) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Typed event handle | `ls internal/component/l2tp/events/events.go` |
| Observer emits on IP-up | `grep -n 'RouteChangeBatch\|Emit' internal/component/l2tp/route_observer.go` |
| Subsystem passes bus | `grep -n 'eventBus\|EventBus' internal/component/l2tp/subsystem.go` |
| No BGP import in l2tp | `grep -r 'internal/component/bgp' internal/component/l2tp/` returns nothing |
| `.ci` tests exist | `ls test/plugin/redistribute-l2tp-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Subscriber address validated via existing `addr.IsValid()` guard before emission |
| Bus nil-safety | Observer never dereferences a nil bus; guards all emits |
| Resource exhaustion | Emission bounded by session count (already bounded by `max-sessions`) |
| Concurrent reload | Bus handle stored once at construction; no reload-time swap needed |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Observer emits but consumer doesn't react | Check bgp-redistribute subscribed to this handle; check source.Protocol != "bgp" filter |
| Withdraw emitted without prior announce | Per-family flag wrong -- trace `routeRecord` state transitions |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| L2TP needs an injector handle into the BGP RIB | Routes are events; BGP subscribes. L2TP is protocol-neutral. | Design discussion | Dropped cross-component injector entirely |
| L2TP needs to consult redistribute evaluator | Filter runs in the consumer, not the producer | Design discussion | Observer stays simple |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| `PublishInjector` / `LookupInjector` service locator | Coupled L2TP startup ordering to BGP RIB availability | Event handle: L2TP emits unconditionally, consumer subscribes independently |
| Evaluator gate inside observer | Producer-side filtering leaks consumer policy into L2TP | Consumer-side filter in bgp-redistribute |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- L2TP's responsibility ends at "here is what I know". What anyone else (BGP, sysrib, future FIB, billing) does with the knowledge is their concern.
- Per-family emission is the only safe withdraw semantic: a dual-stack subscriber whose IPv4 was announced but whose IPv6 never completed needs exactly one remove-batch on teardown, not two.

## RFC Documentation

RFC 2661 (L2TPv2) session lifecycle triggers remain the same as in
spec-l2tp-7. No new RFC constraints introduced by this spec.

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
|   |          |         |          |        |

### Fixes applied
- (to be filled)

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
- [ ] AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
