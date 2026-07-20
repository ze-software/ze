# Spec: rfc-gate-regression-ratchets

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | plan/spec-rfc-requirement-coverage.md (closed) |
| Phase | - |
| Updated | 2026-07-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/rfc-compliance.md`, `ai/rules/no-test-deletion.md`, `ai/rules/fail-closed-guards.md`
4. `scripts/dev/rfc_requirements.py`, `.claude/hooks/pretool-writeedit.py`

## Task

Close three regression holes in the RFC compliance machinery so that RFC testing stays
valid across future changes:

- **G1** the edit-time guard on RFC-tagged tests only sees the edit hunk, so editing the
  body of a tagged test without touching the tag line escapes it entirely;
- **G2** nothing ratchets a *requirement's* proof: a requirement proven by a positive and a
  negative test today can be demoted to `{gap}` tomorrow and the gate stays green;
- **G3** a newly added `rfc/short/*.md` summary is invisible to the gate by construction,
  so adding an RFC does not add RFC test checking.

Out of scope (explicitly, with user agreement): generating `rfc/audit/*.json` baselines for
the other 164 enrolled RFCs so the existing SHA ratchet arms for all of them (**G4**).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/rfc-compliance.md` - the rules the gate mechanises
  → Constraint: a green gate is bounded by what was extracted; mechanisms must not imply
    more assurance than they carry.
- [ ] `ai/rules/fail-closed-guards.md` - every new check here is a guard
  → Constraint: a miss/error/empty path must not return the permissive value. Where a git
    baseline cannot be read, the existing convention (`_git_baseline_enrolment`) degrades to
    "no baseline" rather than crashing; new baselines follow the same convention and say so.
- [ ] `ai/rules/no-test-deletion.md` - weakening tests to reach green is the failure mode
  → Decision: G1 widens an existing guard rather than adding a parallel one.

### RFC Summaries (MUST for protocol work)
- N/A. This spec changes verification machinery, not protocol behavior. No `rfc/short/`
  requirement is implemented, changed, or newly proven by it.

**Key insights:**
- Enrolment is already monotonic (`check_enrolment`); *proof* is not. G2 extends the same
  idea one level down, from the RFC to the requirement.
- The edit-time guard and the gate are complementary: the hook stops a bad edit, the gate
  stops a bad tree. G1 is a hook fix; G2 and G3 are gate fixes.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/dev/rfc_requirements.py` - the gate. `run_check` (`:1249`) composes
  `check_enrolment` (`:655`), `check_id_allocation` (`:427`), `evaluate` (`:572`),
  `check_status_agreement` (`:791`), `check_audit_freshness` (`:901`), `check_ledger_fresh`
  (`:1206`).
  → Constraint: `evaluate` judges the working tree only. The only HEAD comparisons are
    `_git_baseline_enrolment` (`:688`) and `_git_baseline_ids` (`:704`).
  → Constraint: `check_audit_freshness:934` does `if not verdict: continue`, so the SHA
    ratchet is armed only where `rfc/audit/<rfc>.json` records a verdict. One such file
    exists (`rfc/audit/rfc7606.json`) against 165 enrolled RFCs.
  → Constraint: `_collect_for_check` (`:1229`) discards parse errors for un-enrolled
    summaries, so an un-enrolled summary that does not parse contributes zero requirements
    and no error.
- [ ] `.claude/hooks/pretool-writeedit.py` - the edit-time guard. `_rfc_tagged_change_err`
  (`:1741`) returns `None` unless `_RFC_TAG` matches its `old` argument (`:1761`);
  `c_test_weakening` (`:1771`) passes the Edit hunk's `old_string` as that argument
  (`:1780`).
  → Constraint: comment-only and whitespace-only edits must keep passing
    (`_behavior_bytes`, `:1727`) or the hook gets disabled and protects nothing.
  → Constraint: `rfc-test-change-approved:` is the only approval token; `test-relax:` is
    deliberately not accepted (`:1758`).
- [ ] `scripts/dev/hook-fixture-check.py` - `run_rfc_test_guard` (`:811`) is the existing
  behavioural fixture suite for the guard, run by `make ze-hook-test`.
- [ ] `scripts/status/verify_run.go` - `stagesForMode` (`:112`) lists `ze-rfc-check` in both
  the `ze-verify-changed` (`:125`) and default (`:145`) branches.

**Behavior to preserve:**
- `make ze-rfc-check` exit codes: 0 clean, 2 violation, 2 "cannot run".
- Every existing fixture in `run_rfc_test_guard`: reformat passes, comment edit passes,
  approved edit passes, untagged file unaffected, `.ci` covered, `test-relax:` insufficient.
- Graceful degradation when git cannot answer (detached, first commit, no repo): no crash.
- The 9 currently un-enrolled summaries stay un-enrolled without error (they predate HEAD).

**Behavior to change:**
- G1: the guard's tag search widens from the edit hunk to the enclosing tagged region.
- G2: a new `check_coverage_ratchet` violation class.
- G3: a new `check_new_summaries` violation class.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- G1: a `Write`/`Edit`/`MultiEdit` tool call on a `_test.go` or `test/**/*.ci` file, via the
  `PreToolUse` hook dispatcher.
- G2/G3: `make ze-rfc-check` (directly, or as a stage of `make ze-verify` /
  `make ze-verify-changed`).

### Transformation Path
1. G1: `c_test_weakening` reads the hunk → `_enclosing_tagged_context` reads the file on
   disk and narrows to the enclosing test function → `_rfc_tagged_change_err` decides.
2. G2: `run_check` → `_git_baseline_tag_polarities()` (`git grep -l` at HEAD, then
   `git show` per tagged file, re-parsed with the *same* `scan_go_tags`/`scan_ci_tags`) →
   `check_coverage_ratchet(reqs, tags, enrolled, baseline)`.
3. G3: `run_check` → `_git_baseline_summary_stems()` (`git ls-tree` at HEAD) →
   `check_new_summaries(stems, baseline, enrolled, reqs, parse_errs)`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hook ↔ filesystem | `_enclosing_tagged_context` reads the target file; missing/unreadable falls back to the hunk | [ ] |
| Gate ↔ git | `subprocess` `git grep`/`git show`/`git ls-tree`, non-zero return degrades to empty baseline | [ ] |
| Gate ↔ tag parser | baseline tags are parsed by the SAME `scan_go_tags`/`scan_ci_tags`, never by an ad-hoc regex over `git grep` output | [ ] |

### Integration Points
- `run_check` (`scripts/dev/rfc_requirements.py:1249`) — two new `errs.extend(...)` calls.
- `c_test_weakening` (`.claude/hooks/pretool-writeedit.py:1771`) — one new call.

### Architectural Verification
- [ ] No bypassed layers (baseline tags go through the real tag scanners)
- [ ] No unintended coupling (the hook does not import the gate; the gate does not import the hook)
- [ ] No duplicated functionality (G1 widens the existing guard; no second guard is added)
- [ ] Zero-copy preserved where applicable — N/A (Python tooling)
- [ ] Registration over hardcoding — N/A (no new command/family/handler)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `git grep -l "RFC requirement:" HEAD -- internal pkg test` returns every file the working-tree scanner would find tags in | `scan_tree:540` walks exactly `TEST_ROOTS = ("internal", "pkg", "test")` | baseline under-counts → ratchet silently permissive | compare baseline tag count against `scan_tree()` on a clean tree | **confirmed** — with no tagged file differing from HEAD: 1336 baseline ids vs 1336 tree ids, zero only-in-baseline, zero only-in-tree, zero polarity differences |
| A-2 | `.ci` `terminator=` blocks can embed lines that look like tags | `scan_ci_tags:510` docstring cites `test/plugin/rfc7606-withdraw.ci:35` | a `git grep`-based baseline invents phantom tags → false violations | baseline re-parses file content with `scan_ci_tags`, so the hazard cannot arise | **confirmed** — design avoids it; the exact 1336/1336 agreement in A-1 includes the `.ci` tags, which a regex baseline would have inflated |
| A-3 | Every Go test tag sits inside or directly above a top-level `func` | `scan_go_tags:492` allows a tag on any line | `_enclosing_tagged_scope` misses a tag → G1 no better than today | enumerate every real tag against the union of scopes the widener can produce | **confirmed** — 2515 Go tags across 351 files, **zero** outside an enclosing-func scope, so the narrow scope loses no coverage versus whole-file |
| A-4 | The 9 un-enrolled summaries all exist at HEAD | `git ls-tree HEAD rfc/short` vs working tree | G3 fires on pre-existing work, blocking unrelated commits | run the gate on the clean tree after implementing | **confirmed** — 174 baseline stems vs 174 tree stems, "new since HEAD: none"; gate output unchanged at 39 pre-existing violations |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | G2 fires on a legitimate refactor that moves tags between files | gate red on a rename-only diff | ratchet compares polarity SETS per requirement id, not file:line, so a move is invisible to it |
| R-2 | Per-file `git show` for the baseline makes the gate noticeably slower | `make ze-rfc-check` wall time grows | **MATERIALISED then fixed.** One `git show` per tagged file was ~350 forks and measured 3.36s against 1.73s for the HEAD gate. Replaced with a single `git cat-file --batch` (`_git_cat_blobs`): 2.19s, +0.46s (+27%) over HEAD. A gate that adds seconds to every verify is one people learn to skip |
| R-3 | G1 over-blocks, and someone disables the hook | complaints about editing untagged tests in a tagged file | scope is the enclosing function, not the file, so untagged siblings stay free |
| R-4 | The gate is already red at HEAD (39 stale `rfc7606` verdicts), so a new red is hard to attribute | new violations mixed into existing output | new violation classes carry distinct wording; the pre-existing red is reported to the user separately and not fixed by this spec |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Edit` on a tagged test's body, tag outside the hunk | → | `_enclosing_tagged_context` + `_rfc_tagged_change_err` | `rfc-guard-blocks-body-edit-tag-outside-hunk` (`hook-fixture-check.py`) |
| `make ze-rfc-check` on a tree that dropped a negative tag | → | `check_coverage_ratchet` via `run_check` | `TestCoverageRatchetWiredIntoRunCheck` |
| `make ze-rfc-check` on a tree with a new un-enrolled summary | → | `check_new_summaries` via `run_check` | `TestNewSummaryEnrolmentWiredIntoRunCheck` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `Edit` changes assertions inside a `func Test…` that carries an `RFC requirement:` tag, and the hunk does not contain the tag line | hook exits 2, naming the requirement ids in scope |
| AC-2 | Same edit, but the enclosing function carries no tag while another function in the file does | hook does not block on RFC grounds |
| AC-3 | Comment-only or whitespace-only edit inside a tagged function | hook does not block (existing behavior preserved) |
| AC-4 | Edit inside a tagged function carrying `rfc-test-change-approved: <reason>` in the new text | hook does not block |
| AC-5 | A requirement that had positive+negative tags at HEAD has only positive in the tree | gate exits 2, naming the requirement and the lost polarity |
| AC-6 | Same, but the requirement is now annotated `{gap: …}` with a Partial `rfc-status.md` row | gate still exits 2: an annotation is not a substitute for proof that existed |
| AC-7 | Tags for a requirement move to a different file, polarity set unchanged | gate stays clean |
| AC-8 | A requirement id absent from HEAD (newly added) has fewer polarities than nothing | gate does not fire the ratchet (no baseline to lose) |
| AC-9 | A summary present in the tree but not at HEAD declares ≥1 gated MUST and is not in `rfc/enrolled.txt` | gate exits 2 naming the summary and the count |
| AC-10 | Same new summary, enrolled | gate applies the normal coverage rules, no new-summary violation |
| AC-11 | A new summary declares zero requirements while `rfc/full/<stem>.txt` contains MUST-level keywords | gate exits 2: capture failure, not a non-normative RFC |
| AC-12 | A new summary that fails to parse and is un-enrolled | gate exits 2 (parse errors are grandfathered only for summaries that predate HEAD) |
| AC-13 | The 9 summaries already un-enrolled at HEAD | gate does not fire AC-9/AC-11/AC-12 for them |
| AC-14 | `git` unavailable or HEAD unreadable | gate degrades to "no baseline", reports no spurious violation, does not crash |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | edits an RFC-tagged test's assertions to match broken code | Edit tool → PreToolUse dispatcher → `c_test_weakening` → `_enclosing_tagged_context` → block | `rfc-guard-blocks-body-edit-tag-outside-hunk` |
| 2 | deletes a negative test and annotates the requirement `{gap}` instead | working tree → `make ze-rfc-check` → `check_coverage_ratchet` → exit 2 | `TestCoverageRatchetGapIsNotAnEscape` |
| 3 | adds `rfc/short/rfcNNNN.md` full of MUSTs and commits without enrolling it | working tree → `make ze-rfc-check` → `check_new_summaries` → exit 2 | `TestNewSummaryWithGatedMustsMustEnrol` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCoverageRatchetLostPolarity` | `scripts/dev/rfc_requirements_test.py` | AC-5 | |
| `TestCoverageRatchetGapIsNotAnEscape` | same | AC-6 | |
| `TestCoverageRatchetTagMoveIsClean` | same | AC-7 | |
| `TestCoverageRatchetNoBaselineNoViolation` | same | AC-8, AC-14 | |
| `TestCoverageRatchetSkipsRFCNotEnrolledAtBaseline` | same | scope limit | |
| `TestNewSummaryWithGatedMustsMustEnrol` | same | AC-9 | |
| `TestNewSummaryEnrolledIsClean` | same | AC-10 | |
| `TestNewSummaryZeroCaptureAgainstSource` | same | AC-11 | |
| `TestNewSummaryParseErrorIsReported` | same | AC-12 | |
| `TestPreexistingSummaryGrandfathered` | same | AC-13 | |
| `TestCoverageRatchetWiredIntoRunCheck` | same | wiring | |
| `TestNewSummaryEnrolmentWiredIntoRunCheck` | same | wiring | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| gated requirement count on a new summary | 0..N | 0 (clean when source has no MUSTs) | N/A | 1 (must enrol) |
| polarities lost | 0..2 | 0 (clean) | N/A | 1 (violation) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc-guard-blocks-body-edit-tag-outside-hunk` | `scripts/dev/hook-fixture-check.py` | AC-1 | |
| `rfc-guard-untagged-func-in-tagged-file-passes` | same | AC-2 | |
| `rfc-guard-body-edit-comment-only-passes` | same | AC-3 | |
| `rfc-guard-body-edit-approved-passes` | same | AC-4 | |

