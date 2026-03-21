# Spec: llgr-0-umbrella

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/4 |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9494.md` - LLGR RFC
4. `rfc/short/rfc4724.md` - base GR RFC
5. `internal/component/bgp/plugins/gr/gr.go` - GR plugin entry
6. `internal/component/bgp/plugins/gr/gr_state.go` - GR state machine

## Task

Implement Long-Lived Graceful Restart (LLGR) per RFC 9494 as an extension to the existing GR plugin (`bgp-gr`). LLGR allows stale routes to be retained for much longer periods (hours/days) after the standard GR restart-time expires, with routes marked via the LLGR_STALE well-known community and deprioritized in best-path selection.

This is an umbrella spec. Implementation is in four child specs executed in order.

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| Capability negotiation | LLGR capability (code 71) in OPEN, YANG config for LLST per AFI/SAFI |
| State machine | GR-to-LLGR transition, per-family LLST timers, session re-establishment during LLGR |
| RIB integration | LLGR_STALE community attachment, NO_LLGR route deletion, stale depreference in best-path |
| Readvertisement | Re-advertise with LLGR_STALE, suppress toward non-LLGR peers, partial deployment (IBGP) |
| CLI decode | `ze plugin gr --capa <hex>` extended to decode capability code 71 |

**Out of Scope:**

| Area | Reason |
|------|--------|
| Restarting Speaker | Ze implements Receiving Speaker only (same as RFC 4724 implementation) |
| VPN ATTR_SET (RFC 6368) | Requires VPN infrastructure not yet built |
| RFC 8538 (N-bit / hard reset) | N-bit already detected in `gr.go:474`; full hard-reset handling is a separate spec |

### Child Specs

| Spec | Name | Depends | Status | Description |
|------|------|---------|--------|-------------|
| ~~`spec-llgr-1-capability.md`~~ | Capability Wire + Config | - | Done (`plan/learned/401-llgr-1-capability.md`) | LLGR capability code 71: wire decode/encode, YANG schema, config extraction, CLI decode |
| ~~`spec-llgr-2-state-machine.md`~~ | State Machine | llgr-1 | Done (`plan/learned/402-llgr-2-state-machine.md`) | GR-to-LLGR transition, per-family LLST timers, reconnect during LLGR, timer interactions |
| ~~`spec-llgr-3-rib-integration.md`~~ | RIB Integration | llgr-2 | Done (`plan/learned/403-llgr-3-rib-integration.md`) | LLGR_STALE attachment, NO_LLGR deletion, best-path depreference, generic RIB commands |
| `spec-llgr-4-readvertisement.md` | Readvertisement | llgr-3 | design | Re-advertise stale routes with LLGR_STALE, suppress to non-LLGR peers, partial deployment |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall architecture, plugin model
  → Constraint: plugins coordinate via inter-plugin commands, not direct imports
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage, pool dedup
  → Constraint: attributes stored as pool handles; mutation = new handle + release old

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - LLGR: capability code 71, LLST, LLGR_STALE/NO_LLGR communities
  → Constraint: MUST also advertise GR capability (code 64) or LLGR MUST be ignored
  → Constraint: LLGR_STALE routes treated as least preferred in route selection
  → Constraint: MUST NOT enable by default, requires explicit per-AFI/SAFI config
- [ ] `rfc/short/rfc4724.md` - base GR: capability code 64, restart-time, F-bit
  → Constraint: LLGR extends GR; GR timer expiry triggers LLGR period

**Key insights:**
- LLGR is a second phase after GR: session fails -> GR period (restart-time) -> LLGR period (LLST) -> delete
- LLGR capability code 71 has 7-byte tuples: AFI(2) + SAFI(1) + Flags(1) + LLST(3)
- Two well-known communities: LLGR_STALE (0xFFFF0006), NO_LLGR (0xFFFF0007)
- If GR restart-time=0 but LLST nonzero, skip GR and go straight to LLGR
- LLGR_STALE routes SHOULD NOT be advertised to peers without LLGR capability

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/gr/gr.go` - GR plugin: handles OPEN/state/EOR events, dispatches rib commands
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - state machine: per-peer stale families, restart timer, session lifecycle
- [ ] `internal/component/bgp/plugins/gr/register.go` - registers cap code 64, depends on bgp-rib
- [ ] `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` - restart-time leaf (0-4095)
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - RIB GR commands: retain/release/mark-stale/purge-stale
- [ ] `internal/component/bgp/plugins/rib/rib.go` - peerGRState struct, RIBManager, autoExpireStale
- [ ] `internal/component/bgp/attribute/community.go` - Community type, well-known constants, Communities type

