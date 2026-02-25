# Spec: backpressure

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - engine/plugin boundary, reactor design
4. `internal/plugins/bgp/reactor/session.go` - Session.Run() read loop
5. `internal/plugins/bgp/reactor/peer.go` - deliverChan, runOnce()
6. `internal/plugins/bgp/reactor/delivery.go` - deliveryChannelCapacity

## Task

Add proactive backpressure to the BGP engine. Under stress, the engine should be able to **pause reading from specific peers** (or all peers), causing the kernel recv buffer to fill and TCP window to shrink. This creates natural TCP-level backpressure to the sender.

The write side (KEEPALIVE sending) is independent and continues during read pause. If paused long enough, the hold timer will expire (no received messages to reset it) and the session drops — this is acceptable and acts as a safety valve.

Additionally, document (but do NOT implement) a future Tier 2 mechanism: shrinking `SO_RCVBUF` via syscall to force TCP window toward zero for emergency situations.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor, peer, session architecture
  → Decision: Each peer has its own Session with dedicated read goroutine
  → Constraint: Write path (KEEPALIVE timer) is independent of read path
- [ ] `.claude/rules/goroutine-lifecycle.md` - goroutine patterns
  → Constraint: No per-event goroutines. Pause gate must be checked in existing read loop, not spawned per-message.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - BGP hold timer behavior
  → Constraint: RFC 4271 Section 6.5 — hold timer resets on ANY received message. If we stop reading, hold timer will expire after the negotiated interval.

**Key insights:**
- Session.Run() is a tight loop calling readAndProcessMessage() — no deadline in production path
- deliverChan (256 items) already provides implicit backpressure when full
- KEEPALIVE sending is on the write timer, completely decoupled from reading
- Hold timer expiry during pause is the natural safety valve — acceptable behavior

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugins/bgp/reactor/session.go` - Session.Run() loop at line 726: tight loop calling readAndProcessMessage(). No read deadline in production. Close-on-cancel goroutine watches ctx.Done() and errChan.
- [x] `internal/plugins/bgp/reactor/peer.go` - Peer.runOnce() at line 1083: creates deliverChan (capacity 256), starts delivery goroutine, calls session.Run(). Delivery goroutine drains channel after session exits.
- [x] `internal/plugins/bgp/reactor/delivery.go` - deliveryChannelCapacity = 256. deliveryItem holds peerInfo + RawMessage.
- [x] `internal/plugins/bgp/reactor/reactor.go` - notifyMessageReceiver() at line 4126: for received UPDATEs with deliverChan, does blocking send `peer.deliverChan <- item` (line 4239). No select/default — blocks if full.

**Behavior to preserve:**
- Session.Run() loop structure: read → process → loop
- deliverChan capacity and blocking send semantics in notifyMessageReceiver()
- Close-on-cancel goroutine pattern for clean shutdown
- Hold timer callback via errChan for session termination
- KEEPALIVE timer independence (write path unaffected)
- Zero-copy WireUpdate path through delivery channel
- Delivery goroutine drain on session exit (close channel, wait for done)

**Behavior to change:**
- Add a pause gate in Session.Run() loop — before calling readAndProcessMessage(), check if paused
- Add Pause()/Resume() methods on Session
- Add PauseReading()/ResumeReading() on Peer (delegates to Session)
- Add PausePeer()/ResumePeer()/PauseAllReads()/ResumeAllReads() on Reactor

## Data Flow (MANDATORY)

### Entry Point
- Stress signal enters via Reactor API: PausePeer(addr) or PauseAllReads()
- Signal propagates: Reactor → Peer → Session → read loop gate

### Transformation Path
1. Reactor.PausePeer(addr) looks up peer by address, calls peer.PauseReading()
2. Peer.PauseReading() calls session.Pause() on the active session
3. Session.Pause() sets a pause flag (atomic) that the read loop checks
4. Session.Run() loop: before readAndProcessMessage(), checks pause flag — if set, blocks on resume signal or context cancellation
5. Session.Resume() clears the pause flag, unblocking the read loop
6. While paused: write path (KEEPALIVE timer) continues independently
7. If hold timer expires during pause: errChan receives ErrHoldTimerExpired, cancel goroutine closes connection, Run() returns

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor → Peer | Method call PauseReading() | [ ] |
| Peer → Session | Method call Pause() / Resume() | [ ] |
| Session read loop ↔ pause gate | atomic.Bool + sync.Cond or channel | [ ] |

### Integration Points
- `Session.Run()` (session.go:726) - pause gate inserted before readAndProcessMessage() call
- `Session.errChan` - hold timer still fires during pause, triggers session close
- `Peer.runOnce()` (peer.go:1083) - deliverChan setup unchanged
- `Reactor.notifyMessageReceiver()` (reactor.go:4238) - blocking send unchanged

### Architectural Verification
- [ ] No bypassed layers — pause signal flows Reactor → Peer → Session
- [ ] No unintended coupling — Session only knows about its own pause state
- [ ] No duplicated functionality — extends existing close-on-cancel select pattern
- [ ] Zero-copy preserved — pause gate is before ReadFull, does not affect WireUpdate path

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Session.Pause() called during active read loop | Read loop stops calling readAndProcessMessage(), blocks on resume or context cancel |
| AC-2 | Session.Resume() called after Pause() | Read loop unblocks and resumes reading messages |
| AC-3 | Context cancelled while paused | Session.Run() returns promptly (not stuck on pause) |
| AC-4 | Hold timer expires while paused | Session closes cleanly via errChan → close-on-cancel goroutine |
| AC-5 | Peer.PauseReading() with active session | Delegates to session.Pause(), logs at WARN |
| AC-6 | Peer.PauseReading() with no active session | No-op, no panic |
| AC-7 | Reactor.PausePeer(addr) with valid peer | Pauses that peer's read loop |
| AC-8 | Reactor.PauseAllReads() with multiple peers | All peers' read loops pause |
| AC-9 | Reactor.ResumeAllReads() after PauseAllReads() | All peers resume reading |
| AC-10 | Pause() called on already-paused session | No-op (idempotent) |
| AC-11 | Resume() called on non-paused session | No-op (idempotent) |
| AC-12 | KEEPALIVE timer fires while read is paused | KEEPALIVE is still sent (write path independent) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionPauseBlocksRead` | `internal/plugins/bgp/reactor/session_test.go` | AC-1: Pause() stops read loop from calling ReadFull | |
| `TestSessionResumeUnblocksRead` | `internal/plugins/bgp/reactor/session_test.go` | AC-2: Resume() allows read loop to continue | |
| `TestSessionPauseCancelContext` | `internal/plugins/bgp/reactor/session_test.go` | AC-3: Context cancel while paused returns promptly | |
| `TestSessionPauseHoldTimerExpiry` | `internal/plugins/bgp/reactor/session_test.go` | AC-4: Hold timer still fires and closes session while paused | |
| `TestSessionPauseIdempotent` | `internal/plugins/bgp/reactor/session_test.go` | AC-10, AC-11: Multiple Pause/Resume calls are safe | |
| `TestPeerPauseReadingDelegates` | `internal/plugins/bgp/reactor/peer_test.go` | AC-5, AC-6: Delegates to session, handles nil session | |
| `TestReactorPausePeer` | `internal/plugins/bgp/reactor/reactor_test.go` | AC-7: PausePeer pauses specific peer | |
| `TestReactorPauseAllReads` | `internal/plugins/bgp/reactor/reactor_test.go` | AC-8, AC-9: PauseAllReads/ResumeAllReads affects all peers | |
| `TestSessionPauseKeepaliveContinues` | `internal/plugins/bgp/reactor/session_test.go` | AC-12: Write path independent of read pause | |

