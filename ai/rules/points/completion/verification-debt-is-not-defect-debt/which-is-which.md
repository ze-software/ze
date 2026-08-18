---
kind: table
level:
stage:
---
| What you are holding | Which debt | What to do |
|---|---|---|
| A gate you have not run yet over code you believe correct | verification | Record the row, commit, clear it later |
| A gate that ran and went red on YOUR code | defect | Fix it. The row is not available to you |
| A gate red on another session's uncommitted work | verification | Record the row naming whose work, commit |
| A test that fails deterministically anywhere | defect | Fix the root cause (see "Recording is not fixing" above) |
| A review not yet performed on a spec closure | verification | Record the row; the review is still owed before any push |
| Behavior an acceptance criterion requires and nothing implements | defect | Implement it. Nothing here makes an unfinished AC recordable |
