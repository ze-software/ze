# BGP FSM: Connect State

## TL;DR

The system is actively dialing a TCP connection to the peer. Entered from
Idle on `ManualStart` when the peer is configured with the active bit set.
Exits to OpenSent once TCP is up, or back to Idle on failure or stop.

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

The peer run loop then immediately calls `Session.Connect(ctx)` which
dials TCP via the configured `network.Dialer`. Connect state is
essentially a marker for "dial in progress". Because ze uses a blocking
`DialContext`, the FSM does not sit in Connect waiting for asynchronous
TCP events. The dial either succeeds (and the session fires
`EventTCPConnectionConfirmed`, moving the FSM to OpenSent) or fails (and
the session fires `EventTCPConnectionFails`, moving the FSM back to
Idle).

<!-- source: internal/component/bgp/reactor/session_connection.go — Connect -->
<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished -->

## Events handled in Connect

| Event | Produced by | FSM reaction | Wire side effect | Next state |
|-------|-------------|--------------|------------------|------------|
| `EventManualStart` | duplicate `Session.Start()` call | ignored (RFC 4271) | none | `Connect` |
| `EventManualStop` | `Session.Close/Teardown/CloseWithNotification` | cleanup in caller | optional Cease NOTIFICATION in caller | `Idle` |
| `EventConnectRetryTimerExpires` | not generated in production | no-op comment (reconnect handled externally) | none | `Connect` |
| `EventTCPConnectionConfirmed` | `Session.connectionEstablished` after successful `dialer.DialContext` | log transition | OPEN sent immediately after transition | `OpenSent` |
| `EventTCPConnectionFails` | `Session.Connect` dial error path | cleanup in caller | none | `Idle` |
| `EventBGPHeaderErr` / `EventBGPOpenMsgErr` / `EventNotifMsgVerErr` / `EventNotifMsg` | message decode error paths | log transition | NOTIFICATION in caller | `Idle` |
| any other event | unexpected | log transition | none | `Idle` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleConnect -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Connect error path calls logFSMEvent(EventTCPConnectionFails) -->
<!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished calls fsm.Event(EventTCPConnectionConfirmed) -->

## Timers running in this state

| Timer | Status | Notes |
|-------|--------|-------|
| ConnectRetryTimer | conceptually running per RFC, **not started in ze** | Reconnect backoff lives in the outer peer run loop, not in the FSM timer layer. |
| HoldTimer | not running | started on entry to OpenSent after OPEN is sent. |
| KeepaliveTimer | not running | started in the OpenConfirm -> Established handoff. |

<!-- source: internal/component/bgp/fsm/timer.go — Timers -->
<!-- source: internal/component/bgp/reactor/peer_run.go — exponential backoff at peer level -->

## Wire side effects

- **On `EventTCPConnectionConfirmed` (transition to OpenSent):** the session
  layer immediately sends the OPEN message via `sendOpen(conn)`, after
  tuning socket options (`TCP_NODELAY`, `IP_TOS` DSCP CS6, SO_RCVBUF,
  SO_SNDBUF) and creating buffered reader/writer wrappers.
  <!-- source: internal/component/bgp/reactor/session_connection.go — connectionEstablished socket tuning and sendOpen call -->
- **On `EventTCPConnectionFails`:** no NOTIFICATION is sent. The caller
  simply returns the dial error to the peer run loop.
  <!-- source: internal/component/bgp/reactor/session_connection.go — Connect dial failure path -->
- **On `EventManualStop`:** if a connection somehow exists at this point,
  `Close` / `Teardown` sends the configured Cease NOTIFICATION before
  invoking the FSM event. In practice the connection is nil in Connect
  because the dial is blocking, so no NOTIFICATION is sent.
  <!-- source: internal/component/bgp/reactor/session_connection.go — Close, Teardown -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| State transitions | `internal/component/bgp/fsm/fsm.go` | `handleConnect` |
| Dial and socket setup | `internal/component/bgp/reactor/session_connection.go` | `Connect`, `connectionEstablished` |
| Outer reconnect loop | `internal/component/bgp/reactor/peer_run.go` | run loop (backoff + session recreate) |
| Passive/active decision | `internal/component/bgp/reactor/session.go` | `newSession` setting `fsm.SetPassive` |

<!-- source: internal/component/bgp/fsm/fsm.go — handleConnect -->
<!-- source: internal/component/bgp/reactor/session_connection.go — Connect, connectionEstablished -->
<!-- source: internal/component/bgp/reactor/session.go — newSession -->
<!-- source: internal/component/bgp/reactor/peer_run.go — peer run loop -->

## RFC deviations

- **`EventConnectRetryTimerExpires` never fires in production.** The RFC
  specifies a fixed 120-second ConnectRetryTimer that is started when the
  FSM enters Connect and restarted on failure. Ze replaces this with
  exponential backoff at the peer level (outside the FSM), which is more
  sophisticated than the RFC design. The FSM's Event 9 handler remains
  correct and tested, but it is architecturally unreachable.
  <!-- source: internal/component/bgp/fsm/fsm.go — ARCHITECTURAL NOTES -->
- **Connect is effectively a transient marker.** Because ze's dial is
  blocking, the FSM is never observed sitting in Connect waiting for TCP.
  The state still exists because the FSM is a faithful implementation of
  the RFC state table; it just is not a "waiting" state in practice.
  <!-- source: internal/component/bgp/reactor/session_connection.go — DialContext in Connect -->

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
- `internal/component/bgp/reactor/session_test.go` — end-to-end tests
  that dial to a test peer, verify the Connect -> OpenSent handoff, and
  exercise dial failures.
  <!-- source: internal/component/bgp/reactor/session_test.go -->
