# Spec: RFC 5176 residual CoA atomicity and session matching

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | `plan/immediate/spec-lifecycle-invariants.md` (applied-result contract, AC-3/AC-4; shared design boundary) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`rfc/extraction/rfc5176.json` records sign-off on 2026-09-01. That source-walk
milestone is delivered; it does not prove that every CoA request is applied
atomically or that every subscriber lookup honours the request's full selector.
This spec owns those residual conformance obligations, with the applied-result
contract shared with the lifecycle spec as described below.

The rfc5176 walk of `spec-rfcgate-6` recorded nine defect classes on
2026-08-31: Service-Type never read; attributes not treated as mandatory; only
the first matching session acted on; Proxy-State and State not echoed; a stale
Event-Timestamp NAK'd where Section 6.3 says silently discard; CoA state changes
not atomic; Termination-Action State echo absent.

**Four were verified DEAD at the producer on 2026-09-03**, in
`internal/component/l2tp/plugins/authradius/coa.go`. The row's original path,
`internal/component/radius/coa.go`, is wrong and no file is there.

1. Service-Type IS read: the CoA path tests `pkt.FindAttr(radius.AttrServiceType)` and quotes Section 3.2's Authorize-Only rule above the branch.
2. Attributes ARE treated as mandatory: two allow-listed attribute sets gate what a CoA-Request and a Disconnect-Request may carry.
3. Proxy-State and State ARE echoed: `(*coaListener).sendResponse` copies every attribute whose type is `AttrProxyState` or `AttrState`, quoting Sections 3.1 and 3.3.
4. A stale Event-Timestamp is SILENTLY DISCARDED: the stale arm logs and returns with no answer, quoting Section 6.3, and its comment gives the reason a NAK would be wrong, that it tells a replaying sender the secret is right. An absent Event-Timestamp draws `ErrorCauseInvalidRequest`.

Current ownership, checked against `handleCoA`, `applySubscriberCoA`,
`findSubscriberSession` and `oneSession` in
`internal/component/l2tp/plugins/authradius/coa.go`:

- This spec owns atomic multi-change application. `applySubscriberCoA` emits
  subscriber rate, L2TP rate and CoS changes in sequence and NAKs the first
  failed emit. Earlier changes have no rollback here. Completion owes the
  all-or-nothing result, including downstream application failures.
- `plan/immediate/spec-lifecycle-invariants.md` AC-3/AC-4 owns the applied-result
  contract and PPPoE delivery. A successful emit still does not prove a change
  was applied: `shaper.onSessionRateChange` returns without a result after an
  unknown session or an `applyTC` failure. This spec consumes that contract for
  its atomic transaction; neither spec may count the other's unfinished half
  as complete.
- This spec owns subscriber selector correctness and multi-match refusal.
  `findSubscriberSession` reads only Acct-Session-Id and returns one registry
  lookup before `oneSession` can inspect the L2TP match set. The residual is
  the missing conjunction with other supplied identifiers and the complete
  matching-set decision. One returned record alone proves neither ambiguity
  nor correctness. Preserve the existing 508 response when a request matches
  multiple sessions and Ze does not support a multi-session change.
- Termination-Action is a retained optional-feature question. The signed
  extraction excludes its State obligation as `feature-out-of-scope` because
  RFC 2865 Section 5.29 makes that reauthentication feature optional. Absence
  of that feature is not an unsigned extraction or an unconditional CoA bug.
  The original AC-3 obligation remains conditional on selecting that feature;
  Thomas must resolve that scope question before this spec closes.

