# BGP FSM: OpenSent State

## TL;DR

TCP is up. The system has sent its OPEN and is waiting for the peer's
OPEN. The hold timer is running with a large value. Exits to OpenConfirm
on a valid peer OPEN, or back to Idle on hold-timer expiry, TCP failure,
a received error, or a stop request.

RFC 4271 Section 8.2.2 "OpenSent state".

<!-- source: internal/component/bgp/fsm/state.go — StateOpenSent -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenSent -->

## Entry

Entered from Connect or Active on `EventTCPConnectionConfirmed`. The event
is fired by `connectionEstablished(conn)` after the TCP socket is wired
into the session and tuned.

<!-- source: internal/component/bgp/fsm/fsm.go — handleConnect case EventTCPConnectionConfirmed -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleActive case EventTCPConnectionConfirmed -->
<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished -->

On entry, three things happen in fixed order inside
`connectionEstablished`:

1. The FSM event is fired (Connect/Active -> OpenSent).
2. `sendOpen(conn)` writes the OPEN message onto the TCP connection.
3. `timers.StartHoldTimer()` starts the hold timer with the
   pre-negotiation value (`settings.ReceiveHoldTime`, seeded at session
   construction).

<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished FSM event + sendOpen + StartHoldTimer -->
<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer -->

A variant path exists: `AcceptWithOpen` goes through
`connectionEstablished` (Active -> OpenSent) and then immediately calls
`processOpen`, which fires `EventBGPOpen` (OpenSent -> OpenConfirm) in
the same synchronous call. In that case OpenSent is observed for only
as long as it takes the process to cross the two function boundaries.

<!-- source: internal/component/bgp/reactor/session_connection.go — AcceptWithOpen, processOpen -->

## Events handled in OpenSent

| Event | Produced by | FSM reaction | Wire side effect | Next state |
|-------|-------------|--------------|------------------|------------|
| `EventManualStop` | `Session.Close/Teardown/CloseWithNotification` | cleanup in caller | Cease NOTIFICATION in caller | `Idle` |
| `EventBGPOpen` | `handleOpen` after version + hold-time validation + capability negotiation | log transition | KEEPALIVE sent immediately after transition, hold timer reset to negotiated value | `OpenConfirm` |
| `EventHoldTimerExpires` | hold-timer callback in `Session.newSession` | log transition | NOTIFICATION (HoldTimerExpired) in caller | `Idle` |
| `EventBGPHeaderErr` | `session_read.readAndProcessMessage` on header parse / length error | log transition | NOTIFICATION in caller | `Idle` |
| `EventBGPOpenMsgErr` | `handleOpen` on version, hold-time, or capability validation failure | log transition | NOTIFICATION in caller | `Idle` |
| `EventNotifMsgVerErr` / `EventNotifMsg` | `handleNotification` | cleanup in caller | none | `Idle` |
| `EventTCPConnectionFails` | `handleConnectionClose` on EOF / reset | cleanup in caller | none | `Idle` (RFC says `Active`, see deviation) |
| any other event | unexpected | log transition | none | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenSent -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleOpen validations and fsm.Event(EventBGPOpen) -->
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage header / length error paths -->
<!-- source: internal/component/bgp/reactor/session_read.go — handleConnectionClose fires EventTCPConnectionFails -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleNotification fires EventNotifMsg -->
<!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires callback fires EventHoldTimerExpires -->

### What `handleOpen` actually validates before firing `EventBGPOpen`

In order:

1. `message.UnpackOpen` parses the OPEN message body. Parse failure fires
   `EventBGPOpenMsgErr`.
2. `open.Version` must equal 4; otherwise NOTIFICATION
   (OpenMessage/UnsupportedVersion) is sent and `EventBGPOpenMsgErr` is
   fired.
3. `open.ValidateHoldTime()` rejects hold time 1 or 2 seconds per RFC 4271
   Section 6.2. Failure sends the NOTIFICATION embedded in the error and
   fires `EventBGPOpenMsgErr`.
4. If an `openValidator` is configured (e.g. RFC 9234 role check), it
   runs and may reject with a typed
   `interface{ NotifyCodes() (uint8, uint8) }`. Rejection sends
   NOTIFICATION but does **not** fire an FSM event (the caller returns
   the error and the read loop exits, which later trips
   `handleConnectionClose` -> `EventTCPConnectionFails`).
5. Capabilities are parsed and negotiated via `negotiateWith`.
6. `CheckRequired(requiredFamilies)` must pass. Missing required families
   sends NOTIFICATION (UnsupportedCapability) and fires
   `EventBGPOpenMsgErr`.
7. `validateCapabilityModes(...)` enforces `RequiredCapabilities` and
   `RefusedCapabilities`. Failure returns an error; it does not
   explicitly fire an event, so the read loop exits and the session
   tears down.
8. Finally, `fsm.Event(EventBGPOpen)` is fired. On success, `sendKeepalive`
   writes our KEEPALIVE and `timers.ResetHoldTimer` restarts the hold
   timer with the negotiated value.

<!-- source: internal/component/bgp/reactor/session_handlers.go — handleOpen -->
<!-- source: internal/component/bgp/reactor/session.go — negotiateWith, openValidator wiring -->

## Timers running in this state

