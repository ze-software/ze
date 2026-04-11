# BGP FSM: OpenConfirm State

## TL;DR

The peer's OPEN has been validated and our KEEPALIVE has been sent. The
system is waiting for the peer's KEEPALIVE to confirm the session. Hold
timer is running with the negotiated value. Exits to Established on a
received KEEPALIVE, or back to Idle on hold-timer expiry, TCP failure,
NOTIFICATION, or stop request.

RFC 4271 Section 8.2.2 "OpenConfirm state".

<!-- source: internal/component/bgp/fsm/state.go — StateOpenConfirm -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenConfirm -->

## Entry

Entered from OpenSent on `EventBGPOpen`, fired in `handleOpen` after full
OPEN validation (version, hold time, optional role check, capability
negotiation, required families, required/refused capabilities).

<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenSent case EventBGPOpen -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleOpen fsm.Event(EventBGPOpen) -->

Immediately after the state change, still inside `handleOpen`, the
session layer:

1. Calls `sendKeepalive(conn)` to write our KEEPALIVE to the peer.
2. Calls `timers.ResetHoldTimer()` to restart the hold timer with the
   negotiated value instead of the pre-negotiation "large value".

<!-- source: internal/component/bgp/reactor/session_handlers.go — handleOpen sendKeepalive + ResetHoldTimer -->
<!-- source: internal/component/bgp/fsm/timer.go — ResetHoldTimer -->

The alternate path via `AcceptWithOpen` + `processOpen` performs the
same sequence without going through the outer message read loop. It is
used when inbound collision resolution has already parsed the peer's
OPEN from a competing socket.

<!-- source: internal/component/bgp/reactor/session_connection.go — processOpen fsm.Event(EventBGPOpen) + sendKeepalive + ResetHoldTimer -->

## Events handled in OpenConfirm

| Event | Produced by | FSM reaction | Wire side effect | Next state |
|-------|-------------|--------------|------------------|------------|
| `EventManualStop` | `Session.Close/Teardown/CloseWithNotification` | cleanup in caller | Cease NOTIFICATION in caller | `Idle` |
| `EventKeepaliveMsg` | `handleKeepalive` on received KEEPALIVE | log transition | nothing additional from the FSM | `Established` |
| `EventHoldTimerExpires` | hold-timer callback in `Session.newSession` | cleanup in caller | NOTIFICATION in caller | `Idle` |
| `EventNotifMsg` / `EventNotifMsgVerErr` | `handleNotification` | cleanup in caller | none | `Idle` |
| `EventBGPHeaderErr` / `EventBGPOpenMsgErr` | `session_read.readAndProcessMessage` / `handleOpen` | log transition | NOTIFICATION in caller | `Idle` |
| `EventTCPConnectionFails` | `handleConnectionClose` on EOF / reset | cleanup in caller | none | `Idle` |
| `EventKeepaliveTimerExpires` | keepalive-timer callback in `Session.newSession` | stay (FSM no-op) | KEEPALIVE sent from callback | `OpenConfirm` |
| any other event | unexpected | log transition | none | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenConfirm -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive fires EventKeepaliveMsg -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleNotification fires EventNotifMsg -->
<!-- source: internal/component/bgp/reactor/session.go — OnKeepaliveTimerExpires callback fires EventKeepaliveTimerExpires -->

### Subtle interaction: keepalive timer start happens in `handleKeepalive`

Note the slightly surprising control flow. The keepalive timer is NOT
started on entry to OpenConfirm by `handleOpen`. It is started when the
**peer's** KEEPALIVE arrives, inside `handleKeepalive`, and only if the
FSM is currently in `StateOpenConfirm` at that moment. This is because
ze's `handleKeepalive` is used for both OpenConfirm -> Established
(where the timer must be started) and Established (where it is already
running).

Specifically, `handleKeepalive`:

1. If the current FSM state is `StateOpenConfirm`, starts the keepalive
   timer and starts the RFC 9687 send-hold timer.
2. Fires `fsm.Event(EventKeepaliveMsg)`. The FSM handler
   (`handleOpenConfirm` case `EventKeepaliveMsg`) calls
   `timers.ResetHoldTimer()` per RFC 4271 §8.2.2 Event 26, then
   transitions the FSM to Established.

Because the `state == StateOpenConfirm` check is evaluated **before**
the event fires, the keepalive and send-hold timers are started while
the FSM still reports OpenConfirm. A reader expecting "timers for
Established are started in handleEstablished" will not find them; they
are started here. The HoldTimer restart lives in the FSM handler, not
here, because §8.2.2 attaches it to the event rather than to a state
transition.

<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenConfirm case EventKeepaliveMsg calls f.timers.ResetHoldTimer -->

## Timers running in this state

