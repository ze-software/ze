# Spec: single-owner-lifecycle-2-bgp-session

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Release fix: on a rapid reconnect a goroutine of the ended session can write End-of-RIB and queued routes on the next session's connection, the reconnect backoff never returns to its minimum after a session that ran, and a passive peer added during the standalone start accepts nothing. All three are defects in shipped BGP session behaviour. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 2 of `plan/release/fixes/spec-single-owner-lifecycle.md`, which holds the
decision record and the routing of every journal row. This child owns the three
rows that live in the BGP reactor's session lifecycle. It depends on no other
child: children 1, 3, 4 and 5 touch other packages.

The principle the parent settled: every lifecycle here already has an identity,
and each defect is a path that bypasses it. The fix closes each bypass at its
choke point, by moving state onto the object whose lifetime it has. No new
owner object, no new generation counter, no source gate.

Three defects, one per journal row.

1. **The initial sync outlives its session.** `sendInitialRoutes` is the one
   per-session goroutine that `runOnce` does not join. It carries no session
   identity: it re-reads `Peer.session` at each phase and stores two Peer-level
   flags on every exit. A session that ends and is replaced while the goroutine
   sits in its bounded peer-up barrier wait (2 seconds) leaves a goroutine that
   then drains the queue, writes plugin routes and writes End-of-RIB on the
   successor's connection, and clears the successor's flags. The peer can
   receive End-of-RIB before the routes of the initial update it closes.
2. **The reconnect backoff never restarts.** The reconnect delay and the
   prefix-limit teardown count return to their minimum only when a cycle
   returns no error, which happens only on stop. A peer that ran for a week and
   then lost its session once reconnects on whatever delay its history left.
3. **A passive peer added during the standalone start has no listener.**
   `addPeerLocked` opens a listener only while the reactor is running, and the
   standalone start sets running after its plugin waits, which follow the one
   listener pass.

The goal: a session's initial sync ends with its session and is joined like
every other per-session goroutine; the backoff restarts after a session that
was stable; a passive peer added once the listeners have started has one.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: a goroutine is a long-lived worker or a per-lifecycle goroutine, never one per event. The initial sync stays one goroutine per session.
  → Constraint: a page made wrong by the change is edited in the same phase.
- [ ] `ai/doors/bgp-engine.md` - the door for the reactor
  → Constraint: a protocol peer relies on the RFCs. Any wire-visible change owes `./le test integration interop`; this spec changes no byte and changes the order a peer can observe on a rapid reconnect, so it names one scenario.
  → Constraint: the scoped package test runs with `-tags ze_bgp`; an untagged run does not link BGP.
- [ ] `ai/doors/le-tooling.md` - the door for `test/` and the functional runner
  → Constraint: a `.ci` asserts on state the daemon reports, and a compiled fixture returns an error when an assertion fails.
- [ ] `ai/digests/bgp-reactor.md` - the current flow of a session
  → Constraint: the digest names three goroutines per connection attempt (read, cancel, delivery). It is silent on `sendInitialRoutes`, the recovery worker, the BFD subscriber and `forwardGeneration`. Phase 2 adds what this spec touches.
  → Decision: `sessionDown` runs before the earlier-registered defer that clears `Peer.session`. The join of the initial sync goes in that earlier defer, after `sessionDown` announced the down, beside `joinRecovery`.
- [ ] `docs/architecture/behavior/fsm.md` - FSM states, the ConnectRetryCounter, the reactor notification
  → Constraint: `Peer.cycleReachedEstablished` chooses Event 3 or Event 6 for the next cycle and is "the only input to that choice". `run` can still read it after the cycle returns, because only the next `runOnce` swaps it back. The backoff reads it as one of two conditions: reaching Established does not reset by itself (owner decision), the session must also have stayed up.
  → Constraint: the page says the reconnect loop's exponential backoff "is the damping". It is silent on when the damping forgets. Phase 3 adds it.
- [ ] `docs/architecture/behavior/peer-lifecycle.md` - the reconnect loop and the initial sync
  → Constraint: its "Reconnect backoff" table has a row "Successful session exit (err == nil): reset" that describes a branch only a stop reaches. Phase 3 rewrites the table.
  → Constraint: it records `reconnectMax` as `DefaultReconnectMax` with "no setting yet". The stable time therefore has no configuration leaf to read: it is the peer's `reconnectMax` field, 60 seconds today. Nothing configures it and this spec adds no knob.
- [ ] `ai/rules/goroutine-lifecycle.md`, `ai/rules/performance.md`
  → Constraint: no allocation and no lock is added to a per-UPDATE function. `forwardOrderHold` is called per forwarded UPDATE per destination.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - the FSM and its damping
  → Constraint: RFC 4271 Section 8.1.1: "The DampPeerOscillations option specifies that the implementation engages additional logic to damp the oscillations of BGP peers in the face of sequences of automatic start and automatic stop." and "An example of DampPeerOscillations logic is an increase of the IdleHoldTime value if a BGP peer oscillates connectivity (connected/disconnected) repeatedly within a time period."
  → Constraint: RFC 4271 Section 8.2.2: "The method of preventing persistent peer oscillation is outside the scope of this document." The RFC sets no rule for when the damping forgets, so the rule comes from interoperating implementations (BIRD and FRR, below).
- [ ] `rfc/short/rfc4724.md` - End-of-RIB
  → Constraint: RFC 4724 Section 4: "The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established." A marker written by the previous session's goroutine precedes the successor's initial update, which is the defect of row 1.

Each quote above was read in `rfc/full/rfc4271.txt` and `rfc/full/rfc4724.txt`;
only line breaks were removed.

**Key insights:**
- The peer-up barrier wait (`peerUpBarrierTimeout`, 2 seconds) is the wait a predecessor goroutine survives its session in. The older API-sync wait no longer exists.
- `Session.Done()` is closed when `Session.Run` returns. A sync goroutine exists only after Established, which is reached only inside `Run`, so `Done()` is a session-end signal the barrier wait can select on.
- The forward-overflow hold predicate must stay lock-free: `forward_pool.go` forbids taking `Peer.mu` under the worker's overflow lock. Reading "the current session's flag" there needs a lock-free route to the session.
- The startup convergence hold (`update_delay.go`) starts the sync later, from a release, for a `*Peer`. It is a second start site and it must carry the session.
- The reconnect maximum is not configured anywhere: `setReconnectDelay` has no production caller and no YANG leaf sets it.
- At the delay's cap, two consecutive errors are already about one maximum apart, because the wait between them is the maximum. A quiet period measured error to error would forget on a peer that never came up.
- `Peer.counters.establishedAt` (`peer_stats.go`, `setEstablishedNow`, read by `EstablishedAt`) is stamped from the peer's clock on entry to Established and cleared only by `ClearStats`, which only `Peer.cleanup` calls. After a cycle it still holds that cycle's entry time when the cycle reached Established, and an older session's time when it did not, so it is read only together with `cycleReachedEstablished`.
- `addPeerLocked` gates its listener block on `running`, but `startMultiListeners` itself runs with `running` false, and an inbound connection for a peer that is not started waits in the peer's inbound slot until the barrier lifts (commit 784c25a39c). A listener opened before the barrier accepts nothing early.

## Current Behavior (MANDATORY)

**Source files read:** (every statement below was read at the function named)
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `sendInitialRoutes`, `drainAndCloseQueueGate`, `runQueuedTeardown`, `queuedTeardownSession`
  → Constraint: `sendInitialRoutes` takes no argument. It reads `Peer.currentSession()` once for the static phase, then `Peer.session` again inside the queue drain (per operation), again before the plugin-route phase, and uses that last read for the End-of-RIB phase. `runQueuedTeardown` sends its markers through `Peer.SendUpdate`, which uses whatever session the peer holds then.
  → Constraint: it stores `sendingInitialRoutes` and `initialSyncEOROwed` on six exits: panic recovery, no negotiated capabilities, `drainAndCloseQueueGate` (queue flag only), `runQueuedTeardown`, and the end of the marker loop (marker flag only).
  → Constraint: entry is guarded by a compare-and-swap of the Peer flag from 1 to 2. `setState` stores 1 unconditionally at Established, over a predecessor's 2, "so the new session's goroutine can take the CAS". Two goroutines can therefore run for one peer at once, and the predecessor's later store of 0 opens the successor's gate.
  → Constraint: End-of-RIB claims (`claimInitialSyncEOR`) are a Peer-level map reset in the `runOnce` defer. A predecessor that claims a family after that reset makes the successor's own marker loop skip the family.
