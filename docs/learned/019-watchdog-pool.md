# 019 — Watchdog Pool

## Objective

Design and implement a global pool architecture for watchdog routes enabling dynamic route creation via API (not just toggling config-defined routes), with per-peer announcement state tracking.

## Decisions

- Global pools indexed by name (`Reactor.watchdogPools map[string]*WatchdogPool`) instead of per-peer groups — enables API to create routes that apply to all peers without specifying each peer.
- Per-peer announcement state stored in each `PoolRoute` (`announced map[string]bool`) — routes are re-sent on peer reconnect based on this state.
- On peer disconnect: state is preserved (not reset) — routes remain "announced" logically; they will be re-sent when the peer reconnects.
- `next-hop self` resolved per-peer at send time using `peer.Settings().LocalAddress` — not stored as a resolved IP.
- Check global pools first, then per-peer groups, for backward compatibility with existing config-based watchdog groups.

## Patterns

- Pool operations: `AnnouncePool` returns routes where `announced=false`; `WithdrawPool` returns routes where `announced=true` — caller sends the returned routes.

## Gotchas

- None documented.

## Files

- `internal/reactor/watchdog.go` — WatchdogPool, PoolRoute, WatchdogManager, pool operations
