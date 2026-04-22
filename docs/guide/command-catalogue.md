# Operational Command Catalogue

Cross-vendor reference for operational CLI commands. The goal is to
track, per command, which vendors offer it, whether ze has it today,
and which backend work is needed to offer it. As backends mature
(nftables, VPP classifier, VPP FIB dump, neighbor dump, etc.),
commands move from `planned` to `shipped` without a fresh design pass.

This is a roadmap document, not a user reference. For what ze ships
today, see `docs/guide/command-reference.md`.

## How to read this

### Legend -- ze status

| Marker | Meaning |
|--------|---------|
| shipped | Implemented on every backend required |
| partial | Implemented on some backends; others reject or miss variants |
| wired   | Handler and YANG exist; at least one backend rejects (exact-or-reject) |
| planned | Not yet started; on roadmap when listed backend lands |
| scope   | Out of scope for ze today (e.g., OSPF, cluster, PPPoE client) |

### Legend -- vendors

Column headers refer to: VyOS 1.5+ (the op-mode XML tree),
Junos (operational mode, any recent release), Nokia SR OS (classic CLI),
Arista EOS, FRR (for BGP comparison). An entry of `-` means the vendor
does not expose an equivalent command; `~` means it exists but in a
shape different enough that parity is not automatic.

### Legend -- backend requirements

| Tag | What ze needs |
|-----|---------------|
| nl | netlink read / write |
| nft | nftables backend (already shipped) |
| vpp-if | GoVPP `sw_interface_*` (already shipped) |
| vpp-fib | GoVPP `ip_route_v2_dump` / `ip_route_add_del` |
| vpp-nbr | GoVPP `ip_neighbor_dump` |
| vpp-acl | GoVPP ACL/classifier plugin |
| vpp-stats | VPP stat segment |
| bgp | BGP reactor only; no OS dependency |
| config | Parsed YANG config only |
| process | Daemon process metadata |
| shell | `exec.Command` wrapping OS tool (ping, traceroute, tcpdump) |

### How to update

When a command lands on a new backend, update its row. When a new
command is requested, add a row in the appropriate section. Keep the
table alphabetised inside each section. New vendor columns MAY be
added if a reader needs them (Cisco IOS-XR, SONiC, OpenBSD relayd);
do it per-section, not globally.

## Naming convention

Ze commands are **domain-first, verb-second**. The domain (subsystem,
protocol, object) names itself; the verb says what to do with it. This
matches how operators think: "what am I working on, then what am I
doing to it."

| Form | Example | When to use |
|------|---------|-------------|
| `<domain> <verb>` | `peer list`, `rib inject`, `commit start`, `bgp summary`, `log set`, `rpki status` | State that belongs to a specific subsystem |
| `<domain> <selector> <verb>` | `peer <sel> teardown`, `peer <sel> pause` | Operations targeting one or more instances within a subsystem |
| `<domain> <verb> <object>` | `config rollback <n>` | Domain operations that produce or consume a named object |
| `show <what>` | `show warnings`, `show errors`, `show interface`, `show version`, `show system memory` | Cross-domain read-only introspection |
| `generate <what>` | `generate wireguard keypair`, `generate tech-support archive` (planned) | Produce a new artifact from local state (keys, certs, bundles) |
| `<bare verb>` | `ping`, `traceroute`, `help` | Universal diagnostics / reserved verbs |

**Why this shape.** Domain-first keeps the completion tree shallow:
once you type `bgp`, the valid next tokens are exactly the verbs BGP
offers. Two verbs stay reserved at the root because every vendor uses
them and operators reach for them by muscle memory:

- `show` — universal read-only introspection. Groups by topic at the
  second position (`show system`, `show interface`, `show bgp`), which
  gives back the domain-first grouping one level down.
- `generate` — universal "produce a new artifact" verb. Covers
  keypairs, certificate signing requests, tech-support archives,
  anything where the output is a new file or blob. Same as `show`,
  the second position is a topic (`generate wireguard keypair`,
  `generate tech-support archive`).

Every other verb (`clear`, `reset`, `restart`, `flush`, `inject`,
`withdraw`, `archive`, `rollback`, `teardown`, `pause`, `resume`,
`commit`, `validate`, `fmt`) is second-position, after the domain.

**Rules:**

- The domain name is a subsystem (`bgp`, `peer`, `rib`, `commit`,
  `rpki`, `bmp`, `log`, `cache`, `healthcheck`, `metrics`,
  `wireguard`, `firewall`, `config`, ...). It is NOT a generic verb,
  EXCEPT for the two reserved root verbs (`show`, `generate`).