### Boundary Tests (MANDATORY for numeric inputs)
No new numeric inputs introduced.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (deferred) | — | Full integration with actual TCP connection and backpressure measurement | |

### Future (if deferring any tests)
- Full TCP backpressure functional test deferred — requires actual TCP connection with measurable window behavior, which is complex to set up in CI. Unit tests with net.Pipe verify the pause/resume mechanism itself.

## Files to Modify
- `internal/plugins/bgp/reactor/session.go` - Add pause gate in Run() loop, Pause()/Resume()/IsPaused() methods
- `internal/plugins/bgp/reactor/peer.go` - Add PauseReading()/ResumeReading()/IsReadPaused() methods
- `internal/plugins/bgp/reactor/reactor.go` - Add PausePeer()/ResumePeer()/PauseAllReads()/ResumeAllReads()

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | — |
| RPC count in architecture docs | [ ] No | — |
| CLI commands/flags | [ ] No | — |
| CLI usage/help text | [ ] No | — |
| API commands doc | [ ] No | — |
| Plugin SDK docs | [ ] No | — |
| Editor autocomplete | [ ] No | — |
| Functional test for new RPC/API | [ ] No | — |

## Files to Create
- None — all changes are additions to existing files

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write Session pause/resume unit tests** — Tests for Pause(), Resume(), context cancel while paused, hold timer during pause, idempotency
   → Review: Does each test use net.Pipe to control the connection? Are cancel paths covered?

