# BGP FSM: Established State

## TL;DR

The session is up. The system exchanges UPDATE, KEEPALIVE, ROUTE-REFRESH,
and NOTIFICATION messages with the peer. Hold and keepalive timers are
running. This is where the FSM spends almost all of its time during a
healthy peering.

Exits back to Idle on any error, hold-timer expiry, TCP failure,
NOTIFICATION received, UPDATE error, or manual stop.

RFC 4271 Section 8.2.2 "Established state".

<!-- source: internal/component/bgp/fsm/state.go — StateEstablished -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleEstablished -->

## Entry

Entered from OpenConfirm on `EventKeepaliveMsg`, fired inside
`handleKeepalive` after the peer's KEEPALIVE has been read. The state
change callback registered by the peer run loop fires as part of the
transition.

<!-- source: internal/component/bgp/fsm/fsm.go — handleOpenConfirm case EventKeepaliveMsg -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive -->
<!-- source: internal/component/bgp/reactor/peer_run.go — SetCallback block at "if to == fsm.StateEstablished" -->

The state-change callback in `peer_run.go`:

1. Captures negotiated capabilities via `session.Negotiated()` and
   stores them on the peer (`NewNegotiatedCapabilities`).
2. Sets per-peer encoding contexts from the negotiation.
3. Sets the peer state to `PeerStateEstablished`.
4. Records the establish timestamp via `SetEstablishedNow`.
5. Emits the `session established` info log.
6. Resets the API sync counter to the number of plugin bindings with
   `SendUpdate` permission (used to track initial route replay).

<!-- source: internal/component/bgp/reactor/peer_run.go — fsm.SetCallback closure Established branch -->

## Events handled in Established

| Event | Produced by | FSM reaction | Wire side effect | Next state |
|-------|-------------|--------------|------------------|------------|
| `EventManualStop` | `Session.Close/Teardown/CloseWithNotification` | cleanup in caller | Cease NOTIFICATION in caller | `Idle` |
| `EventKeepaliveMsg` | `handleKeepalive` | stay (hold timer reset in caller) | none | `Established` |
| `EventKeepaliveTimerExpires` | `OnKeepaliveTimerExpires` callback | stay | KEEPALIVE sent from callback body | `Established` |
| `EventUpdateMsg` | `handleUpdate` after RFC 7606 + prefix-limit checks | stay (FSM no-op) | UPDATE forwarded to plugins and peers in caller | `Established` |
| `EventHoldTimerExpires` | hold-timer callback (after congestion extension check) | cleanup in caller | NOTIFICATION (HoldTimerExpired) in caller | `Idle` |
| `EventNotifMsg` / `EventNotifMsgVerErr` | `handleNotification` | cleanup in caller | none | `Idle` |
| `EventUpdateMsgErr` | `processMessage` / RFC 7606 session-reset path | cleanup in caller | NOTIFICATION (Update error) in caller | `Idle` |
| `EventBGPHeaderErr` | `readAndProcessMessage` / `handleUnknownType` | cleanup in caller | NOTIFICATION in caller | `Idle` |
| `EventTCPConnectionFails` | `handleConnectionClose` | cleanup in caller | none | `Idle` |
| any other event | unexpected | log transition | none | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleEstablished -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive, handleUpdate, handleNotification, handleUnknownType -->
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage, processMessage, handleConnectionClose -->
<!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires, OnKeepaliveTimerExpires callbacks -->

### `handleUpdate` is a no-op at the FSM level

`handleUpdate` resets the hold timer, validates address families, then
fires `fsm.Event(EventUpdateMsg)` and returns. The `handleEstablished`
arm for `EventUpdateMsg` is just a documentation comment; the FSM does
nothing. All the real work (WireUpdate construction, RFC 7606
enforcement, prefix limits, forwarding to plugins) happens in
`processMessage` **before** the FSM event is fired.

This means the FSM is still hit on every UPDATE even though the state
does not change. The per-event cost is a locked switch plus a function
call dispatch, which is cheap but not free at very high UPDATE rates.

<!-- source: internal/component/bgp/reactor/session_handlers.go — handleUpdate -->
<!-- source: internal/component/bgp/reactor/session_read.go — processMessage RFC 7606 and prefix-limit paths -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleEstablished case EventUpdateMsg -->

### `handleKeepalive` in Established

When a KEEPALIVE arrives in Established, `handleKeepalive` only resets
the hold timer and fires `EventKeepaliveMsg`. The keepalive timer and
send-hold timer are **not** started here because they were already
started in the OpenConfirm -> Established transition (see OpenConfirm
runbook).

<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive state check -->

## Timers running in this state

