# Verification debt -- commit session cf58eafa

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-20 | cf58eafa | bgp: plugin routes carry the RFC 7911 Path Identifier under ADD-PATH (+2 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-10-18T10:55:37Z) | open |
| 2026-10-20 | cf58eafa | bgp: plugin routes carry the RFC 7911 Path Identifier under ADD-PATH (+1 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
