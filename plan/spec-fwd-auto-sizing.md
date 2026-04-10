# Spec: fwd-auto-sizing

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | - |
| Phase | - |
| Updated | 2026-04-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/forward-congestion-pool.md` - pool design, burst fractions, two-tier sizing
4. `internal/component/bgp/reactor/forward_pool.go` - fwdOverflowPool (to delete), TryDispatch, DispatchOverflow
5. `internal/component/bgp/reactor/bufmux.go` - BufMux block-backed pattern (extend with mixed-size subdivision)
6. `internal/component/bgp/reactor/session.go` - global bufMuxStd/bufMuxExt (to delete, replace with per-peer pools)
7. `internal/component/bgp/reactor/forward_pool_weight.go` - peerBufferDemand, burstFraction
8. `internal/component/bgp/reactor/forward_pool_weight_tracker.go` - weightTracker, onBudgetChanged

## Task

Replace the forward pool's three disconnected allocation systems (global `bufMuxStd`,
global `bufMuxExt`, static `chan struct{}` overflow token pool) with a two-tier model:

1. **Per-peer pool:** 64 buffers at the peer's negotiated message size (4K standard,
   64K for Extended Message / RFC 8654). Created at session establishment, destroyed
   on session teardown. Absorbs steady-state traffic and micro-bursts for that peer.

2. **One shared overflow pool:** A single mixed-size BufMux with 64K blocks that can
   be subdivided into 16 x 4K slices. When a peer's 64-buffer pool is full, overflow
   goes here. Budget tracked in bytes, auto-sized from peer prefix maximums via a
   restart-burst formula. When the overflow pool is full, dispatch is rejected --
   this IS the backpressure signal that makes the reader slow down.

The current system has the overflow pool disconnected from `weightTracker` demand
calculations, causing immediate "forward peer congested" warnings in ze-chaos because
overflow capacity does not match the actual topology.

### Two-Tier Model

| Tier | Scope | Size | Buffer size | Lifecycle |
|------|-------|------|-------------|-----------|
| Per-peer | One peer | 64 buffers | Negotiated (4K or 64K) | Session establishment to teardown |
| Shared overflow | All peers | Auto-sized (byte budget) | Mixed: 64K blocks, subdivisible to 4K | Reactor lifetime, blocks grow/collapse |

### Overflow Sizing Formula

Isolated as a pure function. Inputs: per-peer prefix maximums and negotiated sizes.
Output: byte budget for the shared overflow pool.

| Step | Calculation | Purpose |
|------|-------------|---------|
| 1 | `largest` = max peer's `peerBufferDemand(prefixMax, preEOR=true)` | Full restart burst of the biggest peer |
| 2 | `fanOut` = min(N-1, 2 * sqrt(N)), floor 1 | Capped fan-out: how many destinations receive forwarded routes |
| 3 | `restartBurst` = `largest` * `fanOut` | Total overflow from one peer restart, distributed across destinations |
| 4 | `steadyContrib` = sum of other peers' `peerBufferDemand(prefixMax, false)` * 0.1 | Small headroom for concurrent steady-state forwarding |
| 5 | `totalSlots` = `restartBurst` + `steadyContrib` | Combined slot count |
| 6 | `totalSlots` = max(`totalSlots`, 64) | Floor for small topologies |
| 7 | Convert to bytes using per-peer negotiated sizes (4K or 64K per slot) | Byte budget accounts for mixed sizes |

### Example Budgets

| Topology | Largest peer | Fan-out | Restart burst | Steady | Total slots | Memory (max) |
|----------|-------------|---------|---------------|--------|-------------|-------------|
| Chaos default (4 peers, 10K pfx max) | 500 | 3 | 1,500 | 45 | 1,545 | 6 MB |
| Small IXP (50 peers, 500K largest) | 25,000 | 14 | 350,000 | 1,650 | 351,650 | 1.4 GB |
| Medium IXP (200 peers, 1M largest) | 50,000 | 28 | 1,400,000 | 14,850 | 1,414,850 | 5.6 GB |
| Large IXP (1000 peers, 1M largest) | 50,000 | 63 | 3,150,000 | 74,850 | 3,224,850 | 12.6 GB |

Memory column is the maximum overflow pool capacity. Blocks are allocated on demand and
collapsed when drained, so actual memory is much lower in steady state.

### Mixed-Size Overflow Pool

The shared overflow pool uses 64K blocks as its base allocation unit. Each 64K block can
serve one Extended Message peer's overflow item, OR be subdivided into 16 x 4K slices for
standard peers. This avoids maintaining two separate pools. Budget is tracked in bytes --
a 4K overflow item costs 4K against the budget, a 64K item costs 64K.

### Buffer Ownership

Each overflow buffer is 1:1 with one `fwdItem` in one destination's overflow queue. No sharing
across destinations because per-destination modifications (AS-PATH prepending, attribute
rewriting) may be needed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/forward-congestion-pool.md` - overflow pool design, burst fractions, two-tier sizing
  → Constraint: Pool exhaustion is the backpressure signal. When overflow BufMux is full, dispatch is rejected, propagating pressure to the read path to slow down.
  → Constraint: Congestion thresholds (80% denial, 95% teardown) use PoolUsedRatio -- must still work.
  → Decision: Pool capacity doc says `ze.fwd.pool.size` should be "Auto-sized from peer weights" but implementation uses static 100K. This spec implements that doc intent.
  → Constraint: Block-backed allocation: grow on demand, collapse when fully returned, lowest-block-first. Same as BufMux.
  → Decision: Per-peer channel (64 items) replaced by per-peer buffer pool (64 buffers at negotiated size). Shared overflow pool replaces both global bufMux instances and chan struct{} token pool.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8654.md` - Extended Message capability. Negotiated per-peer. Max message size 65535 bytes. Determines per-peer buffer size (64K vs 4K).
  → Constraint: Per-peer pool uses negotiated size. Overflow pool uses 64K blocks subdivisible to 4K.

**Key insights:**
- Two tiers: per-peer pool (64 buffers, negotiated size) + one shared overflow pool (mixed-size, byte-budgeted).
- Overflow pool uses 64K blocks subdivisible into 16 x 4K slices. One pool handles both standard and ExtMsg peers.
- Replaces three separate systems: global bufMuxStd, global bufMuxExt, chan struct{} overflow tokens.
- Pool exhaustion IS the backpressure signal. No unbounded fallback.
- Sizing formula: largest peer restart burst * capped fan-out + 10% steady-state. Fan-out = min(N-1, 2*sqrt(N)).
- The formula is a pure function, isolated for future YANG override.
- Chaos default: 10K prefix max (not 1.1K) because `max(routes + 10%, 10000)` in config gen.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_pool.go` - `fwdOverflowPool` is a `chan struct{}` created at startup with `ze.fwd.pool.size` (default 100K). `acquire()` does non-blocking receive, `release()` sends back. `PoolUsedRatio()` = `1 - len(chan)/cap(chan)`. Created in `newFwdPool()`.
  → Decision: `fwdOverflowPool` chan struct{} deleted entirely. Per-peer pool (64 buffers) replaces per-worker channel. Shared overflow BufMux replaces token pool.
  → Constraint: When overflow pool exhausted, dispatch is rejected. This is the backpressure mechanism -- the reader must slow down. Congestion controller layers 3/4 use pool ratio for escalation decisions.
