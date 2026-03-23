# Spec: forward-congestion

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-forward-backpressure, spec-prefix maximum |
| Phase | - |
| Updated | 2026-03-22 |

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

## Design: Four-Layer Congestion Response

Routes are never dropped. Congestion is handled by four escalating layers.
Each layer activates only when the previous layer is insufficient.

### Layer 1: Channel Buffer (existing)

Per-destination-peer buffered channel (default 64 items). Absorbs micro-bursts.
No action needed -- this already works.

### Layer 2: Pre-Allocated Overflow Pool

A global memory pool, pre-allocated at startup, shared by all destination peers.

| Property | Value |
|----------|-------|
| Allocation | At startup, one contiguous block |
| Sizing | Auto-sized from routing data + configurable override |
| Scope | Global -- all peers draw from the same pool |
| Fairness | Per-peer share proportional to expected prefix count |
| Item size | fwdItem slots (~200 bytes metadata each, wire bytes shared via cache refcount) |
| When exhausted | Triggers Layer 3 (read throttling) or Layer 4 (teardown) |

**Why pre-allocate:** `append()` to unbounded slices means the memory bound
is theoretical. Pre-allocation makes the bound real -- the memory is committed
at startup, and the system knows exactly how much it has. No GC pressure spikes
during congestion.

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

| Config key | Purpose |
|-----------|---------|
| `ze.fwd.pool.size` | Explicit total pool size override (e.g., "1GB") |
| `ze.fwd.pool.peeringdb` | Enable PeeringDB refresh when local data is stale (boolean) |
| `ze.fwd.pool.peeringdb.user` | PeeringDB API username (optional, for authenticated queries) |
| `ze.fwd.pool.peeringdb.password` | PeeringDB API password |
| `ze.fwd.pool.refresh.age` | Max age before triggering PeeringDB refresh (default 30 days) |
| Per-peer `prefix maximum` | Max expected prefixes (BGP config, see spec-prefix maximum) |

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
| `ze_bgp_throttle_state` | Gauge: 0=none, 1=sleep, 2=window-zero | Current backpressure level applied | Yes |
| `ze_bgp_throttle_sleep_ms` | Gauge: current sleep duration between reads | How hard we're pushing back | Yes |

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

1. Pool fill ratio crosses threshold (e.g., 50%) -- backpressure needed
2. Rank source peers by overflow_ratio (short OR long window)
3. Throttle highest-ratio sources first -- their traffic is the traffic
   filling the pool
4. Low overflow-ratio sources continue at full speed
5. As pool drains, release throttle starting with lowest-ratio sources

**Mechanism: sleep between reads**

The sleep duration between TCP reads from source peers is proportional to
both the pool fill level AND the source peer's overflow ratio.

| Pool fill | Source overflow ratio | Sleep between reads |
|-----------|---------------------|-------------------|
| 0-25% | Any | 0 (normal) |
| 25-50% | Low (<10%) | 0 (this source is not the problem) |
| 25-50% | High (>50%) | Short (1-5ms) |
| 50-75% | Low | 0-1ms |
| 50-75% | High | Medium (10-50ms) |
| 75-100% | Any | Long (100-500ms) -- everyone slows |

**Mechanism: TCP receive window zero**

A harder escalation for severe congestion. Known vendor technique (Nokia/ALU).

TCP window zero is set on source peer connections whose overflow ratio is
highest. The kernel stops accepting data from that source. The source's
writes block in their TCP stack.

**TCP window zero MUST pulse open periodically.** A zero window held
indefinitely blocks ALL data from a source peer -- including UPDATEs,
keepalives, NOTIFICATION, and ROUTE-REFRESH messages. One slow destination
peer would cause all source peers to freeze, which cascades into hold
timer expiry and mass session teardown (the "BGP zombie" problem
documented by Ben Cox).

**Pulse timing is derived from the hold time.** The window must open
frequently enough that the source peer can deliver at least one message
(UPDATE or keepalive) per hold timer interval. Since RFC 4271 resets
the hold timer on both UPDATEs and keepalives, any received message
prevents expiry. The pulse must ensure at least one message arrives
within every hold time period.

