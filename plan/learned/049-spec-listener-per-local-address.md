# 049 — Listener Per Local Address

## Objective

Replace the single global BGP listener (+ `TCP.Bind` env var) with per-peer listeners derived automatically from each peer's `LocalAddress` field, reducing attack surface and eliminating config redundancy.

## Decisions

- `LocalAddress` is now mandatory for all peers — missing it is a hard validation error. This is a breaking change with no automatic migration.
- Listeners keyed by `netip.Addr` in `map[netip.Addr]*Listener`; multiple peers sharing the same `LocalAddress` share one listener.
- Connection handler signature extended to `handleConnectionWithContext(conn, listenerAddr)` — listener address is passed to verify the connection arrived on the expected interface (RFC compliance check).
- Peer lookup on incoming connection is by remote IP (peer's `Address`), NOT by listener address — the listener address is only for validation.
- Dynamic listener lifecycle: `AddPeer` starts a new listener if the `LocalAddress` has no listener yet; `RemovePeer` stops the listener only when the last peer using that address is removed.
- `TCP.Bind` removed from `TCPEnv`; `ListenAddr` removed from `reactor.Config`. Both replaced by peer-derived addresses.
- Self-referential peers (`Address == LocalAddress`) are rejected. Link-local IPv6 addresses are rejected (require zone ID, not portable).

## Patterns

- Startup cleanup on partial failure: if listener N fails to start, all already-started listeners are stopped before returning error — no partial state left behind.
- `ListenAddrs()` returns all active listener addresses for inspection.

## Gotchas

- `TCP.Bind` was environment-variable-only (not in config files) — no config file migration needed, only env var users affected.
- Connection-to-wrong-listener check (peer connects to a local address other than its configured `LocalAddress`) catches remote misconfiguration and routing anomalies. Should be rare in practice.
- `Config.Port = 0` is now rejected with a clear error — previously it silently produced an invalid listen address.

## Files

- `internal/reactor/reactor.go` — `listeners map[netip.Addr]*Listener`, `startListenerForAddress()`, `handleConnectionWithContext()`
- `internal/component/config/environment.go` — removed `TCPEnv.Bind`
- `internal/reactor/peersettings.go` — `LocalAddress` documented as required
- `internal/component/config/loader.go` — validation for missing/invalid `LocalAddress`
