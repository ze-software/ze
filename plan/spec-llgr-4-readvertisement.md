# Spec: llgr-4-readvertisement

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-route-metadata, spec-rib-family-ribout |
| Phase | 1/6 |
| Updated | 2026-04-02 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9494.md` - LLGR_STALE readvertisement rules, partial deployment
4. `plan/spec-route-metadata.md` - route metadata and ModAccumulator infrastructure
5. `plan/spec-rib-family-ribout.md` - per-family ribOut and `rib clear out <peer> <family>`
6. `internal/component/bgp/plugins/gr/gr.go` - GR plugin, LLGR callbacks, peerLLGRCaps
7. `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate, egress filter chain

## Task

When LLGR begins, stale routes with LLGR_STALE attached must be re-advertised to LLGR-capable peers and withdrawn from non-LLGR EBGP peers. For IBGP partial deployment, routes are sent with NO_EXPORT and LOCAL_PREF=0 via the route metadata modification accumulator.

Parent: `spec-llgr-0-umbrella.md`
Depends on:
- `spec-llgr-3-rib-integration.md` (done: LLGR_STALE attached, depreference working)
- `spec-route-metadata.md` (meta `map[string]any` on routes, ModAccumulator on egress filters)
- `spec-rib-family-ribout.md` (per-family ribOut, `rib clear out <peer> <family>`)

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| LLGR egress filter | Registered statically by GR plugin, fast-path bail out via atomic flag |
| Stale on ribOut | `rib mark-stale` propagates StaleLevel to matching ribOut routes |
| Stale metadata via RPC | RIB sendRoutes calls UpdateRouteWithMeta with `meta["stale"]` when Route.StaleLevel > 0 |
| LLGR suppression | Egress filter suppresses LLGR_STALE routes to EBGP non-LLGR peers |
| LLGR partial deployment | Egress filter writes `mods["add:attr:community"]` and `mods["set:attr:local-preference"]` for IBGP non-LLGR peers |
| Per-family readvertisement | GR plugin dispatches `rib clear out !<peer> <family>` per LLGR family |
| Withdrawal from non-LLGR peers | Egress filter sets `mods["withdraw:nlri:*"]=true`; forward path sends withdrawal |
| Community preservation | LLGR_STALE community flows through forward path unchanged (already in wire bytes) |

**Out of Scope:**

| Area | Reason |
|------|--------|
| Role plugin migration to metadata | Separate spec |
| Restarting Speaker procedures | Ze implements Receiving Speaker only |
| VPN ATTR_SET (RFC 6368) | Requires VPN infrastructure |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - forward path, egress filter chain, UPDATE building
  -> Constraint: egress filters called per-destination-peer; metadata is per-route (from ReceivedUpdate)
  -> Decision: GR plugin registers egress filter statically; filter closure captures peerLLGRCaps map

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - LLGR readvertisement rules
  -> Constraint: LLGR_STALE routes SHOULD NOT be advertised to peers without LLGR capability
  -> Constraint: MUST NOT remove LLGR_STALE community when readvertising
  -> Constraint: partial deployment (IBGP): attach NO_EXPORT, set LOCAL_PREF=0
  -> Constraint: only used as last resort (no better route available)