`make ze-hook-test` runs this file; `make ze-rfc-check --selftest` runs the unit tests. Both
are already verify stages, so no new gate wiring is required.

### Interop Tests (MANDATORY for protocol features)
N/A — this spec changes verification tooling, not wire behavior.

### Future (if deferring any tests)
- None.

## Files to Modify
- `scripts/dev/rfc_requirements.py` - `_git_baseline_tag_polarities`, `_git_baseline_summary_stems`, `check_coverage_ratchet`, `check_new_summaries`, `_collect_for_check` (return per-stem parse errors), `run_check`
- `scripts/dev/rfc_requirements_test.py` - unit tests above
- `.claude/hooks/pretool-writeedit.py` - `_enclosing_tagged_context`, `_rfc_tagged_change_err` (tag scope parameter), `c_test_weakening`
- `scripts/dev/hook-fixture-check.py` - `run_rfc_test_guard` new cases
- `ai/rules/rfc-compliance.md` - document the three new guarantees
- `ai/rules/hook-mapping.md` - the widened guard scope
- `ai/INDEX.md` - discoverability per `ai/rules/discovery-updates.md`
- `.claude/skills/ze-rfc/SKILL.md` - what enrolling a new summary now requires

### BGP Family Checklist (if new SAFI / capability / attribute)
N/A — no BGP protocol extension.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | no config surface added |
| CLI commands/flags | No | no new command |
| Env var registration | No | no new setting |
| Doctor check for runtime dependencies | No | no new runtime dependency; `git` was already required by `_git_baseline_enrolment:688` |
| Prometheus counters/metrics | No | build-time tooling |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | developer tooling only |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no `rfc/short/` requirement changes state; `docs/features/rfc-status.md` untouched |
| 10 | Test infrastructure changed? | Yes | `ai/rules/rfc-compliance.md`, `ai/rules/hook-mapping.md`, `ai/INDEX.md`, `.claude/skills/ze-rfc/SKILL.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin/event/send/command/capability changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` and `ai/` for `rfc_requirements.py` and `pretool-writeedit.py` anchors |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `.claude/skills/ze-rfc/SKILL.md` and `.claude/skills/ze-rfc-audit/SKILL.md` describe the gate's checks |

