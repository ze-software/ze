# BGP implementation comparison

A feature comparison of open source BGP daemon implementations. This page keeps the BGP-specific matrix separate from the full Network OS comparison.

> **Disclaimer and evidence:** this comparison was generated with AI assistance and is provided for informational purposes only. All listed projects are under active development and their capabilities change over time. Verify current features against each project's own documentation before making decisions. Rows should be read as evidence-backed advice rather than marketing: code paths link to upstream source where the site can map them, official feature pages are preferred when source links are not practical, and `No` or `Partial` means the cited evidence did not support a stronger claim. Corrections and updates are welcome via the issue tracker.

 Find in this page Section

## Overview

| | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Language | Go | C | C | C | C | Go | Go | Python | Rust | Rust | Java |
| License | AGPL 3.0 | GPL 2.0+ | GPL 2.0+ | GPL 2.0 | ISC | Apache 2.0 | Apache 2.0 | BSD 3-Clause | Apache 2.0 | MIT | Free |
| Primary interface | CLI, SSH, REST, gRPC | CLI | CLI | CLI | CLI | gRPC | gRPC | CLI, API | gRPC | gRPC | CLI |
| First release | 2026 | 2024 | 1998 | 2017 | 2004 | 2014 | 2018 | 2010 | 2019 | 2026 | 2012 |
| Multithreaded | Yes | Yes | No | No | Yes | Yes | Yes | No | Yes | Yes | Yes |
| Multithread model | Goroutines | Cooperative threads | -- | -- | 3-process | Goroutines | Goroutines | -- | Multi-core | Tokio | Per-peer |
| Plugin architecture | Yes | No | No | No | No | No | No | No | No | No | No |
| YANG-modeled config | Yes | No | No | Partial | No | No | No | No | No | No | No |

## Address Families

| AFI/SAFI | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| IPv4 Unicast | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| IPv6 Unicast | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| IPv4 Multicast | Yes | Yes | Yes | Yes | No | Yes | No | No | No | No | Yes |
| IPv6 Multicast | Yes | Yes | Yes | Yes | No | Yes | No | No | No | No | Yes |
| IPv4 Labeled Unicast | Yes | No | No | Yes | No | Yes | No | Yes | No | No | Yes |
| IPv6 Labeled Unicast | Yes | No | No | Yes | No | Yes | No | Yes | No | No | Yes |
| VPNv4 (RFC 4364) | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | No | No | Yes |
| VPNv6 | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | No | No | Yes |
| L2VPN EVPN (RFC 7432) | Yes | Yes | Yes | Yes | No | Yes | No | Yes | No | No | Yes |
| L2VPN VPLS | Yes | No | No | No | No | Yes | No | Yes | No | No | Yes |
| IPv4 FlowSpec (RFC 8955) | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | No | Yes | Yes |
| IPv6 FlowSpec | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | No | Yes | Yes |
| VPN FlowSpec | Yes | No | No | No | No | Yes | No | No | No | No | Yes |
| BGP-LS (RFC 7752) | Decode (40 TLVs) | No | No | No | No | Yes | No | Decode | No | No | Yes |
| SR Policy | Yes | No | No | No | No | Yes | No | No | No | No | Partial |
| IPv4/IPv6 MUP | Yes | No | No | No | No | No | No | No | No | No | Yes |
| IPv4/IPv6 MVPN | Decode | No | No | No | No | No | No | No | No | No | Yes |
| IPv4 RTC (RFC 4684) | Decode | No | No | No | No | No | No | Yes | No | No | Yes |

## Core Protocol

