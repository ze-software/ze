# Spec: fixit-rfc-gate-has-no-level-ratchet

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An RFC requirement can be silently demoted from MUST to SHOULD, and nothing
fires. Demotion is the cheapest route from red to green in the whole RFC gate,
cheaper than `{gap}`, which costs a public disclosure row.

`ai/rules/rfc-compliance.md` names seven ratchets that keep RFC testing valid.
Every one of them protects a different property: enrolment, proof polarity,
requirement existence, new summaries, evidence kind, extraction, public
disclosure. **None protects the requirement's LEVEL.** `scripts/dev/rfc_requirements.py`
holds 23 `check_` functions and `.level` appears in only two roles across the
file: the `is_gated` predicate, which tests `self.level in GATED_LEVELS`, and
message strings that print the level back to the reader. No check compares a
requirement's level against HEAD.

The consequence is exact. A MUST becomes a SHOULD, `is_gated` goes false, the
row leaves the gated population, and every coverage obligation attached to it
disappears with it. `check_coverage_ratchet` cannot see the loss, because it
compares polarities of rows that are still gated. `check_retired_requirements`
cannot see it, because the id is still there.

**Measured, not predicted.** `RFC7296-2.8-1` went `[MUST]` to `[SHOULD]` on
2026-08-15, correctly, because RFC 7296 Section 2.8.1 says the low-nonce SA
"SHOULD be closed". No gate fired at all. The change was caught by a human
re-reading the RFC sentence, which is the control this spec exists to replace.

**The test that is named for this cannot do it.**
`test_rfc7296_ids_are_neither_retired_nor_demoted`
(`scripts/dev/rfc_requirements_test.py`) says so in its own docstring: it builds
its baseline from `r.gated` over the CURRENT rows, so an id that stops being
MUST-level leaves the baseline rather than failing against it. Its assertions are
all true. The reach of its name is the problem, and a reader who greps for a
demotion ratchet finds it and stops looking.

**`GATED_FLOOR` is not the missing ratchet either.** It is a single integer in
the test file, currently 221, covering `rfc7296` alone. It was lowered from 222
when `RFC7296-2.8-1` was corrected. A floor that the correcting commit edits is a
record, not a ratchet, and 165 other enrolled RFCs have no floor at all.

**Why this matters for the release.** The goal driving the fixit backlog is that
every RFC MUST is implemented and tested. That claim rests on the gate. A gate
blind to demotion means the MUST population can shrink without anyone deciding
it should, and the shrink looks identical to green.

**The shape of the fix is a decision, not an edit.** A real level ratchet needs a
recorded MUST-level subset and a place to record an AUTHORISED correction, since
`RFC7296-2.8-1` proves corrections are sometimes right. That is the same shape as
the extraction sign-off (`rfc/extraction/<stem>.json`): a derived fact, an
authored disposition, and a reason required when the count moves the wrong way.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md` -- the seven ratchets, what each protects, and
      why "recording a problem is not addressing it" applies to levels too
- [ ] `rfc/extraction/README.md` -- the five properties of the sign-off contract,
      which is the closest existing analogue: derived facts, authored
      dispositions, a reason required to widen an exclusion
- [ ] `ai/RFC-REQUIREMENTS.md` -- the published backlog this gate feeds

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/rfc_requirements.py` -- every `check_` function, `is_gated`,
      `GATED_LEVELS`, and how `check_coverage_ratchet` and
      `check_retired_requirements` build their HEAD baselines
  → Constraint: the HEAD-comparison machinery already exists and is reused by
    several ratchets. A level ratchet extends it; it does not need a new one.
- [ ] `scripts/dev/rfc_requirements_test.py` -- `GATED_FLOOR`,
      `test_rfc7296_ids_are_neither_retired_nor_demoted`
  → Decision: that test's docstring already states the gap and the shape of the
    answer. Read it before designing.

**Behavior to preserve:**
- Every one of the seven existing ratchets, unchanged.
- A legitimate correction stays possible: `RFC7296-2.8-1` was right to move.
- The gate stays readable from `make ze-rfc-check` with no new entry point.

**Behavior to change:**
- A requirement leaving the MUST-level population fails the gate unless the
  commit records an authorised correction with a reason.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `make ze-rfc-check`. Also `make ze-precommit-verify`, which runs it as a stage.

### Transformation Path
1. Each `rfc/short/<stem>.md` is parsed into `Requirement` rows carrying `rid`,
   `level`, `section`.
2. `is_gated` decides membership of the gated population from `level`.
3. Coverage, evidence and retirement ratchets compare the working tree against
   HEAD over that population.
4. A level change silently changes the population in step 2, so step 3 compares
   a different set and reports nothing.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| gate <-> git | HEAD blobs of `rfc/short/*.md`, already read by the existing ratchets | Yes |
| gate <-> public ledger | `docs/features/rfc-status.md`, guarded by `check_status_completeness` | Yes, per `ai/rules/rfc-compliance.md` |
| gate <-> authorisation | nowhere: there is no place to record an authorised level correction | Yes, that absence is the gap |

