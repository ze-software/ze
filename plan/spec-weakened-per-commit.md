# Spec: weakened-per-commit

| Field | Value |
|-------|-------|
| Status | verification |
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

`plan/spec-fixit-relax-ceiling-raise-is-unreachable.md` existed to make a ceiling
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
| Hook ↔ `test/weakened.md` | the hook reads a tracked file it does not own | No |
| commit_helper ↔ hook detector | `load_detector` imports `_test_weakening_errs` so both judge alike | No |
| Both ↔ `rfc_tagged_scope` | one definition of the enclosing unit, shared, never copied | No |

### Integration Points
- `commit_gate_problems` (`scripts/dev/commit_helper.py`) - one more `problems +=`
- `load_detector` (`scripts/dev/audit-test-relaxation.py`) - already shares the
  hook's detector with the commit-time path
- `stagesForMode` (`scripts/status/verify_run.go`) - swaps the stage

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
| A-1 | An agent can write `test/weakened.md` BEFORE the weakening edit | the hook fires on the edit and cannot see the future | the hatch never opens and every weakening is refused | a fixture driving file-then-edit and edit-then-file | confirmed: `weakened-missing-row-refuses-the-edit` and `weakened-row-opens-the-hatch` in `scripts/dev/hook-fixture-check.py` are the SAME edit against two states of the file, and only the row changes the verdict. `weakened-write-overwrite-needs-a-row` proves it for the Write carrier |
| A-2 | A test NAME identifies the weakening unambiguously | owner decision 6 | two same-named tests in one commit cannot be told apart | `TestNoGoFileBuildsMarkup` exists in `internal/component/lg` AND `internal/component/web`; a fixture weakening both | confirmed, with the `package.TestName` qualifier AC-7 requires: `TestAmbiguousName` in `scripts/dev/check_weakened_tests_test.py` drives all four cases (bare over two refused, both qualified accepted, bare over one accepted, wrong qualifier refused) |
| A-3 | The commit's `--file` list names every test file it weakens | `commit_helper.py` requires explicit `--file` | a weakening rides in unchecked | a fixture committing a weakened test with no row | confirmed: `render_git_add` stages only the declared paths and `render_staging_guard` aborts when the index holds any other, so a weakened file absent from `--file` is absent from the commit. `test_a_weakening_with_no_row_is_refused` and `test_a_weakening_the_commit_does_not_name_is_invisible` (`scripts/dev/commit_helper_test.py`) pin both halves |
| A-4 | Commit-time recomputation against HEAD agrees with what the hook saw | both import `_test_weakening_errs` via `load_detector` | the gate refuses a commit the hook allowed | a fixture asserting both agree on one edit | unvalidated |
| A-5 | Retiring the census breaks no other caller | it is a verify stage in both modes and a `scripts/dev` python test | `ze-verify` fails on a missing target | grep the target, then `make ze-verify-list` | unvalidated |

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
| AC-9 | `plan/spec-fixit-relax-ceiling-raise-is-unreachable.md` | is deleted from `plan/` |
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
