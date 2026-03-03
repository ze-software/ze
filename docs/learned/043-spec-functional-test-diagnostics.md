# 043 — Functional Test Diagnostics

## Objective

Replace the old `test/cmd/functional` runner with a new `test/cmd/selfcheck` (later renamed back to `functional`) that shows AI-friendly failure output: expected vs received messages decoded side-by-side with diffs.

## Decisions

- Chose to copy `internal/test/peer/decode.go` into `test/selfcheck/decode.go` rather than importing it — `internal/test/peer` was in active use, and a copy allows the new package to evolve freely without coupling.
- Dynamic port range allocation: instead of warning about port conflicts, automatically scan for a free range starting from `--port` (default 1790). Reports which range is in use.
- ulimit check uses `parallel × 20 FDs` (not `testCount × 10`) — based on actual FDs per test, not total tests.
- Stress mode (`--count N`) collects `IterationStats` per test; per-iteration failure reports are suppressed in count mode to reduce noise.
- `--save DIR` saves peer/client stdout/stderr + expected/received to named subdirectories for post-mortem analysis.

## Patterns

- Three-form failure report: `cmd` (human-readable API command from `.ci` file) + `raw` (wire hex) + `decoded` (structured attribute output). AI can act on any of the three.
- TTY detection via `golang.org/x/term`: colors on terminal, plain text when piped (CI-safe).
- ExaBGP-style progress line: `timeout [N/M] running N passed N failed N [IDs]`.

## Gotchas

- The old `test/internal/*` package had a reporter bug: all messages with `1:` index merged into one in diagnostic output — only display was wrong, actual comparison was correct.
- Phase 6 (migration + cleanup) removed 14 files from `test/pkg/`, renamed `test/selfcheck` → `internal/test/runner`, renamed `selfcheck` package to `functional`.
- `golang.org/x/term` dependency was added for TTY detection.

## Files

- `test/cmd/functional/main.go` — new runner entry point (renamed from selfcheck)
- `internal/test/runner/` — test infrastructure (decode, report, runner, ports, limits, stress)
