# Verification debt -- commit session fddec904

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-21 | fddec904 | bgp rr: withdraw a departed source under ze's Path Identifier | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-21T12:27:12Z) | open |
| 2026-10-21 | fddec904 | bgp rr: withdraw a departed source under ze's Path Identifier | native structural checks (red) | staticcheck parts 2 and 3 of 6 are red from an environment cache fault in the latest verify run, not attributed to these files; the parent session reruns the gates | open |
| 2026-10-21 | fddec904 | bgp rr: withdraw a departed source under ze's Path Identifier (+2 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-21 | fddec904 | bgp rr: withdraw a departed source under ze's Path Identifier (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=3, at 2026-10-21T17:22:47Z) | open |
| 2026-10-21 | fddec904 | bgp rr: withdraw a departed source under ze's Path Identifier | native structural checks (red) | staticcheck parts 2 and 3 of 6 red from the environment cache fault; scoped tests of rr, gr, fixture and interoplab/bgp green | open |
| 2026-10-21 | fddec904 | bgp rr: withdraw a departed source under ze's Path Identifier | native structural checks (red) | staticcheck parts 2 and 3 are an environment cache fault (stale patched GOROOT after a Homebrew Go upgrade, go-cache entries removed mid-run, disk filled twice); scoped lint over this change's packages reports nothing in this change's files (4 findings sit in HEAD fixture files it does not touch) | open |
| 2026-10-21 | fddec904 | bgp rr: withdraw a departed source under ze's Path Identifier | owner approval for an RFC-tagged test change | type-only follow-through: withdrawalInfo.Family became family.Family, so the tagged rr tests compare against a family.Family instead of its String(); no assertion, input or claim changed | open |