**Key insights:**
- Readvertisement trigger already exists: `onLLGREntryDone` dispatches `rib clear out !<peerAddr>` (gr.go:101)
- With per-family ribOut (spec-rib-family-ribout), this becomes `rib clear out !<peerAddr> <family>`
- Egress filter registered statically on GR plugin, fast-path bail out via atomic when no peer in LLGR
- Route metadata (spec-route-metadata) carries stale level; egress filter reads `meta["stale"]` instead of parsing wire bytes
- ModAccumulator (spec-route-metadata) enables partial deployment: filter writes `mods["add:attr:community"]` and `mods["set:attr:local-preference"]`
- LLGR_STALE community is already in wire bytes (attached by rib attach-community in phase 3); forward path preserves it
- LLGR capability stored in `grPlugin.peerLLGRCaps` (gr.go:65); filter closure captures this map

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/gr/gr.go:65` - `peerLLGRCaps map[string]*llgrPeerCap` stores LLGR cap per peer
- [ ] `internal/component/bgp/plugins/gr/gr.go:100-102` - `onLLGREntryDone` dispatches `rib clear out !<peerAddr>`
- [ ] `internal/component/bgp/plugins/gr/gr.go:90-98` - `onLLGREnter` dispatches rib commands per family
- [ ] `internal/component/bgp/plugins/gr/register.go` - registers cap codes 64+71, no egress filter currently
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:256-272` - egress filter call site in ForwardUpdate
- [ ] `internal/component/bgp/plugins/role/register.go:28` - reference: OTC registers EgressFilter in Registration
- [ ] `internal/component/bgp/plugins/role/otc.go:262-300` - reference: OTCEgressFilter reads config via closure
- [ ] `internal/component/plugin/registry/registry.go:44-47,99` - EgressFilterFunc type and Registration.EgressFilter field

**Behavior to preserve:**
- Normal route advertisement path unchanged
- UPDATE building for non-LLGR routes unchanged
- LLGR_STALE community in wire bytes flows through forward path as-is
- GR plugin's existing LLGR callbacks (onLLGREnter, onLLGRFamilyExpired, onLLGRComplete)
- Role OTC egress filter continues to work alongside LLGR filter

**Behavior to change:**
- GR plugin registers an egress filter at init (static registration)
- `onLLGREntryDone` dispatches per-family `rib clear out !<peerAddr> <family>` instead of all-family
- Egress filter checks route metadata for stale level and suppresses/modifies per RFC 9494
- RIB sets `meta["stale"]` when storing routes with StaleLevel > 0

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- LLGR entry completes for a peer (all families transitioned)
- GR plugin dispatches `rib clear out !<peerAddr> <family>` per affected family
- RIB replays ribOut routes via sendRoutes -> updateRoute -> ForwardUpdate

### Transformation Path
1. GR plugin's `onLLGREntryDone(peerAddr)` fires after all families enter LLGR
2. For each LLGR family: `rib clear out !<peerAddr> <family>` (per-family ribOut from spec-rib-family-ribout)
3. RIB's `outboundResendJSON` replays routes; Route.StaleLevel > 0 triggers `UpdateRouteWithMeta` with `meta["stale"]`
4. Routes reach ForwardUpdate via updateRoute RPC; ReceivedUpdate.Meta carries stale level
5. Per destination peer, egress filter chain runs:
   - LLGR filter: atomic check -- no peers in LLGR? return true immediately (fast path)
   - Reads `meta["stale"]`; if absent or 0, return true (non-stale route, pass through)
   - If stale: checks `peerLLGRCaps[dest.Address]`
   - LLGR-capable peer: return true, no mods (LLGR_STALE already in wire bytes)
   - EBGP non-LLGR peer: return true + `mods.Set("withdraw:nlri:*", true)` (convert to withdrawal)
   - IBGP non-LLGR peer: return true + `mods.Set("add:attr:community", ...)` + `mods.Set("set:attr:local-preference", ...)`
6. Forward path applies mods: withdrawal, or attribute modification, or passthrough
7. Forward pool sends to each peer: original UPDATE, modified UPDATE, or withdrawal

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB -> Forward path | `rib clear out !<peer> <family>` triggers per-family resend | [ ] |
| RIB -> Route metadata | RIB sets `meta["stale"]` on routes with StaleLevel > 0 | [ ] |
| Forward path -> Egress filter | Filter receives metadata via ReceivedUpdate.Meta | [ ] |
| Egress filter -> ModAccumulator | Filter writes mods for IBGP partial deployment | [ ] |
| Forward path -> Wire | Mods applied (add-community, set-local-pref) before sending | [ ] |

