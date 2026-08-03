# Plugin catalog

88 runtime plugins generated from `data/plugin-registry.json`, plus 6 test fixtures. 68 runtime plugins declare configuration roots and 69 ship YANG modules.

The HTML page includes browser-side search across name, purpose, config roots, dependencies, YANG files, and source directories. Clicking a plugin opens its generated local detail page.

## Anomaly

Generated group for registry entries mapped to the Anomaly area. Config roots: `anomaly`. Source area: `internal/plugins/anomaly`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`anomaly-detect-feature-source`](anomaly-detect-feature-source/index.md) | Behavioral anomaly detector (report-only): per-entity pattern-of-life over trafficfeature | `anomaly/detect` | `config-loaded` | `internal/plugins/anomaly/detect` |
| [`anomaly-shape-firewall`](anomaly-shape-firewall/index.md) | Shadow-first autonomous anomaly responder: per-source rate-limit with arm/auto-revert/kill-switch | `anomaly/shape` | `config-loaded` | `internal/plugins/anomaly/shape` |

## BFD

Generated group for registry entries mapped to the BFD area. Config roots: `bfd`. Source area: `internal/component/bfd`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`bfd`](bfd/index.md) | Bidirectional Forwarding Detection (RFC 5880, 5881, 5883) | `bfd` | None | `internal/component/bfd` |

## BGP

