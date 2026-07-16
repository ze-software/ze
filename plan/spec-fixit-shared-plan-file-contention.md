# spec-fixit-shared-plan-file-contention

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-16 |

> **SKELETON.** Captured intent, not designed work. Every file, step and test named
> below is a CANDIDATE. Research via `/ze-spec` before implementing.

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
- [ ] `scripts/dev/commit_helper.py` - machine-reads `plan/known-failures.md` for the
      `--unverified` gate and `plan/learned/.counter` for `learned-next`. Both move
      if the files move.
- [ ] `ai/rules/git-safety.md` "Shared plan files cross-commit" - the documented
      hazard and the four correct reactions.
- [ ] `ai/rules/deferral-tracking.md` - owns the deferrals table format and the
      resolution rules. Any reshaping must keep "no open row without a Destination".

### Architecture Docs
- [ ] `ai/skills/ze-status.md` - already the documented live backlog view; likely the
      natural home for an on-demand aggregate.

## Current Behavior (MANDATORY)

**Source files (cite file:line):**
- [ ] `scripts/dev/spec-session.sh` lines 4-5 - the in-repo precedent and its stated
      rationale: "Each Claude session records the spec it is working on in its OWN
      marker file (tmp/session/.session-<SID>), never a shared file."
- [ ] `scripts/dev/commit_helper.py` lines 1097-1124 - `learned_next()`. Allocation is
      `max(highest + 1, counter)` where `highest` comes from globbing existing
      `[0-9]*-*.md` files (lines 1112-1115), and the file is created with `os.O_EXCL`
      (line 1122), so the file creation IS the atomic lock. The docstring records that
      concurrent sessions already collided here: "both write the same number (13
      duplicate prefixes exist)".
- [ ] `scripts/dev/commit_helper.py` lines 1116-1120 - the `.counter` read, whose
      `except` branch already treats a missing or unreadable counter as `0`. So
      deleting `.counter` leaves `number = highest + 1`, which is the same answer.
- [ ] `scripts/dev/commit_helper.py` lines 375-376 - the gate that FORCES the
      contention: "new learned summaries must add plan/learned/.counter". This is the
      only reason a session must stage the shared counter at all.
- [ ] `scripts/dev/commit_helper.py` lines 997-1017 - the `--unverified` path that
      machine-reads `plan/known-failures.md`; it moves if that file is sharded.
- [ ] `plan/deferrals.md` - single tracked table; every session appends rows. Its own
      header already says to run `/ze-status` for the live backlog, so the committed
      table is arguably already redundant with a command.
- [ ] `plan/known-failures.md` - single tracked file; entries are appended by one
      session and later CORRECTED by another (this session corrected a 2026-07-08
      entry on 2026-07-16).

**Behavior to preserve:**
- Every open deferral still names a Destination (`ai/rules/deferral-tracking.md`).
- `commit_helper.py --unverified` still refuses unless the red is logged as a known
  failure.
- Learned numbers stay unique across concurrent sessions.
- Existing greppability: a human or agent must still be able to find "all open
  deferrals" and "is this test a known red" in one step.

**Behavior to change:**
- A session must be able to stage its own deferral/known-failure record without
  staging any other session's pending edits.

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
| `commit_helper.py` -> plan records | Machine-reads known-failures for `--unverified`; requires staging `.counter` | A sharded layout breaks the gate unless migrated with it |
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
| A session records a deferral, then runs its commit script | -> | the per-owner shard writer (CANDIDATE) | `test_two_sessions_no_cross_commit` |
| `/ze-status` | -> | the on-demand aggregate renderer (CANDIDATE) | `test_aggregate_renders_all_shards` |
| `commit_helper.py --unverified` | -> | known-failures lookup, `commit_helper.py:997-1017` | `test_unverified_finds_sharded_known_failure` |
| `commit_helper.py learned-next <slug>` | -> | `learned_next()`, `commit_helper.py:1097-1124` | `test_learned_next_unique_without_counter` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `test_unverified_finds_sharded_known_failure` (CANDIDATE) | `scripts/dev/commit_helper_test.py` | AC-5: the `--unverified` gate still resolves a logged red, and still refuses an unlogged one |
| `test_learned_next_unique_without_counter` (CANDIDATE) | `scripts/dev/learned_index_test.py` | AC-4: concurrent allocation stays unique once `.counter` is deleted, relying on the `O_EXCL` create |
| `test_aggregate_renders_all_shards` (CANDIDATE) | new, alongside the renderer | AC-2: N shard files render one ordered view with no row dropped |
| `test_two_sessions_no_cross_commit` (CANDIDATE) | new, alongside the renderer | AC-1: each session stages only its own record |

### Functional Tests
N/A. This is agent tooling under `scripts/dev/` and `plan/`; it has no `.ci` surface
(the `.ci` suites cover the ze product). The wiring is proven by the unit tests above
plus the two-session sequence in the Wiring Test table.