### Integration Points
- `gr/register.go` - add EgressFilter to Registration
- `gr/gr.go` - LLGR egress filter function with fast-path atomic bail out
- `gr/gr.go:100-102` - change `rib clear out !<peerAddr>` to per-family variant
- `rib/rib_commands.go` or `rib/rib.go` - set `meta["stale"]` on routes during resend
- ForwardUpdate (spec-route-metadata provides the infrastructure)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (metadata is sideband, wire bytes unchanged unless mods applied)

## Architecture Decisions

### AD-1: Static filter registration with atomic fast-path bail out

GR plugin registers its egress filter at `init()` in `register.go`, like role does. The filter closure captures `grPlugin`'s `peerLLGRCaps` map and an `atomic.Bool` (or `atomic.Int32` tracking count of peers in LLGR). When no peer is in LLGR state (common case), filter returns `true` immediately after one atomic load. No payload parsing, no map lookup.

### AD-2: Stale propagation to ribOut and metadata via RPC

Two-step mechanism:
1. `rib mark-stale <peer> <time> <level>` already marks ribIn routes. It now also sets `StaleLevel` on matching ribOut `Route` structs (new field on `bgp.Route`).
2. When `sendRoutes` replays ribOut, it checks `Route.StaleLevel > 0`. If stale, calls `UpdateRouteWithMeta(ctx, sel, cmd, map[string]any{"stale": staleLevel})` (spec-route-metadata AD-7).
3. The meta flows through the RPC to CommandContext to ReceivedUpdate.Meta.
4. Egress filter reads `meta["stale"]` -- no wire payload parsing needed.

### AD-3: Per-family readvertisement via rib clear out

`onLLGREntryDone` changes from:
- Old: `rib clear out !<peerAddr>` (all families)
- New: iterate LLGR families for this peer, dispatch `rib clear out !<peerAddr> <family>` per family

This avoids resending unrelated families. Depends on spec-rib-family-ribout.

### AD-4: Partial deployment via ModAccumulator

For IBGP non-LLGR peers, the egress filter writes:
- `mods.Set("add:attr:community", []string{"no-export"})`
- `mods.Set("set:attr:local-preference", uint32(0))`

The forward path (spec-route-metadata infrastructure) applies these mods, re-encoding the UPDATE before sending. The route is delivered (not suppressed) but deprioritized.

### AD-5: Withdrawal via ModAccumulator

When the LLGR egress filter decides to suppress a stale route for an EBGP non-LLGR peer that previously received the route, the peer must receive a withdrawal (not just silence). The filter sets `mods.Set("withdraw:nlri:*", true)` instead of returning `false`. The forward path (spec-route-metadata) converts the announce to a withdrawal before sending.

This is cleaner than explicit withdrawal commands because:
- The egress filter already knows the per-peer decision (suppress vs modify vs pass)
- The mod framework handles the wire-level conversion
- No new RIB command needed
- No GR plugin peer enumeration needed

| Peer Category | Filter Returns | Mods | Effect |
|---|---|---|---|
| LLGR-capable | `true` | none | Route sent with LLGR_STALE (already in wire bytes) |
| EBGP non-LLGR | `true` | `withdraw=true` | Withdrawal sent for the NLRIs |
| IBGP non-LLGR | `true` | `add-community`, `set-local-pref` | Route sent with NO_EXPORT + LOCAL_PREF=0 |

Note: the filter always returns `true` (accept). It uses mods to control the outcome. This ensures the forward path always processes the route (either as announce, modified announce, or withdrawal).

### AD-6: IBGP vs EBGP detection in egress filter