**Behavior to preserve:**
- All existing RFC 4724 GR behavior (retain, mark-stale, purge-stale, EOR handling)
- GR capability code 64 wire format and config parsing
- Existing inter-plugin command protocol (rib retain-routes, release-routes, mark-stale, purge-stale)
- Community type and well-known constant naming pattern
- Best-path selection algorithm (extended, not replaced)

**Behavior to change:**
- GR timer expiry: currently purges all stale; will check for LLGR and transition instead
- Register both capability codes (64, 71) in bgp-gr plugin
- YANG schema: add long-lived-stale-time per AFI/SAFI under graceful-restart container
- Best-path: add LLGR-stale depreference as first comparison step

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- LLGR capability arrives in OPEN message as capability code 71 (hex bytes in JSON event)
- LLGR config enters via YANG schema in config file (long-lived-stale-time per family)

### Transformation Path
1. Config parse: YANG schema -> config JSON -> `extractGRCapabilities` extended to also produce code-71 capabilities
2. OPEN decode: received OPEN event -> `handleOpenEvent` extended to also parse code-71 tuples, store LLST per family
3. Timer expiry: GR restart-time fires -> state machine checks for LLGR -> transitions to LLGR period
4. LLGR entry: state machine dispatches `rib enter-llgr` -> RIB attaches LLGR_STALE, deletes NO_LLGR, starts LLST timer
5. Best-path: route selection checks LLGRStale flag -> deprioritizes stale routes
6. Readvertisement: stale routes re-advertised with LLGR_STALE to LLGR-capable peers, suppressed to others

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Plugin | JSON config section in Stage 2 OnConfigure callback | [ ] |
| OPEN -> Plugin | JSON event with capability hex in `open.capabilities` array | [ ] |
| bgp-gr -> bgp-rib | Inter-plugin DispatchCommand (text commands) | [ ] |
| RIB -> Best-path | LLGRStale flag on RouteEntry checked in SelectBest | [ ] |
| RIB -> Forward path | Re-advertise trigger for LLGR_STALE routes | [ ] |

### Integration Points
- `gr.go:handleOpenEvent` - extend to parse cap code 71 alongside code 64
- `gr_state.go:grStateManager` - extend to track LLGR state and per-family LLST timers
- `gr_state.go:handleTimerExpired` - change from purge-all to check-for-LLGR-transition
- `rib_commands.go:handleCommand` - add new LLGR commands
- `rib.go:peerGRState` - extend with LLGR fields
- `storage/routeentry.go:RouteEntry` - add LLGRStale flag
- `bestpath.go:SelectBest` - add LLGR-stale depreference step
- `attribute/community.go` - add LLGR_STALE and NO_LLGR constants

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Architecture Decisions

### AD-1: Extend bgp-gr, not a new plugin

LLGR is tightly coupled to GR state transitions (LLGR begins when GR timer expires). The "delete the folder" test: if `bgp-gr/` is deleted, LLGR should also disappear. A separate plugin would require complex cross-plugin state coordination for what is a single lifecycle.

**Consequence:** `bgp-gr` registers both capability codes (64, 71). RFCs field becomes `["4724", "9494"]`.

### AD-2: LLGR_STALE and NO_LLGR as well-known community constants

Add to `attribute/community.go` alongside existing CommunityNoExport, CommunityNoAdvertise, etc. Value 0xFFFF0006 for LLGR_STALE, 0xFFFF0007 for NO_LLGR.

### AD-3: New RIB commands for LLGR actions

Extend the existing inter-plugin command protocol between `bgp-gr` and `bgp-rib`:

| Command | Purpose |
|---------|---------|
| `rib enter-llgr <peer> <family> <llst>` | Transition family to LLGR: attach LLGR_STALE, delete NO_LLGR routes, start LLST timer |
| `rib depreference-stale <peer>` | Mark stale routes as least-preferred in best-path selection |

### AD-4: Depreference via flag, not LOCAL_PREF mutation

Add an `LLGRStale bool` flag to `storage.RouteEntry`. The best-path comparator checks this flag first: any non-LLGR-stale route beats any LLGR-stale route, regardless of other attributes. Between two LLGR-stale routes, normal tiebreaking applies.

**Rationale:** Avoids pool attribute mutation, keeps depreference reversible if routes become non-stale on reconnect, and matches RFC 9494 semantics ("treat as least preferred" is not the same as "set LOCAL_PREF=0").

### AD-5: Community attachment via pool handle update

When LLGR begins, the RIB attaches LLGR_STALE to stale routes by:
1. Reading existing community pool handle
2. Creating new community set = existing + LLGR_STALE
3. Storing new handle in pool (deduplicates if same set already exists)
4. Updating route entry's community handle
5. Releasing old handle

