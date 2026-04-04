# Forward Congestion Pool Design

How ze handles slow destination peers without dropping routes or consuming
unbounded memory. This document records the design decisions for the overflow
pool, buffer ownership, weighted access, and backpressure.

<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdPool, peerPool, fwdItem -->
<!-- source: internal/component/bgp/reactor/bufmux.go -- MixedBufMux, BufMux, combinedBudget -->
<!-- source: internal/component/bgp/reactor/forward_pool_weight.go -- overflowPoolBudget, overflowFanOut -->
<!-- source: internal/component/bgp/reactor/session.go -- bufMux4K, bufMux64K (probedPool), getReadBuffer -->

## Invariant

**Routes are never dropped.** If the system cannot deliver a route to a peer,
it must either buffer the route or tear down the session. Silent discard is
never acceptable.

## Two-Tier Pool Model

Forward dispatch uses a two-tier pool:

| Tier | Scope | Size | Buffer size | Lifecycle |
|------|-------|------|-------------|-----------|
| Per-peer pool | One peer | 64 slots | Negotiated (4K or 64K) | Peer add to peer remove |
| Shared overflow | All peers | Auto-sized (byte budget) | Mixed: 64K blocks, subdivisible to 4K | Reactor lifetime |

The per-peer pool (`peerPool`) absorbs steady-state traffic and micro-bursts.
When full, items spill into the shared overflow `MixedBufMux`. When the overflow
pool is exhausted (byte budget reached), dispatch proceeds without a pool token
but congestion controller thresholds escalate (denial at 80%, teardown at 95%).

## Four-Layer Congestion Response

| Layer | Mechanism | Trigger |
|-------|-----------|---------|
| 1 | Per-peer pool (64 slots at negotiated message size) | Absorbs micro-bursts |
| 2 | Shared overflow MixedBufMux (auto-sized byte budget) | Per-peer pool full |
| 3 | Buffer denial on culprit source peers (natural TCP backpressure) | Pool filling |
| 4 | Session teardown (GR-aware, last resort) | Pool exhausted, backpressure insufficient |

## Overflow Sizing Formula

The shared overflow pool byte budget is auto-sized from peer prefix maximums
using a restart-burst formula (`overflowPoolBudget()` in `forward_pool_weight.go`):

1. `largest` = max peer's `peerBufferDemand(prefixMax, preEOR=true)`
2. `fanOut` = min(N-1, 2*sqrt(N)), floor 1
3. `restartBurst` = largest * fanOut
4. `steadyContrib` = sum of other peers' `peerBufferDemand(prefixMax, false)` * 0.1
5. `totalSlots` = restartBurst + steadyContrib, floor 64
6. Convert to bytes using per-peer negotiated sizes (4K or 64K per slot)

The formula is recalculated when peers are added, removed, or complete EOR.
`ze.fwd.pool.size` overrides auto-sizing when set (operator escape hatch).

## Pool Capacity Tracks Peer Set

Each configured peer contributes a weight to the pool's maximum capacity.
The weight is derived from expected prefix count (in priority order):

1. Configured `prefix maximum` per peer
2. Previous run observation (zefs)
3. Local RIB count (after session established)
4. Compiled-in PeeringDB/routing table data
5. PeeringDB API refresh (on-demand, for unknown ASNs)

<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdOverflowPool -->

Adding a peer increases the pool's maximum potential size. Removing a peer
decreases it. The pool's maximum is the sum of all peer weights scaled by
a burst fraction that varies with peer size.

## Burst Fraction: Inverse to Peer Size

Operators over-provision `prefix maximum` by a factor that depends on
peer size. Small peers over-state by 4x or more (changing the value
requires coordination). Large peers set it close to actual because the
DFZ grows predictably.

The burst fraction accounts for this asymmetry. It estimates the
realistic burst (convergence event) as a percentage of `prefix maximum`,
not a uniform fraction.

### DFZ Reference (March 2026)

| Table | Prefixes | Growth |
|-------|----------|--------|
| IPv4 full | ~1,100,000 | ~5-10% per year |
| IPv6 full | ~260,000 | ~5-10% per year |
| Combined dual-stack | ~1,360,000 | |

Sources: CIDR Report (cidr-report.org), Geoff Huston (bgp.potaroo.net),
RIPE RIS (stat.ripe.net), RouteViews (routeviews.org).