| Feature | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| RFC 4271 FSM | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| 4-byte ASN (RFC 6793) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Capability negotiation | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Route Refresh (RFC 2918) | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | No | Yes | Yes |
| Enhanced Route Refresh (RFC 7313) | Yes | Yes | Yes | Yes | Yes | No | No | Yes | No | Yes | Yes |
| Graceful Restart (RFC 4724) | Yes | Yes | Yes | Yes | Yes | Yes | No | Partial | No | Yes | Yes |
| Long-Lived GR (RFC 9494) | Yes | Yes | Yes | Partial | No | Yes | No | No | No | Yes | Yes |
| Notification GR (RFC 8538) | Yes | No | No | No | Yes | Yes | No | No | No | Yes | No |
| Add-Path (RFC 7911) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Rx only | Yes | Yes |
| Paths-Limit (draft-abraitis) | Yes | No | No | Yes | No | No | No | Yes | No | No | No |
| Extended Messages (RFC 8654) | Yes | Yes | Yes | Yes | Yes | No | No | Yes | No | Yes | Yes |
| Extended Nexthop (RFC 8950) | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | No | Yes | Yes |
| Route Reflector (RFC 4456) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | No | Yes | Yes |
| Confederation (RFC 5065) | No | Yes | Yes | Yes | No | Yes | No | No | No | No | Yes |
| Admin Shutdown (RFC 8203) | Yes | Yes | Yes | Yes | Yes | Yes | Partial | Yes | No | Yes | Partial |
| BGP Roles (RFC 9234) | Yes | Yes | Yes | No | Yes | No | Yes | No | No | No | Partial |
| Prefix Limit (RFC 4486) | Yes | Yes | Yes | Yes | Yes | Yes | No | No | No | Yes | Yes |

## Cross-Protocol Redistribute

Ze advertises locally-originated routes from non-BGP protocols (connected, static, L2TP, IS-IS, OSPF) into BGP via the `redistribute-orchestrator` plugin. Operators enable it per-destination and per-source via `redistribute { destination <proto> { import <source> { family [...]; } } }`. The same config block also drives the intra-BGP `IngressFilter` ACL when the source is `ibgp` / `ebgp`. Per-peer NEXT_HOP substitution (`nhop self`) is automatic; explicit producer-supplied NEXT_HOP is passed through verbatim.

IS-IS meshes with BGP in **both** directions, matching the vendor IGP-BGP mutual-redistribution operators expect. IPv6 rides the same single-topology SPF tree -- matching one other implementation's single-topology IS-IS default (that implementation also offers RFC 5120 Multi-Topology, which Ze does not yet implement).

OSPFv2 meshes with BGP in both directions like IS-IS, exports OSPF routes into BGP, and injects connected/static/BGP routes as Type 5 AS-External LSAs. Ze also implements stub, totally-stubby, and NSSA areas (RFC 3101) with Type 7 origination, translator election, and Type 7 to Type 5 translation. Per-interface authentication covers simple password, keyed-MD5 (RFC 2328), HMAC-SHA (RFC 5709), and the RFC 7474 extended-sequence variant, with key chains for hitless rotation and sequence-number replay protection.

## Policy & Route Manipulation

Ze takes a programmable approach to policy: external plugin filters manipulate routes via `filter { import [...] export [...] }` chains using named filter instances or explicit `<plugin>:<filter>` references. Filters chain as piped transforms (accept/reject/modify) with delta-only output. RFC-mandated checks run as default filters that can be selectively overridden. Built-in filter plugins shipped with Ze include prefix-list matching (ge/le bounds), AS-path regex filtering, community presence matching (standard/large/extended), route attribute modification (local-preference, MED, origin, next-hop, AS-path prepend), community tag/strip, and RFC 9234 role enforcement.

