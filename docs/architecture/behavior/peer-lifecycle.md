# BGP Peer Lifecycle (outer loop)

## TL;DR

The FSM runbooks cover what happens *inside* a session. This page covers
what happens *around* it: the outer per-peer run loop that creates
sessions, retries with exponential backoff, resolves inbound connection
races, recovers from panics, and ultimately replaces RFC 4271's
`ConnectRetryTimer` (Event 9) with a peer-level strategy.

If you are trying to answer "why does ze reconnect when a session
drops?", "where does backoff come from?", "where does the FSM state
callback fire?", or "where are inbound connection collisions resolved?",
this is the page.

<!-- source: internal/component/bgp/reactor/peer.go — Peer, NewPeer, Start, Stop -->
<!-- source: internal/component/bgp/reactor/peer_run.go — run, safeRunOnce, runOnce -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — Accept/Pending/Inbound connection helpers -->

## Peer states (distinct from FSM states)

`Peer` has its own state enum that is coarser than `fsm.State` and
reflects operator-visible peer status, not protocol detail.

| Peer state | Meaning |
|------------|---------|
| `PeerStateStopped` | Peer goroutine has exited or has not yet started. |
| `PeerStateConnecting` | Peer is actively dialing, backing off, or otherwise trying to reach the remote. Maps to FSM `Connect`, `OpenSent`, `OpenConfirm`, or "between sessions during backoff". |
| `PeerStateActive` | Peer is passive and waiting to accept an incoming TCP connection. Maps to FSM `Active`. |
| `PeerStateEstablished` | FSM has transitioned into `StateEstablished` and the per-session callback ran. |

<!-- source: internal/component/bgp/reactor/peer.go — PeerState enum (PeerStateStopped, PeerStateConnecting, PeerStateActive, PeerStateEstablished) -->

The peer state transitions are driven from `peer_run.go`:

- Set to `PeerStateConnecting` or `PeerStateActive` at the top of each
  `runOnce`, based on `settings.Connection.IsActive()`.
- Set to `PeerStateEstablished` inside the `fsm.SetCallback` closure when
  the FSM moves into `StateEstablished`.
- Set back to `PeerStateConnecting` in the same callback when the FSM
  leaves `StateEstablished` for any reason.
- Set to `PeerStateConnecting` before entering the backoff window after
  a session error.
- Set to `PeerStateStopped` in `cleanup()` on context cancellation.

<!-- source: internal/component/bgp/reactor/peer_run.go — setState calls -->
<!-- source: internal/component/bgp/reactor/peer_run.go — fsm.SetCallback closure -->
<!-- source: internal/component/bgp/reactor/peer_run.go — cleanup -->

## Entry points

| API | Purpose |
|-----|---------|
| `Peer.Start()` | Begin the peer goroutine with a background context. |
| `Peer.StartWithContext(ctx)` | Begin the peer goroutine with a caller-supplied context. Once-only; calling again while running is a no-op. |
| `Peer.Stop()` | Cancel the peer context. The run loop notices at the next select and unwinds cleanly via `cleanup()`. |
| `Peer.Teardown(subcode, msg)` | Send Cease NOTIFICATION to the current session. If `sendInitialRoutes` is in flight, queues the teardown via `opQueue` so routes and EoR are flushed first. Returns `ErrOpQueueFull` if the queue is saturated. |
| `Peer.AcceptConnection(conn)` | Hand an incoming TCP connection to the current session via `session.Accept`. Used by the reactor's inbound dispatch for passive peers that already have a live session. |
| `Peer.SetInboundConnection(conn)` | Store an inbound connection while the peer has no session. Wakes the run loop via a buffered `inboundNotify` channel so backoff can be cut short. |
| `Peer.AcceptConnectionWithOpen(conn, open)` | Accept with a pre-parsed OPEN (used after collision resolution). |
| `Peer.SetPendingConnection(conn)` / `ResolvePendingCollision(open)` / `ClearPendingConnection()` | Collision resolution per RFC 4271 §6.8. See section below. |