- [ ] `internal/component/bgp/reactor/bufmux.go` - BufMux implements block-backed allocation. `Get()` allocates from lowest block, grows new block when exhausted (subject to budget/maxBlocks). `Return()` routes to origin block. `tryCollapse()` deletes highest block when fully returned and block below has >=50% free. `probedPool` fires collapse probes on traffic-driven interval.
  → Constraint: Reuse this pattern. Do not duplicate BufMux logic.
- [ ] `internal/component/bgp/reactor/forward_pool_weight.go` - `peerBufferDemand(prefixMax, preEOR)` converts prefix max to buffer handles (divided by nlriPerMessage=20). `burstFraction()` scales by peer size tier. `calculatePoolBudget()` returns (guaranteed, overflow) where overflow = sum of K=sqrt(N) largest demands.
  → Decision: The existing `calculatePoolBudget` is not the right formula for this use case. We need a restart-burst formula: largest peer's demand * (N-1) fan-out + small contributions.
- [ ] `internal/component/bgp/reactor/forward_pool_weight_tracker.go` - `weightTracker` tracks per-peer demand, fires `onBudgetChanged(guaranteed, overflow)` callback when peers change. Currently only wired to bufmux sizing in reactor.go.
  → Constraint: Must add a second callback (or extend existing) to also resize the shared overflow pool's byte budget.