| Feature | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Prefix matching (ge/le) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | Partial | Yes | Yes |
| AS-path regex | Yes | Yes | Yes | Yes | Yes | Yes | No | No | No | Yes | Yes |
| Standard communities | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Extended communities | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | No | Yes | Yes |
| Large communities (RFC 8092) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | Yes |
| Community add/remove/replace | Yes | Yes | Yes | Yes | Yes | Yes | Yes | API | No | Yes | Yes |
| MED manipulation (set/inc/dec) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | API | No | Yes | Yes |
| LOCAL_PREF set/inc/dec | Yes | Yes | Yes | Yes | Yes | Yes | Yes | API | No | Yes | Yes |
| AS-path length filter | Yes | Yes | Yes | No | Yes | No | No | No | No | No | No |
| AS-path prepend | Yes | Yes | Yes | Yes | Yes | Yes | Yes | API | No | Yes | Yes |
| Next-hop set/self | Yes | Yes | Yes | Yes | Yes | Yes | Yes | API | No | Yes | Yes |
| RPKI validation match | Yes | Yes | Yes | Yes | Yes | Yes | No | No | Yes | Yes | Yes |
| Neighbor/peer matching | Yes | Yes | Yes | Yes | Yes | Yes | No | No | No | Yes | Yes |
| Named policy definitions | Plugin | Yes | Yes | Yes | Yes | Yes | Yes | No | Partial | Yes | Yes |
| Policy chaining | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | No | Yes | Yes |
| Custom filter language | No | Yes | Yes | No | Yes | No | No | No | No | No | No |
| External process policy | Yes | No | No | No | No | No | No | Yes | No | No | No |
| Plugin-based policy | Yes | No | No | No | No | No | No | No | No | No | No |

## Security

| Feature | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| TCP MD5 (RFC 2385) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | Yes |
| TCP-AO (RFC 5925) | No | No | No | No | No | No | No | No | No | No | No |
| GTSM / TTL Security | Yes | Yes | Yes | Yes | Yes | Yes | Partial | Yes | No | Yes | Yes |
| RPKI/RTR (RFC 6810/8210) | Yes | Yes | Yes | Yes | Yes | Yes | No | No | Yes | Yes | Yes |
| ASPA verification | Yes | Yes | Yes | No | Yes | No | No | No | No | Yes | No |
| Private AS removal | Yes | Yes | Yes | Yes | Yes | Yes | No | No | No | Yes | Yes |
| Privilege separation | No | No | No | No | Yes | No | No | No | No | No | No |
| TACACS+ AAA (RFC 8907) | Yes | No | No | Yes | No | No | No | No | No | No | Yes |
| Memory-safe language | Yes | No | No | No | No | Yes | Yes | Yes | Yes | Yes | Yes |

## Monitoring & Observability

| Feature | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Prometheus metrics | Yes | No | No | Yes | No | Yes | Yes | No | No | Yes | No |
| Structured logging (JSON) | Yes | No | No | No | No | No | Yes | No | No | Yes | No |
| BMP (RFC 7854) | Yes | Yes | Yes | Yes | No | Yes | Yes | No | Partial | Yes | Yes |
| MRT dump (RFC 6396) | Yes | Yes | Yes | Yes | Yes | Yes | No | No | Yes | Yes | Yes |
| Flow export (sFlow/NetFlow/IPFIX) | Yes | No | No | No | No | No | No | No | No | No | No |
| Streaming route events | Yes | No | No | No | No | Yes | Yes | Yes | No | Yes | No |
| JSON event protocol | Yes | No | No | No | No | No | No | Yes | No | No | No |
| Built-in DNS resolver | Yes | No | No | No | No | No | No | No | No | No | No |
| Static DNS name-servers | Yes | No | No | Yes | No | No | Yes | Yes | No | No | No |
| Built-in PeeringDB/IRR/Cymru | Yes | No | No | No | No | No | No | No | No | No | No |
| Unified operational reports | Yes | Partial | Partial | Partial | Partial | No | No | No | No | No | Partial |
| SNMP agent (AgentX/MIB) | No | No | No | Yes | No | No | No | No | No | No | Yes |

Most BGP daemons expose operational issues through a mix of per-command output rather than a single aggregated view. Ze provides a cross-subsystem report bus: any subsystem can push warnings (state-based) or errors (event-based) onto a single place, and `ze show warnings` / `ze show errors` return the aggregate as structured JSON. The login banner reads the same source, so nothing is silently hidden.

SNMP is a deliberate non-goal, not a gap: FRR and freeRtr both expose legacy AgentX/MIB agents, but Ze's operational surface (Prometheus, gNMI, gRPC, structured JSON events) already covers what those MIBs would carry, without maintaining a second protocol stack to do it.

