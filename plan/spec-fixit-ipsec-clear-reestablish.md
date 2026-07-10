# Spec: fixit-ipsec-clear-reestablish

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
3. `internal/component/ike/cmd/ipsec.go`, `internal/component/ike/engine/register.go`, `internal/component/ike/engine/reconcile.go`, `internal/component/ike/engine/fsm.go` - the clear/re-establish/responder-accept paths
4. `plan/learned/745-ipsec-10-cli-diag.md` - clear-command design context

## Task

Fix the product gap where `clear vpn ipsec sa` does not re-establish a
site-to-site tunnel when the peer is a separate, still-established responder.

**Observed behavior (2026-07-10, two unprivileged ze instances over loopback,
initiator + responder, PSK, noop dataplane):** after the initiator processes an
operator `clear vpn ipsec sa`, it re-sends `IKE_SA_INIT` (confirmed: 2 sent),
but the responder -- which never ran `clear` and still holds the peer's original
SA -- accepts only the first init (confirmed: 1 accepted) and no second SA
establishes. The tunnel that the operator intended to bounce stays down until
something else re-triggers it. Evidence captured while implementing
`spec-test-coverage-gaps` (`plan/learned/` follow-up; the two-daemon
`test/ipsec/ipsec-sa-installed.ci` documents the deferral in a `test-relax`
note).

Root cause is a **hypothesis to confirm during research**, not an established
fact: the responder's inbound-init path appears to reject or misroute a fresh
`IKE_SA_INIT` (new initiator SPI, zero responder SPI) while it already owns an
established SA for that configured peer. RFC 7296 allows a peer to start a new
IKE SA; the responder must either accept the new SA (superseding or coexisting
with the stale one) or the stale SA must be torn down so the new init is
treated as a first initiation.

### Scope

- IN: make an operator `clear vpn ipsec sa` (all-SAs and per-peer) result in a
  re-established tunnel against a live responder, for `connection-type initiate`
  peers, within a bounded time.
- IN: a functional test proving it end to end (two ze instances).
- OUT: rekey-timer-driven re-negotiation (that path works); MOBIKE; the NAT-T
  `0.0.0.0:4500` hardcoded-port note (tracked separately with the IKE test-port
  seam work).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint:. -->
- [ ] `plan/learned/745-ipsec-10-cli-diag.md` - clear-command design
  -> Constraint: (fill during research) what the clear command was designed to do on each side
- [ ] `rfc/short/rfc7296.md` (IKEv2) - Section 1.2 (initial exchanges), 2.4 (state synchronization / dead peer), 2.8 (rekeying)
  -> Constraint: a responder receiving a new IKE_SA_INIT for a peer it already has an SA with must not silently drop it; define the accept/supersede rule this fix implements