## Files to Create
- None. Every change extends an existing file.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-rfc-check && make ze-hook-test && make ze-lint-changed` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | - |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary; two-commit closure |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — add the three call sites with stub checks that
   always return `[]`/`None`, plus the three wiring tests, which must fail.
   - Tests: `TestCoverageRatchetWiredIntoRunCheck`, `TestNewSummaryEnrolmentWiredIntoRunCheck`, `rfc-guard-blocks-body-edit-tag-outside-hunk`
   - Files: `scripts/dev/rfc_requirements.py`, `.claude/hooks/pretool-writeedit.py`
   - Verify: each wiring test fails because the stub returns nothing
2. **Phase: G1 enclosing-scope guard** — implement `_enclosing_tagged_context`, add the tag
   scope parameter to `_rfc_tagged_change_err`.
   - Tests: the four `rfc-guard-*` fixtures; all six pre-existing fixtures still pass
   - Verify: fail → implement → pass
3. **Phase: G2 coverage ratchet** — implement `_git_baseline_tag_polarities` and
   `check_coverage_ratchet`.
   - Tests: the five `TestCoverageRatchet*` unit tests
   - Verify: fail → implement → pass; measure `make ze-rfc-check` wall time for R-2
4. **Phase: G3 new-summary enrolment** — implement `_git_baseline_summary_stems`,
   `check_new_summaries`, and per-stem parse errors in `_collect_for_check`.
   - Tests: the five `TestNewSummary*`/`TestPreexisting*` unit tests
   - Verify: fail → implement → pass; confirm the clean tree stays clean for the 9
     pre-existing un-enrolled summaries (AC-13)
5. **Functional tests** → the hook fixtures in phase 2 are the functional layer.
6. **RFC refs** → N/A, no protocol code.
7. **Full verification** → `make ze-rfc-check`, `make ze-hook-test`, `make ze-lint-changed`.
8. **Complete spec** → learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Each of G1/G2/G3 is reachable from its real entry point, not only from a unit test |
| Correctness | The ratchet compares polarity SETS per id; a moved tag is not a loss |
| Guard audit | Each new check fails CLOSED where it can, and where it degrades (missing git baseline) the degradation is documented and matches the existing convention |
| Guard entry point | Each guard is driven from `run_check` / the hook dispatcher in a test, not only from its helper |
| Data flow | Baseline tags are parsed by the production scanners, never by an ad-hoc regex |
| Rule: no-layering | `_rfc_tagged_change_err` is extended, not duplicated |
| Rule: fail-closed-guards | No new check returns "clean" because it could not run |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| G1 blocks a body edit with the tag outside the hunk | `python3 scripts/dev/hook-fixture-check.py` shows the new case passing |
| G2 rejects a lost polarity | `python3 scripts/dev/rfc_requirements.py --selftest` |
| G3 rejects an un-enrolled new summary | same |
| Clean tree unaffected by G2/G3 | `python3 scripts/dev/rfc_requirements.py --check` shows only the pre-existing 39 `rfc7606` verdict violations |
| All pre-existing hook fixtures still pass | `make ze-hook-test` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Baseline content comes from `git show`; it is parsed by the same scanners as tree content and a `ParseError` there must not crash the gate |
| Resource exhaustion | Baseline reads are bounded by the number of files `git grep -l` matched, not by repository size |
| Error leakage | Git failures degrade to "no baseline" with no stderr dumped into gate output |
| Guard bypass | No self-service escape is added: `test-relax:` still does not satisfy the RFC guard, and `{gap}` does not satisfy the coverage ratchet |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | N/A (Python) |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Gate red on the clean tree from a NEW violation class | Back to DESIGN: the ratchet scope is wrong |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The HEAD gate ran in 0.071s, so the new baseline made it 47x slower | That 0.071s run was a copy of the gate placed in `tmp/`, where `PROJECT_DIR` resolves one directory too high; it exited immediately with "cannot run" and timed nothing. The real HEAD figure is 1.73s | Read the captured output instead of trusting the exit code: it said `No such file or directory: .../ze/docs/features/rfc-status.md` | None on shipped code. The panic-fix (batch blob reads) was worth keeping on its own merits, but the measurement that justified it was invalid until redone. **A timing comparison of a tool that resolves paths from its own location is invalid unless the copy sits where the original did** |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A check correct in isolation but deletable from its caller with every test green | 2nd time in this spec alone (`check_retired_requirements`, and the original blocker's composed-gate hole) | Every new check in a composed gate needs a test that drives the ENTRY POINT, not just the helper | Already a rule: `ai/rules/fail-closed-guards.md` "drive the guard's test from its entry point, never the helper alone". Not escalated -- the rule existed and I did not apply it. Recorded as lesson 6 in `plan/learned/1223-*.md` |
| A test passing for the wrong reason because two producers emit the same token | 1 | Assert the message TEXT, not just an id, when more than one check can name that id | Recorded as lesson 6; too narrow for a standalone rule |
| Copying a degradation convention without checking how the value is consumed | 1 | - | Recorded as lesson 1; a rule would be premature at n=1 |

## Design Insights

- Enrolment was already ratcheted; proof was not. The asymmetry is the bug: the gate
  protected the *claim* that an RFC is gated while leaving the *evidence* free to evaporate.

## Core Insight

A coverage gate that only reads the working tree can never tell "never proven" from
"stopped being proven". Those two states need different answers, and only a baseline can
distinguish them.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Ratchet the polarity SET per requirement id | Ratchet file:line pairs; ratchet a count | A set is invariant under refactors that move or rename tests, so it fires only on real loss. A count would fire on a merge of two tagged cases |
| Widen G1 to the enclosing function | Whole-file scope | A test file can hold dozens of functions with one tagged; whole-file scope would block unrelated work and get the hook disabled |
| No escape hatch on the coverage ratchet | A `{coverage-dropped: reason}` annotation | The annotation is exactly the move being blocked. The honest exits are: restore the test, or retire the requirement id |
| Skip G4 (audit baselines for 164 RFCs) | Generate them all | The weakening case is already covered by `c_test_weakening` and `audit-test-relaxation.py` at better altitude; G1+G2 cover deletion and downgrade. Mass-generated baselines would assert an audit that never happened |
| Baseline parsed by the production scanners | Regex over `git grep` output | `.ci` `terminator=` blocks embed raw content that looks like tags (`scan_ci_tags:510`); a regex baseline would invent phantom tags |

## Known Limitations

- A tagged test whose assertions are weakened *in place* is caught by `c_test_weakening`
  and `audit-test-relaxation.py`, not by the gate, unless the RFC has an audit file. That
  remains true after this spec (G4 out of scope).
- The ratchet's baseline is HEAD. Work that degrades coverage and is committed anyway (for
  example with a red gate) becomes the new baseline. The ratchet slows decay; it does not
  reverse it.
- `make ze-rfc-check` is currently red at HEAD for 39 stale `rfc7606` audit verdicts. This
  spec neither fixes nor worsens that; it is reported separately.

## RFC Documentation

N/A — no protocol code is added or changed by this spec.

## Implementation Summary

### What Was Implemented
- **G1** `_enclosing_tagged_scope` + `_go_func_scopes` + `_doc_comment_start`
  (`.claude/hooks/pretool-writeedit.py`): the RFC-tagged-test guard's tag search widens
  from the edited hunk to the enclosing test function, and separately blocks tag REMOVAL.
- **G2** `check_coverage_ratchet` + `_git_baseline_tag_polarities` + `_git_cat_blobs` +
  `_scan_tags_tolerant` (`scripts/dev/rfc_requirements.py`): a requirement cannot lose a
  polarity it had at HEAD.
- **G2b** `check_retired_requirements` (added during review): a requirement id of an
  enrolled RFC cannot vanish from its summary. Not in the original design; without it the
  ratchet's own pressure pointed at deleting the line.
- **G3** `check_new_summaries` + `_git_baseline_summary_stems`: a summary new since HEAD
  with gated MUSTs must be enrolled, must parse, and must not capture zero requirements
  while `rfc/full/` shows MUST-level keywords.
- 26 hook fixtures (19 new) and 140 gate unit tests (43 new).

### Bugs Found/Fixed
- All 19 review findings, listed in the Review Gate section. The three most serious were
  authored by me and caught only by independent review: a check that failed LOUD on a
  degraded baseline, a guard scope that falsely blocked 331 untagged functions, and a
  wiring test that passed for the wrong reason.
- No production (`internal/`, `cmd/`) code was touched: this spec changes tooling only.

### Documentation Updates
- `ai/rules/rfc-compliance.md`: new "What Keeps RFC Testing Valid (the four ratchets)"
  section, including what the ratchets do NOT catch.
- `ai/rules/hook-mapping.md`: the widened guard scope, tag-deletion blocking, `replace_all`.
- `ai/INDEX.md`: `make ze-rfc-check` row now names the HEAD ratchets.
- `ai/skills/ze-rfc.md`: step 6 rewritten (a new summary with gated MUSTs must be enrolled
  in the same change), plus two new rules. Synced via `make ze-ai-sync`.
- Checked but NOT changed: `docs/functional-tests.md:500` and
  `docs/contributing/rfc-implementation-guide.md:531` both anchor on
  `scan_go_tags`/`scan_ci_tags`/`evaluate`, whose behavior is unchanged, and both already
  say the hook blocks edits to tagged tests -- a claim that this spec makes MORE accurate
  rather than stale.

### Deviations from Plan
- `check_retired_requirements` was not in the design. It exists because the coverage
  ratchet created an incentive the design did not anticipate.
- The G1 span END changed twice: first to the next func's doc comment (fixing the false
  blocks), then to the func's own closing brace (making the "unowned tag widens" claim
  true rather than aspirational).
- `_git_cat_blobs` was not designed; a per-file `git show` was. The batch read replaced it
  after measurement.
- The union-over-occurrences was narrowed to `replace_all` only, after measurement showed
  it mis-diagnosed ~24% of ambiguous hunks.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| G1 edit-time guard sees tags outside the hunk | done | `pretool-writeedit.py` `_enclosing_tagged_scope` | 0/3072 false blocks, 2515/2515 tags protect their own function |
| G2 proof is monotonic | done | `rfc_requirements.py` `check_coverage_ratchet` | |
| G2b requirements cannot be deleted instead | done | `check_retired_requirements` | added during review, not in the original design |
| G3 a new RFC brings its own checking | done | `check_new_summaries` | |
| G4 audit baselines for the other 164 RFCs | not done, by agreement | - | out of scope in the Task section; the weakening case it covers is handled by `c_test_weakening` and `audit-test-relaxation.py` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `rfc-guard-blocks-body-edit-tag-outside-hunk` | the hunk is the assertion line alone; the tag is two lines above it |
| AC-2 | done | `rfc-guard-untagged-func-in-tagged-file-passes` | proves the scope is the func, not the file |
| AC-3 | done | `rfc-guard-body-edit-reformat-passes` | plus the pre-existing `rfc-guard-allows-reformat` |
| AC-4 | done | `rfc-guard-body-edit-approved-passes` | approval token still honoured under the wider scope |
| AC-5 | done | `TestCoverageRatchet.test_lost_polarity_fails` + `TestCoverageRatchetWiring.test_run_check_fails_on_lost_coverage` | helper and entry point both driven |
| AC-6 | done | `test_gap_annotation_is_not_an_escape` | |
| AC-7 | done | `test_moved_tags_are_clean` | the discriminating case: proves it is not "always fails" |
| AC-8 | done | `test_no_baseline_no_violation` | |
| AC-9 | done | `test_new_summary_with_gated_musts_must_enrol` + `TestNewSummaryEnrolmentWiring.test_run_check_fails_on_new_unenrolled_summary` | |
| AC-10 | done | `test_new_summary_enrolled_is_clean` | |
| AC-11 | done | `test_new_summary_capturing_nothing_fails_against_source` + `test_new_non_normative_summary_is_clean` + `test_new_summary_unknown_source_is_clean` | all three arms of the source comparison |
| AC-12 | done | `test_new_summary_parse_error_is_reported` | |
| AC-13 | done | `test_preexisting_unenrolled_summary_grandfathered`, and empirically: the gate on the real tree reports 39 violations, all pre-existing `rfc7606` stale verdicts, none from G2/G3 |
| AC-14 | done | `TestBaselineReaders` (4 cases: git failure for each reader, and exit-1 "no match" treated as a real answer) | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:** 14 ACs, 12 planned tests (delivered as 43 gate tests + 19 new fixtures), 8 files
- **Done:** all 14 ACs, all tests, all 8 files
- **Partial:** none
- **Skipped:** G4 only, named as out of scope in the Task section with the user's agreement
- **Changed:** 4 deviations, all documented in Deviations from Plan

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| G1 the edit-time guard survives a hunk that excludes the tag | hook fixture + whole-repo sweep | `rfc-guard-blocks-body-edit-tag-outside-hunk`; sweep of all **2515** gate-credited Go tags across 351 files driving a real body edit in each tag's own function: **0 unprotected**. Reviewer's independent sweep: 1900/1900 tagged funcs blocked |
| G1 without over-blocking | whole-repo sweep | **0 of 3072** untagged functions falsely blocked (was 331 of 3220 before the boundary fix; reviewer measured 0/4221 vs 446/4221 with their own walker) |
| G2 proof cannot silently regress | unit + wiring + mutation | `TestCoverageRatchet` (7 cases), `TestCoverageRatchetWiring` drives `run_check`; baseline/tree agreement on an unchanged tree: **1336 ids, 0 differences either way, 0 polarity mismatches** |
| G2b a requirement cannot be deleted instead | unit + wiring + mutation | `TestRetiredRequirements` (8 cases), `TestRetiredRequirementsWiring`; mutant M3 (delete the call) killed |
| G3 adding an RFC adds RFC test checking | unit + wiring | `TestNewSummaryEnrolment` (7 cases), `TestNewSummaryEnrolmentWiring`, `TestDegradedBaselineIsQuiet` |
| No new noise on the real tree | gate run | `make ze-rfc-check` reports exactly the **39 pre-existing** `rfc7606` stale-verdict violations, byte-identical classification to HEAD. Zero from any new check |
| The tests can actually fail | mutation testing | **14/15** gate mutants and **8/8** hook mutants killed; the one survivor is documented as not observable |
| Cost | measurement | 1.73s at HEAD, 2.22s now (+0.5s, ~30%). Per-file `git show` would have been 3.36s |

## Review Gate

Two independent adversarial subagents, one per area, each run twice (review, then
verification of the fixes), plus an author-run mutation pass. Artifact:
`tmp/review/rfc-gate-regression-ratchets-<sid>.md`.

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `check_new_summaries` judged EVERY summary as new when git failed: `stems - baseline_stems` against an empty baseline is `stems`. Violated AC-14 and its own docstring | `_git_baseline_summary_stems` | fixed: reader returns `Optional`, check no-ops on a falsy baseline |
| 2 | BLOCKER | Guard scope ran to the next `func` KEYWORD, swallowing the next function's doc comment: 331 of 3220 untagged functions falsely blocked | `_go_func_scopes` | fixed: span stops at the next func's doc comment |
| 3 | ISSUE | Deleting the checklist line was the cheapest red-to-green route; the ratchet iterates CURRENT requirements so a deleted one is never visited | `check_coverage_ratchet` | fixed: new `check_retired_requirements` |
| 4 | ISSUE | `replace_all` bypass: only the first occurrence of a hunk was inspected, so a tagged copy could be gutted unseen | `_enclosing_tagged_scope` | fixed: union over all occurrences when `replace_all` |
| 5 | ISSUE | Deleting the TAG passed as a comment-only edit, after which `test-relax:` alone bought any later weakening | `_rfc_tagged_change_err` | fixed: tag removal checked first |
| 6 | ISSUE | `_git_cat_blobs` consumed a non-blob body as the next header, silently dropping every following path | `_git_cat_blobs` | fixed: frame every type, `{}` on desync |
| 7 | ISSUE | A `ParseError` in one HEAD file discarded that file's whole baseline, exactly on a fix-the-tag commit | `_git_baseline_tag_polarities` | fixed: `_scan_tags_tolerant` |
| 8 | ISSUE | 4 of 6 new hook fixtures passed without the change; 3 of 5 gate baseline tests passed on an unguarded implementation | fixtures, `_FakeSubprocess` | fixed: fixture order, RFC-specific assertions, tag-bearing failure output |
| 9 | ISSUE | Baseline lacked `scan_tree`'s `testdata`/`vendor` pruning, so a tag there would report a false "no longer proven" | `_git_baseline_tag_polarities` | fixed |
| 10 | NOTE | Docstring cited "3.3s against 0.07s"; the 0.07s came from an invalid measurement | `_git_cat_blobs` | fixed: real figures (1.7s / 3.4s / 2.2s) |

### Fixes applied
- All ten above, plus two the author found between rounds: an unparseable summary made all
  39 of its ids look retired, and a deleted summary FILE did the same.

### Run 2 (verification of the fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `check_retired_requirements` had no wiring test: deleting its call from `run_check` left every test green. Same defect class as Run 1 #1 | `run_check` | fixed: `TestRetiredRequirementsWiring` |
| 2 | ISSUE | The docstring claimed a whole-file fallback for tags outside every span, but spans were contiguous so no such gap existed: a blank-line-separated tag, or a table hoisted BETWEEN funcs, was silently re-homed onto the preceding function | `_go_func_scopes` | fixed: spans end at the func's own closing brace, so the gap is real and the claim is true |
| 3 | ISSUE | Union widening blocked ~24% of ambiguous hunks in untagged funcs, with a wrong-cause message | `_enclosing_tagged_scope` | fixed: union only when `replace_all` |
| 4 | ISSUE | Path-hygiene and tolerant-scan fixes had no tests; the `.ci` fixture was vacuous | tests | fixed, mutation-verified |
| 5 | NOTE | `/* */` block-comment tags are matched by the hook but not the gate; 0 instances in tree | both | dissolved by #2: such a tag is now unowned and widens to the whole file |

### Run 3 (author mutation pass, after Run 2 fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | The Run-2 wiring test PASSED FOR THE WRONG REASON: tags left for the deleted id made `evaluate` emit "unknown RFC requirement: RFC7606-2-2" -- same id, same exit code as the check under test, which was never called | `TestRetiredRequirementsWiring` | fixed: tags follow requirements exactly, assertion names the message text |
| 2 | ISSUE | 5 gate mutants survived (retired-check wiring, grep returncode guard, `.ci` fallback guard, dedup, strict-scanner wiring) | tests | fixed: 14/15 killed |
| 3 | ISSUE | 2 hook mutants survived (next-func-keyword cap, unlocatable-hunk fallback) | fixtures | fixed: 8/8 killed |
| 4 | NOTE | M11 (`.strip()` on the `-z` path) is not observable through the function's output either way | `_git_baseline_tag_polarities` | accepted, documented in-code as correctness by construction rather than verified behavior |

### Final status
- [ ] Review re-run shows 0 BLOCKER, 0 ISSUE — every finding above fixed; final mutation run 14/15 gate, 8/8 hook
- [ ] NOTEs recorded: Run 1 #10, Run 2 #5, Run 3 #4

## Pre-Commit Verification

### Files Exist (ls)
No files were created; every change extends an existing file. The eight changed files are
hashed in the review artifact `tmp/review/rfc-gate-regression-ratchets-<sid>.md`, which
`commit_helper.py` re-checks at commit time (any later edit invalidates it).

| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/rfc_requirements.py` | yes | `--selftest` exit 0, `--check` exit 2 (39 pre-existing) |
| `scripts/dev/rfc_requirements_test.py` | yes | 140 tests, `OK` |
| `.claude/hooks/pretool-writeedit.py` | yes | 131/131 dispatcher parity |
| `scripts/dev/hook-fixture-check.py` | yes | 79/79 fixtures pass, 26 of them rfc-guard |
| 4 doc files | yes | `make ze-ai-sync` re-synced 28 skills |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4 | guard scope behaves at the hunk/function boundary | `rfc-guard-blocks-body-edit-tag-outside-hunk`, `-untagged-func-in-tagged-file-passes`, `-body-edit-reformat-passes`, `-body-edit-comment-only-passes`, `-body-edit-approved-passes` all PASS; mutants H1/H6/H8 killed by them |
| AC-5..AC-8 | coverage ratchet | `TestCoverageRatchet` 7/7; mutants M15 and "drop `baseline_enrolled` scope" killed |
| AC-9..AC-13 | new-summary enrolment, backlog grandfathered | `TestNewSummaryEnrolment` 7/7 plus the real-tree run: 174 baseline stems vs 174 tree stems, "new since HEAD: none", gate output unchanged |
| AC-14 | degraded git baseline stays quiet | `TestBaselineReaders` + `TestDegradedBaselineIsQuiet` drive both the reader and `run_check`; mutants M1/M8/M9 killed |

