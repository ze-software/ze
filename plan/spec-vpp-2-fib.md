# Spec: vpp-2-fib — FIB VPP Plugin (Route Programming)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-vpp-1-lifecycle |
| Phase | 1/3 |
| Updated | 2026-04-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-0-umbrella.md` — parent spec
4. `internal/plugins/fibkernel/` — existing FIB backend to mirror
5. `internal/plugins/fibp4/` — noop FIB template to copy

## Task

Create the fib-vpp plugin that subscribes to `(system-rib, best-change)` events and programs
VPP's FIB via GoVPP `IPRouteAddDel`. Direct copy of fib-p4/fib-kernel structure. Includes
batch optimization (collect multiple changes, dispatch in bulk) and VPP restart recovery
via replay-request.

fibvpp gets the GoVPP connection via direct import of `internal/component/vpp/` (`vpp.Channel()`).
Lifecycle events arrive via EventBus `("vpp", "connected/disconnected/reconnected")`. On
"reconnected", fibvpp emits a replay-request to repopulate VPP's ephemeral FIB.

fibvpp owns its own Prometheus metrics via `ConfigureMetrics` callback (same pattern as
fibkernel): `ze_fibvpp_routes_installed`, `ze_fibvpp_route_installs_total`, etc. No dependency
from vpp-6 telemetry back into fibvpp.

This is the core value proposition: ze's BGP decisions programmed directly into VPP's FIB
with sub-second convergence, no kernel intermediary.

### Reference

- fib-kernel plugin: existing pattern for event subscription, route programming, installed map
- fib-p4 plugin: noop template, ready to copy for fibvpp structure
- GoVPP IPRouteAddDel: VPP binary API for route add/del/replace

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin architecture
  → Constraint: plugins under `internal/plugins/`
- [ ] `internal/plugins/fibkernel/fibkernel.go` - FIB plugin event processing
  → Constraint: subscribe to (system-rib, best-change), process JSON payload
  → Decision: same event format, same installed map pattern
- [ ] `internal/plugins/fibkernel/register.go` - FIB plugin registration
  → Constraint: Dependencies: ["rib", "sysctl"]; fibvpp uses Dependencies: ["rib", "vpp"]
- [ ] `internal/plugins/fibkernel/backend.go` - routeBackend interface
  → Constraint: addRoute/delRoute/replaceRoute/listZeRoutes/close
  → Decision: fibvpp backend extends with batch operations
- [ ] `internal/plugins/fibp4/fibp4.go` - noop FIB backend template
  → Constraint: same structure, copy and adapt
- [ ] `internal/plugins/fibp4/register.go` - P4 FIB registration
  → Constraint: same pattern with YANG augment on /fib:fib
- [ ] `.claude/patterns/registration.md` - registration pattern
  → Constraint: init() + registry.Register()

### RFC Summaries (MUST for protocol work)

Not directly protocol work. MPLS label handling is in spec-vpp-3.

**Key insights:**
- FIB plugins are event-driven: subscribe to (system-rib, best-change) via EventBus in run loop
- Event payload is JSON: `{"family":"ipv4","replay":false,"changes":[{"action":"add","prefix":"10.0.0.0/24","next-hop":"192.168.1.1","protocol":"bgp"}]}`
- fibkernel maintains installed map (prefix -> next-hop) for replace vs add decisions
- VPP FIB is ephemeral: on VPP restart, emit replay-request to repopulate (no sweep needed)
- Batch optimization: collect changes within batch-interval-ms, dispatch as batch
- GoVPP types: `ip.IPRouteAddDel{IsAdd, Route: ip.IPRoute{TableID, Prefix, NPaths, Paths}}`
- FibPath: `fib_types.FibPath{Proto, Nh, Weight, NLabels, LabelStack}` -- NLabels/LabelStack for vpp-3 MPLS
- fibvpp imports `internal/component/vpp/` for GoVPP connection via `vpp.NewConnector().NewChannel()`
- VPP lifecycle events in ("vpp", "connected/disconnected/reconnected") namespace, defined in events.go

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/fibkernel/fibkernel.go` — FIB kernel plugin run loop, processEvent, installed map
  → Constraint: fibvpp follows same event processing pattern
- [ ] `internal/plugins/fibkernel/backend.go` — routeBackend: addRoute(prefix, nextHop), delRoute(prefix), replaceRoute(prefix, nextHop), listZeRoutes(), close()
  → Constraint: fibvpp backend mirrors this with GoVPP calls
- [ ] `internal/plugins/fibkernel/register.go` — Registration: Name "fib-kernel", Dependencies ["rib", "sysctl"], ConfigRoots ["fib.kernel"]
  → Constraint: fibvpp uses Name "fib-vpp", Dependencies ["rib", "vpp"], ConfigRoots ["fib.vpp"]
