# Spec: forward-congestion

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `internal/component/bgp/reactor/forward_pool.go` - forward pool
3. `internal/component/bgp/reactor/session_write.go` - writeMu, write deadlines
4. `internal/component/bgp/reactor/session.go` - keepalive/hold timers
5. `rfc/short/rfc2918.md` - Route Refresh
6. `rfc/short/rfc4724.md` - Graceful Restart

## Task

The forward pool's committed code drops BGP UPDATE messages when a destination
peer's overflow buffer exceeds 256 items. Dropping routes is a correctness bug:
the peer's RIB silently diverges with no recovery mechanism.

**Invariant: routes are never dropped.** If the system cannot deliver a route
to a peer, it must either buffer the route or tear down the session. Silent
discard is never acceptable.

## The Problem

A single slow destination peer (TCP buffer full, remote not reading) causes:

| Without handling | Consequence |
|-----------------|-------------|
| Committed code (drop at 256) | Silent routing inconsistency, peer RIB diverges permanently |
| Working tree (unbounded overflow) | Single slow peer causes OOM, kills all peers |
| Blocking dispatch (original) | One slow peer blocks forwarding to all peers |

None of these are acceptable. The design must bound memory, preserve routes,
and isolate slow peers from fast ones.

## Industry Context

Research into existing BGP implementations reveals:

| Implementation | Approach to slow peers |
|---------------|----------------------|
| BIRD 3 (IXP route servers) | Unbounded bucket queue per peer (no backpressure beyond TCP flow control). Route superseding deduplicates pending updates for same prefix. Hold timer congestion extension: if RX data pending when hold timer fires, extend 10s instead of teardown. TX budget 1024 messages per event loop cycle. Channel fairness: round-robin with 16-message stickiness across AFI/SAFI. fast_rx during handshake for OPEN/KEEPALIVE priority. |
| GoBGP | Unbounded InfiniteChannel per peer (eapache/channels). No backpressure. Write failure is fatal (kills peer). One write() syscall per UPDATE. No TCP_NODELAY. Sets IP_TOS/DSCP via DialerControl + listenControl. TCP keepalive disabled. 1-second write deadline during handshake, none in ESTABLISHED. No Send Hold Timer. No buffer pooling (per-read malloc). |
| RustBGPd | Unbounded mpsc channels from table threads to peer tasks. No write timeout (write_all blocks forever on stuck peer). 4-phase write priority: urgent > withdrawals > announcements > EOR. TX budget max_tx_count=2048 per iteration. SO_SNDBUF-aware TX buffer: min(64KB, SO_SNDBUF/2). SO_REUSEPORT on listener. No TCP_NODELAY, no IP_TOS/DSCP, no Send Hold Timer. Tokio async runtime with per-peer tasks. |
| FRRouting | Dedicated keepalive thread. Write quanta limit per I/O cycle. Configurable SO_SNDBUF. Implements RFC 9687 Send Hold Timer. |
| OpenBGPd | Internal msgbuf queuing on EAGAIN. Keepalive on independent timer. Implements RFC 9687. |
| Cisco IOS-XR | Update groups with slow-peer detection and dynamic isolation into "refresh sub-groups." 32 MB per sub-group cache. |
| Juniper Junos | 17 prioritized output queues. Keepalive generation in dedicated kernel thread. Per-prefix out-delay rate limiting. |
| Nokia (Alcatel-Lucent) | Uses TCP receive window zero as deliberate backpressure. Known to break peers that don't handle it. |

### BIRD Source Code Analysis (2026-03-23)

Detailed analysis of BIRD source code (gitlab.nic.cz/labs/bird, master branch).

**Socket options BIRD sets on BGP connections:**

| Option | Value | File / Function | Purpose |
|--------|-------|----------------|---------|
| IP_TOS (IPv4) | 0xC0 (DSCP CS6, Internet Control) | bgp.c:229,1827 via io.c:sk_set_tos4() | Network-level QoS priority for routing protocol traffic |
| IPV6_TCLASS (IPv6) | 0xC0 (DSCP CS6) | bgp.c:229,1827 via io.c:sk_set_tos6() | Same as above for IPv6 |
| O_NONBLOCK | all sockets | io.c:1141 | Non-blocking I/O for event loop |
| SO_REUSEADDR | listener only | io.c:1666 | Allow quick listener restart |
| TCP_MD5SIG | when configured | sysio.h:511-552 | RFC 2385 authentication |
| TCP-AO | when configured | sysio.h:175-502 | RFC 5925 authentication |
| IP_MINTTL / IPV6_MINHOPCOUNT | when ttl_security configured | sysio.h:555-580 | GTSM (RFC 5082) |
| IP_FREEBIND / IPV6_FREEBIND | when free_bind configured | sysio.h:616-629 | Bind to not-yet-assigned addresses |
| SO_BINDTODEVICE | when interface configured | io.c:1168,1178 | VRF binding |

**Socket options BIRD does NOT set on BGP connections:**

| Option | Status | Notes |
|--------|--------|-------|
| TCP_NODELAY | Not set | Nagle enabled. Relies on kernel coalescing of one-message-per-write pattern |
| TCP_CORK / MSG_MORE | Not used | No explicit write batching |
| SO_SNDBUF | Not set | Kernel default |
| SO_RCVBUF | Not set on TCP | Only set on UDP/IP sockets (io.c:1247) |
| SO_KEEPALIVE | Not used | App-layer timers only |
| SO_PRIORITY | Available but not applied | sk_priority_control=7 exists but BGP sockets don't set priority field |

**BIRD TX path:**

| Property | Detail |
|----------|--------|
| TX buffer | Single buffer per connection: 4096 bytes (standard) or 65535 (extended messages, RFC 8654) |
| Messages per write | ONE per sk_send() call. No multi-message batching. |
| TX budget | 1024 messages per bgp_kick_tx()/bgp_tx() invocation |
| When send buffer full | write() returns EAGAIN, loop exits, resumes on POLLOUT |
| Pending update queue | Unbounded linked list of bgp_bucket structs (no limit, no dropping) |
| Route superseding | Prefix hash deduplicates: new route for already-queued prefix moves to new bucket |

**BIRD congestion handling:**

| Mechanism | Detail |
|-----------|--------|
| Backpressure | None explicit. TCP flow control only. Bucket queue grows unbounded. |
| Hold timer extension | If RX data pending when hold timer fires, extend 10s (CPU congestion detection, bgp.c:1712-1716) |
| Send Hold Timer | RFC 9687. Default 2x hold_time. Reset on every successful sk_send(). Hard disconnect on expiry (no NOTIFICATION). |
| Channel fairness | Round-robin with 16-message stickiness across AFI/SAFI (packets.c:3063-3089) |
| fast_rx mode | ON during OPEN/OPENCONFIRM, OFF in ESTABLISHED. Fast sockets get priority reads (up to 4 per poll cycle) |
| Event loop steps | MAX_STEPS=4 per socket per poll cycle |

**Key takeaway for ze:** BIRD's bucket queue is unbounded with no backpressure --
it relies entirely on TCP flow control and route superseding to manage slow peers.
Ze's four-layer design is strictly more capable. The techniques worth adopting are:
hold timer congestion extension, TX budget limiting, channel fairness, IP_TOS marking,
and route superseding in the overflow pool.

### GoBGP Source Code Analysis (2026-03-23)

Detailed analysis of GoBGP source code (github.com/osrg/gobgp, local copy).

**Socket options GoBGP sets on BGP connections:**

| Option | Value | File / Function | Purpose |
|--------|-------|----------------|---------|
| IP_TOS (IPv4) | Configurable via peer config | sockopt_linux.go:215 via DialerControl() | DSCP marking |
| IPV6_TCLASS (IPv6) | Configurable via peer config | sockopt_linux.go:218 via DialerControl() | Same for IPv6 |
| TCP_MD5SIG | When password configured | sockopt_linux.go:67-90 | RFC 2385 authentication |
| IP_TTL / IPV6_UNICAST_HOPS | Configurable | sockopt_linux.go:162-173 | TTL setting |
| IP_MINTTL / IPV6_MINHOPCOUNT | When configured | sockopt_linux.go:179-184 | GTSM (RFC 5082) |
| TCP_MAXSEG | When configured | sockopt_linux.go:193-204 | MSS clamping |
| SO_BINDTODEVICE | When bind interface set | sockopt_linux.go:206-209 | VRF binding |
| TCP KeepAlive | Disabled (KeepAlive: -1) | fsm.go:929 | BGP has own keepalives |
| Listener IP_TTL | 255 (always) | listener.go:83-90 | TTL security for inbound |
| MPTCP | Disabled | listener.go:152 | Incompatible with TCP MD5 in Go 1.24+ |

**Socket options GoBGP does NOT set:**

| Option | Notes |
|--------|-------|
| TCP_NODELAY | Not set. Nagle left at OS default. |
| SO_SNDBUF / SO_RCVBUF | Not set. OS defaults. |
| TCP_CORK / MSG_MORE | Not used |
| SO_PRIORITY | Not set |
| SO_REUSEPORT | Not set |

**GoBGP TX path:**

| Property | Detail |
|----------|--------|
| Outgoing queue | InfiniteChannel (eapache/channels) -- unbounded, never drops |
| Messages per write | ONE per conn.Write() call. No batching. |
| Write deadline | 1 second during handshake; none in ESTABLISHED |
| Write failure | Fatal -- kills peer immediately, no retry |
| No bufio | Direct io.ReadFull() per message, make([]byte, length) per read |
| UPDATE grouping | CreateUpdateMsgFromPaths() groups paths by family, but each UPDATE is a separate write |

**GoBGP congestion handling:**