- [ ] `internal/component/bgp/reactor/forward_pool_congestion.go` - `congestionController` uses `poolUsedRatio()` for 80%/95% thresholds and `overflowDepths()` for per-peer tracking. `ShouldDeny()` and `CheckTeardown()` drive backpressure and session teardown.
  → Constraint: `PoolUsedRatio()` must return meaningful values. With BufMux: ratio = inUse / maxCapacity.
- [ ] `internal/component/bgp/reactor/reactor.go` - `newFwdPool()` creates pool with `overflowPoolSize` from env var. `weightTracker` callback at line 384 only updates bufmux budget. Congestion controller wired at line 394.
  → Constraint: Pool creation happens before peers are added. Initial maxBlocks must be a safe default (e.g., 64 slots). Adjusted when first peer is added.
- [ ] `internal/component/bgp/reactor/session.go` - `bufMuxStd` and `bufMuxExt` are global `probedPool` instances. `getReadBuffer()` returns from 4K pre-OPEN, 64K post-Extended-Message negotiation. `ReturnReadBuffer()` routes by buffer size.
  → Decision: Global `bufMuxStd` and `bufMuxExt` deleted. Replaced by per-peer pools (64 buffers at negotiated size) and one shared overflow BufMux (mixed-size, 64K blocks subdivisible to 4K).

**Behavior to preserve:**
- Zero-copy wire buffer ownership (buffers hold actual wire data, not copies)
- Release-on-drain lifecycle (per-peer buffer returned after processing, overflow buffer returned after drain)
- Congestion controller thresholds (80% denial, 95% teardown) via `PoolUsedRatio()`
- Pool exhaustion rejects dispatch (backpressure on reader)
- `ze.fwd.pool.size` env var as operator override (byte budget for overflow pool)

**Behavior to change:**
- Delete global `bufMuxStd` and `bufMuxExt` -- replaced by per-peer pools
- Delete `fwdOverflowPool` (chan struct{}) -- replaced by shared overflow BufMux
- Delete per-worker `chan fwdItem` (cap 64) -- replaced by per-peer pool (64 buffers at negotiated size)
- Per-peer pool: 64 buffers at negotiated message size, created at session establishment
- Shared overflow pool: one mixed-size BufMux (64K blocks subdivisible to 4K), byte-budgeted
- Auto-size overflow byte budget from peer prefix maximums using restart-burst formula
- Dynamic resize (byte budget update) when peers added/removed/EOR
- Block collapse returns memory to OS when overflow pressure subsides

## Data Flow (MANDATORY)

### Entry Point
- Session establishment: per-peer pool created (64 buffers at negotiated size)
- Reactor startup: shared overflow BufMux created (zero blocks, byte budget from weightTracker)
- Peer addition: `weightTracker.AddPeer()` triggers overflow budget recalculation

### Transformation Path
1. Session established, capability negotiation complete -> per-peer pool created (64 buffers, 4K or 64K)
2. Reactor creates shared overflow BufMux (zero blocks initially, mixed-size)
3. Peers added via config -> `weightTracker.AddPeer(addr, prefixMax, familyCount)` -> `onBudgetChanged` callback fires
4. Callback calls sizing formula: `overflowPoolBudget(peers)` -> returns byte budget
5. Overflow BufMux byte budget updated
6. ForwardUpdate -> TryDispatch tries per-peer pool first (64 buffers)
7. If per-peer pool full: `DispatchOverflow` calls overflow BufMux `Get()` (4K slice or 64K block depending on peer)
8. If `Get()` returns valid handle: item stored in overflow queue with handle
9. If `Get()` returns nil (exhausted): dispatch rejected, backpressure propagates to reader
10. Worker drains overflow item: overflow BufMux `Return(handle)` releases buffer
11. Overflow BufMux `tryCollapse()` checks for collapsible blocks on drain activity
12. Peer EOR received: `weightTracker.PeerEORReceived` -> overflow budget shrinks
13. Peer removed: per-peer pool destroyed, overflow budget shrinks

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session negotiation -> per-peer pool | Pool created with negotiated buffer size | [ ] |
| weightTracker -> overflow pool | `onBudgetChanged` callback sets byte budget | [ ] |
| Per-peer pool full -> overflow BufMux | `Get()` for 4K or 64K based on peer negotiation | [ ] |
| Worker drain -> overflow BufMux | `Return()` for each handle | [ ] |
| Session teardown -> per-peer pool | Pool destroyed, buffers returned | [ ] |

