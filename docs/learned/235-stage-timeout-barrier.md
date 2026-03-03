# 235 — Stage Timeout Barrier

## Objective

Fix flaky plugin stage tests where the barrier timeout fired before all plugins arrived, caused by measuring timeout from each plugin's arrival rather than from stage start.

## Decisions

- Timeout measured from `stageStart` time (absolute deadline shared by all plugins), not from when each plugin reaches the barrier.
- `context.WithDeadline(stageStart + timeout)` replaces `context.WithTimeout(timeout)`.
- Added `ze.plugin.stage.timeout` env var (dot/underscore fallback) to override the 5s default.
- Test runner sets `ze_plugin_stage_timeout=10s`; production keeps 5s default.

## Patterns

- Stage barrier: all plugins share one absolute deadline, not per-arrival relative timeouts.
- Env var override pattern with dot/underscore fallback for test configuration.

## Gotchas

- `context.WithTimeout` is relative to the caller's moment — different goroutines calling it at different times get skewed deadlines.
- Use `context.WithDeadline` with a pre-computed absolute time when a shared cutoff is needed.

## Files

- `internal/plugin/` — stage timeout barrier logic