- Verbs are lowercase, single-word when possible (`list`, `show`,
  `start`, `stop`, `inject`, `withdraw`, `teardown`, `pause`,
  `resume`, `flush`, `reset`, `generate`, `archive`, `rollback`,
  `fmt`, `validate`).
- Selectors come between domain and verb when they target a single
  instance (`peer <addr> teardown`), not after the verb. This reads as
  "this peer, teardown."
- Reserved-verb forms (`show X`, `generate X`) are the only verb-first
  shapes. Everything else goes domain-first.
- Universal diagnostics (`ping`, `traceroute`, `help`, `command-list`)
  stay bare. They are not subsystem operations.

**Examples of conversion from vendor forms:**

| VyOS / Junos / Nokia | Ze form |
|----------------------|---------|
| `generate wireguard keypair` | `generate wireguard keypair` (already domain-after-verb, kept) |
| `reset ip bgp <addr>` | `peer <addr> teardown` |
| `clear ip bgp <addr> soft in` | `route-refresh <family>` (today) |
| `clear interfaces counters eth0` | `clear interface eth0 counters` (shipped; `clear` verb added 2026-04-18) |
| `show configuration` | `config dump` |
| `request security pki ...` | `pki <verb> ...` (planned) |

When adding a row to the catalogue, the Ze-command cell must follow
this shape. If a vendor command does not fit (rare), state the reason
in the Notes column.

---

## 1. BGP and Routing Policy