### Scaling Curve

| Prefix maximum | Typical real/max ratio | Burst fraction | Reasoning |
|---------------|----------------------|---------------|-----------|
| 1 - 499 | ~25% (4x overstatement) | 100% of max | Peer could genuinely double overnight (new customer). Small absolute numbers. |
| 500 - 9,999 | ~40% | 50% | Medium operators, moderate growth room |
| 10,000 - 99,999 | ~60-70% | 30% | Transit/content, more predictable |
| 100,000 - 499,999 | ~80% | 15% | Large transit, table is mostly stable |
| 500,000+ (full table) | ~90% | 10% | DFZ grows slowly, convergence events are the main burst source |

**Example:** A peer with `prefix maximum 200` has weight 200 (100%
burst fraction). A peer with `prefix maximum 1,000,000` has weight
100,000 (10% burst fraction). Weight reflects the realistic burst, not
the raw limit. Weight is used for both pool share fairness and
usage-to-weight ratio tracking.

<!-- source: internal/component/bgp/reactor/forward_pool_weight.go -- burstFraction, burstWeight -->

### Channel Size (Layer 1)

The per-peer channel absorbs micro-bursts (milliseconds of updates).
Fixed at 64 items for all peers, based on RIPE RIS analysis of burst
patterns across the DFZ. The channel is the fast path (no weighted
access check, no fairness tracking). Configurable via `ze.fwd.chan.size`.

## Two-Tier Pool Sizing

The buffer pool has two tiers: a guaranteed tier that is pre-allocated
and always available, and an overflow tier that grows dynamically during
congestion. When both are exhausted, backpressure engages automatically.

<!-- source: internal/component/bgp/reactor/bufmux.go -- BufMux, combinedBudget -->
<!-- source: internal/component/bgp/reactor/session.go -- bufMux4K, bufMux64K -->

### From NLRIs to Buffers

Each BGP UPDATE message read from TCP consumes one bufmux handle
(one 4K or 64K buffer). A single UPDATE carries multiple NLRIs.
The packing ratio determines how many buffers are needed for a
given number of NLRIs.

| Constant | Value | Purpose |
|----------|-------|---------|
| `nlriPerMessage` | 20 | Conservative NLRIs per UPDATE (actual: 50-200 for shared-attribute batches) |
| `bufSize` | 4096 | Standard BGP message buffer |

Buffers needed for a peer = `ceil(expected_nlris / nlriPerMessage)`.

These constants are intentionally conservative. Real packing is 3-10x
denser, so actual memory usage is well below the calculated maximums.
The values are designed to be easy to change for future configuration.

<!-- source: internal/component/bgp/reactor/forward_pool_weight.go -- nlriPerMessage, buffersNeeded, peerBufferDemand -->

### Pre-EOR vs Post-EOR Demand

A peer's buffer demand depends on its session phase. During initial
table dump (pre-EOR), the peer sends its entire routing table. During
steady state (post-EOR), only incremental updates flow.

| Phase | NLRIs expected | Buffers needed | When |
|-------|---------------|----------------|------|
| Pre-EOR | `prefixMax` (full table) | `prefixMax / 20` | Session start until End-of-RIB |
| Post-EOR | `burstWeight(prefixMax)` | `burstWeight(prefixMax) / 20` | After all family EORs received |

Pre-EOR peers get full-table allocation so initial convergence is never
throttled. When the last EOR arrives (all families done), the peer's
demand drops to burst-only, freeing capacity for other peers.

**Per-peer buffer demand examples (4K buffers):**

| Prefix Max | Pre-EOR buffers | Pre-EOR memory | Burst % | Post-EOR buffers | Post-EOR memory |
|-----------|----------------|----------------|---------|-----------------|-----------------|
| 200 | 10 | 40 KB | 100% | 10 | 40 KB |
| 1,000 | 50 | 200 KB | 50% | 25 | 100 KB |
| 10,000 | 500 | 2 MB | 30% | 150 | 600 KB |
| 50,000 | 2,500 | 10 MB | 30% | 750 | 3 MB |
| 100,000 | 5,000 | 20 MB | 15% | 750 | 3 MB |
| 300,000 | 15,000 | 60 MB | 15% | 2,250 | 9 MB |
| 500,000 | 25,000 | 100 MB | 10% | 2,500 | 10 MB |
| 1,000,000 | 50,000 | 200 MB | 10% | 5,000 | 20 MB |

