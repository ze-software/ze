# 356 — Editor Modes

## Objective
Add dual-mode support to the ze config editor: edit mode (config editing) and command mode (operational commands via daemon RPC). Users toggle with `/command` and `/edit`.

## Decisions
- Mode as enum (`EditorMode`), not interface — two modes is too few for abstraction
- Command executor injected as `func(string) (string, error)` — editor doesn't know about socket/RPC details
- Command completer wraps RPC command tree built from `AllBuiltinRPCs()` + `BgpHandlerRPCs()`
- Per-mode state save/restore: viewport content, scroll position, history, historyTmp, status message
- No-daemon graceful degradation: completions work, execution shows clear error
- `/` prefix for mode commands (IRC/Slack convention) — no collision with config or operational commands

## Patterns
- Bubble Tea `tea.Cmd` pattern for async command execution — `executeOperationalCommand` returns a `tea.Cmd` that produces `commandResultMsg`
- Same `commandResultMsg` type used for both edit-mode and command-mode results — the `handleCommandResult` handler is shared
- Mode-aware `updateCompletions()` dispatches to the right completer based on `m.mode`
- `buildEditorCommandTree()` strips `bgp ` prefix so users type `peer list` not `bgp peer list`

## Gotchas
- `ze:validate` extensions on YANG typedefs do NOT propagate to entries referencing the typedef — goyang resolves the type pattern but not custom extensions. Must keep explicit `ze:validate` on leaf nodes alongside `zt:address-family`.
- List key leaves (e.g., `address` in `peer`) appear as children in the YANG schema but should not be offered as completions or accepted as settable fields — the key is already the list identifier.
- `isTemplate` prompt branch was dead code (identical to non-template) — removed during mode-aware prompt refactor.
- The HeadlessModel test framework's `checkPrompt` needed mode awareness — it only knew about `ze#`/`ze[path]#`, not `ze>`.

## Files
- `internal/component/config/editor/model_mode.go` — EditorMode, modeState, SwitchMode
- `internal/component/config/editor/completer_command.go` — CommandCompleter
- `internal/component/config/editor/model.go` — mode fields, handleEnter intercept
- `internal/component/config/editor/model_render.go` — mode-aware prompt
- `cmd/ze/config/cmd_edit.go` — wireCommandExecutor, buildEditorCommandTree
- `internal/component/config/editor/testing/expect.go` — mode expectation type
- `test/editor/mode/` — functional .et tests
