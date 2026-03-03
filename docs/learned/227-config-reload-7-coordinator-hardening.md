# 227 — Config Reload 7: Coordinator Hardening

## Objective

Harden the reload coordinator against plugin failures and wire all reload entry points through it. Addressed gaps from spec 2's critical review: crashed plugins were silently skipped, apply errors were swallowed, and the daemon RPC bypassed the coordinator entirely.

## Decisions

- `connB == nil` during verify is an error, not a silent skip — plugin registration without a live connection is broken.
- Apply errors are collected and returned to the caller after `SetConfigTree` still runs — the reactor has already applied, so the config tree must reflect the new state regardless.
- A pre-apply alive check aborts the reload if any plugin dies between verify and apply phases, preventing partial applies.
- `handleDaemonReload` gains access to `Server` via `CommandContext.Server` field, enabling the coordinator path when `HasConfigLoader()` is true.

## Patterns

- `beforeVerifyRsp` hook pattern: blocks coordinator before sending verify response, allowing tests to mutate state (nil a `connB`) between phases for deterministic inter-phase testing.
- Only nil-ing the connB pointer (not closing the socket) is safe during in-flight verify — closing breaks the response read.
- `Server.Context()` accessor needed to give handlers the proper server context instead of `context.Background()`.

## Gotchas

- `handleDaemonReload` used `context.Background()` instead of the server context — subtle but important for shutdown propagation.
- `TestReloadApplyCrashedPlugin` was renamed because the test nils connB before the reload starts, so verify catches it, not apply — the name was misleading.
- Reactor `ApplyConfigDiff` error was only logged, not returned — critical review caught this as a missed case.

## Files

- `internal/plugin/reload.go` — verify crash detection, pre-apply check, apply error aggregation, crash handling docs
- `internal/plugin/bgp.go` — `handleDaemonReload` coordinator path with fallback
- `internal/plugin/command.go` — added `Server *Server` field to `CommandContext`
- `internal/plugin/server.go` — `Context()` accessor, Server wiring in two construction sites
