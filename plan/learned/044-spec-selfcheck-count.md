# 044 — Selfcheck --count (Stress Mode)

## Objective

Add `--count N` flag to the functional test runner to run each selected test N times and report per-test pass/fail/timeout statistics for detecting flaky tests.

## Decisions

Mechanical implementation following the existing runner pattern. No significant design decisions beyond what was specified in spec 043 Phase 2.

## Patterns

- `IterationStats` type tracks `Passed`, `Failed`, `TimedOut` counts plus `[]time.Duration` for timing statistics (min/avg/max).
- State reset between iterations: loop resets `Record.State` to `StateNone` before each run.
- Per-iteration failure reports are suppressed in count mode — only the final statistics summary is shown.

## Gotchas

None.

## Files

- `internal/test/runner/stress.go` — `IterationStats`, `RunWithCount()`
- `internal/test/runner/stress_test.go` — 9 unit tests for stats logic
- `test/cmd/functional/main.go` — `--count` flag wired in
