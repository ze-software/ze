# Verification debt -- commit session db0c8bb9

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-18 | db0c8bb9 | plan: design the DHCPv4 client lease maintenance spec | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-18T17:16:37Z) | open |
| 2026-10-18 | db0c8bb9 | plan: design the DHCPv4 client lease maintenance spec | native structural checks (red) | stale: ./le arch tier check and ./le arch iface-resolution both exit 0 on the current tree; this commit carries markdown only | open |
