# Spec: seqmap — Sequence-Indexed Map Library

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/seqmap/seqmap.go` — the library implementation
4. `internal/plugins/bgp-adj-rib-in/rib.go` — integration target

## Task

Create a generic `internal/seqmap/` library that provides a key-value map with
efficient range queries by monotonic sequence number. Integrate into adj-rib-in
so that delta replays are O(log N + K) instead of O(N), where N = total routes
and K = matching entries.

**Motivation:** `buildReplayCommands` iterates ALL routes (O(N)) checking
`rt.SeqIndex < fromIndex`. With 1M routes, each delta replay scans the full
map even when only a handful of new routes match. The convergent delta replay
loop (commit fd12d895) amplifies this — multiple delta iterations per reconnect.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` — adj-rib-in storage model
  → Constraint: ribIn is `map[string]map[string]*RawRoute` — peer → key → route
  → Constraint: seqCounter is global across all peers, monotonically increasing

### RFC Summaries (MUST for protocol work)
- N/A — this is an internal data structure change, no protocol impact.

**Key insights:**
- SeqIndex is assigned globally, not per-peer. Delta replay uses a single fromIndex across all peers.
- Routes are added (with seq), overwritten (same key, new seq), deleted (withdrawal), or bulk-cleared (peer down).
- The seqmap needs per-peer instances sharing a global seq counter managed by the caller.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp-adj-rib-in/rib.go` (~382L) — AdjRIBInManager, RawRoute with SeqIndex field, handleReceived stores routes in `map[string]*RawRoute` per peer, buildReplayCommands does full O(N) scan
  → Constraint: seqCounter++ happens inside `r.mu.Lock()` in handleReceived
  → Constraint: handleState deletes entire peer map on peer-down
- [ ] `internal/plugins/bgp-adj-rib-in/rib_commands.go` (~120L) — statusJSON uses `len(routes)`, showJSON uses `for key, rt := range routes` reading `rt.SeqIndex`, replayCommand calls buildReplayCommands
  → Constraint: showJSON returns seq-index in JSON output
- [ ] `internal/plugins/bgp-adj-rib-in/rib_test.go` (~500L) — 9 tests directly assign `r.ribIn[peer] = map[string]*RawRoute{...}` with SeqIndex field
  → Constraint: all existing test assertions must continue to pass

**Behavior to preserve:**
- replayCommand response format: `{"last-index":N,"replayed":M}`
- statusJSON output format (peer route counts)
- showJSON output format (includes seq-index per route)
- buildReplayCommands excludes target peer's own routes
- Global monotonic seqCounter across all peers

**Behavior to change:**
- Internal storage: inner `map[string]*RawRoute` → `*seqmap.Map[string, *RawRoute]`
- `buildReplayCommands`: full scan → `Since(fromIndex, fn)` per peer
- `RawRoute.SeqIndex` removed — seqmap tracks sequence numbers
- showJSON gets seq from `Range` callback instead of `rt.SeqIndex`

## Data Flow (MANDATORY)

### Entry Point
- UPDATEs arrive as JSON events via DirectBridge → `handleReceived` stores in ribIn
- Replay commands arrive via Socket B → `replayCommand` → `buildReplayCommands`

### Transformation Path
1. Engine delivers UPDATE event to adj-rib-in via DirectBridge
2. `handleReceived` acquires `r.mu.Lock()`, increments `seqCounter`, calls `ribIn[peer].Put(key, seq, route)`
3. seqmap appends entry to internal log (sorted by seq, append-only)
4. On replay: `buildReplayCommands` acquires `r.mu.RLock()`, calls `ribIn[peer].Since(fromIndex, fn)` per peer
5. seqmap binary-searches log, iterates live entries with seq >= fromIndex

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| adj-rib-in ↔ seqmap | Direct function calls (same process) | [ ] |
| bgp-rs → adj-rib-in | execute-command RPC via Socket B | [ ] |

### Integration Points
- `handleReceived` → `seqmap.Put()` (add routes)
- `handleReceived` → `seqmap.Delete()` (withdraw routes)
- `handleState` → `delete(r.ribIn, peer)` (peer down — drops entire seqmap)
- `buildReplayCommands` → `seqmap.Since()` (delta replay)
- `statusJSON` → `seqmap.Len()` (route counts)
- `showJSON` → `seqmap.Range()` (display all routes)

### Architectural Verification
- [ ] No bypassed layers — seqmap is a pure data structure, no protocol knowledge
- [ ] No unintended coupling — seqmap has zero imports beyond `sort`
- [ ] No duplicated functionality — no existing sequence-indexed container in codebase
- [ ] Zero-copy preserved — seqmap stores pointers, no copies

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `handleReceived` (event) | → | `seqmap.Put` stores route | `TestStoreReceivedRoute` (existing, adapted) |
| `handleReceived` (withdrawal) | → | `seqmap.Delete` removes route | `TestRemoveWithdrawnRoute` (existing, adapted) |
| `buildReplayCommands(peer, fromIndex)` | → | `seqmap.Since(fromIndex, fn)` filters efficiently | `TestReplayFromIndex` (existing, adapted) |
| `statusJSON` | → | `seqmap.Len()` for counts | `TestHandleCommand_Status` (existing, adapted) |
| `showJSON` | → | `seqmap.Range()` for display | `TestHandleCommand_Show` (existing, adapted) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `seqmap.Put(key, seq, val)` then `Get(key)` | Returns val and true |
| AC-2 | `seqmap.Put(key, seq1, v1)` then `Put(key, seq2, v2)` | Get returns v2, Len stays 1 |
| AC-3 | `seqmap.Delete(key)` | Get returns false, Len decreases |
| AC-4 | `seqmap.Since(fromSeq, fn)` with mixed seqs | Only entries with seq >= fromSeq visited, in order |
| AC-5 | `Since` after overwrites and deletes | Dead entries skipped |
| AC-6 | Many overwrites (>256 entries, >50% dead) | Auto-compaction fires, Since still correct |
| AC-7 | adj-rib-in `buildReplayCommands` with seqmap | Same results as before (routes, maxSeq), faster for delta |
| AC-8 | adj-rib-in `showJSON` | seq-index still present in output (from Range callback) |
| AC-9 | All existing adj-rib-in tests pass | No behavioral regression |

## 🧪 TDD Test Plan

### Unit Tests — seqmap
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPutAndGet` | `internal/seqmap/seqmap_test.go` | Basic round-trip | ✅ |
| `TestGetMissing` | same | Missing key returns false | ✅ |
| `TestPutOverwrite` | same | Update replaces value and seq, Len stable | ✅ |
| `TestDelete` | same | Delete removes key | ✅ |
| `TestDeleteNonExistent` | same | Missing key returns false | ✅ |
| `TestLen` | same | Accurate through mixed operations | ✅ |
| `TestClear` | same | Resets all state | ✅ |
| `TestSinceAll` | same | Since(0) returns all in seq order | ✅ |
| `TestSincePartial` | same | Only entries with seq >= fromSeq | ✅ |
| `TestSinceSkipsDead` | same | Overwritten/deleted entries skipped | ✅ |
| `TestSinceEarlyStop` | same | fn returning false stops iteration | ✅ |
| `TestSinceOrder` | same | Ascending seq order | ✅ |
| `TestSinceEmpty` | same | No panic on empty map | ✅ |
| `TestSinceBeyondMax` | same | Beyond max returns nothing | ✅ |
| `TestRange` | same | All live entries visited | ✅ |
| `TestRangeEarlyStop` | same | fn returning false stops iteration | ✅ |
| `TestRangeIncludesSeq` | same | Returns updated seq after overwrite | ✅ |
| `TestCompaction` | same | Auto-compaction cleans dead entries | ✅ |
| `TestCompactionPreservesOrder` | same | Since correct after compaction | ✅ |
| `TestSinceAfterDelete` | same | Deleted entries invisible to Since | ✅ |

