# 152 — Hub Phase 1: Schema Infrastructure

## Objective

Extend the 5-stage startup protocol to support YANG schema declarations from plugins, and store collected schemas in a `SchemaRegistry` for handler routing.

## Decisions

- Schema declarations use `declare schema` prefix (consistent with existing `declare cmd`) — no new protocol mechanism needed.
- Multi-line YANG content sent via heredoc (`declare schema yang <<EOF ... EOF`) — reuses shell-convention familiar to operators.
- `FindHandler` uses longest-prefix match for routing — a plugin declaring handler `bgp.peer` receives config for `bgp.peer[address=x]` paths.
- Schema stored per-plugin in SubsystemHandler, aggregated via `AllSchemas()` / `RegisterSchemas()` — clean separation between collection and storage.
- CLI commands placed under `ze bgp schema` (not `ze hub schema`) — follows subsystem ownership principle.

## Patterns

- Heredoc state machine in SubsystemHandler: detect `<<EOF`, accumulate lines, terminate on `EOF`. Stateful parsing inline with the main protocol loop.

## Gotchas

None.

## Files

- `internal/component/plugin/schema.go` — SchemaRegistry, Schema types, Register/FindHandler
- `internal/component/plugin/registration.go` — added `declare schema` parsing and heredoc handling
- `internal/component/plugin/subsystem.go` — schema collection during Stage 1
- `cmd/ze/bgp/schema.go` — CLI commands for schema discovery