## API & Programmability

| Feature | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gNMI | Yes | No | No | Partial | No | No | No | No | No | No | No |
| gRPC API | Yes | No | No | Partial | No | Yes | Yes | No | Yes | Yes | No |
| REST API | Yes | No | No | Partial | No | No | No | No | No | Partial | No |
| YANG model | Yes | No | No | Partial | No | No | No | No | No | No | No |
| Register-once operator surfaces | Yes | No | No | Partial | No | No | No | No | No | No | Partial |
| CLI tool | Yes | Yes | Yes | Yes | Yes | Yes | Partial | Yes | No | Yes | Yes |
| CLI JSON output | Yes | No | No | Yes | Yes | Yes | No | Yes | No | Yes | No |
| Runtime route injection | Yes | No | No | No | No | Yes | No | Yes | Yes | Yes | Yes |
| Hot reconfiguration (no restart) | Yes | Yes | Yes | Yes | Yes | Yes | Partial | Yes | No | Yes | Yes |
| Embeddable library | No | No | No | No | No | Yes | Yes | No | No | No | No |
| Plugin SDK | Yes | No | No | No | No | No | No | No | No | No | No |
| External process protocol | Yes | No | No | No | No | No | No | Yes | No | No | No |
| MCP (Model Context Protocol) server | Yes | No | No | No | No | No | No | No | No | No | No |
| SSH CLI access | Yes | No | No | No | No | No | No | No | No | No | Yes |

Ze's register-once row means a command, config node, plugin, RPC, or event can feed the CLI, web workbench, REST/gRPC, MCP, generated docs, completion, authorization, and audit paths. FRR has partial YANG/API evidence. freeRtr has integrated CLI, NETCONF, and help generation, but not the same all-surface pipeline in the inspected sources.

## Operations

| Feature | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Crash capture (syslog + file) | Yes | No | No | No | No | No | No | No | No | No | No |
| Config error diagnostics | Yes | No | No | No | No | No | Partial | No | No | Yes | Partial |
| Runtime health monitoring | Yes | No | No | No | No | No | No | No | No | No | No |
| Pre-start readiness checks | Yes | No | No | No | No | No | No | No | No | No | No |
| Product doctor/debug workflow | Yes | No | No | No | No | No | No | No | No | No | Partial |
| Docker image | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | Yes | Yes |
| Fuzz testing | Yes | No | No | No | No | No | Yes | No | No | Yes | No |
| Interop test suite | Yes | No | No | No | No | No | Partial | No | No | Yes | Yes |
| Static routes (ECMP+BFD) | Yes | Yes | Yes | Yes | Yes | No | No | No | No | No | Yes |
| Policy-based routing (PBR) | Yes | No | No | Yes | No | No | No | No | No | No | Yes |
| FIB/kernel integration | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | No | No | Yes |
| Sysctl management | Yes | No | No | Partial | Partial | No | No | No | No | No | No |
| Route server mode | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | No | Yes | Yes |
| Dynamic neighbors | Yes | Yes | Yes | Yes | No | Yes | No | No | Yes | No | Yes |
| Looking glass | Yes | Yes | Yes | No | Yes | No | Yes | No | No | Yes | Yes |
| BFD integration | Partial | Yes | Yes | Yes | No | No | No | No | No | No | Yes |
| Firewall (nftables) | Yes | No | No | Yes | Yes | No | No | No | No | No | Yes |
| Config commit/rollback (candidate + active) | Yes | No | No | No | No | No | No | No | No | No | No |

**Update groups:** Ze automatically groups peers by encoding context and builds each UPDATE once per group, fanning out the wire bytes to all members. No configuration needed -- one other implementation in this table requires explicit peer-group assignment for the same optimization.

## Best-Path Selection

ExaBGP does not perform best-path selection -- it forwards all received routes to external processes and injects routes from them. It is a route injector/receiver, not a router.

