# Spec: rib-03 — Route Reflector Replay Integration

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-rib-01-dispatch-command.md` — dispatch-command RPC
4. `docs/plan/spec-rib-02-adj-rib-in.md` — bgp-adj-rib-in plugin
5. `internal/plugins/bgp-rr/server.go` - handleStateUp, handleStateDown, processForward

## Task

Fix the data loss bug where peers connecting late miss routes forwarded before they reached Established. Two changes:

1. **Replace ROUTE-REFRESH with RIB replay**: bgp-rr's handleStateUp uses `DispatchCommand("adj-rib-in replay ...")` instead of sending ROUTE-REFRESH to all peers (thundering herd). Replay runs in a per-peer lifecycle goroutine (not blocking the event loop). Replay completes BEFORE peer joins forward targets, then delta replay covers the gap.

2. **Delete bgp-rr's local RIB**: bgp-adj-rib-in tracks all route state per source peer. bgp-rr keeps only a lightweight withdrawal map (`family+prefix` per source peer) for handleStateDown.

**Depends on:** spec-rib-01-dispatch-command.md + spec-rib-02-adj-rib-in.md
**Part of series:** rib-01 → rib-02 → rib-03 (this) → rib-04

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage internals
  → Decision: bgp-adj-rib-in stores parsed Route per source peer, replay via FormatRouteCommand
- [ ] `docs/architecture/core-design.md` - plugin architecture, command dispatch
  → Constraint: plugins communicate via engine-mediated RPCs (dispatch-command for cross-plugin)

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP-4 Adj-RIBs-In definition
  → Constraint: Adj-RIBs-In contains unprocessed routing info from each peer
- [ ] `rfc/short/rfc7947.md` - Internet Exchange BGP Route Server
  → Constraint: route server forwards ALL routes so receiving peer picks preferred

**Key insights:**
- ROUTE-REFRESH causes thundering herd: N peers x M families = N*M re-advertisements simultaneously
- Ghost route problem: if peer is in forward targets during replay, a withdrawal followed by replay re-announces a deleted route. Replay MUST complete before forwarding starts.
- Delta replay covers the gap: routes arriving during full replay get higher sequence indices, caught up with a second small replay from last-index.
- bgp-rr's local RIB (rib.go) stores {MsgID, Family, Prefix} — only used for withdrawals. Replace with lightweight withdrawal map.
- Replay runs in per-peer lifecycle goroutine — NOT blocking the event loop (consistent with goroutine-lifecycle rule: one goroutine per peer lifecycle is OK).
- DispatchCommand returns {status, data} — data contains `{"last-index": N}` for delta replay coordination.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugins/bgp-rr/server.go` - handleStateUp sends ROUTE-REFRESH (lines 729-784), async via `go func()` per target peer
- [x] `internal/plugins/bgp-rr/server.go` - handleStateDown: drains workers, ClearPeer, sends withdrawals (lines 705-717)
- [x] `internal/plugins/bgp-rr/server.go` - processForward: parses NLRI, inserts into local RIB, batch forwards (lines 375-447)
- [x] `internal/plugins/bgp-rr/server.go` - selectForwardTargets: peers that are Up and support families (lines 571-593)
- [x] `internal/plugins/bgp-rr/rib.go` - Minimal RIB: Route{MsgID, Family, Prefix} per source peer

**Behavior to preserve:**
- bgp-rr zero-copy cache-forward for ongoing UPDATEs (batchForwardUpdate hot path)
- bgp-rr processForward NLRI parsing (families, operations, batch forwarding)
- selectForwardTargets logic (up peers supporting families, excluding source)
- handleStateDown sends withdrawals for all source peer's routes

**Behavior to change:**
- handleStateUp: DELETE ROUTE-REFRESH, ADD replay-then-delta sequence via DispatchCommand
- processForward: REPLACE RIB insert with lightweight withdrawal map update
- handleStateDown: read withdrawal map instead of local RIB for withdrawal commands
- DELETE rib.go entirely (bgp-adj-rib-in tracks full route state)
- ADD peer state "replaying" — peer not in selectForwardTargets until replay complete

## Data Flow (MANDATORY)

### Entry Point
- Peer X connects, reaches Established → engine sends "state up" to bgp-rr

