# 385 — Editor Mode Switching Redesign

## Objective

Replace `/command` with `run` for mode switching and add cross-mode completions so users can type any command from either mode.

## Decisions

- `run` (bare) in edit mode switches to command mode; `run <args>` switches and executes immediately
- `edit` (bare) in command mode switches to edit mode
- Config commands (`set`, `delete`, `show`, etc.) typed in command mode auto-switch to edit mode and execute
- Cross-mode completions: `run ` prefix in edit mode gets operational completions; edit commands with args in command mode get YANG completions
- Command mode top-level merges operational + config command completions — user sees everything available
- Removed `/command` and `/edit` slash-command variants entirely

## Patterns

- `isEditCommandWithArgs()` distinguishes "set" (still at command level, show in merged list) from "set " (entering YANG path, switch to YANG-only completions)
- `updateCompletions()` uses a 4-way switch: edit+run-prefix, command+edit-args, command-toplevel, edit-default
- `applyCompletion()` works correctly with "run " prefix because it replaces the last word in the full input, leaving the prefix intact
- Ghost text also works cross-mode: `commandCompleter.GhostText("pe")` returns "er" which appends correctly after "run pe"

## Gotchas

- `edit` serves double duty: mode switch (bare) AND config command (`edit bgp peer ...`) — `handleEnter` distinguishes by checking if bare or has args
- Tests using `"edit"` in command tree broke because `isEditCommandWithArgs` routes to YANG completions — changed test tree to use `"peer > show"` instead
- `TestCommandModeCompletionsWired` expected exactly 2 completions — now expects merged (operational + edit commands)
- RPC registration kept `edit` for mode switching back; only `command` was renamed to `run`

## Files

- Modified: `model.go` (updateCompletions cross-mode logic, handleEnter rewrite)
- Modified: `model_mode.go` (isEditCommandWithArgs, editModeCommands map)
- Modified: `model_mode_test.go` (updated expectations, added cross-mode tests)
- Modified: `completer.go` (cmdRun in commands list)
- Modified: `init.go` (RPC: command→run)
- Modified: `model_render.go` (help text updated)
