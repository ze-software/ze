# Spec: weakened-followups

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | tooling |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | - |
| Handoff | `0d210a7f5` |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two items left by `spec-weakened-per-commit`, homed here at the owner's
direction when the weekly token budget ran to 3% on day 3 (2026-08-16). Neither
blocks the mechanism, which is implemented and green.

**Item A: the branch audit still reads the retired token.**
`scripts/dev/audit-test-relaxation.py` is in the parent spec's Files to Modify
and no phase touched it. `_RELAX_LINE`, `_RELAX_LINE_ANY` and `relax_reasons`
still scan a diff for `test-relax:` comments, so `run_audit` shows a reviewer a
weakening that HAS an accepted row as an unexplained `[WEAKENED]`. `/ze-review`
step 0 and `/ze-review-deep` both drive it. The two skills now say plainly that
the audit does not read `test/weakened.md` and the reviewer must, so the gap is
disclosed rather than silent.

The design question the parent spec did not answer, and this one must: the audit
judges a BRANCH RANGE, and `test/weakened.md` is replaced per commit, so no
single version of the file covers the range. The rows are in history, one set
per commit, which `git log -p -- test/weakened.md` over the range yields.

**Item B: 129 rule points carry retrospective content.**
`ai/rules/points/rule-format/the-body-has-a-budget-too/what-keeps-a-rule-body-short.md`
gained a row on 2026-08-16: a point says what to DO next and carries no history.
The existing corpus predates it. Measured the same day: 129 of 2,238 points carry
a date, a post-mortem count, a commit SHA, or a past-tense account of a failure,
totalling about 160KB of the corpus's 900KB. Every one of them is read into every
session.

## Required Reading

### Architecture Docs
- [x] `ai/rules/rule-format.md` -- the body budget, and the row added 2026-08-16
  → Constraint: the new row is the acceptance test for item B. A point earns a
     permanent seat by changing what a reader DOES next.
- [x] `ai/rules/planning.md` -- how a deferred item is homed
  → Decision: this file is the destination spec for both items, so neither is an
     unhomed row.

### Source Files
- [x] `scripts/dev/audit-test-relaxation.py` -- `run_audit`,
      `audit_changes`, `accepted_rows`, `load_detector`
  → Constraint: `load_detector` is the shared import of the hook's
     `_test_weakening_errs` and MUST stay shared. Only the token scanning goes.
- [x] `scripts/dev/check_weakened_tests.py` -- `parse_weakened_file`,
      `weakened_units`, `row_matches`, `weakened_tests`
  → Decision: the audit reuses these rather than growing a second parser. That
     is why `row_matches` is public.
- [x] `scripts/dev/audit_relaxation_test.py` -- 22 cases
  → Constraint: 10 of the 19 are token-specific and must be rewritten, not
     deleted, or the audit loses its proof.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [x] `scripts/dev/audit-test-relaxation.py` - formerly scanned a diff for
      `test-relax:` tokens; now audits each commit with only that commit's rows
- [x] `scripts/dev/check_weakened_tests.py` - the per-commit checker the hook and
      `commit_helper.py` both share
- [x] `ai/rules/points/` - 2,238 point files, 129 carrying a retrospective signal

**Behavior to preserve:**
- The audit keeps importing the hook's detector through `load_detector`.
- A rule's directives keep their RFC 2119 level and their trigger line.
- A date that gives a live directive its AUTHORITY stays: it says whose decision
  it is and that it stands.

**Behavior to change:**
- The audit reads accepted rows from history instead of tokens from the diff.
- A rule point carries no post-mortem.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `/ze-review` step 0 runs the audit over a branch range.
- A rule author edits a point under `ai/rules/points/`.

### Transformation Path
1. The audit resolves the range's commits.
2. It collects each commit's `test/weakened.md` rows from history.
3. It matches them against the weakenings the range introduces.
4. It reports only weakenings no row in the range accepted.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| audit ↔ git history | reads each commit's changed row file and parent-to-commit test diff | Yes, `test_rows_are_read_from_every_commit_in_the_range` and repeated-unit regression |
| audit ↔ check_weakened_tests | shares `parse_weakened_file`, `weakened_units`, and `row_matches` | Yes, source trace through `accepted_rows` and `audit_changes` |