### Unit Tests — adj-rib-in (existing, adapted)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStoreReceivedRoute` | `internal/plugins/bgp-adj-rib-in/rib_test.go` | Route stored via seqmap.Put | ✅ |
| `TestStoreAllFamilies` | same | VPN/EVPN routes stored | ✅ |
| `TestRemoveWithdrawnRoute` | same | Withdrawal via seqmap.Delete | ✅ |
| `TestReplayAllSources` | same | Replay excludes target peer | ✅ |
| `TestReplayFromIndex` | same | Delta replay via seqmap.Since | ✅ |
| `TestReplayReturnsLastIndex` | same | maxSeq from Since callback | ✅ |
| `TestSequenceIndexMonotonic` | same | Monotonic seq via Range callback | ✅ |
| `TestClearPeerOnDown` | same | Peer down drops seqmap | ✅ |
| `TestHandleCommand_Status` | same | Len() for counts | ✅ |
| `TestHandleCommand_Show` | same | Range() for display | ✅ |
| `TestMultipleNLRIsPerUpdate` | same | Multiple NLRIs stored individually | ✅ |
| `TestComplexFamilyMultiNLRI` | same | Complex family raw blob | ✅ |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| fromSeq | 0–max uint64 | max uint64 | N/A (0 = all) | N/A (uint64) |
| compactMinLog | 256 | 256 entries | N/A | N/A |

