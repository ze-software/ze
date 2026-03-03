# 169 — API Plugin Command Registration

## Objective

Allow external plugin processes to extend ZeBGP's command set at runtime, so CLI users can discover and invoke plugin-defined commands without ZeBGP knowing about them at compile time.

## Decisions

- One process per command (single ownership): no load-balancing or multi-process sharing — simplicity over flexibility.
- Plugins cannot shadow builtins: registration rejected if name conflicts; first registration wins for plugin-vs-plugin conflicts.
- Shared prefix tree with unique leaves: `myapp status` and `myapp config` can coexist across different processes; full command name must be unique.
- Sync dispatch (CLI blocks until response): async can be added later; sync is simpler and sufficient.
- 30s default command timeout, 500ms for completion requests — completion must be fast or users abandon it.
- Per-process limit of 100 pending requests: prevents memory exhaustion from stuck processes.
- `completable` flag on registration: ZeBGP only routes argument completion requests to processes that declared support, avoiding unnecessary round-trips.

## Patterns

- Process death → `registry.UnregisterAll(proc)` + `pending.CancelAll(proc)`: cleans all in-flight state atomically.
- Streaming response: `@serial+` for partial chunks, `@serial done` terminates — same serial, `+` suffix is the continuation marker.
- Longest-prefix match sorted by name length descending: `peer status` matches before `peer` for input `peer status web`.

## Gotchas

- Completion timeout must be fixed (500ms), NOT the per-command timeout — users wait for tab completions interactively; configurable timeouts per command don't apply to completions.
- Process must re-register commands after respawn — registry is cleared on process death; the new instance is a fresh start.

## Files

- `internal/plugin/registry.go` — `CommandRegistry` type
- `internal/plugin/pending.go` — `PendingRequests` with alpha serial counter
- `internal/plugin/command.go` — `Dispatcher` extended with registry + pending
- `internal/plugin/server.go` — `register`/`unregister`/`@serial` parsing in `handleOutput`
