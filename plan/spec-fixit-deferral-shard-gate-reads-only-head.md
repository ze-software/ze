# Spec: the deferral shard-removal gate reads only HEAD, so a two-commit closure can delete a live row

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/5 |
| Deferral shard | - |
| Updated | 2026-08-08 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** `deferral_shard_removal_problems` (`scripts/dev/commit_helper.py`) BLOCKS a
commit that removes a `plan/deferrals/` shard still holding a live row. It reads that shard
with `git show HEAD:<rel>` and judges the rows it finds there. Its one escape,
`_live_rows_added_elsewhere`, reads the WORKING TREE, but only for shards the SAME create call
ADDS, which covers a rename and nothing else.

**The symptom.** Spec closure is two commits from one script (`ai/rules/planning.md`, "Spec
Closure"): commit A carries the spec and its edits, commit B carries `--remove plan/<spec>`
plus the shard removals. Both `create` calls run before `bash` runs either commit, so at
commit B's gate HEAD still holds the PRE-commit-A shard. A row written into that shard by
commit A is invisible: `live` is empty, no offender is recorded, the BLOCK passes, and commit
B deletes a live row one commit after it was written. Nothing else notices, because every
other signal over these rows folds across the directory, which the function's own docstring
names as the reason this gate must BLOCK rather than warn.

**Why it matters now.** `ai/rules/completion.md` ("A problem you FIND while working on
something else gets a SPEC") makes this the normal case rather than a corner: a session that
finds a problem while closing writes its deferral row into the closing spec's own shard. That
rule currently carries a directive telling the agent to drop the shard from commit B by hand.
Prose is exactly the control the docstring says cannot be the only one.

**The goal.** The gate judges the shard as the COMMIT will leave it, not as HEAD holds it, and
keeps refusing to read silence as innocence. The tool already builds a prospective commit view
for the discovery indexes (`extract_head_into`, `apply_commit_overlay`, `stale_in_commit_view`),
so the shape exists; whether to reuse it, read the working tree, or diff the prepared script's
earlier blocks is the design question. The `--remove` of a genuinely all-terminal shard must
stay possible, and the rename escape must keep working.

**Then delete the hand-written workaround.** The directive in
`ai/rules/points/completion/directives/spec-a-found-problem-close-then-ask.md` that tells the
agent to drop the shard from commit B exists only because the gate cannot see the row. It goes
when the gate can.

**Provenance.** Found on 2026-08-08 while landing the "found problem gets a spec" rule
(`plan/learned/1366-found-problem-spec-first.md`, commit `a756f094c`). Independent review
rounds 3 and 5 verified the mechanism against the producing function.

### Amendment (2026-08-08, RESEARCH): the defect has TWO directions, not one

Reading the producer showed a second, opposite failure in the same `git show HEAD:` read. The
Task above describes only the first.

| Direction | Sequence | Today | Correct |
|-----------|----------|-------|---------|
| 1 (Task, above) | Commit A writes a live row into the shard; commit B removes the shard | HEAD is pre-commit-A, so the row is invisible and the gate PASSES | BLOCK |
| 2 (found in research) | Commit A resolves the shard's LAST live row to `done`; commit B removes the now-residue shard | HEAD still shows the row live, so the gate BLOCKS | ALLOW |

Direction 2 makes a workflow `ai/rules/planning.md` mandates unreachable through the helper:
"An all-terminal orphaned shard is residue, and the actor who deletes it is the closer of the
LAST spec that homed one of its rows. Setting the final row to `done` is what makes the shard
residue, so the same commit removes the file." Both directions are the same defect stated by
the goal above ("judges the shard as the COMMIT will leave it"), one read fixes both, and
fixing only direction 1 leaves a mandated workflow broken. Direction 2 is therefore IN SCOPE.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs

There is no `docs/architecture/` document for the commit tooling. The governing documents are
rules, and they are read in full below.

- [ ] `ai/rules/planning.md` - "Spec Closure" and "Deferral Tracking" define what the gate protects
  → Decision: closure is TWO commits from ONE script. Commit A carries code, spec, learned summary. Commit B carries `--remove plan/<spec>` plus the shard removals. Both `create` calls run before either commit does
  → Constraint: "A shard that still holds a live row SURVIVES its source spec, and keeps its source-keyed name." The gate must keep enforcing this
  → Constraint: "Setting the final row to `done` is what makes the shard residue, so the same commit removes the file." The gate must STOP forbidding this. It is direction 2 of the amendment above
  → Constraint: "An orphaned shard is not a defect to sweep." Dropping the `--remove` is always a correct outcome, never a workaround

- [ ] `ai/rules/git-safety.md` - the checkout is shared between concurrent sessions
  → Constraint: "A SHARED CHECKOUT NEVER GIVES A CLEAN `ze-verify`" because gates that read the working tree also read other sessions' half-finished edits. A BLOCK gate that reads the working tree inherits that hazard, so its message MUST name an escape the blocked session can take alone
  → Decision: the existing message already names one ("or drop the `--remove` for this shard"). It must survive the change verbatim in intent

- [ ] `ai/rules/evidence.md` - guards fail closed
  → Constraint: a view the gate cannot read is never innocent. Every existing "cannot see it" branch (unknown `git show` failure, unreadable object store) must keep reporting rather than passing
  → Constraint: a newly added guard that fails open is always in scope at review. The vanished-row guard in AC-5 is exactly such a guard and carries its own test

- [ ] `ai/rules/points/completion/directives/spec-a-found-problem-close-then-ask.md` - carries the workaround
  → Decision: bullet 4's last sentence, "Drop the shard from commit B yourself.", is the workaround the Task says to delete
  → Constraint: the three sentences before it DESCRIBE the defect ("no gate will tell you", "passes the BLOCK and deletes the live row"). They become false when the gate is fixed, so `ai/rules/stale-comments.md` requires rewriting them, not only deleting the last one
  → Constraint: bullet 3 ("The new spec and its deferral row ride commit A") stays true after the fix and does not depend on bullet 4

- [ ] `ai/rules/points/planning/deferral-tracking/the-gate-that-blocks-removing-a-live-bearing-shard.md` - states the mechanism in the rule
  → Constraint: it says the gate "reads the shard at HEAD and BLOCKS when any row is non-terminal". That sentence goes stale on this change and must be rewritten in the point file, then re-rendered

### RFC Summaries (Scope: protocol)

N/A. Scope is `tooling`; no protocol surface is touched.

**Key insights:** (minimal context to resume after compaction)
- The gate runs at `create` time ONLY. `render_block` embeds no commit_helper gate in the generated script, so there is no re-check when the operator runs it.
- `_create` writes a message file and a script and stages nothing. It never runs `git add`, `git rm`, or `git commit`. So at commit B's gate the working tree still holds commit A's edits, and `git commit` does not alter the working tree either.
- `render_block` emits `git rm -- <paths>`, NOT `git rm -f`. Git refuses to remove a tracked file carrying local modifications, so a working copy that no commit in the script stages makes the script fail loudly rather than silently deleting an unrecorded state.
- The 13 existing tests live in `TestDeferralShardRemoval` (`scripts/dev/commit_helper_test.py`) and are driven through `commit_gate_problems`, never through the gate function alone. The class docstring makes that mandatory.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/dev/commit_helper.py`, `deferral_shard_removal_problems` - for each `remove_paths` entry under `plan/deferrals/` ending `.md`, runs `git show HEAD:<rel>` under `LC_ALL=C`. A non-zero exit matching `_GIT_SHOW_NOTHING_COMMITTED` is benign and skipped; any other non-zero exit is reported as "cannot be read at HEAD". Rows are parsed with `_deferral_row_cells`; a row whose cell 5 is in `DEFERRAL_TERMINAL_STATUSES` is skipped. Surviving rows are rendered and, if any remain after subtracting `_live_rows_added_elsewhere`, returned as one BLOCK string
- [ ] `scripts/dev/commit_helper.py`, `_live_rows_added_elsewhere` - reads the WORKING TREE copy of every `plan/deferrals/**.md` in `add_paths` and returns their live-row renderings. This is the rename escape, and it is the only working-tree read in the gate today
- [ ] `scripts/dev/commit_helper.py`, `commit_gate_problems` - the BLOCKING assembly; `deferral_shard_removal_problems` is one entry
- [ ] `scripts/dev/commit_helper.py`, `_create` - normalizes `--file` and `--remove` into `add_paths` / `remove_paths`, calls `commit_gate_problems`, raises `UsageError` on any problem, then calls `write_outputs`. Stages nothing
- [ ] `scripts/dev/commit_helper.py`, `render_block` - writes `git rm -- <quoted paths>` into the generated script
- [ ] `scripts/dev/commit_helper_test.py`, `TestDeferralShardRemoval` - 13 tests, all driven through `commit_gate_problems`, seeded by `_seed_shard_repo` which does a real `git init` plus a commit
- [ ] `ai/rules/points/completion/directives/spec-a-found-problem-close-then-ask.md` - bullet 4 carries the workaround

**Behavior to preserve:** (unless the user explicitly said to change it)
- The gate stays BLOCK severity and stays assembled in `commit_gate_problems`. Tests drive it through that entry point
- `--remove` of a shard whose rows are all terminal stays possible
- The rename escape: rows re-added by the same call at another path are a move, not a deletion. A PARTIAL move still blocks on the rows left behind
- Deleting the working copy before running `create --remove` does NOT clear the gate
- Every "cannot see it" branch reports rather than passes: an unknown `git show` failure, and an unreadable object store
- An unborn HEAD passes, and a path absent from BOTH HEAD and the working tree passes
- Scope stays `plan/deferrals/**.md`, recursive, matching `deferral_shard_paths`
- The message keeps both escapes: resolve each row, or drop the `--remove`
- `DEFERRAL_TERMINAL_STATUSES` is unchanged, and an unrecognised status stays live

**Behavior to change:** (only what the user asked for)
- A shard ABSENT at HEAD but present in the working tree with a live row now BLOCKS. Today it passes, because `git show` fails benignly. This is direction 1 where commit A CREATES the shard
- A shard at HEAD whose live rows the working copy has re-statused to terminal now PASSES. Today it blocks on the stale HEAD row. This is direction 2
- A row live at HEAD whose `What` has vanished from the working copy entirely, rather than being re-statused, now BLOCKS. This guard is new and exists only because the change starts trusting the working tree

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An agent preparing a closure commit runs `python3 scripts/dev/commit_helper.py create --append --remove plan/spec-<stem>.md --remove plan/deferrals/<stem>.md`
- Format at entry: `argparse` string lists `args.file` and `args.remove`

### Transformation Path
1. `_create` normalizes both lists with `rel_path` then `unique_paths`, producing the `add_paths` and `remove_paths` tuples: repo-relative, order-preserving, deduped
2. `commit_gate_problems` assembles the BLOCK list. `deferral_shard_removal_problems` is its second entry
3. Per removed path under `plan/deferrals/` ending `.md`: read HEAD with `git show HEAD:<rel>`, read the working copy if it exists, and compute the set of live rows the removal would destroy
4. `_live_rows_added_elsewhere` subtracts the rows this same call re-adds elsewhere, which is the rename escape
5. A non-empty result becomes a `UsageError` raised by `_create`. No script and no message file are written, so the closure cannot proceed

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Git object store ↔ gate | `git show HEAD:<rel>` under `LC_ALL=C`, so the benign stderr phrases stay in English | Yes |
| Working tree ↔ gate | `Path.read_text(encoding="utf-8", errors="replace")` on the shard, the same call `_live_rows_added_elsewhere` already makes | Yes |
| Gate ↔ operator | `UsageError` text naming the offending rows and both escapes | Yes |
| Gate ↔ generated script | None. The gate runs at `create` time only; `render_block` embeds no re-check | Yes |

### Integration Points
- `commit_gate_problems` - the BLOCK assembly the gate is already wired into. No new wiring is added
- `_deferral_row_cells` - the existing row parser, reused unchanged for both the HEAD blob and the tree blob
- `DEFERRAL_TERMINAL_STATUSES` - the terminal denylist, reused unchanged
- `_live_rows_added_elsewhere` - the rename escape, applied to the computed live set exactly as today

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The change is inside `deferral_shard_removal_problems`; `_create` and `commit_gate_problems` are untouched, and the tests still drive the gate through `commit_gate_problems` |
| No unintended coupling (components stay isolated) | Yes | No new caller and no new module. The only new dependency is `Path.is_file` / `read_text` on a path the function already names |
| No duplicated functionality (extends existing, does not recreate) | Yes | Row parsing reuses `_deferral_row_cells` and the terminal set; the tree read reuses the idiom in `_live_rows_added_elsewhere`. A shared row-extraction helper removes the copy rather than adding one |
| Zero-copy preserved where applicable (refs, not copies) | N/A | Python dev tooling reading one small markdown file per removed shard. `ai/rules/performance.md` governs Go wire encoding, not this |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N/A | No command, view, family, or handler is added. The gate is already registered in `commit_gate_problems` |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | At commit B's `create`, the working tree holds commit A's shard edits | `_create` writes only a message file and a script, and stages nothing; `git commit` does not alter the working tree | Both directions stay broken, because the gate would read a tree that predates commit A | Read `_create`; then a test seeding exactly that tree state and driving `commit_gate_problems` | confirmed |
| A-2 | A working copy that no commit in the script stages cannot survive the generated `git rm` | `render_block` emits `git rm --`, not `git rm -f`, and git refuses a tracked file carrying local modifications | The gate would trust a tree state that never reaches history, so a hand edit to the Status column would become a silent bypass | Read `render_block`; then a test that runs `git rm` on a locally-modified tracked file and asserts it fails | unvalidated |
| A-3 | The `What` cell identifies a row well enough to compare HEAD against the working tree | `_live_rows_added_elsewhere` already keys the rename escape on a rendering built from cells 2, 4 and 5 | The vanished-row guard misfires, either blocking a legitimate edit or missing a deleted row | A test with two rows sharing a `What`, and one with a row whose `What` is edited rather than deleted | unvalidated |
| A-4 | No other reader resolves a removed shard's rows; they all fold over the directory | The gate's own docstring claims it. `deferral_shard_paths` callers are `deferral_unassigned_problems`, `deferral_orphans.py` and `.claude/hooks/session-end-deferrals.sh`, all directory folds | The BLOCK severity would be unjustified and a WARN might do | Grep of `deferral_shard_paths` callers | confirmed |
| A-5 | Flipping `test_index_added_but_uncommitted_shard_is_benign` is a correction, not a weakening | Its comment reasons about COMMITTED rows only; the goal restates the gate as protecting the rows the COMMIT will leave | A deliberate protection is removed and nobody notices | Rewrite the test's comment to state the new premise, and keep `test_a_shard_new_in_this_commit_is_not_read_from_head` covering the absent-from-both case | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Shared checkout: another session's uncommitted live row in the shard blocks MY closure, and no action of mine makes it green | The blocked rows name a `Source` that is not my spec | The message's second escape needs nobody else: drop the `--remove`, and the shard outlives its source spec, which `ai/rules/planning.md` calls the correct end state. Unlike `ze-verify`, this gate is always satisfiable in one local move. Raised by Thomas on 2026-08-08 |
| R-2 | The vanished-row guard is a NEW guard, and a guard that fails open is silent | Nothing. That is the point: silence is the failure mode | AC-5 tests it directly, and `ai/rules/planning.md` puts a newly added guard that fails open in the always-in-scope review list |
| R-3 | Trusting the working tree lets an agent flip a Status column by hand to get past the gate | A commit whose shard diff re-statuses rows it did not do the work for | Bounded by A-2: the edit must be carried by a commit in the same script or `git rm` fails. The edit is then visible in commit A's diff and in review |
| R-4 | Reading two views doubles the per-shard cost | None measurable | One extra `read_text` of a small markdown file per removed shard, on a path already named. Removals are rare and few |
| R-5 | Direction 2 loosens the gate, and a loosened BLOCK is how live rows get deleted | A shard removed in the same commit that resolved its last row, where the resolution was wrong | The resolution itself is committed and reviewable in commit A. The gate's job is the record, not the judgment that the work landed. `ai/rules/planning.md` already names this actor and this sequence as correct |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible in the daemon: this is commit tooling. A mis-BLOCK stalls a closure and is loud and recoverable. A mis-PASS deletes the record of live deferred work from a shard, and is silent, which is why the gate is BLOCK severity |
| How is it reverted? | Single commit revert. The gate is pure Python, holds no state, and writes no files |
| Who else touches this path? | `deferral_unassigned_problems` (WARN), `scripts/dev/deferral_orphans.py`, and `.claude/hooks/session-end-deferrals.sh` all read `plan/deferrals/`, but every one of them folds over the directory and none reads a removed shard. `plan/spec-fixit-review-loop-has-no-termination-bound.md` touches `review_gate.py`, a different gate in the same `create` flow |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `commit_helper.py create --remove plan/deferrals/<stem>.md`, shard created by commit A | → | `commit_gate_problems` → `deferral_shard_removal_problems` | `test_a_row_written_by_commit_a_blocks_commit_b` |
| `commit_helper.py create --remove plan/deferrals/<stem>.md`, last row resolved by commit A | → | `commit_gate_problems` → `deferral_shard_removal_problems` | `test_resolving_the_last_row_in_this_commit_allows_the_removal` |
| `commit_helper.py create --remove plan/deferrals/<stem>.md`, HEAD-live row deleted from the tree | → | `commit_gate_problems` → `deferral_shard_removal_problems` | `test_a_head_row_deleted_from_the_working_copy_still_blocks` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The shard does not exist at HEAD, exists in the working tree, holds a live row, and is in `--remove` | BLOCK, naming that row |
| AC-2 | The shard exists at HEAD with only terminal rows, the working tree copy adds a live row, and it is in `--remove` | BLOCK, naming the added row |
| AC-3 | The shard's rows are live at HEAD, the working tree copy re-statuses every one of them to terminal, and it is in `--remove` | PASS. Removing residue the same commit created stays possible |
| AC-4 | The working copy is deleted before `create` runs, and HEAD holds a live row | BLOCK, naming the HEAD row. An absent view is never innocence |
| AC-5 | A row is live at HEAD and its `What` is absent from the working copy entirely, rather than re-statused | BLOCK, naming that row |
| AC-6 | Every live row of the removed shard is re-added by the same call at another path | PASS. The rename escape is unchanged |
| AC-7 | Only SOME live rows are re-added at another path | BLOCK, naming only the rows left behind |
| AC-8 | The path is absent from BOTH HEAD and the working tree | PASS. Nothing exists to destroy |
| AC-9 | `git show` fails for a reason outside `_GIT_SHOW_NOTHING_COMMITTED`, and a readable working copy exists | BLOCK, reporting "cannot be read at HEAD". A readable second view never converts an unreadable first view into a pass |
| AC-10 | `ai/rules/completion.md` is regenerated from its point files | It contains no instruction to drop the shard from commit B by hand, and its description of the gate matches the new behavior |
| AC-11 | `ai/rules/planning.md` is regenerated from its point files | It no longer states that the gate reads the shard at HEAD |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_a_row_written_by_commit_a_blocks_commit_b` | `scripts/dev/commit_helper_test.py` | AC-1: shard absent at HEAD, live row in the tree | |
| `test_a_live_row_added_to_a_terminal_shard_blocks_its_removal` | `scripts/dev/commit_helper_test.py` | AC-2: terminal at HEAD, live in the tree | |
| `test_resolving_the_last_row_in_this_commit_allows_the_removal` | `scripts/dev/commit_helper_test.py` | AC-3: direction 2, the residue workflow | |
| `test_deleting_the_working_copy_first_does_not_clear_the_gate` | `scripts/dev/commit_helper_test.py` | AC-4: EXISTING test, must still pass unchanged | |
| `test_a_head_row_deleted_from_the_working_copy_still_blocks` | `scripts/dev/commit_helper_test.py` | AC-5: the new vanished-row guard | |
| `test_two_rows_sharing_a_what_are_not_confused` | `scripts/dev/commit_helper_test.py` | A-3: the identity key holds when `What` repeats | |
| `test_renaming_a_live_shard_is_a_move_not_a_deletion` | `scripts/dev/commit_helper_test.py` | AC-6: EXISTING test, must still pass unchanged | |
| `test_a_partial_move_still_blocks_on_the_rows_left_behind` | `scripts/dev/commit_helper_test.py` | AC-7: EXISTING test, must still pass unchanged | |
| `test_a_shard_new_in_this_commit_is_not_read_from_head` | `scripts/dev/commit_helper_test.py` | AC-8: EXISTING test, must still pass unchanged | |
| `test_an_unexpected_git_failure_is_reported` | `scripts/dev/commit_helper_test.py` | AC-9: EXISTING test, must still pass unchanged | |
| `test_an_unreadable_shard_is_reported_not_waved_through` | `scripts/dev/commit_helper_test.py` | AC-9: EXISTING test, must still pass unchanged | |
| `test_index_added_but_uncommitted_shard_is_benign` | `scripts/dev/commit_helper_test.py` | EXISTING test whose premise this spec changes. Rewritten under AC-1, comment restated | |
| `test_git_rm_refuses_a_locally_modified_shard` | `scripts/dev/commit_helper_test.py` | A-2: the bound that makes trusting the tree sound | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

No numeric input exists. The gate's inputs are file paths and a five-word status vocabulary;
`DEFERRAL_TERMINAL_STATUSES` is a denylist whose complement is live, and it is unchanged here.

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestPythonUnitTests` | `scripts/dev/python_tests_test.go` | N-A as a `.ci`. `scripts/dev` has no `.ci` surface: it is dev tooling, not a daemon feature. `TestPythonUnitTests` is the runner that executes `commit_helper_test.py`, reached by `make ze-test-pkg PKG=./scripts/dev`. The tests above build a real git repository with `git init` and a real commit, then drive `commit_gate_problems`, which is the entry point `create` calls | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | N/A | N/A | Scope is `tooling`. No wire-visible behavior and no peer daemon | |

Vacuity control, which DOES apply: every AC test above must fail when the change is reverted.
AC-1, AC-2, AC-3 and AC-5 each fail against today's code, which is what makes them evidence
rather than decoration. The seven tests marked EXISTING are the opposite control: they must
pass both before and after.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `scripts/dev/commit_helper.py` - `deferral_shard_removal_problems` computes the live-row set from the prospective commit view rather than from HEAD alone, and a shared row-extraction helper serves both views and `_live_rows_added_elsewhere`. Its docstring's "Read from HEAD, not the working tree" paragraph is rewritten
- `scripts/dev/commit_helper_test.py` - four new tests, one rewritten, seven pinned unchanged
- `ai/rules/points/completion/directives/spec-a-found-problem-close-then-ask.md` - bullet 4 rewritten: the workaround sentence goes, and the three sentences describing the defect are restated as the gate's new behavior
- `ai/rules/points/planning/deferral-tracking/the-gate-that-blocks-removing-a-live-bearing-shard.md` - the "reads the shard at HEAD" sentence is rewritten
- `ai/rules/completion.md` - GENERATED by `make ze-rules-render`. Never hand-edited
- `ai/rules/planning.md` - GENERATED by `make ze-rules-render`. Never hand-edited
- `ai/rules/TRIGGERS.md`, `ai/rules/CORE.md` - GENERATED by `make ze-rules-condensed`. Committed with the rule change

## Files to Create

None. Every change extends an existing file.

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Dev tooling. No daemon config surface is touched |
| YANG validation constraints | N-A | No YANG leaf is added |
| YANG custom validators | N-A | No YANG leaf is added |
| CLI commands/flags | N-A | No flag is added or changed. `create` keeps `--file` and `--remove` exactly as today |
| CLI grammar (keyword before value) | N-A | `ai/rules/cli.md` governs the `ze` CLI. `commit_helper.py` is an `argparse` dev script, not a `ze` verb |
| Editor autocomplete | N-A | No YANG leaf is added |
| Functional test for new RPC/API | N-A | No RPC or API is added. The runner is `TestPythonUnitTests`, named in the Functional Tests table |
| Pipe completeness | N-A | No `ze` CLI output is produced |
| Env var registration | N-A | No env var is read or added |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, kernel module, listen port, procfs entry, netlink use, binary, or certificate. The gate calls `git`, which `commit_helper.py` already requires throughout |
| Prometheus counters/metrics | N-A | A commit-time gate in a dev script exposes no runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | No daemon feature. `docs/features.md` describes the network OS, not the commit tooling |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | No `ze` verb touched, and `create`'s flags are unchanged |
| 4 | API/RPC added/changed? | No | No API surface touched |
| 5 | Plugin added/changed? | No | No plugin touched |
| 6 | Has a user guide page? | No | Commit tooling is documented in `ai/rules/git-safety.md`, which this change does not alter |
| 7 | Wire format changed? | No | No wire surface touched |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Scope is `tooling`; no RFC obligation is in reach |
| 10 | Test infrastructure changed? | No | Tests are added to an existing file run by an existing target. No new runner, fixture format, or harness |
| 11 | Affects daemon comparison? | No | Nothing a peer daemon can observe |
| 12 | Internal architecture changed? | No | One gate function's read strategy. No component, tier, or boundary moves |
| 13 | Route metadata keys added/changed? | No | No route metadata touched |
| 14 | Prometheus counters added/changed? | No | No counter added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` and `ai/` for `commit_helper.py` and for `deferral_shard_removal_problems`, and correct every claim that the gate reads HEAD. The two point files above are the hits found so far; the grep is repeated at implementation time because another may have landed |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/rules/git-safety.md` shows the commit workflow. Verify its `--remove` guidance still matches after the change, and correct it if the residue-removal case now reads differently |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the three Wiring Test rows are reachable and currently wrong
   - Tests: `test_a_row_written_by_commit_a_blocks_commit_b`, `test_resolving_the_last_row_in_this_commit_allows_the_removal`, `test_a_head_row_deleted_from_the_working_copy_still_blocks`
   - Files: `scripts/dev/commit_helper_test.py`
   - Verify: all three FAIL against today's `deferral_shard_removal_problems`, each for the reason its AC names, and each reaches the gate through `commit_gate_problems`. A test that fails for a setup error proves nothing
2. **Phase: Prospective read** -- the live-row set comes from the view the commit will leave
   - Tests: AC-1, AC-2, AC-3, AC-4, AC-8, AC-9 tests go green; the seven EXISTING tests stay green
   - Files: `scripts/dev/commit_helper.py`
   - Verify: HEAD is still read on every path so no "cannot see it" branch is lost, and the rename escape still runs on the computed set
3. **Phase: Vanished-row guard** -- a HEAD-live row absent from the tree is a deletion, not a resolution
   - Tests: `test_a_head_row_deleted_from_the_working_copy_still_blocks`, `test_two_rows_sharing_a_what_are_not_confused`
   - Files: `scripts/dev/commit_helper.py`
   - Verify: A-3 is settled by the shared-`What` test, and the guard fails CLOSED when the tree copy cannot be parsed
4. **Phase: Bound the trust** -- settle A-2 so the tree read is sound rather than convenient
   - Tests: `test_git_rm_refuses_a_locally_modified_shard`
   - Files: `scripts/dev/commit_helper_test.py`
   - Verify: the test exercises real `git rm`, not a claim about it
5. **Phase: Retire the workaround** -- the prose control goes now that the mechanical one works
   - Tests: AC-10, AC-11 checked by `make ze-doc-test`, which runs `rules_points.py render --check`, `roundtrip` and `coverage`
   - Files: the two point files, then `make ze-rules-render` and `make ze-rules-condensed`, committing all generated digests with the rule
   - Verify: no instruction to drop the shard by hand survives anywhere, and no rule still says the gate reads HEAD

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a named test |
| Feature completeness | Both failure directions of the amendment are covered, not only the one the Task names |
| Correctness | The gate never converts an unreadable HEAD into a pass because a readable tree copy exists. AC-9 is the case, and reading the tree first would break it |
| Correctness | The rename escape is applied AFTER the prospective set is computed, not to the HEAD set only |
| Naming | The new helper names the view it reads, not the file it reads from: the gate now has two views and a name like `_shard_rows` would hide which |
| Data flow | No caller outside `deferral_shard_removal_problems` changes. `commit_gate_problems` and `_create` are untouched |
| Rule: `ai/rules/evidence.md` | The vanished-row guard fails closed. An unparseable tree copy must report, never pass |
| Rule: `ai/rules/stale-comments.md` | The docstring's "Read from HEAD, not the working tree" paragraph and both rule point files are rewritten, not left describing the old read |
| Rule: `ai/rules/git-safety.md` | The message still names an escape the blocked session can take alone, because the gate now reads a shared working tree (R-1) |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The gate blocks both directions and allows residue removal | `make ze-test-pkg PKG=./scripts/dev` |
| The seven preserved behaviors still hold | The same run: the seven EXISTING tests are named in the TDD table |
| Every new test discriminates | Revert the change, re-run, and confirm AC-1, AC-2, AC-3 and AC-5 go red |
| The workaround is gone from the rendered rule | `grep -rn "Drop the shard from commit B" ai/` returns nothing |
| No rule still describes the old read | `grep -rn "reads the shard at HEAD" ai/` returns nothing |
| Rendered rules match their point files | `make ze-doc-test` |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Shard paths reach the gate already normalized by `rel_path`, which is what makes the `plan/deferrals/` prefix test sound. The tree read must use that same normalized path joined to `repo`, never a raw `args.remove` string |
| Authorization that could fail open | This IS the fail-open surface. The gate guards a destructive action whose failure is silent, so every branch that cannot read a view must report. AC-4 and AC-9 pin the two ways a view goes missing |
| Resource exhaustion | One extra small-file read per removed shard. Removals are rare, and `deferral_shard_paths` already globs the whole directory on every commit |
| Error leakage | The message prints row text from a repository file to a local operator. `git show` stderr is already truncated to 160 characters; the tree-read error path must truncate the same way |

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
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

- **A gate that reads HEAD is judging the wrong tree when the commit it gates is not the next one.** The helper prepares N commits before any runs. Every gate written as "compare against HEAD" is correct only for the FIRST block in a script, and silently wrong for the rest. This gate is one instance; the class is worth a sweep once this lands.
- **The same stale read produced opposite bugs.** Direction 1 passes what it should block, direction 2 blocks what it should pass. A reviewer looking only for the loosening would have found half of it. When a check reads the wrong snapshot, ask what it wrongly permits AND what it wrongly forbids.
- **`git rm` without `-f` is a load-bearing safety property, not a style choice.** It is what makes trusting the working tree sound here: a tree state no commit in the script carries cannot survive the script. If `render_block` ever gains `-f`, this gate's premise dies with it, and nothing would say so. A-2's test exists to make that coupling visible.
- **The existing test that most looks like an obstacle is the one that proves the design.** `test_deleting_the_working_copy_first_does_not_clear_the_gate` forbids a naive tree read, and its comment says so. A tree-preferred read with a HEAD fallback satisfies it untouched, which is a strong signal the fallback is the right shape rather than a patch over a failing test.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Read the working-tree copy when it exists, fall back to HEAD when it does not, and always read HEAD as well so the vanished-row guard and every unreadable-view branch survive | (a) Parse the prepared script's earlier `BLOCK_MARKER` provenance lines to learn what commit A stages, and trust the tree only then. (b) Reuse `extract_head_into` plus `apply_commit_overlay` to build a real prospective tree | (a) is more literally "what the commit will leave", but the marker merges `add_paths` and `remove_paths` into one list so it cannot tell an add from a remove, the gate runs before `resolve_target_script`, and a closure that appends commit B to a different script leaves no record at all. It buys nothing over the chosen read, because `git rm --` already refuses a tree state no commit carries (A-2). (b) cannot fix the bug: the overlay sees only THIS call's `add_paths`, so commit A stays invisible. It also costs a tar extraction of the whole tree per check, and `apply_commit_overlay` silently skips a missing source, which is the exact fail-open shape the AC-4 test forbids |
| Fix both failure directions in one change | Fix direction 1 only, spec direction 2 separately | One read produces both. Fixing one would mean touching the same function twice, and would leave a workflow `ai/rules/planning.md` mandates unreachable in the meantime. The Task's own goal covers both: "judges the shard as the COMMIT will leave it" |
| Add a vanished-row guard | Trust the tree copy outright | Trusting the tree introduces a bypass that does not exist today: delete the row's line rather than resolve it, and the removal passes. A newly added guard that fails open is always in scope at review (`ai/rules/planning.md`), so this is not optional scope |
| Keep the tests driven through `commit_gate_problems` | Call `deferral_shard_removal_problems` directly | The class docstring already requires it: a gate that works when called directly and is never wired into the assembly is the failure that idiom drives out. Only the two tests that mock `subprocess.run` call the function directly, and they keep doing so |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- **The gate still runs at `create` time only.** Nothing re-checks when the operator runs the script minutes or hours later, so a shard that gains a live row in between is removed anyway. `review_gate.py check` is the only run-time re-check the script carries. Closing this would mean embedding a re-check in `render_block`, which is a larger change to the script contract and is not attempted here.
- **The shared checkout hazard is mitigated, not removed** (R-1). Another session's uncommitted live row can block a closure. The escape is local and always available, but the gate can be reddened by work that is not yours.
- **Class-wide sweep not attempted.** The first Design Insight says every "compare against HEAD" gate in `commit_helper.py` is suspect for any block after the first. This spec fixes one gate. The sweep is a separate piece of work and gets its own spec if the insight holds up at review.

## RFC Documentation (Scope: protocol)

N/A. Scope is `tooling`. No protocol-implementing code is touched, so no
`// RFC NNNN Section X.Y:` annotation is owed.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
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