Generated group for registry entries mapped to the BGP area. Config roots: `bgp`, `environment`. Source area: `internal/component/bgp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`bgp`](bgp/index.md) | BGP routing daemon | `bgp` | None | `internal/component/bgp/plugin` |
| [`bgp-adj-rib-in`](bgp-adj-rib-in/index.md) | Adj-RIB-In storage (raw hex replay) | None | None | `internal/component/bgp/plugins/adj_rib_in` |
| [`bgp-aigp`](bgp-aigp/index.md) | Accumulated IGP Metric (RFC 7311) | None | None | `internal/component/bgp/plugins/aigp` |
| [`bgp-bmp`](bgp-bmp/index.md) | BMP receiver and sender (RFC 7854, 8671) | `bgp`, `environment` | None | `internal/component/bgp/plugins/bmp` |
| [`bgp-capa`](bgp-capa/index.md) | Core BGP capability decoding (multiprotocol, asn4, add-path, paths-limit, extended-nexthop, extended-message) | None | None | `internal/component/bgp/plugins/capa` |
| [`bgp-gr`](bgp-gr/index.md) | Graceful Restart capability and mechanism plugin | `bgp` | `bgp`, `bgp-rib` | `internal/component/bgp/plugins/gr` |
| [`bgp-healthcheck`](bgp-healthcheck/index.md) | Service healthcheck plugin with watchdog route control | `bgp` | `bgp`, `bgp-watchdog` | `internal/component/bgp/plugins/healthcheck` |
| [`bgp-hostname`](bgp-hostname/index.md) | FQDN capability decoding | `bgp` | `bgp` | `internal/component/bgp/plugins/hostname` |
| [`bgp-llnh`](bgp-llnh/index.md) | Link-Local Next-Hop capability plugin | `bgp` | `bgp` | `internal/component/bgp/plugins/llnh` |
| [`bgp-persist`](bgp-persist/index.md) | Route Persistence | None | None | `internal/component/bgp/plugins/persist` |
| [`bgp-rib`](bgp-rib/index.md) | Route Information Base storage | `bgp` | None | `internal/component/bgp/plugins/rib` |
| [`bgp-role`](bgp-role/index.md) | RFC 9234 BGP Role capability | `bgp` | `bgp` | `internal/component/bgp/plugins/role` |
| [`bgp-route-refresh`](bgp-route-refresh/index.md) | Route Refresh capability decoding | `bgp` | `bgp` | `internal/component/bgp/plugins/route_refresh` |
| [`bgp-rpki`](bgp-rpki/index.md) | RPKI origin validation via RTR protocol | `bgp` | `bgp`, `bgp-adj-rib-in` | `internal/component/bgp/plugins/rpki` |
| [`bgp-rpki-decorator`](bgp-rpki-decorator/index.md) | Correlates UPDATE + RPKI events into merged update-rpki events | None | `bgp`, `bgp-rpki` | `internal/component/bgp/plugins/rpki_decorator` |
| [`bgp-rr`](bgp-rr/index.md) | Route Reflector | None | `bgp-adj-rib-in` | `internal/component/bgp/plugins/rr` |
| [`bgp-rs`](bgp-rs/index.md) | Route Server | `bgp` | None | `internal/component/bgp/plugins/rs` |
| [`bgp-softver`](bgp-softver/index.md) | Software Version capability (code 75) | `bgp` | `bgp` | `internal/component/bgp/plugins/softver` |
| [`bgp-watchdog`](bgp-watchdog/index.md) | Watchdog route management plugin | `bgp` | `bgp` | `internal/component/bgp/plugins/watchdog` |

## BGP Filter

Generated group for registry entries mapped to the BGP Filter area. Config roots: `bgp`. Source area: `internal/component/bgp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`bgp-filter-aspath`](bgp-filter-aspath/index.md) | Named AS-path regex filter (ordered entries, first match wins, accept/reject) | `bgp` | `bgp` | `internal/component/bgp/plugins/filter_aspath` |
| [`bgp-filter-aspath-length`](bgp-filter-aspath-length/index.md) | Named AS-path length filter (accept/reject based on hop count) | `bgp` | `bgp` | `internal/component/bgp/plugins/filter_aspath_length` |
| [`bgp-filter-community`](bgp-filter-community/index.md) | Community tag/strip filter (standard, large, extended) | `bgp` | `bgp` | `internal/component/bgp/plugins/filter_community` |
| [`bgp-filter-community-match`](bgp-filter-community-match/index.md) | Named community match filter (ordered entries, first match wins, accept/reject) | `bgp` | `bgp` | `internal/component/bgp/plugins/filter_community_match` |
| [`bgp-filter-family`](bgp-filter-family/index.md) | Named address-family policy filter: remove a family's NLRI or tear down the session | `bgp` | `bgp` | `internal/component/bgp/plugins/filter_family` |
| [`bgp-filter-irr`](bgp-filter-irr/index.md) | IRR-based prefix-list filter for eBGP peers | `bgp` | `bgp` | `internal/component/bgp/plugins/filter_irr` |
| [`bgp-filter-modify`](bgp-filter-modify/index.md) | Named route attribute modifier (set local-preference, med, origin, next-hop) | `bgp` | `bgp` | `internal/component/bgp/plugins/filter_modify` |
| [`bgp-filter-prefix`](bgp-filter-prefix/index.md) | Named prefix-list filter (CIDR + ge/le + accept/reject) | `bgp` | `bgp` | `internal/component/bgp/plugins/filter_prefix` |
| [`bgp-filter-remove-private-as`](bgp-filter-remove-private-as/index.md) | Named AS-path action filter that removes RFC 6996 Private Use ASNs | `bgp` | `bgp` | `internal/component/bgp/plugins/filter_remove_private_as` |
| [`loop`](loop/index.md) | Route loop detection (RFC 4271 S9, RFC 4456 S8) | None | None | `internal/component/bgp/reactor/filter` |

## BGP NLRI

