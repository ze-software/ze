# Spec: fwd-auto-sizing

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-31 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/forward-congestion-pool.md` - pool design, burst fractions, two-tier sizing
4. `internal/component/bgp/reactor/forward_pool.go` - fwdOverflowPool, TryDispatch, DispatchOverflow
5. `internal/component/bgp/reactor/bufmux.go` - BufMux block-backed pattern to reuse
6. `internal/component/bgp/reactor/forward_pool_weight.go` - peerBufferDemand, burstFraction
7. `internal/component/bgp/reactor/forward_pool_weight_tracker.go` - weightTracker, onBudgetChanged

## Task

Replace the static `fwdOverflowPool` (`chan struct{}` with 100K fixed capacity) with a
block-backed BufMux that auto-sizes from peer prefix maximums. The current overflow pool
is disconnected from the `weightTracker` demand calculations: `calculatePoolBudget` computes
correct demand-based budgets but only feeds wire buffer (bufmux) sizing, not the overflow
token pool. This causes immediate "forward peer congested" warnings in ze-chaos because the
overflow capacity doesn't match the actual topology.

The new overflow pool uses 4K base granularity (one BGP UPDATE per slot). Peers that
negotiated Extended Message (RFC 8654) consume 16 consecutive slots (64K) from the same
block. Blocks grow on demand, collapse when fully drained (memory returned to OS), and
allocate lowest-block-first -- the same pattern as the existing read-path BufMux.

The pool maximum is set by a restart-burst formula: the largest peer's full-table demand
multiplied by a capped fan-out factor (routes forwarded to other peers), plus 10% of
other peers' steady-state demand. The formula is isolated as a pure function so a future
YANG setting can replace it without restructuring.

### Sizing Formula

Isolated as a pure function. Inputs: per-peer prefix maximums. Output: total overflow slot count.

| Step | Calculation | Purpose |
|------|-------------|---------|
| 1 | `largest` = max peer's `peerBufferDemand(prefixMax, preEOR=true)` | Full restart burst of the biggest peer |
| 2 | `fanOut` = min(N-1, 2 * sqrt(N)), floor 1 | Capped fan-out: how many destinations receive forwarded routes |
| 3 | `restartBurst` = `largest` * `fanOut` | Total overflow from one peer restart, distributed across destinations |
| 4 | `steadyContrib` = sum of other peers' `peerBufferDemand(prefixMax, false)` * 0.1 | Small headroom for concurrent steady-state forwarding |
| 5 | `total` = `restartBurst` + `steadyContrib` | Combined budget |
| 6 | Return max(`total`, 64) | Floor for small topologies |

### Example Budgets

| Topology | Largest peer | Fan-out | Restart burst | Steady | Total slots | Memory (max) |
|----------|-------------|---------|---------------|--------|-------------|-------------|
| Chaos default (4 peers, 10K pfx max) | 500 | 3 | 1,500 | 45 | 1,545 | 6 MB |
| Small IXP (50 peers, 500K largest) | 25,000 | 14 | 350,000 | 1,650 | 351,650 | 1.4 GB |
| Medium IXP (200 peers, 1M largest) | 50,000 | 28 | 1,400,000 | 14,850 | 1,414,850 | 5.6 GB |
| Large IXP (1000 peers, 1M largest) | 50,000 | 63 | 3,150,000 | 74,850 | 3,224,850 | 12.6 GB |

Memory column is the maximum pool capacity. Blocks are allocated on demand and collapsed when
drained, so actual memory is much lower in steady state. These numbers are comparable to the
pre-EOR peaks in the architecture doc's IXP memory profiles.

### Multi-Slot Allocation for Extended Message

Each overflow slot is 4K (one standard BGP UPDATE). Extended Message peers (RFC 8654) consume
16 consecutive 4K slots from the same block (ceil(65535 / 4096) = 16). Block size must be a
multiple of 16 so 64K allocations always fit within one block.

### Buffer Ownership

Each overflow slot is 1:1 with one `fwdItem` in one destination's overflow queue. No sharing
across destinations because per-destination modifications (AS-PATH prepending, attribute
rewriting) may be needed. The wire data buffer itself (from the read-path BufMux) is
reference-counted via `RecentUpdateCache.retainCount` -- that mechanism is unchanged.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/forward-congestion-pool.md` - overflow pool design, burst fractions, two-tier sizing
  → Constraint: Routes are never dropped. If pool exhausted, fall back to unbounded (existing behavior).
  → Constraint: Congestion thresholds (80% denial, 95% teardown) use PoolUsedRatio -- must still work.
  → Decision: Pool capacity doc says `ze.fwd.pool.size` should be "Auto-sized from peer weights" but implementation uses static 100K. This spec implements that doc intent.
  → Constraint: Block-backed allocation: grow on demand, collapse when fully returned, lowest-block-first. Same as BufMux.
  → Decision: Per-peer channel (64 items) stays unchanged -- it absorbs micro-bursts. This spec is about the shared overflow pool only.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8654.md` - Extended Message capability. Negotiated per-peer. Max message size 65535 bytes = 16 x 4K slots.
  → Constraint: Overflow slots must be 4K base. Extended Message peers consume 16 consecutive slots.

**Key insights:**
- The overflow BufMux provides real 4K buffers (block-backed, grow/collapse). Each overflow item gets its own slot (1:1, no sharing) because per-destination modifications may be needed.
- Wire data buffer ownership is unchanged: read-path BufMux handles, reference-counted via RecentUpdateCache.retainCount. The overflow BufMux slot is separate from the wire buffer.
- Sizing formula: largest peer restart burst * capped fan-out + 10% steady-state from others. Fan-out = min(N-1, 2*sqrt(N)).
- Extended Message peers consume 16 consecutive 4K slots (64K). Block size must be a multiple of 16.
- Go channels cannot be resized -- the chan struct{} token bucket must be replaced, not resized.
- The formula is a pure function, isolated for future YANG override.
- Chaos default: 10K prefix max (not 1.1K) because `max(routes + 10%, 10000)` in config gen.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_pool.go` - `fwdOverflowPool` is a `chan struct{}` created at startup with `ze.fwd.pool.size` (default 100K). `acquire()` does non-blocking receive, `release()` sends back. `PoolUsedRatio()` = `1 - len(chan)/cap(chan)`. Created in `newFwdPool()`.
  → Constraint: `fwdItem.pooled` flag marks items that acquired a token. `done()` callback must release exactly once. This lifecycle must be preserved.
  → Constraint: When pool exhausted, items still go to unbounded overflow (routes never dropped). Escalation via congestion controller layers 3/4.
