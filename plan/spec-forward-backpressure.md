# Spec: forward-backpressure

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/reactor/forward_pool.go` - forward pool implementation
4. `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate dispatches to pool
5. `internal/component/bgp/reactor/session_write.go` - TCP write primitives
6. `internal/component/bgp/plugins/rs/server.go` - bgp-rs forward loop

## Task

Eliminate head-of-line blocking in Ze's forward path. Currently one slow destination peer blocks forwarding to all other peers because: (1) `fwdPool.Dispatch()` blocks when a peer's channel is full, (2) `ForwardUpdate` iterates peers sequentially so a blocked dispatch stalls later peers, (3) TCP writes have no deadline so a stuck peer blocks the worker indefinitely, (4) bgp-rs `forwardLoop` is a single goroutine so one blocked forward RPC stalls the entire pipeline.

Six phases: write deadline, TryDispatch + overflow, congestion tracking, ForwardUpdate integration, congestion events, multiple forward senders.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - Forward pool architecture, per-peer workers
  -> Constraint: per-peer FIFO ordering must be preserved
  -> Constraint: cache lifecycle (Retain/Release) must be maintained
- [ ] `.claude/rules/goroutine-lifecycle.md` - long-lived workers only
  -> Constraint: no per-event goroutines in hot paths
- [ ] `.claude/rules/buffer-first.md` - no new allocations in encoding hot path
  -> Constraint: reuse slices, no append in hot path
- [ ] `.claude/rules/go-standards.md` - env var pattern
  -> Constraint: use env.MustRegister + env.GetDuration/GetInt

