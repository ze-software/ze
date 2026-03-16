# 106 — msg-id Cache Control Commands

## Objective

Add API commands for plugins to manage UPDATE cache lifetime: `msg-id retain`, `msg-id release`, `msg-id expire`, `msg-id list`. Enables plugins to hold references for graceful restart replay.

## Decisions

- Mechanical implementation, no design decisions beyond following existing handler registration pattern (`RegisterXxxHandlers(d)` in `handler.go`).

## Patterns

- Cache entries get a `retained` flag: retained entries survive the lazy TTL-based cleanup, persist in `List()`, and can still be `Take()`n. Release clears the flag and resets TTL.
- Handler registration: create `msgid.go` with handler functions, register via `RegisterMsgIDHandlers(d)` in `handler.go`. Matches existing pattern for all other command groups.

## Gotchas

None.

## Files

- `internal/reactor/recent_cache.go` — `retained` flag, `Retain()`, `Release()`, `List()` methods
- `internal/component/plugin/msgid.go` — command handler implementations
- `internal/component/plugin/handler.go` — `RegisterMsgIDHandlers` registration
- `internal/component/plugin/types.go` — `RetainUpdate`, `ReleaseUpdate`, `ListUpdates` added to `ReactorInterface`
