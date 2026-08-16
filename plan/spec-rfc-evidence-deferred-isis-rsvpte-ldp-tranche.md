# Spec: rfc-evidence-deferred-isis-rsvpte-ldp-tranche

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Bind the IS-IS, RSVP-TE and LDP unit-only requirements at `functional/verify`.
84 gated MUST-level requirements sit there: IS-IS 52, RSVP-TE 25, LDP 7. All
three subsystems already have runnable suites, so the tier is reachable today:
`mk/test-functional.mk` `all_suites` names `isis`, `isis-wire`, `rsvpte` and
`ldp`.

Deferred out of `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md`,
which is still open. The row that homes this work lives in
`plan/deferrals/rfcgate-2-deferred-nonunit-evidence-backfill.md`.

**This cluster was ranked 4 and never examined.** That ranking came from
protocol-family intuition, which the source spec then measured and found weaker
than ranking by oracle. Do not inherit the rank. Apply the selection rule's test
1 (oracle independence) as a scan across each of the three RFCs BEFORE picking
one, and let the scan set the order.

### Constraints carried from the source spec

- Apply the selection rule before writing any test. All three of its tests must
  hold: oracle independence, boundary observability, tier reachability.
- The unit of analysis is the REQUIREMENT, not the test. A self-oracled test
  beside a known-answer test is harmless and is not a candidate.
- Land only POSITIVE assertions. A check that passes because something failed to
  happen is the vacuity trap in `ai/rules/interop-and-goal-validation.md`.
- Write no `{gap}`. A requirement that cannot be proven at any tier is an owner
  question (`ai/rules/rfc-compliance.md`).
- Measure by importing `scripts/dev/rfc_requirements.py`. Do not render
  `ai/RFC-REQUIREMENTS.md` to read a number: the regen sweeps other sessions'
  uncommitted tags into your commit.

## Required Reading

### Architecture Docs
- [ ] `scripts/dev/rfc_requirements.py` - `CARRIERS`, `carrier_for`, `functional_suites`
  → Constraint: the four suites above are named in `all_suites`, so a `.ci` in any of them earns `functional/verify`.
- [ ] `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md` - the selection rule and the measured ranking
  → Decision: the rule tests the ORACLE, never the requirement text.
- [ ] `mk/test-functional.mk` - `all_suites`
  → Constraint: the suite list is the tier gate; read it rather than assuming.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5305.md` and the other IS-IS, RSVP-TE and LDP summaries
  → Constraint: fill at design time, per requirement picked.

**Key insights:** (minimal context to resume after compaction)
- Tier reachability is already satisfied here, unlike the 242-requirement cluster. The open question is which requirements are proven badly, not whether they can be proven.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/dev/rfc_requirements.py` - resolves a test path to a carrier, an evidence kind and an execution tier

**Behavior to preserve:**
- Every existing expectation in the four suites, and the `kind/tier` model.

**Behavior to change:**
- None expected. This is evidence work, not daemon work. A defect it uncovers is a separate fix.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Fill at design time: the PDU or command each landed test uses to reach the daemon.

### Transformation Path
1. Fill at design time.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ external peer | IS-IS, RSVP-TE or LDP PDUs | No |

### Integration Points
- `scripts/dev/rfc_requirements.py` `scan_ci_tags` - discovers the `# RFC requirement:` lines.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The four suites boot enough of each subsystem to observe these obligations at a boundary | they are named in `all_suites` and run in `make ze-precommit-verify` | the requirement needs a tier the suite cannot give | run each suite and read what it exercises | unvalidated |
| A-2 | A useful share of the 84 are self-oracled | untested. Rank 4 came from intuition, and the L2TP cluster measured BETTER covered than predicted | the tranche buys tier without discrimination | run the oracle scan before picking | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The backfill raises tier without raising discrimination | a mutation that reddens the `.ci` also reddens the unit suite | mutation-verify both layers and report honestly |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible if the work stays test-only. A vacuous test would publish evidence that does not discriminate |
| How is it reverted? | Delete the landed tests; the evidence ratchet then fires, which is the intended alarm |
| Who else touches this path? | Any session working `internal/plugins/isis/**`, `internal/plugins/rsvpte/**` or `internal/plugins/ldp/**` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `# RFC requirement:` line in a landed `.ci` | → | `scan_ci_tags` then `carrier_for` (`scripts/dev/rfc_requirements.py`) | the landed `.ci` resolves to `functional/verify` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Fill at design time | Fill at design time |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Fill at design time | Fill at design time | Fill at design time |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none expected) | - | This spec adds evidence, not production Go | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Fill at design time | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Fill at design time | `test/isis/*.ci`, `test/rsvpte/*.ci`, `test/ldp/*.ci` | Fill at design time | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Fill at design time | `test/interop/scenarios/` | FRR | Fill at design time | |

## Files to Modify
- Fill at design time.

## Files to Create
- `test/isis/*.ci`, `test/rsvpte/*.ci`, `test/ldp/*.ci` - the tranche.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | evidence work, no config surface |
| YANG validation constraints | No | no leaf added |
| YANG custom validators | No | no leaf added |
| CLI commands/flags | No | no command added |
| CLI grammar (keyword before value) | No | no command added |
| Editor autocomplete | No | no leaf added |
| Functional test for new RPC/API | Yes | the tranche itself |
| Pipe completeness | No | no command output added |
| Env var registration | No | uses existing test env vars |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | No | no observable state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | evidence work |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `ai/RFC-REQUIREMENTS.md` regen, and `docs/features/rfc-status.md` if a support level moves |
| 10 | Test infrastructure changed? | No | uses the existing suite shape |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | No | no source file changed |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove a `.ci` tag in each target suite resolves to `functional/verify`
   - Tests: `carrier_for` on a draft path in each suite
   - Files: drafts in `test/draft/`
   - Verify: the carrier resolves and the scanner accepts the tag
2. **Phase: Oracle scan** -- apply selection-rule test 1 across all three RFC families, and let the result set the order
   - Verify: a per-requirement candidate list, with the excluded ones named

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has evidence |
| Vacuity | Every landed check is a positive assertion |
| Oracle independence | Expected values come from the RFC, not from a Ze helper |
| Tier honesty | The claimed tier is read from `carrier_for`, not asserted |
| Rule: `ai/rules/rfc-compliance.md` | No `{gap}` written; conformance questions are raised |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The oracle scan | the per-requirement candidate list |
| The tranche's bindings | `scan_ci_tags` over the landed files |
| Mutation evidence | a mutation table, both layers |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the tests send PDUs to a loopback listener; no new exposure |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Fill at design time.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
