# Spec: a BGP session blocked in a socket write never ends

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped lifecycle defect: a peer that stops reading wedges its session for good, so the RFC 9687 send hold timer and the RFC 4271 hold timer never close the connection and the peer never reconnects |
| RFC gaps | - |
| Depends | - |
| Phase | - |
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
3. The writer releases the write path, which clears the deadline unless the session was retired
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
| A-1 | Setting a write deadline in the past on a Go TCP connection from another goroutine fails a write already blocked in the kernel | Go `net.Conn` contract: deadlines apply to pending I/O; `requestWriteDeadline`'s cancel callback already relies on it | The interrupt step does nothing and the close has to close the socket to unblock the writer | `TestDeadlinePokeFailsBlockedTCPWrite`, on a real loopback TCP pair whose reader never reads | unvalidated |
| A-2 | `net.Pipe` blocks a writer whose other end never reads and honours write deadlines | Go `net.Pipe` documentation; existing tests use it (`pipeSession`) | The unit tests need a real TCP pair throughout | The first red unit test | unvalidated |
| A-3 | No writer holds `writeMu` across an unbounded wait other than the socket write | Read of the writers in the census; the export filter callback (`egressRouteFilter`) runs under `writeMu` and may reach a plugin | A close still waits, with the socket already closed, until that other wait ends | Read the filter call path and its timeout during Phase 3; a test holding the path by hand proves the socket closes regardless | unvalidated |
| A-4 | A 30 s budget per flush does not cut a healthy initial sync short | KEEPALIVE and forward writes already retire a session whose peer accepts nothing for 10 s to 30 s | Healthy slow peers are dropped during a large sync | `TestInitialSyncOutlastsBudget` and the interop scenario's route exchange | unvalidated |
| A-5 | The interop lab can pause and resume the peer container a scenario started | Docker supports it; the lab code was not read for an existing step | The scenario needs a new lab step, which Phase 5 then adds | Read `internal/le/interoplab` at the start of Phase 5 | unvalidated |
| A-6 | Arming a deadline once per destination per batch adds no allocation to the fast path | `SetWriteDeadline` on a `net.TCPConn` is not expected to allocate | The fast path regresses and the arm point has to move | `BenchmarkDirectWriteBudget`, before and after, same toolchain | unvalidated |
| A-7 | The test peer can keep a connection open, keep writing KEEPALIVEs and read nothing | `runMessageLoop` already separates its read deadline loop from its writes, and `silent` is the mirror option | The functional test needs a different stall mechanism | Phase 1 wiring test | unvalidated |
| A-8 | A session close raised as a transient event reaches `ze doctor` through the existing recent-events check | `CodeDoctorRecentEvents` exists in `codes.go`; its producer was not read | The code needs a doctor check of its own in the reactor package | Read `docs/architecture/doctor-and-health-checks.md` and the recent-events producer in Phase 2 | unvalidated |

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
| `TestUnboundedWritersAreBounded` | `internal/component/bgp/reactor/session_wedge_test.go` | AC-3, one sub-test per writer of the census | |
| `TestTeardownClosesBehindStuckWriter` | `internal/component/bgp/reactor/session_wedge_test.go` | AC-4, operator teardown | |
| `TestPeerRemovalAndStopCloseBehindStuckWriter` | `internal/component/bgp/reactor/session_wedge_test.go` | AC-4, reload removal and daemon stop | |
| `TestCloseDeliversNotificationWhenPathFree` | `internal/component/bgp/reactor/session_wedge_test.go` | AC-5, one sub-test per close path | |
| `TestFastPathStalledDestinationDoesNotStallSource` | `internal/component/bgp/reactor/forward_rs_direct_wedge_test.go` | AC-6 | |
| `TestInitialSyncOutlastsBudget` | `internal/component/bgp/reactor/session_wedge_test.go` | AC-7 | |
| `TestWriteBudgetDoesNotLeak` | `internal/component/bgp/reactor/session_wedge_test.go` | R-1: a writer after a released acquisition sees no stale deadline | |
| `TestWriteBudgetExpiryReason` | `internal/component/bgp/reactor/session_wedge_test.go` | AC-10, the close reason and the log line | |
| `TestDeadlinePokeFailsBlockedTCPWrite` | `internal/component/bgp/reactor/session_wedge_test.go` | A-1, on a real loopback TCP pair | |
| `TestCloseHoldsNoSessionLockWhileWaiting` | `internal/component/bgp/reactor/session_wedge_test.go` | A reader of the session state returns while a close waits | |
| `TestKeepaliveGivesUpOnBusyWritePath` | `internal/component/bgp/reactor/session_wedge_test.go` | R-8 | |
| `TestAdminDownRefusalIsBounded` | `internal/component/bgp/reactor/peer_admin_wedge_test.go` | AC-12 | |
| `BenchmarkDirectWriteBudget` | `internal/component/bgp/reactor/forward_rs_direct_wedge_test.go` | AC-11, allocations per forwarded UPDATE | |
| `TestPeerOptionStopReading` | `internal/test/peer/stop_reading_test.go` | The test-peer option parses, refuses unknown keys, and stops the read loop | |

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