| Hold time | Pulse count | Closed duration | Open duration |
|-----------|-------------|-----------------|---------------|
| 90s (keepalive=30s) | 5 minimum | ~4s | ~2s |
| 180s (keepalive=60s) | 5 minimum | ~10s | ~2s |
| 30s (keepalive=10s) | 5 minimum | ~1s | ~1s |

Formula: `closed_duration = keepalive_interval / (pulse_count + 1)`,
`open_duration = 2-3s` (enough to drain kernel receive buffer).
Per-source-peer calculation uses that peer's negotiated hold time / 3
as the keepalive interval.

**Why 5 pulses minimum:** The source peer sends either UPDATEs or
keepalives (or both). With 5 open windows per keepalive interval, there
are 5 chances to receive at least one message. Even with jitter, 5
windows makes it near-certain something lands in an open window.

**Burst on re-open:** When the window opens, the source peer's kernel
flushes its entire send buffer at once. This is NOT a trickle -- it is
everything that accumulated during the closed period, delivered as a
burst at wire speed. The duty cycle does NOT control effective rate the
way sleep-between-reads does.

This means the open window is a burst-accept phase:
- The source kernel dumps its queued data
- Our kernel receive buffer fills quickly
- We read what we can during the open window
- When we close again, whatever we didn't read stays in our kernel buffer
  and is available on the next open

The practical effect of the pulse is not rate limiting but **volume
limiting per cycle**: the total data accepted per cycle is bounded by
our kernel receive buffer size plus whatever we read during the open
duration. The closed period prevents new data from entering our kernel
buffer (zero window means the source stops sending).

**Implication for pool sizing:** With TCP window pulsing, the overflow
pool must absorb bursts, not a smooth stream. Each open window can
produce a burst of messages equal to `source_kernel_send_buffer +
our_kernel_recv_buffer` worth of BGP messages. The pool must be large
enough to absorb several such bursts per source peer without
exhausting.

**Combining both mechanisms:**

| Pool fill | Sleep between reads | TCP window |
|-----------|-------------------|------------|
| 0-50% | Proportional sleep (high-ratio sources only) | Normal (open) |
| 50-85% | Maximum sleep (high-ratio sources) | Normal (open) |
| 85-95% | Maximum sleep (all sources) | Pulsing on highest-ratio sources |
| 95-100% | Maximum sleep (all sources) | Pulsing on all sources + evaluating Layer 4 teardown |

Sleep-between-reads is the primary backpressure tool (smooth, proportional,
keepalive-safe, targeted by overflow ratio). TCP window pulsing is the
escalation when sleep alone cannot reduce inflow enough.

**Critical constraint: hold timer safety**

Both mechanisms affect message delivery from source peers. Since RFC 4271
resets the hold timer on both UPDATEs and keepalives, the constraint is:
at least one message (any type) must arrive within each hold time period.

The pulse design guarantees this by construction: 5 open windows per
keepalive interval means 5 chances to receive at least one message.
UPDATEs count -- if the source peer is sending routes, those reset our
hold timer just as well as keepalives do.

The read throttle (sleep) MUST also respect the source peer's hold timer.
Maximum sleep bounded by `keepalive_interval / (pulse_count + 1)`.

#### Defensive: Handling Window Zero FROM Peers

Ze must also handle the case where a REMOTE peer sets its TCP receive
window to zero toward us (the Nokia/ALU scenario, the Ben Cox attack).

**RFC 9687 Send Hold Timer** is the defense. If ze cannot send any
message to a peer for the configured Send Hold Time (recommended:
max(8 minutes, 2x HoldTime)), send NOTIFICATION Error Code 8 and
tear down the session. This prevents "BGP zombie" sessions where
a peer holds window zero indefinitely.

### Layer 4: Session Teardown (last resort)

