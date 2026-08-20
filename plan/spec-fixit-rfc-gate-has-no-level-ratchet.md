# Spec: fixit-rfc-gate-has-no-level-ratchet

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-17 |

→ Constraint: Status is left as the implementing session found it. `spec-session.sh claim`
  exits 3 (26 specs in-progress against a cap of 12) and editing the field by hand to
  route around that check is banned (`ai/skills/ze-implement.md` step 1). The transition
  is the main thread's.

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

**Three, not one (audited 2026-08-17).** A walk over all 200 commits that touched
`rfc/short/` -- 721 file revisions, 4715 requirement ids -- finds THREE ids that were
MUST-level at some revision and are advisory now, all three while their RFC was enrolled:
`RFC7296-2.8-1` and `RFC7947-x-3` on 2026-08-15, in a commit whose subject says it
corrected five misquoting rows, and `RFC7947-x-1` on 2026-07-23, inside a commit about
prepending the local AS on eBGP announces. Each is textually correct and each carries a
`Correction` paragraph in its summary, so the corpus is sound; what the corpus does not
have is a control. The 2026-07-23 case is the shape that matters: a level moved inside a
commit about something else, and nothing in the gate looked. One more datum from the same
walk, outside this spec and NOT a defect: `RFC5036-2.4.2-1` was MUST-level and left
`rfc/short/rfc5036.md` on 2026-07-20, before `check_retired_requirements` existed. Its
removal is recorded in that summary's own prose, which states that Section 2.4.2 is
Extended Discovery and carries no hold-time obligation, so the row was a misattribution
rather than a dropped MUST. It is named here because the audit surfaced it, and because
under today's gate the same repair would have to keep the id.

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
  → Decision (2026-08-17): the authorisation is recorded WHERE THE CORPUS ALREADY
    RECORDS IT. `rfc/short/rfc7947.md` carries three dated `Correction <YYYY-MM-DD>:`
    paragraphs and `rfc/short/rfc7296.md` two, each naming the id in backticks and quoting
    the RFC, one spelling blockquoted and one plain. `parse_corrections` reads both. No new
    file, no new annotation kind: a parallel ledger beside a convention already in the
    tree is layering (`ai/rules/no-layering.md`), and this record sits beside the row a
    reader is looking at.
  → Decision (2026-08-17): the reason MUST quote the RFC verbatim, at least 24 characters
    after whitespace is squashed (`MIN_CORRECTION_QUOTE`, `authorising_correction`). A
    free-text reason beside the row it excuses is what `GATED_FLOOR` already was: a note
    the demoting commit wrote about itself. The quotation is checked against
    `source_text`, so it is evidence rather than assertion; a missing RFC text fails
    the check CLOSED rather than waving the demotion through.
  → Decision (2026-08-17): the quotation is NOT required to contain the new level's
    keyword. It reads well and refuses the best correction in the corpus: RFC7947-x-1 is
    justified by RFC 7947's own "is a recommendation rather than a requirement", which
    states the strength and spells no keyword.
  → Decision (2026-08-17): `GATED_FLOOR` stays, with its comment rewritten to say it is a
    record and not the control. Deleting a live assertion is a coverage reduction
    (`ai/rules/testing.md`); what it still reaches is a count no HEAD-to-HEAD comparison
    can supply, a mass deletion spread over several commits.

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
| A-1 | The HEAD-baseline machinery can be reused as-is for levels | several ratchets already build one | a new baseline reader is needed and the change is larger | read `check_coverage_ratchet` and `check_retired_requirements` | confirmed: `_git_baseline_ids` already parsed every HEAD summary and threw the level away. It now derives from `_git_baseline_levels`, which keeps the level. One reader, 0.2s per call |
| A-2 | Demotions are rare enough that a required reason is not friction | one in the corpus this year | the reason becomes a rubber stamp | count level changes across the last 200 commits touching `rfc/short/` | confirmed: 200 commits, 721 file revisions, 4715 ids ever seen, THREE gated-to-advisory moves (RFC7296-2.8-1, RFC7947-x-1, RFC7947-x-3) |
| A-3 | Today's levels are correct, so the baseline can be taken from HEAD | the corpus is checked per-RFC at enrolment | the ratchet freezes existing misquotes | sample: re-read N rows against their RFC text before freezing | confirmed on the sample that matters: all three rows that MOVED carry a correction whose quotation is verbatim in the RFC source, so each is a repaired misquote rather than a reduction. The 4699 rows that never moved are unsampled, and the ratchet's own Known Limitation covers them |

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
| `make ze-rfc-check` over a tree where one `rfc/short/` row moved MUST to SHOULD | -> | `run_check` -> `check_level_ratchet` | `TestLevelRatchetWiring.test_run_check_fails_on_an_unrecorded_demotion` |
| the same, with an authorised correction recorded | -> | `summary_corrections` -> `authorising_correction` | `TestLevelRatchetWiring.test_run_check_passes_with_the_correction_recorded` |

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
| `TestLevelRatchet.test_unrecorded_demotion_fails` | `scripts/dev/rfc_requirements_test.py` | AC-1 | done |
| `TestLevelRatchet.test_recorded_correction_passes` | `scripts/dev/rfc_requirements_test.py` | AC-2 | done |
| `TestLevelRatchet.test_promotion_needs_no_record` | `scripts/dev/rfc_requirements_test.py` | AC-3 | done |
| `TestLevelRatchet.test_retired_row_is_left_to_its_own_ratchet` | `scripts/dev/rfc_requirements_test.py` | AC-4 | done |
| `TestRealTreeIsGreen.test_run_check_exits_zero_on_the_real_tree` plus the mirror comparison against HEAD's own script | `scripts/dev/rfc_requirements_test.py` | AC-5 | done: the new check adds no violation to the corpus |
| `TestLevelRatchet` -- the correction that names another id, the quotation absent from the RFC, the keyword-sized quotation, the missing RFC text | `scripts/dev/rfc_requirements_test.py` | AC-2 discrimination | done |
| `TestParseCorrections` -- both spellings, a wrapped quotation, prose that is not a correction, a correction with no quotation | `scripts/dev/rfc_requirements_test.py` | the reader | done |
| `TestCorrectionsInTheRealCorpus` | `scripts/dev/rfc_requirements_test.py` | the three corrections the tree already carries authorise their rows; an uncorrected row is not authorised | done |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestLevelRatchetWiring` drives `run_check` end to end | `scripts/dev/rfc_requirements_test.py` | an agent runs the gate over a demoted tree and is refused; the same tree with the correction recorded passes | done |
| `TestRealTree.test_rfc7296_pre_pilot_musts_are_still_gated_or_corrected` over the REAL summary and the REAL RFC text | `scripts/dev/rfc_requirements_test.py` | the corpus's one recorded demotion is authorised, and it is refused the moment the correction paragraph is removed | done |
| `make ze-rfc-check` over a mirrored tree with one row demoted | `scripts/dev/rfc_requirements.py`, driven from the session scratch mirror | the gate reports the demotion and nothing else changes | done |

## Files to Modify

- `scripts/dev/rfc_requirements.py` -- `Correction`, `parse_corrections`,
  `summary_corrections`, `authorising_correction`, `check_level_ratchet`,
  `_git_baseline_levels`, and the `run_check` call site
- `scripts/dev/rfc_requirements_test.py` -- `TestParseCorrections`, `TestLevelRatchet`,
  `TestLevelRatchetWiring`, `TestCorrectionsInTheRealCorpus`, `PRE_PILOT_LEVELS` and
  `test_rfc7296_pre_pilot_musts_are_still_gated_or_corrected`; the misleading test renamed
  to `test_rfc7296_ids_are_neither_retired_nor_lose_a_polarity`; the `GATED_FLOOR` comment
- `ai/rules/points/rfc-compliance/` -- the section renamed to "the eight ratchets", the
  eighth table row, and one row in the ask-Thomas table for lowering a level. Rendered
  into `ai/rules/rfc-compliance.md`, `ai/rules/CORE.md`, `ai/rules/TRIGGERS.md`,
  `ai/rules/INDEX.md`
- `ai/rules/points/RETIRED.md` -- ten rows, one per point id the section rename retired
- `ai/skills/ze-rfc.md` -- "Correcting a LEVEL", the convention a summary author needs
- `ai/RFC-REQUIREMENTS.md` -- unchanged. The published backlog needs no level column: the
  level is already in each row's text, and the gate reads the summaries

## Files to Create

- none. The authorisation is a paragraph in the summary that already holds the row
  (see the Decision in Current Behavior)

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