### Integration Points
- `run_audit` (`scripts/dev/audit-test-relaxation.py`)
- `/ze-review` step 0, `/ze-review-deep`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `run_audit` → `accepted_rows` / `audit_changes` → shared parser, detector, and matcher |
| No unintended coupling (components stay isolated) | Yes | the audit imports the hook detector and weakened-row module through their existing boundaries |
| No duplicated functionality (extends existing, does not recreate) | Yes | no second row parser, unit extractor, or name matcher |
| Zero-copy preserved where applicable (refs, not copies) | N-A | cold developer CLI over Git text; no encoding path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | no runtime registration or plugin surface changed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every accepted row over a range is recoverable from history | `test/weakened.md` is committed with the change it accepts (AC-10 of the parent spec) | the audit cannot judge a range and must report per commit | temporary `audit-range` branch: `git log -p <baseline>..HEAD -- test/weakened.md` returned the distinct `TestFirst` and `TestSecond` rows from its two commits | confirmed |
| A-2 | A date in a rule point is separable into authority and history | the 129 flagged points, read individually | a mechanical sweep strips a date that makes a ban read as negotiable | Phase B human disposition list in the shared state file; all 129 read, authority dates retained, ambiguous slug retained | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The audit's 10 token-specific tests are deleted rather than rewritten, and the audit loses its proof | the case count falls | rewrite each against a row; the count must not drop |
| R-2 | Item B strips a date that carried a live directive's authority | a ban starts reading as advice | ambiguous stays and is reported; never a regex sweep |
| R-3 | Item B is done as one bulk edit and collides with other sessions | a render race on the generated rule files and digests | batch by rule, one render at the end |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing an operator runs. Item A misleads a reviewer; item B costs context or weakens a rule's authority. |
| How is it reverted? | Single commit revert for each item. |
| Who else touches this path? | Every session reads the rules; `/ze-review` drives the audit. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `/ze-review` step 0 over a range whose commit carries a row | → | `run_audit` | `test_a_row_in_the_range_explains_the_weakening` |
| `/ze-review` step 0 over a range with a weakening and no row | → | `run_audit` | `test_a_weakening_with_no_row_in_the_range_is_reported` |
| `make ze-rules-lint` after item B | → | `rules_lint` | `test_every_rule_still_lints` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A branch range whose commits carry rows accepting every weakening in it | `run_audit` reports no unexplained weakening |
| AC-2 | A range with a weakening no row in the range accepts | `run_audit` reports it, naming the test |
| AC-3 | `scripts/dev/audit-test-relaxation.py` | scans no `test-relax:` token; `_RELAX_LINE` and `_RELAX_LINE_ANY` are gone |
| AC-4 | `scripts/dev/audit_relaxation_test.py` | has at least 19 cases, the 10 token-specific ones rewritten against rows rather than deleted |
| AC-5 | The 129 flagged rule points | each is read; history removed, authority-bearing dates kept, ambiguous kept and listed |
| AC-6 | After item B | `make ze-rules-lint` and `make ze-doc-test` pass, and every rule keeps its trigger and levels |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_a_row_in_the_range_explains_the_weakening` | `scripts/dev/audit_relaxation_test.py` | AC-1 | PASS, 22-test suite |
| `test_a_weakening_with_no_row_in_the_range_is_reported` | `scripts/dev/audit_relaxation_test.py` | AC-2 | PASS, 22-test suite |
| `test_the_audit_scans_no_token` | `scripts/dev/audit_relaxation_test.py` | AC-3 | PASS, 22-test suite |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| commits in the audited range | 1..n | n | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `weakened-audit-reads-history` | `scripts/dev/audit_relaxation_test.py` | a reviewer runs `/ze-review` on a branch and sees only weakenings the carrying commit accepted | PASS, 22-test suite and `--selftest` |

## Files to Modify
- `scripts/dev/audit-test-relaxation.py` - read rows from history, drop the token scan
- `scripts/dev/audit_relaxation_test.py` - rewrite the 10 token cases
- `ai/skills/ze-review.md`, `ai/skills/ze-review-deep.md` - drop the disclosure once item A lands
- `ai/rules/points/**` - the 129 flagged points, batched by rule

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | agent tooling; nothing reaches the daemon |
| YANG validation constraints | N-A | no YANG leaf |
| YANG custom validators | N-A | no YANG leaf |
| CLI commands/flags | N-A | no `ze` verb; the audit is a dev script |
| CLI grammar (keyword before value) | N-A | no CLI surface |
| Editor autocomplete | N-A | no YANG leaf |
| Functional test for new RPC/API | N-A | no RPC; the audit's harness is `scripts/dev/audit_relaxation_test.py`, named in the TDD plan |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | no `ze.*` env var |
| Doctor check for runtime dependencies | N-A | no runtime dependency: both items read tracked repo files from dev tooling |
| Prometheus counters/metrics | N-A | no daemon-observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | agent-facing tooling |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | the audience is agents |
| 7 | Wire format changed? | No | not protocol work |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | not protocol work |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, if item A changes what a reviewer runs |
| 11 | Affects daemon comparison? | No | nothing an operator sees |
| 12 | Internal architecture changed? | No | no runtime architecture |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no target added or removed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors naming `audit-test-relaxation.py` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/skills/ze-review.md` and `ze-review-deep.md` carry the disclosure item A removes |

## Implementation Steps

1. **Phase A1** -- the audit reads history, with its tests.
2. **Phase A2** -- remove the disclosure from the two review skills.
3. **Phase B** -- the rule sweep, batched by rule, one render at the end.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has code plus a passing test at file:line. AC-5 has no mechanical test: its evidence is the list of points read and the disposition of each |
| Correctness, item A | The audit collects rows from EVERY commit in the range, never from the worktree file. A range whose first commit accepts a weakening its last commit still carries must report nothing |
| No second parser | `run_audit` reaches the table through `check_weakened_tests.parse_weakened_file` and the pairing through `row_matches`. Grep for a second table reader or a second name matcher: either is the defect `rfc_tagged_scope.py`'s docstring records |
| Shared detector intact | `run_audit` still unpacks `(blocking, advisory)` from the detector `load_detector` imports. The audit and the two gates MUST NOT disagree about what a weakening is |
| Test count did not fall | `scripts/dev/audit_relaxation_test.py` keeps at least 19 cases. Ten were token-specific and are REWRITTEN against a row. A lower count means proof was deleted rather than moved (R-1) |
| Discrimination | Revert the history read and watch `test_a_row_in_the_range_explains_the_weakening` go red. A test green against both the old and the new reader proves nothing |
| Item B kept every obligation and ruling | Trace every changed MUST/MUST NOT point and every removed owner-ruling phrase to its surviving canonical location; sampling is not sufficient |
| Item B changed no directive | Every touched rule keeps its trigger line, its severity, and every RFC 2119 level. `make ze-rules-lint` proves the shape; the levels need a read |
| Item B is not a regex sweep | The diff shows per-point judgement, not one mechanical substitution. Ambiguous points stay and are listed (A-2) |
| Render is not raced | Points edited, then ONE `make ze-rules-render`, then the digests committed with the rule (R-3) |
| CLI grammar | N/A: no `ze` verb changes |
| Doctor checks | N/A: no runtime dependency; both items read tracked repo files from dev tooling |
| YANG validation | N/A: no YANG leaf |
| Prometheus counters | N/A: no daemon-observable state |

## Design Insights

Item B is the parent spec's own lesson applied to the rules: a record that every
reader pays for must change what the reader does.

## Key Design Decisions

| Decision | Why |
|----------|-----|
| The audit reads history, not the worktree file | A range spans commits, and each has its own replaced file |
| The sweep is read per point, never a regex | A date can carry authority or history, and only a reader can tell |

## Known Limitations

The semantic decision to remove or retain retrospective prose remains a human
read. Inventory completeness, frontmatter identity, and RFC 2119 token
preservation are mechanically cross-checked.

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-6 all demonstrated
- [x] Every user story has a working path and a passing test
- [x] Wiring Test table complete: every row has a concrete test name
- [x] Final scoped gate passed: `make ze-validate` and every targeted verification command below
- [x] Feature is integrated at the developer entry points; runtime `internal/*` / `cmd/*` is N-A
- [x] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [x] Architectural Verification table filled, including registration over hardcoding
- [x] Critical Review passes, including the final independent Review Gate
- [x] Every A-N confirmed, none `unvalidated`
- [x] Deferral shard is `-`; no live row needs a destination

### TDD
- [x] Tests written
- [x] Tests FAIL: discrimination mutation made the repeated-unit regression fail because the audit returned clean
- [x] Tests PASS: 22 tests in 9.720s, OK
- [x] Boundary tests cover 0 commits, 1..n commits, repeated units, worktree isolation, and rename
- [x] Functional behavior is covered by the real Python CLI harness; no daemon `.ci` applies
- [x] Interop tests N-A: no protocol behavior

### Closure
- [x] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [x] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [x] Learned-summary artifact N-A under the current owner directive; three session journal rows preserve the reusable failures
- [x] **Commit A:** remaining closure artifacts and completed spec (implementation is already in handoff `0d210a7f5`)
- [x] **Commit B:** remove only `plan/spec-weakened-followups.md` after Commit A preserves it

### Deliverables Checklist

| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| The branch audit reads accepted weakening rows from each commit that changes `test/weakened.md` | `python3 scripts/dev/audit_relaxation_test.py` | PASS, 22 tests in 9.720s |
| The audit reports a later unaccepted weakening of a unit that an earlier commit accepted | `test_an_earlier_row_does_not_accept_a_second_weakening_of_the_same_unit` | PASS; discrimination mutation failed as required |
| The review skills state the accepted-row suppression contract and contain no retired-token instructions | inspect `ai/skills/ze-review.md` and `ai/skills/ze-review-deep.md` | Done in handoff `0d210a7f5` |
| Each flagged rule point keeps its authority and loses only retrospective prose | complete 129-entry trace plus rule gates | Done: 81 changed, 48 unchanged, no missing or duplicate B entry/path |
| Rule directives state RFC 2119 levels and all changed prose passes the STE ratchet | rule-language, hook-fixture, lint, and render gates | PASS in supplied scoped verification |

### Security Review Checklist

| Check | What to look for | Result |
|-------|------------------|--------|
| Revision input | Git revisions and paths are passed as argument lists, never through a shell | PASS, `git()` receives argument arrays |
| Historical row parsing | Missing or malformed historical rows fail closed and cannot suppress an unexplained weakening | PASS, parser errors return `Audit.err` |
| Range isolation | A row suppresses only the weakening in the commit that carries the row | PASS after RG-1; three boundary tests cover repeated unit, worktree, and rename |
| Resource use | The audit reads only `test/weakened.md` and changed test files from the selected commit range | PASS; work is bounded by selected commits and changed tests |
| Privilege and secrets | The developer audit adds no runtime privilege, credential, network listener, or secret output | PASS, repository-local subprocess and file reads only |

---

## Implementation Summary

### What Was Implemented
- `accepted_rows`, `audit_changes`, and `run_audit` in
  `scripts/dev/audit-test-relaxation.py` now audit every parent-to-commit diff
  with only the weakening rows carried by that commit, then audit
  `HEAD`-to-worktree with no committed acceptance.
- `scripts/dev/audit_relaxation_test.py` contains 22 cases for history,
  malformed rows, per-commit isolation, repeated units, worktree isolation,
  renames, deleted files, non-Go carriers, and the shared RFC detector.
- `ai/skills/ze-review.md` and `ai/skills/ze-review-deep.md` state the
  accepted-row contract and no longer describe the retired token path.
- Every B-001 through B-129 point was read and dispositioned. The handoff
  changed 81 scoped points, left 48 scoped points unchanged, regenerated the
  affected rule documents, and kept the one ambiguous point.
- `go_func_scopes` in `scripts/dev/rfc_tagged_scope.py` caches immutable spans
  by exact content in a bounded LRU, removing repeated parsing from the RFC
  requirement gate.

### Bugs Found/Fixed
- **RG-1:** range-wide row flattening let an earlier accepted `TestA` suppress a
  later unaccepted weakening of `TestA`. Per-commit evaluation fixes the
  producer; `test_an_earlier_row_does_not_accept_a_second_weakening_of_the_same_unit`,
  the HEAD/worktree case, and the rename case cover the boundary.
- Two cleaned rule points narrated facts contradicted by their hook producers.
  The canonical points now state the live marker and session-lifecycle behavior;
  `plan/journal/rule-cleanup-changed-live-facts.md` records the failure class.
- One point cleanup left invalid blank-line spacing. The canonical point was
  corrected and rerendered; `plan/journal/rule-cleanup-left-invalid-spacing.md`
  records the failure class.
- `rfc_requirements_test.py` exceeded the 180-second package gate because
  `go_func_scopes` parsed 495 identical contents 10,278 times. The bounded
  exact-content cache reduced the direct 792-test script to 36.750 seconds;
  `plan/journal/test-gate-repeats-expensive-work.md` records the failure class.

### Documentation Updates
- Agent-facing review instructions changed in `ai/skills/ze-review.md` and
  `ai/skills/ze-review-deep.md`.
- Canonical rule points and their generated `ai/rules/*.md` documents changed.
  No user, CLI, API, configuration, protocol, or runtime architecture behavior
  changed. A `docs/` anchor search found only the existing tagged-scope claim in
  `docs/functional-tests.md`, whose semantics remain true.
- Supplied documentation evidence: `make ze-doc-test` passed every stage before
  a concurrent point edit made two generated files stale; the required narrow
  rerender was followed by clean `make ze-rules-render-check` and
  `make ze-rules-lint`.

### Deviations from Plan
- Shared-checkout commit `0d210a7f5` landed the scoped Phase A and Phase B
  implementation together with foreign rule-lint work before closure. This
  spec therefore entered verification status with that handoff; Commit A
  carries only the remaining closure files and does not duplicate attribution.
- RG-1 increased the audit suite from 19 to 22 tests and changed the range
  implementation from aggregate matching to per-commit matching.
- The RFC tagged-scope cache was not planned. It was the source fix for the
  blocking package-test timeout discovered during final verification.
- Review corrections required narrow rerenders after the planned render. They
  fixed producing point sources rather than editing generated documents.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Accepted rows were flattened over the whole range | One row accepts one commit's weakening, not later changes to the same unit | Independent Review Gate repeated-unit trace | Evaluate each commit separately and add three boundary tests |
| approach | Retrospective cleanup preserved stale hook facts | The producer uses different markers and has no `verification` outcome | Implementation critical review against hook producers | Correct canonical points, rerender, record journal row |
| approach | Point deletion left invalid spacing | Canonical point formatting is part of the render round trip | Narrow rule render/lint failure | Remove the extra blank line, rerender, record journal row |
| approach | RFC package verification timed out | One content parse was repeated 10,278 times | Direct 792-test script and call-count diagnosis | Add bounded exact-content LRU and record journal row |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Audit reads accepted rows from commit history | Done | `scripts/dev/audit-test-relaxation.py` `accepted_rows`, `audit_changes`, `run_audit` | Each commit is isolated; worktree receives no committed row |
| Remove retired token path without duplicating parser or detector | Done | `load_weakened_module`, `load_detector`; two review skills | Shared parser, matcher, unit extractor, and detector remain canonical |
| Read and disposition all 129 retrospective rule points | Done | B-001..B-129 in the session state; handoff `0d210a7f5` | 129 unique IDs and paths, 81 changed, 48 unchanged |
| Keep every directive level, trigger, and authority-bearing ruling | Done | complete obligation and ruling trace below | No sampling; no missing obligation or authority |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | accepted-row, every-commit, rename, deleted-file, and `.ci` cases | 22-test suite passed |
| AC-2 | Done | unexplained, wrong-name, wrong-qualifier, repeated-unit, and worktree cases | RG-1 regression discriminates aggregate reuse |
| AC-3 | Done | `test_the_audit_scans_no_token` | retired symbols, token, and `RELAXED` verdict absent |
| AC-4 | Done | 22 discovered tests in `scripts/dev/audit_relaxation_test.py` | count increased from 19; no case was deleted |
| AC-5 | Done | exact B-number/path inventory and complete semantic trace below | ambiguous B-094 retained |
| AC-6 | Done | rule lint, round trip, render check, doc test narrow rerun, and `make ze-validate` | frontmatter identical; obligations retained |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Accepted row explains carrying commit | Done | `test_a_row_in_the_range_explains_the_weakening` | PASS |
| Missing row reports named weakening | Done | `test_a_weakening_with_no_row_in_the_range_is_reported` | PASS |
| Audit scans no retired token | Done | `test_the_audit_scans_no_token` | PASS |
| Range isolation boundary | Done | repeated-unit, HEAD/worktree, and rename tests | Three cases passed in 1.108s |
| Full audit CLI behavior | Done | 22-test file and `--selftest` | 22 tests in 9.720s, OK; SELFTEST PASS |
| Rule corpus behavior | Done | rule lint, point round trip, render check | 28 rules conform and render fresh |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/audit-test-relaxation.py` | Done in handoff | Per-commit accepted rows and fail-closed reads |
| `scripts/dev/audit_relaxation_test.py` | Done in handoff | 22 behavioral cases |
| `ai/skills/ze-review.md`, `ai/skills/ze-review-deep.md` | Done in handoff | Accepted-row contract |
| 129 `ai/rules/points/**` paths and generated rules | Done in handoff | 81 changed, 48 unchanged |
| `scripts/dev/rfc_tagged_scope.py` | Changed | Verification-discovered source fix, remaining in Commit A |
| Three session-owned `plan/journal/*.md` rows | Changed | Required records for the three implementation failures |
| `plan/spec-weakened-followups.md` | Done | Verification handoff and completed closure record |

### Complete Rule-Point Inventory Proof
- The shared disposition list contains exactly 129 entries, B-001 through
  B-129, with 129 unique numbers, 129 unique paths, no gaps, no duplicate, and
  no missing file.
- In `0d210a7f5^..0d210a7f5`, 81 of those paths changed and 48 were unchanged.
  The observed 83-path point diff also contains
  `rule-format/directives/point-frontmatter-fields.md` and
  `rule-format/every-directive-states-a-level/every-directive-states-its-rfc-2119-level.md`.
  Those two foreign, non-B-numbered paths are not part of the AC-5 denominator.
- All 129 pre/post frontmatter blocks are byte-identical.

### Complete MUST-Level Obligation Trace
Every modified point with frontmatter `level: MUST` or `level: MUST NOT` was
treated as a potentially removed directive. The complete affected set is:
B-007, B-015, B-016, B-017, B-021, B-022, B-025, B-028, B-029, B-031,
B-032, B-035, B-039, B-043, B-046, B-048, B-049, B-050, B-054, B-059,
B-060, B-064, B-065, B-066, B-067, B-068, B-073, B-074, B-076, B-078,
B-085, B-086, B-087, B-088, B-105, B-106, B-107, and B-109.

For each ID, the surviving obligation lives in the same canonical path mapped
by the one-to-one B disposition list. Human old/new sentence review confirmed
that the obligated subject and action remain. The body RFC 2119 token multiset
is identical for 37 paths. B-065 is the separately identified foreign rewrite:
it strengthens the same deferral obligation from one `MUST` to three `MUST`
tokens and removes none.

### Complete Owner-Ruling Trace
| B ID | Removed wording examined | Surviving authority and location |
|------|--------------------------|----------------------------------|
| B-015 | Earlier supersession history | Owner directive and 2026-08-10 date remain in the same canonical point |
| B-048 | Helper collision history around commit and push workflow | Explicit owner-instruction push condition remains; the same point also retains the 2026-08-10 no-lesson-gate owner directive |
| B-049 | Historical lead-in to the tracked-build exemption | “Thomas ruled … KEEP IT” and 2026-08-04 remain in the same point |
| B-051 | History of the original absolute push ban | Thomas's 2026-08-05 amendment and its allowed-only condition remain in the same point |
| B-054 | History of the retired 600-line tier | Thomas's 2026-08-01 1000-line ruling remains in the same point |
| B-070 | Failure narrative after the delegation decision | Thomas's 2026-07-28 delegation shape remains in the same point |
| B-073 | Stale hook-registration history | Thomas's standing request remains in the same point; incorrect live facts were replaced from the producer |
| B-078 | History of the retired lesson-artifact gate | The 2026-08-10 and 2026-08-03 owner directives remain in the same point |
| B-105 | `(decision 2026-04-16)` history marker | Authority remains in byte-identical `level: MUST` frontmatter and the same imperative to extend the in-memory Go injector |

The B-022 deleted phrase “the owner asked” was historical narrative, not a
ruling or obligation. No other removed line in the 129-path diff contains an
owner, Thomas, ruling, or decision marker.

### Audit Summary
- **Total implementation rows:** 23 (4 task requirements, 6 ACs, 6 test groups, 7 file groups)
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 deviations recorded above

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A reviewer sees only weakenings accepted by the commit that introduced them | Functional and discrimination tests | 22 tests in 9.720s; repeated-unit mutation returned clean and made its regression fail, proving the test distinguishes the old aggregate behavior |
| The 129-point corpus loses retrospective cost without losing directives or authority | Complete inventory and semantic review | 129 unique B entries/paths; 81 changed, 48 unchanged; byte-identical frontmatter; all 38 changed MUST-level points and all owner-ruling candidates traced above |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| N-A | done | Metadata names no deferral shard; both inherited items are implemented and no live row remains |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/weakened-followups-shared.md` |
| `review_gate.py check` | `OK (3 code files, clean, hashes match)` |
| Rounds | 2: Run 1 found RG-1; Run 2 reviewed its source fix and the complete scoped handoff/worktree with 0 BLOCKER and 0 ISSUE |
| Reviewer lenses used | logic+wiring+removed behavior; security+edge cases+fail-closed guards; performance+simplicity+generated-rule authority |

### Run 1
- **BLOCKER RG-1:** `accepted_rows` flattened rows from every commit while
  `run_audit` matched one aggregate range diff. An earlier accepted unit name
  could authorize a later unaccepted weakening of the same unit.
- Five-part diagnosis: the symptom was cross-commit suppression; the root cause
  was discarded commit provenance; the owning layer was the audit data flow;
  manual reconciliation or discarding old rows were workarounds; per-commit
  evaluation was the source fix because it preserves valid acceptance and
  fails closed on later changes.

### Run 2
- Reviewed `accepted_rows`, `audit_changes`, and `run_audit`, all 22 audit
  cases, the bounded exact-content RFC scope cache, both review-skill changes,
  and the 129-point handoff diff.
- Wiring reaches the real `/ze-review` and `/ze-review-deep` audit entry point.
  No user-facing daemon behavior, runtime dependency, protocol, allocation hot
  path, or registration surface changed.
- Complete removed-behavior audit found no lost test assertion, directive,
  owner authority, error path, or guard after RG-1.
- Final result: **0 BLOCKER, 0 ISSUE**.

### Review-Gate Inventory and Authority Proof
- The final review independently reproduced the 129 unique ID/path result, the
  81 changed plus 48 unchanged denominator, the two excluded foreign point
  paths, and byte-identical frontmatter.
- It traced all 38 changed MUST/MUST NOT points and every owner-ruling candidate
  to the surviving locations recorded in the Implementation Audit. B-065 only
  strengthens its obligation. B-022 is narrative, not a ruling.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| RG-1 | BLOCKER | Earlier row accepted a later unaccepted weakening of the same unit | `scripts/dev/audit-test-relaxation.py` `accepted_rows`, `run_audit` | Per-commit `accepted_rows` plus `audit_changes`; three boundary tests |

## Pre-Commit Verification

All rows below attribute the already-run final scoped verification to the main
thread and the RG-1 verification handoff. Shared-checkout verification status
is not FRESH after concurrent commits, so commit preparation uses an accurate
`--unverified` reason rather than claiming a different tree was tested.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| Planned created files | N-A | `Files to Create` is `none`; the Python functional harness exists at `scripts/dev/audit_relaxation_test.py` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Carrying-commit rows explain weakenings | 22-test suite passed; every-commit and rename cases passed |
| AC-2 | Missing carrying-commit row reports the unit | repeated-unit, HEAD/worktree, wrong-name, and wrong-qualifier cases passed |
| AC-3 | Retired token scanner is absent | `test_the_audit_scans_no_token` passed |
| AC-4 | Test proof did not shrink | 22 tests in 9.720s, OK |
| AC-5 | All flagged points were judged | exact 129-entry/path proof and complete obligation/ruling trace above |
| AC-6 | Rule corpus and generated documents are valid | 28-rule lint and round trip passed; render check fresh after the narrow rerender |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `/ze-review` step 0, accepted weakening | Python CLI harness (no daemon `.ci`) | `test_a_row_in_the_range_explains_the_weakening`, PASS |
| `/ze-review` step 0, unaccepted weakening | Python CLI harness (no daemon `.ci`) | `test_a_weakening_with_no_row_in_the_range_is_reported`, PASS |
| Rule point authoring and render | Rule lint and render gates | 28 rules conform, round trip byte-identical, render fresh |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | temporary two-commit range recovered distinct rows; per-commit suite passed |
| A-2 | confirmed | all 129 points read; 38 MUST-level paths and every ruling candidate traced; ambiguous B-094 retained |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Review skills | Handoff `0d210a7f5` updates both skills to the per-commit accepted-row contract | Yes |
| Generated rule documents | Canonical-point render, 28-rule lint, point round trip, and render check | Yes |
| Test infrastructure docs | `docs/` anchor search found only `docs/functional-tests.md` tagged-scope claims; cache does not change scope semantics | No update needed |
| User guide, config, CLI, API/RPC, plugin, wire format, RFC status, metrics, daemon comparison | Scoped diff changes developer audit/rules only; no runtime or protocol producer changed | N-A |

### Final Scoped Verification
| Command | Result |
|---------|--------|
| `python3 scripts/dev/audit_relaxation_test.py` | 22 tests in 9.720s, OK |
| Three RG-1 boundary tests | 3 tests in 1.108s, OK |
| RG-1 discrimination mutation | repeated-unit test failed because old aggregate behavior returned clean |
| `python3 scripts/dev/audit-test-relaxation.py --selftest` | SELFTEST PASS |
| `python3 scripts/dev/audit-test-relaxation.py` | clean; HEAD `0d210a7f57e6` to worktree; 1 changed test file examined |
| `python3 scripts/dev/rfc_requirements_test.py` | 792 tests in 36.750s, OK |
| `make ze-rules-lint` | 28 rule files conform; 2232 RFC 2119-level points |
| `make ze-rules-points-roundtrip` | all 28 rules byte-identical |
| `make ze-rules-render-check` | 28 rules fresh after narrow rerender |
| `make ze-verify-wiring-docs` | wiring, discovery, digest, and citation checks passed |
| `make ze-doc-test` | all stages passed; concurrent stale render repaired and narrow checks passed |
| `make ze-validate` | all checks passed |
| `git diff --check` | clean |

## Core Insight

An acceptance record tied to a commit loses its safety property when a
range-level reader flattens away commit provenance. History must be evaluated
as parent-to-commit transitions before results are aggregated.
