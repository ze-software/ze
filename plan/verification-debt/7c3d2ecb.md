# Verification debt -- commit session 7c3d2ecb

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-21 | 7c3d2ecb | mup: T1ST carries its Source Address Length octet, 0 with no source | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-10-18T10:55:37Z) | open |
| 2026-10-21 | 7c3d2ecb | mup: T1ST carries its Source Address Length octet, 0 with no source | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-21 | 7c3d2ecb | mup: T1ST carries its Source Address Length octet, 0 with no source | owner approval for an RFC-tagged test change | fixtures of tagged MUP tests corrected to the draft-01 T1ST layout (Source Address Length octet); assertions unchanged | open |