Generated group for registry entries mapped to the BGP NLRI area. Source area: `internal/component/bgp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`bgp-nlri-evpn`](bgp-nlri-evpn/index.md) | EVPN family plugin | None | None | `internal/component/bgp/plugins/nlri/evpn` |
| [`bgp-nlri-flowspec`](bgp-nlri-flowspec/index.md) | FlowSpec NLRI encoding/decoding | None | None | `internal/component/bgp/plugins/nlri/flowspec` |
| [`bgp-nlri-labeled`](bgp-nlri-labeled/index.md) | Labeled Unicast family plugin (RFC 8277) | None | None | `internal/component/bgp/plugins/nlri/labeled` |
| [`bgp-nlri-ls`](bgp-nlri-ls/index.md) | BGP-LS family plugin | None | None | `internal/component/bgp/plugins/nlri/ls` |
| [`bgp-nlri-mup`](bgp-nlri-mup/index.md) | Mobile User Plane family plugin (draft-mpmz-bess-mup-safi) | None | None | `internal/component/bgp/plugins/nlri/mup` |
| [`bgp-nlri-mvpn`](bgp-nlri-mvpn/index.md) | Multicast VPN family plugin (RFC 6514) | None | None | `internal/component/bgp/plugins/nlri/mvpn` |
| [`bgp-nlri-rtc`](bgp-nlri-rtc/index.md) | Route Target Constraint family plugin (RFC 4684) | None | None | `internal/component/bgp/plugins/nlri/rtc` |
| [`bgp-nlri-srpolicy`](bgp-nlri-srpolicy/index.md) | SR-Policy family plugin (RFC 9830, SAFI 73) | None | None | `internal/component/bgp/plugins/nlri/srpolicy` |
| [`bgp-nlri-vpls`](bgp-nlri-vpls/index.md) | VPLS family plugin (RFC 4761) | None | None | `internal/component/bgp/plugins/nlri/vpls` |
| [`bgp-nlri-vpn`](bgp-nlri-vpn/index.md) | VPN family plugin | None | None | `internal/component/bgp/plugins/nlri/vpn` |

## BGP Redistribute

Generated group for registry entries mapped to the BGP Redistribute area. Config roots: `redistribute`. Source area: `internal/component/bgp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`bgp-redistribute`](bgp-redistribute/index.md) | Route redistribution ingress filter with loop prevention and family filtering | None | `bgp` | `internal/component/bgp/plugins/redistribute_ingress` |
| [`redistribute-orchestrator`](redistribute-orchestrator/index.md) | Redistribute orchestrator: dispatches protocol route events to registered consumers | `redistribute` | `bgp` | `internal/component/bgp/plugins/redistribute_egress` |

## Class of Service

Generated group for registry entries mapped to the Class of Service area. Config roots: `class-of-service`. Source area: `internal/plugins/cos`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`cos`](cos/index.md) | 802.1p class-of-service profile definitions | `class-of-service` | None | `internal/plugins/cos` |

## Connected

Generated group for registry entries mapped to the Connected area. Config roots: `connected`. Source area: `internal/plugins/connected`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`connected`](connected/index.md) | Connected routes: redistribute directly connected interface prefixes | `connected` | None | `internal/plugins/connected` |

## Control Plane Protection

Generated group for registry entries mapped to the Control Plane Protection area. Config roots: `control-plane-protection`. Source area: `internal/plugins/copp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`copp-input-chain`](copp-input-chain/index.md) | Control-plane policing: rate-limit new TCP connections to BGP listen port | `control-plane-protection` | `firewall` | `internal/plugins/copp` |

## DDoS

Generated group for registry entries mapped to the DDoS area. Config roots: `ddos`. Source area: `internal/plugins/ddos`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`ddos-detect-flow-source`](ddos-detect-flow-source/index.md) | Automatic DDoS attack detector with two-stage detection | `ddos/detect` | `config-loaded` | `internal/plugins/ddos/detect` |
| [`ddos-flowspec`](ddos-flowspec/index.md) | DDoS FlowSpec/RTBH responder: upstream mitigation with leak-probe clear | `ddos/flowspec` | None | `internal/plugins/ddos/flowspec` |
| [`ddos-flowtriq`](ddos-flowtriq/index.md) | DDoS incident reporter for Flowtriq cloud API | `ddos/flowtriq` | None | `internal/plugins/ddos/flowtriq` |
| [`ddos-local`](ddos-local/index.md) | DDoS local responder: on-host nft drop on attack detection | `ddos/local` | `firewall` | `internal/plugins/ddos/local` |
| [`ddos-observe`](ddos-observe/index.md) | DDoS observability: incident store and show ddos status/incidents CLI | `ddos/observe` | None | `internal/plugins/ddos/observe` |

## Environment

Generated group for registry entries mapped to the Environment area. Config roots: `environment`. Source area: `internal/plugins/ntp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`ntp`](ntp/index.md) | NTP client: system clock synchronization | `environment` | None | `internal/plugins/ntp` |