- [ ] `internal/component/bgp/reactor/bufmux.go` - BufMux implements block-backed allocation. `Get()` allocates from lowest block, grows new block when exhausted (subject to budget/maxBlocks). `Return()` routes to origin block. `tryCollapse()` deletes highest block when fully returned and block below has >=50% free. `probedPool` fires collapse probes on traffic-driven interval.
  → Constraint: Reuse this pattern. Do not duplicate BufMux logic.
- [ ] `internal/component/bgp/reactor/forward_pool_weight.go` - `peerBufferDemand(prefixMax, preEOR)` converts prefix max to buffer handles (divided by nlriPerMessage=20). `burstFraction()` scales by peer size tier. `calculatePoolBudget()` returns (guaranteed, overflow) where overflow = sum of K=sqrt(N) largest demands.
  → Decision: The existing `calculatePoolBudget` is not the right formula for this use case. We need a restart-burst formula: largest peer's demand * (N-1) fan-out + small contributions.
- [ ] `internal/component/bgp/reactor/forward_pool_weight_tracker.go` - `weightTracker` tracks per-peer demand, fires `onBudgetChanged(guaranteed, overflow)` callback when peers change. Currently only wired to bufmux sizing in reactor.go.
  → Constraint: Must add a second callback (or extend existing) to also resize the overflow pool's maxBlocks.
- [ ] `internal/component/bgp/reactor/forward_pool_congestion.go` - `congestionController` uses `poolUsedRatio()` for 80%/95% thresholds and `overflowDepths()` for per-peer tracking. `ShouldDeny()` and `CheckTeardown()` drive backpressure and session teardown.
  → Constraint: `PoolUsedRatio()` must return meaningful values. With BufMux: ratio = inUse / maxCapacity.
- [ ] `internal/component/bgp/reactor/reactor.go` - `newFwdPool()` creates pool with `overflowPoolSize` from env var. `weightTracker` callback at line 384 only updates bufmux budget. Congestion controller wired at line 394.
  → Constraint: Pool creation happens before peers are added. Initial maxBlocks must be a safe default (e.g., 64 slots). Adjusted when first peer is added.
