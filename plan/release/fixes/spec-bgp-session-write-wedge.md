# Spec: a BGP session blocked in a socket write never ends

| Field | Value |
|-------|-------|
| Status | skeleton |
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

## Required Reading

### RFC text
- [ ] `rfc/full/rfc9687.txt` Section 5 - the send hold timer exists for exactly this failure
  → Constraint: Section 5: "If the local system does not send any BGP messages within the period specified in SendHoldTime, then a NOTIFICATION message with the "Send Hold Timer Expired" Error Code MAY be sent and the BGP connection MUST be closed."
- [ ] `rfc/full/rfc4271.txt` Section 6.5 and Section 8.2.2 (Established, Event 10)
  → Constraint: Section 6.5: "If a system does not receive successive KEEPALIVE, UPDATE, and/or NOTIFICATION messages within the period specified in the Hold Time field of the OPEN message, then the NOTIFICATION message with the Hold Timer Expired Error Code is sent and the BGP connection is closed."
  → Constraint: Section 8.2.2, Established: "If the HoldTimer_Expires event occurs (Event 10), the local system: ... - drops the TCP connection, ... - changes its state to Idle."

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - the queued-route drain and the config-route sends call `sendUpdateWithSplit(context.Background(), ...)`; the held phase writes through `sendUpdateHeld` under `holdWrites` (`writeMu.Lock`)
- [ ] `internal/component/bgp/reactor/peer_static_wire.go` - static route announces and withdrawals call `sendUpdateWithSplit` / `sendBodyWithSplit` with `context.Background()`
- [ ] `internal/component/bgp/reactor/peer_send.go` - `sendUpdateWithSplit` passes the context through `sendUpdateOwnedWithSplit` to `sendUpdateCounted` for every chunk
- [ ] `internal/component/bgp/reactor/session_write.go` - `sendUpdateCounted` calls `requestWriteDeadline` only when `ctx.Done() != nil`, so a background context gets no `SetWriteDeadline`; `requestWriteDeadline` itself sets one only from `ctx.Deadline()` or on cancellation. `sendUpdateHeld`, `SendUpdate` and `SendAnnounce` set none either. `writeMessageHeld` (KEEPALIVE, OPEN, NOTIFICATION) does set one, but `writeMessageWithin` takes `writeMu.Lock` first, so it queues behind the stuck writer
- [ ] `internal/component/bgp/reactor/session_connection.go` - `closeConn` takes `s.mu`, then `flushForClose` takes `writeMu.Lock` and flushes, and only then calls `CloseWrite` and `Close`. `ceaseAfterPanic` is the one path that bounds the wait (`tryNotificationWithin`) and closes the socket first, and its comment names this exact wedge
- [ ] `internal/component/bgp/reactor/session.go` - the hold timer callback sends its NOTIFICATION through `sendNotificationWithin`, which takes `writeMu.Lock` unbounded, then signals `errChan`; `Run`'s cancel goroutine and its deferred `closeConn` both reach `flushForClose`
- [ ] `internal/component/bgp/reactor/session_write.go` `sendHoldTimerExpired` - `logNotifyErr` (blocking `writeMu`), then `closeConn`: the RFC 9687 close also wedges
- [ ] `internal/component/bgp/reactor/forward_pool.go` `fwdBatchHandler` and `session_write.go` `flushFwdDirty` - set a write deadline (`fwdWriteDeadline`, 30 s default) after taking `writeMu`: the forward rails are bounded

No TCP user timeout or keepalive option on the BGP socket breaks the wait: none
is set under `internal/component/bgp`, and keepalive probes do not fire while
data is outstanding, and a live peer answers zero-window probes.

**Behavior to preserve:**
- The NOTIFICATION is still attempted before the close when the write path is free
- The forward rails keep their own write deadline and their `writeMu` discipline
- The lock order `s.mu` then `writeMu`

**Behavior to change:**
- Writes with no deadline (initial sync, static routes, `SendUpdate`, `SendAnnounce`, the held initial-sync phase) get a bounded write
- No close path waits on `writeMu` with the socket still open

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Request write deadline | `session_write.go` `requestWriteDeadline` | Binds a socket write to its request's context deadline and cancellation | Extends: the design decides whether background callers get a bounded context or a session default |
| Control write deadline | `session_write.go` `writeMessageHeld`, `controlWriteDeadline` | Bounds KEEPALIVE, OPEN and NOTIFICATION writes | Extends: the same budget idea for UPDATE writers |
| Forward write deadline | `forward_pool.go` `fwdBatchHandler`, `fwdWriteDeadline` | Bounds forward-pool and dirty-flush writes | Extends: the bound already used by the forward rails |
| Bounded write-path wait | `session_write.go` `tryWriteMessageWithin`; `session_connection.go` `ceaseAfterPanic` | Waits for `writeMu` no longer than a deadline, then closes the socket without the NOTIFICATION | Extends: the shape every close path needs |
| Write retirement | `session_write.go` `retireWrite` | Seals and closes the connection under `writeMu` after a failed write | Does not apply directly: it runs only once a write returns |
| Send hold timer | `session_write.go` `sendHoldTimerExpired` | RFC 9687 close when nothing was sent for SendHoldTime | Fixed by this spec: today it wedges on `writeMu` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A route write from the initial sync, a static route, or an API route send, on the peer's session
- The session end: hold timer, send hold timer, a received NOTIFICATION, a teardown, `Run`'s cancel goroutine

### Transformation Path
1. Writer takes `writeMu` (`sendUpdateCounted`, `holdWrites`) and writes to `bufWriter` and the socket
2. Close path takes `s.mu`, then `writeMu` (`closeConn`, `flushForClose`), then closes the socket

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to peer | TCP socket write | No |

### Changed-Wire Census

**Search evidence:** no wire format changes: the fix bounds when a write gives up and when the socket closes, and the bytes written are unchanged.

### Integration Points
- `requestWriteDeadline`, `tryWriteMessageWithin`, `retireWrite` - the existing bounds this extends

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | skeleton |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [skeleton: session with a peer that stops reading] | → | `closeConn` | [skeleton: test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [skeleton] | `internal/component/bgp/reactor/` | AC-1 to AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [skeleton] | [skeleton] | A peer that stops reading is dropped and reconnects | |

## Files to Modify
- `internal/component/bgp/reactor/session_write.go` - bound the UPDATE writers; bound the hold and send hold NOTIFICATION waits
- `internal/component/bgp/reactor/session_connection.go` - `closeConn` closes the socket before any wait on `writeMu`
- `internal/component/bgp/reactor/session.go` - the hold timer callback

## Implementation Steps
1. **Phase: Wiring** -- [skeleton]

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Established session, peer stops reading during the initial sync, send hold timer expires | The connection is closed within SendHoldTime plus the NOTIFICATION budget, `Run` returns and the peer reconnects | The NOTIFICATION may be absent: RFC 9687 Section 5 makes it MAY, the close MUST |
| AC-2 | Same, receive hold timer expires first | The connection is dropped and the FSM reaches Idle (RFC 4271 Section 8.2.2, Event 10) | As AC-1 |
| AC-3 | A static-route, `SendUpdate` or `SendAnnounce` write to a peer that stopped reading | The write returns an error within a bounded time and the session is retired | None |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `lock-held-across-a-join` | 2026-11-14 | a write with no deadline holds `writeMu`; `closeConn` and the hold and send-hold expiry wait on it before closing |

## Known Limitations
- [skeleton: to be designed]

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/bgp/reactor/` and the functional tests named in the TDD Test Plan. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`), not library-only
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
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script
</content>
</invoke>
