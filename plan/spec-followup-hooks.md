# Spec: followup-hooks

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-07-09 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/hooks/pretool-writeedit.py`, `.claude/hooks/pretool-bash.py`, `.claude/hooks/validate-spec.sh`
4. `ai/rules/hook-mapping.md` - hook registry (stale rows are part of this spec's scope)
5. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Enable or fix three dead/broken agent-guard hooks that currently run on a dead path or no-op.

This was a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Designed 2026-07-09; all evidence below re-verified against the tree at that date.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Enable BGP format-file append-idiom guard (L232)** - `c_format_alloc` returns `None` early (`pretool-writeedit.py:740`); the real check is unreachable dead code. Deliberate switch-on would start blocking `fmt.Sprintf`/`strings.Builder`/`strings.Join` in the ~~9 `bgp/format/*.go` files~~ guarded format files (design correction: only 7 of the 9 listed paths are under `bgp/format/`; one is `bgp/reactor/filter_format.go`; and `bgp/attribute/text.go` no longer exists - the `attribute` package was removed in commit `3e66070f8`).
- **Fix validate-spec.sh (L233)** - still carries a `set -e` crash + Unicode-arrow vs ASCII `->` mismatch, so it aborts before validating some specs. Fix the arrow match + set -e fragility, then decide block-vs-warn.
- **Fixture tests for commit-gated Bash checks (L234)** - spec-audit/deferral-in-diff/deferral-unassigned/wiring-at-commit/doc-drift only fired on a dead `git commit` path; spec-audit was never ported. Add fixture tests driving each check directly, and re-home the gates onto the sanctioned commit path so the fixtures test live behavior.

### Design-time corrections (2026-07-09, all re-verified with file:line)

| Triage claim | Reality today |
|--------------|---------------|
| Guard covers "9 bgp/format/*.go files" | `fmt_files` list (`pretool-writeedit.py:745-755`) has 9 entries: 7 under `bgp/format/`, plus `bgp/reactor/filter_format.go` (exists) and `bgp/attribute/text.go` (**deleted** in `3e66070f8`); `bgp/format/json.go` exists but is unguarded |
| Enabling the guard is a one-line change | The dead check lacks the comment-line exemption `c_sprintf_new` has (`pretool-writeedit.py:321,325`); `bgp/format/text.go` lines 8 and 91 are comments matching `strings.Builder` / `strings.ReplaceAll(` - verbatim enablement would block full-file Writes of current text.go |
| Commit-gated checks fire on `git commit` | Doubly dead: the four checks gate on the literal commit phrase (`pretool-bash.py:183,239,279,316`) which (a) never appears in the sanctioned commit path `bash tmp/commit-<SESSION>.sh`, and (b) when it does appear, `check_destructive_git` (`pretool-bash.py:69-92`) blocks the command outright |
| Hook testing exists | Only `scripts/dev/hook-parity-check.py` (golden exit-code regression, no make target, no CI wiring); its temp dirs are not git repos, so all commit-gated check bodies have zero coverage today |

## Required Reading

### Source files / docs

- [ ] `.claude/hooks/pretool-writeedit.py` (`c_format_alloc` :731-771)
  → Constraint: dead check body already complete - `fmt_files` list :745-755, banned patterns :758-767, block at :770; enablement = remove `return None` at :740, fix stale list, add comment filtering per `c_sprintf_new` (:312-350, comment filter :321,:325)
  → Constraint: content arrives via `std_content` (:55-60): Write = full file, Edit = `new_string` only; MultiEdit `edits[].new_string` is NOT aggregated for this check
  → Decision: `c_sprintf_new` already blocks `fmt.Sprintf/Fprintf/Printf` + `strconv.Format*` in ALL non-test .go; the incremental value of this guard is the `strings.Join/Builder/NewReplacer/ReplaceAll` bans on the format files
- [ ] `.claude/hooks/validate-spec.sh`
  → Constraint: `set -e` at :5; crash site is the `WIRING_ROWS=` assignment :247 - empty grep pipeline exits 1 and aborts the script before the output stage (:335-348), swallowing queued ERRORS
  → Constraint: Unicode-arrow greps at :241 and :247 (`|.*→.*|`); ~40 of 130 existing specs use ASCII `->` wiring rows; both conventions are institutionalized (`plan/TEMPLATE.md:106` uses `→`, `.claude/rules/post-compaction.md` prescribes ASCII `->`)
  → Decision: `ai/rules/hook-mapping.md:131-137` records the deliberate decision to keep this hook out of the dispatcher and prescribes exactly this fix
- [ ] `.claude/hooks/pretool-bash.py`
  → Constraint: CHECKS tuple :336-347; commit-gated checks `check_deferral_unassigned` :181, `check_deferral_in_diff` :237, `check_wiring_at_commit` :277, `check_doc_drift` :314; all gate on the literal commit phrase and never fire on the sanctioned script path
  → Constraint: exit semantics - 0 allow, 1 warn, 2 block, worst wins, fail-open on internal error (:365-396)
  → Constraint: `check_system_tmp` (:131-141) blocks Bash commands referencing `/tmp`, and `check_destructive_git` blocks any command containing the commit phrase - fixture-test code and commit scripts must avoid both
- [ ] `scripts/dev/commit_helper.py`
  → Constraint: creation-time gates already exist (verify freshness :464-482, structural reds :506-526) - the natural home for commit-time repo-state checks; contains zero deferral/spec-audit logic today
- [ ] `scripts/dev/hook-parity-check.py`
  → Constraint: golden-table harness (BASH_CMDS :37-55, WE_CASES :60-214, GOLDEN :448-586); temp project dirs are NOT git repos, so commit-gated checks return None before their bodies run; extending coverage means adding git-initialized fixtures
- [ ] `ai/rules/hook-mapping.md`
  → Constraint: stale on two counts - :57 lists a "spec-audit" pretool-bash check that does not exist; :63 and pretool-bash docstring :12-19 describe settings.json entries that are gone; this spec must update the doc (`ai/rules/discovery-updates.md`)
- [ ] `git show b71b4ad6d^:.claude/hooks/pre-commit-spec-audit.sh` (the never-ported original)
  → Constraint: 8 checks on the selected spec at commit time (wiring/functional .ci exist, Files-to-Create exist, TDD files exist, audit tables non-empty, AC evidence, audit summary, Pre-Commit Verification rows, learned summary)
  → Constraint: keyed off `tmp/session/selected-spec` which no longer exists - replaced by per-session markers (`scripts/dev/spec-session.sh`, commit `276d72c99`); a port must use the marker substrate or hook into commit_helper.py

**Key insights:**
- All three items re-verified still open on 2026-07-09; no commits touched `.claude/hooks/` since 2026-07-05.
- The format guard's real work is list hygiene + comment exemption, not just deleting a return (verified firsthand: `internal/component/bgp/format/text.go:8` is a comment matching `strings.Builder`).
- validate-spec.sh failure was reproduced empirically: ASCII-arrow spec → exit 1, zero output, trace stops at :247.
- The commit-gate rehoming decision (where checks fire) is the load-bearing design choice; fixtures alone would test dead code, violating `ai/rules/no-workarounds-for-missing-behavior.md`.

## Current Behavior (MANDATORY)

**Source files read (2026-07-09):**

- [ ] `.claude/hooks/pretool-writeedit.py` - `c_format_alloc` (:731) returns None at :740 with complete-but-dead check below; `c_sprintf_new` (:312) provides the comment-filter pattern to copy
- [ ] `.claude/hooks/validate-spec.sh` - PostToolUse Write|Edit on plan/spec-*.md, blocking:true; exit 2 on ERRORS (:341), accidental exit 1 via set -e at :247 on specs without Unicode-arrow wiring rows
- [ ] `.claude/hooks/pretool-bash.py` - four commit-gated checks never fire on the sanctioned commit path; `check_destructive_git` blocks the literal path they do match
- [ ] `scripts/dev/commit_helper.py` - creation-time gates run when the commit script is generated; no deferral/spec-audit checks
- [ ] `scripts/dev/hook-parity-check.py` - non-git temp fixtures; commit-gated bodies uncovered
- [ ] `internal/component/bgp/format/text.go` - :8 comment names the banned primitives (doc of the buffer-first rule); the code body is already compliant

**Behavior to preserve:**
- All existing hook checks and their exit codes (golden tables in hook-parity-check.py re-blessed only for deliberate changes).
- Fail-open on internal error in dispatchers (pretool-bash.py:389-396).
- validate-spec.sh continues to hard-block (exit 2) genuinely malformed specs.
- The sanctioned commit flow (`scripts/dev/commit_helper.py` create → `bash tmp/commit-<SID>.sh`) keeps working for compliant commits.

**Behavior to change:**
- `c_format_alloc` becomes live (blocking) with a corrected file list and comment exemption.
- validate-spec.sh stops crashing on ASCII-arrow specs; accepts both arrow conventions.
- The four commit-gated checks + a ported spec-audit gate fire on the real commit path.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Agent Write/Edit tool calls (stdin JSON) → `pretool-writeedit.py` dispatcher; PostToolUse → `validate-spec.sh`
- Agent Bash tool calls → `pretool-bash.py` dispatcher
- Commit-script generation → `scripts/dev/commit_helper.py` creation-time gates

### Transformation Path
1. Hook receives stdin JSON, builds ctx (`std_content`: Write = full content, Edit = new_string)
2. Each check inspects cmd/content/repo state, returns None or (code, message)
3. Dispatcher takes worst code; 2 rejects the operation, 1 warns
4. commit_helper gates run once at script-creation time and refuse to emit the script on failure

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| tool call → hook | stdin JSON contract | [ ] |
| hook → agent | exit code 0/1/2 semantics | [ ] |
| commit_helper → repo state | git plumbing + plan/ file parsing at script-creation time | [ ] |

### Integration Points
- `.claude/hooks/pretool-writeedit.py` - enable + fix `c_format_alloc`
- `.claude/hooks/validate-spec.sh` - arrow + set -e fix
- `.claude/hooks/pretool-bash.py` - commit-gate path fix (also match `bash tmp/commit-` prefix)
- `scripts/dev/commit_helper.py` - ported spec-audit gate + deferral gates at creation time
- `scripts/dev/hook-parity-check.py` - extended fixtures (git-initialized cases)
- `ai/rules/hook-mapping.md` - registry rows updated

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | re-verified 2026-07-09 (research pass, spot-checked :740, :5, :241, :247, :183/:239/:279/:316, :336-347) | Re-scope the item | grep/LSP at implement-audit time | confirmed |
| A-2 | The 8 existing guarded format files contain zero non-comment banned-primitive occurrences, so enabling the guard breaks no current workflow | grep of all listed files 2026-07-09: only comment hits in `bgp/format/text.go:8,91` (verified firsthand) | Add comment exemption AND fix offending code first | re-grep at implement time: 0 code-level hits; only comments at text.go:8,91 | confirmed |
| A-3 | Fixing arrow-matching + set -e makes validate-spec.sh runnable across all existing plan/spec-*.md without mass failures | ~40 ASCII-arrow specs fail only via the crash today | Survey results drive block-vs-warn; fix legacy specs or soften to warn | survey over 124 specs (below): 0 crashes; the arrow fix moved 6 specs 1->0 (valid, was swallowed), 0 arrow false-positives | confirmed |
| A-4 | commit_helper.py creation-time gates are the right home for repo-state commit checks (spec-audit, deferrals) | verify/structural gates already live there (:464-526) | Fall back to pretool-bash.py matching `bash tmp/commit-` prefix | gates added to `commit_gate_problems/_warnings`, called in `create()`; fixtures + real-path `create` block test pass | confirmed |
| A-5 | Re-blessing hook-parity-check.py goldens after enabling the guard does not mask unrelated regressions | harness design (per-case goldens) | Bless only the touched cases, diff the rest | parity vs HEAD hooks produced IDENTICAL results; c_format_alloc touched 0 goldens; re-blessed nothing; 131/131 after the /tmp->~/.cache fixture-dir fix | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Enabled format guard false-positives on comments/docs edits in guarded files | guard blocks an edit containing only comment text | Copy `c_sprintf_new` comment filtering; fixture case for comment-only content |
| R-2 | validate-spec.sh made fully blocking rejects edits to legacy specs missing newer sections | survey shows >5 legacy specs failing post-fix | Keep exit 2 for new/structural errors, downgrade legacy-section gaps to warnings, or fix the failing specs in the same change |
| R-3 | Commit gates at creation time diverge from commit time (files staged between create and run) | commit script runs on drifted staging | Gates re-check inside the generated script (helper already embeds checks in script body) or accept creation-time semantics and document |
| R-4 | spec-audit port re-blocks commits for umbrella/multi-session specs (historic false positives) | gate fires on a spec legitimately still open | Key off per-session claimed spec only; skip when no spec claimed; align with `spec-closure-check.py` confidence tiers |
| R-5 | Scope drift into rewriting the whole hook system | diffs touch checks unrelated to the three items | Confine changes to named functions + registry doc |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Write of guarded `bgp/format/text_json.go` containing `strings.Join(` in code | → | `c_format_alloc` returns 2 | `hook-fixture-check.py` case `format-alloc-live-join` |
| Write of guarded file with banned primitive only in a `//` comment | → | `c_format_alloc` returns None | `hook-fixture-check.py` case `format-alloc-comment-exempt` |
| PostToolUse Write of an ASCII-arrow spec | → | validate-spec.sh completes validation (no set -e abort) | `hook-fixture-check.py` case `validate-spec-ascii-arrows` |
| Commit-script generation with staged deferral-pattern diff and unstaged `plan/deferrals.md` | → | deferral-in-diff gate blocks script creation | `hook-fixture-check.py` cases `commit-gate-deferral-in-diff`, `commit-gate-create-blocks-deferral` |
| Commit-script creation with claimed spec whose Pre-Commit Verification is empty | → | ported spec-audit gate blocks | `hook-fixture-check.py` case `commit-gate-spec-audit-blocks` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Write/Edit of any currently-existing guarded format file (7 `bgp/format/*.go` + `bgp/reactor/filter_format.go`) whose new content has a banned primitive in code | Hook exits 2 with the guarded-file message; stale `bgp/attribute/text.go` entry removed from the list; decision recorded whether `bgp/format/json.go` joins the list (grep it and decide from its content) |
| AC-2 | Same edit where banned pattern appears only in comment lines | Hook allows (comment exemption equivalent to `c_sprintf_new` :321-325) |
| AC-3 | validate-spec.sh runs on a spec containing only ASCII `->` wiring rows | Script reaches its output stage and returns 0 or 2 on merit; never aborts via set -e; both `→` and `->` rows are recognized as wiring rows |
| AC-4 | Fixed validate-spec.sh executed over every `plan/spec-*.md` in the tree | Zero crashes; survey table pasted into this spec; block-vs-warn decision recorded and applied (default: keep exit 2 blocking; downgrade only if survey shows widespread legacy failures) |
| AC-5 | The sanctioned commit path (commit_helper.py create + generated script) with (a) open unassigned deferral, (b) deferral-pattern diff without deferrals.md staged, (c) plugin .go staged without .ci, (d) doc drift | Each of the four gates fires on the real path (block for a/b, warn for c/d, preserving today's severities at pretool-bash.py:213,:274,:311,:331) |
| AC-6 | Commit including a learned summary for the claimed spec while the spec's Pre-Commit Verification section is unfilled | Ported spec-audit gate blocks with an actionable message keyed off the per-session marker (`scripts/dev/spec-session.sh current`); no marker claimed → gate skips |
| AC-7 | `hook-parity-check.py` (or successor fixture runner) executed | All fixtures above run in one command; a make target (e.g. `ze-hook-test`) exists and is listed in `ai/rules/hook-mapping.md`; goldens re-blessed only for intended changes |
| AC-8 | `ai/rules/hook-mapping.md` after implementation | Stale rows corrected: validate-spec.sh no longer "Currently broken"; spec-audit row points at the real gate location; commit-gate trigger documented as the sanctioned script path |

## AC-4 Survey Results (validate-spec.sh over all specs)

Fixed `validate-spec.sh` run over every `plan/spec-*.md` (124 specs, 2026-07-09):

| Metric | Count |
|--------|-------|
| specs surveyed | 124 |
| exit 0 (valid / warnings only) | 90 |
| exit 1 (set -e crash) | 0 |
| exit 2 (structural errors, blocks) | 34 |

OLD (HEAD) vs NEW exit-code delta, isolating exactly what the fix changes:

| OLD -> NEW | Count | Meaning |
|-----------|-------|---------|
| 0 -> 0 | 84 | valid specs, unaffected |
| 1 -> 0 | 6 | were crashing; actually VALID (the crash was hiding them) |
| 1 -> 2 | 28 | were crashing; genuinely INVALID (crash swallowed real errors) |
| 2 -> 2 | 6 | already blocking, unaffected |

The OLD script crashed (exit 1) on 34 specs; the NEW script crashes on 0.

**Block-vs-warn decision: keep exit 2 blocking (no downgrade).** The arrow fix
introduces ZERO false positives: the 6 `1->0` specs were valid all along and the
crash was hiding them. Every one of the 28 newly-surfaced blocks is a genuine
structural error (missing required section, missing `Tests written/FAIL/PASS`
checklist item, unresolved `rfc/short/rfc*.md` summary, or Functional Tests
without a `.ci`) that the crash had been swallowing. Downgrading to warn-only
would gut the hook's structural-enforcement purpose (Key Design Decision 3). The
28 blocked specs are pre-existing structural debt (mostly `-0-umbrella` and
skeleton/design-stage specs), not artifacts of this change, and fixing them is
not part of this spec's remit. This spec (`spec-followup-hooks.md`) itself
validates at exit 0.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `format-alloc-live-join`, `format-alloc-live-builder`, `format-alloc-comment-exempt`, `format-alloc-unguarded-file`, `format-alloc-json-guarded`, `format-alloc-stale-attribute-path`, `format-alloc-reactor-filter`, `format-alloc-test-file-skip` | `scripts/dev/hook-fixture-check.py` (calls `c_format_alloc` directly) | AC-1, AC-2 | PASS (8/8) |
| `validate-spec-ascii-arrows`, `validate-spec-unicode-arrows`, `validate-spec-missing-section-blocks` | `scripts/dev/hook-fixture-check.py` (drives validate-spec.sh) | AC-3 | PASS (3/3) |
| `commit-gate-deferral-unassigned`(+`-assigned-ok`), `commit-gate-deferral-in-diff`(+`-logged-ok`), `commit-gate-wiring-warn`(+`-with-ci-ok`), `commit-gate-doc-drift-absent-skips`, `commit-gate-doc-drift-warns`, `commit-gate-create-blocks-deferral` | `scripts/dev/hook-fixture-check.py` (git fixtures) | AC-5 | PASS |
| `commit-gate-spec-audit-blocks`, `commit-gate-spec-audit-filled-ok`, `commit-gate-spec-audit-no-claim-skips`, `commit-gate-spec-audit-non-closure-skips` | `scripts/dev/hook-fixture-check.py` (git fixtures) | AC-6 | PASS |

> The format-alloc and commit-gate cases live in the new
> `scripts/dev/hook-fixture-check.py`, not the `hook-parity-check.py` WE corpus:
> the parity harness measures the WHOLE dispatcher's exit code in a non-git dir,
> where `c_pre_write_go`/`c_require_design_ref` return 2 for any `internal/*.go`
> and the commit gates have no git repo -- so it can never isolate these checks.
> See "Files to Create" (the spec anticipated this fallback).

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no numeric config inputs | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | internal hook tooling - validated by fixture tests, not `.ci` (no daemon path) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - no wire protocol | - | - | - | |

## Files to Modify

- `.claude/hooks/pretool-writeedit.py` - enable `c_format_alloc`: remove :740 return, drop dead `bgp/attribute/text.go` path, add comment exemption, decide json.go
- `.claude/hooks/validate-spec.sh` - guard :247 pipeline against empty result, accept `→` and `->` in :241/:247 patterns, audit remaining set -e crash sites
- `.claude/hooks/pretool-bash.py` - extend the commit-gate trigger to the sanctioned path (`bash tmp/commit-` prefix) OR delegate to commit_helper (per A-4 decision); keep severities
- `scripts/dev/commit_helper.py` - add deferral + ported spec-audit creation-time gates (A-4 primary design)
- `scripts/dev/hook-parity-check.py` - git-initialized fixture support + new corpus entries + re-blessed goldens
- `ai/rules/hook-mapping.md` - correct stale rows, document the fixture runner + make target
- `mk/` (appropriate include) - `ze-hook-test` target so the fixtures are discoverable/runnable

## Files to Create

- `scripts/dev/hook-fixture-check.py` - CREATED. The parity harness (whole-dispatcher
  exit code, non-git dir) cannot isolate `c_format_alloc` (dominated by
  `c_pre_write_go`/`c_require_design_ref` on any `internal/*.go`) or exercise the
  commit gates (need a git repo), so a dedicated runner was the right home. It
  imports `c_format_alloc` and the `commit_helper` gate functions and drives
  `validate-spec.sh` directly. Wired into `make ze-hook-test`.

## Implementation Steps

1. **Phase: wiring (validate-spec.sh fix first - it gates every subsequent spec edit)** - fix :247 crash + arrow patterns; fixture proving ASCII-arrow spec validates; survey all specs (AC-3, AC-4).
2. **Phase: format guard (TDD)** - fixtures fail → enable + fix `c_format_alloc` → fixtures pass; re-bless intended goldens (AC-1, AC-2).
3. **Phase: commit gates (TDD)** - git-initialized fixtures fail → rehome gates per A-4 (+ port spec-audit onto per-session markers) → fixtures pass (AC-5, AC-6).
4. **Phase: discovery** - make target + hook-mapping.md updates (AC-7, AC-8) per `ai/rules/discovery-updates.md`.
5. **Full verification** - `make ze-verify` + fixture runner.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-followup-hooks.md`, two-commit closure.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | The live `deferral_in_diff` gate detects the very phrases it is defined with: its `DEFERRAL_PATTERNS` list, the docstrings that explain it, and the fixtures that test it all contain the trigger phrases, so committing the gate (or editing its rule doc) self-trips. | `scripts/dev/commit_helper.py` `deferral_in_diff_problems` | fixed: added `_deferral_prose` (blank quoted/backticked spans before matching) + fixtures `commit-gate-deferral-in-diff-code-literal-exempt` / `-prose-still-caught`; reworded spec/summary prose to avoid bare trigger phrases |
| 2 | NOTE | `doc_drift_warnings` runs `go run scripts/docvalid/doc_drift.go` on every commit-script creation (~2s). | `scripts/dev/commit_helper.py` `doc_drift_warnings` | acknowledged: matches the original check's per-commit behaviour; advisory (non-blocking) |
| 3 | NOTE | Existing `commit_helper_test.py` (4) and `commit_helper_test.go` re-run to prove the new gates did not regress the helper. | `scripts/dev/commit_helper_test.*` | verified: both pass |

### Fixes applied
- `scripts/dev/commit_helper.py`: `deferral_in_diff_problems` now matches `_deferral_prose(line)` (string-literal + inline-code spans blanked), so the pattern list / rule docs / fixtures do not self-trip; bare markdown/comment prose still caught.
- `scripts/dev/hook-fixture-check.py`: added the two exemption fixtures.
- `plan/spec-followup-hooks.md`, `plan/learned/1093-followup-hooks.md`: reworded prose to keep trigger phrases quoted/backticked (honest -- there are no real deferrals in this work).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (none) | Fresh pass over the full diff after the fix: wiring OK (every new gate reachable from `create()` / `make ze-hook-test`), no logic/security/perf/allocation findings, no removed-invariant, no test weakening (`audit-test-relaxation.py` clean). | - | 0 BLOCKER, 0 ISSUE |

Evidence: `make ze-hook-test` exit 0 (parity 131/131, fixtures 26/26); `commit_helper_test.{py,go}` pass; `make ze-validate` clean; real-commit simulation over the commit-A file set shows every commit gate EMPTY.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| scripts/dev/hook-fixture-check.py | yes | new file; `make ze-hook-test` runs it |
| .claude/hooks/pretool-writeedit.py | yes | c_format_alloc enabled |
| .claude/hooks/validate-spec.sh | yes | arrow + set -e fix |
| .claude/hooks/pretool-bash.py | yes | four dead checks removed |
| scripts/dev/commit_helper.py | yes | commit-time gates added |
| scripts/dev/hook-parity-check.py | yes | fixture-dir portability fix |
| ai/rules/hook-mapping.md | yes | stale rows corrected |
| Makefile | yes | ze-hook-test target |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | guarded file w/ banned primitive in code -> exit 2; attribute/text.go dropped; json.go added | hook-fixture-check.py: format-alloc-live-join/live-builder/json-guarded/reactor-filter PASS; format-alloc-stale-attribute-path PASS (None) |
| AC-2 | banned pattern only in comment -> None | format-alloc-comment-exempt PASS |
| AC-3 | ASCII-arrow spec validates, no set -e abort | validate-spec-ascii-arrows PASS (rc 0); OLD HEAD script rc 1 |
| AC-4 | survey, zero crashes, decision recorded | survey table above: 0 crashes; decision = keep exit 2 blocking |
| AC-5 | four gates fire on the real path, severities preserved | commit-gate deferral-unassigned/in-diff (block), wiring/doc-drift (warn), create-blocks-deferral (rc 2) PASS |
| AC-6 | closure commit w/ unfilled PCV blocks; no claim skips | commit-gate-spec-audit-blocks/filled-ok/no-claim-skips/non-closure-skips PASS |
| AC-7 | make ze-hook-test runs all fixtures; listed in hook-mapping.md | `make ze-hook-test` exit 0 (parity 131/131 + fixtures 24/24); hook-mapping.md "Hook tests" section |
| AC-8 | hook-mapping.md stale rows corrected | format-alloc live, validate-spec fixed, commit gates re-homed |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| c_format_alloc on guarded Write | n/a (dev tool; hook-fixture-check.py) | yes -- 8/8 format-alloc cases |
| validate-spec.sh PostToolUse | n/a | yes -- 3/3 cases + 124-spec survey |
| commit_helper create() gates | n/a | yes -- create-blocks-deferral drives real create() to rc 2 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | file:line evidence re-verified at audit |
| A-2 | confirmed | 0 code-level banned primitives in the 8 guarded files |
| A-3 | confirmed | 124-spec survey: 0 crashes, 0 arrow false-positives |
| A-4 | confirmed | gates in commit_helper.create(); fixtures + real-path test pass |
| A-5 | confirmed | parity vs HEAD identical; 0 goldens re-blessed; 131/131 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| hook-mapping.md format-alloc / validate-spec / commit-gate rows | match edited pretool-writeedit.py / validate-spec.sh / commit_helper.py | yes |
| Makefile ze-hook-test target | `make ze-hook-test` exit 0 | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] Every work item has feature code + fixture test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Rehome commit gates into commit_helper.py creation-time gates (primary), with `bash tmp/commit-` prefix match in pretool-bash.py as fallback | Only widen the pretool-bash substring match | Helper already hosts repo-state gates (:464-526); creation time is when the agent can still fix the commit; fixtures drive functions directly either way |
| Enable format guard as blocking (exit 2) with comment exemption | Warn-only rollout | Consistent with `c_sprintf_new`; A-2 grep shows zero code-level violations exist, so no disruption |
| Fix validate-spec.sh then default to keeping exit-2 blocking, gated on the AC-4 survey | Permanent warn-only | The hook is registered blocking:true and its ERRORS are structural; survey protects against legacy-spec breakage |
| Port spec-audit keyed to per-session spec markers | Resurrect tmp/session/selected-spec | selected-spec substrate was removed in `276d72c99`; markers are the live mechanism |

## Known Limitations

- Edit-tool calls still only expose `new_string` to content checks; a banned primitive already present in an untouched part of a guarded file is not detected by the enabled guard (same limitation as every content check in the dispatcher).
- Fixture runner covers hook logic, not Claude-harness JSON quirks; the stdin contract itself is assumed stable.

## Notes
- Designed 2026-07-09 from skeleton; user instruction 2026-07-09 authorized batch conversion to ready.
- Functional `.ci` tests intentionally N/A: no daemon-visible surface (fixture tests are the goal evidence; internal dev tooling exemption).
