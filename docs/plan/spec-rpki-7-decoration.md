# Spec: rpki-7-decoration

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-rpki-5-wiring |
| Phase | - |
| Updated | 2026-03-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/server/subscribe.go` - Subscription type, ParseSubscription, event types
4. `internal/component/bgp/server/events.go` - onMessageReceived, event delivery
5. `pkg/plugin/sdk/sdk_engine.go` - SDK methods available to plugins
6. `internal/component/bgp/plugins/rpki/rpki.go` - plugin entry point, validation worker

## Task

Enable plugins to receive BGP UPDATE events enriched with RPKI validation state. The design uses three independent layers:

1. **rpki plugin** emits lightweight per-prefix validation events (new `rpki` event type)
2. **Engine** routes rpki events through normal subscription/delivery (no decoration machinery)
3. **Union helper** (SDK library) correlates UPDATE + rpki events by message ID and delivers joined pairs to the consumer's handler

The consumer wires up the union helper before their business logic. The engine gains no new complexity. The rpki plugin gains one new responsibility (emit validation events). The union helper is a general-purpose building block reusable for any future correlated event pairs.

**Parent spec:** `spec-rpki-0-umbrella.md`

**Design evolution:** Earlier iteration explored engine-managed "decoration chains" where the engine withholds delivery, routes through decorators, and re-emits enriched events. That design was rejected due to: engine complexity (pending state, timeouts, chain tracking), backpressure coupling (UPDATE delivery blocked by rpki round-trip), and performance cost (full JSON rebuild per UPDATE). The separate-events + union-helper approach eliminates all of these.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - event delivery, parallel UPDATE dispatch
  -> Constraint: UPDATEs delivered in parallel; events carry message ID for correlation
  -> Decision: rpki events are a new event type in the bgp namespace, delivered through same pipeline
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage protocol, subscriptions
  -> Constraint: Subscriptions registered in Stage 5 or at runtime
  -> Decision: rpki event type added to ValidBgpEvents

### RFC Summaries
- [ ] `rfc/short/rfc6811.md` - validation states (Valid, Invalid, NotFound)
  -> Constraint: Validation is per-prefix; single UPDATE may have mixed states

**Key insights:**
- Events already carry `message.id` (uint64) -- this is the natural correlation key
- The event delivery pipeline is format-agnostic; adding a new event type requires minimal changes
- rpki plugin already subscribes to UPDATEs and validates each prefix -- it just needs to emit the result
- The union helper is pure library code with no engine dependencies

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/subscribe.go` - Subscription struct, ParseSubscription, GetMatching
- [ ] `internal/component/bgp/server/events.go` - onMessageReceived, format cache, message ID on events
- [ ] `internal/component/plugin/events.go` - ValidBgpEvents map, event type constants
- [ ] `pkg/plugin/rpc/types.go` - SubscribeEventsInput, EmitEventInput (to be created)
- [ ] `pkg/plugin/sdk/sdk_engine.go` - SDK methods: UpdateRoute, DispatchCommand, SubscribeEvents
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - OnEvent callback, event parsing
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - validation worker, ROA cache lookup

**Behavior to preserve:**
- All existing UPDATE delivery (parallel, unchanged, no latency impact)
- rpki plugin's existing validation-gate flow (accept/reject to adj-rib-in) unchanged
- All existing subscription parsing, matching, and event types
- DirectBridge structured delivery for internal plugins
- CLI monitor delivery

**Behavior to change:**
- New `rpki` event type in bgp namespace (ValidBgpEvents)
- rpki plugin emits validation events after validating each UPDATE
- New SDK method for plugins to emit events
- New RPC: `ze-plugin-engine:emit-event` for plugins to push events into the engine
- New union helper in SDK for correlating two event streams

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE received from peer (wire bytes via reactor) -- unchanged
- rpki validation result produced by rpki plugin after UPDATE processing -- new event source

### Transformation Path

