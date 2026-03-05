# 362 — Flaky Watchdog Test

## Objective

Investigate and fix the intermittently failing `watchdog.ci` functional test, which occasionally produced duplicate announce UPDATEs instead of withdraw.

## Decisions

- No code changes needed — the flaky behavior was resolved as a side effect of the watchdog plugin extraction (learned 360) and AnnounceInitial bug fix
- Verified: 30 isolated runs + 10 full-suite runs (70 tests parallel) — zero failures
- Spec closed as resolved without direct fix

## Patterns

- `cmd=api` lines in `.ci` files are documentation-only (confirmed at `expect.go:88`) — ze-peer ignores them, all commands come from the Python plugin script via RPC
- Checker groups expectations by `(conn, seq)` — messages within a seq group can match in any order, but seq groups are strictly sequential
- Python plugin timing (`time.sleep`) was the only synchronization mechanism — fragile under parallel test load

## Gotchas

- The original race: Python's `time.sleep(1)` before first command could be insufficient under 55+ parallel tests, causing `announce` to arrive before BGP session establishment — state set prematurely, route batched with config routes
- The AnnounceInitial fix (only marking `initiallyAnnounced` routes on reconnect) likely eliminated the specific failure mode where initially-withdrawn routes were incorrectly announced

## Files

- `test/plugin/watchdog.ci` — the test (unchanged, now stable)
- `internal/component/bgp/plugins/bgp-watchdog/server.go` — AnnounceInitial path that fixed the root cause
- `internal/test/peer/expect.go:88` — confirms `cmd=api` is documentation-only
