# IS-IS Implementation Reference: FRR as a Complement to the bio-rd Guide

## How to read this document

This is the second of two research documents on IS-IS. The first,
`isis-implementation-guide.md`, derives its shape from bio-routing's Go
implementation. bio-rd is a small, clean library but it is incomplete: no SPF,
no Level 1, no LFA, no Multi-Topology, no Segment Routing, no BFD, no graceful
restart, no real authentication, no IPv6 reachability TLVs, and no route
installation. A clean-room implementation in ze that only follows the bio-rd
guide would end up with exactly the same holes.

FRR's `isisd` is the opposite: roughly 62,000 lines of C, derived from the
GNU Zebra IS-IS daemon and continuously maintained by a broad community of
network operators, vendors, and academics since 2001. It implements everything
bio-rd punts on, plus a lot more (OpenFabric, SR-MPLS, SRv6, Flex-Algo, LDP-IGP
sync, SNMP, NETCONF, topotests). It is production-grade, warts included.

This document walks through the FRR implementation with two goals in mind:

1. **Fill the gaps.** Every area where the bio-rd guide says "not implemented,"
   "stubbed," or "out of scope for a first pass," this document tells you how
   FRR actually does it, with file:line citations you can verify.
2. **Offer a second architectural lens.** FRR and bio-rd made very different
   choices on threading, state machines, flooding, and configuration. Seeing
   both clarifies what is an IS-IS requirement versus an implementation choice.
   ze can then pick deliberately.

It is not a replacement for the bio-rd guide. The protocol overview, wire
format, TLV registry, and known-hard-problems in sections 1-2 and 12 of the
first document apply verbatim. Treat this as an overlay.

### A note on the code base and clean-room discipline

All citations below refer to the checkout at
`~/Code/github.com/FRRouting/frr/isisd/` at the state explored on 2026-04-11.
File sizes and line numbers are from that snapshot. FRR moves, so verify before
copying an exact line number into a ze commit message.

Following the same discipline as the bio-rd guide, this document avoids direct
reproduction of FRR source code. Field names, enum values, and function
signatures are cited because they are part of the published data model and
protocol interface. Algorithms are described in prose or pseudocode, not by
quoting the implementation. A ze implementer should be able to read this
document and write a compatible implementation without reading the FRR source.
The file:line citations exist so the reference can be verified, not so the
reference can be copied.

The OpenFabric daemon (`fabricd`) shares the isisd codebase via the `FABRICD`
preprocessor define (`isisd.h:32-57`). Most of what follows applies to both,
and the `#ifdef FABRICD` sites are the places where IS-IS and OpenFabric
diverge. ze should target IS-IS; OpenFabric is out of scope.

---

## 1. FRR isisd at a glance

Line counts (verified, see `tmp/research/frr-isis-lines-sorted.txt`):

| File | LOC | Purpose |
|------|----:|---------|
| `isis_tlvs.c` | 8,468 | All TLV/sub-TLV encode/decode. Single largest file |
| `isis_nb_config.c` | 4,897 | YANG northbound config callbacks |
| `isis_cli.c` | 4,133 | VTY/CLI command handlers |
| `isisd.c` | 3,994 | Area lifecycle, VRF binding, public API |
| `isis_spf.c` | 3,587 | Dijkstra, per-level SPF trees, route generation |
| `isis_snmp.c` | 3,467 | RFC 4444 SNMP MIB |
| `isis_pdu.c` | 2,629 | PDU encode, RX dispatch, send helpers |
| `isis_lsp.c` | 2,547 | LSPDB, aging, origination, flooding helpers |
| `isis_lfa.c` | 2,372 | LFA, TI-LFA, Remote LFA |
| `isis_te.c` | 2,227 | Traffic Engineering extensions |
| `isis_circuit.c` | 1,721 | Per-interface lifecycle |
| `isis_zebra.c` | 1,627 | ZAPI client (route install, interface events) |
| `isis_nb.c` | 1,463 | Northbound callback table |
| `isis_sr.c` | 1,384 | Segment Routing MPLS |
| `isis_vty_fabricd.c` | 1,141 | OpenFabric-specific CLI |
| `isis_route.c` | 957 | Route construction from SPF results |
| `isis_adjacency.c` | 949 | Adjacency state machine |
| `isis_redist.c` | 893 | Redistribution filters |
| `isis_srv6.c` | 789 | Segment Routing over IPv6 |
| `fabricd.c` | 784 | OpenFabric daemon entry |
| ... | ... | plus ~25 smaller files for ~10k more lines |

**Total: 62,273 lines**, compared to bio-rd's ~15 files and roughly 3,500 lines
of IS-IS code (most of which is TLV codecs). The 18x difference tracks the
feature gap almost exactly: FRR's TLV code alone is larger than all of bio-rd's
IS-IS implementation, because FRR supports TE, SR, SRv6, MT, Flex-Algo, LDP
sync, graceful restart, and router capability sub-TLVs. The remaining 50k lines
go into the protocol engine, SPF, LFA, YANG, CLI, SNMP, zebra integration, and
platform abstraction.

For scoping purposes: a minimum viable ze IS-IS (L2 only, IPv4+IPv6, SPF,
flooding, Dijkstra, no LFA/SR/TE/MT) is realistic in the 8-12k line range in
Go. That is roughly 3x bio-rd (because you add SPF and IPv6 and real auth) and
5x-8x smaller than FRR (because you skip the operator plumbing).

---

## 1.5. How FRR organises the code

Before diving into the data model, it is worth understanding how FRR
decomposes the IS-IS daemon across files. A reader coming from bio-rd
(which organises around a handful of Go packages) or from ze (which uses a
plugin-registration model) will find FRR's structure unfamiliar. Reading
the decomposition gives you three things: (a) a mental index so you can
navigate the source without a lost-in-the-woods moment, (b) the rationale
behind FRR's packaging choices, and (c) a starting point for ze's own
package layout that is not just copied from the bio-rd guide's section 15.

### The one-line summary

FRR splits isisd along three axes: **what the code does** (protocol
engine, data model, operator surface), **who it talks to** (raw sockets,
zebra, LDP, BFD, northbound clients), and **which feature it implements**
(core IS-IS vs SR vs TE vs LFA vs MT vs OpenFabric). Each axis gets its
own compilation unit. Headers are declarations only; source files hold
the implementation. There is no "public API" vs "private API"
distinction, because the whole daemon is one statically linked binary.

### Axis 1: by subsystem (the protocol pipeline)

These are the files that would exist in any IS-IS daemon, regardless of
extensions:

| Subsystem | Files | Role |
|-----------|-------|------|
| Platform packet I/O | `isis_pfpacket.c`, `isis_bpf.c`, `isis_dlpi.c` | Raw L2 frame socket, one file per OS backend |
| PDU codec | `isis_pdu.c`, `isis_tlvs.c` | Wire format encode/decode, dispatch on PDU type |
| Daemon lifecycle | `isisd.c`, `isis_main.c` | Area creation/destruction, VRF binding, process entry |
| Circuit lifecycle | `isis_circuit.c`, `isis_csm.c` | Per-interface state machine and I/O driver |
| Adjacency lifecycle | `isis_adjacency.c` | Neighbour FSM, hold timer, reason codes |
| LSP lifecycle | `isis_lsp.c` | LSPDB operations, origination, aging, purge |
| Flooding | `isis_tx_queue.c`, `isis_flags.c` | Per-circuit LSP transmission with retry |
| SPF | `isis_spf.c`, `isis_route.c` | Dijkstra + route construction from results |
| Redistribution | `isis_redist.c` | Importing non-IS-IS routes into LSPs |
| FIB integration | `isis_zebra.c` | Sending routes to zebra for kernel install |
| DR election | `isis_dr.c` | LAN DIS election and pseudo-node LSPs |
| Hostname cache | `isis_dynhn.c` | RFC 5301 Dynamic Hostname TLV handling |
| Common types | `isis_common.h`, `isis_constants.h`, `isis_errors.[ch]` | Shared definitions |
| Checksum | `iso_checksum.[ch]` | Fletcher algorithm |
| PDU counters | `isis_pdu_counter.[ch]` | RX/TX/drop counters per PDU type |
| Misc utilities | `isis_misc.[ch]`, `isis_events.[ch]` | Format helpers, event log entries |

### Axis 2: by feature (optional extensions)

Each of these is an IS-IS extension beyond the base RFC 10589 behaviour,
and each lives in its own file (or small pair of files):

| Feature | Files | Scope |
|---------|-------|-------|
| LFA / TI-LFA / Remote LFA | `isis_lfa.[ch]` | 2544 LOC |
| Multi-Topology | `isis_mt.[ch]` | 672 LOC |
| Segment Routing (MPLS) | `isis_sr.[ch]` | 1619 LOC |
| Segment Routing v6 | `isis_srv6.[ch]` | 973 LOC |
| Traffic Engineering | `isis_te.[ch]` | 2351 LOC |
| Flex-Algo | `isis_flex_algo.[ch]` | 383 LOC |
| LDP-IGP sync | `isis_ldp_sync.[ch]` | 695 LOC |
| BFD integration | `isis_bfd.[ch]` | 235 LOC |
| Affinity maps | `isis_affinitymap.[ch]` | 97 LOC |
| OpenFabric variant | `fabricd.[ch]`, `isis_vty_fabricd.c` | 1970 LOC, gated by `#ifdef FABRICD` |

The value of this axis is that every feature is a grep-able boundary. If
a ze implementer wants to know "what would LFA actually touch", the
answer is: the contents of `isis_lfa.c`, plus the LFA-specific fields on
`isis_area`, `isis_circuit`, and `isis_spftree`. No hunting across files.

### Axis 3: by operator surface (how humans interact)

This is the largest single area of FRR isisd (roughly 13,000 LOC combined,
~20% of the daemon). It exists because FRR supports five simultaneous
management interfaces: CLI (vty), SNMP, NETCONF, gRPC, and a direct C API
through northbound:

| Interface | Files | Role |
|-----------|-------|------|
| CLI command handlers | `isis_cli.c` (4133 LOC) | Interactive VTY commands |
| Northbound callback table | `isis_nb.[ch]` (2306 LOC) | Single-source-of-truth YANG dispatch |
| Config writers | `isis_nb_config.c` (4897 LOC) | One callback per writable leaf |
| State readers | `isis_nb_state.c` (632 LOC) | One callback per operational leaf |
| Notification emitters | `isis_nb_notifications.c` (586 LOC) | YANG notification generation |
| SNMP MIB | `isis_snmp.c` (3467 LOC) | RFC 4444 MIB implementation |

### The header/source discipline

Every `.c` has a matching `.h`, with a strict rule: **headers declare,
source files implement**. FRR applies this consistently:

- **In a header**: struct definitions (when passed across files), enum
  declarations, function prototypes, macros used by multiple files. No
  code bodies except for a handful of trivial `static inline` helpers
  where the compiler needs to inline across compilation units.
- **In a source file**: static helper functions (never appear in
  headers), lifecycle code, event handlers. Statics are marked `static`
  and kept file-local.
- **Include guards** on every header (`#ifndef FOO_H`).
- **Minimal transitive includes**: an `isis_foo.h` that references
  `struct event *` includes `"frrevent.h"`, not a catch-all bundle.

The one deliberate exception is `isis_spf_private.h` (412 lines, 20+
`static inline` helpers). The `_private.h` suffix signals "this header
is internal to the SPF subsystem, do not include it elsewhere". FRR uses
this trick to keep SPF-scoped helpers visible across `isis_spf.c` and
`isis_lfa.c` without exposing them to the whole daemon. It is the closest
C gets to a "package-private" visibility level.

### File size is not a decomposition signal

`isis_tlvs.c` is 8468 lines, the single largest file in the daemon. A
reader might expect it split into `tlv_area.c`, `tlv_extended_ip.c`, and
so on. FRR does not do this. The (implicit) rationale: all TLV codec
functions share a common dispatch table and a lot of bytestream helpers.
Splitting into N files would force those helpers into a header, polluting
the public surface. Keeping them in one file lets them stay `static`.

`isis_spf.c` at 3587 lines follows the same reasoning: `process_N()`,
`add_to_paths()`, `preload_tent()` are file-private helpers that call
each other. A split would expose them.

`isis_lfa.c` at 2372 lines is arguably a different story, but the LFA
variants (plain, TI-LFA node, TI-LFA link, Remote LFA) share the
P-space/Q-space computation and keeping them together preserves helper
encapsulation.

**The rule FRR follows**: split a file when the split gives you a
testable boundary or a reusable abstraction. Do not split for file-size
cosmetics. Keep helpers file-local as long as you can.

### The `isisd.c` / `isis_lsp.c` / `isis_circuit.c` trio

These three "core" files together own 8262 lines. The division is worth
calling out:

- **`isisd.c`** (3994 lines) owns the **daemon lifecycle**: area create
  and destroy, VRF binding, top-level config application, the C API that
  CLI and northbound call into. Nothing that walks packets or LSPs lives
  here. This is "glue that holds an isisd daemon together".