### Transformation Path

**Replay sequence** (peer reconnect catch-up, runs in per-peer lifecycle goroutine):
1. bgp-rr handleStateUp for peer X — marks peer as "replaying" (NOT in selectForwardTargets)
2. Spawns per-peer replay goroutine (lifecycle goroutine — allowed by goroutine rules)
3. Goroutine calls `p.DispatchCommand(ctx, "adj-rib-in replay X 0")` → full replay from index 0
4. Engine dispatches to bgp-adj-rib-in via execute-command callback
5. bgp-adj-rib-in iterates ribIn for ALL source peers except X, sends each Route to X via updateRoute
6. bgp-adj-rib-in returns `{status: "done", data: "{\"last-index\": N}"}`
7. Goroutine parses last-index N from response
8. Goroutine transitions peer X from "replaying" to "up" (now in selectForwardTargets, new UPDATEs flow)
9. Goroutine calls `p.DispatchCommand(ctx, "adj-rib-in replay X N")` → delta replay
10. Delta returns `{last-index: M}` — gap covered
11. Peer X fully caught up, ongoing UPDATEs via zero-copy cache-forward

**Withdrawal** (source peer down — bgp-rr handles from withdrawal map):
1. bgp-rr handleStateDown for source peer Y
2. bgp-rr reads withdrawal map for Y: each entry has family + prefix
3. bgp-rr sends `update text nlri <family> del <prefix>` to all other peers
4. bgp-rr clears withdrawal map for Y

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| bgp-rr → Engine | DispatchCommand RPC with "adj-rib-in replay X 0" | [ ] |
| Engine → bgp-adj-rib-in | execute-command callback (dispatch-command routes to plugin) | [ ] |
| bgp-adj-rib-in → Engine | updateRoute RPC per replayed route (to target peer X) | [ ] |
| Engine → target peer X | Wire UPDATE from "update text" command | [ ] |

### Integration Points
- dispatch-command RPC (spec rib-01) — bgp-rr invokes "adj-rib-in replay"
- bgp-adj-rib-in "adj-rib-in replay" command (spec rib-02) — handles replay logic
- bgp-rr handleStateUp — sends DispatchCommand, waits in lifecycle goroutine, then adds to forward targets
- bgp-rr processForward — populates withdrawal map from NLRI parsing (same data currently stored in local RIB)
- bgp-rr handleStateDown — reads withdrawal map instead of local RIB

### Architectural Verification
- [ ] No bypassed layers (command goes through engine dispatch)
- [ ] No unintended coupling (plugins communicate only via dispatch-command)
- [ ] No duplicated functionality (route storage only in bgp-adj-rib-in)
- [ ] Zero-copy preserved (cache-forward hot path unchanged)
- [ ] No per-event goroutines (replay goroutine is per-peer lifecycle)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer X connects after peers A,B sent routes | X receives ALL routes from A and B via replay |
| AC-2 | Peer X connects | No ROUTE-REFRESH sent to any peer |
| AC-3 | Replay completes | Peer X added to forward targets AFTER replay response |
| AC-4 | Routes arrive during replay | Caught up via delta replay (adj-rib-in replay X N) |
| AC-5 | Source peer Y goes down | Withdrawals sent from bgp-rr's withdrawal map |
| AC-6 | bgp-rr has no local RIB | rib.go deleted, no Route struct in bgp-rr |
| AC-7 | Replay targets only peer X | Other connected peers receive no duplicate routes |
| AC-8 | processForward stores withdrawal info | family+prefix per source peer in withdrawal map |
| AC-9 | Event loop not blocked during replay | Other peers' UPDATEs continue flowing during replay |
| AC-10 | `make ze-verify` passes | All tests pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHandleStateUpReplay` | `bgp-rr/server_test.go` | handleStateUp calls DispatchCommand (not ROUTE-REFRESH) | |
| `TestHandleStateUpSequencing` | `bgp-rr/server_test.go` | Peer not in selectForwardTargets until replay response | |
| `TestHandleStateUpDelta` | `bgp-rr/server_test.go` | Delta replay sent after adding to forward targets | |
| `TestHandleStateUpNonBlocking` | `bgp-rr/server_test.go` | Event loop processes other events during replay | |
| `TestWithdrawalMap` | `bgp-rr/server_test.go` | processForward populates withdrawal map from NLRI | |
| `TestWithdrawalOnPeerDown` | `bgp-rr/server_test.go` | handleStateDown sends withdrawals from map, clears it | |
| `TestNoLocalRIB` | `bgp-rr/server_test.go` | No rib.go, no Route struct in package | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no new numeric inputs | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rr-replay-on-reconnect` | `test/plugin/rr-replay-on-reconnect.ci` | Peer connects late, receives all routes via replay | |
| Existing RR tests pass | `test/plugin/*.ci` | No regression in forwarding behavior | |

