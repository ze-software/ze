# Spec: rpki-decorator

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | 1/2 |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/events.go` - static ValidEvents map (Phase 1 target)
4. `internal/component/plugin/registry/registry.go` - Registration struct (Phase 1: add EventTypes field)
5. `internal/component/bgp/plugins/rpki/rpki.go` - rpki plugin (emits rpki events, Phase 2 dependency)
6. `pkg/plugin/sdk/union.go` - Union correlator (Phase 2: first production consumer)

## Task

Two-phase feature enabling plugins to register custom event types and enabling RPKI validation state to be merged with BGP UPDATE events before delivery to consumers.

**Phase 1: Dynamic event type registration.** Plugins declare event types they produce in their Registration. The engine adds them to ValidEvents at startup. Replaces the static `ValidBgpEvents` map with a seed set augmented by plugin registrations. Same pattern as family registration.

**Phase 2: `bgp-rpki-decorator` plugin.** A new plugin that subscribes to `update direction received` and `rpki` events, correlates them via the SDK Union helper, and emits `update-rpki` events. The `update-rpki` event type is registered by the decorator plugin's own YANG schema. Consumers subscribe to `update-rpki` to receive UPDATE events enriched with RPKI validation state. The decorator is automatically loaded when any consumer subscribes to `update-rpki`.

**Design context:** The project previously decided (plan/learned/394-rpki-7-decoration.md) that correlation is a consumer concern, not an engine concern. This spec formalizes that decision: the decorator is a consumer plugin that produces a new event type. The engine stays content-agnostic.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - event delivery, plugin dispatch
  -> Constraint: UPDATE events delivered in parallel to all subscribers
  -> Constraint: Engine is content-agnostic (bus never type-asserts)
  -> Decision: Decorator is a plugin, not an engine feature
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage startup, TopologicalTiers
  -> Constraint: Dependencies determine startup order (tier 0 first)
  -> Decision: Decorator declares Dependencies on bgp-rpki (rpki must emit before decorator correlates)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6811.md` - validation states (Valid, Invalid, NotFound)
  -> Constraint: Three states, locally computed

