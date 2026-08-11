# Spec: relax-token-gate-is-per-file-not-per-change

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/relax-token-gate-is-per-file-not-per-change.md` |
| Updated | 2026-08-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The test-weakening gate can be switched off for a whole file, and the audit that
reports relaxations can print the wrong reason. Both come from one shape: the
`test-relax:` token is matched against a whole file, or by position in a list,
never against the change that needs it.

**The gate opens per file, on the Write path.** `c_test_weakening` in
`.claude/hooks/pretool-writeedit.py` calls `_has_relax_token` and returns early,
before `_test_weakening_errs` runs. On the `Edit` and `MultiEdit` branches the
text it searches is the replacement string, so the token must sit inside the
change, and that is correct. On the `Write` branch the text is the ENTIRE new
file, so one pre-existing token anywhere in that file disables the weakening
check for every later overwrite of it. The hook's own RFC-branch message states
the opposite, that the check reads only the replacement text of this edit. That
sentence is true for Edit and false for Write. `_has_relax_token`'s docstring
counts 315 files carrying the hash-comment form of the token, so the population
that can be overwritten unchecked is large.

This is a guard that fails open and says nothing, which `ai/rules/evidence.md`
refuses.

**The audit names reasons by position.** `run_audit` in
`scripts/dev/audit-test-relaxation.py` counts the old file's tokens, then takes
the tail of the new file's token list from that index. A reason inserted
anywhere but last makes the audit print a different, pre-existing reason. Found
on 2026-08-09 while adding a third token to `test/install/kernel-compose.ci`:
the marker had to be placed where the slice would find it rather than beside the
lines it explains.

Reproduction, gate: overwrite with Write any test file that already carries a
`test-relax:` token, deleting assertions in the overwrite. The check returns
without looking. Reproduction, audit: add a reason above a middle hunk of a file
that already carries one, then read the reason the audit prints.

## Required Reading

### Architecture Docs
- [ ] `.claude/hooks/README.md` - what each hook gates and how it is tested
  → Constraint: the fix must keep the Edit path exactly as it is; only Write is wrong
- [ ] `ai/rules/testing.md` - the standing ban on reaching green by weakening a test
  → Decision: the token is an auditable escape hatch, not a switch, so it must bind to a change
- [ ] `ai/rules/evidence.md` - a guard fails closed or says something
  → Constraint: the silence on the Write path is the defect, not the strictness

**Key insights:**
- The two producers share one detection module: the audit imports the hook's
  detector, so a change to the token model must satisfy both readers.

## Current Behavior (MANDATORY)

Source files read:
- [ ] `.claude/hooks/pretool-writeedit.py` - `c_test_weakening` sets the compared
  text from `old_string` and `new_string` on the Edit branch, joins the edits on
  the MultiEdit branch, and on the Write branch reads the file from disk as the
  old text and takes the tool's whole `content` as the new text. It then calls
  `_has_relax_token` on that new text and returns None on a hit, before
  `_test_weakening_errs` is called. The Write branch also leaves `hunks` empty,
  so `_enclosing_tagged_scope` has nothing to widen on.
- [ ] `scripts/dev/audit-test-relaxation.py` - `run_audit` counts the old file's
  reasons and slices the new file's reason list from that index, so the reported
  reasons are whichever tokens sit at the tail positions.
- [ ] `scripts/dev/audit_relaxation_test.py` - the audit's own tests, built on
  fixture repositories that symlink the real hook.

Behavior to preserve: the three properties below, unchanged by the fix.
- The Edit and MultiEdit paths keep binding the token to the replacement text.
- The RFC-tagged branch keeps running BEFORE the token escape hatch, because
  the token is self-service and RFC approval is not.
- A documented relaxation carrying a genuine reason keeps passing, on every path.

## Data Flow (MANDATORY)

### Entry Point
A `Write`, `Edit`, or `MultiEdit` tool call on a test file, and the
`scripts/dev/audit-test-relaxation.py` run that reports what changed.

### Transformation Path
1. The hook decides the target is a test carrier.
2. It builds the old text and the new text, per tool branch.
3. The RFC-tagged branch runs and can block.
4. `_has_relax_token` searches the new text and returns None on a hit.
5. `_test_weakening_errs` compares counts and shapes, and blocks on a reduction.
6. Separately, the audit imports that detector and reports added reasons by slice.

### Boundaries Crossed

| From | To | Carried |
|------|----|---------|
| Tool call | hook stdin | tool name, file path, replacement text or whole content |
| Hook | audit script | the imported weakening detector |
| Audit | operator output | the reason text attributed to a change |

### Integration Points
- `scripts/dev/hook-fixture-check.py` runs the hook's fixtures.
- `scripts/dev/audit_relaxation_test.py` runs the audit's own tests.
- Any commit path that calls the audit before a commit.

### Architectural Verification
- One detection module stays one module. The fix must not fork the token model
  into a hook copy and an audit copy.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | Validation |
|----|------------|-------|-----------|
| A-1 | A Write on a test file carrying a token is reachable in normal work | agents overwrite whole `.ci` files | write the fixture and watch the gate return None |
| A-2 | Binding the token to the changed region is expressible on the Write path | the branch already holds both whole texts | design phase derives the changed region from a diff of the two |

### Risks

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | A stricter Write path blocks a legitimate whole-file rewrite that carries its reason elsewhere | the block message names the line the token must sit on |
| R-2 | Moving reason attribution off the positional slice changes the audit output for existing files | run the audit over the repository before and after, and compare |

## Blast Radius

Every test file edit in the repository passes through this gate, and every
relaxation report reads that attribution.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| A Write that deletes assertions from a test file already carrying a token | -> | the Write branch of `c_test_weakening` | a hook fixture named for the Write bypass |
| A reason added above a middle hunk | -> | reason attribution in `run_audit` | a case in `scripts/dev/audit_relaxation_test.py` |

## Acceptance Criteria

| AC | Input / Condition | Expected Behavior |
|----|-------------------|-------------------|
| AC-1 | A Write deletes assertions from a test file whose pre-existing token is unrelated | the gate blocks and names what was deleted |
| AC-2 | A Write deletes assertions and carries its own reason on the changed lines | the gate allows it, as Edit does today |
| AC-3 | A reason is added above a middle hunk | the audit prints that reason, not a pre-existing one |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| Write bypass is refused | the hook fixture set | AC-1 |
| Write with an in-change reason passes | the hook fixture set | AC-2 |
| reason attribution is not positional | `scripts/dev/audit_relaxation_test.py` | AC-3 |

### Functional Tests

| Test | File | Validates |
|------|------|-----------|
| none | - | the surface is a hook and a script, with no daemon path |

## Files to Modify

- `.claude/hooks/pretool-writeedit.py` - bind the token to the changed region on the Write path
- `scripts/dev/audit-test-relaxation.py` - attribute an added reason to its hunk, not to a tail slice
- `scripts/dev/audit_relaxation_test.py` - the AC-3 case
- `.claude/hooks/README.md` - correct the statement that the check reads only the replacement text

## Files to Create

None expected; the fixture set already has a home.

### Integration Checklist
- [ ] The hook fixture runner and the audit test both pass
- [ ] The audit output over the whole repository is compared before and after

## Implementation Steps

1. Derive the changed region on the Write path and search only it for the token.
2. Attribute an added reason to the hunk it sits on in the audit.
3. Correct the hook message and the README sentence that contradict the Write path.

### Critical Review Checklist
- [ ] The Edit and MultiEdit paths behave identically after the fix
- [ ] The RFC-tagged branch still runs before the token escape hatch
- [ ] The block message names the line the reason must sit on

## Known Limitations

The token stays self-service by design. This spec makes it bind to a change; it
does not make it an approval.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1, AC-2, AC-3 each proven by a named test
- [ ] Tests written
- [ ] Tests FAIL before the fix
- [ ] Tests PASS after the fix

### Quality Gates
- [ ] `make ze-verify`
- [ ] `make ze-lint-changed`

### Closure
- [ ] Deferral shard row closed

---

## Deliverables Checklist

<!-- Added at closure. This spec was filed as a `skeleton` defect record on
     2026-08-09 and never went through /ze-spec, so it carried no Deliverables,
     Security or Documentation checklist. They are filled here from the finished
     work rather than left absent, because /ze-close consumes them. -->

| Deliverable | Verification | Evidence |
|-------------|--------------|----------|
| The hatch binds to the change, on every tool branch | hook fixture | `relax-token-does-not-persist-across-a-write`, `relax-token-carried-through-a-hunk-does-not-exempt` PASS |
| A documented relaxation still passes | hook fixture | `relax-token-written-in-this-edit-accepted`, `relax-ci-hash-token-accepted`, `relax-ci-legacy-slash-token-still-accepted` PASS |
| Reason attribution is not positional | unit test | `test_a_token_inserted_at_the_top_is_the_one_reported`, `test_deleting_one_token_and_adding_another_is_not_silent` PASS |
| Edit and MultiEdit unchanged in behaviour | hook fixture + parity | 392/392 `hook-fixture-check.py`, `hook-parity-check.py` OK |
| RFC branch still precedes the hatch | hook fixture | `rfc-guard-relax-token-insufficient` PASS |
| One detection module, not two | source | `scripts/dev/audit-test-relaxation.py` `load_detector` still imports `_test_weakening_errs` from the hook |

## Security Review Checklist

| Concern | Finding |
|---------|---------|
| The hatch is an authorization bypass by design; does this change widen it? | No. It narrows it. `_writes_new_relax_reason` requires a justification NEW in this edit and a higher reason count, where `_has_relax_token` accepted any token anywhere in the replacement text |
| Can the narrowed hatch be forged? | Only by writing a fresh justification, which is the intended cost. Text-only keying was forgeable with one invisible character (U+200B); the count condition closes that. Pinned by `relax-token-zero-width-space-is-not-a-new-justification` |
| Does the RFC-approval path stay above the hatch? | Yes. `c_test_weakening` calls `_rfc_tagged_change_err` before the hatch; `rfc-guard-relax-token-insufficient` pins it |
| Untrusted input / injection | None. Both producers read file text and tool payloads and emit only counts and quoted text. No shell, no eval, no path construction from content |
| Unbounded allocation | `_relax_reasons` and `relax_reasons` are linear in the text; `_REASON_MAX_LINES` caps the per-reason walk. `relax-census.py` reads one tracked file at a time |
| Path traversal | `tracked_test_files` takes paths from `git ls-tree`/`git ls-files` only, never from content |
| Fail-open paths | Removed. `census` refuses (exit 2) on an unreadable tracked file and on a zero count against a live ceiling, where it previously skipped and reported clean |

## Documentation Update Checklist

| Category | Update needed? | File and section |
|----------|----------------|------------------|
| Feature list | No | No user-facing feature; this is a dev gate |
| User guide | No | Not operator-visible |
| Config syntax | No | No config touched |
| CLI reference | No | No `ze` command changed |
| API/RPC | No | None touched |
| Plugin SDK | No | None touched |
| Wire format | No | None touched |
| RFC compliance | No | No protocol behaviour; `rfc-guard-*` fixtures unchanged in intent |
| Comparison table | No | Not a product capability |
| Test infrastructure | **Yes** | `docs/architecture/testing/test-health.md`, new section "The `test-relax:` ceiling"; `ai/INDEX.md` Dev Tools row for `make ze-relax-census`; `ai/rules/points/testing/make-targets/...` inventory row; `ai/rules/points/repo-maintenance/hook-to-rule-mapping/...` `c_test_weakening` row; `ai/rules/points/testing/test-deletion-and-weakening/...` directive; `ai/skills/ze-review.md` step 0 |
| Architecture design | **Yes** | Same `test-health.md` section: what it counts, why HEAD and not the working tree |
| Doctor checks | No | No new runtime dependency |

`make ze-doc-test` PASSED after these updates.

---

## Implementation Summary

### What Was Implemented

- `_relax_reasons` / `_writes_new_relax_reason` (`.claude/hooks/pretool-writeedit.py`)
  replace `_has_relax_token` + a whole-text search. The hatch opens only when this
  edit writes a justification that is NEW as a normalized sentence AND raises the
  reason count. `_RELAX_TOKEN` / `_RELAX_TOKEN_ANY` were deleted with their last
  caller.
- `run_audit` (`scripts/dev/audit-test-relaxation.py`) attributes added reasons by
  multiset difference instead of a tail slice, and reports what disappeared as well
  as what arrived.
- `relax_reasons` captures the whole multi-line justification; `report` wraps it.

Beyond the spec, in the same change because they are the same defect class
(`TEST-RELAX-AUDIT.md`):

- The `.ci`/`.et` arm of `_test_weakening_errs` counts what can fail a run
  (`_CI_COVERAGE`, `_CI_REJECT`, `_CI_EMPTY_NEEDLE`) instead of non-comment lines.
- `_TAUTOLOGY` refuses an assertion that cannot fail.
- `is_test` in `c_test_weakening` admits `.et`; the guard had been inert over all
  164 editor tests.
- Counting arms read comment-stripped text on both carriers, so prose can neither
  pay for a deleted assertion nor be blocked as one.
- `scripts/dev/relax-census.py` + `test/relax-ceiling.txt` + `make ze-relax-census`
  cap the token stock, in `ze-verify` both modes.

### Bugs Found/Fixed

| Bug | Where | Fix |
|-----|-------|-----|
| Hatch was per-file under `Write` (468 files exempt) | `c_test_weakening` | `_writes_new_relax_reason` |
| Added reason reported by position | `run_audit` | multiset difference |
| Reason truncated to its first line (62% ran longer) | `relax_reasons` | continuation walk |
| `.ci` judged by line count: refused improvements, admitted gutting | `_test_weakening_errs` | `_CI_COVERAGE` |
| `.et` never judged at all | `is_test` | `.et` admitted |
| Prose counted as an assertion, so the gate refused its own cleanup | `_test_weakening_errs` | comment-stripped counting |
| Nothing counted the stock | none existed | `make ze-relax-census` |

### Documentation Updates

`docs/architecture/testing/test-health.md` (new section), `ai/INDEX.md`,
`Makefile` help, three `ai/rules/points/**` files, `ai/skills/ze-review.md`,
`TEST-RELAX-AUDIT.md` (the audit and its resolution). `make ze-doc-test` PASSED.

### Deviations from Plan

| Planned | Actual | Why |
|---------|--------|-----|
| "Derive the changed region on the Write path and search only it" | Key on the justification SENTENCE and its count, not on a changed region | A line-region diff was defeated by re-indenting a token one column; the sentence is the thing a reviewer judges |
| "Correct the statement in `.claude/hooks/README.md`" | Not done: no such statement exists there | `grep -c "replacement text" .claude/hooks/README.md` = 0. The sentence lives in the RFC-branch block message, where it is accurate for the RFC marker |
| Scope was D-2 and D-4 | Five defects plus a ratchet | The audit found three more of the same class in the same two functions; splitting them would have left the gate misfiring, which is what produced the corpus |

## Mistake Log

| # | Mistake | Cost | Root cause | Prevention |
|---|---------|------|------------|------------|
| 1 | Added an `.et` arm to the detector without checking `is_test` admits `.et` | Round-1 BLOCKER; the arm was dead and its documentation claimed coverage that did not exist | Edited the inner function and assumed the outer predicate | `relax-et-expectation-removal-blocked` |
| 2 | Keyed the hatch on raw LINES | Round-1 BLOCKER; one space defeated it | Chose the cheapest comparison, not the meaningful one | Three fixtures on whitespace and reword variants |
| 3 | Counted `\bassert\b` unanchored | Round-1 BLOCKER; two words of comment paid for a deleted `expect=` | Wrote the pattern against the happy case, not against an adversary | Two fixtures on comment and string-literal offsets |
| 4 | Made the census read the working tree | Round-2 BLOCKER; would have shipped a gate red on other sessions' work | Followed the sensitivity ratchet's convention without checking it against a shared checkout | `test_an_uncommitted_edit_to_a_tracked_test_does_not_red_the_gate` |
| 5 | Justified a ceiling raise on the PRESENCE of `raised-for:` | Round-2 BLOCKER; the ratchet would self-destruct on its first correct use | Tested for the marker, not for a fresh one | `test_an_old_reason_does_not_justify_a_later_raise` |
| 6 | Published three irreconcilable token counts | Round-1 ISSUE, and again in round 2 | Measured at different moments in a tree three other sessions were editing, and did not state a basis | Every number now carries its basis; the gate counts HEAD |
| 7 | Closed the reword hole with a COUNT arm | Round-3 ISSUE; it refused the honest drain, so keeping dead justifications became the only way through -- the corpus growth this spec exists to stop | Fixed the symptom (a reword has the same count) instead of the identity question (what makes two justifications the same) | `_reason_key`; `relax-token-drain-then-justify-allowed` |
| 8 | Stripped comments to end-of-line on the Go side | Round-3 ISSUE; `//` inside a Go string literal is not a comment, so a fixture whose value began `"//go:build ...` lost its `t.Fatal` from the OLD side alone and hid a real deletion | Assumed a strip is symmetric because it runs on both sides. It is not, when the two sides differ in what it eats | Whole-line comments only |
| 9 | Wrote a fixture that asserted only the exit code | Round-3 ISSUE; the shape trips two arms, so the fixture passed with the arm it names reverted | Asserted the verdict, not the reason for it | It now asserts the message |
| 10 | Compared `raised-for:` lines as a multiset | Round-3 ISSUE; duplicating one justified any raise | A copy-paste is a difference to a Counter and not to a reader | A set |

## Implementation Audit

### Requirements from Task

| Requirement | Implemented | Evidence |
|-------------|-------------|----------|
| The gate must not open per file on the Write path | Yes | `_writes_new_relax_reason`; `relax-token-does-not-persist-across-a-write` |
| The audit must not name reasons by position | Yes | `run_audit` multiset; `test_a_token_inserted_at_the_top_is_the_one_reported` |

### Acceptance Criteria

| AC | Implemented | Evidence |
|----|-------------|----------|
| AC-1 Write deletes assertions, pre-existing token unrelated -> blocks and names what was deleted | Yes | `relax-token-does-not-persist-across-a-write` PASS; message lists `removing assertions`, `adding t.Skip` |
| AC-2 Write deletes assertions and carries its own reason -> allowed, as Edit | Yes | `relax-token-written-in-this-edit-accepted` PASS |
| AC-3 reason added above a middle hunk -> the audit prints that reason | Yes | `test_a_token_inserted_at_the_top_is_the_one_reported` PASS |

### Tests from TDD Plan

| Planned test | Exists as | Result |
|--------------|-----------|--------|
| Write bypass is refused | `relax-token-does-not-persist-across-a-write` | PASS |
| Write with an in-change reason passes | `relax-token-written-in-this-edit-accepted` | PASS |
| reason attribution is not positional | `test_a_token_inserted_at_the_top_is_the_one_reported` | PASS |

### Files from Plan

| Planned | Touched | Note |
|---------|---------|------|
| `.claude/hooks/pretool-writeedit.py` | Yes | |
| `scripts/dev/audit-test-relaxation.py` | Yes | |
| `scripts/dev/audit_relaxation_test.py` | Yes | 3 new cases for AC-3 and its siblings |
| `.claude/hooks/README.md` | No | The statement the spec named does not exist there (measured) |

### Audit Summary

Every AC implemented and pinned by a named test. One planned file untouched, for
a stated and measured reason. Scope exceeded the plan by three further defects in
the same two functions, recorded above.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence |
|------------------|----------|
| "The gate opens per file, on the Write path" is no longer true | `relax-token-does-not-persist-across-a-write` blocks a gutting Write over a file carrying an unrelated token; `relax-token-written-in-this-edit-accepted` shows the honest case still passes |
| "The audit names reasons by position" is no longer true | Two tests, and the whole-repo before/after comparison (spec risk R-2) shows the same four findings on other sessions' work with reasons now printed whole instead of cut mid-sentence |
| "A guard that fails open and says nothing" is fixed | The Write path now blocks and names what was deleted; the census refuses rather than passing on a partial read |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/relax-token-gate-is-per-file-not-per-change-681f2376-7913-425e-9861-c9579ff7230c.md` |
| `review_gate.py check` | OK (11 code files, clean, hashes match) |
| Rounds | 6. Rounds 4-6 were earned by PRODUCT defects a later round found, each one introduced by the previous round's fix: R3 the drain refusal, R4 the trailing-comment payment and the duplicate-wording false positive, R5 the unbalanced-quote hole. R6 found record defects only, so it is the last round (`ai/rules/planning.md`) |
| Reviewer lenses used | guard-evasion, ratchet-integrity, correctness-and-claims (rounds 1-2, three lenses each), then one closure lens per round |

Totals: 6 BLOCKER and 33 ISSUE, all fixed. Final round: 0 BLOCKER, 0 product ISSUE.

### Findings fixed

Only the findings that changed behaviour are listed. The record defects (wrong
counts in comments, an overstated block message, two tests asserting an exit code
where the shape trips a second arm) were corrected in place and are recorded in
the Mistake Log.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `.et` never judged: `is_test` named `_test.go` and `.ci` only, so the guard was inert over all 164 editor tests | `c_test_weakening` | `.et` under `test/` admitted; `relax-et-expectation-removal-blocked` |
| 2 | BLOCKER | A whitespace variant of an existing token opened the hatch | `_has_relax_token` | `_reason_key`; three cosmetic-edit fixtures |
| 3 | BLOCKER | `\bassert\b` matched prose, so two words of comment paid for a deleted `expect=` | `_CI_COVERAGE` | comment-stripped, statement-anchored counting |
| 4 | BLOCKER | The census counted the working tree; in a shared checkout it would have shipped red | `census` | counts HEAD, prints the worktree delta |
| 5 | BLOCKER | A staged new test file made the census exit 2 for a session that touched nothing | `tracked_test_files` | population and content both from HEAD |
| 6 | BLOCKER | The first `raised-for:` line justified every later raise | `ceiling_raise` | the justification must be new since HEAD |
| 7 | ISSUE | The hatch was per-file under `Write`; 468 files exempt | `_has_relax_token` | `_writes_new_relax_reason` |
| 8 | ISSUE | The audit named an added reason by position, and went blind on delete-plus-add | `run_audit` | multiset difference |
| 9 | ISSUE | The reason was truncated to its first line; 62% run longer | `relax_reasons` | continuation walk |
| 10 | ISSUE | Rewording an existing token bought a severity downgrade | `run_audit` | reports what arrived AND what vanished |
| 11 | ISSUE | The count arm refused the honest drain, leaving dead justifications as the only way through | `_writes_new_relax_reason` | count arm removed; `relax-token-drain-then-justify-allowed` |
| 12 | ISSUE | End-of-line comment stripping hid a deletion (`//` inside a Go string literal) | `_test_weakening_errs` | quote-aware `_strip_comments` |
| 13 | ISSUE | Whole-line stripping let a TRAILING comment pay for deleted coverage | `_strip_comments` | same, plus two fixtures |
| 14 | ISSUE | One apostrophe disabled the comment cut for the rest of the line | `_strip_line` | two-pass: an unbalanced quote means the line is not quoted text |
| 15 | ISSUE | Subtracting the on-disk file refused a legitimately duplicated wording (10 files at HEAD do this) | `_writes_new_relax_reason` | reverted; the bound is documented instead |
| 16 | ISSUE | `_CI_EMPTY_NEEDLE` refused `contains=ExecStart=` and allowed emptying it | `_CI_EMPTY_NEEDLE` | the trailing `key=` must follow a `:` |
| 17 | ISSUE | A duplicated `raised-for:` line justified any raise | `_raised_for_lines` | a set, keyed by the hook's `_reason_key` |
| 18 | ISSUE | `--lower` could leave the ceiling above HEAD's | `main` | measured against HEAD |
| 19 | ISSUE | An unreadable tracked file was skipped silently | `census` | refuses, naming the paths |
| 20 | ISSUE | A zero count against a live ceiling read as a pass | `main` | refuses |
| 21 | ISSUE | `git ls-files` without `-z` dropped a quoted path, failing OPEN | `tracked_test_files` | `-z` on both listings |
| 22 | ISSUE | Dead code (`_RELAX_TOKEN*`) with a comment claiming callers | module scope | deleted |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 2026-08-09: `c_test_weakening` searches the whole new file on Write; `run_audit` attributes by tail slice | done | Both fixed in this spec. `_writes_new_relax_reason` and `run_audit`'s multiset difference; AC-1/AC-2/AC-3 evidence above |

The shard holds one row and it is terminal, so the shard is removed by this
closure.

## Pre-Commit Verification

### Files Exist (ls)

| Path | `ls` |
|------|------|
| `.claude/hooks/pretool-writeedit.py` | present |
| `scripts/dev/audit-test-relaxation.py` | present |
| `scripts/dev/audit_relaxation_test.py` | present |
| `scripts/dev/hook-fixture-check.py` | present |
| `scripts/dev/relax-census.py` | present (new) |
| `scripts/dev/relax_census_test.py` | present (new) |
| `test/relax-ceiling.txt` | present (new) |

### AC Verified (grep/test)

| AC | Command | Result |
|----|---------|--------|
| AC-1 | `python3 scripts/dev/hook-fixture-check.py` | `relax-token-does-not-persist-across-a-write` PASS, in 397/397 |
| AC-2 | `python3 scripts/dev/hook-fixture-check.py` | `relax-token-written-in-this-edit-accepted` PASS |
| AC-3 | `python3 scripts/dev/audit_relaxation_test.py` | `test_a_token_inserted_at_the_top_is_the_one_reported` PASS, in 19 tests OK |
| AC-3 sibling | `python3 scripts/dev/audit_relaxation_test.py` | `test_deleting_one_token_and_adding_another_is_not_silent` PASS |

### Wiring Verified (end-to-end)

| Entry point | Command | Result |
|-------------|---------|--------|
| `make ze-relax-census` | `make ze-relax-census` | `SELFTEST PASS`; `751 token(s) in 466 file(s) at HEAD, ceiling 751`; exit 0 |
| `ze-verify` both modes | `grep -n ze-relax-census scripts/status/verify_run.go` | two hits, `ze-verify-changed` and `modeFullVerify` |
| The census's own suite | `python3 scripts/dev/relax_census_test.py` | 27 tests OK |
| A real edit through the hook | `python3 scripts/dev/hook-parity-check.py` | OK |

### Assumptions Resolved

| ID | Status | Evidence |
|----|--------|----------|
| A-1 A Write on a test file carrying a token is reachable in normal work | confirmed | 468 tracked test files carried a token; the fixture drives the exact shape and it was allowed before the fix |
| A-2 Binding the token to the changed region is expressible on the Write path | **broken, superseded** | A changed-REGION derivation was defeated by re-indenting a token one column. Binding to the justification's reason key is what works. Recorded under Deviations and Mistake Log #2 |

### Documentation Verified

| Doc | Command | Result |
|-----|---------|--------|
| `docs/architecture/testing/test-health.md`, `ai/INDEX.md`, rule points | `make ze-doc-test` | Documentation tests PASSED |
| Rendered rules in step with their points | `make ze-rules-lint`, `rules_points.py render --check` | exit 0, no drift |
| No spec citation left dangling | `make ze-spec-citation-check` | exit 0 |

## Core Insight

A guard's false positives are not an annoyance to be traded against its true
positives. They are what destroys it. This one refused every mechanical
improvement to a `.ci` and admitted every in-place gutting, so its escape hatch
became routine, and 751 unread justifications accumulated behind it. The token
model was then judged safe because a human was expected to read them.

Two properties are what make an escape hatch survivable, and this spec's defect
was the first of them: it must bind to ONE change, and the stock must be small
enough that somebody reads it. Neither held.