- **`isis_circuit.c`** (1721 lines) owns the **per-circuit lifecycle**:
  interface binding, raw socket open and close, per-circuit timer setup,
  transmit helpers. The circuit data model lives here; the circuit FSM
  itself is next door in `isis_csm.c`.
- **`isis_lsp.c`** (2547 lines) owns the **per-LSP lifecycle**: LSPDB
  operations, LSP construction (`lsp_build`), aging, flooding trigger,
  sequence number management. All code that manipulates LSP content.

There is **no single "IS-IS protocol engine" file**. The protocol logic
is distributed across these three plus `isis_pdu.c` (packet handling)
and `isis_spf.c` (route computation). A reader wanting "the IS-IS
implementation" has to follow the data as it flows through multiple
files.

### The `_nb_` northbound convention

FRR's northbound split is the clearest operator-surface pattern in the
daemon and the one most worth copying into ze. Five files:

| File | Purpose |
|------|---------|
| `isis_nb.h` | Declares every callback function prototype, one per YANG leaf. Roughly 200 prototypes. |
| `isis_nb.c` | The callback table. One `struct nb_option` entry per YANG path. Single source of truth for "what YANG paths does isisd support". |
| `isis_nb_config.c` | Implementation of writer callbacks (create, modify, destroy). Each function validates, updates the data model, schedules side effects. |
| `isis_nb_state.c` | Implementation of reader callbacks. Pure data extraction from the in-memory model. |
| `isis_nb_notifications.c` | Emitters for YANG notifications (adjacency state change, LSP received, authentication failure). |

The value of this split: anyone asking "what does YANG path X do" greps
`isis_nb.c` for the path, jumps to the function name, lands in
`_config.c` or `_state.c`, and reads a 15-line callback. No hunting
across the codebase. ze already has a northbound equivalent for its BGP
configuration; extending this discipline to an IS-IS component is a
natural fit.

### Platform abstraction: one contract, three implementations

FRR's raw-frame I/O story (section 6 of this document) deserves a
structural note. FRR does **not** have an abstract `platform.c` with a
runtime dispatch table. Instead, the **build system** selects exactly one
of `isis_pfpacket.c`, `isis_bpf.c`, `isis_dlpi.c` at compile time. The
selected file provides the `rx` and `tx` function pointers that go into
`struct isis_circuit`. The contract is the function signature in
`isis_circuit.h:96,98`. No vtable, no interface struct; the linker does
the dispatch.

In Go this translates to **build tags**: `circuit_linux.go`,
`circuit_darwin.go`, `circuit_freebsd.go`, each implementing the same
Go interface. You get the same zero-runtime-overhead dispatch without a
preprocessor and without a platform-select shim. This is the right
pattern for ze.

### What stays in `isisd/` vs what moves to `lib/`

FRR's `lib/` directory holds code shared across all daemons: bgpd, ospfd,
isisd, ripd, ldpd, pimd, zebra itself. isisd reaches into `lib/` for:
event loop (`lib/frrevent.c`), keychain (`lib/keychain.c`), prefix
handling (`lib/prefix.c`), northbound framework (`lib/northbound.c`),
red-black tree wrapper (`lib/typesafe.h`), skiplist (`lib/skiplist.c`),
route table radix tree (`lib/table.c`), SPF backoff (`lib/spf_backoff.c`),
Fletcher checksum (`lib/checksum.c`).

**The rule**: a utility that could plausibly be used by another routing
protocol goes in `lib/`. An IS-IS-specific artifact stays in `isisd/`.
Notice that the SPF algorithm itself stays in `isisd/` even though
Dijkstra is protocol-generic, because the vertex types and LFA extensions
are IS-IS-specific. The priority-queue primitive (`lib/skiplist.c`) is
the generic part, and that is what gets promoted to shared.

For ze: the equivalent question is "what goes in `pkg/ze/...` or a
shared internal helper package, vs what stays in `internal/component/isis/`".
Generic primitives (priority queue, red-black tree, Fletcher) either
reuse standard library or get a shared home outside the component.
IS-IS-specific types (System ID, LSP ID, TLV codec) stay inside the
component.

### Patterns NOT to carry over into Go

Three FRR structural choices that are C-isms and should not be ported:

1. **Preprocessor-branched daemon variants** (`#ifdef FABRICD` scattered
   through the codebase). Go has no preprocessor. OpenFabric or any
   future IS-IS-derived protocol in ze should be a separate package that
   imports and composes IS-IS primitives, not a compile-time variant.
2. **All-in-one 8000-line source files**. Defensible in C because `static`
   helpers are file-scoped. In Go, unexported (`lowercase`) identifiers
   are package-scoped, so the same "hide the helpers" effect is
   achievable with a subpackage. Split by concern where it gives a
   useful Go package boundary.
3. **Global singletons for the daemon master and debug flags**
   (`struct isis_master *im` and `unsigned long debug_*`). FRR's
   concurrency story is "nothing runs in parallel", so globals work. Go
   code needs context passing or daemon-handle injection; avoid
   package-level mutable state.

### Recommended Go layout for ze IS-IS

Translating FRR's decomposition into Go package rules, with ze's existing
patterns. This is meant to complement (not replace) the bio-rd guide's
section 15.

```
internal/component/isis/
├── isis.go              # Component registration, plugin entry point
├── daemon.go            # Top-level Daemon, VRF binding (FRR's isisd.c role)
├── area.go              # Area struct and lifecycle
├── config.go            # Binding to ze's YANG config tree
├── types/               # Domain types (no dependencies)
│   ├── systemid.go
│   ├── sourceid.go
│   ├── lspid.go
│   ├── net.go
│   └── area.go
├── wire/                # PDU encode/decode (FRR's isis_pdu.c + isis_tlvs.c)
│   ├── header.go
│   ├── hello.go
│   ├── lsp.go
│   ├── csnp.go
│   ├── psnp.go
│   ├── tlv.go           # TLV interface and registry
│   ├── tlv_area.go
│   ├── tlv_extended_is.go
│   ├── tlv_extended_ip.go
│   ├── tlv_ipv6_reach.go
│   ├── tlv_hostname.go
│   ├── tlv_auth.go
│   ├── tlv_padding.go
│   ├── tlv_lsp_entries.go
│   ├── tlv_p2p_adj.go
│   ├── tlv_router_cap.go
│   └── checksum.go      # Fletcher
├── circuit/             # FRR's isis_circuit.c + isis_csm.c
│   ├── circuit.go       # Circuit struct and manager
│   ├── state.go         # Four-state circuit FSM
│   ├── circuit_linux.go # AF_PACKET (build tag: linux)
│   ├── circuit_bsd.go   # BPF (build tag: freebsd,openbsd,netbsd)
│   ├── circuit_darwin.go
│   └── circuit_test.go
├── adjacency/           # FRR's isis_adjacency.c
│   ├── adjacency.go
│   ├── state.go         # Adjacency FSM, separate from circuit FSM
│   ├── threeway.go      # RFC 5303 three-way handshake state
│   └── reason.go        # Typed reason-code enum
├── lspdb/               # FRR's isis_lsp.c
│   ├── lspdb.go         # LSPDB store (sorted map or btree)
│   ├── lsp.go           # LSP entry type
│   ├── compare.go       # lsp_compare equivalent
│   ├── aging.go         # Per-second tick, purge, zero-age handling
│   ├── origination.go   # LSP build with self-tracing triggers
│   └── fragment.go      # Fragmentation across frag 0..255
├── flood/               # FRR's isis_tx_queue.c
│   ├── txqueue.go       # Per-circuit queue with self-rescheduling retry
│   └── txqueue_test.go
├── snp/                 # CSNP/PSNP handling
│   ├── csnp.go
│   └── psnp.go
├── spf/                 # FRR's isis_spf.c + isis_route.c
│   ├── spf.go           # Dijkstra entry point
│   ├── vertex.go        # Vertex types (IS, pseudo-node, IP reach)
│   ├── tent.go          # TENT priority queue (min-heap + dedup map)
│   ├── relax.go         # Edge relaxation step
│   ├── backoff.go       # RFC 8405 SPF backoff
│   ├── route.go         # Route construction from SPF results
│   └── spf_test.go
├── auth/                # FRR's TLV 10/241 handling
│   ├── auth.go          # Authentication dispatch
│   ├── md5.go           # HMAC-MD5
│   ├── sha.go           # HMAC-SHA-*
│   └── keychain.go      # Integration with ze's keychain
├── redist/              # FRR's isis_redist.c
│   └── redist.go
├── fib/                 # FRR's isis_zebra.c boundary
│   └── installer.go     # Send computed routes to ze's sysrib
├── nb/                  # FRR's isis_nb*.c (YANG dispatch)
│   ├── nb.go            # Callback table, single source of truth
│   ├── config.go        # Writer callbacks
│   ├── state.go         # Reader callbacks
│   └── notifications.go # Notification emitters
├── yang/
│   └── isis.yang        # YANG schema
└── cli/                 # FRR's isis_cli.c (operational commands)
    └── commands.go
```

**Deferred** (not in the first cut, added as separate subpackages when
implemented): `lfa/`, `mt/`, `sr/`, `srv6/`, `te/`, `bfd/`, `ldpsync/`,
`gr/`. Each gets its own subpackage when added, not a file dumped into
the core.

### Divergences from the bio-rd guide's layout

The bio-rd guide section 15 proposes a flatter layout. Differences from
the FRR-influenced version above:

| Bio-rd guide | FRR-influenced proposal | Rationale for the change |
|---|---|---|
| `packet/` | `wire/` | Matches ze's existing BGP terminology (`internal/component/bgp/wire/`) |
| `neighbor/` merged with circuit | `adjacency/` separate from `circuit/` | FRR-proven separation of circuit FSM from adjacency FSM |
| No explicit `flood/` | `flood/` as its own subpackage | TX queue is a non-trivial abstraction that deserves isolation and its own tests |
| No `nb/` split | `nb/` with four files | Matches FRR's operator-tested northbound discipline |
| No `auth/` subpackage | `auth/` subpackage | Authentication needs its own keychain integration, crypto primitives, and per-level testing |
| No `fib/` subpackage | `fib/` subpackage | Makes the FIB boundary explicit, so the integration can be tested with a fake installer |

Both layouts are defensible. The FRR-influenced version trades more
subpackages for tighter encapsulation and easier per-component testing.
The bio-rd version trades discoverability for simplicity. For ze, the
tighter-encapsulation side is probably the right choice because ze
already uses many small subpackages elsewhere in `internal/component/`.
Pick one and commit before writing the first struct.

---

## 2. Top-level data model

bio-rd's guide treats the daemon as a flat collection of interfaces with a
shared LSPDB. FRR is explicitly hierarchical.

```
isis_master                  (one per process; owns the event loop)
  isis[]                     (one per VRF)
    area_list[]              (one per configured area tag)
      lspdb[ISIS_LEVELS]             (two LSPDBs, one per level)
      spftree[SPFTREE_COUNT][LEVELS] (IPv4, IPv6, DSTSRC per level)
      circuit_list[]
        u.bc.adjdb[LEVELS]   (broadcast: list of adjacencies per level)
        u.p2p.neighbor       (point-to-point: single adjacency)
      adjacency_list[]       (cross-reference of all adjacencies in the area)
```

Verified against `isisd.h:73-263` and `isis_circuit.h:41-181`.

### Key struct fields worth knowing

**`struct isis_master`** (`isisd.h:73-82`):
- `list *isis` list of per-VRF IS-IS instances
- `event_loop *master` pointer to FRR's libfrr event loop (the daemon's
  single scheduler thread)

**`struct isis`** (`isisd.h:86-103`):
- `vrf_id_t vrf_id` binds to a Linux VRF
- `uint8_t sysid[ISIS_SYS_ID_LEN]` the 6-byte System ID
- `list *area_list` multiple areas per instance
- `route_table *ext_info[REDIST_PROTOCOL_COUNT]` external routes
  learned from zebra for each redistribute source

**`struct isis_area`** (`isisd.h:128-263`) is the fattest struct in the daemon.
Fields that matter for a reimplementer:
- `struct lspdb_head lspdb[ISIS_LEVELS]` red-black tree per level
- `struct isis_spftree *spftree[SPFTREE_COUNT][ISIS_LEVELS]` three trees
  per level: IPv4 (`SPFTREE_IPV4=0`), IPv6 (`SPFTREE_IPV6=1`),
  IPv6-destination-source (`SPFTREE_DSTSRC=2`). Defined at `isisd.h:109-114`.
- `lsp_mtu` the size threshold that triggers fragmentation (default 1497,
  `isisd.h:132`)
- `list *circuit_list`, `list *adjacency_list` the circuits attached to
  this area and every adjacency across all of them
- `struct event *t_tick, *t_lsp_refresh[ISIS_LEVELS], *spf_timer[ISIS_LEVELS]`
  per-area timers, covered in section 4
