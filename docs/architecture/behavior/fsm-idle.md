# BGP FSM: Idle State

## TL;DR

Initial state. No TCP connection, no timers, no resources committed to the
peer. BGP refuses all incoming connections for this peer until a
`ManualStart` event moves it out.

RFC 4271 Section 8.2.2 "Idle state".

<!-- source: internal/component/bgp/fsm/state.go — StateIdle -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleIdle -->

## Entry

Idle is reached one of four ways:

1. **Initial construction.** `fsm.New()` returns a fresh FSM already in
   `StateIdle`.
   <!-- source: internal/component/bgp/fsm/fsm.go — New -->

2. **Transition from any other state on `ManualStop`.** `Session.Close()`,
   `Session.CloseWithNotification()`, and `Session.Teardown()` all fire
   `EventManualStop` via `logFSMEvent`, which resolves to Idle in every
   non-Idle state.
   <!-- source: internal/component/bgp/reactor/session_connection.go — Close, CloseWithNotification, Teardown -->

3. **Transition from Connect/Active/OpenSent/OpenConfirm/Established on
   error events.** TCP connection failure, hold-timer expiry, notification
   received, header or OPEN message error, or an unexpected event. Every
   per-state handler ultimately routes these to Idle.
   <!-- source: internal/component/bgp/fsm/fsm.go — handleConnect, handleActive, handleOpenSent, handleOpenConfirm, handleEstablished -->

4. **Transition from Connect on `ManualStop`.** Stops the session cleanly
   without sending a NOTIFICATION.
   <!-- source: internal/component/bgp/fsm/fsm.go — handleConnect case EventManualStop -->

When Idle is re-entered from another state, the session layer performs
cleanup before or after the FSM event: stopping all timers via
`Timers.StopAll()`, sending a Cease NOTIFICATION where applicable, and
closing the TCP connection via `closeConn()`. The FSM itself knows nothing
about any of this. It only observes the state change.

<!-- source: internal/component/bgp/fsm/timer.go — StopAll -->
<!-- source: internal/component/bgp/reactor/session_connection.go — closeConn -->

## Events handled in Idle

| Event | Produced by | FSM reaction | Wire side effect | Next state |
|-------|-------------|--------------|------------------|------------|
| `EventManualStart` | `Session.Start()` | passive flag decides next state | none | `Active` if `passive == true`, else `Connect` |
| `EventManualStop` | n/a | ignored (per RFC 4271 8.2.2) | none | `Idle` |
| any other event | n/a | ignored (per RFC 4271 8.2.2) | none | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleIdle -->
<!-- source: internal/component/bgp/reactor/session.go — Start -->

The passive flag is set at session construction time from
`settings.Connection.IsActive()` and remembered on the FSM via
`SetPassive`. A peer configured with the passive bit only listens and
transitions Idle to Active on start; a peer with the active bit dials and
transitions Idle to Connect.

<!-- source: internal/component/bgp/reactor/session.go — newSession SetPassive -->
<!-- source: internal/component/bgp/fsm/fsm.go — SetPassive, IsPassive -->

## Timers running in this state

| Timer | Status | Managed by |
|-------|--------|------------|
| HoldTimer | not running | started later on entry to OpenSent |
| KeepaliveTimer | not running | started later on entry to Established (via OpenConfirm KEEPALIVE path) |
| ConnectRetryTimer | not running | declared but never generated in production (see architectural note below) |
| SendHoldTimer (RFC 9687) | not running | started later on entry to Established path |

<!-- source: internal/component/bgp/fsm/timer.go — Timers -->

## Wire side effects

None. Idle holds no TCP connection. Any NOTIFICATION that the system sends
while transitioning *into* Idle is written by the caller of the FSM event
(typically `Session.Close`, `Session.Teardown`, or the hold-timer callback)
*before* the FSM is told to move. The FSM does not own wire I/O.

<!-- source: internal/component/bgp/reactor/session_connection.go — Close, Teardown -->
<!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires callback -->

## Code map

Every file that participates when the system is in Idle:

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleIdle` |
| Initial state constant | `internal/component/bgp/fsm/state.go` | `StateIdle` |
| Start/stop entrypoints | `internal/component/bgp/reactor/session.go` | `Start`, `Stop` |
| Teardown + NOTIFICATION on exit toward Idle | `internal/component/bgp/reactor/session_connection.go` | `Close`, `CloseWithNotification`, `Teardown` |
| Outer peer run loop that re-enters Idle via session lifecycle | `internal/component/bgp/reactor/peer_run.go` | run loop body |

<!-- source: internal/component/bgp/fsm/fsm.go — handleIdle -->
<!-- source: internal/component/bgp/fsm/state.go — StateIdle -->
<!-- source: internal/component/bgp/reactor/session.go — Start, Stop -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Close, CloseWithNotification, Teardown -->
<!-- source: internal/component/bgp/reactor/peer_run.go — run loop around session.Start/Connect/Accept -->

## RFC deviations

None specific to Idle.

## Architectural notes

- **`EventConnectRetryTimerExpires` is defined but never generated in
  production.** Ze uses a blocking `DialContext` for outgoing TCP
  connections plus peer-level exponential backoff in `peer.go`, not the
  RFC's fixed `ConnectRetryTimer`. The FSM handler for Event 9 exists and
  is tested but is architecturally unreachable during normal operation.
  Documented in the header of `internal/component/bgp/fsm/fsm.go`.
  <!-- source: internal/component/bgp/fsm/fsm.go — ARCHITECTURAL NOTES -->

## Tests exercising this state

- `internal/component/bgp/fsm/fsm_test.go` — direct FSM state tests,
  including `ManualStart` from Idle with and without the passive flag.
  <!-- source: internal/component/bgp/fsm/fsm_test.go -->
- `internal/component/bgp/reactor/session_test.go` — end-to-end session
  lifecycle that enters and re-enters Idle on teardown, error paths, and
  reconnection.
  <!-- source: internal/component/bgp/reactor/session_test.go -->