### Integration Points
- `ai/rules/rfc-compliance.md` lists the ratchets and must gain the eighth.
- `docs/features/rfc-status.md` is the public claim a demotion silently narrows.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the ratchet joins the existing HEAD-comparison pass |
| No unintended coupling (components stay isolated) | Yes | stays inside `scripts/dev/` |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses the HEAD baseline machinery the other ratchets already build |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Python tooling |
| Registration over hardcoding | N-A | no registration surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|-----------|-------|----------|------------|--------|
| A-1 | The HEAD-baseline machinery can be reused as-is for levels | several ratchets already build one | a new baseline reader is needed and the change is larger | read `check_coverage_ratchet` and `check_retired_requirements` | unvalidated |
| A-2 | Demotions are rare enough that a required reason is not friction | one in the corpus this year | the reason becomes a rubber stamp | count level changes across the last 200 commits touching `rfc/short/` | unvalidated |
| A-3 | Today's levels are correct, so the baseline can be taken from HEAD | the corpus is checked per-RFC at enrolment | the ratchet freezes existing misquotes | sample: re-read N rows against their RFC text before freezing | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Freezing today's levels ratchets in a wrong one, and correcting it then costs a disclosure | a correction is refused on a row the RFC clearly contradicts | the authorised-correction path is part of the first deliverable, not a follow-up |
| R-2 | A promotion (SHOULD to MUST) is blocked by a ratchet aimed at demotion | a conformance improvement is refused | the ratchet is one-directional by construction: gaining gated rows never fires |

## Blast Radius

`make ze-rfc-check` and every commit touching `rfc/short/*.md`. No daemon code,
no wire behavior. The gate becomes stricter, so previously-green commits that
lowered a level would now be refused.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rfc-check` over a tree where one `rfc/short/` row moved MUST to SHOULD | -> | the new level ratchet | a case in `scripts/dev/rfc_requirements_test.py` |
| the same, with an authorised correction recorded | -> | the authorisation path | a second case, green |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A requirement's level moves from a gated level to an ungated one, with nothing recorded | `make ze-rfc-check` FAILS, naming the rid, both levels, and the RFC section |
| AC-2 | The same change, with an authorised correction recorded and a reason quoting the RFC sentence | The gate PASSES |
| AC-3 | A requirement's level moves from an ungated level to a gated one | The gate PASSES with no record required: conformance improvements are never gated |
| AC-4 | A requirement id disappears entirely | `check_retired_requirements` still fires, unchanged |
| AC-5 | The whole enrolled corpus, unmodified | The gate PASSES: the baseline matches the tree it was taken from |
| AC-6 | `test_rfc7296_ids_are_neither_retired_nor_demoted` | Renamed to what it actually asserts, or extended to assert what it is named for |
| AC-7 | `ai/rules/rfc-compliance.md` | Lists the eighth ratchet with its producer and its firing condition |

## End-to-End User Stories

- An agent re-reads an RFC, finds a row overstating the level, corrects it, and
  the gate makes it record the sentence that authorised the correction.
- An agent under gate pressure lowers a level to make a coverage red disappear,
  and the gate refuses.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| an unrecorded demotion fails the gate | `scripts/dev/rfc_requirements_test.py` | AC-1 | |
| a recorded, reasoned correction passes | `scripts/dev/rfc_requirements_test.py` | AC-2 | |
| a promotion needs no record | `scripts/dev/rfc_requirements_test.py` | AC-3 | |
| the unmodified corpus passes | `scripts/dev/rfc_requirements_test.py` | AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-rfc-check` on a demoted tree | the gate itself | an agent runs the gate and is refused | |

## Files to Modify

- `scripts/dev/rfc_requirements.py` -- the new check, beside the existing ratchets
- `scripts/dev/rfc_requirements_test.py` -- `GATED_FLOOR` and
  `test_rfc7296_ids_are_neither_retired_nor_demoted`, whose docstring already
  states this gap
- `ai/rules/rfc-compliance.md` -- the ratchet table, via its point file under
  `ai/rules/points/rfc-compliance/`
- `ai/RFC-REQUIREMENTS.md` -- if the published backlog gains a level column

## Files to Create

- whatever records an authorised correction, if the design puts it in a file
  rather than in the summary row

## Implementation Steps

1. **Phase: Reproduce** -- a case that demotes a row and expects a red
   - Verify: AC-1 is RED against today's gate, which reports clean
2. **Phase: Design the authorisation** -- where a correction is recorded, and what
   a reason must contain. Read `rfc/extraction/README.md` first: the sign-off
   contract already solved the derived-fact plus authored-disposition problem
   - Verify: the shape is written down before any check is added
3. **Phase: Baseline** -- take the MUST-level subset, validating A-3 by sampling
   - Verify: AC-5 over the unmodified corpus
4. **Phase: Ratchet** -- add the check, one-directional
   - Verify: AC-1, AC-2, AC-3, AC-4
5. **Phase: Correct the misleading test and the rule**
   - Verify: AC-6, AC-7, and `make ze-rules-condensed-update` then `make ze-rules-lint`

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations

- The ratchet freezes levels as they read today. It cannot tell a correct level
  from an incorrect one; only the RFC text does that. It stops a level moving
  without a decision, which is the property that was missing.
- It says nothing about SHOULD-level obligations, which are the backlog's second
  phase. A SHOULD demoted to MAY is the same shape one tier down.
