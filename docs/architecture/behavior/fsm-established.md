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

One thing delays that entry. A peer running BFD strict mode with a
`hold-down` interval stays in OpenConfirm until the BFD session has been Up
for the whole interval (draft-ietf-idr-bgp-bfd-strict-mode Section 10):
`handleKeepalive` arms the timer, resets the HoldTimer, and returns without
firing the event, and the timer's expiry re-enters the transition. The
second rail into Established is the strict-mode release itself, from the
`OpenSentConfirmedBfdUpPending` sub-state, where the peer's KEEPALIVE
already arrived and the BFD session comes Up afterwards.

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
| `EventManualStart` / `EventAutomaticStartWithDampPeerOscillations` | administrative start if delivered after startup; normal startup uses a fresh Idle FSM | ignored; ConnectRetryCounter untouched | none | `Established` |
| `EventManualStop` | `Session.Stop` / `Session.Teardown` | cleanup in caller; **sets ConnectRetryCounter to zero** | Cease NOTIFICATION from `Session.Teardown` when a conn exists; `Session.Stop` sends nothing | `Idle` |
| `EventAutomaticStop` / `EventOpenCollisionDump` | `Session.teardownAutomatic` (BFD down, out of resources) / `Session.CloseWithNotification` (collision) | cleanup in caller; **increments ConnectRetryCounter** | Cease NOTIFICATION in caller | `Idle` |
| `EventKeepaliveMsg` | `handleKeepalive` | stay (hold timer reset in caller) | none | `Established` |
| `EventKeepaliveTimerExpires` | `OnKeepaliveTimerExpires` callback | stay | KEEPALIVE sent from callback body | `Established` |
| `EventUpdateMsg` | `processMessage` after validation, prefix-limit checks and callback delivery | stay; reset a nonzero HoldTimer | UPDATE forwarded to plugins and peers in caller | `Established` |
| `EventHoldTimerExpires` | hold-timer callback | cleanup in caller; **increments ConnectRetryCounter** | NOTIFICATION (HoldTimerExpired) from the callback | `Idle` |
| `EventNotifMsg` / `EventNotifMsgVerErr` | `handleNotification` | cleanup in caller; **increments ConnectRetryCounter** (Established is the one state whose Event 24 clause carries the counter line) | none | `Idle` |
| `EventUpdateMsgErr` | `processMessage` / RFC 7606 session-reset path | cleanup in caller; **increments ConnectRetryCounter** | NOTIFICATION (Update error) in caller | `Idle` |
| `EventBGPHeaderErr` | `readAndProcessMessage` / `handleUnknownType` | cleanup in caller; **increments ConnectRetryCounter** | Message Header Error NOTIFICATION in caller (`notifyHeaderErr`: 1/1 for a bad Marker, 1/2 for a bad Length, 1/3 for an unknown Type), the Section 6.1 code rather than the Finite State Machine Error the "any other event" list names (rfc/corrections/rfc4271.md, `RFC4271-8.2.2-28`) | `Idle` |
| `EventTCPConnectionFails` | `handleConnectionClose` | cleanup in caller; **increments ConnectRetryCounter** | none | `Idle` |
| any other event (RFC 4271 lists 9, 12-13, 20-22) | a second OPEN (`EventBGPOpen`, from `handleOpen`); Ze has no Event 9, 12, 13 or 20, and Events 21 and 22 follow Section 6 (header error above, OPEN refused with Cease before parsing) | `ErrFSMError`; **increments ConnectRetryCounter** | Cease NOTIFICATION from `handleOpen`; no Finite State Machine Error NOTIFICATION is sent in this state | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleEstablished -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive, handleNotification, handleUnknownType -->
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage, processMessage, handleConnectionClose -->

The reader observes complete original packets before semantic processing.
Wire observers therefore see the received UPDATE, not a synthesized withdrawal
or rewritten attribute set produced later by `processMessage`.
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage -->

<!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires, OnKeepaliveTimerExpires callbacks -->

### Accepted UPDATEs restart the HoldTimer via the FSM

