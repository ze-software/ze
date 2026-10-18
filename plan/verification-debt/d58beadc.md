# Verification debt -- commit session d58beadc

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-18 | d58beadc | interoplab: one-shots run named and removed when canceled (+2 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-18T17:16:37Z) | open |
| 2026-10-18 | d58beadc | interoplab: one-shots run named and removed when canceled (+1 more) | native structural checks (red) | arch iface-resolution red on internal/plugins/iface/dhcp, another session's files; arch tier check exits 0 run alone on this tree | open |
| 2026-10-18 | d58beadc | interoplab: one-shots run named and removed when canceled (+1 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-18 | d58beadc | plan: phase 3 handover for the lab Docker MUST adoption | native structural checks (red) | arch iface-resolution red on internal/plugins/iface/dhcp, another session's files; this commit carries plan prose only | open |