- [ ] `internal/component/bgp/reactor/peer_run.go` - `run`, `runOnce` and its defers
  → Constraint: `runOnce` defers, in registration order: the session-end clear (first registered, last run), `endNegotiatedSession`, `sessionDown`. The clear calls `session.joinRecovery()`, then stores both Peer flags to zero, wakes the forward overflow, resets the End-of-RIB claims and sets `Peer.session` to nil. Its comment says the reset is needed "because session.Teardown() may return while the old sendInitialRoutes() goroutine is still inside its bounded waits": the goroutine is known to be unjoined.
  → Constraint: `run` holds `delay` as a local. It resets `delay` to `reconnectMin` on `ErrTeardown`, on the administrative wake, on an inbound connection during the backoff, on the administrative hold, and in the branch where `safeRunOnce` returned nil. Every other error waits `fsm.Jitter(delay)` (0.75 to 1.0 of the base) and then doubles `delay` up to `reconnectMax`.
  → Constraint: `prefixTeardownCount` is read and written only in `run`. It is reset only in the nil branch. `runOnce` returns nil only when the peer is stopping or administratively down, so neither reset follows a session that ran.
  → Constraint: `run` already records `attemptStart` before each cycle and computes `sessionElapsed` after it, for a metric and a debug line.
- [ ] `internal/component/bgp/reactor/peer.go` - `setState`, `shouldQueue`, `queueGateClosedLocked`, `forwardOrderHold`, `wakeForwardOverflow`, `pendingSync`, `teardownOf`, `initialReplayToken`, `waitPeerUpBarrier`, `advanceForwardGeneration`
  → Constraint: `setState` advances `forwardGeneration` and stores both flags before it publishes `PeerStateEstablished`, so an observer of Established already sees the gate closed. Its one production caller for Established is `enterEstablished` (`source_policy_reload.go`), called from the FSM callback, which holds the session.
  → Constraint: `forwardOrderHold` reads only atomics. `takeOverflowReleased` (`forward_pool.go`) calls it under the worker's overflow lock, and that file states a predicate taking `Peer.mu` there would invert the lock order against the peer's own goroutine.
  → Constraint: `waitPeerUpBarrier` selects on the barrier channel and `clock.After(peerUpBarrierTimeout)` only. It does not observe the session.
- [ ] `internal/component/bgp/reactor/update_delay.go` - `startInitialRoutes`, `holdInitialUpdate`, `runHeldInitialUpdates`
  → Constraint: the hold keys its held set by `*Peer` and `runHeldInitialUpdates` starts one goroutine per held peer. Its comment relies on the Peer-flag compare-and-swap and the negotiated-capabilities read to refuse a peer whose session died while held.
- [ ] `internal/component/bgp/reactor/session.go`, `session_connection.go`, `session_write.go`, `session_ownership.go` - `Session.Done`, `seal`, `sendUpdateHeld`, `checkInitialWrite`
  → Constraint: `seal` is called by a teardown, by `Reactor.stop` and by a failed write. A session ended by a received NOTIFICATION, a hold-timer expiry or a closed connection is not sealed. What refuses a write on an ended session is the FSM state (`sendUpdateHeld` returns `ErrInvalidState` outside Established) and the closed connection.
  → Constraint: `checkInitialWrite` runs under the session's write lock and reads `item.peer.initialSyncEOROwed`, the Peer flag, although it already holds the session. The comment on `Session.initialReplay` says the receipt "expires with Peer.initialSyncEOROwed".
- [ ] `internal/component/bgp/reactor/peer_send.go` - `sendRefreshMarker`
  → Constraint: it reads the Peer flag and appends the marker under one hold of the peer lock, with `MarkerSession` set to the session the peer holds then.
- [ ] `internal/component/bgp/reactor/reactor.go`, `reactor_peers.go` - `StartWithContext`, `StartPeers`, `openPeerStartBarrierLocked`, `startMultiListeners`, `listenerSpecs`, `startListenerForAddressPort`, `newListenerFactory`, `addPeerLocked`
  → Constraint: `StartWithContext` calls `startMultiListeners` once, releases the reactor lock, and in standalone mode waits for the plugins, validates families and may wait `ze.bgp.announce.delay`, all with `running` false. It then sets `running` and opens the barrier in one critical section.
  → Constraint: `addPeerLocked` runs its listener block only under `r.running && !r.stopping`. The block has two arms written inline: start a listener when none exists for the peer's address and port, and stop and rebuild an existing one when the new peer carries an MD5 key, because the key set is read once, when the socket is created (`newListenerFactory`).
  → Constraint: in borrow mode (production, under the hub) `running` is set at the end of `StartWithContext` and the barrier opens later, in `StartPeers`. The same gap exists there only between the lock release and that write.

**Behavior to preserve:**
- The queue gate closes before `PeerStateEstablished` is published, in the same call.
- One End-of-RIB per family per session, from whichever producer claims it first.
- A peer-level teardown that finds no session stays at the head of the queue for the next session; operations left in the queue stay for the next session.
- `behavior { manual-eor true }` withholds the automatic marker and still clears the owed fact.
- The 2 second peer-up barrier timeout for a live session, with its warning.
- `ErrTeardown`, the administrative wake, the administrative hold and an inbound connection during the backoff reset the delay to its minimum.
- The prefix-limit reconnect modes (`never`, `timer`, `backoff`) and the one hour cap.
- The choice between Event 3 and Event 6 from `cycleReachedEstablished`.
- A listener start failure for a peer added while running rolls that peer's addition back.