| Mechanism | Detail |
|-----------|--------|
| Backpressure | None. InfiniteChannel accepts all. RIB has no way to know peer is congested. |
| Send Hold Timer | Not implemented. Stuck write blocks sendMessageloop forever. |
| Rate limiting | None |
| Slow-peer detection | None |
| Route superseding | Not in output queue (InfiniteChannel is FIFO, no dedup) |

**Key takeaway for ze:** GoBGP confirms IP_TOS/DSCP is standard practice (ze now does
this). GoBGP's lack of TCP_NODELAY, buffer pooling, and write batching are performance
gaps ze already avoids. GoBGP's InfiniteChannel is functionally identical to BIRD's
unbounded bucket queue -- no implementation has solved backpressure.

### RustBGPd Source Code Analysis (2026-03-23)

Detailed analysis of RustBGPd source code (github.com/osrg/rustybgp, local copy).

**Socket options RustBGPd sets on BGP connections:**

| Option | Value | File / Function | Purpose |
|--------|-------|----------------|---------|
| SO_REUSEADDR | true | event.rs:2458 | Quick listener restart |
| SO_REUSEPORT | true | event.rs:2459 | Load-balanced accept across threads |
| IPV6_V6ONLY | true (IPv6 listener) | event.rs:2455 | Separate IPv4/IPv6 listeners |
| TCP_MD5SIG | When password configured | auth.rs:58-71 | RFC 2385 (Linux only) |
| O_NONBLOCK | all sockets | event.rs:2460 | Tokio async I/O |

**Socket options RustBGPd does NOT set:**

| Option | Notes |
|--------|-------|
| TCP_NODELAY | Not set |
| IP_TOS / IPV6_TCLASS | Not set. No DSCP marking. |
| SO_SNDBUF | Reads but never sets. TX buffer sized as min(64KB, SO_SNDBUF/2). |
| SO_RCVBUF | Not set. RX buffer fixed at 64KB. |
| TCP_CORK / MSG_MORE | Not used |
| SO_PRIORITY | Not set |
| SO_KEEPALIVE | Not set |
| IP_MINTTL | Not set (no GTSM support) |

**RustBGPd TX path (most sophisticated of the open-source implementations):**

| Property | Detail |
|----------|--------|
| Write phases | 4-phase priority: (1) urgent (OPEN/KEEPALIVE/NOTIFICATION) > (2) withdrawals > (3) announcements > (4) EOR |
| TX budget | max_tx_count=2048 attribute records per event loop iteration |
| TX buffer sizing | min(64KB, SO_SNDBUF/2) -- auto-adapts to actual socket buffer |
| Write timeout | NONE. write_all().await blocks forever on stuck peer. |
| Outgoing channels | Unbounded mpsc from table threads to peer tasks |
| Runtime | Tokio async with per-peer tasks |
| Attribute grouping | PendingTx deduplicates by attribute set (routes sharing attributes batched into one UPDATE) |

**RustBGPd congestion handling:**

| Mechanism | Detail |
|-----------|--------|
| Backpressure | None. Unbounded channels from table threads. |
| Send Hold Timer | Not implemented. Stuck write_all blocks peer task indefinitely. |
| Rate limiting | TX budget (2048) is fairness, not rate limiting |
| Slow-peer detection | None. Peer task blocks on write, queues grow unbounded. |
| Route superseding | PendingTx deduplicates by prefix within a batch, but channel queue is not deduped |

**Key takeaway for ze:** RustBGPd's 4-phase write priority and SO_SNDBUF-aware
buffer sizing are worth studying. The TX budget (2048 per iteration) is similar
to BIRD's (1024) and validates ze's planned TX budget limiting (AC-24). RustBGPd's
critical weakness is no write timeout -- a stuck peer blocks the entire task with
no recovery. Ze's RFC 9687 Send Hold Timer addresses exactly this gap.

### Cross-Implementation Comparison (2026-03-23)

| Feature | BIRD | GoBGP | RustBGPd | FRR | Ze (planned) |
|---------|------|-------|----------|-----|-------------|
| TCP_NODELAY | No | No | No | Yes | **Done** |
| IP_TOS/DSCP CS6 | Yes (0xC0) | Yes (configurable) | No | Yes | **Done** |
| TCP_MD5SIG | Yes | Yes (Linux) | Yes (Linux) | Yes | Yes |
| GTSM (IP_MINTTL) | Yes | Yes | No | Yes | Yes |
| TCP-AO (RFC 5925) | Yes | No | No | Partial | Not yet |
| SO_SNDBUF tuning | No | No | Reads, doesn't set | Yes (configurable) | Planned |
| Send Hold Timer (RFC 9687) | Yes | **No** | **No** | Yes | Planned |
| Write timeout | N/A (non-blocking) | None in ESTABLISHED | **None** | Dedicated thread | Planned |
| Backpressure | None (unbounded queue) | None (InfiniteChannel) | None (unbounded mpsc) | Write quanta | **4-layer design** |
| Route superseding | Yes (prefix hash) | No | Partial (per-batch) | Yes | Planned |
| TX budget | 1024 msgs/cycle | None | 2048 attrs/iteration | Write quanta | Planned |
| Hold timer congestion ext. | Yes (10s if RX pending) | No | No | No | Planned |
| Write priority phases | No (fixed priority) | No | Yes (4-phase) | No | Planned |
| Buffer pooling | No (malloc per msg) | No (malloc per read) | No (BytesMut) | No | **Yes (pool)** |
| bufio batching | No | No | App-level buffering | No | **Yes (16KB)** |

**Industry-wide gaps ze addresses:**
- No open-source BGP daemon has real backpressure (ze: 4-layer design)
- Only BIRD/FRR/OpenBGPd implement RFC 9687 (ze: planned)
- No implementation combines TCP_NODELAY + IP_TOS + buffer pooling + bufio (ze: done)
- Only BIRD has hold timer congestion extension (ze: planned, AC-22)

**Universal industry rule:** Never drop routes silently. Every major implementation
chooses memory growth or backpressure over route loss.

**RFC 9687 (Send Hold Timer, November 2024):** Directly addresses the "stuck peer"
problem. If ze cannot send ANY message to a peer for a configured duration
(recommended: max(8 minutes, 2x HoldTime)), tear down the session. Sends
NOTIFICATION Error Code 8 "Send Hold Timer Expired." Implemented by FRR, OpenBGPd,
BIRD. Ze should implement this as a safety net independent of congestion handling.

**TCP window zero is a known vendor technique.** Nokia/Alcatel-Lucent routers use
it deliberately as backpressure. It broke ExaBGP and was documented to the community.
Ben Cox (2021) demonstrated that holding window zero without release creates "BGP
zombies" -- sessions that appear alive but cannot deliver withdrawals, causing stale
routes in the DFZ. Every tested implementation (BIRD, Cisco, Junos, Arista, FRR)
hangs without tearing down.

**TCP_NODELAY:** ~~Ze should set TCP_NODELAY on all peer sockets.~~ Done (commit
1c43e11d). Set in connectionEstablished() on all production BGP sessions.

**IP_TOS / DSCP CS6 (0xC0):** BIRD sets IP_PREC_INTERNET_CONTROL on all BGP sockets
(both listener and outgoing). RFC 4271 Section 5.1 recommends IP precedence for BGP.
DSCP CS6 tells network devices to prioritize BGP traffic over regular data traffic.
Under network congestion, routers with QoS policies will preferentially forward BGP
packets, reducing the chance of hold timer expiry due to packet loss. Ze should set
IP_TOS on all peer sockets -- both dialer (via Control callback) and accepted connections.

**Hold timer congestion extension (BIRD technique):** When the hold timer expires,
check if there is unread data in the kernel receive buffer (via poll with zero timeout
or syscall). If data is pending, the daemon is CPU-congested, not the peer -- extend
the hold timer by 10 seconds instead of tearing down. This prevents false hold timer
expirations when the event loop is overloaded processing a burst of UPDATEs from
other peers.

**Route superseding in pending queue (BIRD technique):** When a new route arrives
for a destination already queued for sending, replace the old entry instead of
appending. This bounds queue growth to the number of unique prefixes rather than the
number of updates. Ze's overflow pool should do the same -- a new update for an
already-queued prefix should replace, not append.

## Implementation Phases

| Phase | Name | ACs | Status |
|-------|------|-----|--------|
| 1 | Foundation: overflow pool, metrics, socket options | AC-1, AC-7, AC-11, AC-15, AC-16, AC-17, AC-18, AC-21 | Done |
| 2 | Safety: write deadline, Send Hold Timer, hold timer extension | AC-8, AC-14, AC-22 | Done |
| 3 | Pool multiplexer: replace sync.Pool, block-backed handles | AC-26, AC-27, AC-7 (enhanced) | Done |
| 4 | Weight + dynamic sizing: burst fraction, pool auto-sizing, TX budget | AC-28, AC-10, AC-19, AC-20, AC-24 | Done |
| 5 | Backpressure + teardown: buffer denial, GR-aware teardown | AC-2, AC-3, AC-4, AC-5, AC-6, AC-9, AC-12 | Done |

~~Phase 3 is independent -- can start now.~~
~~Phase 4 depends on `spec-prefix maximum` for the weight source.~~
~~Phase 5 depends on Phase 4 (needs weights for buffer denial decisions).~~

**Update (2026-03-27): All phases complete.**
- Phase 4 dependency on `spec-prefix maximum` is resolved: prefix-limit is fully implemented (learned summaries 413, 415, 429). PrefixMaximum values are wired into the weight tracker via `reactor_peers.go:AddPeer()`.
- Phase 3 ReadThrottle (sleep-based) was superseded and cancelled. Buffer denial (Phase 5) is the backpressure mechanism.
- Phase 5 implemented: `congestionController` type in `forward_pool_congestion.go`. `ShouldDeny()` wired into `DispatchOverflow`, `CheckTeardown()` wired into `runWorker`. GR-aware teardown via `congestionTeardownPeer()`. 11 unit tests. Pre-existing `updateBufMuxBudget` race fixed.

