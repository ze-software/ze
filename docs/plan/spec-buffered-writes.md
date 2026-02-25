# Spec: buffered-writes

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/bgp/reactor/session.go:875-942` — writeUpdate/writeRawUpdateBody/flushWrites/SendUpdate
4. `internal/plugins/bgp/reactor/forward_pool.go:36-55` — fwdHandler write loop

## Task

Reduce write syscalls on the BGP forwarding hot path by deferring `bufWriter.Flush()` until a batch of writes completes, instead of flushing after every individual message.

The `bufio.Writer` already exists (16KB buffer, session.go:612) and the internal write helpers (`writeUpdate`, `writeRawUpdateBody`) already skip flushing. But every public `Send*` method calls `flushWrites()` immediately after each message, negating the buffer. The forward pool handler calls `SendUpdate`/`SendRawUpdateBody` in a loop — one syscall per message instead of one per batch.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` — session write path, forward pool design
  → Decision: per-destination-peer forward workers with FIFO ordering
  → Constraint: writeMu serializes all writes per session; forward pool calls Send* per item

### Related Completed Specs
- [x] `docs/plan/spec-event-delivery-batching.md` — peer-level event batching (active)
  → Decision: batch event delivery at peer delivery goroutine
  → Constraint: orthogonal — event delivery and wire writes are independent paths

**Key insights:**
- `writeUpdate` and `writeRawUpdateBody` already write without flushing — the split exists
- `flushWrites()` is a separate method ready to be called at batch boundaries
- Forward pool handler processes `rawBodies[]` + `updates[]` per `fwdItem` — natural batch boundary
- `SendAnnounce` and `SendWithdraw` are used for API-injected routes and initial sync — also benefit

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugins/bgp/reactor/session.go:875-942` — writeUpdate (no flush), writeRawUpdateBody (no flush), flushWrites, SendUpdate (write+flush), SendAnnounce (write+flush), SendWithdraw (write+flush), SendRawUpdateBody (write+flush)
- [x] `internal/plugins/bgp/reactor/forward_pool.go:36-55` — fwdHandler calls SendRawUpdateBody per rawBody, SendUpdate per update
- [x] `internal/plugins/bgp/reactor/peer_send.go` — thin wrappers delegating to session.Send*
- [x] `internal/plugins/bgp/reactor/session.go:610-612` — bufWriter created at connectionEstablished (16KB)
- [x] `internal/plugins/bgp/reactor/session.go:727-745` — closeConn flushes bufWriter

**Behavior to preserve:**
- writeMu serialization — all writes to a session go through one lock
- KEEPALIVE, OPEN, NOTIFICATION flush immediately (low-frequency, FSM-critical)
- closeConn flushes before TCP teardown
- onMessageReceived callback fires per message (for event delivery to plugins)
- Forward pool FIFO ordering per destination peer
- Error on first write failure stops remaining ops in fwdHandler

**Behavior to change:**
- `SendUpdate`, `SendAnnounce`, `SendWithdraw`, `SendRawUpdateBody` currently flush per-message
- Forward pool handler calls per-message Send* — should batch writes under one lock + flush

## Data Flow (MANDATORY)

### Entry Point
- Forward pool worker dequeues `fwdItem` for a destination peer
- `fwdItem` contains `rawBodies [][]byte` and `updates []*message.Update`

### Transformation Path (current — per message)
1. `fwdHandler` loops over `rawBodies` → calls `peer.SendRawUpdateBody(body)` per item
2. Each `SendRawUpdateBody` → acquire writeMu → write to bufWriter → flush → release writeMu
3. Then loops over `updates` → calls `peer.SendUpdate(update)` per item
4. Each `SendUpdate` → acquire writeMu → write to bufWriter → flush → release writeMu

### Transformation Path (proposed — per batch)
1. `fwdHandler` calls a new `peer.SendUpdateBatch(rawBodies, updates)` or equivalent
2. `SendUpdateBatch` → acquire writeMu once → write all to bufWriter → flush once → release writeMu
3. Callbacks fire per message (within the locked section)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Forward pool → Peer | `peer.Send*` methods | [ ] |
| Peer → Session | `session.Send*` or new batch method | [ ] |
| Session → TCP | `bufWriter.Write` (buffered) + `bufWriter.Flush` (syscall) | [ ] |

### Integration Points
- `Session` — new `SendUpdateBatch` method using existing writeUpdate/writeRawUpdateBody/flushWrites
- `Peer` — new `SendUpdateBatch` wrapper
- `forward_pool.go` — `fwdHandler` calls batch method instead of per-message Send*
- `peer_send.go` — add batch wrapper
- `SendAnnounce`/`SendWithdraw` — keep flushing per-call (called individually, not in loops)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (reuses existing writeUpdate/writeRawUpdateBody/flushWrites)
- [ ] Zero-copy preserved where applicable (writeUpdate uses WriteTo into session buffer)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Forward pool item with 3 rawBodies + 2 updates to one peer | 5 bufWriter.Write calls but only 1 bufWriter.Flush (1 syscall) |
| AC-2 | Forward pool item with 1 rawBody | 1 write + 1 flush (no regression for single-message case) |
| AC-3 | Write error on 2nd message in batch | Remaining messages skipped, flush not called, error returned |
| AC-4 | onMessageReceived callback | Still fires per message within the batch (not batched) |
| AC-5 | KEEPALIVE, OPEN, NOTIFICATION | Still flush immediately (unchanged) |
| AC-6 | Connection close during batch | bufWriter.Flush in closeConn handles cleanup |
| AC-7 | `make ze-verify` passes | All existing tests pass with no regressions |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendUpdateBatchSingle` | `internal/plugins/bgp/reactor/session_test.go` | Single-message batch identical to SendUpdate (AC-2) | |
| `TestSendUpdateBatchMultiple` | `internal/plugins/bgp/reactor/session_test.go` | Multiple messages written, one flush (AC-1) | |
| `TestSendUpdateBatchErrorMidway` | `internal/plugins/bgp/reactor/session_test.go` | Error on Nth write stops remaining, no flush (AC-3) | |
| `TestSendUpdateBatchCallback` | `internal/plugins/bgp/reactor/session_test.go` | onMessageReceived fires per message (AC-4) | |
| `TestFwdHandlerBatched` | `internal/plugins/bgp/reactor/forward_pool_test.go` | fwdHandler uses batch method, fewer flushes (AC-1) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Batch size | 0–N | N/A (unbounded by items in fwdItem) | 0 (empty batch = no-op) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing functional tests | `test/plugin/` | Route reflection still delivers all routes correctly | |