| Step | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| LOCAL_PREF | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| AS-path length | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| ORIGIN | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| MED | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| eBGP over iBGP | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| CLUSTER_LIST length | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | No | Yes | Yes |
| ORIGINATOR_ID | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | No | Yes | Yes |
| Stale route demotion (GR) | Yes | Yes | Yes | Yes | Yes | Yes | No | N/A | No | Yes | Yes |
| RPKI preference | Yes | Yes | Yes | Yes | Yes | Yes | No | N/A | Yes | Yes | Yes |
| AIGP | Yes | No | No | Yes | No | Yes | No | N/A | No | No | Yes |
| IGP cost to next-hop | Yes | Yes | Yes | Yes | Yes | No | No | N/A | No | No | Yes |
| Recursive next-hop | Yes | Yes | Yes | Yes | Yes | No | No | N/A | No | No | Yes |
| Multipath/ECMP | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | No | Partial | Yes |

## BNG Capabilities

Ze includes a production BNG stack with two access methods: L2TPv2 (RFC 2661) and PPPoE (RFC 2516), both with RADIUS integration (RFC 2865/2866). Most BGP daemons in the comparison table have no BNG functionality at all. L2TP and PPPoE run concurrently on the same daemon and share the same auth, pool, and shaper plugins through a transport-agnostic PPP driver. RADIUS accounting includes real per-subscriber traffic counters read from the kernel PPP interface. Control-plane scale test infrastructure validates 2000 concurrent L2TP sessions across 10 tunnels without requiring root, kernel modules, or Docker.

## Where Ze is behind today

After the detail tables above: the gaps, stated plainly, not buried in a "No" cell thirteen tables deep.