BGP operational visibility. Ze is BGP-first; this table should be
close to fully shipped.

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| BGP summary | `show ip bgp summary` | `show bgp summary` | `show router bgp summary` | `show ip bgp summary` | `show ip bgp summary` | `bgp summary` | shipped | bgp | |
| BGP summary per-family | `show ip bgp ipv4/ipv6 summary` | `show bgp summary family inet` | `show router bgp summary family ...` | `show bgp ipv4/ipv6 summary` | `show bgp ipv4/ipv6 summary` | `bgp summary <afi/safi>` | shipped | bgp | One handler (`ze-bgp:summary`) branches on argv; shorthands `ipv4`, `ipv6`, `l2vpn` expand to `/unicast` (or `/evpn`); unknown families rejected with the list actually negotiated on this daemon; length+charset guard on the argument |
| Peer list brief | `show ip bgp neighbors` | `show bgp neighbor brief` | `show router bgp neighbor` | `show ip bgp neighbors` | `show bgp neighbors` | `peer list` | shipped | bgp | |
| Peer detail | `show ip bgp neighbors <addr>` | `show bgp neighbor <addr> extensive` | `show router bgp neighbor <addr> detail` | `show ip bgp neighbors <addr>` | `show bgp neighbors <addr>` | `peer <sel> detail` | shipped | bgp | |
| Peer negotiated capabilities | `show ip bgp neighbors <addr> received-capabilities` | included in detail | included in detail | `show ip bgp neighbors <addr> capabilities` | `show bgp neighbors <addr>` | `peer <sel> capabilities` | shipped | bgp | |
| Adj-RIB-In per peer | `show ip bgp neighbors <addr> received-routes` | `show route receive-protocol bgp <addr>` | `show router bgp neighbor <addr> received-routes` | `show ip bgp neighbors <addr> received-routes` | `show bgp ipv4 unicast neighbors <addr> received-routes` | `rib routes received <peer>` | shipped | bgp | |
| Adj-RIB-Out per peer | `show ip bgp neighbors <addr> advertised-routes` | `show route advertising-protocol bgp <addr>` | `show router bgp neighbor <addr> advertised-routes` | `show ip bgp neighbors <addr> advertised-routes` | `show bgp ipv4 unicast neighbors <addr> advertised-routes` | `rib routes sent <peer>` | shipped | bgp | |
| BGP best RIB | `show ip bgp` | `show route protocol bgp` | `show router bgp routes` | `show ip bgp` | `show ip bgp` | `rib show best` | shipped | bgp | |
| BGP route for prefix | `show ip bgp <prefix>` | `show route <prefix> protocol bgp` | `show router route-table <prefix>` | `show ip bgp <prefix>` | `show ip bgp <prefix>` | `rib show best` without prefix filter | partial | bgp | Add `rib show best <prefix>` |
| BGP route-map / policy view | `show policy route-map` | `show policy <name>` | `show router policy <name>` | `show route-map <name>` | `show route-map <name>` | | planned | config+bgp | Requires policy introspection API |
| BGP communities | `show ip bgp community <community>` | `show route community <community>` | `show router bgp community <community>` | `show ip bgp community <community>` | `show ip bgp community <community>` | | planned | bgp | |
| BGP as-path filter view | ~ | `show route aspath-regex <regex>` | `show router bgp as-path-list` | `show ip bgp regexp <regex>` | `show ip bgp regexp <regex>` | | planned | bgp | |
| BGP large-communities | - | `show route large-community` | ~ | `show ip bgp large-community` | `show ip bgp large-community` | | planned | bgp | |
| BGP memory / attr pool stats | - | `show bgp summary` (partial) | `show router bgp statistics` | ~ | `show bgp memory` | `rib status` counters | shipped | bgp | Ze reports per-attr pool dedup |
| BGP update groups | - | `show bgp group` | `show router bgp group` | `show ip bgp peer-group` | `show bgp peer-group` | | planned | bgp | |
| BGP graceful restart state | ~ | `show bgp neighbor` detail | `show router bgp graceful-restart` | `show ip bgp neighbors <addr>` detail | `show bgp neighbors <addr>` | inside peer detail, gr plugin | shipped | bgp | |
| BGP LLGR state | - | ~ | - | - | `show bgp ipv4 unicast` (long-lived) | bgp-gr plugin | shipped | bgp | |
| BGP route-refresh send | `reset ip bgp <addr>` | `clear bgp neighbor <addr> soft-inbound` | `clear router bgp neighbor <addr> soft-inbound` | `clear ip bgp <addr> soft in` | `clear ip bgp <addr> soft in` | `route-refresh <family>` | shipped | bgp | |
| BGP hard reset | `reset ip bgp <addr>` | `clear bgp neighbor <addr>` | `clear router bgp neighbor <addr>` | `clear ip bgp <addr>` | `clear ip bgp <addr>` | `peer <sel> teardown` | shipped | bgp | |
| BGP pause/resume peer | - | - | `tools perform router bgp neighbor ... enable/disable` | ~ | - | `peer <sel> pause/resume` | shipped | bgp | Ze-unique (flow control) |
| BGP inject route | ~ (conf-set only) | - | - | - | static + redist | `rib inject`, `peer ... update text` | shipped | bgp | Ze-unique (test tool) |
| BGP update dump (live) | - | `monitor traffic protocol bgp` | `debug router bgp peer ... events` | - | `debug bgp updates` | `bgp monitor` | shipped | bgp | SSE stream |
| BGP monitoring protocol (BMP) | ~ | ~ | `show router bmp` | `show bgp bmp` | `show bmp` | `bmp sessions/peers/collectors` | shipped | bgp | |
| BGP FlowSpec rules | - | `show firewall filter detail` | `show filter ip-filter` | - | `show bgp ipv4 flowspec detailed` | installed via fibkernel; no dedicated view | partial | bgp+nl | Add `show bgp flowspec` |
| BGP ASPA verification | - | - | - | - | ~ | | planned | bgp | spec-bgp-2-aspa |
| BGP AIGP | - | - | - | - | ~ | | planned | bgp | spec-bgp-3-aigp |
| BGP RPKI cache state | `show rpki cache` | `show validation session` | `show router rpki session` | `show rpki cache-connection` | `show rpki cache` | `rpki status/cache/roa` | shipped | bgp | |

## 2. Routing Table / FIB

Global and per-VRF routing table visibility. Ze's current RIB view is
internal; kernel / VPP FIB view is not yet exposed.

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| IPv4 routing table | `show ip route` | `show route` | `show router route-table` | `show ip route` | `show ip route` | `show ip route` | shipped | nl+vpp-if | Kernel FIB view via netlink RouteList; VPP rejects (kernel FIB not authoritative under VPP). Covers IPv4+IPv6 in one call. `--limit N` caps response (default 100 000 rows). |
| IPv6 routing table | `show ipv6 route` | `show route table inet6.0` | `show router route-table ipv6` | `show ipv6 route` | `show ipv6 route` | | planned | nl+vpp-fib | |
| Route for prefix | `show ip route <prefix>` | `show route <prefix>` | `show router route-table <prefix>` | `show ip route <prefix>` | `show ip route <prefix>` | `show ip route <prefix>` | shipped | nl+vpp-if | Exact-match filter; `default` matches 0.0.0.0/0 and ::/0; invalid CIDRs reject |
| Route by protocol | `show ip route bgp` / `static` / `connected` | `show route protocol <proto>` | `show router route-table protocol bgp` | `show ip route bgp` | `show ip route bgp` | | planned | nl+vpp-fib | RTPROT filter |
| Route summary counts | `show ip route summary` | `show route summary` | `show router route-table summary` | `show ip route summary` | `show ip route summary` | | planned | nl+vpp-fib | |
| FIB (forwarding) table | ~ | `show route forwarding-table` | `show router fib` | `show ip route vrf` (close) | `show fib` | | planned | nl+vpp-fib | Separates RIB from FIB |
| Static routes installed | `show ip route static` | `show route protocol static` | `show router static-route` | `show ip route static` | `show ip route static` | | planned | nl+vpp-fib | Phase 2 of static-routes spec |
| VRF route table | `show ip route vrf <name>` | `show route table <vrf>.inet.0` | `show router <vrf> route-table` | `show ip route vrf <name>` | `show ip route vrf <name>` | | scope | - | VRF not yet shipped |
| Kernel-programmed (by ze) | ~ | - | - | - | `show ip route bgp` | fib-kernel `showInstalled` | shipped | nl | |

