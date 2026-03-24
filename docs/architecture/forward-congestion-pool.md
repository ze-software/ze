# Forward Congestion Pool Design

How ze handles slow destination peers without dropping routes or consuming
unbounded memory. This document records the design decisions for the overflow
pool, buffer ownership, weighted access, and backpressure.

<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdOverflowPool, fwdItem -->
<!-- source: internal/component/bgp/reactor/session.go -- readBufPool4K, readBufPool64K, getReadBuffer -->

## Invariant

**Routes are never dropped.** If the system cannot deliver a route to a peer,
it must either buffer the route or tear down the session. Silent discard is
never acceptable.

## Four-Layer Congestion Response

| Layer | Mechanism | Trigger |
|-------|-----------|---------|
| 1 | Per-peer channel buffer (existing, 64 items) | Absorbs micro-bursts |
| 2 | Global overflow pool with weighted access | Channel full |
| 3 | Buffer denial on culprit source peers (natural TCP backpressure) | Pool filling |
| 4 | Session teardown (GR-aware, last resort) | Pool exhausted, backpressure insufficient |

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

### Channel Size (Layer 1)

The per-peer channel absorbs micro-bursts (milliseconds of updates).
Its size is derived from the peer's weight (burst-adjusted prefix count),
scaled down because the channel only needs to cover the time between
drain cycles.

| Peer weight (burst-adjusted) | Channel size | Reasoning |
|-----------------------------|-------------|-----------|
| 1 - 99 | 16 | Small peer, small bursts. Floor prevents starvation. |
| 100 - 999 | 32 | Medium peer |
| 1,000 - 9,999 | 64 | Current default, fits most peers |
| 10,000 - 99,999 | 128 | Large peer, bigger micro-bursts |
| 100,000+ | 256 | Full table peer, convergence events touch many prefixes |

The channel is the fast path (no weighted access check, no fairness
tracking). It should be large enough that normal operation rarely hits
the overflow path, but small enough that the overflow pool is the
primary buffer during real congestion.

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

### Shrink: Lazy Collapse on Get()

Blocks are not freed on Return(). Instead, every 100th `Get()` call
runs a collapse check on the highest block:

| Condition | Action |
|-----------|--------|
| Highest block fully returned AND block below has >=50% free | Delete highest block, repeat check on new highest |
| Highest block has outstanding buffers | No collapse |
| Highest block fully returned but block below <50% free | No collapse (would need to regrow immediately) |
| Only one block exists | No collapse (nothing to collapse into) |

The collapse cascades: if blocks 2, 1, 0 are all fully returned, one
check pass deletes them top-down until only the block with active
allocations remains.

**Why on Get(), not Return():** After a burst subsides, the overflow
path stops receiving `Get()` calls -- traffic is normal. But the normal
read path calls `Get()` on the same multiplexer for every BGP message
received from the network. Every peer session reads from TCP into a
buffer obtained from the mux. This constant activity drives the collapse
check: every 100th network message read triggers a check on whether
overflow blocks can be freed. No timer needed -- network traffic is
the heartbeat. One atomic counter increment per `Get()`, one collapse
check per 100 -- negligible cost.

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

<!-- source: internal/component/bgp/reactor/session.go -- readBufPool4K, readBufPool64K, buildBufPool -->

### Current: sync.Pool (to be replaced)

Three `sync.Pool` instances with per-buffer `make()` in `New`:

| Pool | Buffer size | Purpose |
|------|------------|---------|
| `readBufPool4K` | 4096 | TCP reads (pre-Extended Message) |
| `readBufPool64K` | 65535 | TCP reads (post-Extended Message) |
| `buildBufPool` | 4096 | Building UPDATE attributes (outbound) |

`buildBufPool` is the same size as `readBufPool4K` -- these merge into
one 4K instance. Three pools become two (4K and 64K).

`sync.Pool` cannot route returns to the correct block. It is an unordered
free list -- `Get()` returns an arbitrary item, and there is no way to
preferentially drain a specific block. The GC can also evict entries
between cycles, which prevents deterministic block freeing.

### New: Pool Multiplexer with Block-Backed Slices

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
| `Get()` | Allocate from lowest block with free buffers. If none free, grow. Every 100th call, run collapse check first. |
| `Return(handle)` | Route to `blocks[handle.ID]` free list. Increment block's free count. O(1). |
| `grow()` | Allocate new block (one backing array), assign next sequential ID, register with multiplexer. |
| `tryCollapse()` | If highest block fully returned AND block below has >=50% free: delete highest, repeat. |

**Allocation preference:** `Get()` takes from the lowest-numbered
block with free buffers. This consolidates steady-state traffic in
low blocks, letting higher blocks drain and become collapse candidates.

**Lazy collapse:** Every 100th `Get()` call checks whether the highest
block can be deleted. The check cascades downward, collapsing multiple
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

`buildBufPool` is eliminated -- reads and builds draw from the same
4K multiplexer.

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

| Decision | Input |
|----------|-------|
| Grow (allocate another 10% block) | Combined usage of 4K + 64K > 90% of combined allocation |
| Shrink (drop empty blocks) | Combined free across 4K + 64K > 20% of combined allocation |
| Backpressure (deny buffer) | Combined usage-to-weight ratio |

This prevents a scenario where the 64K multiplexer is 95% full (real
memory pressure) but the 4K multiplexer has headroom, and the system
fails to trigger backpressure because each looks at itself in isolation.

## Configuration

| Config key | Purpose | Default |
|-----------|---------|---------|
| `ze.fwd.pool.size` | Explicit total pool size override | Auto-sized from peer weights |
| Per-peer `prefix maximum` | Max expected prefixes (drives weight) | Required (PeeringDB fallback) |

## Related Documents

- `docs/architecture/congestion-industry.md` -- industry survey of slow-peer handling
- `docs/architecture/buffer-architecture.md` -- buffer-first encoding architecture
- `docs/architecture/message-buffer-design.md` -- zero-copy message buffer design
- `plan/spec-forward-congestion.md` -- implementation spec with acceptance criteria
