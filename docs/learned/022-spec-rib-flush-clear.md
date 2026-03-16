# 022 — RIB Flush and Clear

## Objective

Implement three API commands for RIB management: `rib flush out` (re-send all Adj-RIB-Out routes), `rib clear in` (clear received routes), `rib clear out` (withdraw all announced routes).

## Decisions

Mechanical implementation following existing handler patterns. No novel design decisions.

## Patterns

- Three new reactor methods: `ClearRibIn() int`, `ClearRibOut() int`, `FlushRibOut() int` — each returns count of routes affected.
- Three new API handlers in `handler.go`, registered as `rib flush out`, `rib clear in`, `rib clear out`.
- Response format matches existing `handleRIBShowIn` pattern: `{"status": "ok", "routes_cleared": N}`.
- ExaBGP reference: `flush_adj_rib_out` → re-send, `clear_adj_rib in` → clear received, `clear_adj_rib out` → withdraw.

## Gotchas

- `rib flush out` semantics: re-queues all *previously sent* routes for re-announcement — useful after peer reconnect to force full resync without waiting for session re-establishment.

## Files

- `internal/reactor/reactor.go` — ClearRibIn(), ClearRibOut(), FlushRibOut()
- `internal/component/plugin/handler.go` — three new handlers + registration
