# Verification debt -- commit session 9ae1d9de

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-20 | 9ae1d9de | fix(ldp): name the next step in the interface read warnings (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-20 | 9ae1d9de | fix(ldp): name the next step in the interface read warnings | native structural checks (red) | ./le arch tier check and ./le arch iface-resolution both run on this tree and exit 0; the gate charged them for want of path attribution only | open |
| 2026-10-20 | 9ae1d9de | fix(ldp): name the next step in the interface read warnings | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-20 | 9ae1d9de | rfc(3031): re-record the four LDP tags on targeted gomu mutants | native structural checks (red) | ./le arch tier check and ./le arch iface-resolution both exit 0 on this tree; the gate charged them for want of path attribution only | open |