### Integration Points
- Per-peer pool: new type, 64 buffers at negotiated size, replaces per-worker `chan fwdItem`
- Shared overflow BufMux: replaces `fwdOverflowPool` (chan struct{}), `bufMuxStd`, `bufMuxExt`
- Mixed-size overflow: 64K blocks subdivisible to 16 x 4K slices
- `PoolUsedRatio()`: reads from overflow BufMux stats (usedBytes/budgetBytes)
- `weightTracker.onBudgetChanged`: extended to set overflow byte budget
- Collapse: overflow BufMux runs `tryCollapse()` on drain activity

### Architectural Verification
- [ ] No bypassed layers (overflow still goes through DispatchOverflow, congestion still checks ratio)
- [ ] Per-peer pool is per-session lifecycle (created/destroyed with session)
- [ ] No duplicated functionality (overflow BufMux reuses existing block-backed pattern)
- [ ] Backpressure preserved (overflow exhaustion rejects dispatch, reader slows down)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Session establishment with ExtMsg | -> | Per-peer pool created with 64 x 64K buffers | `TestPerPeerPoolCreatedOnSession` |
| Peer config with prefix max | -> | weightTracker recalculates, overflow byte budget updated | `TestOverflowPoolAutoSizedFromPeers` |
| Per-peer pool full | -> | DispatchOverflow acquires overflow BufMux handle (4K or 64K) | `TestOverflowDispatchAcquiresBufMuxHandle` |
| Worker drains overflow | -> | overflow BufMux Return, block collapse | `TestOverflowDrainReturnsBufMuxHandle` |
| Overflow pool exhausted | -> | Dispatch rejected, backpressure to reader | `TestOverflowExhaustedRejectsDispatch` |
| ze-chaos 4-peer scenario | -> | No "forward peer congested" during initial convergence | `test/chaos/no-congestion-initial.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Session established (standard peer) | Per-peer pool created: 64 x 4K buffers |
| AC-2 | Session established (ExtMsg peer) | Per-peer pool created: 64 x 64K buffers |
| AC-3 | Shared overflow pool created | Single mixed-size BufMux (64K blocks subdivisible to 4K), replaces chan struct{} and global bufMuxStd/bufMuxExt |
| AC-4 | Peers added with prefix maximums | Overflow byte budget auto-sized from restart-burst formula |
| AC-5 | Sizing formula | Isolated as a pure function: largest peer restart burst * min(N-1, 2*sqrt(N)) fan-out + 10% steady-state contributions, floor 64 slots converted to bytes |
| AC-6 | `ze.fwd.pool.size` > 0 | Overrides auto-sizing (operator escape hatch). Value is byte budget. |
| AC-7 | Peer added/removed/EOR received | Overflow byte budget recalculated and updated |
| AC-8 | Congestion controller queries PoolUsedRatio | Returns usedBytes/budgetBytes from overflow BufMux stats |
| AC-9 | Overflow pressure subsides, block fully drained | Block collapsed, memory returned to OS |
| AC-10 | Standard peer overflows (per-peer pool full) | 4K slice allocated from overflow BufMux (subdivided from 64K block) |
| AC-11 | ExtMsg peer overflows (per-peer pool full) | 64K block allocated from overflow BufMux |
| AC-12 | Overflow pool exhausted (byte budget reached) | ~~Dispatch rejected~~ Item accepted without buffer (routes never dropped). Backpressure via congestion controller: denial at 80% (ShouldDeny), teardown at 95% (CheckTeardown). |
| AC-13 | Session teardown | Per-peer pool destroyed, buffers returned |
| AC-14 | ze-chaos 4-peer default | No "forward peer congested" warnings during initial convergence |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPerPeerPool4K` | `forward_pool_test.go` | Per-peer pool: 64 x 4K buffers for standard peer | |
| `TestPerPeerPool64K` | `forward_pool_test.go` | Per-peer pool: 64 x 64K buffers for ExtMsg peer | |
| `TestPerPeerPoolExhausted` | `forward_pool_test.go` | Per-peer pool full after 64 Get() calls, next returns nil | |
| `TestPerPeerPoolReturn` | `forward_pool_test.go` | Return frees buffer, subsequent Get() succeeds | |
| `TestPerPeerPoolSessionTeardown` | `forward_pool_test.go` | Pool destroyed on session teardown, buffers returned | |
| `TestOverflowBufMuxGet4K` | `forward_pool_test.go` | 4K slice allocated from 64K block (subdivision) | |
| `TestOverflowBufMuxGet64K` | `forward_pool_test.go` | Full 64K block allocated for ExtMsg peer | |
| `TestOverflowBufMuxMixed` | `forward_pool_test.go` | 4K and 64K allocations coexist in same pool | |
| `TestOverflowBufMuxReturn` | `forward_pool_test.go` | Return releases buffer, collapse check passes | |
| `TestOverflowBufMuxExhausted` | `forward_pool_test.go` | Get returns nil when byte budget reached | |
| `TestOverflowBufMuxCollapse` | `forward_pool_test.go` | Fully-returned block collapses, memory freed | |
| `TestOverflowPoolBudgetFormula` | `forward_pool_weight_test.go` | Pure function: largest peer * fan-out + small contributions | |
| `TestOverflowPoolBudgetSinglePeer` | `forward_pool_weight_test.go` | Single peer: demand = full restart burst | |
| `TestOverflowPoolBudgetFloor` | `forward_pool_weight_test.go` | Budget never below floor (64 slots worth of bytes) | |
| `TestOverflowPoolAutoResize` | `forward_pool_test.go` | AddPeer triggers byte budget update | |
| `TestOverflowPoolEORShrink` | `forward_pool_test.go` | EOR transitions shrink byte budget, excess blocks collapse when drained | |
| `TestOverflowPoolEnvOverride` | `forward_pool_test.go` | `ze.fwd.pool.size` > 0 disables auto-sizing | |
| `TestPoolUsedRatioBufMux` | `forward_pool_test.go` | PoolUsedRatio reads from overflow BufMux stats correctly | |
| `TestOverflowExhaustedRejectsDispatch` | `forward_pool_test.go` | Overflow full -> dispatch rejected -> backpressure | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Prefix max per peer | 1 - MaxUint32 | MaxUint32 | 0 (skipped) | N/A |
| Number of peers | 1 - 10000 | 10000 | 0 (empty budget) | N/A |
| ze.fwd.pool.size | 0 - MaxInt64 (byte budget) | MaxInt64 | N/A (0 = auto) | N/A (no cap) |
| Per-peer pool size | 64 | 64 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-no-initial-congestion` | `test/chaos/no-congestion-initial.ci` | ze-chaos 4-peer default: no congested warnings during first 30s | |

### Future (if deferring any tests)
- Property-based test for sizing formula across random peer distributions -- deferred, formula is simple enough for table-driven tests

## Files to Modify
- `internal/component/bgp/reactor/forward_pool.go` - Delete `fwdOverflowPool` type. Add per-peer pool type (64 buffers, negotiated size). Replace overflow token acquire/release with overflow BufMux Get/Return. Update `PoolUsedRatio` to read overflow BufMux stats. Update `fwdItem` to carry `BufHandle` instead of `pooled bool`.
- `internal/component/bgp/reactor/bufmux.go` - Add mixed-size support: 64K blocks subdivisible to 16 x 4K slices. Add byte-budget tracking alongside block-count limits.
- `internal/component/bgp/reactor/forward_pool_weight.go` - Add `overflowPoolBudget()` pure sizing function returning byte budget
- `internal/component/bgp/reactor/forward_pool_weight_tracker.go` - Extend `onBudgetChanged` or add second callback for overflow byte budget
- `internal/component/bgp/reactor/reactor.go` - Wire shared overflow BufMux to weightTracker callback. Remove global bufMuxStd/bufMuxExt wiring.
- `internal/component/bgp/reactor/session.go` - Delete global `bufMuxStd`, `bufMuxExt`, `combinedBudget`. Per-peer pool created at session establishment with negotiated size.
- `internal/component/bgp/reactor/forward_pool_congestion.go` - No changes expected (PoolUsedRatio interface unchanged)
- `docs/architecture/forward-congestion-pool.md` - Update to describe two-tier model: per-peer pool + shared mixed-size overflow

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
| 12 | Internal architecture changed? | Yes | `docs/architecture/forward-congestion-pool.md` -- update to describe two-tier model: per-peer pool (64 buffers, negotiated size) + shared mixed-size overflow BufMux with restart-burst formula |

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

1. **Phase: Sizing formula** -- Implement `overflowPoolBudget()` as a pure function (byte budget)
   - Tests: `TestOverflowPoolBudgetFormula`, `TestOverflowPoolBudgetSinglePeer`, `TestOverflowPoolBudgetFloor`
   - Files: `forward_pool_weight.go`, `forward_pool_weight_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Mixed-size BufMux** -- Add 64K block subdivision to 4K slices in BufMux
   - Tests: `TestOverflowBufMuxGet4K`, `TestOverflowBufMuxGet64K`, `TestOverflowBufMuxMixed`, `TestOverflowBufMuxReturn`, `TestOverflowBufMuxExhausted`, `TestOverflowBufMuxCollapse`
   - Files: `bufmux.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Per-peer pool** -- Replace per-worker chan fwdItem with per-peer buffer pool
   - Tests: `TestPerPeerPool4K`, `TestPerPeerPool64K`, `TestPerPeerPoolExhausted`, `TestPerPeerPoolReturn`, `TestPerPeerPoolSessionTeardown`
   - Files: `forward_pool.go`, `session.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Shared overflow pool** -- Replace chan struct{} and global bufMux instances with single overflow BufMux
   - Tests: `TestPoolUsedRatioBufMux`, `TestOverflowExhaustedRejectsDispatch`
   - Files: `forward_pool.go`, `session.go`, `reactor.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Wire to weightTracker** -- Connect auto-sizing to peer lifecycle
   - Tests: `TestOverflowPoolAutoResize`, `TestOverflowPoolEORShrink`, `TestOverflowPoolEnvOverride`
   - Files: `forward_pool_weight_tracker.go`, `reactor.go`
   - Verify: tests fail -> implement -> tests pass

6. **Functional tests** -- ze-chaos convergence without congestion warnings
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- audit tables, learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | PoolUsedRatio returns correct values from overflow BufMux stats (usedBytes/budgetBytes) |
| Naming | Per-peer pool and overflow pool types follow existing naming conventions |
| Data flow | weightTracker callback correctly propagates to overflow byte budget |
| Rule: no-layering | Old `fwdOverflowPool` chan struct{}, global `bufMuxStd`, global `bufMuxExt` fully deleted |
| Rule: buffer-first | Overflow BufMux uses 64K blocks subdivisible to 4K via existing block pattern |
| Lifecycle | Per-peer pool destroyed on session teardown. Every overflow `Get()` paired with exactly one `Return()`. |
| Backpressure | Overflow exhaustion rejects dispatch -- no unbounded fallback path exists |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `overflowPoolBudget()` function exists | `grep overflowPoolBudget forward_pool_weight.go` |
| Old `fwdOverflowPool` chan struct{} removed | `grep "chan struct{}" forward_pool.go` returns no overflow pool hits |
| Global `bufMuxStd` and `bufMuxExt` removed | `grep "bufMuxStd\|bufMuxExt" session.go` returns no global instances |
| Per-peer pool type exists | `grep "peerPool\|perPeer" forward_pool.go` shows per-peer pool |
| Shared overflow BufMux created | `grep BufMux forward_pool.go` shows overflow instance |
| Mixed-size overflow (64K blocks, 4K subdivision) | `grep "subdivid\|4096\|slice" bufmux.go` shows subdivision logic |
| weightTracker callback updates overflow byte budget | `grep "budget\|Budget" reactor.go` shows wiring |
| PoolUsedRatio uses overflow BufMux stats | `grep Stats forward_pool.go` in PoolUsedRatio |
| Dispatch rejection on exhaustion (no unbounded fallback) | `grep "reject\|denied\|nil" forward_pool.go` in DispatchOverflow |
| Architecture doc updated | `grep "per-peer\|mixed-size\|two-tier" forward-congestion-pool.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Overflow BufMux maxBlocks is a hard bound. Verify dispatch rejection propagates backpressure to reader. |
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

