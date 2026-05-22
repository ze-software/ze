# Spec: mpls-3-rsvp-te

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-mpls-1-kernel |
| Phase | 3 of 3 (MPLS) |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/fib/kernel/fibkernel.go` - kernel FIB backend (MPLS from phase 1)
4. `internal/component/ldp/` - LDP component (if phase 2 implemented, reference for MPLS signaling)

## Task

Implement RSVP-TE (Resource Reservation Protocol - Traffic Engineering) as a Ze
component. RSVP-TE establishes explicitly-routed MPLS LSPs with bandwidth
reservation. This is the traffic engineering layer: operators define LSPs with
explicit paths, bandwidth constraints, and protection, and RSVP-TE signals them
hop-by-hop.

RSVP-TE is significantly more complex than LDP. Where LDP distributes labels for
IGP shortest-path forwarding, RSVP-TE builds constraint-based tunnels with
admission control and protection switching.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component registration pattern
- [ ] `internal/component/ike/` - reference for complex protocol engine (FSM, crypto, wire)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3209.md` - RSVP-TE: Extensions to RSVP for LSP Tunnels (MUST CREATE)
  → Constraint: PATH/RESV message flow, LABEL_REQUEST/LABEL objects, ERO/RRO
  → Constraint: SESSION object identifies LSP (tunnel endpoint, tunnel ID, extended tunnel ID)
- [ ] `rfc/short/rfc2205.md` - RSVP (base protocol) (MUST CREATE)
  → Constraint: soft-state model, refresh timer, PATH/RESV/PathTear/ResvTear/PathErr/ResvErr
  → Constraint: RSVP runs directly on IP (protocol 46), not TCP/UDP
- [ ] `rfc/short/rfc4090.md` - Fast Reroute for RSVP-TE (SHOULD CREATE)
  → Constraint: facility backup (bypass tunnels), one-to-one backup (detour LSPs)
- [ ] `rfc/short/rfc3630.md` - TE Extensions to OSPF (SHOULD CREATE)
  → Constraint: TE metric, max bandwidth, max reservable bandwidth
- [ ] `rfc/short/rfc3031.md` - MPLS Architecture (from phase 1)

**Key insights:**
- RSVP-TE is soft-state: PATH and RESV messages must be refreshed (default 30s)
- PATH flows downstream (ingress to egress), RESV flows upstream (egress to ingress)
- ERO (Explicit Route Object): strict/loose hops defining the path
- RRO (Record Route Object): records actual path taken
- Label allocation: egress allocates label in RESV, each hop allocates and swaps
- Make-before-break: new LSP established before old torn down (RFC 3209 Section 6)
- Bandwidth admission: per-interface available bandwidth tracked, reject if oversubscribed
- IP protocol 46, no TCP/UDP
- No IGP in Ze: RSVP-TE uses static EROs or CSPF over BGP-LS data

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] Ze has no RSVP-TE implementation today
- [ ] Ze has no IGP (OSPF/IS-IS) with TE extensions
- [ ] Ze has BGP-LS which could provide TE topology for CSPF

**Behavior to preserve:**
- BGP labeled unicast and LDP (if implemented) remain independent
- Kernel MPLS FIB programming from phase 1 unchanged

**Behavior to change:**
- No RSVP-TE support exists; this is entirely new
- No LSP tunnel abstraction exists
- No bandwidth admission control exists

## Data Flow (MANDATORY)

### Entry Point
- Config: LSP tunnel definitions with ERO, bandwidth, priority
- Wire: RSVP PATH/RESV messages from neighbors (IP protocol 46)