This follows the same pattern as any attribute update in the pool architecture.

### AD-6: Timer interaction (GR restart-time = 0)

Per RFC 9494: if GR restart-time is 0 but LLST is nonzero, skip the GR period entirely and go straight to LLGR. The state machine must handle this edge case.

| GR Restart Time | LLST | Behavior |
|-----------------|------|----------|
| 0 | nonzero | Skip GR, enter LLGR immediately on session drop |
| nonzero | 0 | GR only, no LLGR (current behavior) |
| 0 | 0 | Neither GR nor LLGR |
| nonzero | nonzero | GR then LLGR (serial) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with long-lived-stale-time | -> | YANG parse + capability extraction | `test/parse/graceful-restart-llgr.ci` (spec-llgr-1) |
| OPEN with cap 71 hex | -> | decodeGR extended for LLGR | `test/decode/capability-llgr.ci` (spec-llgr-1) |
| GR timer expiry with LLGR negotiated | -> | LLGR state transition | `test/plugin/llgr-transition.ci` (spec-llgr-2) |
| LLGR period entry | -> | RIB LLGR_STALE attachment + NO_LLGR deletion | `test/plugin/llgr-rib-stale.ci` (spec-llgr-3) |
| Best-path with LLGR-stale candidate | -> | Depreference in SelectBest | Unit test in `bestpath_test.go` (spec-llgr-3) |
| LLGR readvertisement | -> | UPDATE with LLGR_STALE to capable peers | `test/plugin/llgr-readvertise.ci` (spec-llgr-4) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `long-lived-stale-time 3600` under graceful-restart per family | LLGR capability (code 71) advertised in OPEN with correct LLST value |
| AC-2 | Received OPEN with capability code 71 | LLGR capability decoded and stored per-peer with per-family LLST |
| AC-3 | GR restart-time expires, LLGR negotiated | State transitions to LLGR period instead of purging all stale |
| AC-4 | LLGR period begins for a family | LLGR_STALE community attached to stale routes, NO_LLGR routes deleted |
| AC-5 | Route selection with LLGR-stale and normal candidates | Normal route always preferred over LLGR-stale route |
| AC-6 | LLST timer expires | Stale routes for that family deleted |
| AC-7 | Session re-established during LLGR | RFC 4724 procedures apply; missing/F=0 families purged |
| AC-8 | GR restart-time=0, LLST nonzero | LLGR entered immediately on session drop (no GR period) |
| AC-9 | LLGR_STALE route with non-LLGR peer | Route NOT advertised to that peer |
| AC-10 | CLI decode of capability code 71 hex | Human-readable output with LLST per family |

## 🧪 TDD Test Plan

Tests are defined in individual child specs. Each child spec has its own TDD plan.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| See child specs | See child specs | Each child has unit tests for its phase | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LLST | 0-16777215 | 16777215 | N/A | 16777216 |
| Capability code 71 tuple length | 7 bytes per family | 7*N | 6 (truncated) | N/A (ignored) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| See child specs | See child specs | Each child has functional tests for its phase | |

### Future (if deferring any tests)
- VPN ATTR_SET handling (requires VPN infrastructure)
- Restarting Speaker procedures (ze only implements Receiving Speaker)

## Files to Modify

- `internal/component/bgp/plugins/gr/register.go` - add cap code 71, RFC 9494
- `internal/component/bgp/plugins/gr/gr.go` - LLGR cap parsing, LLGR event handling
- `internal/component/bgp/plugins/gr/gr_state.go` - LLGR state machine extensions
- `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` - LLST config
- `internal/component/bgp/plugins/rib/rib_commands.go` - new LLGR commands
- `internal/component/bgp/plugins/rib/rib.go` - peerGRState LLGR fields
- `internal/component/bgp/plugins/rib/storage/routeentry.go` - LLGRStale flag
- `internal/component/bgp/plugins/rib/bestpath.go` - depreference step
- `internal/component/bgp/attribute/community.go` - LLGR_STALE, NO_LLGR constants

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` |
| RPC count in architecture docs | [ ] | `docs/architecture/api/architecture.md` |
| CLI commands/flags | [ ] | Existing `ze plugin gr --capa` extended |
| CLI usage/help text | [ ] | Same as above |
| API commands doc | [x] | `docs/architecture/api/commands.md` (new rib commands) |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/llgr-*.ci` |

## Files to Create

- `test/parse/graceful-restart-llgr.ci` - config parsing test for LLGR
- `test/decode/capability-llgr.ci` - capability code 71 decode test
- `test/plugin/llgr-transition.ci` - GR-to-LLGR transition test
- `test/plugin/llgr-rib-stale.ci` - RIB LLGR_STALE attachment test
- `test/plugin/llgr-readvertise.ci` - readvertisement test

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + relevant child spec |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Child spec phases (one child at a time) |
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

