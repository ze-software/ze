# BGP Peer Lifecycle (outer loop)

## TL;DR

The FSM runbooks cover what happens *inside* a session. This page covers
what happens *around* it: the outer per-peer run loop that creates
sessions, retries with exponential backoff, resolves inbound connection
races, recovers from panics, and schedules connection retries. A held
DelayOpen connection in Connect, or any Active connection, preserves the
restarted FSM timer's deadline on transport failure; ordinary failures use
peer-level exponential backoff.

If you are trying to answer "why does ze reconnect when a session
drops?", "where does backoff come from?", "where does the FSM state
callback fire?", or "where are inbound connection collisions resolved?",
this is the page.

A peer outlives its sessions, so the `Peer` also carries the two counters
that fence its forwarded routes across them. `forwardGeneration` advances
when a new session reaches Established and when the peer is stopped, and it
takes each new value from one process-wide clock (`advanceForwardGeneration`),
so the first session of a `Peer` built again for an address that was removed
and re-added is above every generation the old one used (the new `Peer` holds
zero until that session reaches Established). `forwardRevocation` advances when a
reload unbinds processes from a session that stays up (`unbindProcesses`),
because that targeted down moves no generation. An Adj-RIB-Out path is still
owned by its source only while both counters match the values it recorded.

<!-- source: internal/component/bgp/reactor/peer.go — Peer, NewPeer, Start, Stop -->
<!-- source: internal/component/bgp/reactor/peer_derived_bindings.go — unbindProcesses -->
<!-- source: internal/component/bgp/reactor/relay_recovery.go — adjOutOwnerSessionCurrent -->
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
| `PeerStateIdleHold` | Peer is deliberately down and is trying nothing. A prefix limit stopped the session and the family that overflowed asks for no reconnect. Only an operator recreating the peer brings it back. Shown as `idle-hold`. |

<!-- source: internal/component/bgp/reactor/peer.go — PeerState enum (PeerStateStopped, PeerStateConnecting, PeerStateActive, PeerStateEstablished, PeerStateIdleHold) -->

The peer state transitions are driven from `peer_run.go`:

- Set to `PeerStateConnecting` or `PeerStateActive` at the top of each
  `runOnce`, based on `settings.Connection.IsActive()`.
- Set to `PeerStateEstablished` inside the `fsm.SetCallback` closure when
  the FSM moves into `StateEstablished`.
- Set back to `PeerStateConnecting` in the same callback when the FSM
  leaves `StateEstablished` for any reason.
- Set to `PeerStateConnecting` before entering the backoff window after
  a session error.
- Set to `PeerStateIdleHold` in `holdDownAfterPrefixTeardown` when a prefix
  limit stopped the session and the offending family asks for no reconnect.
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
| `Peer.Teardown(subcode, msg)` | Send Cease NOTIFICATION to the current session. If `sendInitialRoutes` is in flight, queues the teardown via `opQueue` so routes and EoR are flushed first. Returns `ErrOpQueueFull` if the queue is saturated. This is a peer-level teardown. A reload's binding restart uses `teardownOf` with the session it decided on instead: it does nothing once the peer has left that session, and the drain drops it if still queued when a later session holds the peer. |
| `Peer.acceptConnection(conn)` | Hand an incoming TCP connection to the current session via `session.Accept`. Used by the reactor's inbound dispatch for passive peers that already have a live session. |
| `Peer.setInboundConnection(conn)` | Store an inbound connection while the peer has no session. Wakes the run loop via a buffered `inboundNotify` channel so backoff can be cut short. |
| `Peer.setPendingConnection(conn)` / `Reactor.handlePendingCollision` / `Peer.resolveIncoming` | Reserve one independent incoming Session, examine its actual OPEN per RFC 4271 §6.8, and transfer the same winning Session through the inbound slot. A decided loser retains admission through joined retirement. See below. |

<!-- source: internal/component/bgp/reactor/peer.go — Start, StartWithContext, Stop, Teardown -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — acceptConnection, setInboundConnection, takeInboundConnection, setPendingConnection, hasPendingConnection -->
<!-- source: internal/component/bgp/reactor/incoming_candidate.go -- handlePendingCollision, resolveIncoming, beginIncomingRetirement -->

## The run loop

`Peer.run()` is the peer goroutine. It holds the reconnect delay in a local,
`delay`, which starts at `reconnectMin`. Each pass of the loop takes these
steps in order:

| # | Step | What happens |
|---|------|--------------|
| 1 | Stop checks | Return when the peer is stopping or its context ended. A peer held administratively down parks in `holdAdminDown`, then `delay` is reset to `reconnectMin` |
| 2 | One cycle | `err := safeRunOnce()`, one session lifecycle. The context is checked again after it |
| 3 | Stable session | When `err` is not nil and `cycleStayedEstablished` holds (the cycle reached Established and stayed there at least `reconnectMax`), `delay` is reset to `reconnectMin` and `prefixTeardownCount` to 0 |
| 4 | Local teardown | `ErrTeardown`: `delay` is reset to `reconnectMin` and the loop redials at once, with no wait |
| 5 | Prefix teardown | The mode of the family that overflowed decides. `never` (the default) holds the peer down until it stops. `timer` increments `prefixTeardownCount` and waits `idle-timeout * 2^(count-1)`, capped at one hour. `backoff` goes on to step 6 |
| 6 | Any other error | A transport failure in Active, or in Connect while DelayOpen holds its socket, waits only the remainder of its actual restarted FSM retry deadline. Other errors wait `fsm.Jitter(delay)` and double `delay` after a full wait, capped at `reconnectMax`. Administrative wake or inbound connection ends either wait early and resets `delay` to `reconnectMin` |
| 7 | No error | `runOnce` returns nil only when the peer is stopping or held administratively down when it would publish the session. `delay` is reset to `reconnectMin` and `prefixTeardownCount` to 0, and step 1 ends or parks the loop |

<!-- source: internal/component/bgp/reactor/peer_run.go — run, cycleStayedEstablished -->

**Backoff defaults:**

| Constant | Default | Source |
|----------|---------|--------|
| `DefaultReconnectMin` | 5 seconds | `peer.go` |
| `DefaultReconnectMax` | 60 seconds | `peer.go` |
| `reconnectMin` override | None. `NewPeer` sets `DefaultReconnectMin` and no config leaf changes it. `settings.ConnectRetry` is the connect TIMEOUT, not the backoff floor: wiring it in made floor(120s) exceed ceiling(60s) and stranded peers `connecting` for two minutes (spec-fixit-redistribute-establishment-stall). | `peer.go` `NewPeer` |
| `reconnectMax` override | `DefaultReconnectMax` (no setting yet). `reconnectMax` is also the stable time: a session that stays in Established that long restarts the delay and the prefix teardown count | `peer.go` `NewPeer`; `peer_run.go` `cycleStayedEstablished` |

<!-- source: internal/component/bgp/reactor/peer.go — DefaultReconnectMin, DefaultReconnectMax, NewPeer, setReconnectDelay -->

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

Two nested boundaries contain a panic in one peer's session, both through the
BGP subsystem's `taskguard` (`bgpPanics` in `panic_guard.go`). Each contained
panic is counted in `ze_task_panics_total{component="bgp",task}`, written as a
crash record that `ze show crashes` lists with kind `recovered`, and logged at
ERROR with the peer, the message type being processed and the stack.

| Boundary | Task | What the panic ends | How |
|----------|------|---------------------|-----|
| `Session.Run`, around the read loop | `read` | The session | `ceaseAfterPanic`: Cease / Unspecific to the peer within the hold-expiry notify deadline, then Event 8 (AutomaticStop). `Run` returns the contained panic as its error |
| `safeRunOnce`, around `runOnce` | `connect` | The connection cycle | What runs outside `Run` (dial, OPEN setup, teardown) returns the contained panic as the cycle's error |

Either error feeds the run loop's normal backoff: the peer reconnects after
the usual delay and the rest of the daemon keeps running. The read boundary
names the message type it was processing (`UPDATE` when an UPDATE panicked,
including one flushed from the coalescing buffer).

