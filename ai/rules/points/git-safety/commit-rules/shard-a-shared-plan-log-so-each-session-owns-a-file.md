---
kind: directive
level: MUST
stage:
---
**A shared single-file plan log cross-commits even with a correct, explicit
`--file` list.** The ban on the bare staging verbs fixes staging *timing*; it
cannot fix staging *granularity*. `git add <file>` stages the WHOLE file, including hunks
another session left uncommitted in it. You MUST SHARD the log so each
session writes only files it owns and git merges disjoint creations without
conflict. **Both cross-spec logs are now sharded.** Deferrals live one file
per source under `plan/deferrals/` (`ai/rules/planning.md`), so `git
add plan/deferrals/<source>.md` stages only your row. Known failures live one
file per failure under `plan/known-failures/` (a `<make-target>-<test-name>.md`
shard, with `RESOLVED.md` archiving the history and `README.md` holding the
logging instructions), so `git add plan/known-failures/<make-target>-<test-name>.md`
stages only your entry. A shared unsharded log lets concurrent sessions stage
each other's entries.
