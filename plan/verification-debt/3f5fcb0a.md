# Verification debt -- commit session 3f5fcb0a

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-18 | 3f5fcb0a | rfc(2545): re-record RFC2545-3-1 and 3-2 after buildRelayUpdate moved (+6 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-17T10:46:25Z) | open |
| 2026-10-18 | 3f5fcb0a | rfc(2545): re-record RFC2545-3-1 and 3-2 after buildRelayUpdate moved (+2 more) | native structural checks (red) | recorded reds come from a verify that predates the 2026-10-18 rebase; ./le arch tier check and ./le arch iface-resolution both pass over this tree now | open |
| 2026-10-18 | 3f5fcb0a | test(reactor): drop the RFC naming marker from decoders_test.go (+10 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-18 | 3f5fcb0a | test(bgp): RFC 9494 4.3 units pin the advertised trigger and cache rail (+3 more) | native structural checks (red) | recorded reds come from a verify that predates the 2026-10-18 rebase; ./le arch tier check and ./le arch iface-resolution both pass over this tree; a full verify is in progress | open |
| 2026-10-18 | 3f5fcb0a | journal: cache disk full during the 2026-10-18 full verify (+13 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-10-18T13:57:23Z) | open |
| 2026-10-18 | 3f5fcb0a | journal: cache disk full during the 2026-10-18 full verify | native structural checks (red) | recorded reds come from a verify that predates the 2026-10-18 rebase; a full verify is in progress | open |
| 2026-10-18 | 3f5fcb0a | fix(exabgp): bridge writes the selector into clear adj-rib (+12 more) | native structural checks (red) | arch tier/check and iface-resolution reds are stale; both gates pass when run now | open |
