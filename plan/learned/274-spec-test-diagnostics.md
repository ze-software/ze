# 274 — Test Diagnostics

## Objective

Fix seven concrete test diagnostic issues so every failure gives developers a clear, copy-pasteable path to resolution — full hex in debug commands, structured failure reports, field-level JSON diff, named suite failures, and parse test reproduction commands.

## Decisions

- Fuzz tests for the attribute package (7 fuzz functions) enumerated individually as `-fuzz=FuzzParse*` instead of `-fuzz=.` — Go rejects `-fuzz=.` when it matches more than one fuzz test; other packages (wireu, storage, pool) have 1-2 fuzz tests each and keep `-fuzz=.`.
- `likelyCause()` and `likelyCauseTimeout()` pattern-match on error strings and output presence — actionable hints without false positives, no heuristics beyond string matching.
- Field-level JSON diff uses recursive `jsonFieldDiff()` + `jsonSliceDiff()` — avoids dumping two full JSON blobs that developers must visually scan.
- Parse test reproduction command only added for file-based configs (not inline) — inline configs have no path to show.
- Makefile `failed_names` variable tracks failed suite names alongside the counter — minimal change, no shell refactoring needed.

## Patterns

- `Report` struct writes to configurable `io.Writer` (not hardcoded stdout) — tests can capture output without stdout redirection.
- Test named `TestJSONDiffFieldLevel` in plan renamed to `TestComparePluginJSON_MismatchFieldLevel` to match the existing `TestComparePluginJSON_*` naming convention in the file.

## Gotchas

- None.

## Files

- `Makefile` — fuzz enumeration (7 individual targets) + `failed_names` suite tracking
- `internal/test/runner/report.go` — full hex in debug commands, structured generic report, `likelyCause()` / `likelyCauseTimeout()`
- `internal/test/runner/json.go` — `jsonFieldDiff()` + `jsonSliceDiff()` field-level diff
- `internal/test/runner/parsing.go` — `ze validate <config-path>` reproduction command on failure
- `internal/test/runner/report_test.go` — 4 unit tests (created)
- `internal/test/runner/json_test.go` — 2 tests added for field-level diff