**Key insights:**
- ValidBgpEvents is a static Go map checked by emitEvent and subscribe-events. Must become dynamic.
- Registration struct has no EventTypes field. Families, commands, YANG are registered but not event types.
- Union helper exists in SDK, has 9 unit tests, zero production consumers.
- rpki plugin already emits standalone rpki events via EmitEvent. Decorator is the first subscriber.
- monitor plugin hardcodes valid event type names in an error message (line 236) -- must use dynamic list.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/events.go` - static ValidBgpEvents/ValidRibEvents/ValidEvents maps. EventRPKI added manually. emitEvent in dispatch.go validates against this.
- [ ] `internal/component/plugin/registry/registry.go` - Registration struct. Has Families, CapabilityCodes, ConfigRoots, Dependencies, Features. No EventTypes field.
- [ ] `internal/component/plugin/server/dispatch.go` - emitEvent validates event type: `plugin.ValidEvents[input.Namespace]`. Rejects unknown types.
- [ ] `internal/component/plugin/server/subscribe.go` - validateEventType checks ValidEvents. parseSubscriptionArgs checks ValidEvents for namespace.
- [ ] `internal/component/bgp/plugins/cmd/monitor/monitor.go` - hardcodes valid event types in error message (line 236). Must use ValidEventNames().
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - emits rpki events via EmitEvent RPC. Subscribes to "update direction received".
- [ ] `internal/component/bgp/plugins/rpki/emit.go` - buildRPKIEvent, buildRPKIEventUnavailable. Builds standalone rpki JSON.
- [ ] `pkg/plugin/sdk/union.go` - Union correlator. Correlates by peer + msgID. Handles timeout, eviction, flush.

**Behavior to preserve:**
- Existing event delivery (parallel UPDATEs, sequential state/EOR) unchanged
- All existing plugins continue to work without modification
- rpki plugin behavior unchanged (still emits standalone rpki events)
- Union API unchanged (add production consumer, don't modify the API)
- emitEvent validation still rejects truly unknown event types
- subscribe-events validation still rejects truly unknown event types

**Behavior to change:**
- ValidEvents becomes a seed set augmented by plugin registrations at startup
- Registration struct gains an EventTypes field for plugins to declare event types they produce
- monitor plugin uses dynamic ValidEventNames() instead of hardcoded string
- New plugin: bgp-rpki-decorator subscribes to update + rpki, emits update-rpki

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Phase 1: Dynamic Event Registration

#### Entry Point
- Plugin registration via `init()` in `register.go` files
- Engine startup calls `registry.All()` to collect all registrations

#### Transformation Path
1. Plugin registers with `EventTypes: []string{"update-rpki"}` in Registration
2. Engine startup iterates registrations, collects EventTypes
3. For each EventType, engine calls `plugin.RegisterEventType(namespace, eventType)`
4. RegisterEventType adds to ValidEvents map (must happen before any subscribe-events or emit-event)

#### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin registry -> ValidEvents | RegisterEventType at startup | [ ] |
| ValidEvents -> emitEvent validation | Map lookup (existing) | [ ] |
| ValidEvents -> subscribe-events validation | Map lookup (existing) | [ ] |

### Phase 2: Decorator Plugin

#### Entry Point
- BGP UPDATE event delivered to decorator plugin (parallel, same as any subscriber)
- rpki event emitted by rpki plugin, delivered to decorator plugin

#### Transformation Path
1. Decorator subscribes to `update direction received` and `rpki` events
2. UPDATE event arrives -- fed to Union as primary
3. rpki event arrives -- fed to Union as secondary
4. Union correlates by peer + message ID, calls merge handler
5. Merge handler injects rpki validation section into UPDATE JSON
6. Decorator emits merged event as `update-rpki` via EmitEvent
7. Engine delivers `update-rpki` to subscribers of that event type

#### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> Decorator (UPDATE) | Standard event delivery | [ ] |
| Engine -> Decorator (rpki) | Standard event delivery (via EmitEvent from rpki plugin) | [ ] |
| Decorator -> Engine (update-rpki) | EmitEvent RPC | [ ] |
| Engine -> Consumer (update-rpki) | Standard event delivery | [ ] |

### Integration Points
- `internal/component/plugin/events.go` - RegisterEventType function (new)
- `internal/component/plugin/registry/registry.go` - EventTypes field on Registration (new)
- `internal/component/plugin/server/dispatch.go` - emitEvent validation (unchanged, uses ValidEvents)
- `pkg/plugin/sdk/union.go` - Union correlator (first production consumer)
- `internal/component/bgp/plugins/rpki/rpki.go` - rpki plugin (unchanged, decorator subscribes to its events)

### Architectural Verification
- [ ] No bypassed layers -- decorator uses standard event delivery + EmitEvent
- [ ] No unintended coupling -- engine does not know about rpki or decorator internals
- [ ] No duplicated functionality -- decorator uses existing Union, existing EmitEvent
- [ ] Zero-copy not applicable -- JSON string events, not wire bytes

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin registers EventTypes in Registration | -> | EventType appears in ValidEvents | `test/plugin/rpki-decorator-register.ci` |
| Consumer subscribes to `update-rpki` | -> | Subscription accepted (not rejected as unknown) | `test/plugin/rpki-decorator-subscribe.ci` |
| UPDATE + rpki event emitted | -> | Decorator correlates and emits `update-rpki` | `test/plugin/rpki-decorator-merge.ci` |
| UPDATE with no rpki event (timeout) | -> | Decorator emits `update-rpki` with UPDATE only (no rpki section) | `test/plugin/rpki-decorator-timeout.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin registers with EventTypes: ["update-rpki"] | `update-rpki` is accepted by subscribe-events and emit-event validation |
| AC-2 | Plugin registers with EventTypes: ["update-rpki"] and no plugin registers that type | subscribe-events rejects `update-rpki` as unknown |
| AC-3 | Decorator receives UPDATE + rpki event for same peer/msgID | Emits `update-rpki` event containing UPDATE data + rpki validation section |
| AC-4 | Decorator receives UPDATE but rpki event times out | Emits `update-rpki` event containing UPDATE data without rpki section |
| AC-5 | Decorator receives rpki event but UPDATE times out | rpki event discarded (orphan secondary) |
| AC-6 | No consumer subscribes to `update-rpki` | Decorator plugin not loaded. Zero overhead. |
| AC-7 | rpki plugin not loaded, decorator loaded | Decorator receives UPDATEs only. All `update-rpki` events contain UPDATE data without rpki section. |
| AC-8 | monitor plugin lists valid event types | Dynamic list includes plugin-registered types |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterEventType` | `internal/component/plugin/events_test.go` | RegisterEventType adds to ValidEvents | |
| `TestRegisterEventTypeDuplicate` | `internal/component/plugin/events_test.go` | Duplicate registration is idempotent | |
| `TestRegisterEventTypeInvalidNamespace` | `internal/component/plugin/events_test.go` | Unknown namespace returns error | |
| `TestValidEventNamesIncludesRegistered` | `internal/component/plugin/events_test.go` | ValidEventNames includes dynamically registered types | |
| `TestDecoratorMergeEvent` | `internal/component/bgp/plugins/rpki_decorator/decorator_test.go` | UPDATE + rpki JSON merged correctly | |
| `TestDecoratorMergeEventUnavailable` | `internal/component/bgp/plugins/rpki_decorator/decorator_test.go` | UPDATE + unavailable rpki merged correctly | |
| `TestDecoratorTimeoutNoRPKI` | `internal/component/bgp/plugins/rpki_decorator/decorator_test.go` | UPDATE emitted as update-rpki without rpki section on timeout | |
| `TestDecoratorOrphanRPKI` | `internal/component/bgp/plugins/rpki_decorator/decorator_test.go` | Orphan rpki event discarded | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Union timeout | 100ms-60s | 60s | 50ms | 120s |
| Union maxPending | 1-100000 | 100000 | 0 | 100001 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-rpki-decorator-merge` | `test/plugin/rpki-decorator-merge.ci` | Config with rpki + decorator, UPDATE arrives, consumer receives update-rpki with validation state | |
| `test-rpki-decorator-timeout` | `test/plugin/rpki-decorator-timeout.ci` | Config with decorator but no RTR server, UPDATE arrives, consumer receives update-rpki without rpki section | |
| `test-rpki-decorator-register` | `test/plugin/rpki-decorator-register.ci` | Decorator registers update-rpki event type, consumer subscribes successfully | |

