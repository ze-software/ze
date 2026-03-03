# 051 — Debug Teardown Timing

## Objective

Fix flaky teardown test that sometimes completed in ~6s and sometimes took ~17s, occasionally exceeding the 15s timeout.

## Decisions

- Shutdown signal to plugins written synchronously (bypassing async write queue) because the queue might not drain before context cancellation — a race condition would silence the shutdown message
- Test runner changed from SIGKILL to SIGTERM + 2s grace period so ZeBGP can run cleanup before process death

## Patterns

- None.

## Gotchas

- Root cause was ZeBGP never sending a shutdown signal to API processes at all — Python scripts always waited the full 5s shutdown timeout
- Timing variance came from whether shutdown arrived during `wait_for_ack` (~6–8s fast) or after `wait_for_shutdown` timed out (~13–15s slow)
- SIGKILL bypasses all cleanup; always use SIGTERM in test runners when the process needs to do shutdown work

## Files

- `internal/plugin/process.go` — `SendShutdown()` (synchronous write, bypasses async queue)
- `internal/test/runner/runner.go` — SIGTERM + 2s grace period
- `test/data/api/teardown.ci` — Adjusted timeout to 18s
