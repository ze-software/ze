# 199 — CLI Restructure

## Objective

Move config/schema/validate commands from `ze bgp` to the `ze` root level; create separate packages per command.

## Decisions

- `ze bgp validate` → `ze validate`, `ze bgp schema` → `ze schema`, `ze bgp config-dump` → `ze config dump`.
- Deprecation warnings (not hard errors) on old paths — allows existing scripts to continue while signaling the change.
- Separate packages per command (`cmd/ze/validate/`, `cmd/ze/schema/`) for modularity; avoids one package owning unrelated concerns.

## Patterns

- None.

## Gotchas

- None.

## Files

- `cmd/ze/validate/` — validate command (moved from bgp)
- `cmd/ze/schema/` — schema command (moved from bgp)
- `cmd/ze/config/` — config dump subcommand
- `cmd/ze/main.go` — root dispatch updated