### Functional Tests
- N/A — seqmap is an internal data structure with no user-facing surface. Existing adj-rib-in functional tests cover the replay path end-to-end.

## Files to Modify
- `internal/plugins/bgp-adj-rib-in/rib.go` — change ribIn type, update handleReceived/buildReplayCommands, remove RawRoute.SeqIndex
- `internal/plugins/bgp-adj-rib-in/rib_commands.go` — statusJSON uses Len(), showJSON uses Range()
- `internal/plugins/bgp-adj-rib-in/rib_test.go` — adapt 9 direct map assignments to seqmap

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A — internal data structure |
| RPC count in docs | No | N/A — no new RPCs |
| CLI commands | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Functional test | No | Existing replay tests cover path |

## Files to Create
- `internal/seqmap/seqmap.go` — generic Map[K,V] with Put/Get/Delete/Since/Range/Clear
- `internal/seqmap/seqmap_test.go` — 20 TDD tests

## Implementation Steps

1. **Write seqmap tests** → Review: edge cases, compaction, dead entry handling
2. **Run tests** → Verify FAIL (package doesn't exist)
3. **Implement seqmap.go** → Minimal: map + append-only log + binary search
4. **Run tests** → Verify PASS
5. **Update rib_test.go** → Adapt map literals to seqmap.New + Put
6. **Run tests** → Verify FAIL (rib.go still uses raw maps)
7. **Update rib.go** → Change type, handleReceived, buildReplayCommands
8. **Update rib_commands.go** → statusJSON, showJSON
9. **Run tests** → Verify PASS
10. **Verify all** → `go test -race ./internal/seqmap/... ./internal/plugins/bgp-adj-rib-in/... ./internal/plugins/bgp-rs/...`

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 7 (fix type mismatches) |
| Test fails wrong reason | Step 5 (fix test adaptation) |
| Lint failure | Fix inline |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- seqmap auto-compaction threshold (dead > len/2 && len > 256) balances memory vs CPU
- Removing SeqIndex from RawRoute eliminates data duplication — seq lives in seqmap only
- Per-peer seqmaps with global seq counter enables cross-peer range queries with a single fromIndex

## Implementation Summary

### What Was Implemented
- Created `internal/seqmap/seqmap.go` (~130L) — generic `Map[K,V]` with append-only log, binary search `Since`, auto-compaction
- Created `internal/seqmap/seqmap_test.go` (~300L) — 20 TDD tests covering all operations, compaction, and edge cases
- Changed `rib.go`: `ribIn` type from `map[string]map[string]*RawRoute` to `map[string]*seqmap.Map[string, *RawRoute]`, removed `SeqIndex` from `RawRoute`, updated `handleReceived` to use `Put`/`Delete`, updated `buildReplayCommands` to use `Since`
- Changed `rib_commands.go`: `statusJSON` uses `Len()`, `showJSON` uses `Range()` with seq from callback
- Changed `rib_test.go`: adapted all 12 tests from direct map assignment to seqmap API

### Bugs Found/Fixed
- None

### Documentation Updates
- None — seqmap is an internal data structure with no user-facing surface

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Generic seqmap library | ✅ Done | `internal/seqmap/seqmap.go` | Map[K,V] with Put/Get/Delete/Since/Range/Clear/Len |
| O(log N + K) delta replay | ✅ Done | `seqmap.go:82` Since uses sort.Search | Binary search on append-only log |
| Auto-compaction | ✅ Done | `seqmap.go:107` maybeCompact | dead > len/2 && len > 256 |
| Integrate into adj-rib-in | ✅ Done | `rib.go:71,178-218,258-279` | ribIn type changed, handleReceived/buildReplayCommands updated |
| Remove RawRoute.SeqIndex | ✅ Done | `rib.go:57-62` | SeqIndex field removed, seq tracked by seqmap |
| All existing tests pass | ✅ Done | 14 adj-rib-in tests, 20 seqmap tests | Verified with race detector |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestPutAndGet` | Put then Get returns value |
| AC-2 | ✅ Done | `TestPutOverwrite` | Overwrite replaces value, Len stays 1 |
| AC-3 | ✅ Done | `TestDelete` | Delete removes key, Len decreases |
| AC-4 | ✅ Done | `TestSincePartial`, `TestSinceOrder` | Only seq >= fromSeq, in order |
| AC-5 | ✅ Done | `TestSinceSkipsDead`, `TestSinceAfterDelete` | Dead entries invisible |
| AC-6 | ✅ Done | `TestCompaction`, `TestCompactionPreservesOrder` | Auto-compaction fires and preserves correctness |
| AC-7 | ✅ Done | `TestReplayFromIndex`, `TestReplayAllSources` | Same replay results with seqmap |
| AC-8 | ✅ Done | `TestHandleCommand_Show` | seq-index in JSON from Range callback |
| AC-9 | ✅ Done | All 14 adj-rib-in tests pass | No behavioral regression |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPutAndGet` | ✅ Done | `seqmap_test.go` | |
| `TestGetMissing` | ✅ Done | `seqmap_test.go` | |
| `TestPutOverwrite` | ✅ Done | `seqmap_test.go` | |
| `TestDelete` | ✅ Done | `seqmap_test.go` | |
| `TestDeleteNonExistent` | ✅ Done | `seqmap_test.go` | |
| `TestLen` | ✅ Done | `seqmap_test.go` | |
| `TestClear` | ✅ Done | `seqmap_test.go` | |
| `TestSinceAll` | ✅ Done | `seqmap_test.go` | |
| `TestSincePartial` | ✅ Done | `seqmap_test.go` | |
| `TestSinceSkipsDead` | ✅ Done | `seqmap_test.go` | |
| `TestSinceEarlyStop` | ✅ Done | `seqmap_test.go` | |
| `TestSinceOrder` | ✅ Done | `seqmap_test.go` | |
| `TestSinceEmpty` | ✅ Done | `seqmap_test.go` | |
| `TestSinceBeyondMax` | ✅ Done | `seqmap_test.go` | |
| `TestRange` | ✅ Done | `seqmap_test.go` | |
| `TestRangeEarlyStop` | ✅ Done | `seqmap_test.go` | |
| `TestRangeIncludesSeq` | ✅ Done | `seqmap_test.go` | |
| `TestCompaction` | ✅ Done | `seqmap_test.go` | |
| `TestCompactionPreservesOrder` | ✅ Done | `seqmap_test.go` | |
| `TestSinceAfterDelete` | ✅ Done | `seqmap_test.go` | |
| `TestStoreReceivedRoute` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestStoreAllFamilies` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestRemoveWithdrawnRoute` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestReplayAllSources` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestReplayFromIndex` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestReplayReturnsLastIndex` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestSequenceIndexMonotonic` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestClearPeerOnDown` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestHandleCommand_Status` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestHandleCommand_Show` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestMultipleNLRIsPerUpdate` | ✅ Done | `rib_test.go` | Adapted to seqmap |
| `TestComplexFamilyMultiNLRI` | ✅ Done | `rib_test.go` | Adapted to seqmap |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/seqmap/seqmap.go` | ✅ Created | ~130 lines, generic Map with Since/Range |
| `internal/seqmap/seqmap_test.go` | ✅ Created | ~300 lines, 20 tests |
| `internal/plugins/bgp-adj-rib-in/rib.go` | ✅ Modified | Type change, SeqIndex removal, handler updates |
| `internal/plugins/bgp-adj-rib-in/rib_commands.go` | ✅ Modified | Len(), Range() |
| `internal/plugins/bgp-adj-rib-in/rib_test.go` | ✅ Modified | All tests adapted to seqmap API |

### Audit Summary
- **Total items:** 43 (6 requirements + 9 AC + 32 tests + 5 files — some overlap with wiring tests counted in both)
- **Done:** 43
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec
