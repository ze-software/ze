# 339 — Config Completion CLI

## Objective

Add a `ze config completion` CLI command for non-interactive probing of the YANG-driven completer, enabling quick bug triage and regression testing without the full TUI.

## Decisions

- `--input` flag uses `+` encoding for spaces to avoid shell quoting issues in test runner
- `--context` uses `/` separator to avoid `strings.Fields()` splitting on dotted paths
- New `test/ui/` directory with `ze-test bgp ui` runner for completion functional tests
- Completer bugs (missing `environment`/`plugin` roots, `show`/`edit` missing leaves, set-path list key navigation) fixed in-place rather than deferred

## Patterns

- CLI completion commands reuse the existing `Completer` type directly — no TUI dependency needed, just wiring
- Functional tests for completion use the same `.ci` format as encoding tests, run via `EncodingTests` runner
- `+` encoding in test flags is a lightweight alternative to shell escaping for space-containing arguments

## Gotchas

- The `Completer` had several bugs only visible when exercised from CLI context (root-level targets missing, leaf nodes filtered from `show`/`edit`)
- List key navigation in `set` paths required special handling — descending into a list entry context differs from container navigation

## Files

- `cmd/ze/config/cmd_completion.go` — new CLI handler
- `internal/component/config/editor/completer.go` — bug fixes
- `internal/component/config/editor/completer_test.go` — expanded tests
- `test/ui/` — new functional test directory
