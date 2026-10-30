# A review pin is checked against content the commit does not land

The closure review artifact pins each file by hash, and the check hashes a
version of the file the commits do not carry, so the gate refuses correct
reviews or passes ones that never saw the landed content.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | config-reference-completion (found, not caused) | `judgeReviewCoverage` hash callback in `internal/le/commit/review.go` | For a spec-work file the current commit does not carry by `hunks`, the pin is hashed from the working tree; a file an earlier commit landed by `hunks` beside another session's hunks never matches, so closure needed two scripts and a re-pin over the foreign hunk | open: hash the committed blob for files the spec's earlier commits landed, with a test over a hunks-landed file |