## Files to Create
- `internal/component/bgp/reactor/session_wedge_test.go`, `forward_rs_direct_wedge_test.go`, `peer_admin_wedge_test.go` - the unit tests above
- `internal/test/peer/stop_reading_test.go` - the option's unit test
- `test/plugin/session-peer-stops-reading.ci` - the functional test
- `test/interop/scenarios/peer-stops-reading-bird/` - the interop scenario and its checker
- `rfc/discrimination/` records for the two tagged tests, written by `./le rfc record`

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
| Doctor code per problem class | Yes | `internal/core/diagnostic/codes.go`: one code for "session closed: the peer accepted no data for the write budget", named by the WARN line of AC-10, reported at the runtime phase. A-8: Phase 2 reads `docs/architecture/doctor-and-health-checks.md` and either confirms the recent-events check reports it or registers a check in the reactor package |
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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by `// Design:` headers and named here: `docs/architecture/core-design.md` (edited), `docs/architecture/bgp/structural-forwarding.md` (edited), `docs/architecture/testing/ci-format.md` (edited), `docs/architecture/buffer-architecture.md` (unaffected: no buffer ownership or encoding changes), `docs/features/ai-first.md` (unaffected: a code is added through the existing mechanism it describes), `docs/architecture/config/syntax.md` (unaffected: no config grammar changes, only an env description), `docs/guide/add-path.md` (unaffected: no ADD-PATH encoding or negotiation changes). The other pages `./le spec citation anchors` lists only mention these files and are re-checked at closure |
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
   - Verify: tests fail, implement, tests pass; the wiring `.ci` now passes through the budget
