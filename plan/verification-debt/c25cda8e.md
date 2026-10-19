# Verification debt -- commit session c25cda8e

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-19 | c25cda8e | test integration: live-rpki runs under ZE_REQUIRE_DOCKER=1 (+2 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T00:22:49Z) | open |
| 2026-10-19 | c25cda8e | test integration: live-rpki runs under ZE_REQUIRE_DOCKER=1 (+2 more) | native structural checks (red) | arch tier/check and arch iface-resolution charged for want of path attribution; both run directly over the tree exit 0 | open |
| 2026-10-19 | c25cda8e | test integration: live-rpki runs under ZE_REQUIRE_DOCKER=1 (+2 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
