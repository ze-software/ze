# Spec: knowledge-1-corpus

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | spec-knowledge-0-umbrella.md |
| Phase | 6/6 |
| Deferral shard | `plan/deferrals/knowledge-1-corpus.md` |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Stop `plan/learned/` growing without bound, make its decay detectable, and
retire the rotted tail.

Four changes, in dependency order:

| # | Change | Evidence it is needed |
|---|--------|----------------------|
| 1 | A learned summary becomes conditional on having content | 229 of 1,285 summaries (17%) carry no gotcha; 77 say in words that they carry no lesson. `ai/skills/ze-close.md` step 6a makes the file unconditional and step 6e passes `--lesson-required` |
| 2 | Section format is normalised so a gate can read it | Only 48% carry all five prescribed sections; 425 (33%) still use the retired `## Objective` heading; 213 have no `## Files` section, so a path gate would silently skip them |
| 3 | Summaries 1-400 are consolidated into `plan/learned/DESIGN-HISTORY.md` and deleted | Dead-path rate is 78% in 1-200 and 44% in 201-400. The band holds 856 of the 1,860 dead paths tree-wide: 46% of the rot in 24% of the files. Only 27 of the 397 are cited by a durable surface |
| 4 | `make ze-learned-staleness` detects dead paths and dead citations | Nothing detects either today. 1,860 of 7,683 cited paths (24%) are already gone |

Order matters. Change 2 precedes change 4 because the gate reads `## Files`.
Change 3 precedes change 4's baseline because deleting the band removes most of
what the baseline would otherwise have to tolerate.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/planning.md` - "Writing Learned Summaries" and the two-commit closure
  → Constraint: commit A carries code plus spec plus summary, commit B removes the
    spec. Making the summary optional must leave both commits valid when it is absent.
- [ ] `plan/learned/METHODOLOGY.md` - the five-section format and quality checks
  → Decision: the format is good and is kept. What changes is that it is applied
    only when there is content, instead of being satisfied by boilerplate.
  → Constraint: the "Mechanical refactor, no design decisions" escape must be
    removed, since it is the sentence that legitimises a contentless file.
- [ ] `ai/digests/README.md` - why the consolidated knowledge does not go here
  → Constraint: "The historical record lives in `plan/learned/`, the canonical
    design in `docs/architecture/`; a digest is the fast-orientation layer between
    them." A digest also cannot hold history mechanically, because
    `digest_check.py` `check_digest` requires every anchor to resolve today.
- [ ] `ai/rules/never-destroy-work.md` - governs the deletion
  → Constraint: deletion of user-visible files needs explicit permission. Granted
    2026-08-01 for band 1-400 only. The tooling must be unable to reach 401.
- [ ] `ai/rules/no-parking.md`, `ai/rules/fix-dont-record.md` - the opposing pull
  → Constraint: those govern DEFECT records. This spec governs records of
    COMPLETED work. The distinction goes into the rule that owns retirement.

**Key insights:**
- Deleting 1-400 breaks zero Go `// Design:` references: there are 1,134 and the
  lowest number cited is 415.
