# BGP Implementation Reference: BIRD as a Complement to the Existing Comparison

## How to read this document

The ze repo already contains a BIRD analysis under
`docs/research/comparison/bird/` (22 files, about 8000 lines total). That
analysis is a high-level comparative survey: protocol vtable patterns,
filter language overview, attribute dedup summary, and so on. It was
largely written against BIRD 2.x, the long-stable single-threaded branch.
This document exists because BIRD 3.x is a substantially different
daemon, and because a ze BGP implementer needs a deeper look at the
parts that matter: the per-protocol event loop, the unified attribute
model, the lockfree route table journal, and the Adj-RIB-Out bucket
system that drives BIRD's IX route-server scalability.

Treat this as an overlay on top of `comparison/bird/` and
`bgp-implementations-analysis.md`. Where those documents say "BIRD
interns attributes" or "BIRD has a filter language", this one tells you
exactly which function does it and what the tradeoffs are. It is also
the BGP counterpart to `isis-frr-reference.md`: same discipline, same
"file:line citation or it did not happen" rule, same "no verbatim
source" rule.

All citations refer to BIRD tag `v3.2.1` at commit
`2500b450b404ab41eeff3b2d92ba943d0178fb50`. Line numbers are valid at
that tag. BIRD moves; verify before copying an exact number into a
commit message. Following the same clean-room discipline as
`isis-frr-reference.md`, algorithms are described in prose and short
signatures, not by quoting BIRD source. Struct-field and function-signature
citations exist so the reference can be verified, not copied.

### The two BIRD versions question

BIRD 2.x and BIRD 3.x are not the same daemon. The jump to v3 was a
multi-year rewrite ("the journey to threads") that reorganised the
route table, attribute system, protocol framework, and I/O layer
for concurrent per-protocol execution. The existing
`docs/research/comparison/bird/00-overview.md` says BIRD is
"single-threaded (except BFD)"; that is accurate for 2.x and wrong
for 3.x. For ze, 3.x is the interesting version, because ze is
already a goroutine-per-peer design. Throughout this document "BIRD"
means "BIRD 3.2.1" unless explicitly qualified.

---

## 1. BIRD 3.2.1 BGP at a glance

Line counts in the BGP daemon proper (verified against
`wc -l proto/bgp/*.c proto/bgp/*.h`):

| File | LOC | Purpose |
|------|----:|---------|
| `proto/bgp/bgp.c` | 4144 | Protocol lifecycle, FSM, connection management, config apply, reconfigure |
| `proto/bgp/packets.c` | 3646 | Wire encode and decode, RX dispatch, TX scheduling |
| `proto/bgp/attrs.c` | 3043 | BGP attribute codec hooks, best-path selection, next-hop resolution |
| `proto/bgp/bgp.h` | 984 | Data model, constants, enum values |
| `proto/bgp/config.Y` | (Bison) | Lex/Yacc grammar integration |

Total BGP-specific code: about 11,800 lines across three .c files plus
the header. Compared to FRR's BGP daemon (about 180k lines in
`bgpd/`), BIRD's BGP is dramatically smaller. The size difference is
partly because FRR reimplements a lot of the protocol-adjacent
machinery (VTY, northbound, topotests, BGP daemon-private YANG
bindings) that BIRD pushes into the shared `nest/`, `filter/`, `conf/`,
and `lib/` directories, and partly because BIRD skips features FRR
supports (EVPN, VPNv4 as a first-class service, extensive VRF tooling).

The shared-infrastructure files that any serious reader of BIRD BGP
ends up in:

| File | LOC | Role |
|------|----:|------|
| `nest/rt-table.c` | 5804 | The core routing table: import, announce, best-path recompute |
| `nest/rt-attr.c` | 1794 | The `ea_list` model, global attribute interning, reference counting |
| `nest/rt-export.c` | 630 | Export journal drain, per-feeder refeed |
| `nest/proto.c` | 3448 | Protocol and channel framework |
| `nest/protocol.h` | 794 | The proto and channel contracts, state constants |
| `nest/route.h` | 941 | `rte`, `rte_storage`, `ea_list`, `eattr`, `ea_class`, `rt_exporter` |
| `sysdep/unix/io-loop.c` | 2306 | The birdloop event-loop implementation, thread groups |
| `lib/io-loop.h` | 134 | The birdloop API |
| `lib/locking.h` | 532 | DOMAIN locks, the locking order, `BLO_LOCK` |
| `lib/rcu.c` | 106 | RCU primitive (read-side + grace period) |
| `lib/lockfree.c` | 648 | Lockfree journal (`lfjour`), lockfree use-count (`lfuc`) |
| `lib/netindex.c` | 338 | Shared prefix-to-index hash |

Adding BMP support: `proto/bmp/bmp.c` (1613 LOC) leans on BGP's
`bgp_bmp_encode_rte` helper (see section 24).

The takeaway for scoping a ze BGP reimplementation: a ze BGP core of
12k-15k Go lines is realistic if ze reuses the same separation (BGP
code stays in the BGP package, table infrastructure lives in a shared
rib package, attribute pool lives in a shared attrs package). If ze
keeps everything inside `internal/component/bgp/`, expect the numbers
to be larger.

---

## 1.5. How BIRD organises the BGP code

A reader coming from bio-rd, from FRR's bgpd, or from ze's
`internal/component/bgp/` will find BIRD's three-file BGP unusual.
bio-rd uses roughly 30 files split by concern; FRR uses about 60 files
split by concern plus feature; ze has subpackages for reactor, wire,
message, capability, fsm, and more. BIRD compresses all of that into
`bgp.c`, `packets.c`, and `attrs.c` plus the header. Reading the
decomposition gives you the mental map you need to navigate the source
without thrashing.

### The rule BIRD follows

BIRD splits a BGP source file when the split gives you either a
testable boundary or a reusable abstraction. It does not split for
file-size cosmetics. All three BGP files are `static`-heavy, using the
same trick FRR uses in `isis_tlvs.c` (section 1.5 of
`isis-frr-reference.md`): keep helpers file-private so they do not
pollute the header.

### What goes in each file

**`bgp.c`** owns the **protocol lifecycle**: protocol start and stop
(`bgp_start` at `proto/bgp/bgp.c:2717`, `bgp_stop` at
`proto/bgp/bgp.c:1248`, `bgp_shutdown` at `proto/bgp/bgp.c:2850`), the
connection FSM transitions (`bgp_conn_enter_established_state` at
`proto/bgp/bgp.c:1290`, `bgp_conn_enter_close_state` at
`proto/bgp/bgp.c:1491`, `bgp_conn_enter_idle_state` at
`proto/bgp/bgp.c:1508`), the Active/Connect transitions (`bgp_active`
at `proto/bgp/bgp.c:1915`, `bgp_connect` at `proto/bgp/bgp.c:1935`),
the timer callbacks (`bgp_hold_timeout` at `proto/bgp/bgp.c:1807`,
`bgp_keepalive_timeout` at `proto/bgp/bgp.c:1838`,
`bgp_send_hold_timeout` at `proto/bgp/bgp.c:1847`), the
incoming-connection handler (`bgp_incoming_connection` at
`proto/bgp/bgp.c:2058`), the shared-listen-socket machinery
(`bgp_open` at `proto/bgp/bgp.c:196`, `bgp_listen_open` at
`proto/bgp/bgp.c:249`), and the reconfigure entry point
(`bgp_reconfigure` at `proto/bgp/bgp.c:3460`). If it changes protocol
state or socket state, it lives here.

**`packets.c`** owns the **wire format**: encode and decode of every
BGP message type, the capability TLV codecs, the FSM transitions
driven by the packet processor, and the RX/TX loops. `bgp_rx_open` at
`proto/bgp/packets.c:892`, `bgp_rx_update` at `proto/bgp/packets.c:2818`,
`bgp_rx_notification` at `proto/bgp/packets.c:3485`, `bgp_rx_keepalive`
at `proto/bgp/packets.c:3525`, `bgp_rx_route_refresh` at
`proto/bgp/packets.c:3033`. Transmit: `bgp_fire_tx` at
`proto/bgp/packets.c:3144`, `bgp_create_update` at
`proto/bgp/packets.c:2609`, `bgp_create_end_mark` at
`proto/bgp/packets.c:2741`. Capabilities: `bgp_prepare_capabilities`
at `proto/bgp/packets.c:248`, `bgp_read_capabilities` at
`proto/bgp/packets.c:484`, `bgp_check_capabilities` at
`proto/bgp/packets.c:699`.

**`attrs.c`** owns **everything attribute-shaped**: the per-attribute
codec table (`bgp_attr_table` near `proto/bgp/attrs.c:1058`), the
decode dispatcher (`bgp_decode_attr` at `proto/bgp/attrs.c:1530`),
the UPDATE-level decoder (`bgp_decode_attrs` at
`proto/bgp/attrs.c:1564`), the encoder (`bgp_encode_attrs` at
`proto/bgp/attrs.c:1482`), the export-time attribute
rewriter (`bgp_update_attrs` at `proto/bgp/attrs.c:2338`), the
best-path comparator (`bgp_rte_better` at `proto/bgp/attrs.c:2525`),
and the deterministic-MED recalculator (`bgp_rte_recalculate` at
`proto/bgp/attrs.c:2762`). Next-hop rewriting (`bgp_update_next_hop`)
and the BMP encode helper (`bgp_bmp_encode_rte` at
`proto/bgp/packets.c:2576`, sitting in packets.c only because it
reuses the bucket writer) round it out.

### What is NOT in proto/bgp/

BIRD keeps route storage, attribute interning, filter execution,
socket I/O, AS-path and community helpers, and BMP **outside** the
BGP daemon, in `nest/`, `filter/`, `lib/`, `sysdep/unix/`, and
`proto/bmp/` respectively. The rule: anything another protocol could
plausibly use moves up a level. This is a direct consequence of the
protocol-agnostic route table in `nest/`. FRR makes the opposite
choice and `bgpd/` balloons as a result.

Compared to the FRR IS-IS reference's decomposition axes (by
subsystem, by feature, by operator surface), BIRD BGP collapses all
three: three files by subsystem, no per-feature files (GR, LLGR,
ADD-PATH, RR live inline), exactly one operator surface (`birdc`).
For a ze implementer, the lesson is that operator-surface code is
a large fraction of a real daemon, and its cost is often hidden when
comparing "core protocol engine" line counts.

---

## 2. Top-level BGP data model

The BGP data model lives in `proto/bgp/bgp.h`. It is denser than
bio-rd's equivalent but much sparser than FRR's, largely because BIRD
pushes everything route-shaped out to `nest/`. The hierarchy:

```
bgp_proto              (one per configured peer; inherits proto)
  struct proto p       (the protocol framework hook)
  struct bgp_conn      outgoing_conn  (the FSM slot BIRD owns)
  struct bgp_conn      incoming_conn  (the FSM slot the peer initiates)
  struct bgp_conn *conn (the winning slot once collision is resolved)
  bgp_channel[]        (one per negotiated AFI/SAFI, via proto->channels)
  list listen          (shared listen-socket requests)
  bgp_ao_state ao      (TCP-AO keys, if enabled)
  timer *startup_timer (restart delay after previous error)
  timer *gr_timer      (graceful-restart timer)
```

Verified against `proto/bgp/bgp.h:419-470`. The hierarchy's key feature
is that a `bgp_proto` is not a single peer state machine but a
**container for two half-FSMs** (plus the winner). BIRD's collision
resolution lives entirely inside `bgp_rx_open` and rearranges which
`bgp_conn` becomes `conn` once both sides have exchanged OPENs. Section
6 covers the algorithm.

### `bgp_proto` fields that matter

At `proto/bgp/bgp.h:419-470`:

- `struct proto p`: inherited from the protocol framework. `p.channels`
  is the list of per-AFI channels. `p.ea_state` is the protocol state
  journal (section 17). `p.name`, `p.debug`, `p.main_source` come from
  the framework.
- `ip_addr local_ip, remote_ip`: the peer addressing. `local_ip` can be
  `IPA_NONE` until the kernel picks a source address.
- `u32 local_as, remote_as, public_as`: the ASNs. `public_as` differs
  from `local_as` when confederations are active: the public ASN is
  the confederation ID, `local_as` is the member ASN.
- `u32 local_id, remote_id`: BGP identifiers.
- `u32 rr_cluster_id`: route-reflector cluster ID, distinct from
  `local_id` for topology-aware RR deployments.
