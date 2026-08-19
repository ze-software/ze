# Record written before the operation succeeds

A command persists a record of what it is about to do. It then fails, and the
work never happens. The record outlives the failure. The next reader cannot
tell it apart from a record of real work, because both are just rows.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-19 | - | commit_helper verification-debt shard | `create` calls `record_debt` (`scripts/dev/commit_helper.py`) before it validates the subject. A create refused for a 73-character subject had already appended its two rows. The retry carried a shortened subject, which renders a different row, so the shard held two rows naming a commit that does not exist. `--push` refuses over them. `debt-clear` re-runs a gate for them. Met when I committed the test-coverage gate | not fixed. The two orphan rows were deleted by hand. The record belongs after the last check that can refuse a create, and `create` has several |