`processMessage` validates address families before callback delivery and then
fires `fsm.Event(EventUpdateMsg)`. The callback may transfer the received
UPDATE's storage to a consumer that recycles it before returning. No UPDATE
payload is read after that handoff. A policy teardown requested by the callback
takes precedence over the accepted-message event.

`processMessage` runs those steps in this order, and the order is a
contract rather than an accident:

1. Build the `WireUpdate` over the received body at the session's receive
   context.
2. `enforceRFC7606`, which judges what the PEER sent and can rewrite the
   payload or synthesize withdrawals.
3. `collapseASPathFamily`, which reconciles the AS-path family to
   four-octet truth once (RFC 6793 Sections 4.1 and 4.2.3) and relabels
   the payload with a four-octet context. It runs AFTER step 2 so RFC
   7606 never judges ze's own rewrite, and BEFORE every consumer that
   reads an AS path: the ingress filters, the forward cache, the RIB and
   both forward rails.
4. `validateUpdateFamilies`, then `checkPrefixLimits`.
5. `onMessageReceived`, which is handed the socket's own body beside the
   reconciled `WireUpdate`, so an MRT archive and a pcap record the wire
   rather than the reconciliation.

A step 3 that cannot produce a canonical AS path DROPS the UPDATE and
keeps the session: RFC 7606 has already found the attributes acceptable,
so a NOTIFICATION would be ze's verdict rather than the RFC's, and a
half-rewritten AS path must reach no consumer. The one remaining responsibility the RFC assigns to
`EventUpdateMsg` is "restart the HoldTimer, if the negotiated HoldTime
value is non-zero" — and that now happens inside the FSM handler for
`EventUpdateMsg`, via the `*Timers` reference wired at
`reactor.NewSession`. This gives the FSM event a real job and keeps the
liveness rule in one place: the FSM package.

The per-event cost is a locked switch, a function-call dispatch, and a
brief `Timers.mu` acquisition for the timer reset. Measured cost is in
the tens of nanoseconds per UPDATE.

<!-- source: internal/component/bgp/reactor/session_read.go — processMessage RFC 7606 and prefix-limit paths -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleEstablished case EventUpdateMsg calls f.timers.ResetHoldTimer -->
<!-- source: internal/component/bgp/reactor/session.go — fsm.SetTimers wiring -->

### Prefix-limit route identity

`checkPrefixLimits` runs before publication. The default `offered` mode counts
announcement and withdrawal events using the family's registered framing.
The `installed` mode counts a set of logical routes using the registered
semantic key, not the complete NLRI bytes. MPLS labels and withdrawal
Compatibility octets do not identify a route; VPN RDs and negotiated ADD-PATH
identifiers do. Path Identifier zero is a path, not absence of ADD-PATH.
The inventory boundary checks that an ADD-PATH entry contains all four Path
Identifier octets even after a registered splitter visits it. A shorter entry
creates neither an installed key nor a rollback record.

Withdrawals use `nlrisplit.GetWithdraw`, so a Compatibility field whose S bit is
clear is still exactly three octets (RFC 8277 Section 2.4). Installed keys come
from `nlrisplit.GetPrefixKey`. The per-session scratch is reused between
entries; the rollback journal retains the original message slices and action
context, not those temporary keys. If any family refuses the UPDATE, rollback
restores every installed set before publication is skipped.

<!-- source: internal/component/bgp/reactor/session_prefix.go -- forEachPrefixEntry, prefixSetWalk.identity, rollbackPrefixSets -->

### `handleKeepalive` in Established

When a KEEPALIVE arrives in Established, `handleKeepalive` just fires
`EventKeepaliveMsg`. The FSM handler for that event performs the
RFC 4271 §8.2.2 Event 26 HoldTimer restart. The keepalive timer and
send-hold timer are **not** started here because they were already
started in the OpenConfirm -> Established transition (see OpenConfirm
runbook).

<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive state check -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleEstablished case EventKeepaliveMsg calls f.timers.ResetHoldTimer -->

## Timers running in this state