| Timer | Status | Reset/fired by |
|-------|--------|-----------------|
| HoldTimer | **running** (negotiated value) | reset on any received message (`handleKeepalive`, `handleUpdate`); fires `EventHoldTimerExpires` on expiry (subject to congestion extension) |
| KeepaliveTimer | **running** (hold/3) | periodic refire; callback sends KEEPALIVE and fires `EventKeepaliveTimerExpires` |
| SendHoldTimer (RFC 9687) | **running** | reset on every successful write to the peer; fires teardown if we cannot send for too long |
| ConnectRetryTimer | not running | not used in production |

<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer, StartKeepaliveTimer, ResetHoldTimer -->
<!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires, OnKeepaliveTimerExpires -->
<!-- source: internal/component/bgp/reactor/session_write.go — startSendHoldTimer, stopSendHoldTimer -->

### Hold timer congestion extension

Same behavior as described in the OpenSent runbook: if `recentRead` is
true when the hold timer fires, it is reset instead of teardown. This
lets a CPU-congested daemon survive temporary processing lag without
dropping healthy sessions.

<!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires recentRead.Swap -->

## Wire side effects

- **On receive UPDATE:** forwarded to plugins via the
  `onMessageReceived` callback before validation, then UPDATE-specific
  processing (RFC 7606, prefix limits) runs before the FSM event is
  fired. Plugin delivery and peer forwarding happen in
  `processMessage`.
  <!-- source: internal/component/bgp/reactor/session_read.go — processMessage -->
- **On receive KEEPALIVE:** hold timer reset, FSM no-op. No wire output.
- **On receive ROUTE-REFRESH:** handled in `handleRouteRefresh`, gated
  by capability negotiation per RFC 2918 / RFC 7313. No FSM event is
  fired.
  <!-- source: internal/component/bgp/reactor/session_handlers.go — handleRouteRefresh -->
- **On receive NOTIFICATION:** `handleNotification` stops all timers,
  fires `EventNotifMsg`, closes the connection. No response
  NOTIFICATION.
- **On `EventKeepaliveTimerExpires`:** the callback fires the FSM event
  then calls `sendKeepalive(conn)`.
