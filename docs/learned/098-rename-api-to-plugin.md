# 098 — Rename `ze bgp api` to `ze bgp plugin`

## Objective

Rename the CLI command from `zebgp api` to `ze bgp plugin` to clarify that "api" referred to the protocol, not the subcommand. ~250 files affected.

## Decisions

- CLI renamed (`ze bgp plugin`) but config block syntax kept as `api { ... }` — config "api" refers to the API protocol between ze and external processes, not the CLI subcommand name.
- Internal config types `APIBindings`, `PeerAPIBinding`, `Field("api", ...)` also kept — they name the protocol concept, not the CLI.
- Hard break: no backward compat, no aliases. Ze has never been released.

## Patterns

- When renaming something with dual meaning (CLI term vs protocol term), rename only the CLI-facing surface; keep protocol-facing names intact.

## Gotchas

- `test/data/api/` directory rename to `test/data/plugin/` was the largest mechanical change (110 files, content unchanged except `reconnect.conf`).

## Files

- `cmd/ze/bgp/plugin.go` (renamed from `api.go`), `plugin_rr.go`, `plugin_persist.go`
- `internal/plugin/` package — 66 files `package api` → `package plugin`
- 18 import path updates across consumers
