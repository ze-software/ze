# Spec: problem-journal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 4/4 |
| Deferral shard | `-` (see the note below) |
| Updated | 2026-08-09 |

This spec has no shard of its own, and the metadata named one that never
existed. Every row it deferred is homed at the spec that OWNS the work, and
`plan/deferrals/<stem>.md` is named for that destination
(`scripts/dev/deferral_orphans.py` `spec_for_shard`), so a
`plan/deferrals/problem-journal.md` would have duplicated rows that already live
at their destinations. The three shards carrying this spec's rows, each naming
`plan/spec-problem-journal.md` in its Source cell, are
`plan/deferrals/fixit-dead-design-pointers-in-tests.md`,
`plan/deferrals/doc-claims-are-checked-not-just-resolved.md` and
`plan/deferrals/fixit-unexport-package-private-symbols.md`. All four rows across
them are accounted for in Deferrals Resolved below.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`plan/learned/` holds 943 summaries, 68,720 lines, and no tool counts a repeat.
Measured over 161 main sessions and 1,111 subagents on this machine: ordinary
work (contexts not tasked on the corpus itself) opened a summary in 123 contexts,
289 read events, touching 149 of 943 files. Two thirds of all reading came from
tasks whose subject WAS the corpus. `ai/LEARNED-INDEX.md` was opened 16 times in
1,272 contexts. Of 824 edits to a Go file carrying a `// Design: plan/learned/NNN`
pointer, the pointed summary was opened 56 times.

Goal: replace the corpus with a journal whose recurrence is countable, and delete
the 943 shards.

The journal is `plan/journal/<class>.md`, one file per problem class, each holding
one table:

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|

Recurrence is the row count. A second row in a file is the alarm. The class
vocabulary is the directory listing, so no validator is needed. Two sessions
hitting different classes touch different files, which is why `plan/deferrals/`
is sharded the same way (`scripts/dev/commit_helper.py` `deferral_shard_paths()`).

The `Spec` column is not decoration. `commit_helper.py` `spec_closure_stem()`
recognises commit A as a spec closure by the NEW `plan/learned/NNN-<stem>.md`
path it adds, and passes that stem to `review_gate_problems()`, which is what
blocks a closure whose code carries no independent review. A class-named file
carries no stem, so without this column the review gate stops firing on the
commit that holds the code. Rows written outside a spec carry `-`.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/planning.md` - "Writing Learned Summaries" is the rule the journal replaces
  → Constraint: the closure artifact is what `spec-closure-check.py` keys on
- [ ] `ai/rules/simplicity.md` - the journal must not grow machinery the corpus lacked
  → Constraint: one new script, one new make target, no index file

**Key insights:**
- A committed generated index republishes other sessions' unlanded work
  (`plan/learned/1352`). The journal therefore has no committed index; the
  detector computes on demand.
- `scripts/dev/deferral_orphans.py` imports `_deferral_row_cells` and
  `deferral_shard_paths` from `commit_helper`, so the shard-and-table reader
  already exists and the journal reader mirrors it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/deferral_orphans.py` - reads sharded tables via `commit_helper._deferral_row_cells()`; `spec_for_shard()` shows the stem-to-file convention the journal copies
- [ ] `mk/inventory.mk` - `ze-discovery-index(-check)` runs `learned_index.py`; the `ze-learned-*` targets are the machinery the journal removes
- [ ] `scripts/dev/spec-closure-check.py` - `_committed_learned()` treats a git-tracked summary as the closure signal
- [ ] `scripts/dev/rules_router.py` - `load_corpus()` builds the rule-routing corpus from each summary's `## Context` section
- [ ] `scripts/dev/check_doc_links.py` - resolves every `plan/learned/NNN` citation repo-wide
- [ ] `scripts/dev/commit_helper.py` - `lesson_comment()` and `lesson_worthy()` demand a summary

**Behavior to preserve:**
- Spec closure keeps an artifact gate. Only the artifact changes.
- `plan/deferrals/` is untouched. The journal records problems, not deferred work.
- `plan/known-failures/` is untouched.

**Behavior to change:**
- The `commit_helper` lesson gate demands a journal row, not a summary file.
- `plan/learned/` is deleted except `DESIGN-HISTORY.md`, `HOOK-FRICTION.md`,
  `RECURRING-PATTERNS.md`, which outread every individual summary (30, 22, and
  the hand-written recurrence record, against a best shard of 10).

## Data Flow (MANDATORY)

### Entry Point
- An agent identifies a problem. It appends one row to `plan/journal/<class>.md`,
  creating the file when the class is new.

### Transformation Path
1. `commit_helper.py` `lesson_comment()` demands the row on the same trigger it
   demands a summary today.
2. `scripts/dev/journal.py` reads `plan/journal/*.md` at git HEAD and groups rows.
3. `make ze-journal` prints every class with 2 or more rows, newest first.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Commit gate ↔ journal | `lesson_comment()` matches a `plan/journal/` path in `--file` | No |