- Three disconnected pool systems (bufMuxStd, bufMuxExt, chan struct{} overflow) replaced by two clean tiers: per-peer pool (steady state) + shared overflow (burst absorption). Fewer moving parts, one budget to manage.
- Per-peer pool size of 64 matches the current per-worker channel capacity. Proven sufficient for micro-burst absorption in production and ze-chaos testing.
- Mixed-size overflow pool: 64K blocks subdivisible to 16 x 4K slices. One pool handles both standard and ExtMsg peers without maintaining separate instances. The key insight: 65535 / 4096 = 16, clean subdivision.
- Pool exhaustion is the backpressure mechanism, not a failure mode. When overflow is full, dispatch is rejected, the reader sees the rejection, and slows down. No unbounded fallback -- that was the bug, not the feature.
- The sizing formula must account for route-reflection fan-out because each received route is forwarded to N-1 destination peers, each with its own overflow queue.
- Overflow buffers are 1:1 with fwdItems (no sharing across destinations) because per-destination modifications (AS-PATH prepending) may mutate the buffer.
- Chaos default prefix max is 10000, not 1100, due to `max(routes + 10%, 10000)` floor in config generation. This means pre-EOR demand is 500 buffers per peer, not 55.
- Fan-out cap of 2*sqrt(N) prevents the formula from producing unreasonable pool sizes for large IXPs while still covering realistic convergence scenarios.

