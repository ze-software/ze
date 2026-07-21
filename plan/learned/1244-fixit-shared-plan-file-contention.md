# 1244 -- fixit-shared-plan-file-contention

## Context

Concurrent sessions cross-committed each other's rows in three shared plan files
(`plan/learned/.counter`, `plan/deferrals.md`, `plan/known-failures.md`): `git add <file>`
stages the WHOLE file, so whoever commits first carries everyone else's pending hunks into
their commit (observed twice: this-session deferrals edits landed inside two unrelated VRRP
commits). The commit-script rule fixes staging TIMING; it cannot fix staging GRANULARITY.
The structural fix is to shard each contended file so every session writes only files it
owns, and compute the aggregate ON READ (never tracked). Shipped as three independent
commits, cheapest-and-most-proven first.

## Decisions

- **Shard per RECORD, never per session** (the skeleton's per-session premise, A-1, was
  broken). A deferral, a known failure, and a learned number all OUTLIVE their session, so
  they key on the record (spec stem, `<make-target>-<test-name>`, summary number), not the
  author. `spec-session.sh`'s per-session precedent does not transfer -- a spec CLAIM dies
  with the session; these records do not.
- **The aggregate is folded on read, never stored or committed (R-1, load-bearing).** A
  tracked rollup is just the contended file again with extra steps. `git` already merges
  disjoint file creations without conflict; that IS the mechanism. Readers glob + fold on
  demand (`/ze-status`, the health collector, the commit gates).
- **Phase 1 -- `.counter` retired.** The `NNNN-slug.md` filenames ARE the record;
  `learned_next` allocates `max(glob)+1` with a bounded `O_EXCL` retry (a losing racer
  re-globs and lands on the next free number instead of an uncaught `FileExistsError`, R-6).
- **Phase 2 -- `plan/deferrals.md` sharded per source spec** (`<spec-stem>.md`;
  `ad-hoc-<date>-<sha1[:6]>.md` for ad-hoc rows). Per-source-spec over per-row: per-row gives
  maximal isolation but unbounded file growth; per-source-spec bounds the directory by open
  specs (already GC'd -- spec closure now also removes the spec's deferral shard, R-4).
- **Phase 3 -- `plan/known-failures.md` sharded per failure** (one live shard each;
  `RESOLVED.md` archives history verbatim; `README.md` keeps the logging checklist). A
  cleared red deletes its shard and appends to `RESOLVED.md`.

## Gotchas

- **Reading only the PRODUCER of the answer, not the other CONSUMERS of the file, missed two
  hidden machine readers.** Both A-4 (.counter) and A-5 (known-failures) claimed "nothing
  else reads it" from a single-function read, and both were wrong: (1) `learned_numbers.py`
  invariant 3 asserted `.counter >= highest+1` (added AFTER the skeleton), so deleting the
  file reddened four gates -- phase 1 was a MIGRATION, not a `git rm`. (2)
  `testing_health.py` `collect_known_failures` PARSES `known-failures.md` for a health metric
  (live vs resolved `###` counts), so phase 3 had to rewrite the collector to fold the
  directory while holding the live count invariant (AC-5': 6 live before == 6 after). Grep
  every reader of a file repo-wide before deleting it.
- **The commit GATES read the files they gate.** `commit_helper.py` `deferral_unassigned`
  (folds the shard dir) and `deferral_in_diff` (cleared by any staged `plan/deferrals/`
  shard, via `startswith` -- lookalikes like `plan/deferrals-notes.md` correctly rejected)
  both had to migrate, plus `hook-fixture-check.py` fixtures and `session-end-deferrals.sh`.
  Modifying the tool you commit WITH means verifying it still works (dry-run) before relying
  on it: closing THIS spec used the counter-free `learned-next` to reserve 1244.
- **A gate that skips-and-logs on a pass is fail-open, and a lookalike-path clear is a hole.**
  Independent review caught: a vacuous `deferral_in_diff` test (asserted a shard path never
  written to disk, so `git add` aborted and it passed under old AND new code -- fixed to write
  the shard); a latent glob/startswith mismatch (`deferral_unassigned` globbed top-level while
  the clear accepted any depth -- aligned to `rglob`); and the phase-1 BLOCKER (agent skills +
  METHODOLOGY still told agents to stage the deleted `.counter`).
- **Deleting a shared file dangles its inbound references.** Retiring `known-failures.md`
  left a deferral shard citing it as a Destination (advisory WARN) and stale doc/comment
  refs; repoint them in the same work.
- **Sharding is a same-tree mechanism only.** Two branches are two trees; a cross-branch
  learned-number collision is unreachable by any shard layout -- `learned_numbers.py --check`
  stays as the backstop.

## Files

- Phase 1: `scripts/dev/commit_helper.py` (counter-free `learned_next` + retry, dropped gate), `learned_numbers.py` (dropped invariant 3), `rebase_learned.py`, `mk/inventory.mk`/`Makefile` (dropped `ze-learned-counter`), 4 skills + `METHODOLOGY.md`, deleted `plan/learned/.counter`.
- Phase 2: `plan/deferrals.md` -> `plan/deferrals/` (68 shards); `commit_helper.py` (both deferral gates), `hook-fixture-check.py`, `session-end-deferrals.sh`, `posttool-writeedit.py`; deferral-tracking/planning/git-safety prose.
- Phase 3: `plan/known-failures.md` -> `plan/known-failures/` (6 live shards + RESOLVED.md + README.md); `testing_health.py` `collect_known_failures` + test; `commit_helper.py` text; git-safety + rule prose.
