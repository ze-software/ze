# Spec: l2tp-7b -- L2TP CLI .ci coverage

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-l2tp-7-subsystem |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-l2tp-7-subsystem.md` -- parent spec (the tests this one completes)
3. `plan/deferrals.md` 2026-04-17 rows pointing here

## Task

Land the remaining 16 `.ci` tests from spec-l2tp-7 Phase 8. All
depend on a working L2TP handshake (`modprobe l2tp_ppp pppol2tp` on
the test host, or a kernel-stub harness) and/or the reload path
routing through the test runner. spec-l2tp-7 delivers all underlying
code; this spec closes the `.ci` surface.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-l2tp-7-subsystem.md` -- parent spec with AC-1..AC-26
- [ ] `test/plugin/show-l2tp-empty.ci` -- precedent (wiring proof only)
- [ ] `.claude/rules/testing.md` -- `.ci` format

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2661 -- L2TP

**Key insights:** (filled during RESEARCH phase)

## Current Behavior (MANDATORY)

**Source files read:** (to be filled during RESEARCH phase)

**Behavior to preserve:** every AC already demonstrated by unit tests keeps passing.

**Behavior to change:** none -- this spec only adds `.ci` coverage.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `.ci` test runner -> ze daemon -> CLI handlers -> Subsystem facade -> Reactor

### Transformation Path
1. Test config boots ze with L2TP listener.
2. Python observer dispatches `show l2tp ...` / `l2tp * teardown ...` text commands.
3. Daemon handlers return JSON; observer asserts fields.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Observer -> engine dispatch | `ze-plugin-engine:dispatch-command` | plugin-hub TLS path |

### Integration Points
- Observer plugin registered under `plugin { external ... }`
- Hub TLS set up by test runner (BGP-peer side-effect today)

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show l2tp tunnels` with a live tunnel | -> | handleTunnels returns one row | `test/plugin/show-l2tp-tunnels.ci` |
| `l2tp tunnel teardown <id>` | -> | handleTunnelTeardown sends StopCCN | `test/plugin/teardown-tunnel.ci` |
| SIGHUP with new `shared-secret` | -> | Reload hot-applies to new tunnels | `test/plugin/reload-shared-secret.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Each spec-l2tp-7 AC that has no unit-test equivalent | Demonstrated by a `.ci` test in this spec |
| AC-2 | `make ze-verify-fast` | Passes with the new `.ci` tests included |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| n/a -- unit tests land in spec-l2tp-7 | | | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| n/a -- boundary covered in spec-l2tp-7 | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| show-l2tp-tunnels | `test/plugin/show-l2tp-tunnels.ci` | Tunnel list after handshake | |
| show-l2tp-sessions | `test/plugin/show-l2tp-sessions.ci` | Session list with username + IP | |
| show-l2tp-tunnel-detail | `test/plugin/show-l2tp-tunnel-detail.ci` | Tunnel detail fields | |
| show-l2tp-session-detail | `test/plugin/show-l2tp-session-detail.ci` | Session detail fields | |
| show-l2tp-statistics | `test/plugin/show-l2tp-statistics.ci` | Protocol counters | |
| show-l2tp-config | `test/plugin/show-l2tp-config.ci` | Effective config | |
| teardown-tunnel | `test/plugin/teardown-tunnel.ci` | StopCCN on operator teardown | |
| teardown-session | `test/plugin/teardown-session.ci` | CDN on operator teardown | |
| teardown-tunnel-all | `test/plugin/teardown-tunnel-all.ci` | Every tunnel receives StopCCN | |
| teardown-session-all | `test/plugin/teardown-session-all.ci` | Every session receives CDN | |
| offline-show-tunnels | `test/plugin/offline-show-tunnels.ci` | `ze l2tp show tunnels` matches daemon-side | |
| reload-shared-secret | `test/plugin/reload-shared-secret.ci` | SIGHUP updates secret for new tunnels | |
| reload-hello-interval | `test/plugin/reload-hello-interval.ci` | SIGHUP updates interval for new tunnels | |
| reload-listener-rejected | `test/plugin/reload-listener-rejected.ci` | SIGHUP rejects listener change | |
| redistribute-inject | `test/plugin/redistribute-inject.ci` | `/32` appears in RIB on IPCP | |
| redistribute-withdraw | `test/plugin/redistribute-withdraw.ci` | `/32` withdrawn on session-down | |

### Future (if deferring any tests)
- None

## Files to Modify
- (none -- this spec only adds test files)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| `.ci` tests | [ ] | `test/plugin/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | |
| 2 | Config syntax changed? | [ ] | |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [ ] | |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [ ] | |
| 12 | Internal architecture changed? | [ ] | |

## Files to Create
- `test/plugin/show-l2tp-*.ci` (6)
- `test/plugin/teardown-*.ci` (4)
- `test/plugin/reload-*.ci` (3)
- `test/plugin/redistribute-*.ci` (2)
- `test/plugin/offline-show-tunnels.ci` (1)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + l2tp-7 |
| 2. Audit | Files to Create |
| 3. Implement (TDD) | One test at a time |
| 4. Full verification | `make ze-verify-fast` |
| 5-12 | Standard flow |

### Implementation Phases
1. Write handshake-establishing tests that leave tunnels/sessions in place.
2. Add show/teardown tests on top.
3. Add reload tests using the SIGHUP path.
4. Add redistribute inject/withdraw tests once the RIB inject path is callable (see spec-l2tp-7c-rib-inject).

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N in spec-l2tp-7 has a `.ci` test row |
| Correctness | Deliberately break production code; verify test fails |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| 16 `.ci` tests landed | `ls test/plugin/show-l2tp-*.ci test/plugin/teardown-*.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Tests assert that teardown of unknown IDs errors cleanly |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
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

Add RFC 2661 pointers in test comments where protocol behavior is asserted.

## Implementation Summary

### What Was Implemented
- (to be filled)

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
- [ ] AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

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
- [ ] Write learned summary
- [ ] Summary included in commit
