# Spec: session-write-mutex

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/bgp/reactor/session.go` - Session struct and all send methods
4. `internal/plugins/bgp/reactor/peer.go` - sendInitialRoutes, ShouldQueue

## Task

Fix data race on `Session.writeBuf` that causes the `fast.ci` functional test to flake. `Session.writeBuf` (a `*wire.SessionBuffer`) is shared between all send methods but has **no write synchronization**. Multiple goroutines call send methods concurrently (keepalive timer, sendInitialRoutes, plugin RPC handlers, forward pool workers), causing the `Reset → WriteTo → conn.Write` sequence to race on the shared buffer.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - Session/Peer architecture
  → Constraint: zero-allocation wire writing via SessionBuffer

### RFC Summaries (MUST for protocol work)
- Not applicable (internal concurrency fix, no wire format changes)

**Key insights:**
- `Session.writeBuf` is used by 6 methods, none hold any mutex during the write sequence
- The keepalive timer fires via `time.AfterFunc` in an independent goroutine
- Comments on SendUpdate/SendAnnounce/SendWithdraw say "externally synchronized" but no caller provides that synchronization

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugins/bgp/reactor/session.go` - Session struct (line 114), writeBuf field (line 139), all 6 send methods, negotiateWith (line 1480)
- [x] `internal/plugins/bgp/reactor/peer.go` - sendInitialRoutes (line 1560-1726), ShouldQueue (line 842), SendUpdate wrapper (line 1273)
- [x] `internal/plugins/bgp/reactor/session_test.go` - setupEstablishedSessionEBGP helper (line 1393), TestSendRawUpdateBody pattern (line 1700)
- [x] `internal/plugins/bgp/wire/writer.go` - SessionBuffer type (not goroutine-safe)

**Behavior to preserve:**
- All send method signatures unchanged
- Zero-allocation write pattern (Reset → WriteTo → conn.Write)
- Lock ordering: `s.mu` before `s.writeMu` (never reverse)
- onMessageReceived callback continues to fire after each send

**Behavior to change:**
- Add `writeMu sync.Mutex` to Session to serialize all writeBuf access
- Remove misleading "externally synchronized" comments

## Data Flow (MANDATORY)

### Entry Point
- Multiple goroutines call `peer.SendUpdate()` / `peer.SendAnnounce()` / etc.
- These delegate to `session.SendUpdate()` / `session.SendAnnounce()` / etc.

### Transformation Path
1. Caller invokes Peer send method (acquires `p.mu.RLock` to get session pointer)
2. Session method acquires `s.mu.RLock` to check conn/state
3. Session method uses `s.writeBuf`: Reset → WriteTo → conn.Write
4. Callback fires with buffer slice reference

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer → Session | `p.session.SendUpdate()` | [x] |
| Session → TCP | `conn.Write(s.writeBuf.Buffer()[:n])` | [x] |

### Integration Points
- `sendInitialRoutes` goroutine calls `p.SendUpdate(BuildEOR)` at peer.go:1723
- `AnnounceNLRIBatch` calls `peer.sendUpdateWithSplit` at reactor.go:2038
- Keepalive timer calls `s.sendKeepalive` via `time.AfterFunc` callback
- Forward pool workers call `peer.SendRawUpdateBody` at forward_pool.go:38

### Architectural Verification
- [x] No bypassed layers — mutex added inside Session, callers unchanged
- [x] No unintended coupling — writeMu is internal to Session
- [x] No duplicated functionality — single mutex for all write paths
- [x] Zero-copy preserved — writeBuf pattern unchanged, just serialized

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 10 goroutines call SendRawUpdateBody concurrently on same session | No data race detected by `-race` flag |
| AC-2 | sendInitialRoutes sends EOR while AnnounceNLRIBatch sends UPDATE | Messages serialized, both reach peer |
| AC-3 | Keepalive timer fires during SendUpdate | No writeBuf corruption |
| AC-4 | All existing tests | Pass unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendUpdateConcurrentNoRace` | `internal/plugins/bgp/reactor/session_test.go` | AC-1: concurrent SendRawUpdateBody with `-race` | [ ] |

### Boundary Tests (MANDATORY for numeric inputs)
No numeric inputs — concurrency fix only.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fast.ci` | `test/plugin/fast.ci` | Existing test that flakes — should become reliable | [ ] |

### Future
- AC-2 and AC-3 are proven by the `-race` flag on the concurrent test + all existing tests passing. Dedicated tests for those specific race windows would require precise goroutine scheduling, which is fragile.

## Files to Modify
- `internal/plugins/bgp/reactor/session.go` - Add writeMu field + lock in 6 send methods + negotiateWith + update comments
- `internal/plugins/bgp/reactor/session_test.go` - Add TestSendUpdateConcurrentNoRace

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| CLI commands/flags | No | N/A |
| Plugin SDK docs | No | N/A |
| Architecture docs | No | Internal concurrency fix |
| Functional test for new RPC/API | No | No new API |

## Files to Create
- None

## Implementation Steps