- Boolean flags: `is_internal` (iBGP), `is_interior` (iBGP or
  intra-confederation), `as4_session`, `rr_client`, `rs_client`,
  `ipv4`, `passive`, `route_refresh`, `enhanced_refresh`, `gr_ready`,
  `llgr_ready`, `gr_active_num` (non-zero while peer is
  mid-graceful-restart).
- `u32 *afi_map`, `struct bgp_channel **channel_map`: indexed arrays
  for O(1) lookup from channel index (a compact integer) to
  AFI/SAFI and channel pointer.
- `struct bgp_conn *conn, struct bgp_conn outgoing_conn, struct bgp_conn incoming_conn`:
  see section 6.
- `list listen`: the peer's requests for a listening socket. See
  section 7.
- `struct bfd_request_ref *bfd_req`, `callback bfd_notify`: BFD
  integration. Shared with the `bfd` protocol in `proto/bfd/`.
- `struct bgp_ao_state ao`: TCP-AO (RFC 5925) key table. Replaces
  MD5 in modern deployments.
- `struct rt_uncork_callback uncork`: route table uncork hook. BIRD's
  backpressure mechanism (section 16).
- `btime last_established, last_rx_update`: operator-visible
  timestamps.
- `timer *startup_timer`: fires after `startup_delay` seconds if the
  previous session ended in error. Prevents flap loops.
- `timer *gr_timer`: fires when the peer's advertised graceful-restart
  interval has elapsed without reestablishment.
- `u8 last_error_class, u32 last_error_code`: the reason the last
  session ended.

### `bgp_conn`: the per-connection FSM slot

At `proto/bgp/bgp.h:354-381`:

- `struct birdsock *sk`: the TCP socket (BIRD's socket abstraction
  around a real file descriptor plus buffers).
- `u8 state`: one of `BS_IDLE`, `BS_CONNECT`, `BS_ACTIVE`, `BS_OPENSENT`,
  `BS_OPENCONFIRM`, `BS_ESTABLISHED`, `BS_CLOSE` (`proto/bgp/bgp.h:887-894`).
  `BS_CLOSE` is a BIRD-specific transition state used while a
  NOTIFICATION is draining.
- `byte *local_open_msg, *remote_open_msg`: saved OPEN messages for
  collision detection, LLGR replay, and BMP peer-up dumps. `bgp_caps
  *local_caps, *remote_caps` carry the parsed capabilities.
- `timer *connect_timer, *hold_timer, *keepalive_timer,
  *send_hold_timer`: four independent timers per connection. Note
  that each BGP peer has two `bgp_conn`s, each with its own four
  timers, so a full peer has eight timers on the event loop at any
  time. This is normal for BIRD because timers are cheap and canceling
  a stale one is trivial.
- `u32 packets_to_send`: a bitmap over `PKT_*` types. Setting a bit
  schedules that type of packet for transmission.
  `bgp_schedule_packet` is the enqueue function.
- `u32 channels_to_send`: a bitmap over BGP channel indices, indicating
  which channels have pending UPDATEs. Used by `bgp_fire_tx`
  (`proto/bgp/packets.c:3144`) to pick the next channel to drain in a
  fair round-robin.
- `u8 last_channel, last_channel_count`: the round-robin rotation
  state (section 19).
- `int notify_code, notify_subcode; byte *notify_data; int notify_size`:
  pending NOTIFICATION payload when an error is on the way out.

### `bgp_channel`: per-AFI state

At `proto/bgp/bgp.h:476-521`:

- `struct channel c`: the framework channel. Carries the route table
  binding, in/out filters, import and export state.
- `u32 afi`, `const struct bgp_af_desc *desc`: which AFI/SAFI this
  channel is and the codec descriptor for that AF. `bgp_af_desc` at
  `proto/bgp/bgp.h:62-73` is a small vtable of four function
  pointers (`encode_nlri`, `decode_nlri`, `encode_next_hop`,
  `decode_next_hop`) plus an `update_next_hop` hook. Each AF (IPv4
  unicast, IPv6 unicast, VPN4, VPN6, flow4, flow6, etc.) has one.
- `rtable *igp_table_ip4, *igp_table_ip6`: tables used for recursive
  next-hop lookup.
- `rtable *base_table`: used by flowspec for validation.
- `union bgp_ptx *tx`: the per-channel TX encapsulation (the
  Adj-RIB-Out). Section 18 dissects this.
- `u8 ext_next_hop, gr_ready, gr_active, add_path_rx, add_path_tx`:
  negotiated per-AF capabilities.
- `timer *stale_timer`: LLGR stale timer. Fires when the LLGR hold
  time elapses after a session loss.
- `struct rt_export_feeder stale_feed`, `event stale_event`: the
  feeder state that walks the table marking routes stale when LLGR
  triggers. See section 20.
- `struct rt_feeding_request enhanced_rr_request`, `event
  enhanced_rr_restart`: the refeed request used for enhanced route
  refresh (RFC 7313). See section 21.
- `u8 feed_state, load_state`: `BFS_NONE`, `BFS_LOADING`,
  `BFS_LOADED`, `BFS_REFRESHING`, `BFS_REFRESHED`
  (`proto/bgp/bgp.h:937-941`). Tracks whether an End-of-RIB or EoRR
  marker is pending.

### Where ze differs

ze's current BGP reactor has a single FSM per peer and a single
connection. The dual-`bgp_conn` pattern is worth copying in any daemon
that wants to resolve simultaneous-open collisions without a global
"I'm connecting right now" flag. The flag-based approach has a known
race where both sides abandon their active connection and reconnect,
producing a double-flap. Two slots plus a deterministic collision rule
(section 6) is race-free.

The bgp_channel -> table binding is a feature ze does not have. ze
has one cache per peer, not a per-AFI channel that can subscribe to a
shared table. For a pure multi-peer IBGP mesh this does not matter;
for a route server or a route reflector with many clients it matters
a lot (section 22).

---

## 3. The per-protocol event loop: birdloops and thread groups

BIRD 3.x's central concurrency innovation is the **birdloop**: a
private event queue plus time heap plus socket list that belongs to
exactly one protocol instance. Each BGP peer (each `bgp_proto`) has
its own birdloop. Each OSPF area has its own birdloop. The route table
has its own birdloop. They communicate through the lockfree journal
(section 16) and DOMAIN locks (section 4).

### What a birdloop is

From `lib/io-loop.h:17-58` and `sysdep/unix/io-loop.c:2128-2225`, a
birdloop contains:

- A **time loop** (`struct timeloop *`) with its own domain lock. The
  time loop holds the timer heap for this loop's timers.
- An **event list** for priority (run-asap) events and a **defer list**
  for lower-priority events.
- A **socket list** for BIRD's socket abstraction wrappers.
- A **resource pool** for allocations that should die with the loop.
- A pointer to the **thread group** this loop is assigned to, and
  linkage into the thread group's loop list.

Events are dispatched by the thread that currently owns the loop:
`ev_run` drains the event lists in order, then sockets are polled,
then timers are fired. Section 4 explains who "owns" a loop.

### Thread groups

A thread group (`struct thread_group_config` at `lib/io-loop.h:111-127`)
is a pool of OS worker threads that cooperatively process a set of
birdloops. The default worker group has `thread_count = 1` (one OS
thread), and the default express group is similar
(`sysdep/unix/io-loop.c:651-668`). An operator can configure additional
groups and assign protocols to them.

At runtime (`sysdep/unix/io-loop.c:936`, `bird_thread_main`), each
thread in a group:

1. Picks a birdloop off the group's unassigned list (a loop is
   "unassigned" when no thread is currently running it).
2. Enters the loop (atomic hand-off of ownership).
3. Runs events and socket callbacks in that loop until it has nothing
   to do, then releases the loop back to the unassigned list.
4. Picks the next loop. Loop selection uses `loop_count` and `busy`
   counters for rough fairness.

The invariant is that **at most one thread is inside a given birdloop
at any time**. This means code inside a birdloop can access
loop-private state without locks as long as it does not hold the loop
across a yield point. Cross-loop state transfer goes through DOMAIN
locks (section 4), the lockfree journal (section 16), or `birdloop_ping`
which posts a wake-up to another loop (`sysdep/unix/io-loop.c:367`).

### birdloop_enter / birdloop_leave

`lib/io-loop.h:60-90` defines the entry and exit helpers:

- `birdloop_enter(struct birdloop *loop)` (declared at
  `lib/io-loop.h:89`, implemented at `sysdep/unix/io-loop.c:2237`)
  atomically claims a loop. If another thread is inside, it parks
  until the current thread leaves.
- `birdloop_leave(struct birdloop *loop)` (declared at
  `lib/io-loop.h:90`, implemented at `sysdep/unix/io-loop.c:2262`)
  releases the loop and, if there is pending work, kicks the group's
  meta-scheduler.
- `BIRDLOOP_ENTER(_loop)` (macro at `lib/io-loop.h:61`) is a scoped
  version built on `CLEANUP(birdloop_leave_cleanup)`. The C compiler
  automatically inserts the `leave` call when the scope ends,
  including on return paths. This is BIRD's RAII for birdloops.

A BGP RX handler runs inside the peer's birdloop; it touches
`bgp_proto` freely; it does not touch anything outside without first
entering the target birdloop.

### Why this matters

A Go implementation of BIRD's model is conceptually straightforward:
one goroutine per peer, one channel to receive events, selection on
the channel plus timers. ze already does this. The hard part BIRD
solved in C is not the "one thread per peer" story; it is the
**deadlock-free cross-loop locking discipline** described in the next
section. Go programs that use lock hierarchies commonly hit the same
wall. The BIRD solution is worth studying even if ze never ports it
directly.

---

## 4. DOMAIN locks and the locking order

BIRD's cross-loop locking lives in `lib/locking.h`. The single most
important declaration in that file is the partial-order enum at
`lib/locking.h:19-35`:

```
the_bird  >  meta  >  control  >  proto  >  subproto  >
service  >  rtable  >  attrs  >  logging  >  resource
```

(Exact names and positions verified at `lib/locking.h:19-35`.) A
thread may hold locks from left to right but never right to left.
Attempts to take a lock out of order trip an assertion in the locking
stack.

### What a DOMAIN is

`DOMAIN(type)` (`lib/locking.h:45-46`) is a type-tagged opaque wrapper
around `domain_generic`. The point of the type tag is compile-time
checking: you cannot pass a `DOMAIN(rtable)` to a function expecting a
`DOMAIN(attrs)`. Each domain has a reader-writer lock
(`rws_*_lock` at `lib/locking.h:152-225`) built on atomic bit fields:

- `RWS_READ_PENDING`, `RWS_READ_ACTIVE`: waiting-read and active-read
  counters.
- `RWS_WRITE_PENDING`, `RWS_WRITE_ACTIVE`: waiting-write and
  active-write counters.

Contention is resolved by spinning, with `birdloop_yield` called
periodically so the current thread can give up its current birdloop to
another thread in the group if it is stuck waiting on a lock. This is
cooperative: a loop that holds a lock and blocks without yielding
starves its group.

### The locking stack

`lib/locking.h:42` declares `locking_stack`, a per-thread stack of
currently-held locks. Every `LOCK_DOMAIN` push checks the new lock's
order against the top of the stack and asserts if the order is wrong.
Every `UNLOCK_DOMAIN` pop checks LIFO discipline. The stack is
compile-time tiny (a handful of slots) because the order is so rigid.

### RTA_LOCK: the attribute domain

`lib/route.h:96-99` declares `attrs_domain` as a `DOMAIN(attrs)` and
provides `RTA_LOCK` / `RTA_UNLOCK` macros. Any code that inserts,
looks up, or frees an interned `ea_list` takes this lock. Most of the
time the lock is uncontended because attribute insertion is fast and
the hot path (lookup by already-interned handle) does not need the
lock at all.

### BGP sockets and the BLO_LOCK pattern

BGP's shared listening sockets (section 7) are a cross-loop
abstraction: multiple `bgp_proto` instances share one physical socket.
BIRD exposes this through a birdloop-scoped lock pattern at
`proto/bgp/bgp.h:328-344`:

```
struct bgp_socket_private { BGP_SOCKET_PUB; ... };
typedef union bgp_socket { struct { BGP_SOCKET_PUB; }; struct bgp_socket_private priv; } bgp_socket;
BLO_UNLOCK_CLEANUP(bgp_socket);
#define BGP_SOCKET_LOCKED(_pub, _priv)  BLO_LOCKED(_pub, _priv, bgp_socket)
```

