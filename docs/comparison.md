# BGP Implementation Comparison

A feature comparison of open-source routing daemon implementations. Most tables compare BGP. The OSPF table compares the three daemons here that implement it.

> **Disclaimer:** This comparison was generated with AI assistance (partially based on
> [rustbgpd's comparison](https://github.com/lance0/rustbgpd/blob/main/docs/COMPARISON.md))
> and is provided for informational purposes only. All listed projects are under active
> development and their capabilities change over time. Verify current features against each
> project's own documentation before making decisions. Corrections and updates are welcome
> via the [issue tracker](https://github.com/ze-software/ze/issues).

Last updated: 2026-09-01

## Overview

|  | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
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
| IPv4/IPv6 MUP | Yes | No | No | No | No | Yes | No | No | No | No | Yes |
| IPv4/IPv6 MVPN | Decode | No | No | No | No | No | No | No | No | No | Yes |
| IPv4 RTC (RFC 4684) | Decode | No | No | No | No | No | No | Yes | No | No | Yes |

<!-- source: https://github.com/osrg/gobgp/blob/v4.7.0/pkg/packet/bgp/bgp.go -- RF_MUP_IPv4, RF_MUP_IPv6 -->
<!-- source: https://github.com/osrg/gobgp/blob/v4.7.0/pkg/packet/bgp/mup.go -- MUPNLRI.decodeFromBytes, MUPNLRI.Serialize -->

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
| Notification GR (RFC 8538) | No | No | No | No | Yes | Yes | No | No | No | Yes | No |
| Add-Path (RFC 7911) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Rx only | Yes | Yes |
| Paths-Limit (draft-abraitis) | Yes | No | No | Yes | No | No | No | Yes | No | No | No |
| Extended Messages (RFC 8654) | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | No | Yes | Yes |
| Extended Nexthop (RFC 8950) | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes | No | Yes | Yes |
| Route Reflector (RFC 4456) | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | No | Yes | Yes |
| Confederation (RFC 5065) | No | Yes | Yes | Yes | No | Yes | No | No | No | No | Yes |
| Admin Shutdown (RFC 8203) | Yes | Yes | Yes | Yes | Yes | Yes | Partial | Yes | No | Yes | Partial |
| BGP Roles (RFC 9234) | Yes | Yes | Yes | No | Yes | No | Yes | No | No | No | Partial |
| Prefix Limit (RFC 4486) | Yes | Yes | Yes | Yes | Yes | Yes | No | No | No | Yes | Yes |

<!-- source: https://github.com/osrg/gobgp/blob/v4.7.0/pkg/packet/bgp/bgp.go -- CapExtendedMessage, BGPMessage.Serialize -->

## Cross-Protocol Redistribute

Ze advertises locally-originated routes from non-BGP protocols (connected,
static, L2TP, IS-IS, OSPF) into BGP via the
`redistribute-orchestrator` plugin. Operators enable it per-destination and per-source via
`redistribute { destination <proto> { import <source> { family [...]; } } }`. The same config block
also drives the intra-BGP `IngressFilter` ACL when the source is `ibgp` /
`ebgp`. Per-peer NEXT_HOP substitution (`nhop self`) is automatic; explicit
producer-supplied NEXT_HOP is passed through verbatim.

IS-IS meshes with BGP in **both** directions, matching the vendor IGP<->BGP
mutual-redistribution operators expect: `import isis` exports IS-IS SPF routes
into BGP (single source `isis`, both levels), and `destination isis { import
connected/static/bgp }` injects those prefixes into the IS-IS link-state database
as Extended IP Reachability (TLV 135). Like FRR/bird, TLV 135 has no external bit;
the up/down bit (RFC 2966) is set only on a down-level leak. IS-IS is dual-stack
(RFC 5308): IPv6 rides the same single-topology SPF tree (TLV 232 / 236, NLPID
0x8E), and IPv6 redistribution works the same way — matching FRR's single-topology
IS-IS default (FRR also offers RFC 5120 Multi-Topology, which Ze does not yet
implement).

OSPFv2 meshes with BGP in both directions like IS-IS: it installs intra-area,
inter-area ABR, and AS-external SPF routes into the kernel through the same shared
Loc-RIB -> sysrib -> fibkernel pipeline, exports OSPF routes into BGP
(`import ospf`), and injects connected/static/BGP routes as Type 5 AS-External
LSAs (`destination ospf`). `default-information originate` advertises a Type 5
default. Unlike IS-IS TLV 135, OSPF externals carry an explicit metric type:
type 1 (E1) adds the internal cost to the ASBR, type 2 (E2) keeps the advertised
metric, and E1 always wins over E2. Ze also implements stub, totally-stubby, and
NSSA areas (RFC 3101) with Type 7 origination, highest-Router-ID translator
election, Type 7 to Type 5 translation, and the §2.5 preference -- matching the
FRR/bird NSSA feature set. On the opaque-LSA carrier (RFC 5250) Ze
matches FRR's Traffic Engineering LSA (RFC 3630/5392) and Router Information LSA
(RFC 7770): it advertises the informational capability bits (graceful restart,
stub router, TE) in an Opaque type-4 LSA for OSPFv2 and a function-code-12 LSA for
OSPFv3, and exposes a consumer-neutral TLV hook that a later Segment Routing module
plugs into -- the same extensibility FRR's RI implementation provides. Ze also matches
FRR's RFC 7684 Extended Prefix/Link Opaque LSAs (Opaque type 7/8): it originates and
decodes the prefix/link attribute containers, associating each with the Route Type / link
identity of the base LSA FRR sees, and offers a generic sub-TLV registration hook for the
SID values a Segment Routing module (RFC 8665) fills -- Ze ships the RFC-7684 containers,
which are conformant on their own since RFC 7684 defines no sub-TLV values. Per-interface
authentication covers simple password, keyed-MD5 (RFC 2328), HMAC-SHA (RFC 5709), and
the RFC 7474 extended-sequence variant, with key chains for hitless rotation and
sequence-number replay protection. Ze also matches FRR's OSPF Graceful Restart
(RFC 3623 for IPv4 `ospfd`, RFC 5187 for IPv6 `ospf6d`) in both roles: as a restarter it
floods Grace-LSAs (the IPv4 Opaque type 3 / IPv6 native LS type 0x000B), suppresses self-LSA
origination and route churn while keeping the RTPROT_ZE FIB programmed, and re-installs
before the sweep deadline; as a helper it holds the adjacency and suppresses LSDB churn with
strict-LSA-checking and the stub-area exception. Unlike FRR, Ze drives both address families
through one shared control plane with a single family-neutral `graceful-restart` config.
<!-- source: internal/plugins/ospf/gr.go -- grManager, registerGraceConsumer -->
<!-- source: internal/plugins/ospf/packet/auth_verify.go -- Sign, Verify -->
<!-- source: internal/plugins/ospf/spf/install.go -- Installer Apply -->
<!-- source: internal/plugins/ospf/spf/computer.go -- Computer Run -->
<!-- source: internal/plugins/ospf/spf/interarea.go -- ComputeInterArea -->
<!-- source: internal/plugins/ospf/spf/external.go -- ComputeExternal E1/E2 -->
<!-- source: internal/plugins/ospf/redistribute/consumer.go -- Consumer InjectRoute -->

<!-- source: internal/component/bgp/plugins/redistribute_egress/redistribute.go -- consumer -->
<!-- source: internal/plugins/isis/redistribute/source.go -- isis source + producer emit -->
<!-- source: internal/plugins/isis/redistribute/consumer.go -- isis consumer (TLV 135) -->
<!-- source: internal/core/redistevents/events.go -- producer-shared payload -->

## Policy & Route Manipulation

Ze takes a programmable approach to policy: external plugin filters manipulate routes
via `filter { import [...] export [...] }` chains using named filter instances or
explicit `<plugin>:<filter>` references.
Filters chain as piped transforms (accept/reject/modify) with delta-only output.
RFC-mandated checks run as default filters that can be selectively overridden.
Built-in filter plugins (shipped with ze) include `bgp-filter-prefix` for
prefix-list matching with ge/le bounds, `bgp-filter-aspath` for AS-path regex
filtering, `bgp-filter-community-match` for community presence matching
(standard/large/extended), `bgp-filter-modify` for route attribute modification
(local-preference, MED, origin, next-hop, AS-path prepend),
`bgp-filter-community` for community tag/strip, `bgp-filter-path-asn` for
rejecting a path that carries a named ASN at a named position (the RFC 7454
Section 9 transit-leak check), and `bgp-role` for RFC 9234
roles enforcement. Filters compose in ordered chains:
`filter import [ prefix-list:X as-path-list:Y modify:Z ]`.
<!-- source: internal/component/bgp/plugins/filter_prefix/ -- bgp-filter-prefix cmd-4 -->
<!-- source: internal/component/bgp/plugins/filter_aspath/ -- bgp-filter-aspath cmd-5 -->
<!-- source: internal/component/bgp/plugins/filter_community_match/ -- bgp-filter-community-match cmd-6 -->
<!-- source: internal/component/bgp/plugins/filter_modify/ -- bgp-filter-modify cmd-7 -->
<!-- source: internal/component/bgp/plugins/filter_path_asn/ -- bgp-filter-path-asn cmd-8 -->

Some community policy is config rather than a filter in a chain. The ingress
community filter carries `scrub-own-ga` for the RFC 7454 Section 11
own-Global-Administrator scrub, with a `scrub-keep-function` carve-out for the
function numbers customers signal on, `relation-tag` for the RFC 8195 relation
tag, and `blackhole-propagation` to bound how far a received BLACKHOLE route
travels. A `modify` policy carries its own `match` container over standard, large
and extended communities, so one policy changes only the routes that carry a
stated value and passes the rest through unchanged. Its `del { med; }` directive
is the mechanism RFC 4271 Section 5.1.4 requires, and it works on import chains
only.

Blackhole honoring (RFC 7999) is per session and off by default. One
`blackhole { communities; prefixes; }` container states both Section 3.3
conditions, and the same list gates the send side, so `announce blackhole`
reaches only the sessions that agreed. See
[`docs/config-reference.md`](config-reference.md#blackhole-rfc-7999).
<!-- source: internal/component/bgp/plugins/filter_community/yang/ze-filter-community.yang -- grouping community-filter-fields -->
<!-- source: internal/component/bgp/plugins/rib/yang/ze-rib.yang -- grouping blackhole-honor-fields -->

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
| Boot-time management-plane exposure guard | Yes | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear |

**Boot-time management-plane exposure guard:** Ze refuses to start when a
management service would bind a non-loopback address with no authentication. The
check covers the web server in insecure mode, MCP, gNMI, and the API server that
serves REST and gRPC. It runs once, after every address and credential is
resolved and before anything binds, so a refusal leaves nothing bound and the
process exits 1. A wildcard address, an empty address, a DNS name, and
`localhost` all count as non-loopback, and a surface that declares itself
unauthenticated with no resolved address is refused rather than passed. A reload
re-runs the same check against the rebuilt authentication and rejects an unsafe
migration.

The web listener also serves a certificate named in the PKI store rather than a
self-signed one. It sends the leaf certificate and every stored intermediate, it
stops the server when the name does not resolve, and it rotates the material on
reload through a per-handshake lookup with no rebind.

The other ten daemons are marked `Unclear` because this claim was not checked
against their source. Several of them expose no comparable management plane, so
`Unclear` here says the comparison was not made, not that the guard is missing
from something that needs one.
<!-- source: cmd/ze/hub/mgmt_guard.go -- checkMgmtListeners, listenAddrIsNonLoopback -->
<!-- source: cmd/ze/hub/main.go -- mgmtListeners declaration sites, exit on refusal -->
<!-- source: internal/component/web/yang/ze-web-conf.yang -- leaf certificate -->

## Monitoring & Observability

| Feature | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Prometheus metrics | Yes | No | No | Yes | No | Yes | Yes | No | No | Yes | No |
| Structured logging (JSON) | Yes | No | No | No | No | No | Yes | No | No | Yes | No |
| BMP (RFC 7854) | Yes | Yes | Yes | Yes | No | Yes | Yes | No | Partial | Yes | Yes |
| MRT dump (RFC 6396) | Yes | Yes | Yes | Yes | Yes | Yes | No | No | Yes | Yes | Yes |
| Session capture and replay | Yes | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear | Unclear |
| Flow export (sFlow/NetFlow/IPFIX) | Yes | No | No | No | No | No | No | No | No | No | No |
| Streaming route events | Yes | No | No | No | No | Yes | Yes | Yes | No | Yes | No |
| JSON event protocol | Yes | No | No | No | No | No | No | Yes | No | No | No |
| Built-in DNS resolver | Yes | No | No | No | No | No | No | No | No | No | No |
| Static DNS name-servers | Yes | No | No | Yes | No | No | Yes | Yes | No | No | No |
| Built-in PeeringDB/IRR/Cymru | Yes | No | No | No | No | No | No | No | No | No | No |
| Unified operational reports (`show warnings` / `show errors`) | Yes | Partial | Partial | Partial | Partial | No | No | No | No | No | Partial |
| SNMP agent (AgentX/MIB) | No | No | No | Yes | No | No | No | No | No | No | Yes |

**Session capture and replay:** Ze records one peer's inbound protocol events as
raw wire bytes in a JSONL file, and `ze-test replay` feeds that file back through
the same read path with a fake clock, so a session bug on an operator's box
reproduces on a developer's machine. This is not MRT: MRT records ROUTES for
analysis, after decoding, while a capture records the BYTES the peer sent,
including the malformed UPDATE that MRT would never represent. The other ten
daemons are marked `Unclear` because this claim was not checked against their
source; several ship MRT, which the row above already counts, and MRT is a
different capability.

<!-- source: internal/component/bgp/reactor/capture_replay.go -- sessionCapture, teeCapture -->
<!-- source: internal/test/cli/cmd_replay.go -- runReplay -->

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

SNMP is a deliberate non-goal, not a gap: FRR and freeRtr both expose
legacy AgentX/MIB agents, but Ze's operational surface (Prometheus, gNMI,
gRPC, structured JSON events) already covers what those MIBs would carry,
without maintaining a second protocol stack to do it.

## API & Programmability

| Feature | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gNMI | Yes | No | No | Partial | No | No | No | No | No | No | No |
| gRPC API | Yes | No | No | Partial | No | Yes | Yes | No | Yes | Yes | No |
| REST API | Yes | No | No | Partial | No | No | No | No | No | Partial | No |
| YANG model | Yes | No | No | Partial | No | No | No | No | No | No | No |
| CLI tool | Yes | Yes | Yes | Yes | Yes | Yes | Partial | Yes | No | Yes | Yes |
| CLI JSON output | Yes | No | No | Yes | Yes | Yes | No | Yes | No | Yes | No |
| Runtime route injection | Yes | No | No | No | No | Yes | No | Yes | Yes | Yes | Yes |
| Hot reconfiguration (no restart) | Yes | Yes | Yes | Yes | Yes | Yes | Partial | Yes | No | Yes | Yes |
| Embeddable library | No | No | No | No | No | Yes | Yes | No | No | No | No |
| Plugin SDK | Yes | No | No | No | No | No | No | No | No | No | No |
| External process protocol | Yes | No | No | No | No | No | No | Yes | No | No | No |
| MCP (Model Context Protocol) server, revision 2026-07-28 | Yes | No | No | No | No | No | No | No | No | No | No |
| MCP `server/discover` capability advertisement | Yes | No | No | No | No | No | No | No | No | No | No |
| MCP elicitation (form mode, via Multi Round-Trip Requests) | Yes | No | No | No | No | No | No | No | No | No | No |
| MCP background tasks (`io.modelcontextprotocol/tasks` extension, polled) | Yes | No | No | No | No | No | No | No | No | No | No |
| MCP Apps (UI resources, `io.modelcontextprotocol/ui` extension) | Yes | No | No | No | No | No | No | No | No | No | No |
| MCP cacheable results (`ttlMs`, `cacheScope`) | Yes | No | No | No | No | No | No | No | No | No | No |
| SSH CLI access | Yes | No | No | No | No | No | No | No | No | No | Yes |

**MCP scope for this table:** Ze's rows describe MCP protocol revision
`2026-07-28` as implemented in the inspected checkout. `No` in the other columns
means no MCP server was found in the inspected scope of that project, not a
claim that none exists anywhere.

**The tool surface is two handcrafted tools plus one per command.** `ze_execute`
dispatches a raw command, and `ze_reference` returns the machine-readable AI
reference. Beside those, Ze generates one typed tool for every command in the
registry, named from the command path (`show bgp rib` becomes `ze_show_bgp_rib`).
A plugin that registers a command therefore adds its own tool with no MCP code.
<!-- source: internal/component/mcp/tools.go -- handcraftedNames, toolName, generateTools -->

**Ze's elicitation changed shape, and the row names the shape.** Ze shipped
server-initiated `elicitation/create` under revision `2025-06-18`. Revision
`2026-07-28` removed protocol-level sessions and the server-to-client stream.
That revision also states that a server "**MUST NOT** send independent JSON-RPC
*requests*" on any stream.

A server can therefore no longer push a prompt. The server returns the prompt
instead, as a Multi Round-Trip Request: `ze_execute` called without a `command`
answers `resultType: "input_required"` with an `inputRequests` map. The client
then retries the original call with `inputResponses`, which carries the value.

Two limits qualify the `Yes`. Ze emits **form mode only**, and url mode is not
implemented. Ze also attaches no `requestState`. That omission is conformant,
because requirement 6 asks for "at least one of `inputRequests` or
`requestState`" and `inputRequests` alone meets it. But Ze holds no
continuation state across a retry, and Ze does not support a flow that needs
one.
<!-- source: internal/component/mcp/mrtr.go -- inputRequiredForMissingCommand, rejectUnsolicitedRequestState -->
<!-- source: internal/component/mcp/elicit.go -- newElicitRequest, elicitModeForm -->

## Operations

| Feature | Ze | BIRD 3 | BIRD 2 | FRR | OpenBGPd | GoBGP | bio-rd | ExaBGP | RustyBGP | rustbgpd | freeRtr |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Crash capture (syslog + file) | Yes | No | No | No | No | No | No | No | No | No | No |
| Config error diagnostics | Yes | No | No | No | No | No | Partial | No | No | Yes | Partial |
| Runtime health monitoring | Yes | No | No | No | No | No | No | No | No | No | No |
| Pre-start readiness checks | Yes | No | No | No | No | No | No | No | No | No | No |
| Docker image | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | Yes | Yes |
| netlab lab integration | Yes (daemon, in-repo, unvalidated) | No | Yes (daemon) | Yes (device) | Yes (device) | Not found | Not found | Not found | Not found | Not found | Not found |
| Fuzz testing | Yes | No | No | No | No | No | Yes | No | No | Yes | No |
| Interop test suite | Yes | No | No | No | No | No | Partial | No | No | Yes | Yes |
| Static routes (ECMP+BFD) | Yes | Yes | Yes | Yes | Yes | No | No | No | No | No | Yes |
| Policy-based routing (PBR) | Yes | No | No | Yes | No | No | No | No | No | No | Yes |
| FIB/kernel integration | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | No | No | Yes |
| Sysctl management | Yes | No | No | Partial | Partial | No | No | No | No | No | No |
| Sysctl profiles | Yes | No | No | No | No | No | No | No | No | No | No |
| Route server mode | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | No | Yes | Yes |
| Dynamic neighbors | Yes | Yes | Yes | Yes | No | Yes | No | No | Yes | No | Yes |
| Looking glass | Yes | Yes | Yes | No | Yes | No | Yes | No | No | Yes | Yes |
| Multicast RPF lookup | Yes | No | No | Yes | No | No | No | No | No | No | Yes |
| BFD integration | Partial | Yes | Yes | Yes | No | No | No | No | No | No | Yes |
| Firewall (nftables) | Yes | No | No | Yes | Yes | No | No | No | No | No | Yes |
| IPv6 Router Advertisement sender (radvd role) | Yes | No | No | Yes | No | No | No | No | No | No | Unclear |
| Modular subsystem loading | Yes | Partial | Partial | No | No | No | No | No | No | No | No |
| Config commit/rollback (candidate + active) | Yes | No | No | No | No | No | No | No | No | No | No |
| Schema discovery (CLI) | Yes | No | No | No | No | No | No | No | No | No | No |
| Healthcheck tool | Yes | No | No | No | No | No | Partial | Yes | No | No | No |
| SMART disk management | Yes | No | No | No | No | No | No | No | No | No | No |
| PeeringDB prefix integration | Yes | No | No | No | No | No | No | No | No | No | No |
| Propagation benchmark tool | Yes | No | No | No | No | No | No | No | No | No | No |
| Update groups | Auto | No | No | Explicit | No | No | No | No | No | No | No |

**IPv6 Router Advertisement sender:** Ze sends Router Advertisements on an
interface unit (RFC 4861), so a separate radvd is not needed for SLAAC, a
default router, or RDNSS resolvers. FRR does the same work in zebra, through its
`ipv6 nd` interface commands. The other BGP daemons in this table manage no
interfaces at all, so the row is `No` for them rather than a gap. freeRtr is
marked `Unclear`: it runs its own IP stack, and this claim was not checked
against its source.
<!-- source: internal/plugins/iface/ra/sender_linux.go -- Router Advertisement send loop -->
<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- container router-advertisement -->

**netlab lab integration:** [netlab](https://netlab.tools) builds a lab from a YAML
topology and runs each node under containerlab. Scope: netlab 26.08, inspected at
`netsim/daemons/` and `netsim/devices/`. `Not found` means not found in that scope.
BIRD is a daemon there and the recipe builds BIRD 2.19.1, so the BIRD 3 column is `No`
rather than a gap. OpenBGPd is reached through the `openbsd` device and a vrnetlab
image. FRR is reached through a device and the `quay.io/frrouting/frr:10.6.1`
container.

Each daemon declares what it supports in a `features:` block. Ze declares 6 keys
(initial, bgp, ospf, isis, bfd, routing), BIRD declares 10, and FRR declares 19. FRR
covers EVPN, MPLS, SR, SRv6, VRF, VXLAN, VLAN, LAG and STP. Ze declares none of them.
IS-IS is the one place where the ze daemon declares a protocol the BIRD daemon does
not.

Every ze key is rendered and parsed by `./le netlab render-check`. No key is
validated against netlab's own integration tests, because a live lab was never started
here. Ze's artifacts are in this repository at `contrib/netlab/` and are not upstream
yet.
<!-- source: contrib/netlab/ze.yml -- features, daemon_config -->
<!-- source: netlab 26.08 (Python package netsim, __init__.py __version__) -- netsim/daemons/bird.yml features and clab.sw_version 2.19.1; netsim/devices/frr.yml features and clab.image; netsim/devices/openbsd.yml clab.image -->

**Update groups:** Ze automatically groups peers by encoding context (ContextID) and builds each UPDATE once per group, fanning out the wire bytes to all members. No configuration needed. FRR requires explicit peer-group assignment for update group optimization. BIRD batches updates in its write loop but does not have a cross-peer build-sharing mechanism.
<!-- source: internal/component/bgp/reactor/update_group.go -- automatic grouping by sendCtxID -->

**Reactor RS fast path:** For route-server deployments, Ze can forward UPDATEs directly from the session read goroutine, bypassing the plugin dispatch chain entirely. This reduces the number of boundary crossings from 6 to 1 (wire to forward pool), approaching BIRD's 2-hop architecture. Enabled via `rs-fast-path` in the peer group behavior config.
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRS -->

**Config completeness:** a YANG-modeled daemon can accept a config subtree that reaches no code. Ze fails the build when that happens: `TestConfigSchemaRootsClaimed` resolves the whole config schema, unions the config roots the plugin registry declares with the handler paths the schema registry binds, and fails on a subtree neither covers. The inverse runs too, so a declared config root that names no schema node fails as well. Five paths are recorded as exceptions, each with the file and symbol that reads them. At run time `ze doctor` reports `doctor-config-root-unclaimed` for a configured subtree the running binary delivers to nobody, which is what an operator sees when a plugin is compiled out or did not start.
<!-- source: internal/component/plugin/all/config_claims_test.go -- TestConfigSchemaRootsClaimed -->
<!-- source: internal/component/config/claims/claims.go -- Audit -->

## Best-Path Selection

ExaBGP does not perform best-path selection -- it forwards all received routes to external
processes and injects routes from them. It is a route injector/receiver, not a router.

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
| AIGP | Partial | No | No | Yes | No | Yes | No | N/A | No | No | Yes |
| IGP cost to next-hop | Yes | Yes | Yes | Yes | Yes | No | No | N/A | No | No | Yes |
| Recursive next-hop | Yes | Yes | Yes | Yes | Yes | No | No | N/A | No | No | Yes |
| Multipath/ECMP | Yes | Yes | Yes | Yes | Yes | Yes | Yes | N/A | No | Partial | Yes |

## OSPF Standards Coverage

The tables above compare BGP. This one compares OSPF, against the two daemons that
also implement it natively. Ze, FRR and BIRD are the whole population here: the
other implementations in this document speak BGP only.

**Version basis.** Ze at commit `35f84e060`. FRR at release tag `frr-10.7.1`
(2026-08-31). BIRD 2.19.2 and BIRD 3.3.2 (both 2026-07-30), which carry an
identical OSPF standards list, so one column serves both.

**What each column asserts.** The three columns are not judged alike, so no cell
here says a bare "yes".

A Ze cell states what the code does with the value, in three grades. **Originates
and consumes** means Ze builds it, parses it on receipt, and something acts on the
parsed value. **Originates** means built and parsed, with no consumer beyond
display. **No** means no producer exists.

An FRR or BIRD cell names the evidence: the configuration command, or the source
file that implements it, at the version above. That is what was checked. Neither
daemon was run, and no cell here claims interop.

| RFC | Ze | FRR 10.7.1 | BIRD 2.19.2 / 3.3.2 |
| --- | --- | --- | --- |
| 8362 OSPFv3 Extended LSAs | Originates 3 of the 7 LSA types, and only under segment routing. Decoded on receipt only to read Prefix-SIDs. No consumer in SPF. Non-conformant on the wire, see the note below | No. The OSPFv3 LS-type list in `ospf6d/ospf6_lsa.h` ends at `GRACE_LSA 0x000b` and bounds the handler table there | No. Absent from the maintainers' own standards list in `proto/ospf/ospf.c` |
| 8666 OSPFv3 Segment Routing | Originates and consumes: installs labels to the MPLS FIB. 3 MUST-level gaps declared | No. `ospf6d/` holds no `ospf6_sr.*`, and OSPFv3 SR is defined over the Extended LSAs above | No. Same standards list |
| 8665 OSPFv2 Segment Routing | Originates and consumes: installs labels. 14 MUST-level gaps declared | Yes: `segment-routing on`, `ospfd/ospf_sr.c`. Upstream marks it EXPERIMENTAL | No. Same standards list |
| 5286 Loop-Free Alternate | Originates nothing on the wire, correctly. Computes LFA and TI-LFA, and the backup next hops reach the kernel with `RTNH_F_LINKDOWN` | TI-LFA only: `fast-reroute ti-lfa`, and the docs say it requires a Segment Routing configuration. FRR's RFC 5286 code is `isisd/isis_lfa.c`, which `ospfd` does not use | No. The OSPF grammar in `proto/ospf/config.Y` has no LFA, backup or alternate keyword |
| 7684 Extended Prefix and Link | Originates and consumes: Segment Routing reads the prefix SIDs. A received Adj-SID is decoded and discarded | Yes: `ospfd/ospf_ext.c`, a direct RFC 7684 implementation | No. Same standards list |
| 7770 Router Information | Originates and consumes: a remote SRGB read from these LSAs drives label computation | Yes: `router-info [as \| area]`, `ospfd/ospf_ri.c`. Its header cites the predecessor RFC 4970 | Listed as supported, but dormant: `ospf_originate_ri_lsa()` is commented out at its only call site, and `lsa_validate_ri` says "we do not really process RI LSAs" |
| 3630 and 5392 Traffic Engineering | Originates both. The TE database feeds `show ospf te-database` and metrics. No CSPF or admission consumer reads it: `LookupLink` has no non-test caller | Yes: `mpls-te on`, `mpls-te router-address`, `mpls-te inter-as area \| as`, `ospfd/ospf_te.c` | No. Neither RFC is on the standards list |
| 5250 Opaque LSAs | Originates and consumes, with a registry other features register consumers into | Yes: `ospf opaque-lsa`, `capability opaque`, `ospfd/ospf_opaque.c` | Carries and floods them, including unknown types, and originates none |
| 3623 and 5187 Graceful Restart | Originates and consumes, both versions: Grace-LSA out, helper in, with self-LSA suppression, install suppression and FIB retention | Yes, both: `graceful-restart`, `graceful-restart helper enable`, `ospfd/ospf_gr.c` and `ospf6d/ospf6_gr.c` | Yes, both: `graceful restart on \| aware`. The default is `aware`, which is helper only |
| 5443 LDP-IGP Synchronization | Originates and consumes: an LDP session event substitutes the advertised metric, and the Router-LSA is re-originated | Yes: `mpls ldp-sync` with a holddown, default VRF only | No, and BIRD has no LDP at all |
| 4577 OSPF as PE/CE | No. No DN-bit originator, no VRF, no sham link | No. `OSPF_OPTION_DN` is defined in `ospfd/ospfd.h` and consumed nowhere, and the interface-type set in `lib/libospf.h` has no sham-link type | The DN bit only: `vpn pe` in the grammar, consumed in `rt.c`. No sham link in its interface-type set either |

**Where the open ground is.** RFC 8362 and RFC 8666 travel together, because
OSPFv3 Segment Routing is defined over the Extended LSAs. Neither FRR nor BIRD
implements either, so this is the one part of the table where Ze is alone.

**A caveat on RFC 8362, stated because the row above would otherwise mislead.**
Ze's Extended LSAs carry the U-bit clear. RFC 8362 Section 2 requires it set, "so
that the LSAs will be flooded by OSPFv3 routers that do not understand them". The
same section assigns LS types 0xA021 through 0xA029, and Ze declares those types
less 0x8000.

With the U-bit clear, RFC 5340 Section 4.4.1 confines an unrecognized LSA to
link-local scope. Ze's Prefix-SIDs therefore stop at the first router in a mixed
area that does not support them. Having the feature and having it interoperate
are different claims. Only the first is true today.

<!-- source: internal/plugins/ospf/v3/types/lsa.go -- the Extended LSA types -->
<!-- source: internal/plugins/ospf/sr_origination_v6.go -- v6OriginateSR -->
<!-- source: internal/plugins/ospf/te_originate.go -- teOriginateType1, teOriginateType6 -->
<!-- source: internal/plugins/ospf/ldp_sync.go -- effectiveP2PCost, ldpSyncWithholdTransit -->
<!-- source: internal/plugins/ospf/spf/lfa.go -- the LFA and TI-LFA computation -->
<!-- source: internal/plugins/ospf/sr_install.go -- srRemoteCapabilities, srRemotePrefixSIDs -->

## Positioning

**Ze** is an open-source network operating system and the successor to ExaBGP. It runs as a
daemon on any Linux (systemd or any process manager) or as a dedicated appliance image built
with gokrazy for purpose-built hardware -- same binary, same config. It speaks BGP, manages
network interfaces (ethernet, bridge, VLAN, tunnels, WireGuard, DHCP), installs routes into
the kernel FIB or VPP data plane (recursive next-hop resolution, ECMP nexthop groups, route type/metric/table, MPLS label push/swap/pop from BGP labeled unicast, SRv6), and serves a config editor over SSH and a web UI. A plugin
architecture with YANG-modeled schemas allows extending the engine without modifying it.
Lazy-parsed wire format and pool-based attribute deduplication reduce memory overhead; when
encoding contexts match, UPDATEs are forwarded without re-parsing. Written in Go with an
estimated 10-15% overhead vs. C/Rust (not yet benchmarked at scale; see
[Performance Trade-offs](DESIGN.md#performance-trade-offs)). ExaBGP configuration files can be
migrated via `ze config migrate`. Built-in RPKI validation, policy filters (prefix-list,
AS-path regex, community matching, attribute modification), Prometheus metrics, and structured
JSON logging. The web UI automatically enriches displayed values using YANG-declared decorators
(e.g., AS numbers annotated with organization names via Team Cymru DNS). External process
plugins extend policy further via the JSON event and text command protocol.
<!-- source: internal/component/bgp/wireu/wire_update.go -- lazy-parsed WireUpdate -->
<!-- source: internal/component/bgp/attrpool/pool.go -- pool-based attribute dedup -->
<!-- source: internal/core/bgp/context/registry.go -- ContextID encoding context matching -->
<!-- source: internal/component/bgp/plugins/rpki/register.go -- RPKI validation plugin -->
<!-- source: internal/component/web/decorator.go -- DecoratorRegistry, YANG decorator framework -->
<!-- source: internal/component/config/cli/cmd_migrate.go -- ze config migrate -->

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
<!-- source: https://codeberg.org/m36/freeRtr -- upstream project -->

## BNG Capabilities

Ze includes a production BNG stack with two access methods: L2TPv2 (RFC 2661)
and PPPoE (RFC 2516), both with RADIUS integration (RFC 2865/2866). Most BGP
daemons in the comparison table have no BNG functionality. L2TP and PPPoE run
concurrently on the same daemon and share the same auth, pool, and shaper
plugins through the transport-agnostic PPP Driver. The following RADIUS Access-Accept subscriber profile
attributes are consumed:

| Attribute | RFC | Ze Behavior |
|-----------|-----|-------------|
| Framed-IP-Address (8) | RFC 2865 S5.8 | Bypasses pool; assigns address directly to PPP session |
| Framed-IP-Netmask (9) | RFC 2865 S5.9 | Extracted and stored (consumed by future PPP interface config) |
| Framed-Pool (88) | RFC 2865 | Selects a named pool for IP allocation |
| Session-Timeout (27) | RFC 2865 S5.27 | Enforces maximum session duration; CDN on expiry |
| Idle-Timeout (28) | RFC 2865 S5.28 | Disconnects after inactivity period (Linux RX byte counters) |
| Filter-Id (11) | RFC 2865 S5.11 | Multi-valued: "cos:\<name\>" selects a dynamic 802.1p CoS profile for the access VLAN; other values set the initial shaping rate |
| Vendor-Specific (26) | RFC 2865 S5.26 | Extracts CoS profile names from Cisco-AVPair (`subscriber:sub-qos-policy-{in,out}`), Juniper ERX (Ingress/Egress-Policy-Name), Nokia (Alc-Subscriber-QoS-Override), and Huawei (HW-Subscriber-QoS-Profile) VSAs; extracts shaper rate from MikroTik (Mikrotik-Rate-Limit). Ze "cos:" Filter-Id takes priority over vendor VSAs. Unknown vendor IDs silently ignored. |
| Acct-Interim-Interval (85) | RFC 2869 S5.16 | Sets the per-session accounting update interval, clamped to [60,3600]s. RFC 2869 S2.1 gives a configured `acct-interval` precedence over it, so the attribute decides only for a deployment that leaves that leaf unset |

<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- extractAuthMetadata -->
<!-- source: internal/component/l2tp/plugins/authradius/extract_vsa.go -- vendor VSA CoS/rate extraction -->
<!-- source: internal/component/l2tp/session_timeout.go -- timeout enforcement -->
<!-- source: internal/component/l2tp/plugins/shaper/filter_rate.go -- Filter-Id rate parsing -->

RADIUS Accounting (Interim-Update and Stop) includes real per-subscriber
traffic counters read from the pppN kernel interface:

| Attribute | RFC | Ze Behavior |
|-----------|-----|-------------|
| Acct-Input-Octets (42) | RFC 2866 S5.7 | Bytes received from subscriber (pppN rx_bytes mod 2^32) |
| Acct-Output-Octets (43) | RFC 2866 S5.8 | Bytes sent to subscriber (pppN tx_bytes mod 2^32) |
| Acct-Input-Packets (47) | RFC 2866 S5.9 | Packets received from subscriber |
| Acct-Output-Packets (48) | RFC 2866 S5.10 | Packets sent to subscriber |
| Acct-Input-Gigawords (52) | RFC 2869 S5.1 | Input octet counter wraps (present when >0) |
| Acct-Output-Gigawords (53) | RFC 2869 S5.2 | Output octet counter wraps (present when >0) |

<!-- source: internal/component/l2tp/plugins/authradius/acct.go -- buildAcctPacket, splitGigawords -->

### Scale Validation

Ze includes control-plane scale test infrastructure (`ze-test l2tp-scale`)
that validates 2000 concurrent L2TP sessions across 10 tunnels on loopback.
The test measures session establishment rate, RADIUS auth/accounting
handling, IP pool allocation correctness, and teardown completeness without
requiring root, kernel modules, or Docker.

<!-- source: internal/test/cli/cmd_l2tp_scale.go -- LAC simulator + mock RADIUS -->

## Where Ze is behind today

After the detail tables above: the gaps, stated plainly, not buried in a
"No" cell thirteen tables deep.

- **OSPF as PE/CE (RFC 4577) is absent** -- no DN-bit originator, no VRF and no sham link. FRR has none of it either. BIRD has the DN bit alone.
- **OSPFv3 Extended LSAs (RFC 8362) do not interoperate** -- Ze builds 3 of the 7 LSA types and sets the U-bit wrong on all of them. They stop at the first OSPFv3 router that does not support them. Neither FRR nor BIRD implements RFC 8362 at all.
- **No BGP confederations (RFC 5065)** — BIRD 3, bio-rd (partial), FRR, GoBGP, BIRD 2, and freeRtr all support it.
- **No privilege separation** — a signature feature of at least one other implementation in this table.
- **BFD integration is "Partial"** — several other implementations here have full support.
- **No embeddable library mode** — at least two other implementations in this table offer one.
- **No custom filter language** — several implementations here have their own filter DSL; Ze relies on plugin chains instead.- **No Confederation, no Multi-Topology IS-IS (RFC 5120)** — Ze's IS-IS matches the single-topology default other implementations ship, but not their optional multi-topology extension.
- **Pre-release, first release 2026** — sitting in the same table as implementations with years to decades of production hardening (one dates to 1998).
- **Performance is not yet benchmarked at scale.** Go carries an estimated 10-15% CPU overhead versus C/Rust implementations; this has not been measured under load. See [Performance Trade-offs](DESIGN.md#performance-trade-offs).

None of this is hidden in the tables above — it's restated here because a
visitor shouldn't have to hunt for it.

## FAQ

**Ze is pre-release — why should I trust it yet?**

Don't take that on faith: it's backed by 10,000+ unit tests, 1,200+ end-to-end
tests, 50+ fuzz targets, and interop testing against seven independent BGP
implementations. That's evidence you can check, not a promise. What it
doesn't have yet is operational mileage — real deployments, over real time,
on real networks. Use it in labs first.

**Why no BGP confederations yet?**

Not implemented yet. It's a real gap against implementations that have had
it for years, and it's listed as one plainly above rather than left for you
to find in a table.

**Why no custom filter language?**

Ze doesn't have a bespoke filter DSL like some implementations here do.
Instead, filters are external plugins chained per peer/group: JSON events in,
text commands out, over a TLS connect-back socket, in any language that can
read lines. That trades a purpose-built mini-language for the full power of
a real programming language -- write a filter in Go, Python, or whatever you
already know, instead of learning a new syntax.

**Is Ze's performance actually competitive with C/Rust implementations?**

Unknown at scale. The current estimate is 10-15% CPU overhead from the Go
runtime, but that number has not been benchmarked under real load. Treat it
as an open question, not a claim. See [performance.md](performance.md) for
the actual convergence and throughput numbers measured so far.

**Does Ze support everything FRR or freeRtr does?**

No. Both have broader AFI/SAFI coverage — FRR as the most feature-complete
open-source routing suite, freeRtr with the broadest coverage of any
implementation in this table, including MUP, MVPN, RTC, and VPN FlowSpec.
Ze chose depth in fewer areas (BGP, BNG, plugin architecture, YANG
configuration) over matching every implementation's full breadth on day one.