## 3. Interfaces

Interface state, counters, addressing, type-specific details.

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| List all interfaces | `show interfaces` | `show interfaces terse` | `show router interface` | `show interfaces` | `show interface brief` | `show interface brief` | shipped | nl+vpp-if | |
| Interface detail | `show interfaces <name>` | `show interfaces <name> extensive` | `show router interface <name> detail` | `show interfaces <name>` | `show interface <name>` | `show interface <name>` | shipped | nl+vpp-if | |
| Filter by type | `show interfaces ethernet` / `bridge` / `vxlan` / ... | `show interfaces terse | match <type>` | ~ | `show interfaces type ...` | - | `show interface type <type>` | shipped | nl+vpp-if | |
| Filter by error counters | `show interfaces counters error` (dup below) | `show interfaces extensive | match errors` | - | - | - | `show interface errors` | shipped | nl+vpp-if | |
| Interface counters | `show interfaces counters` | inside extensive | `show port statistics` | `show interfaces counters` | `show interface counters` | part of `show interface` | shipped | nl+vpp-if | |
| Counter reset | `clear interfaces counters [name]` | `clear interfaces statistics [name]` | `clear router statistics` | `clear counters [name]` | `clear counters [name]` | `clear interface [<name>] counters` | shipped | nl+vpp-if | Linux netlink has no generic reset -- baseline-delta fallback stored in iface component; VPP real reset pending sw_interface_clear_stats |
| Interface IP addresses | `show interfaces ethernet <name> address` | shown in terse | `show router interface <name> address` | `show ip interface` | `show ip interface` | `show interface <name>` | shipped | nl+vpp-if | |
| Interface MAC table (bridge) | `show bridge <br>` | `show ethernet-switching table` | `show service fdb` | `show mac address-table` | `show bridge` | | planned | nl+vpp-if | Requires bridge FDB dump |
| Interface errors only | `show interfaces counters error` | `show interfaces extensive | match errors` | `show port statistics error` | `show interfaces counters errors` | `show interface errors` | | planned | nl+vpp-if | Filter existing counters |
| LLDP neighbors | `show lldp neighbors` | `show lldp neighbors` | `show system lldp neighbor` | `show lldp neighbors` | - | | scope | - | No LLDP subsystem |
| Interface transceivers / optics | `show interfaces ethernet <n> physical` | `show interfaces diagnostics optics <n>` | `show port <n> detail` | `show interfaces transceiver` | - | | scope | - | Hardware-specific |
| Interface flap history | ~ | `show interfaces <n> | find flapped` | `show port history` | `show interfaces status` | - | | planned | nl | Event bus already tracks |
| Bond / LAG state | `show interfaces bonding <b>` | `show interfaces ae<n>` | `show lag <n>` | `show port-channel <n>` | `show interface bond<n>` | | partial | nl+vpp-if | |
| Bridge state | `show interfaces bridge <br>` | `show bridge` | `show service fdb` | `show bridge` | `show bridge` | | partial | nl+vpp-if | |
| VLAN interfaces | `show interfaces vif <v>` | `show interfaces irb` | `show router interface tag` | `show vlan` | `show vlan` | | partial | nl+vpp-if | |
| VXLAN interfaces | `show interfaces vxlan <v>` | `show interfaces vtep` | `show service vxlan` | `show vxlan interface` | `show vxlan` | | planned | nl+vpp-if | |
| WireGuard interfaces | `show interfaces wireguard <w>` | - | - | - | - | | partial | nl | Config-driven today |
| GRE / IP-in-IP tunnels | `show interfaces tunnel <t>` | `show interfaces gr-0/0/0` | `show router interface tunnel` | `show interfaces tunnel` | - | | planned | nl+vpp-if | spec-iface-tunnel |
| PPPoE client | `show interfaces pppoe` | `show interfaces pp<n>` | `show subscriber-mgmt ppp` | - | - | | scope | - | L2TP spec covers LNS; no PPPoE client |
| L2TPv3 tunnel | `show interfaces l2tpv3` | `show services l2tp` | `show l2tp tunnel` | - | - | | partial | config+nl | spec-l2tp-* |
| SSTPC | `show interfaces sstpc` | - | - | - | - | | scope | - | |
| Wireless | `show interfaces wireless` | - | - | - | - | | scope | - | |
| Macsec | `show interfaces macsec` | - | - | - | - | | scope | - | |

