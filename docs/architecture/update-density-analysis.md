# BGP UPDATE Density Analysis

Empirical analysis of real-world BGP UPDATE traffic from public route collectors,
measuring NLRI density per message and per-peer burst patterns. These numbers
directly inform per-peer channel sizing in the forward pool.

<!-- source: cmd/ze-analyse/density.go — density analysis tool -->

## Data Source

RIPE RIS collector rrc00 (Amsterdam, multi-hop), 5-minute BGP4MP update file.
Date: 2026-03-24 00:00 UTC. 55 source peers, 300 seconds of continuous data.

Collected and analysed with:

```
ze-analyse download
ze-analyse density test/internet/ripe-updates.20260324.0000.gz
```

## NLRI Density: How Many Prefixes Per UPDATE

| NLRIs per UPDATE | Cumulative | Observation |
|-----------------|------------|-------------|
| 1 | 72% | The dominant case. Most UPDATEs carry a single prefix. |
| 1-2 | 83% | Two-prefix UPDATEs are the second most common. |
| 1-5 | 94% | Five or fewer covers nearly all messages. |
| 1-10 | 96% | Ten or fewer covers the vast majority. |
| 10+ | 4% | Batched convergence events. Rare but bursty. |

**Average: 3.2 NLRIs per UPDATE** (2.93 announced + 0.26 withdrawn).

### Implication for Channel Sizing

Since 72% of UPDATEs carry exactly 1 NLRI, the forward pool channel effectively
counts **updates**, not prefixes. There is no large multiplier: 1 UPDATE creates
roughly 1-3 items in the forward pool. The channel size in items (16, 32, 64)
maps nearly directly to UPDATEs absorbed.

<!-- source: cmd/ze-analyse/density.go — countUpdateNLRIs -->

## Setup vs Maintenance: Two Traffic Modes

BGP traffic from any peer operates in two distinct modes:

| Mode | Description | Share of Traffic | Per-Peer Rate |
|------|-------------|-----------------|---------------|
| **Setup** | Session establishment, full table dump, large convergence | 91% of updates | 10-30 UPD/s sustained, peaks to 960 UPD/s |
| **Maintenance** | Steady-state churn, individual route changes, flaps | 9% of updates | 1-5 UPD/s typical |

The per-peer channel only needs to handle **maintenance**. Setup traffic overflows
to the global pool by design.

### Classification Method

Each source peer (identified by peer_as in BGP4MP records) is tracked independently.
Consecutive seconds of updates from the same peer form a "run." Runs with more
than 100 total updates are classified as setup; fewer as maintenance.

Of the 55 source peers in this sample:
- 39 had setup bursts (sending full tables or convergence events)
- 16 were maintenance-only (steady-state churn)

<!-- source: cmd/ze-analyse/density.go — detectPeerRuns, printBurstAnalysis -->

## Maintenance Traffic Distribution

The per-peer per-second rate during maintenance periods:

| Percentile | Rate (UPD/s) | Observation |
|-----------|-------------|-------------|
| P50 | 3 | Half of maintenance seconds have 3 or fewer updates |
| P95 | 17 | 95th percentile includes occasional micro-bursts |
| P99 | 59 | 99th percentile: rare maintenance spikes |

**Distribution shape:** Most maintenance peer-seconds are at 1-5 UPD/s.
The distribution is heavily right-skewed: most seconds are quiet, with
occasional micro-bursts from route flaps or policy changes.

## Setup Traffic Distribution

The per-peer per-second rate during setup/convergence bursts:

| Percentile | Rate (UPD/s) | Observation |
|-----------|-------------|-------------|
| P50 | 11 | Median setup rate is modest |
| P75 | 19 | Most setup seconds are under 20 UPD/s |
| P95 | 28 | 95th percentile still under 30 |
| P99 | 68 | Rare peaks |
| Peak | 959 | Single-second maximum (one peer) |

Most setup traffic is a sustained stream of 10-30 UPD/s over many seconds
(full table trickle), not a single massive spike. The largest contributor
(AS202365) sent 38,682 updates over 271 seconds, averaging 143 UPD/s.

## Channel Sizing Conclusions

<!-- source: docs/architecture/forward-congestion-pool.md — Channel Size (Layer 1) -->

### The Channel Absorbs Maintenance Only

The per-peer channel is Layer 1 of the congestion response. It absorbs normal
maintenance churn so the overflow pool (Layer 2) is never touched during
steady-state operation. Setup/convergence bursts overflow by design.

### Empirical Channel Size Requirements

| Goal | Required Size | Based On |
|------|--------------|----------|
| Absorb P50 maintenance | 3-5 | Handles half of maintenance seconds |
| Absorb P95 maintenance | 17 | Handles 95% without overflow |
| Absorb P99 maintenance | 59 | Handles 99% without overflow |

### Recommended Tier Table

Cross-referencing the empirical data with the burst fraction model from
the forward congestion pool design:

| Peer weight (prefix maximum) | Channel size | Rationale |
|-----------------------------|-------------|-----------|
| 1-1,000 | 16 | P90 maintenance. Small peer, entire convergence may also fit. |
| 1,001-10,000 | 32 | P95 maintenance. Medium peer. |
| 10,001-100,000 | 64 | P99 maintenance. Headroom for micro-burst absorption. |
| 100,001-500,000 | 128 | Extra buffer. Convergence overflows regardless. |
| 500,001+ | 256 | Full table peer. Maximum channel. Setup always overflows. |

**Memory impact:** 1000 peers at the maximum (256 slots * ~64 bytes per fwdItem) =
16 MB. Negligible. Even the smallest tier (16 slots) is empirically sufficient
for 90% of maintenance seconds.

### Why Not a Larger Channel

A channel of 1024 would absorb more setup traffic, but this is counterproductive:
- The overflow pool provides **weighted access** and **backpressure signalling**
- A large channel delays congestion detection
- Setup traffic must reach the overflow pool for the system to track it
- The channel is a fast-path optimization, not a capacity buffer

## Reproducing This Analysis

Download fresh data and run the analysis:

```
ze-analyse download                                    # fetch latest data
ze-analyse density test/internet/ripe-updates.*.gz     # analyse updates
```

For a longer observation window, download multiple 5-minute files:

```
ze-analyse download 20260324 0000
ze-analyse download 20260324 0005
ze-analyse download 20260324 0010
ze-analyse density test/internet/ripe-updates.*.gz
```

RouteViews data can also be used (15-minute intervals, fewer peers):

```
ze-analyse density test/internet/rv-updates.*.gz
```

## Related

- [Forward Congestion Pool Design](forward-congestion-pool.md) -- channel sizing
  tiers and burst fraction model that this analysis validates
- [Congestion Industry Survey](congestion-industry.md) -- how BIRD, GoBGP, FRR,
  and others handle backpressure (none have empirical sizing data)
- [Buffer Architecture](buffer-architecture.md) -- BufMux block-backed pools
