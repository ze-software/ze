# Why Ze / Why Not Ze

An honest assessment of when Ze is the right tool and when it is not.

> Ze is pre-release software. This page reflects the current state of development
> and will be updated as features mature. Last updated: 2026-03-21. Corrections
> welcome via the [issue tracker](https://codeberg.org/thomas-mangin/ze/issues).

## When to use Ze

### Programmable route injection

Ze's primary strength is letting external processes interact with BGP. If your use case
is "receive BGP events in my Python/Go/Rust program and inject routes back," Ze was
built for this. JSON events out, text commands in, any language that reads lines.

ExaBGP pioneered this model and is deployed worldwide for DDoS mitigation, traffic
engineering, and SDN integration. Ze is its successor: same programmable model, but
multithreaded, with broader protocol coverage, and a plugin SDK for deeper integration.

| What you get | How |
|---|---|
| BGP events as structured JSON | External process on stdin/stdout, or plugin SDK |
| Route injection from scripts | Text command protocol or plugin API |
| Any programming language | Anything that reads lines and writes lines |
| Atomic route updates | `commit start` / `commit end` workflow |
| Raw wire access | Hex-mode encoding and decoding, `bgp cache forward` |

<!-- source: pkg/plugin/sdk/sdk.go -- plugin SDK entry point -->
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn for concurrent RPCs -->

ExaBGP pioneered this space. GoBGP offers gRPC but limits you to the operations its
API exposes. BIRD has no programmatic API at all. Ze gives raw wire access alongside
structured events, in a compiled multithreaded daemon.

### BGP monitoring and analysis

Ze decodes 21 address families across 15 path attribute types. The `ze bgp decode`
command turns hex wire bytes into structured JSON. The event subscription system
streams every BGP state transition and route change in real time. The Adj-RIB-In
plugin can replay raw hex for forensic analysis.

If you need to observe BGP sessions, parse UPDATE messages, or build monitoring
tools, Ze provides the parsing and event infrastructure so you don't have to write
your own BGP parser.

### Route server with custom policy

Ze separates the BGP engine from policy. The engine handles FSM, wire parsing, and
message forwarding. Plugins handle everything else: RIB storage, best-path selection,
route reflection, graceful restart. This means policy decisions are not constrained
by a built-in filter language. You can write policy in Go (as a plugin), Python (as
an external process), or anything else.

For IX operators who need policy logic that goes beyond prefix lists and AS-path
regex, such as integration with member databases, real-time RPKI feeds, or custom
business rules, Ze's plugin model is more flexible than any built-in policy language.

### ExaBGP migration

`ze config migrate` converts ExaBGP configuration files. `ze exabgp plugin` runs
existing ExaBGP processes with Ze as the BGP engine. If you have ExaBGP deployments
and want multithreading, broader address family support, or the plugin ecosystem,
Ze provides a migration path that does not require rewriting your scripts.
<!-- source: cmd/ze/config/cmd_migrate.go -- ze config migrate -->
<!-- source: cmd/ze/exabgp/main.go -- ze exabgp plugin -->

### Wire-level protocol tooling

Ze is useful even without running a daemon. `ze bgp decode` and `ze bgp encode`
are standalone tools for converting between human-readable route descriptions and
BGP wire bytes. `ze config validate` checks configuration files offline. `ze schema methods`
introspects the YANG-modeled RPC surface. These tools are valuable for protocol
debugging, test generation, and education.

## When not to use Ze

### You need a router

Ze does not install routes into the kernel forwarding table. It speaks BGP but does
not route packets. If you need a BGP daemon that populates the FIB, use FRR, BIRD,
or OpenBGPd.

Ze is a protocol speaker and route injector, not a router. This is a deliberate
design choice inherited from ExaBGP: the daemon's job is to talk BGP and expose
events, not to manage the data plane.

### You need a full routing suite

Ze does BGP. It does not do OSPF, IS-IS, LDP, PIM, BFD, or MPLS control plane. If
you need multi-protocol routing, FRR is the only open-source option with full coverage.

### You need proven production stability

Ze has not been released yet. It has extensive testing (8,000+ unit tests, 550+
functional tests, fuzz testing, chaos testing, interop tests against FRR, BIRD,
and GoBGP), but it has no production deployments. BIRD has been running IXP route
servers since 1998. FRR runs in commercial products. OpenBGPd operates at LINX and
Netnod. Production confidence comes from production use, and Ze does not have that yet.

### You need features Ze does not have

| Missing feature | Impact | Alternative |
|---|---|---|
| BMP (RFC 7854) | No route collection export to BMP collectors | rustbgpd, BIRD 3, FRR, GoBGP |
| MRT dump (RFC 6396) | No route data archival in standard format | rustbgpd, BIRD 3, FRR, GoBGP |
| BFD integration | No sub-second failure detection; relies on hold timer (90s default) | BIRD 3, FRR |
| Dynamic neighbors | Every peer must be explicitly configured | BIRD 3, FRR, GoBGP |
| Confederation (RFC 5065) | Cannot deploy in large ISP confederations | BIRD 3, FRR, GoBGP |
| Private AS removal | Route servers cannot strip private ASNs | rustbgpd, BIRD 3, FRR, GoBGP, OpenBGPd |
| ASPA verification | No path validation beyond RPKI origin | rustbgpd, BIRD 3, OpenBGPd |
| gRPC API | No industry-standard programmatic interface | rustbgpd, GoBGP, FRR (partial) |
| Embeddable library | Cannot import Ze as a Go library into your application | GoBGP |
| FIB/kernel integration | Cannot install routes into the forwarding table | BIRD 3, FRR, GoBGP, OpenBGPd |
| AIGP | No accumulated IGP metric support | FRR, GoBGP |
| TCP-AO (RFC 5925) | No modern session authentication | Nobody has this yet |

Some of these are planned. Check the [feature comparison](comparison.md) for the
current state.

### You need gRPC

The industry converged on gRPC and protobuf for network automation. gNMI, gNOI, and
OpenConfig all use gRPC. Ze uses a custom text command protocol and JSON events over
Unix sockets. Every automation tool that talks to Ze needs a Ze-specific client.

If your infrastructure is built around gRPC, GoBGP or rustbgpd integrate with less
friction.

### You need vendor adoption or a permissive license

Ze is AGPL-3.0. This requires anyone who provides Ze as a network service to release
their modifications. Network equipment vendors (Cisco, Arista, Cumulus, SONiC) will
not ship AGPL code. FRR (GPL-2.0), GoBGP (Apache-2.0), BIRD (GPL-2.0+), and OpenBGPd
(ISC) are all more permissive.

If commercial adoption or vendor integration matters to your use case, AGPL is a
blocker.

### You need a large contributor community

Ze is primarily developed by one person (architect) with AI-assisted implementation.
ExaBGP had the same single-maintainer dynamic. For infrastructure software that you
depend on, bus factor matters. FRR has dozens of active contributors across multiple
organizations. BIRD is maintained by CZ.NIC. GoBGP has the OSRG team. OpenBGPd has
the OpenBSD community.

## Honest trade-offs

### The plugin architecture

Ze's most distinctive design choice is also its most controversial: the RIB, best-path
selection, graceful restart, and route reflection are all plugins, not part of the
engine.

**The upside:** Any of these can be replaced, extended, or written in another language.
The engine remains a minimal BGP speaker. New functionality can be added without
modifying the engine. External plugins run as separate processes, so a crash does not
bring down the engine.

**The downside:** Correctness is distributed across process boundaries. A GR bug could
be in the plugin, the IPC encoding, the event dispatch, or the cache forwarding. Every
other BGP daemon keeps the RIB in-process because centralized state is easier to reason
about.

**The mitigation:** All shipped plugins run in-process via DirectBridge, which bypasses
IPC serialization entirely. The plugin boundary is a logical separation with near-zero
runtime cost for internal plugins. The IPC overhead only applies to external plugins,
which are out-of-process by design for isolation.

### Performance vs. C and Rust

Ze is written in Go. Compared to C (BIRD, FRR) and Rust (rustbgpd) implementations:

| Factor | Mitigation |
|---|---|
| Garbage collection | Pool-based dedup, sync.Pool for hot-path structs, and stack-allocated caches keep most data off the GC-scanned heap |
| Goroutine scheduling | Long-lived workers on channels, no per-event goroutines |
| Bounds checking | Unavoidable in Go, but modern CPUs branch-predict these away |
| Interface dispatch | Concrete types in hot paths where possible |

<!-- source: internal/component/bgp/attrpool/pool.go -- pool-based dedup -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- long-lived forward workers -->

**Estimated composite overhead:** Ze is expected to leave roughly 10-15% on the table
compared to an optimal C/Rust monolith when using internal plugins (DirectBridge). Most
of this is the inherent cost of Go's runtime. The plugin architecture adds a smaller
amount on top (channel hops, batch struct copies).

For external plugins, the JSON serialization path adds significant overhead (estimated
40-50%). This is the price of language-agnostic programmability and is comparable to
ExaBGP's approach.

For context: BGP CPU performance is rarely the bottleneck in operations. For most
deployments, the control plane is idle once converged. The scenarios where this 10-15%
matters are large IXP route servers with 1000+ peers during full reconvergence, and
there BIRD 3 and rustbgpd have the edge.

**Caveat:** these overhead percentages are estimates based on code path analysis, not
measured benchmarks. Ze has not been benchmarked at DFZ scale (1M+ prefixes). The chaos
testing framework validates correctness under fault injection with small route counts,
not performance at scale.

### Zero-copy claims

Ze's `WireUpdate` holds a byte slice reference to the TCP read buffer. Lazy parsing
avoids decoding into intermediate structs. When source and destination peers share the
same encoding context (ContextID), UPDATE messages are forwarded as cached wire bytes
with no parsing at all.
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate byte slice reference -->
<!-- source: internal/component/bgp/context/registry.go -- ContextID matching -->

This is not zero-copy in the strictest sense: Go's TCP layer copies bytes from the
kernel into a Go slice. True zero-copy (kernel buffer to userspace without copying)
requires `io_uring` or `mmap`, which Go does not expose. Ze's "zero-copy" means zero
additional copies within the application, after the initial TCP read. Rust with
`bytes::Bytes` can achieve reference-counted sharing from closer to the kernel buffer.

The practical impact is small. The TCP read copy is a single `memcpy` per message,
dwarfed by the cost of parsing, policy evaluation, and re-encoding.

### Custom IPC vs. gRPC

Ze uses a custom multiplexed text protocol (`#<id> <verb> [<json>]`) for plugin
communication instead of gRPC. This was a deliberate choice:
<!-- source: pkg/plugin/rpc/framing.go -- wire framing for multiplexed protocol -->
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn multiplexer -->

- **Simplicity:** The protocol is human-readable and debuggable with `cat`.
- **No code generation:** No `.proto` files, no generated stubs, no protobuf dependency.
- **Language agnostic:** Any language that reads and writes lines can be a plugin.
- **Lightweight:** No HTTP/2 framing overhead for in-process communication.

The cost is that every new RPC requires hand-writing a handler on both sides, and
there is no auto-generated client library for external consumers. For a project with
a small contributor base, this is manageable. For a large ecosystem with many
third-party integrations, gRPC would scale better.

## Further reading

- [Feature list](features.md) for the complete feature inventory.
- [Comparison](comparison.md) for a feature-by-feature comparison with other BGP daemons.
- [Design document](DESIGN.md) for architecture and principles.
- [Quick start](guide/quickstart.md) to try Ze yourself.