## 4. Neighbor / ARP / ND

ARP (v4) and NDP (v6) neighbor cache inspection and manipulation.

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| IPv4 ARP table | `show ip arp` | `show arp` | `show router arp` | `show arp` | `show arp` | `show ip arp` | shipped | nl+vpp-if | Returns IPv4 ARP + IPv6 ND; `--family ipv4\|ipv6` narrows; unknown positional args reject. VPP rejects pending ip_neighbor_dump wiring. |
| IPv6 neighbor table | `show ipv6 neighbors` | `show ipv6 neighbors` | `show router neighbor` | `show ipv6 neighbors` | `show ipv6 neighbors` | | planned | nl+vpp-nbr | |
| ARP flush per entry | `force arp interface <i> address <ip>` | `clear arp hostname <ip>` | `clear router arp <ip>` | `clear arp <ip>` | `clear arp <ip>` | | planned | nl+vpp-nbr | |
| ARP flush all | `clear arp` | `clear arp` | `clear router arp` | `clear arp` | `clear arp` | | planned | nl+vpp-nbr | |
| IPv6 ND flush | `force ipv6-nd interface <i>` | `clear ipv6 neighbor` | `clear router neighbor` | `clear ipv6 neighbor` | `clear ipv6 neighbor` | | planned | nl+vpp-nbr | |
| Static ARP install | `set protocols static arp` (config) | `set security arp static` (config) | `set router arp static-arp` (config) | `set arp static` | config | config | config-only; no op command |

## 5. Firewall / Filters / ACLs

Stateful filter and ACL visibility.

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| Show firewall ruleset | `show firewall name <n>` | `show firewall filter <n>` | `show filter ip-filter <n>` | `show ip access-lists <n>` | - | `show firewall ruleset <n>` | shipped | nft (shipped) / vpp-acl (planned) | Joins applied desired state with per-rule counters from kernel; every rule auto-carries an anonymous counter + term name via `Rule.UserData`. VPP-active rejects under exact-or-reject. |
| Show firewall counters | `show firewall statistics` | `show firewall counter <c>` | `show filter ip-filter <n> counters` | `show ip access-lists <n>` | - | part of `show firewall ruleset <n>` | shipped | nft | Counters returned inline per-term in the ruleset response |
| Show firewall groups (address / network / port) | `show firewall group <g>` | `show security address-book <g>` | `show service l3-vprn <svc> interface prefix-list` | - | - | `show firewall group [<g>]` | shipped | config | Reads from applied `firewall.Set` snapshot; bare form lists known group names, named form returns members. Group typing still derives from `SetType` (ipv4, ipv6, inet-service, ifname, ...); purpose-specific address-/network-/port-group YANG is a future refinement. |
| Active conntrack / flow table | `show conntrack` / `show conntrack table` | `show security flow session` | `show filter ip-filter <n> session` | `show flow-tracker hardware` | - | | planned | nl(conntrack) / vpp-stats | |
| NAT rules view | `show nat source rules` / `destination rules` | `show security nat source rule all` | `show service nat` | `show ip nat translations` | - | | planned | nft / vpp-acl | |
| NAT translations / pool | `show nat source translations` | `show security nat source pool all` | `show service nat pool` | `show ip nat translations` | - | | planned | nft / vpp-acl | |
| Drop a conntrack entry | `reset conntrack-sync entry` | `clear security flow session` | - | `clear flow` | - | | planned | nl(conntrack) | |
| Firewall ruleset resequence | `generate firewall rule-resequence <n>` | ~ | ~ | ~ | - | | scope | - | Config management convenience |
| Policy-based routing view | `show policy route-map <name>` | `show policy <name>` | `show router policy-options` | `show route-map <name>` | `show route-map <name>` | | planned | config | spec-policy-routing |
| QoS / traffic-control view | `show queueing interface <i>` | `show class-of-service interface <i>` | `show qos egress-queue-scheduler <i>` | `show qos interfaces <i>` | - | | planned | nl(tc) / vpp-stats | `tc`/VPP policer |

