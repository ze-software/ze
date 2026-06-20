# Spec: bugfix-bgp-message-validation-before-delivery

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/review-bug-review-bgp-engine.md` findings BENG-001 and BENG-003
3. `internal/component/bgp/capability/capability.go`
4. `internal/component/bgp/reactor/session_read.go`
5. `internal/component/bgp/reactor/session_handlers.go`
6. `internal/component/bgp/message/header.go`
7. `rfc/short/rfc5492.md`, `rfc/short/rfc8654.md`, `rfc/short/rfc2918.md`, `rfc/short/rfc7313.md`

## Task

Fix BENG-001 and BENG-003. BGP input validation must reject malformed known capabilities and malformed ROUTE-REFRESH messages before negotiated state or plugin/event consumers can observe them.

## Required Reading

### Architecture Docs
- [ ] `plan/review-bug-review-bgp-engine.md` - source findings and regression plans
  -> Decision: fix capability TLV validation and route-refresh pre-delivery validation in one receive-path validation spec.
  -> Constraint: unknown capabilities remain ignored, while malformed known capabilities and malformed message lengths reject.
- [ ] `docs/architecture/core-design.md:496-600` - receive and plugin delivery paths
  -> Decision: plugin delivery must happen after validation for messages that can be rejected.
  -> Constraint: do not deliver malformed peer input to plugins before RFC handling.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5492.md` - capability optional parameter TLVs
  -> Constraint: TLVs must fit within the optional parameter; unknown capability codes are ignored.
- [ ] `rfc/short/rfc8654.md` - Extended Message capability
  -> Constraint: Extended Message capability length is zero.
- [ ] `rfc/short/rfc2918.md` - Route Refresh
  -> Constraint: Route Refresh capability length is zero and message payload is fixed.
- [ ] `rfc/short/rfc7313.md` - Enhanced Route Refresh
  -> Constraint: invalid route-refresh message length sends ROUTE-REFRESH Message Error, Invalid Message Length.