### Guaranteed Tier (Pre-Allocated)

The guaranteed tier is the sum of all peers' buffer demands. These
bufmux blocks are pre-allocated at startup (or when peers are added)
so the buffers are always available without runtime allocation.

```
guaranteed = sum(buffers_needed) across all peers
```

Each peer contributes its current-phase demand (pre-EOR or post-EOR).

### Overflow Tier (Dynamic)

The overflow tier absorbs the case where destination peers are slow and
source read buffers stay pinned longer than expected. It grows
dynamically on demand and collapses when traffic subsides.

The overflow size is the sum of the K largest peers' buffer demands,
where K scales with the square root of the total peer count:

```
K = max(1, sqrt(total_peers))
overflow = sum of K largest peer buffer demands
```

This says: "the overflow pool can handle sqrt(N) of the largest peers
being stuck simultaneously." The scaling adapts without configuration:

| Total peers | K (simultaneous slow) | Reasoning |
|------------|----------------------|-----------|
| 4 | 2 | Small deployment, 2 slow peers is realistic |
| 25 | 5 | Medium, multiple slow peers possible |
| 100 | 10 | Large, but unlikely more than 10 stuck at once |
| 500 | 22 | IXP scale |
| 1,000 | 31 | Large IXP |

### Total Pool Budget

```
total_buffers = guaranteed + overflow
maxBlocks = ceil(total_buffers / blockSize)
```

The combined byte budget (`ze.fwd.pool.maxbytes`) is set to
`total_buffers * bufSize`. When explicitly configured, the operator's
value overrides auto-sizing.

### Dynamic Recalculation

<!-- source: internal/component/bgp/reactor/forward_pool_weight_tracker.go -- weightTracker, AddPeer, PeerEORReceived, RemovePeer -->

The pool budget recalculates on three events:

| Event | Action |
|-------|--------|
| AddPeer | Add peer's pre-EOR demand, recalculate guaranteed + overflow, grow budget |
| EOR received (all families) | Switch peer to post-EOR demand, recalculate, collapse excess blocks |
| RemovePeer | Remove peer demand, recalculate, collapse excess blocks |

On a route server restart, all peers connect simultaneously. The pool
starts at peak (all pre-EOR). As EORs arrive over the next minutes,
each peer's allocation shrinks and the pool collapses to steady state.

### IXP Memory Profiles

Real-world memory consumption for route server deployments. All
calculations use 4K buffers and 20 NLRIs per message (conservative).

**Small IXP (50 peers)**
Peer distribution: 30 @ 500, 15 @ 10K, 4 @ 100K, 1 @ 500K prefixes.

| Phase | Guaranteed | Overflow (K=7) | Total |
|-------|-----------|----------------|-------|
| Pre-EOR (all starting) | 213 MB | 180 MB | **393 MB** |
| Post-EOR (steady state) | 34 MB | 23 MB | **57 MB** |

**Medium IXP (200 peers)**
Peer distribution: 100 @ 1K, 60 @ 20K, 30 @ 100K, 8 @ 300K, 2 @ 1M prefixes.

| Phase | Guaranteed | Overflow (K=14) | Total |
|-------|-----------|-----------------|-------|
| Pre-EOR (all starting) | 1.74 GB | 1.07 GB | **2.81 GB** |
| Post-EOR (steady state) | 284 MB | 124 MB | **408 MB** |

**Large IXP (1000 peers)**
Peer distribution: 500 @ 500, 300 @ 20K, 150 @ 100K, 40 @ 500K, 10 @ 1M prefixes.

| Phase | Guaranteed | Overflow (K=31) | Total |
|-------|-----------|-----------------|-------|
| Pre-EOR (all starting) | 10.3 GB | 6.4 GB | **16.7 GB** |
| Post-EOR (steady state) | 1.46 GB | 410 MB | **1.87 GB** |

**Key observations:**

- Steady state is modest: under 2 GB even for a 1000-peer IXP
- Pre-EOR peaks at 8-9x the steady-state allocation but is transient
- Route server machines typically have 128+ GB RAM; even peak fits easily
- The 20 NLRIs/message ratio is 3-10x more conservative than real packing,
  so actual memory usage is well below these calculated maximums
