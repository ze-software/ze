# Verification debt -- commit session 0bad9eb8

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-22 | 0bad9eb8 | meta: M14, interop peers run latest stable upstream (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-22 | 0bad9eb8 | meta: M14, interop peers run latest stable upstream (+1 more) | native structural checks (red) | commit is one markdown row in ai/META-DECISIONS.md; arch tier/check and arch iface-resolution are red from other sessions' Go work and cannot be affected by it | open |