**Key insights:**
- Capability code 2, 6, and 70 parse without length checks today.
- `ParseFromOptionalParams` drops parse errors and continues.
- `processMessage` invokes `onMessageReceived` before route-refresh body validation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/capability/capability.go:197-208` - fixed-length known capabilities are accepted regardless of payload length.
- [ ] `internal/component/bgp/capability/capability.go:814-845` - parse errors from capability optional parameters are ignored.
- [ ] `internal/component/bgp/reactor/session_read.go:221-240` - plugin callback runs before route-refresh handler validation.
- [ ] `internal/component/bgp/reactor/session_handlers.go:263-329` - route-refresh length validation happens after callback and after `onRefreshRecv`.

**Behavior to preserve:**
- Unknown capability codes are ignored per RFC 5492.
- Valid zero-length Route Refresh, Extended Message, and Enhanced Route Refresh capabilities still negotiate.
- Valid Route Refresh, BoRR, and EoRR still reach plugin/API consumers.

**Behavior to change:**
- Malformed known capability lengths return an OPEN Message Error and prevent session establishment.
- Malformed capability TLV boundaries return an OPEN Message Error instead of being silently ignored.
- Malformed ROUTE-REFRESH payloads are rejected before `onMessageReceived` and before `onRefreshRecv`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- BGP peer sends OPEN optional parameters or ROUTE-REFRESH messages.

### Transformation Path
1. TCP read produces header and body.
2. OPEN body parsing extracts optional parameters and capabilities.
3. Known capability validators enforce exact lengths.
4. ROUTE-REFRESH body length and subtype are validated.
5. Only validated messages are delivered to callbacks or plugin/API consumers.
6. Invalid messages produce the correct NOTIFICATION and session close.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| peer wire -> capability parser | OPEN optional parameter TLVs | [ ] malformed capability tests |
| parser -> negotiation state | parsed capabilities | [ ] malformed known capabilities not negotiated |
| peer wire -> plugin event | `onMessageReceived` callback | [ ] route-refresh spy is not called on malformed body |

### Integration Points
- `internal/component/bgp/capability/capability.go`
- `internal/component/bgp/message/open.go`
- `internal/component/bgp/reactor/session_read.go`
- `internal/component/bgp/reactor/session_handlers.go`
- `internal/component/bgp/message/header.go`

### Architectural Verification
- [ ] No bypassed layers: validation happens before callback delivery.
- [ ] No unintended coupling: capability parser remains in capability package.
- [ ] No duplicated functionality: one exact-length helper can serve all zero-length capabilities.
- [ ] Zero-copy preserved where applicable: validation reads slices without copying.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | OPEN negotiation can distinguish malformed known capability from unknown capability | RFC 5492 and parser code | malformed known caps remain ignored | session OPEN tests | unvalidated |
| A-2 | Route-refresh validation can run before callback without breaking valid refresh consumers | BENG-003 receive path | valid refresh events stop flowing | valid refresh regression test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Error code/subcode selection differs for base RR vs enhanced RR | tests disagree with RFC summary | cite RFC summary and keep behavior consistent with existing NOTIFICATION constants |
| R-2 | Existing tests rely on ignoring malformed capability data | capability tests fail after stricter behavior | update tests only if they were testing unknown capabilities, not malformed known capabilities |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| malformed known capability in OPEN | -> | capability parser and session OPEN rejection | `TestOpenRejectsMalformedKnownCapability` |
| truncated capability TLV in optional parameter | -> | optional parameter parser returns error | `TestOptionalParamRejectsTruncatedCapabilityTLV` |
| malformed ROUTE-REFRESH body | -> | route-refresh validation before callback | `TestRouteRefreshInvalidLengthNotDelivered` |
| valid ROUTE-REFRESH body | -> | callback delivery still happens | `TestRouteRefreshValidLengthDelivered` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | OPEN contains Route Refresh, Extended Message, or Enhanced Route Refresh capability with non-zero payload length | session rejects OPEN with OPEN Message Error and does not negotiate that capability |
| AC-2 | OPEN contains a Type 2 capability optional parameter with a truncated inner TLV | session rejects OPEN with OPEN Message Error |
| AC-3 | OPEN contains an unknown capability with any syntactically valid length | session ignores the unknown capability and continues |
| AC-4 | ROUTE-REFRESH body length is not exactly 4 | callback/event consumers are not called, NOTIFICATION is sent, and session closes |
| AC-5 | Valid ROUTE-REFRESH body is received after negotiation | callback/event consumers still receive the event |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a malformed known capability | TCP OPEN -> capability parser -> session error | `TestOpenRejectsMalformedKnownCapability` |
| 2 | Sends malformed route-refresh | TCP RR -> validation -> notification, no plugin event | `TestRouteRefreshInvalidLengthNotDelivered` |
| 3 | Sends valid route-refresh | TCP RR -> validation -> callback delivery | `TestRouteRefreshValidLengthDelivered` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseRejectsMalformedKnownCapabilityLength` | `internal/component/bgp/capability/capability_test.go` | AC-1 parser layer | |
| `TestOptionalParamRejectsTruncatedCapabilityTLV` | `internal/component/bgp/capability/capability_test.go` | AC-2 | |
| `TestOpenRejectsMalformedKnownCapability` | `internal/component/bgp/reactor/session_test.go` | AC-1 session behavior | |
| `TestRouteRefreshInvalidLengthNotDelivered` | `internal/component/bgp/reactor/session_test.go` | AC-4 | |
| `TestRouteRefreshValidLengthDelivered` | `internal/component/bgp/reactor/session_test.go` | AC-5 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| zero-length capability payload | 0 | 0 | N/A | 1 |
| capability TLV length | 0..remaining | remaining bytes | N/A | remaining+1 |
| ROUTE-REFRESH body length | exactly 4 | 4 | 3 | 5 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Add malformed OPEN or route-refresh fixture if existing runner supports raw peer injection | `test/interop/` or reactor Go test | malformed peer input is rejected before plugin delivery | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Existing BGP interop smoke | `test/interop/scenarios/` | FRR/BIRD/GoBGP as applicable | valid capability and route-refresh behavior remains compatible | |

### Future (if deferring any tests)
- No deferral approved. Unit tests must cover malformed peer input even if no interop fixture exists.

## Files to Modify

- `internal/component/bgp/capability/capability.go` - exact length checks and error-returning optional parameter parser.
- `internal/component/bgp/reactor/session_read.go` - validate route-refresh before callback delivery.
- `internal/component/bgp/reactor/session_handlers.go` - avoid pre-validation `onRefreshRecv` for invalid bodies.
- `internal/component/bgp/message/header.go` if exact route-refresh length belongs at header validation.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| Functional test for new RPC/API | No | protocol unit tests |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | bug fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | behavior corrected to RFC |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | add or verify RFC comments in changed code; docs only if architecture text claims old behavior |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | grep docs during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create

- Capability parser tests in `internal/component/bgp/capability`.
- Session receive tests in `internal/component/bgp/reactor`.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior and RFC summaries |
| 3. Wiring phase | malformed input tests |
| 4. Implement | parser and receive ordering fixes |
| 5. Review gate | Critical Review Checklist |
| 6. Full verification | targeted tests, `make ze-test-bgp`, final gate |

