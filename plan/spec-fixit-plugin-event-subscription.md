# Spec: fixit-plugin-event-subscription

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/server/dispatch.go`, `internal/component/plugin/server/startup.go`, `internal/component/plugin/resolve.go` - subscription registration + event delivery
4. `internal/component/plugin/server/subscribe.go` - runtime subscribe parser (the working path)

## Task

Fix two related gaps in the plugin event pub/sub surface that make non-BGP
component events (IPsec/`vpn-ipsec`, and any future namespace) hard to observe
from an external plugin:

**Gap A -- startup subscriptions are namespace-locked to `bgp`.** A plugin's
`SetStartupSubscriptions(...)` (`pkg/plugin/sdk/sdk_callbacks.go` `SetStartupSubscriptions`)
carries an event list but no namespace; the engine registers it via
`registerSubscriptions`, which hardcodes the namespace to the single default
registered at startup. That default is `bgp` and only `bgp`
(`RegisterDefaultEventNamespace` is called exactly once, by the BGP component).
So a plugin cannot subscribe to `vpn-ipsec/sa-up` (or any non-bgp namespace) at
startup at all -- the subscription silently lands in the `bgp` namespace and
never matches.

**Gap B -- delivered events carry a bare payload with no namespace/name
envelope.** When the engine delivers an event to an external plugin, it sends
only the marshaled payload JSON. A subscriber that watches several event types
cannot tell from the wire which event arrived, because the payload has no
`namespace`/`name` fields of its own (e.g. the IPsec `SAEvent` carries peer
fields only).

**Why this matters (concrete):** implementing `spec-test-coverage-gaps` AC-2
(the `.ci` `expect=event` directive), the engine-step executor could not use a
single up-front subscription and match by name. The workaround was a per-step
*exclusive* subscription (subscribe -> wait for any delivery -> unsubscribe),
which only works because the *runtime* `request subscribe <ns> event <name>`
command DOES accept an explicit namespace (`ParseSubscription`). The startup
path and the delivery envelope are the gaps; the runtime command is the escape
hatch that proved the underlying delivery works.

### Scope

- IN (Gap A): let a plugin express a namespace in its startup subscriptions so
  non-bgp events can be subscribed before the first delivery.
- IN (Gap B): give delivered events enough identity that a multi-subscription
  plugin can discriminate which (namespace, event) arrived, WITHOUT breaking
  existing raw-payload consumers.
- OUT: the runtime `request subscribe` command (already works); the IPsec
  clear/re-establish bug (`spec-fixit-ipsec-clear-reestablish.md`); rearchitecting
  the event bus.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - plugin RPC + event delivery contract
  -> Constraint: (fill during research) the deliver-event wire shape and who consumes it (SDK OnEvent, bridge, exabgp)
- [ ] `ai/rules/plugin-design.md` - plugin SDK / protocol change rules
  -> Constraint: an envelope change must not break the raw-payload consumers (exabgp bridge, existing subscribers)
- [ ] `internal/component/plugin/resolve.go` (:84-98 RegisterDefaultEventNamespace) - the single default namespace mechanism
  -> Decision: Gap A either threads a per-subscription namespace through the startup RPC, or registers additional default namespaces per component

### RFC Summaries (MUST for protocol work)
- N/A - internal plugin protocol, no RFC.

**Key insights:** (fill during research)

## Current Behavior (MANDATORY)

**Source files read (2026-07-10, spec author):**
- [ ] `internal/component/plugin/server/dispatch.go` (:135-153 registerSubscriptions) - subscribe-events RPC carries no namespace; `namespace := plugin.DefaultEventNamespace()` (:148); warns when no default is registered (:150); `nsID := events.LookupNamespaceID(namespace)` (:153)
- [ ] `internal/component/plugin/resolve.go` (:84-98 RegisterDefaultEventNamespace) - sets ONE global default; `internal/component/bgp/plugin/register.go:64` calls it with `bgpevents.Namespace` ("bgp"); nothing else calls it
- [ ] `internal/component/plugin/server/startup.go` (:605-614 engineStartupSink.onReady) - startup `ReadyInput.Subscribe` also routes through `registerSubscriptions` (:611), inheriting the same bgp-only default
- [ ] `pkg/plugin/rpc/types.go` (:480-486 SubscribeEventsInput) - fields `Events`/`Peers`/`Format`/`Encoding`; NO namespace field
- [ ] `internal/component/plugin/server/subscribe.go` (:180-252 ParseSubscription) - the RUNTIME `request subscribe [<ns>] event <type>` command DOES accept an explicit namespace (:209-222) and validates it against registered namespaces (:217) -- this is the working path
- [ ] `internal/component/plugin/server/dispatch.go` (:245-283 deliverEvent, :286-304 payloadToJSON) - delivered event JSON is the bare payload: string passthrough (:294), json.RawMessage passthrough (:297), else `json.Marshal(payload)` (:300) -- no namespace/name envelope added
- [ ] `internal/component/plugin/server/deliver_subscriber_test.go` (spec-test-coverage-gaps) - proves EmitEngineEvent delivers to a SubscriptionManager subscriber with delivered==1, so the delivery leg itself works

**Behavior to preserve:**
- The runtime `request subscribe <ns> event <name>` command and its parser.
- Existing raw-payload consumers: the exabgp bridge and any plugin that parses `SetStartupSubscriptions([]string{"*"}, ...)` deliveries as bare payloads (`internal/plugins/exabgp/`, `internal/plugins/flowspec-firewall/engine.go`).
- BGP-namespace startup subscriptions (the common case) must keep working unchanged.
- Lazy-marshal-once delivery performance (`payloadToJSON` marshals only when an external subscriber exists).

**Behavior to change:**
- Gap A: a startup subscription can name a non-bgp namespace.
- Gap B: delivered events are discriminable by (namespace, event) without breaking raw-payload consumers. The exact mechanism (envelope opt-in, sidecar field, or per-subscription encoding) is a design decision, not presupposed here.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Startup: `ze-plugin-engine:ready` with `ReadyInput.Subscribe` (SDK `SetStartupSubscriptions`)
- Runtime: `request subscribe <ns> event <name>` dispatch-command
- Delivery: engine `EmitEngineEvent(namespace, eventType, payload)` -> `deliverEvent` -> per-process `Deliver`

### Transformation Path
1. Subscribe registration -> `SubscriptionManager.Add(proc, sub)` (namespace resolved here -- the Gap A choke point)
2. Emit -> `deliverEvent` -> `GetMatching(ns, et, ...)` -> `payloadToJSON` (the Gap B choke point) -> `proc.Deliver`
3. External proc delivery goroutine -> deliver-event RPC / bridge -> SDK `OnEvent(string)`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| plugin -> engine (startup) | ready RPC Subscribe field | [ ] |
| plugin -> engine (runtime) | dispatch-command subscribe | [ ] |
| engine -> plugin (delivery) | deliver-event RPC / bridge | [ ] |

### Integration Points
- `internal/component/plugin/server/` subscription manager, dispatch, startup sink
- `pkg/plugin/rpc/types.go` SubscribeEventsInput (Gap A field)
- `pkg/plugin/sdk/` startup subscription API (Gap A surface)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - namespace resolution is registry-driven, not a hardcoded default (this fix removes a hardcoded default)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Startup subscriptions are namespace-locked to bgp | dispatch.go:148 + resolve.go single default + only bgp registers it | Gap A is narrower | grep RegisterDefaultEventNamespace callers | confirmed (only `internal/component/bgp/plugin/register.go:64`) |
| A-2 | Delivered events carry no namespace/name envelope | payloadToJSON dispatch.go:286-304 | Gap B does not exist | read a delivered payload for a typed event | confirmed (bare payload passthrough/marshal) |
| A-3 | Existing consumers rely on the bare-payload shape | exabgp bridge, flowspec-firewall use `*`/parsed formats | an envelope breaks them | grep SetStartupSubscriptions consumers, read their OnEvent | unvalidated |
| A-4 | The runtime subscribe path is the correct namespace precedent to mirror | ParseSubscription subscribe.go:209-222 | design a new namespace mechanism | reuse LookupNamespaceID validation | confirmed (parser accepts + validates ns) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An envelope change breaks the exabgp bridge / existing subscribers | exabgp or flowspec `.ci` red | make the envelope opt-in per subscription/encoding; keep bare-payload the default |
| R-2 | Per-subscription namespace re-marshals on the hot path, hurting BGP throughput | perf regression in stress runs | keep lazy-marshal-once; envelope only when the subscriber requested it |
| R-3 | Namespace threading touches the SDK ABI and breaks external plugins built against pkg/plugin | external plugin build breaks | additive field with a sensible default (empty = bgp legacy behavior) |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin declares a startup subscription in a non-bgp namespace | -> | startup sink registers it in that namespace | `TestStartupSubscriptionHonorsNamespace` (unit) |
| Engine emits a non-bgp event; subscriber discriminates it | -> | delivery envelope / identity | `TestDeliveredEventIsDiscriminable` (unit) |
| A plugin subscribes to `vpn-ipsec/sa-up` at startup and receives it | -> | end-to-end startup subscribe + deliver | `test/plugin/plugin-startup-subscribe-namespace.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin sets a startup subscription naming namespace `vpn-ipsec`, event `sa-up` | The engine registers it in `vpn-ipsec`; a `vpn-ipsec/sa-up` emit is delivered to that plugin |
| AC-2 | Plugin subscribed to two event types receives one of them | The delivered event is discriminable by (namespace, event) without parsing peer-specific payload fields |
| AC-3 | Existing bgp-namespace startup subscription (no explicit namespace) | Unchanged: still lands in `bgp`, still delivers |
| AC-4 | exabgp bridge / flowspec-firewall existing subscriptions | Unchanged delivery; their `.ci`/interop tests stay green (R-1) |
| AC-5 | BGP stress path | No throughput regression from the change (R-2) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An external plugin observes IPsec sa-up at startup | SetStartupSubscriptions(ns=vpn-ipsec) -> ready RPC -> register -> emit -> deliver -> OnEvent | `test/plugin/plugin-startup-subscribe-namespace.ci` |
| 2 | A multi-event plugin routes by event identity | deliver-event with discriminable identity -> plugin handler switch | `TestDeliveredEventIsDiscriminable` + `.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStartupSubscriptionHonorsNamespace` | `internal/component/plugin/server/*_test.go` | AC-1 Gap A | |
| `TestDeliveredEventIsDiscriminable` | `internal/component/plugin/server/*_test.go` | AC-2 Gap B | |
| `TestBgpStartupSubscriptionUnchanged` | `internal/component/plugin/server/*_test.go` | AC-3 regression guard | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric input) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-startup-subscribe-namespace.ci` | `test/plugin/` | plugin subscribes to a non-bgp namespace at startup and receives the event | |
| `plugin-event-discriminable.ci` | `test/plugin/` | multi-subscription plugin routes by event identity | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - internal plugin protocol, no external peer daemon; existing exabgp/bgp `.ci` cover the regression surface | - | - | - | |

## Files to Modify
- `internal/component/plugin/server/dispatch.go` - namespace resolution (Gap A) + delivery identity (Gap B)
- `internal/component/plugin/server/startup.go` - startup sink passes the namespace through
- `pkg/plugin/rpc/types.go` - `SubscribeEventsInput` namespace field (additive, Gap A)
- `pkg/plugin/sdk/sdk_callbacks.go` - `SetStartupSubscriptions` namespace surface (Gap A)
- `internal/component/plugin/resolve.go` - only if the default-namespace mechanism changes

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | N/A | no config surface |
| CLI commands/flags | N/A | runtime subscribe unchanged |
| Functional test for new RPC/API | Yes | `test/plugin/plugin-startup-subscribe-namespace.ci` |
| Plugin SDK/protocol changed | Yes | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| Doctor check | N/A | no new runtime dependency |
| Prometheus counters | N/A | no new observable product state |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` (subscribe-events namespace) |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 12 | Internal architecture changed? | [ ] | `ai/digests/` plugin event-flow digest |
| 15 | Registered event type / send type changed? | [ ] | `docs/plugin-overview.md`, `docs/features/plugins.md` |

