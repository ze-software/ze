# Verification debt -- commit session c00ebbb3

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-02 | c00ebbb3 | rfc: renew LDP and OSPF discrimination records (+14 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-10-03 | c00ebbb3 | arch: enforce plugin import ownership in tier checks (+9 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