## Exabgp

Generated group for registry entries mapped to the Exabgp area. Config roots: `exabgp`. Source area: `internal/plugins/exabgp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`exabgp-bridge`](exabgp-bridge/index.md) | In-process ExaBGP compatibility bridge: runs an operator ExaBGP-format script as a subprocess and translates to/from ze events (RFC-agnostic transport shim) | `exabgp` | None | `internal/plugins/exabgp/bridgeplugin` |

## FIB

Generated group for registry entries mapped to the FIB area. Config roots: `fib`. Source area: `internal/plugins/fib`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`fib-kernel`](fib-kernel/index.md) | FIB kernel: programs OS routes from system RIB via netlink/route socket | `fib/kernel` | `rib`, `sysctl` | `internal/plugins/fib/kernel` |
| [`fib-p4`](fib-p4/index.md) | FIB P4: programs P4 switch forwarding entries from system RIB via gRPC/P4Runtime | `fib/p4` | `rib` | `internal/plugins/fib/p4` |
| [`fib-vpp`](fib-vpp/index.md) | FIB VPP: programs VPP FIB entries from system RIB via GoVPP binary API | `fib/vpp` | `rib`, `vpp` | `internal/plugins/fib/vpp` |

## Firewall

Generated group for registry entries mapped to the Firewall area. Config roots: `firewall`. Source area: `internal/component/firewall`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`firewall`](firewall/index.md) | Packet filter and NAT rules (nftables on Linux) | `firewall` | None | `internal/component/firewall` |
| [`firewall-irr`](firewall-irr/index.md) | IRR-based prefix-list filtering for firewall rules | `firewall` | `firewall` | `internal/component/firewall/plugins/irr` |

## Flow Export

Generated group for registry entries mapped to the Flow Export area. Config roots: `flow-export`. Source area: `internal/plugins/flowexport`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`flow-export-conntrack-tracking`](flow-export-conntrack-tracking/index.md) | sFlow, NetFlow v9, and IPFIX counter export | `flow-export` | `config-loaded` | `internal/plugins/flowexport` |

## Flowspec Firewall

Generated group for registry entries mapped to the Flowspec Firewall area. Source area: `internal/plugins/flowspec-firewall`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`flowspec-firewall`](flowspec-firewall/index.md) | Translates BGP FlowSpec routes into nftables firewall rules | None | `firewall` | `internal/plugins/flowspec-firewall` |

## IS-IS

Generated group for registry entries mapped to the IS-IS area. Config roots: `isis`. Source area: `internal/plugins/isis`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`isis`](isis/index.md) | Intermediate System to Intermediate System (ISO/IEC 10589, RFC 1195): native link-state IGP | `isis` | `fib-kernel`, `sysctl` | `internal/plugins/isis` |

## Interface

Generated group for registry entries mapped to the Interface area. Config roots: `interface`. Source area: `internal/component/iface`, `internal/plugins/iface`, `internal/plugins/vrrp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`iface-dhcp`](iface-dhcp/index.md) | DHCP client: DHCPv4/DHCPv6 lease acquisition and renewal | None | `interface` | `internal/plugins/iface/dhcp` |
| [`interface`](interface/index.md) | OS network interface monitoring and management | `interface` | `sysctl` | `internal/component/iface` |
| [`vrrp`](vrrp/index.md) | Virtual Router Redundancy Protocol (RFC 9568 / RFC 3768): first-hop gateway redundancy | `interface` | `interface` | `internal/plugins/vrrp` |

