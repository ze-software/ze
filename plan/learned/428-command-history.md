# 428 -- Command History Persistence

## Context

Both `ze config edit` and `ze cli` maintained per-mode command history in memory only. On restart, all history was lost. The goal was to persist history to the zefs blob store so commands survive application restarts, with per-mode storage (edit vs command), per-user scoping, consecutive dedup, and a configurable rolling window.

## Decisions

- Chose a single `History` type owning browsing state + persistence over separate HistoryStore interface + browsing logic, because the two concerns are tightly coupled (append triggers save, mode switch snapshots both).
- Chose `historyRW` interface (ReadFile/WriteFile) over depending on `storage.Storage` or `*zefs.BlobStore` directly, because both satisfy it without an adapter.
- Chose newline-delimited text over JSON for storage, because history entries are single-line strings and the format is trivially human-readable via `ze data cat`.
- Chose `.et` editor tests over `.ci` functional tests for persistence verification, because the `.ci` framework handles BGP protocol testing (wire bytes, exit codes) while `.et` supports headless TUI keystroke simulation. Extended the `.et` framework with `option=history:store`, `option=mode:value=command`, and `restart=` to support persistence testing across simulated restarts.
- Per-user key scoping (`meta/history/<user>/<mode>`) over flat keys, because SSH sessions carry username and multiple operators share one config store.

## Consequences

- The `.et` framework now supports command-only mode (`NewHeadlessCommandModel`) and blob-backed history, enabling future TUI persistence tests without framework changes.
- `restart=` action in `.et` tests provides a general-purpose "simulate app restart" capability, useful beyond history testing.
- History max is clamped to [1, 10000] on load; values outside this range are silently corrected. No runtime reconfiguration -- max is read once at `NewHistory` time.
- AC-9 (concurrent sessions) uses last-write-wins by design. This is acceptable for v1 but means rapid concurrent edits from different SSH sessions may lose history entries from the slower writer.

## Gotchas

- The `.ci` test framework cannot test interactive TUI features -- it has no keystroke simulation. All TUI functional tests must use `.et` format. Previous specs listed `.ci` paths for TUI tests that could never have worked.
- `ze data import` writes to `file/active/<base>` only -- it cannot set arbitrary meta keys. Testing `meta/history/max` configuration requires either Go integration tests or extending `ze data` with a `set` subcommand.
- `NewCommandModel()` creates history with `NewHistory(nil, "")` (in-memory). The caller must call `SetHistory()` after construction to enable persistence. This is easy to forget when adding new entry points.

## Files

- `internal/component/cli/history.go` -- History type (browsing + persistence)
- `internal/component/cli/history_test.go` -- 18 unit tests
- `internal/component/cli/model.go` -- SetHistory, save on Enter
- `internal/component/cli/model_test.go` -- 2 integration tests (edit + command mode)
- `cmd/ze/config/cmd_edit.go` -- Wire history from blob storage
- `cmd/ze/cli/main.go` -- Wire history from zefs
- `internal/component/cli/testing/runner.go` -- option=history:store, option=mode, restart=
- `internal/component/cli/testing/parser.go` -- StepRestart type
- `internal/component/cli/testing/headless.go` -- NewHeadlessCommandModel
- `test/editor/commands/cmd-history-persist.et` -- Edit-mode persistence test
- `test/editor/commands/cmd-history-persist-command.et` -- Command-mode persistence test
- `test/editor/commands/cmd-history-max.et` -- Rolling window persistence test
