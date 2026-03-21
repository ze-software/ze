# 067 — Peer Lifecycle Callbacks

## Objective

Add `PeerLifecycleObserver` interface to the reactor so components (initially the API server) can receive notifications when peers reach or leave Established state, enabling ExaBGP-compatible `{"type":"state","neighbor":{"state":"up/down"}}` messages.

## Decisions

- Observer slice copied under read lock, iterated without lock — prevents deadlock if observer accesses reactor; slice is read-only after registration so copy is safe
- Peer holds a `reactor *Peer` reference protected by `peer.mu`; callbacks invoke reactor methods via this reference
- Callbacks must not block — documented contract; future consideration: async dispatch if latency becomes a problem
- `OnPeerEstablished` fires BEFORE `sendInitialRoutes()` — intentional ordering so plugins see Established before routes arrive
- Close reason distinguishes "connection lost" (to Idle) from "session closed" (other transitions from Established)

## Patterns

- Observer registered via `AddPeerObserver()` — pattern is extensible: Phase 1 (future) wraps observers in Plugin interface with dependency ordering

## Gotchas

- FSM callback is set during session creation, not peer creation — `SetReactor()` must be called on the Peer before sessions are created
- `apiStateObserver` is nil-safe (checks `server == nil`) — safe to register even when API server is not configured

## Files

- `internal/reactor/reactor.go` — PeerLifecycleObserver interface, peerObservers, notifyPeerEstablished/Closed, apiStateObserver, AddPeerObserver
- `internal/reactor/peer.go` — reactor field, SetReactor(), modified FSM callback
- `internal/component/plugin/server.go` — EmitPeerState()