The `BLO_LOCK` / `BLO_LOCKED` macros (`lib/io-loop.h:80-86`) combine a
birdloop enter with a pointer-style "locked at" check, so that access
to `bgp_socket_private` is legal only from inside the socket's own
birdloop. The type-punned `union bgp_socket` exposes `BGP_SOCKET_PUB`
fields (read-only from outside) and hides the `_private` variant inside
the locked scope. This is the same pattern used for `bgp_ptx` (the
per-channel TX encapsulation, section 18), for route tables, and for
many other cross-loop resources.

The trick is that the public pointer `bgp_socket *` is safe to hold
from any loop, but **dereferencing the private fields requires taking
the lock**, and taking the lock also transfers you into the socket's
birdloop. The compiler enforces the inside-vs-outside distinction
through the struct layout: `_priv` does not exist in the public view.

### For ze

Go has no compile-time locking-order enforcement and no
`birdloop_yield`. ze cannot copy BIRD's discipline verbatim. What ze
can copy is the **partial order** approach: pick an ordering for ze's
shared mutexes and document it, then audit call sites. ze's hot
data-plane locks (per-peer pools, the BGP cache mutex, the telemetry
journal, the plugin hub) already form a rough hierarchy; writing it
down in a dependency diagram would catch mistakes at review time.

The more valuable BIRD pattern to port is the **public/private struct
split**. Exposing only "safe from outside" fields via an interface
and hiding the inside fields behind a method that requires a
loop/lock to be held is a Go pattern too: the interface defines the
outside view, the concrete struct (unexported fields) is only
reachable from the package that owns the lock. ze already uses this in
the BGP reactor's session buffer.

---

## 5. BGP FSM: dual connection slots and collision resolution

### States

`proto/bgp/bgp.h:885-895` defines the seven FSM states:

```
BS_IDLE         0
BS_CONNECT      1   attempting to connect
BS_ACTIVE       2   waiting for connection retry and listening
BS_OPENSENT     3
BS_OPENCONFIRM  4
BS_ESTABLISHED  5
BS_CLOSE        6   used during transition to BS_IDLE
```

Six of these map directly to RFC 4271 Section 8. `BS_CLOSE` is a
BIRD-specific transition state used while a NOTIFICATION drains. The
standard FSM collapses `Idle -> ... -> Established` and back; BIRD's
`BS_CLOSE` makes the "sending a NOTIFICATION then actually closing
the socket" phase explicit so that the hold timer, send hold timer,
and keepalive timer can be cleaned up consistently.

### The dual-slot design

A BGP peer runs two connection attempts: one it initiates and one it
accepts. Under normal operation only one of them establishes; the
other is closed by collision resolution. BIRD represents both
attempts as two `bgp_conn` structs embedded in `bgp_proto`
(`proto/bgp/bgp.h:446-447`): `outgoing_conn` and `incoming_conn`.
The field `conn` (a pointer, also at `proto/bgp/bgp.h:445`) tracks the
winner, set by collision resolution. Before collision, both
`outgoing_conn.state` and `incoming_conn.state` can be advanced
independently.

### Transition functions

The FSM transitions live in `proto/bgp/bgp.c`, one function per
target state:

- **`bgp_conn_enter_openconfirm_state`** (declared at
  `proto/bgp/bgp.h:701`, called after a valid OPEN) sets the
  connection state to `BS_OPENCONFIRM`. This is an internal marker;
  most of the work is in `bgp_rx_open`.
- **`bgp_conn_enter_established_state`** (`proto/bgp/bgp.c:1290`)
  activates a connection. It records `last_established`, captures
  negotiated parameters (hold time, keepalive time, AS4, enhanced
  refresh), evaluates per-AF capabilities to decide which channels
  become active, primes each channel's `feed_state` to `BFS_NONE`,
  hooks the enhanced-route-refresh restart event, and unlocks GR
  timers if the session is resuming a graceful restart.
- **`bgp_conn_enter_close_state`** (`proto/bgp/bgp.c:1491`) enters the
  NOTIFICATION-drain phase: keepalive timer stopped, socket rx hook
  removed (stop accepting data), send-hold timer re-armed for 10
  seconds (if the NOTIFICATION cannot be sent in that time, the
  connection is torn down the hard way).
- **`bgp_conn_enter_idle_state`** (`proto/bgp/bgp.c:1508`) is the
  final transition. It calls `bgp_close_conn` to free the socket,
  stops all four timers, sets `state = BS_IDLE`, and schedules the
  decision event that either restarts the outgoing attempt or
  transitions the protocol fully down.
- **`bgp_active`** (`proto/bgp/bgp.c:1915`) puts the protocol in
  `BS_ACTIVE`: a minimum `connect_delay_time` (default 1 second, up
  to 5 on prior error) is scheduled before trying to initiate.
- **`bgp_connect`** (`proto/bgp/bgp.c:1935`) creates the outgoing
  socket, configures bind address, destination, TCP buffer sizes
  (increased to 64 KB when extended messages are negotiated),
  authentication (MD5 or AO), and transitions the outgoing connection
  to `BS_CONNECT`. The socket layer drives the actual TCP handshake.

### Timer callbacks

- **`bgp_hold_timeout`** (`proto/bgp/bgp.c:1807`) fires when no BGP
  packet has arrived within the negotiated hold time. Three branches:
  if already `BS_CLOSE`, immediately idle; if the socket has bytes
  pending, grant a 10-second grace period (assume the peer is just
  slow); if LLGR is active, trigger a graceful restart instead of a
  hard error; otherwise, raise `bgp_error(conn, 4, 0, NULL, 0)` (RFC
  4271 code 4 subcode 0, "Hold Timer Expired").
- **`bgp_keepalive_timeout`** (`proto/bgp/bgp.c:1838`) schedules a
  `PKT_KEEPALIVE` via `bgp_schedule_packet` and rearms itself. The
  outgoing keepalive cadence is controlled by the keepalive-to-hold
  ratio (default 1/3).
- **`bgp_send_hold_timeout`** (`proto/bgp/bgp.c:1847`) implements RFC
  9687. If outgoing data sits in BIRD's send buffer for the full
  send-hold interval without being drained, the session is torn down
  immediately (no NOTIFICATION, because the blocked buffer prevents
  that anyway). Send hold defaults to 2x hold time but is capped by
  configuration.
- **`bgp_start_timer`** (`proto/bgp/bgp.c:1029`) is the canonical
  timer-arming helper. For any non-zero value it applies a jitter of
  up to 25% (RFC 4271 Section 10), for zero it stops the timer.

### Collision resolution

The algorithm lives at `proto/bgp/packets.c:892` (`bgp_rx_open`).
When a connection receives an OPEN and the peer's other
`bgp_conn` slot is already in `BS_OPENCONFIRM`, RFC 4271 Section 6.8
says the winner is the connection initiated from the router with the
higher BGP identifier, with higher ASN as tiebreaker. BIRD's test
evaluates `(p->local_id < id) || ((p->local_id == id) && (p->public_as
< p->remote_as))`: if true, the **remote** router is dominant, so the
connection "initiated by the dominant side" (incoming) wins and the
outgoing connection is closed with cease subcode 7 (Connection
Collision Resolution). If false, the local router is dominant and the
current incoming connection is rejected. The rejected side sees the
other enter `BS_CLOSE`, drain its NOTIFICATION, then go to `BS_IDLE`.

The dual-slot design makes this race-free because **both slots
advance independently**. A peer that sees "I sent an OPEN on my
outgoing conn" and "I received an OPEN on my incoming conn" just
proceeds to OPENCONFIRM on both, and the first to pick up on the
collision (by processing an OPEN while the other slot is already past
OPENSENT) closes the loser. Compare to a single-slot FSM that has to
abandon its current attempt, producing a noticeable flap window.

### For ze

ze's current FSM is single-slot. The bug this opens up is the race
above: a simultaneous open can produce a double-close followed by a
back-off, dropping routes for the duration of the connect timer.
Adding a second slot is a contained change inside the BGP reactor
(the rest of ze does not need to care which slot is the winner) and
would eliminate the collision flap.

---

## 6. Shared listen sockets

BIRD 3.x supports multiple BGP peers that bind the same local address
and port. Binding twice is impossible at the kernel level; BIRD
handles it by sharing a single `listen` socket across peers and
dispatching accepted connections to the right `bgp_proto` based on the
peer's remote address.

### The socket pool

`proto/bgp/bgp.h:315-344` defines the data model:

- `struct bgp_socket_params`: the bind parameters (`addr`, `iface`,
  `vrf`, `port`, `flags`). Two peers with matching params share a
  socket.