## Design: Four-Layer Congestion Response

Routes are never dropped. Congestion is handled by four escalating layers.
Each layer activates only when the previous layer is insufficient.

### Layer 1: Channel Buffer (dynamic sizing)

Per-destination-peer buffered channel. Absorbs micro-bursts. ~~Default 64 items.~~
Channel size is dynamic, derived from the peer's weight (burst-adjusted prefix
count). See `docs/architecture/forward-congestion-pool.md` for the sizing table.

### Layer 2: Global Overflow Pool with Weighted Access

Full design: `docs/architecture/forward-congestion-pool.md`

A global buffer pool, lazily allocated in 10% steps, shared by all peers.
During congestion, the overflow pool provides read buffers directly to source
peers -- the buffer is allocated from the pool, TCP reads into it, and it
stays in the pool until the destination drains it (zero-copy by construction).

| Property | Value |
|----------|-------|
| Growth | Block allocation on exhaustion: one contiguous backing array per block |
| Growth trigger | `Get()` finds no free buffer in any existing block |
| Shrink | Lazy collapse: every 100th `Get()`, delete highest block if fully returned and block below has >=50% free |
| Permanent floor | None -- all blocks are freeable. "Hot reserve" is whichever block is currently active. |
| Maximum | Sum of all peer weights (prefix counts), or explicit `ze.fwd.pool.size` |
| Allocation order | Lowest block first (consolidates steady-state in low blocks, higher blocks drain and collapse) |
| Scope | Global -- all peers draw from the same pool |
| Fairness | Weighted access: priority diminishes with usage-to-weight ratio |
| Buffer role | Pool provides read buffers during congestion (not local read pool) |
| Backpressure | Buffer exhaustion IS the backpressure -- no buffer means no read |
| When exhausted | Source peers cannot read (natural backpressure), then Layer 4 (teardown) |

~~**Why pre-allocate:** `append()` to unbounded slices means the memory bound
is theoretical. Pre-allocation makes the bound real -- the memory is committed
at startup, and the system knows exactly how much it has. No GC pressure spikes
during congestion.~~
~~**Superseded (2026-03-23):** Asymmetric allocation/release replaces full pre-allocation.
Growth in 10% contiguous blocks, shrink per-buffer on return. First 10% block permanent.
Memory tracks actual congestion, not theoretical worst case.~~
**Superseded (2026-03-24):** Three simple rules replace thresholds. Allocate from highest
block. Grow on exhaustion (not at 90%). Lazy collapse every 100th Get() (not on Return).
No permanent block. See `docs/architecture/forward-congestion-pool.md`.

#### Buffer Sizing: Data-Driven

A fixed buffer size (256, 10000, or any constant) is wrong because it ignores
the actual routing table. A peer announcing 1M prefixes can have a convergence
event touching 10% of them -- that is 100K updates in a burst. The buffer must
be sized to the real workload, not an arbitrary number.

**Sizing sources (in priority order):**

| Priority | Source | What it provides | When available |
|----------|--------|-----------------|----------------|
| 1 | Configured prefix maximum per peer | Operator intent -- "I expect at most N prefixes from this peer" | Always (if configured) |
| 2 | Previous run data (zefs) | Actual observed prefix count from last session with this peer | At startup |
| 3 | Local RIB (adj-rib-in) | Actual prefix count from current session | After session established |
| 4 | Compiled-in PeeringDB/routing table | Per-ASN prefix count snapshot from build time | Always (fallback) |
| 5 | PeeringDB public API refresh | Updated per-ASN data for unknown or stale ASNs | On-demand background |

#### Prefix-Limit (pre-requisite spec: spec-prefix maximum)

**Prefix-limit is mandatory for every peer.** No peer runs without one.
Like Junos `prefix maximum`, it does double duty:

| Purpose | How prefix maximum helps |
|---------|----------------------|
| Safety | Tear down session if peer exceeds limit (prevents route leaks, misconfig) |
| Buffer sizing | Directly tells the congestion system the expected workload per peer |

**Two ways to set the value:**

| Mode | Config syntax | Behavior |
|------|-------------|----------|
| Explicit | `prefix maximum 100000;` | Operator sets the value. Hard enforcement. |
| PeeringDB lookup | `prefix maximum peeringdb;` | Ze resolves the value NOW: queries compiled-in data or PeeringDB API, gets the number, and writes the actual number into the config. Config ends up with e.g. `prefix maximum 95000;` |

`peeringdb` is a one-time resolution keyword, not a persistent mode.
The config always contains a concrete number after resolution. If the
compiled-in data doesn't have the ASN, ze queries the PeeringDB public
API dynamically to get it.

This means:
- The operator can type `prefix maximum peeringdb;` in the CLI or config
- Ze resolves it immediately to a real number
- The saved config has the resolved number, not the keyword
- Next time ze reads the config, it sees a normal integer
- The advisory system (below) will tell the operator if the number
  becomes too tight over time

**Prefix-limit advisory system:**

The limit protects against route leaks and misconfig, but it must not
be so tight that normal routing table growth triggers a session teardown.
Ze tracks whether a peer is approaching its limit and advises the operator.

| Observed prefix count | What ze does |
|----------------------|-------------|
| Below 90% of limit | Normal operation |
| Reaches 90% of limit | Records "hot" event to zefs: peer, timestamp, observed count, current limit |
| Reaches limit | Session enforcement (teardown per spec-prefix maximum) |

**Ze never auto-changes the running prefix maximum.** The limit is a safety
mechanism -- silently raising it could mask a route leak. Instead:

On next startup or config edit, ze reports recommendations:

| Report | Example |
|--------|---------|
| Peer approaching limit | "peer 10.0.0.1 (AS65001): prefix maximum 100000, observed peak 92347 on 2026-03-20. Suggest raising to 102000." |
| Peer well below limit | "peer 10.0.0.2 (AS65002): prefix maximum 500000, observed peak 12400. Limit may be unnecessarily high." |
| Auto-derived value stale | "peer 10.0.0.3 (AS65003): prefix maximum auto (PeeringDB: 45000, compiled 2026-01-15). Consider setting explicit value based on observed peak 51200." |

This is advisory only. The operator decides. The report appears:
- On `ze bgp status` or similar CLI command
- In logs at startup if any peer has a "hot" record in zefs
- Via Prometheus metric (`ze_bgp_prefix_limit_headroom_ratio` per peer)

**Prefix-limit is a separate spec** (`plan/spec-prefix maximum.md`) because
it involves BGP config, YANG schema, session enforcement, NOTIFICATION
generation, and the advisory system. This congestion spec CONSUMES the
prefix maximum value for buffer sizing but does not implement it.

#### Previous Run Data (zefs)

On session teardown or periodic flush, ze records per-peer observed data
to `database.zefs`:

| Field | Value |
|-------|-------|
| Peer address | IP |
| Peer ASN | uint32 |
| Prefix count | Max observed prefix count during session |
| Last updated | Timestamp |

On next startup, this data is loaded and used for buffer sizing before
any session establishes. The previous run's actual prefix count is
better than any external estimate.

#### Embedded + Cached Network Size Data

A build-time code generator analyzes PeeringDB data and public routing
table snapshots (RIPE RIS, RouteViews) to produce a compiled-in table
of per-ASN expected prefix counts. This table ships inside the ze binary.

| Component | Purpose |
|-----------|---------|
| Build-time generator | Fetches PeeringDB + routing table snapshots, produces Go source with per-ASN prefix count map |
| Compiled-in table | Embedded in binary. Available at startup with zero network access. |
| zefs persistence | Previous-run data and refreshed PeeringDB data written to `database.zefs` |
| PeeringDB refresh | When compiled-in + zefs data is older than threshold, background query updates for stale/missing ASNs |
| Authenticated PeeringDB | For ASNs not in public data, user configures credentials. Queries only for missing ASNs. |

**Data flow:**

1. At startup: load compiled-in table (always present)
2. Overlay zefs data (previous run observations + any prior PeeringDB refreshes)
3. For each configured peer: use prefix maximum if set, else zefs observation, else compiled-in/PeeringDB
4. If ASN missing from all sources AND PeeringDB refresh enabled: queue background query
5. On session establishment: actual RIB count becomes the live source
6. On session teardown / periodically: write observed prefix counts to zefs
7. If data age exceeds threshold: background PeeringDB refresh for stale ASNs

**Staleness handling:** The compiled-in data gets stale as the binary ages.
The zefs cache extends its life with real observations from previous runs.
PeeringDB refresh is the last resort for unknown ASNs. Even stale data
(within months) is far better than a fixed constant -- the DFZ grows
slowly and per-ASN proportions are relatively stable.

**Auto-sizing algorithm:**

1. At startup, allocate pool based on configured size or available memory
2. For each peer, determine expected prefix count (priority order above)
3. Per-peer buffer share = proportional to expected prefix count
4. A peer with prefix maximum 500K gets a proportionally larger share than
   one with prefix maximum 1K
5. On session establishment, refine share using actual RIB count
6. If actual count exceeds prefix maximum: session enforcement handles it
   (separate spec)

**Configuration:**

| Config key | Purpose | Default |
|-----------|---------|---------|
| `ze.fwd.pool.maxbytes` | Combined byte budget for 4K+64K pools (explicit override) | Auto-sized from peer weights |
| `ze.fwd.pool.headroom` | Extra memory beyond auto-sized baseline (e.g. "512MB", "2GB") | 0 |
| `ze.fwd.pool.size` | Overflow pool token count (items) | 100000 |
| `ze.fwd.teardown.grace` | Seconds at >95% + >2x weight before forced teardown | 5s |
| `ze.fwd.pool.peeringdb` | Enable PeeringDB refresh when local data is stale (boolean) | false |
| `ze.fwd.pool.peeringdb.user` | PeeringDB API username (optional, for authenticated queries) | - |
| `ze.fwd.pool.peeringdb.password` | PeeringDB API password | - |
| `ze.fwd.pool.refresh.age` | Max age before triggering PeeringDB refresh | 30 days |
| Per-peer `prefix maximum` | Max expected prefixes (BGP config, see spec-prefix maximum) | Required |

