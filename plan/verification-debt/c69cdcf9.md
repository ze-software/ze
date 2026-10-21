# Verification debt -- commit session c69cdcf9

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-21 | c69cdcf9 | handover: ike-eap closure progress, runs waiting on the slot | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-21T12:27:12Z) | open |
| 2026-10-21 | c69cdcf9 | handover: ike-eap closure progress, runs waiting on the slot | native structural checks (red) | commit carries one markdown handover file and no Go; the staticcheck red belongs to the latest verify run over other work | open |
| 2026-10-21 | c69cdcf9 | plan: ike-eap closure stopped on a full disk; checklist added (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=3, at 2026-10-21T17:22:47Z) | open |
| 2026-10-21 | c69cdcf9 | plan: ike-eap closure stopped on a full disk; checklist added | native structural checks (red) | markdown only | open |
| 2026-10-21 | c69cdcf9 | plan: ike-eap closure stopped on a full disk; checklist added | native structural checks (red) | commit carries three markdown files and no Go; the staticcheck red belongs to the latest verify run over other work | open |