**Behavior to change:**
- The initial sync goroutine belongs to one session, ends with it and is joined before `runOnce` returns.
- The two initial-sync flags are the session's.
- The reconnect delay and `prefixTeardownCount` restart after a session that reached Established and stayed up at least `reconnectMax`.
- A passive peer added after the listeners started has a listener, whether or not the reactor is running yet.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The `*Session` pointer as the identity of a connection cycle | `reactor/peer_run.go` `runOnce`; `reactor/peer.go` `teardownOf`; `peerOp.TeardownSession`, `peerOp.MarkerSession` | A teardown or a refresh marker that names a session is dropped once another session holds the peer | Extends: the initial sync takes its `*Session` at start and uses that pointer for every write, instead of re-reading `Peer.session` |
| `Session.Done()` | `reactor/session.go` `Session.Done`, closed by `Session.Run` | Tells the cancel goroutine and outside callers the read loop exited | Extends: the peer-up barrier wait selects on it, so the wait ends with the session |
| The `runOnce` defer's joins | `reactor/peer_run.go` `runOnce`: `sessionDown` (delivery worker, leave signal, BFD subscriber), `endNegotiatedSession`, `session.joinRecovery`, `stopSessionTimers` | Every other per-session goroutine is joined before `Peer.session` is cleared | Extends: one more join, of the initial sync, beside `joinRecovery` |
| `Session.tearingDown` and `seal` | `reactor/session_connection.go` `seal` | Refuses a new connection and new writes on a session a teardown, a stop or a write failure ended | Does not apply: a session ended by the peer is never sealed, so the seal cannot be what refuses the predecessor. The session's own start latch (below) and its FSM state do that |
| `Session.initialReplay` | `reactor/session.go`; `reactor/session_ownership.go` `checkInitialWrite` | A per-session receipt that fences initial-replay writes to the session that issued it | Extends: `checkInitialWrite` reads the owed flag on the session it already holds, so the receipt and the flag belong to one object |
| `Peer.forwardGeneration`, `forwardSourceCurrent`, `initialSourceCurrent` | `reactor/peer.go` `advanceForwardGeneration`; `reactor/forward_pool.go` | Fences delayed forwarded UPDATEs across sessions, by source | Does not apply: it fences items by their source's generation. The hold predicate asks about the destination's sync, which the items do not carry. No second generation is added |
| The Peer-level initial-sync flags | `reactor/peer.go` `sendingInitialRoutes`, `initialSyncEOROwed` | Gate route queueing and record the owed marker, for whichever session the peer holds | Replaces: both become fields of `Session`. The Peer fields are deleted first, then every reader is moved (`ai/rules/no-layering.md`) |
| `startInitialRoutes` as the one door onto the sync | `reactor/update_delay.go` | The FSM callback and the convergence hold both start the goroutine | Extends: the door takes the session, and the hold records the session with the peer |
| `cycleReachedEstablished` | `reactor/peer.go`; `reactor/peer_run.go` `runOnce` | Chooses Event 3 or Event 6 for the next cycle | Extends: `run` loads it (without swapping) as the first condition of the backoff restart. Its writer and its swap are unchanged |
| The Established timestamp | `reactor/peer_stats.go` `setEstablishedNow`, `EstablishedAt` | Stamps entry to Established from the peer's clock; feeds uptime in `show bgp peer` and the metrics | Extends: `run` reads it as the start of the stable time. Chosen over `sessionElapsed`, which also counts the dial and the handshake and, for a passive peer, the wait for an inbound connection |
| `attemptStart` and `sessionElapsed` in `run` | `reactor/peer_run.go` `run` | Feed a metric and a debug line | Does not apply: it measures the whole cycle, not the time in Established |
| The listener block of `addPeerLocked` | `reactor/reactor_peers.go` `addPeerLocked`; `reactor/reactor.go` `startListenerForAddressPort` | While `running`: starts a listener for a pair that has none, and rebuilds an existing one for a peer with an MD5 key | Extends: the same block, both arms unchanged, runs once the listeners have started instead of once `running` is set |
| `startMultiListeners` and `stopAllListeners` | `reactor/reactor.go` | Start one listener per configured pair at start; stop them all on stop and on an aborted start | Extends: they are the two writers of one new reactor fact, "listeners started", which `addPeerLocked` reads |
| The inbound slot and `peersStarted` | `reactor/peer_connection.go` `setInboundConnection`; `reactor/reactor.go` `openPeerStartBarrierLocked` | A connection for a peer with no session waits in the peer's slot; peers start only when the barrier lifts | Does not apply as a change: it is why an earlier listener is safe |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Row 1: the FSM callback in `runOnce` on entry to Established, and the release of the startup convergence hold.
- Row 2: the error `safeRunOnce` returns to `Peer.run`.
- Row 3: `AddPeer` (config reload, API) reaching `addPeerLocked` while the standalone start waits.

### Transformation Path

Row 1, the initial sync:

1. The FSM callback calls `enterEstablished` with the session. That function raises the fences as today, then closes the session's queue gate and marks its marker owed, publishes the session as the peer's lock-free established session, and only then publishes `PeerStateEstablished`.
2. `startInitialRoutes` receives the session. Either it starts the sync through the session's start latch, or the convergence hold records the peer and the session and starts it at release through the same latch.
3. The latch has three values: not started, started, ended. Starting moves not started to started and is the only way a goroutine is spawned. A second start, or a start after the end, spawns nothing.
4. The goroutine uses its own session for every phase: the static routes, the queue drain, the plugin routes, the queued teardown and its markers, the End-of-RIB loop. An operation that reads `Peer.session` under the peer lock to compare identities keeps doing so, against the goroutine's session.
5. The peer-up barrier wait returns when the barrier opens, at the timeout, or when the session's `Done()` closes.
6. The goroutine's stores clear its own session's flags. It closes the latch's completion signal as its last action, a panic included.
7. The `runOnce` session-end defer moves the latch to ended. When it finds it started, it waits for the completion signal. Only then does it reset the End-of-RIB claims, clear the lock-free established session and set `Peer.session` to nil.

Row 2, the backoff, inside `run` when a cycle returned any error, before the `ErrTeardown` branch:

1. Load `cycleReachedEstablished`. When it is false, nothing restarts, whatever the cycle's duration: a dial that hung for a minute, or a passive peer that waited an hour for a connection and then failed its handshake, resets nothing.
2. When it is true, compute the stable time: the peer clock's now minus `EstablishedAt()`.
3. When the stable time is at least the peer's `reconnectMax`, set `delay` to `reconnectMin` and `prefixTeardownCount` to zero.
4. Continue as today: `ErrTeardown` resets the delay and redials at once; otherwise the prefix-limit decision with the count, or the wait of `fsm.Jitter(delay)` followed by the doubling.

The check runs before the `ErrTeardown` branch so that an operator's teardown of a stable session also clears the prefix count. That branch resets the delay on its own, as today, stable or not. The inbound-connection, administrative-wake and administrative-hold resets act during or instead of the wait and are independent of the check.

Row 3, listeners:

1. `startMultiListeners` records that the listeners have started. `stopAllListeners` clears it, so a stop and an aborted start both clear it.
2. `addPeerLocked` runs its listener block when the listeners have started and the reactor is not stopping. Both arms are unchanged: a missing listener is started, an existing one is rebuilt for a peer with an MD5 key, and a failure rolls the addition back.
3. `addPeerLocked` starts the peer only when `running` and `peersStarted` both hold, as today. A peer added before the barrier is started by the barrier.
4. An `AddPeer` during the configuration load, before `StartWithContext`, opens nothing, as today: `startMultiListeners` covers it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to peer (wire) | The same UPDATE and End-of-RIB frames, written by the session that owes them | No |
| Reactor to forward pool | `forwardOrderHold` read under the worker's overflow lock, atomics only | No |
| Reactor to plugins | None changed: `peer up`, `peer down` and the barrier acknowledgement are untouched | No |

### Changed-Wire Census

No wire changes. Evidence: the frames the initial sync writes are built by
`sendStaticRoutes`, `sendDefaultOriginateRoutes`, `buildRIBRouteUpdate`,
`sendPluginRoutesVia` and `message.BuildEOR`, none of which this spec edits;
the change is which goroutine may call them and on which session. No plugin
event, RPC field, command word or config leaf is added or renamed. The backoff
change alters when Ze opens a TCP connection, never a byte on it.

What a peer can observe changes in one case. Today, on a reconnect inside the
predecessor's barrier wait, the peer can receive End-of-RIB before the routes
of the initial update, and can miss the successor's own marker, because the
predecessor claimed the family. After this spec the peer receives the
successor's routes and then one marker per family, on every reconnect.

