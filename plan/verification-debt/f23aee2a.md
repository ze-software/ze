# Verification debt -- commit session f23aee2a

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-15 | f23aee2a | fix(arch): allowlist the BFD link-local fixture's own veth lookups | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-15T04:35:33Z) | open |
| 2026-10-15 | f23aee2a | fix(arch): allowlist the BFD link-local fixture's own veth lookups | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