The egress filter receives `dest.PeerAS` via `PeerFilterInfo`. The GR plugin captures `localAS` from its OnConfigure callback (already parses BGP config for restart-time). The filter closure captures `localAS`. IBGP = `dest.PeerAS == localAS`. This is the same pattern role uses (`getFilterConfig` captures per-peer config at configure time).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| LLGR entry triggers per-family rib clear out | -> | Per-family resend with metadata | `test/plugin/llgr-readvertise.ci` |
| Stale route to LLGR-capable peer | -> | Route accepted by egress filter | `TestLLGREgressFilter_LLGRPeer` |
| Stale route to EBGP non-LLGR peer | -> | Route suppressed by egress filter | `TestLLGREgressFilter_EBGPNonLLGR` |
| Stale route to IBGP non-LLGR peer | -> | Route accepted with mods (NO_EXPORT + LOCAL_PREF=0) | `TestLLGREgressFilter_IBGPPartial` |
| Non-stale route | -> | Filter passes through immediately (fast path) | `TestLLGREgressFilter_NonStale` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LLGR_STALE route, destination peer has LLGR capability | Route advertised with LLGR_STALE community included |
| AC-2 | LLGR_STALE route, EBGP destination peer lacks LLGR capability | Withdrawal sent (via mods["withdraw:nlri:*"]=true) |
| AC-3 | LLGR_STALE route readvertised | LLGR_STALE community NOT removed from attributes |
| AC-4 | Partial deployment: IBGP peer without LLGR | Route sent with NO_EXPORT + LOCAL_PREF=0 via ModAccumulator |
| AC-5 | Session re-established, routes become non-stale | Routes re-advertised normally (without LLGR_STALE) to all peers |
| AC-6 | Multiple LLGR-stale routes for same prefix | Only best among them forwarded (depreference already applied in spec-llgr-3) |
| AC-7 | No peers in LLGR state | Egress filter returns immediately (atomic fast path, zero overhead) |
| AC-8 | LLGR entry completes | Per-family `rib clear out` dispatched (not all-family) |
| AC-9 | `rib mark-stale` with level 2 | ribOut routes for that peer also have StaleLevel=2 |
| AC-10 | RIB resends stale route (StaleLevel > 0) | UpdateRouteWithMeta called with `meta["stale"]` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLLGREgressFilter_LLGRPeer` | `gr/gr_egress_test.go` | AC-1: stale route accepted for LLGR-capable peer | |
| `TestLLGREgressFilter_EBGPNonLLGR` | `gr/gr_egress_test.go` | AC-2: stale route suppressed for EBGP non-LLGR peer | |
| `TestLLGREgressFilter_CommunityPreserved` | `gr/gr_egress_test.go` | AC-3: LLGR_STALE community in wire bytes unchanged | |
| `TestLLGREgressFilter_IBGPPartial` | `gr/gr_egress_test.go` | AC-4: mods set for IBGP non-LLGR (add-community, set-local-pref) | |
| `TestLLGREgressFilter_NonStale` | `gr/gr_egress_test.go` | AC-7: non-stale route passes immediately | |
| `TestLLGREgressFilter_NoLLGRActive` | `gr/gr_egress_test.go` | AC-7: atomic fast path when no peers in LLGR | |
| `TestLLGREntryDonePerFamily` | `gr/gr_state_test.go` | AC-8: per-family rib clear out dispatched | |
| `TestLLGRReadvertisement_SessionReestablish` | `gr/gr_state_test.go` | AC-5: non-stale routes re-advertised normally | |
| `TestMarkStalePropagatesToRibOut` | `rib/rib_test.go` | AC-9: ribOut Route.StaleLevel updated | |
| `TestSendRoutesWithStaleMeta` | `rib/rib_test.go` | AC-10: UpdateRouteWithMeta called for stale routes | |
| `TestLLGREgressFilter_EBGPWithdrawal` | `gr/gr_egress_test.go` | AC-2: mods["withdraw:nlri:*"]=true for EBGP non-LLGR | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no new numeric inputs in this phase) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `llgr-readvertise` | `test/plugin/llgr-readvertise.ci` | LLGR_STALE route forwarded to capable peer, suppressed to non-LLGR EBGP | |

### Future (if deferring any tests)
- Multi-peer partial deployment functional test (requires multi-peer .ci infrastructure)

## Files to Modify

- `internal/component/bgp/plugins/gr/register.go` - add EgressFilter to Registration
- `internal/component/bgp/plugins/gr/gr.go` - LLGR egress filter function, atomic LLGR-active counter, update onLLGREntryDone to per-family, localAS capture from OnConfigure
- `internal/component/bgp/plugins/gr/gr_state.go` - increment/decrement LLGR-active counter on state transitions
- `internal/component/bgp/route.go` - add StaleLevel field to Route struct
- `internal/component/bgp/plugins/rib/rib_commands.go` - `markStaleCommand` propagates StaleLevel to ribOut routes
- `internal/component/bgp/plugins/rib/rib.go` - `sendRoutes` calls UpdateRouteWithMeta when Route.StaleLevel > 0

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/plugin/llgr-readvertise.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- LLGR readvertisement |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc9494.md` -- readvertisement section |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create