- [ ] `internal/plugins/fibp4/fibp4.go` — Noop FIB plugin, same structure
  → Constraint: copy structure, replace noop backend with GoVPP backend
- [ ] `internal/plugins/fibp4/backend.go` — p4Backend interface: addRoute, delRoute, replaceRoute, close
  → Constraint: fibvpp extends with batch add/del and list operations
- [ ] `pkg/ze/eventbus.go` — EventBus.Subscribe(namespace, eventType, handler)
  → Constraint: handler receives JSON string payload

**Behavior to preserve:**
- fib-kernel continues to work independently
- EventBus event format unchanged
- sysRIB best-change payload format unchanged
- Both fib-kernel and fib-vpp can be active simultaneously

**Behavior to change:**
- New fib-vpp plugin subscribes to same events as fib-kernel
- Routes programmed in VPP FIB via GoVPP instead of kernel via netlink

## Data Flow (MANDATORY)

### Entry Point
- sysRIB emits (system-rib, best-change) event with JSON payload
- fibvpp's subscribed handler receives the payload string

### Transformation Path
1. Handler receives JSON payload string from EventBus
2. Parse payload: extract family, replay flag, changes array
3. For each change: extract action (add/del/replace), prefix, next-hop
4. Collect changes into batch (up to batch-size or batch-interval-ms)
5. Dispatch batch:
   - For add: call GoVPP IPRouteAddDel with IsAdd=true
   - For del: call GoVPP IPRouteAddDel with IsAdd=false
   - For replace: call GoVPP IPRouteAddDel with IsAdd=true (VPP replaces by default)
6. Update installed map (prefix -> next-hop) for state tracking
7. On replay (VPP restart recovery): process all routes as adds

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| sysRIB → fibvpp | EventBus JSON payload (system-rib, best-change) | [ ] |
| fibvpp → GoVPP | Binary API call via VPPConn from vpp component | [ ] |
| GoVPP → VPP | Unix socket binary protocol (/run/vpp/api.sock) | [ ] |

### Integration Points
- `internal/component/vpp/conn.go` — shared VPPConn provides IP client for route operations
- `internal/plugins/fibkernel/` — coexists, same event subscription, different backend
- `pkg/ze/eventbus.go` — event subscription
- `internal/plugin/registry.go` — plugin registration with dependency on "vpp"

