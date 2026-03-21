# 383 — Command Package Extraction

## Objective

Extract pipe operators, tree completer, and formatting utilities from `cmd/ze/cli/` into a shared `internal/component/command/` package so both CLI and config editor can reuse them.

## Decisions

- New package `internal/component/command/` holds all pipe/format/completer logic
- `CommandNode` tree and `TreeCompleter` extracted from editor's `completer_command.go` — editor now delegates via thin adapter
- `BuildCommandTree()` centralised in command package, takes `[]RPCInfo` instead of accessing plugin registry directly
- `ProcessPipesDefaultTable()` added — appends table at end of pipeline when no explicit format specified
- `| text` pipe added for space-aligned columns without box-drawing (uses `tableStyle{plain: true}`)
- `| count` returns `{"count": N}` JSON — count is a data transform, not a display format
- Multiple formatters (e.g., `| text | json`) return an error instead of silently passing through
- `HasFormatOp` checks only display formatters (json/table/text/yaml), not count

## Patterns

- `tableStyle` struct carries rendering mode through recursive render calls — avoids duplicating all render functions
- `drawBorder()` returns empty string in plain mode; `writeRow()` uses 2-space separators instead of `│`
- `FoldServerPipeline()` rewrites `rib show` pipe segments into server-side args, keeping only client-side ops

## Gotchas

- `HasFormatOp` initially included count, which suppressed default table formatting for all piped output
- Prepending default table (old approach) broke count — count would count table lines not data items. Appending at end is correct
- Editor's `CommandNode` became a type alias `= command.Node` to preserve API without breaking existing code

## Files

- Created: `internal/component/command/` (pipe.go, pipe_table.go, pipe_test.go, pipe_table_test.go, completer.go, completer_test.go, format.go, node.go)
- Deleted: `cmd/ze/cli/pipe.go`, `pipe_table.go`, `pipe_test.go`, `pipe_table_test.go`
- Modified: `cmd/ze/cli/main.go`, `cmd/ze/config/cmd_edit.go`, `internal/component/config/editor/completer_command.go`
