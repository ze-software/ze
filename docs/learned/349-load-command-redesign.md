# 349 — Load Command Redesign

## Objective

Replace the two-variant `load <file>` / `load merge <file>` editor commands with an explicit four-keyword syntax `load <source> <location> <action> [path]` that separates where content comes from, where it applies, and how it merges.

## Decisions

- Chose explicit keywords over defaults: user must specify all three dimensions (source, location, action); no implied behaviour. Ze has no users — clean break, no backwards compatibility.
- Paste mode (Ctrl-D terminator) chosen over pre-TUI detection for terminal input: Bubble Tea consumes stdin, so the editor must enter a paste-collecting state rather than reading stdin before TUI starts.
- Old syntax rejected with a clear error message pointing to new syntax, not silently remapped.

## Patterns

- `parseLoadArgs()` returns all four dimensions as typed values — a single parser function for dispatch rather than nested if/else on args.
- `replaceAtContext()` and `mergeAtContext()` for context-relative operations — operate on the current `contextPath` from the editor model.

## Gotchas

- Index out of bounds when `contextPath` has only 1 element (e.g., `["bgp"]` from `edit bgp`): code accessed `contextPath[len-2]` before checking length. Fixed by checking length first. Added `TestLoadFileRelativeReplaceSingleContext` and `TestLoadFileRelativeMergeSingleContext` to cover this.

## Files

- `internal/config/editor/model.go` — `parseLoadArgs()`, `cmdLoadNew()`, `applyLoadAbsolute()`, `applyLoadRelative()`, paste mode state
- `test/editor/lifecycle/` — new `.et` test files, deleted old `load-file.et` and `load-merge.et`