| Step | Actor | Action |
|------|-------|--------|
| 1 | Engine | UPDATE arrives, delivered to ALL subscribers in parallel (unchanged) |
| 2 | rpki plugin | Receives UPDATE, extracts origin AS, validates each prefix against ROA cache |
| 3 | rpki plugin | Builds lightweight rpki event JSON (message ID + per-prefix states) |
| 4 | rpki plugin | Calls `emit-event` RPC to push rpki event into engine |
| 5 | Engine | Receives rpki event, matches against `rpki` event subscribers |
| 6 | Engine | Delivers rpki event to subscribers (same pipeline as any event) |
| 7 | Consumer | Union helper receives both UPDATE and rpki event |
| 8 | Consumer | Union helper correlates by message ID, calls consumer's handler with joined pair |

**Key property:** Steps 1 and 2-6 are independent. UPDATE delivery is never blocked by rpki. The rpki event arrives asynchronously after the UPDATE.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> rpki (UPDATE) | Normal event delivery (already exists) | [ ] |
| rpki -> Engine (rpki event) | New RPC: `emit-event` | [ ] |
| Engine -> consumer (UPDATE) | Normal event delivery (unchanged) | [ ] |
| Engine -> consumer (rpki event) | Normal event delivery (new event type) | [ ] |
| UPDATE + rpki -> consumer handler | Union helper correlates in SDK | [ ] |

### Integration Points
- `ValidBgpEvents` map: add `"rpki"` event type
- New RPC `ze-plugin-engine:emit-event`: plugin pushes an event into engine's delivery pipeline
- rpki plugin: emit rpki events after validation
- SDK: `EmitEvent` method, `Union` helper type
- Consumer plugin: uses union helper to join streams

### Architectural Verification
- [ ] No bypassed layers -- rpki events flow through normal delivery pipeline
- [ ] No unintended coupling -- engine knows "rpki" as an event type string, not rpki internals
- [ ] No duplicated functionality -- rpki reuses existing validation; union helper is new library code
- [ ] Zero-copy preserved -- UPDATE delivery path completely unchanged

## Design Decisions

### D1: Separate events, not enriched events

rpki emits a standalone validation event rather than rebuilding the UPDATE with extra fields injected. This means: no JSON cloning/modification in the hot path, no engine withholding, no round-trip blocking UPDATE delivery.

### D2: Union helper in SDK, not in engine

Correlation is a consumer concern, not an engine concern. The union helper is a composable building block in the SDK that any plugin can use. The engine stays simple (just event routing).

### D3: Consumer wires union before their handler

The union helper sits between event reception and business logic. The consumer creates a union, registers it as the event handler, and provides a callback that receives the joined (UPDATE, rpki) pair. No magic -- explicit wiring.

### D4: rpki event uses same message ID as UPDATE

Events already carry `message.id` (uint64). The rpki event references the same ID. This is the correlation key. No new ID scheme needed.

### D5: rpki event is lightweight

The rpki event contains only: message ID, peer address, and per-prefix validation states. It does not repeat the UPDATE's attributes, NLRIs, or raw bytes. This minimizes serialization cost and bandwidth.

### D6: No engine withholding

UPDATEs are delivered immediately to all subscribers. If a consumer wants to wait for rpki data before acting, the union helper buffers. If the consumer doesn't care about rpki, it just subscribes to UPDATEs and ignores rpki events. Zero overhead for non-rpki consumers.

### D7: Lazy emission

rpki plugin only emits validation events if any subscriber has subscribed to the `rpki` event type. The engine tracks subscriber count per event type (it already does this for GetMatching). rpki checks whether it has rpki-event subscribers before doing the emit work.

### D8: Missing rpki plugin is a subscription error

If a consumer subscribes to `rpki` events but the rpki plugin is not loaded (nobody will emit rpki events), the subscription is accepted but the consumer will never receive rpki events. This is the same behavior as subscribing to any event type when no producer exists. The union helper handles this via its timeout: after the timeout, it delivers the UPDATE without rpki data. Alternatively, config validation can warn if rpki events are subscribed but no rpki plugin is loaded.

### D9: Withdrawal UPDATEs

rpki plugin emits rpki events for withdrawal UPDATEs too, but with no per-prefix states (empty rpki section). This lets the union helper complete the correlation immediately without waiting for a timeout.

### D10: Per-prefix result cache