<!-- source: internal/component/bgp/reactor/peer.go — Start, StartWithContext, Stop, Teardown -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — AcceptConnection, SetInboundConnection, takeInboundConnection, AcceptConnectionWithOpen, SetPendingConnection, ResolvePendingCollision, ClearPendingConnection, HasPendingConnection -->

## The run loop

`Peer.run()` is the peer goroutine. Its shape:

```text
for {
    check ctx.Done

    err := safeRunOnce()   # one session lifecycle

    check ctx.Done

    if err == ErrTeardown:
        delay = reconnectMin
        continue   # immediate reconnect

    if err == ErrPrefixLimitExceeded and PrefixIdleTimeout > 0:
        wait idleBase * 2^(teardownCount-1), capped at 1 hour

    if any other err:
        wait `delay`, wake early on inbound notify
        delay *= 2 (capped at reconnectMax)

    if err == nil:
        delay = reconnectMin
        prefixTeardownCount = 0
}
```

<!-- source: internal/component/bgp/reactor/peer_run.go — run -->

**Backoff defaults:**

| Constant | Default | Source |
|----------|---------|--------|
| `DefaultReconnectMin` | 5 seconds | `peer.go` |
| `DefaultReconnectMax` | 60 seconds | `peer.go` |
| `reconnectMin` override | `settings.ConnectRetry` if non-zero, else `DefaultReconnectMin` | `peer.go` `NewPeer` |
| `reconnectMax` override | `DefaultReconnectMax` (no setting yet) | `peer.go` |

<!-- source: internal/component/bgp/reactor/peer.go — DefaultReconnectMin, DefaultReconnectMax, NewPeer, SetReconnect -->

**Per-attempt accounting:**

The run loop records timing metrics per attempt:

- `peerConnectAttempts` counter incremented at the start of each attempt.
- `peerConnectAttemptSeconds` histogram observing the wall time of
  `safeRunOnce`.
- `peerDialSeconds{ok|fail}` histogram observing dial time inside
  `runOnce`.
- `peerBackoffSeconds` histogram observing how long the backoff window
  held.

<!-- source: internal/component/bgp/reactor/peer_run.go — run, runOnce metric calls -->

## `safeRunOnce` and panic recovery

`safeRunOnce` is a one-line wrapper that defers a `recover()` around
`runOnce`. If anything inside the session lifecycle panics (FSM, message
handling, plugin callback, timer callback), the panic is logged with a
4KB stack trace and converted into an error. The error feeds back into
the run loop's normal backoff path: the peer reconnects after the usual
delay instead of crashing the daemon.

This is the **primary failure domain boundary** in the BGP plugin. A bug
in one peer's message handling drops and retries that peer, not the
rest of the daemon. Matches the ExaBGP failure-domain model.

<!-- source: internal/component/bgp/reactor/peer_run.go — safeRunOnce -->

## `runOnce`: one session lifecycle

`runOnce` is where one `Session` is created, wired, driven, and torn
down. Step-by-step:

1. **Reset `notificationExchanged` flag.** Used later to decide whether
   to raise `session-dropped` on the report bus.
2. **Create and wire the session.** `NewSession` + `SetClock`,
   `SetDialer`, metric hookups, `onMessageReceived`, `onNotifSent`,
   `onNotifRecv`, `SetSourceID`, `SetPluginCapabilityGetter`,
   `SetPluginFamiliesGetter`, `SetOpenValidator`.
3. **Publish the session on `p.session` under lock** so other API
   callers (`Teardown`, `AcceptConnection`, metrics) can see it.
4. **Deferred cleanup:** clear negotiated capabilities, clear encoding
   contexts, clear prefix-threshold warnings, reset
   `sendingInitialRoutes` flag, and null out `p.session`.
5. **Set initial peer state** to `PeerStateConnecting` or
   `PeerStateActive` based on connection mode.
6. **Start the FSM:** `session.Start()` fires `EventManualStart`.
7. **Dial if active:** `session.Connect(p.ctx)` which is blocking.
   Dial failure returns immediately with the dial error.
8. **Take buffered inbound connection if passive:**
   `takeInboundConnection()` returns any connection that arrived while
   the session was nil; `session.Accept(conn)` wires it in.