Each phase corresponds to a child spec, executed in order.

1. **Phase: Capability (spec-llgr-1)** -- wire decode/encode, YANG, config, CLI decode
   - Tests: see spec-llgr-1 TDD plan
   - Files: gr.go, register.go, YANG schema, community.go
   - Verify: capability parsed from OPEN, generated from config

2. **Phase: State Machine (spec-llgr-2)** -- GR-to-LLGR transition, timers
   - Tests: see spec-llgr-2 TDD plan
   - Files: gr_state.go, gr.go
   - Verify: GR timer expiry transitions to LLGR instead of purge

3. **Phase: RIB Integration (spec-llgr-3)** -- community attachment, depreference
   - Tests: see spec-llgr-3 TDD plan
   - Files: rib_commands.go, rib.go, routeentry.go, bestpath.go
   - Verify: LLGR_STALE attached, NO_LLGR deleted, best-path depreferences

4. **Phase: Readvertisement (spec-llgr-4)** -- re-advertise, peer filtering
   - Tests: see spec-llgr-4 TDD plan
   - Files: forward path files (TBD in spec-llgr-4 research)
   - Verify: stale routes re-advertised with LLGR_STALE to capable peers only

5. **Functional tests** -- Create after each phase works. Cover user-visible behavior.

6. **Full verification** -- `make ze-verify`

7. **Complete spec** -- Fill audit tables, write learned summary to `plan/learned/`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N from all child specs has implementation with file:line |
| Correctness | LLGR timer values correct (24-bit LLST), community values match RFC |
| Naming | JSON keys use kebab-case, YANG uses kebab-case, Go constants follow existing pattern |
| Data flow | LLGR state managed in bgp-gr only, RIB receives commands (no direct state access) |
| Rule: no-layering | GR timer expiry path fully replaced (not duplicated) for LLGR case |
| Rule: buffer-first | Any new wire encoding uses WriteTo(buf, off) pattern |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| LLGR capability decode works | `test/decode/capability-llgr.ci` passes |
| LLGR config parses | `test/parse/graceful-restart-llgr.ci` passes |
| GR-to-LLGR transition | `test/plugin/llgr-transition.ci` passes |
| LLGR_STALE community constants | grep for CommunityLLGRStale in `attribute/community.go` |
| Best-path depreference | grep for LLGRStale in `bestpath.go` |
| New RIB commands registered | grep for `enter-llgr` in `rib_commands.go` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | LLST from OPEN: validate 24-bit range, reject truncated tuples |
| Timer bounds | LLST up to ~194 days: ensure no integer overflow in Duration conversion |
| Community injection | LLGR_STALE attachment must not corrupt existing community data |
| Resource exhaustion | LLGR retains routes much longer; ensure RIB memory is bounded by existing per-peer limits |

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

## Cross-References

| Document | Relevance |
|----------|-----------|
| `rfc/short/rfc9494.md` | Primary RFC |
| `rfc/short/rfc4724.md` | Base GR (existing implementation) |
| `plan/learned/350-gr-receiving-speaker.md` | GR receiving speaker implementation |
| `plan/learned/353-gr-plugin-arch.md` | GR plugin architecture |
| `plan/learned/378-gr-mechanism.md` | Per-route stale tracking |

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Pool attribute mutation complexity (AD-5) | Medium | High | Phase 3 addresses isolated; same pattern as any attr update |
| Readvertisement trigger mechanism missing | Medium | High | Phase 4 investigates existing forward path; may need new command |
| Timer interaction edge cases | Low | Medium | Comprehensive boundary tests in Phase 2 |
| Best-path depreference ordering | Low | Medium | Unit tests with mixed LLGR-stale and normal candidates |

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

Add `// RFC 9494 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: LLGR capability format, LLST timer constraints, community handling, depreference rules.

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- AD-3 revised: spec designed `rib enter-llgr` and `rib depreference-stale` as dedicated commands. Implementation uses generic composable commands: `rib attach-community`, `rib delete-with-community`, `rib mark-stale [level]`. Better design -- RIB has no LLGR knowledge.
- AD-4 revised: spec designed `LLGRStale bool` on RouteEntry. Implementation uses `StaleLevel uint8` (0=fresh, 1=GR-stale, 2+=LLGR-stale) with `DepreferenceThreshold = 2`. More general, supports future extensions.
- Phases 1-3 completed retroactively (summaries in plan/learned/401-403). Phase 4 (readvertisement) remains.

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
- [ ] Write learned summary to `plan/learned/NNN-llgr-0-umbrella.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