Validation results cached per `(prefix, originAS, roaSerial)`. Cache cleared when ROA serial changes (RTR End-of-Data). Avoids redundant trie walks for repeated prefixes across peers.

### D11: Unavailable state

If ROA cache is empty/expired, rpki events carry `"rpki": "unavailable"` (string instead of object). Consumer can distinguish "not validated yet" (no rpki event received) from "cannot validate" (unavailable).

## RPKI Validation Event Format

### Event type

`rpki` -- added to `ValidBgpEvents` in `internal/component/plugin/events.go`

### JSON structure (announce with results)

| JSON path | Type | Description |
|-----------|------|-------------|
| `type` | string | `"bgp"` (same envelope as all bgp events) |
| `bgp.message.id` | uint64 | Message ID of the UPDATE being validated |
| `bgp.message.type` | string | `"rpki"` |
| `bgp.peer.address` | string | Source peer of the UPDATE |
| `bgp.peer.asn` | uint32 | Source peer ASN |
| `bgp.rpki` | object | Per-family, per-prefix validation states |
| `bgp.rpki.<family>` | object | Keys are prefix strings |
| `bgp.rpki.<family>.<prefix>` | string | `"valid"`, `"invalid"`, `"not-found"` |

### JSON structure (unavailable)

| JSON path | Type | Description |
|-----------|------|-------------|
| `bgp.rpki` | string | `"unavailable"` |

### JSON structure (withdrawal)

| JSON path | Type | Description |
|-----------|------|-------------|
| `bgp.rpki` | object | Empty object (no prefixes to validate) |

## Emit Event RPC

### New RPC: `ze-plugin-engine:emit-event`

A general-purpose mechanism for plugins to push events into the engine's delivery pipeline. Not rpki-specific -- any plugin can emit any event type.

| Field | Type | JSON key | Description |
|-------|------|----------|-------------|
| Namespace | string | `"namespace"` | Event namespace (`"bgp"`) |
| EventType | string | `"event-type"` | Event type (`"rpki"`) |
| Direction | string | `"direction"` | Direction for subscriber matching (`"received"`) |
| PeerAddress | string | `"peer-address"` | Peer address for subscriber matching |
| Event | string | `"event"` | Full JSON event string |

### Output

| Field | Type | JSON key | Description |
|-------|------|----------|-------------|
| Status | string | `"status"` | `"done"` or `"error"` |
| Delivered | int | `"delivered"` | Number of subscribers that received the event |

### Engine behavior

| Step | Action |
|------|--------|
| 1 | Validate namespace + event type |
| 2 | GetMatching for (namespace, eventType, direction, peerAddress) |
| 3 | Deliver event string to matched subscribers via normal delivery pipeline |
| 4 | Return count of deliveries |

No pending state. No chains. No timeouts in the engine. Just: receive event, find subscribers, deliver.

## Union Helper (SDK)

### Purpose

Correlates two asynchronous event streams by message ID. The consumer creates a union, subscribes to both event types, and provides a handler that receives joined pairs.

### API