**Search evidence:** scope `internal/component/bgp/reactor/`; terms `sendingInitialRoutes`, `initialSyncEOROwed`, `sendInitialRoutes`, `startInitialRoutes`, `forwardOrderHold`, `BuildEOR`; positive control: the marker loop in `sendInitialRoutes` (writer) and `gr-purge-stale-eor.ci` plus a peer daemon (readers). 22 non-test reference sites to the two flags were found in `peer.go`, `peer_run.go`, `peer_initial_sync.go`, `peer_send.go`, `reactor_api_forward.go`, `session_ownership.go`; none encodes or decodes.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| BGP UPDATE and End-of-RIB of the initial update | writer | `reactor/peer_initial_sync.go` `sendInitialRoutes`, `runQueuedTeardown` | Read: frames built by unchanged builders, written through `sendUpdateHeld` and `sendUpdateWithSplit` | Preserve the bytes. The writer becomes the session's own goroutine only |
| BGP UPDATE and End-of-RIB of the initial update | writer | `reactor/reactor_api_forward.go` `AnnounceEOR` path (a route server's marker) | Read: defers while `initialSyncEOROwed` is set | Preserve. It reads the owed fact of the peer's current session |
| BGP UPDATE and End-of-RIB of the initial update | reader | A protocol peer (FRR, BIRD), outside the repository | RFC 4724 Section 4 sentence above | Preserve the bytes. Order on a rapid reconnect becomes routes, then marker |
| BGP UPDATE and End-of-RIB of the initial update | reader | `test/plugin/*.ci` wire expectations through `ze-peer` (`eor-per-family.ci`, `reconnect-static.ci`, `rib-reconnect.ci`) | Listed in `test/plugin/` | Preserve: re-run, no expectation edited |

### Integration Points
- `Peer.enterEstablished` - takes the session; the one place Established is published for a session.
- `Peer.startInitialRoutes`, `updateDelayHold.holdInitialUpdate`, `runHeldInitialUpdates` - carry the session.
- `runOnce` session-end defer - joins the sync.
- `Peer.run` - the stable-session check, reading `cycleReachedEstablished` and `EstablishedAt()`.
- `Reactor.addPeerLocked` - reads the listeners-started fact; `startMultiListeners` and `stopAllListeners` write it.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## What changes in each named function

| Function | Today | After |
|----------|-------|-------|
| `Peer.setState` | Its Established arm advances the forward generation and stores both Peer flags | Its Established arm advances the forward generation only. The flag stores move to `enterEstablished`, which holds the session. The "Store, not CompareAndSwap" contract is deleted with its comment: a new session's flags start fresh, so nothing is forced over a predecessor |
| `Peer.enterEstablished` | Raises the fences, takes the receipt, calls `setState` | Takes the session. After the receipt it closes the session's gate, marks its marker owed, publishes the lock-free established session, then calls `setState`. The order "gate closed before Established is visible" is unchanged |
| `Peer.shouldQueue`, `queueGateClosedLocked` | Read the Peer flag under the peer lock | Read the gate of `Peer.session` under the same lock. A nil session reads as an open gate, as a zero flag does today |
| `Peer.teardownOf`, `Peer.sendRefreshMarker`, `Peer.pendingSync` | Read the Peer flags under the peer lock | Read the current session's flags under the same lock |
| `Peer.forwardOrderHold`, `initialSyncInProgress` | Load the Peer atomic | Load the lock-free established session, then its gate. Nil reads as not held. No lock, no allocation |
| `AnnounceEOR` path, `logEORSuppressed` (`reactor_api_forward.go`) | Load the Peer owed flag | Call one Peer accessor that reads the current session's owed flag under the peer lock. Cold path: once per End-of-RIB command per peer |
| `Session.checkInitialWrite`, `Peer.initialReplayToken` | Read the Peer owed flag | Read the owed flag of the session they already hold |
| `Peer.wakeForwardOverflow` | Wakes the destination worker | Unchanged. It is still called after every store that opens a gate, and by the session-end defer |
| `runHeldInitialUpdates`, `holdInitialUpdate` | Hold and start by `*Peer` | Hold the peer with the session that asked, replacing an earlier session of the same peer. The release starts each through the session's latch, which refuses an ended session. The comment about the compare-and-swap refusal is rewritten |
| `Peer.waitPeerUpBarrier` | Waits for the barrier or 2 seconds | Takes the session and also returns when its `Done()` closes. It reports which of the three ended the wait, and the timeout warning is logged only for the timeout |
| `sendInitialRoutes` | A Peer method with no argument | Takes the session. Reads negotiated capabilities from that session |

### The session's initial-sync fields

| Field | Type | Description |
|-------|------|-------------|
| Queue gate | atomic integer, values 0, 1, 2 | Today's `sendingInitialRoutes`, with the same three meanings: open, closed, closed with the goroutine running |
| Marker owed | atomic boolean | Today's `initialSyncEOROwed` |
| Start latch | atomic integer, three values | Not started, started, ended. Written by the start door and by the session-end defer only |
| Completion signal | channel, created with the session | Closed by the sync goroutine as its last action. Waited on by the session-end defer when the latch reads started |

| Field on `Peer` | Type | Description |
|-----------------|------|-------------|
| Established session, lock-free | atomic pointer to `Session` | Written by `enterEstablished` before Established is published, cleared by the session-end defer after the join. Read by `forwardOrderHold` only. `Peer.session` under the peer lock stays the declaration; a test asserts the two agree whenever the peer is Established |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every wait the sync goroutine can be in after its session ended is bounded: sends fail on the closed connection or the FSM state, the barrier wait observes `Done()`, and the locks it takes (`staticMu`, the peer lock, the session write lock) are held only by bounded writers | `sendUpdateHeld` returns `ErrInvalidState` outside Established (read). The other write paths and lock holders were not read end to end | The join in `runOnce` blocks and the peer stops reconnecting | `TestInitialSyncJoinReturnsWhenBarrierNeverOpens` and `TestInitialSyncJoinReturnsDuringQueueDrain` | unvalidated |
| A-2 | A teardown the goroutine runs from the queue (`runQueuedTeardown`) on an ended session returns without waiting on anything `runOnce` releases after the join | The `runOnce` comment says a teardown returns while the goroutine is still in its waits; `teardownWith` was not read | A deadlock between the goroutine and the join | `TestInitialSyncJoinWithQueuedTeardown` under `-race` | unvalidated |
| A-3 | No production path publishes `PeerStateEstablished` except through `enterEstablished` | One call site found (`source_policy_reload.go`); tests call `setState` directly | A session reaches Established with its gate open | grep for `setState(PeerStateEstablished)` at implementation, plus `TestEstablishedPublishedWithGateClosed` | unvalidated |
| A-4 | A listener opened before the barrier lifts accepts nothing early: an inbound connection for a peer that is not started waits in the peer's inbound slot | Commit 784c25a39c and `setInboundConnection` (signature read); `startMultiListeners` already opens every configured listener before the barrier. The accept path for an unstarted peer was not read end to end | A peer added in the window could open a session before the plugins its policy names are connected | `TestPassivePeerAddedBeforeBarrierHoldsInboundUntilBarrier` | unvalidated |
| A-5 | The stable time is `Peer.reconnectMax`, 60 seconds for every peer, because no configuration sets it | `NewPeer` and `setReconnectDelay` (read): no production caller, no YANG leaf; `peer-lifecycle.md` says "no setting yet" | A configured maximum exists and the rule reads the wrong value | grep at implementation | unvalidated |
| A-6 | The owner's decision ("D2 - BIRD is better") is kept by its intent, a stable session clears the penalty and a flapping one stays damped, measured as time in Established rather than error to error | Thomas confirmed it on 2026-11-11: "the refined D2 rule stands". See Key Design Decisions | The owner wants BIRD's literal measure, with the restart it produces at the cap | Thomas confirmed on 2026-11-11: "the refined D2 rule stands" | confirmed |
| A-9 | `EstablishedAt()` still holds the ended session's entry time when `run` reads it | `ClearStats` is its only clear and `Peer.cleanup` its only caller (grep) | The stable time reads zero or a later session's stamp | `TestRunBackoffRestartsAfterStableSession` | unvalidated |
| A-7 | A session held by the convergence hold that ends and is replaced before the release is replaced in the held set by the new session of the same peer | `holdInitialUpdate` keys by `*Peer` (read); `updateDelayPeerDown` was not read | A release starts a sync for neither session | `TestHeldInitialUpdateStartsTheCurrentSession` | unvalidated |
| A-8 | FRR at the lab's version reports End-of-RIB received and the received prefix count in the neighbor JSON the scenario reads | Not read in FRR's source | The interop check asserts a field FRR never prints | The implementer reads FRR's source and names the function in the scenario before writing the check | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The join never returns and a peer stays between sessions | A peer in `connecting` with no "connection attempt starting" debug line | Every wait observes the session end (A-1); the three join tests run with `-race -count=20` |
| R-2 | A reader of the flags is missed and still compiles, because it reads a field that kept its name elsewhere | A test that expects queueing sees a direct write | The Peer fields are deleted first, so every reader fails to compile until it is moved |
| R-3 | About 130 references to the two flags in 43 test files set or read the Peer fields directly | Compile errors across `reactor` tests | One test helper that gives a test peer a session with a chosen gate state. This is test maintenance, reported apart from the fix; no assertion changes |
| R-4 | An operator sees a longer outage after a short-lived session than today's code would give after a restart of the peer: the delay still doubles when a session lasted less than 60 seconds | Reconnect waits of 10 to 60 seconds on a peer that flaps | Intended: that is the damping. `ErrTeardown` and an inbound connection still reset |
| R-5 | A peer that exceeds its prefix limit more than 60 seconds after establishing, every time, never escalates the `timer` reconnect wait past its base | `teardown_count` stays 1 in the "prefix limit teardown" warning | Recorded as the consequence of applying one stable time to both counters, which the owner decided. A separate stable time for the prefix count is a new knob and is not added |
| R-6 | Ze grows the delay on every session end, a Cease and a TCP failure included. BIRD grows it on neither (`bgp_rx_notification` skips the update for code 6; `bgp_sock_err` does not call it). An operator moving from BIRD sees Ze wait longer to redial after the far end closes with Cease or the TCP session drops | Reconnect waits after a peer's administrative reset | Not changed by this spec. Recorded as a difference in `peer-lifecycle.md`. An inbound connection from the peer is accepted at once and resets the delay |
| R-7 | An `AddPeer` in the start window now fails when its listener cannot start (address in use), where today it succeeded and the peer silently accepted nothing | `AddPeer` returns the existing "listen on" error and the peer is not added | Intended: it is the contract `AddPeer` already has while running |
| R-10 | The listeners-started fact is left set after a stop, and an `AddPeer` on a stopped reactor opens a socket | A listening socket on a stopped reactor | `stopAllListeners` is its only clear and every stop and abort path calls it; `startListenerForAddressPort` also refuses while `stopping`. `TestAddPeerAfterStopOpensNoListener` |
| R-8 | The functional test for the backoff has to hold a session for more than 60 seconds | A slow `.ci` | It asserts on the delay the daemon logs, never on wall time. If the suite cannot carry a 70 second test, the implementer reports it and stops; shortening the stable time through a new knob is not permitted |
| R-9 | The rapid-reconnect `.ci` and the interop scenario cannot be made to go red by reverting the fix, because the window needs a barrier plugin that withholds its acknowledgement | The revert run stays green | They are claimed only for the positive outcome. The discriminating proof is the unit test that holds the barrier with the fake clock. The revert result is recorded either way |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live sessions: a join that blocks stops a peer reconnecting; a gate read from the wrong session sends a route ahead of the initial update or parks forwarded UPDATEs for the life of a session |
| How is it reverted? | Each phase is one commit and reverts alone. Nothing persists and no peer keeps state from it |
| Who else touches this path? | `spec-bgp-llgr-reconnect-timers`, `spec-bgp-graceful-restart-rfc-defects` and `spec-bgp-peer-state-fsm-and-reason` name reactor session files; check their claims before phase 1. Children 1, 3, 4 and 5 of the parent do not touch `reactor/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| FSM callback on entry to Established | → | `enterEstablished` with the session, the session's start latch | `TestRunOnceJoinsInitialSyncBeforeReturn` |
| Release of the startup convergence hold | → | `runHeldInitialUpdates` through the latch | `TestHeldInitialUpdateRefusesEndedSession` |
| A forwarded UPDATE dispatched to an established destination | → | `forwardOrderHold` reading the session's gate | `TestForwardOrderHoldReadsCurrentSessionGate` |
| A session error returned to `Peer.run` | → | the stable-session check | `TestRunBackoffRestartsAfterStableSession` |
| `AddPeer` during the standalone announce-delay hold | → | the listener block of `addPeerLocked` | `TestPassivePeerAddedBeforeBarrierGetsListener` |
| A peer that drops and reconnects, through the daemon | → | the whole path | `test/plugin/reconnect-rapid-initial-sync.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A session ends while its initial sync is in the peer-up barrier wait, and the peer reconnects at once | `runOnce` returns after the sync goroutine has exited. The next session's connection carries that session's static routes, queued operations and plugin routes, then exactly one End-of-RIB per negotiated family | Frames the first session's sync wrote on its own connection before it ended stay sent: they were owed to that session. Operations still queued stay queued for the next session, which is the queue's contract |
| AC-2 | The same reconnect, observed through `pendingSync` and `request quiesce` | The second session reports pending from Established until its own marker is written, and `request quiesce` returns after that marker | A peer in a handshake still reports pending through `handshakeInFlight`, unchanged |
| AC-3 | The start door is called twice for one session (the FSM callback and a hold release) | One sync goroutine runs for that session and writes its routes and markers once; the second call starts nothing | A later session of the same peer runs its own sync: one per session is the rule |
| AC-4 | The convergence hold releases a peer whose held session has ended, and the peer then establishes again | The peer's next session runs a full initial sync on its own connection: its routes, then one End-of-RIB per family. The release for the ended session starts no goroutine and writes nothing | A held peer whose session is alive starts at release as today |
| AC-5 | A barrier plugin never acknowledges and the session ends | The barrier wait returns when the session ends, the sync goroutine exits, and the wait reports the session end as its cause, with the timeout warning left unlogged | A live session whose plugin never acknowledges still releases End-of-RIB after 2 seconds with the warning: the bound exists so establishment cannot wedge |
| AC-6 | A forwarded UPDATE is dispatched to a destination whose current session has its gate closed | The UPDATE is parked and is written after that session's gate opens. The predicate takes no lock and is measured at 0 allocations per call | A destination that is not Established is not held, and the replay fence and the policy replay hold still hold as today: both are separate facts |
| AC-7 | A route server calls `AnnounceEOR` for a peer while the current session's marker is owed, then again after it is written | The first call is suppressed with the existing log line and the second writes the marker when the family is not yet claimed | `manual-eor` peers get the operator's marker once the automatic one is withheld, as today |
| AC-8 | A session reaches Established, stays up at least `reconnectMax`, and ends in any error | The wait before the next attempt uses `reconnectMin` as its base and `prefixTeardownCount` is zero before the prefix decision | `ErrTeardown`, the administrative wake and hold, and an inbound connection reset the delay as today, stable or not: an operator or the peer asked for the restart |
| AC-9 | A session reaches Established, stays up less than `reconnectMax`, and ends in an error other than `ErrTeardown` | The wait uses the current delay and the delay then doubles, capped at `reconnectMax` | The next cycle still starts with Event 3, which zeroes the ConnectRetryCounter: that is RFC 4271 and a different fact |
| AC-9b | A cycle never reaches Established and lasts longer than `reconnectMax` (a dial that hangs, a passive peer whose first connection fails its handshake after an hour) | The wait uses the current delay and the delay then doubles, capped at `reconnectMax`; `prefixTeardownCount` keeps its value | The first cycle of a peer waits `reconnectMin`, because that is the delay it starts with |
| AC-10 | A prefix-limit teardown under `reconnect timer` ends a session that stayed in Established at least `reconnectMax` | The wait is the family's base idle timeout, and the warning logs `teardown_count` 1 | Teardowns of sessions shorter than `reconnectMax` still double the wait up to one hour |
| AC-11 | A passive peer with a local address is added during the standalone start, after the listeners started and before the barrier opens | A TCP connection to that address and port is accepted, waits for the barrier, and reaches the peer's session once the barrier opens | An active-only peer gets no listener, and a pair that already has one keeps it untouched |
| AC-12 | A passive peer with an MD5 key is added in the same window, on a pair that already has a listener | The listener is rebuilt and its socket is created with that peer's key | A peer without an MD5 key leaves the existing listener in place, as the running path does today |
| AC-13 | The listener for a peer added in that window cannot start | `AddPeer` returns the existing listen error naming the address and port, and the peer is absent from the peer set | A listener failure never aborts the start for a peer added this way: only `startMultiListeners` failures do, as today |
| AC-13b | A peer is added before `StartWithContext`, or after the reactor stopped | The peer is recorded and the reactor holds the same set of listeners as before the call | Peers added before the start get their listener from `startMultiListeners`, which is the start's own pass |
| AC-14 | A reader opens `ai/digests/bgp-reactor.md`, `docs/architecture/behavior/fsm.md` or `docs/architecture/behavior/peer-lifecycle.md` | Each page states the initial sync as a joined per-session goroutine, the stable-session rule with its source anchors, and when `AddPeer` opens a listener | Historical sections of `fsm.md` about ExaBGP stay as they are: they describe another program |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A peer drops its session and reconnects at once | accept or dial, OPEN exchange, Established, `enterEstablished`, the session's sync, routes then End-of-RIB on the wire | `test/plugin/reconnect-rapid-initial-sync.ci` |
| 2 | An operator's peer loses a session that ran for more than a minute | `Session.Run` error, `Peer.run`, stable-session check, redial after the minimum delay | `test/plugin/reconnect-backoff-stable-reset.ci` |
| 3 | A peer flaps every few seconds | the same path, the delay doubling | `test/plugin/reconnect-backoff-stable-reset.ci` (its first two cycles) |
| 4 | A harness adds a passive peer while the standalone reactor waits for its plugins | `AddPeer`, the listener block of `addPeerLocked`, listener accept, the inbound slot, barrier opening, session | `TestPassivePeerAddedBeforeBarrierGetsListener` |

## 🧪 TDD Test Plan

### Unit Tests

All in package `reactor`, run with `-tags ze_bgp`. The tests marked race run
with `-race -count=20` through `./le job run label unit-pkg quiet command go test -race -count=20 -tags ze_bgp -run <name> ./internal/component/bgp/reactor/`.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunOnceJoinsInitialSyncBeforeReturn` (race) | `internal/component/bgp/reactor/peer_initial_sync_join_test.go` | `runOnce` returns only after the sync goroutine exited | |
| `TestInitialSyncJoinReturnsWhenBarrierNeverOpens` (race) | same | The barrier wait ends at session end, before the fake clock reaches 2 seconds | |
| `TestInitialSyncJoinReturnsDuringQueueDrain` (race) | same | A sync inside the queue drain ends when its sends fail | |
| `TestInitialSyncJoinWithQueuedTeardown` (race) | same | A queued teardown executed by the sync does not deadlock the join | |
| `TestPredecessorSyncWritesNothingOnSuccessor` (race) | same | With the barrier held past a reconnect, the successor's connection carries routes then one marker per family | |
| `TestSuccessorPendingSyncUntilOwnMarker` | same | `pendingSync` of the second session is true until its marker | |
| `TestInitialSyncStartsOncePerSession` | same | A second start for one session spawns nothing | |
| `TestHeldInitialUpdateRefusesEndedSession` | `internal/component/bgp/reactor/update_delay_test.go` | A release for an ended session starts nothing | |
| `TestHeldInitialUpdateStartsTheCurrentSession` | same | A peer re-held under a new session starts that session's sync | |
| `TestBarrierTimeoutStillWarnsOnLiveSession` | `internal/component/bgp/reactor/peer_initial_sync_join_test.go` | The 2 second release and its warning are kept | |
| `TestForwardOrderHoldReadsCurrentSessionGate` | `internal/component/bgp/reactor/forward_initial_sync_order_test.go` | Held while the current session's gate is closed, released when it opens | |
| `TestForwardOrderHoldAllocatesNothing` | same | `testing.AllocsPerRun` reports 0 for the predicate | |
| `TestEstablishedPublishedWithGateClosed` | `internal/component/bgp/reactor/peer_established_gate_test.go` | An observer of Established sees the session's gate closed and the lock-free session equal to `Peer.session` | |
| `TestAnnounceEORDefersToCurrentSessionMarker` | `internal/component/bgp/reactor/reactor_api_forward_test.go` | Suppressed while owed, sent after | |
| `TestRunBackoffRestartsAfterStableSession` | `internal/component/bgp/reactor/peer_run_backoff_test.go` | After `reconnectMax` in Established, the next wait base is `reconnectMin` | |
| `TestRunBackoffDoublesAfterShortEstablishedSession` | same | A session shorter than `reconnectMax` doubles the delay | |
| `TestRunBackoffKeepsDelayAfterLongCycleThatNeverEstablished` | same | A cycle longer than `reconnectMax` that never reached Established resets nothing, with a stale `establishedAt` from an earlier session present | |
| `TestRunBackoffKeepsCapOnPeerThatNeverComesUp` | same | Ten failed dials at the cap keep the base at `reconnectMax` | |
| `TestRunPrefixTeardownCountRestartsAfterStableSession` | same | The prefix wait returns to its base after a stable session | |
| `TestRunPrefixTeardownCountGrowsOnShortSessions` | same | Teardowns of short sessions still double the prefix wait | |
| `TestRunTeardownStillResetsDelay` | same | `ErrTeardown` after a short session resets the delay, waits nothing and keeps the prefix count | |
| `TestRunTeardownAfterStableSessionClearsPrefixCount` | same | `ErrTeardown` after a stable session also zeroes the prefix count | |
| `TestPassivePeerAddedBeforeBarrierGetsListener` | `internal/component/bgp/reactor/reactor_listener_start_window_test.go` | A connection to the added peer's pair is accepted before the barrier opens | |
| `TestPassivePeerAddedBeforeBarrierHoldsInboundUntilBarrier` | same | That connection reaches a session only after the barrier | |
| `TestListenerRebuiltForMD5PeerAddedBeforeBarrier` | same | The listener factory was built with the added peer's key | |
| `TestAddPeerBeforeBarrierListenerFailureRollsBack` | same | `AddPeer` returns the listen error and the peer is absent | |
| `TestAddPeerBeforeStartOpensNoListener` | same | An add before `StartWithContext` opens nothing | |
| `TestAddPeerAfterStopOpensNoListener` | same | An add after a stop or an aborted start opens nothing | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Time in Established against `reconnectMax` | 0 to unbounded | `reconnectMax` minus 1 nanosecond keeps the delay; exactly `reconnectMax` restarts it | N/A | N/A |
| `prefixTeardownCount` | 0 to 60 | 60 (`maxPrefixTeardownCount`, unchanged) | N/A | 61 is capped to 60, unchanged |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reconnect-rapid-initial-sync` | `test/plugin/reconnect-rapid-initial-sync.ci` | A peer with static routes and a route-server binding is dropped by the test peer right after Established and reconnects at once, three times. Each connection's wire shows the static routes and then one End-of-RIB per family | |
| `reconnect-backoff-stable-reset` | `test/plugin/reconnect-backoff-stable-reset.ci` | The test peer ends two sessions a few seconds after Established, then holds the third for more than 60 seconds and ends it. The daemon's "timing: backoff starting" lines show the delay doubling for the first two and the minimum after the third | |
| `prefix-teardown-reconnect-backoff` | `test/plugin/prefix-teardown-reconnect-backoff.ci` (existing) | Re-run unchanged: it is a user of the reconnect backoff | |
| `reconnect-static`, `rib-reconnect`, `eor-per-family`, `role-otc-rs-withdraw-eor`, `quiesce-barrier` | `test/plugin/` (existing) | Re-run unchanged: they read the initial sync's wire order across a reconnect | |

Both new `.ci` files are written through the `ze-test` skill. Neither is
claimed to discriminate the fix unless a revert run goes red (R-9).

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rapid-reconnect-initial-sync-frr` | `test/interop/scenarios/` | FRR | After FRR clears the session three times in a row, FRR holds exactly the prefixes Ze originates, each once, and reports End-of-RIB received for every negotiated family on the last session | |

## Files to Modify
- `internal/component/bgp/reactor/session.go` - the session's initial-sync fields and its start latch; the `initialReplay` comment
- `internal/component/bgp/reactor/peer.go` - delete the two Peer flags; `setState`, `shouldQueue`, `queueGateClosedLocked`, `forwardOrderHold`, `initialSyncInProgress`, `pendingSync`, `teardownOf`, `initialReplayToken`, `waitPeerUpBarrier`; the lock-free established session; the stale `prefixTeardownCount` field comment
- `internal/component/bgp/reactor/peer_initial_sync.go` - `sendInitialRoutes` and its helpers take the session
- `internal/component/bgp/reactor/peer_run.go` - the join in the `runOnce` defer; the stable-session check in `run`; the header comment on the backoff
- `internal/component/bgp/reactor/source_policy_reload.go` - `enterEstablished` takes the session
- `internal/component/bgp/reactor/update_delay.go` - the hold carries the session
- `internal/component/bgp/reactor/peer_send.go`, `reactor_api_forward.go`, `session_ownership.go` - readers of the flags
- `internal/component/bgp/reactor/reactor.go`, `reactor_peers.go` - the listeners-started fact (`startMultiListeners`, `stopAllListeners`) and the condition of the listener block in `addPeerLocked`, with its comment
- `internal/component/bgp/reactor/*_test.go` - the tests that set the Peer flags directly (R-3)
- `ai/digests/bgp-reactor.md`, `docs/architecture/behavior/fsm.md`, `docs/architecture/behavior/peer-lifecycle.md`
- `plan/journal/late-write-lands-on-the-successor.md`, `plan/journal/state-read-after-teardown-reset-it.md`, `plan/journal/unwired-feature.md` - the three rows marked fixed in the phase that fixes each

## Files to Create
- `internal/component/bgp/reactor/peer_initial_sync_join_test.go`
- `internal/component/bgp/reactor/peer_run_backoff_test.go`
- `internal/component/bgp/reactor/reactor_listener_start_window_test.go`
- `test/plugin/reconnect-rapid-initial-sync.ci`
- `test/plugin/reconnect-backoff-stable-reset.ci`
- `test/interop/scenarios/bgp-rapid-reconnect-initial-sync-frr/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No leaf and no RPC is added. The owner ruled out a new knob for the stable time |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC or API. The two `.ci` files cover the behaviour through the daemon |
| Pipe completeness | N-A | No command output changes |
| Env var registration | N-A | No env var. `ze.bgp.announce.delay` is read as today |
| Doctor check for runtime dependencies | N-A | No new path, socket, port or module. The listeners are the configured ones, opened earlier than before in one start mode |
| Doctor code per problem class | No | No new refusal, `Warn` or `Error` line. A listener failure in the start window returns the existing listen error through `AddPeer`, the route it already takes while running; the implementer confirms that route already carries a doctor code and names it in the phase 4 commit, or adds the row to the journal if it does not |
| Prometheus counters/metrics | No | `ze_peer_backoff_seconds` keeps its name, labels and meaning |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | No feature level changes; `./le feature report` is checked at closure for the BGP session feature's evidence list |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-peering.md`: one paragraph on when the reconnect delay restarts, beside the prefix `reconnect` table, phase 3 |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4724.md`: the row for the End-of-RIB after the initial update gains the rapid-reconnect test as evidence, with its discrimination record, phase 2 |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it states reconnect damping; otherwise named as unaffected at phase 3 |
| 12 | Internal architecture changed? | Yes | `ai/digests/bgp-reactor.md` (goroutine model gains the initial sync, its join, and what the digest is silent on that this spec touches: `forwardGeneration` against the session's gate), `docs/architecture/behavior/fsm.md` (the damping section gains the stable-session restart; the `sessionDown` section gains the join), `docs/architecture/behavior/peer-lifecycle.md` (backoff table, backoff defaults, initial sync steps, listener start) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/release/fixes/spec-single-owner-lifecycle-2-bgp-session.md` at phase 1 and name each page here. Known from reading: `fsm.md` and `peer-lifecycle.md` anchor `peer_run.go`, `peer.go` and `peer_initial_sync.go`. `docs/architecture/core-design.md` is declared by the `// Design:` header of `peer_initial_sync.go` and is named here as unaffected: a grep of it for `sendInitialRoutes` and "initial sync" on 2026-11-11 found no line, so it describes neither the goroutine nor its flags; the implementer re-checks its reactor section at phase 1 |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The change adds no syntax; the pseudocode of the reconnect loop in `peer-lifecycle.md` is rewritten as prose or a table in phase 3 |

## Implementation Steps

Each phase is one commit that compiles and passes its own tests.

1. **Phase: Wiring (MANDATORY FIRST)** -- the session carries its sync
   - Tests: `TestRunOnceJoinsInitialSyncBeforeReturn`, `TestInitialSyncStartsOncePerSession`, `TestEstablishedPublishedWithGateClosed`, `TestForwardOrderHoldReadsCurrentSessionGate`, `TestForwardOrderHoldAllocatesNothing`, `TestAnnounceEORDefersToCurrentSessionMarker`, `TestSuccessorPendingSyncUntilOwnMarker`
   - Files: `session.go`, `peer.go`, `peer_initial_sync.go`, `source_policy_reload.go`, `update_delay.go`, `peer_send.go`, `reactor_api_forward.go`, `session_ownership.go`, the reactor tests of R-3
   - Order inside the phase: delete the two Peer fields, add the session fields and the latch, move every reader until the package compiles, give `sendInitialRoutes` and `enterEstablished` the session, add the join to the `runOnce` defer
   - Verify: the tests above fail before the change and pass after; the existing reactor package tests pass under `-race`
2. **Phase: the sync ends with its session** -- the barrier wait and the hold
   - Tests: `TestInitialSyncJoinReturnsWhenBarrierNeverOpens`, `TestInitialSyncJoinReturnsDuringQueueDrain`, `TestInitialSyncJoinWithQueuedTeardown`, `TestPredecessorSyncWritesNothingOnSuccessor`, `TestBarrierTimeoutStillWarnsOnLiveSession`, `TestHeldInitialUpdateRefusesEndedSession`, `TestHeldInitialUpdateStartsTheCurrentSession`; `test/plugin/reconnect-rapid-initial-sync.ci`; the interop scenario
   - Files: `peer.go` (`waitPeerUpBarrier`), `peer_initial_sync.go`, `update_delay.go`; `ai/digests/bgp-reactor.md`, the initial-sync sections of `fsm.md` and `peer-lifecycle.md`; `rfc/short/rfc4724.md`; the journal row of 2026-08-31
   - Verify: the race tests pass at `-count=20`; the revert run of the `.ci` and the scenario is recorded (R-9)
   - Phases 1 and 2 may land as one commit if phase 1 alone would leave the join able to wait 2 seconds on every session end: the implementer measures and says which
3. **Phase: the backoff forgets after a stable session**
   - Tests: the eight `TestRun...` tests; `test/plugin/reconnect-backoff-stable-reset.ci`; re-run `prefix-teardown-reconnect-backoff.ci`
   - Files: `peer_run.go`, the `prefixTeardownCount` comment in `peer.go`; `peer-lifecycle.md`, `fsm.md`, `docs/guide/bgp-peering.md`, `docs/comparison.md`; the journal row of 2026-10-11
   - Verify: tests fail, then pass; the delay lines in the `.ci` read 5, 10, then 5 seconds as bases
4. **Phase: listeners in the start window**
   - Tests: the six listener tests
   - Files: `reactor.go`, `reactor_peers.go`; the listener paragraph of `peer-lifecycle.md` and of the digest; the journal row of 2026-10-13
   - Verify: tests fail, then pass; the existing listener and AddPeer tests pass

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No function reached from `sendInitialRoutes` reads `Peer.session` to choose where to write; each uses the goroutine's session. The join sits after `sessionDown` and before the claims reset |
| Naming | The session fields keep the meaning of the Peer fields they replace; no field named for this spec |
| Data flow | `forwardOrderHold` takes no lock; the lock-free session is written at two sites only |
| Operator errors and doctor codes | No new message; the barrier listener failure reuses the existing listen error |
| Rule: `ai/rules/no-layering.md` | The Peer flags are gone, not kept beside the session's |
| Rule: `ai/rules/stale-comments.md` | The comments on `setState`, `runHeldInitialUpdates`, the `runOnce` defer, the `run` header, `prefixTeardownCount` and `Session.initialReplay` describe the new behaviour |
| Rule: `ai/rules/rfc-compliance.md` | The stable-session check carries the RFC 4271 Section 8.2.2 quote and the join carries the RFC 4724 Section 4 quote, byte-exact |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No Peer-level initial-sync flag | `grep -n "sendingInitialRoutes\|initialSyncEOROwed" internal/component/bgp/reactor/peer.go` shows no field on `Peer` |
| The sync is joined | `TestRunOnceJoinsInitialSyncBeforeReturn` passes under `-race -count=20` |
| The hold predicate allocates nothing | `TestForwardOrderHoldAllocatesNothing` passes |
| The backoff restarts | `TestRunBackoffRestartsAfterStableSession` and the `.ci` pass |
| The listener exists | `TestPassivePeerAddedBeforeBarrierGetsListener` passes |
| Three journal rows fixed | `grep -n "2026-08-31\|2026-10-11\|2026-10-13"` in the three class files shows `fixed:` with the commit |
| Pages updated | `./le doc check all` passes for the three pages |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A peer that flaps on purpose cannot make Ze spawn more than one sync goroutine per session, and cannot make the join block |
| Remote-driven backoff | A peer cannot shorten Ze's redial delay by ending sessions quickly; only a session that stayed up `reconnectMax`, its own inbound connection or an operator does |
| Listener exposure | `addPeerLocked` opens only the added peer's configured address and port, with the same MD5 and GTSM factory, and a connection accepted before the barrier waits in the peer's slot |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| The join deadlocks (A-1 or A-2 broken) | STOP. Report the blocked wait and its holder; the design of the join changes, not the test |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The upstream evidence for the backoff rule, read on 2026-11-11.

| Implementation | Function (file) | Deciding statement | What it does |
|----------------|-----------------|--------------------|--------------|
| BIRD 3.2.1 | `bgp_update_startup_delay` (`proto/bgp/bgp.c`) | `if (p->last_proto_error && ((current_time() - p->last_proto_error) >= cf->error_amnesia_time S)) p->startup_delay = 0;` then `p->startup_delay = MIN(2 * p->startup_delay, cf->error_delay_time_max);` | Forgets when the previous error is at least the forget time old, else doubles. Defaults in `config.Y`: forget 300, wait 60 to 300 |
| BIRD 3.2.1 | `bgp_rx_notification` (`proto/bgp/packets.c`) | `int err = (code != 6);` and `if (err) { bgp_update_startup_delay(p); ...` | A received Cease does not grow the delay |
| BIRD 3.2.1 | `bgp_sock_err` (`proto/bgp/bgp.c`) | stores the error and never calls `bgp_update_startup_delay` | A TCP failure does not grow the delay |
| BIRD 3.2.1 | `bgp_conn_enter_established_state` (`proto/bgp/bgp.c`) | no write of `startup_delay` or `last_proto_error` | Reaching Established resets nothing |
| FRR stable/10.8 | `bgp_establish` (`bgpd/bgp_fsm.c`) | `peer->v_start = BGP_INIT_START_TIMER;` under "Clear start timer value to default." | Resets to 1 second at Established |
| FRR stable/10.8 | `bgp_stop_with_error` (`bgpd/bgp_fsm.c`) | `peer->v_start *= 2;` then `if (peer->v_start >= (60 * 2)) peer->v_start = (60 * 2);` | Doubles to a cap of 120 seconds |
| FRR stable/10.8 | `bgp_stop_with_notify` (`bgpd/bgp_fsm.c`) | `peer->v_start = BGP_INIT_START_TIMER;` | Resets when FRR itself sends the NOTIFICATION |

  The two disagree and the RFC is silent, so the owner decided: "D2 - BIRD is
  better". The FRR files were read from a job scratch copy that is removed with
  the job; re-fetch `bgpd/bgp_fsm.c` from FRR `stable/10.8` to re-read them.

- BIRD's own defaults make the forget time equal to the maximum wait, so BIRD
  at its cap forgets on the next error. Ze differs in one way that makes this
  matter more: Ze grows the delay on a failed dial, so a peer that is simply
  down would cycle 5, 10, 20, 40, 60, 5 seconds for ever, and with the 0.75 to
  1.0 jitter the restart would happen on some laps and not others.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| late-write-lands-on-the-successor | 2026-08-31 | `sendInitialRoutes` outlives teardown |
| state-read-after-teardown-reset-it | 2026-10-11 | `Peer.run` backoff reset only on stop |
| unwired-feature | 2026-10-13 | `addPeerLocked` passive peer gets no listener in standalone start |

## Rows found already fixed

None of the three. The row of 2026-08-31 names a 2 second `waitForAPISync`
wait that no longer exists; the defect is unchanged, and the wait it now needs
is the 2 second peer-up barrier. The row's text is corrected when it is marked
fixed.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The flags move to `Session` and the sync is joined | A generation compared before each store, as the journal row proposed | A comparison protects each store somebody remembered to guard. A field on the session cannot be written by another session's goroutine, and the join removes the goroutine. `forwardGeneration` already exists for sources and a second one for destinations would sit beside it |
| A start latch on the session decides whether a goroutine exists | Keep the compare-and-swap on the queue gate as the start guard | The gate opens before the goroutine ends (the End-of-RIB phase runs with the gate open), so the gate cannot say whether there is a goroutine to join. The hold can also start a sync after the session ended, which only a latch the defer closes can refuse |
| The dead session's writes are refused by its FSM state and closed connection, and by the goroutine using only its own session | "The dead session is sealed, so its writes refuse" (the design as first stated) | Read at `seal`: only a teardown, a stop and a failed write seal. A session the peer ended is not sealed. Sealing in the `runOnce` defer was considered and not needed: the join runs there and nothing writes after it |
| `forwardOrderHold` reads an atomic pointer to the established session | Read `Peer.session` under the peer lock; or keep one Peer-level atomic for the hold only | The peer lock is forbidden under the forward worker's overflow lock (`forward_pool.go`). Keeping a Peer-level atomic keeps the defect's shape. The pointer is one more atomic load per call and has two writers, both already bracketing the flags |
| The delay and the prefix count restart when the ended cycle reached Established and stayed there at least `reconnectMax` | (a) BIRD's literal measure, previous error to this error, with the forget time equal to the maximum; (b) the whole cycle's duration (`sessionElapsed`); (c) FRR's rule, reset on reaching Established | This is BIRD's damping intent with a different measure: a session that was stable clears the penalty, a flapping one stays damped. BIRD's `bgp_update_startup_delay` and FRR's `bgp_establish` (Design Insights) are the evidence that the two disagree, and the owner chose BIRD. (a) is rejected because at the cap the wait is itself about the maximum and Ze grows on failed dials, so a peer that never came up would restart at 5 seconds, at random under the jitter. (b) is rejected because a dial that hangs for a minute, or a passive peer's wait for a connection, would count as stable. (c) is what the owner decided against: reaching Established does not reset by itself. Thomas confirmed it on 2026-11-11: "the refined D2 rule stands" (A-6) |
| The stable time starts at `EstablishedAt()` | `sessionElapsed` | The timestamp is the fact the rule is about, is stamped from the peer's injectable clock, and already feeds uptime. It is read only when `cycleReachedEstablished` is true, because nothing clears it between sessions |
| The stable time is `Peer.reconnectMax`, 60 seconds | "The configured reconnect maximum" | Nothing configures it today and this spec adds no knob. The field is the single declaration; if a leaf is added later the rule follows it |
| The check runs before the `ErrTeardown` branch | Run it only for other errors | An operator's teardown of a stable session should clear the prefix count like any other end of a stable session. The delay is reset by that branch in both cases, as today |
| The inbound-connection and administrative resets stay | Remove them so the stable session is the only reset | They are not session-ending errors: the peer or the operator asked for a start. Accepted by the main thread |
| `addPeerLocked` opens the listener once the listeners have started | One reconcile function run when the peer-start barrier opens, with the MD5 set each listener was built with recorded and compared | The rejected design needs a recorded key set per listener, a second call site in each start mode, and a new failure mode where a listener error at the barrier aborts the start after `AddPeer` already answered success. The chosen one changes one condition and adds one fact with two writers. Read in the code: `startMultiListeners` already opens listeners with `running` false, both arms of the block (start, MD5 rebuild) need only the reactor context and the peer set, `startListenerForAddressPort` refuses while `stopping`, and `stopAllListeners` runs on stop and on an aborted start. No function read requires a listener to be absent before the barrier |

## Known Limitations

- Ze grows the reconnect delay on a Cease and on a TCP failure, and BIRD does not (R-6). This spec does not change which session ends grow the delay.
- `Peer.initialUpdateOwed` (the replay fence), `sendingConfigStatic`, the End-of-RIB claims and `staticWire` stay on the `Peer`. They are per-session facts written by the sync or beside it, and the join makes a late write to them impossible. Moving them is not needed to fix the row.
- The convergence hold still keys its expected set by `*Peer`; only the held set gains the session.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

- Above the join in `runOnce` and above the start latch: the RFC 4724 Section 4 sentence quoted in Required Reading.
- Above the stable-session check in `run`: the RFC 4271 Section 8.2.2 sentence, and a line naming BIRD's `bgp_update_startup_delay` as the model.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