3. **Phase: one close routine** -- no close waits on a writer it would unblock
   - Tests: `TestCloseDeliversNotificationWhenPathFree`, `TestPeerRemovalAndStopCloseBehindStuckWriter`, `TestCloseHoldsNoSessionLockWhileWaiting`, and the three Phase 1 unit tests turn green
   - Files: `session_connection.go`, `session_write.go` (`sendHoldTimerExpired`, the NOTIFICATION senders), `session.go` (hold timer callback), `docs/architecture/core-design.md` (close sequence, send hold timer), `docs/architecture/behavior/fsm-established.md`, `docs/architecture/behavior/peer-lifecycle.md`, `ai/digests/bgp-reactor.md`
   - The routine, in order: record the close reason; wait for the write path no longer than the caller's bound (one second for the two timer expiries and the panic guard, the control budget for every other close); if the path came free, send the owed NOTIFICATION, flush with a short budget, send FIN, drain, close; if it stayed held, expire the socket's write deadline, close the socket, and log at ERROR that no NOTIFICATION was sent and why. `s.mu` is taken only to read the connection, and neither `s.mu` nor `openMu` is held across the wait for the write path or across the NOTIFICATION. Once the socket is closed the routine records that fact on the session, so a second call (the cancel goroutine, `Run`'s deferred close) returns at once. It then clears the connection fields in the recorded order, `s.mu` then `writeMu`, waiting for `writeMu` no longer than one second: a writer blocked on the socket has failed by then. If the path is still held, the fields are left set and the routine returns: the session is sealed, its socket is closed, every later write fails, and a sealed session is never reused, so a stale field is read by nobody who could act on it. A NOTIFICATION sent from a read-loop error path waits for the path with the control budget, so the `logNotifyErr` call sites are bounded with no call-site edit
   - Verify A-3 here: read the export filter path that runs under `writeMu` and record its bound
   - Verify: tests fail, implement, tests pass. The two RFC-tagged tests get their discrimination records
4. **Phase: the route-server fast path** -- the last unbounded writer
   - Tests: `TestFastPathStalledDestinationDoesNotStallSource`, `BenchmarkDirectWriteBudget`
   - Files: `forward_rs_direct.go`, `session_write.go` (`appendFwdDirty`, `flushFwdDirty`), `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/forward-congestion-pool.md`, the perf spec's "kept as is" line
   - The budget is armed when a destination first receives a write in a batch and stays armed until the batch's flush clears it. A later write to the same destination in the same batch re-arms it only when another writer held that destination's write path in between, which the session records as a counter read under `writeMu`
   - Verify: tests pass; the allocation count is unchanged against the benchmark run before the change
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
| `./le test bgp plugin session-peer-stops-reading` | AC-8 through the real daemon |
| `./le test bgp plugin deadpeer-holddown` and `./le test bgp plugin open-hold-time-peer-lower-wins` | The existing hold-expiry functional tests still pass |
| The fast-path benchmark with `-benchmem`, through `./le job run`, before and after Phase 4 | AC-11 allocations |
| `./le test integration interop peer-stops-reading-bird` | AC-9 |
| `./le rfc record` for the two tagged tests, then `./le rfc check` | The discrimination records |

The exact form of each command is confirmed against `docs/contributing/running-commands.md` and `./le help` before it is run.

Owed by Thomas, on the Mac: the time figure of the fast path before and after Phase 4 (`./le perf`). The spec's closure does not wait on it; the figure is reported to him as owed.

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
| No writer bypasses the acquisition | A grep for `writeMu.Lock`, `writeMu.TryLock` and `writeMu.LockContext` in `internal/component/bgp/reactor/`, tests excluded, shows only the acquisition and the non-writing takers of the census |
| No hand-set write deadline remains | A grep for `SetWriteDeadline` in `internal/component/bgp/reactor/`, tests excluded, shows only the acquisition, the close routine, the request cancel callback and the two raw-connection refusals |
| The poll loop and `flushForClose` are gone | A grep for `writePathPoll` and `flushForClose` in `internal/component/bgp/reactor/` returns nothing |
| The functional test exists and passes | `./le test bgp plugin session-peer-stops-reading` |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Interrupt, never wait: one budget-arming acquisition and one bounded close routine (approved by Thomas, 2026-11-14) | Budget only: give every write a deadline and leave the close paths | Budget only still queues a close for up to 30 s behind a write and still wedges behind a writer blocked on anything but the socket; it also leaves the bound as a per-caller duty |
| Same | A dedicated non-blocking writer goroutine per session with an output queue, as FRR and BIRD have | It removes the lock, but it rewrites the egress path (the fast path, the initial-sync ordering holds, zero-copy forwarding). Too large for a release fix, and the release fix would still be needed first |
| The session budget is the existing forward budget (`ze.fwd.write.deadline`, 30 s) | The send hold timer as the only bound, as in FRR and BIRD | Ze already retires a peer that accepts nothing for 10 s to 30 s through its KEEPALIVE and forward writes, so one budget states today's behavior as one rule; the send hold timer stays the RFC bound for a session that writes nothing |
| The env name `ze.fwd.write.deadline` is kept and its description widened | Rename it to a session-wide name | Whether it becomes a config leaf, and under what name, is owned by `spec-reactor-tuning-leaves-reach-no-reader`; renaming it here would be done twice |
| The fast path's budget is armed once per destination per batch (approved by Thomas, 2026-11-14) | Arm it on every fast-path write; leave the fast path unbounded | Per write costs a deadline call per forwarded UPDATE on the hottest path; unbounded leaves a writer that can stall a source peer's read loop |
| One-second bound for the timer expiries and the panic guard, the control budget for the other closes | One second everywhere | A teardown or a read-error close on a healthy session can meet a write path that is busy for more than a second during a large sync, and would lose its NOTIFICATION for no reason |
| No socket option (`TCP_USER_TIMEOUT`) | Add it as the bound | It does not fix the lock order, it is Linux-only, and neither FRR nor BIRD sets it |

## Known Limitations
- A writer that holds the write path while it waits on something other than the socket still delays the session's last cleanup until that wait ends. The socket is already closed by then, so the peer is released and the RFC close is met. A-3 records the reading owed in Phase 3; if that wait turns out to be unbounded, it is fixed in this spec
- The time cost of the fast path's budget is measured on Thomas's Mac, not on the development host, which gives no time figure (`ai/META-DECISIONS.md`)

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