`ze.fwd.pool.headroom` adds memory on top of the auto-sized baseline. Operators
on machines with plenty of RAM can set headroom to delay teardown decisions --
trading memory for session stability. Total budget = auto-sized + headroom.
When `ze.fwd.pool.maxbytes` is explicitly set, it overrides everything (headroom
is ignored).

### Layer 3: Read Throttling (backpressure)

When the overflow pool fills, slow down inbound reads from source peers.
The buffer fill level drives the throttle -- a proportional feedback loop.

**Identifying who to throttle:**

Not all source peers contribute equally to congestion. Throttling must
target the peers whose traffic is actually filling the pool, not punish
low-volume peers for the sins of high-volume ones.

#### Source-Side Metrics (per source peer)

| Metric | What | Why needed | Prometheus |
|--------|------|-----------|------------|
| `ze_bgp_updates_received_total` | Counter: total UPDATEs received | Baseline volume, rate() gives send rate | Yes |
| `ze_bgp_updates_forwarded_total` | Counter: items that went through channel (normal path) | The "good" path -- destination kept up | Yes |
| `ze_bgp_updates_overflowed_total` | Counter: items that went to overflow pool | The "pressure" path -- destination couldn't keep up | Yes |
| `ze_bgp_overflow_ratio` | Gauge: overflowed / (forwarded + overflowed) over sliding window | **The key throttle metric.** High ratio = this source's traffic is piling up. | Yes |
| `ze_bgp_throttle_state` | Gauge: 0=granted, 1=denied | Current backpressure level (buffer denial) | Yes |
| `ze_bgp_throttle_denied_total` | Counter: total buffer requests denied | Cumulative backpressure applied to this source | Yes |

**The overflow ratio is the throttle signal.** A source where 80% of updates
overflow is causing pressure. A source where 1% overflows is fine even at
high volume. This distinguishes "fast sender to healthy destinations" from
"sender whose traffic is filling the pool."

**Two time windows for detection:**

| Window | Duration | Detects |
|--------|----------|---------|
| Short | 10s | Bursts: sudden spike in overflow ratio |
| Long | 60s | Slow build: gradual increase over minutes |

Throttle activates if EITHER window shows high overflow ratio for a source peer.

#### Destination-Side Metrics (per destination peer)

| Metric | What | Why needed | Prometheus |
|--------|------|-----------|------------|
| `ze_bgp_overflow_items` | Gauge: items currently in this peer's overflow | Direct pressure indicator | Yes |
| `ze_bgp_overflow_items_total` | Counter: total items that entered overflow | Cumulative congestion history | Yes |
| `ze_bgp_overflow_growth_rate` | Gauge: change in overflow_items over last 10s | Recovering (negative) or worsening (positive) | Yes |
| `ze_bgp_congested` | Gauge: 0 or 1 | Already exists via onCongested/onResumed | Yes |
| `ze_bgp_congested_duration_seconds` | Gauge: seconds in current congestion | Teardown decision input | Yes |
| `ze_bgp_write_errors_total` | Counter: TCP write failures | Detects stuck TCP | Yes |
| `ze_bgp_write_latency_seconds` | Histogram: time in writeMu per batch | Slow writes predict congestion | Yes |

#### Global Pool Metrics

| Metric | What | Prometheus |
|--------|------|------------|
| `ze_bgp_pool_capacity` | Gauge: total slots | Yes |
| `ze_bgp_pool_used` | Gauge: currently allocated slots | Yes |
| `ze_bgp_pool_used_ratio` | Gauge: pool_used / pool_capacity | Yes |

#### Throttle Decision Logic

**Primary mechanism: buffer denial (zero-copy design, 2026-03-23).**

The overflow pool provides read buffers to source peers during congestion.
Throttling is buffer denial: a peer whose destinations have a high
usage-to-weight ratio is denied a buffer. No buffer means no TCP read
means kernel backpressure on the remote sender.

1. Source peer requests buffer from overflow pool
2. Pool checks weighted access: usage-to-weight ratio for destinations
   this source feeds
3. Low ratio: buffer granted, peer reads normally
4. High ratio: buffer denied, peer waits (TCP backpressure)
5. As destinations drain, usage drops, ratio improves, buffers granted again

**No sleep-between-reads needed.** The previous design used proportional
sleep durations. The zero-copy buffer-ownership model makes this unnecessary:
buffer exhaustion creates backpressure automatically, targeted at the
culprit (the source whose destinations are consuming the most pool buffers
relative to their weight).

#### Two-Threshold Enforcement (2026-03-27)

Buffer denial slows inflow but cannot reclaim buffers already held by a
stuck destination. One frozen peer can pin every buffer in the pool,
freezing all source peers' read loops. RustBGPd avoids this through
per-peer tokio tasks (a stuck peer blocks only its own task), but pays
the price: unbounded memory, no backpressure, stuck `write_all` blocks
forever. Ze's shared pool gives memory bounding and zero-copy, but
requires active enforcement to prevent one peer from monopolising it.

**Threshold 1 -- Buffer denial (soft, seconds).**
When `PoolUsedRatio() > 0.8`, deny buffer requests for source peers
whose traffic feeds destination peers with the highest usage-to-weight
ratio. Slows inflow from the culprit source. Does not reclaim buffers
already held.

**Threshold 2 -- Forced teardown (hard, seconds).**
When `PoolUsedRatio() > 0.95` AND the destination peer with the highest
usage-to-weight ratio exceeds 2x its weight share AND this condition
persists for a configurable duration (default 5s, `ze.fwd.teardown.grace`),
tear down that destination peer. All its overflow items fire `done()`,
all handles return to the pool. System recovers immediately.

**Why seconds, not minutes.** The Send Hold Timer (max(8min, 2x hold))
is a safety net for stuck TCP. It is too slow for pool exhaustion -- the
pool can fill in seconds during a convergence burst to a stuck peer.
The congestion teardown fires in 5-10 seconds, reclaiming buffers before
the system freezes. The Send Hold Timer remains as a second safety net
for scenarios where the congestion logic does not trigger (e.g. peer
drains just enough to stay below the ratio threshold but never catches up).

**The check lives in the forward worker.** Every time `fwdBatchHandler`
hits the write deadline (TCP stuck), the worker checks: is the pool
critical? Am I the worst offender? If yes, the worker tears down its own
session. This is natural -- the stuck worker is the one that knows it is
stuck. No separate enforcement goroutine needed.

| Pool state | Worst peer ratio | Duration | Action |
|-----------|-----------------|----------|--------|
| < 80% used | Any | Any | Normal operation |
| 80-95% used | Any | Any | Buffer denial on culprit sources (Threshold 1) |
| > 95% used | < 2x weight | Any | Buffer denial continues, no teardown |
| > 95% used | > 2x weight | < grace period | Buffer denial, teardown pending |
| > 95% used | > 2x weight | >= grace period | Tear down worst peer (Threshold 2) |

**After teardown:** pool recovers instantly (all handles returned).
If another peer becomes the new worst offender, the cycle repeats.
In practice, one stuck peer is the common case -- tearing it down
resolves the congestion.

~~**Mechanism: sleep between reads**~~

~~The sleep duration between TCP reads from source peers is proportional to
both the pool fill level AND the source peer's overflow ratio.~~

| ~~Pool fill~~ | ~~Source overflow ratio~~ | ~~Sleep between reads~~ |
|-----------|---------------------|-------------------|
| ~~0-25%~~ | ~~Any~~ | ~~0 (normal)~~ |
| ~~25-50%~~ | ~~Low (<10%)~~ | ~~0 (this source is not the problem)~~ |
| ~~25-50%~~ | ~~High (>50%)~~ | ~~Short (1-5ms)~~ |
| ~~50-75%~~ | ~~Low~~ | ~~0-1ms~~ |
| ~~50-75%~~ | ~~High~~ | ~~Medium (10-50ms)~~ |
| ~~75-100%~~ | ~~Any~~ | ~~Long (100-500ms) -- everyone slows~~ |

**Superseded (2026-03-23):** Sleep-between-reads replaced by buffer denial
in the zero-copy pool design. See `docs/architecture/forward-congestion-pool.md`.

~~**Mechanism: TCP receive window zero**~~

~~A harder escalation for severe congestion. Known vendor technique (Nokia/ALU).~~

~~TCP window zero is set on source peer connections whose overflow ratio is
highest. The kernel stops accepting data from that source. The source's
writes block in their TCP stack.~~

**Superseded (2026-03-23):** Explicit TCP window zero manipulation removed.
Stopping reads (buffer denial) achieves the same effect naturally: the kernel
receive buffer fills, the kernel advertises progressively smaller TCP windows
automatically, and the remote peer slows its sends. This is how TCP flow
control was designed to work. No platform-specific syscalls, no pulse timing
complexity, no risk of hold timer violations from incorrect pulse logic.

The kernel is better at managing window sizes than application code. Buffer
denial is the single backpressure mechanism for all congestion levels.

~~**Combining both mechanisms:**~~

~~Sleep-between-reads is the primary backpressure tool (smooth, proportional,
keepalive-safe, targeted by overflow ratio). TCP window pulsing is the
escalation when sleep alone cannot reduce inflow enough.~~

**Superseded (2026-03-23):** Single mechanism: buffer denial. No sleep, no
explicit TCP window manipulation. See `docs/architecture/forward-congestion-pool.md`.

**Hold timer safety under buffer denial**