### Future (if deferring any tests)
- Container test with real stayrtr (deferred to spec-rpki-6-container-test)
- Multiple decorator composition (deferred -- no second decorator exists yet)

## Files to Modify

- `internal/component/plugin/events.go` - Add RegisterEventType function, keep seed constants
- `internal/component/plugin/events_test.go` - Tests for RegisterEventType
- `internal/component/plugin/registry/registry.go` - Add EventTypes field to Registration struct
- `internal/component/plugin/server/startup.go` - Call RegisterEventType for each plugin's EventTypes during startup
- `internal/component/bgp/plugins/cmd/monitor/monitor.go` - Replace hardcoded event type list with ValidEventNames()

## Files to Create

- `internal/component/bgp/plugins/rpki_decorator/decorator.go` - Decorator plugin: subscribe update + rpki, Union, emit update-rpki
- `internal/component/bgp/plugins/rpki_decorator/register.go` - Plugin registration with EventTypes and Dependencies
- `internal/component/bgp/plugins/rpki_decorator/merge.go` - JSON merge logic (inject rpki section into UPDATE JSON)
- `internal/component/bgp/plugins/rpki_decorator/merge_test.go` - Merge unit tests
- `internal/component/bgp/plugins/rpki_decorator/decorator_test.go` - Decorator unit tests
- `internal/component/bgp/plugins/rpki_decorator/schema/ze-rpki-decorator.yang` - YANG schema declaring update-rpki event type
- `internal/component/bgp/plugins/rpki_decorator/schema/register.go` - YANG schema registration
- `internal/component/bgp/plugins/rpki_decorator/schema/embed.go` - YANG embed
- `test/plugin/rpki-decorator-merge.ci` - Functional test: merged event delivered
- `test/plugin/rpki-decorator-timeout.ci` - Functional test: timeout graceful degradation
- `test/plugin/rpki-decorator-register.ci` - Functional test: dynamic event type registration

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new event type) | [x] Yes | `rpki_decorator/schema/ze-rpki-decorator.yang` |
| Plugin SDK docs | [x] Yes - document EventTypes field | `.claude/rules/plugin-design.md` |
| Plugin all.go blank import | [x] Yes | `internal/component/plugin/all/all.go` |
| Plugin count in tests | [x] Yes | Update TestAllPluginsRegistered expected count |
| Functional tests | [x] Yes | `test/plugin/rpki-decorator-*.ci` |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
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

1. **Phase 1a: RegisterEventType** -- Add RegisterEventType function to events.go
   - Tests: TestRegisterEventType, TestRegisterEventTypeDuplicate, TestRegisterEventTypeInvalidNamespace
   - Files: `internal/component/plugin/events.go`, `events_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase 1b: EventTypes in Registration** -- Add EventTypes field, wire into startup
   - Tests: TestValidEventNamesIncludesRegistered
   - Files: `registry/registry.go`, `server/startup.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase 1c: Fix monitor hardcoded list** -- Replace hardcoded string with ValidEventNames()
   - Tests: TestValidEventNamesIncludesRegistered (AC-8)
   - Files: `cmd/monitor/monitor.go`
   - Verify: existing monitor tests still pass