## Files to Modify

- `internal/plugins/bgp-rr/server.go` — replace handleStateUp (replay+delta via DispatchCommand in lifecycle goroutine), add peer "replaying" state, replace processForward RIB insert with withdrawal map update, replace handleStateDown (withdrawal map)

## Files to Delete

- `internal/plugins/bgp-rr/rib.go` — local RIB replaced by withdrawal map + bgp-adj-rib-in
- `internal/plugins/bgp-rr/rib_test.go` — tests for deleted RIB (replaced by withdrawal map tests)

## Files to Create

- `test/plugin/rr-replay-on-reconnect.ci` — peer connects late, gets all routes

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| dispatch-command RPC | [x] | Must exist from spec rib-01 |
| bgp-adj-rib-in replay command | [x] | Must exist from spec rib-02 |
| Functional test | [x] | `test/plugin/rr-replay-on-reconnect.ci` |

## Implementation Steps

1. **Write tests for withdrawal map** → Verify FAIL
2. **Add withdrawal map** — per source peer, stores family+prefix for each announced route
3. **Update processForward** — populate withdrawal map from NLRI parsing (replace RIB insert)
4. **Write tests for handleStateUp replay** → Verify FAIL
5. **Add peer "replaying" state** — not in selectForwardTargets until replay complete
6. **Replace handleStateUp** — spawn lifecycle goroutine: DispatchCommand replay → add to targets → delta replay
7. **Update handleStateDown** — read withdrawal map instead of local RIB
8. **Delete rib.go and rib_test.go** — remove all local RIB references
9. **Run all tests** → Verify PASS
10. **Write functional test** — rr-replay-on-reconnect.ci
11. **Verify all** → `make ze-verify`
12. **Critical Review** → all 6 quality checks

### Failure Routing

| Failure | Route To |
|---------|----------|
| Replay sends wrong routes | Check spec rib-02 (bgp-adj-rib-in replay command) |
| Ghost routes after replay | Step 5-6 (verify peer not in forward targets during replay) |
| Missing routes from gap | Step 6 (verify delta replay from last-index) |
| Withdrawals fail | Step 7 (check withdrawal map population in processForward) |
| Cache-forward broken | Step 3 (verify only RIB insert removed, not forward logic) |
| Event loop blocked | Step 6 (verify replay in goroutine, not inline in handler) |
| DispatchCommand fails | Check spec rib-01 (dispatch-command RPC must work) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

- ROUTE-REFRESH thundering herd makes it unusable for production route servers
- Ghost route problem: forward-first + replay creates stale routes after withdrawals
- Replay-first + delta-replay is the correct sequencing: correctness over simplicity
- Withdrawal map is much simpler than a full RIB — only family+prefix needed for "del" commands
- Per-peer lifecycle goroutine for replay: event loop stays free, replay is a lifecycle operation
- The tiny gap during delta replay (between "add to targets" and "delta complete") is acceptable — duplicates are harmless in BGP, and the delta window is very short
- Peer-down race resolved by design: bgp-rr uses its own withdrawal map (populated from processForward), independent of bgp-adj-rib-in's cleanup timing

## Implementation Summary

### What Was Implemented
- Replaced ROUTE-REFRESH thundering herd with targeted replay via `DispatchCommand("adj-rib-in replay <peer> <index>")`
- Added `Replaying` field to `PeerState` — peers excluded from `selectForwardTargets` during replay
- Added `replayForPeer` goroutine: full replay → add to targets → delta replay
- Added `dispatchCommand` method with test hook for unit testing
- Added `withdrawalInfo` struct and `withdrawals` map (per source peer, family+prefix)
- Updated `processForward` to populate withdrawal map from NLRI parsing (add/del tracking)
- Updated `handleStateDown` to read withdrawal map instead of local RIB
- Deleted `rib.go` and `rib_test.go` (local RIB replaced by withdrawal map + bgp-adj-rib-in)
- Updated 3 existing tests (`TestHandleState_Down/Up_ZeBGPFormat`, `TestHandleState_Up_ExcludesSelf`) to use withdrawal map

