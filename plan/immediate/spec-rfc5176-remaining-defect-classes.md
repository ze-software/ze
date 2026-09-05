# Spec: the RFC 5176 defect classes that still block the extraction sign-off

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | `plan/pre-release/spec-rfcgate-6-supported-extraction-signoff.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`rfc/extraction/rfc5176.json` cannot sign off, because 23 of its 72 sites state
obligations Ze does not meet and no exclusion kind in the closed set honestly
covers an unmet obligation. `rfc5176` is one of the first two Tier 1 walks and
is listed `Supported` on `docs/features/rfc-status.md`, so the gap is published.

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

**What this spec owes**, re-verified at the producer on 2026-09-05:

- **CoA state changes are not atomic.** `(*coaListener).applySubscriberCoA`
  quotes Section 2.3's "State changes resulting from a CoA-Request MUST be
  atomic" above itself, then emits the subscriber rate change, then the L2TP
  rate change, then the CoS change, sending a CoA-NAK on the first emit that
  fails. A CoS emit that fails after the two rate emits succeeded answers a NAK
  for a request whose changes were partly made.
- **Multiple matching sessions are handled on one path only.**
  `(*coaListener).oneSession` NAKs with `ErrorCauseMultiSessionUnsupported` when
  more than one L2TP session matches, taking Section 2.3's branch for a NAS that
  does not support multi-session changes. `(*coaListener).handleCoA` tries
  `findSubscriberSession` FIRST, and that function looks up one session by
  Acct-Session-Id through `LookupByAcctSessionID` and evaluates no multi-session
  question at all. A CoA-Request that reaches the subscriber registry therefore
  never earns the 508 the L2TP path would give it.
- **Termination-Action State echo is absent.** No symbol naming
  Termination-Action exists anywhere in non-test source.

Whether this spec runs is Thomas's decision: it is a conformance programme
rather than a walk, and it is the last thing between `rfc5176` and a signed
extraction.

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
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - `applySubscriberCoA` emits up to three events in sequence and NAKs on the first failure, so a partial change can precede a NAK; `oneSession` NAKs 508 on the L2TP path while `handleCoA` tries `findSubscriberSession` first, which resolves one session by Acct-Session-Id and asks no multi-session question; `sendResponse` echoes Proxy-State and State; no Termination-Action handling exists
- [ ] `rfc/extraction/rfc5176.json` - the extraction that cannot sign, 23 of 72 sites unmet

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
| A-1 | the three remaining classes are all that block the sign-off | the 2026-09-03 producer verification | more sites stay unmet | re-run the walk against the 72 sites | unvalidated |

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
| AC-2 | a CoA-Request matching more than one subscriber session | a CoA-NAK with Error-Cause 508, the same answer the L2TP path gives |
| AC-3 | a request whose Termination-Action requires a State echo | the State is echoed as the RFC requires |
| AC-4 | `./le rfc check` over the rfc5176 stem | every site carries a disposition and the extraction signs |

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
| a signed rfc5176 extraction | `./le rfc check` |

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
