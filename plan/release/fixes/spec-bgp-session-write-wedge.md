# Spec: a BGP session blocked in a socket write never ends

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped lifecycle defect: a peer that stops reading wedges its session for good, so the RFC 9687 send hold timer and the RFC 4271 hold timer never close the connection and the peer never reconnects |
| RFC gaps | - |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A session write that carries no deadline can block in the kernel for as long
as the peer keeps its TCP receive window shut while still answering the
zero-window probes. The writer holds the session's `writeMu` for that whole
time. Every path that ends the session waits for `writeMu` before it closes the
socket, and closing the socket is the only thing that would unblock the writer.
So the session never ends: `Session.Run` never returns, the `runOnce`
session-end defer never runs, and the peer stays in a dead session without
reconnecting.

Found 2026-11-14 while reviewing the initial-sync join (commit 08fe903803) for
`spec-single-owner-lifecycle-2-bgp-session`. It predates that commit; the join
does not cause it. Verified by reading the producers below, not by a run.

The goal: every session write is bounded, and every close path closes the
socket without first waiting on a writer the close would unblock, so a
session whose peer stopped reading ends within its hold or send hold time and
the peer reconnects.

Stated goals, each owed a Goal Validation row at closure:

| # | Goal |
|---|------|
| G-1 | Every write to a session socket is bounded by a write budget |
| G-2 | No close path waits on the write path with the socket still open for longer than a stated bound |
| G-3 | A session whose peer stopped reading ends, the FSM reaches Idle and the peer reconnects |
| G-4 | A healthy session behaves as before: the NOTIFICATION is delivered before the close, and a long initial sync is not cut short |
| G-5 | Ze drops and re-establishes against a real BIRD that stops reading |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/le-tooling.md`, `ai/doors/config-yang.md`, `ai/doors/core.md` - the constraints every change in these subsystems keeps
  → Constraint: the protocol peer's contract is the RFC first; the engine holds no policy; no plugin package is named from the reactor
  → Constraint: a test-peer option is one arm in `peerOptionArms`, the registry `ze-peer` derives its option set from; an env key is registered once, beside its reader's component; a diagnostic code is declared once in `internal/core/diagnostic/codes.go` and referenced by its constant
- [ ] `docs/architecture/core-design.md` - "Write Deadline", the close sequence, "Send Hold Timer (RFC 9687)"
  → Constraint: the page says only forward-pool writes carry a deadline and gives the close order as flush, `CloseWrite`, drain, `Close`. Both statements become wrong with this spec and are edited in the phase that changes them
- [ ] `docs/architecture/bgp/structural-forwarding.md` - the route-server fast path and request-scoped writes
  → Constraint: the fast path runs on the SOURCE peer's read goroutine and MUST NOT wait: it takes the destination's write path by `TryLock` only
- [ ] `docs/architecture/forward-congestion-pool.md` - the flush hand-off to the forward worker
  → Constraint: a destination whose write path is contended is handed to its forward worker (`handOffForwardFlush`); that hand-off stays
- [ ] `plan/learned/DESIGN-HISTORY.md` - BGP engine invariants
  → Constraint: "Session lock order is `s.mu` then `s.writeMu`, and `closeConn` must take `writeMu` for its final Flush inside the `s.mu` section": `bufWriter` is never touched without `writeMu`
  → Constraint: the keepalive timer is a concurrent writer, so `writeMu` stays required
- [ ] `internal/component/bgp/reactor/session.go`, the `OnHoldTimerExpires` comment - owner ruling of 2026-08-03
  → Constraint: no hold-expiry reprieve. This spec adds no grace window: the bound it adds to the NOTIFICATION is the existing one-second `holdExpiryNotifyDeadline`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9687.md` and `rfc/full/rfc9687.txt` Section 5 - the send hold timer exists for exactly this failure
  → Constraint: Section 5: "If the local system does not send any BGP messages within the period specified in SendHoldTime, then a NOTIFICATION message with the "Send Hold Timer Expired" Error Code MAY be sent and the BGP connection MUST be closed."
  → Constraint: requirement rows RFC9687-4.3-5 ("drops the TCP connection") and RFC9687-4.3-7 ("changes its state to Idle") are the ones the wedge breaks
- [ ] `rfc/short/rfc4271.md` and `rfc/full/rfc4271.txt` Section 6.5 and Section 8.2.2 (Established, Event 10)
  → Constraint: Section 6.5: "If a system does not receive successive KEEPALIVE, UPDATE, and/or NOTIFICATION messages within the period specified in the Hold Time field of the OPEN message, then the NOTIFICATION message with the Hold Timer Expired Error Code is sent and the BGP connection is closed."
  → Constraint: Section 8.2.2, Established: "If the HoldTimer_Expires event occurs (Event 10), the local system: ... - drops the TCP connection, ... - changes its state to Idle."

### Peer implementations (owner rule: read before a protocol design is approved)
- [ ] FRR 10.5.3 (`cd39d029a4`), `bgpd/bgp_packet.c` `bgp_write_notify`, `bgp_packet_add`; `bgpd/bgp_io.c` `bgp_write`
  → Decision: FRR writes on a non-blocking socket. Its NOTIFICATION is one write attempt ("socket is in nonblocking mode, if we can't deliver the NOTIFY, well, we only care about getting a clean shutdown at this point."), after which the session stops whatever the result. Queued output is discarded, never flushed. So a close that skips the NOTIFICATION and the final flush behind a stuck writer interoperates
  → Decision: FRR's send hold bound is 2 x hold time, checked on enqueue; it bounds no individual write
- [ ] BIRD v3.3.0 (`10682b7bef`), `proto/bgp/bgp.c` `bgp_send_hold_timeout`, `bgp_conn_enter_close_state`
  → Decision: BIRD sends no NOTIFICATION on send hold expiry ("Like bgp_error() but without NOTIFICATION") and closes at once. After a hold expiry it waits at most 10 seconds for the NOTIFICATION to leave ("Timeout for CLOSE state, if we cannot send notification soon then we just hangup")
  → Decision: neither daemon sets `TCP_USER_TIMEOUT`, `SO_SNDTIMEO` or `SO_LINGER` on the BGP socket, so interoperability gives no reason to add one; the bound is in the daemon

**Key insights:** (minimal context to resume after compaction)
- The wedge is one shape in many places: a writer blocked in the kernel holds `writeMu`, and a close path waits on `writeMu` with the socket open
- Nine kinds of writer carry no deadline; three are bounded. One acquisition of the write path that always arms a budget makes the bound a property of the path, not of each caller
- `ceaseAfterPanic` already has the right close shape (bounded wait, then close without the NOTIFICATION). The spec makes it the only close shape
- `closeConn` holds `s.mu` across its wait, and the hold timer callback holds `openMu` across its wait, so the wedge spreads to readers of the session, to a source peer's read loop and to a policy reload
- `writeMu` is a channel-token mutex (`sessionWriteMutex`) that already has a cancellable acquisition, so a bounded wait needs no polling loop
- Design approved by Thomas on 2026-11-14: "interrupt, never wait", and the route-server fast path gets a budget armed once per batch per destination

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - the queued-route drain and the config-route sends call `sendUpdateWithSplit(context.Background(), ...)`; the held phases write through `sendUpdateHeld` under `holdWrites` (`writeMu.Lock`), one hold across many messages
- [ ] `internal/component/bgp/reactor/peer_static_wire.go` - static route announces and withdrawals call `sendUpdateWithSplit` / `sendBodyWithSplit` with `context.Background()`
- [ ] `internal/component/bgp/reactor/peer_send.go` - `sendUpdateWithSplit` passes the context through `sendUpdateOwnedWithSplit` to `sendUpdateCounted` for every chunk; `sendQueuedRefreshMarker` writes through `SendRawMessage`
- [ ] `internal/component/bgp/reactor/session_write.go` - `sendUpdateCounted` calls `requestWriteDeadline` only when `ctx.Done() != nil`, so a background context gets no `SetWriteDeadline`; `requestWriteDeadline` itself sets one only from `ctx.Deadline()` or on cancellation. `sendUpdateHeld`, `SendUpdate`, `SendAnnounce`, `sendWithdraw`, `sendRawUpdateBody` and `SendRawMessage` set none. `writeMessageHeld` (KEEPALIVE, OPEN, NOTIFICATION) does set one, but `writeMessageWithin` takes `writeMu.Lock` first, so it queues behind the stuck writer. `flushWrites` retires the session through `retireWrite` when a flush fails, and resets the send hold timer only after a flush that carried bytes
- [ ] `internal/component/bgp/reactor/session_write_mutex.go` - `sessionWriteMutex` is a one-token channel with `Lock`, `LockContext`, `TryLock` and `Unlock`; a cancelled waiter never acquires later
- [ ] `internal/component/bgp/reactor/session_connection.go` - `closeConn` takes `s.mu` in write mode, then `flushForClose` takes `writeMu.Lock` and flushes with no deadline, and only then calls `CloseWrite`, drains for 100 ms and calls `Close`. `ceaseAfterPanic` is the one path that bounds the wait (`tryNotificationWithin`) and closes the socket first, and its comment names this exact wedge. `teardownSession` sends its NOTIFICATION through `logNotifyErr` (blocking) and then calls `closeConn`
- [ ] `internal/component/bgp/reactor/session.go` - the hold timer callback takes `openMu`, then sends its NOTIFICATION through `sendNotificationWithin`, whose one-second deadline bounds the write but not the wait for `writeMu`, then signals `errChan`; `Run`'s cancel goroutine and its deferred `closeConn` both reach `flushForClose`
- [ ] `internal/component/bgp/reactor/session_write.go` `sendHoldTimerExpired` - `logNotifyErr` (blocking `writeMu`), then `closeConn`: the RFC 9687 close also wedges
- [ ] `internal/component/bgp/reactor/forward_pool.go` `fwdBatchHandler` and `session_write.go` `flushFwdDirty` - set a write deadline (`fwdWriteDeadline`, 30 s default, env `ze.fwd.write.deadline`) after taking `writeMu`
- [ ] `internal/component/bgp/reactor/forward_rs_direct.go` `tryDirectWriteNoFlush` - takes the destination's `writeMu` by `TryLock` and writes with no deadline (its own comment: "Does NOT flush or set write deadlines"); the deadline is set only later, in `flushFwdDirty`. The session `bufWriter` is 16 KiB, so a batch that overflows it writes to the socket inside the buffered write, on the source session's read goroutine
- [ ] `internal/component/bgp/reactor/session_paths_limit.go` `commitUpdateSender.SendUpdate` - calls `sendUpdateCounted(context.Background(), ...)`: one more unbounded writer
- [ ] `internal/component/bgp/reactor/peer_admin.go` `rejectAdminDownConnection` - writes the Connection Rejected NOTIFICATION straight to a raw connection with no deadline, then closes it
- [ ] `internal/component/config/environment.go` - registers `ze.fwd.write.deadline` (duration, default 30s) as "TCP write deadline for forward pool batch writes"; unlike its neighbors it has no YANG leaf
- [ ] `internal/test/peer/expect_keys.go`, `internal/test/peer/peer.go` - `peerOptionArms` holds every option `ze-peer` claims; `silent` stops the automatic KEEPALIVE reply and keeps reading. `runMessageLoop` reads with a 100 ms read deadline in a loop; no option makes it stop reading
- [ ] `internal/core/diagnostic/codes.go` - codes are declared once as constants with a title, description and examples; none exists for a BGP session closed because the peer accepted no data. `CodeDoctorRecentEvents` reports recent transient events