### Architectural Verification
- [ ] No bypassed layers (sysRIB event → parse → GoVPP API → VPP FIB)
- [ ] No unintended coupling (fibvpp depends on vpp component for connection only)
- [ ] No duplicated functionality (parallels fibkernel for different dataplane)
- [ ] Zero-copy preserved where applicable (event payload parsed once)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| sysRIB best-change event (add) | → | fibvpp processEvent → GoVPP IPRouteAddDel(IsAdd=true) | `test/vpp/002-fib-route.ci` |
| sysRIB best-change event (del) | → | fibvpp processEvent → GoVPP IPRouteAddDel(IsAdd=false) | `test/vpp/003-fib-withdraw.ci` |
| VPP restart → reconnect | → | fibvpp emits replay-request → repopulates FIB | `test/vpp/004-vpp-restart.ci` |
| fib.vpp YANG config | → | fibvpp registration with batch-size, table-id | `test/vpp/002-fib-route.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | sysRIB emits best-change with action "add", prefix, next-hop | VPP FIB contains route for prefix via next-hop |
| AC-2 | sysRIB emits best-change with action "del", prefix | VPP FIB no longer contains route for prefix |
| AC-3 | sysRIB emits best-change with action "replace", prefix, new next-hop | VPP FIB route updated to new next-hop |
| AC-4 | Multiple changes within batch-interval-ms | Changes batched into single dispatch (fewer API round trips) |
| AC-5 | VPP restarts, fibvpp detects reconnect | fibvpp emits replay-request, sysRIB replays full table, VPP FIB repopulated |
| AC-6 | fib-kernel and fib-vpp both configured | Both active simultaneously, kernel routes and VPP FIB routes programmed independently |
| AC-7 | IPv4 prefix programmed | VPP FIB has correct IPv4 route entry |
| AC-8 | IPv6 prefix programmed | VPP FIB has correct IPv6 route entry |
| AC-9 | VRF table-id configured | Routes programmed in specified VRF table, not default |
| AC-10 | fibvpp shutdown | Plugin stops cleanly, no leaked goroutines or connections |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProcessEventAdd` | `internal/plugins/fibvpp/fibvpp_test.go` | JSON payload with add action → backend addRoute called | |
| `TestProcessEventDel` | `internal/plugins/fibvpp/fibvpp_test.go` | JSON payload with del action → backend delRoute called | |
| `TestProcessEventReplace` | `internal/plugins/fibvpp/fibvpp_test.go` | JSON payload with replace action → backend replaceRoute called | |
| `TestProcessEventBatch` | `internal/plugins/fibvpp/fibvpp_test.go` | Multiple changes batched within interval | |
| `TestProcessEventReplay` | `internal/plugins/fibvpp/fibvpp_test.go` | Replay flag set → all routes treated as adds | |
| `TestInstalledMapTracking` | `internal/plugins/fibvpp/fibvpp_test.go` | Installed map correctly tracks prefix → next-hop | |
| `TestBackendAddRoute` | `internal/plugins/fibvpp/backend_test.go` | Mock GoVPP client receives correct IPRouteAddDel for add | |
| `TestBackendDelRoute` | `internal/plugins/fibvpp/backend_test.go` | Mock GoVPP client receives correct IPRouteAddDel for del | |
| `TestBackendReplaceRoute` | `internal/plugins/fibvpp/backend_test.go` | Mock GoVPP client receives correct IPRouteAddDel for replace | |
| `TestBackendBatchAdd` | `internal/plugins/fibvpp/backend_test.go` | Batch of routes dispatched as multiple API calls | |
| `TestBackendIPv4` | `internal/plugins/fibvpp/backend_test.go` | IPv4 prefix correctly converted to VPP IP prefix format | |
| `TestBackendIPv6` | `internal/plugins/fibvpp/backend_test.go` | IPv6 prefix correctly converted to VPP IP prefix format | |
| `TestBackendVRF` | `internal/plugins/fibvpp/backend_test.go` | Table ID set correctly in IPRoute struct | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VRF table-id | 0-4294967295 | 4294967295 | N/A (0 = default) | N/A (uint32) |
| batch-size | 1-65535 | 65535 | 0 | 65536 |
| batch-interval-ms | 1-65535 | 65535 | 0 | 65536 |
| IPv4 prefix length | 0-32 | 32 | N/A | 33 |
| IPv6 prefix length | 0-128 | 128 | N/A | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-fib-route` | `test/vpp/002-fib-route.ci` | BGP learns prefix, VPP FIB has route | |
| `test-fib-withdraw` | `test/vpp/003-fib-withdraw.ci` | BGP withdraws prefix, VPP FIB route gone | |
| `test-vpp-restart` | `test/vpp/004-vpp-restart.ci` | VPP restarts, fibvpp replays full table | |
| `test-coexist` | `test/vpp/007-coexist.ci` | fib-kernel + fib-vpp both active, both program routes | |

### Future (if deferring any tests)
- MPLS label operations deferred to spec-vpp-3
- Performance benchmarks (250K routes/sec) deferred to lab environment

## Files to Modify

- `internal/plugins/fibkernel/` — no changes, reference only

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/plugins/fibvpp/schema/ze-fib-vpp-conf.yang` (augments /fib:fib) |
| CLI commands/flags | No | FIB show commands already exist via fib-kernel |
| Editor autocomplete | Yes | YANG-driven (automatic) |
| Functional test | Yes | `test/vpp/002-fib-route.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — add VPP FIB programming |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` — add fib vpp section |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — add fib-vpp |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` — FIB section |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — VPP FIB convergence |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- `internal/plugins/fibvpp/fibvpp.go` — Core: processEvent, run, installed map
- `internal/plugins/fibvpp/backend.go` — vppBackend wrapping GoVPP ip.RPCService
- `internal/plugins/fibvpp/register.go` — Plugin registration, Dependencies: ["rib", "vpp"]
- `internal/plugins/fibvpp/schema/ze-fib-vpp-conf.yang` — YANG augmenting /fib:fib
- `internal/plugins/fibvpp/fibvpp_test.go` — Event processing tests with mock backend
- `internal/plugins/fibvpp/backend_test.go` — Backend tests with mock GoVPP client
- `test/vpp/002-fib-route.ci` — Route programming functional test
- `test/vpp/003-fib-withdraw.ci` — Route withdrawal functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Backend interface + mock** — define vppBackend interface, implement mock for tests
   - Tests: `TestBackendAddRoute`, `TestBackendDelRoute`, `TestBackendReplaceRoute`, `TestBackendIPv4`, `TestBackendIPv6`, `TestBackendVRF`
   - Files: backend.go, backend_test.go
   - Verify: tests fail → implement → tests pass

2. **Phase: Event processing** — processEvent, installed map, batch collection
   - Tests: `TestProcessEventAdd`, `TestProcessEventDel`, `TestProcessEventReplace`, `TestProcessEventBatch`, `TestProcessEventReplay`, `TestInstalledMapTracking`
   - Files: fibvpp.go, fibvpp_test.go
   - Verify: tests fail → implement → tests pass

3. **Phase: Plugin wiring** — registration, YANG, run loop, VPP restart handling
   - Tests: registration test
   - Files: register.go, schema/ze-fib-vpp-conf.yang
   - Verify: tests fail → implement → tests pass

4. **Functional tests** → `test/vpp/002-fib-route.ci`, `test/vpp/003-fib-withdraw.ci`
5. **Full verification** → `make ze-verify`
6. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | GoVPP IPRouteAddDel called with correct IsAdd, Prefix, Paths, TableID |
| Naming | Plugin name "fib-vpp", ConfigRoots ["fib.vpp"], YANG augments /fib:fib |
| Data flow | EventBus payload → parse → batch → GoVPP → VPP FIB |
| Rule: no-layering | No kernel intermediary, direct GoVPP to VPP FIB |
| Rule: single-responsibility | fibvpp.go = event processing, backend.go = GoVPP calls |
| Coexistence | fib-kernel and fib-vpp both subscribe to same events independently |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| fibvpp plugin directory | `ls internal/plugins/fibvpp/` |
| Registration | `grep "registry.Register" internal/plugins/fibvpp/register.go` |
| YANG augment | `grep "augment" internal/plugins/fibvpp/schema/` |
| Event subscription | `grep "Subscribe.*system-rib.*best-change" internal/plugins/fibvpp/` |
| Backend interface | `grep "vppBackend" internal/plugins/fibvpp/backend.go` |
| Installed map | `grep "installed" internal/plugins/fibvpp/fibvpp.go` |
| Functional test | `ls test/vpp/002-fib-route.ci test/vpp/003-fib-withdraw.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | JSON payload: validate prefix format, next-hop format before GoVPP call |
| Resource exhaustion | Installed map grows with route count. Consider memory bounds for full table. |
| Batch bounds | batch-size limits memory usage. Ensure batch dispatch does not block indefinitely. |
| VPP API errors | GoVPP errors handled, logged, not silently swallowed. Retval checked. |
| Concurrent access | Installed map accessed from single goroutine (event handler). No mutex needed if single consumer. |

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