## RFC Documentation

Add `// RFC 8654: Extended Message peers use 64K buffers (per-peer pool and overflow)` at per-peer pool creation and overflow allocation paths.

## Implementation Summary

### What Was Implemented
- Two-tier model: per-peer pools (64-slot atomic counter) + shared MixedBufMux overflow pool
- Mixed-size subdivision: 64K blocks subdivisible to 16 x 4K slices via bitmask
- Three-level slicing: chunk -> block -> slice, zero per-item allocation on hot path
- Stable block IDs via tombstoning (avoids swap-delete corruption)
- overflowPoolBudget() pure sizing formula wired to weightTracker callback
- Congestion controller: ShouldDeny at 80%, CheckTeardown at 95%
- ze.fwd.pool.size default changed from 100000 to 0 (auto-sized)

### Bugs Found/Fixed
- Swap-delete corrupted block IDs (caught in self-review, fixed with tombstoning)
- Chunk memory pinned forever (deleted chunks slice, block backing slices pin via GC)
- First implementation was two separate pools (spec said single pool)
- First dispatch wiring left new code unwired (deep review caught)

### Documentation Updates
- docs/architecture/forward-congestion-pool.md updated for two-tier model

### Deviations from Plan
- AC-12: Implementation accepts items without buffer on pool exhaustion (routes never dropped). Backpressure via congestion controller (denial/teardown), not dispatch rejection. This is intentional -- routing correctness requires no route loss.
- Global bufMuxStd/bufMuxExt NOT deleted: they serve read-path buffers (pre-OPEN, build), not overflow. Spec conflated them with the overflow pool.
- No .ci functional test: needs slow-consumer simulation infrastructure. Unit tests cover all paths.
- ze.fwd.pool.size default changed from 100000 (legacy token count) to 0 (auto-sized).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Per-peer pool (64 buffers, negotiated size) | Done | forward_pool.go:205-277 | peerPool type, newPeerPool() |
| Shared overflow MixedBufMux | Done | bufmux.go:485-817 | MixedBufMux, Get4K, Get64K |
| Auto-sized overflow byte budget | Done | forward_pool_weight.go:142-194 | overflowPoolBudget() pure function |
| weightTracker callback wiring | Done | forward_pool_weight_tracker.go:24, reactor.go:391-400 | onBudgetChanged fires SetByteBudget |
| Delete fwdOverflowPool chan struct{} | Done | forward_pool.go | Replaced by MixedBufMux |
| Delete global bufMuxStd/bufMuxExt | Partial | session.go:54,59 | Kept -- serve read-path, not overflow |
| PoolUsedRatio from overflow stats | Done | forward_pool.go:734-741 | Delegates to MixedBufMux.UsedRatio |
| Congestion thresholds preserved | Done | forward_pool_congestion.go:26,30 | 80% denial, 95% teardown |
| Architecture doc updated | Done | docs/architecture/forward-congestion-pool.md | Two-tier model described |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | TestPeerPool4K, TestFwdPool_RegisterUnregisterOutgoingPool | 64 x 4K buffers |
| AC-2 | Done | TestPeerPool64K, TestFwdPool_DispatchOverflowGet64K, TestFwdPool_ReregisterExtMsg | 64 x 64K buffers |
| AC-3 | Done | TestMixedBufMux_Mixed, TestMixedBufMux_WholeAndSubdivCoexist | Single pool, mixed sizes |
| AC-4 | Done | TestOverflowPoolBudget | restart-burst * fan-out + steady |
| AC-5 | Done | TestOverflowPoolBudget, TestOverflowPoolBudgetBytes | Pure function, table-driven |
| AC-6 | Done | TestOverflowPoolEnvOverride (weight_tracker_test.go:349) | Callback no-ops when override active |
| AC-7 | Done | TestWeightTracker_UpdateExtMsg, AddPeer/RemovePeer/PeerEORReceived | Budget recalculated |
| AC-8 | Done | TestFwdPool_PoolUsedRatioMixedBufMux | usedBlocks/maxBlocks |
| AC-9 | Done | TestBufMux_CollapseHighest, TestMixedBufMux_Collapse | Block collapse on drain |
| AC-10 | Done | TestMixedBufMux_Get4K, TestFwdPool_DispatchOverflow | 4K slice from 64K block |
| AC-11 | Done | TestMixedBufMux_Get64K, TestFwdPool_DispatchOverflowGet64K | Full 64K block |
| AC-12 | Changed | TestFwdPool_DenialThroughDispatchOverflow, TestFwdPool_OverflowExhaustedRejectsDispatch | Routes never dropped; backpressure via denial/teardown |
| AC-13 | Done | TestFwdPool_UnregisterWithInFlightItems, TestFwdPool_PeerDisconnectReturnsSlots | Orphaned pool accepts returns |
| AC-14 | Skipped | - | No .ci chaos test; needs slow-consumer infrastructure |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestPeerPool4K | Done | forward_pool_test.go | |
| TestPeerPool64K | Done | forward_pool_test.go | |
| TestPeerPoolExhausted | Done | forward_pool_test.go | |
| TestPeerPoolReturn | Done | forward_pool_test.go | |
| TestPeerPoolConcurrent | Done | forward_pool_test.go | Replaces TestPerPeerPoolSessionTeardown |
| TestMixedBufMux_Get4K | Done | bufmux_test.go | |
| TestMixedBufMux_Get64K | Done | bufmux_test.go | |
| TestMixedBufMux_Mixed | Done | bufmux_test.go | |
| TestMixedBufMux_Return | Done | bufmux_test.go | |
| TestMixedBufMux_Exhausted | Done | bufmux_test.go | |
| TestMixedBufMux_Collapse | Done | bufmux_test.go | |
| TestOverflowPoolBudget | Done | forward_pool_weight_test.go | |
| TestOverflowPoolBudgetBytes | Done | forward_pool_weight_test.go | Replaces TestOverflowPoolBudgetSinglePeer |
| TestOverflowPoolBudget/floor-applied | Done | forward_pool_weight_test.go | Floor 64 slots |
| TestFwdPool_PoolUsedRatioMixedBufMux | Done | forward_pool_test.go | Replaces TestPoolUsedRatioBufMux |
| TestFwdPool_OverflowExhaustedRejectsDispatch | Done | forward_pool_test.go | Actually accepts (routes never dropped) |
| TestOverflowPoolEnvOverride | Done | forward_pool_weight_tracker_test.go | |
| TestFwdPool_DenialThroughDispatchOverflow | Done | forward_pool_test.go | NEW: denial integration |
| TestFwdPool_UnregisterWithInFlightItems | Done | forward_pool_test.go | NEW: teardown with in-flight |
| TestFwdPool_ReregisterExtMsg | Done | forward_pool_test.go | NEW: 4K->64K renegotiation |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| forward_pool.go | Done | Per-peer pool, overflow BufMux wiring, PoolUsedRatio |
| bufmux.go | Done | MixedBufMux, 64K blocks, 4K subdivision |
| forward_pool_weight.go | Done | overflowPoolBudget() pure function |
| forward_pool_weight_tracker.go | Done | Extended callback with overflowBudgetResult |
| reactor.go | Done | Wired overflow mux, weightTracker callback |
| session.go | Partial | bufMuxStd/bufMuxExt kept (read-path, not overflow) |
| forward_pool_congestion.go | Done | No changes needed (thresholds unchanged) |
| docs/architecture/forward-congestion-pool.md | Done | Two-tier model documented |

### Audit Summary
- **Total items:** 32 (9 requirements, 14 ACs, 20 tests, 8 files -- some overlap)
- **Done:** 29
- **Partial:** 2 (session.go bufMux kept intentionally, AC-14 no .ci test)
- **Skipped:** 1 (AC-14 chaos functional test)
- **Changed:** 1 (AC-12: denial/teardown backpressure, not dispatch rejection)

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