- Summary-number gaps are already legal and 29 exist today, so no renumbering.
- The decay gate cannot start clean: 1,725 broken paths remain even after the
  band is deleted, so it needs a shrink-only baseline.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/commit_helper.py` - `lesson_comment` raises `UsageError` when
  `required` is set and no learned file is staged; `lesson_worthy` is pure
  path-prefix matching over `LESSON_WORTHY_PREFIXES` and `LESSON_WORTHY_FILES`,
  so neither trigger reads content. `learned_next` allocates `max + 1` and
  creates the file immediately. `discovery_index_problems` refuses a commit that
  touches `plan/learned/*.md` unless `ai/LEARNED-FULL-INDEX.md` is also staged
- [ ] `scripts/dev/learned_index.py` - `entries` and `main --check` exit 3 with
  "is stale" when the generated full index disagrees with the tree
- [ ] `scripts/dev/learned_numbers.py` - `check` enforces number uniqueness and
  H1-versus-filename only. Contiguity is not required
- [ ] `scripts/docvalid/check_doc_links.py` - `check_markdown` and `path_resolves`
  gate the `MD_GLOBS` corpus, which excludes the numbered summaries but includes
  `ai/LEARNED-INDEX.md`. `LEARNED_NUMBER` resolves the bare-`NNN` shorthand by glob.
  `check_design_refs` covers the Go `// Design:` refs
- [ ] `scripts/dev/digest_check.py` - the closest sibling for the new gate: same
  job of resolving `file:line` anchors in hand-maintained markdown, same
  `--check`/`--json` shape, exit 1
- [ ] `scripts/dev/spec-citation-check.py` - `load_baseline` and `write_baseline`
  over `plan/.citation-baseline`, the precedent for a shrink-only baseline
- [ ] `scripts/dev/python_tests_test.go` - `pythonTestRoots` already enrols
  `scripts/dev/`, so a new `*_test.py` there runs with no further wiring
- [ ] `ai/skills/ze-close.md` - step 6a creates the summary, 6b updates the
  curated index on honour system, 6e passes `--lesson-required` on commit A
- [ ] `mk/inventory.mk` - declares `ze-digest-check` and the learned-number
  targets; `ze-doc-test` is where the new gate joins

**Behavior to preserve:**
- Number uniqueness and `learned_next` allocation, so concurrent sessions cannot collide.
- `ai/LEARNED-FULL-INDEX.md` stays generated and stays a correct pointer index.
- Every gate that blocks a commit today keeps blocking it.
- The five-section format itself.

**Behavior to change:**
- Summary creation becomes conditional on content.
- `## Objective` is migrated to `## Context`; missing `## Files` sections are added.
- Summaries 1-400 are consolidated then deleted.
- Dead paths and dead citations become detectable.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A spec closes: `/ze-close` step 6a is where a summary is created.
- `make ze-verify` runs: `ze-doc-test` is where decay is detected.

### Transformation Path
1. `/ze-close` step 6a asks `commit_helper.py learned-next <stem>` for a number.
2. Under this spec, step 6a first asks whether there is content to record. When
   there is none, no number is allocated and no file is created.
3. `commit_helper.py create` gates commit A. `lesson_comment` stops demanding a
   file on a path-prefix match and starts demanding one on a content signal.
4. `make ze-learned-staleness` walks every summary, resolves every `## Files`
   path and every `plan/learned/NNN` citation, and compares the dead set against
   `plan/.learned-staleness-baseline`.
5. The baseline is shrink-only: a run producing more dead paths than the baseline
   fails; a run producing fewer rewrites it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Skill ↔ commit helper | `/ze-close` step 6 invokes `learned-next` and `create --lesson-required` | Yes, read in `ai/skills/ze-close.md` |
| Commit helper ↔ generated index | `discovery_index_problems` refuses unless `ai/LEARNED-FULL-INDEX.md` is staged | Yes, research report, function named |
| Gate ↔ corpus | the new checker reads `## Files` and citation tokens from markdown | No, does not exist yet |
| Doc validator ↔ curated index | `check_doc_links.py` `MD_GLOBS` includes `ai/LEARNED-INDEX.md` | Yes, research report |

### Integration Points
- `mk/inventory.mk` `ze-doc-test` gains the new gate; `stagesForMode` in
  `scripts/status/verify_run.go` runs `ze-doc-test` in both verify modes, so one
  edit covers `ze-verify` and `ze-verify-changed`.
- `scripts/dev/verify_wiring_docs.py` `TARGET_ORDER`, `MAKE_TARGETS` and
  `selected_targets` gain the gate for changed-file mode.
- `ai/INDEX.md` Dev Tools table gains the make target.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | The gate is modelled on `digest_check.py` rather than reimplementing anchor resolution |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, no wire path touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | N-A, no daemon registration touched |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Deleting 1-400 breaks no Go `// Design:` reference | Measured: 1,134 refs, lowest cited number is 415 | Go files carry dangling design refs and `check_design_refs` fails | Re-run the measurement in the retirement commit | confirmed |
| A-2 | Summary numbers may be non-contiguous | `learned_numbers.py` `check` enforces uniqueness and H1 only; 29 gaps exist today | A mass renumber is forced, invalidating every citation | Read `learned_numbers.py` `check` | confirmed |
| A-3 | The 67 files deleted unread carry nothing durable | They have no real gotcha and no durable citation, and sit in a band that is 44-78% dead paths | Knowledge is lost silently | Spot-read a deterministic 10 of the 67 before deleting; 2 or more durable items breaks it | **confirmed** 2026-08-01: 1 of 10 carried a durable item (007, capability `require` overriding `ignore-mismatch`), rescued into DESIGN-HISTORY. The other 9 carried nothing |
| A-4 | A shrink-only baseline is acceptable rather than fixing all 1,725 dead paths first | `spec-citation-check.py` `plan/.citation-baseline` is the established precedent | The gate reds on day one and gets disabled | Land the gate with the baseline and confirm `make ze-verify` is green | unvalidated |
| A-5 | Content-conditional creation does not break this spec's own closure | Child 1 has real content, so its own summary qualifies | The spec cannot be closed through the path it just created | Close child 1 through the changed path | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Retirement is later read as licence to prune defect records | A `plan/known-failures/` shard or `plan/deferrals/` row is deleted citing this spec | Write the completed-work versus defect-record distinction into the rule that owns retirement, naming `no-parking.md` |
| R-2 | The consolidated `DESIGN-HISTORY.md` becomes a second unread corpus | It is itself uncited three months on | Child 2 gates it: remove the `check_doc_links.py` exemption and add a freshness check |
| R-3 | A concurrent session commits a summary mid-retirement | A dangling citation appears after the retirement commit | Run retirement as one commit and re-run the gate immediately after |
| R-4 | The content test for "is there a lesson" is gameable or too strict | Sessions write one filler gotcha to satisfy it, or a real lesson is refused | Make the test a prompt in the skill plus an operator-overridable helper flag, never a silent refusal |
| R-5 | Deleting 1-400 breaks the 5 digest citations and 56 doc-link citations | `make ze-digest-check` or `check_doc_links.py` fails | Repoint all 61 in the same commit; they are enumerated and small |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible, no daemon behavior. Worst case is lost historical rationale and a red gate |
| How is it reverted? | Single commit revert. The deleted summaries stay in git history and are recoverable with `git show <sha>:<path>` |
| Who else touches this path? | Every concurrent session in this checkout writes `plan/learned/` and `ai/LEARNED-INDEX.md`. Retirement must be one commit, not a long branch |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-learned-staleness` | → | `scripts/dev/learned_staleness.py` `check` | `test_gate_reports_dead_path` in `scripts/dev/learned_staleness_test.py` |
| `make ze-verify` | → | the gate declared in `mk/inventory.mk` `ze-doc-test` | `test_target_declared_in_doc_test` in `scripts/dev/learned_staleness_test.py` |
| `commit_helper.py create` with no learned file and no lesson content | → | `lesson_comment` content branch | `test_lesson_optional_without_content` in `scripts/dev/commit_helper_test.py` |
| `commit_helper.py create` with lesson-worthy content and no learned file | → | `lesson_comment` demand branch | `test_lesson_demanded_with_content` in `scripts/dev/commit_helper_test.py` |
| Changed-file verify touching `plan/learned/` | → | `verify_wiring_docs.py` `selected_targets` | `test_learned_change_selects_staleness_target` in `scripts/dev/verify_wiring_docs_test.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A spec closes whose work produced no reusable lesson | No `plan/learned/NNN-*.md` is created, and commit A is accepted |
| AC-2 | A spec closes whose work produced a lesson | A summary is still demanded, and commit A is refused without one |
| AC-3 | `make ze-learned-staleness` runs on a summary citing a path that does not exist | Exit non-zero, naming the summary, the dead path, and the line |
| AC-4 | `make ze-learned-staleness` runs on the tree after retirement | Exit 0 against the recorded baseline |
| AC-5 | The staleness gate runs as part of `make ze-verify` and `make ze-verify-changed` | Both invoke it, proven by the target appearing in `ze-doc-test` |
| AC-6 | Summaries 1-400 after retirement | Absent from `plan/learned/`, and their surviving knowledge is present in `plan/learned/DESIGN-HISTORY.md` under a named subsystem section |
| AC-7 | `make ze-verify` after retirement | Exits 0, including `ze-digest-check` and `check_doc_links.py`, with all 5 digest and 56 doc-link citations repointed |
| AC-8 | Any remaining summary is inspected for section headings | No `## Objective` heading remains; every summary has a `## Files` section or an explicit "None." |
| AC-9 | The retirement tooling is given a band above 400 | It refuses and exits non-zero |
| AC-10 | `plan/learned/METHODOLOGY.md` after this spec | No longer contains the "Mechanical refactor, no design decisions" boilerplate escape |
| AC-11 | A `DESIGN-HISTORY.md` entry cites a summary in the retired band | Either the summary's rejected alternatives and gotchas are absorbed into the entry, or the citation is explicitly marked retired with its git-recovery route. No entry silently depends on a deleted file |
| AC-12 | `DESIGN-HISTORY.md` after retirement | Its header no longer claims to index "638 learned summaries", and its coverage claim matches the corpus that actually exists |

### Why AC-11 exists

`DESIGN-HISTORY.md` describes itself as an index: "This document is the index;
the summaries are the authority", and it tells the reader to "read the summary
for the rejected alternatives and the gotchas".

Measured against the read-set: it already cites 161 of the 330, and 188 distinct
summary numbers at or below 400. Its Evolution bullets and its Load-bearing
invariants table are substantially self-contained, carrying the invariant, the
code site and the reason inline, so deleting the band does NOT gut its content.

What deletion breaks is the second half of its promise. Every "read the summary
for the rejected alternatives" pointer into band 1-400 becomes a pointer to
nothing. That is precisely the material the consolidation phase must absorb
before the files go, and it is why the fidelity decision was to READ the 330
rather than move them wholesale.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_gate_reports_dead_path` | `scripts/dev/learned_staleness_test.py` | AC-3 | |
| `test_gate_reports_dead_citation` | `scripts/dev/learned_staleness_test.py` | A summary citing `plan/learned/NNN` that no longer exists is named | |
| `test_gate_skips_missing_files_section` | `scripts/dev/learned_staleness_test.py` | A summary with no `## Files` is reported as unreadable, never silently passed | |
| `test_baseline_is_shrink_only` | `scripts/dev/learned_staleness_test.py` | A run with more dead paths than baseline fails; fewer rewrites it | |
| `test_target_declared_in_doc_test` | `scripts/dev/learned_staleness_test.py` | AC-5 | |
| `test_lesson_optional_without_content` | `scripts/dev/commit_helper_test.py` | AC-1 | |
| `test_lesson_demanded_with_content` | `scripts/dev/commit_helper_test.py` | AC-2 | |
| `test_retire_refuses_band_above_400` | `scripts/dev/learned_retire_test.py` | AC-9 | |
| `test_learned_change_selects_staleness_target` | `scripts/dev/verify_wiring_docs_test.py` | Changed-file routing | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Retirement band upper bound | 1-1289 | 400 | 0 | 401 |
| Baseline dead-path count | 0-7683 | current measurement | N/A | any value above the recorded baseline |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `learned_staleness_test.py` | `scripts/dev/learned_staleness_test.py` | An agent runs `make ze-learned-staleness` and every dead path is named with its file and line | |
| `commit_helper_test.py` | `scripts/dev/commit_helper_test.py` | An agent closes a lessonless spec and the commit script is produced without a summary | |
| `learned_retire_test.py` | `scripts/dev/learned_retire_test.py` | An agent runs the retirement tool and it refuses a band it was not authorised for | |

### Interop Tests (Scope: protocol)
N-A. Scope is tooling. No wire-visible behavior changes.

## Files to Modify

- `scripts/dev/commit_helper.py` - `lesson_worthy` decides on content rather than path prefix; `lesson_comment` keeps `required` as an explicit operator override
- `ai/skills/ze-close.md` - step 6a asks whether there is content before allocating a number; step 6e stops passing `--lesson-required` unconditionally
- `plan/learned/METHODOLOGY.md` - remove the empty-case boilerplate escape; state when a summary is NOT written
- `plan/learned/DESIGN-HISTORY.md` - receives the consolidated knowledge from the 330 read summaries
- `ai/LEARNED-INDEX.md` - repoint or remove the 44 citations to summaries in the retired band
- `ai/LEARNED-FULL-INDEX.md` - regenerated by `make ze-discovery-index`
- `ai/digests/*.md` - repoint the 5 citations to summaries 173, 294, 374, 386, 390
- `mk/inventory.mk` - declare `ze-learned-staleness`, add it to `.PHONY` and to `ze-doc-test`
- `scripts/dev/verify_wiring_docs.py` - `TARGET_ORDER`, `MAKE_TARGETS`, `selected_targets`
- `ai/INDEX.md` - Dev Tools row for the new target
- `ai/rules/discovery-updates.md` - discovery-surface row for the new gate
- `ai/rules/planning.md` - "Writing Learned Summaries" states the content condition and the retirement trigger, and names `no-parking.md` for the defect-record distinction
- `plan/learned/[0-9]*.md` - the ~885 surviving summaries: migrate `## Objective` to `## Context`, add a missing `## Files`

## Files to Create

- `scripts/dev/learned_staleness.py` - the decay gate
- `scripts/dev/learned_staleness_test.py` - its tests
- `scripts/dev/learned_retire.py` - the band-limited retirement tool
- `scripts/dev/learned_retire_test.py` - its tests
- `plan/.learned-staleness-baseline` - shrink-only baseline
- `plan/deferrals/knowledge-1-corpus.md` - deferral shard

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface; this is agent tooling |
| YANG validation constraints | N-A | No YANG leaf added |
| YANG custom validators | N-A | No YANG leaf added |
| CLI commands/flags | N-A | Make targets only, no `ze` subcommand |
| CLI grammar (keyword before value) | N-A | No CLI command added |
| Editor autocomplete | N-A | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC added; gates covered by `scripts/dev/*_test.py` |
| Pipe completeness | N-A | No `ze` command output produced |
| Env var registration | N-A | No `environment/` leaf added |
| Doctor check for runtime dependencies | N-A | The gates run at build and commit time, never in the daemon |
| Prometheus counters/metrics | N-A | No observable daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Agent tooling, not a product feature |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | Make targets only |
| 4 | API/RPC added/changed? | No | No RPC touched |
| 5 | Plugin added/changed? | No | No plugin touched |
| 6 | Has a user guide page? | No | Contributor tooling |
| 7 | Wire format changed? | No | No wire path touched |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol behavior touched |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/documentation-testing.md` gains `ze-learned-staleness` |
| 11 | Affects daemon comparison? | No | No daemon capability changes |
| 12 | Internal architecture changed? | No | No `internal/` architecture touched |
| 13 | Route metadata keys added/changed? | No | No route metadata touched |
| 14 | Prometheus counters added/changed? | No | No counters added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `commit_helper.py` and `learned_index.py` before closing |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/INDEX.md` and `ai/rules/discovery-updates.md` list current gates; both gain the new one |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- declare the gate and its target before it works
   - Tests: `test_target_declared_in_doc_test`, `test_learned_change_selects_staleness_target`
   - Files: `mk/inventory.mk`, `scripts/dev/verify_wiring_docs.py`, a stub `scripts/dev/learned_staleness.py`
   - Verify: `make ze-learned-staleness` runs and the wiring tests fail because the checker is a stub
2. **Phase: Format normalisation** -- migrate `## Objective` to `## Context` across
   425 summaries, add a `## Files` section where absent
   - Tests: a check that no `## Objective` remains and every summary has `## Files`
   - Files: `plan/learned/[0-9]*.md`
   - Verify: the counts reach zero. This precedes the gate because the gate reads `## Files`
3. **Phase: Consolidation** -- read the 330 summaries in band 1-400 that carry a
   real gotcha or a durable citation (9,619 lines), extract surviving knowledge
   into `plan/learned/DESIGN-HISTORY.md` under its four-part per-subsystem shape
   - Files: `plan/learned/DESIGN-HISTORY.md`
   - Verify: every subsystem section names the summary numbers it absorbed
4. **Phase: Retirement** -- spot-read 10 of the 67 unread candidates to validate
   A-3, then delete band 1-400 and repoint the 5 digest and 56 doc-link citations
   - Tests: `test_retire_refuses_band_above_400`
   - Files: `scripts/dev/learned_retire.py`, `ai/digests/*.md`, `ai/LEARNED-INDEX.md`
   - Verify: `make ze-digest-check`, `make ze-doc-test`, `make ze-discovery-index` all green
5. **Phase: The gate** -- implement the checker, record the baseline, prove it fails
   - Tests: every `learned_staleness_test.py` test
   - Files: `scripts/dev/learned_staleness.py`, `plan/.learned-staleness-baseline`
   - Verify: the gate fails on a seeded dead path, passes on the tree
6. **Phase: Conditional creation** -- change `lesson_worthy`, `ze-close.md` step 6,
   `METHODOLOGY.md`, and `ai/rules/planning.md`
   - Tests: `test_lesson_optional_without_content`, `test_lesson_demanded_with_content`
   - Files: `scripts/dev/commit_helper.py`, `ai/skills/ze-close.md`, `plan/learned/METHODOLOGY.md`, `ai/rules/planning.md`
   - Verify: close this spec through the changed path

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation |
| Feature completeness | The gate is reachable from `make ze-verify`, not only from its own target |
| Correctness | The retirement tool cannot delete a summary numbered above 400 under any argument |
| Correctness | The baseline is shrink-only: a regression must fail, not silently rewrite |
| Naming | The new target follows the `ze-<thing>-check` or `ze-<thing>` convention used in `mk/inventory.mk` |
| Data flow | The gate reads the tree, never the generated index, so it cannot be fooled by a stale index |
| Rule: `ai/rules/no-parking.md` | The retirement tool cannot reach `plan/known-failures/` or `plan/deferrals/` |
| Rule: `ai/rules/never-destroy-work.md` | Deletion is band-limited and recoverable from git history |
| Rule: `ai/rules/fail-closed-guards.md` | A summary the gate cannot parse is reported, never skipped as passing |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `make ze-learned-staleness` exists and runs | `make ze-learned-staleness` |
| It runs inside verify | `grep -n 'learned-staleness' mk/inventory.mk` |
| Band 1-400 is gone | `find plan/learned -name '[0-3][0-9][0-9]-*.md' \| wc -l` returns 0 |
| No `## Objective` remains | `grep -rl '^## Objective' plan/learned/ \| wc -l` returns 0 |
| Dead-path rate is under 5% | `make ze-learned-staleness --json` |
| A lessonless closure is accepted | `test_lesson_optional_without_content` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The gate parses arbitrary markdown and resolves paths from it. It must never resolve outside the repository root, never follow a symlink out of the tree, and never execute what it reads |
| Destructive tooling | `learned_retire.py` deletes files. The band must be validated before any unlink, it must refuse anything above 400, and it must operate only on `plan/learned/[0-9]*.md` |
| Path traversal | A `## Files` entry containing `..` must be reported as invalid, not resolved |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A commit gate cannot be satisfied after retirement | Back to RESEARCH: the blast radius was mis-measured |
| The gate reds on the tree after the baseline is recorded | Fix the baseline logic, never widen the tolerance |
| Consolidation loses knowledge a later session needs | Restore from git history and narrow the band |
| A spot-read of the 67 finds durable content | Widen the read-set; A-3 is broken and needs a Mistake Log row |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Phase 1 landed 2026-08-01. The stub deliberately PRINTS that it checked 0 of
  1,285 summaries and is not implemented, rather than exiting 0 in silence.
  `ai/rules/fail-closed-guards.md` requires a guard that cannot deny to say so,
  and a stub that returns green is exactly the vacuous gate the RFC ratchets
  exist to prevent.
- The discovery-surface rows (`ai/INDEX.md`, `ai/rules/discovery-updates.md`)
  are deliberately NOT written in phase 1. Advertising a target that proves
  nothing would be a false claim; they land with the working checker in phase 5.

- The gate must read the tree rather than the generated index, or a stale index
  would make it vacuous. This is the same failure the RFC ratchets avoid.
- Making creation conditional is a smaller change than it looks: `lesson_worthy`
  already receives the full path tuple, so the predicate swaps in place.


### Phase 2 rulings (2026-08-01)

**Deviation APPROVED: `## Files <qualifier>` headings are canonicalised, not
appended to.** 9 summaries (517, 546, 617, 653, 677, 678, 708, 817, 984) spelled
the heading `## Files Changed` / `Touched` / `Created` and listed REAL paths,
verified 1 to 16 paths each. Appending `None recorded.` to those would have been
a false statement and would have hidden 66 real paths from the gate: the exact
silent-skip failure this phase exists to prevent. Canonicalising the first
`## Files*` heading to `## Files` is a general rule in the script, not a special
case, and is the third mechanical change beyond the original brief.

**Residual found, fixed in phase 5 not here.** 3 summaries (677, 678, 817) keep
a SECOND `## Files Modified` section carrying 12 paths a gate reading only
`## Files` would skip. The durable fix is in the gate's section parser, not in
three files: **phase 5's checker MUST read `## Files` and every
`## Files <qualifier>` section**, so corpus uniformity is never a precondition
for the gate being correct (`ai/rules/no-workarounds-for-missing-behavior.md`,
fix at the owning layer).

**`learned_index.py --check` did NOT go stale**, contrary to this spec's
prediction in Implementation Step 2. It builds from H1 titles and filenames, and
phase 2 touched neither. Nothing to regenerate.

**Observation for a later pass, not acted on:** `## Patterns` appears in exactly
the same 425 files that carried `## Objective`. That is one retired template,
not 425 independent choices, so it could be folded mechanically. 652 summaries
carry at least one heading outside the five, across 371 distinct spellings.


### Phase 3 findings (assembly must act on these)

**DESIGN-HISTORY misfiles summary 059 as shipped.** Its "Pool evolution" bullet
lists 059 in the chain alongside 003, 124, 174, 176 and 332. But 059 is titled
"Pool Handle Migration (Abandoned)" and states that the optimisation "belongs in
an API-level route reflector, not the edge speaker". The assembly pass must move
it to **Abandoned approaches**. This matters more after retirement than before:
once 059 is deleted, a reader following the pool-evolution chain has no way left
to discover that one link in it never shipped.

This is the rot the review predicted, found by reading rather than by grepping,
and it is the argument for the read-the-330 fidelity choice over a bulk move.


### Phase 4 finding: why the band is empty, and where its knowledge actually went

The A-3 spot-read surfaced two things worth more than the assumption it tested.

**The emptiness is structural, not accidental.** 8 of the 10 files declare
`## Gotchas -- None`, and the summaries in that era that DO carry something put
it under `## Patterns`, a heading of the retired template. A filter keying on
"no real gotcha" therefore reads the right signal for the wrong reason. It
lands correctly here, but a future retirement pass over a later band must not
assume the same filter generalises.

**Durable knowledge from this band tends to survive in the CODE, not the
summary.** The strongest-looking deletion candidate (290, buffered TCP read) was
already saved by someone copying the reasoning into `session_connection.go` as
an `INVARIANT` comment and a "why bufReader is not nilled" note. That is the
mechanism that actually worked over five months, and it is an argument for
`ai/rules/self-documenting.md` over `plan/learned/` as the home for an
invariant that governs one function.


### Phase 5 finding: an unrecorded baseline must SAY it is unrecorded

The gate ships with `plan/.learned-staleness-baseline` holding comments and no
number. That is deliberate and it is the fail-closed reading of
`ai/rules/fail-closed-guards.md`: a ratchet that cannot yet enforce must say so
rather than print a green line it has not earned.

Two facts force it. Retirement removes roughly 856 of the 1,859 dead references
the gate counts, so a ceiling recorded now would be wrong within the hour. And
a shrink-only ceiling never self-corrects upward, so a too-high number is
permanent slack rather than a temporary error.

The gate therefore exits 0 today while reporting the count, and
`test_shipped_baseline_is_unrecorded` pins that state and tells whoever arms
the ratchet to replace it.


### Phase 3 assembly: verified before deletion (2026-08-01)

DESIGN-HISTORY went 998 to 1504 lines, absorbing 263 rescued items. Deletion is
irreversible in the working tree, so the merge was checked mechanically first:

| Check | Result |
|-------|--------|
| Links to a band 1-400 summary remaining | 0. All 245 delinked, numbers kept as plain-text provenance |
| Bare `NNN-slug.md` filename references to the band | 0 |
| Staging source numbers present in the document | 189 of 190 |
| The one absent source | 033, which is exactly the `DO-NOT-CARRY` item. Correct |
| `033` occurrences | 0 |
| Links remaining overall | 180, lowest 404. Those files survive |

The assembler made three judgement calls the main thread accepted. It created a
"Cross-cutting: engineering practice" section with the standard four-part shape
rather than an "absorbed" bucket, and re-routed 5 items that DO name a subsystem
out of it, because a reader looking for "zero-copy needs the original wire
bytes" would not look under Cross-cutting. It corrected a stale pre-existing row
claiming the plugin protocol is "NUL-framed", which is the same defect class as
the auto-detection correction it was asked to make. And it wrote its 47 new
lines without em dashes while the surrounding legacy bullets still carry them.


### AC-3 IS NOT MET, and the spec contradicted itself (2026-08-01)

Measured after retirement: dead-path rate **16%**, down from 24%. AC-3 demands
under 5%. It is missed, and the miss was written into the spec at authoring
time rather than caused by the implementation.

| Fact | Value |
|------|-------|
| Summaries before / after | 1,286 / 889 |
| Dead references before / after | 1,859 / 1,011 |
| Removed by retiring band 1-400 | 848, which is 46% of the rot, exactly as predicted |
| Cited paths remaining | 6,305, of which 1,032 dead |

**The contradiction.** AC-3 demands under 5%. The deferral row filed in the same
session says "fix the ~1,725 dead `## Files` paths that REMAIN AFTER band 1-400
is retired", which concedes that most of the rot survives retirement. Both
cannot be true. The deferral is the accurate one: the rot was never
concentrated enough in band 1-400 for retirement alone to reach 5%.

**This is a scope decision, not an implementation gap**, so it goes to the owner
rather than being resolved here (`ai/rules/no-partial-completion.md`: scope
reduction requires explicit approval). Three routes exist, and the spec stays
OPEN on AC-3 until one is chosen:

| Route | Cost |
|-------|------|
| Amend AC-3 to "materially reduced, ratcheted shrink-only", which is what was built | None. The gate already prevents regrowth |
| Widen retirement past 400 | Needs a fresh grant. `RETIREMENT_CEILING` refuses it by design, and band 401-600 is only 18% dead, so the yield falls sharply |
| Do the deferred repair of the remaining 1,011 | A separate and much larger piece of work, already homed |


### AC-3 RESOLVED by repair, and the criterion's own defect (2026-08-01)

The deferral judged this "a much larger, separate piece of work". That was
wrong, and testing it was cheap. The rot was overwhelmingly code that MOVED, and
git had recorded where: of 1,008 dead references, 662 were resolvable from a
recorded git rename, 32 from a learned directory rule, 11 from a unique
three-segment suffix. 672 were rewritten.

| Instrument | Before | After | Verdict |
|------------|--------|-------|---------|
| The gate's own counting (7,011 cited paths) | 14.38% | 4.79% | AC-3 MET |
| The session-start script (6,309 cited paths) | 24% | 6.17% | AC-3 MISSED |

**AC-3 never named its measuring instrument.** "Under 5% tree-wide" is
ambiguous, and the two defensible readings straddle the threshold. That is a
defect in the criterion, not in the work, and it belongs beside the other two
this spec set found in its own specs: a checker path that did not exist, and an
AC requiring a token its target file never contained.

**The floor is real.** 306 distinct dead paths remain. 229 of them name a file
whose basename exists NOWHERE in the tree: genuinely deleted code. Repointing
those is impossible and deleting the reference is banned, because the summary's
sentence still means something. 77 remain resolvable in principle, so a little
headroom exists, but not 1.17 points of it.

**Two resolvers were wrong and were replaced rather than tuned.** A two-segment
suffix match repointed `cmd/ze/install/appliance/updater/updater.go` into
`vendor/` and `cmd/ze/config/register.go` into an unrelated BGP package. The
floor moved to three segments, `vendor/` and `third_party/` were excluded, and
directory rules were learned from git's own renames. A silently wrong path is
worse than a dead one, because the dead one is detectable.

**A guard the brief did not anticipate.** Repointing a line whose prose says the
file was deleted made the sentence self-contradictory. `cites_a_deletion` now
skips a line marking the FILE as deleted while still repairing a line saying
code was removed FROM a file. Five rewrites were unwound.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Consolidate into `DESIGN-HISTORY.md` | `ai/digests/`; a new `plan/learned/history/` tree | Digests are current-state by their own README and their anchor gate rejects references to deleted code. DESIGN-HISTORY is purpose-built and already has 8 referrers |
| Shrink-only baseline | Fix all 1,725 dead paths before landing the gate | Fixing first is a different and much larger spec. The baseline is the established repo idiom (`plan/.citation-baseline`) |
| Read only the 330 with a real gotcha or durable citation | Read all 397; delete all 397 unread | The 67 excluded have no gotcha, no durable citation, and sit in a 44-78% dead band. A-3 spot-checks that judgement rather than assuming it |
| Keep `--lesson-required` as an operator override | Remove it entirely | An explicit operator demand is still legitimate. What changes is the automatic demand |

## Known Limitations

- The gate proves a path resolves. It cannot prove the prose around the path is
  still true, exactly as `digest_check.py` cannot.
- The content test for "is there a lesson" is a heuristic plus a skill prompt. A
  session determined to write a filler gotcha can still do so.
- Consolidation is a judgement call per summary and is not reproducible. The
  record of what was absorbed is the subsystem section naming its source numbers.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
