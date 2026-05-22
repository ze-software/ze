# Spec: vpp-fib-query

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/iface/backend.go` - Backend interface definition
4. `internal/component/iface/dispatch.go` - package-level dispatch functions
5. `internal/plugins/iface/vpp/fib.go` - existing VPP FIB dump (ListKernelRoutes)
6. `internal/plugins/iface/vpp/ifacevpp.go` - VPP backend implementation

## Task

`RouteLookup` (used by `show ip route lookup <ip>`) bypasses the iface `Backend` interface and hardcodes netlink's `RouteGet`. When the VPP backend is active, the operator gets the kernel FIB answer instead of the VPP FIB answer. VPP's FIB is authoritative on that backend; returning kernel data misleads operators.

Move `RouteLookup` into the `Backend` interface so it dispatches through the active backend. Implement the VPP path using `IPRouteLookupV2` (GoVPP binding already exists). Move the existing netlink logic into the netlink backend.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - VPP FIB readback design
  -> Constraint: VPP backend is authoritative; kernel routes must not be returned when VPP is active
- [ ] `docs/research/vpp-deployment-reference.md` - VPP API reference for route operations
  -> Decision: use IPRouteLookupV2 (v2 returns fib_source in Src field)
- [ ] `ai/rules/exact-or-reject.md` - backends must not silently approximate
  -> Constraint: if VPP lookup fails, reject with error; never fall back to kernel

### RFC Summaries (MUST for protocol work)
N/A - internal operational command, not protocol work.

**Key insights:**
- VPP has `IPRouteLookupV2` API: takes (TableID, Exact, Prefix), returns the matching `IPRouteV2` with paths and fib source
- Exact=0 is LPM (longest prefix match), Exact=1 is exact-match only
- The existing `routeV2ToKernelRoute` helper in `fib.go` already converts `IPRouteV2` to `iface.KernelRoute`
- `RouteLookup` currently returns `map[string]any`, not `KernelRoute`. The netlink version includes table/protocol/metric fields that `KernelRoute` also has
- Backend interface has 5 implementations: `netlinkBackend` (linux), `stubBackend` (other), `vppBackendImpl`, plus 2 test mocks in `config_test.go` and `rate_test.go`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` - Backend interface; no RouteLookup method. 5 implementations found via LSP goToImplementation.
  -> Constraint: Backend is the single dispatch point for all iface operations
- [ ] `internal/component/iface/dispatch.go` - package-level functions delegate to activeBackend via backendOrErr()
- [ ] `internal/component/iface/route_lookup_linux.go` - RouteLookup uses netlink.RouteGet directly, bypasses Backend. Returns map[string]any with keys: destination, prefix, next-hop, interface, protocol, metric, table
- [ ] `internal/component/iface/route_lookup_other.go` - stub returning "route lookup not available on this platform"
- [ ] `internal/component/cmd/show/route_lookup.go` - `show ip route lookup` handler; calls `iface.RouteLookup(dest)`
- [ ] `internal/component/cmd/show/ip.go` - `show ip route` handler; calls `iface.ListKernelRoutes` (correctly dispatched via Backend)
- [ ] `internal/plugins/iface/vpp/fib.go` - ListKernelRoutes via IPRouteV2Dump; has `routeV2ToKernelRoute`, `prefixToString`, `fibNhString`, `fibSourceName` helpers
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - vppBackendImpl; lazy channel via ensureChannel(). Has errNotSupported helper for unimplemented ops.
- [ ] `internal/plugins/iface/netlink/route_linux.go` - netlinkBackend.ListKernelRoutes
- [ ] `internal/plugins/iface/netlink/backend_other.go` - stubBackend; returns unsupported() for everything
- [ ] `internal/plugins/iface/netlink/backend_linux.go` - netlinkBackend struct definition
- [ ] `vendor/go.fd.io/govpp/binapi/ip/ip.ba.go` - IPRouteLookupV2{TableID uint32, Exact uint8, Prefix ip_types.Prefix}; IPRouteLookupV2Reply{Retval int32, Route IPRouteV2}
- [ ] `internal/component/iface/config_test.go:1888` - test mock implementing Backend (needs RouteLookup stub)
- [ ] `internal/component/iface/rate_test.go:12` - test mock implementing Backend (needs RouteLookup stub)

