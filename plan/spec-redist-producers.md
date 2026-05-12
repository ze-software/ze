# Spec: redist-producers

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | 3/3 |
| Updated | 2026-05-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/static/` (reference implementation for connected)
4. `internal/component/l2tp/route_observer.go` (reference for framed routes)
5. `internal/core/redistevents/` (shared infrastructure)

## Task

Add two new redistribute producers following the static plugin pattern:

1. **Connected route producer** -- new plugin at `internal/plugins/connected/` that watches interface address events and emits `RouteChangeBatch` for connected prefixes into BGP via `redistribute { import connected }`.

2. **L2TP framed-route extension** -- extend L2TP's existing redistribution to also emit RADIUS Framed-Route (attr 22) and Framed-IPv6-Route (attr 99) per-subscriber static routes.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/registration.md`
  -> Constraint: init() -> registry.Register(), blank import in all.go, make generate
- [ ] `ai/patterns/plugin.md`
  -> Constraint: atomic logger, RunEngine entry point, ConfigureEventBus callback
- [ ] `docs/architecture/core-design.md`
  -> Decision: plugins register at startup, communicate via text commands and EventBus

### RFC Summaries
- [ ] RFC 2865 Section 5.22: Framed-Route (attr 22) -- "destination/mask gateway metric" text format, multi-valued
- [ ] RFC 6911 Section 3.2: Framed-IPv6-Route (attr 99) -- "prefix/len gateway metric" text format, multi-valued

