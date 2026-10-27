# BGP FSM: Active State

## TL;DR

The system is listening for an incoming TCP connection. Entered from Idle
on `ManualStart` when the peer is configured with the passive bit, or from
OpenSent when the TCP connection fails. With DelayOpen enabled it also holds an
accepted socket without sending OPEN. Peer OPEN goes directly to OpenConfirm;
DelayOpen expiry sends OPEN and enters OpenSent. Stop or error returns to Idle.

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

A passive peer does not dial. `Session.Accept(conn)` calls
`connectionEstablished`, which fires Event 17. With `timer delay-open 0` it
enters OpenSent immediately. A positive delay starts DelayOpenTimer and holds
the socket in Active, with ConnectRetryTimer stopped and no OPEN written.

<!-- source: internal/component/bgp/reactor/session_connection.go — Accept -->
<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished -->

## Events handled in Active

| Event | Produced by | FSM reaction | Wire side effect | Next state |
|-------|-------------|--------------|------------------|------------|
| `EventManualStart` / `EventAutomaticStart` / `EventAutomaticStartWithDampPeerOscillations` | duplicate `Session.Start()` / `startAutomatic()` / `startDamped()` call | ignored (RFC 4271) | none | `Active` |
| `EventManualStop` | `Session.Stop` / `Session.Teardown` | cleanup in caller; **sets ConnectRetryCounter to zero** | Cease NOTIFICATION from `Session.Teardown` when a conn exists; `Session.Stop` sends nothing | `Idle` |
| `EventAutomaticStop` / `EventOpenCollisionDump` | `Session.teardownAutomatic` / collision retirement | clear timers; **increments ConnectRetryCounter** | Cease for automatic teardown; **no NOTIFICATION** for an OPEN-less collision loser | `Idle` |
| `EventTCPConnectionConfirmed` | `Session.connectionEstablished` after `Accept` | stop ConnectRetryTimer; start DelayOpenTimer when enabled | no OPEN while delayed; otherwise immediate OPEN | `Active` when delayed, otherwise `OpenSent` |
| `EventDelayOpenTimerExpires` | `Session.delayOpenExpired` | clear retry and delay timers; start OPEN-wait HoldTimer | OPEN | `OpenSent` |
| `EventBGPOpenWithDelayOpenTimer` | either OPEN receive rail via `advanceAfterOpen` | clear retry and delay timers; arm negotiated hold/keepalive timers, or stop both for zero hold | OPEN then KEEPALIVE | `OpenConfirm` |
| `EventTCPConnectionFails` | held-socket read failure while DelayOpen runs | restart ConnectRetryTimer, clear DelayOpenTimer, release connection resources, **increment ConnectRetryCounter** | none | `Idle` |
| `EventConnectRetryTimerExpires` | not generated in production | passive check: if not passive, go to Connect | none | `Connect` or `Active` |
| `EventNotifMsgVerErr` | received version-error NOTIFICATION | clear timers; **no increment** with DelayOpen running, increment otherwise | none | `Idle` |
| `EventBGPHeaderErr` / `EventBGPOpenMsgErr` / `EventNotifMsg` | message decode paths | clear timers; **increment ConnectRetryCounter** | NOTIFICATION where applicable | `Idle` |
| any other event | unexpected | clear timers; **increment ConnectRetryCounter** | none for Event 23 on an OPEN-less connection | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleActive -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Accept, acceptWithOpen -->

## Timers running in this state

| Timer | Status | Notes |
|-------|--------|-------|
| ConnectRetryTimer | stopped on TCP success; restarted by Event 18 | The Peer owns reconnect scheduling; old-session cleanup stops the timer after the transition. |
| DelayOpenTimer | running while the accepted socket has sent no OPEN | `timer delay-open` is 0..240 seconds; zero disables it. |
| HoldTimer | not running during DelayOpen | Event 12 starts the OPEN-wait bound; Event 20 starts negotiated hold. |
| KeepaliveTimer | not running during DelayOpen | Event 20 starts it unless negotiated hold is zero. |

<!-- source: internal/component/bgp/fsm/timer.go — Timers -->

## Wire side effects

- **On `EventTCPConnectionConfirmed`:** after `Accept` wires the socket
  into the session, `connectionEstablished` tunes TCP (nodelay, TOS,
  buffer sizes), snapshots local addresses for NEXT_HOP validation, and
  publishes the socket and its observed buffered writer together. Only then
  does it fire the FSM event. With DelayOpen disabled it sends OPEN and starts
  the OPEN-wait hold timer (`ze.bgp.openwait`, default 120 seconds). With
  DelayOpen enabled it retains the local proposal until peer OPEN or expiry.
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

1. Calls `connectionEstablished`, which refuses a sealed Session and otherwise
   publishes the socket and fires Event 17. DelayOpen holds the local OPEN.
2. Observes the original received OPEN bytes on the winning Session.
3. Calls `processOpen(peerOpen)` to validate and negotiate that OPEN, then
   `advanceAfterOpen` for Event 20: local OPEN, OpenConfirm, and KEEPALIVE.

For `connection both`, the start left the FSM in Connect, so both events run
there. For `connection passive`, both run in Active. Neither delayed path visits
OpenSent. With delay zero the old OpenSent sequence remains.

The base DelayOpen path here covers RFC 4271 without negotiated BFD strict mode.
The strict-mode DelayOpen sub-states are a separate implementation phase, not
claimed by this path.

<!-- source: internal/component/bgp/reactor/session_connection.go — acceptWithOpen -->
<!-- source: internal/component/bgp/reactor/session_connection.go — processOpen -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — resolvePendingCollision -->
<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce -->

The `plugin.PeerInfo` that `runOnce` builds for the session's wire observers carries the peer's AS as `rpc.RemoteAS`: a dynamic peer's settings hold 0 until its OPEN is negotiated, and `rpc.RemoteASOf` turns that 0 into the not-yet-known state, so an observer of the OPEN exchange sees `remote as unknown` (text) or no `remote.as` (JSON), never AS 0. See [peer lifecycle](peer-lifecycle.md).

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
- `internal/component/bgp/reactor/delay_open_test.go` proves Active Event 18
  through a real Peer, with delay zero as the opposite path.
  <!-- source: internal/component/bgp/reactor/delay_open_test.go — TestRFC4271ActiveEvent18WithDelayOpen -->