9. **Register the FSM state-change callback** via `session.fsm.SetCallback`.
10. **Create the per-peer delivery channel** and launch the delivery
    goroutine (see "Delivery channel" section below).
11. **Run the session:** `session.Run(p.ctx)` blocks until the session
    exits.
12. **Drain the delivery channel:** close, wait for worker goroutine.
13. **Return the session error** to `run()` which decides backoff vs
    immediate retry.

<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce -->

## FSM state callback

Registered in `runOnce` via `session.fsm.SetCallback`. This is where the
peer-level reactions to FSM state changes happen. The callback is called
from `fsm.change()` with the mutex *temporarily released*, so it may
take additional locks safely.

<!-- source: internal/component/bgp/fsm/fsm.go — change -->
<!-- source: internal/component/bgp/reactor/peer_run.go — SetCallback closure -->

**On transition into `StateEstablished`:**

1. Capture negotiated capabilities via `session.Negotiated()`.
2. Store as `NewNegotiatedCapabilities` on the peer.
3. Set per-peer encoding contexts via `setEncodingContexts(neg)`.
4. `setState(PeerStateEstablished)`.
5. `SetEstablishedNow()` records the establishment timestamp.
6. Emit `session established` info log with local AS, peer AS.
7. Reset API sync counter to the number of plugin bindings with
   `SendUpdate` permission.
8. Set `sendingInitialRoutes` to 1 (important: this happens **before**
   notifying plugins, so `ShouldQueue()` returns true during event
   delivery and routes do not bypass the queue).
9. Notify the reactor via `reactor.notifyPeerEstablished(p)` and
   `reactor.notifyPeerNegotiated(p, neg)`.
10. Spawn `sendInitialRoutes()` in its own goroutine (per-session
    lifecycle, not per-event).

**On transition *out of* `StateEstablished`:**

1. Determine a textual reason (`"session closed"`, `"connection lost"`).
2. If no NOTIFICATION was exchanged (`notificationExchanged == false`),
   raise `session-dropped` on the report bus via
   `raiseSessionDropped`. The check avoids duplicating an event the
   operator already sees elsewhere when NOTIFICATION fired.
3. Notify the reactor via `reactor.notifyPeerClosed(p, reason)`.
4. Clear negotiated capabilities and encoding contexts.
5. `setState(PeerStateConnecting)`.
6. Log `session closed` with the reason.

<!-- source: internal/component/bgp/reactor/peer_run.go — SetCallback Established and leave-Established branches -->
<!-- source: internal/component/bgp/reactor/peer_run.go — sendInitialRoutes spawn -->

## Reconnect backoff

| Scenario | Delay |
|----------|-------|
| Normal session error | Exponential, starting at `reconnectMin` (default 5s), doubling each attempt, capped at `reconnectMax` (default 60s). |
| `ErrTeardown` (API-initiated stop) | Reset `delay` to `reconnectMin`, continue loop immediately. No wait. |
| Successful session exit (err == nil) | Reset `delay` to `reconnectMin` and `prefixTeardownCount` to 0. |
| `ErrPrefixLimitExceeded` with `PrefixIdleTimeout > 0` | Separate backoff: `PrefixIdleTimeout * 2^(teardownCount-1)`, capped at 1 hour. `prefixTeardownCount` capped at 60 to prevent `time.Duration` overflow. Per RFC 4486. |
| Inbound connection arrives during backoff | Reset `delay` to `reconnectMin` and continue immediately. `inboundNotify` is a buffered channel (size 1) signaled by `SetInboundConnection`. |

<!-- source: internal/component/bgp/reactor/peer_run.go — run backoff arms -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — SetInboundConnection signals inboundNotify -->

## Inbound connection handling for passive peers

Passive peers do not dial. They wait for inbound connections dispatched
by the reactor's accept loop. Two race conditions need handling:

1. **Remote reconnects faster than our backoff.** The peer is in the
   `delay` select after a session failure. The reactor's accept loop
   calls `SetInboundConnection(conn)`, which stores the connection and
   signals `inboundNotify`. The run loop's select wakes early, resets
   `delay`, and starts a fresh attempt. The new `runOnce` calls
   `takeInboundConnection()` at step 8 (after `Start` and optional
   `Connect`) and passes the stored connection to `session.Accept`.