## 6. VPN / Tunnel / Services

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| IPsec tunnel state | `show vpn ipsec sa` | `show security ipsec security-associations` | `show ipsec security-association` | `show ip security ipsec` | - | | scope | - | |
| OpenVPN status | `show openvpn server-status` | - | - | - | - | | scope | - | |
| WireGuard peer status | `show interfaces wireguard` | - | - | - | - | | planned | nl | Once WG monitor lands |
| L2TP tunnel / session | `show interfaces l2tpv3` | `show services l2tp tunnel` | `show l2tp tunnel` | - | - | | shipped | config+nl | `show l2tp tunnels/sessions`, `clear l2tp tunnel/session teardown` |
| L2TP redistribute status | - | - | - | - | - | ze-specific; L2TP session to BGP | shipped | bgp+config | `redistribute { import l2tp }` |
| L2TP web UI / CQM | - | - | - | - | - | ze-specific; Firebrick-style CQM | shipped | web | `/l2tp` session list, detail, uPlot CQM graph, SSE live updates |
| L2TP RADIUS auth/acct | - | - | - | - | - | ze-specific; RFC 2865/2866/5176 | shipped | plugin | `l2tp-auth-radius` plugin with CoA/DM |
| PPPoE subscriber list | `show pppoe-server sessions` | `show subscribers` | `show subscriber-mgmt subscriber` | - | - | | scope | - | |
| SSL VPN | - | `show security ike security-associations` | - | - | - | | scope | - | |

## 7. Protocol Stacks Beyond BGP

Everything except BGP. All out of scope until the corresponding
subsystem lands.

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| OSPFv2 | `show ip ospf` | `show ospf *` | `show router ospf *` | `show ip ospf` | `show ip ospf` | | scope | - | No OSPF subsystem |
| OSPFv3 | `show ipv6 ospfv3` | `show ospf3 *` | `show router ospf3 *` | `show ipv6 ospf` | `show ipv6 ospf` | | scope | - | |
| IS-IS | `show ip isis` | `show isis *` | `show router isis *` | `show isis` | `show isis` | | scope | - | |
| RIP | `show ip rip` | `show rip *` | - | `show ip rip` | `show ip rip` | | scope | - | |
| Babel | - | - | - | - | `show ip babel` | | scope | - | |
| MPLS LDP | - | `show ldp *` | `show router ldp *` | `show mpls ldp` | `show mpls ldp` | | scope | - | |
| MPLS RSVP-TE | - | `show rsvp *` | `show router rsvp *` | - | - | | scope | - | |
| EVPN | `show evpn` | `show evpn *` | `show service id <i> evpn` | `show evpn` | `show evpn` | NLRI decode | partial | bgp | Via bgp-nlri-evpn |
| PIM / IGMP / MLD | `show ip pim` / `igmp` | `show pim *` / `igmp *` | `show router pim` / `igmp` | `show ip pim` / `igmp` | `show ip pim` / `igmp` | | scope | - | |
| BFD | `show protocols bfd` | `show bfd session` | `show router bfd session` | `show bfd peers` | `show bfd peers` | peer-level | partial | bgp | spec bfd |
| Segment Routing | - | `show spring-traffic-engineering *` | `show router segment-routing *` | `show mpls lsp` | `show segment-routing` | | scope | - | |
| VRRP | `show vrrp` | `show vrrp brief` | `show vrrp instance` | `show vrrp` | - | | scope | - | |