- **No BGP confederations (RFC 5065)** -- BIRD 3, bio-rd (partial), FRR, GoBGP, BIRD 2, and freeRtr all support it.
- **No privilege separation** -- a signature feature of at least one other implementation in this table.
- **BFD integration is "Partial"** -- several other implementations here have full support.
- **No embeddable library mode** -- at least two other implementations in this table offer one.
- **No custom filter language** -- several implementations here have their own filter DSL; Ze relies on plugin chains instead.
- **No Confederation, no Multi-Topology IS-IS (RFC 5120)** -- Ze's IS-IS matches the single-topology default other implementations ship, but not their optional multi-topology extension.
- **Pre-release, first release 2026** -- sitting in the same table as implementations with years to decades of production hardening (one dates to 1998).
- **Performance is not yet benchmarked at scale.** Go carries an estimated 10-15% CPU overhead versus C/Rust implementations; this has not been measured under load. See [Performance](https://ze-software.net/performance/).

None of this is hidden in the tables above -- it's restated here because a visitor shouldn't have to hunt for it.

## Positioning

**Ze** is an open-source network operating system and the successor to ExaBGP. It runs as a daemon on any Linux, or as a gokrazy appliance where gokrazy init starts Ze with no general shell or package manager. It speaks BGP, manages network interfaces, installs routes into the kernel FIB or VPP plane, and serves a config editor over SSH and a web UI. A plugin architecture with YANG-modeled schemas allows extending the engine without modifying it.

It is also pre-release, first released in 2026, sitting in this table next to implementations with years to decades of production hardening -- one dates to 1998. Its Go runtime carries an estimated 10-15% CPU overhead versus the C/Rust implementations in this table, and that estimate has not yet been benchmarked at scale. It does not yet support BGP confederations, privilege separation, or a custom filter language, all of which at least one other implementation here has shipped for years. Where it is strong -- plugin architecture, YANG-modeled configuration end to end, MCP integration, a production BNG stack alongside BGP -- it is strong because the project chose to build fewer things more deeply rather than match every implementation's full breadth on day one.

**ExaBGP** is the automation specialist. It pioneered the external-process model where BGP events are delivered as JSON to stdin/stdout of user scripts in any language. Deployed worldwide for traffic engineering, DDoS mitigation, route injection, and SDN integration. Broad address family support. Single-threaded Python, no RIB, no best-path selection, no route reflection -- by design. It is a route injector and event source, not a router.

**rustbgpd** is an API-first BGP daemon targeting IX route server and SDN controller use cases. It trades address family breadth for modern operational tooling (gRPC, Prometheus, structured logging, TUI, config diagnostics) and memory safety guarantees.

**bio-rd** is a Go BGP library and daemon originating from DE-CIX. Designed as an embeddable library for building route servers and SDN controllers. Strong route server support with RFC 9234 (BGP Roles), BMP, and ECMP. IPv4/IPv6 unicast only -- no VPN, EVPN, FlowSpec, or other address families. No Graceful Restart or Route Refresh. Apache-2.0 license.

**RustyBGP** is an experimental Rust BGP daemon by the GoBGP team (OSRG). It offers a GoBGP-compatible gRPC API and multi-core design with low memory usage. Explicitly described as "very basic BGP features" -- limited address family and policy support. Useful for research and multi-core experimentation, not yet production-ready.

**FRR** is the most feature-complete open-source routing suite, covering BGP plus OSPF, IS-IS, PIM, and more. Best choice when you need a full routing stack with broad AFI/SAFI coverage and kernel FIB integration.

**BIRD 2/3** dominates IXP route server deployments. Best-in-class memory efficiency and a powerful filter language. BIRD 3 (stable Dec 2024) adds multithreading for 5000+ peer scale. Lacks a programmatic API -- management is CLI/config-file only.

**GoBGP** pioneered the API-first model with gRPC as its primary interface. Broadest AFI/SAFI coverage. Higher memory and CPU usage than C implementations at scale. Best as an SDN controller or route injector rather than a high-performance router.

**OpenBGPd** is security-focused with privilege separation and OpenBSD heritage. Deployed at major IXPs. Lean, reliable, and standards-compliant with strong RFC coverage including BGP Roles and Extended Messages. No programmatic API beyond the CLI socket.

**freeRtr** is a comprehensive router OS written entirely in Java. It implements the full routing stack with its own TCP/IP forwarding plane that can be backed by DPDK, XDP, or P4 dataplanes. Broadest AFI/SAFI coverage of any implementation in this table, including MUP, MVPN, RTC, and VPN FlowSpec. Actively developed since 2012 with 4000+ functional test cases. No programmatic API (CLI-only), no YANG model, no structured logging.

## FAQ

**Ze is pre-release -- why should I trust it yet?**

Don't take that on faith: it's backed by 20,100+ unit tests, 1,400+ end-to-end tests, 72 fuzz targets, and interop testing against 8 independent BGP implementations. That's evidence you can check, not a promise. What it doesn't have yet is operational mileage -- real deployments, over real time, on real networks. Use it in labs first.

**Why no BGP confederations yet?**

Not implemented yet. It's a real gap against implementations that have had it for years, and it's listed as one plainly above rather than left for you to find in a table.

**Why no custom filter language?**

Ze doesn't have a bespoke filter DSL like some implementations here do. Instead, filters are external plugins chained per peer/group: JSON events in, text commands out, over a TLS connect-back socket, in any language that can read lines. That trades a purpose-built mini-language for the full power of a real programming language -- write a filter in Go, Python, or whatever you already know, instead of learning a new syntax.

**Is Ze's performance actually competitive with C/Rust implementations?**

Unknown at scale. The current estimate is 10-15% CPU overhead from the Go runtime, but that number has not been benchmarked under real load. Treat it as an open question, not a claim. See [Performance](https://ze-software.net/performance/) for the actual convergence and throughput numbers measured so far.

**Does Ze implement every BGP feature in this table?**

No. The BGP-specific gaps are listed above: no BGP confederations, partial BFD integration, no custom filter DSL, no privilege separation, and no embeddable library mode. The broader question "does Ze match FRR, VyOS, or freeRtr as a full router OS?" belongs on the [Open Source Network OS comparison](https://ze-software.net/compare/nos/).