2. **Stale buffered connection.** If `Accept` fails (for example, the
   peer has already torn down the half-open connection while it sat in
   the buffer), `runOnce` closes the stale connection quietly and
   returns an error. The normal backoff path applies.

<!-- source: internal/component/bgp/reactor/peer_connection.go — SetInboundConnection, takeInboundConnection -->
<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce takeInboundConnection path -->

## Collision resolution (RFC 4271 §6.8)

When both peers initiate simultaneously, the system may have a live
session and a second incoming connection for the same peer. The
reactor stashes the second connection as a "pending" connection on the
peer via `SetPendingConnection(conn)`. After the pending side sends its
OPEN, the reactor calls `ResolvePendingCollision(pendingOpen)`.

`ResolvePendingCollision`:

1. Reads `session.DetectCollision(pendingOpen.BGPIdentifier)` to decide
   who wins. Higher BGP router-id wins per RFC 4271 §6.8.
2. If the remote wins (pending connection is accepted):
   - Stores the pending OPEN so it can be replayed.
   - Launches a goroutine that sends a Cease/ConnectionCollision
     NOTIFICATION on the existing session and tears it down.
   - Returns `(true, pendingConn, pendingOpen, session.Done())` so the
     caller waits on the existing session's done channel before
     accepting the pending one.
3. If the local side wins:
   - Rejects the pending connection.
   - Keeps the existing session running.

The caller (reactor) then calls `AcceptConnectionWithOpen(conn, open)`
which drives the new session through the `AcceptWithOpen` path
(`connectionEstablished` + `processOpen` in one synchronous sequence,
as documented in the Active and OpenConfirm FSM runbooks).

<!-- source: internal/component/bgp/reactor/peer_connection.go — SetPendingConnection, ResolvePendingCollision, ClearPendingConnection, HasPendingConnection -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — AcceptConnectionWithOpen -->

## Delivery channel (per-peer async message delivery)

`runOnce` creates a buffered `p.deliverChan` and a long-lived worker
goroutine that drains batches of `deliveryItem` and calls
`reactor.messageReceiver.OnMessageBatchReceived`. The batching
amortizes subscription lookup and format-mode computation across
multiple UPDATEs.

Key properties:

- **Channel + worker, not per-event goroutine.** One goroutine per
  peer, not one per UPDATE.
- **Batch drain via `drainDeliveryBatch`.** The worker reads one item,
  then grabs everything else that is currently queued without blocking,
  and processes them as one batch.
- **Panic recovery** around the worker loop. On panic, the goroutine
  logs the stack, closes `deliveryDone`, and exits. `runOnce` waits
  on `<-deliveryDone` before returning, so shutdown is deterministic.
- **Drain on session exit.** After `session.Run` returns, `runOnce`
  closes `p.deliverChan` and waits on `deliveryDone`. The worker
  processes any remaining buffered items and exits.

<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce delivery channel setup, drain, and worker goroutine -->

## `cleanup()`

Runs when the peer goroutine exits (context cancelled or run loop
returned). Invoked via `defer p.cleanup()` in `run()`.

Steps:

1. Clear negotiated capabilities and encoding contexts.
2. Clear stats via `ClearStats()`.
3. Close the current session if one exists (`session.Close()` which
   sends Cease/AdminShutdown NOTIFICATION).
4. Close any stored inbound connection quietly.
5. Null out `cancel` and `inboundConn`.
6. `setState(PeerStateStopped)`.

<!-- source: internal/component/bgp/reactor/peer_run.go — cleanup -->

## RFC 4271 ConnectRetryTimer replacement

RFC 4271 specifies a `ConnectRetryTimer` (mandatory FSM attribute,
Section 8.1.3, Event 9) with a suggested default of 120 seconds. The
RFC expects the FSM to sit in `Connect` or `Active` state waiting for
both TCP completion and the retry timer to fire, retrying the dial on
timer expiry.