| Timer | Status | Reset/fired by |
|-------|--------|-----------------|
| HoldTimer | **running** (negotiated value) | reset inside the FSM when `EventKeepaliveMsg` or `EventUpdateMsg` fires (RFC 4271 §8.2.2 Events 26, 27); fires `EventHoldTimerExpires` on expiry |
| KeepaliveTimer | **running** (configured interval or hold/3, jittered) | each arm samples uniformly from 0.75 to 1.0 of the base, with a one-second minimum; callback sends KEEPALIVE and fires `EventKeepaliveTimerExpires` |
| SendHoldTimer (RFC 9687) | **running** unless the negotiated hold time is zero | reset after a BGP message is written and successfully flushed to the peer; fires teardown if no message is sent within SendHoldTime |
| ConnectRetryTimer | not running | not used in production |

<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer, StartKeepaliveTimer, ResetHoldTimer -->
<!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires, OnKeepaliveTimerExpires -->
<!-- source: internal/component/bgp/reactor/session_write.go — startSendHoldTimer, stopSendHoldTimer, flushWrites -->

A successful writer call is not necessarily a sent message. Export-policy
suppression, duplicate or ownership filtering, PATHS-LIMIT withholding, and
stale forward batches can return without emitting a frame. Those attempts,
including an empty flush, leave the SendHold deadline unchanged. A successful
flush credits newly written messages even when the buffer already wrote them
directly to the transport. KEEPALIVE and ROUTE-REFRESH messages, including BoRR
and EoRR, restart the deadline just as an emitted UPDATE does. A failed write or
flush still retires the connection; it earns no restart.

<!-- source: internal/component/bgp/reactor/session_write.go -- writeUpdateGated, writeUpdateBody, flushWrites, retireWrite -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdBatchHandler -->

### Hold timer expiry

Same behavior as described in the OpenSent runbook: a hold timer expiry
always stops the session, per RFC 4271 Section 8.2.2, Event 10. Ze
grants no reprieve to a CPU-congested daemon.

<!-- source: internal/component/bgp/reactor/session.go -- OnHoldTimerExpires callback -->

## Wire side effects

- **On receive UPDATE:** RFC 7606 validation, AS-path reconciliation,
  NEXT_HOP semantic checks and prefix limits run before plugin delivery.
  The callback receives accepted announcements or synthesized withdrawals;
  it never receives malformed announcements as usable routes.
  Processing and peer forwarding happen in `processMessage`.
  <!-- source: internal/component/bgp/reactor/session_read.go — processMessage -->
- **On receive KEEPALIVE:** hold timer reset, FSM no-op. No wire output.
- **On receive ROUTE-REFRESH:** screened by `screenRouteRefresh` before
  any consumer sees it, then handled in `handleRouteRefresh`, gated
  by capability negotiation per RFC 2918 / RFC 7313. A request for an
  <AFI, SAFI> outside the negotiated families, which includes every family
  this speaker did not advertise, is ignored (RFC 2918 Section 4): it
  reaches no plugin and draws no NOTIFICATION. The Message Subtype
  octet is read only when the peer sent Enhanced Route Refresh (RFC 7313,
  capability 70). A Message Subtype other than 0, 1 or 2 is then ignored
  at any body length (RFC 7313 Section 5): it reaches no plugin, draws no
  NOTIFICATION, and logs an error. Without capability 70 the octet is
  RFC 2918's Reserved field, and the receiver ignores it. No FSM event is
  fired.
  <!-- source: internal/component/bgp/reactor/session_handlers.go — handleRouteRefresh -->
  <!-- source: internal/component/bgp/reactor/session_handlers.go — screenRouteRefresh -->
  <!-- source: internal/component/bgp/reactor/session_handlers.go — routeRefreshSubtypeUnknown -->
- **On receive NOTIFICATION:** `handleNotification` stops all timers,
  fires `EventNotifMsgVerErr` for 2/1 or `EventNotifMsg` for any other
  NOTIFICATION (`notificationEvent`), closes the connection. No response
  NOTIFICATION.
- **On `EventKeepaliveTimerExpires`:** the callback fires the FSM event
  then calls `sendKeepalive(conn)`.
