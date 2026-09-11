# Verification debt -- commit session 6e4caf8d

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-09 | 6e4caf8d | pppoe: every PADO and PADS carries the mandatory Service-Name tag (+13 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-09-05T00:11:54Z) | open |
| 2026-09-09 | 6e4caf8d | pppoe: every PADO and PADS carries the mandatory Service-Name tag (+7 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-09 | 6e4caf8d | a failing subscriber socket is paced, logged and counted (+2 more) | discovery-index freshness | ai/PACKAGE-MAP.md already carries internal/core/pacer at HEAD; ./le discovery-index update produces no diff | open |
| 2026-09-11 | 6e4caf8d | a failing socket is held by seccomp, and the scenario is gated | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-09-11T04:49:35Z) | open |
| 2026-09-11 | 6e4caf8d | a failing socket is held by seccomp, and the scenario is gated | native structural checks (red) | both reds are already in HEAD and neither touches this commit's files: tier/check fails on internal/le/interoplab/pppoe/check_ipv6cp.go, check_padr_replay.go and check_service_name.go importing internal/component/l2tp/pppoe without a ze_l2tp build tag, and iface-resolution fails on an allowlist entry naming internal/le/interoplab/bgp/isis_inject_linux.go that suppresses nothing. All four files are tracked and unmodified in this working tree, so the reds predate every uncommitted change here | open |
