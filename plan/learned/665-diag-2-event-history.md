# 665 -- Diag 2: Event History and FSM Transition Log

## Context

`spec-diag-2-event-history` added three diagnostic capabilities so operators
(and Claude via MCP) can answer "what happened in the last N minutes?":

1. Global event ring tapping `Server.deliverEvent()`
2. Per-peer BGP FSM transition history
3. Per-tunnel and per-session L2TP FSM transition history

## Decisions

- Global ring lives in `internal/component/plugin/server/event_ring.go` because
  `deliverEvent()` is the single dispatch point for ALL events. A ring append
  there captures every event without subscribing individually.
- Fixed-size circular buffer (overwrite-oldest) with non-blocking append. No
  allocation on the emit path.
- BGP FSM history uses the PeerState enum (4 states: idle, active, opensent,
  established) rather than the internal fsm.State (6 states), since PeerState
  is what operators observe.
- L2TP reuses the same ring pattern. Both tunnel and session own their own
  `fsmHistoryRing`. Transitions are recorded at the point where state actually
  changes in `tunnel.go` and `session.go`.
- Show commands:
  - `show event recent [count N] [namespace X]` via `ze-show:event-recent`
  - `show event namespaces` via `ze-show:event-namespaces`
  - `show bgp peer <sel> history` via `ze-bgp:peer-history`
  - `show l2tp tunnel history <tid>` via `ze-l2tp-api:tunnel-history`
  - `show l2tp session history <sid>` via `ze-l2tp-api:session-history`

## Consequences

- Operators get instant access to recent event flow and FSM transitions
  without enabling debug logging or external tooling.
- Ring capacity is fixed at compile time (configurable per ring type).
  Oldest entries are silently overwritten.
- Zero runtime cost when no one queries: the ring just rotates.

## Gotchas

- The global ring taps deliverEvent BEFORE subscriber dispatch, so ring
  entries are visible even if all subscribers reject the event.
- BGP peer history is per-Peer struct instance. If a peer is removed and
  re-added, history starts fresh (the old Peer struct is garbage collected).
- L2TP FSM history ring capacity is 16 entries per tunnel/session. Tunnel
  handshakes typically produce 3-5 transitions, so this covers multiple
  lifecycle iterations.

## Files

- `internal/component/plugin/server/{event_ring.go,event_ring_test.go}`
- `internal/component/plugin/server/dispatch.go` (ring append in deliverEvent)
- `internal/component/cmd/show/show.go` (handleShowEventRecent, handleShowEventNamespaces)
- `internal/component/bgp/reactor/{peer_history.go,peer_history_test.go,peer.go,reactor.go}`
- `internal/component/bgp/plugins/cmd/peer/peer.go` (handlePeerHistory)
- `internal/component/l2tp/{fsm_history.go,tunnel.go,session.go,session_fsm.go}`
- `internal/component/l2tp/{snapshot.go,subsystem_snapshot.go,service_locator.go}`
- `internal/component/cmd/l2tp/l2tp.go` (handleTunnelHistory, handleSessionHistory)
- YANG: `ze-cli-show-cmd.yang`, `ze-l2tp-cmd.yang`