### Implementation Phases
1. **Phase: failing capability tests** - add parser and session OPEN malformed known capability tests.
2. **Phase: failing route-refresh tests** - add invalid and valid route-refresh callback tests.
3. **Phase: capability validation** - enforce exact lengths and propagate parse errors to OPEN handling.
4. **Phase: route-refresh validation ordering** - validate body before callbacks and preserve valid delivery.
5. **Phase: verification** - run targeted tests and BGP component tests.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| RFC | changed validation cites RFC 5492, 8654, 2918, and 7313 where enforced |
| Correctness | unknown capabilities still ignored while malformed known capabilities reject |
| Delivery order | malformed route-refresh cannot reach plugin/event callbacks |
| Tests | valid and invalid boundary lengths covered |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Capability parser tests | `go test -run Capability ./internal/component/bgp/capability` |
| Session route-refresh tests | `go test -run RouteRefresh ./internal/component/bgp/reactor` |
| BGP group | `make ze-test-bgp` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Malformed peer input | no malformed known capability or route-refresh reaches consumers |
| Resource use | no allocation-heavy validation on hot path |
| Error leakage | NOTIFICATION data does not expose unrelated state |

### Failure Routing
| Failure | Route To |
|---------|----------|
| unknown capability behavior regresses | fix parser dispatch, not tests |
| route-refresh valid callback stops firing | adjust validation order before callback, not callback contract |

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

- Peer input should be validated before plugin visibility whenever invalid input can close the session.

## Core Insight

Unknown is ignorable; malformed known protocol is not.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Cluster capability and route-refresh validation | separate specs | both fix receive-path validation before state/plugin visibility |

## Known Limitations

- This spec does not implement the fix.

## RFC Documentation

Add RFC comments above each enforced length/error rule during implementation.

## Implementation Summary

### What Was Implemented
- Fix spec only. Production code is unchanged.
- The spec captures BENG-001 and BENG-003 from `plan/review-bug-review-bgp-engine.md`.
- Regression expectations cover malformed capability OPEN rejection and ROUTE-REFRESH validation before callback delivery.

### Bugs Found/Fixed
- BENG-001 and BENG-003 documented for implementation.
- No production bug was fixed by this review program.

### Documentation Updates
- No user docs required for the fix-spec artifact.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Create actionable fix plan for BENG-001 | done | this spec | malformed known capabilities and TLV boundary errors covered |
| Create actionable fix plan for BENG-003 | done | this spec | ROUTE-REFRESH validation ordering covered |
| Include RFC-linked regression plan | done | Required Reading and TDD sections | RFC 2918, 5492, 7313, and 8654 named |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-6 | planned | acceptance criteria table | to be satisfied by implementation owner |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| capability parser malformed lengths | planned | `internal/component/bgp/capability` | not run by review program |
| session OPEN malformed capability rejection | planned | `internal/component/bgp/reactor` | not run by review program |
| ROUTE-REFRESH validation before delivery | planned | `internal/component/bgp/reactor` | not run by review program |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/capability/capability.go` | planned | implementation target |
| `internal/component/bgp/reactor/session_read.go` | planned | implementation target |
| `internal/component/bgp/reactor/session_handlers.go` | planned | implementation target |

### Audit Summary
- Total items: 2 accepted findings converted to one fix spec.
- Done: fix spec created.
- Partial: implementation pending by design.
- Skipped: no production code changes in review program.
- Changed: new spec file.

## Goal Validation
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Malformed BGP peer input rejected before delivery | spec artifact | this file names RFC constraints, ACs, and regression tests for BENG-001 and BENG-003 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | BENG-001 malformed known capabilities accepted or ignored | child 3 report | fix spec created |
| 2 | ISSUE | BENG-003 malformed ROUTE-REFRESH delivered before validation | child 3 report | fix spec created |

### Fixes applied
- None, review program creates fix specs only.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Fix-spec artifact now has summary, audit, goal validation, review gate, and pre-commit sections | this spec | no action |

### Final status
- [ ] `/ze-review` re-run by implementation owner after code changes
- [x] Fix spec contains source evidence, RFC constraints, ACs, and regression plan

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-bugfix-bgp-message-validation-before-delivery.md` | yes | created by review program |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| review deliverable | fix spec exists | this file |

### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
| peer OPEN capability parser | planned unit tests | pending implementation |
| ROUTE-REFRESH receive path | planned unit tests | pending implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-2 | unresolved for implementation | listed in spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| RFC comments required later | protocol validation fix spec | yes |
