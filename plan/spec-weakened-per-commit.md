# Spec: weakened-per-commit

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Move the justification for a weakened test out of the test file into ONE file
that the commit must have accepted, then retire the ceiling and the census that
existed to cap a pile which can no longer form.

**The root cause was STORAGE, not detection.** A justification explains one diff.
It is written at edit time and read at review time, and once the change is
committed it compares against nothing a reader of the file can see. Storing an
ephemeral record permanently is what let 601 tokens and 2,660 lines of prose
accumulate across 413 test files, at which point nobody could read them, so
nobody did, so writing one cost nothing.

Owner decisions (Thomas, 2026-08-16), settled before this spec opened:

| # | Decision |
|---|----------|
| 1 | ONE file holds every weakening the commit in hand needs accepted |
| 2 | `scripts/dev/commit_helper.py` calls the checker |
| 3 | The hook stays: it makes sure the justification is generated and checked at edit time, and `commit_helper.py` checks it again |
| 4 | The ceiling retires. It makes no sense once the check is per commit |
| 5 | A missing entry REFUSES the commit: BLOCK tier, not WARN |
| 6 | An entry carries the NAME of the test weakened and the reason it is accepted. Nothing else |

**One word for one thing.** The code already says weakening (`c_test_weakening`,
`_test_weakening_errs`) and only the token and the ceiling said relax. This spec
retires the second word.

| Slot | Was | Becomes |
|------|-----|---------|
| The file | `test/relax-ceiling.txt` | `test/weakened.md` |
| Hook check | `c_test_weakening` | unchanged |
| Commit gate | - | `weakened_problems` in `scripts/dev/commit_helper.py` |
| Checker | `scripts/dev/relax-census.py` | `scripts/dev/check_weakened_tests.py` |
| Make target | `ze-relax-census` | `ze-weakened-check` |

**A per-commit file cannot grow, so selectivity stops being needed.**
`c0627a044` made the six count-based arms of `_test_weakening_errs` report rather
than refuse, because each refusal cost a permanent comment. With this file the
record is free, so every weakening kind is recorded again (owner, 2026-08-16).

### What this supersedes

`spec-fixit-relax-ceiling-raise-is-unreachable` existed to make a ceiling
raise enforceable at commit time. Retiring the ceiling voids it, and the owner
directed it be DELETED outright rather than cancelled (2026-08-16). Deleting it
is in scope here, so no later session rebuilds the mechanism this removes.

## Required Reading

### Architecture Docs
- [ ] `TEST-RELAX-AUDIT.md` -- the 2026-08-10 audit and its
      "What this says about the mechanism" section
  → Decision: that section already named the two missing properties, "it must
     expire" and "it must be rare". A per-commit file delivers both as STRUCTURE
     rather than discipline: it cannot accumulate, so it cannot become unreadable.
- [ ] `test/relax-ceiling.txt` -- the whole file, prose included
  → Constraint: the census counts HEAD, never the working tree, because a shared
     checkout's tree moves under whoever reads it (751 → 755 within an hour on
     2026-08-10, three unrelated sessions). The per-commit check MUST keep that
     property: it judges the paths the commit NAMES, never the tree.
- [ ] `ai/rules/testing.md` -- the weakening rule and its refused/reported split
  → Constraint: the rule forbids every weakening; what the hook REFUSES is a
     narrower set. Retiring the ceiling MUST NOT be written up as narrowing the
     rule itself.
- [ ] `ai/rules/git-safety.md` -- what `commit_helper.py` already gates on
  → Constraint: a BLOCK gate MUST NOT fire on another session's files.
     `deferral_unassigned_problems` was demoted to warn for exactly that. Keying
     on the commit's own `--file` list avoids the failure by construction, which
     is what makes decision 5 safe.
- [ ] `ai/rules/no-layering.md` -- delete X before implementing Y
  → Decision: one spec, so the swap is atomic within it. The implementation order
     still retires the census before the new gate arms, so the two never both gate.

### Source Files
- [ ] `scripts/dev/commit_helper.py` -- `commit_gate_problems`, `commit_gate_warnings`
  → Decision: `commit_gate_problems(repo, add_paths, remove_paths)` aggregates
     every BLOCK-severity commit gate and already receives the commit's own add
     and remove path tuples. `weakened_problems` is one more `problems +=` line
     there and needs no new plumbing.
- [ ] `.claude/hooks/pretool-writeedit.py` -- `c_test_weakening`
  → Constraint: the hatch opens on `_writes_new_relax_reason(old, new, fp)`,
     which reads the justification THIS EDIT WRITES, so a token already in the
     file buys nothing. That per-weakening property MUST carry over: an entry has
     to name the test it excuses, or the hatch becomes per-file and permanent.
  → Constraint: `c_test_weakening` returns `(code, message)` or `None`, and the
     dispatcher takes `max(worst, code)`. Code 0 with a message is a notice that
     leaves the verdict alone, which is how the count arms report today.