- **On `EventHoldTimerExpires`:** the callback records `ErrHoldTimerExpired`
  as the close reason before sending NOTIFICATION code 4 (Hold Timer Expired,
  subcode 0), fires the FSM event, and signals `errChan`. A failed notification
  write still retires the transport; its delivery error must not replace the
  already-recorded expiry reason. The session Run loop observes that reason
  and exits. An earlier recorded close reason remains authoritative.
  <!-- source: internal/component/bgp/reactor/session.go — OnHoldTimerExpires signals errChan -->
- **On a second TCP connection:** retain it until its OPEN arrives, then
  reject it with Cease / Connection Collision. The pending reader bounds the
  OPEN length before reading its body. The established connection remains
  usable throughout; no replacement Session is installed for a rejected socket.
  <!-- source: internal/component/bgp/reactor/reactor_connection.go -- acceptOrReject, handlePendingCollision -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleEstablished` |
| Message read loop | `internal/component/bgp/reactor/session_read.go` | `readAndProcessMessage`, `processMessage` |
| Control-message handlers | `internal/component/bgp/reactor/session_handlers.go` | `handleKeepalive`, `handleNotification`, `handleRouteRefresh`, `handleUnknownType` |
| UPDATE validation, delivery and FSM event | `internal/component/bgp/reactor/session_read.go` | `processMessage` (calls `enforceRFC7606` before delivery) |
| RFC 6793 AS-path reconciliation at ingest | `internal/component/bgp/reactor/session_read.go` | `collapseASPathFamily` (calls `wireu.CollapseAS4Family`) |
| Prefix-limit enforcement (RFC 4486 / RFC 7607) | `internal/component/bgp/reactor/session_prefix.go` | `checkPrefixLimits` |
| Hold/keepalive/send-hold timer callbacks | `internal/component/bgp/reactor/session.go` | `NewSession` |
| State-change callback into peer run loop | `internal/component/bgp/reactor/peer_run.go` | `fsm.SetCallback` closure |
| Timer primitives | `internal/component/bgp/fsm/timer.go` | `StartHoldTimer`, `ResetHoldTimer`, `StartKeepaliveTimer` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleEstablished -->
<!-- source: internal/component/bgp/reactor/session_read.go — readAndProcessMessage, processMessage -->
<!-- source: internal/component/bgp/reactor/session_handlers.go — handleKeepalive, handleNotification, handleRouteRefresh, handleUnknownType -->
<!-- source: internal/component/bgp/reactor/session_prefix.go — checkPrefixLimits -->
<!-- source: internal/component/bgp/reactor/session.go — NewSession -->
<!-- source: internal/component/bgp/reactor/peer_run.go — SetCallback closure -->
<!-- source: internal/component/bgp/fsm/timer.go — StartHoldTimer, ResetHoldTimer, StartKeepaliveTimer -->

## RFC deviations

- **`EventKeepaliveTimerExpires` in the FSM is a no-op.** The RFC says
  to send a KEEPALIVE and restart the keepalive timer. Ze does both in
  the session's `OnKeepaliveTimerExpires` callback body; the FSM handler
  is documentation.
  <!-- source: internal/component/bgp/reactor/session.go — OnKeepaliveTimerExpires -->

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

- `internal/component/bgp/reactor/rfc4271_established_header_error_peer_test.go`
  — a header with an all-zero Marker or Length 18 read in Established draws
  exactly one NOTIFICATION 1/1 or 1/2, the peer-down, the released session and
  timers, and a counter of 1; a well-formed KEEPALIVE keeps the session.
  <!-- source: internal/component/bgp/reactor/rfc4271_established_header_error_peer_test.go -->
- `TestRFC8654FatalLengthReleasesInstalledRoutes` starts with two routes in real
  Adj-RIB-In and Loc-RIB storage and on recipient TCP. A Length-4097 header
  without local capability 6 must yield exact NOTIFICATION 1/2 Data `1001`,
  EOF, released session/timers/contexts, final storage purge, and both recipient
  withdrawals. The adjacent Length-4096 control waits for changed MED in
  storage and recipient TCP before checking retained resources. These tests do
  not establish temporal ordering between route deletion and withdrawal emission.
  <!-- source: internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go -- TestRFC8654FatalLengthReleasesInstalledRoutes, TestRFC8654ValidLengthRetainsInstalledRoutes -->
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
