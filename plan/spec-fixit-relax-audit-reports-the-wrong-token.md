# Spec: fixit-relax-audit-reports-the-wrong-token

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-vacuous-eor-family-tests.md` |
| Updated | 2026-08-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**AUDITED CLOSABLE 2026-08-17.** Independent audit judged every AC and Wiring row at its producer. AC-1..AC-3 are moot because the token scanner they describe no longer exists: `run_audit` (`scripts/dev/audit-test-relaxation.py`) is row-based over `test/weakened.md`, and `relax_reasons`, `_RELAX_LINE`, `test-relax:` and `"RELAXED"` are all absent from the script. AC-4 verified by running `scripts/dev/audit_relaxation_test.py` (22 tests, OK). The retirement is itself guarded by `test_the_audit_scans_no_token`, so the defect cannot return by the old path. Ready for the two-commit closure; the file is left in place because deleting user-visible work needs the owner's word (`ai/rules/never-destroy-work.md`).

## Task

`scripts/dev/audit-test-relaxation.py` `run_audit` pairs a file's NEW
`test-relax:` tokens with the wrong text when a token is added ANYWHERE except
after every token that was already there. It prints a reason belonging to an
older, unrelated relaxation, so a reviewer reads a justification that does not
describe the change in front of them.

The producing lines are in `run_audit`. It binds `old_tokens` to the LENGTH of
`relax_reasons(old, new_p)`, binds `new_tokens` to the LIST `relax_reasons(new,
new_p)`, and then takes the added set as `new_tokens[old_tokens:]`.

That slice reads the new list BY POSITION using the COUNT of the
old ones. That is only correct when every added token sorts after every existing
one. Insert a token near the top of a file that already has one further down and
the slice returns the pre-existing token instead of the new one.

**Reproduction.** In this repository at commit `e4cf75070`, add a `test-relax:`
comment near the top of `internal/component/bgp/reactor/peer_test.go`, which
already carries an MVPN relaxation lower down, then run
`python3 scripts/dev/audit-test-relaxation.py`. It prints
`reason: MVPNRoute / mvpnRouteGroupKey / groupMVPNRoutesByKey were removed by`,
which is the OLD token. The new one is never shown.

**How it was found.** An independent reviewer of
`spec-fixit-vacuous-eor-family-tests` noticed the audit quoting a
justification unrelated to that spec's change. Nothing in that spec depends on the
audit's text, so the defect blocks no goal and is written up here rather than
fixed there (`ai/rules/completion.md`, "A problem you FIND gets a SPEC").

**Severity beyond the wrong string.** The audit exists so a human can confirm each
relaxation's reason. A reason belonging to a different change makes that
confirmation worthless in the direction that matters: it reads as already
justified.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` -- the test-deletion and weakening rule the audit serves
- [ ] `ai/rules/repo-maintenance.md` -- the hook-to-rule mapping, which names the
      shared detector this audit imports