**Key insights:**
- fwdPool uses per-destination-peer workers with drain-batch pattern
- Session.conn is net.Conn accessed under Session.mu.RLock
- bgp-rs forwardLoop is a single goroutine reading from forwardCh channel
- env vars registered at package level via `var _ = env.MustRegister(...)`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_pool.go` - per-peer worker pool with blocking Dispatch, drain-batch, idle timeout
  -> Constraint: fwdBatchHandler acquires session.writeMu, writes all items, flushes once
  -> Constraint: done callbacks guaranteed via safeBatchHandle even on panic
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate iterates peers, calls fp.Dispatch(key, item) blocking
  -> Constraint: cache Retain before dispatch, Release in done callback
- [ ] `internal/component/bgp/reactor/session_write.go` - writeRawUpdateBody/writeUpdate/flushWrites under writeMu
  -> Constraint: conn accessed under session.mu.RLock, writes under writeMu
- [ ] `internal/component/bgp/reactor/reactor.go` - env var registration pattern, fwdPool creation in New()
  -> Constraint: env vars use env.MustRegister at package level
- [ ] `internal/component/bgp/plugins/rs/server.go` - single forwardLoop goroutine for fire-and-forget cache-forward RPCs
  -> Constraint: forwardCh capacity 16, blocks when full
- [ ] `internal/component/bgp/server/events.go` - event delivery to plugins
  -> Constraint: events delivered via process.Deliver to long-lived goroutines

**Behavior to preserve:**
- Per-peer FIFO ordering in forward pool
- Cache lifecycle: Retain before dispatch, Release in done callback
- Zero-copy forwarding when contexts match
- Existing inbound backpressure (session pause/resume)
- safeBatchHandle panic recovery and done callback guarantees
- drain-batch buffer reuse pattern
- Idle timeout worker cleanup

**Behavior to change:**
- Add write deadline on TCP forward writes (Phase 1)
- Make dispatch non-blocking with overflow buffer (Phase 2)
- Add congestion tracking with callbacks (Phase 3)
- Replace blocking Dispatch with TryDispatch in ForwardUpdate (Phase 4)
- Add peer-congested/peer-resumed events (Phase 5)
- Replace single forwardLoop with pool of N senders in bgp-rs (Phase 6)

## Data Flow (MANDATORY)

### Entry Point
- Cached UPDATE in RecentUpdateCache, triggered by ForwardUpdate RPC from bgp-rs plugin
- ForwardUpdate iterates matching peers, pre-computes fwdItems, dispatches to fwdPool

### Transformation Path
1. ForwardUpdate pre-computes rawBodies/updates per destination peer
2. Retain cache entry, create done callback with Release
3. Dispatch to per-peer worker via fwdPool.Dispatch (currently blocking)
4. Worker drain-batches items, calls fwdBatchHandler
5. Handler acquires writeMu, writes to bufWriter, flushes
6. done callbacks fire Release

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ForwardUpdate -> fwdPool | fwdItem over channel | [ ] |
| fwdPool -> Session | writeMu + bufWriter.Write | [ ] |
| bgp-rs -> Engine | updateRoute RPC over MuxConn | [ ] |

### Integration Points
- `fwdPool.Dispatch` called from `ForwardUpdate` in reactor_api_forward.go
- `fwdBatchHandler` calls `session.writeRawUpdateBody`/`session.writeUpdate`
- `asyncForward` in bgp-rs sends to forwardCh -> forwardLoop -> updateRoute

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| fwdPool.TryDispatch called from ForwardUpdate | -> | TryDispatch + overflow buffer | TestFwdPool_TryDispatch |
| fwdBatchHandler TCP write | -> | SetWriteDeadline before write | TestFwdBatchHandler_WriteDeadline |
| fwdPool congestion callback | -> | onCongested/onResumed callbacks | TestFwdPool_CongestionCallbacks |
| bgp-rs multiple forward senders | -> | Pool of N sender goroutines | TestRouteServer_MultipleSenders |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | fwdBatchHandler writes to TCP | SetWriteDeadline called before write, cleared after |
| AC-2 | TryDispatch on non-full channel | Item enqueued, returns true |
| AC-3 | TryDispatch on full channel | Returns false immediately (non-blocking) |
| AC-4 | DispatchOverflow adds to overflow buffer | Item stored, processed after channel drains |
| AC-5 | Overflow buffer exceeds overflowMax | Oldest item dropped, its done callback called |
| AC-6 | Worker processes overflow items | Items moved from overflow to processing after channel batch |
| AC-7 | Pool Stop with overflow items | All overflow done callbacks fired |
| AC-8 | Congestion detected (TryDispatch fails) | onCongested callback fires once |
| AC-9 | Worker drains below low-water mark | onResumed callback fires once |
| AC-10 | ForwardUpdate uses TryDispatch | Non-blocking dispatch, overflow on failure |
| AC-11 | bgp-rs with multiple senders | N goroutines read from shared forwardCh |
| AC-12 | bgp-rs shutdown with multiple senders | All senders exit cleanly via WaitGroup |
| AC-13 | Write deadline expires on stuck TCP | Error returned, handled by existing error path |
| AC-14 | Congestion hysteresis | Mark at full, clear at 25% capacity |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFwdBatchHandler_WriteDeadline` | `forward_pool_test.go` | AC-1, AC-13 | |
| `TestFwdPool_TryDispatch` | `forward_pool_test.go` | AC-2, AC-3 | |
| `TestFwdPool_DispatchOverflow` | `forward_pool_test.go` | AC-4, AC-6 | |
| `TestFwdPool_OverflowMax` | `forward_pool_test.go` | AC-5 | |
| `TestFwdPool_StopFiresOverflowDone` | `forward_pool_test.go` | AC-7 | |
| `TestFwdPool_CongestionCallbacks` | `forward_pool_test.go` | AC-8, AC-9, AC-14 | |
| `TestFwdPool_CongestionHysteresis` | `forward_pool_test.go` | AC-14 | |
| `TestForwardUpdate_TryDispatch` | `reactor_api_forward_test.go` | AC-10 | |
| `TestRouteServer_MultipleSenders` | `server_test.go` | AC-11, AC-12 | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| overflowMax | 1-10000 | 10000 | 0 (uses default) | N/A (capped) |
| writeDeadline | 1s-5m | 5m | N/A (uses default) | N/A (uses default) |
| rs.fwd.senders | 1-64 | 64 | 0 (uses default) | N/A (capped) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | Internal engine optimization, no user-facing config change | |