- `struct isis_passwd area_passwd, domain_passwd` level-1 and level-2
  authentication state (`isisd.h:163-164`)
- `struct isis_sr_db srdb, struct isis_srv6_db srv6db` per-area Segment
  Routing databases (`isisd.h:214-216`)
- `bool bfd_signalled_down, bfd_force_spf_refresh` BFD signalling state
  (`isisd.h:155-156`)
- `struct spf_backoff *spf_delay_ietf[ISIS_LEVELS]` RFC 8405 SPF backoff
  state per level (`isisd.h:243-245`)
- Counter fields tracking LSP generations, SPF runs, auth failures, ID length
  mismatches, rejected adjacencies (`isisd.h:199-260`)

**`struct isis_circuit`** (`isis_circuit.h:75-181`):
- `enum isis_circuit_state state` (four-state CSM, see section 5)
- `int fd` raw socket for the L2 frames
- Function pointers `int (*rx)(...)` and `int (*tx)(...)`
  (`isis_circuit.h:96,98`). This is how FRR abstracts Linux AF_PACKET vs
  BSD BPF vs Solaris DLPI. Covered in section 6.
- `struct isis_tx_queue *tx_queue` per-circuit LSP transmission queue
  (`isis_circuit.h:91`; see section 9)
- `union { isis_bcast_info bc; isis_p2p_info p2p; } u;` broadcast-only or
  P2P-only fields. Adjacency storage lives in here: broadcast has per-level
  `adjdb` lists, P2P has a single `neighbor`.
- `enum isis_hello_padding pad_hellos` three-valued: always, disabled, or
  only during adjacency formation (`isis_circuit.h:69-73`). This matters for
  MTU mismatch detection (section 12.6 of the bio-rd guide).

**`struct isis_adjacency`** (`isis_adjacency.h:63-101`):
- Four states (not three as the bio-rd guide implies):
  `ISIS_ADJ_UNKNOWN, ISIS_ADJ_INITIALIZING, ISIS_ADJ_UP, ISIS_ADJ_DOWN`
  (`isis_adjacency.h:34-39`). `UNKNOWN` is the pre-hello state before the
  sender's role is determined.
- `enum isis_threeway_state threeway_state` explicit RFC 5303 three-way
  handshake state, distinct from `adj_state` (`isis_adjacency.h:88`). The
  bio-rd guide glosses over this distinction.
- `struct iso_address *area_addresses` neighbour's advertised areas. Used
  for L1 adjacency filtering (mismatch = reject, section 12.5 of the bio-rd
  guide).
- `uint16_t *mt_set, unsigned int mt_count` topologies this neighbour
  supports, extracted from their MT Router Info TLV. Without this there is
  no multi-topology routing.
- `struct bfd_session_params *bfd_session` back-pointer to the registered
  BFD session (`isis_adjacency.h:95`). Bio-rd has nothing equivalent.
- `struct list *adj_sids, *srv6_endx_sids` Segment Routing adjacency SIDs.

**`struct isis_lsp`** (`isis_lsp.h:24-48`):
- `struct lspdb_item dbe` red-black tree node. The LSPDB is a
  `PREDECL_RBTREE_UNIQ(lspdb)` declared at `isis_lsp.h:17` and defined at
  `isis_lsp.h:51`, using FRR's typesafe RB-tree wrapper from `lib/typesafe.h`.
- `struct isis_lsp_hdr hdr` parsed fixed header (sequence, checksum,
  lifetime, LSP ID)
- `struct stream *pdu` the raw encoded PDU. FRR keeps both the parsed TLV
  tree **and** the raw wire bytes so that retransmission does not require
  re-encoding. (This is halfway between bio-rd's eager-decode model and
  ze's lazy model. See section 8 for ze implications.)
- `union { list *frags; isis_lsp *zero_lsp; } lspu` fragment 0 owns
  a list of its fragment children; fragments 1..255 back-point to
  fragment 0.
- `uint32_t SSNflags[ISIS_MAX_CIRCUITS]` the legacy per-circuit flooding
  flag bitfield. Still referenced but superseded by the TX queue (section 9).
- `struct isis_tlvs *tlvs` the parsed TLV object graph, reused as long as
  the LSP is in the database.
- `int own_lsp, int age_out, time_t installed, time_t last_generated`
  bookkeeping for refresh and purge.

There is no `struct isis_neighbour` distinct from `isis_adjacency`; FRR uses
one struct for both the "raw hello-seen" and "fully up" cases, and tracks the
transition through `adj_state` and `threeway_state`. Bio-rd splits these in
the guide's section 4a, and ze should decide which model it prefers.

---

## 3. Single-threaded, event-driven

FRR does not use OS threads for IS-IS. There is exactly one scheduler, the
libfrr event loop, and every subsystem hangs off it through
`event_add_*()` calls. Timers, socket readability, and immediate deferred
callbacks are all multiplexed into the same `struct event_loop *master`
referenced from `isis_master.master` (`isisd.h:77`) and as the global
`master` variable (`isisd.h:356`).

This is a fundamentally different choice from bio-rd's "one goroutine per
concern" model, and worth understanding in full before ze picks a lane.

### The three event scheduling primitives

FRR uses wrappers from `lib/frrevent.h`:

- `event_add_timer(master, cb, arg, sec, &event_ptr)` wall-clock timer
- `event_add_timer_msec(master, cb, arg, msec, &event_ptr)` millisecond
  timer
- `event_add_read(master, cb, arg, fd, &event_ptr)` fires when fd is
  readable
- `event_add_event(master, cb, arg, 0, &event_ptr)` runs on the next loop
  iteration (a zero-delay defer)
- `event_cancel(&event_ptr)` cancels a pending event and nils the pointer

Each handler takes `struct event *thread` and pulls its context from
`EVENT_ARG(thread)`. Handlers are cooperative: they must not block, and they
must re-arm their own timers if they want to fire again. `tx_queue_send_event`
at `isis_tx_queue.c:103` is a textbook example: it calls
`event_add_timer(master, tx_queue_send_event, e, 5, &e->retry)` at the top,
schedules the next retry, and then does the work. If the work fails, the retry
is already armed.

### Where each subsystem hooks in

| Subsystem | Event source | Hook |
|-----------|--------------|------|
| PDU receive | Raw socket readable | `event_add_read(master, isis_receive, circuit, fd, &t_read)` set up in `isis_circuit_up()` and re-armed at the tail of `isis_receive()` |
| Hello send | `hello_interval` timer | `event_add_timer(master, send_hello, ...)` per circuit, per level |
| CSNP send | `csnp_interval` timer | `t_send_csnp[level]` on each circuit (`isis_circuit.h:89`) |
| PSNP send | `psnp_interval` timer | `t_send_psnp[level]` on each circuit (`isis_circuit.h:90`) |
| LSP refresh | `lsp_refresh[level]` timer | `area->t_lsp_refresh[level]`, arg is `lsp_refresh_arg` at `isisd.h:116-119` |
| LSPDB aging (tick) | 1-second ticker | `area->t_tick` drives `lsp_tick()` at `isis_lsp.h:55` |
| SPF scheduling | RFC 8405 backoff timer | `area->spf_timer[level]`, callback `isis_run_spf_cb()` at approximately `isis_spf.c:2066` |
| DR election | Per-level broadcast ticker | `u.bc.t_run_dr[level]` (`isis_circuit.h:44`) |
| BFD state change | Zebra ZAPI message | `isis_adj_state_change_hook` at `isis_bfd.c:216`, called from the zebra read path |
| Adjacency expiry | Hold-time timer | `adj->t_expire` drives `isis_adj_expire()` at `isis_adjacency.h:131` |

The timers sit directly on the struct they belong to, which is a nice
ownership-colocation pattern: cancel the struct, cancel its timers, and you are
safe from orphaned callbacks.

### Why this matters for ze

Go's goroutine cost is low enough that bio-rd's "one goroutine per concern"
model is idiomatic and works. FRR's event loop model is what C networking
daemons have done since the 1990s because pthreads are expensive and locking
is error-prone. ze can pick either.

If ze picks the goroutine model (likely, given the existing codebase):
- Use channels for inter-goroutine communication, not shared mutable state.
- Every long-running loop wants `Start/Stop/Wait/stopCh` per the bio-rd guide
  section 5 of `code-review.md`.
- LSPDB writes should be serialized through an "actor" goroutine that owns
  the tree, rather than a shared RWMutex. This preserves the FRR semantic of
  "LSPDB changes are atomic with SRM/SSN updates" without the locking overhead.

If ze picks the reactor/event model (unlikely but possible):
- Copy FRR's `event_add_*` abstractions almost verbatim; they work.
- Be aware that Go's `time.Timer` has the same "drain channel on Stop" gotcha
  that C's timerfd does not; use the `stopTimer` helper pattern from
  `code-review.md` section 2.

The in-between ("one reactor per circuit plus a central LSPDB goroutine") is
legitimate and probably fits ze's existing patterns better than either
extreme.

---

## 4. Circuit state machine: FRR has one, bio-rd does not

The bio-rd guide section 4a describes an adjacency FSM with three states
(Down, Initializing, Up). That is only half the picture. FRR models circuits
as first-class entities with their own FSM, separate from the adjacency FSM,
because "is there an interface to run IS-IS on" and "is a neighbour seen" are
different questions that can race.

### States (`isis_csm.c:38-39`)

```
C_STATE_NA     interface absent or not bound to any area
C_STATE_INIT   interface present, link up, but IS-IS not enabled on it
C_STATE_CONF   IS-IS enabled but interface is down or not yet known to zebra
C_STATE_UP     both of the above: raw socket open, hellos sending, ready to form adjacencies
```

### Events (`isis_csm.c:43-46`)

```
NO_STATE
ISIS_ENABLE      user enables IS-IS on the circuit (CLI/YANG)
IF_UP_FROM_Z     zebra says the interface came up
ISIS_DISABLE     user disables IS-IS
IF_DOWN_FROM_Z   zebra says the interface went down
```

### Transition table (`isis_csm.c:50-198`)

The function `isis_csm_state_change(event, circuit, arg)` is a switch on
`old_state` with an inner switch on `event`. Because there are only 4 states
and 5 events, the whole transition table fits on one screen. Transitions
worth knowing:

| From | Event | To | Action |
|------|-------|----|--------|
| `NA` | `ISIS_ENABLE` | `CONF` | `isis_circuit_configure(circuit, area)` |
| `NA` | `IF_UP_FROM_Z` | `INIT` | `isis_circuit_if_add(circuit, ifp)` |
| `INIT` | `ISIS_ENABLE` | `UP` | `isis_circuit_configure` + `isis_circuit_up()`; on failure roll back to `INIT` |
| `INIT` | `IF_DOWN_FROM_Z` | `NA` | `isis_circuit_if_del()` |
| `CONF` | `IF_UP_FROM_Z` | `UP` | symmetric to `INIT + ISIS_ENABLE`; rollback on failure |
| `CONF` | `ISIS_DISABLE` | `NA` | `isis_circuit_deconfigure()` |
| `UP` | `ISIS_DISABLE` | `INIT` | `isis_circuit_down()` + `isis_circuit_deconfigure()` |
| `UP` | `IF_DOWN_FROM_Z` | `CONF` | `isis_circuit_down()` + `isis_circuit_if_del()` |

Every "already in state X" case simply logs and no-ops, no assertion. This is
the right posture for a protocol that can receive duplicate configuration
events from multiple sources (CLI, northbound, peer daemon).

### Why the circuit FSM matters even if you already have an adjacency FSM

- It gives you a single place to test for race conditions between "user
  enables IS-IS on an interface that is not yet up" and "interface comes up
  before configuration is applied". These are real in large deployments where
  CLI config and interface events arrive out of order.
- It is the natural owner of the raw socket lifetime (opened in
  `isis_circuit_up()`, closed in `isis_circuit_down()`).
- It cleanly isolates interface-level failures (raw socket open fails,
  multicast join fails) from adjacency-level failures (hello never arrives,
  authentication rejects).

For ze, a reasonable model is:

```
CircuitState :: Down | Configured | Bound | Up
```

with a state chart exactly like FRR's, plus a separate adjacency FSM per
neighbour that only operates while the circuit is `Up`. This keeps concerns
cleanly separated.

### Reason codes on state change

While we are on FSMs, note the adjacency reason-code enum at
`isis_adjacency.h:45-51`. It defines five reasons for adjacency state
transitions: seeing self in the neighbour's TLVs, area address mismatch,
hold-timer expiry, authentication failure, and checksum failure. Every
adjacency state change carries one of these as a typed value, not a
string.

This is the same pattern bio-rd uses for BGP (the "reason string" returned
from `run() (state, string)` in `code-review.md` section 1) and it is
worth replicating. A reason code field alongside state makes every state
change self-documenting in the logs, without per-callsite log messages.
Prefer a typed enum over a string so the set of reasons is fixed and
exhaustive across the codebase.

---

## 5. Platform abstraction for L2 frames

Because IS-IS rides directly on Ethernet LLC/SNAP, not on TCP or UDP, it
cannot use Go's `net` package (or C's BSD sockets) in the usual way. FRR
factors the platform difference behind two function pointers on each circuit:
`int (*rx)(circuit, ssnpa)` and `int (*tx)(circuit, level)` (`isis_circuit.h:96,98`).