- `struct bgp_socket_private`: contains the actual `sock *sk`
  (BIRD's TCP listener wrapper), a list of `bgp_listen_request`s
  (the peers interested in this socket), and a lock pointer.
- `struct bgp_listen_request`: a per-peer request for access to a
  shared socket. It carries the match criteria (`remote_ip`,
  `remote_range`, `ipatt`), a callback for incoming connections, and a
  pointer to the owning `bgp_proto`.

The global pool sits behind a locked domain. `bgp_open`
(`proto/bgp/bgp.c:196`) creates a listen request from peer config,
then `bgp_listen_open` (`proto/bgp/bgp.c:249`) searches the pool for a
matching socket:

- If found, append the request to the socket's request list. The
  socket already exists, just add the peer to its distribution list.
- If not found, create a new `SK_TCP_PASSIVE` socket, bind and listen
  it, and register it in the pool with `rx_hook =
  bgp_incoming_connection`.

### Dispatching accepted connections

`bgp_incoming_connection` (`proto/bgp/bgp.c:2058`) is the socket's
rx hook, invoked when the kernel accepts a new connection. It reads
the peer's remote address, walks the listen request list, and for
each request compares remote address and interface against the
request's criteria. The first match calls the request's
`incoming_connection` callback, which is a protocol-supplied function
that attaches the accepted socket to the matching `bgp_proto`.

For dynamic BGP (see section below), the dispatching is more elaborate:
if no request matches but a request's `remote_range` contains the
remote address, BIRD may spawn a new `bgp_proto` on the fly with a
name derived from `dynamic_name` / `dynamic_name_digits`
configuration. This is how BIRD implements route-server-style
"accept anyone in 192.0.2.0/24" configurations.

### Why this is interesting

Without shared sockets, N peers need N listen ports (bad, breaks
firewall configuration) or 1 listen socket per (address, port) pair
with manual demultiplexing (error-prone). BIRD's approach matches
what a Linux kernel would do internally with `SO_REUSEPORT`, except
the demultiplexing logic lives in user space and uses protocol-level
criteria.

For ze, this matters for two cases:
1. **Dynamic BGP** (accepting peers from a configured range). ze
   currently requires explicit per-peer configuration. If ze ever
   wants to support IX route servers, dynamic peers are required.
2. **Multi-VRF and multi-table**. A future ze with VRF support needs
   per-VRF listen sockets. The shared-socket pool is the right
   abstraction to share the bind and still dispatch to the right
   per-VRF instance.

---

## 7. Packet RX pipeline

### The dispatch

`bgp_rx` (not shown in the header; it is the socket rx hook) is the
entry point for packet bytes. It validates the 16-byte BGP marker,
extracts the length field, assembles fragmented packets, and calls
`bgp_rx_packet` (around `proto/bgp/packets.c:3550`). The dispatch on
the type byte:

- `PKT_OPEN`: `bgp_rx_open` (`proto/bgp/packets.c:892`)
- `PKT_UPDATE`: `bgp_rx_update` (`proto/bgp/packets.c:2818`)
- `PKT_NOTIFICATION`: `bgp_rx_notification` (`proto/bgp/packets.c:3485`)
- `PKT_KEEPALIVE`: `bgp_rx_keepalive` (`proto/bgp/packets.c:3525`)
- `PKT_ROUTE_REFRESH`: `bgp_rx_route_refresh` (`proto/bgp/packets.c:3033`)

Unknown types fire `bgp_error` with code 1 subcode 3 ("Bad message
type"). The hold timer is restarted on every received packet to reset
the idle timeout.

### bgp_rx_update

`bgp_rx_update` (`proto/bgp/packets.c:2818`) is the single most
interesting function in the BGP daemon. Its job is to turn a stream
of UPDATE bytes into one or more calls into the routing table's
import function (`rte_update` at `nest/rt-table.c:2639`), with
complete RFC 7606 error handling.

The pipeline:

1. **State check**. If the connection is in `BS_OPENCONFIRM`, BIRD
   auto-transitions to `BS_ESTABLISHED` (RFC 4271 compat; some peers
   send UPDATEs before KEEPALIVE). If in `BS_CLOSE` or worse, drop.
2. **Hold timer restart**. Any UPDATE resets the hold timer to the
   full negotiated value.
3. **Parse state allocation**. A `struct bgp_parse_state` (`proto/bgp/bgp.h:606-644`)
   is built on the stack, pointing at `tmp_linpool` (a thread-local
   "per-task" arena that is released at task boundary), with
   `jmp_buf err_jmpbuf` initialised via `setjmp`. If any subsequent
   step calls `bgp_parse_error` (`proto/bgp/bgp.h:688-693`), control
   longjmps back to the top of the handler, which then calls
   `bgp_error` and drops the UPDATE. This is how RFC 7606
   "treat-as-withdraw" is implemented: a parse error raises a flag
   (`err_withdraw = 1`), the longjmp unwinds, and the top of the
   handler processes only the withdrawal side of the UPDATE.
4. **Header parse**. Extract `ip_unreach_len` (IPv4 withdrawn routes
   length), `ip_unreach_nlri` pointer, `attr_len` (path attributes
   length), `attrs` pointer, and the remainder as IPv4 reachable NLRI.
5. **Attribute decode**. `bgp_decode_attrs`
   (`proto/bgp/attrs.c:1564`) walks the attribute bytes and invokes
   one `decode` callback per attribute. It populates
   `s->mp_reach_af`, `s->mp_reach_nlri`, `s->mp_reach_len` (and the
   MP-UNREACH equivalents) as side effects when it sees MP_REACH_NLRI
   or MP_UNREACH_NLRI attributes. Returned value is an `ea_list *`.
6. **End-of-RIB detection**. If `attr_len == 0`, `ip_unreach_len ==
   0`, and `ip_reach_len == 0`, this is an IPv4 End-of-RIB marker;
   call `bgp_rx_end_mark` (`proto/bgp/packets.c:2752`) to transition
   the load state from `BFS_LOADING` to `BFS_NONE`. Multiprotocol
   End-of-RIB uses an empty MP_UNREACH_NLRI.
7. **Process withdrawals**. For each of the two withdrawal variants
   (IPv4 inline, MP_UNREACH), call `bgp_decode_nlri` with a NULL
   attribute list to mark each prefix withdrawn.
8. **Process announcements**. For each of the two reach variants
   (IPv4 inline, MP_REACH), call `bgp_decode_nlri` with the decoded
   attributes. The callee walks the NLRI bytes and, for each prefix,
   calls `bgp_rte_update` to inject the route into the table.

### Treat-as-withdraw

The critical design choice is that attribute parse errors do not
kill the session. Per RFC 7606, a malformed optional attribute can be
ignored; a malformed well-known attribute typically triggers "treat
as withdraw" (the NLRI is withdrawn, session continues). BIRD's
`setjmp`/`longjmp` pattern implements this directly: a `WITHDRAW`
macro (inside `bgp_decode_attr` at `proto/bgp/attrs.c:1530`) raises
`s->err_withdraw = 1` and returns; a `DISCARD` macro logs and skips
the attribute; a fatal error calls `bgp_parse_error` which
longjmps.

The practical consequence is that a broken Cisco sending an
illegal-length COMMUNITIES does not reset sessions with everyone in
its AS path. This was a real production problem before RFC 7606 and
is worth treating as non-negotiable for any BGP implementation built
today.

### For ze

ze already passes wire bytes forward unchanged when destinations can
consume them without modification (the "zero-copy" path). For
destinations that need attribute rewriting, ze parses lazily.
BIRD's approach is eager: every RX UPDATE is fully decoded into an
ea_list even if it will never be modified, because the best-path
recomputation needs to compare attributes.

The lesson for ze is the **setjmp pattern as a declarative error
policy**. In Go, the equivalent is a panic/recover bracket or a
returned error plus a flag. ze's current BGP message parser returns
errors; the question is whether each error site correctly distinguishes
"close the session" from "withdraw this NLRI" from "ignore this
attribute". BIRD's three-outcome macro set is a good vocabulary to
adopt. Document each parse site explicitly with one of the three
outcomes.

---

## 8. UPDATE construction and the bucket system

The transmit path is driven by `bgp_fire_tx`
(`proto/bgp/packets.c:3144`) and `bgp_create_update`
(`proto/bgp/packets.c:2609`). Together they implement BIRD's
signature TX optimisation: **routes with identical attributes share a
single UPDATE message**.

### bgp_fire_tx

`bgp_fire_tx` runs when the TCP socket is writable. It inspects
`conn->packets_to_send` (a bitmap) in priority order:

1. `PKT_SCHEDULE_CLOSE` -> call `bgp_conn_enter_idle_state`, return.
2. `PKT_NOTIFICATION` -> build via `bgp_create_notification`, send,
   schedule `PKT_SCHEDULE_CLOSE`, return.
3. `PKT_OPEN` -> build via `bgp_create_open`, save the local open
   message into `conn->local_open_msg`, send.
4. `PKT_KEEPALIVE` -> emit 19 bytes.
5. Otherwise, walk `channels_to_send` (a per-channel bitmap) in
   round-robin order via the `last_channel` and `last_channel_count`
   fields. For each channel with pending work, call
   `bgp_create_route_refresh`, `bgp_create_begin_refresh`, or
   `bgp_create_update` depending on the channel's
   `packets_to_send` bits.

The rotation between channels uses `last_channel_count` to cap
consecutive packets from the same channel at 16 before moving on.
This prevents a single noisy channel from starving the others while
still giving each channel enough bandwidth to drain efficiently.

### bgp_create_update

`bgp_create_update` receives a channel and a buffer. It initialises a
`bgp_write_state` (`proto/bgp/bgp.h:589-604`) from the channel's
`bgp_ptx` (see section 18), including flags for AS4, ADD-PATH, MPLS,
and the current MP_REACH format. Then it tries, in order:

1. **Withdrawals**. If the channel's `bgp_ptx_private.withdraw_bucket`
   has pending prefixes, encode them. Withdrawals share an UPDATE
   with an empty attribute list, so many prefixes pack into one
   message.
2. **Reachable routes**. Pop buckets off `bgp_ptx_private.bucket_queue`
   in order. For each bucket: encode the bucket's attribute set once
   via `bgp_encode_attrs` (`proto/bgp/attrs.c:1482`), then encode as
   many prefixes as fit into the remaining buffer. If a prefix does
   not fit, put it back and wait for the next TX cycle.
3. **End-of-RIB**. If no prefixes remain and `feed_state` is
   `BFS_LOADED`, emit `bgp_create_end_mark`
   (`proto/bgp/packets.c:2741`). If `feed_state` is `BFS_REFRESHED`,
   emit an EoRR marker instead.

Return value is a pointer to the end of the written data, or NULL if
no work is pending (caller then clears the per-channel bit).

### For ze

ze's BGP cache dedups routes by attribute set internally, but ze's
forwarding path does not batch prefixes sharing attributes into a
single UPDATE per destination peer. ze relies on the wire-bytes
passthrough path to avoid re-encoding, which is a different
optimisation: it works when the destination has compatible
capabilities and no filter modifications, but not when attributes
need rewriting.

The bucket-based approach is worth considering as a fallback for the
"filter modifies attributes" case. A ze bucket would be a pooled
buffer tagged with an attribute set identity; all prefixes that share
the identity share the buffer. When a filter produces a new attribute
set, a new bucket is created. The wire-bytes path still handles the
zero-modification case.

---

## 9. Capability negotiation

### The capability data model

`proto/bgp/bgp.h:262-297` defines `bgp_caps` and `bgp_af_caps`. A
`bgp_caps` has a flexible array `af_data[]` of per-AF capability
blocks. Each `bgp_af_caps` carries a `u32 afi`, plus flags for
multiprotocol readiness (`ready`), graceful restart
(`gr_able`, `gr_af_flags`), long-lived graceful restart
(`llgr_able`, `llgr_time`, `llgr_flags`), extended next hop
(`ext_next_hop`), and ADD-PATH (`add_path`).

The session-level capability block carries `as4_support`,
`ext_messages`, `route_refresh`, `enhanced_refresh`, `role`
(RFC 9234), `gr_aware`, `gr_flags`, `gr_time`, `llgr_aware`,
`any_ext_next_hop`, `any_add_path`, `hostname`, `af_count`, and
`length`.

### bgp_prepare_capabilities

`bgp_prepare_capabilities` (`proto/bgp/packets.c:248`) reads the
protocol configuration and fills a new `bgp_caps`:

- AS4 support if `enable_as4`; ASN is `public_as` (confederation ID
  in a confed, else `local_as`).
- Extended messages if `enable_extended_messages`.
- Route refresh and enhanced route refresh independently.
- BGP role if the operator configured one; `BGP_ROLE_UNDEFINED` means
  do not send the capability.
- Graceful restart with configured `gr_time`. LLGR with
  `llgr_time`.
- Hostname if `enable_hostname`.
- Per-AF blocks: one per configured `bgp_channel`, carrying its
  negotiated next-hop format, ADD-PATH mode, and GR/LLGR per-AF
  flags.

The resulting block is attached to `conn->local_caps` and fed into
`bgp_write_capabilities` for encoding into the OPEN message.

### bgp_read_capabilities

`bgp_read_capabilities` (`proto/bgp/packets.c:484`) walks the
capability TLVs inside a received OPEN. The per-capability dispatch
supports (at least) codes 1, 2, 5, 6, 9, 64, 65, 69, 70, 71, 73.
Unknown capability codes are recorded but not treated as errors (RFC
5492 requires graceful degradation).

### bgp_check_capabilities

`bgp_check_capabilities` (`proto/bgp/packets.c:699`) runs after both
OPEN messages have been exchanged. It compares the local and remote
capability blocks against the per-peer `require_*` configuration
knobs. Any mismatch causes session teardown with cease subcode 7 or
cease subcode 2 as appropriate. The `require_*` knobs let an operator
say "I will not peer with a router that does not support RFC 7313
enhanced route refresh", which is the safe production posture for
iBGP meshes that rely on incremental refeed.

### For ze

ze's capability negotiation is handled inside the wire/capability
package. The interesting design question is whether ze should
separate "supported locally" from "required on peer". BIRD's
per-capability require bits are a good model; ze currently has a
flatter structure. Adding explicit `require_*` booleans to the BGP
YANG would let operators express the same policy declaratively.

---

## 10. The unified attribute model: ea_list in BIRD 3.x

This section is the single most important architectural shift between
BIRD 2.x and BIRD 3.x. If you remember one thing from this document,
remember this.

### BIRD 2.x had two attribute storage types

In BIRD 2.x, a route's attributes lived in two places:

- **`struct rta`**: a fixed-layout struct holding gateway, source
  protocol, dest type, preference, next-hop metrics, origin AS, and
  a handful of other fields. The rta was interned with reference
  counting.
- **`struct ea_list`**: a linked list of extended attributes used for
  everything not in `rta`: BGP AS_PATH, COMMUNITIES, and so on. The
  ea_list was also interned.

Protocol code had to know which layer to look in. A route's gateway
was in `rta`; its AS_PATH was in `ea_list`. Two interning paths, two
comparators, two allocators. Any new "attribute" the core wanted had
to choose a layer.

### BIRD 3.x unifies everything

`lib/route.h:33-44` shows the new `rte`:

```
typedef struct rte {
  RTE_IN_TABLE_WRITABLE;    (flags, stale_cycle)
  u8 generation;
  u32 id;
  struct ea_list *attrs;    <-- the ONLY attribute storage
  const net_addr *net;
  struct rte_src *src;
  struct rt_import_hook *sender;
  btime lastmod;
} rte;
```

There is no `rta`. Next-hop is an attribute (`ea_gen_nexthop`, at
`lib/route.h:523` neighbourhood). Origin, MED, LOCAL_PREF, AS_PATH,
COMMUNITIES, everything, lives in the same `ea_list`. Every
attribute is an `eattr` (`lib/route.h:241-253`):

```
typedef struct eattr {
  word id;        EA_CODE(protocol, attr_id)
  byte flags;
  byte type;      T_INT, T_IP, T_PATH, T_OPAQUE, T_CLIST, T_NEXTHOP_LIST, ...
  byte rfu:5, originated:1, fresh:1, undef:1;
  PADDING(unused, 3, 3);
  union bval u;   embedded data or pointer to adata
} eattr;
```

The `ea_list` (`lib/route.h:261-268`) is a header plus a flexible
array of eattrs, with a `next` pointer for overlay chaining.

### The ea_class registry

Every attribute has a **class** (`struct ea_class` at
`lib/route.h:292-310`). A class holds the name (both filter-visible and
legacy v2-socket name), the auto-assigned numeric id, the data type,
protocol-dependent flags, a format callback, a `stored` hook (called
when the attribute is interned), and a `freed` hook.

Classes are registered at startup. `ea_register_init` fills in stub
metadata; `ea_register_alloc` (`nest/rt-attr.c:713`) allocates a
`ea_class_ref` resource, assigns the next free global id, and records
the class in a global table.

BGP's attribute classes are registered in `bgp_register_attrs` at
`proto/bgp/attrs.c:1351`. They live in `bgp_attr_table[BGP_ATTR_MAX]`,
a union array where each entry extends `ea_class` with three
BGP-specific function pointers: `export`, `encode`, `decode`. This is
how the generic `ea_class` registry coexists with
protocol-specific codec hooks: the ea_class header is the public
vtable; the BGP-specific trailer is reached via a downcast based on
`ea_class_find(a->id)`.

### ea_stored: the lifecycle tag

`lib/route.h:270-278` defines the `ea_stored` enum:

```
EALS_NONE        temporary (e.g., on a stack)
EALS_PREIMPORT   post-decode, pre-filter
EALS_FILTERED    post-filter, pre-table
EALS_IN_TABLE    cached (interned, ref-counted)
EALS_KEY         used as a hash key
EALS_CUSTOM      OR this with a protocol-defined value
```

The tag lets interning functions skip re-normalising a list that is
already in the right shape. `ea_lookup` fast-path checks the tag
first; if it already matches, the caller gets a refcount bump and no
rehash.

### Why this matters for ze

BIRD 2.x's split was the single most confusing thing about the
daemon. ze does not replicate it: ze has one attribute storage model
(per-type pools with refcounted handles), not two. But ze has a
different split: parsed attribute structures versus raw wire bytes.
A ze route handle knows how to reach its attributes either through
the pool (the canonical form) or through the wire buffer (the
lazy-parse form). Whether this two-layer model survives as ze grows
is the analogous question that BIRD's v3 rewrite answered with "one
layer is simpler".

The argument for ze keeping two layers is that wire-byte passthrough
is strictly cheaper than pool round-trip for forwarded-unchanged
routes. The argument for collapsing is that every filter or policy
implementation has to handle both cases, and the surface area for
subtle bugs is larger. BIRD chose simplicity after seven years of
accumulated bug experience; ze should read that as a warning.

---

## 11. Attribute interning and reference counting

### The global table

`nest/rt-attr.c:1522` declares `rta_hash_table` as a `SPINHASH` of
`struct ea_storage`. The SPINHASH is a lockfree hash with atomic
insert and lookup under a RTA_LOCK (`lib/route.h:96-99`) during
contended operations.

`struct ea_storage` (`lib/route.h:280-286`) is a 16-byte prefix
followed by an `ea_list` body:

```
struct ea_storage {
  struct ea_storage *next_hash;   (hash chain)
  _Atomic u64 uc;                  (use count, with in-progress bits)
  u32 hash_key;                    (ea_list hash, precomputed)
  PADDING(unused, 0, 4);
  ea_list l[0];                    (the list itself)
};
```

Every interned ea_list is reachable from an `ea_storage`. Routes
point to the `ea_list` inside, not to the storage; the storage is
found via `SKIP_BACK` when a refcount operation is needed.

### ea_lookup

`lib/route.h:609-616` declares the fast path and `nest/rt-attr.c:1585`
implements `ea_lookup_slow`. The algorithm:

1. Fast path: if the caller's `ea_list` is already interned and its
   `stored` field matches, call `ea_ref` to bump the refcount and
   return.
2. Slow path: normalize the list via `ea_normalize`
   (`nest/rt-attr.c:980`) which sorts, deduplicates, and allocates
   into `tmp_linpool`.
3. Compute the hash via `ea_hash` (`nest/rt-attr.c:1468`), an
   XOR-multiply over the attribute byte stream.
4. Take `RTA_LOCK`.
5. Search the bucket for an existing entry. `ea_lookup_existing`
   (`nest/rt-attr.c:1551`) walks the chain comparing hash-then-body.
6. If found, atomic increment `uc`, release the lock, return.
7. If not found, allocate an `ea_storage` + body (from a slab for
   small lists, from the stonehenge large-allocator for huge ones
   marked with `EALF_HUGE`), copy the body, recursively `ea_ref` any
   nested attribute blocks, initialise `uc` to 1, insert into the
   hash, release the lock, return.

### ea_free_later / ea_free_deferred

Reference-count drops are deferred. `ea_free_later` (declared at
`lib/route.h:629-641`) does not immediately run the release path.
Instead it queues a `deferred_call` processed at task boundary
(`ea_free_deferred` at `nest/rt-attr.c:1635`). The deferred processor
takes `RTA_LOCK`, atomically decrements `uc`, and if `uc` hits zero
removes the entry from the hash and frees the storage.

The deferral is essential because a route table update may drop many
refs in a single pass, and taking RTA_LOCK once per drop would thrash.
Deferred processing batches all drops in a task into a single
`RTA_LOCK` acquisition.

### The "in-progress" encoding

The `uc` field is 64 bits but the effective counter only uses the
low 44 bits. Bits 44 and above encode the number of concurrent
"free in progress" attempts (`lib/route.h:623-626`). This lets
several threads simultaneously race to drop the last reference
without corrupting the count: the loser simply observes a nonzero
in-progress bit and leaves the deallocation to the winner.

### For ze

ze's per-type attribute pools already implement refcounting, but
ze does not (yet) defer drops. For a BGP daemon seeing millions of
route changes per second during convergence, batching drops at task
boundary is a measurable win. ze's forwarding worker loops are the
natural task boundary: a worker drains its queue, then at the end of
the drain runs pending decrements.

The interning granularity question is also worth revisiting. BIRD
interns the **whole attribute set** (one ea_list identity per unique
combination). ze interns **each attribute individually** (one
handle per unique AS_PATH, separate handle per unique COMMUNITY set,
and so on). BIRD's choice is worse for many-routes-with-subtly-different-sets
(every near-duplicate set is a new intern), better for
many-routes-with-exactly-matching-sets (one fat intern covers all).
The crossover depends on the workload. For full-table transit with
high similarity across prefixes, BIRD's whole-set interning is more
memory-efficient. For route-server fan-out where every client sees a
slightly different set, per-attribute interning wins. ze is closer to
the route-server case, so the per-attribute choice is probably
correct.

---

## 12. BGP attribute codec hooks

### The attribute descriptor table

`proto/bgp/attrs.c:1058` onwards defines `bgp_attr_table`, a union
array where each entry is either a plain `ea_class` or a BGP-specific
extension. The BGP extension adds three function pointers:

- `export(struct bgp_export_state *, eattr *)`: called at export
  time. Can rewrite the attribute (for example to swap in the local
  LOCAL_PREF, strip ORIGINATOR_ID, or update the next hop).
- `encode(struct bgp_write_state *, eattr *, byte *buf, uint size)`:
  writes the attribute's wire form into `buf`, returns bytes written.
- `decode(struct bgp_parse_state *, uint code, uint flags, byte *data,
  uint len, ea_list **to)`: parses the wire form, validates per RFC
  4271 / 7606, appends to `*to`.

Some entries are `hidden` (`lib/route.h:303`), meaning they are
technical attributes invisible to the filter language.

### Representative entries

- **BA_ORIGIN** (`proto/bgp/attrs.c:1059` neighbourhood): type
  `T_ENUM_BGP_ORIGIN`, flags `BAF_TRANSITIVE`, encode via the
  generic `bgp_encode_u8` helper, decode via `bgp_decode_origin`
  (range check 0..2).
- **BA_AS_PATH**: type `T_PATH`, flags `BAF_TRANSITIVE`. Encode calls
  `bgp_encode_as_path` which handles the 16-bit-to-32-bit migration
  (when `as4_session == 0`, emit AS_PATH with 16-bit ASNs and
  separately emit AS4_PATH with the real ones). Decode
  (`bgp_decode_as_path`) does the reverse merge.
- **BA_COMMUNITY**: type `T_CLIST`, flags `BAF_OPTIONAL |
  BAF_TRANSITIVE`. Encode is the generic `bgp_encode_u32s` (an adata
  is already a packed u32 array). Decode validates length mod 4 and
  appends.
- **BA_MP_REACH_NLRI**: type `T_OPAQUE`, `hidden`. No encode hook
  (MP_REACH is assembled at UPDATE-level time in
  `bgp_create_update`). Decode parses the AFI/SAFI, extracts the
  next-hop field, and stashes the NLRI pointer and length on the
  parse state for later processing.

### The decode dispatcher

`bgp_decode_attr` (`proto/bgp/attrs.c:1530`) is the per-attribute
entry point called by `bgp_decode_attrs` (`proto/bgp/attrs.c:1564`).
For each attribute it:

1. Checks `BIT32_TEST(s->attrs_seen, code)` (the 256-bit duplicate
   bitmap on the parse state). Duplicate of a well-known attribute is
   a fatal parse error per RFC 7606 3(g); duplicate of an optional
   attribute is `DISCARD`.
2. Validates the RFC 4271 Optional/Transitive/Partial/Extended flags
   against the attribute class's expected flags. Mismatch is
   `WITHDRAW` unless the class is flagged `BAF_DECODE_FLAGS`.
3. Calls the class's decode hook.

The duplicate check and flag validation are the two most commonly
forgotten RFC 7606 compliance points in a fresh BGP implementation.
BIRD's placement of both in a single inline-plus-dispatch wrapper
means every attribute class gets them "for free".

### For ze

ze already has per-attribute codec dispatch. The questions worth
asking against BIRD's table are:

1. Is ze's duplicate bitmap 256 bits wide so it covers
   `BGP_ATTR_MAX`? Yes.
2. Does ze's dispatcher check flags before calling the per-attribute
   decoder, or does each decoder check itself? Centralising it is
   less error-prone.
3. Does ze's dispatcher have a three-outcome vocabulary (fatal /
   withdraw / discard)? Worth auditing.

BIRD also uses `bgp_finish_attrs` (`proto/bgp/attrs.c:1676`) to run
post-decode validation: mandatory-attribute presence for announced
routes, LOCAL_PREF presence for iBGP, and so on. A ze equivalent
would sit after the NLRI loop and before the `rte_update` call.

---

## 13. Route table write path (nest/rt-table.c)

### rte_update

`rte_update` (`nest/rt-table.c:2639`) is the channel-side import
entry point. A BGP receive handler calls it once per decoded
prefix with an `rte` that holds the decoded attributes. The function:

1. Applies the channel's import filter (`f_run`). If the filter
   rejects, the route gets `REF_FILTERED`; if the table retains
   filtered routes, the filtered copy is still stored for "show
   protocol routes filtered" but not used for best-path.
2. Applies any ACL or stats hook.
3. Calls `rte_import` (`nest/rt-table.c:2723`) under the table's
   `RT_LOCKED` (a `DOMAIN(rtable)` lock) to insert or replace the
   route.

### rte_import

`rte_import` looks up the `netindex` for the net, finds or allocates
the `routes` entry (a pointer into the table's per-index block),
inserts the new `rte_storage` at the head of the atomic linked list,
and triggers best-path recomputation. The previous best is saved for
comparison.

### Best-path and rte_announce

Best-path selection is driven by `bgp_rte_better`
(`proto/bgp/attrs.c:2525`) for BGP routes, via the protocol's
`rte_owner_class` vtable. For deterministic MED comparison
(`compare_path_lengths`, `deterministic_med`), BIRD uses
`bgp_rte_recalculate` (`proto/bgp/attrs.c:2762`) which walks the full
set and picks a winner under the stricter rules.

When a new best emerges, `rte_announce` (`nest/rt-table.c:2036`) is
called with the old best and new best. It in turn calls
`rte_announce_to` (`nest/rt-table.c:2006`) twice: once for the
"export_all" journal (subscribers that want every route, even
non-best), once for "export_best" (subscribers that only want the
current best). Each journal push allocates an `rt_pending_export`
item and appends it to the per-net queue. An `announce_kick_event`
is fired to wake subscribers.

### For ze

ze's BGP cache is a separate structure from the routing table. BIRD's
rt_exporter pattern is closer to what ze might want for a general-purpose
route table: a per-table lockfree journal that multiple subscribers
drain asynchronously, with per-net pending queues to serialise updates
to a given prefix. ze currently does per-peer forwarding workers that
pull from the cache directly. For a BGP-only daemon this is fine; for
a route-server-shaped workload with hundreds of clients, the
lockfree journal scales better.

---

## 14. netindex: prefix-to-index mapping

`lib/netindex.c:52-80` declares `netindex_hash_new`. The netindex
hash stores `struct netindex` entries (`lib/netindex.h:19-25`), each
holding:

- The hash key (a hash of the net_addr).
- A compact integer index (`u32`).
- A lockfree use-count (`lfuc`).

The index is used to address route storage: a table's `routes[]`
array is indexed by netindex id, so looking up a net becomes "hash
to get netindex, use netindex.index to index the routes array".
`net_get_index` returns or allocates an index; `net_resolve_index`
goes the other way.

Indices are per-table, not global. Two tables are not guaranteed to
assign the same index to the same prefix. Two channels that import
into the same table will see consistent indices; a `pipe` protocol
that copies routes between tables will see different indices on each
side.

### Why this exists

The primary advantage of netindex over "use the net_addr pointer as a
key" is that it gives you a stable, small, dense integer identity.
Hash tables keyed on u32 are faster than hash tables keyed on a
variable-length `net_addr`, and arrays indexed by u32 are even
faster. For a table holding a full BGP table (1.1M IPv4 prefixes,
200k IPv6), the dense index lets the table hot loop use an array
rather than a hash.

A second advantage is that a refcounted netindex can be shared
across all routes for the same net, so the memory cost of the prefix
bytes is paid exactly once per net, not once per route.

### For ze

ze's BGP cache uses a trie for prefix matching, not dense indices.
The trie gives you longest-prefix-match for free, which netindex does
not. But if ze ever adds a "give me every route for this prefix"
hot path (for example, for show commands or for ADD-PATH
best-N selection), a dense netindex layer on top of the trie would
be worth the cost.

---

## 15. RCU: lockfree reads

`lib/rcu.c` and `lib/rcu.h` implement a minimal RCU (read-copy-update)
primitive. It has two parts:

- **Per-thread state**: each thread has a `this_rcu_thread.ctl` that
  tracks the nesting depth of the current read-side critical section
  and a phase snapshot.
- **Global state**: `rcu_global_phase` (`lib/rcu.h:22`), a u64 that
  increments each time a synchronize-rcu is called.

`rcu_read_lock` (`lib/rcu.h:32-48`) increments the thread's nesting
count and, on the 0-to-1 transition, snapshots the global phase.
`rcu_read_unlock` (`lib/rcu.h:50-55`) decrements.
`synchronize_rcu` (`lib/rcu.c:36-65`) increments the global phase and
spins until every thread's snapshot has advanced past the old value
or every thread is at nesting 0. At that point no reader can still be
holding a reference to data that existed before the phase bump.

### What RCU protects in BIRD

- **netindex lookups**: a rehash can move netindex entries to a new
  table; readers iterating the old table under RCU see a consistent
  view until they release the read lock.
- **rt_storage chains**: `rte_storage.next` is atomic; readers walk
  the chain under RCU while writers atomically publish a new head
  and eventually reclaim old entries.
- **Journal replay**: `rt_export_get` (`nest/rt-export.c:39`) uses
  RCU to hold a reference to the last-consumed journal item while
  the cursor is being advanced.

### RCU_ANCHOR / RCU_RETRY

`lib/locking.h:510-529` defines two macros that wrap retry loops:

```
RCU_ANCHOR(u)   ... RCU_RETRY(u, ...)
```

`RCU_ANCHOR` records the current RCU phase on the stack. `RCU_RETRY`
checks if the phase has advanced and, if so, longjmps back to
`RCU_ANCHOR` to retry the lookup with the fresh table. This is how
BIRD handles the "reader started before the rehash, finishes after"
case without holding locks for the full duration of a complex
lookup.

### For ze

Go's memory model does not have a direct RCU equivalent but has
`sync/atomic` pointer stores. The practical pattern that maps closest
is **generation counters + copy-on-write snapshots**: readers atomically
load a pointer to the current table, iterate it, and release their
reference (either via a generation counter or via the GC). Writers
build a new table, atomically publish it, and trust the GC to reclaim
the old one when readers release. For ze's BGP cache and forwarding
table, this is the right pattern; ze already uses it in a few places.

The RCU_RETRY pattern (restart on phase change) is also portable:
instead of iterating a slice that might be reallocated, iterate a
captured snapshot and retry on version mismatch.

---

## 16. The route export journal (lfjour)

### lfjour: lockfree multi-producer single-consumer

`lib/lockfree.c` implements `lfjour` (lockfree journal), a
**multi-producer**, **multi-consumer** event log. Each event is appended
atomically; readers independently track their position using a
`lfjour_recipient` nested in their request structure.

Events are allocated in page-sized blocks (`lib/lockfree.c:89`). A
producer calls `lfjour_push_prepare` to reserve space and
`lfjour_push_commit` to publish. Publishing atomically advances the
`end` counter (`lib/lockfree.c:113`) so consumers see the new item.

### Why it's lockfree for the happy path

Producers do not coordinate with consumers. A producer only needs an
atomic increment of the journal's end counter and an atomic store of
the item bytes. A consumer holds a per-consumer cursor and advances
it by reading items between its cursor and `end`. If the consumer is
slow, the journal accumulates unread pages; once every consumer has
passed a page, the page can be reclaimed.

### rt_exporter: BIRD's journal-based export

`nest/rt-export.c` and `lib/route.h:214-228` define
`rt_exporter`, the route-table side of the journal. Each table has
two exporters: `export_all` (every route change) and `export_best`
(only when the best route for a prefix changes). A subscriber
(an `rt_export_request` at `lib/route.h:104-189`) attaches to one or
both.

`rt_export_get` (`nest/rt-export.c:39`) is the consumer read
function. A BGP channel's export path sits in a loop:

```
while (rt_export_get(req, &update)) {
    filter update;
    encode into bucket;
    advance cursor;
}
```

The loop does not hold any lock across iterations. If a filter
evaluation is expensive, only this one subscriber slows down; the
journal keeps growing, and other subscribers drain independently.

### Backpressure: cork and uncork

`lib/route.h:71-73` and `lib/route.h:475-499` define the cork
mechanism. When the total pending items across all recipients exceeds
`cork_threshold`, the table "corks" producers: new `rte_update` calls
block until the journal drains below a safe level. Uncorking happens
when consumers catch up, and is signaled by a callback.

BGP's uncork integration (`bgp_proto.uncork`, field at
`proto/bgp/bgp.h:455`) lets a slow BGP consumer pause its own
producers elsewhere. A `bgp_do_uncork` callback
(`proto/bgp/bgp.h:830`) resumes work once the slowdown clears.

### Feeder state

A subscriber can be in one of several states
(`lib/route.h:112-123`):

- `TES_DOWN`: not subscribed.
- `TES_FEEDING`: replaying the table from the start. Used after
  channel bring-up or refresh.
- `TES_PARTIAL`: mid-feed, partial coverage.
- `TES_READY`: caught up, processing the live journal.
- `TES_STOP`: unsubscribing.

`rt_export_refeed_feeder` (`nest/rt-export.c:374`) transitions a
subscriber back to `TES_FEEDING` when a refresh is requested.
Networks are replayed from the table's netindex, not from the
journal history, because the journal is not a full log (it is a
time-bounded window).

### For ze

ze's telemetry system already uses a lockfree journal. ze does not
use the same pattern for BGP route export; it uses per-peer
forwarding workers that pull from the cache under lock. The BIRD
pattern is strictly more scalable for N-subscriber fan-out:

- Producer contention is O(1): a single atomic on the journal end.
- Consumer contention is zero: each consumer has its own cursor.
- Backpressure is explicit: slow consumers trigger cork, and the
  producer knows what to do about it.

For ze BGP today, the per-peer workers approach is adequate. For a
route-server-shaped workload (1 RIB, 200+ peers consuming it with
different filters), a journal-based exporter would be the right
rewrite.

---

## 17. The protocol and channel framework

### Protocol states

`nest/protocol.h:371-375` defines four states:

```
PS_DOWN_XX  0  (down)
PS_START    1
PS_UP       2
PS_STOP     3
```

`PS_DOWN_XX` is the name BIRD chose for "zero is the down state" to
prevent accidental use of the bare symbol. There is no `PS_FLUSH`
because flushing lives inside the PS_STOP transition.

### Channel states

`nest/protocol.h:746-748` and surrounding comments
(`nest/protocol.h:701-748`) define the channel lifecycle:

```
CS_DOWN   0  (not active)
CS_START  1  (starting, routes not flowing yet)
CS_UP     2  (fully up, route exchange allowed)
CS_PAUSE  (paused; soft disconnect, routes retained)
CS_STOP   (stopping)
```

The transition table is in the comments at
`nest/protocol.h:739-743`:

```
CS_DOWN  -> CS_START / CS_UP
CS_START -> CS_UP / CS_STOP
CS_UP    -> CS_PAUSE / CS_STOP
CS_STOP  -> CS_DOWN (automatic)
```

### Export state

Orthogonally, each channel has an export state
(`lib/route.h:112-123`):

- `TES_DOWN`: export not active.
- `TES_FEEDING`: replaying the table.
- `TES_READY`: caught up.
- `TES_STOP`: tearing down.

The feeder state machine is independent of the channel state
machine. A channel can be `CS_UP` with `TES_FEEDING` while the
initial table replay is in progress, then transition to `CS_UP /
TES_READY` when done. This separation is critical: bio-rd's BGP
initialization assumes RX and TX come up together, which races with
slow table feeds.

### Soft reconfig: bgp_reconfigure

`bgp_reconfigure` (`proto/bgp/bgp.c:3460`) is called when the config
is reloaded. It compares old and new `bgp_config` byte-by-byte for
the memcmp-safe region and field-by-field for the rest (filter
pointers, keychain references, ipatt). If the changes are safe (both
configs produce the same session parameters), the function returns
true: the session continues, filters are swapped atomically, and
running FSM state survives.

If any FSM-relevant field changes (ASN, authentication type, hold
time, etc.), the function returns false and the protocol framework
tears down and restarts the session.

The key implementation detail at `proto/bgp/bgp.c:3460` is that the
comparison happens **in place** against the live `bgp_proto`, and the
reconfigure function is responsible for preserving the listen-socket
request list, the established FSM state, and the TCP socket. This
"no restart unless unavoidable" posture is why BIRD reloads are
nearly invisible to peers.

### For ze

ze's config apply path is based on YANG diff, which lets ze identify
exactly which peers need to restart and which can soft-reconfigure.
ze's filter reload path already handles the common case without
bouncing sessions. What ze can learn from BIRD is the **compare-then-act
pattern**: ze should explicitly compare old and new per-peer config
for the fields that matter and log which category of change caused a
restart when one happens. BIRD produces no log at all for a
successful soft reconfigure, which is excellent for ops teams
watching dashboards.

---

## 18. Adj-RIB-Out: the per-channel bgp_ptx

Each `bgp_channel` has a TX encapsulation called `bgp_ptx` defined at
`proto/bgp/bgp.h:523-544`. Its public fields are the lock
(`DOMAIN(rtable) lock`), a back-pointer to the channel, and an
`rt_exporter` that publishes the channel's TX state as if it were a
route table. The private fields (accessible only inside the lock):

- `pool *pool`: for long-lived allocations scoped to this channel.
- `stonehenge *sth`: the bucket allocator. Stonehenge is a
  BIRD-specific slab that efficiently handles variable-sized
  allocations with reference counting.
- `HASH(struct bgp_bucket) bucket_hash`: a hash table of `bgp_bucket`
  keyed by `ea_list` identity (see section 8).
- `struct bgp_bucket *withdraw_bucket`: a distinguished bucket for
  prefixes being withdrawn (no attributes; just an NLRI list).
- `list bucket_queue`: the queue of buckets waiting to be sent. A
  bucket leaves the hash when all its prefixes have drained, and
  leaves the queue when it has been fully sent.
- `HASH(struct bgp_prefix) prefix_hash`: a hash table of
  `bgp_prefix` keyed by `(net, path_id)`. This tracks every
  prefix currently scheduled for transmit.
- `slab *prefix_slab`: the allocator for `bgp_prefix` nodes.

### bgp_prefix and the per-prefix bucket pointer

A `bgp_prefix` (`proto/bgp/bgp.h:555-563`) has the following fields:

- `node buck_node`: intrusive list node for its current bucket's
  prefix list.
- `struct bgp_prefix *next`: hash-chain pointer.
- `struct bgp_bucket *last`: the bucket previously sent for this
  prefix.
- `struct bgp_bucket *cur`: the bucket currently scheduled.
- `btime lastmod`: last modification timestamp.
- `struct rte_src *src`: for ADD-PATH, the source carries the path
  ID.
- `struct netindex *ni`: shared with the table.

**The `last == cur` invariant is the clever bit**: if a bucket change
produces a prefix whose new bucket is the same as the previously sent
bucket, the prefix does not need to be sent again. `bgp_done_prefix`
(declared at `proto/bgp/bgp.h:771`) sets `last = cur` after the send,
and the next iteration's compare-and-skip is what eliminates
redundant transmissions.

### bgp_bucket

`proto/bgp/bgp.h:565-573`:

- `node send_node`: node in `bucket_queue`.
- `struct bgp_bucket *next`: hash chain.
- `list prefixes`: prefixes to send (intrusive via
  `bgp_prefix.buck_node`).
- `u32 hash`: the bucket's attribute hash.
- `u32 px_uc:31`: refcount (how many prefixes reference this
  bucket).
- `u32 bmp:1`: one bit reserved for BMP use (section 24).
- `ea_list eattrs[0]`: the bucket's attribute list, stored inline
  with the bucket header (variable-length allocation).

The bucket's `eattrs` is a **copy** of the attribute set, not a
pointer. This means bucket allocation carries the full weight of the
attribute bytes. The tradeoff is that encoding is fast (one cache
line for the header, straight-line iteration over attrs) and the
bucket is free to mutate its prefix list without touching the
attribute store.

### The lifecycle

A route event (from `rt_export_get` pulling from the table's
journal) flows into the channel export like this:

1. A filter is run. If rejected, the prefix is withdrawn (add to
   `withdraw_bucket`).
2. The filter's output attributes are hashed and looked up in
   `bucket_hash`. If found, the prefix is added to the existing
   bucket. If not, a new bucket is allocated via `sth`, the attribute
   list is copied into its `eattrs[]`, and it is inserted into
   `bucket_hash` and `bucket_queue`.
3. The prefix's `last == cur` check is evaluated. If equal, the
   prefix is dropped (no transmission needed). Otherwise, it is
   appended to the new bucket's prefix list and `cur` is updated.
4. An immediate TX scheduling hint is posted.

When `bgp_fire_tx` runs, it pulls buckets off `bucket_queue` and
encodes them into UPDATE messages via `bgp_create_update`
(section 8).

### For ze

ze's forwarding path is per-destination: every destination peer has a
worker that reads from the cache and encodes directly. There is no
per-destination bucket hash. For ze today (mostly iBGP mesh and
small eBGP), this is fine. For a route-server or route-reflector
deployment, the bucket pattern cuts encoding work by a factor equal
to the average attribute-set sharing (typically 3x to 20x).

A staged adoption: add bucket-style attribute-set identity checking
to ze's per-peer TX workers, so that sequential prefixes with the
same attributes reuse an encoded buffer. The full `bgp_ptx` split
(per-channel state, hash tables, queues) is a larger structural
change.

---

## 19. Graceful restart and LLGR

### The protocol-level hook

`proto/bgp/bgp.h:706-707` declares:

```
void bgp_handle_graceful_restart(struct bgp_proto *p);
void bgp_graceful_restart_done(struct bgp_channel *c);
```

`bgp_handle_graceful_restart` is called from the FSM when a session
ends while the peer advertised GR support. The session stays in the
"GR active" limbo state: existing routes are retained in the table
with a stale flag; new sessions from the same peer are allowed to
recover within `gr_time`.

`gr_timer` on the `bgp_proto` (`proto/bgp/bgp.h:462`) is armed at the
start of the GR hold period. If it fires before the session
reestablishes, all stale routes are withdrawn.

### Long-lived GR (RFC 9494)

LLGR extends the hold period. When a session does not recover within
`gr_time` but the peer is LLGR-capable, routes remain in the table
tagged with the `BGP_COMM_LLGR_STALE` community
(`proto/bgp/bgp.h:974`) and a de-preferenced local preference, for
up to `llgr_time`. Peers that receive LLGR stale routes are expected
to de-prefer them but not withdraw them immediately.

The per-channel LLGR state lives in `bgp_channel.stale_timer`,
`bgp_channel.stale_time`, and the refeed machinery
(`stale_feed`, `stale_event`) at `proto/bgp/bgp.h:507-510`. When
LLGR fires, the channel walks its Adj-RIB-In via the feeder and
marks each route stale via `bgp_rte_modify_stale` (declared at
`proto/bgp/bgp.h:776`).

### For ze

ze's GR implementation is a known gap in
`bgp-implementations-analysis.md`'s P1 list. The BIRD model is a
direct port target: `gr_timer` per-peer, `stale_timer` per-channel,
feeder-based refeed for stale marking, and explicit state machine
transitions via a `BGP_GRS_*` enum (`proto/bgp/bgp.h:253-257`).

The implementation detail worth stealing is that **GR stale marking
reuses the refresh feeder**. BIRD does not need a separate "walk
every route and mark it stale" loop; the same rt_export_feeder that
handles refresh also handles stale. The action (mark stale vs
rewrite attributes) is passed in as a callback.

---

## 20. Enhanced route refresh (RFC 7313)

Plain route refresh (RFC 2918) is a one-shot "please re-send
everything". The receiver has no way to know when the refresh is
complete, so concurrent refreshes interleave ambiguously. RFC 7313
adds BoRR (Begin of Route Refresh) and EoRR (End of Route Refresh)
demarcation: the refresher sends BoRR, then all routes, then EoRR.
The receiver can now tell when a refresh is done.

### BIRD's implementation

The channel's `feed_state` and `load_state` (`proto/bgp/bgp.h:937-941`)
track the four possible states:

```
BFS_NONE        no active feed
BFS_LOADING     initial feed, End-of-RIB expected
BFS_LOADED      ready to send End-of-RIB marker
BFS_REFRESHING  enhanced refresh in progress
BFS_REFRESHED   ready to send EoRR marker
```

`bgp_begin_route_refresh` (`proto/bgp/bgp.c:2531`) is called when a
peer requests refresh. If enhanced refresh is negotiated, it:

1. Sets `feed_state = BFS_REFRESHING`.
2. Schedules `PKT_BEGIN_REFRESH` so the BoRR goes out first.
3. Creates an `rt_feeding_request` with a done callback.
4. Calls `rt_export_refeed` to trigger the table replay.

As the replay runs, UPDATEs stream through the normal TX path. When
the feeder's replay completes, the done callback flips
`feed_state = BFS_REFRESHED`, and the next `bgp_fire_tx` emits an
EoRR via `bgp_create_end_refresh`.

### Concurrent refresh handling

If a second refresh request arrives while one is in progress, the
`enhanced_rr_again` flag on the channel (`proto/bgp/bgp.h:514`) is
set. When the current refresh completes, `bgp_restart_route_refresh`
(an event hook near `proto/bgp/packets.c:2565`) is invoked; it
re-fires the `enhanced_rr_restart` event so the new refresh starts
cleanly.

### For ze

ze's current route refresh support is basic. The main gap is that ze
has no per-channel feed state machine distinct from session state.
Adding one (the BFS_* enum) is a small change; the value is that
enhanced-refresh-capable peers see the correct demarcation and can
distinguish a partial refresh from a dropped session.

---

## 21. Route server mode and IX deployment

### What makes a route server different

A route server at an Internet Exchange has 100 to 1000 eBGP clients
that peer with **the server** but exchange routes with **each other**.
The server applies per-client import and export filters, computes
per-client best paths, and ADD-PATH is mandatory because many
clients want to see all the alternative paths to a given prefix.
Route servers are the canonical use case for BIRD; DE-CIX, AMS-IX,
and LINX all run BIRD.

### BIRD's rs_client configuration

`proto/bgp/bgp.h:107` and `bgp.h:433` track `rs_client` at config
and runtime. A session marked `rs_client` in the config skips some
of the standard eBGP processing: AS-path prepending is suppressed
for local-AS insertion (`public_as` is not added at forwarding),
next-hop is preserved from the originator, and NO_EXPORT interpretation
is adjusted.

### Per-client filters

Every `bgp_channel` has its own import filter and its own export
filter, so each client sees only the routes that the route-server
operator wants it to see. With 500 clients, this is 500 filter
evaluations per received UPDATE, which is the workload for which
BIRD's filter bytecode VM was designed.

### ADD-PATH

RFC 7911 ADD-PATH is essential for route servers: without it, a
client only sees its own best path, losing the visibility into the
alternative paths other clients have chosen. BIRD's per-channel
`add_path_rx` and `add_path_tx` flags (`proto/bgp/bgp.h:516-517`)
drive the per-AF negotiation. `bgp_prefix.path_id` (via `rte_src`)
carries the path ID on the TX side.

### For ze

ze is not a route server. Building one would require:

1. Dynamic BGP (accept peers from ranges, section 6).
2. ADD-PATH support (send and receive).
3. Per-peer import/export filters.
4. A scaling plan for the attribute fan-out (N clients receiving M
   paths each).

Points 1 and 3 are scope-of-existing-features; point 2 is protocol
work; point 4 is architectural. For point 4, BIRD's bucket system
(section 18) is the reference. Whether ze wants to enter this market
is a product decision; this document is neutral on that.

---

## 22. BMP integration

BGP Monitoring Protocol (RFC 7854) sends a copy of every BGP message
to a monitoring station for archival and analytics. BIRD implements
BMP as a separate protocol in `proto/bmp/bmp.c` (1613 LOC).

### How BMP reuses the BGP encoder

`bmp_route_monitor_notify` (`proto/bmp/bmp.c:849`) is called every
time a monitored peer imports a route. It borrows BGP's attribute
encoder via `bgp_bmp_encode_rte` (`proto/bgp/packets.c:2576`):

```
byte *pos = bgp_bmp_encode_rte(c, msg.pos + BGP_HEADER_LENGTH,
                               msg.end, new);
```

The helper takes a "fake" per-channel state (the BMP channel, not a
real BGP channel) and produces an UPDATE-format encoding of the
route that BMP can wrap in its own header. The `bgp_ptx_private.bmp`
bit (`proto/bgp/bgp.h:543`) distinguishes a fake ptx from a real
one; the bucket code checks this bit to disable certain scheduling
logic that would not make sense in the BMP context.

### bgp_bmp_encode_rte

The helper is surprisingly small because it reuses everything. It
allocates a temporary `bgp_bucket`, populates it with the single
route, calls `bgp_encode_attrs` + the bucket's NLRI encoder, and
returns the end pointer. The caller wraps the bytes in a BMP Route
Monitoring PDU and queues it to the monitoring station.

`bmp_route_monitor_end_of_rib` (`proto/bmp/bmp.c:888`) sends a BMP
Peer Up End-of-RIB marker when a monitored peer finishes its initial
table load.

### For ze

ze does not yet support BMP. When it does, BIRD's "reuse the BGP
encoder" pattern is the right implementation: the BMP protocol lives
in its own package, but the actual UPDATE byte stream comes from the
BGP wire layer, because the whole point of BMP is that it speaks BGP
message syntax.

The ze-specific wrinkle is that ze's zero-copy forwarding path
already has wire bytes readily available. A BMP implementation for
ze can subscribe to the incoming wire buffer stream and forward it
to the monitoring station with minimal modification, avoiding the
re-encoding cost that BIRD has to pay (because BIRD eagerly decodes
every UPDATE and then re-encodes for BMP).

---

## 23. Filter language integration

The filter language is BIRD's signature feature and is extensively
covered in `docs/research/comparison/bird/04-filter-language.md`. This
section is a one-screen summary of how BGP interacts with it.

- **Import filter**: called from `rte_update` after attribute decode
  but before table insertion. A filter returns ACCEPT, REJECT, or
  REJECT-AND-WITHDRAW. Filters can mutate attributes before the
  accept (`eattr.fresh = 1` marks mutated attributes for non-cached
  processing).
- **Export filter**: called from the TX path before bucket assignment
  in `bgp_update_attrs` (`proto/bgp/attrs.c:2338`). Same three
  outcomes. Same mutation semantics. The difference is that an
  export filter's mutations produce a new ea_list that may not match
  any existing bucket, forcing bucket allocation.
- **Filter VM**: the filter language is compiled to a linear
  instruction stream and executed by a stack VM
  (`filter/f-inst.c`). Performance is "one filter is not free, but
  100 filters is not 100x worse than one" because the decode is
  done once and the interpreter loop is tight.

For ze's current YANG-driven filter model, the BIRD pattern is a
reference point for "what a programmable filter language would look
like" but not a direct port target. ze's YANG is more declarative and
less expressive. The tradeoff is that YANG is testable via schema and
BIRD filters are testable only by execution.

---

## 24. Memory management: stonehenge and slabs

BIRD 3.x uses three allocator layers:

1. **Slab allocator** (`lib/slab.c`, not read in detail here): fixed-
   size-block allocator for hot objects like `rte_storage`,
   `bgp_prefix`, and small `ea_list` bodies. O(1) alloc and free.
2. **Stonehenge** (`sth_*` functions in rt-attr.c and elsewhere):
   a variable-sized slab variant with reference-counted allocations,
   used for `bgp_bucket` storage (variable size due to inline
   `eattrs[]`) and for large `ea_storage` entries.
3. **linpool** (`lib/mempool.c`): a bump-pointer arena freed in one
   shot. Used for per-task work via `tmp_linpool`: a BGP RX handler
   decodes into tmp_linpool, interns what it wants to keep into the
   slab hash table, and the linpool is reset at task boundary.

The separation is principled: slab is for long-lived objects with
O(1) ref cycles, stonehenge is for variable-sized interned blobs,
linpool is for temporary work that dies at the end of a task. A BGP
RX handler touches all three in a single pass: `tmp_linpool` for the
decode scratch, stonehenge for a newly interned ea_list, slab for
the rte_storage node.

### For ze

ze's Go implementation uses per-type pools plus the GC. The
interesting design question BIRD answers is **when to use arenas vs
GC**: BIRD's linpool is essentially a manual arena, which in Go maps
to "allocate a `[]byte` and use sub-slices" or to a
`sync.Pool`-backed arena. ze already does this in the wire codec
(pooled message buffers) but does not have an explicit
"per-task temporary arena" pattern. Adding one for the BGP RX path
(parse into a per-peer temporary arena, intern interesting bits,
release the arena) would cut GC pressure during convergence.

---

## 25. Constants and tunables

| Constant | Value | Location | Meaning |
|----------|-------|----------|---------|
| `BGP_PORT` | 179 | `bgp.h:646` | TCP port |
| `BGP_VERSION` | 4 | `bgp.h:647` | protocol version |
| `BGP_HEADER_LENGTH` | 19 | `bgp.h:648` | marker + length + type |
| `BGP_HDR_MARKER_LENGTH` | 16 | `bgp.h:649` | RFC 4271 marker |
| `BGP_MAX_MESSAGE_LENGTH` | 4096 | `bgp.h:650` | RFC 4271 max |
| `BGP_MAX_EXT_MSG_LENGTH` | 65535 | `bgp.h:651` | RFC 8654 extended max |
| `BGP_RX_BUFFER_SIZE` | 4096 | `bgp.h:652` | standard RX buffer |
| `BGP_TX_BUFFER_SIZE` | 4096 | `bgp.h:653` | standard TX buffer |
| `BGP_RX_BUFFER_EXT_SIZE` | 65535 | `bgp.h:654` | extended RX |
| `BGP_TX_BUFFER_EXT_SIZE` | 65535 | `bgp.h:655` | extended TX |
| `BGP_MSG_HDR_MARKER_POS` | 0 | `bgp.h:661` | offset of marker |
| `BGP_MSG_HDR_LENGTH_POS` | 16 | `bgp.h:663` | offset of length |
| `BGP_MSG_HDR_TYPE_POS` | 18 | `bgp.h:665` | offset of type |
| `BGP_ROLE_*` | 0..4, 255 | `bgp.h:209-214` | RFC 9234 roles |
| `BFS_*` | 0..4 | `bgp.h:937-941` | feed state |
| `BS_*` | 0..6 | `bgp.h:885-895` | FSM states |
| `BSS_*` | 0..2 | `bgp.h:906-908` | startup substates |
| `BGP_GRS_*` | 0..2 | `bgp.h:253-257` | GR session states |
| `BGP_AUTH_*` | 0..2 | `bgp.h:205-207` | none/MD5/AO |

The `BGP_AF_*` constants (`bgp.h:43-54`) are packed
`(AFI << 16) | SAFI` values covering the 12 AFI/SAFI combinations
BIRD supports. A new AF adds one `BGP_AF_*` plus one entry in
`bgp_af_table` and the codec hooks in `proto/bgp/packets.c`.

---

## 26. Patterns worth stealing

Summarised from the sections above, ranked by value for ze:

1. **Dual-slot FSM for simultaneous open** (section 5): two
   `bgp_conn` per peer, winner chosen at OPEN time. Eliminates
   simultaneous-open flaps.
2. **Public/private struct split with loop lock** (section 4): outside
   view has safe fields only; the private variant is only reachable
   inside the lock.
3. **setjmp / WITHDRAW / DISCARD vocabulary** (section 7): three
   outcomes per decode hook, dispatcher implements the try/catch
   once. Standard RFC 7606 compliance.
4. **Unified ea_list attribute model** (section 10): one storage for
   all attributes, one interning path.
5. **Deferred refcount drops** (section 11): batch decrements at
   task boundary, not inline. 50-80% saving on lock traffic during
   convergence.
6. **Bucket-based UPDATE construction** (section 18): group
   prefixes by attribute-set identity, encode attributes once per
   bucket. Mandatory for route-server scale.
7. **Lockfree route export journal** (section 16): N consumers
   drain the same journal independently, explicit cork/uncork
   backpressure.
8. **BFS_* state machine for enhanced refresh** (section 20):
   explicit per-channel feed state for refresh demarcation.
9. **LLGR stale-route feeder** (section 19): reuse the refresh feeder
   for stale marking, no separate walk loop.
10. **Shared listen sockets with per-peer dispatch** (section 6):
    one socket serves many peers, enables dynamic BGP.

---

## 27. Patterns NOT to copy

1. **Single-threaded dispatch as a baseline assumption**. BIRD 2.x's
   "one thread runs everything" model let shared mutable state work
   and made BIRD 3.x's rewrite painful. ze starts concurrent; never
   introduce "this is safe because we are single-threaded" code,
   even temporarily.
2. **Global RTA_LOCK for every UPDATE**. BIRD's RX path takes the
   global RTA_LOCK to intern every attribute set. Tolerable only
   because single-threaded dispatch already serialises
   everything. Concurrent daemons need per-peer pools, which ze
   already has.
3. **Global symbol table for attribute classes**. BIRD assigns
   ea_class ids at registration, so an attribute's id depends on
   module link order. Load-bearing because the filter bytecode
   references ids directly, but it prevents clean plugin unload.
   Use stable ids from a protocol constant or YANG leaf name.
4. **Bison-generated config grammar**. Powerful but hard to
   maintain, test, and diff. ze's YANG is the right call. BIRD's
   filter bytecode VM is a separate question with its own merits.
5. **Preprocessor-gated protocol variants** (like FRR's
   `#ifdef FABRICD`). BIRD does not do this. Neither should ze.
6. **Verbose per-peer configuration**. Large BIRD deployments have
   hundreds of config lines per peer. ze's YANG tree helps, but
   only if examples and docs do not reintroduce the copy-paste
   pattern.

---

## 28. How BIRD 3.x fills the existing comparison gaps

The existing BIRD comparison files in `docs/research/comparison/bird/`
and the P1 items in `bgp-implementations-analysis.md` are high-level
summaries written largely against BIRD 2.x. This document fills the
v3 gaps they flag:

- **"Attribute deduplication"** (P1): sections 10 and 11 explain the
  unified ea_list model, the SPINHASH intern table, refcounting, and
  deferred drops.
- **"Multithreading (v3.0) 6-8x speedup"**: sections 3 and 4 explain
  birdloops, thread groups, and DOMAIN locks; section 16 explains
  how the route-table journal scales across them.
- **"Bucket-based route export"** (`comparison/bird/13-bucket-export.md`):
  section 18 adds the `last == cur` invariant, the stonehenge
  allocator, the split between `prefix_hash` and `bucket_hash`, and
  the link to `rt_exporter`.
- **"Dual state machines"** (`comparison/bird/01-protocol-abstraction.md`):
  section 17 adds the v3 separation of channel state from feeder
  state, CS_PAUSE, and the soft-reconfigure compare path.
- **"Route refresh"** (P1): section 20 adds the enhanced refresh
  state machine.
- **"GR + LLGR"**: section 19 adds the feeder-reuse pattern for stale
  marking.

What is still not well covered anywhere and is worth a follow-up doc:
BIRD's flowspec validation path (`bgp_channel_config.base_table` at
`proto/bgp/bgp.h:199`), the RPKI protocol in `proto/rpki/`, and the
TCP-AO keychain integration (`bgp_ao_state` at `proto/bgp/bgp.h:303-313`).

