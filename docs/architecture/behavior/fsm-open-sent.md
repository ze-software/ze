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
3. `timers.StartHoldTimer()` starts the hold timer with `ze.bgp.openwait`
   (default 120 seconds), independently of the configured receive hold time.

<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished FSM event + sendOpen + StartHoldTimer -->
<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer -->

A variant path exists: the peer run loop gives a retained collision winner
to a fresh Session after the losing cycle's cleanup. `acceptWithOpen` goes
through `connectionEstablished`, observes the original received OPEN bytes,
and calls `processOpen`. Validation and negotiation precede `advanceAfterOpen`;
the BFD strict-mode wait described below can keep this path in OpenSent too.

<!-- source: internal/component/bgp/reactor/session_connection.go — acceptWithOpen, processOpen -->
<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce -->

The normal reader also observes the complete original packet before semantic
validation. Both paths preserve wire evidence rather than reconstructing an
OPEN from parsed capabilities.
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage -->

## Events handled in OpenSent

| Event | Produced by | FSM reaction | Wire side effect | Next state |
|-------|-------------|--------------|------------------|------------|
| `EventManualStart` / `EventAutomaticStartWithDampPeerOscillations` | administrative start if delivered after startup; normal startup uses a fresh Idle FSM | ignored; ConnectRetryCounter untouched | none | `OpenSent` |
| `EventManualStop` | `Session.Stop` / `Session.Teardown` | cleanup in caller; **sets ConnectRetryCounter to zero** | Cease NOTIFICATION from `Session.Teardown` when a conn exists; `Session.Stop` sends nothing | `Idle` |
| `EventAutomaticStop` / `EventOpenCollisionDump` | `Session.teardownAutomatic` / `Session.CloseWithNotification` | cleanup in caller; **increments ConnectRetryCounter** | Cease NOTIFICATION in caller | `Idle` |
| `EventBGPOpen` | `handleOpen` after version + hold-time validation + capability negotiation, through `advanceAfterOpen` | log transition | KEEPALIVE sent immediately after transition, hold timer reset to negotiated value. Not fired at all while BFD strict mode holds the session (see below) | `OpenConfirm` |
| `EventHoldTimerExpires` | hold-timer callback in `Session.newSession` | log transition; **increments ConnectRetryCounter** | NOTIFICATION (HoldTimerExpired) in caller | `Idle` |
| `EventBGPHeaderErr` | `session_read.readAndProcessMessage` on header parse / length error | log transition; **increments ConnectRetryCounter** | NOTIFICATION in caller | `Idle` |
| `EventBGPOpenMsgErr` | `handleOpen` on version, hold-time, or capability validation failure | log transition; **increments ConnectRetryCounter** | NOTIFICATION in caller | `Idle` |
| `EventNotifMsgVerErr` | `handleNotification`, through `notificationEvent`, for a received NOTIFICATION 2/1 (OPEN Message Error, Unsupported Version Number) | cleanup in caller; ConnectRetryCounter untouched (RFC 4271 8.2.2 gives Event 24 no counter clause in this state) | none: Event 24 is not on the "any other event" list, so no Finite State Machine Error is sent | `Idle` |
| `EventTCPConnectionFails` | `handleConnectionClose` on EOF / reset | cleanup in caller; ConnectRetryCounter untouched (no counter clause in this state); the peer run loop is the restarted ConnectRetryTimer and the listener keeps accepting | none | `Active` (RFC 4271 Section 8.2.2) |
| any other event (RFC 4271 lists 9, 11-13, 20, 25-28): a received NOTIFICATION that is not a version error (`EventNotifMsg`), a KEEPALIVE outside BFD strict mode (`EventKeepaliveMsg`), an UPDATE well formed or not (`EventUpdateMsg`) | `handleNotification`, `handleKeepalive`, and `processMessage` for an UPDATE before it is parsed or reaches a plugin, each through `Session.fsmMessageEvent` | `ErrFSMError`; **increments ConnectRetryCounter** | `fsmMessageEvent` sends NOTIFICATION 5/0 (Finite State Machine Error, subcode Unspecified: Ze does not originate the RFC 6608 subcodes) and closes the connection | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenSent -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — fsmMessageEvent, updateIsUnexpected -->
<!-- source: internal/component/bgp/reactor/session_read.go — processMessage refuses an UPDATE in OpenSent or OpenConfirm -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleOpen validations and fsm.Event(EventBGPOpen) -->
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage header / length error paths -->
<!-- source: internal/component/bgp/reactor/session_read.go — handleConnectionClose fires EventTCPConnectionFails -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleNotification fires notificationEvent: EventNotifMsgVerErr for 2/1, EventNotifMsg otherwise -->
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
4. `validateOpenPeerAS` (`session_open_as.go`) parses the capabilities and
   judges the advertised AS, the ASN4 capability's value when present.
   AS 0 (RFC 7607 Section 2) and an AS `PeerSettings.peerASAccepted`
   (`session_as_migration.go`) refuses both send NOTIFICATION 2/2 Bad Peer AS,
   fire `EventBGPOpenMsgErr`, close the connection and count the refusal in
   `ze_bgp_open_rejected_bad_peer_as_total`. `peerASAccepted` accepts the
   configured remote AS, or either AS of an RFC 7705 Section 4.2 migration
   pair, and skips the comparison for a dynamic peer whose AS is not known yet.
   `validateOpenIdentifier` then applies RFC 6286 Section 2.2 (Bad BGP
   Identifier).