- [ ] `scripts/dev/rfc_tagged_scope.py` -- `go_func_units`, `func_name_in`,
      `scope_reader`
  → Decision: attributing a weakening to its enclosing test BY NAME is already
     solved here. `go_func_units(content)` returns `[(name, text)]` per top-level
     func; `scope_reader` gives Go func spans and the whole file for every other
     carrier, so a `.ci`/`.et` file IS one test and its stem is the name. The
     checker MUST reuse this module: its own docstring records that a second copy
     which drifted would make two gates disagree about which text a rule covers.
- [ ] `scripts/status/verify_run.go` -- `stagesForMode`
  → Constraint: `ze-relax-census` is a stage in BOTH modes, and the list is locked
     against a golden by `TestStagesForModeMatchesGolden`, so swapping the stage
     means updating that golden in the same change.
- [ ] `Makefile` -- the `ze-relax-census` target and its comment block
  → Constraint: the target runs `--selftest` before the real check, so a broken
     counter cannot report a clean tree. `ze-weakened-check` MUST keep that shape.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `.claude/hooks/pretool-writeedit.py` - `c_test_weakening` refuses an edit
      that weakens a test unless the edit itself writes a `test-relax:`
      justification; `_test_weakening_errs` returns blocking and advisory lists
- [ ] `scripts/dev/commit_helper.py` - `commit_gate_problems` aggregates the
      BLOCK-severity commit gates; `create()` calls it with the commit's paths
- [ ] `scripts/dev/relax-census.py` - counts tokens at HEAD against the ceiling;
      `--lower` never raises; `classify` buckets each reason
- [ ] `scripts/dev/rfc_tagged_scope.py` - the single definition of an enclosing
      unit: `go_func_scopes`, `func_name_in`, `go_func_units`, `scope_reader`
- [ ] `scripts/status/verify_run.go` - `stagesForMode` lists `ze-relax-census`
      in both verify modes
- [ ] `Makefile` - the `ze-relax-census` target runs selftest then check
- [ ] `test/relax-ceiling.txt` - one integer plus prose, raised only with a
      `raised-for:` line

**Behavior to preserve:** (unless the user explicitly said to change it)
- The hook still refuses a weakening that carries no justification. Only WHERE
  the justification lives changes.
- The justification is still per-weakening, never per-file.
- Judgement is keyed on paths the commit names, never on the working tree.
- The seven one-directional arms still refuse; `ai/rules/testing.md` still
  forbids every weakening.
- A checker still selftests before judging the real tree.

**Behavior to change:** (only what the user asked for)
- The justification moves from a comment in the test file to `test/weakened.md`.
- That file is replaced per commit; it never accumulates.
- `commit_helper.py` refuses a commit whose weakenings the file does not cover.
- The ceiling, the census, and `test/relax-ceiling.txt` retire.
- Every weakening kind is recorded again, count-based ones included.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An agent edits a test file (`Edit`/`Write`/`MultiEdit`), which the PreToolUse
  hook sees as old text, new text and a path.
- An agent prepares a commit, which `commit_helper.py create` sees as a `--file`
  list and a `--remove` list.

### Transformation Path
1. Edit time: `c_test_weakening` computes the weakenings of this edit.
2. Edit time: the enclosing test's NAME is resolved through
   `rfc_tagged_scope.go_func_units`, or the file stem for `.ci`/`.et`.
3. Edit time: the hatch opens when `test/weakened.md` names that test.
4. Commit time: `weakened_problems` recomputes the weakenings of every test path
   in the commit, against HEAD, and resolves each to a test name the same way.
5. Commit time: every weakened test must have a row, and every row must name a
   test the commit actually weakens.
6. The file is committed with the change, so history holds it.
7. The next weakening replaces the file.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hook ↔ `test/weakened.md` | the hook reads a tracked file it does not own | Yes: `_weakened_rows` reads it under `PROJECT_DIR` at call time, and `weakened-row-opens-the-hatch` drives it from a fixture tree |
| commit_helper ↔ hook detector | `load_detector` imports `_test_weakening_errs` so both judge alike | Yes: `test_the_detector_is_the_hook_own_function` asserts `detector.__name__ == "_test_weakening_errs"` |
| Both ↔ `rfc_tagged_scope` | one definition of the enclosing unit, shared, never copied | Yes: `test_the_module_is_loaded_from_scripts_dev` pins the module path, `test_the_enclosing_test_name_comes_from_rfc_tagged_scope` pins that the reported name is the one it returned |

### Integration Points
- `commit_gate_problems` (`scripts/dev/commit_helper.py`) - one more `problems +=`
- `load_detector` (`scripts/dev/audit-test-relaxation.py`) - already shares the
  hook's detector with the commit-time path