RFC 4486 Section 4 defines no Cease subcode for an internal failure, so the
Cease carries Unspecific (RFC 4271 Section 4.5). When another writer holds the
session's write path past the deadline, `tryNotificationWithin` gives up and no
NOTIFICATION is sent. The close routine (`closeConnWithin`) then does not wait
for the path again: it logs at ERROR that the peer was not told, fails the
stuck writer's socket write and closes the socket, so that writer releases the
path. Every close of a session connection runs that routine: see
[core design](../core-design.md#graceful-tcp-close-half-close).

A panicking incoming reader must release its locks before retirement and
containment can finish. A reactor-wide lock left held by the panicking frame
would stall every other peer (`plan/learned/044-panic-boundaries.md`, "What outlives
the spec"). Incoming collision arbitration holds `Peer.mu` only for its
decision and atomic receipt publication, releasing it even if a decision
panics. Discarding an older inbound receipt, joining candidate retirement and
starting the incumbent handoff happen outside that hold, so the incoming
reader's cleanup can reclaim the same peer owner before containment returns.

<!-- source: internal/component/bgp/reactor/incoming_candidate.go -- resolveIncoming, handlePendingCollision -->
<!-- source: internal/component/bgp/reactor/panic_guard.go — bgpPanicTasks, bgpPanics -->
<!-- source: internal/component/bgp/reactor/session.go — Session.Run, describeRead -->
<!-- source: internal/component/bgp/reactor/session_connection.go — ceaseAfterPanic, beginTeardown, endTeardown, closeConnWithin, dropConn -->
<!-- source: internal/component/bgp/reactor/session_write.go — tryNotificationWithin -->
<!-- source: internal/component/bgp/reactor/peer_run.go — safeRunOnce -->

## `runOnce`: one session lifecycle

`runOnce` publishes, drives and retires one selected `Session`. Step-by-step:

1. **Get the Session.** A selected candidate joins its private reader, handoff
   and cancellation bridge, then retains its original Session. Otherwise,
   `Peer.newSession` constructs one with immutable callbacks,
   capability inputs and capture input before TCP starts.
2. **Publish the Session under `Peer.mu`.** Recheck stop and administrative
   admission in the same hold. A selected receipt publishes its validated remote
   Identifier after incumbent cleanup. A refused Session retires without dialing.
3. **Install cleanup and negotiated-session finalizers.** Every published Session
   owns connection closure, timer retirement, capture join and its identifier
   claim. Selection permits its existing capture input to open a file.
4. **Choose the ordinary start event.** The first cycle takes Event 1. A cycle
   after Established takes Event 3. Other cycles take Event 6. A retained candidate
   keeps its existing FSM. Strict BFD starts before an ordinary FSM start and
   survives retries until Peer cleanup.
5. **Install the ordinary FSM publication callback before TCP.** A private
   candidate keeps its own callback until adoption. The callback publishes
   negotiated state, Established and leave events for the selected Session.
6. **Consume the inbound slot before dialing.** Ordinary sockets use `Accept`.
   Compatibility buffered input uses `acceptWithOpen`. A candidate receipt retains
   its Session, socket, reader, validated OPEN and timer deadlines.
7. **Dial when needed.** An active peer dials only when no selected candidate or
   accepted connection already owns the Session.
8. **Start delivery and install its down finalizer.** The delivery worker exists
   before a retained candidate can publish negotiation or Established.
9. **Adopt a retained candidate.** `adoptIncoming` installs selected publication
   under the Session OPEN lock. An unadvanced OPEN receipt then advances once.
   Adoption preserves the original OPEN and KEEPALIVE writes.
10. **Run and retire the Session.** `Session.Run` drives the selected connection.
    Exits after delivery starts drain it, announce down and join Session resources
    before the next cycle. A transport retry budget reaches `Peer.run` only after
    its failure owner and timers retire.

<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce -->

### One immutable proposal for each OPEN

`Peer.newSession` installs the settings and plugin snapshot getters before either
TCP rail starts, including for a pending incoming candidate. The settings getter
copies the shared settings under the Peer lock. The plugin getter reads the
injector once and derives both the declaration index and the capabilities encoded
in OPEN from that read. It records raw declarations before applying the runtime
GR restart bits, so expiration of the restart window does not look like a
configuration change.
<!-- source: internal/component/bgp/reactor/peer_session_factory.go -- newSession -->
<!-- source: internal/component/bgp/reactor/peer.go -- settingsSnapshot, pluginCapabilitySnapshot -->

`prepareOpen` constructs one immutable `preparedOpen`: the exact message, its
settings input and the plugin declaration index. Repeated preparation reuses it.
OPEN admission, AS and identifier checks, collision comparison, negotiation and
the reload baseline all use this same proposal. An incumbent publishing a dynamic
peer's learned AS cannot change a candidate that already prepared its OPEN; a
candidate prepared afterwards captures the published AS. The Session retains the
shared settings pointer separately so policy reloads still reach live traffic.
<!-- source: internal/component/bgp/reactor/session_open_proposal.go -- preparedOpen, openProposal -->
<!-- source: internal/component/bgp/reactor/session_negotiate.go -- prepareOpen, openSettingsSnapshot, buildOpenFromSnapshot -->

Proposal construction has its own mutex, `proposalMu`, separate from the OPEN
action owner. `prepareOpen` holds it from before either input getter through
publication. A reload probing an unprepared session only tries this lock: a busy
capture forces a restart because it may already hold the old injector snapshot.
On success the probe rereads publication under `Session.mu`, then releases
`proposalMu` before reading any injector or returning its decision. This unlock
runs no protocol callback or owed work. Dynamic reconciliation holds the reactor
lock, so releasing the OPEN action owner there could deliver an owed TCP failure
whose peer callback reacquires that same reactor lock.
<!-- source: internal/component/bgp/reactor/session.go -- Session -->
<!-- source: internal/component/bgp/reactor/peer_settings_negotiation.go -- pluginCapabilitiesKept -->
<!-- source: internal/component/bgp/reactor/reactor_dynamic.go -- reconcileDynamicGroupsJournaled, planDynamicGroupsLocked -->
<!-- source: internal/component/bgp/reactor/peer_derived_bindings.go -- markSessionUp -->

The reload probe captures one candidate plugin snapshot for both its declaration
comparison and candidate OPEN construction. Publication and rollback between
independent store reads therefore cannot pair declarations from one generation
with wire capabilities from another. Each published snapshot remains immutable.
<!-- source: internal/component/bgp/reactor/peer_settings_negotiation.go -- negotiationOutcomeUnchanged, pluginCapabilitiesKept -->
<!-- source: internal/component/bgp/reactor/session_plugin_capabilities.go -- pluginCapabilitySnapshot -->

## FSM state callback

Ordinary Sessions install their publication callback in `runOnce` before TCP.
A private candidate keeps its own callback until `adoptIncoming` installs publication.
The publication callback applies the peer-level reactions below. It is called
from `fsm.change()` with the mutex *temporarily released*, so it may
take additional locks safely.

<!-- source: internal/component/bgp/fsm/fsm.go — change -->
<!-- source: internal/component/bgp/reactor/peer_run.go — SetCallback closure -->

**On transition into `StateEstablished`:**

1. Capture negotiated capabilities via `session.Negotiated()`.
2. Store as `NewNegotiatedCapabilities` on the peer.
3. Set per-peer encoding contexts via `setEncodingContexts(neg)`.
4. `enterEstablished(session)`. It resets the session-ready expectation
   set to the plugin bindings that push routes and declare
   `SignalsSessionReady` (`Peer.resetAPISync`), raises the replay fence
   over those that also declare `FencesLiveForwards`
   (`Peer.raiseReplayFence`), takes the initial-replay receipt, closes the
   session's initial-sync gate
   (`Session.sendingInitialRoutes` to 1), marks its End-of-RIB owed
   (`Session.initialSyncEOROwed`), publishes the session as
   `p.establishedSession`, and only then calls
   `setState(PeerStateEstablished)`. The order matters: an observer
   that sees Established already sees the gate closed, so `shouldQueue()`
   is true during event delivery and routes do not bypass the queue.
   `setState` itself no longer touches the gate.
5. `SetEstablishedNow()` records the establishment timestamp.
6. Emit `session established` info log with local AS, peer AS.
7. Reset the peer-up barrier for this session.
8. Notify the reactor via `reactor.notifyPeerEstablished(p)` and
   `reactor.notifyPeerNegotiated(p, neg)`.
9. Call `startInitialRoutes(session)`. Unless the startup convergence
   hold owns this peer's initial routing update, it calls
   `spawnInitialSync(session)`, which starts the session's one sync
   goroutine (`runInitialSync`, per-session lifecycle, not per-event)
   through the session's start latch. A second start of the same
   session, or a start after it ended, spawns nothing. The runOnce
   session-end defer joins that goroutine before it clears
   `p.session`.

Before its queue drain the sync waits for the peer-up barrier
(`waitPeerUpBarrier`), which reports what ended the wait. Every barrier plugin
took delivery: the sync goes on. `peerUpBarrierTimeout` (2 seconds) passed on a
live session: the sync goes on and logs a warning that the End-of-RIB no longer
implies every plugin registered the peer. The session ended (`Session.Done`):
the sync clears its own session's gate and owed marker and returns, with no
warning, so the join in the session-end defer never waits out the timeout.
Operations still queued stay queued for the next session.

Once teardown seals the session, new API announcements enter the peer's queue
for the next session, even while the peer still reports Established. The seal
precedes NOTIFICATION publication, so a command that answers that event cannot
select the sealed session as its writer. A queued teardown clears its old,
unsent operations at the seal under the peer lock, before publishing that
notification; commands admitted afterward remain for the next session.
<!-- source: internal/component/bgp/reactor/peer.go -- queueGateClosedLocked -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- runQueuedTeardown -->

If the session seals after the API checked the gate, the ordinary announcement
rail puts the identifiable unsent suffix into that same queue. Completed UPDATE
chunks remain sent history, which the RIB restores on reconnect. If export
policy has already sent part of a transformed chunk, the request returns a
partial failure: the original unsent suffix is unknown and is not queued.
A sent-owner receipt or an initial replay retains its original session
authority and never becomes a new local announcement for a later session.
A queue refusal is reported as that refusal, rather than as a warning about
a family that is negotiated. A refusal while retaining the unsent suffix
keeps its original error identity and explains that some routes can already
be queued. It tells the operator to inspect `show bgp peer list` before
repeating the original announcement after reconnect.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- announceBatchToPeers, AnnounceNLRIBatch -->
<!-- source: internal/component/bgp/reactor/reactor_api_retired.go -- queueRetiredAnnounce -->

In production the wait nearly always returns at once. Step 8 delivers the
peer-up event to each barrier plugin and waits for its answer on the FSM
callback goroutine (`deliverPeerState`): an acknowledgement opens the barrier,
and a refusal lowers the expected count to what was acknowledged, which opens it
too. The sync, started in step 9, finds the barrier shut only when the
dispatcher could not find the peer to record a signal or the lowered count.

A peer-level teardown the sync takes from the queue once its session's Run has
returned, before the session-end defer clears `p.session`, is kept at the head
of the queue for the next session rather than sent to the session that ended
(`queuedTeardownEnds`, `sessionRunReturned`).
<!-- source: internal/component/bgp/reactor/peer.go — waitPeerUpBarrier, peerUpBarrierOutcome -->
<!-- source: internal/component/bgp/server/events.go — deliverPeerState -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go — sendInitialRoutes, queuedTeardownEnds, sessionRunReturned -->

### What the End-of-RIB marker orders, and what it does not

The End-of-RIB marker lets the receiver put off recomputing its routes until
the first table has arrived, so that a starting router does not recalculate on
every UPDATE. It has no bearing on what Ze forwards (owner decision,
2026-11-14).

The sync sends the marker once it has written Ze's own table for the session:
the static routes, the default routes, the operations queued for the peer and
the config-originated family routes. That is the one order Ze promises: its own
table, then the marker.

The marker waits for nobody else. A process that creates routes stands where a
peer stands (owner ruling, 2026-09-18): Ze does not hold its marker for a
peer's, so it does not hold it for a process's routes, for a route forwarded
from another peer, or for a plugin's replay of existing routes. Each of those
reaches the peer before or after the marker as timing decides, and forwarding
is never held back for the marker. RFC 4724 Section 4 requires the marker
"once it completes the initial routing update" and does not define that
update or say what may precede the marker.

What IS ordered around the sync is route correctness, not the marker. While
the session's queueing gate is closed, a forwarded UPDATE is parked behind the
peer's own queued operations (`forwardOrderHold`), and while a replay is owed,
a live change is parked behind the replayed routes (the replay fence), so a
withdrawal never overtakes the announcement it takes back.

The sync and a plugin's replay share one marker per family per session: the
first to claim the family sends it and the other stands down
(`claimInitialSyncEOR`). An End-of-RIB a process asks for explicitly is that
process's own marker and is written each time it asks, as ExaBGP does, during
the sync too: it waits in the peer's forward queue until the sync has drained
the operations queued for the peer. When it claims the family first, the sync
sends no marker of its own for that family. `behavior { manual-eor true }`
withholds both automatic markers, the sync's and a replay's, and never a
requested one (`announceEOR`).
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go — sendInitialRoutes -->
<!-- source: internal/component/bgp/reactor/peer.go — forwardOrderHold, raiseReplayFence -->

Established peer events and API snapshots report the TCP connection's local
address and both socket ports, including an automatically chosen local address
or ephemeral source port. Sent message events read an immutable endpoint
snapshot captured before OPEN, because those callbacks run under the write
lock and cannot safely acquire the session connection lock. Missing socket
ports remain zero; listener and dial-target configuration are never substituted.
<!-- source: internal/component/bgp/reactor/session_connection.go -- connectedTransport, connectionEstablished -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- establishedPeerInfo -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyMessageReceiver -->

Every `plugin.PeerInfo` the reactor builds (`runOnce`'s wire observer,
`establishedPeerInfo`, `notifyMessageReceiver`, the close, removal, congestion
and negotiated events) carries the peer's AS as `rpc.RemoteAS`, through
`rpc.RemoteASOf`. A dynamic peer's settings hold 0 until Established, and 0
becomes the not-yet-known state, which text writes as `remote as unknown` and
JSON as an absent `remote.as`. `notifyMessageReceiver` closes one window: the
KEEPALIVEs exchanged in OpenConfirm, after negotiation resolved the AS from the
peer's OPEN but before the settings learn it, name the negotiated AS. The
ingress policy chain and the raw-message AS are fed the plain number, which a
received UPDATE, arriving in Established, always has resolved.
<!-- source: pkg/plugin/rpc/remote_as.go -- RemoteAS, RemoteASOf -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- runOnce -->

### The startup convergence hold

`bgp update-delay max-delay <seconds>` arms a hold when the daemon starts
its peers. While the hold is armed, `startInitialRoutes` registers the
peer and its session with the hold and spawns nothing. When it releases,
the hold calls `spawnInitialSync` with the session it recorded for every
peer it registered; the latch starts nothing for a session that has
ended.

The hold releases on the first of three conditions: every EXPECTED peer
finishes its initial routing update, the optional `establish-wait`
deadline expires with at least one peer held, or `max-delay` expires. The
reactor logs the condition it released on, and `show bgp update-delay`
reports it.

The expected set is recorded BY IDENTITY when the hold is armed: the peers
`StartPeers` had in hand. A dynamic-group member reaches this same FSM
callback through `createDynamicPeer`, so it is HELD like any other peer
and is never COUNTED, because a count taken over the configured peers
would otherwise be satisfied by an inbound dynamic session.

"Finishes its initial routing update" means the peer's End-of-RIB marker
has arrived, counted per negotiated family where the reactor decodes the
inbound marker (`reactor_notify.go`). RFC 4724 Section 4.1 excludes two
kinds of peer from that wait by name, and `trackLocked` excludes the same
two: a peer that advertised no graceful-restart capability, and a peer
whose capability carries the Restart State bit. Each is settled at
Established.

A peer that leaves Established is dropped from both sets by
`updateDelayPeerDown`, so the condition reads "is Established and has
finished" rather than "has been Established".

Best-path selection is NOT deferred. The hold defers the initial routing
update and the End-of-RIB marker that closes it (owner decision,
2026-09-08).

Nothing else on the outbound path changes, and that is the design. Step
8 above already closed the initial-sync gate, so a held peer keeps the
three behaviors establishment gave it: `shouldQueue()` queues a route
operation, `forwardOrderHold()` parks a forwarded UPDATE in the
destination worker's overflow buffer, and no End-of-RIB is sent, because
the initial routing update has not completed. RFC 4724 Section 4.1
requires the marker once that update completes, so a marker sent from
under the hold would claim a completion that had not happened.

An absent `max-delay`, or `max-delay 0`, arms nothing and leaves this
sequence exactly as it reads above.

<!-- source: internal/component/bgp/reactor/update_delay.go — updateDelayHold, Peer.startInitialRoutes -->

**On transition *out of* `StateEstablished`:** the FSM callback runs the
leave arm (`leaveSession`, `peer_session_end.go`) on whichever goroutine drains
the FSM, and `runOnce`'s down (`sessionDown`) announces the down later, on the
`runOnce` goroutine. The leave arm:

1. Record what the close is derived from, in the leave record: the FSM event
   that made the transition, `notificationExchanged` (and the closing
   NOTIFICATION when it is set), the close reason the session recorded
   (`closeCause`) and the forward generation. The down derives the close
   from them (step 4 of the down).
2. Release a non-strict peer's BFD session without waiting for its
   subscriber goroutine, which may be the goroutine running the arm.
3. `p.announceClosed(reactor, session)`: mark the session closing
   under the reactor lock, so a reload attaching a process at the same moment
   tells that process down itself (`docs/architecture/core-design.md`, section
   18a), and forget the peer in the reactor (`forgetClosedPeer`: AIGP, update
   groups).
4. Clear negotiated capabilities and encoding contexts.
5. `setState(PeerStateConnecting)`, build the history row (the down sets its reason), and close the
   leave signal.

The down, on every exit from `runOnce`:

1. When the FSM still reads Established, fire the leave event so the leave arm
   runs: Event 18, or the stop event of a teardown that began
   (`Session.endEvent`).
2. Close the delivery channel and wait for the delivery worker, so every
   UPDATE the session buffered is published before the down.
3. When the FSM ever entered Established, wait on the leave signal, then on
   the BFD subscriber goroutine.
4. Derive the close once with `sessionCloseFor` (`peer_close.go`), from the
   leave record and from whether the session's receive publication failed
   (`receivePublication.failed`), which is final only now that the delivery
   worker exited. The reason is `rpc.ReasonNotification` when a NOTIFICATION
   was sent or received, `rpc.ReasonAdjRIBInInvalid` when none was but some
   received UPDATE could not be published (the Cease the delivery worker's
   teardown built was not written, or the session had already ended),
   `rpc.ReasonTCPFailure` for Event 18 without either, and
   `rpc.ReasonLocalClose` for any other event without either. The GR-aware
   congestion teardown (`forward_pool_congestion.go`) fires Event 18 after
   recording `ErrCongestionTeardown`: ze closed that transport itself, so its
   close is `rpc.ReasonLocalClose` carrying Event 18. The RFC 4271 Section 8.1
   number of the event travels beside it, to the plugins, as
   `StructuredEvent.FSMEvent`. Graceful Restart retains routes for every
   reason `rpc.ReasonForbidsRetention` allows, so neither after
   `rpc.ReasonNotification` (RFC 4724 Section 4) nor after
   `rpc.ReasonAdjRIBInInvalid`, and BMP writes Peer Down reason 4 for
   `rpc.ReasonTCPFailure` and reason 2 with the event code for
   `rpc.ReasonLocalClose` and `rpc.ReasonAdjRIBInInvalid` (RFC 7854 Section
   4.9).
5. If nothing else reported the close (`sessionDroppedOwed`), raise
   `session-dropped` on the report bus via `raiseSessionDropped`. Two
   closes owe none, because the operator already sees their event: one
   where a NOTIFICATION was exchanged (`notificationExchanged == true`),
   and one whose recorded close reason is a write budget expiry
   (`ErrWriteBudgetExpired`), which the session raised as
   `doctor-bgp-peer-not-reading` when the write failed.
6. Notify the reactor via `reactor.notifyPeerClosed(p, closed)`, which tells
   the observers and then schedules a dynamic peer's cleanup.
7. Log `session closed` with the reason and append the history row, which
   carries the same reason.

A delivery worker that cannot publish a batch tears its session down
(`failReceivePublication`), so the session does not stay Established while its
UPDATEs never reach the plugins. A recovered receiver panic (task `delivery`)
sends Cease / Unspecific, because a Ze defect has no defined subcode; no
receiver sends Cease / Out of Resources.

<!-- source: internal/component/bgp/reactor/peer_session_end.go — leaveSession, sessionDown, sessionLeave.close, failReceivePublication -->
<!-- source: internal/component/bgp/reactor/peer_close.go — sessionCloseFor -->
<!-- source: internal/component/bgp/reactor/peer_run.go — SetCallback Established and leave-Established branches -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go — spawnInitialSync, runInitialSync -->

## Reconnect backoff

| Scenario | Delay |
|----------|-------|
| Normal session error | Exponential base, starting at `reconnectMin` (default 5s), doubling each attempt, capped at `reconnectMax` (default 60s). Each wait samples uniform jitter from 0.75 to 1.0 of that base. |
| TCP fails in Active, or in Connect while DelayOpen holds its socket | The actual restarted FSM ConnectRetryTimer supplies an immutable budget holding its sampled deadline and owning clock. This includes Active strict BFD pending after DelayOpen was cleared. `runOnce` carries it in `connectionRetryFailure`; the Peer reads its remaining duration at the wait, so cleanup time is deducted and an already due retry waits zero. The Peer clock supplies the wait channel. This wait does not grow the ordinary backoff. |
| Session error after a stable session | The cycle reached Established and stayed there at least `reconnectMax`, measured from `EstablishedAt()` on the peer clock: `delay` restarts at `reconnectMin` and `prefixTeardownCount` at 0, then the error takes its own row. A cycle that never reached Established restarts nothing, however long it lasted, and a session shorter than `reconnectMax` keeps the doubling. |
| `ErrTeardown` (API-initiated stop) | Reset `delay` to `reconnectMin`, continue loop immediately. No wait. `prefixTeardownCount` is kept, unless the torn-down session was stable (row above). |
| Cycle returns nil (err == nil) | Reset `delay` to `reconnectMin` and `prefixTeardownCount` to 0. This arm runs only when `runOnce` published no session (the peer is stopping or administratively down). |
| Administrative hold or wake | Reset `delay` to `reconnectMin`: `Disable` parks the loop and `Enable` retries at once. |
| Prefix teardown, family mode `never` (the default) | No delay, because there is no next attempt. The peer enters `PeerStateIdleHold`, raises a `prefix-hold` warning, refuses inbound connections and waits for the peer context to end. Only an operator recreating the peer brings it back. |
| Prefix teardown, family mode `timer` (`idle-timeout > 0`) | Separate backoff: `PrefixIdleTimeout * 2^(teardownCount-1)`, capped at 1 hour. `prefixTeardownCount` capped at 60 to prevent `time.Duration` overflow. Per RFC 4486. |
| Prefix teardown, family mode `backoff` | The normal exponential backoff of the first row. This is the explicit opt-in, and it was the accidental behavior of every zero `idle-timeout` until 2026-08-03. |
| Inbound connection arrives during backoff | Reset `delay` to `reconnectMin` and continue immediately. `inboundNotify` is a buffered channel (size 1) signaled by `setInboundConnection`. |

<!-- source: internal/component/bgp/reactor/peer_run.go — run backoff arms, cycleStayedEstablished -->
<!-- source: internal/component/bgp/reactor/peer_stats.go — setEstablishedNow, EstablishedAt -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — setInboundConnection signals inboundNotify -->

The stable-session rule follows BIRD's intent and not its measure. BIRD's
`bgp_update_startup_delay` (`proto/bgp/bgp.c`) forgets the delay when the
previous error is at least `error forget time` old, and doubles it otherwise.
Ze measures the time the ended session spent in Established instead. Ze grows
the delay on a failed dial, so at the cap two errors are already about one
maximum apart, and BIRD's measure would restart the delay of a peer that never
came up. FRR resets its start timer on every entry to Established
(`bgp_establish`, `bgpd/bgp_fsm.c`); Ze does not, so a peer that flaps every
few seconds stays damped. RFC 4271 Section 8.2.2 leaves the method to the
implementation.

**Difference from BIRD: which session ends grow the delay.** Ze doubles the
delay after every error that waits, a received Cease and a TCP failure
included. BIRD does not grow it on either: `bgp_rx_notification`
(`proto/bgp/packets.c`) skips `bgp_update_startup_delay` for error code 6
(Cease), and `bgp_sock_err` (`proto/bgp/bgp.c`) never calls it. An operator
moving from BIRD sees Ze wait longer to redial after the far end resets the
session with a Cease or the TCP session drops. An inbound connection from the
peer is still accepted at once and resets the delay.

<!-- source: internal/component/bgp/reactor/peer_run.go — run, cycleStayedEstablished -->

## Inbound connection handling for passive peers

Passive peers do not dial. They wait for inbound connections dispatched
by the reactor's accept loop. A known static peer is attributed by remote
address on a shared socket. Dynamic members also qualify the actual receiving
local address and listen port before longest-prefix group selection, creation
or reuse. This check runs again under the creation lock, so a concurrent
creation cannot admit a different local socket through the same remote key.
An ineligible connection closes without publishing or counting a member.
An auto-local dynamic template whose local port is default uses the existing
global listener's actual bound port. This also applies to the supported
`Config.ListenAddr` form when `:0` lets the operating system assign the port.
An explicit custom port or configured local address retains its own listener
identity. The member keeps its configured fields; the live socket supplies the
actual connected transport facts.
<!-- source: internal/component/bgp/reactor/reactor_connection.go -- peerForConnection -->
<!-- source: internal/component/bgp/reactor/reactor_dynamic.go -- tryCreateDynamicPeerAt, dynamicPeerAcceptsPort, tryCreateDynamicPeerEligible -->

Two race conditions need handling:

1. **Remote reconnects faster than our backoff.** The peer is in the
   `delay` select after a session failure. The reactor's accept loop
   calls `setInboundConnection(conn)`, which stores the connection and
   signals `inboundNotify`. The run loop's select wakes early, resets
   `delay`, and starts a fresh attempt. The new `runOnce` calls
   `takeInboundConnection()` before any outgoing dial and passes the stored
   connection to `session.Accept`.
2. **Stale buffered connection.** If `Accept` fails (for example, the
   peer has already torn down the half-open connection while it sat in
   the buffer), `runOnce` closes the stale connection quietly and
   returns an error. The normal backoff path applies.

<!-- source: internal/component/bgp/reactor/peer_connection.go — setInboundConnection, takeInboundConnection -->
<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce takeInboundConnection path -->

### When a passive peer gets its listener

The start opens one listener per configured pair in `startMultiListeners`,
then records that the listeners have started (`Reactor.listenersStarted`).
`stopAllListeners` clears that record, and both the stop (`cleanup`) and an
aborted start (`abortStartup`) call it. `AddPeer` opens a listener for a
passive peer with a local address only while the record holds and the reactor
is not stopping:

| When `AddPeer` runs | Listener |
|---------------------|----------|
| During the config load, before the start | None from `AddPeer`: `startMultiListeners` opens it |
| After the listener pass, before the peer start barrier opens (the standalone start's plugin waits and announce delay, or borrow mode before `StartPeers`) | Opened now |
| While running | Opened now |
| After a stop or an aborted start | None |

When the pair has no listener, one is started. When the pair already has one
and the new peer carries an MD5 key, the listener is rebuilt, because the key
set is read only when the socket is created (`newListenerFactory`). When the
listener cannot start, the peer is removed again and `AddPeer` returns the
`listen on <address>:<port>` error. The peer itself starts only once the
reactor is running and the barrier has opened (`peersStarted`). Until then a
connection the listener accepts waits in the peer's inbound slot: the peer
has no session, so `acceptOrReject` stores it with `setInboundConnection`, and
only the peer's own run loop takes it.

<!-- source: internal/component/bgp/reactor/reactor.go — startMultiListeners, stopAllListeners, abortStartup, cleanup, newListenerFactory, Reactor.listenersStarted -->
<!-- source: internal/component/bgp/reactor/reactor_peers.go — addPeerLocked listener block -->
<!-- source: internal/component/bgp/reactor/reactor_connection.go — acceptOrReject -->

## Collision resolution (RFC 4271 §6.8)

An admitted incoming connection has its own Session and FSM, even while the peer already
has an outgoing connection. `setPendingConnection` reserves one pending slot;
`handlePendingCollision` constructs that connection through `Peer.newIncomingSession`
and starts its local OPEN lifecycle. With DelayOpen zero it sends its own OPEN
before waiting for the remote OPEN. With DelayOpen enabled it owns an independent
timer and proposal. A remote OPEN or that timer's expiry releases its local OPEN.
<!-- source: internal/component/bgp/reactor/incoming_candidate.go -- handlePendingCollision -->
<!-- source: internal/component/bgp/reactor/peer_session_factory.go -- newIncomingSession -->

Admission is bounded to one pending incoming exchange per peer. Additional
arrivals during that exchange, its joined refusal or a reserved handoff are
refused before a Session is constructed. This is a capacity policy. RFC 4271
Section 8 makes the FSM conceptual and requires the same externally visible
behavior; a refused socket's lack of a Session is not itself a conformance gap.
The admitted-connection controls check its OPEN exchange and failure isolation.
An excess arrival while the pending exchange or its joined retirement owns
capacity receives Cease / Connection Rejected (6/5), then closes, before OPEN.
This admission refusal is distinct from the OPEN-based collision loser below.
Reserved-handoff and shutdown closures remain silent.
<!-- source: internal/component/bgp/reactor/peer_connection.go -- setPendingConnection -->
<!-- source: internal/component/bgp/reactor/reactor_connection.go -- acceptOrReject -->

The incoming candidate does not publish a second peer-up event. If the existing
session is OpenConfirm, collision resolution compares identifiers under Section
6.8. If it is Established, the completed incoming OPEN causes rejection with
Cease / Connection Collision, and the established session remains in place.
An incomplete or failed incoming OPEN cannot retire that established session.

An OpenSent incumbent whose remote identifier is unknown outside the protocol
is retained while the independent incoming handshake progresses. The incoming
connection can reach Established without receiving an OPEN on the outgoing
connection. Its selection then retires an incumbent still in OpenSent with
Cease / Connection Collision. If the incumbent progressed meanwhile, the
handoff rechecks the OpenConfirm or Established rule. This follows BIRD's
`bgp_rx_open` handling of another OpenSent connection; it does not add FRR's
earlier identifier comparison to that state.

A pending OPEN whose outer parameter framing cannot be decoded receives OPEN
Message Error (2/0), and only that pending connection closes. A correctly
decoded OPEN that loses collision resolution retains Cease / Connection
Collision (6/7).
<!-- source: internal/component/bgp/reactor/incoming_candidate.go -- handlePendingCollision, resolveIncoming -->
<!-- source: internal/component/bgp/reactor/reactor_connection.go -- rejectPendingConnection -->

If the rejection's write deadline cannot be set or the rejection write fails,
Ze still closes only the pending connection. One warning and a
`doctor-bgp-pending-refusal-undelivered` event name the endpoint, failed
operation and cause. The daemon's `show doctor` and `show errors` retain the
event; `ze show bgp peer list` is the next inspection step. A delivered rejection
raises no delivery-failure event.
<!-- source: internal/component/bgp/reactor/pending_refusal_diagnostic.go -- reportPendingRefusalFailure -->
<!-- source: internal/component/bgp/reactor/register_diagnostic.go -- init -->

Connect and Active use the same bounded slot while DelayOpen holds an OPEN-less
incumbent. An incoming OPEN that wins there closes the held loser immediately,
with Event 23 and no NOTIFICATION. The incoming candidate retains its own local
OPEN state; it is not installed in a second fresh Session. The first-OPEN reader
has a ReceiveHoldTime deadline (90 seconds if configured hold is zero). Expiry
closes only that candidate. Its read deadline is cleared after a complete OPEN;
the candidate's original protocol timers continue to bound the handshake.

Selection stores a typed inbound receipt and signals `inboundNotify` before
closing the incumbent. The Peer loop first completes incumbent cleanup, then
joins the candidate's previous reader and cancellation bridge and adopts that
same Session. Its buffered reader, original proposal, FSM and timer ownership
survive the transfer. A buffered OPEN is processed once; an already processed
handshake publishes its negotiated and established effects once. Adoption sends
no duplicate OPEN or KEEPALIVE. Wire observers retain the actual original bytes.

After incumbent cleanup, publishing the selected Session also publishes its
validated remote Identifier from either candidate receipt. This includes a
candidate whose OPEN still needs advancement: it keeps the original validated
receipt rather than validating again.
<!-- source: internal/component/bgp/reactor/peer_run.go -- runOnce -->
<!-- source: internal/component/bgp/reactor/incoming_candidate.go -- candidateOpen, candidateReady -->

The shared Session factory installs all writer and timer callbacks and route
inputs. Each connection owner installs its optional capture input before TCP.
These inputs stay
unchanged through adoption, so periodic KEEPALIVE and SendHold actions cannot
race a replacement callback. Selection-sensitive notification hooks preserve the incumbent's facts, and
timer-display hooks publish for the selected connection; OPEN wire counters
include admitted candidates too. Adoption changes only the FSM publication callback,
under a short, scoped `openMu` hold after delivery and its down finalizer exist.
It replays publication effects without replaying FSM events or resetting timers.
An incoming FSM that expired before adoption is retired instead of restarted;
its private establishment does not announce a peer-up or await an unpublished
down. Its failed attempt still updates the peer-owned ConnectRetryCounter.

Protocol capture records a candidate's original input before selection, using
the existing bounded queue and configuration-event registration. Its worker
waits without owning a file. After incumbent cleanup has joined the old capture,
selection permits the same worker to open the next file and drain that input,
retaining the original header and timestamps. Closing a selected input drains
it even if its worker has not yet processed the selection; closing an
unpublished input discards it without touching the incumbent's file. The
capture sink is never replaced, and the single-file and one-backup disk budget
is unchanged. The initial candidate OPEN crosses the normal Session tee once.

Capture admission is also scoped to this exact Peer's reactor membership.
The shared removal operation deletes the Peer and joins its registered
capture workers under the same lock that admits factory registration. A
factory that started earlier cannot register after that cut. API removal,
reload, dynamic removal and rollback therefore release the capture namespace
before any replacement Peer at the address can publish its own input.
<!-- source: internal/component/bgp/reactor/peer_removal.go -- removalLocked -->
<!-- source: internal/component/bgp/reactor/capture_pending.go -- protocolCapture, pendingSessionCapture, captureSelection, prepareSessionCapture -->
<!-- source: internal/component/bgp/reactor/capture_replay.go -- sessionCapture -->

The inbound variants distinguish an ordinary socket, an externally buffered
OPEN, a candidate with an unprocessed OPEN and a candidate whose handshake has
progressed. Their common socket accessor never reconstructs a connection. The
single pending slot and single inbound slot each own at most one socket and an
OPEN buffer of at most 4096 octets. A rejected candidate changes to a closing
receipt before its NOTIFICATION is written. It no longer reports an active
pending exchange, but the same admission remains occupied until its socket,
bridge, timers, identifier claim and capture have been retired. A blocked
refusal cannot admit further candidate Sessions. Only then is the gate released
for the next arrival. `collisionHandoff`, guarded by the Peer
mutex, reserves the selected Session from selection through installation.
Taking the slot does not release that reservation. A third arrival closes
silently before OPEN and cannot occupy it, including while the selected
candidate is already OpenSent. A handoff whose recheck rejects the candidate
takes back only its own still-owned receipt. Shutdown seals incoming admission, closes owned
sockets and joins the incoming reader, cancellation bridge and handoff workers
before clearing peer statistics or releasing remaining identifier claims.

<!-- source: internal/component/bgp/reactor/incoming_candidate.go -- inboundConnection, incomingCandidate, handlePendingCollision, resolveIncoming, finishIncomingHandoff -->
<!-- source: internal/component/bgp/reactor/peer_session_factory.go -- newSession -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — setPendingConnection, storeInboundLocked, takeInboundConnection -->
<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce, cleanup -->
<!-- source: internal/component/bgp/reactor/session_delay_open.go -- closeHeldForCollision -->
<!-- source: internal/component/bgp/reactor/peer_connection.go -- finishCollisionHandoff -->

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

A received NOTIFICATION waits for the earlier accepted UPDATEs to reach the
observer queues before its synchronous callback runs. This terminal-only wait
uses the session's receive-publication frontier and the peer context. KEEPALIVE
stays synchronous without this wait, so UPDATE delivery does not delay its
HoldTimer restart. If publication fails or the wait is canceled, Ze skips only
the NOTIFICATION observer event. The session still processes the NOTIFICATION,
logs its decoded reason, stops its timers, closes the socket, and reaches Idle.
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyOneMessage -->
<!-- source: internal/component/bgp/reactor/session_read.go -- processMessage -->
<!-- source: internal/component/bgp/reactor/session_handlers.go -- handleNotification -->

`doctor-bgp-received-notification-skipped` records that historical observer gap
in the bounded report bus and at WARN. It does not claim that UPDATE delivery
failed: cancellation can race a completed frontier. Run `ze show errors` to
inspect the cause retained with the event.
<!-- source: internal/component/bgp/reactor/received_notification_diagnostic.go -- reportReceivedNotificationSkipped -->

### Captured-session admission stops

`Session.PauseAdmission(ctx, reserve)` returns an owned receipt only after no
semantic admission is active. Its claim closes the gate before policy evaluation
and publication in both readers, including coalesced flushes and standalone
`ReadAndProcess`. The admission mutex is released before invoking callbacks;
already-admitted asynchronous delivery and forwarding can drain independently.
The receipt does **not** prove that those downstream consumers have applied their
work: their coordinator still needs publication, application and writer receipts.

An idle or partial socket read does not delay acknowledgement. Admission does not
interrupt `ReadFull` or install a read deadline: partial header/body bytes remain
in the existing reader and pool buffer. A complete packet waits before semantic
processing, alongside at most the existing bounded coalesced batch. Release
continues that same processing once, without copying to a new queue, discarding
messages, closing TCP, or changing negotiated capabilities.

The caller MUST supply a transaction deadline and a positive abort/resume
reserve. `Deadline()` is the work deadline: the earlier of that deadline and the
actual armed HoldTimer expiry, minus the reserve. `CompletionDeadline()` exposes
the captured hard bound **before** that subtraction, for bounded compensation,
finalization and resumption. Neither bound moves when ordinary traffic restarts
the HoldTimer. `Check()` judges the work deadline and remaining reserve;
`CheckCompletion()` permits cleanup to use the reserve, but still refuses
session loss, a released receipt, an expired/unarmed timer, or the hard deadline.
A disabled HoldTimer uses the transaction deadline, not an unlimited budget.

Acquisition cancellation releases only its own claim. A successful receipt MUST
be checked before mutation and explicitly released after compensation or
finalization, including failed cleanup. Expiry reports failure; it never
automatically resumes tentative policy. The HoldTimer still expires normally
and outgoing KEEPALIVEs continue.

`Pause`/`Resume` retain their idempotent public backpressure claim, without
acknowledging in-flight processing. `Resume` cannot release an admission receipt;
each receipt releases only its own claim on the captured `Session`, never a
replacement found through a peer lookup. Real session loss wakes the gates and
refuses subsequent receipt checks without manufacturing a DOWN/UP transition.
Terminal completion wakes both running and standalone readers. `closeConn` wakes
the gates after socket retirement, as its last step, with no session lock held; the
writer-retirement path does the same after closing the failed transport. The
HoldTimer path wakes them after its NOTIFICATION attempt and terminal FSM event.
Close-reason publication is not terminal completion: even if all pause owners
release while a NOTIFICATION is pending, both reader gates wait for completion
before returning the stored cause. Otherwise a reader could close the socket
before the teardown producer sends its notification. Receipt checks may report
the impending loss to the coordinator, but cannot turn that report into a
premature reader exit.

### In-place import invalidation (RFC 8212 D-2)

A reload that removes an Established session's last accepting import policy
keeps the session. `Peer.importDecisionFalls` compares the effective
import decision before and after the swap (`filterapi.Policy.AcceptsAtEnd`
on what the swap publishes), whatever the session type, so a change that keeps
the same decision triggers nothing. A candidate the operation could only build
from its config leaves (no config file to resolve it from) carries no filter
chain, so its decision is unknown: a swap that would deliver it over an
accepting policy is refused, never guessed.

Establishment joins the swap's boundary. `swapImportFalling` takes the peer's
`establishGate` and reads its state: a peer that is not Established admits
nothing, so the rejecting policy is published under the gate, with no epoch
change and no owner call, and `enterEstablished` (the FSM's Established
transition) waits for the gate before the first UPDATE is read. Such a peer can
still have paths its previous session admitted: Graceful Restart and LLGR keep
them in bgp-rib, stale and eligible. Once the policy is published,
`purgeRetained` runs bgp-rib's `request bgp rib purge-stale <peer>`, which
deletes them, withdraws every copy sent and re-elects, as the restart timer's
expiry would; with no RIB command registered nothing is retained. The purge
is not undone by a rollback: the peer's next session sends those routes again.
An Established
peer takes the coordinator (`source_policy_reload.go`), which runs inside the
BGP operation journal:

| Step | When | What happens |
|------|------|--------------|
| Prepare | before the journal apply, nothing published | the source-policy owners are captured from the delivery graph (the processes granted the peer's received UPDATEs), in their registered dependency order with ties broken by name; a capture with a forwarder (an owner declaring `FencesLiveForwards`) but without the process holding the sent inventory (the RIB recovery owner) is refused, because nobody would withdraw the copies the forwarder sent; `PauseAdmission` with a one-second abort/resume reserve; the receive-publication cut is awaited; a session replaced meanwhile refuses the change (it holds no pause); each owner gets `prepare` with the captured peer address, peer owner, session generation, policy epoch, receive frontier and template generation |
| Apply | journal apply | the settings are published, the epoch advances (E1) and the source's invalidation window opens under it, every Established destination's writer is joined (pool barrier then `writeMu`), each owner in turn gets `apply` with E1, with the validation-change subscribers drained after each, then the downstream settles (below) |
| Commit check | operation commit | `Reactor.CheckSourcePolicyTx` refuses a commit whose receipt work deadline has passed, so the transaction rolls back |
| Abort before apply | a failed owner prepare, or apply refused before it advanced the epoch (budget exhausted, publish failed) | the previous settings are restored if any were published; the epoch does not move and no window opens, so an owner's re-send stamped with the unchanged epoch stays current; each prepared owner, and the owner whose prepare failed (its call may have timed out after it pinned), gets `abort` with target zero and only releases its pins; an owner that never prepared answers it as done; nothing downstream to settle |
| Abort | journal undo after apply advanced the epoch: a later apply failure, a rolled-back commit, or outer compensation | the previous settings are restored, the epoch advances again (E2) and the window records that this abort issued it, writers are joined, each prepared owner in turn gets `abort` with E2, with the validation-change subscribers drained after each, then the downstream settles (below); input stays paused and the receipt is kept, because the rest of the rollback (section-owned filter definitions included) has not run yet |
| Resume after abort | `config-complete` rejected for the transaction, sent after the whole rollback | the receipt is released, so the first resumed UPDATE is evaluated under the restored config. A rollback inside the transaction gets this receipt from `runTxCoordinator` once `Execute` returns; an outer rejection gets it from `restoreReload` after the section restore. If none arrives by the receipt's completion deadline, a timer releases the pause, logs the expiry, and the late receipt reports it |
| Undo on rejection | `config-complete` rejected while an operation journal of the transaction is still recorded | the BGP plugin rolls those journals back first, last operation first (`completeBGPTransaction`), so each invalidation is aborted with its settings restore before the receipt below resumes input; no journal outlives the receipt |
| Rejection without undo | `config-complete` rejected for an invalidation no journal undid | the published import policy decides (`rejectNotUndone`): while it still rejects, the owners get `finalize` and the receipt reports that the rejected transaction's policy stays in effect, because restoring eligibility would admit the routes under a rejecting policy; only an accepting published policy lets the abort restore them |
| Finalize | `config-complete` accepted | each owner gets `finalize`, bounded by the receipt's own deadline and the session's live HoldTimer expiry, never by the operation's apply deadline, which a slow accepted reload has passed; then the receipt is released, which closes the window and drops its released receipts |

**Downstream settle.** An owner recomputes best paths before it answers
`apply` or `abort`, and publishes each change as a RIB best-change and a
redistribution route change; `bgp-adj-rib-in` publishes an eligibility change
as a validation change, which `bgp-rib` answers with best changes. A subscriber
running as a plugin process receives those events queued and may write to
destinations in answer, so after the owners the coordinator drains every
process subscribed to the three events, validation changes first
(`DrainEventSubscribers`, `process-protocol.md`), then joins the destination
writers again. It does not wait for a recovery round a destination owes: the
round writes later, and the epoch fences refuse any write it admits under a
superseded import policy. A drain or join that misses the deadline fails the apply, which
then aborts, or reports the abort as incomplete compensation. Engine-side
subscribers write inside the emit, before the owner answers, so the join alone
settles them.

The receipt is released exactly once, on every path, failure included. A
failed owner call during apply aborts at once; a failed abort is reported as
"compensation incomplete" naming the owner, never as a rollback, and the
advanced epoch stays. Owners own the destination writes for invalidation and
restore, and among them `bgp-rib` alone writes: its sent inventory holds the
copies the forwarders sent, which only settle their own pending work. The only adj-out work of the coordinator is bookkeeping: a release
closes the window, then drops the receipts destinations kept for the abort
restore (`docs/architecture/bgp/structural-forwarding.md`, "Receive validation and cache changes"), and writes
nothing to the wire. A second
transaction that touches a peer whose invalidation is still open is refused.
The BGP decomposer routes every change to an import layer a peer sees (the
bgp level, its group, a grouped or top-level peer's own `filter import`) as a
modify-peer operation, so the change runs under a transaction. The rest of
the same commit (another group's hold-time, the `router-id`, a dynamic group) reaches the reactor as one
`reconcile-peers` operation ordered after those modify-peers, under its own
journal, so a mixed commit applies whole and rolls back whole
(`transaction-protocol.md`, "Decomposition ownership"). The
section-path journal (`ReconcilePeersWithJournal`, internal
Reload/ApplyConfigDiff) carries no transaction, so a change that still reaches
it is refused there before anything is published, with an error saying to
commit the import change on its own and to report a repeat as a defect.

A dynamic group's template edit takes the same transitions through
`swapPeerJournaled`. The dynamic step judges each swap's refusals while it
plans under the reactor lock (`refuseImportSwapEarly`), before it removes or
restarts any dynamic peer, so a refused step tears nothing down; the apply
judges again.

The D-1 relaxation's undo (`undoImportRelaxation`) is this invalidation run
from the accepting candidate back to the rejecting policy, marked
`relaxationUndo`. Its apply publishes the rejecting policy, so the receipt
finalizes it whether the transaction was accepted or rejected, and the hold
bounds its pause like an abort's. An expired hold finalizes the owners too,
before it releases the pause and under its own one-second bound, since the
completion deadline it fires at has passed; the receipt and the expiry each
claim the end before calling an owner, so only one of them finalizes and
reports. A failure after the rejecting policy was published keeps that policy
and the denial instead of aborting, because an abort would make the paths the
candidate admitted eligible again under a policy that rejects them
(`completeExclusion`): every owner from the one that failed on gets `apply`
again under the same epoch (an owner whose first apply landed answers with its
recorded outcome), and one that still refuses never excluded the paths, so it
cannot be finalized (its ledger admits `finalize` only after `apply`): it gets
`abort` with target zero, which releases its pins, and the error names it,
says its routes stay eligible, and gives `request peer <address> disable`,
then `enable`. An owner whose ledger refuses that abort recorded its apply
with an error, as `bgp-rr` and `bgp-rs` record a downstream settle that
failed (its ledger refuses a zero-target abort once apply is recorded), or
applied late: it is kept for finalize, and the error says it withdrew the
routes but a copy may stay advertised downstream, with the same sequence.
When every owner applied and downstream settled, the undo
reports nothing and only logs the first failure. A failure to publish the
rejecting policy aborts, leaves the accepting candidate in effect and says so
with the same sequence, and its expiry says the same and finalizes nothing. A
peer whose session ended while the relaxation's ROUTE-REFRESH failed, or while
the undo prepared, has nothing to pause, so the undo judges the state again
under `establishGate`, restores the rejecting policy there, so a session that
establishes meanwhile waits for it, and purges what Graceful Restart
retained, as for a peer that is not Established.

<!-- source: internal/component/bgp/reactor/source_policy_reload.go -- importDecisionFalls, enterEstablished, swapImportFalling, sourceInvalidation.finalizeDeadline, prepareImportInvalidation, sourceInvalidation.apply, sourceInvalidation.undo, sourceInvalidation.abort, sourceInvalidation.holdUntilCompletion, sourceInvalidation.expire, sourceInvalidation.finalize, CheckSourcePolicyTx, CompleteSourcePolicyTx -->
<!-- source: internal/component/bgp/reactor/source_policy_reload.go -- sourceInvalidation.fail, sourceInvalidation.completeExclusion, sourceInvalidation.claimEnd, sourceInvalidation.finalizeOwners -->
<!-- source: internal/component/bgp/reactor/source_policy_relax.go -- undoImportRelaxation, relaxationUndoFailed, refuseImportSwapEarly -->
<!-- source: internal/component/bgp/reactor/reactor_dynamic.go -- planDynamicGroupsLocked -->
<!-- source: internal/component/bgp/reactor/source_policy_reload.go -- sourceInvalidation.settleDownstream, joinSourceWriters -->
<!-- source: internal/component/plugin/server/source_policy.go -- DrainEventSubscribers -->
<!-- source: internal/component/bgp/plugin/operation.go -- bgpImportLayerChanges, collectBGPImportLayers, bgpResidualOperation, completeBGPTransaction -->
<!-- source: internal/component/plugin/server/reload_tx.go -- completeRolledBack, sendTransactionReceipt -->
<!-- source: internal/component/bgp/reactor/peer_settings_apply.go -- swapImportInvalidating -->

<!-- source: internal/component/bgp/reactor/session_flow.go -- PauseAdmission, AdmissionReceipt.Check, AdmissionReceipt.CheckCompletion, AdmissionReceipt.CompletionDeadline, AdmissionReceipt.Release, beginAdmission, endAdmission, stopAdmission -->
<!-- source: internal/component/bgp/reactor/session_read.go -- readAndProcessMessage -->
<!-- source: internal/component/bgp/reactor/session_coalesce.go -- readAndProcessCoalesced -->
<!-- source: internal/component/bgp/reactor/session.go -- NewSession, Run -->
<!-- source: internal/component/bgp/reactor/session_connection.go -- closeConn -->
<!-- source: internal/component/bgp/reactor/session_write.go -- retireWrite -->

## `cleanup()`

Runs when the peer goroutine exits (context cancelled or run loop
returned). Invoked via `defer p.cleanup()` in `run()`.
The selected session's run loop and `runOnce` cleanup have already joined;
`p.session` is nil here.

Steps:

1. Clear negotiated capabilities, the prefix-hold warning and encoding contexts,
   then stop the BFD client.
2. Under `p.mu`, seal incoming admission with `incomingClosed`, take and clear
   pending and inbound ownership, and clear the collision reservation and `cancel`.
3. Close the pending socket and discard the inbound receipt, releasing its
   retained connection and OPEN resources.
4. Join incoming work with `incomingWG.Wait()`.
5. Release all router-ID claims owned by the peer, then clear stats via `ClearStats()`.
6. `setState(PeerStateStopped)`.

<!-- source: internal/component/bgp/reactor/peer_run.go — cleanup -->

The fatal-length regression drives the real peer run loop and registered RIB,
Adj-RIB-In, and route-server consumers, not a test-authored DOWN event. It
requires actual storage deletion and recipient TCP withdrawals as well as
session, timer, and context release. Its valid-length control waits for changed
MED in storage and on recipient TCP before checking that resources survive.
These are final-state and wire observables, not proof of deletion/withdrawal
temporal ordering.
<!-- source: internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go -- TestRFC8654FatalLengthReleasesInstalledRoutes, TestRFC8654ValidLengthRetainsInstalledRoutes -->

### Standalone session fixtures

A test that drives `Session.ReadAndProcess` without `Session.Run` owns the
timers as well as the socket. Closing the raw pipe does not run the session's
teardown: the periodic keepalive and RFC 9687 send-hold callbacks can otherwise
survive into later tests. `acceptWithReader` registers cleanup before accepting;
cleanup stops the FSM timers and send-hold timer, closes both pipe ends to
unblock pending I/O, and clears the session connection. Collision fixtures use
the same acceptance helper.
Direct-accept migration fixtures stop both timer owners too; a stopped FSM
HoldTimer does not stop the session-owned SendHoldTimer.

A fixture that starts `Run` MUST cancel and join that invocation before
releasing test-owned observers. The RFC 9687 fixtures wait for their run
goroutine before restoring the package logger, without consuming the result
channel used by the protocol assertions. Their fake clock runs timer callbacks
synchronously; no timer advance remains in flight at cleanup.

<!-- source: internal/component/bgp/reactor/session_test.go -- acceptWithReader, TestEstablishedSessionFixtureStopsTimers -->
<!-- source: internal/component/bgp/reactor/collision_test.go -- collisionAcceptWithReader -->
<!-- source: internal/component/bgp/reactor/rfc7705_session_as_migration_test.go -- migrationSession -->
<!-- source: internal/component/bgp/reactor/rfc8050_migration_epoch_test.go -- TestMRTMigrationEpochUsesActualLocalOPEN -->
<!-- source: internal/component/bgp/reactor/rfc9687_test.go -- rfc9687Run -->

## RFC 4271 ConnectRetryTimer replacement

RFC 4271 specifies a `ConnectRetryTimer` (mandatory FSM attribute,
Section 8.1.3, Event 9) with a suggested default of 120 seconds. The
RFC expects the FSM to sit in `Connect` or `Active` state waiting for
both TCP completion and the retry timer to fire, retrying the dial on
timer expiry.

Ze schedules reconnects at the Peer level, with two policies:

- Outgoing dials use a blocking `DialContext`, so the FSM never sits in
  `Connect` waiting for asynchronous TCP events. After TCP success a configured
  DelayOpen holds the socket in Connect; without it the Session enters OpenSent.
- Reconnect delay lives at the **peer level** in `Peer.run()` with
  **exponential backoff** (min 5s, max 60s) for ordinary failures.
- A transport failure in Active, or while DelayOpen holds a socket in Connect,
  restarts the FSM's ConnectRetryTimer with its initial value. Active's restart
  is unconditional, including its strict BFD pending sub-state after Event 20
  cleared DelayOpen. Before cleanup,
  `handleConnectionCloseLocked` snapshots the sampled deadline and its owning
  clock through `Timers.ConnectRetryBudget`. Every timer retirement drains an
  owed transport failure before clearing DelayOpen. `runOnce` joins all old
  timer work, retires that set, and returns the original error with a
  `connectionRetryFailure` budget. `Peer.run` reads the remaining duration from
  the original timer clock at its wait, then uses its own clock's wait channel,
  without another jitter sample or another full interval after cleanup. No
  destination-clock sample can add time to the original deadline. A canceled
  timer has no transferable budget; an expired but uncanceled arm transfers a
  due retry. Inbound connections and
  administrative wake retain their existing early-interruption behavior.
- The `EventConnectRetryTimerExpires` FSM handlers in `handleConnect` and
  `handleActive` are tested, but the timer's expiry callback is not wired to
  that event in a production Session. Its arm can expire during cleanup; the
  saved budget then reports a due retry. The Peer owns the next connection
  cycle after the old Session and its timer callbacks have been retired.

The separation of session timer ownership from live Peer scheduling is recorded
in the `fsm.go` file header under "ARCHITECTURAL NOTES".

<!-- source: internal/component/bgp/fsm/fsm.go — ARCHITECTURAL NOTES -->
<!-- source: internal/component/bgp/reactor/peer_run.go — run function doc comment -->

## Panic recovery summary

The recover boundaries in the peer lifecycle. The first five go through the
BGP `taskguard` (count, crash record, ERROR log), named by their task:

| Boundary | Scope | Recovery behavior |
|----------|-------|-------------------|
| `Session.Run` (task `read`) | The read loop: message decoding, FSM, plugin callbacks that run synchronously from the session goroutine | Cease / Unspecific to the peer, Event 8, return the panic as `Run`'s error |
| `Session.Run` cancel goroutine (task `cancel`) | Closing the connection when the session's context ends | Close the connection |
| `safeRunOnce` (task `connect`) | The connection cycle outside `Run`: dial, OPEN setup, teardown | Return the panic as the cycle's error to `run()` for normal backoff |
| Delivery worker goroutine (task `delivery`) | Async message delivery to the reactor's message receiver | Fail the receive publication: Cease / Unspecific, session down; close `deliveryDone`, exit worker (remaining buffered items are dropped intentionally) |
| `Listener.safeHandle` (task `accept`) | The handler of one accepted connection | Close that connection; the accept loop keeps running |
| Initial-sync goroutine | Static routes, queued operations, plugin routes and automatic End-of-RIB | Log panic with stack, then end the session (`endSessionAfterSyncPanic`): the peer moves to Connecting, Cease / Unspecific to the peer with no End-of-RIB before it, Event 8 (AutomaticStop); then clear its session's queueing gate and marker-owed flag, wake forwarded overflow, and return. The operation queue is kept for the next session |

Recovery does not itself kill the peer goroutine. A session failure still follows
the normal reconnect path unless the peer is stopped.

The initial sync's recovery ends its session because the initial update
stopped at an unknown point: the peer holds no table it can trust, and an
Established peer whose queue nothing drains would queue every later route and
suppress every later End-of-RIB. The recovery uses the direct teardown a queued
teardown uses (`sessionTeardown`, as `runQueuedTeardown` does), never
`Peer.Teardown` or `teardownOf`, which queue the teardown while the sync is in
flight, into the queue nothing would drain. The peer moves to Connecting first,
so the reconnect is not refused as a collision. The queued operations stay: the
next session's sync drains them (the queue drain in `sendInitialRoutes`, then
`drainAndCloseQueueGate`), and one the interrupted drain had already sent is
sent again, which is harmless for an announce or a withdraw. The gate opens
only after the peer has left Established, so a route that arrives then joins
the queue for the next session. A session that has already ended is not torn
down again. A panic that repeats for one peer's table repeats as a reconnect
loop under the connect-retry backoff, each time with its stack in the log.

<!-- source: internal/component/bgp/reactor/peer_run.go — safeRunOnce, runOnce delivery goroutine recover -->
<!-- source: internal/component/bgp/reactor/listener.go — safeHandle -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go — sendInitialRoutes -->

Initial sync takes two separate session write holds: one for plugin routes and
one for automatic End-of-RIB. Each held phase MUST defer its own release before
calling an encoder or writer, so panic unwinding releases `writeMu` before the
outer recovery runs. The queue drain and forwarding wake between the phases MUST
remain outside either hold; merging the holds would block writers the drain
needs. The recovery's own teardown can therefore acquire the session writer
for its NOTIFICATION.

That NOTIFICATION follows only what the socket took before the panic.
`bufio.Writer.Flush` forgets the octets it offered only when the socket write
returns, so a panic inside that write (the socket, or an observer called after
the socket accepted them) leaves them staged, and the next flush would send
them again ahead of its own message: the interrupted UPDATE or End-of-RIB in
front of the Cease. `observedBGPWriter`, the session's socket boundary, marks a
write that has not returned, and `releaseWrites`, which every write-path holder
defers, drops what is staged when it finds the mark still set
(`discardInterruptedWrite`), before any other writer can take the path. Whether
the socket took those octets cannot be known after the panic, so their AIGP
writes are recorded as not delivered. The same release serves the read loop's
recovery (`ceaseAfterPanic`). The forward worker's recovery keeps its session,
so there the dropped octets are lost to the peer along with the rest of the
batch it abandons.
<!-- source: internal/component/bgp/reactor/session_write_interrupted.go — discardInterruptedWrite -->
<!-- source: internal/component/bgp/reactor/session_wire.go — observedBGPWriter -->
<!-- source: internal/component/bgp/reactor/session_write.go — releaseWrites -->

The static-route phase likewise defers `staticMu` release at its existing
boundary. Both queued RIB drains track their `p.mu` ownership: a codec panic
unwinds a held mutex, while a socket-write panic must not unlock a mutex already
released for the send. Every send retains its unlocked interval, allowing new
operations to enter the queue. These releases happen before initial-sync recovery
ends the session, clears its flags and wakes forwarding: the recovery is the
first deferred call `sendInitialRoutes` registers, so it runs last.

`TestInitialSyncPanicReleasesWriteHolds` injects a one-shot connection panic at
each phase's first write, then uses `TryLock` to assert release without allowing
cleanup to hang a failing test. Existing exact-wire tests retain the protocol
ordering assertions.
<!-- source: internal/component/bgp/reactor/peer_initial_sync_unwind_test.go — TestInitialSyncPanicReleasesWriteHolds -->
`TestInitialSyncPanicReleasesPeerLocks` covers a static write fault and an NLRI
codec fault in each real queue drain. `TestInitialSyncQueuedWritePanicRunsUnlocked`
checks the opposite ownership state by faulting each drain's socket write and
asserting `p.mu` is free at that boundary. Each socket fault is followed by one
more socket write, and the wire then holds what the socket took before the
fault followed by exactly the Cease / Unspecific NOTIFICATION, nothing of the
interrupted message. `TestInitialSyncPanicEndsTheSession`
faults a queued route's codec and asserts the session ended: the peer left
Established, the wire carries only the Cease / Unspecific NOTIFICATION, and the
route is still queued.
<!-- source: internal/component/bgp/reactor/peer_initial_sync_unwind_test.go — TestInitialSyncPanicReleasesPeerLocks, TestInitialSyncQueuedWritePanicRunsUnlocked, TestInitialSyncPanicEndsTheSession -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go — endSessionAfterSyncPanic -->

## Code map

| Concern | File | Symbol |
|---------|------|--------|
| Peer struct, constructor, state enum, forwarding fences | `internal/component/bgp/reactor/peer.go` | `Peer`, `NewPeer`, `PeerState`, `setState`, `forwardGeneration`, `forwardRevocation` |
| Start / Stop / Teardown API | `internal/component/bgp/reactor/peer.go` | `Start`, `StartWithContext`, `Stop`, `Teardown` |
| Reconnect defaults and overrides | `internal/component/bgp/reactor/peer.go` | `DefaultReconnectMin`, `DefaultReconnectMax`, `setReconnectDelay` |
| Outer run loop and backoff | `internal/component/bgp/reactor/peer_run.go` | `run`, `safeRunOnce` |
| Per-attempt session lifecycle | `internal/component/bgp/reactor/peer_run.go` | `runOnce` |
| FSM state-change callback | `internal/component/bgp/reactor/peer_run.go` | `SetCallback` closure inside `runOnce` |
| Initial route synchronization and panic unwind | `internal/component/bgp/reactor/peer_initial_sync.go` | `spawnInitialSync`, `runInitialSync`, `sendInitialRoutes`, `drainAndCloseQueueGate`, `endSessionAfterSyncPanic` |
| Cleanup on peer stop | `internal/component/bgp/reactor/peer_run.go` | `cleanup` |
| Inbound connection buffering | `internal/component/bgp/reactor/peer_connection.go` | `setInboundConnection`, `takeInboundConnection` |
| RFC 6.8 collision resolution | `internal/component/bgp/reactor/incoming_candidate.go` | `handlePendingCollision`, `resolveIncoming`, `finishIncomingHandoff` |

<!-- source: internal/component/bgp/reactor/peer.go — Peer, NewPeer, PeerState, Start, Stop, Teardown, setReconnectDelay, DefaultReconnectMin, DefaultReconnectMax -->
<!-- source: internal/component/bgp/reactor/peer_run.go — run, safeRunOnce, runOnce, cleanup -->
<!-- source: internal/component/bgp/reactor/peer_connection.go — setInboundConnection, takeInboundConnection, setPendingConnection, storeInboundLocked -->
<!-- source: internal/component/bgp/reactor/incoming_candidate.go -- handlePendingCollision, resolveIncoming, finishIncomingHandoff -->

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
- `internal/component/bgp/reactor/rfc8050_collision_epoch_test.go` — two real TCP
  connections drive the live Peer collision lifecycle; the winning socket
  establishes and its unchanged directional OPENs decode mixed-family MRT
  UPDATEs with exact prefixes and Path Identifiers.
  <!-- source: internal/component/bgp/reactor/rfc8050_collision_epoch_test.go — TestMRTWinningCollisionPreservesOPEN -->
- `TestMRTCollisionWinnerReservation` in the same file pauses the actual
  replacement epoch at publication and after taking the winner. A third TCP
  connection must be refused before OPEN; releasing the barrier then completes
  the original winning connection and all exact MRT byte/prefix/ID assertions.

Incoming candidates use the ordinary Session frame reader, including header
errors, NOTIFICATION events and the original-wire capture tee. A first OPEN
passes the same version, Hold Time, AS, Identifier, plugin and capability checks
before it can retire an incumbent. Its validated receipt owns the original
wire and the negotiated result; promotion advances it without validating or
negotiating twice. A collision loser sends no KEEPALIVE.

A private candidate negotiates its own wire capabilities and timers. It begins
no Peer-held route generation until selection, after the old Session cleanup
and capture have joined. Negotiation callbacks run outside the Session mutex.
A terminal refusal seals the candidate and retires its timers before acquiring
the NOTIFICATION writer, so a queued DelayOpen callback cannot send OPEN after
refusal.

An OPEN validator that finishes after the candidate's original HoldTimer
deadline cannot win collision arbitration. Admission, the actual Peer-lock
publication commit, OPEN advancement and adoption all retain that deadline;
the Session OPEN owner delivers the ordinary Hold Timer Expired action even
when the scheduled callback has not acquired that owner yet. Plugin-coded
OPEN refusal delivers Event22 and closes its own socket before retirement,
so it increments retry history once and receives no TCP-failure retry budget.
<!-- source: internal/component/bgp/reactor/session_hold_expiry.go -- rejectExpiredHold, holdTimerExpiredLocked -->