- **On `EventHoldTimerExpires` (after congestion check):** the callback
  fires the FSM event and signals `errChan` with `ErrHoldTimerExpired`.
  The session Run loop observes and tears down, which sends the
  NOTIFICATION.
  <!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires signals errChan -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleEstablished` |
| Message read loop | `internal/component/bgp/reactor/session_read.go` | `readAndProcessMessage`, `processMessage` |
| Message type handlers | `internal/component/bgp/reactor/session_handlers.go` | `handleUpdate`, `handleKeepalive`, `handleNotification`, `handleRouteRefresh`, `handleUnknownType` |
| RFC 7606 validation | `internal/component/bgp/reactor/session_read.go` | `processMessage` (calls `enforceRFC7606`) |
| Prefix-limit enforcement (RFC 4486 / RFC 7607) | `internal/component/bgp/reactor/session_prefix.go` | `checkPrefixLimits` |
| Hold/keepalive/send-hold timer callbacks | `internal/component/bgp/reactor/session.go` | `newSession` |
| State-change callback into peer run loop | `internal/component/bgp/reactor/peer_run.go` | `fsm.SetCallback` closure |
| Timer primitives | `internal/component/bgp/fsm/timer.go` | `StartHoldTimer`, `ResetHoldTimer`, `StartKeepaliveTimer` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleEstablished -->
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage, processMessage -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleUpdate, handleKeepalive, handleNotification, handleRouteRefresh, handleUnknownType -->
<!-- source: internal/component/bgp/reactor/session_prefix.go — checkPrefixLimits -->
<!-- source: internal/component/bgp/reactor/session.go — newSession -->
<!-- source: internal/component/bgp/reactor/peer_run.go — SetCallback closure -->
<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer, ResetHoldTimer, StartKeepaliveTimer -->

## RFC deviations

- **`EventUpdateMsg` is an FSM no-op but still fires.** RFC 4271 says to
  "process the UPDATE and restart the HoldTimer". Ze does both, just in
  the session layer: UPDATE processing happens in `processMessage` and
  `handleUpdate`, and the hold timer is reset by `handleUpdate` before
  the FSM event is fired. The FSM itself does nothing. This is not a
  behavioral deviation, just a placement of responsibilities.
  <!-- source: internal/component/bgp/reactor/session_handlers.go — handleUpdate ResetHoldTimer then fsm.Event -->
- **`EventKeepaliveTimerExpires` in the FSM is a no-op.** The RFC says
  to send a KEEPALIVE and restart the keepalive timer. Ze does both in
  the session's `OnKeepaliveTimerExpires` callback body; the FSM handler
  is documentation.
  <!-- source: internal/component/bgp/reactor/session.go — OnKeepaliveTimerExpires -->
- **Hold timer congestion extension** is a ze-specific extension.

## Compatibility notes

### Double-KEEPALIVE as end-of-RIB marker (non-RFC)

RFC 4724 Section 2 defines the standard End-of-RIB marker used during
Graceful Restart to signal that a peer has finished sending its initial
routing table:

- **IPv4 unicast:** an UPDATE message with zero withdrawn routes, zero
  path attributes, and zero reachable NLRIs (an "empty UPDATE").
- **Other AFI/SAFI:** an UPDATE containing an MP_UNREACH_NLRI path
  attribute with the corresponding (AFI, SAFI) pair and an empty
  withdrawn-routes field.

Some BGP implementations, especially older ones and implementations that
do not support Graceful Restart at all, **do not send the RFC 4724 EoR
marker**. Instead, they signal the end of the initial table transfer by
sending **two KEEPALIVE messages in close succession**: the regular
periodic KEEPALIVE followed immediately by an extra one, with no UPDATE
between them. The second KEEPALIVE arrives well before the next scheduled
keepalive interval (normally `holdTime/3`), and its proximity to the
previous KEEPALIVE is the heuristic that the peer has finished sending
its RIB.

**Known occurrence:** observed on older Cisco IOS routers that predate
(or do not enable) RFC 4724 Graceful Restart. The behavior is not
announced in any capability; a consumer only learns about it by watching
the KEEPALIVE stream during initial session bring-up.

**This convention is not documented by any RFC.** It is a de facto
interoperability habit. It has no IANA code point, no capability flag,
and no negotiated feature advertising it. Consumers interoperating with
such peers must observe the inter-KEEPALIVE gap themselves and decide
whether to treat a short gap as an end-of-sync hint.

**Ze does not currently implement any special case for this heuristic.**
`handleKeepalive` treats every received KEEPALIVE identically: it resets
the hold timer and fires `EventKeepaliveMsg`. The FSM does not expose
KEEPALIVE arrival times to plugins, and the RIB plugin's end-of-sync
detection uses the RFC 7313 BoRR/EoRR markers (for route refresh) and the
RFC 4724 EoR marker (for graceful restart), not the double-KEEPALIVE
heuristic.

<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive -->
<!-- source: internal/component/bgp/plugins/rib/rib.go — EoRR / BoRR handling -->

A plugin that needs to interoperate with a peer that uses double-KEEPALIVE
as an EoR signal would need to:

1. Subscribe to per-peer KEEPALIVE receive events (not currently exposed
   on the plugin bus; the FSM and session layer do not publish them).
2. Record the timestamp of each received KEEPALIVE.
3. Detect "two KEEPALIVEs with a gap much shorter than the negotiated
   `holdTime/3`, no UPDATE between them" as an inferred end-of-sync
   signal.

None of those three steps exist in ze today. This runbook documents the
quirk so future work that needs GR-style initial-sync detection against
non-compliant peers starts from an accurate picture: the heuristic is
known, it is not implemented, and adding it requires exposing KEEPALIVE
receipt events to plugins.

## Architectural notes

- **This is the hot path.** The FSM is called on every UPDATE and every
  KEEPALIVE. Any additional per-event cost in `handleEstablished`
  compounds with ze's routing throughput.
- **Forwarding happens outside the FSM.** The lazy wire / ContextID
  reuse story (forwarding a received UPDATE's raw bytes to matching
  peers without reparsing) lives in the reactor's forwarding pool, not
  in the FSM. The FSM is only aware that an UPDATE happened.
  <!-- source: internal/component/bgp/reactor/forward_pool.go — fwdPool, peerPool, TryDispatch -->
- **RFC 9234 role enforcement runs once on entry via the OPEN
  validator**, not continuously in Established. Policy enforcement on
  incoming routes is handled by filter plugins, not by the FSM.
  <!-- source: internal/component/bgp/reactor/session_handlers.go — openValidator in handleOpen -->

## Tests exercising this state

- `internal/component/bgp/fsm/fsm_test.go` — direct state transition
  tests for `handleEstablished`, including KEEPALIVE/UPDATE no-ops and
  every error arm.
  <!-- source: internal/component/bgp/fsm/fsm_test.go -->
- `internal/component/bgp/reactor/session_handlers_test.go` — full
  coverage of UPDATE, KEEPALIVE, NOTIFICATION, ROUTE-REFRESH handlers.
  <!-- source: internal/component/bgp/reactor/session_handlers_test.go -->
- `internal/component/bgp/reactor/session_test.go` — end-to-end
  Established sessions under various error conditions.
  <!-- source: internal/component/bgp/reactor/session_test.go -->
- `internal/component/bgp/reactor/peer_test.go` — peer-level lifecycle
  including the state change callback into `peer_run.go`.
  <!-- source: internal/component/bgp/reactor/peer_test.go -->
