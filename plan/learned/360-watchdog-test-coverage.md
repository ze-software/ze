# 360 — Watchdog Test Coverage

## Objective

Improve bgp-watchdog plugin test coverage for critical untested scenarios: rapid peer flapping, wildcard dispatch with mixed peer states, multi-pool interactions, and reconnect after explicit withdraw.

## Decisions

- Test-only spec — no feature code changes except the AnnounceInitial bug fix (over-announce on reconnect)
- 7 unit tests covering AC-1 through AC-6 plus AnnounceInitialPool
- 1 functional test (watchdog-reconnect.ci) for end-to-end reconnect resend
- Concurrent stress test deferred — low risk given mutex design in PoolSet

## Patterns

- watchdogServer decoupled from SDK via sendRoute callback — fully unit-testable without plugin infrastructure
- handlePoolAction flips pool state even when peer is down — ensures correct state on reconnect
- AnnounceInitial (selective) vs AnnouncePool (bulk) — reconnect must use selective to avoid resending explicitly-withdrawn routes

## Gotchas

- Mixed initial state bug found during spec work: handleStateUp used AnnouncePool (marks ALL routes) instead of AnnounceInitial (marks only initiallyAnnounced). This caused initially-withdrawn routes to be announced on reconnect.
- Wildcard dispatch silently skips peers without the named pool — not an error

## Files

- `internal/component/bgp/plugins/bgp-watchdog/server_test.go` — TestRapidFlap, TestWildcardMixedPeerStates, TestMultiPoolIndependence, TestExplicitWithdrawSurvivesReconnect, TestReconnectResendAfterEstablished, TestWildcardNonexistentPool
- `internal/component/bgp/plugins/bgp-watchdog/pool_test.go` — TestAnnounceInitialPool
- `internal/component/bgp/plugins/bgp-watchdog/pool.go` — AnnounceInitial method (bug fix)
- `test/plugin/watchdog-reconnect.ci` — functional reconnect test