If the overflow pool is exhausted AND read throttling has not reduced inflow
enough AND a specific destination peer's allocation is at its limit, tear
down that destination peer's session.

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
| Peer's overflow share exhausted AND pool >95% full AND backpressure active for >N seconds | Tear down the slow destination peer |
| All read throttling at maximum AND pool still growing | Tear down the slowest destination peer (highest overflow_items) |
| Send Hold Timer expired (RFC 9687) | Tear down -- cannot send anything to this peer |

The congestion teardown timeout must be shorter than hold timers to bound
memory before the natural hold timer safety net kicks in.

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

1. **Fair-share enforcement:** Hard per-peer limit from the global pool?
   Or soft (advisory, with one peer able to burst into another's share if
   the other is idle)?

2. **Pool sizing default:** What fraction of available memory when no
   prefix maximum, no zefs data, and no PeeringDB? 10%? 25%? Require
   explicit configuration?

3. **Pre-allocation granularity:** One large byte slab? A free-list of
   fwdItem-sized slots? A ring buffer?

4. **RFC 9687 Send Hold Timer value:** max(8 min, 2x HoldTime) per RFC
   recommendation, or configurable?

5. **Prefix-limit burst fraction:** If prefix maximum is 100K, what
   fraction represents "worst expected burst"? 10% (10K items)?
   Configurable per peer or global multiplier?

6. **Advisory report format:** CLI command output? Structured JSON for
   automation? Both?

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
- Replace drop-at-256 with pre-allocated global overflow pool (auto-sized)
- Add write deadline to writeMessage (keepalive bug)
- ~~Set TCP_NODELAY on peer sockets~~ Done (commit 1c43e11d)
- Set IP_TOS/DSCP CS6 (0xC0) on all peer sockets (dialer Control callback + accepted connections)
- Implement RFC 9687 Send Hold Timer
- Add hold timer congestion extension (BIRD technique: extend 10s if RX data pending)
- Add route superseding in overflow pool (replace pending update for same prefix, not append)
- Add TX budget limiting (cap messages per forward batch to prevent one peer starving others)
- Add source-side metrics (overflow ratio, throttle state)
- Add destination-side metrics (overflow items, growth rate, write latency)
- Add read throttling proportional to pool fill AND source overflow ratio
- Add TCP window zero pulsing as escalation
- Add session teardown as last resort (GR-aware)
- Add overflow flush on peer-down
- Add PeeringDB integration for buffer sizing

## Data Flow (MANDATORY)

### Entry Point
- Cached UPDATE via ForwardUpdate from bgp-rs plugin

