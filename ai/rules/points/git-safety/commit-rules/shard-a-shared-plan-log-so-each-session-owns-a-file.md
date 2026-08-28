---
kind: directive
level: MUST
stage:
---
**A shared single-file plan log cross-commits even with a correct, explicit
`file <path>` list.** The ban on bare staging verbs fixes staging *timing*; it
cannot fix staging *granularity*: the generated script stages the whole file,
including hunks another session left uncommitted in it. You MUST SHARD the log so each
session writes only files it owns and git merges disjoint creations without
conflict. **Both cross-spec logs are now sharded.** Deferrals live one file
per source under `plan/deferrals/` (`ai/rules/planning.md`), so one
`file plan/deferrals/<source>.md` pair stages only your row. Known failures live
one file per failure under `plan/known-failures/` (a
`<native-action>-<test-name>.md` shard, with `RESOLVED.md` archiving history), so
one `file plan/known-failures/<native-action>-<test-name>.md` pair stages only your
entry. A shared unsharded log lets concurrent sessions stage each other's entries.