Buffer denial stops reads, which stops hold timer resets on the inbound
side. The constraint remains: at least one message must arrive within each
hold time period. Buffer denial duration is bounded by AC-9:
`keepalive_interval / 6`. Within that window, the source peer's kernel
buffer holds its queued data. When the buffer is granted and reading
resumes, the pending data (including any keepalive) is read immediately.

#### Defensive: Handling Window Zero FROM Peers

Ze must also handle the case where a REMOTE peer sets its TCP receive
window to zero toward us (the Nokia/ALU scenario, the Ben Cox attack).

**RFC 9687 Send Hold Timer** is the defense. If ze cannot send any
message to a peer for the configured Send Hold Time (recommended:
max(8 minutes, 2x HoldTime)), send NOTIFICATION Error Code 8 and
tear down the session. This prevents "BGP zombie" sessions where
a peer holds window zero indefinitely.

### Layer 4: Session Teardown (last resort)

Triggered by Threshold 2 (above): pool > 95% full, worst peer exceeds
2x its weight share, grace period elapsed. The forward worker tears down
its own session.

**Teardown is not dropping.** Both sides know the session ended. On reconnect,
full initial sync occurs. Routing consistency is preserved because the session
boundary invalidates all prior state.

**GR-aware teardown (RFC 4724):**

| Peer GR support | Teardown method | Route fate |
|-----------------|----------------|------------|
| GR capable | TCP close without NOTIFICATION | Peer retains routes, marks stale, re-syncs on reconnect |
| No GR | NOTIFICATION (Cease/Out of Resources, subcode 8) then close | Routes deleted, full sync on reconnect |

Per RFC 4724 Section 4: NOTIFICATION triggers route deletion (Event 24/25).
TCP failure without NOTIFICATION triggers route retention (Event 18). For
GR peers, TCP close is strictly better.

**Teardown triggers:**

| Condition | Action |
|-----------|--------|
| Threshold 2: pool >95%, peer >2x weight, grace elapsed | Tear down the slow destination peer (congestion teardown) |
| Send Hold Timer expired (RFC 9687) | Tear down -- cannot send anything to this peer |

**Timing hierarchy:**

| Mechanism | Fires after | Purpose |
|-----------|-------------|---------|
| Write deadline | 30s | Unblocks writeMu, lets worker detect stuck TCP |
| Congestion teardown (Threshold 2) | ~35s (30s deadline + 5s grace) | Reclaims pool buffers before system freezes |
| Send Hold Timer (RFC 9687) | max(8min, 2x hold) | Safety net for stuck TCP that congestion logic misses |

**Configuration:**

| Config key | Purpose | Default |
|-----------|---------|---------|
| `ze.fwd.teardown.grace` | Seconds at >95% + >2x weight before teardown | 5s |

## Keepalive Under Congestion (corrected analysis)

### Why keepalive priority is a non-problem

RFC 4271 Section 6.5 specifies the hold timer resets on **both KEEPALIVE and
UPDATE** messages. All implementations (ze, BIRD, GoBGP, RustBGPd, FRR)
implement this correctly. This means:

| Scenario | Keepalive delayed? | Remote hold timer | Session alive? |
|----------|-------------------|-------------------|----------------|
| We're sending UPDATEs, keepalive blocked by writeMu | Yes | Reset by every UPDATE received | **Yes -- UPDATEs keep it alive** |
| Session idle, no UPDATEs or keepalives flowing | Keepalive goes out trivially (no writeMu contention) | Reset by keepalive | Yes |
| TCP fully stuck, nothing reaches remote | Both UPDATEs and keepalives blocked | Expires | No -- RFC 9687 Send Hold Timer detects this |

**A delayed keepalive only matters when the session is idle AND the keepalive
is blocked.** But an idle session has no forward batch holding writeMu, so
there is no contention. The keepalive goes out immediately.

**When UPDATEs are flowing, the keepalive is redundant.** The remote peer's
hold timer resets on every UPDATE it receives. A keepalive that arrives 30
seconds late changes nothing -- the remote already knows we're alive from
the UPDATEs it has been receiving.

**When TCP is fully stuck, priority cannot help.** Neither UPDATEs nor
keepalives can be written. No priority system, dedicated thread, or phase
ordering changes this. RFC 9687 Send Hold Timer is the correct safety net:
detect that we cannot send anything, tear down proactively.

### What is still needed: writeMessage write deadline (AC-8)

`writeMessage` (used by keepalive, NOTIFICATION, OPEN) has no write deadline.
If TCP is stuck, `writeMessage` blocks indefinitely under writeMu. This
is not a keepalive priority problem -- it is a resource leak: writeMu is
held forever, preventing the forward worker from detecting the stuck TCP
and triggering teardown.

Fix: `writeMessage` must set a write deadline. Suggested:
`min(holdTime/3, fwdWriteDeadline)`. This bounds the writeMu hold time
and allows the system to detect stuck TCP within a bounded interval.

### Write priority: withdrawals before announcements (AC-25)

**Requires route superseding (AC-23) first. Unsafe without it.**

The value of withdrawal priority is convergence speed for route removal.
A late withdrawal means the remote peer continues forwarding to a dead
next-hop (active traffic loss). A late announcement means the remote peer
uses a longer path but traffic still arrives (suboptimal routing). Sending
withdrawals first during convergence bursts reduces time-to-recovery.

**Intra-message ordering is already correct.** RFC 4271 places the Withdrawn
Routes field before the NLRI field in the UPDATE wire format. Ze goes further:
MP_UNREACH_NLRI (attr 15) is placed first in the attribute section, MP_REACH_NLRI
(attr 14) last (see `docs/architecture/wire/mp-nlri-ordering.md`). Within a single
UPDATE, withdrawals are always processed before announcements.

**The inter-message ordering hazard:** The same prefix can appear as an
announcement then a withdrawal in separate UPDATEs in the same batch (route
flap, policy change). If reordered across messages, the withdrawal is sent
first (no-op at the remote) then the announcement installs a route that
should have been removed. The remote peer's RIB permanently diverges with
no recovery mechanism.

| Batch order | Announce 10.0.0.0/24 then Withdraw 10.0.0.0/24 | Result at remote |
|-------------|------------------------------------------------|-----------------|
| FIFO (current) | Install, then remove | Correct (route removed) |
| Withdrawal-first (naive) | Remove (no-op), then install | **Wrong (stale route persists)** |

**RustBGPd avoids this** because its `PendingTx` structure deduplicates by
prefix: if a withdrawal arrives for a prefix already in the announcement
set, the announcement is replaced. After dedup, no prefix appears in both
the withdrawal and announcement sets -- reordering is safe.

**Ze prerequisite: route superseding (AC-23).** Once the overflow pool
deduplicates by prefix (new update for an already-queued prefix replaces
the old entry), the same prefix cannot appear as both announcement and
withdrawal in the same batch. Only then is withdrawal-first reordering
safe. AC-25 is deferred until AC-23 is implemented.

RustBGPd implements 4-phase write priority: urgent (OPEN/KEEPALIVE/
NOTIFICATION) > withdrawals > announcements > EOR. BIRD uses a priority
bitmask: CLOSE > NOTIFICATION > OPEN > KEEPALIVE > UPDATE (no withdrawal/
announcement distinction). Neither implementation's priority helps when
TCP is congested -- priority only matters when the socket is writable.

### Industry comparison (corrected)

| Implementation | Keepalive mechanism | Why it works |
|---------------|-------------------|-------------|
| BIRD | Priority bitmask, same TX buffer | UPDATEs reset remote hold timer; priority only matters on idle sessions |
| GoBGP | Same goroutine, InfiniteChannel | UPDATEs reset remote hold timer; no contention on idle sessions |
| RustBGPd | Same task, urgent Vec drained first | UPDATEs reset remote hold timer; priority is about withdrawal ordering |
| FRR | Dedicated keepalive thread | Solves a problem that doesn't exist (UPDATEs already keep session alive). Real value: guaranteed keepalive on truly idle sessions even if main thread is stuck processing. |

~~FRR's dedicated keepalive thread is the gold standard.~~ FRR's dedicated
thread solves an edge case (main thread stuck in CPU-heavy processing on an
otherwise idle session) but the primary defense is and always has been
RFC 9687 Send Hold Timer for stuck TCP.

## Interaction: Stuck TCP Outbound

When TCP is stuck outbound (writes fail or block):

| Time | What happens |
|------|-------------|
| 0s | Forward write starts blocking/failing |
| 30s | Forward write deadline expires, writeMu released |
| 30s+ | writeMessage (keepalive/NOTIFICATION) also fails with deadline |
| Ongoing | Remote still receiving nothing -- hold timer counting down |
| 90-180s | Remote hold timer expires, remote tears down |
| max(8min, 2x HoldTime) | RFC 9687 Send Hold Timer expires, we tear down proactively |

The congestion teardown (Layer 4) should fire BEFORE the hold timer
expires. This gives the system control over HOW the session ends (GR-friendly
TCP close vs remote-initiated NOTIFICATION which deletes routes).

RFC 9687 Send Hold Timer provides a second safety net: if the session is
stuck but the hold timer hasn't expired (because we're still receiving
keepalives from the remote), the Send Hold Timer detects that WE can't
send and tears down proactively.

## Interaction: Route Refresh (RFC 2918)

Route Refresh is NOT used as a recovery-from-drops mechanism (there are no
drops). It is relevant in two scenarios:

1. **After teardown + reconnect:** The reconnecting peer receives full initial
   sync automatically. Route Refresh is not needed.

2. **Post-congestion re-validation:** If an operator suspects a peer's view
   may be stale due to prolonged congestion (even without drops), they can
   manually trigger Route Refresh. This is operational, not automatic.

Route Refresh does not play a role in the congestion handling path itself.

## Interaction: Graceful Restart (RFC 4724)

GR affects the TEARDOWN decision only (Layer 4). It does not affect
Layers 1-3. Summary:

- GR peers: TCP close (route retention, reconnect within Restart Time)
- Non-GR peers: NOTIFICATION + close (routes deleted, full re-sync)
- LLGR (RFC 9494): same as GR but with extended retention period
- If WE are the one restarting: not applicable (we're the sender, not receiver)

## Memory Budget

| Component | Per peer | Global |
|-----------|----------|--------|
| Channel buffer | chanSize x ~200B = ~12 KB | N_peers x 12 KB |
| Overflow pool | Fair-share proportional to prefix count | Configured or auto-sized |
| Wire bytes | Shared via cache refcount | Shared, bounded by cache eviction |

**Example sizing from routing data:**

| Deployment | Peers | Avg prefixes/peer | 10% burst | Pool needed |
|-----------|-------|-------------------|-----------|-------------|
| Small IXP | 50 | 10K | 1K updates/peer | ~50K items = ~10 MB |
| Medium IXP | 200 | 50K | 5K updates/peer | ~1M items = ~200 MB |
| Large IXP (DE-CIX scale) | 1000 | 100K | 10K updates/peer | ~10M items = ~2 GB |
| Full table peer | 1 | 1M | 100K updates | ~100K items = ~20 MB |

Auto-sizing from local RIB or PeeringDB makes these numbers real rather
than guessed.

## Open Questions (for review)

1. ~~**Fair-share enforcement:** Hard per-peer limit from the global pool?
   Or soft (advisory, with one peer able to burst into another's share if
   the other is idle)?~~
   **Resolved (2026-03-23):** Weighted access with diminishing rights. No hard
   per-peer limit. Each peer's weight is its expected prefix count. Access
   priority decreases as usage-to-weight ratio increases. Under pressure, the
   highest-ratio peer is denied buffers first (natural backpressure) and is the
   first teardown candidate. Over time, usage rebalances toward weight proportions.
   See `docs/architecture/forward-congestion-pool.md`.

2. ~~**Pool sizing default:** What fraction of available memory when no
   prefix maximum, no zefs data, and no PeeringDB? 10%? 25%? Require
   explicit configuration?~~
   **Resolved (2026-03-23):** ~~Asymmetric allocation/release. Growth in 10%
   contiguous blocks (less fragmentation). Shrink per-buffer on return when
   free space >20%. First 10% block is permanent (never freed) -- hot reserve
   for next burst. Maximum is 100% of peer weight sum.~~
   **Updated (2026-03-24):** Three rules. Grow on exhaustion (no threshold).
   Allocate from highest block. Collapse highest every 100th Get() when fully
   returned and block below has >=50% free. No permanent block.
   See `docs/architecture/forward-congestion-pool.md`.

3. ~~**Pre-allocation granularity:** One large byte slab? A free-list of
   fwdItem-sized slots? A ring buffer?~~
   **Resolved (2026-03-23):** ~~Zero-copy buffer transfer. The overflow pool
   provides read buffers directly to source peers during congestion. No copy,
   no ownership transfer -- the buffer is allocated from the overflow pool,
   TCP reads into it, and it stays in the overflow pool until the destination
   drains it. Buffer exhaustion IS the backpressure mechanism.~~
   **Updated (2026-03-24):** Block-backed multiplexer with handles. Each
   handle carries block ID for deterministic return routing. Blocks are
   contiguous backing arrays. Growth on exhaustion, shrink via lazy collapse
   every 100th Get(). No free-list, no ring buffer, no pre-allocation.
   See `docs/architecture/forward-congestion-pool.md`.

4. ~~**RFC 9687 Send Hold Timer value:** max(8 min, 2x HoldTime) per RFC
   recommendation, or configurable?~~
   **Resolved (2026-03-23):** Default `max(8 minutes, 2x HoldTime)` per RFC 9687
   Section 4 recommendation. Configurable per peer for operators who need to tune.

5. ~~**Prefix-limit burst fraction:** If prefix maximum is 100K, what
   fraction represents "worst expected burst"? 10% (10K items)?
   Configurable per peer or global multiplier?~~
   **Resolved (2026-03-23):** Burst fraction is inverse to peer size. Small
   peers over-provision prefix-maximum by 4x+, large peers (full table) set
   it close to actual. Scaling curve: <500 prefixes = 100%, 500-10K = 50%,
   10K-100K = 30%, 100K-500K = 15%, 500K+ = 10%. DFZ reference: ~1.1M IPv4,
   ~260K IPv6 (March 2026). Channel size (Layer 1) and pool weight (Layer 2)
   both derived from the burst fraction, not raw prefix-maximum.
   See `docs/architecture/forward-congestion-pool.md`.

6. ~~**Advisory report format:** CLI command output? Structured JSON for
   automation? Both?~~
   **Resolved (2026-03-23):** All three: CLI output (`ze bgp status`),
   events via monitor (plugin event stream), and Prometheus metrics
   (`ze_bgp_prefix_limit_headroom_ratio` per peer).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_pool.go` - per-peer workers, TryDispatch, DispatchOverflow
  -> Constraint: committed code drops oldest at overflowMax=256
  -> Constraint: working tree removed limit (unbounded)
  -> Constraint: done() callbacks guaranteed via safeBatchHandle
  -> Constraint: congestion flag with hysteresis (25% low-water)
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate dispatches to pool
  -> Constraint: cache Retain before dispatch, Release in done callback
- [ ] `internal/component/bgp/reactor/session_write.go` - writeMessage has NO write deadline
  -> Constraint: only fwdBatchHandler sets write deadline
- [ ] `internal/component/bgp/reactor/session.go` - keepalive/hold timers
  -> Constraint: hold timer default 90-180s; keepalive interval = holdTime/3
- [ ] `internal/component/bgp/message/notification.go` - NotifyCeaseOutOfResources = subcode 8

**Behavior to preserve:**
- Per-peer FIFO ordering
- Cache lifecycle (Retain/Release)
- Zero-copy forwarding when contexts match
- Congestion callbacks (onCongested/onResumed)
- safeBatchHandle panic recovery
- Drain-batch buffer reuse

**Behavior to change:**
- ~~Replace drop-at-256 with block-backed overflow pool (weighted access, lazy 10% growth)~~ Done (Phase 1, forward_pool.go)
- ~~Add write deadline to writeMessage (keepalive bug)~~ Done (Phase 2, session_write.go controlWriteDeadline)
- ~~Set TCP_NODELAY on peer sockets~~ Done (commit 1c43e11d)
- ~~Set IP_TOS/DSCP CS6 (0xC0) on all peer sockets (dialer Control callback + accepted connections)~~ Done (Phase 1, session_connection.go:215-227)
- ~~Implement RFC 9687 Send Hold Timer~~ Done (Phase 2, session_write.go:111-199)
- ~~Add hold timer congestion extension (BIRD technique: extend 10s if RX data pending)~~ Done (Phase 2, session.go:320-331)
- Add route superseding in overflow pool (deferred optimization, AC-23)
- ~~Add TX budget limiting (cap messages per forward batch to prevent one peer starving others)~~ Done (Phase 4, ze.fwd.batch.limit, forward_pool.go:222)
- ~~Replace sync.Pool with pool multiplexer (handle = block ID + []byte, deterministic block freeing)~~ Done (Phase 3, bufmux.go)
- ~~Add source-side metrics (overflow ratio, throttle state)~~ Done (Phase 2, reactor_metrics.go)
- ~~Add destination-side metrics (overflow items, growth rate, write latency)~~ Done (Phase 2, reactor_metrics.go)
- ~~Add read throttling via buffer denial (weighted access, culprit targeting)~~ Done (Phase 5, forward_pool_congestion.go ShouldDeny, wired in DispatchOverflow)
- ~~Add TCP window zero pulsing as escalation~~ Removed: buffer denial causes natural TCP window shrinkage
- ~~Add session teardown as last resort (GR-aware)~~ Done (Phase 5, forward_pool_congestion.go CheckTeardown + congestionTeardownPeer, wired in runWorker)
- ~~Add overflow flush on peer-down~~ Done (forward_pool.go Stop, TestFwdPool_PeerDisconnectReturnsSlots)
- ~~Add PeeringDB integration for buffer sizing~~ Done (spec-prefix-limit, peeringdb/client.go)

## Data Flow (MANDATORY)

### Entry Point
- Cached UPDATE via ForwardUpdate from bgp-rs plugin

### Congestion Path
1. ForwardUpdate calls TryDispatch per destination peer
2. Channel full: TryDispatch returns false, item goes to overflow path
3. Source peer's next read requests buffer from overflow pool (not local read pool)
4. Pool checks weighted access: usage-to-weight ratio for this peer's destinations
5. If ratio acceptable: buffer granted, TCP reads into overflow-pool-owned buffer
6. If ratio too high: buffer denied, peer cannot read, TCP backpressure on remote
7. Destination worker drains overflow items, returns buffers to pool
8. As buffers return: pool checks free space, nil's excess (GC frees backing block when all nil'd)
9. If pool exhausted and backpressure insufficient: tear down slowest destination

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Read loop -> overflow pool | Buffer request (Get) from block-backed pool | [ ] |
| Pool -> weighted access | Usage-to-weight ratio check before granting buffer | [ ] |
| Buffer denial -> TCP backpressure | No buffer = no read = kernel backpressure on remote | [ ] |
| Buffer return -> pool shrink | Nil slice reference when >20% free, GC frees block | [ ] |
| Pool exhaustion -> teardown | Callback from pool to reactor | [ ] |
| Peer-down -> pool | Return all peer's buffers to pool | [ ] |
| 4K + 64K -> combined capacity | Growth/shrink/backpressure use combined usage | [ ] |
| Metrics -> Prometheus | Standard Prometheus exposition | [ ] |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test | Status |
|-------------|---|--------------|------|--------|
| ForwardUpdate to slow peer, pool fills | -> | Buffer denied to worst destination peer | TestCongestion_ShouldDenyHighRatio, TestCongestion_FastPeerUnaffected | Done (forward_pool_congestion_test.go) |
| Pool exhausted, backpressure insufficient | -> | Slow destination torn down (GR-aware) | TestCongestion_ForcedTeardownFires, TestCongestion_TeardownGRCapable | Done (forward_pool_congestion_test.go) |
| Peer disconnects with pool slots | -> | Slots returned to pool immediately | TestFwdPool_PeerDisconnectReturnsSlots | Done (forward_pool_test.go:922) |
| Send Hold Timer expires on stuck peer | -> | Session torn down with Error Code 8 | TestSendHoldDurationAuto, TestSendHoldDurationExplicit | Done (session_test.go:3053,3080) |
| BGP session established (dial or accept) | -> | IP_TOS=0xC0 set on TCP socket | session_connection.go:215-227 (syscall in dialer Control) | Done (code, no dedicated test) |
| Hold timer fires while RX buffer has data | -> | Timer extended 10s, session not torn down | session.go:320-331 (recentRead.Swap) | Done (code, no dedicated test) |
| Two updates for same prefix in overflow | -> | Second replaces first, pool item count unchanged | Deferred optimization (AC-23) | Deferred |
| Forward batch exceeds TX budget | -> | Batch capped, remaining deferred to next cycle | TestFwdDrainBatchLimit | Done (forward_pool_test.go:553) |
| Forward batch has withdrawals and announcements (after AC-23 dedup) | -> | Withdrawals written before announcements | Deferred optimization (AC-25, blocked on AC-23) | Deferred |
| Overflow metrics visible via Prometheus | -> | Pool ratio, overflow items, overflow ratios | test/plugin/forward-congestion-overflow-metrics.ci | Done |
| Pool auto-sized from peer set | -> | Weight tracker updates BufMux budget on peer add/remove | TestWeightTracker_AddPeer, TestWeightTracker_TotalBudget | Done (weight_tracker_test.go) |
| Burst fraction scales with prefix count | -> | burstFraction returns inverse scaling curve | TestBurstFraction, TestBurstWeight, TestPeerBufferDemand | Done (weight_test.go) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route forwarded to slow peer, channel full | Item stored in overflow pool (never dropped) |
| AC-2 | Overflow pool under pressure, source peer requests buffer | Buffer denied to peer with highest usage-to-weight ratio (backpressure via buffer denial). **Done** (forward_pool_congestion.go:ShouldDeny, TestCongestion_ShouldDenyHighRatio, TestCongestion_FastPeerUnaffected). |
| AC-3 | Destination drains overflow, buffers returned | Peer's usage-to-weight ratio drops, buffer requests granted again. **Done** (ShouldDeny returns false when ratio drops below worst -- TestCongestion_ShouldDenyBelowThreshold). |
| AC-4 | Pool >95% full, destination peer exceeds 2x weight share for grace period | Destination peer session torn down (GR-aware). Worker checks after each batch via CheckTeardown. Grace configurable via `ze.fwd.teardown.grace` (default 5s). **Done** (forward_pool_congestion.go:CheckTeardown, TestCongestion_ForcedTeardownFires, TestCongestion_TeardownGracePeriodResets). |
| AC-5 | Destination with GR capability torn down | TCP close without NOTIFICATION (route retention). **Done** (congestionTeardownPeer: closeConn + fsm.EventManualStop, TestCongestion_TeardownGRCapable). |
| AC-6 | Destination without GR torn down | Cease/Out of Resources NOTIFICATION then close. **Done** (congestionTeardownPeer: Teardown(NotifyCeaseOutOfResources), TestCongestion_ForcedTeardownFires with grCapable=false). |
| AC-7 | Peer disconnects with overflow items | Pool slots returned, done() called for all items |
| AC-8 | writeMessage called on stuck TCP | Write deadline set via controlWriteDeadline(): min(holdTime/3, 30s), minimum 10s. **Done** (session_write.go:99-108). |
| AC-9 | Buffer denial duration | Buffer denial is per-dispatch-call (not continuous blocking). ShouldDeny is checked on each DispatchOverflow call; the worker drain cycle naturally bounds denial duration. No explicit keepalive/6 timer needed -- the source peer's read loop is never paused, only the overflow pool token is skipped. **Done** (design simplification: denial skips token, does not block reads). |
| AC-10 | Pool size configurable | `ze.fwd.pool.size` (overflow token count, default 100K) and `ze.fwd.pool.maxbytes` (combined byte budget, auto-sized from peer set via weight tracker when 0). `ze.fwd.pool.headroom` adds extra memory beyond auto-sized baseline (total = auto + headroom). **Done** (reactor.go:68-72, reactor.go:326-372). |
| AC-11 | Total memory for slow peers | Bounded by pool maximum (sum of peer weights), growth in 10% blocks, shrink per-buffer on return |
| AC-12 | Fast destination peers during congestion | Unaffected by slow destination -- isolation preserved. **Done** (ShouldDeny only targets the worst peer; TestCongestion_FastPeerUnaffected verifies a healthy peer is never denied). |
| AC-13 | ~~TCP window zero on source peer~~ | ~~Pulses open 5x per keepalive interval, 2-3s each~~ **Removed (2026-03-23):** buffer denial causes natural TCP window shrinkage via kernel. Explicit window zero manipulation unnecessary. |
| AC-14 | Remote peer holds window zero toward us | RFC 9687 Send Hold Timer tears down after configured duration. Default max(8min, 2x ReceiveHoldTime). **Done** (session_write.go:111-199, TestSendHoldDurationAuto, TestSendHoldDurationExplicit, config_test.go:180,218). |
| AC-15 | TCP_NODELAY set on peer sockets | All BGP peer TCP connections have TCP_NODELAY enabled. Done (commit 1c43e11d). |
| AC-16 | Source peer overflow ratio visible in Prometheus | ze_bgp_overflow_ratio gauge per peer |
| AC-17 | Destination peer overflow depth visible in Prometheus | ze_bgp_overflow_items gauge per peer |
| AC-18 | Pool utilization visible in Prometheus | ze_bgp_pool_used_ratio gauge |
| AC-19 | Per-peer buffer share proportional to prefix count | Peer with 500K prefixes gets larger share than peer with 200. **Done** (forward_pool_weight.go: burstFraction, burstWeight, peerBufferDemand; forward_pool_weight_tracker.go: AddPeer, TotalBudget, PeerDemand; tests in weight_test.go and weight_tracker_test.go). |
| AC-20 | PeeringDB lookup for unknown peers | **Satisfied by spec-prefix-limit (2026-03-26).** Prefix maximum is mandatory per peer. PeeringDB client exists (`internal/component/bgp/peeringdb/client.go`). `ze bgp peer * prefix update` refreshes values. No additional work needed here. **Done.** |
| AC-21 | IP_TOS/DSCP CS6 set on all peer sockets | Outgoing (dialer Control callback) and accepted connections set IP_TOS=0xC0 (IPv4) / IPV6_TCLASS=0xC0 (IPv6). **Done** (session_connection.go:215-227). |
| AC-22 | Hold timer fires with RX data pending | Hold timer extended 10s instead of teardown (CPU congestion, not peer failure). **Done** (session.go:320-331, recentRead atomic flag). |
| AC-23 | New update for prefix already in overflow pool | **Deferred optimization.** Old entry replaced (route superseding), not appended. Requires per-prefix indexing of the overflow pool (NLRI parsing on overflow entry). Ze's UPDATE-first design avoids NLRI parsing on the forward path; adding it here contradicts that principle. Without dedup, FIFO ordering still converges correctly -- the slow peer processes redundant intermediate UPDATEs but reaches the right final state. Real-world overflow is dominated by convergence events (many distinct prefixes withdrawn once), not flap (same prefix repeated). May not fix a real traffic pattern problem. Revisit if profiling shows high duplicate rate in overflow under production load. |
| AC-24 | Forward batch to single destination peer | TX budget caps messages per batch (prevents one peer starving others in event loop). **Done** (`ze.fwd.batch.limit` default 1024, forward_pool.go:222, TestFwdDrainBatchLimit). |
| AC-25 | Forward batch contains both withdrawals and announcements | **Deferred optimization.** Withdrawals sent before announcements (faster convergence). Requires AC-23 (route superseding) first -- without per-prefix dedup, reordering can invert announce/withdraw for the same prefix, causing permanent stale routes. Blocked on AC-23 which is itself deferred. |
| AC-26 | Read and build buffers | sync.Pool replaced by pool multiplexer (BufMux). Handles carry block ID for deterministic return routing. Two multiplexers: 4K and 64K. buildBufPool merged into 4K instance. Get() allocates from lowest block. Grow on exhaustion. Lazy collapse every 100th Get(). No permanent block. **Done** (bufmux.go, session.go:50-68). |
| AC-27 | Pool capacity decisions | Growth, shrink, and backpressure use combined usage across 4K + 64K instances (shared combinedBudget). Collapse check piggybacked on normal read path. **Done** (bufmux.go:175, session.go:64-74). |
| AC-28 | Pool maximum | Dynamically tracks peer set: adding a peer increases maximum (based on prefix count weight), removing decreases it. **Done** (forward_pool_weight_tracker.go, wired via reactor_peers.go:119 AddPeer). |
| AC-29 | ~~Per-peer channel size~~ | **Dropped (2026-03-26):** RIPE RIS analysis shows fixed 64 is the right size. Dynamic sizing adds complexity for no measurable gain. Channel stays at fixed 64 via `ze.fwd.chan.size`. |

## Files to Modify

- `internal/component/bgp/reactor/forward_pool.go` -- add congestion teardown check in fwdBatchHandler, per-worker congestion timer, GR-aware teardown call
- `internal/component/bgp/reactor/forward_pool_test.go` -- tests for buffer denial, forced teardown, GR-aware teardown, headroom, grace period
- `internal/component/bgp/reactor/forward_pool_weight_tracker.go` -- add WorstPeerRatio method for teardown decision
- `internal/component/bgp/reactor/forward_pool_weight_tracker_test.go` -- test WorstPeerRatio
- `internal/component/bgp/reactor/reactor.go` -- register `ze.fwd.teardown.grace` and `ze.fwd.pool.headroom` env vars, wire into pool config
- `internal/component/bgp/reactor/reactor_metrics.go` -- add throttle state and congestion teardown counter metrics
- `internal/component/bgp/reactor/bufmux.go` -- add headroom to combined budget
- `docs/architecture/forward-congestion-pool.md` -- already updated with enforcement + headroom design

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/plugin/forward-congestion-teardown.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add congestion backpressure + forced teardown |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | RFC 9687 already done (Phase 2) |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- update backpressure row, `docs/architecture/congestion-industry.md` -- update Ze column |
| 12 | Internal architecture changed? | Yes | `docs/architecture/forward-congestion-pool.md` -- already updated |

## TDD Test Plan

### Phase 5a: Buffer denial (weighted access)

| Test | File | AC |
|------|------|----|
| TestFwdPool_BufferDenialHighRatio | forward_pool_test.go | AC-2 |
| TestFwdPool_BufferGrantedLowRatio | forward_pool_test.go | AC-3 |
| TestFwdPool_BufferDenialKeepalifeLimit | forward_pool_test.go | AC-9 |
| TestFwdPool_FastPeerUnaffected | forward_pool_test.go | AC-12 |
| TestWeightTracker_WorstPeerRatio | forward_pool_weight_tracker_test.go | AC-4 |

### Phase 5b: Forced teardown

| Test | File | AC |
|------|------|----|
| TestFwdPool_ForcedTeardownOnPoolExhaustion | forward_pool_test.go | AC-4 |
| TestFwdPool_TeardownGRPeer | forward_pool_test.go | AC-5 |
| TestFwdPool_TeardownNonGRPeer | forward_pool_test.go | AC-6 |
| TestFwdPool_TeardownGracePeriod | forward_pool_test.go | AC-4 |
| TestFwdPool_TeardownReclaims | forward_pool_test.go | AC-4, AC-7 |

### Phase 5c: Headroom + config

| Test | File | AC |
|------|------|----|
| TestFwdPool_HeadroomIncreasesbudget | forward_pool_test.go or bufmux_test.go | AC-10 |
| TestFwdPool_HeadroomIgnoredWhenExplicit | forward_pool_test.go or bufmux_test.go | AC-10 |

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

1. **Phase 5a: Buffer denial** -- weighted access check in forward pool
   - Tests: TestFwdPool_BufferDenialHighRatio, TestFwdPool_BufferGrantedLowRatio, TestFwdPool_BufferDenialKeepalifeLimit, TestFwdPool_FastPeerUnaffected, TestWeightTracker_WorstPeerRatio
   - Files: forward_pool.go, forward_pool_weight_tracker.go
   - Verify: tests fail -> implement -> tests pass
   - Key: OverflowDepths + UsageToWeightRatios already exist. Add denial logic to DispatchOverflow path. Track denial start time per source peer for AC-9 keepalive/6 bound.

2. **Phase 5b: Forced teardown** -- congestion teardown in fwdBatchHandler
   - Tests: TestFwdPool_ForcedTeardownOnPoolExhaustion, TestFwdPool_TeardownGRPeer, TestFwdPool_TeardownNonGRPeer, TestFwdPool_TeardownGracePeriod, TestFwdPool_TeardownReclaims
   - Files: forward_pool.go
   - Verify: tests fail -> implement -> tests pass
   - Key: fwdBatchHandler already detects write deadline failures. Add pool ratio + weight ratio check. If pool > 95% and worker's peer ratio > 2x weight for grace period: GR-aware teardown. GR check via peer.session.Negotiated().GracefulRestart (non-nil = GR capable). GR: closeConn() without NOTIFICATION. Non-GR: Teardown(NotifyCeaseOutOfResources, "").

3. **Phase 5c: Headroom + config** -- env var registration and budget integration
   - Tests: TestFwdPool_HeadroomIncreasesBudget, TestFwdPool_HeadroomIgnoredWhenExplicit
   - Files: reactor.go, bufmux.go
   - Verify: tests fail -> implement -> tests pass
   - Key: `ze.fwd.pool.headroom` adds bytes to auto-sized budget. `ze.fwd.teardown.grace` configures grace period. Both registered in reactor.go, read in pool init.

4. **Phase 5d: Metrics + functional test**
   - Add `ze_bgp_throttle_state`, `ze_bgp_throttle_denied_total`, `ze_bgp_congestion_teardown_total` metrics
   - Create `test/plugin/forward-congestion-teardown.ci` functional test
   - Files: reactor_metrics.go, test/plugin/forward-congestion-teardown.ci

5. **Phase 5e: Documentation**
   - Update `docs/features.md`, `docs/comparison.md`, `docs/architecture/congestion-industry.md`
   - Files: docs only

6. **Full verification** -> `make ze-verify`
7. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every Phase 5 AC (AC-2, AC-3, AC-4, AC-5, AC-6, AC-9, AC-12) has implementation with file:line |
| Correctness | Buffer denial uses UsageToWeightRatios correctly: ratio > threshold denies, not inverted |
| Correctness | GR check: Negotiated().GracefulRestart != nil means GR capable, use closeConn(); nil means non-GR, use Teardown(OutOfResources) |
| Correctness | Teardown fires only when pool > 95% AND ratio > 2x AND grace elapsed -- all three conditions required |
| Correctness | Keepalive safety: denial duration bounded by keepalive_interval / 6 (AC-9), not hold time |
| Data flow | Buffer denial happens in the overflow dispatch path (DispatchOverflow or TryDispatch), not in the read loop directly |
| Data flow | Teardown check is in fwdBatchHandler (write deadline hit path), not in a timer goroutine |
| Rule: goroutine-lifecycle | No new goroutines for enforcement -- checks piggyback on existing worker loop and batch handler |
| Rule: buffer-first | No new allocations in hot path -- denial is a boolean check, not a buffer operation |
| Rule: no-layering | No sleep-based throttle code remains -- buffer denial is the single mechanism |
| Isolation | Fast destination peers unaffected: verify a peer with ratio 0.0 is never denied buffers regardless of global pool state |
| Headroom | ze.fwd.pool.headroom adds to auto-sized, ignored when ze.fwd.pool.maxbytes explicit |
| Metrics | Throttle state, denied count, teardown count all registered and updated |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Buffer denial logic in forward_pool.go | `grep -n 'UsageToWeightRatio\|bufferDenied\|denyBuffer' internal/component/bgp/reactor/forward_pool.go` |
| WorstPeerRatio in weight tracker | `grep -n 'WorstPeerRatio' internal/component/bgp/reactor/forward_pool_weight_tracker.go` |
| Forced teardown in fwdBatchHandler | `grep -n 'teardown\|Teardown\|closeConn' internal/component/bgp/reactor/forward_pool.go` |
| GR-aware teardown path | `grep -n 'GracefulRestart\|closeConn\|OutOfResources' internal/component/bgp/reactor/forward_pool.go` |
| Keepalive/6 safety bound (AC-9) | `grep -n 'keepalive.*6\|denialStart\|denialDuration' internal/component/bgp/reactor/forward_pool.go` |
| ze.fwd.teardown.grace env var | `grep -n 'ze.fwd.teardown.grace' internal/component/bgp/reactor/reactor.go` |
| ze.fwd.pool.headroom env var | `grep -n 'ze.fwd.pool.headroom' internal/component/bgp/reactor/reactor.go` |
| Headroom in budget calculation | `grep -n 'headroom\|Headroom' internal/component/bgp/reactor/bufmux.go` |
| Throttle metrics registered | `grep -n 'throttle_state\|throttle_denied\|congestion_teardown' internal/component/bgp/reactor/reactor_metrics.go` |
| Functional test exists | `ls test/plugin/forward-congestion-teardown.ci` |
| All Phase 5 unit tests pass | `go test -race -run 'TestFwdPool_Buffer\|TestFwdPool_Forced\|TestFwdPool_Teardown\|TestFwdPool_Headroom\|TestWeightTracker_Worst' ./internal/component/bgp/reactor/...` |
| docs/features.md updated | `grep -n 'backpressure\|congestion teardown' docs/features.md` |
| docs/comparison.md updated | `grep -n 'backpressure\|buffer denial' docs/comparison.md` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Grace period timer: ensure no per-peer timer goroutine. Use timestamp comparison, not timer. |
| Denial of service | A malicious peer cannot trigger teardown of a different peer by sending high volumes. Verify teardown targets the destination with highest ratio, not the source. |
| Integer overflow | Usage-to-weight ratio division: check for zero weight (division by zero). PeerDemand returns 0 for unknown peers. |
| Race condition | Congestion grace start time must be accessed atomically or under mutex. Multiple workers checking pool ratio concurrently must not cause double teardown. |
| Information leakage | NOTIFICATION Cease/OutOfResources (subcode 8) does not leak internal pool state. Only standard RFC 4486 subcode. |
| Metric cardinality | Throttle state metric uses peer address label -- same cardinality as existing overflow metrics, not unbounded. |
| Hold timer safety | Buffer denial duration bound (keepalive/6) must be enforced even if the pool remains exhausted. A denied source must eventually be granted a read to prevent hold timer expiry. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Teardown fires too eagerly | Check grace period and ratio threshold constants |
| Teardown never fires | Check pool ratio calculation -- is combined budget reporting correctly? |
| Hold timer expires during denial | Check keepalive/6 bound enforcement -- is the denial timer being reset? |