Three backends:

| Platform | File | LOC | Mechanism |
|----------|------|----:|-----------|
| Linux | `isis_pfpacket.c` | 472 | `AF_PACKET` raw sockets with an attached BPF filter matching DSAP `0xFE` (`ISO_SAP` from `isis_constants.h:21`) |
| BSD | `isis_bpf.c` | 304 | `/dev/bpf*` devices, filter expressed as BPF bytecode |
| Solaris | `isis_dlpi.c` | 593 | DLPI `ioctl()` sequence |

All three implement the same `rx/tx` contract plus multicast group
membership for the IS-IS reserved MACs (`AllL1ISs 01:80:c2:00:00:14`,
`AllL2ISs 01:80:c2:00:00:15`, `AllISs 09:00:2b:00:00:05`). The circuit's
`open_packet_socket()` equivalent is called during `isis_circuit_up()`.

### For ze

Go's `net` package cannot express raw AF_PACKET + BPF filter + multicast MAC
membership. The practical options are:

1. `golang.org/x/net/bpf` plus a direct AF_PACKET syscall wrapper. This works
   on Linux, requires `CAP_NET_RAW`, and is portable to BSD with different
   imports. This is probably the right default.
2. `gopacket/pcap` library. Heavier, requires libpcap, but gives you
   packet-level filtering for free. Good for testing, bad for production.
3. A TUN/TAP abstraction where IS-IS frames ride over a virtual interface.
   Works in containers and for unit tests, not useful in production.

