# BGP FSM: Active State

## TL;DR

The system is listening for an incoming TCP connection. Entered from Idle
on `ManualStart` when the peer is configured with the passive bit. Exits
to OpenSent once a remote peer connects and the session layer accepts,
or back to Idle on stop or error.

RFC 4271 Section 8.2.2 "Active state".

<!-- source: internal/component/bgp/fsm/state.go — StateActive -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleActive -->

## Entry

Entered from `Idle` on `EventManualStart` when the FSM passive flag is
true. `Session.Start()` fires the event, the passive check in
`handleIdle` picks Active over Connect.

<!-- source: internal/component/bgp/fsm/fsm.go — handleIdle passive branch -->
<!-- source: internal/component/bgp/reactor/session.go — Start -->

A peer in Active does not dial. It waits for `Session.Accept(conn)` to
be called from the reactor's inbound connection plumbing. Accept runs
`connectionEstablished(conn)`, which fires
`EventTCPConnectionConfirmed`, moving the FSM to OpenSent.

<!-- source: internal/component/bgp/reactor/session_connection.go — Accept -->
<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished -->

## Events handled in Active

| Event | Produced by | FSM reaction | Wire side effect | Next state |
|-------|-------------|--------------|------------------|------------|
| `EventManualStart` | duplicate `Session.Start()` call | ignored (RFC 4271) | none | `Active` |
| `EventManualStop` | `Session.Close/Teardown/CloseWithNotification` | cleanup in caller | Cease NOTIFICATION in caller if conn exists | `Idle` |
| `EventTCPConnectionConfirmed` | `Session.connectionEstablished` after `Accept` | log transition | OPEN sent immediately after transition | `OpenSent` |
| `EventTCPConnectionFails` | inbound connection setup error | cleanup in caller | none | `Idle` |
| `EventConnectRetryTimerExpires` | not generated in production | passive check: if not passive, go to Connect | none | `Connect` or `Active` |
| `EventBGPHeaderErr` / `EventBGPOpenMsgErr` / `EventNotifMsgVerErr` / `EventNotifMsg` | message decode error paths | log transition | NOTIFICATION in caller | `Idle` |
| any other event | unexpected | log transition | none | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleActive -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Accept, AcceptWithOpen -->

## Timers running in this state

| Timer | Status | Notes |
|-------|--------|-------|
| ConnectRetryTimer | conceptually per RFC, not started | Same note as Connect: reconnect logic is at the peer level. |
| HoldTimer | not running | started on entry to OpenSent. |
| KeepaliveTimer | not running | started later. |

<!-- source: internal/component/bgp/fsm/timer.go — Timers -->

## Wire side effects

- **On `EventTCPConnectionConfirmed`:** after `Accept` wires the socket
  into the session, `connectionEstablished` tunes TCP (nodelay, TOS,
  buffer sizes), fires the FSM event, sends OPEN, and starts the hold
  timer.
  <!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished -->
- **On `EventManualStop`:** if a partial connection exists, the caller
  (`Close`, `Teardown`) sends a Cease NOTIFICATION before invoking the
  FSM event.
  <!-- source: internal/component/bgp/reactor/session_connection.go — Close, Teardown -->

## `AcceptWithOpen` variant

When inbound collision resolution has already read the peer's OPEN from
a competing socket, the reactor calls `AcceptWithOpen(conn, peerOpen)`.
This path:

1. Calls `connectionEstablished` which fires
   `EventTCPConnectionConfirmed` (Active -> OpenSent).
2. Then calls `processOpen(peerOpen)` which fires `EventBGPOpen`
   (OpenSent -> OpenConfirm).
3. Then sends our KEEPALIVE.

So from the outside, a single incoming connection with a pre-buffered
OPEN can drive the FSM from Active through OpenSent to OpenConfirm in
one synchronous sequence.

<!-- source: internal/component/bgp/reactor/session_connection.go — AcceptWithOpen -->
<!-- source: internal/component/bgp/reactor/session_connection.go — processOpen -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleActive` |
| Accept and socket setup | `internal/component/bgp/reactor/session_connection.go` | `Accept`, `AcceptWithOpen`, `connectionEstablished` |
| Pre-buffered OPEN path for collision resolution | `internal/component/bgp/reactor/session_connection.go` | `processOpen` |
| Peer-level inbound connection dispatch | `internal/component/bgp/reactor/peer_run.go` | `takeInboundConnection`, run loop |

<!-- source: internal/component/bgp/fsm/fsm.go — handleActive -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Accept, AcceptWithOpen, connectionEstablished, processOpen -->
<!-- source: internal/component/bgp/reactor/peer_run.go — inbound connection takeover around session.Accept -->

## RFC deviations

- **`EventConnectRetryTimerExpires` branch.** RFC 4271 specifies that an
  Active-state ConnectRetryTimer expiry should restart the timer,
  initiate a TCP connection, and move to Connect. Ze's handler keeps the
  "transition to Connect only if not passive" logic as a safety net,
  but the event is never generated in production. See Connect runbook
  for the reasoning.
  <!-- source: internal/component/bgp/fsm/fsm.go — ARCHITECTURAL NOTES -->

## Tests exercising this state

- `internal/component/bgp/fsm/fsm_test.go` — direct state transition
  tests for Active including `Accept`-driven `TCPConnectionConfirmed` and
  error event handling.
  <!-- source: internal/component/bgp/fsm/fsm_test.go -->
- `internal/component/bgp/reactor/session_test.go` — listener-side
  session tests covering inbound accept flow.
  <!-- source: internal/component/bgp/reactor/session_test.go -->
