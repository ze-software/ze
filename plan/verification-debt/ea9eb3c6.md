# Verification debt -- commit session ea9eb3c6

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-22 | ea9eb3c6 | docs: comparison FlowSpec row states what both sides carry (+2 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-22 | ea9eb3c6 | docs: comparison FlowSpec row states what both sides carry | native structural checks (red) | le arch tier check and le arch iface-resolution exit 0 in this run; the stale red is the last verify run's | open |
| 2026-10-22 | ea9eb3c6 | mup: relay a -02 Type 1 ST route with its Source Address Length | native structural checks (red) | le arch tier check and le arch iface-resolution exit 0 in this session's run; the stale red is the last verify run's | open |
| 2026-10-22 | ea9eb3c6 | mup: relay a -02 Type 1 ST route with its Source Address Length | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-22 | ea9eb3c6 | handover 17: FlowSpec row and MUP relay | native structural checks (red) | le arch tier check and le arch iface-resolution exit 0 in this session's run | open |
