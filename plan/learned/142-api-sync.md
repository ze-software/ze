# 142 — API Process Synchronization

## Objective

Fix a race condition where API processes (like `persist`) hadn't finished initializing before BGP sessions became Established and EOR was sent, violating RFC 4724 (routes must precede EOR). The `teardown` functional test was flaky because of this.

## Decisions

- Chose `plugin session ready` as the signal API processes send after initialization — mandatory protocol, not optional. Ze BGP waits for all configured processes to signal ready before starting peer connections.
- 5-second timeout: if not all processes signal ready within timeout, Ze BGP proceeds anyway with a warning log.
- Same mechanism applies per-session: on state-up (reconnect), Ze BGP waits for ready signals before sending EOR so route replay happens in order.

## Patterns

- `APISyncState` with `atomic.Int32` ready counter and a channel closed when all ready — avoids mutex overhead on the hot path.

## Gotchas

- API tests revealed attribute handling bugs unrelated to API sync: `buildRIBRouteUpdate` used hardcoded LOCAL_PREF=100 instead of stored value, and `AnnounceRoute` omitted MED/LOCAL_PREF/communities. Tests improved from 7/14 to 14/14 after fixing both sync and attributes.

## Files

- `internal/reactor/api_sync.go` — `APISyncState`, `WaitForAPIReady()`, `SignalAPIReady()`
- `internal/reactor/peer.go` — per-session sync
- `test/scripts/ze_bgp_api.py` — added `ready()` function
- All `.run` scripts — call `ready()` after initialization
