# Deferrals: test-sync-quiesce

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-13 | spec-test-sync-quiesce (AC-6) | Migrate `wait_for_ack` off its `time.sleep`. RESOLVED by `spec-quiesce-peer-drain`: the real race is ze's per-peer initial-sync opQueue (`peer.go`), which the forward-pool quiescer misses, drained by a new `DrainPeerSync` reactor barrier + `bgp-peer-sync` quiescer. The load-bearing fix was in the INVOCATION: `wait_for_ack`/`quiesce()` had called `_call_engine("ze-bgp:peer-flush"/"ze-system:quiesce")`, which is `unknown method` (those api-yang RPCs dispatch only via command path), silently swallowed by `except RuntimeError` so the sleep did all the work. Now both invoke `api.dispatch("request quiesce")` (the reachable path) with no sleep. Validated: nexthop.ci sleepless 25/25 + full plugin suite 472/474 green | Fixed at the source (a ze-side quiescer, invoked via dispatch-command) | `plan/learned/1118-quiesce-peer-drain.md` | done |