- `stagesForMode` (`scripts/status/verify_run.go`) - swaps the stage

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Both gates enter through `check_weakened_tests.weakened_problems`; neither reads `test/weakened.md` itself except through `parse_weakened_file`, and neither spells its own weakening rule |
| No unintended coupling (components stay isolated) | Yes | The hook does not import `commit_helper`, and `commit_helper` does not import the hook: both reach `_test_weakening_errs` through `load_detector` (`scripts/dev/audit-test-relaxation.py`). The hook passes its own detector in as `detector=`, which is what stops the module re-executing the hook |
| No duplicated functionality (extends existing, does not recreate) | Yes | `go_func_units` / `scope_reader` are reused, not copied (`test_the_module_is_loaded_from_scripts_dev`); `weakened_problems` is one `problems +=` line in the existing `commit_gate_problems` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Dev tooling reading files at commit time; no wire or hot path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No runtime component. The commit gate follows the existing aggregation shape in `commit_gate_problems` rather than adding a branch anywhere else |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An agent can write `test/weakened.md` BEFORE the weakening edit | the hook fires on the edit and cannot see the future | the hatch never opens and every weakening is refused | a fixture driving file-then-edit and edit-then-file | confirmed: `weakened-missing-row-refuses-the-edit` and `weakened-row-opens-the-hatch` in `scripts/dev/hook-fixture-check.py` are the SAME edit against two states of the file, and only the row changes the verdict. `weakened-write-overwrite-needs-a-row` proves it for the Write carrier |
| A-2 | A test NAME identifies the weakening unambiguously | owner decision 6 | two same-named tests in one commit cannot be told apart | `TestNoGoFileBuildsMarkup` exists in `internal/component/lg` AND `internal/component/web`; a fixture weakening both | confirmed, with the `package.TestName` qualifier AC-7 requires: `TestAmbiguousName` in `scripts/dev/check_weakened_tests_test.py` drives all four cases (bare over two refused, both qualified accepted, bare over one accepted, wrong qualifier refused) |
| A-3 | The commit's `--file` list names every test file it weakens | `commit_helper.py` requires explicit `--file` | a weakening rides in unchecked | a fixture committing a weakened test with no row | confirmed: `render_git_add` stages only the declared paths and `render_staging_guard` aborts when the index holds any other, so a weakened file absent from `--file` is absent from the commit. `test_a_weakening_with_no_row_is_refused` and `test_a_weakening_the_commit_does_not_name_is_invisible` (`scripts/dev/commit_helper_test.py`) pin both halves |
| A-4 | Commit-time recomputation against HEAD agrees with what the hook saw | both import `_test_weakening_errs` via `load_detector` | the gate refuses a commit the hook allowed | a fixture asserting both agree on one edit | confirmed, and by identity rather than by comparison: `test_the_detector_is_the_hook_own_function` asserts the loaded detector IS `_test_weakening_errs`, and `_edited_file_pair` reconstructs the whole file so the hook names the unit from the same text the gate compares. The Write carrier makes the two inputs literally equal, and `weakened-write-overwrite-needs-a-row` / `weakened-write-overwrite-row-opens-the-hatch` drive that pair |
| A-5 | Retiring the census breaks no other caller | it is a verify stage in both modes and a `scripts/dev` python test | `ze-verify` fails on a missing target | grep the target, then `make ze-verify-list` | confirmed: `git grep` for `relax-census` / `relax-ceiling` finds no live caller (only two `plan/journal/` rows recording the 2026-08-10 history, each carrying a `doc-links: ignore` marker), and `make ze-verify-list` names `ze-weakened-check` at stage 12 and no census stage |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Ordering: the agent edits first, is refused, and must undo to proceed | the hook's refusal is the first thing the agent sees | the message names `test/weakened.md` and the exact row, so recovery is one Write and a retry |
| R-2 | A stale row from the previous commit silently authorises the next one | a commit passes carrying a row for a test it does not touch | the check refuses a row naming a test this commit does not weaken |
| R-3 | Two sessions weaken a test at once and overwrite each other's file | one session's row vanishes and its commit is refused | the check is keyed on the commit's own paths, so a foreign row is refused rather than silently accepted |
| R-4 | Same-named tests in different packages collide (A-2) | a row matches two weakened tests in one commit | refuse with a message asking for `package.TestName`; accept the bare name when it resolves to exactly one |
| R-5 | Retiring the ceiling loses the only count anybody had | nothing reports the corpus size afterwards | the corpus is 27 and falling, and the new shape makes accumulation impossible, which is what the count existed to detect |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing an operator runs. The blast radius is agent workflow: either a weakening lands unjustified, or a legitimate edit is refused. |
| How is it reverted? | Single commit revert. The new file and the retired ceiling both come back with it. |
| Who else touches this path? | Any session editing a test, and every session that commits. `hook-fixture-check.py` and `hook-parity-check.py` pin the behaviour. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An `Edit` weakening a test with no row | → | `c_test_weakening` | `weakened-missing-row-refuses-the-edit` |
| An `Edit` weakening a test named in the file | → | `c_test_weakening` | `weakened-row-opens-the-hatch` |
| A row naming a different test | → | `c_test_weakening` | `weakened-row-for-another-test-buys-nothing` |
| `commit_helper.py create` naming a weakened test | → | `weakened_problems` | `test_commit_without_a_row_is_refused` |
| `commit_helper.py create` with a stale row | → | `weakened_problems` | `test_a_row_for_an_untouched_test_is_refused` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A test edit weakens a test and `test/weakened.md` names that test with a reason | the hook allows the edit |
| AC-2 | A test edit weakens a test and the file does not name it | the hook refuses, and the message names the file and the exact row to write |
| AC-3 | A commit names a test file whose HEAD-to-tree diff weakens a test, with no row | `commit_helper.py create` refuses |
| AC-4 | A commit carries a row naming a test the commit does not weaken | `commit_helper.py create` refuses |
| AC-5 | A commit weakens nothing | the file is not required and its content is ignored |
| AC-6 | Any weakening kind occurs, count-based included | it is recorded and none is silently dropped |
| AC-7 | A row names a test whose name exists in two packages, both weakened | the checker refuses and asks for `package.TestName` |
| AC-8 | `test/relax-ceiling.txt` and `make ze-relax-census` are gone, `make ze-weakened-check` exists | `make ze-verify-list` names the new stage and not the old |
| AC-9 | `spec-fixit-relax-ceiling-raise-is-unreachable` | is deleted from `plan/` |
| AC-10 | A commit weakens a test, the worktree `test/weakened.md` has the row, and the commit does NOT carry `test/weakened.md` | `commit_helper.py create` refuses. Found by phase 4: without this the row can be written, left unstaged, and the weakening lands with no record in history, which is the one thing the design exists to guarantee (owner: "the information is there in the commit") |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_commit_without_a_row_is_refused` | `scripts/dev/check_weakened_tests_test.py` | AC-3 | PASS |
| `test_a_row_for_an_untouched_test_is_refused` | `scripts/dev/check_weakened_tests_test.py` | AC-4 | PASS |
| `test_a_commit_that_weakens_nothing_ignores_the_file` | `scripts/dev/check_weakened_tests_test.py` | AC-5 | PASS |
| `test_every_weakening_kind_reaches_the_check` | `scripts/dev/check_weakened_tests_test.py` | AC-6 | PASS |
| `test_an_ambiguous_test_name_is_refused` | `scripts/dev/check_weakened_tests_test.py` | AC-7 | PASS |
| `test_the_enclosing_test_name_comes_from_rfc_tagged_scope` | `scripts/dev/check_weakened_tests_test.py` | the shared-definition constraint | PASS |
| `test_a_weakening_with_no_row_is_refused` | `scripts/dev/commit_helper_test.py` | AC-3 at the commit entry point | PASS |
| `test_a_row_for_a_test_this_commit_does_not_weaken_is_refused` | `scripts/dev/commit_helper_test.py` | AC-4 at the commit entry point | PASS |
| `test_create_itself_refuses_and_names_the_row_to_write` | `scripts/dev/commit_helper_test.py` | `create` exits 2 and gives the row | PASS |
| `test_the_row_must_be_in_the_commit_not_only_in_the_tree` | `scripts/dev/commit_helper_test.py` | AC-10 | PASS |
| `test_the_row_is_what_lets_the_commit_through` | `scripts/dev/commit_helper_test.py` | AC-10's discriminator: the same commit passes when it carries the file | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rows per commit | 0..n | n | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `weakened-missing-row-refuses-the-edit` | `scripts/dev/hook-fixture-check.py` | an agent weakens a test without writing the file first and is refused | PASS |
| `weakened-row-opens-the-hatch` | `scripts/dev/hook-fixture-check.py` | an agent writes the row, then weakens the test, and the edit lands | PASS |
| `weakened-row-for-another-test-buys-nothing` | `scripts/dev/hook-fixture-check.py` | a row naming a different test does not open the hatch | PASS |

## Files to Modify
- `.claude/hooks/pretool-writeedit.py` - the hatch reads `test/weakened.md`
- `scripts/dev/commit_helper.py` - `weakened_problems`, wired into `commit_gate_problems`
- `scripts/dev/audit-test-relaxation.py` - read the file rather than in-file tokens.
  NOT DONE, homed in `plan/spec-weakened-followups.md` item A at the owner's
  direction, 2026-08-16, when the weekly token budget reached 3% on day 3. The
  audit still scans for the retired token, so it shows a reviewer an accepted
  weakening as an unexplained one. `ai/skills/ze-review.md` and
  `ai/skills/ze-review-deep.md` both disclose that, so the gap is stated rather
  than silent
- `scripts/dev/hook-fixture-check.py` - the weakening fixtures
- `scripts/dev/hook-parity-check.py` - `WEAKEN_GOLDEN`
- `scripts/status/verify_run.go` - swap the stage in both modes, and its golden
- `Makefile` - `ze-relax-census` becomes `ze-weakened-check`
- `ai/rules/points/testing/test-deletion-and-weakening/` - the points describing the hatch
- `ai/INDEX.md` - the `make ze-relax-census` row
- `docs/functional-tests.md`, `docs/contributing/testing.md` - the route a test author follows
- `docs/architecture/testing/test-health.md` - the block at :108-113 states
  "Enforced by `make ze-relax-census`", "Reads `test/relax-ceiling.txt`" and
  "Source `scripts/dev/relax-census.py`", all three now false. Found by phase 1,
  absent from this list when the spec was written
- `ai/rules/points/testing/make-targets/what-each-verification-target-runs.md` -
  the `make ze-relax-census` row at :25. Same find; re-render after editing
- `TEST-RELAX-AUDIT.md` - record that the mechanism moved

## Files to Create
- `test/weakened.md` - the file, with prose stating that it is replaced per commit
  and never accumulates, and a two-column table of test name and reason
- `scripts/dev/check_weakened_tests.py` - the checker
- `scripts/dev/check_weakened_tests_test.py` - its unit tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | agent tooling; nothing reaches the daemon or its config tree |
| YANG validation constraints | N-A | no YANG leaf added |
| YANG custom validators | N-A | no YANG leaf added |
| CLI commands/flags | N-A | no `ze` verb; `commit_helper.py` is a dev script |
| CLI grammar (keyword before value) | N-A | no CLI surface |
| Editor autocomplete | N-A | no YANG leaf added |
| Functional test for new RPC/API | N-A | no RPC or API; the hook's functional harness is `scripts/dev/hook-fixture-check.py` and three fixtures are named above |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | no `ze.*` env var; the path is a constant shared by the hook and the checker |
| Doctor check for runtime dependencies | N-A | no runtime dependency: `test/weakened.md` is a tracked repo file read by dev tooling, never by a running daemon |
| Prometheus counters/metrics | N-A | no daemon-observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | agent-facing tooling only |
| 2 | Config syntax changed? | No | no config surface |
| 3 | CLI command added/changed? | No | no `ze` verb |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | the audience is agents, served by `ai/rules/testing.md` |
| 7 | Wire format changed? | No | not protocol work |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | not protocol work |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- the route a test author follows changes |
| 11 | Affects daemon comparison? | No | nothing an operator sees |
| 12 | Internal architecture changed? | No | no runtime architecture touched |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` swaps the `make ze-relax-census` row for `ze-weakened-check` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors naming `relax-census.py`, `commit_helper.py`, `pretool-writeedit.py` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/testing.md` and `ai/rules/testing.md` show the `test-relax:` form and must show the file instead |

## Implementation Steps

1. **Phase: retire** -- delete `test/relax-ceiling.txt` and
   `scripts/dev/relax-census.py`, drop the `ze-relax-census` target and its
   `stagesForMode` stage plus golden, and the `ai/INDEX.md` row. Nothing gates on
   weakening in this window, which is deliberate and short (`ai/rules/no-layering.md`).
2. **Phase: file** -- create `test/weakened.md` with its format and prose.
3. **Phase: checker** -- write `scripts/dev/check_weakened_tests.py` reusing
   `rfc_tagged_scope`, with its unit tests, and the `ze-weakened-check` target.
4. **Phase: commit gate** -- add `weakened_problems` and wire it into
   `commit_gate_problems`.
5. **Phase: hook** -- point the hatch at the file, with its fixtures.
6. **Phase: docs** -- rule points, `TEST-RELAX-AUDIT.md`, `ai/INDEX.md`, and
   delete the superseded spec.

## Design Insights

The token was a good idea stored in the wrong place. Forcing a written reason is
right; keeping it next to the code forever is what destroyed its value.

## Key Design Decisions

| Decision | Why |
|----------|-----|
| One file, replaced per commit | It cannot accumulate, so it cannot become unreadable. Git history holds every past entry. |
| An entry is a test name and a reason | Owner decision 6. The name is what a reader needs; the path is recoverable from it. |
| The enclosing name comes from `rfc_tagged_scope` | That module is already the single definition of a unit, and its docstring records why a second copy is dangerous. |
| The hook still refuses | The justification must exist before the edit lands, or it is written to satisfy a gate rather than to inform a reader. |
| The commit gate recomputes rather than trusting the hook | A hook can be disabled; the commit path cannot be. |
| Ceiling retires | A cap on a pile that cannot form measures nothing. |

## Known Limitations

The ordering requirement (A-1, R-1) is a real workflow change: the row is written
before the edit it excuses. The hook's message has to carry the recovery, or
every agent meets this once and guesses.

A bare test name is ambiguous across packages (A-2, R-4). The design accepts it
when it resolves to exactly one weakened test in the commit and refuses otherwise.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
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

---

## Implementation Summary

### What Was Implemented
- `test/weakened.md`: the record, one row of test name and reason per weakening,
  replaced per commit. Its prose states the shape, the naming rule, and why the
  file never accumulates.
- `scripts/dev/check_weakened_tests.py`: the one checker both gates call. It owns
  the table reader (`parse_weakened_file`) and the pairing rule
  (`row_matches`, `unmatched_problems`), and borrows the two judgements it must
  not respell: `_test_weakening_errs` through `load_detector`, and
  `rfc_tagged_scope.go_func_units` / `scope_reader` for the unit name.
- `weakened_problems` (`scripts/dev/commit_helper.py`), wired into
  `commit_gate_problems`: BLOCK tier, keyed on the commit's own `--file` and
  `--remove` lists, and it also refuses a commit that weakens a test without
  naming `test/weakened.md` (AC-10).
- `c_test_weakening` (`.claude/hooks/pretool-writeedit.py`): the hatch now reads
  the file. `_edited_file_pair` applies the edit to the file on disk first, so the
  hook names the enclosing func rather than the file, which is what the commit
  gate will ask for.
- `make ze-weakened-check`, a stage of `ze-verify` in both modes, selftest first.
- Retired: `test/relax-ceiling.txt`, `scripts/dev/relax-census.py`,
  `scripts/dev/relax_census_test.py`, `make ze-relax-census`, and
  `spec-fixit-relax-ceiling-raise-is-unreachable`.

### Bugs Found/Fixed
- The hook and the commit gate would have named different units for one
  weakening, because an Edit hunk usually carries no `func` line. Fixed by
  `_edited_file_pair`; a mutation that names from the hunk turns four fixtures
  red.
- AC-10 did not exist when the spec was written. Phase 4 found that a row could be
  written, left unstaged, and the weakening would land in history with no reason
  beside it. `test_the_row_must_be_in_the_commit_not_only_in_the_tree` covers it,
  and `test_the_row_is_what_lets_the_commit_through` is its discriminator.
- Closure round 1, ISSUE 1: `_weakened_rows` read the file with `encoding="utf-8"`
  and caught only `OSError`. The hook's dispatcher fails OPEN on an exception from
  a check, so an undecodable byte would have let the weakening edit through. Fixed
  by reading with `errors="replace"`, which is what the delegate already did.

### Documentation Updates
- `docs/contributing/testing.md`, new section "When a test must be weakened": the
  three-step route, the naming rule, and what the count notice means.
- `docs/functional-tests.md`, new section under the draft workflow: the `.ci`/`.et`
  case, where a whole file is one test.
- `docs/architecture/testing/test-health.md`: the section is rewritten around the
  per-commit record, with the `stagesForMode` source anchor kept.
- `ai/INDEX.md`: the `ze-relax-census` row is replaced by `ze-weakened-check`.
- `TEST-RELAX-AUDIT.md`: a header pointing at the new foot section, and "The
  mechanism moved (2026-08-16)" recording what replaced what.
- `ai/rules/testing.md` and its points, `ai/rules/rfc-compliance.md`,
  `ai/rules/repo-maintenance.md`, `ai/rules/rule-format.md`, and the three review
  skills. `make ze-rules-render-check` reports 28 rules fresh;
  `make ze-rules-condensed-check` and `make ze-rules-lint` both exit 0.
- `make ze-validate` exits 0.

### Deviations from Plan
- `scripts/dev/audit-test-relaxation.py` is in Files to Modify and was not
  changed. Homed as item A of `plan/spec-weakened-followups.md` (Status ready) at
  the owner's direction on 2026-08-16, when the weekly token budget reached 3% on
  day 3. `ai/skills/ze-review.md` and `ai/skills/ze-review-deep.md` both state the
  gap, so a reviewer is told the audit does not read `test/weakened.md`.
- No `plan/learned/NNN-<name>.md` file was written. `ai/rules/git-safety.md`
  (owner directive, 2026-08-10) bans a lesson artifact saved beside the commit and
  requires the lesson be applied to the surface that governs behavior. It was:
  `plan/learned/HOOK-FRICTION.md` carries the new `c_test_weakening` entry, and
  `ai/rules/points/rule-format/the-body-has-a-budget-too/what-keeps-a-rule-body-short.md`
  carries the general form of the lesson. The journal row is the closure artifact.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Naming the weakened test from the edit hunk | A hunk usually holds the assertion alone, so it names the FILE while the commit gate names the FUNC | Phase 5, when the hook and the gate asked for two different rows | `_edited_file_pair` reconstructs the file the edit will leave on disk |
| assumption | The spec assumed a row in the working tree was enough | A row that is not committed records nothing | Phase 4 | AC-10 added, with two tests and the `--file` message |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One file holds every weakening the commit needs accepted (decision 1) | Done | `test/weakened.md` | Two columns, one table the parser anchors on |
| `commit_helper.py` calls the checker (decision 2) | Done | `weakened_problems`, `scripts/dev/commit_helper.py` | One `problems +=` line in `commit_gate_problems` |
| The hook stays and checks at edit time (decision 3) | Done | `c_test_weakening`, `_weakened_hatch` | Fails closed when the module does not load |
| The ceiling retires (decision 4) | Done | deleted: `test/relax-ceiling.txt`, `scripts/dev/relax-census.py` | No live caller remains |
| A missing entry REFUSES the commit, BLOCK tier (decision 5) | Done | `commit_gate_problems` | Safe at BLOCK because it reads only the commit's paths |
| An entry carries the test NAME and the reason, nothing else (decision 6) | Done | `Row`, `parse_weakened_file` | A row with no reason is refused |
| Delete the superseded spec | Done | `spec-fixit-relax-ceiling-raise-is-unreachable` gone | AC-9 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `weakened-row-opens-the-hatch` | Same edit as AC-2, differing only by the row |
| AC-2 | Done | `weakened-missing-row-refuses-the-edit` | The message prints the exact row |
| AC-3 | Done | `test_a_weakening_with_no_row_is_refused`, `test_create_itself_refuses_and_names_the_row_to_write` | Helper level and `create` entry point |
| AC-4 | Done | `test_a_row_for_a_test_this_commit_does_not_weaken_is_refused` | Stale row from the last commit |
| AC-5 | Done | `test_a_commit_that_weakens_nothing_ignores_the_file` | The file is not read at all |
| AC-6 | Done | `test_every_weakening_kind_reaches_the_check`, `test_no_detector_finding_is_filtered_out` | Count arms included |
| AC-7 | Done | `test_an_ambiguous_test_name_is_refused` and the three siblings around it | Refuse, and ask for `package.TestName` |
| AC-8 | Done | `make ze-verify-list` names `ze-weakened-check` at stage 12 and no census stage | Both goldens in `verify_run_test.go` updated |
| AC-9 | Done | The file is absent from `plan/` | Deleted in `005a97150` |
| AC-10 | Done | `test_the_row_must_be_in_the_commit_not_only_in_the_tree` | Discriminator: `test_the_row_is_what_lets_the_commit_through` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All 11 unit rows | Done | `scripts/dev/check_weakened_tests_test.py` (35 cases), `scripts/dev/commit_helper_test.py` (142 cases) | Both suites exit 0 |
| All 3 functional rows | Done | `scripts/dev/hook-fixture-check.py` | 9 `weakened-*` fixtures, 417 fixtures total, all pass |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify | Done | Except `scripts/dev/audit-test-relaxation.py`, see Deviations |
| Every file in Files to Create | Done | `test/weakened.md`, `scripts/dev/check_weakened_tests.py` and its test |

### Audit Summary
- **Total items:** 27 (7 requirements, 10 AC, 2 test groups, 2 file groups, 6 doc surfaces)
- **Done:** 26
- **Partial:** 1 (`scripts/dev/audit-test-relaxation.py`, homed in `plan/spec-weakened-followups.md` at the owner's direction)
- **Skipped:** 0
- **Changed:** 1 (AC-10, added during implementation)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The justification leaves the test file and lives in one file the commit must carry | functional (hook fixtures) | `weakened-missing-row-refuses-the-edit` and `weakened-row-opens-the-hatch` are the SAME edit against two states of `test/weakened.md`. Only the row changes the verdict, so the fixture pair discriminates. `weakened-row-for-another-test-buys-nothing` proves the hatch is per-weakening, not per-file |
| The record cannot accumulate | structural | The file is replaced per commit, and `unmatched_problems` refuses a row the commit does not weaken. A row therefore cannot survive its commit. Proven by `test_a_row_for_an_untouched_test_is_refused` |
| The reason sits in history beside the weakening it accepts | functional (commit entry point) | `test_the_row_must_be_in_the_commit_not_only_in_the_tree` refuses the commit; `test_the_row_is_what_lets_the_commit_through` is the same commit carrying the file and passing. `test_create_itself_refuses_and_names_the_row_to_write` drives it from `commit_helper.py create` itself |
| Two gates cannot disagree about what a diff weakens | unit, by identity | `test_the_detector_is_the_hook_own_function` asserts the loaded detector IS `_test_weakening_errs`; `test_the_module_is_loaded_from_scripts_dev` and `test_the_enclosing_test_name_comes_from_rfc_tagged_scope` pin the shared namer. `WEAKEN_GOLDEN` in `scripts/dev/hook-parity-check.py` pins the shell and Python hooks to the same verdicts |
| The ceiling and the census are gone, and nothing calls them | structural | `git grep` for `relax-census` and `relax-ceiling` finds no live caller. `make ze-verify-list` names `ze-weakened-check`. `make ze-weakened-check` exits 0, selftest first |
| Interop | N-A | No protocol behavior. This is agent tooling: nothing reaches the daemon, the wire, or an operator |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard exists | none | The spec metadata carries a dash in the Deferral shard field, and `plan/deferrals/spec-weakened-per-commit.md` does not exist |
| `scripts/dev/audit-test-relaxation.py` still reads the retired token | deferred | `plan/spec-weakened-followups.md` item A, Status ready, at the owner's direction 2026-08-16 |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/weakened-per-commit-363d8cc4-2def-4d57-bb22-0ffe21dd7f08.md` |
| `review_gate.py check` | clean (12 code files, hashes match) |
| Rounds | 2 |
| Reviewer lenses used | size and simplicity, wiring, functional-test coverage, documentation drift, removed-behavior and test-rewrite, guard audit (fail-closed and driven from the entry point), logic correctness, security and allocation, project-rules cross-check, false-positive filter. RFC and interop lenses are N-A: no protocol code |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The hatch reader caught only `OSError`, and the hook's dispatcher fails OPEN on any other exception, so an undecodable byte in `test/weakened.md` would have let the weakening edit land with no row | `_weakened_rows`, `.claude/hooks/pretool-writeedit.py` | Read with `errors="replace"`, the same reader the delegate uses, and say in the docstring which error state is reported and which is absorbed |
| 2 | NOTE | The by-path loader claimed `commit_helper.py` imports this module by path and has no `scripts/dev` on sys.path. It lives in that directory and imports by name | `_load_by_path`, `scripts/dev/check_weakened_tests.py` | Docstring names the one caller that needs the by-path load, and why the other does not |
| 3 | NOTE | The two gates' populations differ for a `.py` carrier: the hook can reach the hatch for an RFC-tagged `check.py`, and `is_test_path` answers False for `.py`, so the commit gate never judges it | `c_test_weakening` and `is_test_path` (`scripts/dev/audit-test-relaxation.py`) | Not fixed. Reachable only behind the stronger RFC gate, which needs the user's own approval, and no such carrier exists in the tree today |
| 4 | NOTE | Eight source files cite this spec, which commit B removes | `Makefile`, the two hooks, four `scripts/dev` files | Not fixed. 79 of 105 spec citations in source are already dangling, and `scripts/dev/spec-citation-check.py` scopes to `plan/spec-*.md` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/weakened.md` | Yes | 80 lines, one two-column table, no rows |
| `scripts/dev/check_weakened_tests.py` | Yes | 666 lines |
| `scripts/dev/check_weakened_tests_test.py` | Yes | 573 lines, 35 cases |
| `test/relax-ceiling.txt`, `scripts/dev/relax-census.py`, `spec-fixit-relax-ceiling-raise-is-unreachable` | No, by design | `ls` reports "No such file or directory" for all three |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | The hook opens on a row and refuses without one | `python3 scripts/dev/hook-fixture-check.py`: 417 fixtures, 0 fail, the 9 `weakened-*` among them |
| AC-3..AC-7, AC-10 | The commit gate refuses a missing row, a stale row, and an unstaged file | `python3 scripts/dev/check_weakened_tests_test.py`: 35 tests, OK. `python3 scripts/dev/commit_helper_test.py`: 142 tests, OK |
| AC-8 | The stage swapped in both verify modes | `make ze-verify-list` prints `ze-weakened-check` at stage 12 and no census stage |
| AC-9 | The superseded spec is gone | `ls spec-fixit-relax-ceiling-raise-is-unreachable` fails |
| All | The checker itself is not vacuous | `python3 scripts/dev/check_weakened_tests.py --selftest` prints SELFTEST PASS; it asserts the refusal AND the acceptance of one edit, plus a stale row and a broken header |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An `Edit` weakening a test with no row | `weakened-missing-row-refuses-the-edit` (`scripts/dev/hook-fixture-check.py`) | Yes: the fixture drives the real hook over a fixture tree and asserts exit 2 |
| An `Edit` weakening a test named in the file | `weakened-row-opens-the-hatch` | Yes: same edit, row present, exit 0 |
| A row naming a different test | `weakened-row-for-another-test-buys-nothing` | Yes |
| `commit_helper.py create` naming a weakened test | `test_create_itself_refuses_and_names_the_row_to_write` | Yes: it drives `create`, not the helper, and asserts exit 2 and the printed row |
| `commit_helper.py create` with a stale row | `test_a_row_for_a_test_this_commit_does_not_weaken_is_refused` | Yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `weakened-missing-row-refuses-the-edit` and `weakened-row-opens-the-hatch` are one edit against two file states; `weakened-write-overwrite-needs-a-row` proves it for `Write` |
| A-2 | confirmed | `TestAmbiguousName` in `scripts/dev/check_weakened_tests_test.py` drives all four cases |
| A-3 | confirmed | `render_git_add` stages only the declared paths, `render_staging_guard` aborts on anything else, and `test_a_weakening_the_commit_does_not_name_is_invisible` pins the consequence |
| A-4 | confirmed | `test_the_detector_is_the_hook_own_function`: the gate's detector IS the hook's function, so agreement is identity rather than comparison. `_edited_file_pair` gives it the same text |
| A-5 | confirmed | `git grep` finds no live `relax-census` caller; `make ze-verify-list` names the new stage |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Q10 test infrastructure: `docs/functional-tests.md` | The new section states a whole `.ci` is one test and its stem is the name. Matches `scope_reader` and the `stem` branch of `weakened_units` | Yes |
| Q10 test infrastructure: `docs/contributing/testing.md` | The three-step route matches `c_test_weakening` (reads from disk, so the row comes first) and `weakened_problems` (refuses a commit not carrying the file) | Yes |
| Q15 inventory: `ai/INDEX.md` | The row names `scripts/dev/check_weakened_tests.py` and both callers; each exists | Yes |
| Q16 source anchors: `docs/architecture/testing/test-health.md` | The `stagesForMode` source anchor still resolves, and that function now lists `ze-weakened-check` in both branches | Yes |
| Q17 examples: `ai/rules/testing.md` and its points | `the-test-relax-token-format.md` now shows the two-column header the parser anchors on, character for character | Yes |
| Q1..Q9, Q11..Q14 answered No | `git grep` over `docs/` for the changed files finds no other anchored claim; nothing here reaches the daemon, the CLI, the wire, or an operator | Yes |
| Doctor check | `test/weakened.md` is a tracked repo file read by dev tooling. No runtime dependency, so no `ze doctor` check is owed | Yes |
| `make ze-doc-test` | Not run: 100+ files in this shared checkout belong to other sessions and three CI stages are red from their uncommitted RFC index staleness (`plan/journal/concurrent-rfc-gate-stale.md`). `make ze-validate`, `make ze-rules-render-check`, `make ze-rules-condensed-check` and `make ze-rules-lint` all exit 0 | Scoped |

## Core Insight

A record every reader pays for must change what the reader does. The `test-relax:`
token was a good idea in the wrong place: forcing a written reason is right, and
keeping it beside the code forever is what destroyed its value. The reason is
ephemeral by nature, because it explains one diff, and permanent storage of an
ephemeral fact has a fixed end state. The pile grows until nobody can read it,
then nobody does, then writing one costs nothing, then the gate that demands one
is theatre. Git history is the correct permanent store, and the working file holds
only the change in hand.
