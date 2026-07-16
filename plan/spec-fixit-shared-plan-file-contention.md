# spec-fixit-shared-plan-file-contention

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-16 |

> **DESIGN.** Research done 2026-07-16; Current Behavior below is traced to producing
> functions. Two claims carried from the skeleton were BROKEN by that research (A-3,
> A-5) and one was half-broken (A-4); superseded text is struck through with the
> reason, per `ai/rules/planning.md` "Editing: append-only". Not `ready`: awaiting
> approval of the shard model in "Shard Model (Decisions)".

## Task

Concurrent sessions cross-commit each other's rows in the shared plan files
(`plan/deferrals.md`, `plan/known-failures.md`, `plan/learned/.counter`). The
commit-script rule in `ai/rules/git-safety.md` fixes staging *timing*; it cannot fix
staging *granularity*, because `git add <file>` stages the whole file including hunks
another session left uncommitted in it. Shard the contended files so each session
writes only files it owns, and keep the aggregated view on demand instead of
committing it. Root cause is CONFIRMED (see Problem / Evidence); the target shard key
per file is UNVERIFIED and is the main research question.

## Origin

Found during `spec-fixit-migrate-sleeps-infra` work, 2026-07-15/16: this session's
`plan/deferrals.md` edits were carried into two unrelated VRRP commits (f9dc6132b,
49d83646b). The rule text added in 5130c26bc documents the hazard and the correct
reaction; this spec is the structural fix.

## Required Reading

### Source (read before designing)
- [ ] `scripts/dev/spec-session.sh` - the in-repo precedent: per-session markers,
      chosen so concurrent agents never collide. Read this FIRST; the fix is likely
      the same pattern applied to deferrals.
      -> Constraint: `.claude/rules/planning.md` states "There is no shared file, so
      many agents editing main concurrently never collide -- nothing to append,
      nothing to remove."
- [ ] `scripts/dev/commit_helper.py` - ~~machine-reads `plan/known-failures.md` for the
      `--unverified` gate and~~ reads `plan/learned/.counter` for `learned-next`.
      -> Decision: only the `.counter` half was true (A-5 broke the other). `learned_next`
      (`:1097-1127`) reads AND writes `.counter`; the `:374-376` gate forces it into every
      learned-summary commit. Those two sites are the entire `.counter` contention.
- [ ] `ai/rules/git-safety.md` "Shared plan files cross-commit" - the documented
      hazard and the four correct reactions.
      -> Constraint: `:55-56` is the corrected guarantee from 89aff42e8. `learned-next` is
      collision-free only for sessions **sharing one working tree**; across branches it
      collides by construction and `make ze-learned-numbers-check` is the backstop. This
      spec must not re-assert the unqualified guarantee that commit removed.
- [ ] `scripts/dev/learned_numbers.py` - added by 89aff42e8; owns the uniqueness,
      H1-matches-filename, and `.counter >= highest+1` invariants (`:93-124`).
      -> Decision: it is a DETECTOR and REPAIR tool, not a prevention (the commit does not
      touch `commit_helper.py`). It stays: it owns the cross-branch dimension that no
      shard layout can reach. Its invariant 3 is the reader that breaks A-4.
- [ ] `ai/rules/deferral-tracking.md` - owns the deferrals table format and the
      resolution rules. Any reshaping must keep "no open row without a Destination".

### Architecture Docs
- [ ] `ai/skills/ze-status.md` - already the documented live backlog view; likely the
      natural home for an on-demand aggregate.

## Current Behavior (MANDATORY)

**Source files (cite file:line):**

The root cause is a three-link chain in `commit_helper.py`. Each link is read below at
its producing function, not inferred from a caller (`ai/rules/no-fabrication.md`).