Whichever you pick, wrap it behind a `type Circuit interface { Read() ... ; Write() ...}`
with the same shape as FRR's rx/tx pointers so the test fake and the live
implementation are interchangeable. Bio-rd's `ethernetInterface` abstraction
(referenced in the first guide's section 8) is roughly this pattern already.

---

## 6. PDU receive pipeline

FRR's PDU handling is concentrated in `isis_pdu.c` (2629 lines). The shape is:

```
raw bytes on fd
  isis_receive(thread)                      [isis_pdu.c around line 1843]
    platform->rx(circuit, ssnpa)            [isis_pfpacket_read/bpf_read/dlpi_read]
    isis_handle_pdu(circuit, ssnpa)         [isis_pdu.c around 1790]
      read PDU type byte from stream
      pdu_len_validate(...)                 [length sanity]
      dispatch on type:
        L1_LAN_HELLO, L2_LAN_HELLO, P2P_HELLO
           -> process_hello()               [isis_pdu.c around 596]
              -> isis_new_adj() / update adj_state
              -> isis_spf_schedule() if came up
        L1_LINK_STATE, L2_LINK_STATE
           -> process_lsp()                 [isis_pdu.c around 852]
              -> iso_csum_verify()          [iso_checksum.c:35]
              -> isis_parse_tlvs()          [isis_tlvs.c]
              -> lsp_compare()              [isis_lsp.c ~905]
              -> lsp_update() or lsp_insert()
              -> _lsp_flood() on every other circuit
              -> isis_spf_schedule() if content changed
        L1_COMPLETE_SEQ_NUM, L2_COMPLETE_SEQ_NUM,
        L1_PARTIAL_SEQ_NUM, L2_PARTIAL_SEQ_NUM
           -> process_snp()                 [isis_pdu.c around 1329]
              -> update per-LSP TX queue entries
                (remove acknowledged entries; enqueue missing LSPs)
  re-arm isis_receive on the same fd
```

Key validation points, in order:

1. **Length** via `pdu_len_validate()`. FRR rejects PDUs longer than the ISO
   MTU of the receiving interface; this is where MTU mismatches cause drops.
2. **Checksum** via `iso_csum_verify()` in `iso_checksum.c`. Only LSPs carry
   a checksum (hellos and SNPs do not, which is a common trap when writing
   fuzz harnesses).
3. **Area address match** for L1 PDUs. A L1 IIH or LSP from a neighbour with
   no overlapping area address is rejected with reason
   `ISIS_ADJ_REASON_AREA_MISMATCH`. Bio-rd's guide section 12.5 flags this
   as "easy to miss and causes silent routing failures"; FRR turns it into
   an explicit counter field `area->rej_adjacencies[level]` (`isisd.h:256`).
4. **Authentication** if configured. Handled in `isis_tlvs.c` via
   `isis_tlvs_auth_is_valid()` with lookup through the libfrr keychain
   (`lib/keychain.c`). Failure increments
   `area->auth_failures[level]` (`isisd.h:258`) and logs with reason
   `ISIS_ADJ_REASON_AUTH_FAILED`.

### The design rule this encodes

Every validation step that can fail has a counter and a reason code. There is
no "silently drop" path. For a reimplementer this means: every place a PDU
is rejected, wire it to a counter, and keep the counters per-level because
debugging an L1/L2 split often comes down to "which level is the auth
failing at".

---

## 7. LSPDB: red-black tree keyed by LSP ID

The LSPDB is a per-level red-black tree declared via FRR's typesafe wrapper:

```c
// isis_lsp.h:17
PREDECL_RBTREE_UNIQ(lspdb);

// isis_lsp.h:51
DECLARE_RBTREE_UNIQ(lspdb, struct isis_lsp, dbe, lspdb_compare);
```

The comparison function `lspdb_compare` in `isis_lsp.c` is a straight
`memcmp()` over the 8-byte LSP ID (6-byte system ID + 1 pseudo-node byte
+ 1 fragment byte). Insertion is O(log N); lookup by LSP ID
(`lsp_search(head, id)` at `isis_lsp.h:81`) is O(log N).

### Why RB-tree rather than hash

- **Range queries.** `lsp_build_list(head, start_id, stop_id, num, list)`
  at `isis_lsp.h:83` walks the tree in order to build a CSNP that covers a
  contiguous range of LSP IDs. A hash table cannot do this in sorted order.
- **Deterministic iteration.** `show isis database` prints in order;
  operators rely on this.
- **Aging walk.** `lsp_tick()` at `isis_lsp.h:55` walks the entire tree
  every second decrementing `rem_lifetime`. In-order walk is trivial on an
  RB-tree.

For ze in Go: `google/btree` or the built-in `container/list` both work, but
the cleanest shape is probably a custom node type that embeds both the LSP
and the tree link. Bio-rd uses a map keyed by LSP ID; the guide notes this
in section 5. For a first cut, a sorted `map[LSPID]*LSP` with a lazy
sorted-keys cache is fine. Upgrade to a real tree only if the
CSNP-range-query hot path shows it.

### LSP comparison

`lsp_compare()` at `isis_lsp.h:105-106`:

```c
int lsp_compare(char *areatag, struct isis_lsp *lsp, uint32_t seqno,
                uint16_t checksum, uint16_t rem_lifetime);
```

Returns `LSP_EQUAL`, `LSP_NEWER`, or `LSP_OLDER` (`isis_lsp.h:94-96`). The
rules per RFC 10589 section 7.3.15 and RFC 5308:

1. If one side has `rem_lifetime == 0` and the other does not, the
   zero-lifetime side is **newer** (purges always win).
2. Otherwise, compare sequence numbers (with 2^31 wraparound threshold per
   RFC 5308 section 3.1).
3. If sequences are equal, compare checksums to detect corruption. Unequal
   checksums at equal sequence mean a storage bug or a collision; FRR logs
   and treats the higher-checksum side as newer (arbitrary but
   deterministic).
4. If sequences and checksums match, the one with the higher
   `rem_lifetime` is newer (freshest refresh).
5. Otherwise `LSP_EQUAL`.

Bio-rd's guide section 5 handwaves this comparison. Implementing it wrong is
a classic trap: if you skip rule 1, purges can be dropped and stale LSPs
linger; if you skip rule 4, refresh storms cause duplicates.

---

## 8. LSP flooding via the TX queue

This is the single most interesting part of FRR's IS-IS from an
architecture standpoint and the bio-rd guide has nothing equivalent.

### The old way: SRM/SSN bitfields

RFC 10589 describes flooding in terms of two flags per LSP, per circuit:
- **SRM** (Send Routing Message): "this circuit needs to send this LSP"
- **SSN** (Send Sequence Number): "this circuit needs to PSNP-ack this LSP"

You iterate LSPDB x circuits, check flags, take action, clear flags. FRR's
original implementation did exactly this: `SSNflags[ISIS_MAX_CIRCUITS]` in
`isis_lsp.h:33`, 32 bits per LSP per circuit, with helper macros in
`isis_flags.h`. The flag bitfield still exists for legacy compatibility but
is no longer the primary flooding driver.

### The new way: per-circuit transmission queue with self-rescheduling retry

In 2018 FRR moved to a per-circuit `isis_tx_queue` structure
(`isis_tx_queue.h`, `isis_tx_queue.c`, 189 lines of implementation by
Christian Franke). The shape, described in prose:

A `isis_tx_queue` belongs to one circuit and holds three things: a back
pointer to the circuit, a function pointer for the actual send routine that
knows how to push an LSP onto that circuit, and a hash table of pending
entries. Each entry in the hash table records: the LSP to send, the send
type (regular LSP or circuit-scoped, relevant for OpenFabric), a boolean
saying whether this attempt is a retry, and a per-entry retry timer. The
hash key is a `(level, LSP ID)` pair (`tx_queue_hash_key` at
`isis_tx_queue.c:39-47`), so the same LSP cannot be queued twice for the
same level. External callers only interact via `isis_tx_queue_add` and
`isis_tx_queue_del`.

**The critical algorithm** is in `tx_queue_send_event` (`isis_tx_queue.c:103-117`).
When an entry's timer fires:

1. First, rearm the retry timer for 5 seconds in the future, saving its
   handle on the entry.
2. If this attempt is flagged as a retry, increment the area-wide
   retransmission counter. Otherwise flip the entry's retry flag to true
   so the next fire will count.
3. Invoke the circuit's send function with the entry's LSP and send type.
4. Do not touch the entry after the send call, because the send path may
   have invoked `isis_tx_queue_del` recursively and freed it.

The order of those steps is the entire point. Scheduling the next retry
**before** the send ensures that if the send crashes, panics, or takes
long enough that the circuit is torn down mid-call, the retry is already
committed to the event loop. A naive "send, then schedule retry on
failure" ordering loses the entry in exactly the failure modes you care
about.

The queue's `add` path is different: `_isis_tx_queue_add` finds or
allocates an entry, cancels any pre-existing retry timer, and calls
`event_add_event(master, tx_queue_send_event, e, 0, &e->retry)` for an
**immediate** zero-delay send on the next loop iteration. New LSPs flood
right away; only retries wait 5 seconds.

### Why this beats the flag-bitfield model

- **No iterator**: nothing walks the LSPDB looking for flags. The queue *is*
  the list of work. An idle LSPDB has empty queues.
- **Per-entry timer**: each pending transmission has its own retry timer.
  Losing one LSP on one circuit does not delay retransmission of other LSPs
  on other circuits.
- **Cancellation is cheap**: a PSNP ack calls `isis_tx_queue_del`, which
  cancels the timer and releases the entry. No "clear SRM bit; maybe walk
  the LSPDB next tick" indirection.
- **Debuggable**: every add/delete carries `__func__/__FILE__/__LINE__`
  through the `_isis_tx_queue_add(..., func, file, line)` macro
  (`isis_tx_queue.c:119-123` and the wrapper in `isis_tx_queue.h`). When
  debugging a flooding bug you can trace the exact origin of a queue
  insertion without sprinkling `printf`s.
- **Constant-time update**: `hash_lookup` + `hash_release` beats walking a
  circuit list checking bits per LSP.

### The retry interval and acks

The 5-second retry is an implementation choice, not an RFC requirement. RFC
10589 section 7.3.15.4 says the retransmission interval is implementation
defined and defaults to `MIN_LSP_RETRANS_INTERVAL = 5` in
`isis_constants.h:63`. Two things to know:

1. **Acks come from PSNP processing**: `process_snp()` at
   approximately `isis_pdu.c:1329` walks the PSNP's LSP Entries TLV. For
   each entry at or above the sequence of the local LSP in the DB, it calls
   `isis_tx_queue_del(circuit->tx_queue, lsp)` to remove the pending retry.
2. **CSNP handles bulk sync**: CSNPs list every LSP the sender has in a
   range. The receiver compares sequence numbers and enqueues missing LSPs
   into the TX queue, plus PSNPs any LSPs it has that the sender is missing.
   This is the "one-shot" synchronization path on P2P circuits (section
   7.3.15.2 of the RFC).

### Recommendation for ze

Adopt this pattern. A Go translation:

```go
type txQueue struct {
    circuit *Circuit
    send    func(*Circuit, *LSP, TxType)
    // Key is LSPID + level.
    entries map[txKey]*txEntry
    mu      sync.Mutex
}

type txEntry struct {
    lsp     *LSP
    retry   *time.Timer  // use stopTimer helper on cancel
    isRetry bool
    txType  TxType
}

func (q *txQueue) Add(lsp *LSP, t TxType) {
    // deliver immediately on next scheduler tick
}
func (q *txQueue) Del(lsp *LSP) {
    // stop timer, drop entry
}
```

The "reschedule-first, send-second" order is not optional: it is the thing
that makes retransmission correct under a crash in the send path.

---

## 9. LSP origination and fragmentation

### Triggering an origination

The entry point is a macro at `isis_lsp.h:59-61`:

```c
#define lsp_regenerate_schedule(area, level, all_pseudo) \
    _lsp_regenerate_schedule((area), (level), (all_pseudo), true, \
                             __func__, __FILE__, __LINE__)
```

Every call site captures its own file and line. This is the same
self-tracing pattern the TX queue uses, and for the same reason: LSP
regeneration is triggered from dozens of places (adjacency up/down, metric
change, circuit flap, redistribute update, SR SID allocation, BFD failure,
overload bit toggle, LSP refresh timer) and when debugging a churn storm
you need to know which trigger fired.

The `_lsp_regenerate_schedule` function (declared at `isis_lsp.h:62-64`)
implements minimum-interval throttling using `area->lsp_gen_interval[level]`
(default 30 seconds per `isis_constants.h:61`). If you just regenerated,
the request is deferred to a timer; if not, it fires immediately via
`event_add_timer_msec(..., lsp_refresh_event, ..., 0, ...)`.

`lsp_regenerate_pending[level]` at `isisd.h:153` is a flag that prevents
duplicate schedules. The comment in that struct is worth reading in full
(`isisd.h:142-152`): the same timer serves both "regular refresh" and
"throttled update" and clearing the flag at the right moment is described
as "of utmost importance". This is the kind of subtle point a Go
implementation gets to avoid if each distinct trigger gets its own channel.

### Building the LSP

`lsp_generate()` at `isis_lsp.h:58` drives the rebuild. The actual
TLV-assembly code is in `isis_lsp.c:lsp_build()` (not shown here, roughly
line 1059). The contents:

1. Area Addresses TLV (1)
2. LSP Bits (overload, attached, partition, IS-type)
3. Authentication TLV (10 or 241, must be first per RFC 5304; FRR honours
   this)
4. Dynamic Hostname TLV (137)
5. Protocols Supported TLV (129) listing NLPIDs: `NLPID_IP=204` and
   `NLPID_IPV6=142` from `isis_constants.h:99-100`
6. IP Interface Addresses TLV (132), IPv6 Interface Addresses TLV (232)
7. **Extended IS Reachability TLV (22)** per adjacency, with sub-TLVs for:
   link IDs (4), IPv4 neighbour address (8), IPv6 neighbour address (12),
   admin group (3), TE metric (18), link delay (19), SRLG (16), adjacency
   SID (31/32). Encode code is in `isis_tlvs.c`, fragment list at
   approximately line 900 onwards.
8. **Extended IP Reachability TLV (135)** for IPv4 prefixes from direct
   circuits and passive interfaces, plus imported externals
9. **IPv6 Reachability TLV (236)** symmetrically for IPv6
10. Router Capability TLV (242) when SR is enabled, carrying SRGB, SRLB,
    Algorithms, SR Local Block sub-TLVs
11. Multi-Topology Router Info TLV (229) when MT is enabled
12. Hook callbacks let subsystems attach arbitrary sub-TLVs: TE
    (`isis_te_lsp_event()`), SR (`isis_sr_lsp_event()`), LDP sync

### Fragmentation

If the assembled TLVs exceed `area->lsp_mtu` (default 1497 at `isisd.h:132`),
FRR splits across LSP numbers 1..255. The fragment zero owns the "list of
fragments" via `lspu.frags` (`isis_lsp.h:30`); fragments 1..N back-point to
zero via `lspu.zero_lsp` (`isis_lsp.h:31`). The algorithm:

1. Populate fragment zero with mandatory TLVs (area addresses,
   authentication, hostname, protocols supported, IS-type).
2. Each reachability TLV (22, 135, 236) is split across fragments as
   needed. The RFC allows an LSP to carry multiple instances of the same
   TLV type, so a single neighbour's sub-TLV list can be spread over two
   fragments if it is large.
3. Each fragment gets its own sequence number and checksum. All fragments
   must be refreshed together (advance the sequence in lockstep), or
   old-sequence fragments will be purged by neighbours.

The `lsp_frag_threshold` field (`isisd.h:198`) lets the operator set a
percentage of `lsp_mtu` at which FRR starts a new fragment rather than
packing to the brim. Default is typically 90%. This gives headroom for
future TLV additions without forcing a full re-fragmentation.

### Sequence number wraparound

`isis_constants.h:23` defines `SEQUENCE_MODULUS 4294967296`. The wraparound
handler lives in `lsp_inc_seqno()` at `isis_lsp.h:110`. Per RFC 10589
section 7.3.12 and RFC 5308, at `0xFFFFFFFF`:

1. Set `rem_lifetime = 0`, advertise the purge on all circuits.
2. Wait `MAX_AGE + ZERO_AGE_LIFETIME` seconds (1200 + 60 = 1260s per
   `isis_constants.h:49-50`) for the purge to propagate.
3. Reoriginate starting at sequence 1.

FRR implements this, counting how many times it had to skip a sequence via
`area->lsp_seqno_skipped_counter` (`isisd.h:202`). In practice this code
runs at most once per daemon lifetime and is almost never exercised, which
means if your reimplementation has a bug here you will not notice until a
router has been running for years. Test explicitly.

### Recommendation

Copy the `__func__/__FILE__/__LINE__` self-tracing pattern for origination
triggers. In Go, `runtime.Caller(1)` gives you the caller's PC, and
`runtime.FuncForPC(pc).FileLine(pc)` resolves it. Log the trigger source on
every LSP generation, gated by a debug flag. This is the one piece of
observability that is worth its weight in every IS-IS debugging session you
will ever have.

---

## 10. The SPF implementation

Bio-rd has no SPF. The bio-rd guide section 5 describes Dijkstra in the
abstract but bio-rd does not implement it, does not have a `struct Vertex`,
and does not install routes. This is the single largest gap in the first
document.

FRR's SPF is a fully realised RFC 10589 Dijkstra with multi-topology,
multi-family, and LFA extensions. The structure is in
`isis_spf_private.h` (412 lines) and the algorithm in `isis_spf.c`
(3587 lines, the fifth-largest file in the daemon).

### Vertex types (`isis_spf_private.h:18-29`)

FRR distinguishes ten vertex types, in three groups:

- **IS vertices**: `PSEUDO_IS`, `PSEUDO_TE_IS`, `NONPSEUDO_IS`,
  `NONPSEUDO_TE_IS`. These represent routers (or DIS-generated pseudo-nodes
  on LANs). The TE variants carry traffic-engineering sub-TLVs; the
  non-TE variants do not.
- **End-system vertex**: `ES`, representing a non-IS endpoint. Rarely
  seen in modern deployments (it is an OSI-era idea) but still declared.
- **IP-reach vertices**: `IPREACH_INTERNAL`, `IPREACH_EXTERNAL`,
  `IPREACH_TE` for IPv4, and `IP6REACH_INTERNAL`, `IP6REACH_EXTERNAL` for
  IPv6. Internal vertices represent prefixes learned via intra-area IS
  reachability; external vertices represent redistributed routes.

There is no single "node" type; the SPF graph has heterogeneous vertices
where "a router" and "a prefix attached to a router" are first-class
vertices of distinct types. The Dijkstra relaxation preserves this
distinction by type-dispatching when a vertex is moved from TENT to PATHS.
The vertex struct carries the type tag and a tagged-union identifier: for
IS and ES vertices it holds a 7-byte ID (system ID plus pseudo-node byte);
for IP-reach vertices it holds a destination prefix, an optional source
prefix for RFC 6754 destination-source routing, a Segment Routing
prefix-SID descriptor, and a prefix-priority enum for LFA and route
selection.

For a pure-router graph you could imagine encoding prefixes as attributes
of the router vertex rather than as vertices in their own right. FRR does
not do this because prefix-SIDs, SRLG-aware LFA, and per-prefix priorities
all need to operate on individual prefix vertices. For a first ze
implementation without SR and LFA, you could skip this distinction and
attach prefixes as a list on the router vertex, then fan out to the route
table after the SPF completes. Be aware you are buying simplicity at the
cost of forward compatibility.

### TENT ordering and the vertex queue

The TENT list uses a three-level sort comparator
(`isis_vertex_queue_tent_cmp` at `isis_spf_private.h:121-146`):

1. **Distance from root** (the primary Dijkstra key). Lower distance
   sorts first.
2. **Vertex type.** At equal distance, IS vertices sort before IP-reach
   vertices, so a router is finalized before any prefix attached to it
   is processed. This is what lets the relaxation step safely read the
   parent's finalized state.
3. **Insertion counter.** A monotonically increasing per-queue counter
   breaks remaining ties in insertion order. This gives ECMP selection
   deterministic behaviour across runs: two paths with equal metric
   resolve in the order they were announced, not in hash order.

The queue itself is backed by a **skiplist** (the `skiplist_new` call at
`isis_spf_private.h:149-152`), giving O(log n) insert, delete, and
extract-min. Alongside the skiplist the queue maintains a hash table keyed
on the vertex identity for O(1) "is this vertex already in TENT" lookup
(the `hash` field at `isis_spf_private.h:77`). The skiplist orders; the
hash deduplicates.

This is the data-structure detail that keeps Dijkstra fast on large IS-IS
domains. A naive linear-scan TENT, or an ordered array searched linearly
for existing entries, is the number-one performance trap in a first
Dijkstra implementation. In Go the equivalent is a min-heap from
`container/heap` plus a `map[VertexID]*Vertex` for deduplication; the two
must be kept in sync on every operation.

### Per-level, per-family, per-topology trees

`isis_area->spftree[SPFTREE_COUNT][ISIS_LEVELS]` from `isisd.h:131`:

- `SPFTREE_IPV4` (`isisd.h:110`) IPv4 unicast
- `SPFTREE_IPV6` (`isisd.h:111`) IPv6 unicast
- `SPFTREE_DSTSRC` (`isisd.h:112`) IPv6 destination/source routing (RFC 6754)

Each combined with L1 and L2 gives six SPF trees per area. Each is a
complete, independent Dijkstra run with its own PATHS, TENT, route_table,
and LFA state. The `struct isis_spftree` at `isis_spf_private.h:293-354`
carries all of this.

For a ze first cut: start with one tree (IPv4 L2), then add IPv6, then add
L1, then add DSTSRC if needed. Do not try to write a generic
"multi-anything" SPF up front; the indirection will hurt you.

### The algorithm

`isis_run_spf(spftree)` at `isis_spf.c` around line 1891 is the entry
point. The top-level shape (per RFC 10589 section 7.2.7) is:

1. **Initialize** TENT and PATHS to empty; insert the root vertex at
   distance 0.
2. **While TENT is non-empty**:
    a. Extract the minimum-distance vertex N from TENT.
    b. Move N to PATHS.
    c. For each reachability TLV in N's LSP (or for each adjacency if N is
       the root):
        - Compute the new distance d' = d(N) + metric.
        - If the neighbour is not in TENT or PATHS, insert it with
          distance d'.
        - If the neighbour is in TENT with a greater distance, update
          (decrease-key).
        - If the distance equals the current, add an ECMP parent.
3. When TENT is empty, PATHS is the full shortest-path tree.

FRR's implementation threads SR label stacks, LFA flags, and Multi-Topology
filtering through every step, which is why the file is 3500+ lines. The
core Dijkstra loop would fit in ~200 lines without the extensions.

### Post-SPF: building the route table

`isis_route.c` converts the SPF PATHS into a `route_table` (libfrr's radix
tree, not the same as the LSPDB RB-tree). Each IP-reachability vertex
becomes a route entry whose next hops are computed by walking back from
the vertex through its parents to the root, collecting the adjacencies on
the first hop from the root. The bio-rd guide section 5 describes this
informally; `isis_route.c:isis_route_update()` (around line 53) is the
authoritative version.

Routes are then handed to zebra via `isis_zebra.c` (section 20 below).

### Debouncing: RFC 8405 SPF backoff

`isisd.h:243-245` declares `struct spf_backoff *spf_delay_ietf[ISIS_LEVELS]`.
The IETF SPF backoff algorithm (RFC 8405) is implemented in
`lib/spf_backoff.c` (not in `isisd/`, it is shared with `ospfd`). The idea:

- First change after quiet triggers an immediate SPF.
- Subsequent changes within a short window add exponentially-growing
  delays.
- A long quiet period resets the state.

