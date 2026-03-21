# 004 — Edit Command

## Objective

Implement an interactive config editor (`ze bgp config edit`) using Bubble Tea with VyOS-like set commands, schema-driven tab completion (ghost text + dropdown), and backup/rollback support.

## Decisions

- Ghost text (single match shows suffix inline) chosen over always showing dropdown, to reduce visual noise for common completions.
- Wildcard template (`edit neighbor *`) creates an inheritance template in the config file itself, stored as `neighbor * { }`, not a separate metadata file — config file is the single source of truth.
- File-based editing with backup on commit (not live apply to running daemon) — live apply was explicitly deferred to a second plan.
- Reused existing `setparser.go` for set/delete command parsing rather than re-implementing tree modification logic.

## Patterns

- Completion engine navigates the schema tree based on input tokens; the schema already knows valid keywords and value types.
- Backup naming: `<dir>/<name>-YYYY-MM-DD-<N>.conf` (date + sequence number, same directory as original).

## Gotchas

- API integration (applying diff to running daemon) was explicitly out of scope and deferred.

## Files

- `internal/component/config/editor/` — editor.go, model.go, completer.go, model_commands.go, model_render.go, reload.go, validator.go
- `cmd/ze/config/cmd_edit.go` — CLI entry point