No TCP user timeout or keepalive option on the BGP socket breaks the wait: none
is set under `internal/component/bgp`, keepalive probes do not fire while data
is outstanding, and a live peer answers zero-window probes.

With default timers the send hold timer does not fire for at least 8 minutes
(`sendHoldDuration`: the configured value, or the larger of 8 minutes and twice
the receive hold time). A peer that stops reading but keeps sending KEEPALIVEs
never trips the receive hold timer.

### Writer census

Search: `writeMu.(Lock|TryLock|Unlock)`, `SetWriteDeadline`, `bufWriter.(Write|Flush)`, `conn.Write(`, `.holdWrites(` and every caller of a `send*` / `write*` entry point, over `internal/component/bgp` and `internal/core/bgp`, tests excluded. Control hit: `fwdBatchHandler` (lock, then `SetWriteDeadline(fwdWriteDeadline())`). BMP and RPKI own their own sockets and are out of scope.

| Writer | Lock taken | Bounded today | Budget source |
|--------|-----------|---------------|---------------|
| `writeMessageHeld` through `writeMessage` / `writeMessageWithin`: KEEPALIVE, OPEN, every NOTIFICATION through `logNotifyErr` | blocking | write yes, lock wait no | `controlWriteDeadline` (hold/3 clamped to 10 s..30 s); 1 s for hold expiry |
| `tryWriteMessageWithin` (`ceaseAfterPanic` only) | `TryLock` poll every 5 ms | yes | 1 s |
| `fwdBatchHandler`, including the handed-off flush | blocking | write yes, lock wait no | `fwdWriteDeadline` |
| `flushFwdDirty` | `TryLock` | yes, the flush only | `fwdWriteDeadline` |
| `tryDirectWriteNoFlush` | `TryLock` | **no** | - |
| `sendUpdateCounted` with a cancellable context (API batch) | `LockContext` | yes | the request's deadline, or now on cancel |
| `sendUpdateCounted` with `context.Background()`: `SendUpdate`, initial-sync drain, static routes, End-of-RIB, the paths-limit commit sender (`session_paths_limit.go`), the RIB commit sender | `LockContext` that never cancels | **no** | - |
| `sendUpdateHeld` under `holdWrites` | blocking, held across many messages | **no** | - |
| `SendAnnounce`, `sendWithdraw`, `sendRawUpdateBody` | blocking | **no** | - |
| `SendRawMessage`, all three modes, including ROUTE-REFRESH and its markers | blocking | **no** | - |
| `flushForClose` | blocking, under `s.mu` | **no** | - |
| `rejectConnectionCollision` (raw connection, no session) | none | yes | 30 s |
| `rejectAdminDownConnection` (raw connection, no session) | none | **no** | - |

Takers of `writeMu` that write nothing and still queue behind a stuck writer: the policy reload barrier and replay (`policy_reload.go`, `source_policy_reload.go`), the paths-limit bound readers, `negotiateWith`, `setSendCtxID`, `restoreHotSwappableSettings`, `connectionEstablished`.

### Close-path census

| Path | What it holds while it waits on the write path | Socket closed before the wait |
|------|------------------------------------------------|-------------------------------|
| `closeConn` | `s.mu` in write mode | no |
| Hold timer callback | `openMu` | no; `errChan` is never signalled |
| `sendHoldTimerExpired` | nothing, then `closeConn` | no |
| `teardownSession` (operator teardown, peer removal on reload, daemon stop, BFD down, congestion teardown) | nothing, then `closeConn` | no |
| `closeCollision` / `CloseWithNotification` | `openMu` | no |
| Read-loop error paths (`logNotifyErr` then `closeConn`), a received NOTIFICATION, `handleConnectionClose` | the read goroutine itself, so `Run` cannot return | no |
| `Run`'s cancel goroutine and `Run`'s deferred `closeConn` | `s.mu` | no |
| `ceaseAfterPanic` | nothing: bounded wait | yes |
| `retireWrite` (runs under `writeMu`, after a write returned) | not applicable | closes at once |

**Behavior to preserve:**
- The NOTIFICATION is still attempted before the close when the write path is free, and the FIN-then-drain close still lets the peer read it
- The forward rails keep their `writeMu` discipline: `TryLock` on the fast path and in `flushFwdDirty`, the hand-off to the forward worker on contention, the deadline armed after the lock is taken
- The lock order `s.mu`, then `writeMu`, then `sendHoldMu`; `sendUpdateHeld` never takes `s.mu`
- `bufWriter`, `writeBuf`, the connection fields and the `sent*` fields are touched only under `writeMu`
- The close reason is first-wins and is recorded before any NOTIFICATION I/O
- The send hold timer is reset only by a flush that carried bytes
- A request-scoped write (`requestWriteDeadline`) keeps its own deadline and its cancellation
- No hold-expiry reprieve (owner ruling, 2026-08-03)

**Behavior to change:**
- Every write to a session socket carries a write budget
- No close path waits on `writeMu` with the socket still open beyond a stated bound, and none holds `s.mu` or `openMu` across an unbounded wait
- A close behind a stuck writer skips the NOTIFICATION and the final flush, and says so in the log

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Session write mutex | `session_write_mutex.go` `sessionWriteMutex` (`Lock`, `LockContext`, `TryLock`) | One-token channel mutex with a cancellable acquisition | Extends: gains the one acquisition every writer uses, which arms the budget, and a wait bounded by a duration. The 5 ms `TryLock` poll of `tryWriteMessageWithin` is replaced by it |
| Request write deadline | `session_write.go` `requestWriteDeadline` | Binds a socket write to its request's context deadline and cancellation | Extends: the budget is the earlier of the request's deadline and the session budget, so a background context is no longer unbounded |
| Control write deadline | `session_write.go` `writeMessageHeld`, `controlWriteDeadline`, `holdExpiryNotifyDeadline` | Bounds KEEPALIVE, OPEN and NOTIFICATION writes | Extends: the same budgets, now applied to the wait for the write path as well as to the write |
| Forward write deadline | `forward_pool.go` `fwdBatchHandler`, `fwdWriteDeadline`; `session_write.go` `flushFwdDirty` | Bounds forward-pool and dirty-flush writes with `ze.fwd.write.deadline` (30 s) | Extends: this budget becomes the session's write budget for every UPDATE writer. The two hand-set deadlines are replaced by the one acquisition |
| Route-server fast path | `forward_rs_direct.go` `tryDirectWriteNoFlush` | Writes to a destination under `TryLock`, unflushed, with no deadline | Extends: the budget is armed when a destination first receives a write in a batch |
| Bounded write-path wait | `session_write.go` `tryWriteMessageWithin`; `session_connection.go` `ceaseAfterPanic` | Waits for `writeMu` no longer than a deadline, then closes the socket without the NOTIFICATION | Replaces: its shape becomes the one close routine every close path uses, and `ceaseAfterPanic` becomes a caller of it |
| Connection close | `session_connection.go` `closeConn`, `flushForClose` | Flush under `writeMu`, FIN, drain, close, all under `s.mu` | Replaces: the unbounded wait and the unbounded flush go; the FIN-then-drain close stays for the case where the write path came free |
| Write retirement | `session_write.go` `retireWrite` | Seals and closes the connection under `writeMu` after a failed write | Does not apply directly: it runs only once a write returns. It is what a writer interrupted by this spec's close runs next, so it stays as is |
| Send hold timer | `session_write.go` `sendHoldTimerExpired` | RFC 9687 close when nothing was sent for SendHoldTime | Fixed by this spec: today it wedges on `writeMu`; it becomes a caller of the close routine |
| Test-peer option registry | `internal/test/peer/expect_keys.go` `peerOptionArms`; `silent` | One arm per option, with its keys and parser; `silent` stops the peer's writes and keeps its reads | Extends: one new arm, `stop-reading`, the mirror of `silent` |
| Diagnostic codes | `internal/core/diagnostic/codes.go` | One declaration per operator-facing problem class | Extends: one code for a session closed because the peer accepted no data |
| Chaos stall actions | `internal/chaos/engine/action.go` `zero-window`, `slow-read` | Make a simulated peer stop or slow its reads | Does not apply: chaos runs are not the scoped proof of this spec. The functional proof is a test-peer option, the interop proof a paused BIRD |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A route write on the peer's session: the initial sync, a static route, an API route send, a forwarded UPDATE, a ROUTE-REFRESH, a control message
- The session end: hold timer, send hold timer, a received NOTIFICATION, a read error, an operator teardown, a config reload that removes the peer, daemon stop, `Run`'s cancel goroutine

### Transformation Path
1. A writer acquires the write path. The acquisition arms the socket's write deadline to now plus the caller's budget
2. The writer encodes into `writeBuf`, writes to `bufWriter`, flushes. A write that passes its deadline fails, and `retireWrite` seals and closes the connection
3. The writer releases the write path, which clears the deadline. As built, the release clears whatever is armed (`Session.writeBudgetArmed`), on a retired session too: `TestDelayOpenExpiryWriteFailureDeliversOneEvent18` waits for that clear after a failed write. The route-server fast path alone releases without clearing
4. A close path records its reason, waits for the write path for no longer than its bound, and sends its NOTIFICATION if the path came free
5. If the path stayed held, the close path expires the socket's write deadline, which fails a write blocked in the kernel, then closes the socket without the NOTIFICATION and without the final flush
6. The connection fields are cleared under `writeMu` once the path is free; `s.mu` is never held across that wait

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to peer | TCP socket write and close | No |

### Changed-Wire Census

**Search evidence:** no wire format changes. The bytes of every message are unchanged: the fix bounds when a write gives up and when the socket closes. Searched `internal/component/bgp/reactor` for every `WriteTo` and `bufWriter.Write` site reached from the writers in the census above (control hit: `writeMessageHeld`); none is edited for its encoding. What a peer can observe changes in one way only: a session that ends behind a stuck writer closes without a NOTIFICATION. RFC 9687 Section 5 makes that NOTIFICATION a MAY, and FRR and BIRD treat a close with no NOTIFICATION as a session end (`bgp_write_notify`, `bgp_send_hold_timeout`).