This avoids the "change storm causes N SPF runs" trap. For ze, the
algorithm is worth copying: it is 200 lines and it is the difference
between "IS-IS adds 5% CPU during convergence" and "IS-IS saturates the
CPU for 30 seconds and the daemon stops responding to configuration".

### Full vs incremental

FRR runs full SPF on every trigger. It does not implement iSPF (incremental
SPF, RFC 10589 appendix C.2.5), and the commit log says this is a deliberate
simplicity choice: iSPF adds complexity (you have to track tree ancestry
and handle partial invalidation) for gains that matter only on very large
domains with very frequent changes. Backoff plus fast full-SPF covers the
same territory at lower implementation cost.

**For ze: do not implement iSPF on the first pass.** Add SPF backoff
instead. If your SPF takes more than 10ms on realistic topologies you have
a data structure bug; fix the bug rather than add iSPF.

---

## 11. LFA, TI-LFA, Remote LFA

bio-rd has none of these. FRR has all three in `isis_lfa.c` (2372 lines)
plus LFA-specific fields scattered through `isis_area`, `isis_circuit`, and
`isis_spftree`.

### What LFA is, in one paragraph

When a router uses IS-IS to forward to a destination, its next-hop
neighbour is determined by SPF. If that neighbour fails, the router must
re-run SPF and reconverge, which can take hundreds of milliseconds. LFA
(RFC 5286) precomputes an **alternate next-hop** for each destination that
can be used immediately when the primary fails, bypassing reconvergence
delay. It is a dataplane fast-failover mechanism, not a control-plane
optimization.

### Plain LFA (RFC 5286)

`isis_lfa_compute()` around `isis_lfa.c:2076` is the core routine. For each
destination vertex in the main SPF result, iterate over all adjacent
neighbours other than the primary next hop; for each, check the RFC 5286
loop-free inequality: `d(N, D) < d(N, S) + d(S, D)` (neighbour's distance
to destination is less than its distance through self). Candidates that
pass are eligible as LFA next-hops.

P-space and Q-space (`isis_spf_private.h:325-326`) are intermediate
computations for deciding which candidates are viable for which protected
link. FRR stores them on the SPF tree so LFA computation reuses them across
multiple LFA queries.

### TI-LFA (RFC 9262, draft-bashandy-rtgwg-segment-routing-ti-lfa)

TI-LFA uses Segment Routing label stacks to construct a **repair path**
that explicitly steers traffic around a failure, regardless of network
topology. Because the repair path is expressed as a label stack, it works
even when no plain LFA exists. Requires SR to be enabled.

FRR's implementation is in the second half of `isis_lfa.c`. The key insight:
after a post-convergence SPF (computed as if the protected link were
already down), the repair list is a sequence of "P-node" and "Q-node"
segment IDs that push the packet onto the post-convergence path. FRR
computes this list during LFA computation and stores it alongside the
primary next-hop in the route_table.

Node protection vs link protection is configurable per circuit:
`circuit->tilfa_node_protection[level]` and
`circuit->tilfa_link_fallback[level]` (`isis_circuit.h:154-155`). Node
protection is strictly stronger but requires more computation and a richer
topology.

### Remote LFA (RFC 7490)

Remote LFA tunnels traffic (via LDP or IGP SR labels) to a "PQ-node"
somewhere in the network that is topologically past the failed link, then
the traffic continues to its destination normally. This extends LFA
coverage to topologies where no direct LFA exists.

FRR's Remote LFA support is in the `remote` section of `isis_lfa.c` plus
`rlfa_tree_head rlfas` and `list *pc_spftrees` on the SPF tree
(`isis_spf_private.h:331-337`). The post-convergence SPF trees are kept
alive until LDP provides the labels for the tunnel endpoints, at which
point the RLFA entries become installable routes.

### Scoping recommendation for ze

Do not attempt LFA on the first pass. It is not required by any RFC to be
IS-IS compliant, and it couples SPF to the SR label allocator for TI-LFA
and to the LDP daemon for RLFA. Do it only after: (a) SPF is stable under
load; (b) SR is implemented; (c) you have an LDP or SRv6 label source. The
complete absence of LFA in bio-rd is not a liability for 95% of deployments.

Do design the route_table so that a "primary + backup" pair is expressible
per prefix. That way you can add LFA later without refactoring the
installation path.

---

## 12. Multi-Topology (RFC 5120)

`isis_mt.c` (559 lines) plus MT fields in every major struct. MT-IDs are
standardized (`isis_mt.c:40-64`):

- `0` IPv4 Unicast (default, always present)
- `1` Reserved (management topology)
- `2` IPv4 Multicast
- `3` IPv6 Unicast
- `4` IPv6 Multicast
- `5` IPv6 Management
- `6` IPv6 Destination/Source (RFC 6754)

An MT-capable router advertises its supported topologies in TLV 229
(Multi-Topology Router Information) and carries MT-specific reachability
TLVs (222 for IS reachability, 235/237 for IP reachability).

### How FRR threads MT through the pipeline

1. **TLV parse** extracts the MT TLV 229 from incoming LSPs and records
   which topologies the originator supports.
2. **Adjacency** records the neighbour's topology set in
   `adjacency->mt_set` (`isis_adjacency.h:93`). When a topology disappears
   from the neighbour's advertisement the adjacency is not torn down, but
   routes in that topology using that neighbour are removed.
3. **SPF** uses `spftree->mtid` (`isis_spf_private.h:310`) to filter which
   reachability TLVs to consider. MT-0 uses classic TLV 22 reachability;
   MT-2 uses TLV 222 for IS reach and TLV 235 for IPv6 reach.
4. **Route table** is implicitly per-MTID because each SPF tree has its
   own `route_table`.

### Scoping for ze

Topologies 0 (IPv4 unicast) and 3 (IPv6 unicast) in a single SPF run with
unified reachability TLVs (22, 135, 236) is the baseline. You can ship
that without any of FRR's MT machinery; it just means you are not
multi-topology-aware and cannot participate in an MT domain. For a first
implementation, document this as "single topology" and move on. MT is
rarely deployed in IS-IS in practice; OSPFv3 got multi-topology wrong, IS-IS
got it mostly right, but most operators use separate routing instances
instead.

---

## 13. Segment Routing, SRv6, TE

The bio-rd guide correctly flags SR, SRv6, and TE as out of scope. FRR has
them all, and the files are worth knowing about even if you do not plan to
implement them:

| File | LOC | Covers |
|------|----:|--------|
| `isis_sr.c` | 1384 | SR-MPLS: SRGB, SRLB, Prefix-SID, Adjacency-SID |
| `isis_srv6.c` | 789 | SRv6 locators and End.X SIDs |
| `isis_te.c` | 2227 | TE sub-TLVs: bandwidth, admin group, SRLG, delay |
| `isis_flex_algo.c` | 328 | RFC 9350 Flex-Algo |

The TLV surface is listed in the bio-rd guide section 2; FRR actually
implements all of it. The key integration points are:

- **Router Capability TLV 242** carries SRGB, SRLB, Algorithms sub-TLVs
- **Extended IS Reachability TLV 22** sub-TLVs 3/4/5/6/8/9/11/12 carry TE
  link attributes
- **Extended IP Reachability TLV 135** sub-TLVs 1-3 carry Prefix-SID info
- **IPv6 Reachability TLV 236** sub-TLVs mirror 135 for IPv6

For ze: if you ever need SR, study `isis_sr.c:isis_sr_init()` for the
registration shape. The hook architecture (TE/SR/LDP-sync attach sub-TLVs
through callbacks when `lsp_build()` runs) is the pattern worth copying.
Do not attempt SR before you have stable Dijkstra.

---

## 14. BFD integration

bio-rd has none; the guide section 4a mentions BFD briefly and moves on.

FRR's `isis_bfd.c` is small (219 lines, shadow of the scale you might
expect) because BFD is implemented in `bfdd`, a separate daemon in the FRR
ecosystem. `isis_bfd.c` is the client side: it registers and deregisters
BFD sessions on IS-IS adjacency events, and reacts to BFD state change
notifications.

Registration flow:

1. Adjacency enters `ISIS_ADJ_UP` state in `isis_adjacency.c`.
2. `isis_adj_state_change_hook` fires (`isis_adjacency.h:117`).
3. `isis_bfd.c:216` registers a listener; it calls `bfd_sess_install()`
   with the neighbour IP, using IPv6 link-local if both are available
   (because BFD over link-local is the correct form on L2-adjacent
   neighbours).
4. On BFD session transition, `bfd_handle_adj_down_callback` is invoked
   from the zebra read path and immediately calls `isis_adj_state_change`
   with state `ISIS_ADJ_DOWN` and reason "bfd session went down".

The per-adjacency `struct bfd_session_params *bfd_session` at
`isis_adjacency.h:95` is just a back-pointer so the cancellation path can
find the session.

BFD configuration per circuit lives in `circuit->bfd_config` at
`isis_circuit.h:144-147`, a struct with `enabled` bool and `profile` name.
The profile is a named set of BFD parameters (interval, detect multiplier)
defined in `bfdd`, not in `isisd`.

### For ze

If ze has or plans a BFD implementation, the adjacency-FSM-hooks pattern
is the right shape: registration on Up, deregistration on Down, a callback
that fires from the BFD state machine and drives the adjacency back to
Down. The hard part of BFD is not the protocol (which is simple) but the
precision timers and the jitter budget; implement it as a separate service
with a well-defined API before wiring it in to IS-IS.

---

## 15. Graceful Restart and LDP-IGP Sync

### Graceful Restart (RFC 5306)

The intent: when IS-IS restarts on one router, its neighbours should keep
the adjacency up and the routes installed, trusting that the restart will
complete before the hold-down kicks in. The mechanism is the Restart TLV
(211) with a non-zero Remaining Time field.

FRR has partial support. The Restart TLV is parsed and its state is
tracked, but the full persistence/replay story is not wired up in all
paths. The relevant code is in `isis_pdu.c` around the IIH processing, with
state stored in the adjacency struct. It is not marked as production-ready
in the code.

### LDP-IGP Sync (RFC 5443, RFC 6138)

`isis_ldp_sync.c` (654 lines) is more mature. The problem this solves:
when IS-IS converges a new path, the new path requires an MPLS label that
LDP has not yet distributed. Traffic hits a label-less path and is dropped.
LDP-IGP sync makes IS-IS announce a **maximum metric** on a circuit until
LDP signals that labels are in place, which keeps traffic on the old
converged path until the dataplane is ready.

State is in `struct ldp_sync_info_cmd ldp_sync_cmd` at `isisd.h:232` and
`struct ldp_sync_info *ldp_sync_info` per circuit at `isis_circuit.h:148`.
Signalling comes from the `ldpd` daemon via zapi messages
(`isis_zebra_read_ldp_sync`).

### For ze

Graceful restart is worth having eventually but is not a launch-blocker.
Operators who want zero-loss restarts usually use BFD plus LFA plus
hitless software upgrade paths, all of which are cheaper to implement than
GR.

LDP sync is only relevant if ze supports LDP. Skip it until LDP is real.

---

## 16. Authentication, properly

bio-rd parses the Authentication TLV (10) but does not verify signatures.
FRR does. Three mechanisms:

1. **Cleartext password** (TLV 10, type 1). Used only as a sanity check.
   Stored in `struct isis_passwd area_passwd, domain_passwd` at
   `isisd.h:163-164`. Circuit-level password at `isis_circuit.h:121`.
2. **HMAC-MD5** (TLV 10 with hash type byte, per RFC 5304). Still the
   most deployed; known weak but good enough for most threat models.
3. **Generic Cryptographic Authentication** (RFC 5310, TLV 10 type 54 with
   extended key-identifier). Supports HMAC-SHA-1 through HMAC-SHA-256.

### Keychains

FRR delegates key management to the libfrr keychain subsystem
(`lib/keychain.c`). A keychain is a named list of keys, each with a key ID,
a secret, and optional accept/send lifetimes. IS-IS references a keychain
by name per level per area or per circuit. At verification time, FRR
iterates the keychain, tries every active key, and returns success if any
key validates the PDU.

This pattern handles key rotation gracefully: you add the new key with a
future send-lifetime, wait for all neighbours to load the new keychain,
then flip the send-lifetime, then remove the old key after the accept
window expires.

### Per-level separation

IS-IS distinguishes **area password** (for L1 traffic), **domain password**
(for L2 traffic), and **circuit password** (for IIHs on a specific
interface). FRR stores all three and uses the correct one depending on the
PDU type and level. Bio-rd's guide handwaves this as "per-area, per-level,
per-interface"; FRR's storage is a concrete example of what that means.

### For ze

- Integrate with whatever keychain mechanism ze has for BGP (MD5/AO/TCP-AO).
  IS-IS authentication is the same shape: per-level configurable keys with
  rotation windows.
- Implement HMAC-MD5 first (for interop with existing infrastructure) and
  HMAC-SHA-256 second (for new deployments).