## Kernel

Generated group for registry entries mapped to the Kernel area. Config roots: `kernel`. Source area: `internal/plugins/kernel`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`kernel`](kernel/index.md) | Kernel routes: redistribute externally-installed kernel routes into BGP | `kernel` | None | `internal/plugins/kernel` |

## L2TP

Generated group for registry entries mapped to the L2TP area. Config roots: `l2tp`. Source area: `internal/component/l2tp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`l2tp-auth-local`](l2tp-auth-local/index.md) | Static local user list for L2TP PPP authentication | `l2tp` | None | `internal/component/l2tp/plugins/authlocal` |
| [`l2tp-auth-radius-servers`](l2tp-auth-radius-servers/index.md) | RADIUS authentication and accounting for L2TP PPP sessions | `l2tp` | `radius-server` | `internal/component/l2tp/plugins/authradius` |
| [`l2tp-pool`](l2tp-pool/index.md) | IPv4 address and IPv6 prefix pool for L2TP PPP sessions | `l2tp` | None | `internal/component/l2tp/plugins/pool` |
| [`l2tp-shaper`](l2tp-shaper/index.md) | Traffic shaping for L2TP subscriber sessions | `l2tp` | None | `internal/component/l2tp/plugins/shaper` |

## LDP

Generated group for registry entries mapped to the LDP area. Config roots: `ldp`. Source area: `internal/plugins/ldp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`ldp-port`](ldp-port/index.md) | Label Distribution Protocol (RFC 5036): MPLS label distribution | `ldp` | `fib-kernel` | `internal/plugins/ldp` |

## MRT

Generated group for registry entries mapped to the MRT area. Config roots: `mrt`. Source area: `internal/plugins/mrt`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`mrt`](mrt/index.md) | MRT routing information export (RFC 6396) | `mrt` | None | `internal/plugins/mrt` |

## OSPF

Generated group for registry entries mapped to the OSPF area. Config roots: `ospf`. Source area: `internal/plugins/ospf`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`ospf`](ospf/index.md) | Open Shortest Path First v2 (RFC 2328): native link-state IPv4 IGP | `ospf` | `interface`, `fib-kernel`, `sysctl` | `internal/plugins/ospf` |

## Policy

Generated group for registry entries mapped to the Policy area. Config roots: `policy`. Source area: `internal/plugins/policyroute`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`policy-routes`](policy-routes/index.md) | Policy-based routing: nftables packet marking and ip rule table selection | `policy` | `firewall` | `internal/plugins/policyroute` |

## RIB

Generated group for registry entries mapped to the RIB area. Config roots: `rib`. Source area: `internal/component/sysrib`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`rib`](rib/index.md) | System RIB: selects best route across protocols by admin distance | `rib` | None | `internal/component/sysrib` |

## RSVP TE

Generated group for registry entries mapped to the RSVP TE area. Config roots: `rsvp-te`. Source area: `internal/plugins/rsvpte`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`rsvp-te-rawsock`](rsvp-te-rawsock/index.md) | RSVP-TE: Resource Reservation Protocol - Traffic Engineering (RFC 3209) | `rsvp-te` | `fib-kernel` | `internal/plugins/rsvpte` |

## Routing Table

Generated group for registry entries mapped to the Routing Table area. Config roots: `routing-table`. Source area: `internal/plugins/routingtable`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`routing-table`](routing-table/index.md) | Named routing table registry: maps names to kernel table IDs | `routing-table` | None | `internal/plugins/routingtable` |

## Service