- `internal/component/bgp/plugins/gr/gr_egress.go` - LLGR egress filter function
- `internal/component/bgp/plugins/gr/gr_egress_test.go` - egress filter unit tests
- `test/plugin/llgr-readvertise.ci` - readvertisement functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + spec-route-metadata + spec-rib-family-ribout |
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

1. **Phase: LLGR egress filter** -- static registration, atomic fast path, capability check
   - Tests: `TestLLGREgressFilter_NonStale`, `TestLLGREgressFilter_NoLLGRActive`, `TestLLGREgressFilter_LLGRPeer`, `TestLLGREgressFilter_EBGPNonLLGR`
   - Files: `gr/gr_egress.go`, `gr/register.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Partial deployment mods** -- IBGP non-LLGR peers get NO_EXPORT + LOCAL_PREF=0
   - Tests: `TestLLGREgressFilter_IBGPPartial`
   - Files: `gr/gr_egress.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Stale metadata on resend** -- RIB sets `meta["stale"]` during outbound resend
   - Tests: unit test verifying meta is set during sendRoutes
   - Files: `rib/rib.go` or `rib/rib_commands.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Per-family readvertisement** -- onLLGREntryDone dispatches per-family clear out
   - Tests: `TestLLGREntryDonePerFamily`
   - Files: `gr/gr.go`, `gr/gr_state.go` (atomic counter)
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Community preservation** -- verify LLGR_STALE not stripped
   - Tests: `TestLLGREgressFilter_CommunityPreserved`
   - Files: (likely no code change -- community is in wire bytes; test verifies)
   - Verify: test passes

6. **Functional test** -- create `test/plugin/llgr-readvertise.ci`

7. **RFC refs** -- add `// RFC 9494 Section 4.5` comments

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has implementation with file:line |
| Correctness | LLGR_STALE never stripped; suppression correct per-peer; mods correct for IBGP |
| Naming | Filter function named consistently with role pattern |
| Data flow | Metadata flows from RIB -> ReceivedUpdate -> egress filter; no wire parsing |
| Rule: no-layering | Single egress filter handles all cases (not separate filters per peer type) |
| Rule: buffer-first | Mod application uses existing re-encode path |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| LLGR egress filter registered | grep `EgressFilter` in gr/register.go |
| Atomic fast path | grep `atomic` in gr/gr_egress.go |
| Stale metadata set on resend | grep `meta.*stale` in rib package |
| Per-family clear out | grep `rib clear out` in gr/gr.go |
| ModAccumulator usage | grep `mods.Set` in gr/gr_egress.go |
| Functional test | ls `test/plugin/llgr-readvertise.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Community integrity | LLGR_STALE not accidentally stripped or duplicated during mod application |
| Partial deployment | NO_EXPORT prevents route leaking beyond AS boundary |
| Atomic safety | LLGR-active counter correctly incremented/decremented on all state paths |

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

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Add `// RFC 9494 Section 4.5: "<quoted requirement>"` above peer filtering code.
MUST document: readvertisement trigger, LLGR_STALE preservation, non-LLGR suppression, partial deployment rules.

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
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
- [ ] Write learned summary to `plan/learned/NNN-llgr-4-readvertisement.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