| Timer | Status | Managed by |
|-------|--------|------------|
| HoldTimer | **running** (large value per RFC, seeded from `settings.ReceiveHoldTime`) | started on entry (`StartHoldTimer`), stopped on any exit (via `StopAll` or `StopHoldTimer`) |
| KeepaliveTimer | not running | started later, in OpenConfirm via the KEEPALIVE receive path |
| ConnectRetryTimer | not running | not used in production in ze |
| SendHoldTimer (RFC 9687) | not running | started later in the OpenConfirm -> Established path |

<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished calls StartHoldTimer -->
<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer, StopHoldTimer, StopAll -->

### Hold timer expiry: congestion extension

Ze extends the RFC's hold-timer behavior with a "recent read" check. If
`recentRead` (an atomic flag set whenever a message header is read from
the TCP connection) is true when the hold timer fires, the session does
**not** fire `EventHoldTimerExpires`. Instead it logs a congestion
warning and resets the hold timer. Only if no data has been read in the
hold window does the timer callback fire the FSM event.

<!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires callback recentRead.Swap -->
<!-- source: internal/component/bgp/reactor/session_read.go — recentRead.Store(true) -->

This is a ze-specific extension inspired by BIRD's technique, not part
of RFC 4271.

## Wire side effects

- **On entry:** OPEN is written to the peer via `sendOpen(conn)`.
- **On exit to OpenConfirm:** our KEEPALIVE is written via
  `sendKeepalive(conn)`. The hold timer is reset to the negotiated
  value.
- **On exit to Idle via validation failure:** NOTIFICATION with the
  appropriate OpenMessage error code is sent via `logNotifyErr`. The
  connection is closed via `closeConn`.
- **On exit to Idle via hold-timer expiry:** the caller is the hold-timer
  callback, which signals `errChan` with `ErrHoldTimerExpired`. The
  session Run loop observes the error and tears down. No NOTIFICATION
  is sent from the callback itself; the teardown path may send one.
- **On exit to Idle via TCP read failure:** no NOTIFICATION (the peer
  is already gone).

<!-- source: internal/component/bgp/reactor/session_connection.go — sendOpen -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — sendKeepalive after EventBGPOpen -->
<!-- source: internal/component/bgp/reactor/session.go — logNotifyErr helper -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleOpenSent` |
| Entry wiring + OPEN send + hold start | `internal/component/bgp/reactor/session_connection.go` | `connectionEstablished`, `sendOpen` |
| OPEN validation + capability negotiation + exit to OpenConfirm | `internal/component/bgp/reactor/session_handlers.go` | `handleOpen` |
| Alternate path with pre-buffered OPEN | `internal/component/bgp/reactor/session_connection.go` | `AcceptWithOpen`, `processOpen` |
| TCP read loop producing message events | `internal/component/bgp/reactor/session_read.go` | `readAndProcessMessage`, `processMessage` |
| Hold timer callback -> `EventHoldTimerExpires` | `internal/component/bgp/reactor/session.go` | `newSession` (wires `OnHoldTimerExpires`) |
| Timer implementation | `internal/component/bgp/fsm/timer.go` | `StartHoldTimer`, `ResetHoldTimer` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenSent -->
<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished, sendOpen, AcceptWithOpen, processOpen -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleOpen -->
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage, processMessage -->
<!-- source: internal/component/bgp/reactor/session.go — newSession OnHoldTimerExpires wiring -->
<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer, ResetHoldTimer -->

## RFC deviations

- **`EventTCPConnectionFails` goes to Idle, not Active.** RFC 4271
  Section 8.2.2 specifies that OpenSent on `TcpConnectionFails` should
  close the BGP connection, restart the `ConnectRetryTimer`, and
  transition to Active. Ze instead transitions directly to Idle. The
  reconnection logic lives in the peer-level run loop with exponential
  backoff, not in an FSM-resident timer. Documented as a deliberate
  simplification in the file header of `fsm.go`.
  <!-- source: internal/component/bgp/fsm/fsm.go — VIOLATIONS section -->
- **Hold timer congestion extension** (see Timers section above) is a
  ze-specific extension not described by RFC 4271.

## Architectural notes

- **NOTIFICATION sending lives outside the FSM.** Every arm of
  `handleOpenSent` only decides "transition to Idle". The actual
  NOTIFICATION is written by the calling path (`handleOpen` via
  `logNotifyErr`, or the teardown helpers in `session_connection.go`)
  *before* the FSM event is fired.
- **Capability negotiation happens in OpenSent, not OpenConfirm.** By the
  time `EventBGPOpen` is fired, `s.negotiated` is already populated. This
  lets the post-event code in `handleOpen` and the OpenConfirm transition
  use the negotiated values immediately.
  <!-- source: internal/component/bgp/reactor/session_handlers.go — negotiateWith before fsm.Event(EventBGPOpen) -->

## Tests exercising this state

- `internal/component/bgp/fsm/fsm_test.go` — direct state transition
  tests for every `handleOpenSent` arm.
  <!-- source: internal/component/bgp/fsm/fsm_test.go -->
- `internal/component/bgp/reactor/session_handlers_test.go` — OPEN
  validation tests covering version, hold-time, capability negotiation,
  and the various NOTIFICATION paths.
  <!-- source: internal/component/bgp/reactor/session_handlers_test.go -->
- `internal/component/bgp/reactor/session_test.go` — end-to-end session
  lifecycle tests driving the full handshake through OpenSent.
  <!-- source: internal/component/bgp/reactor/session_test.go -->