**Ze does not implement this.** Instead:

- Outgoing dials use a blocking `DialContext`, so the FSM never sits in
  `Connect` waiting for asynchronous TCP events. The dial either
  succeeds (transition to OpenSent) or fails (transition to Idle).
- Reconnect delay lives at the **peer level** in `Peer.run()` with
  **exponential backoff** (min 5s, max 60s), which is more robust than
  the RFC's fixed 120s constant.
- The `EventConnectRetryTimerExpires` FSM event is defined and its
  handler arms in `handleConnect` and `handleActive` are correct and
  tested, but the event is **never fired in production**.

This is a deliberate architectural choice documented in the
`fsm.go` file header under "ARCHITECTURAL NOTES".

<!-- source: internal/component/bgp/fsm/fsm.go — ARCHITECTURAL NOTES -->
<!-- source: internal/component/bgp/reactor/peer_run.go — run function doc comment -->

## Panic recovery summary

Two independent `recover()` boundaries in the peer lifecycle:

| Boundary | Scope | Recovery behavior |
|----------|-------|-------------------|
| `safeRunOnce` | Entire session lifecycle (connect, FSM, message handling, timer callbacks, plugin callbacks that run synchronously from the session goroutine) | Log panic with stack, convert to error, return to `run()` for normal backoff |
| Delivery worker goroutine | Async message delivery to the reactor's message receiver | Log panic with stack, close `deliveryDone`, exit worker (remaining buffered items are dropped intentionally) |

Neither boundary ever kills the peer goroutine permanently. The peer
always either reconnects or is stopped via context cancellation.

<!-- source: internal/component/bgp/reactor/peer_run.go — safeRunOnce, runOnce delivery goroutine recover -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| Peer struct, constructor, state enum | `internal/component/bgp/reactor/peer.go` | `Peer`, `NewPeer`, `PeerState`, `setState` |
| Start / Stop / Teardown API | `internal/component/bgp/reactor/peer.go` | `Start`, `StartWithContext`, `Stop`, `Teardown` |
| Reconnect defaults and overrides | `internal/component/bgp/reactor/peer.go` | `DefaultReconnectMin`, `DefaultReconnectMax`, `SetReconnect` |
| Outer run loop and backoff | `internal/component/bgp/reactor/peer_run.go` | `run`, `safeRunOnce` |
| Per-attempt session lifecycle | `internal/component/bgp/reactor/peer_run.go` | `runOnce` |
| FSM state-change callback | `internal/component/bgp/reactor/peer_run.go` | `SetCallback` closure inside `runOnce` |
| Cleanup on peer stop | `internal/component/bgp/reactor/peer_run.go` | `cleanup` |
| Inbound connection buffering | `internal/component/bgp/reactor/peer_connection.go` | `SetInboundConnection`, `takeInboundConnection` |
| RFC 6.8 collision resolution | `internal/component/bgp/reactor/peer_connection.go` | `SetPendingConnection`, `ResolvePendingCollision`, `AcceptConnectionWithOpen` |

<!-- source: internal/component/bgp/reactor/peer.go — Peer, NewPeer, PeerState, Start, Stop, Teardown, SetReconnect, DefaultReconnectMin, DefaultReconnectMax -->
<!-- source: internal/component/bgp/reactor/peer_run.go — run, safeRunOnce, runOnce, cleanup -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — SetInboundConnection, takeInboundConnection, SetPendingConnection, ResolvePendingCollision, AcceptConnectionWithOpen -->

## Tests exercising this layer

- `internal/component/bgp/reactor/peer_test.go` — peer lifecycle tests
  covering start, stop, teardown, reconnect backoff, and the FSM
  state-change callback.
  <!-- source: internal/component/bgp/reactor/peer_test.go -->
- `internal/component/bgp/reactor/peer_connection_test.go` —
  collision resolution and inbound connection tests.
  <!-- source: internal/component/bgp/reactor/peer_connection_test.go -->
- `internal/component/bgp/reactor/session_test.go` — end-to-end session
  tests that exercise peer + session + FSM together.
  <!-- source: internal/component/bgp/reactor/session_test.go -->