### Wiring Verified (end-to-end)
No `.ci` applies: this spec changes build-time tooling, which has no daemon path. The
equivalent proof is that each check is driven from its real entry point and that removing
the call fails a test.

| Entry Point | Test | Verified |
|-------------|------|----------|
| `run_check` → `check_coverage_ratchet` | `TestCoverageRatchetWiring` | yes |
| `run_check` → `check_new_summaries` | `TestNewSummaryEnrolmentWiring` | yes |
| `run_check` → `check_retired_requirements` | `TestRetiredRequirementsWiring` | yes, mutant M3 killed |
| `_git_baseline_tag_polarities` → `_scan_tags_tolerant` | `test_tolerant_scan_is_the_path_the_baseline_reader_uses` | yes, mutant M14 killed |
| PreToolUse dispatcher → `c_test_weakening` → `_enclosing_tagged_scope` | 26 rfc-guard fixtures | yes, mutant H8 killed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | 1336 baseline ids vs 1336 tree ids, zero differences either direction, zero polarity mismatches, on a tree with no tagged file differing from HEAD |
| A-2 | confirmed | the baseline re-parses with the production scanners, so the `terminator=` phantom-tag hazard cannot arise; `test_tolerant_scan_drops_a_malformed_ci_whole` pins the one place a fallback could reintroduce it (mutant M13 killed) |
| A-3 | confirmed, then superseded | 2515 of 2515 Go tags sit inside a scope the widener produces. The assumption was that a tag outside every scope needs a whole-file fallback; Run 2 showed the fallback could never fire because spans were contiguous. Fixed, and now `rfc-guard-tag-between-funcs-widens` / `-blank-line-tag-widens` prove it does |
| A-4 | confirmed | 174 = 174 stems, "new since HEAD: none"; the 9 pre-existing un-enrolled summaries produce no violation |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