## 8. System / Daemon / Process

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| Daemon uptime | `show system uptime` | `show system uptime` | `show system uptime` | `show uptime` | - | `show uptime` | shipped | process | |
| Daemon memory | `show system memory` | `show task memory` | `show system memory-pools` | `show processes memory` | `show memory` | `show system memory` | shipped | process | Complements `rib status`. Nests a `hardware` subobject from `host.DetectMemory()`; same data available standalone via `show host memory` |
| Daemon CPU | `show system cpu` | `show task cpu` | `show system cpu` | `show processes cpu` | - | `show system cpu` | shipped | process | Nests a `hardware` subobject from `host.DetectCPU()`; same data available standalone via `show host cpu` |
| Daemon date/time | `show date` | `show system time` | `show system time` | `show clock` | - | `show system date` | shipped | process | |
| Daemon version | `show version` | `show version` | `show version` | `show version` | - | `ze show version` | shipped | process | |
| Host date / time | `show date` | `show system time` | `show system time` | `show clock` | - | | planned | process | |
| Host hostname | `show system hostname` | `show version` | `show system information` | `show hostname` | - | | scope | - | Read from config |
| Process list / daemons | `show system processes` | `show system processes` | `show system processes` | `show processes` | - | | scope | - | Single-process daemon |
| Kernel log / dmesg | `show log kernel` | - | - | `show logging` | - | | scope | - | OS-level |
| License info | `show license` | `show system license` | `show system license` | `show version detail` | - | | scope | - | |
| Hardware / chassis | `show hardware cpu` etc. | `show chassis hardware` | `show chassis` | `show environment all` | - | | scope | - | |

## 9. Logging & Telemetry

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| Recent log tail | `show log` | `show log messages` | `show log log-id <id>` | `show logging` | - | `log show` | partial | process | |
| Log levels runtime | `show log level` | set command | `admin debug` | `show logging level` | - | `log set` | shipped | process | |
| Warnings report | - | - | `show log log-id warning` | `show logging | include warn` | - | `show warnings` | shipped | process | |
| Errors report | - | - | `show log log-id error` | `show logging | include error` | - | `show errors` | shipped | process | |
| Prometheus metrics | - | - | - | - | - | `metrics show` | shipped | process | |
| Structured event stream (SSE) | - | `monitor *` | - | - | - | `bgp monitor` | shipped | bgp | SSE |
| Operational reports (healthcheck) | - | - | - | - | - | `healthcheck show` | shipped | process | ze-unique |
| Tech-support archive | `generate tech-support archive` | `request support information` | `admin tech-support` | `show tech-support` | - | | planned | process | Zip of logs + running config |

## 10. Diagnostics

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| Ping | `ping <t>` | `ping <t>` | `ping <t>` | `ping <t>` | - | `ping <t>` | shipped | shell | Bare verb (universal diagnostic) |
| Traceroute | `traceroute <t>` | `traceroute <t>` | `traceroute <t>` | `traceroute <t>` | - | `traceroute <t>` | shipped | shell | Bare verb (universal diagnostic) |
| MTR | `mtr <t>` | - | - | - | - | | planned | shell | Lowest priority |
| Packet capture (live) | `monitor traffic interface <i>` | `monitor traffic interface <i>` | `tools dump eth <i>` | `tcpdump interface <i>` | - | | scope | - | Use host tcpdump |
| Bandwidth monitor | `monitor bandwidth <i>` | `monitor interface <i>` | - | - | - | | planned | nl+vpp-stats | Counter delta loop |
| Port scan | `execute port-scan` | - | - | - | - | | scope | - | Not operator tooling ze intends to ship |
| MTU discover / path MTU | `force mtu-host` | - | - | `traceroute mtu` | - | | planned | shell | ping -M |
| Wake-on-LAN | `wake-on-lan <mac>` | - | - | - | - | | scope | - | |
| DNS lookup | `force dns update` | `request dns resolve` | - | - | - | | scope | - | |

## 11. Configuration Operations

Runtime-level config manipulation that is not `set` / `del`.

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| Show running config | `show configuration` | `show configuration` | `admin display-config` | `show running-config` | `show running-config` | `ze config dump` | shipped | config | |
| Show candidate config | - | `show | compare` | `candidate view` | - | `show running-config differences` | editor dashboard | shipped | config | |
| Validate config | - | `commit check` | `candidate check` | `show running-config | section ...` | `configure terminal`+checks | `ze config validate` | shipped | config | |
| Commit config | - | `commit` | `commit` | `commit` | `end` | editor `commit` | shipped | config | |
| Rollback revision | `configure > rollback` | `rollback <n>` | `rollback <n>` | `configure replace` | - | `ze config rollback <N>` | shipped | config | spec-config-2-archive |
| List revisions | - | `show system rollback` | `admin rollback list` | `show configuration sessions` | - | `ze config history` | shipped | config | |
| Archive to URL | `copy file ... scp://...` | `file copy` | `file copy` | `copy running-config scp://` | `copy running-config` | `ze config archive` | shipped | config | |
| Diff two configs | - | `show | compare <file>` | `compare` | `diff` | - | `ze config diff` | shipped | config | |
| Import / merge config | - | `load merge` | `load <file>` | `copy running-config` | `copy running-config` | `ze config import` | shipped | config | |
| Format / canonicalise | - | - | - | - | - | `ze config fmt` | shipped | config | Ze-unique |
| Force commit archive | `force commit-archive` | - | `admin save` | - | - | | planned | config | |
| Generate SSH host key | `generate ssh server-key` | `request security pki ...` | - | `security ssh generate` | - | | scope | - | OS admin task |
| Generate WireGuard keypair | `generate wireguard keypair` | - | - | - | - | `generate wireguard keypair` | shipped | shell | Reserved verb-first form (`generate`) |