- Counter failures separately per level: `auth_type_failures[level]` and
  `auth_failures[level]` (`isisd.h:257-258`). Operators debugging
  auth problems cannot tell L1 from L2 otherwise.
- Enforce the "TLV 10 must be first" rule of RFC 5304 when encoding, even
  if the receiver tolerates it out of order. It is a trap that breaks
  interop with strict implementations.

---

## 17. Northbound and YANG

FRR's "northbound" is a unified config management layer (`lib/northbound.h`)
that sits above all FRR daemons and exposes CLI, NETCONF, gRPC, and a
direct C API over a single YANG-driven dispatch table. IS-IS hooks into it
through four files:

| File | LOC | Purpose |
|------|----:|---------|
| `isis_nb.h` | 843 | Declarations of every YANG path and callback signature |
| `isis_nb.c` | 1463 | The callback table mapping YANG paths to C functions |
| `isis_nb_config.c` | 4897 | Config change callbacks (one per writable leaf) |
| `isis_nb_state.c` | 632 | Operational state queries |
| `isis_nb_notifications.c` | 586 | YANG notification emission |

**Total: ~8400 lines** of northbound code, roughly 14% of the whole daemon.

### How a config change flows

1. User input arrives (CLI command, NETCONF XML, gRPC protobuf, JSON from
   a web UI). All normalize to a YANG path and a new value.
2. Northbound validates against the YANG schema (types, constraints,
   must-statements, uniqueness).
3. Northbound looks up the callback for that path in the IS-IS
   `frr_yang_module_info` table (declared in `isis_nb.h`, populated in
   `isis_nb.c`).
4. The callback runs in the phase the transaction requires: `validate` to
   check semantic correctness, `prepare` to allocate but not apply,
   `abort` to roll back, `apply` to commit the change.
5. Callback modifies the in-memory data structures (calls
   `isis_area_lsp_mtu_set`, for example) and schedules side effects
   (LSP regeneration, SPF, adjacency reset).

The `isis_instance_create` callback at `isis_nb_config.c` is a good
starting point to see the pattern. It takes the YANG node, extracts the
area tag, calls `isis_area_create()`, and returns `NB_OK`. Every writable
YANG leaf has a similar small callback.

### Example: area address

YANG path: `/frr-isisd:isis/instance/area-address`

Callbacks in `isis_nb.c`:
- `.create` -> `isis_instance_area_address_create` in `isis_nb_config.c`
- `.destroy` -> `isis_instance_area_address_destroy`
- `.cli_show` -> `cli_show_isis_area_address` (formats for CLI output)

`_create` parses the textual ISO address, calls into the area object to
add it, and triggers LSP regeneration via `lsp_regenerate_schedule`. The
whole thing is 15 lines. Every configurable item is a tiny callback like
this.

### Why this matters for ze

ze already uses YANG for configuration and already has a northbound-ish
architecture. The question is whether IS-IS should fit that pattern the
same way BGP does. The answer from FRR's experience:

- **Yes, use YANG for everything.** Operators expect it, NETCONF clients
  require it, and the per-leaf callback pattern is easy to test.
- **Yes, separate config callbacks from state callbacks from notification
  callbacks.** FRR's three-file split (`_config`, `_state`,
  `_notifications`) maps exactly to YANG's three access modes. ze should
  mirror it.
- **Yes, centralize the callback table.** One file (like FRR's
  `isis_nb.c`) that lists every YANG path and the functions that handle
  it, as opposed to spreading "YANG path to function" mapping across the
  codebase. This is the single biggest operator-friendliness win.
- **Carefully design the transaction model.** FRR's four-phase
  validate/prepare/abort/apply pattern handles rollback without leaving
  partial state. It is overkill for most changes but indispensable for
  "change my system ID and half my area addresses at the same time".

A minimal ze YANG skeleton is already sketched in the bio-rd guide
section 9. Use that as the starting point and add the per-leaf callback
dispatch shape from FRR.

---

## 18. Zebra integration (the FIB boundary)

`isis_zebra.c` is 1627 lines and is the biggest single piece of glue in
the daemon. It does:

1. **Open a zapi client** (`zclient_new`, `zclient_init`) and connect to
   `/var/run/frr/zserv.api`
2. **Subscribe to interface events** (add/remove, address add/remove,
   state changes) and push them into the circuit state machine via
   `isis_csm_state_change`
3. **Subscribe to router-ID updates** for the LSP origination trigger
4. **Install routes** via
   `isis_zebra_route_add_ipv4/v6(route, family, nexthop, metric, labels)`,
   which packages the route into a `zapi_route` and sends it to zebra
5. **Handle BFD session replay** on daemon restart (zebra buffers state
   while isisd is down and replays on reconnect)

### The contract

IS-IS never touches the kernel directly. Every FIB change goes as a zapi
message to zebra, which does the kernel install. This is how every FRR
daemon works and it is why zebra is the single point of truth for "what
routes are actually in the kernel".

Default administrative distance is 115 for IS-IS (vs 20 for BGP-eBGP, 200
for BGP-iBGP, 110 for OSPF). This is set in `isis_zebra.c` when
initializing the zapi.

### For ze

ze has its own FIB architecture (`sysrib` plugin). The IS-IS-to-FIB
contract should mirror this: one outgoing interface (Add/Delete route,
with prefix + metric + nexthops + optional labels) and one incoming
interface (Interface events + address events + router-ID). Match the
abstraction at this boundary and you get the same clean separation FRR
has.

---

## 19. Redistribution

`isis_redist.c` (893 lines) handles importing routes from other protocols
(BGP, OSPF, static, connected, kernel) into IS-IS LSPs.

The flow:

1. Operator configures `redistribute <source> level-<N> [metric M]
   [route-map R]` via YANG.
2. `isis_redist.c` subscribes to zebra for the matching protocol via
   `zclient_redistribute(ZEBRA_REDISTRIBUTE_ADD, ...)`.
3. zebra streams matching routes to isisd.
4. Each route runs through an optional prefix-list and route-map (FRR's
   filtering language, implemented in `lib/routemap.c`).
5. Accepted routes land in `area->ext_reach[protocol][level]` (a
   `route_table` per source, per level, `isisd.h:241`).
6. `lsp_regenerate_schedule()` is called; next LSP build walks
   `ext_reach` and emits TLV 130 (external) or TLV 135 with the external
   bit set.
7. When the source route is withdrawn, the entry is removed from
   `ext_reach` and the LSP is regenerated without it.

This is symmetric on the egress side: the bio-rd guide section 5 mentions
"routes computed by SPF are installed in the FIB" and here is the other
direction, "routes from the FIB are fed into IS-IS".

### For ze

Redistribution is where the operator story gets complex. You want:
- Per-source configuration (`redistribute connected`, `redistribute bgp`, etc.)
- Per-level routing (`level-1`, `level-2`, `level-1-2`)
- Per-prefix filtering (prefix list or route map)
- Optional metric override per source
- Optional tag propagation for loop detection (`isis_redist.c` uses
  administrative tags per RFC 5130)

FRR's model is a good template. The shared `ext_reach` table per
(protocol, level) is the key data structure; walk it when building LSPs.

---

## 20. Constants and default timers

Verified against `isis_constants.h`:

| Constant | Value | RFC | Purpose |
|----------|------:|-----|---------|
| `MAX_AGE` | 1200 s | 10589 §7.3.21 | Max LSP age in database |
| `ZERO_AGE_LIFETIME` | 60 s | 10589 | Hold zero-lifetime LSP before delete |
| `MIN_LSP_LIFETIME` | 350 s | 10589 | Minimum allowed LSP rem_lifetime |
| `MAX_LSP_LIFETIME` | 65535 s | 10589 | Maximum |
| `DEFAULT_LSP_LIFETIME` | 1200 s | 10589 | Default rem_lifetime on origination |
| `DEFAULT_MAX_LSP_GEN_INTERVAL` | 900 s | 4444 | LSP refresh interval |
| `DEFAULT_MIN_LSP_GEN_INTERVAL` | 30 s | 4444 | Minimum between regenerations (throttle) |
| `MIN_LSP_RETRANS_INTERVAL` | 5 s | 10589 | TX queue retry, section 9 |
| `DEFAULT_CSNP_INTERVAL` | 10 s | 10589 | CSNP period |
| `DEFAULT_PSNP_INTERVAL` | 2 s | 10589 | PSNP period |
| `DEFAULT_HELLO_INTERVAL` | 3 s | 10589 | **IIH period (note: bio-rd uses 10)** |
| `DEFAULT_HELLO_MULTIPLIER` | 10 | | Holding time = 10 * 3 = 30 s |
| `DEFAULT_PRIORITY` | 64 | 10589 | DIS election priority |
| `DEFAULT_CIRCUIT_METRIC` | 10 | 10589 | Default per-link cost |
| `MAX_NARROW_LINK_METRIC` | 63 | 10589 | 6-bit old-style metric |
| `MAX_WIDE_LINK_METRIC` | 0x00FFFFFF | 4444 | 24-bit new-style metric |
| `MAX_WIDE_PATH_METRIC` | 0xFE000000 | 3787 | Total path metric ceiling |
| `SEQUENCE_MODULUS` | 2^32 | 10589 | Sequence number wrap boundary |

### Two notable differences from the bio-rd guide

The bio-rd guide recommends:
- Hello interval 10 s, multiplier 3 -> holding time 30 s.

FRR's defaults:
- Hello interval 3 s, multiplier 10 -> holding time 30 s.

Both give the same 30 s detection window but with different interpretations:
bio-rd sends less often and tolerates fewer losses; FRR sends more often
and tolerates more losses. The FRR defaults are more forgiving in lossy
environments and are what most operators expect. Adopt FRR's defaults in
ze unless there is a specific reason to diverge.

LSP refresh interval:
- Bio-rd guide: 900 seconds (15 minutes), implied from "typically every 15-20 minutes"
- FRR: 900 seconds (`DEFAULT_MAX_LSP_GEN_INTERVAL`)

These agree; the bio-rd guide is correct.

### Jitter

FRR applies jitter to every periodic timer to avoid synchronization of
network-wide events (`isis_constants.h:30-38`):

- `IIH_JITTER = 10%` on hello timers
- `MAX_AGE_JITTER = 5%` on LSP lifetime decrement
- `MAX_LSP_GEN_JITTER = 5%` on LSP refresh
- `CSNP_JITTER = 10%` on CSNP period
- `PSNP_JITTER = 10%` on PSNP period

The bio-rd guide does not mention jitter. Skipping jitter is subtle but
dangerous: in a large flat LAN, N routers that booted at the same moment
will all try to send their LSPs at exactly the same tick, and the burst
can exceed the link's burst-tolerance or the CPU's single-second LSP
processing rate. Adopt the FRR jitter percentages verbatim.

---

## 21. Patterns worth stealing

The bio-rd guide's `code-review.md` identifies 10 patterns from bio-rd's
BGP implementation. Here are the equivalents from FRR's IS-IS worth
adopting:

### 1. Self-tracing origination triggers (`__func__/__FILE__/__LINE__`)

The `lsp_regenerate_schedule` macro at `isis_lsp.h:59-61` and the
`_isis_tx_queue_add` macro wrap every call site with its own source
location. This turns every debug log into an audit trail: "who scheduled
this regeneration, from which line".

Go equivalent using `runtime.Caller`:

```go
func (a *Area) ScheduleRegenerate(level int, reason string) {
    _, file, line, _ := runtime.Caller(1)
    a.scheduleRegenerate(level, reason, file, line)
}
```

Use it for every "defer the real work" entry point. Cheap. Life-changing
in a debugging session.

### 2. TX queue with self-rescheduling retry (section 9 of this document)

Already covered in full. The key insight is that the retry timer is armed
**before** the current send runs. This is not the "naive" order; it is the
order that makes recovery correct under crash.

### 3. Per-level counter fields for every failure path

`isisd.h:256-260` declares five 64-bit counter arrays on the area struct,
indexed by level (L1 and L2): rejected adjacencies, authentication type
failures, authentication failures, ID-length mismatches, and LSP error
counts. Every error has its own level-indexed counter. When an operator
asks "why is L1 not forming adjacencies", you can answer from counters
alone without enabling debug logging. ze should do the same across every
BGP and IS-IS failure path that is not already counted.

### 4. Reason codes alongside state changes

`enum isis_adj_updown_reason` at `isis_adjacency.h:45-51`. Every state
change comes with exactly one reason. This is the same pattern bio-rd's
BGP FSM uses with string reasons ("run() (state, string)"). ze should
adopt it uniformly for BGP FSMs and IS-IS adjacency FSMs.

### 5. Four-state circuit FSM separated from adjacency FSM

Section 5 of this document. The bio-rd guide merges the two; FRR keeps
them separate and the separation is the cleaner model. Use it in ze.

### 6. Red-black tree for LSPDB with range queries

The CSNP path wants sorted iteration over a key range. A sorted map or an
RB-tree is the right primitive. Do not use a hash map and then re-sort on
every CSNP; it looks fine in benchmarks and fails under churn.

