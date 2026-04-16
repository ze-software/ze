# BGP Implementation Comparison

A feature comparison of open-source BGP daemon implementations.

> **Disclaimer:** This comparison was generated with AI assistance (partially based on
> [rustbgpd's comparison](https://github.com/lance0/rustbgpd/blob/main/docs/COMPARISON.md))
> and is provided for informational purposes only. All listed projects are under active
> development and their capabilities change over time. Verify current features against each
> project's own documentation before making decisions. Corrections and updates are welcome
> via the [issue tracker](https://codeberg.org/thomas-mangin/ze/issues).

Last updated: 2026-03-25

## Overview

|  | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Language | Go | Rust | C | Go | Rust | C | Go | Python | C | C | Java |
| License | AGPL-3.0 | MIT | GPL-2.0+ | Apache-2.0 | Apache-2.0 | GPL-2.0 | Apache-2.0 | BSD-3-Clause | ISC | GPL-2.0+ | Free |
| Primary interface | CLI + SSH + REST/gRPC | gRPC | CLI (birdc) | gRPC | gRPC | CLI (vtysh) | gRPC | CLI + API | CLI (bgpctl) | CLI (birdc) | CLI (telnet/SSH) |
| First release | 2026 | 2026 | 2024 | 2018 | 2019 | 2017 | 2014 | 2010 | 2004 | 1998 | 2012 |
| Maturity | Pre-release | Pre-release | Production | Niche | Experimental | Production | Production | Production | Production | Production | Production |
| Multithreaded | Yes (goroutines) | Yes (tokio) | Yes | Yes (goroutines) | Yes (multi-core) | No | Yes (goroutines) | No | Yes (3-process) | No | Yes (per-peer) |
| Plugin architecture | Yes | No | No | No | No | No | No | No | No | No | No |
| YANG-modeled config | Yes | No | No | No | No | Partial | No | No | No | No | No |

## Address Families

| AFI/SAFI | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| IPv4 Unicast | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| IPv6 Unicast | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| IPv4 Multicast | Yes | No | Yes | No | No | Yes | Yes | No | No | Yes | Yes |
| IPv6 Multicast | Yes | No | Yes | No | No | Yes | Yes | No | No | Yes | Yes |
| IPv4 Labeled Unicast | Yes | No | No | No | No | Yes | Yes | Yes | No | No | Yes |
| IPv6 Labeled Unicast | Yes | No | No | No | No | Yes | Yes | Yes | No | No | Yes |
| VPNv4 (RFC 4364) | Yes | No | Yes | No | No | Yes | Yes | Yes | Yes | Yes | Yes |
| VPNv6 | Yes | No | Yes | No | No | Yes | Yes | Yes | Yes | Yes | Yes |
| L2VPN EVPN (RFC 7432) | Yes | No | Yes | No | No | Yes | Yes | Yes | No | Yes | Yes |
| L2VPN VPLS | Yes | No | No | No | No | No | Yes | Yes | No | No | Yes |
| IPv4 FlowSpec (RFC 8955) | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes | Yes |
| IPv6 FlowSpec | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes | Yes |
| VPN FlowSpec | Yes | No | No | No | No | No | Yes | No | No | No | Yes |
| BGP-LS (RFC 7752) | Decode (40 TLVs) | No | No | No | No | No | Yes | Decode | No | No | Yes |
| SR Policy | No | No | No | No | No | No | Yes | No | No | No | Partial |
| IPv4/IPv6 MUP | Yes | No | No | No | No | No | No | No | No | No | Yes |
| IPv4/IPv6 MVPN | Decode | No | No | No | No | No | No | No | No | No | Yes |
| IPv4 RTC (RFC 4684) | Decode | No | No | No | No | No | No | Yes | No | No | Yes |

## Core Protocol

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| RFC 4271 FSM | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| 4-byte ASN (RFC 6793) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Capability negotiation | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Route Refresh (RFC 2918) | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes | Yes |
| Enhanced Route Refresh (RFC 7313) | Yes | Yes | Yes | No | No | Yes | No | Yes | Yes | Yes | Yes |
| Graceful Restart (RFC 4724) | Yes | Yes | Yes | No | No | Yes | Yes | Partial | Yes | Yes | Yes |
| Long-Lived GR (RFC 9494) | Yes | Yes | Yes | No | No | Partial | Yes | No | No | Yes | Yes |
| Notification GR (RFC 8538) | Yes | Yes | No | No | No | No | Yes | No | Yes | No | No |
| Add-Path (RFC 7911) | Yes | Yes | Yes | Yes | Rx only | Yes | Yes | Yes | Yes | Yes | Yes |
| Extended Messages (RFC 8654) | Yes | Yes | Yes | No | No | Yes | No | Yes | Yes | Yes | Yes |
| Extended Nexthop (RFC 8950) | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes | Yes |
| Route Reflector (RFC 4456) | Yes | Yes | Yes | Yes | No | Yes | Yes | No | Yes | Yes | Yes |
| Confederation (RFC 5065) | No | No | Yes | No | No | Yes | Yes | No | No | Yes | Yes |
| Admin Shutdown (RFC 8203) | Yes | Yes | Yes | Partial | No | Yes | Yes | Yes | Yes | Yes | Partial |
| BGP Roles (RFC 9234) | Yes | No | Yes | Yes | No | No | No | No | Yes | Yes | Partial |
| Prefix Limit (RFC 4486) | Yes | Yes | Yes | No | No | Yes | Yes | No | Yes | Yes | Yes |

## Policy & Route Manipulation

Ze takes a programmable approach to policy: external plugin filters manipulate routes
via the `redistribution {}` config block using `<plugin>:<filter>` references.
Filters chain as piped transforms (accept/reject/modify) with delta-only output.
RFC-mandated checks run as default filters that can be selectively overridden.
Built-in filter plugins (shipped with ze) include `bgp-filter-prefix` for
prefix-list matching with ge/le bounds, `bgp-filter-aspath` for AS-path regex
filtering, `bgp-filter-community-match` for community presence matching
(standard/large/extended), `bgp-filter-modify` for route attribute modification
(local-preference, MED, origin, next-hop, AS-path prepend),
`bgp-filter-community` for community tag/strip, and `bgp-role` for RFC 9234
roles enforcement. Filters compose in ordered chains:
`filter import [ prefix-list:X as-path-list:Y modify:Z ]`.
<!-- source: internal/component/bgp/plugins/filter_prefix/ -- bgp-filter-prefix cmd-4 -->
<!-- source: internal/component/bgp/plugins/filter_aspath/ -- bgp-filter-aspath cmd-5 -->
<!-- source: internal/component/bgp/plugins/filter_community_match/ -- bgp-filter-community-match cmd-6 -->
<!-- source: internal/component/bgp/plugins/filter_modify/ -- bgp-filter-modify cmd-7 -->

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Prefix matching (ge/le) | Yes | Yes | Yes | Yes | Partial | Yes | Yes | No | Yes | Yes | Yes |
| AS-path regex | Yes | Yes | Yes | No | No | Yes | Yes | No | Yes | Yes | Yes |
| Standard communities | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Extended communities | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes | Yes |
| Large communities (RFC 8092) | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | Yes | Yes | Yes |
| Community add/remove/replace | API | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes | Yes |
| MED manipulation | Yes | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes | Yes |
| LOCAL_PREF set | Yes | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes | Yes |
| AS-path prepend | Yes | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes | Yes |
| Next-hop set/self | Yes | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes | Yes |
| RPKI validation match | Yes | Yes | Yes | No | Yes | Yes | Yes | No | Yes | Yes | Yes |
| Neighbor/peer matching | Yes | Yes | Yes | No | No | Yes | Yes | No | Yes | Yes | Yes |
| Named policy definitions | Plugin | Yes | Yes | Yes | Partial | Yes | Yes | No | Yes | Yes | Yes |
| Policy chaining | Yes | Yes | Yes | Yes | No | Yes | Yes | No | Yes | Yes | Yes |
| Custom filter language | No | No | Yes | No | No | No | No | No | Yes | Yes | No |
| External process policy | Yes | No | No | No | No | No | No | Yes | No | No | No |
| Plugin-based policy | Yes | No | No | No | No | No | No | No | No | No | No |

## Security

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| TCP MD5 (RFC 2385) | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | Yes | Yes | Yes |
| TCP-AO (RFC 5925) | No | No | No | No | No | No | No | No | No | No | No |
| GTSM / TTL Security | Yes | Yes | Yes | Partial | No | Yes | Yes | Yes | Yes | Yes | Yes |
| RPKI/RTR (RFC 6810/8210) | Yes | Yes | Yes | No | Yes | Yes | Yes | No | Yes | Yes | Yes |
| ASPA verification | No | Yes | Yes | No | No | No | No | No | Yes | Yes | No |
| Private AS removal | No | Yes | Yes | No | No | Yes | Yes | No | Yes | Yes | Yes |
| Privilege separation | No | No | No | No | No | No | No | No | Yes | No | No |
| TACACS+ AAA (RFC 8907) | Yes | No | No | No | No | Yes | No | No | No | No | Yes |
| Memory-safe language | Yes | Yes | No | Yes | Yes | No | Yes | Yes | No | No | Yes |

## Monitoring & Observability

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Prometheus metrics | Yes | Yes | No | Yes | No | Yes | Yes | No | No | No | No |
| Structured logging (JSON) | Yes | Yes | No | Yes | No | No | No | No | No | No | No |
| BMP (RFC 7854) | Yes | Yes | Yes | Yes | Partial | Yes | Yes | No | No | Yes | Yes |
| MRT dump (RFC 6396) | No | Yes | Yes | No | Yes | Yes | Yes | No | Yes | Yes | Yes |
| Streaming route events | Yes | Yes | No | Yes | No | No | Yes | Yes | No | No | No |
| JSON event protocol | Yes | No | No | No | No | No | No | Yes | No | No | No |
| Built-in DNS resolver | Yes | No | No | No | No | No | No | No | No | No | No |
| Built-in PeeringDB/IRR/Cymru | Yes | No | No | No | No | No | No | No | No | No | No |
| Unified operational reports (`show warnings` / `show errors`) | Yes | No | Partial | No | No | Partial | No | No | Partial | Partial | Partial |

<!-- source: internal/core/report/report.go -- cross-subsystem report bus -->
<!-- source: internal/component/cmd/show/show.go -- handleShowWarnings, handleShowErrors -->

Most BGP daemons expose operational issues through a mix of per-command
output (`show protocols all` in BIRD, `show bgp summary` in FRR, counters
in OpenBGPd) rather than a single aggregated view. Ze provides a cross-
subsystem report bus: any subsystem can push warnings (state-based) or
errors (event-based) onto a single place, and `ze show warnings` /
`ze show errors` return the aggregate as structured JSON. The login
banner reads the same source, so nothing is silently hidden. See
[`docs/guide/operational-reports.md`](guide/operational-reports.md).

## API & Programmability

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gRPC API | Yes | Yes | No | Yes | Yes | Partial | Yes | No | No | No | No |
| REST API | Yes | Partial | No | No | No | Partial | No | No | No | No | No |
| YANG model | Yes | No | No | No | No | Partial | No | No | No | No | No |
| CLI tool | Yes | Yes | Yes | Partial | No | Yes | Yes | Yes | Yes | Yes | Yes |
| CLI JSON output | Yes | Yes | No | No | No | Yes | Yes | Yes | Yes | No | No |
| Runtime route injection | Yes | Yes | No | No | Yes | No | Yes | Yes | No | No | Yes |
| Hot reconfiguration (no restart) | Yes | Yes | Yes | Partial | No | Yes | Yes | Yes | Yes | Yes | Yes |
| Embeddable library | No | No | No | Yes | No | No | Yes | No | No | No | No |
| Plugin SDK | Yes | No | No | No | No | No | No | No | No | No | No |
| External process protocol | Yes | No | No | No | No | No | No | Yes | No | No | No |
| SSH CLI access | Yes | No | No | No | No | No | No | No | No | No | Yes |

## Operations

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Config error diagnostics | Yes | Yes | No | Partial | No | No | No | No | No | No | Partial |
| Docker image | No | Yes | Yes | Yes | No | Yes | Yes | Yes | No | Yes | Yes |
| Fuzz testing | Yes | Yes | No | Yes | No | No | No | No | No | No | No |
| Interop test suite | Yes | Yes | No | Partial | No | No | No | No | No | No | Yes |
| FIB/kernel integration | Yes | No | Yes | Yes | No | Yes | Yes | No | Yes | Yes | Yes |
| Sysctl management | Yes | No | No | No | No | Partial | No | No | Partial | No | No |
| Sysctl profiles | Yes | No | No | No | No | No | No | No | No | No | No |
| Route server mode | Yes | Yes | Yes | Yes | No | Yes | Yes | No | Yes | Yes | Yes |
| Dynamic neighbors | No | No | Yes | No | Yes | Yes | Yes | No | No | Yes | Yes |
| Looking glass | Yes | Yes | Yes | Yes | No | No | No | No | Yes | Yes | Yes |
| BFD integration | Partial | No | Yes | No | No | Yes | No | No | No | Yes | Yes |
| Modular subsystem loading | Yes | No | Partial | No | No | No | No | No | No | Partial | No |
| Chaos testing framework | Yes | No | No | No | No | No | No | No | No | No | No |
| Atomic commit workflow | Yes | No | No | No | No | No | No | No | No | No | No |
| Schema discovery (CLI) | Yes | No | No | No | No | No | No | No | No | No | No |
| Healthcheck tool | Yes | No | No | Partial | No | No | No | Yes | No | No | No |
| PeeringDB prefix integration | Yes | No | No | No | No | No | No | No | No | No | No |
| Propagation benchmark tool | Yes | No | No | No | No | No | No | No | No | No | No |
| Update groups | Auto | No | No | No | No | Explicit | No | No | No | No | No |

**Update groups:** Ze automatically groups peers by encoding context (ContextID) and builds each UPDATE once per group, fanning out the wire bytes to all members. No configuration needed. FRR requires explicit peer-group assignment for update group optimization. BIRD batches updates in its write loop but does not have a cross-peer build-sharing mechanism.
<!-- source: internal/component/bgp/reactor/update_group.go -- automatic grouping by sendCtxID -->

## Best-Path Selection

ExaBGP does not perform best-path selection -- it forwards all received routes to external
processes and injects routes from them. It is a route injector/receiver, not a router.

| Step | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| LOCAL_PREF | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| AS-path length | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| ORIGIN | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| MED | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| eBGP over iBGP | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| CLUSTER_LIST length | Yes | Yes | Yes | Yes | No | Yes | Yes | N/A | Yes | Yes | Yes |
| ORIGINATOR_ID | Yes | Yes | Yes | Yes | No | Yes | Yes | N/A | Yes | Yes | Yes |
| Stale route demotion (GR) | Yes | Yes | Yes | No | No | Yes | Yes | N/A | Yes | Yes | Yes |
| RPKI preference | Yes | Yes | Yes | No | Yes | Yes | Yes | N/A | Yes | Yes | Yes |
| AIGP | No | No | No | No | No | Yes | Yes | N/A | No | No | Yes |
| Multipath/ECMP | Partial | Partial | Yes | Yes | No | Yes | Yes | N/A | Yes | Yes | Yes |

## Positioning

**Ze** is a programmable BGP daemon and the successor to ExaBGP. It targets SDN, route injection,
monitoring, and route server use cases where external processes need to interact with BGP events.
A plugin architecture with YANG-modeled schemas allows extending the engine without modifying it.
Lazy-parsed wire format and pool-based attribute deduplication reduce memory overhead; when
encoding contexts match, UPDATEs are forwarded without re-parsing. Written in Go with an
estimated 10-15% overhead vs. C/Rust (not yet benchmarked at scale; see
[Performance Trade-offs](DESIGN.md#performance-trade-offs)). ExaBGP configuration files can be
migrated via `ze config migrate`. Built-in RPKI validation, Prometheus metrics, and structured JSON
logging. The web UI automatically enriches displayed values using YANG-declared decorators
(e.g., AS numbers annotated with organization names via Team Cymru DNS).
No FIB integration or built-in policy language -- policy is implemented by plugins and
external processes via the JSON event and text command protocol.
<!-- source: internal/component/bgp/wireu/wire_update.go -- lazy-parsed WireUpdate -->
<!-- source: internal/component/bgp/attrpool/pool.go -- pool-based attribute dedup -->
<!-- source: internal/component/bgp/context/registry.go -- ContextID encoding context matching -->
<!-- source: internal/component/bgp/plugins/rpki/register.go -- RPKI validation plugin -->
<!-- source: internal/component/web/decorator.go -- DecoratorRegistry, YANG decorator framework -->
<!-- source: cmd/ze/config/cmd_migrate.go -- ze config migrate -->

**ExaBGP** is the automation specialist. It pioneered the external-process model where BGP events
are delivered as JSON to stdin/stdout of user scripts in any language. Deployed worldwide for
traffic engineering, DDoS mitigation, route injection, and SDN integration. Broad address family
support. Single-threaded Python, no RIB, no best-path selection, no route reflection -- by design.
It is a route injector and event source, not a router.

**rustbgpd** is an API-first BGP daemon targeting IX route server and SDN controller use cases.
It trades address family breadth for modern operational tooling (gRPC, Prometheus, structured
logging, TUI, config diagnostics) and memory safety guarantees.

**bio-rd** is a Go BGP library and daemon originating from DE-CIX. Designed as an embeddable
library for building route servers and SDN controllers. Strong route server support with RFC 9234
(BGP Roles), BMP, and ECMP. IPv4/IPv6 unicast only -- no VPN, EVPN, FlowSpec, or other address
families. No Graceful Restart or Route Refresh. gRPC API with streaming RIB observation. Used in
production at IXPs. Apache-2.0 license.

**RustyBGP** is an experimental Rust BGP daemon by the GoBGP team (OSRG). It offers a
GoBGP-compatible gRPC API and multi-core design with low memory usage. Explicitly described as
"very basic BGP features" -- limited address family and policy support. Useful for research
and multi-core experimentation, not yet production-ready.

**FRR** is the most feature-complete open-source routing suite, covering BGP plus OSPF, IS-IS,
PIM, and more. Best choice when you need a full routing stack with broad AFI/SAFI coverage and
kernel FIB integration.

**BIRD 2/3** dominates IXP route server deployments. Best-in-class memory efficiency and a powerful
filter language. BIRD 3 (stable Dec 2024) adds multithreading for 5000+ peer scale. Lacks a programmatic API --
management is CLI/config-file only.

**GoBGP** pioneered the API-first model with gRPC as its primary interface. Broadest AFI/SAFI
coverage. Higher memory and CPU usage than C implementations at scale. Best as an SDN controller
or route injector rather than a high-performance router.

**OpenBGPd** is security-focused with privilege separation and OpenBSD heritage. Deployed at major
IXPs (LINX, Netnod). Lean, reliable, and standards-compliant with strong RFC coverage including
BGP Roles and Extended Messages. No programmatic API beyond the CLI socket.

**freeRtr** is a comprehensive router OS written entirely in Java. It implements the full routing
stack (BGP, OSPF, IS-IS, RIP, EIGRP, LDP, RSVP-TE, and more) with its own TCP/IP forwarding
plane that can be backed by DPDK, XDP, or P4 dataplanes. Broadest AFI/SAFI coverage of any
implementation in this table, including MUP, MVPN, RTC, and VPN FlowSpec. Full best-path
selection with AIGP. Has BMP, MRT dumps, BFD, and SSH CLI access. Actively developed since 2012
with 4000+ functional test cases. No programmatic API (CLI-only), no YANG model, no structured
logging. The own-stack design means Docker integration requires a raw socket bridge (rawInt.bin)
between the container interface and freeRtr's virtual network layer.
<!-- source: external -- codeberg.org/m36/freeRtr -->
