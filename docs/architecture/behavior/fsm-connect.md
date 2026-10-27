# BGP FSM: Connect State

## TL;DR

The system is dialing a TCP connection, or holding a connected socket while
DelayOpen runs. With `timer delay-open 0` (the default), TCP success sends OPEN
immediately and enters OpenSent. A non-zero delay keeps Connect until peer OPEN
(Event 20, directly to OpenConfirm) or timer expiry (Event 12, to OpenSent).

RFC 4271 Section 8.2.2 "Connect state".

<!-- source: internal/component/bgp/fsm/state.go — StateConnect -->
<!-- source: internal/component/bgp/fsm/fsm.go — handleConnect -->

## Entry

Entered from `Idle` on `EventManualStart` when the FSM passive flag is
false. Triggered by `Session.Start()` which is itself invoked from the
outer peer run loop once per reconnection cycle.

<!-- source: internal/component/bgp/fsm/fsm.go — handleIdle case EventManualStart -->
<!-- source: internal/component/bgp/reactor/session.go — Start -->
<!-- source: internal/component/bgp/reactor/peer_run.go — run loop calling session.Start then session.Connect -->

The peer run loop calls `Session.Connect(ctx)` through the configured
`network.Dialer`. Dial failure returns to Idle. After TCP success,
`connectionEstablished` fires Event 17 (also used for the outbound Event 16
condition). With DelayOpen enabled, the socket and its reader remain owned by
this Session, the retry timer is stopped, and no OPEN is written yet.

<!-- source: internal/component/bgp/reactor/session_connection.go — Connect -->
<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished -->

## Events handled in Connect

| Event | Produced by | FSM reaction | Wire side effect | Next state |
|-------|-------------|--------------|------------------|------------|
| `EventManualStart` / `EventAutomaticStart` / `EventAutomaticStartWithDampPeerOscillations` | duplicate `Session.Start()` / `startAutomatic()` / `startDamped()` call | ignored (RFC 4271) | none | `Connect` |
| `EventManualStop` | `Session.Stop` / `Session.Teardown` | cleanup in caller; **sets ConnectRetryCounter to zero** | Cease NOTIFICATION from `Session.Teardown` when a conn exists; `Session.Stop` sends nothing | `Idle` |
| `EventAutomaticStop` / `EventOpenCollisionDump` | `Session.teardownAutomatic` / collision retirement | clear timers; **increments ConnectRetryCounter** | no OPEN after canceled dial; an OPEN-less collision loser closes without NOTIFICATION | `Idle` |
| `EventConnectRetryTimerExpires` | not generated in production | clear DelayOpenTimer (Peer owns reconnect scheduling) | none | `Connect` |
| `EventTCPConnectionConfirmed` | `Session.connectionEstablished` | stop ConnectRetryTimer; start DelayOpenTimer when configured | no OPEN while delayed; otherwise immediate OPEN | `Connect` when delayed, otherwise `OpenSent` |
| `EventDelayOpenTimerExpires` | `Session.delayOpenExpired` | clear DelayOpenTimer and ConnectRetryTimer; arm OPEN-wait HoldTimer | OPEN | `OpenSent` |
| `EventBGPOpenWithDelayOpenTimer` | `Session.advanceAfterOpen`, including `acceptWithOpen` | clear DelayOpenTimer and ConnectRetryTimer; arm negotiated hold/keepalive timers (both off for zero hold) | OPEN then KEEPALIVE | `OpenConfirm` |
| `EventTCPConnectionFails` | dial error or held-socket read failure | with DelayOpen running, clear it and restart ConnectRetryTimer; otherwise stop retry | none | `Active` when delayed, otherwise `Idle` |
| `EventNotifMsgVerErr` | received version-error NOTIFICATION | clear timers; no counter increment while DelayOpen runs, increment otherwise | none | `Idle` |
| `EventBGPHeaderErr` / `EventBGPOpenMsgErr` / `EventNotifMsg` | message decode paths | clear timers; **increments ConnectRetryCounter** | error NOTIFICATION when applicable | `Idle` |
| any other event (RFC 4271 lists 8, 10-11, 13, 19, 23, 25-28; Ze declares no Event 13) | unexpected | log transition; **increments ConnectRetryCounter** | none | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleConnect -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Connect error path calls logFSMEvent(EventTCPConnectionFails) -->
<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished calls fsm.Event(EventTCPConnectionConfirmed) -->

## Timers running in this state

| Timer | Status | Notes |
|-------|--------|-------|
| ConnectRetryTimer | stopped on TCP success; restarted on held-socket failure | The outer Peer still owns reconnect scheduling and destroys the old session's timer on exit. |
| DelayOpenTimer | running only while the connected socket has sent no OPEN | `timer delay-open` is 0..240 seconds; zero disables it. |
| HoldTimer | not running during DelayOpen | Event 12 starts the OPEN-wait bound; Event 20 starts the negotiated bound. |
| KeepaliveTimer | not running during DelayOpen | Event 20 starts it unless negotiated hold time is zero. |

<!-- source: internal/component/bgp/fsm/timer.go — Timers -->
<!-- source: internal/component/bgp/reactor/peer_run.go — exponential backoff at peer level -->