- [ ] `internal/component/bgp/reactor/session.go` - `bufMux4K` and `bufMux64K` are global `probedPool` instances. `getReadBuffer()` returns from 4K pre-OPEN, 64K post-Extended-Message negotiation. `ReturnReadBuffer()` routes by buffer size.
  → Constraint: The overflow BufMux is a separate instance from the read-path BufMux instances. Different lifecycle, different budget.

**Behavior to preserve:**
- Zero-copy wire buffer ownership (overflow items hold read-path buffers, not copies)
- `fwdItem.pooled` flag and release-on-drain lifecycle
- Congestion controller thresholds (80% denial, 95% teardown) via `PoolUsedRatio()`
- Unbounded fallback when pool exhausted (routes never dropped)
- `ze.fwd.pool.size` env var as operator override
- Per-peer channel (64 items) unchanged

**Behavior to change:**
- Replace `chan struct{}` overflow pool with BufMux-backed pool (4K base, multi-slot for 64K)
- Auto-size pool maximum from peer prefix maximums using restart-burst formula
- Dynamic resize (maxBlocks update) when peers added/removed/EOR
- Block collapse returns memory to OS when overflow pressure subsides

## Data Flow (MANDATORY)

### Entry Point
- Reactor startup: `newFwdPool()` creates overflow BufMux (zero blocks, default maxBlocks)
- Peer addition: `weightTracker.AddPeer()` triggers budget recalculation

### Transformation Path
1. Reactor creates `fwdPool` with overflow BufMux (zero blocks initially)
2. Peers added via config -> `weightTracker.AddPeer(addr, prefixMax, familyCount)` -> `onBudgetChanged` callback fires
3. Callback calls sizing formula: `overflowPoolBudget(peers)` -> returns slot count
4. Slot count converted to maxBlocks: `maxBlocks = ceil(slots / blockSize)`
5. `overflowBufMux.SetMaxBlocks(maxBlocks)` updates the limit
6. On TryDispatch failure (channel full): `DispatchOverflow` calls `overflowBufMux.Get()` (1 slot for 4K, 16 slots for 64K)
7. If `Get()` returns valid handle: item marked `pooled`, handle stored in fwdItem
8. If `Get()` returns nil (exhausted): unbounded fallback (existing behavior)
9. Worker drains overflow item: `overflowBufMux.Return(handle)` releases slots
10. Probe fires on read-path `Get()`: overflow BufMux `tryCollapse()` checks for collapsible blocks
11. Peer EOR received: `weightTracker.PeerEORReceived` -> budget shrinks -> `SetMaxBlocks(newMax)`
12. Peer removed: budget shrinks similarly

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| weightTracker -> overflow pool | `onBudgetChanged` callback sets maxBlocks | [ ] |
| DispatchOverflow -> BufMux | `Get()` for 1 or 16 slots based on peer negotiation | [ ] |
| Worker drain -> BufMux | `Return()` for each handle | [ ] |
| Read-path probe -> overflow BufMux | `AddProbe()` chains collapse check | [ ] |

### Integration Points
- `fwdPool.overflowPool` field: changes from `*fwdOverflowPool` to `*BufMux` (or wrapper)
- `fwdItem`: `pooled bool` replaced by `overflowHandles []BufHandle` (1 handle for 4K, 16 for 64K)
- `PoolUsedRatio()`: reads from BufMux Stats instead of channel length
- `weightTracker.onBudgetChanged`: extended to also set overflow maxBlocks
- Collapse probe: overflow BufMux wired to read-path probe chain via `AddProbe()`

