# BGP Congestion Handling: Industry Survey

How BGP implementations handle slow peers, stuck TCP, and UPDATE congestion.

## Sources

Source code analysis (March 2026) of six open-source implementations, plus
documentation review of three commercial/vendor implementations.

### Source Code Analyzed

| Implementation | Version | Repository | Key Files | Language |
|---------------|---------|-----------|-----------|----------|
| BIRD 3 | v3.2.0 | [gitlab.nic.cz/labs/bird](https://gitlab.nic.cz/labs/bird/-/tree/v3.2.0) | [bgp.c](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/proto/bgp/bgp.c) (session, keepalive, hold timer), [packets.c](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/proto/bgp/packets.c) (TX dispatch, priority, batching), [bgp.h](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/proto/bgp/bgp.h) (constants, buffer sizes), [io.c](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/sysdep/unix/io.c) (socket I/O, sk_send, poll loop), [sysio.h](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/sysdep/linux/sysio.h) (setsockopt, MD5, TCP-AO, TOS) | C |
| GoBGP | v4.3.0 | [github.com/osrg/gobgp](https://github.com/osrg/gobgp) | [fsm.go](https://github.com/osrg/gobgp/blob/v4.3.0/pkg/server/fsm.go) (FSM, send/recv loops, InfiniteChannel, hold timer), [sockopt_linux.go](https://github.com/osrg/gobgp/blob/v4.3.0/internal/pkg/netutils/sockopt_linux.go) (socket options, MD5, TOS, TTL), [server.go](https://github.com/osrg/gobgp/blob/v4.3.0/pkg/server/server.go) (peer dispatch), [listener.go](https://github.com/osrg/gobgp/blob/v4.3.0/internal/pkg/netutils/listener.go) (listener, accept) | Go |
| RustBGPd | d2a3ab4 | [github.com/osrg/rustybgp](https://github.com/osrg/rustybgp) | [event.rs](https://github.com/osrg/rustybgp/blob/d2a3ab4/daemon/src/event.rs) (peer task, 4-phase write, buffer sizing, channels, holdtime), [auth.rs](https://github.com/osrg/rustybgp/blob/d2a3ab4/daemon/src/auth.rs) (TCP MD5SIG) | Rust |
| FRRouting | frr-10.5.3 | [github.com/FRRouting/frr](https://github.com/FRRouting/frr) | [bgp_network.c](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_network.c) (socket options, MD5, TOS, TTL, SO_SNDBUF), [bgp_io.c](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_io.c) (writev, write quanta, input backpressure), [bgp_packet.c](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_packet.c) (Send Hold Timer, output backpressure, TCP_NODELAY on NOTIFICATION), [bgp_keepalives.c](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_keepalives.c) (dedicated keepalive pthread), [bgpd.h](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgpd.h) (constants, queue limits) | C |
| bio-rd | v0.1.10 | [github.com/bio-routing/bio-rd](https://github.com/bio-routing/bio-rd) | [tcpsock_posix.go](https://github.com/bio-routing/bio-rd/blob/v0.1.10/net/tcp/tcpsock_posix.go) (socket options, TCP_NODELAY, TTL, SO_DONTROUTE), [fsm.go](https://github.com/bio-routing/bio-rd/blob/v0.1.10/protocols/bgp/server/fsm.go) (FSM, message recv, send functions), [fsm_established.go](https://github.com/bio-routing/bio-rd/blob/v0.1.10/protocols/bgp/server/fsm_established.go) (established state, keepalive, hold timer), [update_sender.go](https://github.com/bio-routing/bio-rd/blob/v0.1.10/protocols/bgp/server/update_sender.go) (UPDATE batching and dispatch), [listen.go](https://github.com/bio-routing/bio-rd/blob/v0.1.10/net/tcp/listen.go) (listener, SO_REUSEADDR/PORT), [md5sig.go](https://github.com/bio-routing/bio-rd/blob/v0.1.10/net/tcp/md5sig.go) (TCP MD5SIG) | Go |
| OpenBGPd | 9.0 | [github.com/openbsd/src](https://github.com/openbsd/src/tree/master/usr.sbin/bgpd) | [session.c](https://github.com/openbsd/src/blob/master/usr.sbin/bgpd/session.c) (socket setup, event loop, write path, backpressure XOFF/XON), [session_bgp.c](https://github.com/openbsd/src/blob/master/usr.sbin/bgpd/session_bgp.c) (FSM, Send Hold Timer, keepalive, hold timer), [bgpd.h](https://github.com/openbsd/src/blob/master/usr.sbin/bgpd/bgpd.h) (constants, error codes, watermarks), [pfkey.c](https://github.com/openbsd/src/blob/master/usr.sbin/bgpd/pfkey.c) (TCP MD5SIG) | C |

### Documentation Reviewed

| Implementation | Source |
|---------------|--------|
| Cisco IOS-XR | Cisco documentation on update groups and slow-peer detection |
| Juniper Junos | Junos documentation on output queues and keepalive threading |
| Nokia SR OS | Ben Cox (2021) "BGP Zombies" research on TCP window zero behavior |
| Nokia SR OS | Ben Cox (2021) "BGP Zombies" research on TCP window zero behavior |

### Ze Source Files

| Feature | Ze File |
|---------|---------|
| TCP_NODELAY + IP_TOS | `internal/component/bgp/reactor/session_connection.go` -- connectionEstablished() |
| Write deadline | `internal/component/bgp/reactor/session_write.go` -- writeMessage() |
| Send Hold Timer (RFC 9687) | `internal/component/bgp/reactor/session_write.go` -- sendHoldTimer methods |
| Hold timer congestion ext. | `internal/component/bgp/reactor/session.go` -- OnHoldTimerExpires callback |
| Forward pool batch writes | `internal/component/bgp/reactor/forward_pool.go` -- fwdBatchHandler() |
| NOTIFICATION Error Code 8 | `internal/component/bgp/message/notification.go` -- NotifySendHoldTimerExpired |

## Universal Rule

No production BGP implementation drops routes silently. Every implementation
chooses unbounded memory growth or backpressure over route loss. Silent discard
causes permanent RIB divergence with no recovery mechanism.

## Socket Options

| Option | BIRD | GoBGP | RustBGPd | FRR | OpenBGPd | bio-rd | Ze |
|--------|------|-------|----------|-----|----------|--------|-----|
| TCP_NODELAY | No | No | No | NOTIFICATION only | **Yes (all traffic)** | **Yes (all traffic)** | **Yes (all traffic)** |
| IP_TOS/DSCP | Yes (CS6) | Yes (configurable) | No | Yes (configurable) | Yes (configurable) | No | **Yes (CS6)** |
| TCP_MD5SIG | Yes | Yes (Linux) | Yes (Linux) | Yes | Yes | Yes (Linux) | Yes |
| TCP-AO (RFC 5925) | Yes | No | No | No | No | No | Not yet |
| GTSM (IP_MINTTL) | Yes | Yes | No | Yes | Yes | Yes (via TTL) | Yes |
| SO_SNDBUF | No | No | Reads, doesn't set | Yes (configurable) | Yes (65535) | No | Not yet |
| SO_RCVBUF | No (TCP) | No | No | Yes (configurable) | Yes (65535) | No | Not yet |
| SO_REUSEPORT | No | No | Yes | Yes | No | Yes (optional) | Not yet |
| SO_KEEPALIVE | No | Disabled | No | Yes (tunable) | No | No | No (BGP timers) |
| TCP MSS | No | Yes | No | Yes | No | No | No |
| SO_DONTROUTE | No | No | No | No | No | Yes (EBGP) | No |
| MPTCP | No | Disabled (MD5 conflict) | No | No | No | No | No |

*Sources: BIRD [bgp.c](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/proto/bgp/bgp.c), GoBGP [sockopt_linux.go](https://github.com/osrg/gobgp/blob/v4.3.0/internal/pkg/netutils/sockopt_linux.go), RustBGPd [event.rs](https://github.com/osrg/rustybgp/blob/d2a3ab4/daemon/src/event.rs), FRR [bgp_network.c](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_network.c)*

**TCP_NODELAY:** Disables Nagle's algorithm. Ze sets it on all BGP connections.
FRR only sets it before sending NOTIFICATION ([bgp_packet.c:755](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_packet.c#L755))
to force immediate delivery of error messages. BIRD, GoBGP, and RustBGPd do not
set it at all.

**TCP_NODELAY:** Disables Nagle's algorithm. BGP messages are application-framed
and flushed explicitly. Nagle adds latency with no throughput benefit. The
Nagle/Delayed-ACK interaction can add 200ms+ to small message delivery.

**IP_TOS 0xC0 (DSCP CS6):** DSCP CS6 (Internet Control, RFC 4594 Section 3.1)
is the standard marking for network control traffic including routing protocols.
Tells network devices to prioritize BGP traffic over regular data. Under network
congestion, routers with QoS policies preferentially forward BGP packets, reducing
hold timer expiry risk from packet loss. BIRD sets `IP_PREC_INTERNET_CONTROL`
on all BGP sockets ([bgp.c:229](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/proto/bgp/bgp.c#L229), [bgp.c:1827](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/proto/bgp/bgp.c#L1827)).

## Outgoing Queue Architecture

Every implementation uses unbounded queues. None have real backpressure from
the TX path back to the routing table.

| Implementation | Queue Type | Bounded? | Route Superseding |
|---------------|-----------|----------|-------------------|
| BIRD | Linked list of `bgp_bucket` structs | No | Yes (prefix hash dedup) |
| GoBGP | `InfiniteChannel` (eapache/channels) | No | No |
| RustBGPd | Unbounded `mpsc` channels (tokio) | No | Partial (per-batch dedup) |
| FRR | `stream_fifo` per peer (obuf) | Yes (outq_limit default 10K) | Yes (update groups) |
| OpenBGPd | `msgbuf` per peer (wbuf) | XOFF at 2000 bytes, XON at 500 | No |
| bio-rd | Unbounded `toSend` map per address family | No | No |
| Ze (current) | Per-peer channel + unbounded overflow | No | Not yet (planned) |

*Sources: BIRD `packets.c` (bgp_bucket), GoBGP `fsm.go` (InfiniteChannel), RustBGPd `event.rs` (mpsc)*

**Route superseding** (BIRD): When a new route arrives for a prefix already
queued for sending, the old entry is replaced instead of appended. This bounds
queue growth to the number of unique prefixes rather than the number of updates.
Implemented via a prefix hash table (`bgp_get_prefix`) that finds existing entries
and moves them to the new attribute bucket.

**RustBGPd partial superseding:** The `PendingTx` structure deduplicates by
attribute set within a batch (routes sharing attributes are grouped into one
UPDATE), but the channel queue from table threads is not deduped.

## Write Path

| Property | BIRD | GoBGP | RustBGPd | FRR | OpenBGPd | bio-rd | Ze |
|----------|------|-------|----------|-----|----------|--------|-----|
| TX buffer | 4KB or 65KB (RFC 8654) | None (direct write) | min(64KB, SO_SNDBUF/2) | stream_fifo (obuf) | msgbuf (ibuf queue) | None (unix.Write) | 16KB bufio.Writer |
| Messages per write | 1 per sk_send() | 1 per conn.Write() | Batched by txbuf_size | writev() up to 64 msgs | ibuf_write (msgbuf flush) | 1 per unix.Write() | Batched by bufio flush |
| TX budget per cycle | 1024 messages | None | 2048 attribute records | 64 (configurable 1-64) | 25 (MSG_PROCESS_LIMIT) | None | Per forward batch |
| Write deadline | N/A (non-blocking) | 1s handshake, none ESTABLISHED | None | Send Hold Timer check on enqueue | Send Hold Timer (holdtime) | None | 30s forward, 10-30s control |
| Write failure | Retry on POLLOUT | Fatal (kills peer) | Fatal (kills task) | EAGAIN retry, else TCP_fatal_error | EPIPE/EIO: EVNT_CON_FATAL | KEEPALIVEs fatal, UPDATEs ignored | Logged, triggers FSM |

*Sources: BIRD [io.c](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/sysdep/unix/io.c) (sk_send), GoBGP [fsm.go:1734](https://github.com/osrg/gobgp/blob/v4.3.0/pkg/server/fsm.go#L1734), RustBGPd [event.rs:3731](https://github.com/osrg/rustybgp/blob/d2a3ab4/daemon/src/event.rs#L3731), FRR [bgp_io.c:321](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_io.c#L321) (bgp_write, writev)*

**FRR writev():** Packs up to `wpkt_quanta` (default 64) messages into an iovec
array and sends them in a single `writev()` syscall
([bgp_io.c:365](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_io.c#L365)).
Handles partial writes by adjusting stream positions and retrying. Configurable
via CLI: `write-quanta (1-64)`.

### Write Priority

| Priority | BIRD | RustBGPd | Ze (planned) |
|----------|------|----------|-------------|
| 1 (highest) | CLOSE | OPEN/KEEPALIVE/NOTIFICATION (urgent Vec) | N/A (separate paths) |
| 2 | NOTIFICATION | Withdrawals | Withdrawals (planned AC-25) |
| 3 | OPEN | Announcements | Announcements |
| 4 | KEEPALIVE | EOR (End-of-RIB) | EOR |
| 5 (lowest) | UPDATE | - | - |

**BIRD** uses a bitmask (`packets_to_send`) checked in fixed priority order in
`bgp_fire_tx()`. KEEPALIVE is checked before UPDATE, but all share a single 4KB
TX buffer. When the buffer is full, priority is meaningless -- nothing can be written.

**RustBGPd** uses 4 explicit write phases in the peer task event loop. The `urgent`
Vec is drained completely before any route data. Withdrawals go before announcements
for faster convergence: a late withdrawal means the remote peer continues forwarding
to a dead next-hop (active traffic loss), while a late announcement means traffic
takes a longer path but still arrives (suboptimal routing). This reordering is safe
in RustBGPd because `PendingTx` deduplicates by prefix -- a withdrawal for an
already-queued announcement replaces it, so the same prefix never appears in both
sets. Without per-prefix dedup, reordering can invert an announce/withdraw sequence
for the same prefix, causing permanent stale routes at the remote peer.

## Keepalive Under Congestion

RFC 4271 Section 8.2.2 (Established state) specifies the hold timer resets on **both KEEPALIVE (Event 26) and UPDATE (Event 27)**.
All implementations (BIRD, GoBGP, RustBGPd, ze) implement this correctly.

| Scenario | Impact |
|----------|--------|
| Sending UPDATEs, keepalive delayed | Remote hold timer reset by UPDATEs. Session alive. |
| Session idle, no contention | Keepalive goes out trivially. |
| TCP fully stuck | Neither UPDATEs nor keepalives reach remote. RFC 9687 detects this. |

**Delayed keepalives are a non-problem when UPDATEs flow.** Priority systems (BIRD
bitmask, RustBGPd phases) help with ordering but cannot overcome a full TCP send
buffer. The real defense against stuck TCP is RFC 9687 Send Hold Timer.

| Implementation | Keepalive mechanism | Dedicated thread? |
|---------------|-------------------|-------------------|
| BIRD | Same event loop, priority bitmask | No |
| GoBGP | Same goroutine, InfiniteChannel | No |
| RustBGPd | Same tokio task, urgent Vec | No |
| FRR | Dedicated keepalive pthread | **Yes** |
| OpenBGPd | Same event loop, timer-driven | No |
| bio-rd | Same FSM goroutine, time.NewTimer | No |
| Junos | Dedicated kernel thread | **Yes** |
| Ze | Same write path (writeMu), timer callback | No |

**FRR's dedicated keepalive pthread**
([bgp_keepalives.c](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_keepalives.c)):
Separate thread with its own hash table of peers and monotonic clock. Calculates
elapsed time since last keepalive per peer, sends 100ms early to batch similar-timed
keepalives. Uses `pthread_cond_timedwait` to sleep until the next peer needs service.
Completely independent of the I/O thread -- keepalives are never blocked by UPDATE
processing or write contention. This guarantees keepalive delivery even if the main
thread is stuck in CPU-heavy processing. However, as discussed in "Keepalive Under
Congestion" above, this solves an edge case -- the primary defense against stuck TCP
is the Send Hold Timer.

## Send Hold Timer (RFC 9687)

Detects when the local side cannot send any data. If no message is successfully
written for the configured duration, the session is torn down.

| Implementation | Implemented? | Duration | On Expiry |
|---------------|-------------|----------|-----------|
| BIRD | Yes | Default 2x hold_time, configurable | Hard disconnect (no NOTIFICATION -- can't send) |
| FRR | Yes | 2x holdtime (checked on every packet enqueue) | NOTIFICATION + teardown. Warning at 1x holdtime. |
| OpenBGPd | Yes | max(holdtime, 90s). Reset after every successful ibuf_write. | NOTIFICATION Error Code 8, then IDLE. GR-aware: triggers graceful restart path if GR negotiated. |
| GoBGP | **No** | - | Stuck write blocks sendMessageloop forever |
| RustBGPd | **No** | - | Stuck write_all blocks peer task forever |
| bio-rd | **No** | - | Stuck unix.Write blocks FSM goroutine forever |
| Ze | **Yes** | max(8min, 2x holdTime) | Try NOTIFICATION Code 8, then close |

*Sources: BIRD [bgp.c:1741](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/proto/bgp/bgp.c#L1741) (bgp_send_hold_timeout), FRR [bgp_packet.c:109](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_packet.c#L109) (bgp_packet_add), ze `session_write.go` (sendHoldTimerExpired)*

**FRR's approach is unique:** no separate timer. `bgp_packet_add()` checks
`last_sendq_ok` (timestamp of last empty output queue) on every packet enqueue.
If `delta > 2 * holdtime`: teardown. If `delta > holdtime`: warning (throttled
to every 5s). Piggybacks on keepalive send path since keepalives are enqueued
periodically.

**GoBGP and RustBGPd lack Send Hold Timer protection.**

GoBGP clears the write deadline on entering ESTABLISHED
([fsm.go:1903](https://github.com/osrg/gobgp/blob/v4.3.0/pkg/server/fsm.go#L1903):
`SetWriteDeadline(time.Time{})`). A blocked `conn.Write()` in the send goroutine
([fsm.go:1734](https://github.com/osrg/gobgp/blob/v4.3.0/pkg/server/fsm.go#L1734))
hangs indefinitely. The inbound hold timer still runs in a separate goroutine and
will eventually close the connection if the remote also stops sending -- but there
is no detection of outbound-only stalls (the case where we receive but cannot send).

RustBGPd is more vulnerable: `write_all().await`
([event.rs:3704](https://github.com/osrg/rustybgp/blob/d2a3ab4/daemon/src/event.rs#L3704))
suspends the entire `select_biased!` event loop for that peer. While blocked on
write, the keepalive timer, holdtime check, and management channel in the same
select loop cannot fire. The peer task is completely unresponsive until TCP either
delivers the data or errors out.

## Hold Timer Congestion Extension

When the hold timer fires, check if there is unread data from the peer. If yes,
the daemon is CPU-congested (busy processing other peers), not the remote peer.
Extend the timer instead of tearing down.

| Implementation | Technique |
|---------------|-----------|
| BIRD | `sk_rx_ready()` (poll with zero timeout). If data pending, extend by 10 seconds only. |
| Ze | Atomic `recentRead` flag set by read loop. If true, reset to full hold duration. Difference from BIRD: ze resets the full timer rather than adding a fixed 10s extension, so under sustained CPU congestion BIRD's timer erodes while ze's does not. |
| Others | Not implemented |

*Source: BIRD [bgp.c:1712-1716](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/proto/bgp/bgp.c#L1712-1716)*

This prevents false hold timer expirations when the event loop is overloaded
processing a burst of UPDATEs from other peers. The extension is safe because
receiving data proves the peer is alive.

## Backpressure Mechanisms

| Layer | BIRD | GoBGP | RustBGPd | FRR | OpenBGPd | bio-rd | Ze (planned) |
|-------|------|-------|----------|-----|----------|--------|-------------|
| Channel buffer | N/A | InfiniteChannel | Unbounded mpsc | stream_fifo (obuf) | msgbuf (wbuf) | Unbuffered channels | 64-item channel |
| Queue limit | Unbounded | Unbounded | Unbounded | outq_limit (default 10K), inq_limit (default 10K) | XOFF at 2000B, XON at 500B | Unbounded map | Pre-allocated pool |
| Read throttling | None | None | None | Stops reading at inq_limit (TCP window shrinks) | None | None (unbuffered chan blocks) | Sleep + TCP window zero |
| Write throttling | None | None | None | Stops generating UPDATEs at outq_limit | XOFF signal pauses RDE route generation | None | N/A (pool-based) |
| Session teardown | Hold timer only | Write failure (fatal) | Write failure (fatal) | Send Hold Timer (2x holdtime) | Send Hold Timer (holdtime). GR-aware. | KEEPALIVE failure only | GR-aware teardown |

*Sources: FRR [bgp_io.c:171](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_io.c#L171) (inq_limit check), [bgp_packet.c:471](https://github.com/FRRouting/frr/blob/frr-10.5.3/bgpd/bgp_packet.c#L471) (outq_limit check)*

**FRR and OpenBGPd are the only open-source BGP daemons with real backpressure.**

FRR implements configurable input and output queue limits (default 10,000 messages
each, CLI: `bgp input-queue-limit`, `bgp output-queue-limit`). When `inq_limit` is
reached, reading stops and TCP window shrinks naturally. When `outq_limit` is
reached, UPDATE generation pauses.

OpenBGPd implements XOFF/XON flow control between the session process and the
route decision engine (RDE). When the per-peer write buffer exceeds 2000 bytes
(`SESS_MSG_HIGH_MARK`), an XOFF signal pauses route generation. When it drops
below 500 bytes (`SESS_MSG_LOW_MARK`), XON resumes it
([session.c:407-427](https://github.com/openbsd/src/blob/master/usr.sbin/bgpd/session.c#L407)).

BIRD, GoBGP, and RustBGPd all use unbounded queues and rely solely on TCP flow
control.

Ze's planned four-layer design (channel buffer, pre-allocated overflow pool,
read throttling, GR-aware teardown) is novel in the BGP ecosystem.

## Fairness

| Mechanism | BIRD | RustBGPd | FRR | OpenBGPd | bio-rd | Ze (planned) |
|-----------|------|----------|-----|----------|--------|-------------|
| Per-peer isolation | Single-threaded, round-robin | Per-peer tokio task | Per-peer I/O events + update groups | Single-threaded, poll loop | Per-peer FSM goroutine | Per-peer worker goroutine |
| Cross-family fairness | 16-message stickiness, then rotate | All families in same write loop | Update groups batch by shared attributes | Serial per peer | Per-family update sender | Per forward batch |
| TX budget | 1024 messages per event cycle | 2048 attribute records per iteration | 64 messages per writev (configurable 1-64) | 25 (MSG_PROCESS_LIMIT) | None (ticker-based batching) | Planned (AC-24) |
| fast_rx during handshake | Yes (priority reads for OPEN/KEEPALIVE) | No | No | No | No | No |

*Source: BIRD [packets.c:3063](https://gitlab.nic.cz/labs/bird/-/blob/v3.2.0/proto/bgp/packets.c#L3063) (bgp_get_channel_to_send)*

**BIRD's fast_rx:** During OPEN/OPENCONFIRM, sockets are marked `fast_rx=1` and get
priority processing (up to 4 reads per poll cycle). On entering ESTABLISHED, fast_rx
is cleared and the socket shares the event loop fairly with other protocols.

## Summary: Where Ze Stands

| Capability | Industry Status | Ze Status |
|-----------|----------------|-----------|
| TCP_NODELAY (all traffic) | Ze, OpenBGPd, bio-rd (FRR: NOTIFICATION only) | **Done** |
| IP_TOS DSCP | BIRD, GoBGP, FRR, OpenBGPd | **Done** |
| Send Hold Timer (RFC 9687) | BIRD, FRR, OpenBGPd (GR-aware) | **Done** |
| Hold timer congestion extension | BIRD only | **Done** |
| Write deadline on control messages | No implementation does this explicitly | **Done** |
| Buffer pooling + bufio batching | No implementation does both | **Done** (existing) |
| Route superseding | BIRD, FRR | Planned |
| Withdrawal priority | RustBGPd (safe due to per-prefix dedup) | Planned (AC-25, requires AC-23 route superseding first) |
| TX budget | BIRD (1024), RustBGPd (2048) | Planned (AC-24) |
| Real backpressure | FRR (queue limits), OpenBGPd (XOFF/XON) | Planned (4-layer design) |
| GR-aware congestion teardown | None explicitly | Planned |