5. If an `openValidator` is configured (e.g. RFC 9234 role check), it
   runs and may reject with a typed
   `interface{ NotifyCodes() (uint8, uint8) }`. Rejection sends
   NOTIFICATION but does **not** fire an FSM event (the caller returns
   the error and the read loop exits, which later trips
   `handleConnectionClose` -> `EventTCPConnectionFails`).
6. Capabilities are parsed and negotiated via `negotiateWith`, which also fixes
   the session's peer AS and its internal verdict. `sessionPeerAS` (`peer.go`)
   answers the AS, taking the configured value before the one the OPEN
   advertises. `PeerSettings.isIBGPWith` (`session_as_migration.go`) answers the
   verdict. Both go into `capability.Negotiate` as a `PeerIdentity`, so
   `Negotiated.Identity` is what every later reader of the two gets.
7. `CheckRequired(requiredFamilies)` must pass. Missing required families
   sends NOTIFICATION (UnsupportedCapability) and fires
   `EventBGPOpenMsgErr`.
8. `validateCapabilityModes(...)` enforces `RequiredCapabilities` and
   `RefusedCapabilities`. Failure returns an error; it does not
   explicitly fire an event, so the read loop exits and the session
   tears down.
9. Finally `advanceAfterOpen` runs, and it forks. In the ordinary case it
   fires `fsm.Event(EventBGPOpen)`, `sendKeepalive` writes our KEEPALIVE and
   `timers.ResetHoldTimer` restarts the hold timer with the negotiated value.
   Under BFD strict mode it does none of that: see the section below.

### The BFD strict-mode fork

When `bfd { strict true }` is configured AND the peer advertised capability 74
AND the BFD session is neither Up nor AdminDown, draft-ietf-idr-bgp-bfd-strict-mode
Section 8.5.5 withholds the KEEPALIVE. `advanceAfterOpen` then calls
`FSM.EnterBfdUpPending` instead of firing Event 19, so the session STAYS in
OpenSent carrying the `OpenSentBfdUpPending` sub-state, and the BfdHoldTimer is
armed if the negotiated hold time is zero.

Two events leave that sub-state. A BFD Up (or AdminDown, or Disabled) sends the
withheld KEEPALIVE and advances. A KEEPALIVE received from the peer, which is
an FSM error in the unmodified state, moves the sub-state to
`OpenSentConfirmedBfdUpPending` so the next BFD Up goes straight to Established.
Both are `Session.handleBFDEvent` and `FSM.handleOpenSent`, and the whole table
is in `fsm.md`.

<!-- source: internal/component/bgp/reactor/session_bfd_strict.go — advanceAfterOpen, bfdStrictHolds, handleBFDEvent -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleOpen -->
<!-- source: internal/component/bgp/reactor/session_negotiate.go — negotiateWith -->
<!-- source: internal/component/bgp/reactor/peer.go -- sessionPeerAS, openAdvertisedAS -->
<!-- source: internal/component/bgp/reactor/session_as_migration.go -- isIBGPWith -->
<!-- source: internal/component/bgp/reactor/session_open_as.go -- validateOpenPeerAS, rejectOpenPeerAS -->
<!-- source: internal/component/bgp/reactor/session_as_migration.go -- peerASAccepted -->
<!-- source: internal/component/bgp/reactor/session.go — openValidator, setOpenValidator -->

## Timers running in this state

| Timer | Status | Managed by |
|-------|--------|------------|
| HoldTimer | **running** (large value per RFC, seeded from `settings.ReceiveHoldTime`) | started on entry (`StartHoldTimer`), stopped on any exit (via `StopAll` or `StopHoldTimer`) |
| KeepaliveTimer | not running | started later, in OpenConfirm via the KEEPALIVE receive path |
| ConnectRetryTimer | not running | not used in production in ze |
| SendHoldTimer (RFC 9687) | not running | started later in the OpenConfirm -> Established path |