Generated group for registry entries mapped to the Service area. Config roots: `service`. Source area: `internal/plugins/as112`, `internal/plugins/dhcpserver`, `internal/plugins/geodns`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`as112`](as112/index.md) | AS112 anycast DNS node: authoritative sink for misdirected RFC 1918 / link-local reverse-DNS queries (RFC 7534, RFC 7535) | `service` | None | `internal/plugins/as112` |
| [`dhcpserver`](dhcpserver/index.md) | DHCP server: address assignment for LAN clients (RFC 2131) | `service` | None | `internal/plugins/dhcpserver` |
| [`geodns`](geodns/index.md) | GeoDNS server: DNS answers selected by client source IP (RFC 1035, RFC 7871 client-subnet) | `service` | None | `internal/plugins/geodns` |
| [`imageserver`](imageserver/index.md) | Image server: HTTP provisioning for disk images and boot files | `service` | None | `internal/plugins/imageserver` |
| [`tftpserver`](tftpserver/index.md) | TFTP server: read-only file serving for PXE boot (RFC 1350, RFC 2347 option negotiation) | `service` | None | `internal/plugins/tftpserver` |

## Static

Generated group for registry entries mapped to the Static area. Config roots: `static`. Source area: `internal/plugins/static`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`static`](static/index.md) | Static routes: config-driven kernel/VPP route programming with ECMP | `static` | `routing-table` | `internal/plugins/static` |

## Sysctl

Generated group for registry entries mapped to the Sysctl area. Config roots: `sysctl`. Source area: `internal/component/sysctl`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`sysctl`](sysctl/index.md) | Kernel tunable management: three-layer precedence, restore on stop | `sysctl` | None | `internal/component/sysctl` |

## Traffic

Generated group for registry entries mapped to the Traffic area. Config roots: `traffic`. Source area: `internal/component/traffic`, `internal/plugins/trafficusage`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`traffic`](traffic/index.md) | Traffic control (tc) qdisc, class, and filter management | `traffic/control` | None | `internal/component/traffic` |
| [`traffic-usage`](traffic-usage/index.md) | eBPF TCX per-port and per-IP byte accounting | `traffic/usage` | `interface` | `internal/plugins/trafficusage` |

## VPN

Generated group for registry entries mapped to the VPN area. Config roots: `pki`, `vpn`. Source area: `internal/component/ike`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`ipsec-cookie-threshold`](ipsec-cookie-threshold/index.md) | IKEv2 engine for native IPsec VPN | `vpn`, `pki` | `config-loaded` | `internal/component/ike/engine` |

## VPP

Generated group for registry entries mapped to the VPP area. Config roots: `vpp`. Source area: `internal/component/vpp`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`vpp`](vpp/index.md) | VPP data plane lifecycle management | `vpp` | None | `internal/component/vpp` |

## Test Harness

Generated group for registry entries mapped to the Test Harness area. Config roots: `ddos`. Source area: `internal/test/plugins`.

| Plugin | Used for | Config | Depends on | Source path |
|--------|----------|--------|------------|-------------|
| [`ddos-fake`](ddos-fake/index.md) | Test-only synthetic DDoS attack injector for the ddos-local withdraw test (harmless unless `ddos { fake { enabled true; } }` is configured) | `ddos/fake` | `firewall` | `internal/test/plugins/fakeddos` |
| [`fakeas112`](fakeas112/index.md) | Test-only synthetic AS112 route producer (use ze.fakeas112; harmless when not invoked) | None | None | `internal/test/plugins/fakeas112` |
| [`fakeenrich`](fakeenrich/index.md) | Test-only in-process enricher (harmless when not invoked) | None | None | `internal/test/plugins/fakeenrich` |
| [`fakefib`](fakefib/index.md) | Test-only sysrib event emitter for FIB functional tests (use ze.fakefib) | None | None | `internal/test/plugins/fakefib` |
| [`fakel2tp`](fakel2tp/index.md) | Test-only synthetic L2TP route producer (use ze.fakel2tp; harmless when not invoked) | None | None | `internal/test/plugins/fakel2tp` |
| [`fakeredist`](fakeredist/index.md) | Test-only synthetic route producer (use ze.fakeredist; harmless when not invoked) | None | None | `internal/test/plugins/fakeredist` |