## Files to Create
- `test/plugin/plugin-startup-subscribe-namespace.ci`, `test/plugin/plugin-event-discriminable.ci`
- `internal/component/plugin/server/<name>_test.go` (unit tests above)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, validate A-3 (consumer survey) first |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases
1. **Phase: Consumer survey (validate A-3)** - enumerate every `SetStartupSubscriptions` caller and every raw-payload OnEvent consumer; confirm the envelope/namespace change is additive for them.
2. **Phase: Gap A** - additive namespace on the startup subscription (RPC field + SDK surface + startup sink), defaulting to bgp legacy behavior; unit + `.ci`.
3. **Phase: Gap B** - discriminable delivery (opt-in envelope or per-subscription encoding), preserving bare-payload default and lazy-marshal; unit + `.ci`.
4. **Phase: regression + perf** - bgp startup path unchanged; exabgp/flowspec green; stress path unregressed.
5. **Full verification + closure.**

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Backward compat | bgp startup subscriptions + raw-payload consumers unchanged (AC-3/AC-4) |
| Registration over hardcoding | namespace comes from the registry, not a single hardcoded default |
| Performance | lazy-marshal-once preserved; envelope only when requested (R-2) |
| Plugin ABI | SDK/RPC change is additive with a legacy default (R-3) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| non-bgp startup subscribe works | `go test -run TestStartupSubscriptionHonorsNamespace ./internal/component/plugin/server/` |
| delivered event discriminable | `go test -run TestDeliveredEventIsDiscriminable ./internal/component/plugin/server/` |
| exabgp unregressed | `bin/ze-test exabgp --all` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | namespace strings validated against registered namespaces (LookupNamespaceID) |
| Information leakage | envelope must not expose more than the payload already did |

### Failure Routing
| Failure | Route To |
|---------|----------|
| exabgp/flowspec test red | envelope not additive -> back to DESIGN (make it opt-in) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- (fill during design) whether Gap B is an opt-in envelope vs a per-subscription encoding is decided during design; both must keep the bare-payload default.

## RFC Documentation
- N/A (internal plugin protocol).

## Implementation Summary
### What Was Implemented
- (fill at completion)
### Bugs Found/Fixed
- (fill at completion)
### Documentation Updates
- (fill at completion)
### Deviations from Plan
- (fill at completion)

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| non-bgp startup subscription works | functional | `plugin-startup-subscribe-namespace.ci` |
| events discriminable | functional + unit | `plugin-event-discriminable.ci`, `TestDeliveredEventIsDiscriminable` |
| no regression | functional | exabgp + bgp plugin suites green |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE]

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
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `pkg/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

## Notes
- Authored 2026-07-10 from the `spec-test-coverage-gaps` AC-2 engine-step
  implementation (see that spec's Design Insights: the executor's exclusive-window
  subscription workaround and the `deliver_subscriber_test.go` delivery proof).
  Skeleton = captured intent with verified `file:line` evidence; moves to `design`
  when picked up.