### Architectural Verification
- [ ] No bypassed layers (overflow still goes through DispatchOverflow, congestion still checks ratio)
- [ ] No unintended coupling (overflow BufMux is independent instance, not shared with read-path)
- [ ] No duplicated functionality (reuses BufMux, does not reimplement block logic)
- [ ] Zero-copy preserved (overflow items still hold read-path buffers, overflow BufMux is counting only)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Peer config with prefix max | -> | weightTracker recalculates, overflow maxBlocks updated | `TestOverflowPoolAutoSizedFromPeers` |
| TryDispatch fails (channel full) | -> | DispatchOverflow acquires overflow BufMux handle | `TestOverflowDispatchAcquiresBufMuxHandle` |
| Worker drains overflow | -> | overflow BufMux Return, block collapse | `TestOverflowDrainReturnsBufMuxHandle` |
| ze-chaos 4-peer scenario | -> | No "forward peer congested" during initial convergence | `test/chaos/no-congestion-initial.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Overflow pool created | Uses BufMux with 4K buffer size (not chan struct{}) |
| AC-2 | Peers added with prefix maximums | Overflow pool maxBlocks auto-sized from restart-burst formula |
| AC-3 | Sizing formula | Isolated as a pure function: largest peer restart burst * min(N-1, 2*sqrt(N)) fan-out + 10% steady-state contributions, floor 64 |
| AC-4 | `ze.fwd.pool.size` > 0 | Overrides auto-sizing (operator escape hatch). Value is slot count. |
| AC-5 | Peer added/removed/EOR received | Overflow maxBlocks recalculated and updated |
| AC-6 | Congestion controller queries PoolUsedRatio | Returns inUse/maxCapacity from overflow BufMux stats |
| AC-7 | Overflow pressure subsides, block fully drained | Block collapsed, memory returned to OS |
| AC-8 | Extended Message peer overflows | 16 consecutive 4K slots acquired (64K total) |
| AC-9 | Pool exhausted (maxBlocks reached) | Unbounded fallback (existing behavior), congestion controller escalates |
| AC-10 | ze-chaos 4-peer default | No "forward peer congested" warnings during initial convergence |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOverflowBufMuxGet4K` | `forward_pool_test.go` | Single slot acquisition for standard peer | |
| `TestOverflowBufMuxGet64K` | `forward_pool_test.go` | 16 consecutive slot acquisition for ExtMsg peer | |
| `TestOverflowBufMuxReturn` | `forward_pool_test.go` | Return releases slots, collapse check passes | |
| `TestOverflowBufMuxExhausted` | `forward_pool_test.go` | Get returns nil when maxBlocks reached | |
| `TestOverflowBufMuxCollapse` | `forward_pool_test.go` | Fully-returned block collapses via probe | |
| `TestOverflowPoolBudgetFormula` | `forward_pool_weight_test.go` | Pure function: largest peer * fan-out + small contributions | |
| `TestOverflowPoolBudgetSinglePeer` | `forward_pool_weight_test.go` | Single peer: demand = full restart burst | |
| `TestOverflowPoolBudgetFloor` | `forward_pool_weight_test.go` | Budget never below floor (64 slots) | |
| `TestOverflowPoolAutoResize` | `forward_pool_test.go` | AddPeer triggers maxBlocks update | |
| `TestOverflowPoolEORShrink` | `forward_pool_test.go` | EOR transitions shrink maxBlocks, excess blocks collapse when drained | |
| `TestOverflowPoolEnvOverride` | `forward_pool_test.go` | `ze.fwd.pool.size` > 0 disables auto-sizing | |
| `TestPoolUsedRatioBufMux` | `forward_pool_test.go` | PoolUsedRatio reads from BufMux stats correctly | |
| `TestDispatchOverflow64K` | `forward_pool_test.go` | DispatchOverflow for ExtMsg peer acquires 16 slots | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Prefix max per peer | 1 - MaxUint32 | MaxUint32 | 0 (skipped) | N/A |
| Number of peers | 1 - 10000 | 10000 | 0 (empty budget) | N/A |
| ze.fwd.pool.size | 0 - fwdOverflowPoolMaxSize | 10000000 | N/A (0 = auto) | 10000001 (capped) |
| ExtMsg slot count | 16 | 16 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-no-initial-congestion` | `test/chaos/no-congestion-initial.ci` | ze-chaos 4-peer default: no congested warnings during first 30s | |

### Future (if deferring any tests)
- Property-based test for sizing formula across random peer distributions -- deferred, formula is simple enough for table-driven tests

## Files to Modify
- `internal/component/bgp/reactor/forward_pool.go` - Replace `fwdOverflowPool` type with BufMux-based pool, update `acquire`/`release`/`PoolUsedRatio`, update `fwdItem` to carry `[]BufHandle` instead of `pooled bool`
- `internal/component/bgp/reactor/forward_pool_weight.go` - Add `overflowPoolBudget()` pure sizing function
- `internal/component/bgp/reactor/forward_pool_weight_tracker.go` - Extend `onBudgetChanged` or add second callback for overflow pool sizing
- `internal/component/bgp/reactor/reactor.go` - Wire overflow pool to weightTracker callback, wire collapse probe
- `internal/component/bgp/reactor/forward_pool_congestion.go` - No changes expected (PoolUsedRatio interface unchanged)
- `docs/architecture/forward-congestion-pool.md` - Update "Pool Capacity Tracks Peer Set" to reflect BufMux-backed implementation

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/forward-congestion-pool.md` -- update "Pool Capacity" section to describe BufMux-backed overflow and restart-burst formula |