- [ ] `scripts/dev/commit_helper.py` lines 1097-1127 - `learned_next()`, the producer of
      the `.counter` write. `highest` is globbed from existing `[0-9]*-*.md` filenames
      (`:1112-1115`); `counter` is read at `:1116-1119` with an `except (OSError,
      ValueError)` branch yielding `0`; the number is `max(highest + 1, counter)`
      (`:1120`); the file is created `os.O_EXCL` (`:1122`); and **`:1125` writes
      `.counter` back**.
      -> Decision: `:1125` is the ONLY writer that dirties `.counter`. The contention is
      manufactured entirely inside this one function plus the gate below. Nothing else
      in the repo writes it (`grep '\.counter'` over `scripts/ mk/ Makefile ai/ .claude/`
      returns only readers, tests, docs, and `rebase_learned.py`'s conflict resolver).
      -> Constraint: LIVE PROOF, this working tree, right now: `.counter` is modified
      `1156 -> 1157` next to a new untracked `plan/learned/1156-rebase-learned-driver.md`.
      That diff is `:1125` firing. It is not an edit anyone chose to make.
- [ ] `scripts/dev/commit_helper.py` lines 374-376 - the gate inside `lesson_line()`:
      `if learned: if not existing_lesson and "plan/learned/.counter" not in add_paths:
      raise UsageError(...)`. This is the second link: it FORCES every session with a new
      learned summary to stage the shared counter.
      -> Decision: `:1125` dirties the file, `:375-376` compels staging it, and
      `git add` stages whole files. That is the complete cross-commit mechanism for
      `.counter`. Remove either link and the contention for this file ends.
- [ ] `scripts/dev/commit_helper.py` lines 1120 + 1112-1115 - `.counter` is **redundant
      state**. `highest` is derived from the filenames, which are themselves the record.
      `max(highest+1, counter)` with a missing counter (`= 0`) yields `highest + 1`, the
      same answer. The counter is a cache of a value the directory already holds.
- [ ] `scripts/dev/commit_helper.py` lines 1122 + 1236-1239 - the `O_EXCL` create has **no
      `try`/`except`**, and `main()` catches only `UsageError`. A same-tree concurrent
      loser therefore does not retry and does not allocate the next number: it dies with
      an uncaught `FileExistsError` traceback.
      -> Constraint: `O_EXCL` prevents a duplicate by crashing the loser, not by
      resolving. The docstring at `:1098-1104` says allocation is collision-free; that is
      true in-tree and the crash is why. Any redesign must keep the mutual exclusion and
      should add a bounded retry rather than inherit the traceback.
- [ ] `scripts/dev/learned_numbers.py` lines 93-124 - `check()`, added by 89aff42e8. Three
      invariants: numbers unique (`:100-103`), H1 matches filename (`:105-115`), and
      **`.counter >= highest + 1`** (`:117-123`), where `counter_value()` (`:86-90`)
      returns `0` for a missing file.
      -> Constraint: this invariant is a NEW consumer of `.counter` that did not exist
      when the skeleton judged A-4 "confirmed, nothing breaks". Deleting `.counter`
      makes `counter_value` return `0`, `check()` reports `.counter is 0 but the highest
      summary is NNNN`, and the gate goes red. A-4 is downgraded to `broken` below.
- [ ] `Makefile:423`, `mk/inventory.mk:94`, `:130`, `:137-138` - the four invocations of
      `learned_numbers.py --check` (`ze-doc-test`, `ze-regen-check`,
      `ze-discovery-index-check`, standalone `ze-learned-numbers-check`). All four go red
      if `.counter` is deleted without also dropping invariant 3.
- [ ] `scripts/dev/python_tests_test.go` lines 37-68 - `TestPythonUnitTests` globs
      `scripts/dev/*_test.py` and runs each under `go test`.
      -> Decision: a new `<tool>_test.py` is auto-wired into `make ze-unit-test` with no
      make target and no list edit. The TDD plan below needs no wiring work.
- [ ] `scripts/dev/commit_helper_test.go` lines 177-192 - a Go test that stages
      `plan/learned/.counter` and asserts the `:375-376` gate passes. It is a reader of
      the gate and must change with it.
- [ ] `scripts/dev/learned_numbers_test.py` lines 105-113 - `test_stale_counter_is_reported`,
      asserting invariant 3. Must be retired with invariant 3.
- [ ] `ai/skills/ze-status.md` line 25 - step 4 in full: "**Deferrals:** Read
      `plan/deferrals.md`. Count open items. List any that reference the selected spec."
      -> Constraint: `ze-status` is an **LLM skill (a markdown prompt), not a program**.
      There is no renderer function anywhere to extend. "Render an aggregate from shards"
      is a one-line prompt edit (read a glob instead of a file), which is nearly free;
      but it also means there is NO machine-readable aggregate for any non-LLM consumer.
      A-3's basis was wrong (it assumed a program). See A-3 below.
- [ ] `plan/deferrals.md` lines 1-14 + row count - 47 table rows, of which 7 carry `ad-hoc` in
      the Source column. The header states a row "lives here only while the work has no
      home" and directs readers to `/ze-status` for the live backlog.
      -> Decision: the committed table is already declared a staging area, not the
      system of record. Sharding it is consistent with its own stated design.
- [ ] `plan/known-failures.md` lines 1-12 - single tracked file; scope is non-deterministic
      test reds only; entries are appended by one session and later CORRECTED by another.

**~~`scripts/dev/commit_helper.py` lines 997-1017 - the `--unverified` path that
machine-reads `plan/known-failures.md`~~** -- **STRUCK: FALSE, and it was the premise of
AC-5.** `plan/known-failures.md` is **never machine-read by anything**. A repo-wide grep
over `scripts/`, `.claude/hooks/`, `mk/`, `Makefile` returns six hits and every one is a
comment or a help string: `commit_helper.py:490` (comment), `:997`, `:1005`, `:1017`,
`:1028` (comment + error text), `:1216` (the `--unverified` `help=` string), and
`commit_helper_test.go:262` (comment). The gate at `:1022-1029` tests only
`if not args.unverified` -- the truthiness of a free-text reason string. Nothing parses
the file, resolves a test name in it, or verifies the reason corresponds to a logged red.
-> Decision: known-failures is pure honor-system prose for humans and agents. This CUTS
scope (no machine reader to migrate, contradicting the skeleton's "6+ readers" cost) and
INVALIDATES AC-5 and `test_unverified_finds_sharded_known_failure`, which tested a lookup
that does not exist.

**What 89aff42e8 does and does not fix (asked explicitly; answered from the diff):**

`git show --stat 89aff42e8` shows it **does not touch `scripts/dev/commit_helper.py`**.
It added `learned_numbers.py` (`--check`/`--fix`), renumbered 26 colliding files, and
corrected the over-broad guarantee in `git-safety.md:55-56`.

| Dimension | Before 89aff42e8 | After | Verdict |
|-----------|------------------|-------|---------|
| Same-tree concurrent allocation | `O_EXCL` at `:1122` excludes the loser (by crash) | unchanged | Was already prevented; still is |
| Cross-BRANCH allocation | Collides by construction: `:1112-1115` globs the LOCAL tree and cannot see a number on an unmerged branch | Still collides by construction | **NOT fixed. Prevention unchanged.** |
| Detecting a collision | Nothing. `learned_index.py` globbed and rendered two rows with the same number | `--check` at `:100-103`, wired into 4 gates | **Fixed (detection)** |
| Repairing a collision | By hand | `--fix` at `:170` | **Fixed (repair)** |
| `.counter` staging contention | `:375-376` forces it | `:375-376` unchanged; `:117-123` ADDS a consumer | **Not fixed; made slightly worse** |

-> Decision: **89aff42e8 is a detector plus a repair tool, not a prevention.** It closes
the "nothing noticed" hole (the real damage: 22 numbers, 26 excess files, undetected back
to 477) and it is the correct response to the cross-branch case, which no local allocator
can prevent. It leaves this spec's problem intact: the allocation still races by
construction, and the `.counter` staging contention this spec targets is untouched.
-> Constraint: therefore this spec must NOT propose replacing `--check`/`--fix`. Those
handle the cross-branch dimension, which single-writer sharding cannot reach (two branches
are two trees; there is no shared filesystem to be atomic over). Sharding kills the
`.counter` cross-commit; `--check`/`--fix` stays as the cross-branch backstop. The two are
complementary, not alternatives.

**Behavior to preserve:**

**Behavior to preserve:**
- Every open deferral still names a Destination (`ai/rules/deferral-tracking.md`).
- ~~`commit_helper.py --unverified` still refuses unless the red is logged as a known
  failure.~~ **STRUCK: it never did this (A-5).** `:1022` refuses only when
  `--unverified` is absent; the reason is free text and is never checked against
  `plan/known-failures.md`. What MUST be preserved is the real gate:
  `structural_gate_reds()` (`:1009-1021`) refuses `--unverified` outright while a
  deterministic structural gate is red. That code reads `tmp/ze-verify-failures.json`,
  not the plan file, so sharding cannot touch it.
- Learned numbers stay unique within a working tree (`O_EXCL`, `:1122`) and duplicates
  across branches stay detectable (`learned_numbers.py --check`, invariants 1-2).
- Existing greppability: a human or agent must still be able to find "all open
  deferrals" and "is this test a known red" in one step.

**Behavior to change:**
- A session must be able to stage its own deferral/known-failure record without
  staging any other session's pending edits.

## Shard Model (Decisions)

The research question was "what is the shard key per file". Answered below.

**The invariant that makes this work, stated once:** *every path has exactly one writer
at any time, and every shared view is a pure fold over a directory, computed on read and
never stored.* A lock exists to stop two writers touching one cell. Delete the shared
cell and there is nothing to lock. Git already merges disjoint file creations without
conflict; that is the whole mechanism.

-> Decision: **the aggregate is never tracked, never generated to a file, never staged.**
It is computed at read time by whoever asks. This is R-1 and it is the load-bearing
constraint of the spec: a tracked aggregate is just the contended file again, with extra
steps.

| # | Question | Answer |
|---|----------|--------|
| 1 | What is the shard unit? | Per file: see the table below. Never "per session" except where the record genuinely belongs to the session. |
| 2 | Who writes a shard? | Exactly one session: the one that owns the record. Ownership is defined per file below and is a property of the RECORD, not of who is typing. |
| 3 | Who aggregates, and when? | The reader, at read time, on demand. `/ze-status` for humans/agents; a `--list` mode for machines if one is ever needed. Nobody aggregates at write time. |
| 4 | How does the aggregate stay correct without a lock? | It has no state to be incorrect. It is `fold(sort(glob(dir)))` over immutable-once-written files. Determinism comes from the sort key, not from ordering of writes. |

### Shard key per file

| File | Shard unit | Owner (single writer) | Aggregate | Why not per-session |
|------|-----------|----------------------|-----------|---------------------|
| `plan/learned/.counter` | **none: delete the file.** The shard already exists and is the `NNNN-slug.md` summary itself | the allocating session, via the existing `O_EXCL` create at `commit_helper.py:1122` | `max(glob)`, already computed at `:1112-1115` and by `learned_numbers.py` | N/A. The counter is a redundant cache of `max(glob)`; there is nothing to shard, only a cache to drop |
| `plan/deferrals.md` | **per source spec**: `plan/deferrals/<spec-stem>.md`, plus `plan/deferrals/ad-hoc-<YYYY-MM-DD>-<sid>.md` for the 7 rows whose Source is `ad-hoc` | the session holding that spec (`ai/rules/planning.md`: "One spec at a time per session") | `/ze-status` globs `plan/deferrals/`, sorts by (Date, Source) | A session id is meaningless once the session ends (R-4 garbage) and does not survive into the record's natural life. The `Source` column already IS the key |
| `plan/known-failures.md` | **per failure**: `plan/known-failures/<make-target>-<test-name>.md` | whoever logs the failure creates it; whoever corrects or clears it edits THAT file | `/ze-status` globs the directory | Per-session is WRONG and the skeleton already said so: it files a correction under the corrector instead of on the failure. Per-failure makes the correction land on the record it is about |

-> Decision: chose **per-source-spec** over per-row for deferrals. Per-row gives maximal
isolation but 47 files today and unbounded growth (R-4). Per-source-spec bounds the
directory by the number of open specs, which is already bounded and already garbage
collected (spec closure deletes the spec). The residual contention (a session resolving
another spec's deferral row) is real but is a genuine conflict about a genuine record,
not the false sharing we are removing, and it is narrowed from one file for the whole
repo to one file per spec.
-> Constraint: a deferral shard must be deleted by the spec-closure commit (commit B)
that removes its spec, or it becomes R-4 garbage. Closure already deletes
`plan/spec-<stem>.md`; it must also delete `plan/deferrals/<stem>.md`.

-> Decision: `.counter` deletion is the **cheapest and most separable of the three** and
should land FIRST, alone. It is the only one with a live, reproducible, currently-dirty
instance in this working tree, it needs no new file format, no reader migration, and no
aggregate: the aggregate (`max(glob)`) already exists in two places. It is a deletion
plus the removal of the gate that requires the deleted thing.
-> Constraint: it is NOT free, contrary to the skeleton's A-4. It requires dropping
invariant 3 (`learned_numbers.py:117-123`), its test (`learned_numbers_test.py:105-113`),
the gate (`commit_helper.py:374-376`), the `:1125` write, the `--existing-lesson` flag's
`.counter` rationale (`:1205-1207`), and the Go test's staging of it
(`commit_helper_test.go:177-192`), plus the rule text at `ai/rules/planning.md:206`,
`:214`, `:344` and `ai/rules/git-safety.md:55`, `:107`, `:333-334`. Six code sites, five
rule sites. Still small, but it is a migration, not a `git rm`.

### What sharding does NOT fix

-> Constraint: sharding is a same-tree mechanism. Two branches are two trees with no
shared filesystem, so no local allocator and no shard layout can prevent a cross-branch
learned-number collision. That dimension belongs to `learned_numbers.py --check`/`--fix`
(89aff42e8) and stays. Deferral and known-failure shards do not have this problem: their
keys (spec stem, test name) are semantically unique, so two branches creating the same
path are describing the same record and SHOULD conflict.

## Problem / Evidence

**CONFIRMED.** `git add <file>` stages the whole file. `plan/deferrals.md` is a single
file that every session appends to, so whoever commits first carries everyone else's
pending rows into their commit.

Observed twice on 2026-07-15/16: this session's `deferrals.md` edits (the
`spec-migrate-sleeps-infra` -> `spec-fixit-migrate-sleeps-infra` rename, then 7 rows
resolved to their receiving specs) landed inside f9dc6132b ("feat: add VRRP") and
49d83646b ("spec: vrrp-6 review gate"). The content is correct and preserved; only the
attribution is wrong, and it is not worth rewriting history to reclaim.

**CONFIRMED: this is not misconduct, and the obvious explanation is false.** Forensics
over `tmp/commit-*.sh` found no script using a broad add (`add -A`, `add .`, or
`commit -a`). Three concurrent sessions (ping, ipc, lg) each had `plan/deferrals.md`
in their own explicit `--file` list at the same time. Every one followed
`ai/rules/git-safety.md` correctly. The cross-commit is structural.

**CONFIRMED: the three files do not share one access pattern**, so one shard shape
probably does not fit all three:

| File | Pattern | Consequence for sharding |
|------|---------|--------------------------|
| `plan/deferrals.md` | pure append; each row owned by the appending session; the table already carries a `Source` column naming the owning spec | a per-source-spec (or per-session) shard fits directly |
| `plan/known-failures.md` | append PLUS later correction of other sessions' entries | a per-session shard is WRONG: it would file a correction under the corrector instead of fixing the entry. Per-failure (one file per test/suite) is the candidate |
| `plan/learned/.counter` | allocation, not append | the `NNN-*.md` files are already one-per-session and never collide; only the counter is shared, and it duplicates information already in the filenames |

**UNVERIFIED.** Whether `/ze-status` reads the file or can read a directory; whether
any consumer parses the deferrals table positionally; the real cost of the reader
migration (8+ referencing files for deferrals, 6+ for known-failures, including the
machine-read `commit_helper.py` gate).

## Data Flow

### Entry Point
A session records a deferral, logs a known failure, or allocates a learned number
while preparing a commit script.

### Transformation Path
Today: session edits the single shared file -> `git add <file>` stages the entire
file, including other sessions' pending hunks -> `git commit` carries them.
Candidate: session writes `plan/deferrals/<owner>.md` -> `git add` touches only that
path -> aggregate rendered on demand by `/ze-status`.

### Boundaries Crossed
| Boundary | Crossing | Consequence of divergence |
|----------|----------|---------------------------|
| Working tree -> git index | `git add <file>` stages the WHOLE file, not the session's hunks | Another session's pending rows enter this session's commit. This boundary IS the defect |
| Session A -> Session B (concurrent) | A single tracked file is the shared medium | First committer carries everyone's rows; attribution is lost and cannot be reclaimed |
| `commit_helper.py` -> plan records | ~~Machine-reads known-failures for `--unverified`;~~ (STRUCK, A-5: nothing reads it; `:1022` tests only the reason string's truthiness) requires staging `.counter` (`:374-376`) | Only the `.counter` gate is a real code dependency. Known-failures sharding is text-only |
| `learned_next()` -> `.counter` | `:1125` writes the counter as a side effect of allocating | This write is the SOURCE of the dirty file. No human chose it; it is why `.counter` is modified in this tree right now |
| Session tree -> other branches | `:1112-1115` globs the LOCAL tree only | Cross-branch numbers collide by construction. Unreachable by sharding; owned by `learned_numbers.py --check`/`--fix` (89aff42e8) |
| Skills (`ze-status`/`ze-progress`/`ze-debrief`) -> plan records | Read the aggregate view | Sharding without an aggregate loses "what is open" in one step |

### Integration Points
`scripts/dev/commit_helper.py` (known-failures gate, learned-next), `ai/skills/
ze-status.md`, `ai/skills/ze-progress.md`, `ai/skills/ze-debrief.md`,
`ai/rules/deferral-tracking.md`.

## Wiring Test

Note: `.ci` functional tests are N/A for this spec. `.ci` covers the ze product; this
is agent tooling under `scripts/dev/` and `plan/`, which has no `.ci` surface. The
test names below are intended targets to be created by this work.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| A session records a deferral, then runs its commit script | -> | `plan/deferrals/<spec-stem>.md`, written by the spec-holding session only | `test_two_sessions_no_cross_commit` |
| `/ze-status` step 4 | -> | the glob in `ai/skills/ze-status.md:25` (a PROMPT, not a function: A-3) | `test_aggregate_renders_all_shards` (fixture + documented rendering; see the TDD note) |
| ~~`commit_helper.py --unverified` -> known-failures lookup, `:997-1017`~~ | -> | **STRUCK (A-5): no such lookup exists.** `:1022` tests `if not args.unverified` and nothing more | AC-5' asserts the migration is a no-op for code |
| `commit_helper.py learned-next <slug>` | -> | `learned_next()`, `commit_helper.py:1097-1127`, minus the `:1125` counter write, plus a retry at `:1122` | `test_learned_next_unique_without_counter`, `test_learned_next_retries_on_existing` |
| `commit_helper.py create --file plan/learned/NNNN-x.md` | -> | `lesson_line()` gate, `commit_helper.py:374-376` | `test_learned_summary_commits_without_counter` (AC-6) |
| `make ze-learned-numbers-check` | -> | `check()`, `learned_numbers.py:93-124`, minus invariant 3 | `test_check_passes_without_counter` (AC-7) |

## 🧪 TDD Test Plan

### Unit Tests
Wiring note: `python_tests_test.go:37-68` globs `scripts/dev/*_test.py` and runs each
under `go test`, so every test below is picked up by `make ze-unit-test` with no make
target and no list edit (A-7 confirmed).

| Test | File | Validates |
|------|------|-----------|
| ~~`test_unverified_finds_sharded_known_failure`~~ | ~~`scripts/dev/commit_helper_test.py`~~ | **STRUCK with AC-5.** A-5 broke: there is no known-failures lookup to test. Writing this test would have asserted invented behavior and passed vacuously |
| `test_learned_next_unique_without_counter` | `scripts/dev/commit_helper_test.py` (new; `learned_next` lives in `commit_helper.py`, not `learned_index.py` -- the skeleton put it in the wrong file) | AC-4: two allocations in one tree with no `.counter` present yield distinct numbers, relying solely on the `O_EXCL` create at `:1122` |
| `test_learned_next_retries_on_existing` | `scripts/dev/commit_helper_test.py` | AC-4/R-6: pre-create the target `NNNN-slug.md`, assert the allocator advances to the next free number instead of raising `FileExistsError`. RED today (`:1122` has no handler; `main()` catches only `UsageError`) |
| `test_learned_summary_commits_without_counter` | `scripts/dev/commit_helper_test.go` (amends `:177-192`, which stages `.counter` today) | AC-6: the `:374-376` gate no longer demands `plan/learned/.counter` |
| `test_check_passes_without_counter` | `scripts/dev/learned_numbers_test.py` (replaces `test_stale_counter_is_reported` at `:105-113`) | AC-7/R-7: `check()` is green with no `.counter`, and still reports a seeded duplicate number and a seeded H1 mismatch |
| `test_aggregate_renders_all_shards` | see note | AC-2: N shard files render one ordered view with no row dropped. **Note: `/ze-status` is a prompt, not a program (A-3), so there is no function to unit-test.** This is a fixture directory plus a documented expected rendering, verified by running the skill. If AC-2 needs a real automated test, it needs a real renderer, which is a scope increase requiring approval |
| `test_two_sessions_no_cross_commit` | `scripts/dev/commit_helper_test.py` | AC-1: two commit scripts, each `--file`-ing only its own shard, produce two commits with disjoint file lists while both shards are dirty. This is the regression test for the whole spec |

### Functional Tests
N/A. This is agent tooling under `scripts/dev/` and `plan/`; it has no `.ci` surface
(the `.ci` suites cover the ze product). The wiring is proven by the unit tests above
plus the two-session sequence in the Wiring Test table.

## Files to Modify

Grounded by the greps in Current Behavior. ~~known-failures lookup~~ is struck: there is
no lookup (A-5).

**Phase 1 (`.counter`, self-contained):**
- [ ] `scripts/dev/commit_helper.py` - delete the `.counter` write (`:1125`); delete the
      staging gate (`:374-376`); wrap the `O_EXCL` create (`:1122`) in a bounded retry
      (R-6); simplify `:1116-1120` to `highest + 1`; revisit the `--existing-lesson`
      help text (`:1205-1207`), whose entire rationale is `.counter`
- [ ] `scripts/dev/learned_numbers.py` - remove invariant 3 (`:117-123`) and
      `counter_value()` (`:86-90`), now dead
- [ ] `scripts/dev/learned_numbers_test.py` - replace `test_stale_counter_is_reported`
      (`:105-113`)
- [ ] `scripts/dev/commit_helper_test.go` - amend the `.counter` staging fixture
      (`:177-192`)
- [ ] `scripts/dev/rebase_learned.py` - drop `.counter` from `BOOKKEEPING` (`:69`, `:71`)
      and its resolution rule (`:16`); the file it reconciles will not exist. **This is
      Thomas's uncommitted file; coordinate before touching it**
- [ ] `ai/rules/planning.md` - `:206`, `:214`, `:344` all instruct bumping/staging `.counter`
- [ ] `ai/rules/git-safety.md` - `:55` (the allocation guarantee), `:107` (the `--file`
      example), `:333-334` (the pre-commit checklist's two counter lines)
- [ ] `ai/INDEX.md` - `:186`, the `learned_numbers.py` row, describes invariant 3
- [ ] `plan/learned/.counter` - delete (last, after every reader above)

**Phase 2 (deferrals):**
- [ ] `ai/rules/deferral-tracking.md` - `:8` names `plan/deferrals.md` as "the single
      source of truth"; record location and per-shard format
- [ ] `ai/skills/ze-status.md` - `:25`, step 4: read the glob, not the file
- [ ] `ai/rules/planning.md` - Spec Closure must also delete `plan/deferrals/<stem>.md`
      in commit B (R-4)
- [ ] `plan/deferrals.md` -> `plan/deferrals/` (40 rows by spec stem, 7 ad-hoc)

**Phase 3 (known-failures):**
- [ ] `plan/known-failures.md` -> `plan/known-failures/` (per failure)
- [ ] `scripts/dev/commit_helper.py` - `:1216` help string and the `:997`-`:1028`
      comments/error text reference the path. **Text only; no logic**
- [ ] `ai/rules/git-safety.md` - `:207`, `:212`, `:229`, `:288`, `:304`

## Implementation Steps

~~Step 1 (pick the shard key)~~ is done: see "Shard Model (Decisions)". The three files
are independent and ship as three separate commits, cheapest and most-proven first.

**Phase 1 -- retire `plan/learned/.counter`. Ship alone, first.**
Independent of phases 2-3, has a live reproducible instance in this tree right now, needs
no new format and no aggregate (`max(glob)` already exists at `commit_helper.py:1112-1115`
and in `learned_numbers.py`).
1. RED: `test_learned_next_retries_on_existing`, `test_learned_next_unique_without_counter`.
2. Make the allocator counter-free and retrying (`:1112-1125`, R-6).
3. Drop the staging gate (`:374-376`); amend `commit_helper_test.go:177-192`.
4. Drop invariant 3 + `counter_value()` from `learned_numbers.py`; replace its test.
5. Update the 5 rule/index sites, then `git rm plan/learned/.counter` LAST.
6. Verify: `make ze-learned-numbers-check`, `make ze-doc-test`, `make ze-regen-check`,
   `make ze-unit-test` (all four `--check` call sites, R-7).
   -> Constraint: `rebase_learned.py` is uncommitted work of Thomas's that resolves
   `.counter` rebase conflicts. Deleting `.counter` makes half of it dead. Coordinate;
   do not edit it unilaterally (`ai/rules/never-destroy-work.md`).

**Phase 2 -- shard `plan/deferrals.md` per source spec.**
1. Teach `/ze-status` to glob `plan/deferrals/` AND fall back to `plan/deferrals.md`
   while both exist (R-2: aggregate first, migrate readers, retire the file last).
2. RED: `test_two_sessions_no_cross_commit`.
3. Migrate 40 rows to `plan/deferrals/<spec-stem>.md`, 7 to `ad-hoc-<date>-<sid>.md`.
4. Update `ai/rules/deferral-tracking.md:8`; add the commit-B shard deletion to
   `ai/rules/planning.md` Spec Closure (R-4).
5. Retire `plan/deferrals.md` and the `/ze-status` fallback.

**Phase 3 -- shard `plan/known-failures.md` per failure.**
Cheapest of the three now that A-5 is broken: no machine reader exists, so this is a file
move plus text edits. Sequenced last only because it is the least urgent (its cross-commit
rate is lowest: entries are rare).
1. Migrate each entry to `plan/known-failures/<make-target>-<test-name>.md`.
2. Update `/ze-status`, the `:1216` help string, the `:997`-`:1028` comment text, and the
   5 `git-safety.md` sites. AC-5' asserts no behavior changes, because no code reads it.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two sessions concurrently record a deferral, then each commits | Each commit contains only its own record; neither carries the other's |
| AC-2 | An agent asks "what is open" | `/ze-status` answers in one step across all shards, sorted deterministically by (Date, Source), with no row dropped. Scope: the LLM-skill reader (A-3 broke the premise of a programmatic renderer). A machine-readable `--list` is NOT in scope; no consumer needs one today (`ai/rules/no-fabrication.md`: do not build for an unverified consumer) |
| AC-3 | A session corrects a known failure first logged by another session | The correction lands on the failure's record, not filed under the corrector |
| AC-4 | Two sessions allocate a learned number concurrently in one working tree | Numbers are unique with no `.counter` file present, and the losing session RETRIES to the next free number rather than raising `FileExistsError` (R-6) |
| AC-5 | ~~`commit_helper.py --unverified` with a logged known red -> still passes; still refuses when the red is not logged~~ | **STRUCK: tests behavior that does not exist.** A-5 broke: nothing machine-reads `plan/known-failures.md`; `:1022` tests only the truthiness of the `--unverified` reason string. There is no lookup to preserve. Replaced by AC-5' |
| AC-5' | `commit_helper.py --unverified "<reason>"` after `plan/known-failures.md` becomes `plan/known-failures/` | Behaves exactly as today (free-text reason accepted; structural-gate reds still refused via `structural_gate_reds()` at `:1009`). Proves the migration touches no code path, because none reads the file |
| AC-6 | Any commit adding a new `plan/learned/NNNN-<slug>.md` | Succeeds WITHOUT staging `plan/learned/.counter`, because the file no longer exists and the gate at `:374-376` no longer demands it |
| AC-7 | `make ze-learned-numbers-check` with `.counter` deleted | Green. Invariants 1 (uniqueness) and 2 (H1 matches filename) still fire on a seeded duplicate; invariant 3 is gone with the file it described (R-7) |

## Risks & Assumptions

### Assumptions

All cheap assumptions were validated by grep/read on 2026-07-16, per
`ai/rules/planning.md` ("Validate cheap ones (grep/read) during the audit, before
coding"). Two came back broken. Evidence is in Current Behavior.

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | ~~Per-session sharding is the right pattern~~ **Superseded: per-RECORD sharding is. Session identity is the wrong key for every one of the three files** | `spec-session.sh` shards per session because a spec CLAIM is genuinely session-scoped and dies with the session. A deferral, a known failure and a learned number all outlive their session, so they key on the record (spec stem, test name, summary number), not the author | Would have produced R-4 garbage directories and, for known-failures, filed corrections under the corrector | Read `spec-session.sh` header vs. the three records' lifetimes | **broken** (usefully: the precedent is real but its rationale does not transfer) |
| A-2 | The deferrals `Source` column is a usable shard key | Every row names a source spec or `ad-hoc` | Ad-hoc rows need a different key | Counted rows in `plan/deferrals.md`: 47 table rows, 7 contain `ad-hoc` | **confirmed with a caveat**: usable for 40/47; the 7 ad-hoc rows need the `ad-hoc-<date>-<sid>` fallback named in the Shard Model |
| A-3 | ~~`/ze-status` can render an aggregate from a directory~~ | assumed `/ze-status` was a program with a renderer | An aggregate generator is needed elsewhere | Read `ai/skills/ze-status.md` | **broken, in our favour**: `ze-status` is an LLM skill (a markdown prompt), not a program (`ai/skills/ze-status.md:25` = "Read `plan/deferrals.md`. Count open items."). There is no renderer to extend; the change is a one-line prompt edit. But there is also no machine-readable aggregate for any non-LLM consumer, so AC-2 must specify which reader it means |
| A-4 | ~~`.counter` can be dropped safely. "Nothing [breaks]; the counter only raises the floor"~~ | read `learned_next()` only | The `.counter` work is a migration, not a deletion | Grepped every `.counter` reader repo-wide | **broken**: the allocation half stands (`:1116-1120` tolerates absence), but the "nothing breaks" half is false. `learned_numbers.py:117-123` (added by 89aff42e8, AFTER the skeleton was written) asserts `.counter >= highest+1`; `counter_value():86-90` returns 0 when absent, so `--check` reports a problem and 4 gates go red (`Makefile:423`, `mk/inventory.mk:94`, `:130`, `:138`). Plus `learned_numbers_test.py:105-113` and `commit_helper_test.go:177-192`. Root cause of the miss: only the producer of the ANSWER was read, not the other consumers of the FILE |
| A-5 | ~~`commit_helper.py --unverified` machine-reads `plan/known-failures.md`~~ (carried implicitly by AC-5, the Wiring Test, Data Flow, and the "6+ readers" cost) | the skeleton cited `commit_helper.py:997-1017` for it | AC-5 and one unit test are testing a lookup that does not exist; known-failures sharding is cheaper than costed | Grepped `known-failures` across `scripts/`, `.claude/hooks/`, `mk/`, `Makefile`: 6 hits, all comments or `help=` strings | **broken**: nothing parses the file. The gate at `:1022` tests only `if not args.unverified`, the truthiness of a free-text reason. `plan/known-failures.md` is honor-system prose |
| A-6 | Deleting `.counter` does not reintroduce the same-tree allocation race | the mutual exclusion is the `O_EXCL` create at `:1122`, not the counter; the counter only raises the floor | The `.counter` deletion is unsafe and the whole first phase is wrong | Read `:1112-1125` | **confirmed**, with a defect found: `O_EXCL` at `:1122` has no `try`/`except` and `main():1236-1239` catches only `UsageError`, so the losing session gets an uncaught `FileExistsError` traceback instead of retrying. Mutual exclusion holds; the ergonomics do not. See R-6 |
| A-7 | A new `scripts/dev/*_test.py` needs no wiring to run in CI | - | The TDD plan needs a make target | Read `python_tests_test.go:37-68`: globs `*_test.py`, runs each under `go test`, and fails loudly on an empty glob | **confirmed** |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A generated aggregate gets COMMITTED, reintroducing the contention unchanged | The aggregate appears in a commit script's `--file` list | Keep the aggregate on demand only; never track it. This is the single most important constraint in this spec |
| R-2 | Migration breaks the 8+ deferrals and 6+ known-failures readers | Rules or skills reference a path that no longer exists | Build the aggregate first, migrate readers, retire the file last |
| R-3 | Sharding trades cross-commits for lost visibility (nobody reads N files) | Agents stop recording deferrals | AC-2: one step must still answer "what is open" |
| R-4 | Per-session shards accumulate as garbage once sessions end | The directory grows without bound | ADDRESSED by the Shard Model: no record is keyed by session. Deferral shards are keyed by spec stem and deleted by the spec's own closure commit B; known-failure shards are keyed by test name and deleted when the red is cleared. The 7 ad-hoc deferral shards are the only session-flavoured key and carry a date for triage |
| R-5 | Two sessions work the same spec, so "one writer per deferral shard" does not hold | Two `tmp/session/.session-<SID>` markers name the same spec | `spec-session.sh` claims are markers, not exclusive locks (`.claude/rules/planning.md`: "no shared file, nothing to append"), so this is possible. Accepted: it degrades to one small per-spec file, never to the repo-wide file we have today. Not worth a locking scheme |
| R-6 | The `.counter` deletion leaves the `O_EXCL` loser crashing with a traceback (A-6) | A session reports `FileExistsError` from `learned-next` | Wrap `:1122` in a bounded retry that re-globs and re-allocates. In scope for this spec: removing `.counter` makes `:1122` the SOLE allocation mechanism, so its ergonomics stop being cosmetic |
| R-7 | Dropping invariant 3 from `learned_numbers.py` weakens the 89aff42e8 gate | `--check` stops reporting a class of problem it reports today | Invariant 3 (`.counter >= highest+1`) is meaningless once `.counter` does not exist; it is not a weakening but a removal of a check on a deleted file. Invariants 1 (uniqueness) and 2 (H1 matches filename), which are what 89aff42e8 exists for, are untouched. Verify by running `make ze-learned-numbers-check` after removal |

## Open Questions

### Answered by research (2026-07-16)

- ~~Shard key for `plan/deferrals.md`: source spec or session id?~~ **Source spec.** The
  `Source` column already is the key (40/47 rows); the 7 `ad-hoc` rows take
  `ad-hoc-<date>-<sid>`. Session id rejected: a deferral outlives its session, so a
  session key produces R-4 garbage and the spec-session precedent does not transfer (A-1
  broken). See "Shard Model".
- ~~Is `plan/known-failures.md` better sharded per failure than per session?~~ **Per
  failure**, confirmed and now cheap: A-5 broke, so there is no machine reader to migrate.
- ~~Can `plan/learned/.counter` simply be deleted? Does anything else read it?~~
  **Yes to deletion, but NOT "simply" -- A-4 is broken.** The allocation tolerates absence
  (`:1116-1120`), but 89aff42e8 added `learned_numbers.py:117-123` asserting
  `.counter >= highest+1`, wired into 4 gates, AFTER this skeleton was written. Deleting
  the file without dropping invariant 3 turns `ze-doc-test`, `ze-regen-check`,
  `ze-discovery-index-check` and `ze-learned-numbers-check` red. Six code sites, five rule
  sites. Still the cheapest of the three; still lands first; not a `git rm`.
- ~~Does anything parse these files positionally rather than by grep?~~ **Nothing parses
  any of the three at all.** `.counter` is read as a single int (`:1117`, `learned_numbers.py:88`);
  `known-failures.md` is never read (A-5); `deferrals.md` is read only by an LLM prompt
  (`ze-status.md:25`). There is no positional parser to break.
- ~~Should the aggregate be a command only, or a generated untracked file?~~ **Command
  only.** A generated file is a cache, and a cache of a shared view is what `.counter`
  was: `learned_next:1125` wrote it, `:374-376` forced it into a commit, and that is the
  entire bug. Generating the aggregate to a file recreates that shape one layer up. Fold
  on read, never store (R-1).

### Needs Thomas's decision (blocking `ready`)

1. **Approve the shard model** in "Shard Model (Decisions)": per-record keys (spec stem /
   test name / summary filename), single writer per path, aggregate as an unstored fold.
2. **Approve the phase split into three independent commits**, phase 1 (`.counter`) first
   and alone. Phases 2-3 touch `plan/` files with your uncommitted edits right now.
3. **`rebase_learned.py` is yours and uncommitted.** Half of it (`:16`, `:69`, `:71`)
   exists to resolve `.counter` rebase conflicts. Phase 1 deletes `.counter` and makes
   that half dead. Do you want phase 1 to update it, or will you?
4. **AC-2 scope.** `/ze-status` is a prompt, not a program (A-3), so "one step answers
   what is open" is satisfied by a prompt edit and cannot be automatically tested. Accept
   that, or is a real machine-readable aggregate renderer in scope? The latter is a scope
   increase and no consumer needs it today.
5. **R-6 in or out of phase 1?** Deleting `.counter` makes `O_EXCL` at `:1122` the sole
   allocation mechanism, and its loser currently dies on an uncaught `FileExistsError`
   rather than retrying. Recommend in: it is ~5 lines and the bug becomes load-bearing.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (red) before implementation
- [ ] Tests PASS (green) after implementation
- [ ] `make ze-test` green
- [ ] Aggregate view is NOT tracked in git (R-1)
- [ ] Two-session concurrent record proves no cross-commit (AC-1)
- [ ] All four `learned_numbers.py --check` call sites green with `.counter` gone:
      `make ze-learned-numbers-check`, `ze-doc-test`, `ze-regen-check`,
      `ze-discovery-index-check` (R-7, AC-7)
- [ ] `learned_numbers.py --check` still catches a seeded duplicate number after
      invariant 3 is removed (proves 89aff42e8's purpose survives)
- [ ] No commit in this spec's series stages `plan/deferrals.md`,
      `plan/known-failures.md` or `plan/learned/.counter` alongside unrelated work
      (the spec would otherwise reproduce its own bug while fixing it)
- [ ] `rebase_learned.py` coordination resolved with Thomas before `.counter` deletion
