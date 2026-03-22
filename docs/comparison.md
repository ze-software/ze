# BGP Implementation Comparison

A feature comparison of open-source BGP daemon implementations.

> **Disclaimer:** This comparison was generated with AI assistance (partially based on
> [rustbgpd's comparison](https://github.com/lance0/rustbgpd/blob/main/docs/COMPARISON.md))
> and is provided for informational purposes only. All listed projects are under active
> development and their capabilities change over time. Verify current features against each
> project's own documentation before making decisions. Corrections and updates are welcome
> via the [issue tracker](https://codeberg.org/thomas-mangin/ze/issues).

Last updated: 2026-03-21

## Overview

|  | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Language | Go | Rust | C | Go | Rust | C | Go | Python | C | C |
| License | AGPL-3.0 | MIT | GPL-2.0+ | Apache-2.0 | Apache-2.0 | GPL-2.0 | Apache-2.0 | BSD-3-Clause | ISC | GPL-2.0+ |
| Primary interface | CLI (ze) + SSH | gRPC | CLI (birdc) | gRPC | gRPC | CLI (vtysh) | gRPC | CLI + API | CLI (bgpctl) | CLI (birdc) |
| First release | 2026 | 2026 | 2024 | 2018 | 2019 | 2017 | 2014 | 2010 | 2004 | 1998 |
| Maturity | Pre-release | Pre-release | Production | Niche | Experimental | Production | Production | Production | Production | Production |
| Multithreaded | Yes (goroutines) | Yes (tokio) | Yes | Yes (goroutines) | Yes (multi-core) | No | Yes (goroutines) | No | Yes (3-process) | No |
| Plugin architecture | Yes | No | No | No | No | No | No | No | No | No |
| YANG-modeled config | Yes | No | No | No | No | Partial | No | No | No | No |

## Address Families

| AFI/SAFI | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| IPv4 Unicast | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| IPv6 Unicast | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| IPv4 Multicast | No | No | Yes | No | No | Yes | Yes | No | No | Yes |
| IPv6 Multicast | No | No | Yes | No | No | Yes | Yes | No | No | Yes |
| IPv4 Labeled Unicast | Yes | No | No | No | No | Yes | Yes | Yes | No | No |
| IPv6 Labeled Unicast | Yes | No | No | No | No | Yes | Yes | Yes | No | No |
| VPNv4 (RFC 4364) | Yes | No | Yes | No | No | Yes | Yes | Yes | Yes | Yes |
| VPNv6 | Yes | No | Yes | No | No | Yes | Yes | Yes | Yes | Yes |
| L2VPN EVPN (RFC 7432) | Yes | No | Yes | No | No | Yes | Yes | Yes | No | Yes |
| L2VPN VPLS | Yes | No | No | No | No | No | Yes | Yes | No | No |
| IPv4 FlowSpec (RFC 8955) | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes |
| IPv6 FlowSpec | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes |
| VPN FlowSpec | Yes | No | No | No | No | No | Yes | No | No | No |
| BGP-LS (RFC 7752) | Decode (40 TLVs) | No | No | No | No | No | Yes | Decode | No | No |
| SR Policy | No | No | No | No | No | No | Yes | No | No | No |
| IPv4/IPv6 MUP | Yes | No | No | No | No | No | No | No | No | No |
| IPv4/IPv6 MVPN | Decode | No | No | No | No | No | No | No | No | No |
| IPv4 RTC (RFC 4684) | Decode | No | No | No | No | No | No | Yes | No | No |

## Core Protocol

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| RFC 4271 FSM | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| 4-byte ASN (RFC 6793) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Capability negotiation | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Route Refresh (RFC 2918) | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes |
| Enhanced Route Refresh (RFC 7313) | Yes | Yes | Yes | No | No | Yes | No | Yes | Yes | Yes |
| Graceful Restart (RFC 4724) | Yes | Yes | Yes | No | No | Yes | Yes | Partial | Yes | Yes |
| Long-Lived GR (RFC 9494) | Yes | Yes | Yes | No | No | Partial | Yes | No | No | Yes |
| Notification GR (RFC 8538) | Yes | Yes | No | No | No | No | Yes | No | Yes | No |
| Add-Path (RFC 7911) | Yes | Yes | Yes | Yes | Rx only | Yes | Yes | Yes | Yes | Yes |
| Extended Messages (RFC 8654) | Yes | Yes | Yes | No | No | Yes | No | Yes | Yes | Yes |
| Extended Nexthop (RFC 8950) | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes |
| Route Reflector (RFC 4456) | Yes | Yes | Yes | Yes | No | Yes | Yes | No | Yes | Yes |
| Confederation (RFC 5065) | No | No | Yes | No | No | Yes | Yes | No | No | Yes |
| Admin Shutdown (RFC 8203) | Yes | Yes | Yes | Partial | No | Yes | Yes | Yes | Yes | Yes |
| BGP Roles (RFC 9234) | Yes | No | Yes | Yes | No | No | No | No | Yes | Yes |
| Prefix Limit (RFC 4486) | Yes | Yes | Yes | No | No | Yes | Yes | No | Yes | Yes |

## Policy & Route Manipulation

Ze and ExaBGP take a programmable approach to policy: external processes manipulate routes
via a command protocol rather than a built-in policy language.

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Prefix matching (ge/le) | No | Yes | Yes | Yes | Partial | Yes | Yes | No | Yes | Yes |
| AS-path regex | No | Yes | Yes | No | No | Yes | Yes | No | Yes | Yes |
| Standard communities | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Extended communities | Yes | Yes | Yes | No | No | Yes | Yes | Yes | Yes | Yes |
| Large communities (RFC 8092) | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | Yes | Yes |
| Community add/remove/replace | API | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes |
| MED manipulation | API | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes |
| LOCAL_PREF set | API | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes |
| AS-path prepend | API | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes |
| Next-hop set/self | API | Yes | Yes | Yes | No | Yes | Yes | API | Yes | Yes |
| RPKI validation match | Yes | Yes | Yes | No | Yes | Yes | Yes | No | Yes | Yes |
| Neighbor/peer matching | Yes | Yes | Yes | No | No | Yes | Yes | No | Yes | Yes |
| Named policy definitions | No | Yes | Yes | Yes | Partial | Yes | Yes | No | Yes | Yes |
| Policy chaining | No | Yes | Yes | Yes | No | Yes | Yes | No | Yes | Yes |
| Custom filter language | No | No | Yes | No | No | No | No | No | Yes | Yes |
| External process policy | Yes | No | No | No | No | No | No | Yes | No | No |
| Plugin-based policy | Yes | No | No | No | No | No | No | No | No | No |

## Security

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| TCP MD5 (RFC 2385) | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | Yes | Yes |
| TCP-AO (RFC 5925) | No | No | No | No | No | No | No | No | No | No |
| GTSM / TTL Security | Yes | Yes | Yes | Partial | No | Yes | Yes | Yes | Yes | Yes |
| RPKI/RTR (RFC 6810/8210) | Yes | Yes | Yes | No | Yes | Yes | Yes | No | Yes | Yes |
| ASPA verification | No | Yes | Yes | No | No | No | No | No | Yes | Yes |
| Private AS removal | No | Yes | Yes | No | No | Yes | Yes | No | Yes | Yes |
| Privilege separation | No | No | No | No | No | No | No | No | Yes | No |
| Memory-safe language | Yes | Yes | No | Yes | Yes | No | Yes | Yes | No | No |

## Monitoring & Observability

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Prometheus metrics | Yes | Yes | No | Yes | No | Yes | Yes | No | No | No |
| Structured logging (JSON) | Yes | Yes | No | Yes | No | No | No | No | No | No |
| BMP (RFC 7854) | No | Yes | Yes | Yes | Partial | Yes | Yes | No | No | Yes |
| MRT dump (RFC 6396) | No | Yes | Yes | No | Yes | Yes | Yes | No | Yes | Yes |
| Streaming route events | Yes | Yes | No | Yes | No | No | Yes | Yes | No | No |
| JSON event protocol | Yes | No | No | No | No | No | No | Yes | No | No |

## API & Programmability

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gRPC API | No | Yes | No | Yes | Yes | Partial | Yes | No | No | No |
| REST API | No | Partial | No | No | No | Partial | No | No | No | No |
| YANG model | Yes | No | No | No | No | Partial | No | No | No | No |
| CLI tool | Yes | Yes | Yes | Partial | No | Yes | Yes | Yes | Yes | Yes |
| CLI JSON output | Yes | Yes | No | No | No | Yes | Yes | Yes | Yes | No |
| Runtime route injection | Yes | Yes | No | No | Yes | No | Yes | Yes | No | No |
| Hot reconfiguration (no restart) | Yes | Yes | Yes | Partial | No | Yes | Yes | Yes | Yes | Yes |
| Embeddable library | No | No | No | Yes | No | No | Yes | No | No | No |
| Plugin SDK | Yes | No | No | No | No | No | No | No | No | No |
| External process protocol | Yes | No | No | No | No | No | No | Yes | No | No |
| SSH CLI access | Yes | No | No | No | No | No | No | No | No | No |

## Operations

| Feature | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Config error diagnostics | Yes | Yes | No | Partial | No | No | No | No | No | No |
| Docker image | No | Yes | Yes | Yes | No | Yes | Yes | Yes | No | Yes |
| Fuzz testing | Yes | Yes | No | Yes | No | No | No | No | No | No |
| Interop test suite | Yes | Yes | No | Partial | No | No | No | No | No | No |
| FIB/kernel integration | No | No | Yes | Yes | No | Yes | Yes | No | Yes | Yes |
| Route server mode | Yes | Yes | Yes | Yes | No | Yes | Yes | No | Yes | Yes |
| Dynamic neighbors | No | No | Yes | No | Yes | Yes | Yes | No | No | Yes |
| Looking glass | No | Yes | Yes | Yes | No | No | No | No | Yes | Yes |
| BFD integration | No | No | Yes | No | No | Yes | No | No | No | Yes |
| Chaos testing framework | Yes | No | No | No | No | No | No | No | No | No |
| Atomic commit workflow | Yes | No | No | No | No | No | No | No | No | No |
| Schema discovery (CLI) | Yes | No | No | No | No | No | No | No | No | No |
| Healthcheck tool | No | No | No | Partial | No | No | No | Yes | No | No |

## Best-Path Selection

ExaBGP does not perform best-path selection -- it forwards all received routes to external
processes and injects routes from them. It is a route injector/receiver, not a router.

| Step | Ze | rustbgpd | BIRD 3 | bio-rd | RustyBGP | FRR | GoBGP | ExaBGP | OpenBGPd | BIRD 2 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| LOCAL_PREF | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes |
| AS-path length | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes |
| ORIGIN | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes |
| MED | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes |
| eBGP over iBGP | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | Yes | Yes |
| CLUSTER_LIST length | Yes | Yes | Yes | Yes | No | Yes | Yes | N/A | Yes | Yes |
| ORIGINATOR_ID | Yes | Yes | Yes | Yes | No | Yes | Yes | N/A | Yes | Yes |
| Stale route demotion (GR) | Yes | Yes | Yes | No | No | Yes | Yes | N/A | Yes | Yes |
| RPKI preference | Yes | Yes | Yes | No | Yes | Yes | Yes | N/A | Yes | Yes |
| AIGP | No | No | No | No | No | Yes | Yes | N/A | No | No |
| Multipath/ECMP | Partial | Partial | Yes | Yes | No | Yes | Yes | N/A | Yes | Yes |

## Positioning

**Ze** is a programmable BGP daemon and the successor to ExaBGP. It targets SDN, route injection,
monitoring, and route server use cases where external processes need to interact with BGP events.
A plugin architecture with YANG-modeled schemas allows extending the engine without modifying it.
Lazy-parsed wire format and pool-based attribute deduplication reduce memory overhead; when
encoding contexts match, UPDATEs are forwarded without re-parsing. Written in Go with an
estimated 10-15% overhead vs. C/Rust (not yet benchmarked at scale; see
[Performance Trade-offs](DESIGN.md#performance-trade-offs)). ExaBGP configuration files can be
migrated via `ze config migrate`. Built-in RPKI validation, Prometheus metrics, and structured JSON
logging. No FIB integration or built-in policy language -- policy is implemented by plugins and
external processes via the JSON event and text command protocol.

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