2. **Run tests** → Verify FAIL (paste output). Tests should fail because Pause()/Resume() don't exist yet.
   → Review: Tests fail for the RIGHT reason (missing methods), not syntax errors?

3. **Implement Session pause gate** — Add atomic pause flag and channel-based gate to Session. Modify Run() loop to check before readAndProcessMessage(). Add Pause()/Resume()/IsPaused() methods. Log at WARN on pause/resume.
   → Review: Is the pause gate non-blocking on the fast path (when NOT paused)? Does it compose correctly with existing close-on-cancel goroutine?

4. **Run tests** → Verify PASS
   → Review: All tests deterministic? No race conditions under `-race`?

5. **Write Peer/Reactor level tests** — PauseReading/ResumeReading on Peer, PausePeer/ResumePeer/PauseAllReads/ResumeAllReads on Reactor
   → Review: Nil session handling? Peer not found handling?

6. **Run tests** → Verify FAIL

7. **Implement Peer and Reactor methods** — Peer delegates to session, Reactor iterates peers
   → Review: Thread safety? Reactor lock ordering correct?

8. **Run tests** → Verify PASS

9. **Verify all** — `make ze-lint && make ze-unit-test && make ze-functional-test`
   → Review: Zero lint issues? No regressions?

10. **Final self-review** — Re-read all changes. Check: unused code, debug statements, consistent naming, logging.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 (fix syntax/types) |
| Test fails wrong reason | Step 1 or 5 (fix test) |
| Test fails behavior mismatch | Re-read Session.Run() → may need different pause gate mechanism |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Race detected | Step 3 or 7 — review atomic/lock usage |

## Design Notes

### Pause Gate Mechanism

The pause gate uses `atomic.Bool` for the fast path check (no overhead when not paused) plus a channel for blocking:

| Field | Type | Purpose |
|-------|------|---------|
| `paused` | `atomic.Bool` | Fast-path check — false in normal operation |
| `resumeCh` | `chan struct{}` | Blocking wait — closed by Resume() to unblock |
| `pauseMu` | `sync.Mutex` | Protects resumeCh create/close |

In Session.Run() loop, before readAndProcessMessage():
1. Check `s.paused.Load()` — fast path returns false, zero overhead
2. If paused: select on `s.resumeCh` or `ctx.Done()` or `s.errChan`
3. Resume() closes resumeCh, unblocking the select

This gives O(0) overhead on the normal (unpaused) path — just an atomic load.

### TCP Window Squeeze (Future Tier 2 — Research Only)

For emergency situations, `SO_RCVBUF` can be shrunk via syscall to force TCP window toward minimum:

| Step | Detail |
|------|--------|
| 1. Type-assert | `conn.(*net.TCPConn)` — fails gracefully for net.Pipe in tests |
| 2. SyscallConn | `rawConn.Control(func(fd uintptr) { ... })` |
| 3. Shrink | `syscall.SetsockoptInt(fd, SOL_SOCKET, SO_RCVBUF, 0)` |
| 4. Kernel minimum | Linux enforces ~256 bytes minimum, not literal zero |
| 5. Restore | Must save and restore original value |
| 6. Logging | ERROR level — this is an emergency measure |

Caveats: platform-dependent, Go type assertion required, kernel won't set literal zero, recovery timing unpredictable. Not implemented in this spec.

### Logging

| Event | Level | Attributes |
|-------|-------|------------|
| Read paused | WARN | peer address, reason |
| Read resumed | WARN | peer address, pause duration |
| Hold timer expired while paused | WARN | peer address, pause duration |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

RFC 4271 Section 6.5: Hold timer resets on any received BGP message. Pausing reads means hold timer will expire after the negotiated interval. This is intentional — it's the safety valve.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Pause reading from specific peers | | | |
| Pause reading from all peers | | | |
| Resume reading | | | |
| KEEPALIVE continues during pause | | | |
| Hold timer expiry as safety valve | | | |
| WARN-level logging on pause/resume | | | |
| Document TCP window squeeze for future | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |
| AC-11 | | | |
| AC-12 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSessionPauseBlocksRead | | | |
| TestSessionResumeUnblocksRead | | | |
| TestSessionPauseCancelContext | | | |
| TestSessionPauseHoldTimerExpiry | | | |
| TestSessionPauseIdempotent | | | |
| TestPeerPauseReadingDelegates | | | |
| TestReactorPausePeer | | | |
| TestReactorPauseAllReads | | | |
| TestSessionPauseKeepaliveContinues | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/bgp/reactor/session.go` | | |
| `internal/plugins/bgp/reactor/peer.go` | | |
| `internal/plugins/bgp/reactor/reactor.go` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