**Key insights:**
- Static plugin is the reference: events subpackage, redistevents.RegisterProtocol/RegisterProducer, ConfigureEventBus callback
- L2TP already emits /32 and /128 subscriber routes; framed routes extend the same observer
- Connected routes need no kernel programming (kernel already has them), only redistribute emission
- Framed-Route is multi-valued (FindAllAttr), text format with space-separated fields

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/static/events/events.go` -- reference events package
  -> Constraint: RegisterProtocol + RegisterProducer + events.Register pattern
- [ ] `internal/plugins/static/inject.go` -- route manager emitting RouteChangeBatch
  -> Constraint: AcquireBatch/ReleaseBatch lifecycle, AFI/SAFI from family
- [ ] `internal/plugins/static/register.go` -- plugin registration with ConfigureEventBus
  -> Constraint: RunEngine, CLIHandler, ConfigRoots, YANG schema
- [ ] `internal/component/l2tp/route_observer.go` -- existing subscriber route emit
  -> Constraint: routeRecord tracks v4/v6, emitAdd/emitRemove pattern
- [ ] `internal/component/l2tp/session_metadata.go` -- AuthMetadata struct
  -> Constraint: sync.Map store, Store/Load/Clear lifecycle
- [ ] `internal/plugins/l2tpauthradius/extract.go` -- RADIUS attribute extraction
  -> Constraint: extractAuthMetadata called from handler.go on Access-Accept
- [ ] `internal/component/radius/dict.go` -- RADIUS attribute constants
  -> Constraint: AttrFramedRoute (22) and AttrFramedIPv6Route (99) not yet defined
- [ ] `internal/component/iface/events/events.go` -- interface event types
  -> Constraint: EventAddrAdded, EventAddrRemoved with namespace "interface"
- [ ] `internal/component/config/redistribute/registry.go` -- RegisterSource
  -> Constraint: idempotent, rejects protocol conflicts

**Behavior to preserve:**
- Existing L2TP subscriber route emission (/32, /128) unchanged
- Static plugin pattern unchanged
- redistevents infrastructure unchanged

**Behavior to change:**
- Add connected route producer (new plugin)
- Extend L2TP to emit framed routes from RADIUS alongside subscriber routes
- Add RADIUS Framed-Route and Framed-IPv6-Route attribute parsing

## Data Flow (MANDATORY)

### Connected Routes

#### Entry Point
- Interface address added/removed via netlink monitor
- Emits `(interface, addr-added)` or `(interface, addr-removed)` on EventBus

#### Transformation Path
1. Netlink monitor detects address change, emits iface event
2. Connected plugin receives event via EventBus subscription
3. Extracts address and prefix length, computes network prefix
4. Emits RouteChangeBatch with ActionAdd/ActionRemove via redistevents

#### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface -> connected plugin | EventBus subscription (interface, addr-added/removed) | [ ] |
| connected plugin -> bgp-redistribute | EventBus emission (connected, route-change) | [ ] |

### L2TP Framed Routes

#### Entry Point
- RADIUS Access-Accept response with Framed-Route (22) / Framed-IPv6-Route (99) attributes

#### Transformation Path
1. RADIUS handler receives Access-Accept
2. extractAuthMetadata parses Framed-Route text format into []netip.Prefix + metrics
3. AuthMetadata stored per-session via StoreSessionMetadata
4. On OnSessionIPUp, route observer reads metadata, emits framed routes as additional RouteChangeBatch entries
5. On OnSessionDown, observer withdraws all framed routes

#### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RADIUS -> l2tp auth | extractAuthMetadata extracts Framed-Route attrs | [ ] |
| l2tp auth -> l2tp reactor | StoreSessionMetadata / LoadSessionMetadata | [ ] |
| l2tp reactor -> bgp-redistribute | EventBus emission (l2tp, route-change) | [ ] |

### Integration Points
- `redistevents.RegisterProtocol("connected")` -- new protocol registration
- `redistevents.RegisterProducer(ProtocolID)` -- marks connected as producer
- `redistribute.RegisterSource(RouteSource{Name: "connected"})` -- config layer
- Existing `l2tp` protocol ID reused for framed routes

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved (pooled batches)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config `redistribute { import connected }` + interface address | -> | connected plugin emitAdd | `test/plugin/redistribute-connected-announce.ci` |
| Interface address removed | -> | connected plugin emitRemove | `test/plugin/redistribute-connected-withdraw.ci` |
| RADIUS Access-Accept with Framed-Route | -> | route observer emitFramedRoutes | `test/plugin/redistribute-l2tp-framed-route.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `connected { }` + interface with address | Connected plugin registers as redistribute producer |
| AC-2 | Interface addr-added event | Connected plugin emits RouteChangeBatch with network prefix (not host), ActionAdd |
| AC-3 | Interface addr-removed event | Connected plugin emits RouteChangeBatch with ActionRemove |
| AC-4 | Interface 10.0.0.1/24 added | Emitted prefix is 10.0.0.0/24 (network, not host address) |
| AC-5 | RADIUS Access-Accept with Framed-Route "10.0.0.0/8 0.0.0.0 1" | AuthMetadata contains parsed FramedRoute with prefix 10.0.0.0/8 and metric 1 |
| AC-6 | L2TP session up with framed routes in metadata | Route observer emits framed route prefixes alongside subscriber /32 |
| AC-7 | L2TP session down with framed routes | Route observer withdraws framed routes alongside subscriber /32 |
| AC-8 | Multiple Framed-Route attributes in RADIUS response | All routes parsed and emitted |
| AC-9 | Framed-IPv6-Route "2001:db8::/32 :: 1" | Parsed as IPv6 prefix with metric |
| AC-10 | Connected plugin with no EventBus | No panic, state tracking still works |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConnectedEmitAdd` | `internal/plugins/connected/connected_test.go` | AC-2: addr-added emits correct batch |  |
| `TestConnectedEmitRemove` | `internal/plugins/connected/connected_test.go` | AC-3: addr-removed emits remove |  |
| `TestConnectedNetworkPrefix` | `internal/plugins/connected/connected_test.go` | AC-4: host addr -> network prefix |  |
| `TestConnectedNilBus` | `internal/plugins/connected/connected_test.go` | AC-10: nil bus no panic |  |
| `TestExtractFramedRoute` | `internal/plugins/l2tpauthradius/extract_test.go` | AC-5: parse Framed-Route text |  |
| `TestExtractFramedIPv6Route` | `internal/plugins/l2tpauthradius/extract_test.go` | AC-9: parse Framed-IPv6-Route |  |
| `TestExtractMultipleFramedRoutes` | `internal/plugins/l2tpauthradius/extract_test.go` | AC-8: multi-valued |  |
| `TestRouteObserverFramedRoutes` | `internal/component/l2tp/route_observer_test.go` | AC-6: emit framed routes on session up |  |
| `TestRouteObserverFramedRouteWithdraw` | `internal/component/l2tp/route_observer_test.go` | AC-7: withdraw on session down |  |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Framed-Route metric | 0-4294967295 | 4294967295 | N/A | N/A |
| Prefix length IPv4 | 0-32 | 32 | N/A | 33 |
| Prefix length IPv6 | 0-128 | 128 | N/A | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-connected-announce` | `test/plugin/redistribute-connected-announce.ci` | connected route appears in BGP UPDATE |  |
| `redistribute-connected-withdraw` | `test/plugin/redistribute-connected-withdraw.ci` | connected route withdrawn on addr removal |  |
| `redistribute-l2tp-framed-route` | `test/plugin/redistribute-l2tp-framed-route.ci` | L2TP framed route appears in BGP UPDATE |  |