- As each peer completes initial sync (sends EOR), its allocation drops
  and pool blocks are collapsed via the lazy probe mechanism

## Block-Backed Allocation

Each block is a single contiguous `make([]byte, N*bufSize)`. Individual
buffers are slices into this backing array -- no per-buffer allocation.

| Property | Value |
|----------|-------|
| Block backing | One `[]byte` allocation per block |
| Buffer | Slice into backing array: `block[i*bufSize : (i+1)*bufSize]` |
| Slice header cost | 24 bytes (pointer + length + capacity) |
| GC objects | One per block, not millions of individual buffers |

### GC-Based Release: All-or-Nothing Per Block

The Go GC treats each backing array as one object. It is freed only when
every slice pointing into it becomes unreachable.

| Scenario | Block freed by GC? |
|----------|--------------------|
| All buffers from block returned, references nil'd | Yes |
| 99/100 returned, 1 still in overflow | No -- entire block retained |

This is the accepted tradeoff for the performance gain. In practice,
congestion bursts arrive and drain together -- buffers from the same
block tend to be freed in a similar time window. The "one straggler pins
the whole block" scenario requires one buffer from an old block stuck at
a still-slow destination while all siblings have drained, which is
uncommon because a destination draining 99/100 items will drain the last
one too.

### Allocation: Lowest Block First

`Get()` allocates from the lowest-numbered block with free buffers.
This keeps steady-state traffic consolidated in low blocks, letting
higher blocks drain and become candidates for collapse.

During a burst, blocks are grown sequentially (0, 1, 2, ...). After
the burst subsides, FIFO returns arrive for all blocks. The lowest
block (0) receives returns first (its buffers were allocated earliest,
sit at the front of overflow queues) and also receives new steady-state
allocations. Higher blocks only receive returns and never get refilled
-- they converge to fully-free in natural top-down order and are
collapsed by the periodic check.

### Growth: On Exhaustion

The pool starts with zero blocks. A block is allocated when `Get()`
finds no free buffer in any existing block.

| Pool state | Action |
|-----------|--------|
| No blocks exist | Allocate block 0 |
| All blocks full, below maximum | Allocate next sequential block |
| All blocks full, at maximum | Return zero handle (pool exhausted -- backpressure) |

No speculative growth. No 90% threshold. A block is allocated exactly
when needed.

### Shrink: Lazy Collapse via Traffic-Driven Probe

Blocks are not freed on Return(). Instead, the `probedPool` wrapper
fires a probe on every `Get()`. The probe target owns the counter and
triggers a collapse check every N calls (default 100):

| Condition | Action |
|-----------|--------|
| Highest block fully returned AND block below has >=50% free | Delete highest block, repeat check on new highest |
| Highest block has outstanding buffers | No collapse |
| Highest block fully returned but block below <50% free | No collapse (would need to regrow immediately) |
| Only one block exists | No collapse (nothing to collapse into) |

The collapse cascades: if blocks 2, 1, 0 are all fully returned, one
check pass deletes them top-down until only the block with active
allocations remains.

**Why traffic-driven, not Return()-driven or timer-driven:** After a
burst subsides, the overflow path stops receiving `Get()` calls --
traffic is normal. But the normal read path calls `Get()` on the same
multiplexer for every BGP message received from the network. The
`probedPool` wrapper fires a probe callback on every `Get()`. The probe
target (overflow pool) owns the counter and decides when to act --
currently every 100th tick triggers a collapse check. No timer needed
-- network traffic is the heartbeat. The wrapper is a pure trigger; the
counter and interval belong to the target, not the wrapper.

**No permanent block.** If all traffic stops and every buffer returns,
all blocks are deleted. The next `Get()` allocates a fresh block. The
"hot reserve" is whichever block is currently serving traffic, not a
designated block 0.

### Example

Pool block size is 100 buffers each.