### 7. Jitter on every periodic timer

Section 20 of this document. Mechanical and cheap; apply everywhere.

### 8. Per-leaf YANG callback table

Section 17 of this document. One file that lists every YANG path and the
functions that handle create/destroy/modify/state/notification for it.
ze already does some of this; FRR is the gold standard for how
comprehensive it should be.

### 9. Keychain-based authentication with multiple active keys

Section 16 of this document. Not reinventing this is the only way to
support hitless key rotation.

### 10. Hook architecture for TLV composition

`lsp_build()` calls registered hooks from SR, TE, LDP-sync, each of which
attaches its own sub-TLVs to the LSP under construction. This decouples
the LSP builder from optional features. For ze, the plugin SDK already
has a "give me a chance to inject data into outbound messages" concept
for BGP; extend it to IS-IS LSP origination the same way.

---

## 22. Patterns NOT to copy

Fairness check. FRR has things that are not best practices:

### 1. Single huge `isis_area` struct

`struct isis_area` has ~80 fields of mixed concerns (LSPDB, SPF, LFA,
timers, counters, SR, TE, LDP sync, BFD, flags). It is a "god struct" and
the corresponding lifecycle (create/destroy) is pages long. Bio-rd avoids
this by splitting state into smaller structs held by reference. ze should
do the same: `type Area struct { LSPDB *LSPDB; SPF *SPFState; Timers
*TimerSet; ... }`.

### 2. Macros that hide parameters

`lsp_regenerate_schedule(area, level, all_pseudo)` hides the
`__func__/__FILE__/__LINE__` parameters in a macro. In Go the equivalent
is a wrapper function that calls `runtime.Caller`. Do not try to replicate
the C macro trick with Go's `//go:linkname`; the wrapper function is
cleaner.

### 3. Global mutable state for debug flags

`debug_adj_pkt`, `debug_snp_pkt`, etc. are package-level `unsigned long`
variables at `isisd.h:358-373` that control debug output. This is C
convention. In Go prefer a `type DebugFlags struct{}` on the instance,
accessed via a pointer, even if it means passing it through more
functions.

### 4. Legacy SSN/SRM bitfields alongside the TX queue

`SSNflags[ISIS_MAX_CIRCUITS]` at `isis_lsp.h:33` is no longer the primary
flooding driver but still exists in the struct and is still read in some
paths. This is technical debt from the 2018 migration. Do not carry the
old model into ze; start with the TX queue and have nothing to remove.

### 5. `FABRICD` preprocessor branches through the whole codebase

`#ifdef FABRICD` appears in dozens of files. OpenFabric is a separate
protocol and it shares 90% of the IS-IS daemon at compile time. For ze,
if you ever want OpenFabric, use runtime plug-ins rather than preprocessor
surgery.

### 6. Per-protocol SNMP MIB in the same daemon

`isis_snmp.c` is 3467 lines implementing the RFC 4444 IS-IS MIB. SNMP
should not be in the daemon at all; it should be a separate agent that
queries the daemon over gRPC or JSON-RPC. For ze, never put SNMP in the
protocol code.

---

## 23. Cross-reference: FRR fills which bio-rd guide gap

The bio-rd guide has 15 sections. Here is where this document addresses
each:

| Bio-rd guide section | Gap | FRR-reference section |
|---|---|---|
| §1 Protocol Overview | complete | not repeated |
| §2 Wire Format and PDUs | complete | not repeated |
| §3 Domain Types | complete | not repeated |
| §4a Adjacency FSM | 3 states; missing UNKNOWN, missing 3-way state | §2, §4 |
| §4b DIS Election | "not implemented" in bio-rd | not covered here; see RFC 10589 §8.4.5 and `isis_dr.c:309` in FRR |
| §4c LSP Flooding | SRM/SSN flags only; no retry mechanism | §8 |
| §5 LSPDB and SPF | LSPDB partial, **SPF completely absent** | §7, §10 |
| §6 Circuit Types | correct | small correction in §5 (circuit FSM) |
| §7 Authentication | parsing only | §16 |
| §8 Concurrency | goroutine model | §3 (contrasting event-loop model) |
| §9 Configuration Shape | YANG sketch | §17 (real YANG dispatch) |
| §10 Plugin Model for ze | good | not repeated |
| §11 Testing Strategy | good but generic | should add topotest references (see below) |
| §12 Known Hard Problems | comprehensive | §7 (comparison rules), §9 (wraparound verification) |
| §13 What bio-rd Does NOT Implement | accurate | every item is covered here as "FRR does it like this" |
| §14 Implementation Order | good | complement in §24 below |
| §15 Code Organization for ze | good | see §24 |

### Things the bio-rd guide missed entirely

Items the bio-rd guide does not mention at all but that FRR has and that
matter for a production implementation:

1. **TX queue model for flooding** (section 9 here)
2. **Self-tracing origination triggers** (section 10 here)
3. **Jitter on all periodic timers** (section 20 here)
4. **Per-level, per-family, per-topology SPF trees** (section 10 here)
5. **SPF backoff (RFC 8405)** (section 10 here)
6. **Reason codes on every adjacency state change** (sections 4, 15 here)
7. **Separate circuit FSM from adjacency FSM** (section 5 here)
8. **Platform abstraction for L2 raw frames** (section 6 here)
9. **Per-circuit counters** (section 21.3 here)
10. **LDP-IGP sync** (section 15 here)

### Things to check against FRR specifically

If you have a disagreement between the bio-rd guide and your
implementation, these are the places to go look at FRR for authority:

| Question | Authoritative reference |
|---|---|
| How many states does the adjacency FSM have? | `isis_adjacency.h:34-39` (four: UNKNOWN, INITIALIZING, UP, DOWN) |
| How is three-way handshake represented? | `isis_adjacency.h:88` (separate `threeway_state` enum) |
| What is the default hello interval? | `isis_constants.h:77` (3 seconds, not 10) |
| What is the LSP retransmission interval? | `isis_constants.h:63` (5 seconds) |
| Does "first" in RFC 5304 mean byte-first or TLV-first? | RFC says byte-first; FRR enforces it in `isis_tlvs.c` encode path |
| How do you distinguish "legitimate purge" from "local expiry"? | `lsp_compare()` returns LSP_NEWER for zero-lifetime; `lsp_tick()` handles local expiry differently (age-out counter) |
| What sorting primitives does TENT use? | `isis_spf_private.h:149-152` (skiplist), comparison at :121-146 |

---

## 24. Recommended reading order and first-cut phases

The bio-rd guide section 14 proposes 10 phases. This is a revised version
drawing on FRR where bio-rd's phases are insufficient.

### Phase 0: Decide threading model

Not code, decision. One week of design work, including:
- Goroutine model, event-loop model, or hybrid
- Circuit owner: reactor or dedicated goroutine per circuit
- LSPDB access: mutex, RWMutex, or actor goroutine
- SPF scheduling: callback or channel

Write it down. This will be referenced by every subsequent phase.

### Phase 1: Domain types (unchanged from bio-rd guide)

SystemID, SourceID, LSPID, NET, AreaID. Unit tests. Round-trip parsing.
Nothing networky. Maybe half a week.

### Phase 2: Platform abstraction for L2 frames

Not in the bio-rd guide. Comes first because you cannot send or receive
without it. Build a `Circuit` interface with `Read/Write/Join/Leave`
methods and implement it on Linux AF_PACKET. Test with a pair of
TUN/TAP interfaces in a test harness.

Reference: FRR `isis_pfpacket.c`, sections 5 and 6 here. RFC 10589 for
the LLC/SNAP framing.

### Phase 3: PDU codec

Headers plus every TLV the first cut needs. Bio-rd's guide phase 2 is
correct but add the following to the list:

- TLV 129 Protocols Supported
- TLV 132 IP Interface Addresses
- TLV 229 Multi-Topology Router Info (decode only; advertise MT-0
  implicit)
- TLV 232 IPv6 Interface Addresses
- TLV 236 IPv6 Reachability
- TLV 240 Three-Way Adjacency (for P2P hellos)
- TLV 242 Router Capability (decode as opaque; do not emit)

Test vectors from Wireshark captures against a running FRR. The bio-rd
guide recommends generating from captures; FRR ships a `tests/`
directory with known-good packets you can crib.

### Phase 4: Circuit FSM and adjacency FSM (two machines, not one)

Bio-rd guide phase 3 merges these. Split them. See section 5 here.

Test gate: an in-memory circuit pair forms adjacency within 2 hellos.
Bring the circuit down with `IF_DOWN_FROM_Z` and observe the FSM return
to `C_STATE_CONF` without tearing down the area.

### Phase 5: LSPDB + LSP origination + aging

Identical to the bio-rd guide phase 4, with one addition: use an RB-tree
or sorted map, not a hash. Add the `lsp_tick` per-second aging walk and
the purge-with-age-out mechanism from section 7 here.

### Phase 6: LSP flooding via TX queue

Not in the bio-rd guide. This replaces the SRM/SSN flag model. See
section 9 here. Write a unit test where circuit B's "send" method
returns an error on the first call and succeeds on the second; verify
the retry fires.

### Phase 7: CSNP/PSNP synchronization

Same as bio-rd guide phase 5. One new requirement: CSNP must walk the
LSPDB in LSPID order (which is why you needed the RB-tree in phase 5).

### Phase 8: SPF

Same as bio-rd guide phase 6, except now you have an actual reference
implementation to steal from. Start with a single tree (L2, IPv4) and
hardcode to one topology. Use a skiplist or sorted `container/list` for
TENT. Write unit tests against manual topologies before running against
a live network.

Add SPF backoff (RFC 8405) from day one. Without it your implementation
will saturate the CPU on churn.

### Phase 9: Route installation via ze sysrib

Same as bio-rd guide phase 6, but split out because it is a distinct
interface. Install routes into ze's equivalent of zebra. Delete routes
on SPF re-run when they disappear. Handle ECMP (multiple parents on
the same vertex).

### Phase 10: DIS election and pseudo-node LSPs

Bio-rd guide phase 7. Required for broadcast circuits; can be skipped on
P2P-only deployments. Reference: FRR `isis_dr.c` (309 lines, small and
approachable).

### Phase 11: IPv6 reachability (second SPF tree)

Not in bio-rd guide as a separate phase. Add `SPFTREE_IPV6` alongside
`SPFTREE_IPV4`. This is the point where the original assumption "one
SPF tree" breaks. Refactor before adding more.

### Phase 12: Authentication

Bio-rd guide phase 8. Integrate with ze's keychain mechanism (share it
with BGP TCP-MD5/AO if possible). Start with HMAC-MD5, add HMAC-SHA-256.
Write a test that validates against a Wireshark-captured MD5-authed
PDU from FRR.

### Phase 13: Configuration via YANG

Bio-rd guide phase 9. Use FRR's per-leaf callback table pattern from
section 17 here. Do not inline config handling into the protocol code.

### Phase 14: Redistribution

Not in bio-rd guide. Add after basic functionality. One
`ext_reach[source][level]` table per redistribute source, walk it during
LSP origination, subscribe to ze's FIB event stream.

### Phase 15: Interop testing

Bio-rd guide phase 10. The FRR topotest suite
(`tests/topotests/isis_*`) is the reference environment. Run your
implementation against an FRR container, verify adjacency, verify LSPs
flow both ways, verify routes install on both sides, verify convergence
time on link flap. This is a week of work, not an afternoon.

### Deferred, explicitly

Do not attempt on the first pass, even if you see the bio-rd guide
mention them:

- Multi-topology (ship single-topology IPv4+IPv6 first)
- Segment Routing (needs a label source)
- LFA/TI-LFA/RLFA (needs SR)
- Graceful Restart (cosmetic until BFD is in place)
- BFD integration (ship a separate BFD daemon first)
- L1/L2 route leaking (ship L2-only first)
- Administrative tags (RFC 5130) beyond parse-and-propagate
- SRv6 (immature in FRR; do not imitate an immature implementation)
- Flex-Algo (even rarer than MT)

Each of the above can be added incrementally once the core is stable.
None of them is on the path to a working L2-only IS-IS with SPF.

---

## Closing note

The bio-rd guide and this document should be read together. The bio-rd
guide teaches the protocol from the perspective of a small, clean library
that chose to do less. This document teaches the protocol from the
perspective of a production daemon that chose to do more, including the
things bio-rd punted on.

ze's first IS-IS should be closer in spirit to bio-rd (small, clean,
focused) than to FRR (enormous, production-grade, battle-scarred). But on
the specific points where bio-rd is incomplete (SPF, LFA is out of scope
but SPF is not, flooding, authentication, configuration, route install),
FRR is the authoritative reference.

When in doubt between the two:

- For "how does the protocol work", trust the RFC first, then both guides.
- For "what shape should the implementation take", trust bio-rd's guide.
- For "what does correct look like in practice", trust FRR's code.
- For "what can I skip on the first pass", trust the deferred-list above.

And always: `make ze-verify` before closing the spec.