`plan/pre-release/spec-rfcgate-6-supported-extraction-signoff.md` is the parent
evidence programme. Its recorded signature must not be repeated as a future
deliverable or used to close these residual behaviours.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the extraction sign-off, the tagged-test routes and the discrimination record
  → Constraint: <to be filled>

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5176.md` - the declared requirement ids for this stem
  → Constraint: <to be filled>
- [ ] `rfc/full/rfc5176.txt` - Sections 2.3, 3.1, 3.3 and 6.3, read at the source text
  → Constraint: Section 2.3, "State changes resulting from a CoA-Request MUST be atomic"

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - sequential emissions, subscriber Acct-Session-Id lookup before the L2TP match-set check, and the existing State/Proxy-State response echo.
- [ ] `internal/component/l2tp/plugins/shaper/shaper.go` - `onSessionRateChange` has no application-result return to the CoA caller.
- [ ] `rfc/extraction/rfc5176.json` - signed 2026-09-01; its optional Termination-Action exclusion is distinct from the remaining atomicity and selector proofs.

**Behavior to preserve:** (unless the user explicitly said to change it)
- the four classes already verified dead, each with its RFC sentence quoted in the code beside it
- the Section 3.2 decision taken in `fe51839da0`: Ze does not offer Authorize-Only, and every Service-Type in a CoA-Request earns `CodeCoANAK` with `ErrorCauseUnsupportedService`

**Behavior to change:** (only what the user asked for)
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a CoA-Request or a Disconnect-Request arriving on the CoA UDP listener
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CoA listener ↔ subscriber registry | session lookup | No |
| CoA listener ↔ shaper plugins | typed events on the bus | No |

### Integration Points
- `internal/component/subscriber/` - the registry `findSubscriberSession` reads
- `internal/component/l2tp/` - the service `findSessions` reads

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The signed extraction's mappings remain accurate after the residual fixes | the 2026-09-01 artifact accounts for the source sites but does not prove these behaviours | a signature masks stale evidence | review affected sites, tests and discrimination records against each changed producer | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | making the change set atomic needs a rollback the shaper events cannot give | an emit that cannot be undone | <to be filled> |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a Dynamic Authorization Client's changes are applied partly, or refused wrongly |
| How is it reverted? | <to be filled> |
| Who else touches this path? | `spec-rfcgate-6-supported-extraction-signoff`, the RADIUS component specs |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a CoA-Request carrying a rate change and a CoS change where the second fails | → | `applySubscriberCoA` | <to be filled> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a CoA-Request whose second authorization change fails | no change is left applied, and a CoA-NAK is sent |
| AC-2 | A subscriber CoA-Request supplies several session identifiers or matches more than one session | All supplied identifiers constrain the match; a mismatch is refused, and an unsupported multi-session change receives CoA-NAK with Error-Cause 508 without changing any session |
| AC-3 | The retained Termination-Action question is resolved | If Thomas selects the optional reauthentication feature, its Access-Request echoes State as required and carries end-to-end proof. Otherwise the recorded exclusion remains explicit and no implementation or conformance completion is inferred from the absence of attribute 29 |
| AC-4 | The residual behaviours and affected RFC 5176 evidence are checked | Atomicity and selector tests prove both success and refusal, carry valid discrimination records, and preserve the accepted extraction and existing proofs; the 2026-09-01 signature is not counted as new work |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | a Dynamic Authorization Client changes a subscriber's rate and CoS profile in one request | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| <to be filled> | `internal/component/l2tp/plugins/authradius/` | <to be filled> | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| <to be filled> | `test/` | <to be filled> | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| <to be filled> | `test/interop/scenarios/` | FreeRADIUS | a conformant Dynamic Authorization Client's requests are applied atomically | |

## Files to Modify
- `internal/component/l2tp/plugins/authradius/coa.go` - <to be filled>
- `rfc/short/rfc5176.md` - the requirement ids the fixes prove
- `rfc/extraction/rfc5176.json` - the dispositions

## Files to Create
- <to be filled>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| Functional test for new RPC/API | | <to be filled> |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfc5176.md`, `docs/features/rfc-status.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- <to be filled>
2. **Phase: <to be filled>**

## RFC Documentation (Scope: protocol)

Every MUST this spec implements carries
`// RFC 5176 Section X.Y: "<quoted requirement>"` directly above the enforcing
code, and every tagged test it adds carries a discrimination record written by
`./le rfc discriminate-record`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Rule: rfc-compliance | no claim is wider than what the test body checks |
| Completeness | the subscriber path and the L2TP path answer a multi-session request the same way |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Proven atomic application and complete subscriber matching | AC-1/AC-2 end-to-end results plus current tagged-test discrimination records; preserve the existing signed extraction |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations
- <to be filled>

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