| Event | Blocks | In use | Free | Action |
|-------|--------|--------|------|--------|
| First Get() | 0 | 1 | 99 | Block 0 allocated, buffer from 0 |
| Burst fills block 0 | 0 | 100 | 0 | -- |
| Next Get(), block 0 full | 0, 1 | 101 | 99 | Block 1 allocated, buffer from 1 |
| Block 1 fills | 0, 1, 2 | 201 | 99 | Block 2 allocated on next Get() |
| Burst peaks at 250 | 0, 1, 2 | 250 | 50 | Within capacity |
| Burst subsides, returns arrive | 0, 1, 2 | 180 | 120 | FIFO: block 0 buffers return first |
| Steady state resumes | 0, 1, 2 | 80 | 220 | New allocations go to block 0 (lowest). Blocks 1, 2 only receive returns. |
| Block 2 fully returned | 0, 1, 2 | 60 | 240 | Block 2 has all buffers home |
| 100th Get() collapse check | 0, 1 | 60 | 40 | Block 2 (highest) fully returned, block 1 has >=50% free: delete block 2 |
| Block 1 fully returned | 0, 1 | 30 | 70 | Block 1 has all buffers home |
| 100th Get() collapse check | 0 | 30 | 70 | Block 1 (highest) fully returned, block 0 has >=50% free: delete block 1 |
| Settled | 0 | 30 | 70 | Only block 0 remains |

After the spike, only block 0 remains -- the one serving steady-state
traffic. Higher blocks drained because lowest-first allocation directed
all new gets to block 0, while higher blocks only received returns.

## Zero-Copy Buffer Ownership

The key design decision: during congestion, the overflow pool provides
read buffers to the source peer directly. There is no copy or ownership
transfer step.

### Normal Path (no congestion)

| Step | Buffer |
|------|--------|
| Read loop requests handle from multiplexer | `Get()` returns `{ID, Buf}` from preferred block |
| TCP read fills `handle.Buf` with UPDATE | Handle held by read loop |
| UPDATE processed, forwarded via channel | Handle passed through |
| Processing complete | `Return(handle)` routes to `blocks[ID]` free list |

<!-- source: internal/component/bgp/reactor/session.go -- getReadBuffer, ReturnReadBuffer -->

### Congestion Path (channel full)

| Step | Buffer |
|------|--------|
| Read loop detects destination channel full | -- |
| Read loop requests handle from multiplexer (weighted access check) | `Get()` checks usage-to-weight ratio before granting |
| TCP read fills `handle.Buf` with UPDATE | Handle held by multiplexer (buffer is in a block) |
| Item queued in overflow backlog | Handle passed to backlog -- no copy, buffer stays in same block |
| Destination worker drains item, forwards to peer | Handle passed through |
| Processing complete | `Return(handle)` routes to `blocks[ID]` free list |

The buffer was always in its block's backing array. It was allocated from
the multiplexer, read into directly, and returned to the same block after
the destination peer processes it. Zero copies by construction.

### Natural Backpressure

When the overflow pool runs low on buffers, the source peer cannot obtain
a buffer to read into. It physically cannot read from TCP. This causes the
kernel receive buffer to fill, which causes TCP to advertise a smaller
receive window, which causes the remote peer to slow its sends.

The pool IS the backpressure mechanism. No separate throttle logic is needed
for the basic case -- buffer exhaustion creates backpressure automatically.

## Weighted Access

Not all peers have equal claim to the pool. A peer announcing 500K prefixes
can generate larger convergence bursts than one announcing 200 prefixes.
Access rights are proportional to weight but diminish with usage.

### Weight Assignment

Weight = burst-adjusted prefix count. It is the peer's `prefix maximum`
(or observed/estimated count) scaled by the burst fraction for that peer's
size tier. Weight determines both the peer's proportional share of the
pool (fairness) and the denominator in the usage-to-weight ratio
(backpressure targeting).

### Diminishing Access

A peer's effective priority to claim pool buffers decreases as it consumes
more. The access decision considers the ratio of usage to weight:

| Peer | Weight (prefix count) | Pool buffers used | Usage/weight ratio | Effective priority |
|------|----------------------|-------------------|-------------------|-------------------|
| A | 500K (50%) | 0 | 0.0 | Highest |
| B | 300K (30%) | 0 | 0.0 | High |
| A | 500K (50%) | 400 | 0.8 | Low |
| C | 200K (20%) | 0 | 0.0 | Full weight |

When the pool is under pressure, the peer with the highest usage-to-weight
ratio is the first to be denied buffers (backpressure) and the first
candidate for session teardown (Layer 4).