- [ ] `.claude/skills/ze-review/SKILL.md` -- step 0 runs this audit and tells the
      reviewer to quote each reason

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/audit-test-relaxation.py` -- `run_audit`, and `relax_reasons`
      which it calls twice
- [ ] `scripts/dev/audit_relaxation_test.py` -- whether any case adds a token
      before an existing one
  → Constraint: if no case does, the defect is invisible to the suite, and the
    first deliverable is the failing test.

**Behavior to preserve:**
- Every existing finding class: `[DELETED]`, `[WEAKENED]`, `[RELAXED]`, and the
  RFC-tagged-change branch that reuses the hook's own detector.
- The `[RELAXED]` versus `[WEAKENED]` split, which turns on whether any token was
  added at all.

**Behavior to change:**
- Added tokens are identified by IDENTITY, not by list position.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A reviewer runs `python3 scripts/dev/audit-test-relaxation.py` at step 0 of
  `/ze-review`. That is the ONLY entry point: no `mk/*.mk` target and no stage of
  `make ze-precommit-verify` invokes it, and `scripts/dev/commit_helper.py` does not either.
  So a wrong `reason:` line is seen by a human reviewer or by nobody.

### Transformation Path
1. `resolve_anchor` picks the commit to diff against; `changed_test_files` lists
   the changed test paths.
2. For each path, `run_audit` reads the OLD blob and the worktree file.
3. `relax_reasons` extracts the `test-relax:` token texts from each side.
4. The added set is derived, and its texts are printed as `reason: ...`.
5. Step 4 is the defect: the derivation is a positional slice.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Audit ↔ git | `git show <anchor>:<path>` for the old side | Yes, read in `run_audit` |
| Audit ↔ the write hook | `rfc_detector` is the hook's own `_rfc_tagged_change_err`, imported | Yes, read in `run_audit` |
| Audit ↔ reviewer | the printed `reason:` line is the only text a human confirms | Yes |

### Integration Points
- `.claude/skills/ze-review/SKILL.md` step 0 runs it and quotes each reason.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The fix stays inside `run_audit` |
| No unintended coupling (components stay isolated) | Yes | The shared RFC detector is untouched |
| No duplicated functionality (extends existing, does not recreate) | Yes | `relax_reasons` keeps its single caller pair |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Python tooling, not a hot path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No registration surface |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `python3 scripts/dev/audit-test-relaxation.py` over a file whose new token sits above an old one | → | `run_audit` added-token derivation | a new case in `scripts/dev/audit_relaxation_test.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A file with one existing `test-relax:` token gains a second one ABOVE it | The audit prints the NEW token's reason, never the old one |
| AC-2 | A file's only token is unchanged, and the file changes some other way | No token is reported as added, so the finding stays `[WEAKENED]` |
| AC-3 | A token's TEXT is edited in place, count unchanged | The audit reports the edited token as added, because its justification is new |
| AC-4 | Every existing case in `scripts/dev/audit_relaxation_test.py` | Still passes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| a new token added ABOVE an existing one reports its own reason | `scripts/dev/audit_relaxation_test.py` | AC-1 | |
| an unchanged token is not reported as added | `scripts/dev/audit_relaxation_test.py` | AC-2 | |
| a token edited in place is reported as added | `scripts/dev/audit_relaxation_test.py` | AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `scripts/dev/audit_relaxation_test.py` | `scripts/dev/` | The audit is a dev tool with no daemon surface, so it has no `.ci`. The suite's `audit()` fixture method runs the real script as a subprocess, which is the end-to-end test | |

## Files to Modify

- `scripts/dev/audit-test-relaxation.py` -- `run_audit`
- `scripts/dev/audit_relaxation_test.py` -- the failing case first

## Implementation Steps

1. **Phase: Reproduce** -- add the AC-1 case and watch it report the OLD token
   - Verify: the assertion names the new token and the test is RED
2. **Phase: Fix** -- derive the added set by identity in `run_audit`
   - Verify: AC-1 green, AC-2 and AC-3 added and green
3. **Phase: Regression** -- `make ze-unit-pkg-test PKG=./scripts/dev`
   - Verify: AC-4, every existing case still passes

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
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

- Matching by identity cannot tell a MOVED token from an unchanged one when the
  text is byte-identical. That is the correct answer anyway: an unchanged
  justification is not a new one.

---

## Implementation Summary

### What Was Implemented

- No code. The mechanism this spec describes was retired before the spec ran.
  `run_audit` (`scripts/dev/audit-test-relaxation.py`) reads no token: it walks
  `<anchor>..HEAD`, takes each commit's accepted rows from `test/weakened.md`
  through `accepted_rows`, and reports `[DELETED]` and `[WEAKENED]` findings in
  `report`. There is no `reason:` line and no positional slice.
- Verified at the producer on 2026-08-17: `relax_reasons`, `_RELAX_LINE`,
  `test-relax:` and `"RELAXED"` each return no hit in
  `scripts/dev/audit-test-relaxation.py`.
- The token is gone from the write side as well. `c_test_weakening`
  (`.claude/hooks/pretool-writeedit.py`) matches no `test-relax:` text: every
  mention left in that file is a comment about the retired hatch, and the
  justification route is a row in `test/weakened.md`, read through
  `scripts/dev/check_weakened_tests.py`. Both sides now agree on one hatch, which
  is why no wrong reason can be printed anywhere.

### Bugs Found/Fixed

- None owed by this closure. The defect was fixed on 2026-08-11 and then removed
  with its scanner: `plan/journal/escape-hatch-scoped-wider-than-its-justification.md`
  carries the row for `relax-token-gate-is-per-file-not-per-change`, whose Fix
  cell records that the audit diffs reason multisets.
- Found while closing, and not this spec's work: two rows of
  `plan/future/spec-harness-fail-open-guard-backlog.md` (D and I) named this spec
  as their surveyed destination. Row D also described the retired token. Both are
  repointed in commit A, and D is restated at its producer: `changed_test_files`
  builds its population from `git diff --name-status`, so an untracked test file
  is invisible to the audit.

### Documentation Updates

- None. `grep -rn "test-relax" docs/` returns one line,
  `docs/architecture/testing/test-health.md`, and it already dates the
  retirement ("Until 2026-08-16 the justification was a `// test-relax:` comment
  in the test"). No doc claims the audit scans the token today.

### Deviations from Plan

- Implementation Steps 1 to 3 are void. Each names an edit inside the token
  scanner, and the scanner does not exist.
- No test was written for AC-1 to AC-3. A test over a retired surface asserts
  nothing. `test_the_audit_scans_no_token` (`scripts/dev/audit_relaxation_test.py`)
  already pins the absence of `_RELAX_LINE`, `relax_reasons`, `test-relax:` and
  `"RELAXED"`, so the old path cannot return unnoticed.
- The Functional Tests row was N-A and now names the concrete driving surface,
  `scripts/dev/audit_relaxation_test.py`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The spec assumed its defect would still be reachable when somebody implemented it | The scanner that held the defect was retired, so every AC named a surface the tree had dropped | Closure read `run_audit` at its producer instead of trusting the Task text | Closed with no code change, plus a row in `plan/journal/comment-describes-superseded-behaviour.md`. The `stale-spec-claims-done` class is the closer fit, and its file already carries two rows from another live session, so the row went to the class that names the same failure in a clean file |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The audit must print the reason belonging to the token that was added | Changed | `run_audit`, `scripts/dev/audit-test-relaxation.py` | No reason line exists to be wrong. Weakenings are explained by rows in `test/weakened.md` |
| Preserve `[DELETED]`, `[WEAKENED]`, `[RELAXED]` and the RFC-tagged branch | Changed | `report` and `audit_changes`, same file | `[RELAXED]` went with the scanner. `[DELETED]` and `[WEAKENED]` remain, and `run_audit` still passes the hook's own `rfc_detector` through |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Changed | grep over `scripts/dev/audit-test-relaxation.py` finds no `test-relax:`, `relax_reasons`, `_RELAX_LINE` or `"RELAXED"` | A file cannot gain a second token in the audit's view, because the audit has no token view |
| AC-2 | Changed | same grep | The `[RELAXED]` versus `[WEAKENED]` split is gone. An unexplained weakening is `[WEAKENED]`, asserted by `test_a_weakening_with_no_row_in_the_range_is_reported` |
| AC-3 | Changed | same grep | An edited justification now lives in `test/weakened.md`, and `test_an_earlier_row_does_not_accept_a_second_weakening_of_the_same_unit` covers the case the AC was reaching for |
| AC-4 | Done | `python3 scripts/dev/audit_relaxation_test.py` -> `Ran 22 tests`, `OK` | Every existing case passes |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| a new token added ABOVE an existing one reports its own reason | Changed | - | Surface retired. `test_the_audit_scans_no_token` covers the retirement instead |
| an unchanged token is not reported as added | Changed | - | Same |
| a token edited in place is reported as added | Changed | - | Same |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/audit-test-relaxation.py` | Changed | Unchanged by this closure. Earlier work removed its token scanner |
| `scripts/dev/audit_relaxation_test.py` | Changed | Unchanged by this closure. It carries the retirement guard |

### Audit Summary

- **Total items:** 11
- **Done:** 1 (AC-4)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 10 (recorded in Deviations from Plan)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A reviewer never reads a justification that belongs to a different change | functional, the tool's own suite driving the real script | The audit prints no justification text. A finding names the file and the unit, and only an accepted row in the commit range removes it: `test_a_row_in_the_range_explains_the_weakening` and `test_a_weakening_with_no_row_in_the_range_is_reported`. `python3 scripts/dev/audit_relaxation_test.py` -> `Ran 22 tests`, `OK` |
| The wrong-text defect cannot return by the old path | unit | `test_the_audit_scans_no_token` reads the script's own source and fails if `_RELAX_LINE`, `relax_reasons`, `test-relax:` or `"RELAXED"` reappears |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 2026-08-09, the positional slice in `run_audit` prints the OLD token's reason (`plan/deferrals/fixit-vacuous-eor-family-tests.md`) | done | The scanner that held the slice was retired, verified at `run_audit`, and `test_the_audit_scans_no_token` pins the absence. Commit A marks the row `done`; commit B removes the shard, whose only row this was, and whose source spec `spec-fixit-vacuous-eor-family-tests` closed earlier |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-relax-audit-reports-the-wrong-token-7584d469-e988-48fc-910f-c68d4a139d89.md` |
| `review_gate.py check` | clean |
| Rounds | 2. Round 2 re-read the fixes of round 2's finding 2 against the hook and the shared row parser |
| Reviewer lenses used | claim versus producer (every closure sentence read against `run_audit`, `c_test_weakening` and the suite), record integrity (citers, deferral row, journal row format), documentation drift (`grep` over `docs/` and `ai/rules/` for the retired token) |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Two backlog rows named this spec as their destination, and row D described the retired token as a live surface | `plan/future/spec-harness-fail-open-guard-backlog.md`, rows D and I | Repointed both, and restated D at `changed_test_files` |
| 2 | ISSUE | This closure asserted a false product property: that `c_test_weakening` still matches the `test-relax:` token on the write side. The hook matches no such text; every mention left in it is a comment about the retired hatch | this spec, Implementation Summary and Documentation Verified | Re-read the hook, corrected both statements, and named the live route: a row in `test/weakened.md` parsed by `scripts/dev/check_weakened_tests.py`, which `load_weakened_module` in the audit imports as well |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/audit-test-relaxation.py` | Yes | `ls -l` -> 21576 bytes |
| `scripts/dev/audit_relaxation_test.py` | Yes | `ls -l` -> 23132 bytes |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | No token pairing exists to be wrong | `grep -n "relax_reasons\|_RELAX_LINE\|test-relax:\|RELAXED" scripts/dev/audit-test-relaxation.py` returns only `test/weakened.md` reader lines, no token line |
| AC-2 | An unexplained weakening reports `[WEAKENED]` | `test_a_weakening_with_no_row_in_the_range_is_reported` passes in the 22-test run |
| AC-3 | A changed justification is read from the commit's own rows | `test_rows_are_read_from_every_commit_in_the_range` and `test_an_earlier_row_does_not_accept_a_second_weakening_of_the_same_unit` pass in the same run |
| AC-4 | Every existing case still passes | `python3 scripts/dev/audit_relaxation_test.py` -> `Ran 22 tests in 4.027s`, `OK` |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `python3 scripts/dev/audit-test-relaxation.py` | `scripts/dev/audit_relaxation_test.py` | Yes for the entry point, and the token condition is unreachable. The suite's `audit()` fixture method runs the real script as a subprocess inside a temporary git repository, so every case drives the same command a reviewer types. No case can add a token above another, because the script reads no token |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| - | none declared | The spec carries no Risks & Assumptions section. Its one implicit assumption, that the defect would still be reachable, is recorded as broken in the Mistake Log |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No document presents the token scanner as a live audit surface | `grep -rn "test-relax" docs/` -> one hit, `docs/architecture/testing/test-health.md`, which already reads "Until 2026-08-16 the justification was a `// test-relax:` comment in the test" | Yes |
| No rule teaches the token as a live escape hatch | `grep -n "weakened.md\|test-relax" ai/rules/testing.md` -> the rule teaches a row in `test/weakened.md`, and every `test-relax` hit in `.claude/hooks/pretool-writeedit.py` sits in a comment describing the retired hatch, not in a live match | Yes |