### Future
- Property tests for overflow ordering under concurrent access
- Benchmark comparing blocking vs non-blocking dispatch throughput

## Files to Modify
- `internal/component/bgp/reactor/forward_pool.go` - TryDispatch, overflow, congestion, write deadline
- `internal/component/bgp/reactor/reactor_api_forward.go` - Replace Dispatch with TryDispatch
- `internal/component/bgp/reactor/reactor.go` - env var registrations, wire congestion callbacks
- `internal/component/bgp/plugins/rs/server.go` - multiple forward senders
- `internal/component/bgp/server/events.go` - peer-congested/peer-resumed event types (if needed)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | Internal optimization |

## Files to Create
- (none - all changes to existing files)

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### Implementation Phases

1. **Phase 1: Write deadline on TCP forward writes** -- Add SetWriteDeadline in fwdBatchHandler
   - Tests: TestFwdBatchHandler_WriteDeadline
   - Files: forward_pool.go, reactor.go (env var)
   - Verify: test fails -> implement -> test passes

2. **Phase 2: TryDispatch + per-peer overflow buffer** -- Non-blocking dispatch with bounded overflow
   - Tests: TestFwdPool_TryDispatch, TestFwdPool_DispatchOverflow, TestFwdPool_OverflowMax, TestFwdPool_StopFiresOverflowDone
   - Files: forward_pool.go, reactor.go (env vars)
   - Verify: tests fail -> implement -> tests pass

3. **Phase 3: Congestion tracking + callbacks** -- Hysteresis-based congestion detection
   - Tests: TestFwdPool_CongestionCallbacks, TestFwdPool_CongestionHysteresis
   - Files: forward_pool.go
   - Verify: tests fail -> implement -> tests pass

4. **Phase 4: ForwardUpdate integration** -- Replace Dispatch with TryDispatch + DispatchOverflow
   - Tests: TestForwardUpdate_TryDispatch
   - Files: reactor_api_forward.go
   - Verify: tests fail -> implement -> tests pass

5. **Phase 5: peer-congested/peer-resumed events** -- Emit events through reactor callbacks
   - Tests: (covered by congestion callback tests)
   - Files: reactor.go, events.go (if new event types needed)
   - Verify: existing tests pass

6. **Phase 6: Multiple forward senders in bgp-rs** -- Replace single forwardLoop with N senders
   - Tests: TestRouteServer_MultipleSenders
   - Files: rs/server.go
   - Verify: tests fail -> implement -> tests pass

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation |
| Correctness | FIFO ordering preserved per peer, cache lifecycle intact |
| Naming | Env vars follow ze.xxx.yyy pattern |
| Data flow | Overflow items processed in order, done callbacks always fire |
| Rule: goroutine-lifecycle | No per-event goroutines added |
| Rule: buffer-first | No new allocations in hot path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| TryDispatch method on fwdPool | grep TryDispatch forward_pool.go |
| DispatchOverflow method on fwdPool | grep DispatchOverflow forward_pool.go |
| Write deadline in fwdBatchHandler | grep SetWriteDeadline forward_pool.go |
| Overflow buffer on fwdWorker | grep overflow forward_pool.go |
| Congestion callbacks on fwdPool | grep onCongested forward_pool.go |
| ForwardUpdate uses TryDispatch | grep TryDispatch reactor_api_forward.go |
| Multiple forward senders in bgp-rs | grep fwdSenders server.go |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Overflow buffer bounded by overflowMax |
| Input validation | env var defaults are safe, invalid values use defaults |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