## Files to Create
- None (all changes are modifications to existing files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Sizing formula** -- Implement `overflowPoolBudget()` as a pure function
   - Tests: `TestOverflowPoolBudgetFormula`, `TestOverflowPoolBudgetSinglePeer`, `TestOverflowPoolBudgetFloor`
   - Files: `forward_pool_weight.go`, `forward_pool_weight_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: BufMux-backed overflow pool** -- Replace `fwdOverflowPool` type
   - Tests: `TestOverflowBufMuxGet4K`, `TestOverflowBufMuxGet64K`, `TestOverflowBufMuxReturn`, `TestOverflowBufMuxExhausted`, `TestOverflowBufMuxCollapse`, `TestPoolUsedRatioBufMux`
   - Files: `forward_pool.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Wire to weightTracker** -- Connect auto-sizing to peer lifecycle
   - Tests: `TestOverflowPoolAutoResize`, `TestOverflowPoolEORShrink`, `TestOverflowPoolEnvOverride`
   - Files: `forward_pool_weight_tracker.go`, `reactor.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Multi-slot dispatch for ExtMsg** -- 16-slot acquisition in DispatchOverflow
   - Tests: `TestDispatchOverflow64K`
   - Files: `forward_pool.go`
   - Verify: tests fail -> implement -> tests pass

5. **Functional tests** -- ze-chaos convergence without congestion warnings
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- audit tables, learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | PoolUsedRatio returns correct values with BufMux stats (inUse/maxCapacity) |
| Naming | Overflow pool type/methods follow existing BufMux naming conventions |
| Data flow | weightTracker callback correctly propagates to overflow maxBlocks |
| Rule: no-layering | Old `fwdOverflowPool` chan struct{} type fully deleted, not wrapped |
| Rule: buffer-first | Overflow BufMux allocates 4K buffers via existing block pattern |
| Lifecycle | Every `Get()` paired with exactly one `Return()` -- no leaks on pool stop |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `overflowPoolBudget()` function exists | `grep overflowPoolBudget forward_pool_weight.go` |
| Old `fwdOverflowPool` chan struct{} removed | `grep "chan struct{}" forward_pool.go` returns no overflow pool hits |
| Overflow BufMux created in newFwdPool | `grep BufMux forward_pool.go` shows overflow instance |
| weightTracker callback updates overflow | `grep SetMaxBlocks reactor.go` shows wiring |
| PoolUsedRatio uses BufMux stats | `grep Stats forward_pool.go` in PoolUsedRatio |
| 16-slot ExtMsg handling | `grep "16\|extMsg\|ExtMsg" forward_pool.go` shows multi-slot path |
| Architecture doc updated | `grep "BufMux\|block-backed" forward-congestion-pool.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Overflow BufMux maxBlocks prevents unbounded growth. Verify fallback path on exhaustion. |
| Memory leak | Every Get() must have Return() -- check pool Stop() cleanup path |
| Integer overflow | Sizing formula with large prefix max values (MaxUint32) must not overflow |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

- The overflow pool's `chan struct{}` was a quick implementation that worked for the initial forward pool but was never connected to the weight-based demand system that the architecture doc anticipated.
- Single BufMux with 4K granularity handles both standard (1 slot) and Extended Message (16 slots) peers without needing separate pools. The key insight: 65535 / 4096 = 16, and 16 divides cleanly into reasonable block sizes.
- The sizing formula must account for route-reflection fan-out because each received route is forwarded to N-1 destination peers, each with its own overflow queue.
- Overflow slots are 1:1 with fwdItems (no sharing across destinations) because per-destination modifications (AS-PATH prepending) may mutate the buffer. Wire buffer refcounting via RecentUpdateCache is a separate mechanism and stays unchanged.
- Chaos default prefix max is 10000, not 1100, due to `max(routes + 10%, 10000)` floor in config generation. This means pre-EOR demand is 500 buffers per peer, not 55.
- Fan-out cap of 2*sqrt(N) prevents the formula from producing unreasonable pool sizes for large IXPs while still covering realistic convergence scenarios. The cap aligns with the existing sqrt(N) model in `overflowPeerCount`.

## RFC Documentation

Add `// RFC 8654: Extended Message peers consume 16 x 4K overflow slots` in the multi-slot acquisition path.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