### Integration Points
- `scripts/dev/deferral_orphans.py` - same shard-and-table parsing shape, extend rather than reinvent

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire path |
| Registration over hardcoding | N-A | no plugin surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Class names stay few enough that a directory listing is a usable vocabulary | `plan/learned/RECURRING-PATTERNS.md` already names 28 patterns by hand | the journal fragments and counts nothing | 28 seed classes is the starting size; recount after 30 days of use | partial |
| A-2 | Deleting summaries does not change the always-on rule set | `rules_router.py` `load_corpus()` reads them as corpus and `rules_condensed.core_members()` derives `CORE.md` from it | `ai/rules/CORE.md` membership changes silently | ran `rc.ARTIFACTS` regeneration against both corpora: `TRIGGERS.md` and `CORE.md` byte-identical, same 8 always-on rules | confirmed |
| A-3 | The Go files citing a summary can lose the pointer without losing knowledge | 1150 tracked non-vendor Go files, 1155 pointers, 204 distinct summaries; measured follow rate 56 of 824 | design rationale becomes unreachable | grep the cited summaries for content absent from `docs/architecture/` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The journal becomes a second write-only corpus | rows accumulate, `make ze-journal` never run | the detector runs inside `ze-doc-test`, so a repeat is printed without being asked for |
| R-2 | Deleting 943 files loses something still load-bearing | a doc-links red, or a spec citing a deleted summary | `learned_retire.py` `scan_citations()` already cross-checks citations before removal |
| R-3 | A gate is made quiet rather than honest | `check_index_budget()` early-returns on a missing `INDEX_FILE`, so deleting the index alone leaves a green check with no target | delete the check, do not leave it pointed at nothing |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. Agent workflow gates only: spec closure, doc links, rule routing |
| How is it reverted? | Single commit revert while the deletion is in history |
| Who else touches this path? | Every session that closes a spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-journal` | → | `scripts/dev/journal.py` `report()` | `test_report_flags_second_occurrence` |
| `commit_helper.py create` on a rules change | → | `lesson_comment()` | `test_lesson_gate_accepts_journal_row` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `plan/journal/<class>.md` holds 2 or more rows | `make ze-journal` prints the class, its row count, and the span between first and last date |
| AC-2 | Every class file holds 1 row | `make ze-journal` prints nothing and exits 0 |
| AC-3 | A commit that would demand a learned summary today stages a `plan/journal/` row instead | `commit_helper.py create` accepts it without `--lesson-not-needed` |
| AC-4 | A journal file has a malformed table | `make ze-journal` names the file and exits non-zero |
| AC-5 | `plan/learned/` is deleted apart from the three aggregates | `make ze-doc-test` passes, with no dead `plan/learned/NNN` citation anywhere |
| AC-6 | A spec closes | `spec-closure-check.py` accepts a staged journal row naming that spec as the commit-A artifact |
| AC-7 | Commit A adds a journal row naming a spec and carries code files | `commit_helper.py` `review_gate_problems()` still blocks when no clean independent review exists for that spec |
| AC-8 | A Go file carries a `// Design:` pointer | it names a file under `docs/architecture/` that exists and states the rationale, and `check_doc_links.py` `check_design_refs()` resolves it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_report_flags_second_occurrence` | `scripts/dev/journal_test.py` | AC-1 | |
| `test_report_silent_on_singletons` | `scripts/dev/journal_test.py` | AC-2 | |
| `test_malformed_table_is_named` | `scripts/dev/journal_test.py` | AC-4 | |
| `test_lesson_gate_accepts_journal_row` | `scripts/dev/commit_helper_test.py` (`TestLessonIsContentDriven`) | AC-3 | |
| `test_journal_row_is_a_closure_signal` | `scripts/dev/commit_helper_test.py` | AC-7 | |
| `TestSpecClosureAcceptsJournalRow` | `scripts/dev/spec_closure_check_test.go` | AC-6 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-journal` inside `ze-doc-test` | `mk/inventory.mk` | an agent sees a repeat without asking for it | |

## Files to Modify
- `scripts/dev/commit_helper.py` - add `"plan/journal/"` to `ROUTE_PREFIXES`, which is the whole lesson-gate change (`routed_paths()` already satisfies `lesson_comment()`); rewrite `spec_closure_stem()` and `_LEARNED_STEM_RE` to take the stem from a new journal row's `Spec` cell; `closure_reminder()` follows the same signal; delete the `learned-next` subcommand
- `scripts/dev/spec-closure-check.py` - `_committed_learned()` and `_learned_files()` become journal-row readers. Note it uses `git ls-files`, so the artifact counts once staged, not once committed. `SpecReport.learned_exact` matches on the row's `Spec` cell instead of filename slug tokens
- `scripts/dev/rules_router.py` - `main()` passes `plan/` only. Keep `plan/` in `load_corpus()`: the three kept aggregates contribute nothing anyway (`stem.isupper()` skips them), and an empty corpus flips off derivation 4 in `rules_condensed.unreachable_blocking()`
- `scripts/dev/check_doc_links.py` - delete check 3 whole: `check_index_budget()`, `index_entry_length()`, `INDEX_FILE`, `INDEX_ENTRY`, `INDEX_SEPARATOR`, `POINTER_BUDGET`, `INDEX_BUDGET_BLOCKING`, its `main()` arm, and the docstring's "Four checks". Leaving it pointed at a deleted file makes it pass quietly, which is the failure. Drop the `LEARNED_NUMBER` branch in `path_resolves()`
- `scripts/dev/check_doc_links_test.py` - `test_real_corpus_is_present_and_clean` asserts `INDEX_FILE` exists; drop that arm
- `ai/INDEX.md` (43 refs) and `ai/rules/*.md` (21 refs across 11 rules), `ai/rationale/git-safety.md` - citations to deleted summaries, which `check_markdown()` reds on
- `mk/inventory.mk` - remove the `ze-learned-*` targets and `learned_index.py` from `ze-discovery-index`, add `ze-journal`
- `ai/rules/planning.md`, `ai/skills/ze-close.md` - the closure artifact is a journal row

## Files to Create
- `scripts/dev/journal.py` - the reader and the detector
- `scripts/dev/journal_test.py` - its tests
- `plan/journal/README.md` - the format, in the shape of `plan/deferrals/`

## Files to Delete
- `plan/learned/[0-9]*.md` - 943 shards
- `ai/LEARNED-INDEX.md`, `ai/LEARNED-FULL-INDEX.md`, `scripts/dev/learned_*.py`
Not deleted, REPOINTED (owner decision, 2026-08-09): the 1155 `// Design:`
pointers across 1150 tracked non-vendor Go files cite 184 distinct summaries.
The rationale of those 184 moves to `docs/architecture/` and every pointer
references the file there. The other 739 summaries stay deleted and remain
recoverable from git history.
- The `learned-next` subcommand and the seven doc sites naming it:
  `ai/rules/git-safety.md`, `ai/rules/planning.md`, `ai/skills/ze-close.md`,
  `ai/skills/ze-commit.md`, `ai/skills/ze-commit-check.md`, `ai/skills/ze-progress.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface |
| YANG validation constraints | N-A | no config surface |
| YANG custom validators | N-A | no config surface |
| CLI commands/flags | N-A | a make target, not a `ze` subcommand |
| CLI grammar (keyword before value) | N-A | no CLI surface |
| Editor autocomplete | N-A | no config surface |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no route output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | no daemon state |
| BGP family surface | N-A | no protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | agent workflow only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md`, `ai/rules/repo-maintenance.md` gate list |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on the deleted scripts |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- `plan/journal/` exists and the detector is reachable
   - Tests: `test_report_flags_second_occurrence`, `test_report_silent_on_singletons`
   - Files: `scripts/dev/journal.py`, `plan/journal/README.md`, `mk/inventory.mk`
   - Verify: `make ze-journal` runs and reports nothing on an empty directory
2. **Phase: Gates** -- the two artifact gates accept a journal row
   - Tests: `test_lesson_gate_accepts_journal_row`, `test_closure_accepts_journal_row`
   - Files: `commit_helper.py`, `spec-closure-check.py`
   - Verify: both accept a row, and still refuse an empty commit
3. **Phase: Seed** -- one class file per `###` pattern in `plan/learned/RECURRING-PATTERNS.md`, 28 of them, the class name being the kebab-case of the pattern heading and the seed rows being the occurrences that document already claims. The list is not copied into this spec: the document is the source, and a copy would drift
   - Files: `plan/journal/*.md`
   - Verify: `make ze-journal` prints the classes that already repeated, the largest being the two hook classes at 30 and 15 claimed occurrences
4. **Phase: Delete** -- the corpus goes, the three aggregates stay
   - Files: the deletion list above
   - Verify: `make ze-doc-test`, `make ze-rules-router-report` diffed against the pre-deletion always-on set

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The detector reads git HEAD, never the working tree (`plan/learned/1352`) |
| Naming | Class file names name the problem, not the subsystem: the corpus failed because titles named `plugin`, `config`, `ospf` |
| Data flow | No committed index anywhere in the design |
| Rule: `ai/rules/simplicity.md` | One script, one target, no new abstraction |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `make ze-journal` reports repeats | `make ze-journal` on the seeded directory |
| No dead learned citation remains | `make ze-doc-test` |
| Rule routing shift is known, not silent | `make ze-rules-router-report` diff recorded in the spec |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A journal file is repo-authored, not untrusted input. Malformed tables are named and refused, never skipped silently |

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

- The corpus could never detect recurrence because it recorded no countable axis.
  Titles cluster on subsystem (`plugin` 31, `config` 27, `ospf` 16), never on
  failure class.
- Of 591 searches issued against `plan/learned/`, the commonest patterns are
  `^## Files`, `^## Objective` and heading checks. The corpus mostly answered
  questions about its own format.
- The repo reached the same conclusion independently. The comment above
  `ROUTE_PREFIXES` in `scripts/dev/commit_helper.py` records: "Measured
  2026-08-03 over 903 summaries: 13 were referenced by a rule or a hook. The
  other 890 reached nothing that governs behaviour. A gate that demands a
  document produces an archive; a gate that demands a destination produces
  guidance." This spec finishes that move.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One file per class, table rows inside | One file per record with front matter; one shared JSONL | Row count IS the recurrence count. A shared single file cross-commits between parallel sessions (`ai/rules/git-safety.md`) |
| Directory listing is the vocabulary | A closed enum with a validator | Creating a file is already a deliberate act. A validator would be machinery the problem does not need |
| No committed index | Generate one like `ai/LEARNED-FULL-INDEX.md` | A working-tree regeneration publishes other sessions' unlanded work (`plan/learned/1352`), and the old index was opened 16 times in 1,272 contexts |
| Keep the three aggregates | Delete everything | They outread every shard: 30, 22, and the existing hand-written recurrence record. `commit_helper.py` `ROUTE_FILES` already names all three as canonical homes |
| A `Spec` column on every row | Key closure on commit B alone | `spec_closure_stem()` feeds `review_gate_problems()`; commit B carries no code, so keying on it alone stops the review gate covering the code |
| Delete `learned-next` rather than port it | Allocate journal numbers the same way | A class file is named by its class and appended to. Numbering exists only to keep two sessions from colliding on one filename, which sharding by class already prevents |

## Known Limitations
- The journal records that a class repeated. It does not explain why. The
  explanation stays in the fix commit.
- Rule routing loses 821 of its 1071 corpus entries. Measured, the always-on set
  does not move: both artifacts regenerate byte-identical from the 250 open specs
  alone. The residual risk is later, not here: as specs close the corpus shrinks
  toward zero, and an empty corpus turns derivation 4 off for its three members
  at once (`rules_condensed.unreachable_blocking()`).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional coverage: the detector runs inside `ze-doc-test`
- [ ] Interop tests N-A: no protocol surface

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Journal row written for anything this spec itself learned
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/spec-problem-journal.md` only

---

## Implementation Summary

### What Was Implemented

- `scripts/dev/journal.py` -- the reader and the detector. `journal_row_cells()`
  is the ONE row parser in the repo; `read_journal_at_head()` reads `git
  ls-tree`/`git show` at HEAD and returns the rows plus the class files the
  working tree holds that HEAD does not; `report()` prints every class with 2 or
  more rows, its count, and the span between the first and last date.
- `scripts/dev/journal_test.py` -- 16 tests, all green.
- `plan/journal/` -- `README.md` plus 28 class files seeded from the `###`
  patterns in `plan/learned/RECURRING-PATTERNS.md`.
- `mk/inventory.mk` -- the `ze-journal` target, and `journal.py` folded into the
  `ze-doc-test` recipe so a repeat prints without being asked for.
- `scripts/dev/commit_helper.py` -- `"plan/journal/"` added to `ROUTE_PREFIXES`,
  which is the whole lesson-gate change (`routed_paths()` already satisfies
  `lesson_comment()`); `spec_closure_stem()`, `closure_reminder()` and
  `spec_audit_problems()` derive the stem from a new row's `Spec` cell through
  `_journal_added_spec_stems()`; `_spec_closed_earlier()` drops a stem whose spec
  git once held and no longer holds; `journal_row_problems()` BLOCKS a commit
  that adds a row the parser cannot read; the `learned-next` subcommand is gone.
- `scripts/dev/spec-closure-check.py` -- `_journal_evidence()` maps
  `{spec-stem: journal-relpath}` from committed rows; `SpecReport.journal_match`
  counts as closure evidence only alongside the spec's own finished Review Gate;
  the file imports `journal_row_cells` and `MALFORMED` from `journal.py` rather
  than carrying a second parser.
- `scripts/dev/check_doc_links.py` -- check 3 deleted whole (`check_index_budget`,
  `index_entry_length`, `INDEX_FILE`, `INDEX_ENTRY`, `INDEX_SEPARATOR`,
  `POINTER_BUDGET`, `INDEX_BUDGET_BLOCKING`, its `main()` arm), and the
  `LEARNED_NUMBER` branch dropped from `path_resolves()`.
- `scripts/dev/rules_router.py` -- `main()` passes `plan/` alone to
  `load_corpus()`.
- 943 summaries deleted; `DESIGN-HISTORY.md`, `HOOK-FRICTION.md` and
  `RECURRING-PATTERNS.md` kept. `ai/LEARNED-INDEX.md`, `ai/LEARNED-FULL-INDEX.md`,
  `scripts/dev/learned_*.py` and every `ze-learned-*` target removed.
- The rationale of the 184 cited summaries moved to `docs/architecture/` (269
  tracked files). All 2668 `// Design:` pointers in tracked Go now name a file
  under `docs/architecture/`; `git grep "// Design: plan/learned/" -- "*.go"`
  returns nothing.

### Bugs Found/Fixed

- `ai/CODE-TO-DOCS.md` was stale at HEAD: the landed commit regenerated it and
  then added further `<!-- source: -->` anchors to `docs/architecture/core-design.md`
  in the same commit, so `code_to_docs.py --check` went red inside `ze-doc-test`
  (2012 paths recorded against 2015 real). Regenerated with `make ze-doc-index`;
  it is in Commit A. Covered by `ze-doc-test`, which now passes.
- `spec_audit_problems()` matched only `plan/learned/NNN-<stem>.md`, so with the
  corpus deleted the Pre-Commit Verification gate could not fire on ANY closure.
  It now also asks `_journal_added_spec_stems()`. (Review round 3.)
- `spec_closure_stem()` returned a months-old stem for a commit that only EDITED
  an existing journal row, refusing an ordinary typo fix in the name of a spec
  nobody was closing. `_spec_closed_earlier()` filters those stems.
  (Review round 4.)
- `closure_reminder()` was missing the same filter and nudged the caller to
  prepare a commit B for a spec git removed months ago. (Review round 5.)
- `read_journal_at_head()` keyed its "HEAD carries no journal" test on `names`,
  which counts `README.md`. A HEAD holding the README and no class file took the
  early exit and reported zero rows over a working tree full of classes -- the
  exact vacuous green the guard exists to refuse, and the README is committed one
  ordering step before the first class file. It keys on `paths` now.
- `report()` accepted a `-` in the Date cell. A seed of 52 rows carrying `-`
  everywhere passed every gate while losing the span, which is the only thing the
  report says beyond the count. An unparseable Date is now non-zero
  (`test_dash_date_is_an_error`).
- `journal_row_cells()` returned `None` for a malformed row, which a caller could
  not tell from prose and so skipped. It returns `[MALFORMED]`, and
  `spec-closure-check.py`'s private copy of the parser (which returned `None`)
  was deleted in favour of the import.

### Documentation Updates

- `ai/INDEX.md`: the discovery row, the "record a problem" row, and the
  `journal.py` tool row.
- `ai/rules/repo-maintenance.md`: the discovery-surface table row and the
  commit-time gate table row for `journal_row_problems`, plus the two point files
  those render from under `ai/rules/points/repo-maintenance/`.
- `ai/rules/planning.md`: "Writing Journal Rows" replaces "Writing Learned
  Summaries"; the closure checklist step and the Commit A `--file` list name
  `plan/journal/<class>.md`; the `spec-closure-check.py` detector paragraph
  states the journal-row rule.
- `ai/skills/ze-close.md`: step 6a is the journal row, step 6d `--file`s the class
  file.
- `ai/CODE-TO-DOCS.md`: regenerated (see Bugs Found/Fixed).
- `make ze-doc-test`: PASSED (`tmp/close-doctest2.log`).

### Deviations from Plan

- **The `// Design:` pointers were repointed, not deleted.** The spec's original
  Files to Delete said the pointers went with the corpus. The owner ruled on
  2026-08-09 that the rationale of the 184 cited summaries moves to
  `docs/architecture/` and every pointer references it there. That is recorded in
  the spec body and produced about 130 new architecture documents.
- **This spec has no deferral shard of its own.** The metadata named
  `plan/deferrals/problem-journal.md`, which was never created. Corrected at
  closure to name the three destination-owned shards that actually hold the rows.
  See the note under the metadata table.
- **`journal_row_problems()` was not in the spec.** It was added because
  `_journal_added_spec_stems()` derives the closure stem from the same rows: a
  row the parser cannot read yields no stem, and `review_gate_problems()` then
  returns `[]`, landing a closure commit carrying code with no independent
  review. The miss path returned the permissive verdict, which is the fail-open
  shape `ai/rules/evidence.md` forbids.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 assumed a Go file loses no knowledge when it loses its `// Design:` pointer, so the plan deleted the pointers with the corpus | The knowledge in the 184 cited summaries had no other home, and deleting the pointer would have stranded it | Owner ruling, 2026-08-09, while the deletion list was being executed | Repointed all 2668 pointers to about 130 new `docs/architecture/` documents instead of deleting them (Deviations, above) |
| approach | The generated `ai/CODE-TO-DOCS.md` was regenerated mid-commit and then invalidated by later doc edits in the same commit | A generated artifact is fresh only against the FINAL state of the commit that carries it | `make ze-doc-test` red at closure, `code_to_docs.py --check` | Regenerated with `make ze-doc-index`; Commit A carries it |
| escalation | Every doc-freshness gate in the repo checks that a reference RESOLVES and none checks that the CLAIM is true; 82 of 1611 anchors named an absent symbol, all green | Reference integrity and claim truth are different properties, and a resolving anchor lends a false sentence credibility | Sweeping 1611 anchors while moving the pointers, then an independent reviewer finding 7 of them by hand | Journal row in `plan/journal/reference-checked-claim-unchecked.md`; spec at `plan/spec-doc-claims-are-checked-not-just-resolved.md` |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Replace the corpus with a journal whose recurrence is COUNTABLE | Done | `scripts/dev/journal.py` `report()` | `make ze-journal` prints 22 classes with 2+ rows and a real date span each |
| One file per problem class, one table inside | Done | `plan/journal/` (28 class files + README) | `plan/journal/README.md` states the five-cell format |
| Delete the 943 shards | Done | `plan/learned/` holds only `DESIGN-HISTORY.md`, `HOOK-FRICTION.md`, `RECURRING-PATTERNS.md` | `ls plan/learned/` |
| The `Spec` column keeps the review gate firing on the commit that holds the code | Done | `scripts/dev/commit_helper.py` `spec_closure_stem()` feeding `review_gate_problems()` | `test_journal_row_is_a_closure_signal` |
| No committed index anywhere in the design | Done | `scripts/dev/journal.py` computes on demand from HEAD | no generated journal index exists; `ai/LEARNED-FULL-INDEX.md` deleted |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `scripts/dev/journal.py` `report()` | prints `class: N rows, Dd span (first .. last)`; `test_report_flags_second_occurrence` |
| AC-2 | Done | `scripts/dev/journal.py` `report()`, the `len(rows) < 2` continue | `test_report_silent_on_singletons` |
| AC-3 | Done | `scripts/dev/commit_helper.py` `ROUTE_PREFIXES` carrying `"plan/journal/"`, read by `routed_paths()` for `lesson_comment()` | `test_lesson_gate_accepts_journal_row` |
| AC-4 | Done | `scripts/dev/journal.py` `journal_row_cells()` returning `[MALFORMED]`, named by `report()` with exit 1 | `test_malformed_table_is_named` |
| AC-5 | Done | `plan/learned/` holds the three aggregates only; `make ze-doc-test` PASSED | `python3 scripts/dev/check_doc_links.py` -- "all corpus path references resolve" |
| AC-6 | Done | `scripts/dev/spec-closure-check.py` `_journal_evidence()` and `SpecReport.journal_match` | `TestSpecClosureAcceptsJournalRow` in `scripts/dev/spec_closure_check_test.go` |
| AC-7 | Done | `scripts/dev/commit_helper.py` `spec_closure_stem()` feeding `review_gate_problems()` | `test_journal_row_is_a_closure_signal`; this closure's own commit A is subject to it |
| AC-8 | Done | `scripts/dev/check_doc_links.py` `check_design_refs()` calling `path_resolves()` | 2668 `// Design: docs/architecture/` pointers, 0 `// Design: plan/learned/`, check exits 0 |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `test_report_flags_second_occurrence` | Done | `scripts/dev/journal_test.py` | AC-1 |
| `test_report_silent_on_singletons` | Done | `scripts/dev/journal_test.py` | AC-2 |
| `test_malformed_table_is_named` | Done | `scripts/dev/journal_test.py` | AC-4 |
| `test_lesson_gate_accepts_journal_row` | Done | `scripts/dev/commit_helper_test.py`, `TestLessonIsContentDriven` | AC-3 |
| `test_journal_row_is_a_closure_signal` | Done | `scripts/dev/commit_helper_test.py` | AC-7 |
| `TestSpecClosureAcceptsJournalRow` | Done | `scripts/dev/spec_closure_check_test.go` | AC-6 |
| `make ze-journal` inside `ze-doc-test` | Done | `mk/inventory.mk` | functional row |
| Added beyond the plan: `test_dash_date_is_an_error`, `test_head_with_only_readme_is_an_error`, `test_a_class_file_not_at_head_is_named`, `test_uncommitted_journal_is_an_error`, `test_git_failure_is_an_error` | Done | `scripts/dev/journal_test.py` | the review rounds' vacuous-green findings |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/journal.py` | Done | created, 273 lines |
| `scripts/dev/journal_test.py` | Done | created, 295 lines, 16 tests |
| `plan/journal/README.md` | Done | created, 34 lines |
| `scripts/dev/commit_helper.py` | Done | `ROUTE_PREFIXES`, `spec_closure_stem`, `closure_reminder`, `spec_audit_problems`, `_spec_closed_earlier`, `journal_row_problems`; `learned-next` deleted |
| `scripts/dev/spec-closure-check.py` | Done | `_journal_evidence`, `SpecReport.journal_match`, parser imported from `journal.py` |
| `scripts/dev/rules_router.py` | Done | `main()` passes `plan/` only |
| `scripts/dev/check_doc_links.py` | Done | check 3 deleted whole; `LEARNED_NUMBER` branch dropped |
| `scripts/dev/check_doc_links_test.py` | Done | the `INDEX_FILE` arm is gone from `test_real_corpus_is_present_and_clean` |
| `ai/INDEX.md`, `ai/rules/*.md`, `ai/rationale/git-safety.md` | Done | 0 `plan/learned/NNN` citations remain in any of them |
| `mk/inventory.mk` | Done | `ze-learned-*` removed, `ze-journal` added, `journal.py` folded into `ze-doc-test` |
| `ai/rules/planning.md`, `ai/skills/ze-close.md` | Done | closure artifact is a journal row |
| `plan/learned/[0-9]*.md` (943) | Done | deleted; three aggregates kept |
| `ai/LEARNED-INDEX.md`, `ai/LEARNED-FULL-INDEX.md`, `scripts/dev/learned_*.py` | Done | deleted |
| `// Design:` pointers | Changed | REPOINTED to `docs/architecture/`, not deleted (owner decision, see Deviations) |

### Audit Summary

- **Total items:** 31 (5 requirements, 8 ACs, 7 planned tests, 11 file groups)
- **Done:** 30
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the `// Design:` pointer treatment, owner decision, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Recurrence is countable, where the corpus counted nothing | functional | `make ze-journal` prints 22 classes with 2+ rows, each with a real date span, for example `env-var-double-registration: 2 rows, 20d span (2026-03-29 .. 2026-04-18)` and `test-against-broken-path: 3 rows, 8d span`. The corpus produced no such output at all |
| The 943 shards are deleted without losing a live reference | functional | `python3 scripts/dev/check_doc_links.py` gives "all corpus path references resolve", exit 0; `make ze-doc-test` PASSED; `git grep "// Design: plan/learned/" -- "*.go"` returns nothing while 2668 pointers resolve into `docs/architecture/` |
| The closure artifact gate survives the swap | functional | `TestSpecClosureAcceptsJournalRow` and `test_journal_row_is_a_closure_signal` pass; and THIS closure is the end-to-end proof: commit A adds a row naming `problem-journal`, `spec_closure_stem()` returns that stem, and `review_gate_problems()` demands the artifact that `review_gate.py check --spec problem-journal` reports clean |
| The detector cannot become a second write-only corpus (R-1) | functional | `journal.py` runs inside the `ze-doc-test` recipe in `mk/inventory.mk`, so a repeat prints on every doc-test run without being asked for. Confirmed in `tmp/close-doctest2.log` |
| The rule-routing shift is known, not silent (A-2) | functional | `make ze-rules-condensed` regenerated `ai/rules/TRIGGERS.md` (28 rules) and `ai/rules/CORE.md` (8 always-on rules) from the post-deletion corpus and `git status` shows no change: byte-identical |

Discrimination (`ai/rules/interop-and-goal-validation.md`): the journal detector
is not an absence assertion. `test_report_flags_second_occurrence` asserts a
printed line with a count and a span, `test_report_silent_on_singletons` asserts
the opposite on one-row input, and `test_dash_date_is_an_error` plus
`test_head_with_only_readme_is_an_error` exist because two earlier shapes DID
pass while checking nothing.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 133 `// Design: plan/spec-*.md` pointers in `_test.go` files name specs removed at closure; `go_files()` in `check_doc_links.py` excludes `_test.go` so no gate sees them (`plan/deferrals/fixit-dead-design-pointers-in-tests.md`) | deferred | Owned by `plan/spec-fixit-dead-design-pointers-in-tests.md`, which exists. Shard KEPT: the row is live |
| Every doc-freshness gate checks reference integrity and none checks claim truth; 82 of 1611 anchors named an absent symbol (`plan/deferrals/doc-claims-are-checked-not-just-resolved.md`) | deferred | Owned by `plan/spec-doc-claims-are-checked-not-just-resolved.md`, which exists. Shard KEPT: the row is live |
| Full audit of whether the rule corpus keeps design documentation current (`plan/deferrals/doc-claims-are-checked-not-just-resolved.md`) | deferred | Homed at that spec's research phase, per the owner's instruction that the sweep runs after the work in hand. Shard KEPT |
| 467 `exported symbol X has no cross-package non-test caller` findings from `make ze-validate` (`plan/deferrals/fixit-unexport-package-private-symbols.md`) | deferred | Owned by `plan/spec-fixit-unexport-package-private-symbols.md`, which exists. Shard KEPT: the row is live. This is also the attribution on this closure's `--unverified` |

No shard is removed by this closure: every row above is live and homed at a spec
that exists, so all three shards outlive this spec (`ai/rules/planning.md`, and
`deferral_shard_removal_problems` would block the removal). No FOREIGN shard was
emptied by these resolutions.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/problem-journal-25bc4cd0-5849-463d-914a-dbc78053445c.md` |
| `review_gate.py check` | clean -- `review_gate: OK (0 code files, clean, hashes match ...)` |
| Rounds | 5, verdict `clean`. The artifact's `rounds-reason` prices the rounds past three: "round 3 found a dead closure gate (spec_audit_problems matched only plan/learned, so Pre-Commit Verification stopped firing on every closure) plus two fail-open guards; round 4 found spec_closure_stem returning a months-old stem for an edit-only journal commit; round 5 found that filter missing from closure_reminder" |
| Reviewer lenses used | 5 independent `ze-read` subagents, one per round, on `claude-opus-5`: gate correctness and fail-open shapes; closure-signal derivation; deletion blast radius over the 943 shards and the 1155 pointers; doc-anchor truth sampling; final sweep |

Per-round severity, as carried into this closure: round 1 five BLOCKERs, round 2
six BLOCKERs, round 3 one BLOCKER and two ISSUEs, round 4 one ISSUE, round 5 zero
product defects in the code. The artifact's `## Findings` section records
`(none recorded)`, so the per-finding detail for rounds 1 and 2 is not in the
machine record. The product defects the record DOES name are transcribed below,
and each was re-verified against source at closure.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The Pre-Commit Verification gate matched only `plan/learned/NNN-<stem>.md`, so with the corpus deleted it could not fire on ANY closure | `scripts/dev/commit_helper.py` `spec_audit_problems()` | Also asks `_journal_added_spec_stems()`; the docstring records why the two call sites read it differently |
| 2 | ISSUE | `spec_closure_stem()` returned a months-old stem for a commit that only EDITED an existing journal row, refusing an ordinary typo fix | `scripts/dev/commit_helper.py` `spec_closure_stem()` | `_spec_closed_earlier()` filters those stems; both halves of its test (gone from disk AND git once held it) are load-bearing |
| 3 | ISSUE | `closure_reminder()` lacked the same filter and nudged for a commit B on a spec removed months ago | `scripts/dev/commit_helper.py` `closure_reminder()` | Same filter applied to its journal arm |
| 4 | BLOCKER | `read_journal_at_head()` keyed its emptiness test on `names`, which counts `README.md`; a HEAD holding the README alone reported zero rows over a full working tree | `scripts/dev/journal.py` `read_journal_at_head()` | Keys on `paths`; `test_head_with_only_readme_is_an_error` |
| 5 | BLOCKER | An unparseable Date was silently tolerated; a seed of 52 rows with `-` in every Date cell passed every gate while losing the span | `scripts/dev/journal.py` `report()` | Returns 1 and names the cells; `test_dash_date_is_an_error` |
| 6 | BLOCKER | A malformed row returned `None`, indistinguishable from prose, so callers skipped it; `spec-closure-check.py` carried a second copy of the parser with the same defect | `scripts/dev/journal.py` `journal_row_cells()`, `scripts/dev/spec-closure-check.py` | Returns `[MALFORMED]`; the private copy deleted in favour of the import; `journal_row_problems()` BLOCKS a malformed added row |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/journal.py` | Yes | `wc -l` gives 273 |
| `scripts/dev/journal_test.py` | Yes | `wc -l` gives 295 |
| `plan/journal/README.md` | Yes | `wc -l` gives 34 |
| `plan/journal/*.md` class files | Yes | `find plan/journal -name '*.md' -not -name 'README.md' \| wc -l` gives 28, 29 after this closure's new class |
| `plan/journal/reference-checked-claim-unchecked.md` | Yes | written by this closure, one row, Spec cell `problem-journal` |
| `plan/learned/` kept aggregates | Yes | `ls plan/learned/` gives `DESIGN-HISTORY.md`, `HOOK-FRICTION.md`, `RECURRING-PATTERNS.md` |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 2+ rows prints class, count, span | `make ze-journal` gives 22 lines, for example `test-against-broken-path: 3 rows, 8d span (2026-03-21 .. 2026-03-29)`, exit 0 |
| AC-2 | singletons print nothing | `python3 scripts/dev/journal_test.py` gives `Ran 16 tests ... OK`, including `test_report_silent_on_singletons` |
| AC-3 | a journal row satisfies the lesson gate | `python3 -m pytest scripts/dev/commit_helper_test.py -k "journal or lesson or review_gate"` gives 33 passed |
| AC-4 | a malformed table is named, non-zero | `test_malformed_table_is_named` in the 16-test green run |
| AC-5 | corpus deleted, no dead citation | `python3 scripts/dev/check_doc_links.py` gives "all corpus path references resolve", exit 0; `make ze-doc-test` gives "Documentation tests PASSED" |
| AC-6 | closure check accepts a staged journal row | `go test ./scripts/dev/ -run TestSpecClosure -count=1` gives `ok`, 12 tests including `TestSpecClosureAcceptsJournalRow` |
| AC-7 | the review gate still blocks | `test_journal_row_is_a_closure_signal` green; and `commit_helper.py create` for this closure runs `review_gate.py check --spec problem-journal`, which reports OK and clean |
| AC-8 | every `// Design:` pointer names an existing `docs/architecture/` file | `git grep -h "// Design: docs/architecture/" -- "*.go" \| wc -l` gives 2668; `git grep "// Design: plan/learned/" -- "*.go"` gives nothing; `check_doc_links.py` exits 0 |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-journal` reaching `journal.py` `report()` | `scripts/dev/journal_test.py` `test_report_flags_second_occurrence` | Yes -- the `ze-journal` recipe in `mk/inventory.mk` runs `python3 scripts/dev/journal.py`, and the same script is run inside the `ze-doc-test` recipe; `tmp/close-doctest2.log` shows the stage banner "Problem journal (classes with 2+ rows)" followed by the 22 class lines |
| `commit_helper.py create` on a rules change reaching `lesson_comment()` | `scripts/dev/commit_helper_test.py` `test_lesson_gate_accepts_journal_row` | Yes -- read the test: it stages a `plan/journal/` path and asserts the gate accepts it with no `--lesson-not-needed`. This closure's own commit A exercises the same path |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | 943 summaries collapsed to 28 class files, 29 with this closure's new class. A directory listing of 29 entries is a usable vocabulary, and `make ze-journal` groups them into 22 with countable recurrence. The 30-day recount the spec asked for re-measures a confirmed result; it is not an open validation |
| A-2 | confirmed | `make ze-rules-condensed` regenerated `ai/rules/TRIGGERS.md` (28 rules, 4655 chars) and `ai/rules/CORE.md` (8 rules, 49207 chars) from the post-deletion corpus of open specs alone; `git status` reports no modification to either file, so both are byte-identical to the pre-deletion artifacts. The always-on set is the same 8 rules |
| A-3 | broken | Deleting a pointer strands the knowledge it named. The owner ruled on 2026-08-09 that the 184 cited summaries' rationale moves to `docs/architecture/` and all 2668 pointers reference it there. Recorded in Deviations and the Mistake Log. What that fix left behind is the reason for `plan/spec-doc-claims-are-checked-not-just-resolved.md`: the pointers now RESOLVE, and 82 of 1611 source anchors still name a symbol their file does not hold |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #15 registered inventory changed, `ai/INDEX.md` | its `journal.py` row describes reading `plan/journal/*.md` at git HEAD and being folded into `make ze-doc-test`; both halves checked against `scripts/dev/journal.py` `read_journal_at_head()` and the `ze-doc-test` recipe in `mk/inventory.mk` | Yes |
| #15 gate list, `ai/rules/repo-maintenance.md` | its `journal-row` row describes `journal_row_problems` as BLOCK on a row lacking the five cells; checked against `scripts/dev/commit_helper.py` `journal_row_problems()` | Yes |
| #16 changed files behind existing source anchors | `make ze-doc-index` regenerated `ai/CODE-TO-DOCS.md` (2012 to 2015 paths) and `code_to_docs.py --check` now passes inside `ze-doc-test` | Yes |
| #1-#14 and #17 answered No | `make ze-doc-test` PASSED with `doc_drift.go`, `commands.go` (YANG/handler contract) and `digest_check.py` (3037 anchors across 23 digests) all green, which is the machine form of "no user-facing, config, CLI, API, plugin, wire, RFC or example surface moved" | Yes |

## Core Insight

A gate that verifies a reference RESOLVES is not a gate that verifies the CLAIM
is true, and the resolving reference makes the false claim harder to doubt. The
corpus this spec deleted had the same shape one level up: it demanded a
DOCUMENT and never asked whether anybody read it. Measured, 890 of 903 summaries
reached nothing that governs behaviour. The journal applies to knowledge the
correction the pending doc-claims spec applies to anchors: count the thing you
actually care about, or the green bar is about something else.
