# Verification debt -- commit session 6d6a3fdc

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-21 | 6d6a3fdc | nlrisplit: one labelled withdrawal framing for every writer (AC-27) (+4 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=3, at 2026-10-21T17:22:47Z) | open |
| 2026-10-21 | 6d6a3fdc | nlrisplit: one labelled withdrawal framing for every writer (AC-27) | native structural checks (red) | arch tier check is red on internal/le/setup/dockerhost/static.go (another session's lab-docker-cleanup work, run here 2026-10-21); staticcheck parts 2 and 3 were red at ee108f4731 before this change; scoped golangci-lint over nlrisplit, message, reactor and rib reports nothing in the files this commit carries | open |
| 2026-10-21 | 6d6a3fdc | nlrisplit: one labelled withdrawal framing for every writer (AC-27) (+3 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-21 | 6d6a3fdc | nlrisplit: one labelled withdrawal framing for every writer (AC-27) | owner approval for an RFC-tagged test change | TestAIGPForwardedMetricsReachFinalWire and TestRFC7311DirectLinkCostNeverWrapsOrDisappears: signature only, a nil source context added to their buildWithdrawalPayload call, claim unchanged (spec RFC-tagged tests table, round 14 I-1) | open |
| 2026-10-21 | 6d6a3fdc | ribevents: delete RecoveryRequest.PrefixOnly, which nothing produced (+1 more) | native structural checks (red) | arch tier check is red on internal/le/setup/dockerhost/static.go (another session's lab-docker-cleanup work); staticcheck parts 2 and 3 were red at ee108f4731 before this change | open |
| 2026-10-21 | 6d6a3fdc | handover: rs replacement Phase 3b complete, round 9 owed | native structural checks (red) | arch tier check is red on internal/le/setup/dockerhost/static.go (another session's work); staticcheck parts 2 and 3 were red at ee108f4731; this commit is prose | open |
| 2026-10-21 | 6d6a3fdc | bgp: round 9 fixes, labelled withdrawal interop legs and refusal tests | native structural checks (red) | arch tier check is red on internal/le/setup/dockerhost/static.go (another session's work); staticcheck parts 2 and 3 were red at ee108f4731 before this change | open |