| Timer | Status | Managed by |
|-------|--------|------------|
| HoldTimer | **running** (negotiated value, restart on `EventKeepaliveMsg` inside the FSM) | started/reset in OpenSent exit; restart driven by RFC §8.2.2 Event 26 in `handleOpenConfirm` |
| KeepaliveTimer | **starts during OpenConfirm** via `handleKeepalive` just before the transition to Established | `StartKeepaliveTimer` |
| SendHoldTimer (RFC 9687) | **starts during OpenConfirm** via `handleKeepalive` | `startSendHoldTimer` |
| ConnectRetryTimer | not running | not used in production |

<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive StartKeepaliveTimer + startSendHoldTimer -->
<!-- source: internal/component/bgp/fsm/timer.go — StartKeepaliveTimer, ResetHoldTimer -->

## Wire side effects

- **On entry (from `handleOpen` or `processOpen`):** KEEPALIVE is sent via
  `sendKeepalive(conn)`. This is our response to the peer's OPEN.
- **On `EventKeepaliveMsg` exit to Established:** no additional wire
  output from the FSM layer itself. Downstream, the outer peer run loop
  reacts to the state change (see Established runbook).
- **On `EventHoldTimerExpires`:** the hold-timer callback in
  `newSession` calls `logFSMEvent(EventHoldTimerExpires)` and signals
  `errChan`. The session Run loop observes the error and tears down;
  the teardown path sends a NOTIFICATION (HoldTimerExpired).
- **On `EventNotifMsg`:** `handleNotification` calls
  `timers.StopAll()`, fires the FSM event, and closes the connection.
  No NOTIFICATION is sent in response to a received NOTIFICATION.
- **On `EventKeepaliveTimerExpires`:** the callback in `newSession`
  fires the FSM event first, then calls `sendKeepalive(conn)`.

<!-- source: internal/component/bgp/reactor/session_handlers.go — sendKeepalive, handleKeepalive, handleNotification -->
<!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires, OnKeepaliveTimerExpires callbacks -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleOpenConfirm` |
| Entry wiring (KEEPALIVE + hold reset) | `internal/component/bgp/reactor/session_handlers.go` | `handleOpen` tail |
| Alternate entry (`AcceptWithOpen`) | `internal/component/bgp/reactor/session_connection.go` | `processOpen` |
| KEEPALIVE reception + timer start + exit | `internal/component/bgp/reactor/session_handlers.go` | `handleKeepalive` |
| NOTIFICATION handling | `internal/component/bgp/reactor/session_handlers.go` | `handleNotification` |
| Hold/keepalive timer callbacks | `internal/component/bgp/reactor/session.go` | `newSession` |
| Timer primitives | `internal/component/bgp/fsm/timer.go` | `ResetHoldTimer`, `StartKeepaliveTimer` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenConfirm -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleOpen, handleKeepalive, handleNotification -->
<!-- source: internal/component/bgp/reactor/session_connection.go — processOpen -->
<!-- source: internal/component/bgp/reactor/session.go — newSession timer callback wiring -->
<!-- source: internal/component/bgp/fsm/timer.go — ResetHoldTimer, StartKeepaliveTimer -->

## RFC deviations

- **`EventKeepaliveTimerExpires` is a no-op in the FSM.** RFC 4271 says
  to send a KEEPALIVE and stay in OpenConfirm. Ze fires the event from
  the callback, but the actual send happens in the callback body in
  `session.go`, not in the FSM handler. The handler simply observes that
  the event was received and stays in OpenConfirm.
  <!-- source: internal/component/bgp/reactor/session.go — OnKeepaliveTimerExpires callback body -->

## Architectural notes

- **Timer setup spans OpenConfirm even though the state is short.**
  `handleKeepalive` is where the keepalive and RFC 9687 send-hold timers
  come online. By the time Established is reached, those timers are
  already running.
- **`handleNotification` always stops all timers before firing the FSM
  event.** This is consistent across OpenConfirm, Established, and any
  other state where a NOTIFICATION might arrive.
  <!-- source: internal/component/bgp/reactor/session_handlers.go — handleNotification -->

## Tests exercising this state

- `internal/component/bgp/fsm/fsm_test.go` — direct state transition
  tests for OpenConfirm, including KEEPALIVE receive and error arms.
  <!-- source: internal/component/bgp/fsm/fsm_test.go -->
- `internal/component/bgp/reactor/session_handlers_test.go` — tests for
  `handleKeepalive` covering the OpenConfirm -> Established handoff
  and the keepalive timer start side effect.
  <!-- source: internal/component/bgp/reactor/session_handlers_test.go -->
- `internal/component/bgp/reactor/session_test.go` — full session
  handshake through OpenConfirm.
  <!-- source: internal/component/bgp/reactor/session_test.go -->