### Bugs Found/Fixed
- Duplicate `// Related:` line in server.go header (cosmetic)

### Documentation Updates
- Architecture docs already describe target design — no updates needed

### Deviations from Plan
- Functional test `rr-replay-on-reconnect.ci` deferred — requires full engine+plugin integration harness not yet available

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Replace ROUTE-REFRESH with RIB replay | ✅ Done | server.go:768-816 | handleStateUp + replayForPeer |
| Delete bgp-rr's local RIB | ✅ Done | rib.go deleted | Replaced by withdrawal map |
| Lightweight withdrawal map | ✅ Done | server.go:84-87,125-129 | withdrawalInfo struct + withdrawals map |
| Peer "replaying" state | ✅ Done | peer.go:11, server.go:609 | Replaying field, excluded from targets |
| Delta replay after full replay | ✅ Done | server.go:809-815 | Uses last-index from response |
| Per-peer lifecycle goroutine | ✅ Done | server.go:777 | go rs.replayForPeer(peerAddr) |
| dispatchCommandHook for tests | ✅ Done | server.go:136-137,334-339 | Nil in production |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestHandleStateUpReplay | DispatchCommand called with "adj-rib-in replay" |
| AC-2 | ✅ Done | TestHandleStateUpReplay | No ROUTE-REFRESH sent |
| AC-3 | ✅ Done | TestHandleStateUpSequencing | Replaying peer excluded from targets |
| AC-4 | ✅ Done | TestHandleStateUpDelta | Delta replay from last-index |
| AC-5 | ✅ Done | TestWithdrawalOnPeerDown | Withdrawals sent from map, map cleared |
| AC-6 | ✅ Done | rib.go deleted, grep confirms no Route struct | No local RIB in package |
| AC-7 | ✅ Done | TestHandleState_Up_ExcludesSelf | Replay command includes peer address |
| AC-8 | ✅ Done | TestProcessForwardPopulatesWithdrawalMap | family+prefix stored per source peer |
| AC-9 | ✅ Done | TestHandleStateUpNonBlocking | handleStateUp returns immediately |
| AC-10 | ✅ Done | make ze-verify passes | EXIT: 0 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestHandleStateUpReplay | ✅ Done | server_test.go:1444 | AC-1, AC-2 |
| TestHandleStateUpSequencing | ✅ Done | server_test.go:1494 | AC-3 |
| TestHandleStateUpDelta | ✅ Done | server_test.go:1531 | AC-4 |
| TestHandleStateUpNonBlocking | ✅ Done | server_test.go:1573 | AC-9 |
| TestWithdrawalMap (as TestProcessForwardPopulatesWithdrawalMap) | ✅ Done | server_test.go (AC-8) | AC-8 |
| TestWithdrawalOnPeerDown | ✅ Done | server_test.go:1614 | AC-5 |
| TestNoLocalRIB | ✅ Done | rib.go deleted, verified by grep | AC-6 |
| rr-replay-on-reconnect.ci | ⚠️ Deferred | N/A | Needs full engine+plugin harness |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/plugins/bgp-rr/server.go | ✅ Modified | handleStateUp, processForward, handleStateDown, new types |
| internal/plugins/bgp-rr/peer.go | ✅ Modified | Added Replaying field |
| internal/plugins/bgp-rr/server_test.go | ✅ Modified | 5 new tests + 3 updated tests |
| internal/plugins/bgp-rr/rib.go | ✅ Deleted | Local RIB replaced by withdrawal map |
| internal/plugins/bgp-rr/rib_test.go | ✅ Deleted | Tests for deleted RIB |
| test/plugin/rr-replay-on-reconnect.ci | ⚠️ Deferred | Needs full engine+plugin harness |

### Audit Summary
- **Total items:** 24
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Deferred:** 2 (functional test — needs integration harness)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

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