### Rebalancing Under Pressure

When the pool is full and reads pause:

1. Slow peer's destination worker continues draining overflow items
2. As items drain, that peer's usage drops, effective priority rises
3. No new items arrive (source peer cannot get buffers)
4. Over time, usage ratios converge toward weight ratios
5. Reads resume when pool has headroom

The system naturally rebalances toward proportional usage without any
explicit rebalancing algorithm.

### Forced Teardown: Preventing Pool Monopolisation

Buffer denial slows inflow but cannot reclaim buffers already held by a
stuck destination. One frozen peer can pin every buffer in the pool,
freezing all source peers' read loops. Unlike RustBGPd (per-peer tokio
tasks where a stuck peer blocks only itself), ze's shared pool requires
active enforcement.

Two thresholds:

| Threshold | Trigger | Action |
|-----------|---------|--------|
| 1 (soft) | `PoolUsedRatio > 0.8` | Deny buffers to sources feeding the worst destination |
| 2 (hard) | `PoolUsedRatio > 0.95` AND worst peer > 2x weight share for grace period | Tear down worst destination peer (GR-aware) |

The grace period (default 5s, `ze.fwd.teardown.grace`) prevents teardown
on transient spikes. The check lives in `fwdBatchHandler`: when the write
deadline fires (TCP stuck, 30s), the worker checks pool state and its own
ratio. If Threshold 2 is met, the worker tears down its own session.

After teardown, all overflow handles return to the pool. The system
recovers immediately. If another peer becomes the new worst offender,
the cycle repeats.

<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdBatchHandler, fwdWorker -->

## Throttle the Culprit

Read throttling targets the source peers whose traffic is filling the pool.
The system cannot accelerate sending to slow destinations -- it can only
slow down reading from sources that are causing the pressure.

The culprit is identified by which source peer's traffic is
disproportionately consuming pool buffers. A source whose destinations are
all healthy (traffic flows through channels, not overflow) is not throttled
regardless of volume.

When the overflow pool runs low, the peer with the worst usage-to-weight
ratio is denied buffers first. This is the throttle: no buffer means no
read means TCP backpressure on the source.

## Pool Multiplexer

<!-- source: internal/component/bgp/reactor/bufmux.go -- BufMux, probedPool, withCollapseProbe -->
<!-- source: internal/component/bgp/reactor/session.go -- bufMux4K, bufMux64K -->

### Pool Multiplexer with Block-Backed Slices

The pool hands out a handle, not a raw `[]byte`. The handle carries the
block ID so returns are always routed to the correct block.

**Handle type:**

| Field | Type | Purpose |
|-------|------|---------|
| ID | uint16 | Block this buffer belongs to. Assigned at allocation, never changes. |
| Buf | []byte | The buffer (slice into backing array). Used for reads/writes. |

**Multiplexer operations:**

| Operation | What happens |
|-----------|-------------|
| `Get()` | Allocate from lowest block with free buffers. If none free, grow. Collapse triggered externally by `probedPool` probe. |
| `Return(handle)` | Route to `blocks[handle.ID]` free list. Increment block's free count. O(1). |
| `grow()` | Allocate new block (one backing array), assign next sequential ID, register with multiplexer. |
| `tryCollapse()` | If highest block fully returned AND block below has >=50% free: delete highest, repeat. |

**Allocation preference:** `Get()` takes from the lowest-numbered
block with free buffers. This consolidates steady-state traffic in
low blocks, letting higher blocks drain and become collapse candidates.

**Lazy collapse:** The `probedPool` wrapper fires a probe on every
`Get()`. The probe target's counter triggers `tryCollapse()` every N
calls (default 100). The check cascades downward, collapsing multiple
fully-returned blocks in one pass. This replaces timers and
shrink-on-return logic. The 50% free threshold on the block below
prevents oscillation (collapsing when the survivor is nearly full
would force an immediate regrow).

**Deterministic block freeing:** Each block tracks its total count and
free count. When `freeCount == totalCount`, every buffer is home. The
multiplexer can drop the block with certainty -- no guessing, no
stale-reference risk.

**Per-block free list:** Each block has its own free list (simple slice
of available buffer indices). No global free list. The block ID in the
handle routes returns to the correct list in O(1).

