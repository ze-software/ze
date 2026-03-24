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

## Asymmetric Allocation and Release

Growth and shrink use different granularities. Allocation is in large
blocks to reduce fragmentation. Release is per-buffer to return memory
promptly after a congestion spike subsides.

### Block Structure: One Backing Array Per 10% Step

Each 10% growth step is a single `make([]byte, N*bufSize)`. Individual
buffers are slices into this backing array -- no per-buffer allocation.

| Property | Value |
|----------|-------|
| Block backing | One `[]byte` allocation per 10% step |
| Buffer | Slice into backing array: `block[i*bufSize : (i+1)*bufSize]` |
| Slice header cost | 24 bytes (pointer + length + capacity) |
| GC objects at max pool | 10 (one per block), not millions of individual buffers |

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

### Growth: 10% Blocks

The pool does not pre-allocate its full maximum at startup. It grows in
10% block increments -- each block is one contiguous backing array.

| Pool state | Action |
|-----------|--------|
| Startup | Allocate first 10% block (permanent, never freed) |
| Usage reaches 90% of current allocation | Allocate another 10% block |
| Upper bound | 100% of maximum (sum of all peer weights) |

### Shrink: Per-Block on Return

When a buffer is returned, the block ID routes it to the correct block's
free list. The multiplexer then checks whether the block can be freed:

| Condition | Action |
|-----------|--------|
| Block is permanent (ID 0) | Always keep. Buffer goes to block's free list. |
| Block has all buffers free AND pool has >20% excess capacity | Drop the block: nil all refs, remove from multiplexer. GC frees backing array. |
| Block has outstanding buffers | Keep. Buffer goes to block's free list. |

Because each buffer carries its block ID, returns are always routed to
the correct block. No searching, no pointer comparison. When a block's
free count equals its total count, the multiplexer knows with certainty
that every buffer is home -- the block can be dropped safely.

The first block (ID 0) is permanent -- the multiplexer never drops it.
This provides a hot reserve that absorbs the next burst with zero
allocation latency.

### Example

Pool maximum is 1000 buffers. Startup allocates block 1 (100 buffers,
one backing array).

| Event | Blocks | In use | Free | Action |
|-------|--------|--------|------|--------|
| Startup | 1 (permanent) | 0 | 100 | Block 1 allocated |
| Burst starts | 1 | 90 | 10 | 90% full, allocate block 2 |
| Burst continues | 2 | 180 | 20 | 90% full, allocate block 3 |
| Burst peaks at 250 | 3 | 250 | 50 | Within capacity |
| Burst subsides | 3 | 180 | 120 | >20% free, returning slices nil'd |
| Block 3 fully drained | 2 | 150 | 50 | GC frees block 3 backing array |
| Block 2 fully drained | 1 | 30 | 70 | GC frees block 2 backing array |
| Settled | 1 (permanent) | 30 | 70 | Only block 1 remains |

After the spike, only the permanent first block remains. Block 2 and 3
backing arrays are collected by the GC once all their slices are nil'd.

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
| `Get()` | Pick preferred block (permanent first, then lowest-numbered with free buffers). Return handle. |
| `Return(handle)` | Route to `blocks[handle.ID].free(handle.Buf)`. O(1), no searching. |
| `Grow()` | Allocate new block (one backing array), assign next ID, register with multiplexer. |
| `Shrink(blockID)` | When block has all buffers free and pool has excess capacity, drop block. Nil all refs, remove from multiplexer. GC frees backing array. |

**Allocation preference:** `Get()` takes from the permanent block first
(ID 0), then the lowest-numbered block with free buffers. This keeps
pressure on higher-numbered blocks, making them candidates for shrink.

**Deterministic block freeing:** Each block tracks its total count and
free count. When `freeCount == totalCount`, every buffer is home. The
multiplexer can drop the block with certainty -- no guessing, no
stale-reference risk.

**Per-block free list:** Each block has its own free list (simple slice
of available buffer indices). No global free list. The block ID in the
handle routes returns to the correct list in O(1).

**Concurrency:** One mutex per multiplexer (not per block). The
multiplexer is only used during congestion (normal path uses per-peer
channels). Overflow is already the slow path -- a mutex is acceptable.
The fast path (channel send/receive) never touches the multiplexer.

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