- [ ] `ai/rules/interop-and-goal-validation.md` - strongSwan interop obligations for IPsec
  -> Constraint: if wire behavior changes, an interop scenario is required (see TDD Test Plan)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` - verify the summary exists; create if missing before asserting RFC behavior
  -> Constraint: cite the exact section the fix enforces above the enforcing code

**Key insights:** (fill during research)

## Current Behavior (MANDATORY)

**Source files read (2026-07-10, spec author):**
- [ ] `internal/component/ike/cmd/ipsec.go` (:17-44 handleClearIPsecSA) - `clear vpn ipsec sa` -> `engine.TerminateAllSAs()`; `clear vpn ipsec sa peer <name>` -> `engine.TerminatePeerSA(name)`
- [ ] `internal/component/ike/engine/register.go` (:72-104 TerminateAllSAs, :106-135 TerminatePeerSA) - stops each PeerSession, removes its SA from the SATable, emits sa-down, then calls `reEstablishFn` (:100-102 / :131-133)
- [ ] `internal/component/ike/engine/register.go` (:199-213 reEstablish closure) - `reEstablish` -> `reconcilePeers(rc.cfg, nil, activePeers, table, rc.tr, eb, log)`; `reEstablishFn.Store(&reEstablish)` (:213)
- [ ] `internal/component/ike/engine/reconcile.go` (:176-250 reconcilePeers) - for each desired peer not currently `active`, `startPeerSession(...)` (:245) -- this DOES re-initiate for `initiate`-type peers, matching the observed 2nd IKE_SA_INIT
- [ ] `internal/component/ike/engine/register.go` (:527-564 tryResponderSAInit) - responder accept path: guards on IKE_SA_INIT request + zero responder SPI (:530-538), `matchResponderPeer` (:539), `responderBusy` CAS (:546-549), `newResponderSA` + `table.Insert` (:550-560); logs "accepting inbound IKE_SA_INIT" (:562)
- [ ] `internal/component/ike/engine/fsm.go` (:185 responderBusy reset on "responder ready") - the busy gate frees after establishment, so it is NOT the drop cause for a post-establishment init

**Behavior to preserve:**
- Rekey-timer re-negotiation, single-in-flight-handshake gating (`responderBusy`, register.go:546), SATable identity semantics, existing sa-up/sa-down event emission.
- `clear vpn ipsec sa peer <name>` and all-SAs both keep their JSON result shape (`action`/`terminated`/`peer`).

**Behavior to change:**
- After an operator clear, the tunnel must re-establish against a live responder. The exact source change is determined during research (responder accepts the new init / stale-SA teardown / initiator signals the peer). Do not presuppose the fix location beyond the paths above.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- CLI/RPC `clear vpn ipsec sa [peer <name>]` (`ze-clear:vpn-ipsec-sa`, ipsec.go:13)

### Transformation Path
1. Handler -> `TerminateAllSAs` / `TerminatePeerSA` (register.go)
2. Local SATable teardown + sa-down emit + `reEstablishFn`
3. `reconcilePeers` -> `startPeerSession` re-initiates for initiate-type peers
4. Wire: initiator IKE_SA_INIT -> responder `tryResponderSAInit` (register.go:527)
5. Responder either accepts (new SA) or the packet is dropped/misrouted (the bug)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI/RPC -> engine | dispatch-command `ze-clear:vpn-ipsec-sa` | [ ] |
| initiator -> responder | UDP IKE_SA_INIT (RFC 7296) | [ ] |
| engine -> SATable | Insert/Remove by SPI pair | [ ] |

### Integration Points
- `internal/component/ike/engine/` SATable, PeerSession, reconcile, responder accept
- Event bus sa-up/sa-down (`internal/component/ike/engine/events.go`)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The initiator already re-initiates after clear (reconcilePeers -> startPeerSession) | reconcile.go:227-250; observed 2 IKE_SA_INIT sent | fix must also cover the initiator side | packet-count trace / unit test | unvalidated |
| A-2 | The drop is on the responder inbound-init path while it holds a stale established SA | tryResponderSAInit register.go:527-564; observed 1 accepted of 2 | fix is elsewhere (dispatch routing, SATable lookup) | add responder-side trace at the drop point during research | unvalidated |
| A-3 | RFC 7296 permits accepting a new IKE_SA_INIT superseding/coexisting with the stale SA | RFC 7296 Section 1.2 / 2.8 | narrow the fix to stale-SA teardown before accept | rfc/short/rfc7296.md summary | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Accepting new inits opens a DoS / SA-churn vector (an attacker forces re-negotiation) | SA table growth, CPU under repeated inits | keep `responderBusy` single-in-flight gating; supersede rather than accumulate; rate-limit if needed |
| R-2 | Superseding a live SA drops in-flight traffic on a working tunnel | traffic gap on supersede | only supersede on a genuinely new initiator SPI after the operator clear; preserve the working-tunnel path |
| R-3 | Fix changes wire behavior and breaks strongSwan interop | interop scenario red | add/adjust `test/ipsec-interop/` scenario; validate against strongSwan (interop rule) |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `clear vpn ipsec sa` on the initiator daemon | -> | re-established SA against a live responder | `test/ipsec/ipsec-clear-reestablish.ci` |
| responder receives a fresh IKE_SA_INIT while holding a stale SA | -> | responder accept/supersede path | `TestResponderAcceptsReinitAfterStaleSA` (unit) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two ze instances (initiate + respond) with an established tunnel; operator runs `clear vpn ipsec sa` on the initiator | The tunnel re-establishes within a bounded time; `show vpn ipsec sa` on the initiator reports the peer `established` again |
| AC-2 | Same, `clear vpn ipsec sa peer <name>` | Same re-establishment for that peer only; other peers untouched |
| AC-3 | Responder holds an established SA and receives a fresh IKE_SA_INIT (new initiator SPI) from the configured peer | Responder accepts the new SA (superseding/coexisting per the RFC rule the fix documents), not a silent drop |
| AC-4 | An established, healthy tunnel with no operator clear | No spurious re-negotiation, no traffic gap (R-2 guard) |
| AC-5 | Wire behavior verified against strongSwan | Interop scenario passes (or a documented reason it is N/A) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator bounces a stuck tunnel with `clear vpn ipsec sa` | clear -> teardown -> re-initiate -> responder accept -> established | `test/ipsec/ipsec-clear-reestablish.ci` |
| 2 | Operator clears one peer of many | `clear vpn ipsec sa peer <name>` -> that peer re-establishes | `test/ipsec/ipsec-clear-reestablish-peer.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResponderAcceptsReinitAfterStaleSA` | `internal/component/ike/engine/*_test.go` | AC-3 responder accept/supersede | |
| `TestTerminateAllSAsReinitiates` | `internal/component/ike/engine/*_test.go` | AC-1 clear triggers re-initiation | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| re-establish bound (seconds) | define during design | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-clear-reestablish.ci` | `test/ipsec/` | operator clear -> tunnel re-establishes (two-ze, engine-step directives, noop dataplane) | |
| `ipsec-clear-reestablish-peer.ci` | `test/ipsec/` | per-peer clear -> that peer re-establishes | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-clear-reestablish` | `test/ipsec-interop/scenarios/` | strongSwan | ze re-init after clear accepted by a real responder | |

## Files to Modify
- `internal/component/ike/engine/register.go` - responder accept and/or clear re-establish path (exact change per research)
- `internal/component/ike/engine/reconcile.go` - if the initiator re-initiation needs adjustment
- `internal/component/ike/engine/fsm.go` - if the responder state handling changes
- `internal/component/ike/cmd/ipsec.go` - only if the clear handler semantics change

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | no new config surface |
| CLI commands/flags | N/A | `clear vpn ipsec sa` already exists |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-clear-reestablish*.ci` |
| Doctor check for runtime dependencies | N/A | no new runtime dependency |
| Prometheus counters/metrics | [ ] | consider an ipsec re-negotiation counter during design (observable state) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (clear behavior) |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` (`clear vpn ipsec sa` semantics) |
| 9 | RFC behavior implemented/changed? | [ ] | `rfc/short/rfc7296.md`, `docs/features/rfc-status.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `ai/digests/ipsec-ike.md` |

## Files to Create
- `test/ipsec/ipsec-clear-reestablish.ci`, `test/ipsec/ipsec-clear-reestablish-peer.ci`
- `internal/component/ike/engine/<name>_test.go` (unit tests above)
- `test/ipsec-interop/scenarios/NN-clear-reestablish/` (if wire behavior changes)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Risks & Assumptions (validate A-1..A-3 first) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases
1. **Phase: Research + reproduce** - add a responder-side trace at the drop point, confirm A-2, pin the exact drop location; write the failing unit + `.ci`.
2. **Phase: Fix at source** - responder accept/supersede (or stale-SA teardown), per RFC 7296; keep R-1/R-2 guards.
3. **Phase: Functional + interop** - two-ze `.ci` green; strongSwan interop if wire behavior changed.
4. **Full verification + closure.**

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | Re-establishment is bounded; no SA leak/accumulation; working tunnels unaffected |
| RFC | Enforcing code carries the exact RFC 7296 section it implements |
| Security | R-1 DoS vector mitigated (single-in-flight, supersede-not-accumulate) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| clear re-establishes | `bin/ze-test ipsec --pattern clear-reestablish` |
| responder accepts re-init | `go test -run TestResponderAcceptsReinitAfterStaleSA ./internal/component/ike/engine/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| DoS / resource exhaustion | Repeated inits cannot grow the SA table unbounded |
| Traffic disruption | Superseding never drops a working tunnel outside the operator-clear path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails behavior mismatch | Re-read Current Behavior; RESEARCH if the drop location was wrong |
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
- Re-establishment timing depends on the initiator's connect-retry cadence; the bound is defined during design.

## RFC Documentation

Add `// RFC 7296 Section X.Y: "<quoted requirement>"` above the responder accept/supersede code.

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
| `clear` re-establishes against a live responder | functional (two-ze) + interop | `ipsec-clear-reestablish.ci`; strongSwan scenario |

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
- [ ] Feature code integrated (`internal/*`)
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
- Authored 2026-07-10 from the `spec-test-coverage-gaps` two-ze IKE investigation
  (see that spec's Design Insights and `tmp/session/session-state-test-coverage-gaps-*.md`).
  Skeleton = captured intent with verified `file:line` evidence; moves to `design`
  when picked up. The root cause is a hypothesis (A-2), not a settled fact.