1. **Write unit test** `TestSendUpdateConcurrentNoRace` → Review: exercises concurrent path?
2. **Run tests** → Tests FAIL (race detected on writeBuf). Fail for RIGHT reason?
3. **Implement** → Add `writeMu` field, lock in all 6 methods + negotiateWith, update comments
4. **Run tests** → Tests PASS. `make ze-unit-test` (includes `-race`), `make ze-lint`
5. **Run full verify** → `make ze-verify`

### Failure Routing

| Failure | Route To |
|---------|----------|
| Race still detected | Step 3 — missed a send path |
| Deadlock | Step 3 — lock ordering violated |
| Lint failure | Step 3 — fix inline |
| Existing test fails | Step 3 — check lock scope |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation

No RFC documentation needed — internal concurrency fix.

## Implementation Summary

### What Was Implemented
- Added `writeMu sync.Mutex` field to Session struct to serialize all writeBuf access
- Locked writeMu in 6 methods: writeMessage, SendUpdate, SendAnnounce, SendWithdraw, SendRawUpdateBody, SendRawMessage (msgType != 0 path only)
- Locked writeMu in negotiateWith around writeBuf.Resize()
- Updated comments: replaced "externally synchronized" with "serialized by writeMu"
- Added TestSendUpdateConcurrentNoRace: 10 goroutines x 50 sends with -race

### Bugs Found/Fixed
- Data race on Session.writeBuf between concurrent send methods (the target bug)

### Documentation Updates
- None needed — internal concurrency fix

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Add writeMu sync.Mutex to Session | ✅ Done | session.go:141 | Between mu and writeBuf |
| Lock in writeMessage | ✅ Done | session.go:1634 | Covers sendOpen, sendKeepalive, sendNotification |
| Lock in SendUpdate | ✅ Done | session.go:1709 | After state check, before writeBuf access |
| Lock in SendAnnounce | ✅ Done | session.go:1757 | Same pattern |
| Lock in SendWithdraw | ✅ Done | session.go:1805 | Same pattern |
| Lock in SendRawUpdateBody | ✅ Done | session.go:1843 | Same pattern |
| Lock in SendRawMessage | ✅ Done | session.go:1888 | Only msgType != 0 path |
| Lock in negotiateWith for Resize | ✅ Done | session.go:1512 | Lock/unlock around Resize only |
| Remove "externally synchronized" comments | ✅ Done | session.go:1700,1746,1793 | Replaced with "serialized by writeMu" |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Pass | TestSendUpdateConcurrentNoRace (10 goroutines, -race) | Race detected before fix, clean after |
| AC-2 | ✅ Pass | writeMu serializes all send paths including writeMessage (used by EOR) and SendUpdate (used by AnnounceNLRIBatch) | Proven by -race on full suite |
| AC-3 | ✅ Pass | writeMessage (used by keepalive timer via sendKeepalive) and SendUpdate both acquire writeMu | Proven by -race on full suite |
| AC-4 | ✅ Pass | make ze-verify passes (lint + unit + functional) | All 247 functional + unit tests pass |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSendUpdateConcurrentNoRace | ✅ Done | session_test.go:2527 | Failed before fix, passes after |
| fast.ci | ✅ Pass | test/plugin/fast.ci | 55/55 plugin tests pass |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/plugins/bgp/reactor/session.go | ✅ Modified | writeMu field + 7 lock sites |
| internal/plugins/bgp/reactor/session_test.go | ✅ Modified | Added TestSendUpdateConcurrentNoRace |

### Audit Summary
- **Total items:** 14
- **Done:** 14
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Critical Review

| Check | Question | Pass? |
|-------|----------|-------|
| Correctness | writeMu covers all 6 writeBuf methods + Resize. -race detects no races. | [x] |
| Simplicity | Single mutex, no new types, no abstraction. Minimal change. | [x] |
| Consistency | Same lock-after-state-check pattern in all send methods. | [x] |
| Completeness | No TODOs, no FIXMEs. All send paths covered. | [x] |
| Quality | No debug statements. Comments updated. | [x] |
| Tests | TestSendUpdateConcurrentNoRace covers the race. All existing tests pass. | [x] |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-4 all demonstrated
- [x] `make ze-unit-test` passes
- [x] `make ze-functional-test` passes
- [x] Feature code integrated (`internal/*`)
- [x] Integration completeness proven end-to-end
- [x] Critical Review passes (all 6 checks in `rules/quality.md`)

### Quality Gates (SHOULD pass)
- [x] `make ze-lint` passes
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed — no mistakes to escalate

### Design
- [x] No premature abstraction — single mutex, no new types
- [x] No speculative features — fixes observed race only
- [x] Single responsibility — writeMu serializes writes only
- [x] Explicit > implicit — mutex is visible in struct
- [x] Minimal coupling — internal to Session

### TDD
- [x] Tests written → FAIL → implement → PASS
- [x] Boundary tests — N/A (no numeric inputs)
- [x] Functional tests — existing fast.ci passes

### Completion (BLOCKING — before ANY commit)
- [x] Critical Review passes — all 6 checks documented
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] Spec included in commit