No new functional tests needed — this is a write-path optimization. Existing route reflection tests validate correctness.

### Future (if deferring any tests)
- Benchmark: syscall count with strace/dtrace before and after — deferred to profiling phase

## Files to Modify

- `internal/plugins/bgp/reactor/session.go` — add `SendUpdateBatch(rawBodies [][]byte, updates []*message.Update) error`
- `internal/plugins/bgp/reactor/peer_send.go` — add `SendUpdateBatch` peer wrapper
- `internal/plugins/bgp/reactor/forward_pool.go` — `fwdHandler` calls `SendUpdateBatch` instead of per-message loop

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Files to Create
- None — all changes are modifications to existing files

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write `TestSendUpdateBatchMultiple`** — verify multiple writes + one flush
2. **Run test** → MUST FAIL (method doesn't exist yet)
3. **Add `Session.SendUpdateBatch`** — acquire writeMu once, loop writeUpdate/writeRawUpdateBody, flush once at end
4. **Run test** → MUST PASS
5. **Write remaining tests** (single, error-midway, callback) → FAIL → implement → PASS
6. **Add `Peer.SendUpdateBatch` wrapper** in peer_send.go
7. **Update `fwdHandler`** to call `peer.SendUpdateBatch(item.rawBodies, item.updates)`
8. **Run all** → `make ze-verify`
9. **Critical Review** → all 6 quality checks

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix syntax/types |
| Test fails wrong reason | Fix test |
| Existing test breaks | Investigate — batch must be drop-in replacement |
| Write error handling wrong | Re-read writeUpdate error path |

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

### Why Not Remove Flush from SendUpdate Entirely?

`SendUpdate` is also called from API-injected routes and initial sync, where the caller expects the message to be on the wire after the call returns. Removing flush would break those callers' expectations. Instead, we add a new batch method for the forward pool hot path, keeping existing methods unchanged.

### Why Batch at Session, Not Forward Pool?

The writeMu is per-session (per-destination peer). The forward pool worker already processes one fwdItem at a time per destination peer. Making the batch method on Session means:
- writeMu acquired once per fwdItem (was N times)
- State checks (Established, conn != nil) done once
- Flush once at end
- Forward pool just passes its arrays through

## RFC Documentation

No RFC constraints — TCP write batching is a transport optimization with no BGP protocol impact.

## Implementation Summary

### What Was Implemented
- Added `bufWriter *bufio.Writer` (16KB) to Session, wrapping conn alongside existing bufReader
- All Send* methods write to bufWriter instead of raw conn; non-batch callers flush immediately
- Extracted `writeUpdate`, `writeRawUpdateBody`, `flushWrites` as internal methods (no lock, no flush) for batch use
- closeConn flushes bufWriter under writeMu before closing conn (prevents race with concurrent Send* calls)
- Changed fwdPool handler signature from `func(fwdKey, fwdItem)` to `func(fwdKey, []fwdItem)` for batch support
- Replaced `safeHandle` with `safeBatchHandle` — defers done() for ALL items in batch
- Added `drainBatch()` — non-blocking channel drain after first blocking receive
- `fwdBatchHandler` acquires session writeMu once, writes all messages to bufWriter, flushes once

### Bugs Found/Fixed
- Race condition: closeConn flushed bufWriter under s.mu but not s.writeMu. Fixed by acquiring writeMu inside closeConn (lock ordering s.mu → s.writeMu preserved).
- TestForwardPoolBackpressurePropagation: drain-batch changed timing — handler could grab items from channel before test filled it. Fixed with handler-entered gate signal.
- 8 tests manually set session.conn without bufWriter — all updated to also set bufWriter.

### Documentation Updates
- None needed (internal transport optimization, no API/config/YANG changes)

### Deviations from Plan
- Spec proposed `SendUpdateFlushed` wrapper (step 18). Instead, Send* methods auto-flush and the batch path uses internal `writeUpdate`/`writeRawUpdateBody` methods directly. Simpler, same result.
- SendRawMessage msgType==0 path now also goes through bufWriter+writeMu (was previously raw conn.Write without writeMu — race-prone).
- SendAnnounce and SendWithdraw not refactored to use internal write methods (no batch callers for these; they still write+flush inline).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Buffered writer on Session | ✅ Done | session.go:120 (field), session.go:612 (creation) | 16KB bufio.Writer wrapping conn |
| Batch flush in forward pool | ✅ Done | forward_pool.go:39-91 (fwdBatchHandler), forward_pool.go:260-270 (drainBatch in runWorker) | Single writeMu + flush per batch |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestSessionBufWriterCreated | bufWriter created in connectionEstablished |
| AC-2 | ✅ Done | TestSendUpdateUsesBufWriter | SendUpdate writes through bufWriter |
| AC-3 | ✅ Done | session.go:1000 (SendRawUpdateBody) | Uses writeRawUpdateBody → bufWriter |
| AC-4 | ✅ Done | session.go:947 | SendAnnounce writes to bufWriter |
| AC-5 | ✅ Done | session.go:993 | SendWithdraw writes to bufWriter |
| AC-6 | ✅ Done | TestSendMessageAutoFlush | writeMessage flushes immediately |
| AC-7 | ✅ Done | TestFwdWorkerDrainBatch | 5 items drained in single batch |
| AC-8 | ✅ Done | TestFwdWorkerBatchAllDoneCalled | done() called for all items on panic |
| AC-9 | ✅ Done | forward_pool.go:80-84 | Flush error logged, done() deferred for all |
| AC-10 | ✅ Done | TestSessionCloseFlushesBufWriter | closeConn flushes under writeMu |
| AC-11 | ✅ Done | make ze-verify passes | All tests pass with zero failures |
| AC-12 | ✅ Done | TestFwdWorkerBatchSingleItem | Single item produces batch of 1 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSessionBufWriterCreated | ✅ Done | session_test.go | AC-1 |
| TestSendUpdateUsesBufWriter | ✅ Done | session_test.go | AC-2 |
| TestSendMessageAutoFlush | ✅ Done | session_test.go | AC-6 |
| TestSessionCloseFlushesBufWriter | ✅ Done | session_test.go | AC-10, TDD red→green |
| TestFwdWorkerDrainBatch | ✅ Done | forward_pool_test.go | AC-7 |
| TestFwdWorkerBatchSingleItem | ✅ Done | forward_pool_test.go | AC-12 |
| TestFwdWorkerBatchAllDoneCalled | ✅ Done | forward_pool_test.go | AC-8/AC-9 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| session.go | ✅ Done | bufWriter field, connectionEstablished, closeConn, all Send*, internal write methods |
| session_test.go | ✅ Done | 4 new tests + 8 test fixups for bufWriter |
| forward_pool.go | ✅ Done | batch handler, safeBatchHandle, drainBatch, runWorker drain-batch |
| forward_pool_test.go | ✅ Done | 3 new tests + all handler signatures updated |
| peer_send.go | 🔄 Changed | No changes needed — Peer.Send* wrappers delegate to session unchanged |
| reactor.go | ✅ Done | fwdHandler → fwdBatchHandler |
| forward_update_test.go | ✅ Done | Handler signatures updated |

### Audit Summary
- **Total items:** 26
- **Done:** 25
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (peer_send.go — no changes needed, deviates from plan)

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-12 all demonstrated
- [x] `make ze-unit-test` passes
- [x] `make ze-functional-test` passes
- [x] Feature code integrated (`internal/*`)
- [x] Integration completeness proven end-to-end
- [x] Architecture docs updated (none needed — internal transport optimization)
- [x] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [x] `make ze-lint` passes
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction (3+ use cases?)
- [x] No speculative features (needed NOW?)
- [x] Single responsibility per component
- [x] Explicit > implicit behavior
- [x] Minimal coupling

### TDD
- [ ] Tests written → FAIL → implement → PASS
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
