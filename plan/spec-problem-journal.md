# Spec: problem-journal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 4/4 |
| Deferral shard | `plan/deferrals/problem-journal.md` |
| Updated | 2026-08-09 |

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