## Wire side effects

- **On TCP success:** the session tunes the socket, publishes its reader and
  writer, and prepares its local OPEN proposal. DelayOpen keeps that proposal
  off the wire. Event 20 sends it before KEEPALIVE, without OpenSent; Event 12
  sends it and starts `ze.bgp.openwait` (120 seconds by default).
  <!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished -->
  <!-- source: internal/component/bgp/reactor/session_delay_open.go — delayOpenExpired -->
- **On `EventTCPConnectionFails`:** no NOTIFICATION is sent. A dial error
  returns to the peer run loop. A held-socket read failure first applies Event
  18 while DelayOpen is still running, then closes the socket and stops timers.
  <!-- source: internal/component/bgp/reactor/session_connection.go — Connect dial failure path -->
  <!-- source: internal/component/bgp/reactor/session_read.go — handleConnectionClose -->
- **On stop or error:** timer cleanup includes DelayOpen. An inbound OPEN that
  wins over an OPEN-less held connection uses `closeHeldForCollision`: Event 23,
  one retry-counter increment, and no NOTIFICATION. If the existing OPEN won
  first, the collision handoff rechecks RFC 4271 Section 6.8 instead.
  <!-- source: internal/component/bgp/reactor/session_delay_open.go — closeHeldForCollision -->
  <!-- source: internal/component/bgp/reactor/peer_connection.go — finishCollisionHandoff -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleConnect` |
| Dial and socket setup | `internal/component/bgp/reactor/session_connection.go` | `Connect`, `connectionEstablished` |
| Outer reconnect loop | `internal/component/bgp/reactor/peer_run.go` | run loop (backoff + session recreate) |
| Passive/active decision | `internal/component/bgp/reactor/session.go` | `NewSession` setting `fsm.SetPassive` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleConnect -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Connect, connectionEstablished -->
<!-- source: internal/component/bgp/reactor/session.go — NewSession -->
<!-- source: internal/component/bgp/reactor/peer_run.go — peer run loop -->

## RFC deviations

- **`EventConnectRetryTimerExpires` never fires in production.** The RFC
  specifies a fixed 120-second ConnectRetryTimer that is started when the
  FSM enters Connect and restarted on failure. Ze replaces this with
  exponential backoff at the peer level (outside the FSM), which is more
  sophisticated than the RFC design. The FSM's Event 9 handler remains
  correct and tested, but it is architecturally unreachable.
  <!-- source: internal/component/bgp/fsm/fsm.go — ARCHITECTURAL NOTES -->
- **Connect can hold a live connection.** DelayOpen is an actual waiting state,
  bounded by its own timer. After a held-socket failure the FSM reports Active;
  the old Session still ends and the Peer owns the next connection cycle.
  <!-- source: internal/component/bgp/fsm/fsm.go — handleConnect -->

## Architectural notes

- **Reconnect is at the peer level, not the FSM level.** After any exit
  from Connect back to Idle, the outer peer run loop in `peer_run.go`
  recreates the session, applies exponential backoff, and calls
  `Session.Start()` again. The FSM has no memory of the previous attempt.
- **Socket tuning happens in `connectionEstablished`**, not in the FSM or
  timer layer. This keeps the FSM free of any OS-level concerns.

## Tests exercising this state

- `internal/component/bgp/fsm/fsm_test.go` — direct state transition
  tests covering `EventTCPConnectionConfirmed`, `EventTCPConnectionFails`,
  `EventManualStop`, and the error event cluster.
  <!-- source: internal/component/bgp/fsm/fsm_test.go -->
- `internal/component/bgp/fsm/rfc4271_connect_any_other_event_test.go` --
  every declared event of the "any other event" list, in Connect and in
  Active, increments the ConnectRetryCounter and leaves Idle; the start
  events and Event 9 leave the state and the counter alone.
  <!-- source: internal/component/bgp/fsm/rfc4271_connect_any_other_event_test.go -->
- `internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go`
  -- the same list at peer level: an AutomaticStop on a passive Peer in
  Active counts the attempt, stops the timers, drops the session and refuses
  a later TCP connection on it.
  <!-- source: internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go -->
- `internal/component/bgp/reactor/rfc4271_connect_any_other_event_peer_test.go`
  -- the same list in Connect at peer level: a dial-only Peer is held inside
  its dial, an AutomaticStop counts the attempt and moves to Idle, and when
  the dial returns the far end reads end of file with no OPEN and the Peer
  drops the session; a repeated ManualStart during the dial changes nothing.
  <!-- source: internal/component/bgp/reactor/rfc4271_connect_any_other_event_peer_test.go -->
- `internal/component/bgp/reactor/session_test.go` — end-to-end tests
  that dial to a test peer, verify the Connect -> OpenSent handoff, and
  exercise dial failures.
  <!-- source: internal/component/bgp/reactor/session_test.go -->
- `internal/component/bgp/reactor/delay_open_test.go` and
  `delay_open_collision_test.go` exercise held sockets, expiry, buffered OPEN,
  and both identifier orders through real Peer cycles.
  <!-- source: internal/component/bgp/reactor/delay_open_test.go -->
  <!-- source: internal/component/bgp/reactor/delay_open_collision_test.go -->