## 12. PKI / Crypto / Security

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| Show TLS cert | `show pki certificate <n>` | `show security pki local-certificate` | `show certificate detail` | `show management api` | - | | planned | config | When web/MCP certs become configurable |
| Show SSH keys | `show system ssh` | `show security pki` | `show system security ssh` | `show ssh` | - | | scope | - | |
| Show TACACS+ state | `show system tacacs` | `show system accounting tacplus` | `show system security tacplus` | `show tacacs` | - | tacacs plugin | partial | config | spec-tacacs |
| Show authz roles | - | - | - | `show user-account` | - | authz plugin | shipped | config | |
| Reset SSH session | - | - | `admin disconnect` | `clear ip ssh-server` | - | | planned | process | |

## 13. Appliance & Lifecycle

Ze targets the gokrazy appliance model; these are limited by that scope.

| Command (generic) | VyOS | Junos | Nokia | Arista | FRR | Ze command | Ze status | Backend | Notes |
|-------------------|------|-------|-------|--------|-----|---------|-----------|---------|-------|
| Graceful restart of daemon | `restart frr` | `restart routing` | `admin restart` | `reload` | - | `ze signal restart` | shipped | process | |
| Stop daemon | `service restart <svc>` | `restart routing gracefully` | `admin shutdown` | `reload` | - | `ze signal stop` | shipped | process | |
| Reload config | `run show configuration commands` | `commit` | `admin save` then restart | `copy running-config startup-config` | `reload` | `ze signal reload` | shipped | process | |
| Factory reset | `reset system config` | `load factory-default` | `admin factory-reset` | `write erase` | - | | scope | - | |
| Firmware upgrade | `add system image <url>` | `request system software add` | `admin software upgrade` | `install source <url>` | - | | scope | - | gokrazy handles |
| Reboot host | `reboot` | `request system reboot` | `admin reboot` | `reload now` | - | | scope | - | gokrazy handles |
| Shutdown host | `poweroff` | `request system halt` | `admin halt` | `reload halt` | - | | scope | - | gokrazy handles |

---

## How this drives specs

When a batch of commands shares a backend dependency, group them into a
focused spec and reference this catalogue's rows by name.

| Trigger | Unlocks rows in |
|---------|-----------------|
| VPP firewall backend (ACL/classifier plugin) lands | §5 (all `partial` → `shipped` on VPP) |
| VPP FIB dump (`ip_route_v2_dump`) lands | §2 (IPv4/IPv6/VRF routing table) |
| VPP neighbor dump (`ip_neighbor_dump`) lands | §4 (ARP/ND) |
| conntrack netlink reader | §5 conntrack rows |
| Linux `tc` stats reader | §5 QoS row |
| VPP stat segment reader | §5 QoS, §10 bandwidth monitor |
| OSPF subsystem (future) | §7 OSPF rows |
| IS-IS subsystem (future) | §7 IS-IS rows |
| Process memory / CPU sampler | §8 rows 2, 3 |
| `wg` and `ip neighbor` binding helpers | §11 WireGuard keypair |

## Not tracked here

- Configuration (`set` / `del`) syntax - lives in `docs/guide/configuration.md`.
- ExaBGP equivalence - lives in `docs/exabgp/`.
- Plugin-author commands - lives in `docs/plugin-development/commands.md`.
- Wire-format decoders (`ze bgp decode`) - these are tools, not
  operational commands; see `docs/features/cli-commands.md`.

## Source pointers

- VyOS op-mode: `~/Code/github.com/vyos/vyos-1x/op-mode-definitions/`
  (193 XML files; see the 2026-04-18 analysis in session history).
- Junos: general knowledge of Junos CLI idioms; no in-repo reference.
- Nokia SR OS: classic CLI; no in-repo reference.
- Arista EOS: Cisco-like CLI; no in-repo reference.
- FRR: relevant for BGP comparison; references in `docs/comparison.md`.

When adding new rows, verify vendor spellings against the vendor's own
documentation, not from memory. Mark uncertain vendor cells with `~`
rather than inventing a command.