## Files to Modify

- [ ] `scripts/dev/commit_helper.py` - known-failures lookup; `learned-next`
      allocation if `.counter` is dropped (CANDIDATE)
- [ ] `ai/rules/deferral-tracking.md` - record location and format (CANDIDATE)
- [ ] `ai/rules/git-safety.md` - fold the hazard note into the new mechanism once it
      exists (CANDIDATE)
- [ ] `ai/skills/ze-status.md` - render the aggregate from shards (CANDIDATE)
- [ ] `plan/deferrals.md`, `plan/known-failures.md`, `plan/learned/.counter` -
      migrate or retire (CANDIDATE)

## Implementation Steps

1. CANDIDATE: pick the shard key per file (see the access-pattern table); this is the
   research output, not a given.
2. CANDIDATE: build the on-demand aggregate FIRST, so readers keep working during
   migration.
3. CANDIDATE: migrate `commit_helper.py`'s known-failures gate and `learned-next`.
4. CANDIDATE: migrate existing rows; update the 8+/6+ referencing rules and skills.
5. CANDIDATE: retire the shared files.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two sessions concurrently record a deferral, then each commits | Each commit contains only its own record; neither carries the other's |
| AC-2 | An agent asks "what is open" | One step still answers it, across all shards |
| AC-3 | A session corrects a known failure first logged by another session | The correction lands on the failure's record, not filed under the corrector |
| AC-4 | Two sessions allocate a learned number concurrently | Numbers are unique with no shared counter file |
| AC-5 | `commit_helper.py --unverified` with a logged known red | Still passes; still refuses when the red is not logged |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Per-session sharding is the right pattern | `scripts/dev/spec-session.sh` already does exactly this for spec claims and states the same rationale | The fix shape changes | Read spec-session.sh and its rationale | unvalidated |
| A-2 | The deferrals `Source` column is a usable shard key | Every row already names a source spec or "ad-hoc" | Ad-hoc rows need a different key (session id or date) | Count the ad-hoc rows in the current table | unvalidated |
| A-3 | `/ze-status` can render an aggregate from a directory | It is already the documented live-backlog view | An aggregate generator is needed elsewhere | Read `ai/skills/ze-status.md` | unvalidated |
| A-4 | `.counter` can be dropped safely | CONFIRMED by reading `learned_next()` (`commit_helper.py:1097-1124`): `highest` is globbed from the existing `NNN-*.md` filenames (:1112-1115), the create uses `os.O_EXCL` (:1122) so the file IS the lock, and the `.counter` read's `except` branch already treats it as absent = 0 (:1118-1119). Deleting it leaves `number = highest + 1`, the same answer | Nothing; the counter only raises the floor | Already validated by reading the producer | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A generated aggregate gets COMMITTED, reintroducing the contention unchanged | The aggregate appears in a commit script's `--file` list | Keep the aggregate on demand only; never track it. This is the single most important constraint in this spec |
| R-2 | Migration breaks the 8+ deferrals and 6+ known-failures readers | Rules or skills reference a path that no longer exists | Build the aggregate first, migrate readers, retire the file last |
| R-3 | Sharding trades cross-commits for lost visibility (nobody reads N files) | Agents stop recording deferrals | AC-2: one step must still answer "what is open" |
| R-4 | Per-session shards accumulate as garbage once sessions end | The directory grows without bound | Resolution/GC story must be part of the design, not an afterthought |

## Open Questions (research before design)

- Shard key for `plan/deferrals.md`: source spec (stable, meaningful, matches the
  existing `Source` column) or session id (matches the spec-session precedent but is
  meaningless after the session ends)? What about "ad-hoc" rows?
- Is `plan/known-failures.md` better sharded per failure than per session, given
  entries are corrected by other sessions later? A per-failure file also gives a
  natural home for the correction history.
- ~~Can `plan/learned/.counter` simply be deleted?~~ ANSWERED while writing this spec:
  yes. `learned_next()` already derives `highest` from the `NNN-*.md` filenames and
  locks with `os.O_EXCL`, and already tolerates a missing counter (`:1118-1119`). The
  counter prevents no race the filenames do not. The real blocker is
  `commit_helper.py:375-376`, which REQUIRES every learned summary to stage
  `plan/learned/.counter` and so manufactures the contention. Remaining question is
  narrow: does anything else read `.counter` (`scripts/dev/learned_index_test.py`,
  `scripts/dev/commit_helper_test.go`) in a way that breaks? This looks like the
  cheapest of the three fixes and could land first, independently.
- Does anything parse these files positionally rather than by grep?
- Should the aggregate be a command only (`/ze-status`), or also a generated but
  untracked file? Anything tracked reintroduces R-1.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (red) before implementation
- [ ] Tests PASS (green) after implementation
- [ ] `make ze-test` green
- [ ] Aggregate view is NOT tracked in git (R-1)
- [ ] Two-session concurrent record proves no cross-commit (AC-1)