**Concurrency:** One mutex per multiplexer (not per block). The
multiplexer is used for all buffer allocation (normal and overflow
paths). A single mutex is acceptable -- `Get()` and `Return()` are
fast O(1) operations.

**Two multiplexer instances:**

| Instance | Buffer size | Purpose |
|----------|------------|---------|
| 4K multiplexer | 4096 | Reads (pre-Extended Message), UPDATE building |
| 64K multiplexer | 65535 | Reads (post-Extended Message) |

Build and read paths share the same 4K multiplexer (`bufMux4K`).

The overflow pool is not a separate pool. During congestion, the source
peer draws from the same 4K or 64K multiplexer -- the buffer just stays
in use longer (held in overflow backlog until destination drains it).

**Every callsite that currently passes `[]byte` passes a handle instead.**
The `Buf` field is used for TCP reads, wire writes, etc. The full handle
is passed to `Return()`.

### Combined Capacity Tracking

Buffer sizes are incompatible (can't hand a 4K buffer to a 64K reader),
so the two multiplexers maintain separate inventories. But memory
pressure is a shared resource -- growth, shrink, and backpressure
decisions use the combined usage across both multiplexers.

<!-- source: internal/component/bgp/reactor/bufmux.go — combinedBudget -->
<!-- source: internal/component/bgp/reactor/session.go — initBufMuxBudget, CombinedBufMuxStats -->

**Shared byte budget:** Both multiplexers share a `combinedBudget`
(atomic counter). Each mux increments the counter on block growth
and decrements on collapse. The budget check is lock-free -- no
cross-mux deadlock risk.

| Decision | Mechanism |
|----------|-----------|
| Grow (new block) | `combinedBudget.canGrow(blockBytes)` — denies if total allocated across both muxes would exceed `ze.fwd.pool.maxbytes` |
| Shrink (collapse) | Per-mux collapse (unchanged). Budget counter decremented on collapse via `recordCollapse`. |
| Backpressure (deny buffer) | `CombinedBufMuxUsedRatio()` — ratio of in-use bytes across both muxes. Available to overflow pool and metrics. |

**Configuration:** `ze.fwd.pool.maxbytes` sets the combined byte limit
(default 0 = unlimited). Phase 4 will set this dynamically based on
peer weights.

This prevents a scenario where the 64K multiplexer is 95% full (real
memory pressure) but the 4K multiplexer has headroom, and the system
fails to trigger backpressure because each looks at itself in isolation.

## Configuration

| Config key | Purpose | Default |
|-----------|---------|---------|
| `ze.fwd.pool.maxbytes` | Combined byte budget for 4K+64K pools | Auto-sized from peer weights |
| `ze.fwd.pool.headroom` | Extra memory beyond auto-sized baseline (e.g. "512MB", "2GB") | 0 (no extra) |
| `ze.fwd.pool.size` | Overflow pool token count (items) | Auto-sized from peer weights |
| `ze.fwd.chan.size` | Per-peer dispatch channel capacity | 64 |
| `ze.fwd.teardown.grace` | Seconds at >95% + >2x weight before forced teardown | 5s |
| Per-peer `prefix maximum` | Max expected prefixes (drives weight) | Required (PeeringDB fallback) |

When `ze.fwd.pool.maxbytes` is explicitly set, the operator's value
overrides auto-sizing entirely. When unset (0), the pool budget is
calculated dynamically from the peer set as described in Two-Tier
Pool Sizing.

`ze.fwd.pool.headroom` adds memory on top of the auto-sized baseline.
The auto-sized budget (guaranteed + overflow tiers) is the minimum
the system needs. Headroom gives extra room for congestion before
backpressure and teardown engage. Operators on machines with plenty
of RAM can set headroom to delay teardown decisions -- trading memory
for session stability. The total budget becomes
`auto_sized_baseline + headroom`. This does not affect `ze.fwd.pool.maxbytes`
when it is explicitly set (explicit overrides everything).

## Related Documents

- `docs/architecture/congestion-industry.md` -- industry survey of slow-peer handling
- `docs/architecture/buffer-architecture.md` -- buffer-first encoding architecture
- `docs/architecture/message-buffer-design.md` -- zero-copy message buffer design
- `plan/spec-forward-congestion.md` -- implementation spec with acceptance criteria
