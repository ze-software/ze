# 229 — Command Context Server Refactor

## Objective

Remove 6 redundant/dead fields from `CommandContext` and route all handler access through `Server`. When `Server` was added to `CommandContext`, 5 fields became duplicates and 2 were already dead (`Encoder`, `Serial`).

## Decisions

- Nil-safe accessor methods on `CommandContext` (`ctx.Reactor()`, etc.) rather than requiring `ctx.Server.Reactor()` everywhere — preserves nil-safety for tests that don't set up a full Server and keeps handler code clean.
- Server fields remain unexported; getter methods follow the existing pattern (`Server.Context()`, `Server.HasConfigLoader()`).
- Tests in the same `plugin` package set unexported Server fields directly — more explicit than a helper, each test documents exactly which dependencies it needs.

## Patterns

- Go's field/method name collision (cannot have field `Reactor` and method `Reactor()` on the same struct) requires a big-bang swap — the package won't compile in intermediate states, so all 111 sites must be migrated atomically.
- Dead field detection: grep for read usages, not just definitions. `Encoder` and `Serial` had write sites but zero read sites.

## Gotchas

- `handleBoRR`/`handleEoRR` in `refresh.go` had a pre-existing `dupl` lint issue (structurally identical) — required fixing as part of lint compliance by extracting `handleRefreshMarker`.
- 111 test construction sites across 8 files — underestimating mechanical scope is a risk; count first.

## Files

- `internal/component/plugin/command.go` — struct reduced to 3 fields, 4 nil-safe accessor methods added
- `internal/component/plugin/server.go` — 4 accessor methods added, 2 construction sites simplified
- 12 handler files — all `ctx.Field` → `ctx.Method()` access updated
- 8 test files — all 111 construction sites updated