### Transformation Path
1. **Ingress (head-end):** operator configures LSP -> RSVP-TE builds PATH with ERO, LABEL_REQUEST
2. **PATH processing (transit):** receive PATH, validate ERO, install state, forward downstream
3. **Egress (tail-end):** receive PATH, allocate label, send RESV upstream with LABEL object
4. **RESV processing (transit):** receive RESV, allocate local label, program swap, forward upstream
5. **Head-end RESV:** receive RESV, program push entry, LSP is up
6. **Refresh:** periodic PATH/RESV refresh maintains soft-state
7. **Teardown:** PathTear/ResvTear removes state hop-by-hop

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> RSVP engine | Raw IP socket (protocol 46) | [ ] |
| RSVP engine -> MPLS FIB | Label operations via sysrib -> fibkernel | [ ] |
| Config -> RSVP engine | LSP tunnel definitions | [ ] |
| RSVP engine -> CLI/web | LSP state, bandwidth utilization | [ ] |

### Integration Points
- New component: `internal/component/rsvpte/` (engine, wire, FSM, admission)
- Config: YANG schema under `protocol { rsvp-te { ... } }`
- Raw IP socket: protocol 46, requires CAP_NET_RAW or root
- Sysrib: LSP as a route source (tunnel next-hop)
- Kernel FIB: MPLS swap/push/pop entries from phase 1
- BGP-LS: optional CSPF using TE topology data
- CLI: `show rsvp-te session`, `show rsvp-te interface`, `show rsvp-te tunnel`
- Web: LSP topology visualization, bandwidth utilization
- Metrics: LSP count, bandwidth reserved/available, setup/teardown rates

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| config `protocol { rsvp-te { ... } }` | -> | RSVP-TE component startup | `TestRSVPTEComponentStart` |
| raw IP packet received (proto 46) | -> | PATH message processing | `TestRSVPTEPathReceive` |
| LSP config with ERO | -> | PATH message sent | `TestRSVPTEPathSend` |
| RESV received at head-end | -> | label push programmed | `TestRSVPTELabelPush` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LSP configured with ERO and bandwidth | PATH message sent toward egress along ERO |
| AC-2 | PATH received at egress | Label allocated, RESV sent upstream |
| AC-3 | RESV received at transit | Label swap entry programmed, RESV forwarded upstream |
| AC-4 | RESV received at head-end | Label push entry programmed, LSP operational |
| AC-5 | Refresh timer fires | PATH/RESV refreshed, soft-state maintained |
| AC-6 | Link failure on LSP path | PathErr sent to head-end |
| AC-7 | Make-before-break reroute | New LSP signaled before old torn down, traffic shifted |
| AC-8 | Bandwidth oversubscription | PathErr with admission control failure |
| AC-9 | `show rsvp-te session` | Display all LSPs with state, bandwidth, ERO, RRO |
| AC-10 | `show rsvp-te interface` | Per-interface bandwidth (reserved, available, max) |
| AC-11 | PathTear received | MPLS entries removed, bandwidth released |
| AC-12 | Interop with FRR rsvpd | LSP established, labels exchanged, traffic forwarded |
| AC-13 | FRR (fast reroute) with bypass tunnel | Local repair on link failure (deferrable) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRSVPPathEncode` | `internal/component/rsvpte/wire_test.go` | PATH message encoding | |
| `TestRSVPPathDecode` | `internal/component/rsvpte/wire_test.go` | PATH message decoding | |
| `TestRSVPResvEncode` | `internal/component/rsvpte/wire_test.go` | RESV message encoding | |
| `TestRSVPEROEncode` | `internal/component/rsvpte/wire_test.go` | ERO object encoding | |
| `TestRSVPRRODecode` | `internal/component/rsvpte/wire_test.go` | RRO object decoding | |
| `TestRSVPLabelObject` | `internal/component/rsvpte/wire_test.go` | LABEL object encode/decode | |
| `TestRSVPSessionFSM` | `internal/component/rsvpte/fsm_test.go` | LSP state transitions | |
| `TestRSVPAdmission` | `internal/component/rsvpte/admission_test.go` | Bandwidth accept/reject | |
| `TestRSVPRefreshTimer` | `internal/component/rsvpte/refresh_test.go` | Soft-state refresh | |
| `TestRSVPMakeBeforeBreak` | `internal/component/rsvpte/reroute_test.go` | SE-style reroute | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Label | 0-1048575 | 1048575 | N/A | 1048576 |
| Tunnel ID | 0-65535 | 65535 | N/A | 65536 |
| Bandwidth (IEEE float) | 0-max | N/A | negative | N/A |
| Priority (setup/hold) | 0-7 | 7 | N/A | 8 |
| Refresh period | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rsvpte-lsp-setup` | `test/rsvpte/rsvpte-lsp-setup.ci` | LSP establishment end-to-end | |
| `rsvpte-lsp-teardown` | `test/rsvpte/rsvpte-lsp-teardown.ci` | LSP graceful teardown | |
| `rsvpte-bandwidth` | `test/rsvpte/rsvpte-bandwidth.ci` | Bandwidth admission control | |
| `rsvpte-reroute` | `test/rsvpte/rsvpte-reroute.ci` | Make-before-break | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `rsvpte-lsp-frr` | `test/interop/scenarios/` | FRR | LSP setup, label exchange | |
| `rsvpte-bandwidth-frr` | `test/interop/scenarios/` | FRR | Bandwidth admission | |
| `rsvpte-reroute-frr` | `test/interop/scenarios/` | FRR | Make-before-break reroute | |