---

## 29. Recommended reading order for a ze BGP implementer

Read BIRD's source in this order if you are building or reviewing
ze's BGP. Each step has a specific question it answers, so skim
until you hit something worth pursuing. Total: about 3.5 hours.

| Step | File | Lines | Focus |
|------|------|-------|-------|
| 1 | `proto/bgp/bgp.h` | full | Data model and constants (15 min) |
| 2 | `proto/bgp/bgp.c` | 196-260, 1248-1530, 1807-1870, 1915-2020, 2717-2860 | Listen sockets, FSM, timers, start/stop, reconfigure (30 min) |
| 3 | `proto/bgp/packets.c` | 892-1060, 2609-2760, 2818-2920, 3144-3250 | bgp_rx_open/collision, bgp_create_update, bgp_rx_update, bgp_fire_tx (30 min) |
| 4 | `proto/bgp/attrs.c` | 1058-1250, 1458-1676, 2338-2400, 2525-2860 | Attribute table, encode/decode/finish, update_attrs, best-path (20 min) |
| 5 | `lib/route.h` | 1-400 | ea_list, eattr, rte, rt_export_request (30 min) |
| 6 | `nest/rt-attr.c` | 980-1650 | normalize, hash, lookup_slow, free_deferred (20 min) |
| 7 | `nest/rt-table.c` | 2006-2100, 2639-2800 | rte_announce, rte_update, rte_import (30 min) |
| 8 | `nest/protocol.h` | 371-780 | Protocol + channel state machines (20 min) |
| 9 | `lib/io-loop.h` + `sysdep/unix/io-loop.c:2080-2270` | | Birdloops and thread groups (20 min) |
| 10 | `lib/locking.h` | 1-300 | DOMAIN, LOCK_DOMAIN, order enum (15 min) |