**Behavior to preserve:**
- `show ip route lookup <ip>` returns JSON with keys: destination, prefix, next-hop, interface, protocol, metric, table
- Netlink backend behavior unchanged on Linux
- Non-linux platforms return "not available" error
- Error on invalid destination IP (handled at handler level, before Backend)
- `show ip route` (FIB dump) behavior unchanged (ListKernelRoutes)

**Behavior to change:**
- `RouteLookup` dispatches through `Backend` instead of hardcoding netlink
- VPP backend returns VPP FIB lookup result via `IPRouteLookupV2`
- Non-linux netlink stub returns unsupported error (consistent with other stub methods)

## Data Flow (MANDATORY)

### Entry Point
- CLI: `show ip route lookup 10.0.0.1`
- RPC wire method: `ze-show:route-lookup`
- Handler: `handleRouteLookup` in `internal/component/cmd/show/route_lookup.go`

### Transformation Path
1. `handleRouteLookup` parses IP address from args via `netip.ParseAddr`
2. Calls `iface.RouteLookup(dest)` (package-level dispatch)
3. **Currently:** hardcoded netlink RouteGet. **After:** dispatch to `activeBackend.RouteLookup(dest)`
4. VPP backend: `ensureChannel()` -> build `/32` or `/128` host prefix -> `IPRouteLookupV2{TableID: 0, Exact: 0, Prefix: hostPrefix}` -> convert reply via helpers
5. Response returned as `map[string]any` matching existing key schema

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> RPC | JSON via pluginserver dispatch | [ ] |
| RPC handler -> iface dispatch | `iface.RouteLookup(dest)` function call | [ ] |
| iface dispatch -> Backend | `activeBackend.RouteLookup(dest)` interface call | [ ] |
| VPP Backend -> GoVPP | `ch.SendRequest(&ip.IPRouteLookupV2{...})` | [ ] |

### Integration Points
- `iface.Backend` interface (`backend.go:49`) - add RouteLookup method
- `dispatch.go` - add package-level RouteLookup function
- `handleRouteLookup` in `route_lookup.go` - no change needed (already calls `iface.RouteLookup`)
- Existing VPP helpers in `fib.go`: `routeV2ToKernelRoute`, `prefixToString`, `fibNhString`, `fibSourceName`

