# BGP FSM: Active State

## TL;DR

The system is listening for an incoming TCP connection. Entered from Idle
on `ManualStart` when the peer is configured with the passive bit, or from
OpenSent when the TCP connection fails. Exits
to OpenSent once a remote peer connects and the session layer accepts,
or back to Idle on stop or error.

RFC 4271 Section 8.2.2 "Active state".

<!-- source: internal/component/bgp/fsm/state.go — StateActive -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleActive -->

## Entry

Entered from `Idle` on `EventManualStart` when the FSM passive flag is
true. `Session.Start()` fires the event, the passive check in
`handleIdle` picks Active over Connect.

Also entered from `OpenSent` on `EventTCPConnectionFails` (RFC 4271
Section 8.2.2: OpenSent "changes its state to Active"). That Session is
finished by then: its connection is closed, and the peer run loop starts
the next attempt on a new Session.

<!-- source: internal/component/bgp/fsm/fsm.go — handleIdle passive branch -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenSent EventTCPConnectionFails -->
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
| `EventManualStart` / `EventAutomaticStartWithDampPeerOscillations` | duplicate `Session.Start()` / `startDamped()` call | ignored (RFC 4271) | none | `Active` |
| `EventManualStop` | `Session.Stop` / `Session.Teardown` | cleanup in caller; **sets ConnectRetryCounter to zero** | Cease NOTIFICATION from `Session.Teardown` when a conn exists; `Session.Stop` sends nothing | `Idle` |
| `EventAutomaticStop` / `EventOpenCollisionDump` | `Session.teardownAutomatic` / `Session.CloseWithNotification` | cleanup in caller; **increments ConnectRetryCounter** | Cease NOTIFICATION in caller | `Idle` |
| `EventTCPConnectionConfirmed` | `Session.connectionEstablished` after `Accept` | log transition | OPEN sent immediately after transition | `OpenSent` |
| `EventTCPConnectionFails` | not generated in production: Active holds no connection, because without DelayOpen `EventTCPConnectionConfirmed` moves it to OpenSent at once, and a `connectionEstablished` setup error returns an error without firing an event. The Event 18 producers (`Session.Connect` dial failure, `handleConnectionClose`, the forward-pool congestion teardown) fire in Connect or later states. RFC4271-8.2.2-13 carries `{feature-declined}` for this reason | **increments ConnectRetryCounter** (FSM arm kept for completeness) | none | `Idle` |
| `EventConnectRetryTimerExpires` | not generated in production | passive check: if not passive, go to Connect | none | `Connect` or `Active` |
| `EventBGPHeaderErr` / `EventBGPOpenMsgErr` / `EventNotifMsgVerErr` / `EventNotifMsg` | message decode error paths | log transition; **increments ConnectRetryCounter** | NOTIFICATION in caller | `Idle` |
| any other event | unexpected | log transition | none | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleActive -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Accept, acceptWithOpen -->

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
  buffer sizes), snapshots local addresses for NEXT_HOP validation, and
  publishes the socket and its observed buffered writer together. Only then
  does it fire the FSM event, send OPEN, and start the OPEN-wait hold timer
  (`ze.bgp.openwait`, default 120 seconds). A sealed Session refuses setup.
  <!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished -->
- **On `EventManualStop`:** if a partial connection exists, the caller
  (`Teardown`) sends a Cease NOTIFICATION before invoking the FSM event.
  `Session.Stop` fires the same event and sends nothing.
  <!-- source: internal/component/bgp/reactor/session_connection.go — Teardown -->
  <!-- source: internal/component/bgp/reactor/session.go — Stop -->

## `acceptWithOpen` variant

When inbound collision resolution has already read the peer's OPEN from
a competing socket, the reactor retains the socket, parsed OPEN, and original
wire bytes for a fresh peer-owned Session after the losing cycle is cleaned up.
The run loop calls `acceptWithOpen(conn, peerOpen, wire)` on that Session.
This path:

1. Calls `connectionEstablished`, which refuses a sealed Session before
   publishing the socket, otherwise fires `EventTCPConnectionConfirmed`
   (Active -> OpenSent) and sends the local OPEN.
2. Observes the original received OPEN bytes on the winning Session.
3. Calls `processOpen(peerOpen)` to validate and negotiate the OPEN, then
   `advanceAfterOpen` to enter OpenConfirm and send KEEPALIVE when permitted.

So from the outside, a single incoming connection with a pre-buffered
OPEN can drive the FSM from Active through OpenSent to OpenConfirm in
one synchronous sequence.

The transition and KEEPALIVE share `Session.advanceAfterOpen`; under BFD
strict mode it does neither: KEEPALIVE is withheld, no `EventBGPOpen`
is fired, and the session waits in OpenSent carrying a sub-state until the
BFD session is Up (draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.5). The
collision winner gets the same wait as any other connection, because the
draft draws no distinction between them. The OpenSent runbook has the
whole fork.

<!-- source: internal/component/bgp/reactor/session_connection.go — acceptWithOpen -->
<!-- source: internal/component/bgp/reactor/session_connection.go — processOpen -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — resolvePendingCollision -->
<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleActive` |
| Accept and socket setup | `internal/component/bgp/reactor/session_connection.go` | `Accept`, `acceptWithOpen`, `connectionEstablished` |
| Pre-buffered OPEN path for collision resolution | `internal/component/bgp/reactor/session_connection.go` | `processOpen` |
| Peer-level inbound connection dispatch | `internal/component/bgp/reactor/peer_run.go` | `takeInboundConnection`, run loop |

<!-- source: internal/component/bgp/fsm/fsm.go — handleActive -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Accept, acceptWithOpen, connectionEstablished, processOpen -->
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