### Integration Points
- `sessionWriteMutex` - gains the budget-arming acquisition and the duration-bounded wait
- `requestWriteDeadline` - composes the request deadline with the session budget
- `retireWrite` - unchanged; it is what an interrupted writer runs
- `closeConn` - becomes the bounded close routine; `ceaseAfterPanic`, the two timer expiries and `teardownSession` call it
- `flushFwdDirty` / `appendFwdDirty` - the batch boundary the fast path's budget follows

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | To prove: no writer reaches `bufWriter` except through the one acquisition (grep in the Deliverables Checklist) |
| No unintended coupling (components stay isolated) | No | To prove: the change stays inside `internal/component/bgp/reactor`, `internal/test/peer`, one env description and one diagnostic code |
| No duplicated functionality (extends existing, does not recreate) | No | To prove: `tryWriteMessageWithin`'s poll loop and the hand-set deadlines are deleted, not kept beside the new acquisition |
| Zero-copy preserved where applicable (refs, not copies) | No | To prove: the allocation benchmark of the fast path shows no added allocation |
| Registration over hardcoding, outbound | No | N-A to prove: no command, family or handler is added |
| Registration over hardcoding, inbound | No | To prove: the test-peer option is one arm added to `peerOptionArms`, the registry `ze-peer` already derives its option set from |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Setting a write deadline in the past on a Go TCP connection from another goroutine fails a write already blocked in the kernel | Go `net.Conn` contract: deadlines apply to pending I/O; `requestWriteDeadline`'s cancel callback already relies on it | The interrupt step does nothing and the close has to close the socket to unblock the writer | `TestDeadlinePokeFailsBlockedTCPWrite`, on a real loopback TCP pair whose reader never reads | confirmed (2026-11-14: the test passes) |
| A-2 | `net.Pipe` blocks a writer whose other end never reads and honours write deadlines | Go `net.Pipe` documentation; existing tests use it (`pipeSession`) | The unit tests need a real TCP pair throughout | The first red unit test | confirmed (2026-11-14: `TestUnboundedWritersAreBounded` is green on `net.Pipe`, each writer blocked on a peer end that never reads and failed by its deadline) |
| A-3 | No writer holds `writeMu` across an unbounded wait other than the socket write | Read of the writers in the census; the export filter callback (`egressRouteFilter`) runs under `writeMu` and may reach a plugin | A close still waits, with the socket already closed, until that other wait ends | Read the filter call path and its timeout during Phase 3; a test holding the path by hand proves the socket closes regardless | broken (2026-11-14, read): a writer delivers the sent event to plugins under `writeMu` (`writeUpdateBody`, `writeMessageHeld` -> `onMessageReceived` -> `Reactor.notifyMessageReceiver` -> `Process.Deliver`, `internal/component/plugin/process/delivery.go`), a channel send with no timeout that a socket close does not wake. It also holds the adj-RIB-Out table mutex across that call, and for a dynamic peer whose settings AS is still 0 it takes `s.mu` under `writeMu` (`notifyOneMessage` -> `Session.Negotiated`), the reverse of the recorded order. The approved design holds: see "Consequences of A-3" below |
| A-4 | A 30 s budget per flush does not cut a healthy initial sync short | KEEPALIVE and forward writes already retire a session whose peer accepts nothing for 10 s to 30 s | Healthy slow peers are dropped during a large sync | `TestInitialSyncOutlastsBudget` and the interop scenario's route exchange | confirmed (2026-11-14: `TestInitialSyncOutlastsBudget` is green; it drives the session calls the held phases make, with the budget armed per message by `sendUpdateHeld`. The interop scenario `peer-stops-reading-bird` exchanges its routes and passes) |
| A-5 | The interop lab can pause and resume the peer container a scenario started | Docker supports it; the lab code was not read for an existing step | The scenario needs a new lab step, which Phase 5 then adds | Read `internal/le/interoplab` at the start of Phase 5 | confirmed (2026-11-14, read): `Lab.Pause` / `Lab.Unpause` (`internal/le/interoplab/lab.go`), already used by `checkHoldtimeDeadPeer` (`bgp/check_special.go`, scenario `bgp-holdtime-deadpeer-frr`). No lab step is owed |
| A-6 | Arming a deadline once per destination per batch adds no allocation to the fast path | `SetWriteDeadline` on a `net.TCPConn` is not expected to allocate | The fast path regresses and the arm point has to move | `BenchmarkDirectWriteBudget`, before and after, same toolchain | confirmed (2026-11-14: `TestFastPathBudgetAddsNoAllocation` is green, zero allocations for the arm and for a standing budget. No "before" benchmark figure exists, because the benchmark file is new) |
| A-7 | The test peer can keep a connection open, keep writing KEEPALIVEs and read nothing | `runMessageLoop` already separates its read deadline loop from its writes, and `silent` is the mirror option | The functional test needs a different stall mechanism | Phase 1 wiring test | confirmed (2026-11-14: `test/plugin/session-peer-stops-reading.ci` passes, and 80 stress runs of it showed no failure. The option lives in `internal/test/peer/stop_reading.go`: the KEEPALIVE loop runs on the connection's own goroutine) |
| A-8 | A session close raised as a transient event reaches `ze doctor` through the existing recent-events check | `CodeDoctorRecentEvents` exists in `codes.go`; its producer was not read | The code needs a doctor check of its own in the reactor package | Read `docs/architecture/doctor-and-health-checks.md` and the recent-events producer in Phase 2 | confirmed (2026-11-14, read): `recentEventsCheck` (`internal/component/doctor/checks_recent_events.go`) reports any bus event under its own code when the code is registered. Owed: the constant and `builtinCodes` entry, `diagnostic.RegisterEventCode` from the reactor's `init()`, the log line with the code and the fix, and `report.RaiseErrorFix` at the same site (template: `internal/plugins/fib/kernel`). The existing `session-dropped` raise is not raised for this cause, so one close gives one event |

### Consequences of A-3 (found 2026-11-14, before any feature code)

A writer can hold the write path while it waits on a plugin, not only on the
socket. The close routine as approved does not depend on that writer: it waits
a bounded time, closes the socket, and returns. What A-3 adds:

| Constraint | Phase | Why |
|------------|-------|-----|
| No step of the close routine, of `Run`'s exit or of the peer's session-end path takes `writeMu` or the adj-RIB-Out table mutex with an unbounded wait | 3 | The parked writer holds both; the session must end and the peer reconnect while it is parked. Phase 3 reads every close-path taker of the adj-RIB-Out mutex (`relay_recovery.go`, `recovery_owed.go`, `session_paths_limit_elect.go`, `beginAdjOut`) and proves it with `TestSessionEndsWhileWriterParkedOnPlugin` |
| The field clearing after the close takes `writeMu` without waiting on it while `s.mu` is held | 3 | A dynamic peer's writer takes `s.mu` under `writeMu`; waiting on `writeMu` under `s.mu` is the deadlock the recorded order exists to prevent |
| The reverse acquisition (`Session.Negotiated` under `writeMu` in `notifyOneMessage`) is removed: the negotiated peer AS is read without `s.mu` | 2 | It is a lock-order defect in the write path this spec is rewriting, and it deadlocks against any holder of `s.mu` that wants the write path |

The delivery wait itself (a plugin that does not drain its event queue blocks
the session's writer) is a different defect with a different owner: how plugin
backpressure reaches a BGP writer. It gets a journal row and is put to Thomas
as a decision at the end of this spec: fixed here, or in a spec of its own.

**The cost of A-3, read at the session-end path (2026-11-14).** The first row
of the table above is met for the close routine and for `Session.Run`'s exit,
and it is not met for the peer's session-end path. The adj-RIB-Out table is
the Peer's, not the session's (`peer_run.go` `runOnce` sets the session's
`adjOut` to the Peer's table), so every session of the peer shares it, and a
writer parked on a plugin holds its mutex together with `writeMu`. The
session-end defer of `runOnce` joins the recovery worker (`joinRecovery`) and
the initial-sync goroutine (`endInitialSync`); the initial-sync goroutine takes
`writeMu` on its way out, and the main thread read the recovery worker as
waiting on the same two locks. So with a writer parked on a plugin the socket
closes and `Run` returns, but the peer does not reconnect until the plugin
drains or stops. That was true before this spec: the spec neither worsens nor
fixes it. `TestSessionEndsWhileWriterParkedOnPlugin` proves that the socket
closes and that `Run` returns, and nothing more.

### Phase 3 decisions from the close-path worklist (2026-11-14)

Decided by the main thread from the worklist of every close site, and corrected
here to what Phase 3 built (`session_connection.go` `closeConnWithin`).

| Subject | Decision, as built | Why |
|---------|--------------------|-----|
| Idempotence of the close | Per connection, never per session. A close claims the connection before it flushes or drops (`Session.socketClosed` holds a `socketClaim`: the connection plus a completion signal, taken by `claimSocket`). The claim is taken under the write path when the close got it, and under `s.mu` when the path stayed held. One claim per connection, taken atomically, decides which close reports and which close drops. A close that meets another close's claim closes nothing and reports no held path. It waits for the claimant to finish on the claim's own signal, for at most `closeClaimWait` (the claimant's flush, drain and field clearing, plus slack), never by probing the write path, then clears the fields if they are still set (`awaitClaimedClose`). So a close that returned means the connection is no longer installed, except behind a writer parked on a plugin. A NOTIFICATION that found the write path busy is recorded once, by whichever close took its mark (`reportUnsentNotification`). The first build was different: it recorded the socket only after a drop behind a held path (`markSocketClosed`), and a second close made one attempt to clear the fields without waiting, then returned. The review rounds replaced it, because that second close could return while the first was still in its flush ("Review rounds") | An unsealed Session takes a second connection (a passive peer's `Run` and `Accept`), so a per-session latch would refuse to close the next connection. `TestSecondConnectionOnAReusedSessionCloses`, `TestSecondCloseWaitsForTheClaimant`, `TestTwoClosesBehindHeldPathReportOnce` |
| Sealing | The close routine does not seal the session and records no close reason. `beginTeardown` and `retireWrite` seal, as before | Same reason: the session can be used again. The spec's earlier wording "the session is sealed" was wrong |
| `openMu` across a close | About twenty close sites hold `openMu` across a NOTIFICATION and the close that follows. A holder waits for the write path once, never twice: a NOTIFICATION that found the path held leaves a mark (`notifyMetBusyPath`, set by `sendNotificationWithin` and `tryNotificationWithin`), and the close that follows takes the mark and tries the path without waiting. The bound is documented beside `openMu` in `session.go` | Two waits would double the time `openMu` is held behind one stuck writer. `TestCloseDoesNotWaitTwiceAfterBusyNotification` |
| Bound of the wait | `closeWait`: one second when the recorded close reason is a hold timer or a send hold timer expiry, the control budget for every other close. `sendHoldTimerExpired` and `ceaseAfterPanic` pass the one second themselves | The cancel goroutine's close and `Run`'s deferred close after a timer expiry get the one-second bound with no call-site edit |
| Field clearing | After the socket is closed, by the close that holds the claim (`clearClosedConnWithin`): `clearClosedConn` takes `s.mu`, then the write path by `TryLock` only, and clears the connection, the transport and the claim. When the path is held, the claimant waits for it up to one second (`closeClearWait`) with no session lock held, releases it, and tries once more. If the path is still held the fields stay set. A close that waited for the claimant makes one more attempt to clear them, without waiting for the path | A dynamic peer's writer may want `s.mu` while it holds the write path, so the path is never waited for under `s.mu`. Only a writer parked on a plugin outlasts the second (A-3) |
| What a healthy close keeps | A bare close still flushes buffered bytes when the path is free (`closeFlushed`: flush under a one-second budget, FIN, 100 ms drain, close). The ERROR line keeps the words "no NOTIFICATION was sent to the peer" when a NOTIFICATION was due | Pinned by `TestSessionCloseFlushesBufWriter` and `TestSessionReadPanicWriteLockHeld`, which were not edited |
| Callers that closed the socket themselves | `ceaseAfterPanic` now calls the routine (`closeConnWithin` with the one-second bound). `sendUpdateCounted` cannot: its two closes run under the write path the writer itself holds, so they take the routine's own close step, `dropConn` (expire the write deadline, close), and `Run`'s close clears the fields afterwards | One close step for a path that cannot be waited for, with the RFC quotes above it |
| What the operator sees | A close that met a held path logs one ERROR line and raises one event under `doctor-bgp-write-path-held` (`reportWritePathHeld`) | The existing `doctor-bgp-peer-not-reading` says a budget passed, which a close does not know, and a writer parked on a plugin is a second cause |

## What Phase 3 learned

- **A teardown records its stop event before anything can close the
  connection, and whoever reaches the FSM first delivers it.**
  `beginTeardown` stores the event (`Session.stopEvent`, first teardown wins);
  `teardownSession` then sends the NOTIFICATION, closes, and delivers the event
  (`endTeardown`). The read loop on a closed connection and the Peer's session
  end deliver `Session.endEvent`: the recorded stop when one exists, Event 18
  otherwise. A second delivery finds Idle, where the FSM ignores a stop. Two
  tests force this shape together. `TestCloseCauseReachesTheStateEvent` needs
  an operator's stop to leave Established as ManualStop, never as Event 18:
  the old close held `s.mu` until the fields were cleared, which kept `Run`
  from returning before the teardown fired its event, and the new close holds
  no session lock. `TestSessionDownFollowsBufferedDeliveries` needs the close
  not to sit behind the Peer's leave arm, which runs on the goroutine that
  fires the event and can be held for as long as the session's buffered
  UPDATEs take to publish. Event first fails the second test, close first
  with no record fails the first. The second test pins a Ze contract with
  plugins (the down follows the buffered deliveries, from
  `spec-bgp-rs-replacement-on-withdrawal`), not an RFC requirement.
- **One event per close.** A budget expiry reports under
  `doctor-bgp-peer-not-reading` (`reportWriteBudgetExpiry`), a close behind a
  held write path under `doctor-bgp-write-path-held` (`reportWritePathHeld`).
  `sessionDroppedOwed` answers false for both, so `session-dropped` is not
  raised beside either. The held-path case is carried to the session end by
  `leaveCause`, read after `Run` returned, because the hold timer fires its FSM
  event before the close reports.
- **`TestDelayOpenExpiryWriteFailureDeliversOneEvent18` changed, with the test
  judge's approval.** Its barrier arranged the interleaving by polling until a
  read lock on `s.mu` failed, that is until cancellation's close held `s.mu`
  in write mode while it waited for the write path. That hold is the defect
  this spec removes, and `TestCloseHoldsNoSessionLockWhileWaiting` pins its
  opposite, so the barrier could never become true. It now waits for the close
  to reach its bounded wait for the write path (the timer `LockWithin` creates
  on the session clock, for `closeWait`). Every assertion after the barrier is
  unchanged, and one is added: the DelayOpenTimer is still running at that
  point. Commit 1c6795e9e4 carries the `Weakening-approved:` line.
- **One claim per connection decides who closes and who reports.** Phase 3
  first recorded a closed socket only after a drop behind a held path, and a
  second close of that connection returned after one attempt to clear the
  fields. Two closes that both gave up on the path then each reported it, and
  a close that met another close in its final flush took that close for a
  stuck writer. As committed, a close claims the connection (`claimSocket`)
  before it flushes or drops, and the claim carries a completion signal. A
  close that finds the claim taken closes nothing, reports no held path, and
  waits on that signal for at most `closeClaimWait`, never on the write path:
  two closes that both probed the path could each find it taken by the
  other's probe and each leave the fields set. It then clears the fields if
  they are still set, so a close that returned means the connection is no
  longer installed, except behind a writer parked on a plugin.
- **A NOTIFICATION that found the write path busy is recorded once.** Its
  sender logs nothing and leaves a mark. The close that takes the mark writes
  the line, also when another close holds the claim or has finished with the
  connection (`reportUnsentNotification`), unless a close of the session has
  already reported a held path.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A deadline left armed by one writer fails the next writer's healthy write | Sessions retired with an i/o timeout while the peer is reading | The release clears the deadline; the fast path tracks which acquisition armed it. `TestWriteBudgetDoesNotLeak` |
| R-2 | The held initial-sync phases write many messages under one hold, so a budget armed once per hold cuts a long sync | A large config-route set drops the session at 30 s | The budget is re-armed at every flush inside a hold. AC-7 |
| R-3 | The bounded NOTIFICATION wait skips the NOTIFICATION on a healthy session whose write path is busy for a moment | Peers log a close with no NOTIFICATION during an operator teardown | Only the two timer expiries and the panic guard use the one-second bound; the other closes wait for the control budget. AC-5 |
| R-4 | Closing a socket outside `writeMu` races a writer that is about to write | Data race reports; a write on a closed connection | Closing a connection is safe against concurrent writes; the fields are still cleared only under `writeMu`. The race detector run of the reactor package |
| R-5 | The budget on the fast path costs time per batch on the hottest path | The fast-path benchmark slows | Armed once per destination per batch, never per message. Time figure owed from Thomas's Mac |
| R-6 | A change to an RFC-tagged test (`rfc9687_*_test.go`, `session_hold_expiry_test.go`) lowers what it proves | An edit to a tagged unit | New behavior gets new tests; a tagged test is edited only with the test judge's approval |
| R-7 | Another in-progress spec edits the same files (`spec-bgp-fsm-delayopen-and-automatic-start` on `session_connection.go`, the fast-path perf spec on `forward_rs_direct.go`) | Foreign hunks in those files at commit time | Commit this spec's hunks alone with the `hunks` form; correct the perf spec's "kept as is" line in the phase that changes the fast path |
| R-8 | The KEEPALIVE timer keeps firing while the write path is held, parking one goroutine per fire | Goroutine count grows on a stalled session | The KEEPALIVE write waits for the path no longer than its own budget and gives up |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live sessions dropped: a budget or a close that fires on a healthy peer resets the session and withdraws its routes |
| How is it reverted? | Revert of the implementation commits; no config or stored state changes |
| Who else touches this path? | `plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md` (`session_connection.go`), `plan/release/validation/spec-perf-rs-fast-path-regression-and-pass-labels.md` (`forward_rs_direct.go`), and the ready specs `spec-dropped-error-1-bgp-engine`, `spec-daemon-drain-before-stop`, `spec-bgp-peer-state-fsm-and-reason` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A configured peer that stops reading after Established while Ze announces routes | → | write budget, `retireWrite`, the run loop's reconnect | `test/plugin/session-peer-stops-reading.ci` |
| Hold timer expiry on a session whose write path is held | → | the close routine | `TestHoldExpiryClosesBehindStuckWriter` |
| Send hold timer expiry on a session whose write path is held | → | the close routine | `TestSendHoldExpiryClosesBehindStuckWriter` |
| Operator teardown of a peer whose writer is blocked | → | `teardownSession`, the close routine | `TestTeardownClosesBehindStuckWriter` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Established session, another writer holds the write path, the send hold timer expires | The connection is closed within the one-second NOTIFICATION bound, `Run` returns `ErrSendHoldTimerExpired`, the FSM is Idle | The NOTIFICATION may be absent: RFC 9687 Section 5 makes it MAY, the close MUST |
| AC-2 | Same, the receive hold timer expires | The connection is dropped within the one-second bound, `Run` returns `ErrHoldTimerExpired`, the FSM is Idle (RFC 4271 Section 8.2.2, Event 10) | The NOTIFICATION may be absent: it cannot be written to a peer that reads nothing, and FRR and BIRD close without it in the same state |
| AC-3 | Each writer the census marks unbounded writes to a peer end that never reads | The write returns an error once its budget has passed, the session is retired, and `Run` returns | None: an unbounded writer is the defect |
| AC-4 | Operator teardown, peer removal by reload and daemon stop, each while a writer is blocked on a peer that never reads | The call returns within the control budget and the socket is closed | The Cease NOTIFICATION may be absent, for the reason of AC-2 |
| AC-5 | Healthy session, write path free, each close path that owes a NOTIFICATION | The peer reads the NOTIFICATION, then the close | None: the bounded wait must not cost a healthy session its NOTIFICATION |
| AC-6 | Route server, one destination stops reading while a source keeps sending | The source's read loop keeps forwarding to the other destinations, and the stalled destination is retired within the write budget | The stalled destination's unflushed routes are lost with its session: it re-syncs on reconnect |
| AC-7 | Healthy initial sync whose held phase lasts longer than the write budget in total, the peer reading throughout | Every route and the End-of-RIB reach the peer and the session stays Established | None: the budget bounds one flush, not a hold |
| AC-8 | Functional: a test peer reaches Established, stops reading, keeps the connection open and keeps sending KEEPALIVEs, while Ze announces enough routes to fill the socket | Ze logs that the peer stopped accepting data, drops the session, reconnects, and the peer's second connection receives the routes | None |
| AC-9 | Interop: BIRD is paused mid-session, then resumed | Ze drops the session by its hold time plus the one-second bound, re-establishes after the resume, and BIRD holds Ze's routes again | The NOTIFICATION may be absent, for the reason of AC-2 |
| AC-10 | A session retired because a write passed its budget | One WARN line names the peer, says the peer accepted no data for the budget and that the session was closed and will reconnect, and carries its diagnostic code; the peer's last error reports the same cause | None |
| AC-11 | The route-server fast path, before and after the budget | The allocation count per forwarded UPDATE is unchanged | None: an added allocation on this path is a regression |
| AC-12 | A raw connection refused because the peer is administratively down | The refusal write carries the same 30 s bound the collision refusal has, and the connection is closed afterwards | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs Ze against a router that hangs while still holding the TCP connection | write budget or timer expiry -> close routine -> `Run` returns -> run loop backoff -> reconnect -> initial sync | `test/plugin/session-peer-stops-reading.ci`, interop `peer-stops-reading-bird` |
| 2 | Tears down or removes a peer that has stopped reading | `Peer.Teardown` -> `teardownSession` -> close routine | `TestTeardownClosesBehindStuckWriter` |
| 3 | Reads why a session went down | `retireWrite` close reason -> WARN line -> peer last error | `test/plugin/session-peer-stops-reading.ci` (log expectation), `TestWriteBudgetExpiryReason` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendHoldExpiryClosesBehindStuckWriter` | `internal/component/bgp/reactor/session_wedge_test.go` | AC-1. Tagged RFC9687-4.3-5; owes a discrimination record | |
| `TestHoldExpiryClosesBehindStuckWriter` | `internal/component/bgp/reactor/session_wedge_test.go` | AC-2. Tagged RFC4271-8.2.2-4; owes a discrimination record | |
| `TestUnboundedWritersAreBounded` | `internal/component/bgp/reactor/session_write_budget_test.go` | AC-3, one sub-test per writer of the census | |
| `TestTeardownClosesBehindStuckWriter` | `internal/component/bgp/reactor/session_wedge_test.go` | AC-4, operator teardown | |
| `TestPeerRemovalAndStopCloseBehindStuckWriter` | `internal/component/bgp/reactor/session_close_test.go` | AC-4, reload removal and daemon stop | |
| `TestCloseDeliversNotificationWhenPathFree` | `internal/component/bgp/reactor/session_close_test.go` | AC-5, eight sub-tests: operator teardown, automatic teardown, peer removal by reload, daemon stop, collision resolution, hold timer expiry, send hold timer expiry, read loop error | |
| `TestFastPathStalledDestinationDoesNotStallSource` | `internal/component/bgp/reactor/forward_rs_direct_wedge_test.go` | AC-6 | |
| `TestInitialSyncOutlastsBudget` | `internal/component/bgp/reactor/session_write_budget_test.go` | AC-7, through the session calls the held phases make | |
| `TestWriteBudgetDoesNotLeak` | `internal/component/bgp/reactor/session_write_budget_test.go` | R-1: a writer after a released acquisition sees no stale deadline | |
| `TestWriteBudgetExpiryReason` | `internal/component/bgp/reactor/session_write_budget_report_test.go` | AC-10, the close reason | |
| `TestDeadlinePokeFailsBlockedTCPWrite` | `internal/component/bgp/reactor/session_wedge_test.go` | A-1, on a real loopback TCP pair | |
| `TestCloseHoldsNoSessionLockWhileWaiting` | `internal/component/bgp/reactor/session_close_test.go` | A reader of the session state returns while a close waits | |
| `TestKeepaliveGivesUpOnBusyWritePath` | `internal/component/bgp/reactor/session_write_budget_test.go` | R-8 | |
| `TestAdminDownRefusalIsBounded` | `internal/component/bgp/reactor/peer_admin_wedge_test.go` | AC-12 | |
| `BenchmarkDirectWriteBudget` | `internal/component/bgp/reactor/forward_rs_direct_wedge_test.go` | AC-11, allocations per forwarded UPDATE, for a standing budget, an arm and no connection | |
| `TestPeerOptionStopReading` | `internal/test/peer/stop_reading_test.go` | The test-peer option parses, refuses unknown keys, and stops the read loop | |

Tests that exist and were not in the plan above, added by the phases that needed them:

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWriteBudgetExpiryLogsOneCodedWarning` | `internal/component/bgp/reactor/session_write_budget_report_test.go` | AC-10: one WARN line with the code `doctor-bgp-peer-not-reading` | |
| `TestWriteBudgetExpiryRaisesOneEvent` | same file | AC-10 and A-8: one event on the report bus | |
| `TestWriteBudgetExpiryRecordsPeerLastError` | same file | AC-10: the close cause reaches the peer's last-error record | |
| `TestWriteFailureOtherThanBudgetIsNotAnExpiry` | same file | A failed write that met no budget is not reported as an expiry | |
| `TestCanceledWriteIsNotABudgetExpiry` | same file | A canceled request's early deadline is not reported as an expiry | |
| `TestAttachStatsCallbacksWiresWriteBudgetClose` | same file | The Peer attaches the hook that records the close cause | |
| `TestSessionDroppedOwed` | same file | `session-dropped` is not raised beside a budget expiry | |
| `TestSessionDroppedNotOwedAfterHeldWritePath` | `internal/component/bgp/reactor/peer_close_write_path_held_test.go` | `session-dropped` is not raised beside `doctor-bgp-write-path-held` | |
| `TestSessionEndsWhileWriterParkedOnPlugin` | `internal/component/bgp/reactor/session_close_test.go` | A-3: the socket closes and `Run` returns while a writer holds the path and waits on something else than the socket | |
| `TestCloseDoesNotWaitTwiceAfterBusyNotification` | same file | A holder of `openMu` waits for the write path once | |
| `TestSecondConnectionOnAReusedSessionCloses` | same file | The close is idempotent per connection, not per session | |
| `TestFastPathArmsOncePerBatch` | `internal/component/bgp/reactor/forward_rs_direct_wedge_test.go` | R-5: one deadline call per destination per batch | |
| `TestFastPathRenewsAnAgedBudget` | same file | A standing budget is armed again after a sixteenth of it has passed, and not before | |
| `TestFastPathBudgetDoesNotLeak` | same file | R-1 for the fast path: four ways a batch ends, each followed by an ordinary writer and by a fast-path batch | |
| `TestFastPathBudgetAddsNoAllocation` | same file | AC-11 and A-6: zero allocations, on a recording socket and on a loopback TCP socket | |
| `TestLastErrorNamesCloseWithoutNotification` | `internal/component/bgp/plugins/cmd/peer/last_error_close_cause_test.go` | AC-10: the peer summary's last error names a close ze made with no NOTIFICATION | |
| `TestStopReadingRefusesIncompatiblePeer`, `TestStopReadingBlocksTheRemoteWriter`, `TestHoldUnreadReturnsWhenTheRemoteCloses`, `TestHoldUnreadFailsWhenHeldToTeardown`, `TestStopReadingRefusesExpectationsLeftBehind`, `TestStopReadingLeavesOtherConnectionsRead` | `internal/test/peer/stop_reading_test.go` | The option stalls a real TCP writer, ends on the remote's close, and refuses a peer block it cannot serve | |
| `TestPeerFileConfigMergeIsComplete` (existing, not edited) | `internal/test/cli/cmd_peer_test.go` | The option reaches the peer: red until `cmd_peer.go` carried the field | |
| `peerStopsReadingCheckerBranches`, run by `TestBespokeCheckerBranches` | `internal/le/interoplab/bgp/check_peer_stops_reading_test.go`, `bgp_test.go` | Both polarities of every predicate of the interop checker, with no container | |
| `TestFormatKeepsOneUndeclaredFieldInItsBlock` | `internal/component/config/serialize_unknown_field_test.go` | `ze config fmt` keeps a lone undeclared field inside its block, the formatter defect the scenario's config met | |
| `TestDelayOpenExpiryWriteFailureDeliversOneEvent18` (existing, barrier changed with the judge's approval) | `internal/component/bgp/reactor/delay_open_regression_test.go` | See "What Phase 3 learned" | |

Tests added by the review rounds ("Review rounds"):

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTwoClosesBehindHeldPathReportOnce` | `internal/component/bgp/reactor/session_close_test.go` | Two closes of one connection that both give up on a held write path give one ERROR line and one event under `doctor-bgp-write-path-held`, and both return | |
| `TestCloseBehindAGracefulCloseReportsNothing` | same file | A close that meets another close of the same connection in its final flush reports no held path, and the first close still closes the socket | |
| `TestSecondCloseWaitsForTheClaimant` | same file | A second close does not return while the first is in its flush; it returns with the socket closed and no connection installed, and logs and raises nothing | |
| `TestCloseFlushIsBounded` | same file | The final flush of a close has a budget of its own (`closeFlushBudget`): the close returns within it against a peer that reads nothing, with the socket closed and the fields cleared | |
| `TestTeardownDuringHeldRunDeliversCease` | same file | A teardown in the middle of a held run of UPDATEs ends the run within one message, returns far inside the control budget, and the peer reads the Cease last | |
| `TestBusyNotificationIsLoggedOnceByTheClose` | same file | A NOTIFICATION that found the write path held is logged once, by the close that follows; its sender logs nothing | |
| `TestBusyNotificationBehindAnotherCloseIsLoggedOnce` | same file | The same line and event are written once when the close that took the mark finds another close holds the socket, and both closes return with no connection installed | |
| `TestRequestDeadlineIsNotABudgetExpiry` | `internal/component/bgp/reactor/session_write_budget_report_test.go` | A write that met its request's own deadline answers `context.DeadlineExceeded` and closes the session, with no budget-expiry close reason, line or event | |
| `TestWriteBoundRecordsWhoseItIs` | same file | The acquisition records whether the armed bound is the session budget or the request's deadline, and the classification reads that record | |
| `TestWriteBudgetExpiryReachesPeerInfo` | same file | AC-10: the reactor's `PeerInfo` builder carries the close cause and its time, the fields the peer summary's last error reads | |
| `TestBudgetExpiryInForwardFlushLogsOnce` | same file | A budget expiry met by the fast path's deferred flush logs the coded line and no second line for the same flush | |
| `TestSessionBFDStrictDelayOpenBoundedWait` (existing, not edited) | `internal/component/bgp/reactor/bfd_delay_open_test.go` | Went red on round 1's first fix (a second close returning while the first was in its flush), before that fix was committed | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ze.fwd.write.deadline` | a positive duration | any positive duration | zero or negative: the default 30 s is used (existing behavior of `initFwdWriteDeadline`, kept) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `session-peer-stops-reading` | `test/plugin/session-peer-stops-reading.ci` | A peer stops reading with the connection up; Ze drops it, reconnects, and the second connection receives the routes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `peer-stops-reading-bird` | `test/interop/scenarios/` | BIRD 3 | Ze drops a session whose BIRD peer was paused, re-establishes after the resume, and BIRD holds Ze's routes again. Red when the close routine is reverted | |

## Files to Modify
- `internal/component/bgp/reactor/session_write_mutex.go` - the budget-arming acquisition and the duration-bounded wait
- `internal/component/bgp/reactor/session_write.go` - every writer goes through the acquisition; the control writes wait for the path with a bound; `sendHoldTimerExpired` calls the close routine; `flushFwdDirty` uses the acquisition; `tryWriteMessageWithin`'s poll loop goes
- `internal/component/bgp/reactor/session_connection.go` - `closeConn` becomes the bounded close routine; `flushForClose` goes; `ceaseAfterPanic` and `teardownSession` call it
- `internal/component/bgp/reactor/session.go` - the hold timer callback; the KEEPALIVE timer callback reads the connection without the write lock and gives up on a busy path
- `internal/component/bgp/reactor/forward_rs_direct.go` - the budget armed on a destination's first write in a batch
- `internal/component/bgp/reactor/forward_pool.go` - `fwdBatchHandler` uses the acquisition; the description of the budget
- `internal/component/bgp/reactor/peer_initial_sync.go` - the held phases re-arm the budget at each flush
- `internal/component/bgp/reactor/session_paths_limit.go`, `peer_static_wire.go`, `peer_send.go` - no caller change is expected once the acquisition bounds a background context; confirm and leave untouched if so
- `internal/component/bgp/reactor/peer_admin.go` - `rejectAdminDownConnection` gets the bound
- `internal/component/config/environment.go` - the description of `ze.fwd.write.deadline` says it bounds every session write
- `internal/test/peer/expect_keys.go`, `internal/test/peer/peer.go` - the `stop-reading` peer option
- `internal/core/diagnostic/codes.go` - the code for a session closed because the peer accepted no data
- `docs/architecture/core-design.md` - "Write Deadline", the close sequence, "Send Hold Timer (RFC 9687)"
- `docs/architecture/behavior/fsm-established.md`, `docs/architecture/behavior/peer-lifecycle.md` - how a session ends when the write path is held
- `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/forward-congestion-pool.md` - the fast path's budget
- `docs/architecture/config/environment.md` - the description of `ze.fwd.write.deadline`
- `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md` - the `stop-reading` option
- `docs/architecture/testing/interop.md` - the pause step, if Phase 5 adds one
- `ai/digests/bgp-reactor.md` - "Lock ordering" and the close bullets
- `rfc/short/rfc9687.md`, `rfc/short/rfc4271.md` - the new test links on RFC9687-4.3-5 and RFC4271-8.2.2-4
- `plan/release/validation/spec-perf-rs-fast-path-regression-and-pass-labels.md` - its "kept as is" statement about the fast path
- `plan/journal/lock-held-across-a-join.md` - the owned row is resolved at closure

Also modified, found by the work (each in the commit of its phase):
- `internal/component/bgp/reactor/session_read.go`, `peer_session_end.go` - the read loop and the Peer's session end deliver the recorded stop event (`Session.endEvent`); `peer_session_end.go` also asks `sessionDroppedOwed` with `leaveCause`
- `internal/component/bgp/reactor/peer_close.go` - `sessionDroppedOwed`, `leaveCause`, `ErrWritePathHeld`
- `internal/component/bgp/reactor/peer_stats.go` - the peer's last close cause and its time
- `internal/component/bgp/reactor/reactor_notify.go`, `session_negotiate.go` - the negotiated peer AS is read without `s.mu` under the write path (`negotiatedPeerASNoWait`)
- `internal/component/bgp/reactor/reactor_api.go` - copies the last close cause into `plugin.PeerInfo`. These two lines are in the working tree and in no commit as of this edit (2026-11-14): without them the cause does not reach the peer summary through the daemon
- `internal/component/plugin/types_bgp.go` - `CloseCause` and `PeerInfo.LastCloseCause`
- `internal/component/bgp/plugins/cmd/peer/summary.go` - the last error names the close cause when it is the newer record
- `internal/component/bgp/reactor/delay_open_regression_test.go` - the barrier of `TestDelayOpenExpiryWriteFailureDeliversOneEvent18`, with the judge's approval
- `internal/test/cli/cmd_peer.go` - the peer block's `stop-reading` option is carried into the peer's config
- `internal/component/config/serialize.go` - `ze config fmt` keeps a lone undeclared field inside its block (commit dad63913ba)
- `internal/le/interoplab/bgp/bgp_test.go` - one line, the new checker's branch test
- `docs/architecture/behavior/fsm.md`, `docs/architecture/api/commands.md`, `docs/features/configuration.md`, `docs/guide/command-reference.md`, `docs/guide/debugging-tools.md`, `docs/guide/operational-reports.md` - the two event codes, the WARN and ERROR lines, the down order
- `docs/architecture/testing/interop.md` - the scenario's inventory row; no pause step was added (A-5)
- Not modified, as expected: `session_paths_limit.go`, `peer_static_wire.go`, `peer_send.go` (the acquisition bounds a background context). `peer_initial_sync.go` changed in comments only: `sendUpdateHeld` arms the budget for each message

## Files to Create
- `internal/component/bgp/reactor/session_wedge_test.go`, `forward_rs_direct_wedge_test.go`, `peer_admin_wedge_test.go` - the unit tests above
- `internal/test/peer/stop_reading_test.go` - the option's unit test
- `test/plugin/session-peer-stops-reading.ci` - the functional test
- `test/interop/scenarios/peer-stops-reading-bird/` - the interop scenario and its checker
- `rfc/discrimination/` records for the two tagged tests, written by `./le rfc record`

Created, as built:
- `internal/component/bgp/reactor/session_write_budget.go` - the one place that decides a failed write was a budget expiry, and its report
- `internal/component/bgp/reactor/register_diagnostic.go` - registers the two event codes from `init()`
- `internal/component/bgp/reactor/session_write_budget_test.go`, `session_write_budget_report_test.go`, `session_close_test.go`, `peer_close_write_path_held_test.go` - the Phase 2 and Phase 3 unit tests, beside the three test files named above
- `internal/test/peer/stop_reading.go` - the whole `stop-reading` option
- `internal/component/bgp/plugins/cmd/peer/last_error_close_cause_test.go`, `internal/component/config/serialize_unknown_field_test.go`
- `internal/le/interoplab/bgp/check_peer_stops_reading.go`, `register_peer_stops_reading.go`, `check_peer_stops_reading_test.go` - the scenario's checker, which lives with the lab's other checkers and not in the scenario directory
- `test/interop/scenarios/peer-stops-reading-bird/` holds `ze.conf`, `ze-reload.conf` and `bird.conf`
- `rfc/discrimination/rfc4271/RFC4271-8.2.2-4.json` and `rfc/discrimination/rfc9687/RFC9687-4.3-5.json` hold the two records

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config is added. Whether `ze.fwd.write.deadline` gets a leaf belongs to `spec-reactor-tuning-leaves-reach-no-reader` |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC; the functional test of the behavior is `test/plugin/session-peer-stops-reading.ci` |
| Pipe completeness | N-A | No command output |
| Env var registration | Yes | `internal/component/config/environment.go`: the existing `ze.fwd.write.deadline` registration, description updated; no second registration |
| Doctor check for runtime dependencies | N-A | No new file, socket, module or port |
| Doctor code per problem class | Yes | `internal/core/diagnostic/codes.go`: one code for "session closed: the peer accepted no data for the write budget", named by the WARN line of AC-10, reported at the runtime phase. A-8: Phase 2 reads `docs/architecture/doctor-and-health-checks.md` and either confirms the recent-events check reports it or registers a check in the reactor package. As built, two codes: `doctor-bgp-peer-not-reading` (a write passed its budget, WARN, `reportWriteBudgetExpiry`) and `doctor-bgp-write-path-held` (a close met a write path another writer held, ERROR, `reportWritePathHeld`). Both are constants in `codes.go`, registered as event codes from `internal/component/bgp/reactor/register_diagnostic.go`, and reported by the existing recent-events check |
| Prometheus counters/metrics | N-A | The existing `wireWriteErrors` counter already counts a failed write and its label set is unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | A defect fix inside an existing feature; `./le feature report` is read at closure to confirm no level moves |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/debugging-tools.md`: the new WARN line and what it means, in Phase 2 |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9687.md`, `rfc/short/rfc4271.md`: test links for the two rows, with their discrimination records |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md`: the `stop-reading` option; `docs/architecture/testing/interop.md` if a pause step is added |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about write bounds (to confirm by grep in Phase 6) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `docs/architecture/behavior/fsm-established.md`, `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/forward-congestion-pool.md`, `docs/architecture/config/environment.md`, `ai/digests/bgp-reactor.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by `// Design:` headers and named here: `docs/architecture/core-design.md` (edited), `docs/architecture/bgp/structural-forwarding.md` (edited), `docs/architecture/testing/ci-format.md` (edited), `docs/architecture/buffer-architecture.md` (unaffected: no buffer ownership or encoding changes), `docs/features/ai-first.md` (unaffected: a code is added through the existing mechanism it describes), `docs/architecture/config/syntax.md` (unaffected: no config grammar changes, only an env description), `docs/guide/add-path.md` (unaffected: no ADD-PATH encoding or negotiation changes). Declared by the files the work added to the list: `docs/architecture/behavior/fsm.md`, `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/api/commands.md`, `docs/architecture/testing/interop.md` (all edited), `docs/architecture/doctor-and-health-checks.md` (unaffected: the two event codes are registered through the `RegisterEventCode` mechanism the page describes, and it lists no code by name), `docs/architecture/api/architecture.md` (declared by `peer_stats.go`; unaffected: the page describes no peer statistics field and no last-error record, so the added close cause changes nothing it says), `docs/architecture/api/process-protocol.md` (declared by `internal/component/plugin/types_bgp.go`; unaffected: the page names `PeerInfo` only for its message context and its policy fields, and the added `LastCloseCause` is an operational field the peer summary reads, with no event or RPC payload changed). The other pages `./le spec citation anchors` lists only mention these files and are re-checked at closure |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example shows a write deadline or a close sequence command |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the test means and the red tests
   - Tests: `TestPeerOptionStopReading`, `test/plugin/session-peer-stops-reading.ci`, `TestHoldExpiryClosesBehindStuckWriter`, `TestSendHoldExpiryClosesBehindStuckWriter`, `TestTeardownClosesBehindStuckWriter`, `TestDeadlinePokeFailsBlockedTCPWrite`
   - Files: `internal/test/peer/expect_keys.go`, `internal/test/peer/peer.go`, the test files, `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md`
   - The `stop-reading` option is a peer-block, check-mode option, the mirror of `silent`: once the daemon's first UPDATE has arrived the peer reads nothing more, keeps the connection open and writes a KEEPALIVE on a timer so Ze's receive hold timer never fires. The functional test sets a short `ze.fwd.write.deadline`, gives Ze enough routes to fill the socket buffers, and allows two connections so the reconnect is observed. The fill must not depend on the host's default socket buffer sizes: if a plain stop is not deterministic, the option also shrinks the peer's receive buffer. Write the `.ci` through the `ze-test` skill
   - Verify: the option works, and the unit and functional tests fail because the session never ends. Paste the red output
2. **Phase: one acquisition of the write path** -- every writer is bounded
   - Tests: `TestUnboundedWritersAreBounded`, `TestWriteBudgetDoesNotLeak`, `TestInitialSyncOutlastsBudget`, `TestWriteBudgetExpiryReason`, `TestKeepaliveGivesUpOnBusyWritePath`, `TestAdminDownRefusalIsBounded`
   - Files: `session_write_mutex.go`, `session_write.go`, `forward_pool.go`, `peer_initial_sync.go`, `session.go` (KEEPALIVE callback), `peer_admin.go`, `environment.go`, `codes.go`, `docs/architecture/core-design.md` ("Write Deadline"), `docs/architecture/config/environment.md`, `docs/guide/debugging-tools.md`
   - The acquisition takes the write path, either blocking, with a context, or for no longer than a duration, and arms the socket's write deadline to now plus the caller's budget. The release clears the deadline unless the session was retired. The budget is `fwdWriteDeadline` for UPDATE writers, `controlWriteDeadline` for control messages, the earlier of the request deadline and the session budget for a request-scoped write. Inside a hold that spans many messages the budget is re-armed at each flush. Every writer of the census is converted; the hand-set deadlines in `fwdBatchHandler`, `flushFwdDirty` and `writeMessageHeld` are deleted, not kept beside it
   - Corrected after the build: the release clears whatever budget is armed, retired session or not (Data Flow, step 3). The forms are `acquireWrite`, `acquireWriteContext`, `acquireWriteWithin` and `tryAcquireWrite` in `session_write_mutex.go`, with `holdWrites` as the held form, which arms nothing itself: `sendUpdateHeld` arms the budget for each message, and `fwdBatchHandler` arms it after its policy-replay requeue. A socket that refuses its deadline is a closed one: an UPDATE writer's form retires the session for it, and a control message's form only returns the error. The classification of a failed write as a budget expiry is in one place, `recordWriteFailure` in the new `session_write_budget.go`: a deadline error, with a budget armed, at or after the armed end, so a canceled request's early deadline and the close routine's expired deadline do not read as an expiry
   - Tests as built: the Phase 2 tests live in `session_write_budget_test.go` and `session_write_budget_report_test.go`
   - Verify: tests fail, implement, tests pass; the wiring `.ci` now passes through the budget
3. **Phase: one close routine** -- no close waits on a writer it would unblock
   - Tests: `TestCloseDeliversNotificationWhenPathFree`, `TestPeerRemovalAndStopCloseBehindStuckWriter`, `TestCloseHoldsNoSessionLockWhileWaiting`, and the three Phase 1 unit tests turn green
   - Files: `session_connection.go`, `session_write.go` (`sendHoldTimerExpired`, the NOTIFICATION senders), `session.go` (hold timer callback), `docs/architecture/core-design.md` (close sequence, send hold timer), `docs/architecture/behavior/fsm-established.md`, `docs/architecture/behavior/peer-lifecycle.md`, `ai/digests/bgp-reactor.md`
   - The routine, in order: record the close reason; wait for the write path no longer than the caller's bound (one second for the two timer expiries and the panic guard, the control budget for every other close); if the path came free, send the owed NOTIFICATION, flush with a short budget, send FIN, drain, close; if it stayed held, expire the socket's write deadline, close the socket, and log at ERROR that no NOTIFICATION was sent and why. `s.mu` is taken only to read the connection, and neither `s.mu` nor `openMu` is held across the wait for the write path or across the NOTIFICATION. Once the socket is closed the routine records that fact on the session, so a second call (the cancel goroutine, `Run`'s deferred close) returns at once. It then clears the connection fields in the recorded order, `s.mu` then `writeMu`, waiting for `writeMu` no longer than one second: a writer blocked on the socket has failed by then. If the path is still held, the fields are left set and the routine returns: the session is sealed, its socket is closed, every later write fails, and a sealed session is never reused, so a stale field is read by nobody who could act on it. A NOTIFICATION sent from a read-loop error path waits for the path with the control budget, so the `logNotifyErr` call sites are bounded with no call-site edit
   - Corrected after the build (the table "Phase 3 decisions from the close-path worklist" holds the detail): the routine records the closed socket per connection, not on the session, and it never seals; "the session is sealed ... a sealed session is never reused" above is wrong, because an unsealed Session takes a second connection. A stale field is harmless for another reason: its socket is closed, so every later read and write on it fails. The clearing takes `s.mu`, then the write path without waiting; the one-second wait for the path runs with no session lock held. The routine waits for the path not at all when the NOTIFICATION sent just before already found it held. The teardown records its stop event first ("What Phase 3 learned")
   - Corrected again after the review rounds: "a second call ... returns at once" above, and the per-connection record of the first correction, describe the first build. As committed, a close claims the connection before it flushes or drops, and a second close of that connection waits for the claimant to finish, on the claim's own signal and for at most `closeClaimWait`, then clears the fields if they are still set. It reports no held path. A NOTIFICATION that found the write path busy is recorded once, by the close that took its mark. The row "Idempotence of the close" holds the detail
   - Tests as built: the Phase 3 tests live in `session_close_test.go`, and `TestSessionDroppedNotOwedAfterHeldWritePath` in `peer_close_write_path_held_test.go`
   - Verify A-3 here: read the export filter path that runs under `writeMu` and record its bound
   - Verify: tests fail, implement, tests pass. The two RFC-tagged tests get their discrimination records
4. **Phase: the route-server fast path** -- the last unbounded writer
   - Tests: `TestFastPathStalledDestinationDoesNotStallSource`, `BenchmarkDirectWriteBudget`
   - Files: `forward_rs_direct.go`, `session_write.go` (`appendFwdDirty`, `flushFwdDirty`), `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/forward-congestion-pool.md`, the perf spec's "kept as is" line
   - Tests as built: `TestFastPathStalledDestinationDoesNotStallSource`, `TestFastPathArmsOncePerBatch`, `TestFastPathRenewsAnAgedBudget`, `TestFastPathBudgetDoesNotLeak`, `TestFastPathBudgetAddsNoAllocation`, `BenchmarkDirectWriteBudget`
   - Files as built: `forward_rs_direct.go`, `session_write_mutex.go` (`armFastPathBudget`, `releaseFastPathWrites`), comments only in `session_write.go` and `session.go`, the three pages
   - As built (shape chosen by Thomas on 2026-11-14, Key Design Decisions): the fast path arms the budget after it took the destination's write path and before the first body, and gives the path back with a release of its own that leaves the deadline standing. A later fast-path write to the same destination arms nothing while the standing budget has at least fifteen sixteenths of its length left; once a sixteenth has passed it is armed again, because a batch has no bound in time. No counter exists. The record that another writer held the path in between is the armed mark itself (`Session.writeBudgetArmed`, owned by `writeMu`): every release but the fast path's clears the deadline and the mark together, so a mark still set at a fast-path take says nobody released in between. A socket that refuses the deadline is closed, and the fast path retires the session for it
   - What clears a standing budget: the batch's flush (`flushFwdDirty`), the forward worker that took the flush over (`fwdBatchHandler`), or the next writer of any kind, each through `releaseWrites`. A batch that ends with no flush of the destination leaves the deadline on a socket nobody writes to, where it fails nothing: every other writer arms its own, and the fast path reads a standing budget's age before it trusts it
   - Verify: tests pass. No "before" benchmark exists, because the benchmark file is new: the AC-11 evidence is `TestFastPathBudgetAddsNoAllocation`, which asserts zero allocations for the arm and for the standing case, on a recording socket and on a real loopback TCP socket
5. **Phase: interop** -- a real peer that stops reading
   - Tests: `peer-stops-reading-bird`
   - Files: `test/interop/scenarios/peer-stops-reading-bird/`, the lab's pause step if A-5 shows none exists, `docs/architecture/testing/interop.md`
   - The scenario establishes with BIRD, exchanges routes, pauses the BIRD container this run started, expects Ze to report the session down by the hold time plus the bound, resumes it, and expects the session and the routes back. Read `bgp_hold_timeout` and the reconnect path in BIRD 3 before writing what the scenario expects of BIRD after the resume
   - Verify: red with the close routine reverted and the image rebuilt, green with it restored
6. **Phase: closing reads** -- pages and records the earlier phases did not own
   - Files: `docs/comparison.md` (grep only), `rfc/short/rfc9687.md`, `rfc/short/rfc4271.md`, `plan/journal/lock-held-across-a-join.md`
   - Verify: `./le rfc check` is listed as an owed command

### Owed commands (run by the main thread, never by an agent)
| Command | Proves |
|---------|--------|
| `./le test unit pkg path ./internal/component/bgp/reactor/ run 'Wedge\|StuckWriter\|WriteBudget\|Unbounded\|InitialSyncOutlasts\|CloseDelivers\|CloseHolds\|KeepaliveGivesUp\|AdminDownRefusal\|DeadlinePoke\|FastPathStalled'` | The unit ACs, with the race detector |
| `./le test unit pkg path ./internal/component/bgp/reactor/` | No regression in the reactor package |
| `./le test unit pkg path ./internal/test/peer/` | The test-peer option |
| `./le --name session test bgp plugin --pattern session-peer-stops-reading` | AC-8 through the real daemon |
| `./le --name session test bgp plugin --pattern deadpeer-holddown` and `./le --name session test bgp plugin --pattern open-hold-time-peer-lower-wins` | The existing hold-expiry functional tests still pass |
| `./le test unit pkg path ./internal/component/bgp/reactor/ run 'FastPath'` | AC-6 and AC-11: `TestFastPathBudgetAddsNoAllocation` asserts zero allocations. No "before" benchmark exists, because the benchmark file is new |
| `INTEROP_SCENARIO=peer-stops-reading-bird ./le --name session test integration interop` | AC-9 |
| `./le test unit pkg path ./internal/component/bgp/plugins/cmd/peer/`, `./internal/le/interoplab/bgp/`, `./internal/test/cli/ run 'TestPeerFileConfigMergeIsComplete'`, `./internal/component/config/ run 'TestFormatKeepsOneUndeclaredFieldInItsBlock\|FuzzFormatRoundTrip'` (each as `./le test unit pkg path <dir>/ run '<regex>'`) | The last-error record, the interop checker's predicates, the option reaching the peer, the formatter fix |
| `./le rfc record` for the two tagged tests, then `./le rfc check` | The discrimination records |

The forms above are the ones that work, corrected on 2026-11-14 from the first draft of this table. A functional test is selected with `--pattern <name>`, not with a bare name, and `--name session` makes the launcher build from the working tree, which a test of uncommitted code needs. An interop scenario is selected with the `INTEROP_SCENARIO` environment variable, not with an argument. A unit run of one package is `./le test unit pkg path <dir>/ run '<regex>'`. The main thread runs each through `./le job run`.

Owed by Thomas, on the Mac: the time figure of the fast path before and after Phase 4 (`./le perf`). The spec's closure does not wait on it; the figure is reported to him as owed.

### Evidence obtained (2026-11-14)

Run by the main thread in the main checkout. Each row is a result already seen, not a command still owed.

| What ran | Result | What it shows |
|----------|--------|---------------|
| The reactor package, `./le test unit pkg path ./internal/component/bgp/reactor/` | Green except `TestOrdinaryRefreshDoesNotDoublePrepend` | The unit ACs hold. The one red was red before this work and is owned by `spec-bgp-refresh-reapplies-export-policy` |
| The three close-path tests of Phase 1 (`StuckWriter`) | Red before Phase 3 ("the connection was still open"), green after | The wedge, and its repair |
| `TestDeadlinePokeFailsBlockedTCPWrite` | Green | A-1 |
| Functional `session-peer-stops-reading` | Red before Phase 2 (60 s timeout, the peer stalled and ze never closed), green after; 80 stress runs with no failure before it was promoted to `test/plugin/` | AC-8, and that the test can go red |
| Functional `deadpeer-holddown` and `open-hold-time-peer-lower-wins` | Green | The existing hold-expiry tests still pass |
| Interop `peer-stops-reading-bird`, against BIRD 3.4.0 | Green twice. Red with the close step (`dropConn`) disabled in `closeConnWithin`: the session closed 31.1 s after the freeze, against a 20.5 s bound | AC-9, G-5, and that the scenario discriminates |
| `TestDelayOpenExpiryWriteFailureDeliversOneEvent18` with a close that stops the DelayOpenTimer early | Red; green with the change removed | The changed barrier still catches the defect the test exists for |
| `./le rfc record` for the two tagged tests | Two records written: `rfc/discrimination/rfc4271/RFC4271-8.2.2-4.json` (unit `TestHoldExpiryClosesBehindStuckWriter`) and `rfc/discrimination/rfc9687/RFC9687-4.3-5.json` (unit `TestSendHoldExpiryClosesBehindStuckWriter`), both by the revert route with `dropConn` as the producer | Each tagged test goes red when the close step is disabled |
| Reruns on the final close routine: functional `session-peer-stops-reading`, `deadpeer-holddown` and `open-hold-time-peer-lower-wins` | Green, all three | The claim-based close keeps AC-8 and the existing hold-expiry tests |
| Rerun on the final close routine: interop `peer-stops-reading-bird` | Green, 58 s | AC-9 and G-5 hold on the committed close |
| Rerun on the final close routine: the reactor package | Green except `TestOrdinaryRefreshDoesNotDoublePrepend`, the red named in the first row | No regression from the review fixes |
| `TestSessionBFDStrictDelayOpenBoundedWait` and `TestSecondCloseWaitsForTheClaimant`, 100 repetitions under the race detector | No failure | The second close waits for the claimant on every schedule the run met |
| `TestPolicyFinalReplayPreservesPreAdmittedBatchOrder` | Failed once in one full run of the package under load, then passed 40 repetitions | A red on a path this change does not reach, named here and not investigated |
| RFC records and verdicts refreshed | Six discrimination records re-recorded because their producer changed: RFC2918-4-3, RFC4271-4.4-2, RFC9494-4.3-1 (positive and negative), RFC9687-4.3-3 (two units). Audit verdicts re-judged or resealed: RFC4271-8.2.2-4, RFC4271-8.2.2-13, RFC7911-2-1, RFC7911-2-2, RFC8654-5-1, RFC9687-4.3-5 | Bookkeeping this change owed and paid. It is not a defect |
| Judge-approved test changes | `TestDelayOpenExpiryWriteFailureDeliversOneEvent18`: `Weakening-approved:` trailer in 1c6795e9e4. `TestSendHoldExpiryClosesBehindStuckWriter`: `RFC-approved:` trailer in 5d862bbab9 | Each change was judged by an independent agent, and its commit carries the verdict |

### Review rounds (2026-11-14)

These are the implementation's own review rounds, run on its commits as they landed. The Review Gate of this spec is `/ze-close`'s, and none of these rounds stands for it.

| Round | Scope | Found | Fixed in |
|-------|-------|-------|----------|
| 1 | The whole change set, commits 85c1a3c4cb to 73fac8af9c | 1 BLOCKER, 9 ISSUE, 8 NOTE | 43cfa9af6e |
| 2 | 43cfa9af6e | 0 BLOCKER, 3 ISSUE, 7 NOTE | 5d862bbab9 |
| 3 | 5d862bbab9 | 0 BLOCKER, 1 ISSUE (a comment that stated the `openMu` bound too low), 6 NOTE | The issue and notes 2 and 3 are corrected in the commit that follows 5d862bbab9 |

- The blocker of round 1: the cause of a write budget expiry never reached the peer summary, because two lines were left out of 178cffd97d.
- The regression round 1's own fix introduced, caught by round 1 before that fix was committed: a second close returned while the first was still in its flush. `TestSessionBFDStrictDelayOpenBoundedWait` went red on it, and `TestSecondCloseWaitsForTheClaimant` pins the repair.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every writer row of the census is converted, and every close-path row calls the close routine |
| Feature completeness | The three user stories each pass their test |
| Correctness | No path holds `s.mu` or `openMu` across a wait for the write path; the lock order is unchanged; `bufWriter` is touched only under `writeMu` |
| No layering | `tryWriteMessageWithin`'s poll loop, `flushForClose` and the three hand-set deadlines are deleted |
| Budget hygiene | No deadline survives a release; the fast path's deadline survives only to its batch's flush |
| Healthy sessions | The NOTIFICATION reaches the peer on every close path when the write path is free |
| Operator errors and doctor codes | The WARN and ERROR lines say what happened, why, and what Ze does next, and name their code |
| Rule: `ai/rules/rfc-compliance.md` | The close routine quotes RFC 9687 Section 5 and RFC 4271 Section 8.2.2 above the statements that perform them; the two tagged tests claim only what they assert |
| Rule: `ai/rules/goroutine-lifecycle.md` | No goroutine is started per write or per close |
| Rule: `ai/rules/performance.md` | No allocation is added to a write path |
| Rule: `ai/rules/stale-comments.md` | The comments that describe the old close order and "Does NOT ... set write deadlines" are rewritten |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No writer bypasses the acquisition | A grep for `writeMu.Lock`, `writeMu.TryLock`, `writeMu.LockContext` and `writeMu.LockWithin` in `internal/component/bgp/reactor/`, tests excluded, shows only: the four forms of the acquisition in `session_write_mutex.go`; `holdWrites` in `session_write.go`; the close routine's own waits in `session_connection.go` (`closeConnWithin` and `clearClosedConnWithin` by `LockWithin`, `clearClosedConn` by `TryLock`); the fast path's `writeMu.TryLock` in `forward_rs_direct.go`, which arms with `armFastPathBudget` and releases with `releaseFastPathWrites`; and the non-writing takers of the census (`connectionEstablished`, `negotiateWith`, `setSendCtxID`, the policy reload files, `peer_settings_apply.go`, `session_paths_limit_elect.go`). Checked on 2026-11-14: the grep shows exactly these |
| No hand-set write deadline remains | A grep for `SetWriteDeadline` in `internal/component/bgp/reactor/`, tests excluded, shows only: `armWriteDeadline` in the acquisition, which `closeFlushed` also reaches through `armWriteBudget` for the close's final flush; the clearing in `releaseWrites`; `dropConn` in the close routine; the request cancel callback in `requestWriteDeadline`; and the two raw-connection refusals (`rejectAdminDownConnection`, `rejectConnectionCollision`). Checked on 2026-11-14: six sites, exactly these |
| The poll loop and `flushForClose` are gone | A grep for `writePathPoll` and `flushForClose` in `internal/component/bgp/reactor/`, tests excluded, returns nothing. Checked on 2026-11-14 |
| The functional test exists and passes | `./le --name session test bgp plugin --pattern session-peer-stops-reading` |
| The interop scenario exists and discriminates | The recorded red run with the close routine reverted |
| The pages are corrected | `./le doc check all`, and a read of "Write Deadline" in `docs/architecture/core-design.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A peer that stops reading can no longer pin a session, its goroutines and the locks other sessions wait on; the KEEPALIVE timer parks no goroutine per fire |
| Denial of service by a slow peer | The budget retires only a peer that accepts nothing for the whole budget; a slow but reading peer keeps its session (AC-7) |
| Fail-open | A close that cannot send its NOTIFICATION still closes the socket; no path returns with the socket open |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The wedge is a property of the lock, not of any one writer: a bound placed on each caller is a list that the next writer forgets. The bound belongs to the acquisition of the write path
- FRR and BIRD cannot have this defect because their writes never block. Ze keeps a blocking write under a lock, so it owes the same property by construction: whatever closes the connection can always interrupt the writer

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `lock-held-across-a-join` | 2026-11-14 | a write with no deadline holds `writeMu`; `closeConn` and the hold and send-hold expiry wait on it before closing |

## Defects met in passing, not yet journaled

None of these has a journal row yet. The main thread decides the owning spec of each with Thomas, and the rows are written then.

| Defect | Where | What it costs |
|--------|-------|---------------|
| The functional runner accepts `expect=stdout:` under a test-peer command, and the expectation can never pass | The runner captures a peer's output in buffers of its own, so that output is in no command span; `docs/architecture/testing/ci-format.md` says so now, and nothing refuses the line at parse time | A test author writes an assertion that is red for a reason unrelated to the product. The repair is a refusal at parse time |
| The reactor's NOTIFICATION log lines use the `code` log key for the BGP error code | `session_write.go` `notificationWritten` ("notification sent"), `session.go` `logNotifyOutcome` ("notification send failed", which `logNotifyErr` reaches), and the same line in `ceaseAfterPanic` | `ai/rules/user-facing-errors.md` reserves `code` for the doctor code, which the two lines this spec added use it for. One key then carries two meanings in the same log |
| A writer parked on a plugin (A-3) | A writer delivers the sent event to plugins under `writeMu`, a channel send with no timeout that a socket close does not wake ("Consequences of A-3") | The parked goroutine and the locks it holds stay until the plugin drains or stops, and the peer does not reconnect until then. Thomas decides: fixed here, or in a spec of its own |
| A stalled fast-path destination holds the source's read loop for up to the write budget | `forward_rs_direct.go` `tryDirectWriteNoFlush` writes synchronously on the source peer's read goroutine | A source whose hold time is under the budget can lose its own session meanwhile (Known Limitations). The territory of the fast-path performance spec |
| The record that a held write path was reported is kept on the Session, not on the connection's claim | `session.go` `Session.writePathHeldReported`, read by `reportUnsentNotification` and `leaveCause`, never cleared | On a Session used for a second connection, an earlier held-path report would silence a later missed-NOTIFICATION line and one `session-dropped`. Reviewed as unreachable past Established: a Peer runs one Session per run cycle |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Interrupt, never wait: one budget-arming acquisition and one bounded close routine (approved by Thomas, 2026-11-14) | Budget only: give every write a deadline and leave the close paths | Budget only still queues a close for up to 30 s behind a write and still wedges behind a writer blocked on anything but the socket; it also leaves the bound as a per-caller duty |
| Same | A dedicated non-blocking writer goroutine per session with an output queue, as FRR and BIRD have | It removes the lock, but it rewrites the egress path (the fast path, the initial-sync ordering holds, zero-copy forwarding). Too large for a release fix, and the release fix would still be needed first |
| The session budget is the existing forward budget (`ze.fwd.write.deadline`, 30 s) | The send hold timer as the only bound, as in FRR and BIRD | Ze already retires a peer that accepts nothing for 10 s to 30 s through its KEEPALIVE and forward writes, so one budget states today's behavior as one rule; the send hold timer stays the RFC bound for a session that writes nothing |
| The env name `ze.fwd.write.deadline` is kept and its description widened | Rename it to a session-wide name | Whether it becomes a config leaf, and under what name, is owned by `spec-reactor-tuning-leaves-reach-no-reader`; renaming it here would be done twice |
| The fast path's budget is armed once per destination per batch (approved by Thomas, 2026-11-14) | Arm it on every fast-path write; leave the fast path unbounded | Per write costs a deadline call per forwarded UPDATE on the hottest path; unbounded leaves a writer that can stall a source peer's read loop |
| Amended by Thomas on 2026-11-14, after Phase 4 showed the row above unsafe as written: the budget is armed on a destination's first write of a batch and left standing for the flush, and a standing budget is trusted for one sixteenth of its length, then armed again (`Session.armFastPathBudget`, `fastPathBudgetShare`) | Arm at the socket boundary, once per 16 KiB socket write, only when the holder is the fast path: one clock read and one deadline call per socket write, nothing per buffered UPDATE and no standing state | A batch has no bound in time: `Session.Run` ends one only when the source's read buffer is empty, so a source that stops in the middle of a message or sends for longer than the budget keeps the batch open. A deadline armed once for all of it would already have passed, and it would fail the next overflow write to a destination that reads every octet, so a slow or hostile source could reset other peers' sessions. The renewal costs one clock read per fast-path take. The socket-boundary shape was set aside because it departs further from the approved shape |
| One-second bound for the timer expiries and the panic guard, the control budget for the other closes | One second everywhere | A teardown or a read-error close on a healthy session can meet a write path that is busy for more than a second during a large sync, and would lose its NOTIFICATION for no reason |
| No socket option (`TCP_USER_TIMEOUT`) | Add it as the bound | It does not fix the lock order, it is Linux-only, and neither FRR nor BIRD sets it |

## Known Limitations
- A writer that holds the write path while it waits on a plugin (A-3) stays parked after the session has ended: its goroutine and the old session's locks are released only when the plugin drains or stops. The session itself ends and the peer reconnects, which is this spec's goal. Bounding that wait is plugin backpressure design, recorded as a journal row and put to Thomas
- With a writer parked on a plugin the peer also does not reconnect until the plugin drains or stops, although the socket closes and `Run` returns: the session-end defer joins goroutines that wait on the locks the parked writer holds ("Consequences of A-3"). That was true before this spec
- The time cost of the fast path's budget is measured on Thomas's Mac, not on the development host, which gives no time figure (`ai/META-DECISIONS.md`)
- A stalled destination still holds the source peer's read goroutine for up to the write budget (30 s by default): the fast path writes synchronously on that goroutine, and the budget bounds the write, it does not make it non-blocking. A source whose hold time is under the budget can lose its own session to its hold timer meanwhile. AC-6 is met as written (the source's loop goes on, and the stalled destination is retired within the budget). Only a fast path that never blocks removes the hold, which is the territory of the fast-path performance spec (`plan/release/validation/spec-perf-rs-fast-path-regression-and-pass-labels.md`); it is listed under "Defects met in passing, not yet journaled"
- A report that the peer "accepted no data for" the budget can follow a fast-path write that blocked for somewhat less than the full budget: a standing budget is renewed only after a sixteenth of it has passed, so the write that fails had at least fifteen sixteenths of it (28.1 s of 30 s)
- A budget expiry is still reported to plugins as a transport failure (`rpc.ReasonTCPFailure`), as a failed write was before this spec; whether a local-close reason fits better was not decided here

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The close routine quotes RFC 9687 Section 5 above the statement that closes
without a NOTIFICATION, and RFC 4271 Section 8.2.2 above the statement that
drops the connection. Callers of the routine carry the section reference.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run in the main checkout by the main thread: the owed commands above. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
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
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script