### Future (if deferring any tests)
- AC-13 (fast reroute with bypass tunnels): significant standalone complexity, defer to follow-up spec

## Files to Modify
- `internal/plugins/sysrib/sysrib.go` - accept tunnel routes from RSVP-TE
- `cmd/ze/hub/main.go` - wire RSVP-TE component startup

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/yang/modules/ze-rsvp-te.yang` |
| CLI commands/flags | [x] | `show rsvp-te session`, `show rsvp-te interface`, `show rsvp-te tunnel` |
| CLI grammar (action before identifier) | [x] | `.claude/rules/cli-grammar.md` |
| Editor autocomplete | [x] | YANG-driven |
| Functional test for new RPC/API | [x] | `test/rsvpte/*.ci` |
| Doctor check for runtime dependencies | [x] | CAP_NET_RAW for IP protocol 46 |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [x] | `docs/guide/rsvp-te.md` |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc3209.md`, `rfc/short/rfc2205.md` |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` (new component) |

## Files to Create
- `internal/component/rsvpte/` - entire RSVP-TE component (wire, FSM, admission, engine)
- `internal/yang/modules/ze-rsvp-te.yang` - RSVP-TE config schema
- `test/rsvpte/*.ci` - functional tests
- `rfc/short/rfc3209.md` - RSVP-TE summary
- `rfc/short/rfc2205.md` - RSVP base protocol summary

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
| 7-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register RSVP-TE component, write failing wiring tests
   - Tests: `TestRSVPTEComponentStart`
   - Files: `internal/component/rsvpte/register.go`
   - Verify: component registers, engine is a stub
2. **Phase: Wire codec** -- RSVP message and object encoding/decoding
   - Tests: `TestRSVPPathEncode`, `TestRSVPPathDecode`, `TestRSVPResvEncode`, `TestRSVPEROEncode`, `TestRSVPRRODecode`, `TestRSVPLabelObject`
   - Files: `internal/component/rsvpte/wire.go`
   - Verify: round-trip for all message types
3. **Phase: Raw socket** -- IP protocol 46 listener, Router Alert option
   - Files: `internal/component/rsvpte/transport.go`
   - Verify: send/receive RSVP packets
4. **Phase: PATH/RESV FSM** -- per-LSP state machine (PSB, RSB)
   - Tests: `TestRSVPSessionFSM`
   - Files: `internal/component/rsvpte/fsm.go`
   - Verify: state transitions match RFC 3209
5. **Phase: Label allocation** -- egress label, transit swap, head-end push
   - Tests: `TestRSVPTELabelPush`
   - Files: `internal/component/rsvpte/label.go`
   - Verify: labels programmed via sysrib -> fibkernel
6. **Phase: Refresh** -- periodic PATH/RESV refresh, state cleanup on timeout
   - Tests: `TestRSVPRefreshTimer`
   - Files: `internal/component/rsvpte/refresh.go`
   - Verify: soft-state maintained, expired state cleaned
7. **Phase: Admission control** -- per-interface bandwidth tracking
   - Tests: `TestRSVPAdmission`
   - Files: `internal/component/rsvpte/admission.go`
   - Verify: accept/reject based on available bandwidth
8. **Phase: Teardown** -- PathTear/ResvTear, graceful deletion
   - Files: `internal/component/rsvpte/teardown.go`
   - Verify: state and FIB entries cleaned up
9. **Phase: Make-before-break** -- SE-style LSP reroute
   - Tests: `TestRSVPMakeBeforeBreak`
   - Files: `internal/component/rsvpte/reroute.go`
   - Verify: new LSP before old teardown
10. **Phase: Config** -- YANG schema, LSP tunnel definitions
    - Files: `ze-rsvp-te.yang`, config parser
    - Verify: config accepted and drives LSP setup
11. **Phase: CLI/Web** -- show commands, LSP visualization
    - Verify: `show rsvp-te session` displays LSPs
12. **Phase: Metrics** -- Prometheus counters
13. **Phase: Interop** -- FRR rsvpd test scenarios
14. **Phase: Fast reroute** (optional, deferrable) -- facility backup bypass tunnels

### Scoping Recommendation

A minimal viable RSVP-TE could defer:
- Fast reroute (AC-13): significant standalone complexity
- CSPF / automatic path computation: use explicit ERO only
- Summary refresh (RFC 2961): periodic refresh works at small scale
- P2MP LSPs (RFC 4875): point-to-point covers the primary use case
- Preemption: all LSPs at same priority initially

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Wire format matches RFC 3209/2205 exactly |
| Naming | YANG uses kebab-case, CLI uses `show rsvp-te <noun>` |
| Data flow | Labels flow RSVP-TE -> sysrib -> fibkernel, no bypass |
| CLI grammar | All commands follow action-before-identifier |
| Doctor checks | CAP_NET_RAW check registered |
| Soft-state | Refresh timers correctly maintain state |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| RSVP-TE component directory | `ls internal/component/rsvpte/` |
| YANG module | `ls internal/yang/modules/ze-rsvp-te.yang` |
| Functional tests | `ls test/rsvpte/*.ci` |
| RFC summaries | `ls rfc/short/rfc3209.md rfc/short/rfc2205.md` |
| Interop test | `ls test/interop/scenarios/rsvpte-lsp-frr/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | All RSVP objects validated (length, type) before processing |
| Authentication | RSVP integrity (RFC 2747) for message authentication |
| Resource exhaustion | Maximum LSP/session limits, bandwidth accounting overflow |
| Privilege | Raw IP socket requires CAP_NET_RAW |
| Soft-state | Refresh timeout must clean up all state (no leak on malformed teardown) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
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

Add `// RFC 3209 Section X.Y: "<quoted requirement>"` above enforcing code.
Add `// RFC 2205 Section X.Y: "<quoted requirement>"` for base RSVP behavior.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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
| LSP establishment | functional test | `rsvpte-lsp-setup.ci` |
| Bandwidth admission | functional test | `rsvpte-bandwidth.ci` |
| Make-before-break | functional test | `rsvpte-reroute.ci` |
| FRR interop | interop test | `rsvpte-lsp-frr` |

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
- [ ] AC-1..AC-12 all demonstrated (AC-13 deferrable with user approval)
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-mpls-rsvp-te.md`
- [ ] Summary included in commit
