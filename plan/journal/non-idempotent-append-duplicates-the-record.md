# Non-Idempotent Append Duplicates The Record

A writer appends a row without testing whether the file already holds it, so a
second run of the same step records the same fact twice.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-19 | - | commit_helper.py, verification-debt shard | `record_debt` (`scripts/dev/commit_helper.py`) appends one row per owed gate and never tests whether the shard already carries that row, so a second `create` for the same session, subject and gate writes it again. Measured on commit `d0ddef174`: two `create` runs left four rows in `plan/verification-debt/aabf4645.md` where two were owed. The first run was killed at a 2-minute timeout after it had appended, and `--replace` on the same commit would do the same | not fixed. The two duplicate rows were deleted at owner request; the append is still non-idempotent |