### Congestion Path
1. ForwardUpdate calls TryDispatch per destination peer
2. Channel full: TryDispatch returns false, item goes to overflow pool
3. Pool slot allocated from pre-allocated global pool (no malloc)
4. Source peer's overflow counter incremented (for ratio tracking)
5. If pool utilization crosses threshold: evaluate source overflow ratios
6. High-ratio sources get sleep between reads or TCP window pulsing
7. Destination worker drains overflow into channel, processes batches
8. Pool utilization drops: throttle eases (lowest-ratio sources first)
9. If pool exhausted and backpressure insufficient: tear down slowest destination

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ForwardUpdate -> overflow pool | Slot allocation from global pool | [ ] |
| ForwardUpdate -> source metrics | Increment overflow counter per source | [ ] |
| Pool fill level -> read throttle | Shared atomic read by session read loops | [ ] |
| Source overflow ratio -> throttle targeting | Per-source ratio checked in read loop | [ ] |
| Pool exhaustion -> teardown | Callback from pool to reactor | [ ] |
| Peer-down -> pool | Return all peer's slots to pool | [ ] |
| Metrics -> Prometheus | Standard Prometheus exposition | [ ] |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ForwardUpdate to slow peer, pool fills | -> | Read throttle activates on high-ratio source peers | test/plugin/forward-congestion-throttle.ci |
| Pool exhausted, backpressure insufficient | -> | Slow destination torn down (GR-aware) | test/plugin/forward-congestion-teardown.ci |
| Peer disconnects with pool slots | -> | Slots returned to pool immediately | TestFwdPool_FlushPeerReturnsSlots |
| Send Hold Timer expires on stuck peer | -> | Session torn down with Error Code 8 | test/plugin/forward-send-hold-timer.ci |
| BGP session established (dial or accept) | -> | IP_TOS=0xC0 set on TCP socket | TestSession_IPTOS_Set |
| Hold timer fires while RX buffer has data | -> | Timer extended 10s, session not torn down | TestSession_HoldTimerCongestionExtension |
| Two updates for same prefix in overflow | -> | Second replaces first, pool item count unchanged | TestFwdPool_RouteSuperseding |
| Forward batch exceeds TX budget | -> | Batch capped, remaining deferred to next cycle | TestFwdPool_TXBudgetLimit |
| Forward batch has withdrawals and announcements (after AC-23 dedup) | -> | Withdrawals written before announcements | TestFwdPool_WithdrawalPriority |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route forwarded to slow peer, channel full | Item stored in overflow pool (never dropped) |
| AC-2 | Overflow pool crosses fill threshold | Source peer read throttle activates, targeted by overflow ratio |
| AC-3 | Read throttle active, pool drains | Throttle eases proportionally, lowest-ratio sources first |
| AC-4 | Overflow pool exhausted, destination still slow | Slowest destination peer session torn down |
| AC-5 | Destination with GR capability torn down | TCP close without NOTIFICATION (route retention) |
| AC-6 | Destination without GR torn down | Cease/Out of Resources NOTIFICATION then close |
| AC-7 | Peer disconnects with overflow items | Pool slots returned, done() called for all items |
| AC-8 | writeMessage called on stuck TCP | Write deadline set (keepalive bug fix) |
| AC-9 | Read throttle duration | Never exceeds source peer keepalive_interval / 6 |
| AC-10 | Pool size configurable | ze.fwd.pool.size env var; auto-sized from RIB/PeeringDB if not set |
| AC-11 | Total memory for slow peers | Bounded by pool size, no growth beyond pre-allocation |
| AC-12 | Fast destination peers during congestion | Unaffected by slow destination -- isolation preserved |
| AC-13 | TCP window zero on source peer | Pulses open 5x per keepalive interval, 2-3s each |
| AC-14 | Remote peer holds window zero toward us | RFC 9687 Send Hold Timer tears down after configured duration |
| AC-15 | TCP_NODELAY set on peer sockets | All BGP peer TCP connections have TCP_NODELAY enabled. Done (commit 1c43e11d). |
| AC-16 | Source peer overflow ratio visible in Prometheus | ze_bgp_overflow_ratio gauge per peer |
| AC-17 | Destination peer overflow depth visible in Prometheus | ze_bgp_overflow_items gauge per peer |
| AC-18 | Pool utilization visible in Prometheus | ze_bgp_pool_used_ratio gauge |
| AC-19 | Per-peer buffer share proportional to prefix count | Peer with 500K prefixes gets larger share than peer with 200 |
| AC-20 | PeeringDB lookup for unknown peers | Query PeeringDB by ASN when no local RIB data, if configured |
| AC-21 | IP_TOS/DSCP CS6 set on all peer sockets | Outgoing (dialer Control callback) and accepted connections set IP_TOS=0xC0 (IPv4) / IPV6_TCLASS=0xC0 (IPv6) |
| AC-22 | Hold timer fires with RX data pending | Hold timer extended 10s instead of teardown (CPU congestion, not peer failure) |
| AC-23 | New update for prefix already in overflow pool | Old entry replaced (route superseding), not appended. Pool item count bounded by unique prefixes. |
| AC-24 | Forward batch to single destination peer | TX budget caps messages per batch (prevents one peer starving others in event loop) |
| AC-25 | Forward batch contains both withdrawals and announcements | Withdrawals sent before announcements (faster convergence). **Requires AC-23 (route superseding) first** -- without per-prefix dedup, reordering can invert announce/withdraw for the same prefix, causing permanent stale routes. |