## Backend Interface

fib-vpp backend (extends fib-kernel's routeBackend pattern):

| Method | Parameters | Returns | Semantics |
|--------|-----------|---------|-----------|
| addRoute | prefix (netip.Prefix), nextHop (netip.Addr) | error | IPRouteAddDel with IsAdd=true |
| delRoute | prefix (netip.Prefix) | error | IPRouteAddDel with IsAdd=false |
| replaceRoute | prefix (netip.Prefix), nextHop (netip.Addr) | error | IPRouteAddDel with IsAdd=true (VPP replaces) |
| addRoutes | routes (slice of routeEntry) | error | Batch add: multiple IPRouteAddDel calls |
| delRoutes | prefixes (slice of netip.Prefix) | error | Batch del: multiple IPRouteAddDel calls |
| listInstalledRoutes | - | slice of routeEntry, error | Read installed map (not VPP FIB dump) |
| close | - | error | Release GoVPP client resources |

Route entry fields: prefix (netip.Prefix), nextHop (netip.Addr), label (uint32, 0 = no label, used in vpp-3).

## VPP Restart Recovery

1. VPP crashes or restarts
2. GoVPP detects disconnect via connection event channel
3. fibvpp marks itself unavailable, stops processing events
4. GoVPP reconnects (handled by vpp component, spec-vpp-1)
5. vpp component notifies fibvpp that connection is restored
6. fibvpp emits (system-rib, replay-request) event
7. sysRIB replays full best route table as (system-rib, best-change) events with replay=true
8. fibvpp processes replay events as adds, repopulating VPP FIB
9. No sweep needed: VPP FIB was empty after restart

This is simpler than fib-kernel's crash recovery (where kernel routes persist across ze restarts and need sweep/reconcile).

## YANG Config

Augments /fib:fib (same pattern as fib-p4):

| Container | Leaf | Type | Default | Description |
|-----------|------|------|---------|-------------|
| fib/vpp | enabled | boolean | false | Enable VPP FIB programming |
| fib/vpp | table-id | uint32 | 0 | VRF table ID |
| fib/vpp | batch-size | uint16 | 256 | Max routes per batch dispatch |
| fib/vpp | batch-interval-ms | uint16 | 10 | Max time before batch dispatch |

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
- (To be filled after implementation)

### Bugs Found/Fixed
- (To be filled)

### Documentation Updates
- (To be filled)

### Deviations from Plan
- (To be filled)

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
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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
- [ ] Write learned summary to `plan/learned/NNN-vpp-2-fib.md`
- [ ] Summary included in commit
