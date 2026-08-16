# Spec: weakened-followups

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/3 |
| Deferral shard | - |
| Handoff | - |
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
- [ ] `ai/rules/rule-format.md` -- the body budget, and the row added 2026-08-16
  → Constraint: the new row is the acceptance test for item B. A point earns a
     permanent seat by changing what a reader DOES next.
- [ ] `ai/rules/planning.md` -- how a deferred item is homed
  → Decision: this file is the destination spec for both items, so neither is an
     unhomed row.

### Source Files
- [ ] `scripts/dev/audit-test-relaxation.py` -- `relax_reasons`, `run_audit`,
      `load_detector`, `_RELAX_LINE`, `_RELAX_LINE_ANY`
  → Constraint: `load_detector` is the shared import of the hook's
     `_test_weakening_errs` and MUST stay shared. Only the token scanning goes.
- [ ] `scripts/dev/check_weakened_tests.py` -- `parse_weakened_file`,
      `weakened_units`, `row_matches`, `weakened_tests`
  → Decision: the audit reuses these rather than growing a second parser. That
     is why `row_matches` is public.
- [ ] `scripts/dev/audit_relaxation_test.py` -- 19 cases
  → Constraint: 10 of the 19 are token-specific and must be rewritten, not
     deleted, or the audit loses its proof.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/dev/audit-test-relaxation.py` - scans a diff for `test-relax:`
      tokens and reports `[RELAXED]` / `[WEAKENED]` findings over a branch range
- [ ] `scripts/dev/check_weakened_tests.py` - the per-commit checker the hook and
      `commit_helper.py` both share
- [ ] `ai/rules/points/` - 2,238 point files, 129 carrying a retrospective signal

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
| audit ↔ git history | reads past versions of one file over a range | No |
| audit ↔ check_weakened_tests | shares the parser and `row_matches` | No |

### Integration Points
- `run_audit` (`scripts/dev/audit-test-relaxation.py`)
- `/ze-review` step 0, `/ze-review-deep`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every accepted row over a range is recoverable from history | `test/weakened.md` is committed with the change it accepts (AC-10 of the parent spec) | the audit cannot judge a range and must report per commit | `git log -p -- test/weakened.md` over a range that carries a weakening | unvalidated |
| A-2 | A date in a rule point is separable into authority and history | the 129 flagged points, read individually | a mechanical sweep strips a date that makes a ban read as negotiable | a human read of each flagged point; ambiguous stays | unvalidated |

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
| `test_a_row_in_the_range_explains_the_weakening` | `scripts/dev/audit_relaxation_test.py` | AC-1 | |
| `test_a_weakening_with_no_row_in_the_range_is_reported` | `scripts/dev/audit_relaxation_test.py` | AC-2 | |
| `test_the_audit_scans_no_token` | `scripts/dev/audit_relaxation_test.py` | AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| commits in the audited range | 1..n | n | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `weakened-audit-reads-history` | `scripts/dev/audit_relaxation_test.py` | a reviewer runs `/ze-review` on a branch and sees only weakenings nobody accepted | |

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
| Item B kept authority | Spot-check points that lost a date: one carrying an owner decision stays, one narrating a past failure goes. A ban that now reads as advice is the failure (R-2) |
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

Item B cannot be mechanically verified. Its acceptance is a human read.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