4. **Phase 2a: Merge logic** -- JSON merge function (inject rpki section into UPDATE JSON)
   - Tests: TestDecoratorMergeEvent, TestDecoratorMergeEventUnavailable
   - Files: `rpki_decorator/merge.go`, `merge_test.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase 2b: Decorator plugin** -- Subscribe to update + rpki, Union, emit update-rpki
   - Tests: TestDecoratorTimeoutNoRPKI, TestDecoratorOrphanRPKI
   - Files: `rpki_decorator/decorator.go`, `register.go`, `decorator_test.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase 2c: YANG + registration** -- Schema, embed, register, all.go import
   - Files: `rpki_decorator/schema/`, `all/all.go`
   - Verify: `make ze-lint` passes, plugin count updated

7. **Functional tests** -- Create .ci tests for merge, timeout, registration
   - Files: `test/plugin/rpki-decorator-*.ci`
   - Verify: `make ze-functional-test` passes

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Merged JSON is valid, rpki section correctly placed, no key collisions |
| Naming | Event type is `update-rpki` (kebab-case), YANG uses kebab-case |
| Data flow | Decorator uses standard EmitEvent, not a new mechanism |
| Rule: plugin-design | Decorator in `bgp/plugins/rpki_decorator/`, not in engine or reactor |
| Rule: no-layering | ValidBgpEvents seed set kept (not duplicated), augmented at runtime |
| Rule: goroutine-lifecycle | Union sweep goroutine is long-lived, stopped on plugin shutdown |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| RegisterEventType function exists | `grep 'func RegisterEventType' internal/component/plugin/events.go` |
| EventTypes field on Registration | `grep 'EventTypes' internal/component/plugin/registry/registry.go` |
| Decorator plugin registered | `grep 'bgp-rpki-decorator' internal/component/plugin/all/all.go` |
| update-rpki in YANG schema | `grep 'update-rpki' internal/component/bgp/plugins/rpki_decorator/schema/*.yang` |
| Monitor uses dynamic list | `grep 'ValidEventNames' internal/component/bgp/plugins/cmd/monitor/monitor.go` |
| Functional tests exist | `ls test/plugin/rpki-decorator-*.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | RegisterEventType rejects empty strings, whitespace, reserved names |
| JSON injection | Merge function uses json.Marshal (struct-based), not string concatenation |
| Resource exhaustion | Union maxPending bounds memory. EventTypes registration bounded by plugin count. |
| Event loops | Self-delivery prevention in EmitEvent (existing). Decorator must not subscribe to update-rpki. |

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

## Design Decisions

### D1: Dynamic event registration follows family registration pattern

Families are registered dynamically by plugins via registry.Register(). Event types follow the same model. The static ValidBgpEvents map becomes a seed set (engine-native events like update, state, eor) augmented at runtime by plugin registrations. This avoids central enums that require engine changes for each new event type.

### D2: Decorator is a plugin, not engine infrastructure

The engine stays content-agnostic. It routes events by type, validates types against the dynamic registry, and delivers to matching subscribers. The decorator is a regular plugin that subscribes to two event types and emits a third. No new engine primitives needed.

### D3: update-rpki defined in decorator's YANG, not centrally

The decorator plugin owns its event type. If the decorator plugin is not loaded, update-rpki does not exist. This follows the proximity principle (rules/plugin-design.md): the event type lives with the code that produces it.

### D4: Graceful degradation on timeout

If the rpki event does not arrive within the Union timeout, the decorator emits update-rpki with the UPDATE data but no rpki section. Consumers always get the UPDATE. This matches Ze's fail-open philosophy (RPKI validation timeout promotes pending routes).

### D5: Auto-loading via Dependencies

When a consumer plugin declares it subscribes to update-rpki, the decorator plugin must be loaded. This can be expressed as a dependency (consumer depends on bgp-rpki-decorator) or via auto-loading when the event type is referenced. Phase 2 will determine the exact mechanism.

### D6: rpki plugin unchanged

The rpki plugin continues to emit standalone rpki events. It does not know about the decorator. The decorator is a consumer of rpki events, not a modification to the rpki plugin.

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

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]
