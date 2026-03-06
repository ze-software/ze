# 372 — Show Routes (ze show)

## Objective

Add `ze show` convenience command with dynamically discovered read-only commands
from the same RPC registrations that power interactive CLI autocomplete.

## Decisions

- `ze show` exposes only read-only RPCs (`ReadOnly: true`); `ze run` exposes all commands
- Shared `cmdutil` package extracts common logic (tree walk, validation, suggestion, help listing)
- Local validation via `IsValidCommand()` rejects unknown commands before connecting to daemon
- Single source of truth: `AllCLIRPCs()` → `BuildCommandTree(readOnly)` drives both show/run and interactive CLI

## Patterns

- `BuildCommandTree(readOnly bool)` with filter — same tree builder, flag controls inclusion
- `cmdutil.RunCommand()` handles --socket extraction, validation, delegation to `cli.Run(["--run", ...])`
- `SuggestFromTree()` reuses existing Levenshtein `suggest.Command()` for "did you mean?" hints
- Package-level `var tree = cli.BuildCommandTree(true)` — built once at init, zero runtime cost

## Gotchas

- First attempt used hardcoded command mapping (`resolveShowCommand` with routes/summary) —
  user rejected in favor of dynamic discovery from RPC registrations
- `ze show` and `ze run` ended up nearly identical — extracted shared `cmdutil` to avoid duplication

## Files

- `cmd/ze/show/main.go` — show command (read-only tree, help, validation)
- `cmd/ze/run/main.go` — run command (all-commands tree)
- `cmd/ze/internal/cmdutil/cmdutil.go` — shared utilities
- `cmd/ze/cli/main.go` — exported `AllCLIRPCs`, `BuildCommandTree`, `Command`
- `internal/component/plugin/server/handler.go` — `ReadOnly` field on `RPCRegistration`