### Future
- VPP backend for connected routes (requires VPP address event integration)

## Files to Modify

- `internal/component/radius/dict.go` -- add AttrFramedRoute, AttrFramedIPv6Route constants
- `internal/plugins/l2tpauthradius/extract.go` -- parse Framed-Route and Framed-IPv6-Route
- `internal/component/l2tp/session_metadata.go` -- add FramedRoutes to AuthMetadata
- `internal/component/l2tp/route_observer.go` -- emit framed routes on session up/down

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `internal/plugins/connected/schema/ze-connected-conf.yang` |
| Plugin registration | [x] | `internal/plugins/connected/register.go` |
| Blank import (all.go) | [x] | `make generate` |
| Functional tests | [x] | `test/plugin/redistribute-connected-*.ci`, `test/plugin/redistribute-l2tp-framed-route.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add connected route redistribution |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- connected block |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- add connected plugin |
| 6 | Has a user guide page? | [x] | `docs/guide/redistribution.md` -- add connected and framed routes |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | Framed-Route RFC 2865 S5.22, Framed-IPv6-Route RFC 6911 S3.2 |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- connected route redistribution |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/plugins/connected/register.go` -- plugin registration
- `internal/plugins/connected/connected.go` -- route observer, addr event handler
- `internal/plugins/connected/connected_test.go` -- unit tests
- `internal/plugins/connected/events/events.go` -- redistevents producer registration
- `internal/plugins/connected/eventbus.go` -- atomic EventBus storage
- `internal/plugins/connected/logger.go` -- atomic logger
- `internal/plugins/connected/schema/ze-connected-conf.yang` -- YANG config
- `internal/plugins/connected/schema/register.go` -- YANG registration
- `internal/plugins/connected/schema/embed.go` -- YANG embed
- `test/plugin/redistribute-connected-announce.ci` -- functional test
- `test/plugin/redistribute-connected-withdraw.ci` -- functional test
- `test/plugin/redistribute-l2tp-framed-route.ci` -- functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase: L2TP Framed Routes** -- RADIUS parsing + route observer extension
   - Tests: TestExtractFramedRoute, TestExtractFramedIPv6Route, TestExtractMultipleFramedRoutes, TestRouteObserverFramedRoutes, TestRouteObserverFramedRouteWithdraw
   - Files: dict.go, extract.go, session_metadata.go, route_observer.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Connected Plugin** -- new plugin with events, registration, addr handler
   - Tests: TestConnectedEmitAdd, TestConnectedEmitRemove, TestConnectedNetworkPrefix, TestConnectedNilBus
   - Files: all connected/ files
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Functional Tests + Docs** -- .ci tests and documentation
   - Tests: redistribute-connected-announce.ci, redistribute-connected-withdraw.ci, redistribute-l2tp-framed-route.ci
   - Files: test/plugin/*.ci, docs/*.md
   - Verify: make ze-functional-test

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Network prefix computation correct (mask host bits), Framed-Route text parsing robust |
| Naming | Protocol name "connected" matches convention, YANG uses kebab-case |
| Data flow | Connected subscribes to iface events, not polling; L2TP reads metadata at session-up |
| Rule: no-layering | No wrapper around redistevents, direct use |
| Rule: value-types | RouteChangeBatch entries are value types, no pointers cross boundaries |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Connected plugin registered | `grep -r 'RegisterProtocol.*connected' internal/plugins/connected/` |
| Connected events package | `ls internal/plugins/connected/events/events.go` |
| YANG schema | `ls internal/plugins/connected/schema/ze-connected-conf.yang` |
| Framed-Route constants | `grep AttrFramedRoute internal/component/radius/dict.go` |
| Framed-Route extraction | `grep -n FramedRoute internal/plugins/l2tpauthradius/extract.go` |
| AuthMetadata extended | `grep FramedRoutes internal/component/l2tp/session_metadata.go` |
| Route observer extended | `grep -n framedRoutes internal/component/l2tp/route_observer.go` |
| Functional tests | `ls test/plugin/redistribute-connected-*.ci test/plugin/redistribute-l2tp-framed-route.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Framed-Route text parsing: reject malformed, missing fields, invalid IPs |
| Resource exhaustion | Bound max framed routes per session (RADIUS can send many) |
| Panic safety | Connected plugin nil bus, malformed addr events |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
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

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Framed-Route (RFC 2865 Section 5.22): multi-valued, text format "destination/mask gateway metric"
Framed-IPv6-Route (RFC 6911 Section 3.2): multi-valued, text format "prefix/len gateway metric"

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/685-redist-producers.md`
- [ ] Summary included in commit