### Architectural Verification
- [ ] No bypassed layers (RouteLookup now goes through Backend like ListKernelRoutes)
- [ ] No unintended coupling (VPP backend uses its own GoVPP channel, no netlink dependency)
- [ ] No duplicated functionality (reuses routeV2ToKernelRoute and related helpers from fib.go)
- [ ] Zero-copy preserved where applicable (single VPP API call, no intermediate copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show ip route lookup <ip>` via CLI RPC | -> | `vppBackendImpl.RouteLookup` | `TestVPPRouteLookup` in `fib_test.go` |
| `iface.RouteLookup(dest)` dispatch | -> | `Backend.RouteLookup` | existing `handleRouteLookup` handler + new backend method |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | VPP backend active, `show ip route lookup 10.20.0.1` with route 10.20.0.0/24 in FIB | Returns prefix=10.20.0.0/24, next-hop, device, protocol from VPP FIB (not kernel) |
| AC-2 | VPP backend active, `show ip route lookup <ip>` with no matching route | Returns error (VPP retval non-zero) |
| AC-3 | Netlink backend active, `show ip route lookup <ip>` | Behavior unchanged from current (netlink RouteGet) |
| AC-4 | VPP backend not ready (channel not acquired), `show ip route lookup <ip>` | Returns iface.ErrBackendNotReady error |
| AC-5 | Response JSON keys match existing schema: destination, prefix, next-hop, interface, protocol, metric, table | Keys present and correctly populated from VPP IPRouteLookupV2Reply |
| AC-6 | IPv6 destination on VPP backend | Correctly builds ADDRESS_IP6 prefix and returns IPv6 route |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVPPRouteLookup` | `internal/plugins/iface/vpp/fib_test.go` | VPP backend RouteLookup with mock channel returning IPRouteLookupV2Reply | |
| `TestVPPRouteLookupNoRoute` | `internal/plugins/iface/vpp/fib_test.go` | VPP backend RouteLookup when VPP returns non-zero retval | |
| `TestVPPRouteLookupIPv6` | `internal/plugins/iface/vpp/fib_test.go` | IPv6 address builds correct prefix and parses reply | |
| `TestVPPRouteLookupChannelNotReady` | `internal/plugins/iface/vpp/fib_test.go` | Returns ErrBackendNotReady when channel not acquired | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Destination IP | valid IPv4/IPv6 addr | any valid addr | malformed string (caught at handler) | N/A |
| VPP Retval | 0 = success | 0 | negative = VPP error | positive = VPP error |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fib-route-lookup` | `test/vpp/NNN-fib-route-lookup.ci` | VPP stub receives IPRouteLookupV2, returns route; CLI shows correct prefix/nexthop | |

### Interop Tests (MANDATORY for protocol features)
N/A - internal operational command, not protocol feature.

### Future (if deferring any tests)
- Per-table (VRF) route lookup: deferred to spec-vrf-0-umbrella. The TableID field exists in IPRouteLookupV2 but the CLI has no table argument today.
- ECMP multi-path display: deferred. Only first path surfaced, matching existing ListKernelRoutes behavior.

## Files to Modify
- `internal/component/iface/backend.go` - add `RouteLookup(dest netip.Addr) (map[string]any, error)` to Backend interface
- `internal/component/iface/dispatch.go` - add package-level RouteLookup dispatching to backend
- `internal/component/iface/route_lookup_linux.go` - replace hardcoded netlink with backend dispatch (or merge into dispatch.go and delete)
- `internal/component/iface/route_lookup_other.go` - replace with backend dispatch (or merge into dispatch.go and delete)
- `internal/plugins/iface/vpp/fib.go` - implement RouteLookup on vppBackendImpl using IPRouteLookupV2
- `internal/plugins/iface/vpp/fib_test.go` - tests for VPP RouteLookup
- `internal/plugins/iface/netlink/route_linux.go` - add RouteLookup method to netlinkBackend (move logic from route_lookup_linux.go)
- `internal/plugins/iface/netlink/backend_other.go` - add RouteLookup stub returning unsupported()
- `internal/component/iface/config_test.go` - add RouteLookup stub to test mock (near line 1888)
- `internal/component/iface/rate_test.go` - add RouteLookup stub to test mock (near line 12)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A - existing RPC `ze-show:route-lookup` unchanged |
| CLI commands/flags | No | N/A - existing `show ip route lookup` unchanged |
| CLI grammar (action before identifier) | No | N/A - no grammar change |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/vpp/NNN-fib-route-lookup.ci` |
| Doctor check for runtime dependencies | No | N/A - VPP dependency already checked |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A - existing command, new backend path |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/features/interfaces.md` - note RouteLookup now dispatches through Backend |

## Files to Create
- `test/vpp/NNN-fib-route-lookup.ci` - functional test for VPP FIB route lookup via stub

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** -- add RouteLookup to Backend interface and dispatch
   - Tests: `TestVPPRouteLookupChannelNotReady` (VPP stub returning ErrBackendNotReady)
   - Files: `backend.go` (interface), `dispatch.go` (package-level function), delete `route_lookup_linux.go` and `route_lookup_other.go`
   - Also: add `RouteLookup` stubs to test mocks in `config_test.go` and `rate_test.go`, and `stubBackend` in `backend_other.go`
   - Verify: compiles; existing tests pass; VPP stub returns ErrBackendNotReady

2. **Phase: Netlink backend** -- move existing netlink.RouteGet logic into netlinkBackend.RouteLookup
   - Tests: existing `show ip route lookup` behavior preserved (linux-only, integration-level)
   - Files: `internal/plugins/iface/netlink/route_linux.go` (add method)
   - Verify: `show ip route lookup` still works identically on netlink backend

3. **Phase: VPP backend** -- implement RouteLookup using IPRouteLookupV2
   - Tests: `TestVPPRouteLookup`, `TestVPPRouteLookupNoRoute`, `TestVPPRouteLookupIPv6`
   - Files: `internal/plugins/iface/vpp/fib.go`, `internal/plugins/iface/vpp/fib_test.go`
   - Verify: unit tests pass with mock channel; response matches expected JSON key schema

4. **Functional tests** -- VPP stub functional test
   - Files: `test/vpp/NNN-fib-route-lookup.ci`
   - Verify: `bin/ze-test vpp NNN-fib-route-lookup` passes

5. **Full verification** -- `make ze-verify`

6. **Complete spec** -- audit tables, write learned summary to `plan/learned/761-vpp-fib-query.md`, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has implementation with file:line |
| Correctness | VPP RouteLookup returns same JSON key schema as netlink version (destination, prefix, next-hop, interface, protocol, metric, table) |
| Naming | RouteLookup method name matches existing Backend convention; `netip.Addr` parameter matches Go stdlib |
| Data flow | RouteLookup dispatches through Backend; no netlink fallback on VPP; no hardcoded backend bypass remaining |
| Rule: exact-or-reject | VPP backend never falls back to kernel when VPP API fails; returns error |
| Rule: no-layering | route_lookup_linux.go / route_lookup_other.go fully replaced or deleted; no dead code path |
| Test mock completeness | Both test mocks in config_test.go and rate_test.go have RouteLookup stubs |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `RouteLookup` in Backend interface | `grep 'RouteLookup' internal/component/iface/backend.go` |
| Package-level `RouteLookup` dispatch | `grep 'func RouteLookup' internal/component/iface/dispatch.go` |
| VPP implementation | `grep 'func.*vppBackendImpl.*RouteLookup' internal/plugins/iface/vpp/fib.go` |
| Netlink implementation | `grep 'func.*netlinkBackend.*RouteLookup' internal/plugins/iface/netlink/route_linux.go` |
| No hardcoded netlink bypass remaining | route_lookup_linux.go deleted or contains no netlink.RouteGet |
| Unit tests pass | `go test ./internal/plugins/iface/vpp/ -run TestVPPRouteLookup -v` |
| Functional test exists | `ls test/vpp/*fib-route-lookup*` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Destination IP parsed via netip.ParseAddr before reaching Backend; invalid rejects at handler level (already done in route_lookup.go) |
| Resource exhaustion | IPRouteLookupV2 returns a single route; no unbounded allocation risk |
| Error leakage | VPP retval errors should surface as operational "no route" messages, not expose raw VPP error codes |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
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
N/A - internal operational command.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| VPP FIB lookup returns VPP data, not kernel data | unit test + functional test | TestVPPRouteLookup, test/vpp/NNN-fib-route-lookup.ci |
| Netlink backend behavior preserved | unit test | existing route_lookup behavior preserved via netlinkBackend.RouteLookup |
| Backend dispatch wiring complete | unit test | TestVPPRouteLookupChannelNotReady |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
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
- [ ] Interop tests for protocol features (or N/A: internal operational command)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/761-vpp-fib-query.md`
- [ ] Summary included in commit