<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished calls StartHoldTimer -->
<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer, StopHoldTimer, StopAll -->

### Hold timer expiry

When the hold timer fires, the callback always fires
`EventHoldTimerExpires`. Ze grants no reprieve, per RFC 4271
Section 8.2.2, Event 10.

<!-- source: internal/component/bgp/reactor/session.go -- OnHoldTimerExpires callback -->

## Wire side effects

- **On entry:** OPEN is written to the peer via `sendOpen(conn)`.
- **On exit to OpenConfirm:** our KEEPALIVE is written via
  `sendKeepalive(conn)`. The hold timer is reset to the negotiated
  value. Under BFD strict mode there is no exit yet and no KEEPALIVE: the
  session stays here until BFD is Up (see the fork above).
- **On exit to Idle via validation failure:** NOTIFICATION with the
  appropriate OpenMessage error code is sent via `logNotifyErr`. The
  connection is closed via `closeConn`.
- **On exit to Idle via hold-timer expiry:** the hold-timer callback
  sends NOTIFICATION code 4 (Hold Timer Expired, subcode 0), then
  signals `errChan` with `ErrHoldTimerExpired`. The session Run loop
  observes the error and starts the teardown.
  <!-- source: internal/component/bgp/reactor/session.go -- OnHoldTimerExpires sendNotificationWithin -->
- **On exit to Idle via TCP read failure:** no NOTIFICATION (the peer
  is already gone).

<!-- source: internal/component/bgp/reactor/session_connection.go — sendOpen -->
<!-- source: internal/component/bgp/reactor/session_bfd_strict.go — advanceAfterOpen -->
<!-- source: internal/component/bgp/reactor/session.go — logNotifyErr helper -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleOpenSent` |
| Entry wiring + OPEN send + hold start | `internal/component/bgp/reactor/session_connection.go` | `connectionEstablished`, `sendOpen` |
| OPEN validation + capability negotiation + exit to OpenConfirm | `internal/component/bgp/reactor/session_handlers.go` | `handleOpen` |
| The BFD strict-mode fork on that exit | `internal/component/bgp/reactor/session_bfd_strict.go` | `advanceAfterOpen` |
| Alternate path with pre-buffered OPEN | `internal/component/bgp/reactor/session_connection.go` | `acceptWithOpen`, `processOpen` |
| TCP read loop producing message events | `internal/component/bgp/reactor/session_read.go` | `readAndProcessMessage`, `processMessage` |
| Hold timer callback -> `EventHoldTimerExpires` | `internal/component/bgp/reactor/session.go` | `newSession` (wires `OnHoldTimerExpires`) |
| Timer implementation | `internal/component/bgp/fsm/timer.go` | `StartHoldTimer`, `ResetHoldTimer` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenSent -->
<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished, sendOpen, acceptWithOpen, processOpen -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleOpen -->
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage, processMessage -->
<!-- source: internal/component/bgp/reactor/session.go — newSession OnHoldTimerExpires wiring -->
<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer, ResetHoldTimer -->

## Architectural notes

- **`EventTCPConnectionFails` goes to Active.** RFC 4271 Section 8.2.2
  says OpenSent on `TcpConnectionFails` closes the BGP connection,
  restarts the `ConnectRetryTimer`, keeps listening, and "changes its
  state to Active". The caller closes the connection, the reconnect delay
  is the peer-level run loop with exponential backoff rather than an
  FSM-resident timer, and the reactor listener keeps accepting.
  <!-- source: internal/component/bgp/fsm/fsm.go — handleOpenSent EventTCPConnectionFails -->

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

- `internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go` -- a
  running Peer in OpenSent that receives a NOTIFICATION other than a
  version error, a KEEPALIVE, an UPDATE or a malformed UPDATE writes exactly
  one NOTIFICATION 5/0, drops the connection and the session, and counts the
  attempt; the same Peer given an OPEN, then a KEEPALIVE, then an UPDATE in
  Established writes no NOTIFICATION.
  <!-- source: internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go -->
- `internal/component/bgp/reactor/rfc4271_version_error_peer_test.go` -- a
  running Peer in OpenSent or OpenConfirm that receives NOTIFICATION 2/1
  writes nothing back, drops the connection and the session, reaches Idle,
  and leaves the ConnectRetryCounter alone; NOTIFICATION 2/2 takes the
  Event 25 path instead.
  <!-- source: internal/component/bgp/reactor/rfc4271_version_error_peer_test.go -->

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