| Method | Description |
|--------|-------------|
| `NewUnion(primary, secondary string, timeout, handler)` | Create union for two event types |
| `union.OnEvent(event)` | Feed an event into the union (called from SDK's event dispatch) |
| `handler(primary, secondary)` | Consumer's callback; secondary may be nil on timeout |

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| primary | string | Primary event type (e.g., `"update"`) |
| secondary | string | Secondary event type (e.g., `"rpki"`) |
| timeout | duration | Max time to wait for secondary after primary arrives |
| handler | function | Called with (primary event, secondary event or nil) |

### Behavior

| Scenario | Union action |
|----------|-------------|
| Primary arrives first | Buffer primary, wait for secondary up to timeout |
| Secondary arrives first | Buffer secondary, wait for primary up to timeout |
| Both arrive | Call handler immediately with both |
| Timeout (secondary missing) | Call handler with (primary, nil) |
| Timeout (primary missing) | Discard secondary (orphan) |

### Correlation key

Message ID (`bgp.message.id`) + peer address (`bgp.peer.address`). Both are present in all bgp events.

### Memory management

| Concern | Mitigation |
|---------|------------|
| Unbounded buffer growth | Max pending entries (configurable, default 10,000) |
| Stale entries | Timeout sweep runs every second; expired entries delivered with nil secondary |
| Peer disconnect | Consumer can call `union.FlushPeer(addr)` to deliver all pending for a peer |

### Consumer wiring

The consumer plugin wires the union in its startup:

| Step | What happens |
|------|-------------|
| 1 | Consumer creates `union := sdk.NewUnion("update", "rpki", 5*time.Second, myHandler)` |
| 2 | Consumer subscribes to both: `update direction received` and `rpki direction received` |
| 3 | Consumer sets `OnEvent` to call `union.OnEvent(event)` |
| 4 | Union dispatches to `myHandler(update, rpkiEvent)` when both arrive or timeout |
| 5 | `myHandler` has the UPDATE + per-prefix RPKI states (or nil if unavailable/timeout) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| rpki plugin receives UPDATE, emits rpki event | -> | Engine delivers rpki event to subscriber | `test/plugin/rpki-event-valid.ci` |
| rpki plugin receives UPDATE with mixed prefixes | -> | rpki event has per-prefix states | `test/plugin/rpki-event-multi.ci` |
| rpki plugin has no ROA cache | -> | rpki event has "unavailable" | `test/plugin/rpki-event-unavailable.ci` |
| Consumer subscribes to update + rpki, uses union | -> | Consumer receives correlated pair | `test/plugin/rpki-union-join.ci` |
| Consumer subscribes to update only, no rpki sub | -> | UPDATE delivered immediately, no rpki overhead | `test/plugin/rpki-union-passthrough.ci` |
| rpki plugin not loaded, consumer subscribes to rpki | -> | Consumer receives UPDATEs, rpki events timeout, handler called with nil | `test/plugin/rpki-union-timeout.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | rpki plugin receives UPDATE with valid prefix | rpki event emitted with `"valid"` for that prefix |
| AC-2 | rpki plugin receives UPDATE with invalid prefix | rpki event emitted with `"invalid"` for that prefix |
| AC-3 | rpki plugin receives UPDATE with 3 prefixes (valid, invalid, not-found) | rpki event has all three states independently |
| AC-4 | Consumer subscribes to update only | UPDATE delivered immediately, no rpki event overhead |
| AC-5 | Consumer subscribes to rpki events, rpki plugin not loaded | No rpki events arrive; union times out, handler called with nil secondary |
| AC-6 | rpki plugin has empty ROA cache | rpki event emitted with `"unavailable"` |
| AC-7 | Withdrawal UPDATE | rpki event emitted with empty rpki section |
| AC-8 | No subscriber for rpki events | rpki plugin skips emit work (lazy) |
| AC-9 | Union helper receives both events | Handler called with (update, rpki) pair |
| AC-10 | Union helper: rpki event never arrives | Handler called with (update, nil) after timeout |
| AC-11 | Same prefix from two peers | Each gets independent rpki event with correct message ID |
| AC-12 | ROA cache changes mid-session | Next rpki event uses fresh validation result |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEmitEventRPC` | `internal/component/plugin/server/dispatch_test.go` | Engine receives emitted event, delivers to subscribers | |
| `TestEmitEventNoSubscribers` | `internal/component/plugin/server/dispatch_test.go` | Emit with no subscribers returns delivered=0 | |
| `TestEmitEventValidation` | `internal/component/plugin/server/dispatch_test.go` | Invalid namespace/event type rejected | |
| `TestRPKIEventBuild` | `internal/component/bgp/plugins/rpki/rpki_test.go` | Builds correct rpki event JSON from validation results | |
| `TestRPKIEventUnavailable` | `internal/component/bgp/plugins/rpki/rpki_test.go` | Empty cache produces "unavailable" event | |
| `TestRPKIEventWithdrawal` | `internal/component/bgp/plugins/rpki/rpki_test.go` | Withdrawal produces empty rpki section | |
| `TestValidationResultCache` | `internal/component/bgp/plugins/rpki/rpki_test.go` | Cache hit/miss/invalidation on serial change | |
| `TestUnionBothArrive` | `pkg/plugin/sdk/union_test.go` | Both events arrive, handler called with pair | |
| `TestUnionPrimaryFirst` | `pkg/plugin/sdk/union_test.go` | Primary arrives first, secondary follows, handler called | |
| `TestUnionSecondaryFirst` | `pkg/plugin/sdk/union_test.go` | Secondary arrives first, primary follows, handler called | |
| `TestUnionTimeout` | `pkg/plugin/sdk/union_test.go` | Secondary never arrives, handler called with nil after timeout | |
| `TestUnionFlushPeer` | `pkg/plugin/sdk/union_test.go` | FlushPeer delivers all pending for that peer | |
| `TestUnionMaxPending` | `pkg/plugin/sdk/union_test.go` | Oldest entries evicted when max pending reached | |
| `TestUnionCorrelationKey` | `pkg/plugin/sdk/union_test.go` | Same message ID + different peers are separate entries | |
| `TestValidBgpEventsIncludesRPKI` | `internal/component/plugin/events_test.go` | "rpki" is a valid bgp event type | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Union timeout | 1ms-60s | 60s | 0 (rejected) | N/A (clamped) |
| Union max pending | 1-1000000 | 1000000 | 0 (rejected) | N/A (clamped) |
| Message ID | 0-max uint64 | max uint64 | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-event-valid` | `test/plugin/rpki-event-valid.ci` | rpki plugin emits validation event for valid prefix | |
| `rpki-event-multi` | `test/plugin/rpki-event-multi.ci` | Three prefixes, independent states in rpki event | |
| `rpki-event-unavailable` | `test/plugin/rpki-event-unavailable.ci` | No RTR server, rpki event shows "unavailable" | |
| `rpki-union-join` | `test/plugin/rpki-union-join.ci` | Consumer receives correlated UPDATE + rpki pair | |
| `rpki-union-passthrough` | `test/plugin/rpki-union-passthrough.ci` | No rpki subscription, UPDATE delivered immediately | |
| `rpki-union-timeout` | `test/plugin/rpki-union-timeout.ci` | rpki not loaded, union times out, handler gets nil | |

### Future (if deferring any tests)
- Multiple union pairs (e.g., update+rpki AND update+flowspec) -- deferred to when second event source exists
- Performance benchmark: union overhead per UPDATE

## Files to Modify

- `internal/component/plugin/events.go` - Add `EventRPKI = "rpki"` constant, add to `ValidBgpEvents`
- `internal/component/plugin/server/dispatch.go` - Handle `emit-event` RPC
- `pkg/plugin/rpc/types.go` - Add `EmitEventInput`, `EmitEventOutput` types
- `pkg/plugin/sdk/sdk_engine.go` - Add `EmitEvent` SDK method
- `internal/component/bgp/plugins/rpki/rpki.go` - Emit rpki events after validation

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `ze-plugin-engine.yang` (emit-event) |
| RPC count in architecture docs | Yes | `docs/architecture/api/architecture.md` |
| CLI commands/flags | No | No new CLI commands |
| CLI usage/help text | No | |
| API commands doc | Yes | `docs/architecture/api/commands.md` |
| Plugin SDK docs | Yes | `.claude/rules/plugin-design.md` SDK tables |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/plugin/rpki-event-*.ci`, `rpki-union-*.ci` |

## Files to Create

- `pkg/plugin/sdk/union.go` - Union helper: correlates two event streams by message ID
- `pkg/plugin/sdk/union_test.go` - Unit tests for union helper
- `internal/component/bgp/plugins/rpki/emit.go` - rpki event emission: build JSON, call EmitEvent
- `internal/component/bgp/plugins/rpki/emit_test.go` - Unit tests for rpki event building
- `test/plugin/rpki-event-valid.ci` - Functional test: rpki event emission
- `test/plugin/rpki-event-multi.ci` - Functional test: multi-prefix rpki event
- `test/plugin/rpki-event-unavailable.ci` - Functional test: unavailable state
- `test/plugin/rpki-union-join.ci` - Functional test: union correlation
- `test/plugin/rpki-union-passthrough.ci` - Functional test: no rpki overhead
- `test/plugin/rpki-union-timeout.ci` - Functional test: timeout handling

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
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

1. **Phase: Event type and emit RPC** -- Add `rpki` event type to ValidBgpEvents. Add `EmitEventInput/Output` RPC types. Add `EmitEvent` SDK method. Implement `emit-event` handler in engine dispatch: validate, GetMatching, deliver.
   - Tests: `TestValidBgpEventsIncludesRPKI`, `TestEmitEventRPC`, `TestEmitEventNoSubscribers`, `TestEmitEventValidation`
   - Files: `events.go`, `dispatch.go`, `rpc/types.go`, `sdk/sdk_engine.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Union helper** -- SDK library: NewUnion, OnEvent, timeout sweep, FlushPeer, max pending eviction. Pure library code, no engine dependencies.
   - Tests: `TestUnionBothArrive`, `TestUnionPrimaryFirst`, `TestUnionSecondaryFirst`, `TestUnionTimeout`, `TestUnionFlushPeer`, `TestUnionMaxPending`, `TestUnionCorrelationKey`
   - Files: `sdk/union.go`, `sdk/union_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: rpki event emission** -- rpki plugin builds rpki event JSON after validation. Calls EmitEvent. Per-prefix result cache. Lazy gate (skip if no rpki subscribers). Handles withdrawal, unavailable.
   - Tests: `TestRPKIEventBuild`, `TestRPKIEventUnavailable`, `TestRPKIEventWithdrawal`, `TestValidationResultCache`
   - Files: `rpki/emit.go`, `rpki/emit_test.go`, `rpki/rpki.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: YANG schema** -- Add emit-event RPC to `ze-plugin-engine.yang`
   - Tests: YANG compilation via lint
   - Files: YANG schema files
   - Verify: `make ze-lint` passes

5. **Phase: Functional tests** -- Create all .ci test files
   - Tests: all functional tests from TDD Plan
   - Files: `test/plugin/rpki-event-*.ci`, `test/plugin/rpki-union-*.ci`
   - Verify: `make ze-functional-test` passes

6. **Phase: Full verification** -- `make ze-verify`

7. **Phase: Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | rpki events carry correct message ID and per-prefix states; union correlates correctly |
| Naming | JSON keys use kebab-case; event type is lowercase `"rpki"` |
| Data flow | UPDATE delivered immediately; rpki event follows asynchronously; union joins them |
| Rule: no-layering | Engine has no rpki-specific code; emit-event is generic |
| Rule: goroutine-lifecycle | No per-event goroutines; rpki uses existing worker; union uses single sweep timer |
| Rule: design-principles | Union helper is lazy (no work if unused), explicit (consumer wires it up), minimal coupling |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `rpki` event type registered | grep `EventRPKI` in events.go |
| `EmitEventInput` type exists | grep in rpc/types.go |
| `EmitEvent` SDK method exists | grep in sdk/sdk_engine.go |
| `union.go` SDK file exists | ls pkg/plugin/sdk/union.go |
| rpki emit.go exists | ls internal/component/bgp/plugins/rpki/emit.go |
| 6 functional tests exist | ls test/plugin/rpki-event-*.ci test/plugin/rpki-union-*.ci |
| YANG RPC registered | grep emit-event in .yang files |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | emit-event: validate namespace and event type are valid; reject unknown types |
| Resource exhaustion | Union max-pending cap prevents unbounded memory; timeout sweep prevents stale entries |
| Event integrity | Emitted events must include valid peer address; engine does not trust plugin-provided peer without validation |
| Abuse prevention | Rate-limit emit-event per plugin to prevent event flooding |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Engine-managed decoration chains | Too complex: pending state, timeouts, chain tracking, backpressure coupling, JSON rebuild per UPDATE | Separate events + union helper |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- Engine-managed decoration chains are an attractive abstraction but create tight coupling between UPDATE delivery and decorator latency. Keeping events separate and correlating at the consumer is simpler, faster, and more composable.
- The emit-event RPC is generic (not rpki-specific). Any plugin can emit any event type. This opens the door for other enrichment sources without engine changes.

## RFC Documentation

No new RFC constraints for the event/union mechanism. RPKI validation (RFC 6811) constraints documented in spec-rpki-0-umbrella.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `pkg/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-rpki-7-decoration.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