Deeper dives should start from a specific question, not from
"read more BIRD".

---

## Lessons for ze (the short list)

1. **Read `lib/route.h` before writing another attribute type**.
   BIRD 3.x's unified ea_list is the correct answer to "how should
   route attributes be stored"; ze's current dual-layer approach
   (pool handles plus wire bytes) needs to justify itself against it.
2. **Adopt the dual-slot FSM for simultaneous open**. Small change,
   eliminates a real production flap.
3. **Adopt the three-outcome parse vocabulary** (fatal / withdraw /
   discard) and audit every decode hook against it. This is the
   single most common RFC 7606 compliance gap.
4. **Implement enhanced route refresh demarcation** (BFS_* state
   machine). The delta from plain refresh is small and the operator
   visibility is much improved.
5. **Study the bucket pattern before ze grows to route-server
   scale**. Sections 18 and 22 are the architectural prerequisite.
6. **Port the deferred-ref-drop pattern** for ze's attribute pools.
   Under convergence, the lock savings are measurable.
7. **Learn from BIRD 2.x to 3.x**. Every ze design decision should
   ask "what happens when this becomes multi-threaded?" because
   BIRD's rewrite is proof that retrofitting is painful.
8. **Keep operator-surface code out of BGP proper**. BIRD's
   `proto/bgp/` is 12k lines because the operator surface lives in
   `nest/`, `filter/`, and `conf/`. ze should enforce the same
   discipline: the BGP package owns protocol logic, and CLI, YANG,
   and web UI live elsewhere.
9. **The lockfree journal is the right N-consumer export
   pattern**. ze's telemetry system already has one; extending the
   model to BGP route export is a scalability investment that pays
   back at 100+ peer counts.
10. **When in doubt, trust the RFC**. BIRD implements RFC 7606
    treat-as-withdraw, RFC 9687 send hold, RFC 7313 enhanced
    refresh, and RFC 9234 roles because each of these fixes a real
    production failure mode. ze should implement them for the same
    reason.
