# Spec: home-cpe-23b-dns-source-consumers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P3 |
| Release reason | Feature half of child 23, split out by owner decision D-10 (`spec-home-cpe-23-dns-source-fall-through`, "Split"): the per-consumer source lists of the LAN resolver, DHCPv4 option 6, RA RDNSS and the subscriber pool, the RADIUS-supplied subscriber DNS, the pass-through defaults, `show system dns`, doctor and metrics. Thomas placed it in `release/fixes` after 23a (D-10), beside the other home-CPE children |
| RFC gaps | RFC7084-4.3-12 |
| Depends | Implementation: `spec-home-cpe-23-dns-source-fall-through` (23a, its design in `docs/features/dns-resolver.md`: the record registry, the shared selection rule, the list grammar, the `system-dns` event, the router consumer). A LAN DHCPv6 consumer exists only after `post-release/features/spec-dhcpv6-server.md` |
| Phase | 8/8 |
| Handoff | - |
| Updated | 2026-11-09 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md. Sibling and
dependency: `spec-home-cpe-23-dns-source-fall-through` (23a). Every
acceptance criterion of the original child 23 lives in exactly one of the
two files.

The shared design is read from two places: what 23a built is on
`docs/features/dns-resolver.md`, and what only this spec builds (the owner
decisions it depends on, the per-consumer classes, the built-in default
lists of consumers 2 to 5 and the VyOS mapping) is in "Design carried from
23a" below.

RFC gaps, in full: RFC7084-4.3-12 (L-11) is closed for its RDNSS half only,
when the unit's selection comes from the `received` class (the D-5 default
on a unit with no `rdnss server`, or an explicit list); the DNSSL half stays with
`post-release/features/spec-router-advertisement-options-out-of-scope.md`.

IPv6 subscriber DNS (RFC 6911 attribute 169, RDNSS in the PPP RA, DHCPv6
option 23 to subscribers) is excluded from this spec by owner decision D-4 and
owned by `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (RFC 5737 and RFC 3849 prefixes, `example.net`).

## Task

Thomas, dictated (carried from 23a): "we should have an option which allow
people to settle if they want to use a particular dns or when we go via L2TP,
there will be passed one via RADIUS. So something like that can be passed
through. We should consider DNS-received, DNS-configured, what is being used by
the local code. We want the router itself to use the received or configured
DNS, and if we want to get the end user the configured or received DNS, the
full configuration option should be there. Also we may want to provide the
configuration as a backup to the receive and vice versa. So to have a fall
through of the list, to have the DNS fall flow."

Read for meaning, the requirement has four points:

| # | Point |
|---|-------|
| 1 | Resolver addresses come from three kinds of source: configured by the operator, received from the network (DHCPv4, DHCPv6, IPCP, RA), and, on the access side, supplied by RADIUS for one subscriber |
| 2 | Several things consume resolver addresses: the router itself, and every path that hands resolvers to an end user (LAN clients, PPP subscribers). Each needs the full choice of sources |
| 3 | Each consumer takes an ordered list of sources and falls through it: configured as a backup to received, or received as a backup to configured |
| 4 | A RADIUS-supplied resolver passes through to the subscriber |

The owner decisions are in "Design carried from 23a". This spec delivers points 2, 3
and 4 for every consumer other than the router: the LAN resolver's upstreams,
DHCPv4 option 6, RA RDNSS and the subscriber pool, each with its `dns-source`
list over the 23a registry, the RADIUS-supplied subscriber DNS, and the
operator surfaces (`show system dns`, doctor, metrics).

### Owner decisions owned here

Moved from 23a at the split; the numbering is kept so every reference stays
valid.

| # | Decision (owner, 2026-11-08) | Reading recorded in this spec | Answers |
|---|------------------------------|-------------------------------|---------|
| D-8 | RFC 2548 is listed optional for the `lns` profile | `rfc/short/rfc2548.md` is written with `Optional use cases: lns` and walked; the MS-Primary-DNS-Server and MS-Secondary-DNS-Server rows are proven by 23b's subscriber tests, and every other attribute's rows stay gaps. Writing it is owed RFC tooling work in 23b ("RFC 2548 in this spec") | O-5 |
| D-9 | Shorten DHCPv4 leases while LAN clients would hold ISP DNS and the WAN is down | A subnet whose option 6 list (written or default) names `received`, and whose evaluation finds no usable `received` entry, hands out 60-second leases; once a `received` entry is usable it returns to its configured lease time. A configured lease shorter than 60 seconds is kept. RFC 5625 Section 5.3: "Alternatively, the DHCP server might only issue very short (i.e., 60 second) leases while the WAN link is down, only reverting to more typical lease lengths once the WAN link is up and the upstream DNS servers are known." The same section says "Whilst no specific recommendations are given here", so this is Ze's chosen behaviour, not a requirement row | O-8 |

| # | Decision | Reading recorded in this spec | Answers |
|---|----------|-------------------------------|---------|
| D-16 | A consumer that hands addresses to another host never hands out a loopback or an unscoped link-local address, whatever its origin | The consumers that hand addresses on are DHCPv4 option 6, RA RDNSS and subscriber DNS. The LAN resolver is not one of them: it forwards from Ze itself, so a loopback upstream such as 127.0.0.53 is usable there, and its exposure to clients is `self`, which D-5 refuses on `forward dns-source`. IPv4 link-local (169.254.0.0/16) stays out of D-16 deliberately: it carries no zone to lose, and a server at such an address on the client's own link is reachable from it. A loopback address (127.0.0.0/8, ::1) and an IPv6 link-local address with no zone are wrong on all of them: a configured 127.0.0.1 is as wrong as the host's 127.0.0.53 (D-13), because either names a server only Ze itself can reach. The shared selection rule drops them for those consumers only, as part of "usable": each consumer's list states at construction whether it serves Ze or hands addresses on, and the router consumer keeps loopback and link-local, because Ze's own lookups can reach them. Each drop is logged with the consumer path and the address. It lands in 23b, as AC-27, in phases 5 (DHCPv4 option 6 and RA RDNSS) and 6 (subscriber), tested by `TestSelectDropsHostOnlyAddressesForHandedOutLists` in the core leaf and by one case in each consumer's pass-through test | a host-only address handed to a LAN client or a subscriber |
| D-17 | The LAN resolver keeps its answer and negative caches when its forward upstream set changes, as dnsmasq does by default | A fresh `system-dns` snapshot or a reselection of `forward dns-source` replaces only the forward list of the published policy (`resolver.relearn`, `internal/plugins/dnsresolver/state.go`); the caches are emptied only by `clear dns resolver cache all` (`resolver.clearCaches`). This is dnsmasq 2.91's default: `poll_resolv` (`src/dnsmasq.c`) re-reads the upstream servers and calls `clear_cache_and_reload` only under `option_bool(OPT_RELOAD)`, which only the opt-in `--clear-on-reload` sets, documented as "Whenever /etc/resolv.conf is re-read or the upstream servers are set via DBus, clear the DNS cache." An answer cached from the old upstreams, a negative one included, is therefore served until its TTL ends. A test that queries before the learned upstream arrives asks a name no later stage asks, so the earlier upstream's negative answer cannot decide a later stage (5c6105dc87). `docs/architecture/dns/lan-resolver.md` states it | whether the LAN resolver clears its caches when its upstreams change |
| D-18 | The subscriber's two IPCP DNS slots are filled primary, then secondary, from the pool's selection in list order | `selectSubscriberDNS` (`internal/component/l2tp/plugins/pool/dns_source.go`) runs the shared rule over the pool's list, keeps the first `ipcpDNSSlots` (2) servers, puts the first in Primary-DNS and the second in Secondary-DNS, and logs the rest at INFO with the consumer path. A RADIUS reply with only MS-Primary-DNS-Server therefore offers that one alone under `first-usable` and, under `merge`, that one then the pool's primary; the accel-ppp per-slot overwrite is not copied (Known Limitations). `slotAddresses` reads a primary and secondary pair (RADIUS or the pool's leaves) in slot order, an unset slot contributing nothing | how a selection of any length becomes the two IPCP slots |
| D-19 | A handed-on list also refuses an IPv6 link-local whose zone is not the link the consumer hands on to | Review round 1 finding 4: a learned RDNSS `fe80::x%wan0` is reachable only on the WAN, and RFC 8106 Section 5.1 says "When a resolver sends a DNS query message to an RDNSS identified by a link-local address, it MUST use the corresponding link." D-16 named only the zone-less case; fffefb45f9 makes the RA consumer drop a link-local of another link, logged as a drop, and RFC8106-5.1-2 is recorded on it (83f4316387) | a link-local resolver learned on the WAN advertised on the LAN |

### Design carried from 23a

23a (`spec-home-cpe-23-dns-source-fall-through`) recorded the shared design
once. What 23a built (the record, the four classes, the list grammar with
`received-interface`, the mode rule and the router's refusals and default)
is on `docs/features/dns-resolver.md`, "Where the router's servers come from"
and "The `dns-source` list and the learned record". What only this spec
builds is carried here from 23a unchanged, so 23a can be deleted at its
closure.

Owner decisions (Thomas, 2026-11-08; D-12 to D-15 governed 23a's own
implementation and stay in its closure commit):

| # | Thomas, as stated | Reading recorded | Answers |
|---|-------------------|------------------|---------|
| D-1 | "The fall-through rule should be explicitly configured." | Every consumer's `dns-source` container carries a `mode` leaf the operator sets: `first-usable` (the first source with usable entries wins) or `merge` (every source, in list order). The leaf has no YANG default. A list naming more than one source without `mode` is refused at `ze config validate`, commit and boot, with the fix in the message. A one-source list needs no `mode`, because both modes give the same answer for it | O-1 |
| D-2 | "RFC 5625 inside child 23" | RFC 5625 (BCP 152) is followed, and it is listed for the home-CPE role (the `fttx-edge` profile, which already carries RFC 7084) as decided in scope | O-7 |
| D-3 | "By default the behavior should be to pass the DNS through." | With no `dns-source` config, a consumer passes on the DNS it received rather than substituting Ze's own address. Each built-in default is a stated list with a stated mode, documented and shown, never a hidden choice. D-5 fixes where configured servers stand against received ones | O-2 |
| D-4 | "spec of its own ... I don't want to delay it, but I don't want to hold back the [home-CPE] umbrella completion. We need the feature, but not for the test I want to do in my house." | IPv6 subscriber DNS is a separate spec, `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md`, queued after the home-CPE umbrella, which it does not block. This spec excludes it | O-6 |
| D-5 | "We should have a default behavior of we can be passed through our local DNS with the default DNS being passed through therefore the behavior should be passed through by default and if you config DNS, they will only become actively passed through." Then, on the YANG shape: "lgtm" | Configured beats received when set, on every consumer, the LAN resolver's upstreams included. The approved shape: a typedef `dns-source-class` (`received`, `configured`, `self`, `radius`) and a grouping `dns-source` declared once in the shared types module `ze-types`, `uses`d by each consumer beside its existing configured-server leaves. With the container absent, a consumer uses its configured servers when any are set on it, otherwise it passes the received ones through: `source [configured received] mode first-usable`. The subscriber pool's default is `radius`, then `configured`, then `received`. `mode` is required once `source` names two or more classes, and `self` is refused on the system consumer, because Ze cannot be its own upstream | P-1, P-2 |
| D-6 | `self` hands out Ze's address on the interface the client is on | DHCP subnet: Ze's IPv4 address on the interface that serves the subnet (the address inside the subnet). RA: the first global or unique-local IPv6 address Ze holds on the advertising unit. Pool: the LNS's local IPCP address on the subscriber's PPP link. With no such address the `self` source has no usable entry and the list falls through | O-3 |
| D-7 | WAN RA RDNSS is received now, as a `received` source like IPCP | Built by 23a: origin `ra`, rank 400 | O-4 |
| D-10 | 23b goes to `release/fixes` after 23a | This file, P3, `Depends` on 23a | O-9 |
| D-11 | `received` gains an optional per-interface filter | Built by 23a: `received-interface`. It restores the VyOS `system name-server <if>`, `service dns forwarding dhcp <if>` and `no-peer-dns` mappings to equivalent | the per-interface difference recorded under D-5 |

Source classes per consumer:

| Class | Meaning | Allowed in |
|-------|---------|-----------|
| `configured` | the consumer's own static list: router `system name-server`; resolver `forward server`; DHCP subnet `dns-server`; RA `rdnss server`; pool `dns-primary`, `dns-secondary` | every consumer |
| `received` | every unexpired learned record, by origin rank, interface, unit; with `received-interface` set, only the records of the named interfaces, by the leaf-list order, then origin rank, then unit (D-11) | every consumer |
| `self` | Ze's own address on the interface the client is on (D-6). No such address means no usable entry | DHCP subnet, RA, pool. Refused on the system consumer (D-5: Ze cannot be its own upstream), and on the LAN resolver's `forward` for the same reason |
| `radius` | the subscriber's MS-Primary/Secondary-DNS-Server (IPv4 only; IPv6 subscriber DNS is D-4's separate spec) | pool only; elsewhere no subscriber exists to carry it |

A `received-interface` naming an interface that holds no record is accepted
(the interface may learn later); `show system dns` shows the selection empty
for that interface, and the doctor check warns. After selection the consumer
applies its own bound (8 RDNSS addresses, 63 for option 6, two IPCP slots
filled primary then secondary), keeping list order and logging what it
drops. An empty answer means the consumer offers none, which is what an
absent leaf means today. The LAN resolver's `self` refusal names
`service dns-resolver forward dns-source` and the fix
`set service dns-resolver forward server <address>`. Each consumer calls the
core-leaf checks from its `InProcessConfigVerifier`, its `OnConfigVerify` and
its boot parse.

Built-in default lists (D-1, D-3, D-5). A default applies only while the
consumer's `dns-source` container is absent; it is written in the YANG
`description` of that container, in the operator guide, and shown by
`show system dns` with the word `default` beside it:

| Consumer | YANG home | Default list | Default mode | Against today |
|----------|-----------|--------------|--------------|---------------|
| 2 LAN resolver upstreams | `service dns-resolver forward dns-source`, beside `forward server` | `configured`, `received` | `first-usable` | same while any `forward server` is set; changes with none: the resolver forwards to the learned servers instead of having no upstream |
| 3 DHCPv4 option 6 | `service dhcp-server shared-network subnet dns-source`, beside `dns-server` | `configured`, `received` | `first-usable` | same while `dns-server` is set; changes with none: the WAN-learned IPv4 servers are handed out instead of no option 6 |
| 4 RA RDNSS | `interface ... unit ... ipv6 router-advertisement rdnss dns-source`, beside `rdnss server` | `configured`, `received` | `first-usable` | same while `rdnss server` is set; changes with none: the WAN-learned IPv6 servers are advertised instead of no RDNSS option |
| 5 Subscriber IPCP (LNS and PPPoE server) | `l2tp pool ipv4 dns-source` and `l2tp pool named-pool dns-source`, beside `dns-primary`, `dns-secondary` | `radius`, `configured`, `received` | `first-usable` | changes: an Access-Accept's MS-Primary/Secondary-DNS-Server are offered; the pool's `dns-primary`/`dns-secondary` only when RADIUS gives none; the learned servers only when neither exists |

Consumer 1, the router, is 23a's (`system dns-source`, the same default).
Each consumer module `uses dns-source` beside its configured-server leaves:
`ze-dns-resolver-conf.yang`, `ze-dhcp-server-conf.yang`, `ze-iface-conf.yang`
(`router-advertisement rdnss`) and `ze-l2tp-pool-conf.yang` (`l2tp pool ipv4`
and each `named-pool`).

VyOS mapping (vyos-1x 611550d23, input to child 10):

| VyOS | Ze form | Class |
|------|---------|-------|
| `system name-server 192.0.2.53` | `system name-server 192.0.2.53` | equivalent |
| `system name-server eth0` (servers learned on eth0) | `system dns-source source received`, `system dns-source received-interface eth0` | equivalent (D-11): only eth0's records, DHCPv4 before DHCPv6 as VyOS lists the `dhcp-` tag before `dhcpv6-` |
| `system name-server 192.0.2.53` plus `system name-server eth0` | `system dns-source` sources `configured`, then `received`, mode `merge`, `received-interface eth0` | equivalent: VyOS merges by tag order, and the migration writes `merge` explicitly. Several interface entries keep their VyOS order in `received-interface` |
| `interfaces pppoe pppoe0 no-peer-dns` | the migration leaves pppoe0 out of every `received-interface` it writes, and writes an explicit `dns-source` on every consumer so no built-in default takes pppoe0's lease | equivalent (D-11): VyOS stores no peer DNS for pppoe0, and in Ze no list that the migration writes selects it |
| `service dns forwarding name-server X` | `service dns-resolver forward server X` | equivalent |
| `service dns forwarding system` (with `name-server X`) | `forward server X` plus each `system name-server` entry, and `forward dns-source` sources `configured`, `received`, mode `merge` | equivalent in the servers asked; the migration copies the `system name-server` entries into `forward server`, because no class reads the router's selection (D-5) |
| `service dns forwarding dhcp eth0` | `forward dns-source source received`, `forward dns-source received-interface eth0` (mode `merge` when listed with other sources) | equivalent (D-11) |
| `service dhcp-server ... option name-server X` | subnet `dns-server X` | equivalent: under the D-5 default a set `dns-server` wins |
| `service pppoe-server name-server X`, `vpn l2tp remote-access name-server X` | `l2tp pool ipv4 dns-primary`, `dns-secondary` | equivalent for two IPv4 servers when RADIUS sends no MS DNS; with it, Ze's default (D-5) offers RADIUS's, as accel-ppp does |
| accel-ppp RADIUS MS DNS override | pool default `radius`, `configured`, `received`, `first-usable` | equivalent when RADIUS sends both servers or neither. accel-ppp overwrites each option independently, so RADIUS sending only the primary keeps the configured secondary there; Ze's `first-usable` offers the RADIUS primary alone and `merge` offers the RADIUS primary then the pool's primary. Neither mode copies the per-slot overwrite, which is recorded as a difference |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/system-plugins.md`, `ai/doors/forwarding.md`, `ai/doors/access-security.md`, `ai/doors/core.md`
  → Constraint: `dnsresolver`, `dhcpserver`, the iface RA sender and the L2TP `pool` plugin can each run in another process; they read records and selections only from the 23a `system-dns` event, with replay on subscribe, never by a Go call to `system.ProcessResolvers()`
  → Constraint: a removed config root arrives as `{}`; a consumer whose `dns-source` list is removed reverts to its default list, never to "no change"
  → Constraint: `ze config validate` never runs a live `OnConfigVerify`; each consumer calls the 23a core-leaf checks from its `InProcessConfigVerifier`, its `OnConfigVerify` and its boot parse
- [ ] `docs/features/dns-resolver.md`, "Where the router's servers come from" and "The `dns-source` list and the learned record" (record shape, source classes, mode rule, refusals, the router's default, YANG shape), and "Design carried from 23a" above (owner decisions, per-consumer classes and defaults, VyOS mapping)
  → Constraint: every consumer evaluates its own list with the one shared rule; no class reads another consumer's selection
- [ ] `docs/architecture/dns/lan-resolver.md` "Forwarding", "Reload"
  → Constraint: `resolver.forward` tries the `forward server` entries in order, one pass, equal time share; a reload that changes only `forward` keeps every socket. A learned-upstream change reuses that publish path (no rebind) and never serves under a half-applied list
- [ ] `docs/architecture/l2tp/subscriber-session-model.md`
  → Constraint: subscriber auth, pool and shaper reach a session through `l2tp/subscriber` handler registries shared by L2TP and PPPoE; RADIUS DNS reaches IPCP through `l2tp.AuthMetadata` and the pool plugin, the path Framed-IP and Framed-Pool already take

### RFC Summaries (Scope: config, with protocol consumers)
- [ ] `rfc/short/rfc1877.md` (enrolled; use cases fttx-edge, lns)
  → Constraint: Section 1.1: "This Configuration Option defines a method for negotiating with the remote peer the address of the primary DNS server to be used on the local end of the link." The LNS is that remote peer for a subscriber; the address it Naks with is consumer 5's output
- [ ] `rfc/short/rfc7084.md` (enrolled; fttx-edge)
  → Constraint: RFC7084-4.3-11 (L-10, DHCPv6 DNS_SERVERS) has no LAN DHCPv6 server to carry it; RFC7084-4.3-12 (L-11) RDNSS is advertised from static config only today; RFC7084-4.3-19 (L-12, SHOULD) asks that WAN-learned options reach the LAN: the `received` class is that path
- [ ] `rfc/short/rfc2132.md` (enrolled; fttx-edge)
  → Constraint: option 6 length is a multiple of 4; the existing encoder holds it, a source only changes which addresses fill it
- [ ] `rfc/short/rfc8106.md` (enrolled; optional for fttx-edge)
  → Constraint: RFC8106-5.1-1 "A value of zero means that the RDNSS addresses MUST no longer be used." An advertised RDNSS whose source is withdrawn is sent once with Lifetime 0
- [ ] `rfc/short/rfc5625.md` (written by 23a, phase 3)
  → Constraint: Section 3: "Except when required to enforce an active security or network policy (such as maintaining a pre-authentication "walled garden"), end-users SHOULD be able to send their DNS queries to specified upstream resolvers, thereby bypassing the proxy altogether." The pass-through defaults (D-3, D-5) of the DHCPv4 and RA consumers are this sentence
  → Constraint: Section 5.3: "If the DHCP lease is obtained whilst the WAN link is down, then the DHCP client (and hence the DNS client) will not receive the correct values until the DHCP lease is renewed." R-2 carries it, and D-9 is Ze's answer
- [ ] `rfc/short/rfc6911.md` (enrolment out-of-scope, optional for lns)
  → Constraint: Section 3.2: "This Attribute MAY be included multiple times in Access- Accept packets when the intention is for a NAS to announce more than one DNS server address to an RG/host." `extractAuthMetadata` reads none of types 168 to 172. Attribute 169 (IPv6 subscriber DNS) is excluded here by D-4 and owned by `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md`
- [ ] RFC 2548 (no summary yet; text at `rfc/full/rfc2548.txt`)
  → Constraint: Section 2.7.6: "The MS-Primary-DNS-Server Attribute is used to indicate the address of the primary Domain Name Server (DNS) [16, 17] server to be used by the PPP peer."
  → Decision (D-8): listed optional for `lns`. The summary `rfc/short/rfc2548.md` (`Optional use cases: lns`), its extraction walk and its enrolment are owed RFC tooling work here ("RFC 2548 in this spec"); the summary is what lists it, never a hand-edited profile

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang` - `service dns-resolver forward server` (ordered-by user, max 8, `ip`, `port`); its container description says LAN hosts learn the resolver from DHCPv4 option 6, RDNSS or IPCP, and nothing puts it there automatically
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - subnet `dns-server` leaf-list (IPv4, max 63, order kept). No DHCPv6 server and no DHCP server show command exist
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` `router-advertisement rdnss { server (max 8); lifetime }` - static only
- [ ] `internal/component/l2tp/plugins/pool/register.go` - IPCP DNS for a subscriber is `pool.dnsPrimary`/`dnsSecondary` of the default or Framed-Pool named pool, also on the Framed-IP path
- [ ] `internal/component/l2tp/plugins/pool/yang/ze-l2tp-pool-conf.yang` - `l2tp pool ipv4 dns-primary|dns-secondary`, same under `named-pool`
- [ ] `internal/component/l2tp/plugins/authradius/extract.go` `extractAuthMetadata` - reads Framed-IP, netmask, Framed-Pool, timeouts, Filter-Id, Framed-Route, IPv6 prefixes; no DNS attribute. The radius dictionary defines `VendorMicrosoft = 311` but no MS DNS attribute ids
- [ ] `internal/component/l2tp/ppp/ra_send.go` `raSender.send` - the subscriber RA sets M and O and carries no RDNSS; the PPP DHCPv6 code defines `D6OptDNSServers` (23) and nothing sends it, so an IPv6 subscriber is told to fetch other configuration and gets no resolver
- [ ] accel-ppp 06f64b1 (2026-03-16) `accel-pppd/radius/radius.c` `rad_proc_attrs` and `accel-pppd/ppp/ipcp_opt_dns.c` `ev_dns`: MS-Primary-DNS-Server and MS-Secondary-DNS-Server from the Access-Accept overwrite the configured `[dns]` value, each option independently

**Behavior to preserve:**
- With no new config, a consumer that has configured servers (`forward server`, `dns-server`, `rdnss server`) carries exactly what it carries today, because the D-5 default uses `configured` whenever any is set
- With no new config and no learned or RADIUS DNS present, every consumer, subscriber IPCP included, carries exactly what it carries today (23a AC-5)
- The LAN resolver keeps its sockets when only its upstream list changes

**Behavior to change:**
- A DHCPv4 subnet whose option 6 list names `received` hands out 60-second leases while no `received` entry is usable (D-9)
- Pass-through by default (D-3, D-5): with no `dns-source` config, a consumer with no configured servers passes the received ones through: the LAN resolver with no `forward server` forwards to the learned servers, a subnet with no `dns-server` hands them out as option 6, and a unit with no `rdnss server` advertises them as RDNSS. A PPP subscriber receives the RADIUS MS-Primary/Secondary-DNS-Server before the pool's `dns-primary`/`dns-secondary`, and the learned servers when neither exists. Today each of those consumers offers nothing when its configured leaf is absent, and the pool offers only its `dns-primary`/`dns-secondary`. Every repository user of those leaves is converted or re-validated in the same work (`ai/META-DECISIONS.md`, "A change to what Ze does"), listed under "Files to Modify"

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| Record registry, shared rule, `system-dns` event | 23a: `internal/component/config/system/resolv.go`, the `internal/core/dnssource` leaf, the event namespace with replay | records from every source; one selection rule; the router consumer | extends: each consumer here subscribes and evaluates its own list; nothing in the registry or the rule names a consumer |
| LAN resolver upstreams | dnsresolver `resolver.forward`, `resolverInstaller.install` (`docs/architecture/dns/lan-resolver.md`) | forwards to `forward server` in order | extends: the forward list is the evaluated list; `configured` is `forward server`; a change republishes without rebinding |
| DHCPv4 option 6 | `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` `dns-server` | static per subnet | extends: option 6 is the subnet's evaluated list; `configured` is `dns-server` |
| RA RDNSS | `internal/component/iface/yang/ze-iface-conf.yang` `rdnss`; the ndp encoder `BuildRA` | static per unit, one option, max 8 | extends: RDNSS is the unit's evaluated IPv6 list; a withdrawn address is advertised once with Lifetime 0 |
| Subscriber IPCP DNS | `internal/component/l2tp/plugins/pool/register.go` IP response; `l2tp.AuthMetadata` | pool `dns-primary`/`dns-secondary` | extends: the evaluated list fills primary then secondary; `radius` reads two new `AuthMetadata` fields |
| RADIUS Access-Accept extraction | `internal/component/l2tp/plugins/authradius/extract.go` `extractAuthMetadata` | Framed-* attributes into `AuthMetadata` | extends: vendor 311 types 28 and 29 (type 169 is D-4's separate spec) |
| DHCPv4 lease time | dhcpserver subnet lease time, used by the reply builder | one configured lease time per subnet | extends: the reply builder takes 60 seconds while D-9's condition holds, else the configured time |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The 23a `system-dns` event (record set and router selection, replayed on subscribe); a commit changing a consumer's own configured list or its `dns-source`; a RADIUS Access-Accept for a subscriber (per session, not registry state)

### Transformation Path
The path is 23a's "Transformation Path", steps 4 to 6: each consumer plugin
subscribes (replay on start), evaluates its own list over the records with the
shared rule, filters by its family, and applies it; each publishes its
selection, which `show system dns` renders; the subscriber path stores
MS-Primary and MS-Secondary in `AuthMetadata` for the pool's `radius` class.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Registry to plugin consumers | EventBus namespace `system-dns` (23a) with record-set and selection events and a replay request; JSON kebab-case | No |
| RADIUS to pool | two new `l2tp.AuthMetadata` fields, read by the pool as Framed-Pool is | No |

### Changed-Wire Census
| Wire | Role (writer / reader) | Carrier | Source evidence | Migration or preservation disposition |
|------|------------------------|---------|-----------------|---------------------------------------|
| DHCPv4 option 6 | writer | dhcpserver reply builder | static leaf-list today | identical while `dns-server` is set (D-5); a subnet with none now carries the received IPv4 servers instead of no option 6; encoding unchanged |
| DHCPv4 lease time (option 51) | writer | dhcpserver reply builder | configured per subnet | 60 seconds while D-9's condition holds; identical otherwise |
| RA RDNSS option | writer | ndp `BuildRA` | static today | identical while `rdnss server` is set (D-5); a unit with none now advertises the received IPv6 servers instead of no option; adds the Lifetime 0 withdrawal |
| IPCP options 129/131 from the LNS | writer | pool IP response | pool leaves today | content changes under the default when the Access-Accept carries MS DNS, or when neither RADIUS nor the pool names a server and a received one exists (D-5); identical otherwise |
| LAN resolver upstream queries | writer | dnsresolver `resolver.forward` | `forward server` today | identical while any `forward server` is set (D-5); a resolver with none now forwards to the received servers |
| RADIUS vendor 311 types 28/29 | reader | `extractAuthMetadata` | not read today | new read |
| DHCPv4 option 6 | reader | a LAN DHCPv4 client (outside the repository), and Ze's own DHCPv4 client (`internal/plugins/iface/dhcp`, `setV4Resolvers`) | RFC 2132 Section 3.8 encoding, which 23b does not change | preserved: the encoding is unchanged, only which addresses it carries |
| DHCPv4 lease time (option 51) | reader | a LAN DHCPv4 client (outside the repository) | RFC 2132 Section 9.2 encoding, unchanged | preserved: a 60-second value is a valid lease time to every client |
| RA RDNSS option | reader | a LAN host (outside the repository), and Ze's own RA reader (`internal/core/ndp/rdnss.go` `ParseRDNSS`, `internal/component/iface/ra_rdnss.go`) | RFC 8106 Section 5.1 encoding, unchanged | preserved: the encoding is unchanged; a Lifetime 0 withdrawal is RFC 8106 Section 6.2 behaviour every reader already handles |
| IPCP options 129/131 from the LNS | reader | a PPP subscriber (outside the repository), and Ze's own PPPoE client IPCP | RFC 1877 encoding, unchanged | preserved: only the addresses offered change |
| LAN resolver upstream queries | reader | the upstream resolver (outside the repository) | plain DNS queries, unchanged | preserved: only which upstream is asked changes |
| RADIUS vendor 311 types 28/29 | writer | the operator's RADIUS server (outside the repository); no carrier in the repository sends them today | RFC 2548 Sections 2.7.6 and 2.7.7 encoding | new: a server that sends none leaves today's behaviour |

**Search evidence:** scope `internal/plugins/dhcpserver`, `internal/core/ndp`, `internal/component/iface`, `internal/component/l2tp/ppp`, `internal/component/l2tp/plugins/pool`, `internal/component/l2tp/plugins/authradius`, `internal/plugins/dnsresolver` (Go, tests excluded), and `internal`, `test`, `cmd` for Go, Python and shell carriers of the MS DNS attributes; terms `dns-server`, `DNSServers`, `func BuildRA`, `RDNSS`, `DNSPrimary`, `DNSSecondary`, `VendorSpecific`, `311`, `MS-Primary`, `MSPrimary`, `func (*resolver) forward`. Positive controls: `dhcpserver/config.go` and `handler.go` (option 6), `ndp/ra.go` `BuildRA` and `iface/ra_rdnss.go` (RDNSS), `pool/register.go` `DNSPrimary`/`DNSSecondary` in the IP response, `dnsresolver/resolve.go` `resolver.forward`. No carrier reads MS-Primary or MS-Secondary-DNS-Server today (`authradius/extract_vsa.go` reads other vendor attributes), and no carrier outside these packages writes any of the wires.

### Integration Points
- `show system dns` (new): records (origin, interface, unit, addresses, expires) and selections (consumer, list, mode, `default` when the built-in default applies, class or classes used, servers), on every command surface

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | consumers read records only from the event |
| No unintended coupling | Yes | plugins never touch the registry's state; they receive values over the bus |
| No duplicated functionality | Yes | every consumer uses the one 23a selection rule |
| Zero-copy preserved where applicable | Yes | option encoders unchanged; selection runs on a lease or config change, never per packet or per query |
| Registration over hardcoding, outbound | Yes | each consumer owns its list in its own YANG and publishes its own selection; `show system dns` renders whatever selections arrive |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-3 | A subscriber keeps the DNS negotiated at IPCP until it renegotiates | IPCP has no unsolicited update; LNS renegotiation is child 15 | none | RFC 1661 reading | confirmed: the pool computes the DNS once per IP request (`selectSubscriberDNS` called only from the IPCP response path, `internal/component/l2tp/plugins/pool/register.go` line 317) |
| A-5 | `show system dns` has an owning command package for `system` operational data | not yet checked | a new `*-cmd` plugin is owed | `./le cli ownership` at implementation | broken: no owner existed; the new `system-dns-cmd` plugin owns it (`internal/plugins/system-dns-cmd/cmd/show_dns.go`, `registerShowSystemDNS`), the "if wrong" branch |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-2 | A LAN DHCP client keeps a received server after the WAN source is gone, or holds no server when it leased before the WAN came up (RFC 5625 Section 5.3). The pass-through default (D-3, D-5) makes this the common case on a subnet with no `dns-server`, not an opt-in | stale option 6 in a client lease | D-9: 60-second leases while no `received` entry is usable, so a client that leased with the WAN down renews within a minute of it coming up (AC-25); a client holding a server when the WAN drops keeps it until its renewal, because FORCERENEW is not implemented; the guide states both |
| R-9 | `self` hands out an address the LAN resolver does not listen on | LAN clients time out on Ze's address | the doctor check (Integration Checklist) warns when a consumer's selection holds a `self` address no `service dns-resolver` listener serves |
| R-8 | A box that is not a CPE (a PXE provisioning server, itself a DHCP client on its management port) with no `dns-server` on a subnet starts handing its management lease's resolvers to the provisioned hosts under the pass-through default. A subnet with `dns-server` set is unaffected (D-5) | option 6 present on a subnet that sent none | the guide names `dns-source source configured`, which keeps a subnet with no `dns-server` sending no option 6; the conversion of repository configs keeps every existing expectation |
| R-3 | A loop: Ze's own lookups or the LAN resolver's upstreams pointed at Ze's LAN resolver | the resolver forwarding to itself | `self` refused on the system consumer and on `forward dns-source` by the verifier check (D-5); unit test |
| R-5 | A consumer in another process misses the first selection | empty upstream list after restart | replay on subscribe; AC-13 |
| R-6 | RADIUS sends a non-unicast or 0.0.0.0 DNS | subscriber gets an unusable resolver | the same validity check as `isValidSubscriberIP`; invalid values ignored with a log line |
| R-7 | IPv6 addresses offered to an IPv4 consumer | option 6 with an IPv6 value | the rule filters by family before a class can win |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | name resolution on LAN clients or subscribers |
| How is it reverted? | single commit revert per phase; config leaves are additive |
| Who else touches this path? | 23a (registry and rule), child 8 (RA on the delegated prefix), child 2 (DHCP subnets), `spec-dhcpv6-server.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `service dns-resolver forward dns-source source received` | → | dnsresolver forward list from the event | `test/plugin/dns-resolver-forward-received.ci` |
| subnet `dns-source source self` | → | dhcpserver option 6 | `test/dhcp/dhcp-server-dns-source-self.ci` |
| subnet with no `dns-server` and no `dns-source`, WAN DHCPv4 lease with option 6; then `dns-server` added | → | dhcpserver option 6 under the D-5 default | `test/dhcp/dhcp-server-dns-pass-through.ci` |
| `service dns-resolver` with no `forward server` and no `dns-source`, a WAN DHCPv4 lease | → | dnsresolver forward list under the D-5 default | `test/plugin/dns-resolver-forward-default.ci` |
| `system dns-source source self` through `ze config validate` and `ze start` | → | the core-leaf class check from the verifier and the boot parse | `test/parse/dns-source-class-refused.ci` |
| RA `rdnss dns-source source received`, then the lease withdrawn | → | RA sender, Lifetime 0 | `TestRDNSSWithdrawnAdvertisedLifetimeZero` |
| Access-Accept with MS-Primary-DNS-Server, pool with no `dns-source` | → | `extractAuthMetadata`, pool response under the default `radius`, `configured`, `received` | `TestPoolIPCPDNSFromRadius`, `test/l2tp/l2tp-pool-dns-source-radius.ci`, interop scenario `radius-subscriber-dns` under `test/interop-radius-subscriber/` |
| subnet with no `dns-server`, no usable `received` entry, then a WAN lease arrives | → | dhcpserver lease time under D-9 | `TestLeaseShortWhileReceivedUnusable`, `test/dhcp/dhcp-server-short-lease-wan-down.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-6 | `forward dns-source source received` with a `ppp` lease | the LAN resolver forwards to the IPCP servers; withdrawing the lease moves it to the next class without rebinding a listener | none |
| AC-7 | DHCP subnet 192.0.2.0/24 served on an interface where Ze holds 192.0.2.1, `dns-source source self`; and an RA unit with `rdnss dns-source source self` where Ze holds 2001:db8:1::1 | option 6 carries 192.0.2.1 (D-6); the RA's RDNSS carries 2001:db8:1::1; a unit with no global or unique-local address advertises no RDNSS from `self` and falls through | none |
| AC-8 | RA `rdnss dns-source source received` with a DHCPv6 lease carrying 2001:db8::53 | RDNSS carries 2001:db8::53 with a Lifetime not above the lease's remaining valid lifetime; on withdrawal one RA carries it with Lifetime 0 | none |
| AC-9 | Access-Accept with MS-Primary-DNS-Server 198.51.100.53, pool with no `dns-source` (default `radius`, `configured`, `received`, `first-usable`) | the subscriber's IPCP Configure-Nak offers 198.51.100.53 as primary | none |
| AC-10 | Access-Accept with no DNS attribute, same pool | the pool's `dns-primary` is offered; with no `dns-primary`/`dns-secondary` on the pool and a WAN lease carrying 192.0.2.53, 192.0.2.53 is offered | none |
| AC-11 | A list naming a class its consumer refuses (`radius` under `system dns-source`; `self` under `system dns-source` or `forward dns-source`) | `ze config validate`, commit and boot refuse it, naming the path and the allowed classes; the `self` refusal carries the what, why and fix text in 23a ("When a `mode` is required") | none |
| AC-12 | `show system dns` | answers records and every consumer's selection with its list, mode, the `default` marker where the built-in default applies, and the class used, on every command surface | none |
| AC-13 | A consumer plugin restarts while leases exist | it applies the current selection from the replay before answering | none |
| AC-14 | DHCP subnet with no `dns-server` and no `dns-source`; a WAN DHCPv4 lease carries 192.0.2.53 | option 6 carries 192.0.2.53; once `dns-server 198.51.100.1` is committed on the subnet, the next reply carries 198.51.100.1 alone (D-5); once the lease is withdrawn or expires with no `dns-server`, no reply carries option 6 | none |
| AC-15 | RA unit with no `rdnss server` and no `dns-source`; a WAN DHCPv6 lease carries 2001:db8::53 | RDNSS carries 2001:db8::53; once `rdnss server 2001:db8:1::53` is committed, the next RA carries 2001:db8:1::53 and one RA carries 2001:db8::53 with Lifetime 0; on withdrawal of the lease with no `rdnss server`, one RA carries 2001:db8::53 with Lifetime 0 | none |
| AC-16b | The `dns-source` of `service dns-resolver forward`, a DHCP subnet, an RA `rdnss` or an `l2tp pool` (`ipv4` or `named-pool`) with two sources and no `mode` (the router's half is 23a AC-16) | `ze config validate`, commit and `ze start` refuse it with the what, why and fix text in 23a, naming that consumer's path; the same list with `mode` set is accepted; a one-source list without `mode` is accepted | none |
| AC-20 | `service dns-resolver` with no `forward server` and no `forward dns-source`; a WAN DHCPv4 lease carries 192.0.2.53 | the LAN resolver forwards to 192.0.2.53; once `forward server 198.51.100.53` is committed it forwards to 198.51.100.53 alone, without rebinding a listener (D-5) | none |
| AC-25 | DHCP subnet with no `dns-server`, no `dns-source`, configured lease time 3600, no usable `received` entry; then a WAN DHCPv4 lease with 192.0.2.53 arrives (D-9) | replies carry lease time 60 and no option 6; after the WAN lease, replies carry lease time 3600 and option 6 192.0.2.53; a subnet with `dns-server` set keeps 3600 throughout | none |
| AC-27 | A WAN lease or the `host` record carries 127.0.0.53 and fe80::1 with no zone, a subnet `dns-server` is absent, and `system name-server` names 127.0.0.1 (D-16) | option 6, RDNSS and the subscriber's IPCP DNS carry neither address, and the next usable entry or class is used; each drop is logged with the consumer path and the address; resolv.conf, Ze's internal resolver and the LAN resolver's upstream list keep 127.0.0.1 and 127.0.0.53 | none |
| AC-26 | `rfc/short/rfc2548.md` (D-8) | `Optional use cases: lns`; every row a verbatim span; the MS-Primary-DNS-Server and MS-Secondary-DNS-Server rows tagged on `TestExtractMSDNSServers` and `TestPoolIPCPDNSFromRadius` with discrimination records; other attributes' rows are gaps; `./le rfc check` green | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1b | Home user sets no DNS; WAN is PPPoE (the router half is 23a story 1) | IPCP lease → registry → event → LAN DHCPv4 clients on a subnet with no `dns-server` get the IPCP servers by the pass-through default (D-5) | `access-up` DNS clause (child 19), extended to option 6 |
| 3 | LNS operator's RADIUS sets a subscriber's DNS, with no DNS config on the pool beyond its `dns-primary` | Access-Accept → `AuthMetadata` → pool default `radius`, `configured`, `received` | interop `radius-subscriber-dns` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOption6PassThroughDefault` | dhcpserver `handler_test.go` | AC-14 | |
| `TestRDNSSPassThroughDefault` | iface `reconcile_ra_test.go` | AC-15 | |
| `TestForwardPassThroughDefault` | dnsresolver `config_tx_test.go` | AC-20 | |
| `TestSelfRefusedOnSystemAndForward` | dnssource `self_refused_test.go` | AC-11, the `self` refusal's what, why and fix parts for both consumers | |
| `TestModeRequiredEachConsumer` | each consumer's config test | AC-16b through the consumer's verifier and its boot parse | |
| `TestForwardListFromSelectionNoRebind` | dnsresolver `config_tx_test.go` | AC-6 | |
| `TestOption6FromDNSSource` | dhcpserver `handler_test.go` | AC-7, and 23a AC-5 for option 6 | |
| `TestRDNSSWithdrawnAdvertisedLifetimeZero` | iface `reconcile_ra_test.go` | AC-8 | |
| `TestExtractMSDNSServers` | authradius `extract_test.go` | parse and R-6 | |
| `TestPoolIPCPDNSFromRadius`, `TestPoolIPCPDNSRadiusAbsent` | pool `register_test.go` | AC-9, AC-10 | |
| `TestDNSSourceClassRefused` | each consumer's config test | AC-11 | |
| `TestSelfIsAddressOnClientInterface` | iface `ra_dns_source_test.go`, pool `dns_source_test.go`; for dhcpserver the `self` case of `TestOption6FromDNSSource` (`handler_dns_source_test.go`) | AC-7, D-6 for each consumer | |
| `TestLeaseShortWhileReceivedUnusable` | dhcpserver `handler_test.go` | AC-25 | |
| `TestSelectDropsHostOnlyAddressesForHandedOutLists` | the core leaf's package test | AC-27, D-16: a handed-out list drops loopback and zone-less IPv6 link-local and falls through; a router list keeps both | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| RDNSS addresses per option | 1..8 | 8 | N/A | 9 (first 8 in list order, logged) |
| option 6 addresses | 1..63 | 63 | N/A | 64 (first 63, logged) |
| DHCPv4 lease time under D-9 | 1..configured | a configured lease below 60 s is kept | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dns-resolver-forward-received` | `test/plugin/dns-resolver-forward-received.ci` | AC-6, AC-13 | |
| `dhcp-server-dns-source-self` | `test/dhcp/dhcp-server-dns-source-self.ci` | AC-7 | |
| `dns-source-class-refused` | `test/parse/dns-source-class-refused.ci` | AC-11, `radius` and `self` under `system dns-source` | |
| `dhcp-server-dns-pass-through` | `test/dhcp/dhcp-server-dns-pass-through.ci` | AC-14 | |
| `dns-resolver-forward-default` | `test/plugin/dns-resolver-forward-default.ci` | AC-20 | |
| `show-system-dns` | `test/plugin/show-system-dns.ci` | AC-12 | |
| `l2tp-pool-dns-source-radius` | `test/l2tp/l2tp-pool-dns-source-radius.ci` | AC-9, AC-10 through the LNS | |
| `dhcp-server-short-lease-wan-down` | `test/dhcp/dhcp-server-short-lease-wan-down.ci` | AC-25 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `radius-subscriber-dns` | `test/interop-radius-subscriber/` | FreeRADIUS replying MS-Primary-DNS-Server; read its `dictionary.microsoft` first | AC-9 on the wire | |
| `access-up` DNS clause | child 19 | Ze CPE, Ze LNS | story 1b | |

## Files to Modify
- `internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang`, and dnsresolver `config.go`, `config_tx.go`, `state.go` - `forward dns-source`
- `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang`, and dhcpserver `config.go`, `handler.go` - subnet `dns-source`
- dhcpserver reply builder - the 60-second lease under D-9
- `internal/component/iface/yang/ze-iface-conf.yang`, and iface `config_ra.go`, `reconcile_ra.go` - `rdnss dns-source`, Lifetime 0 withdrawal
- radius dictionary `dict.go` - MS DNS attribute ids
- `internal/component/l2tp/plugins/authradius/extract.go`, and l2tp `session_metadata.go` - DNS fields
- `internal/component/l2tp/plugins/pool/register.go`, `internal/component/l2tp/plugins/pool/yang/ze-l2tp-pool-conf.yang` - pool `dns-source`
- `docs/architecture/dns/lan-resolver.md`, `docs/guide/dns-resolver.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md`, `docs/features/interfaces.md`
- every consumer `dns-source` container's YANG `description` states that consumer's built-in default list and mode (D-3, D-5), and `docs/guide/dns-resolver.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md` and the DHCP server guide state the pass-through default, the change against today (only a consumer with no configured server changes, and the pool), and the explicit `source configured` that keeps the old behaviour
- Repository users of the changed defaults (D-3), converted or re-validated in the same work: the configs and `.ci` bodies naming `dns-server`, `rdnss` or `dns-primary`/`dns-secondary` (17, 2 and 26 files under `test/` today, including `test/parse/`, `test/l2tp/`, `test/plugin/`, `test/interop/scenarios/dhcpv4-rebind-two-servers-dnsmasq/`, `test/interop/scenarios/dhcpv4-renew-nak-dnsmasq/`, `test/interop/scenarios/dns-resolver-dhcp-lan/`, `test/interop-pppoe/scenarios/`, `test/interop-topology/cpe/`, `test/interop-topology/scenarios/two-ze-pppoe-link/`; the set is re-derived by grep at implementation). Under D-5 a test that sets the configured leaf keeps its answer; each one whose expectation would now change (no configured leaf while a learned DNS is present, or a pool test whose Access-Accept carries MS DNS) gets either its expectation corrected to the pass-through answer, or an explicit `dns-source` stating what it tests; a test that stays green only because no learned source exists is re-checked that it still proves what it claims
- `rfc/short/rfc2548.md` (new, through `/ze-rfc`, `Optional use cases: lns`), `rfc/extraction/rfc2548.json` (D-8)
- `rfc/short/rfc7084.md` (L-11 RDNSS, L-12), `rfc/short/rfc8106.md` (RFC8106-5.1-1, the advertising side)
- `docs/architecture/l2tp/bng-1-radius-attributes.md` (declared by authradius: the MS DNS attributes), `docs/research/l2tpv2-ze-integration.md` (declared by the l2tp code: checked, edited only if it describes subscriber DNS)

## Files to Create
- the `show system dns` command YANG and handler in its owning package (A-5)
- the functional tests named above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `uses dns-source` in the four consumer modules (dnsresolver, dhcpserver, iface RA, l2tp pool), and the `show system dns` command YANG in its owning package (A-5) |
| YANG validation constraints | No | the per-consumer class check and the mode check are verifier calls from the 23a core leaf, not `must` |
| YANG custom validators | No | the class values are the `dns-source-class` enumeration |
| CLI commands/flags | Yes | `show system dns` |
| CLI grammar (keyword before value) | Yes | `source <class>`, `mode <mode>` |
| Editor autocomplete | Yes | enum classes and modes automatic |
| Functional test for new RPC/API | Yes | `test/plugin/show-system-dns.ci` |
| Pipe completeness | Yes | the show answer goes through the pipe processor |
| Env var registration | No | no env var |
| Doctor check for runtime dependencies | Yes | checks that warn when a consumer's selection is empty while its list (written or default) names `received`; when a `received-interface` entry names an interface that holds no record (D-11); and when a `self` address is not served by a `service dns-resolver` listener (D-6, R-9); no file, socket or port is added |
| Prometheus counters/metrics | Yes | `ze_dns_source_records` gauge by origin; `ze_dns_source_selection_changes_total` by consumer |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the DNS and interfaces entries under `features/` (level read from `./le feature report`) |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md`, `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (dnsresolver, dhcpserver, pool) |
| 6 | Has a user guide page? | Yes | `docs/guide/dns-resolver.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md` |
| 7 | Wire format changed? | No | encoders unchanged; only the source of their content |
| 8 | Plugin SDK/protocol changed? | No | the `system-dns` namespace is 23a's |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8106.md` (RFC8106-5.1-1, advertising side), `rfc/short/rfc7084.md` (L-11 RDNSS, L-12), `rfc/short/rfc2548.md` (new, D-8) |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, the VyOS DNS rows |
| 12 | Internal architecture changed? | Yes | `docs/architecture/dns/lan-resolver.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation by `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-23b-dns-source-consumers.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/dns-resolver.md`, the pool examples in `docs/guide/l2tp.md` |

## Implementation Steps

Phase numbers continue 23a's, so references in either file stay valid.

4. **Phase: LAN resolver** - forward `dns-source` over the event, the D-5 default, the `self` refusal, no rebind. AC-6, AC-13, AC-20
5. **Phase: DHCPv4 option 6 and RA RDNSS** - D-5 defaults and explicit lists, `self` per D-6, the 60-second lease of D-9, the D-16 drop. AC-7, AC-8, AC-14, AC-15, AC-25, AC-27 (option 6, RDNSS)
6. **Phase: Subscriber** - MS DNS extraction, pool `dns-source`, default `radius`, `configured`, `received`; `rfc/short/rfc2548.md` through `/ze-rfc` with `Optional use cases: lns`, its walk and records (D-8), the D-16 drop. AC-9, AC-10, AC-26, AC-27 (subscriber), interop
7. **Phase: Conversion** - every repository user of the changed defaults converted or re-validated (Files to Modify), in the same work as phases 5 and 6
8. **Phase: Show, doctor, metrics, docs** - AC-11, AC-12, AC-16b

### Progress

Landed, by phase (each commit's body holds its evidence and the reds it names):

| Phase | Commits | State |
|-------|---------|-------|
| Shared rule (D-16) | fb83d97fbb (a list states its audience; a handed-on list drops host-only addresses) | landed |
| 4 LAN resolver | d66f28fe7c, 23fc8313b6, 292ffc64cb (queries held until the learned replay); fixtures 9836156bf9, be17cde719, 04d21c6207, 5c6105dc87 | landed; D-17 recorded (caches kept on upstream change) |
| 5 DHCPv4 option 6 and RA RDNSS | ae8ad2d589, 83f0f6c683, 130c9ffb9b (RFC 7084 L-11 RDNSS half), 728c515223 | landed |
| 6 Subscriber | 1d36bd5ec6, 1f69bc459c (R-6 log) | code landed; `rfc/short/rfc2548.md` written; its extraction walk, the two tags and their discrimination records owed; the AC-9/AC-10 `.ci` and the interop scenario owed |
| 7 Conversion | inside each phase's commits | re-checked at closure |
| 8 Show, doctor, metrics | fefd30ab74, a3cfaa083f, e31b2ae512 (`show system dns`, every consumer publishes its selection), 311177442c (`show doctor` checks), de1ad423ab (one reader refuses a malformed `dns-source`), 02f90959a4 (metrics) | code landed; the AC-11 consumer `.ci`, AC-12 `test/plugin/show-system-dns.ci`, the AC-16b `.ci` and a doctor `.ci` owed |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the family filter runs before a class can win; no handed-out list carries a loopback or zone-less link-local address (D-16) |
| Naming | JSON keys kebab-case; class names identical in YANG, show output and docs |
| Data flow | plugins read records only from the event |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `show system dns` | `test/plugin/show-system-dns.ci` |
| pass-through defaults on the four consumers | the four pass-through tests and `.ci` files above |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | RADIUS addresses are unicast, carry no line break, and are bounded per consumer (R-6) |
| Loop | `self` on the system consumer and on `forward dns-source`, which would make Ze or its resolver forward to itself, is refused (R-3, D-5) |
| Host-only addresses | no loopback or zone-less link-local address is handed to a LAN client or a subscriber (D-16) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The IPv6 subscriber path hands out no resolver at all: the PPP RA sets O and nothing sends DHCPv6 option 23. Owned by `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md` (D-4); subscriber IPv6 itself is not up yet (`plan/post-release/features/spec-l2tp-ipv6-subscriber.md`).

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `self` is Ze's address on the client's interface (D-6) | the DHCP server identifier, the RA source address, the pool gateway | Thomas decided; a doctor warning covers an address no LAN resolver listener serves (R-9) |
| 60-second DHCPv4 leases while no `received` entry is usable on a subnet whose list names `received` (D-9) | leave lease length alone | Thomas decided; RFC 5625 Section 5.3 names it as an alternative to a DNS proxy |
| IPv6 subscriber DNS is a separate spec after the umbrella (D-4) | in this spec | Thomas: "spec of its own"; subscriber IPv6 does not come up before `spec-l2tp-ipv6-subscriber` anyway |
| The LAN resolver keeps its caches when its upstreams change (D-17) | clear both caches on every forward-list change | dnsmasq's default, whose `--clear-on-reload` is opt-in; `clear dns resolver cache all` stays the operator's tool |

The shared decisions (one registry, the event carrier, the class vocabulary,
the explicit mode, the defaults, the `self` refusal) are 23a's "Key Design
Decisions".

## Known Limitations
- LAN DHCPv6 DNS_SERVERS (RFC 7084 L-10) needs a LAN DHCPv6 server: `post-release/features/spec-dhcpv6-server.md`; its consumer joins this grammar when it lands.
- IPv6 subscriber DNS (RFC 6911 attribute 169, RDNSS in the PPP RA, DHCPv6 option 23): `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md` (D-4).
- accel-ppp's per-option RADIUS DNS overwrite is not reproduced by either mode (23a, VyOS mapping table).
- A LAN DHCPv4 client that holds a received server when the WAN drops keeps it until it renews (RFC 5625 Section 5.3), because FORCERENEW is not implemented; D-9's 60-second leases bound the opposite case, a client that leased while the WAN was down.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code: RFC
8106 Section 5.1 above the Lifetime 0 withdrawal, RFC 2548 Section 2.7.6 above
the MS DNS extraction. RFC 5625 Section 3 ("end-users SHOULD be able to send
their DNS queries to specified upstream resolvers, thereby bypassing the proxy
altogether.") above the pass-through defaults of the DHCPv4 and RA consumers.

### RFC 2548 in this spec (D-8)

| Step | Owed work | Tool |
|------|-----------|------|
| 1 | Write `rfc/short/rfc2548.md` with `Optional use cases: lns`, `Use cases: -`, implementer kind declared, every row a verbatim span of its section | `/ze-rfc` |
| 2 | Extraction walk, forward and reverse, signed off in `rfc/extraction/rfc2548.json` | the RFC extraction tooling |
| 3 | The Section 2.7.6 and 2.7.7 MS DNS rows tagged on the subscriber tests, each with a discrimination record; every other attribute's rows recorded as gaps, never excluded for being unimplemented | `./le rfc record`, `./le rfc check` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (RFC 2548's is written here, D-8)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-6 to AC-15, AC-16b, AC-20, AC-25, AC-26, AC-27 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test functional plugin`, `./le test functional dhcp`, `./le test functional parse`, `./le test functional l2tp`, `./le test integration interop-radius-subscriber`, `./le rfc check` (the RFC 2548 rows), and the scoped package tests of the changed packages through `./le job run`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-home-cpe-23b-dns-source-consumers.md` only, in the same `./le commit create` script

---

## Implementation Summary

Closure prepared 2026-11-09 while four agents still fix the items marked
**pending** below. Evidence logs are under
`tmp/session/2026-11-07-104251/scratch/rwinv/` (`out-gb11/`, `out-gb12/`).

### What Was Implemented
- Shared rule: a list states its audience, and a handed-on list drops loopback and zone-less IPv6 link-local (D-16, fb83d97fbb), and a link-local of another link (D-19, fffefb45f9).
- LAN resolver: `forward dns-source` over the `system-dns` event, the D-5 default, the `self`/`radius` refusal, relearn without rebind, queries held until the learned replay (d66f28fe7c, 23fc8313b6, 292ffc64cb); caches kept on upstream change (D-17); only bound listeners published, no selection while disabled (a5356d64e0).
- DHCPv4 option 6 and RA RDNSS: D-5 defaults, explicit lists, `self` per D-6, the D-9 60-second lease, Lifetime 0 withdrawal (ae8ad2d589, 83f0f6c683, 130c9ffb9b, 728c515223); RDNSS and delegated /64 withdrawal queues bounded (fffefb45f9, ad3cecbf68).
- Subscriber: MS-Primary/Secondary-DNS-Server extraction, pool `dns-source` with default `radius`, `configured`, `received`, IPCP slots filled primary then secondary (D-18) (1d36bd5ec6, 1f69bc459c); a removed pool plugin withdraws its selections (f6f6b0a7dd).
- Operator surfaces: `show system dns` in the new `system-dns-cmd` plugin, every consumer publishes its selection, `show doctor` checks, one reader refusing a malformed `dns-source`, metrics (fefd30ab74, a3cfaa083f, e31b2ae512, 311177442c, de1ad423ab, 02f90959a4); a removed consumer withdraws its selection (ac5ab17dad); rejected-snapshot counter resolved after metrics start (7d8d935e4f).
- RFC: `rfc/short/rfc2548.md` written and walked (59367ae574, `rfc/extraction/rfc2548.json`), MS DNS rows tagged (36f54771b5), records observed red (8d83d8bde0, 83f4316387).
- Tests: four dns-resolver forward-learned `.ci` promoted (67eda8e941); subscriber, show and doctor fixtures and the `radius-subscriber-dns` interop scenario (c85d2718f2).

### Bugs Found/Fixed
Review round 1 (`tmp/session/2026-11-07-104251/scratch/c23b-review-r1.md`) findings and their fixes are in "Findings fixed" under Review Gate.

### Documentation Updates
- 3f5eb284d9: `ai/INDEX.md`, `docs/architecture/api/commands.md`, `docs/architecture/config/syntax.md`, `docs/features/dns-resolver.md`, `docs/guide/command-reference.md` (the `self` key), `docs/guide/plugins.md`, `docs/guide/status.md`.
- Earlier phase commits: `docs/guide/dns-resolver.md` (forward `dns-source` rows), `docs/guide/l2tp.md` line 1163, `docs/guide/pppoe.md` line 177, `docs/guide/configuration.md` (resolver, subnet and pool `dns-source` rows with the `poolDNSDefault, selectSubscriberDNS` anchor), `docs/architecture/dns/lan-resolver.md` line 194, `docs/plugin-development/metrics.md` lines 250-252, `docs/architecture/testing/interop.md` (c85d2718f2).
- **pending:** `docs/comparison.md` has no VyOS DNS-source rows (Doc checklist row 11; its DNS rows are lines 338-339), and `features/lan-dns-resolver.md` names no `dns-source` (row 1). Owed run: `./le doc check all` after both edits.

### Deviations from Plan
- A-5 broke: no owner existed for `show system dns`, so the new plugin `internal/plugins/system-dns-cmd` owns it.
- D-19 added after review: a foreign-zone link-local is dropped on handed-on lists, beyond D-16's zone-less case.
- The dhcpserver `self` case lives in `TestOption6FromDNSSource`, not a separate `TestSelfIsAddressOnClientInterface` (769ec28eca corrected the TDD table).
- `TestDNSSourceClassRefused` exists only in dnsresolver, and `TestModeRequiredEachConsumer` only in config/system, dnsresolver and pool; the DHCP and RA halves of AC-11 and AC-16b rest on the pending parse drafts.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5 assumed an owning command package for `system` operational data | none existed | implementation of phase 8 | `system-dns-cmd` plugin created |
| approach | Phases 4 to 8 landed unit tests only, and the `.ci` and interop proof was left to the end | review round 1 BLOCKER 1: no daemon-level test reached 23b behaviour | independent review | `.ci` written as drafts and promoted one by one (67eda8e941, c85d2718f2, pending rest) |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Point 2: every consumer has the full choice of sources | Done | resolver `parseForwardSources` (`internal/plugins/dnsresolver/config.go`), dhcpserver and iface RA `dns-source`, pool `parsePoolSources` | the router is 23a's |
| Point 3: ordered list with fall-through | Done | `dnssource.Select` (`internal/core/dnssource/select.go`) | one shared rule |
| Point 4: RADIUS DNS passes through | Done | `extractAuthMetadata` (`authradius/extract.go`), `selectSubscriberDNS` (`pool/dns_source.go`) | interop pending |
| Operator surfaces | Partial (pending) | `registerShowSystemDNS`, doctor `dnsSourceFindings`, metrics | `show-system-dns.ci` red |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-6 | Done | `TestForwardListFromSelectionNoRebind`; `dns-resolver-forward-received`, `dns-resolver-forward-ppp-lease` pass clean and red under break (`out-gb12/g12-clean.log`, `c23b-fwd-received.log`, `c23b-ppp-lease.log`) | |
| AC-7 | Partial | `TestOption6FromDNSSource` (self case), `TestSelfIsAddressOnClientInterface` (iface, pool) | `test/dhcp/dhcp-server-dns-source-self.ci` not written; owed |
| AC-8 | Done (unit) | `TestRDNSSWithdrawnAdvertisedLifetimeZero`, RFC8106-5.1-1 record (83f4316387, `rec-RFC8106-5.1-1-pos.log`) | no RA `.ci` in the plan |
| AC-9 | Partial (pending) | `TestPoolIPCPDNSFromRadius`; draft `l2tp-pool-dns-source-radius` passes clean, red under break (`c23b-pool-radius.log`, `c23b-extract-ms-dns.log`) | promotion and the `radius-subscriber-dns` interop pending |
| AC-10 | Partial (pending) | `TestPoolIPCPDNSRadiusAbsent`; draft `l2tp-pool-dns-source-configured` passes clean, red under break (`c23b-pool-configured.log`) | `l2tp-pool-dns-source-received` draft red (105.9 s, `g12-clean.log`) |
| AC-11 | Partial (pending) | `TestSelfRefusedOnSystemAndForward` (0b21d66b9f), `TestDNSSourceClassRefused` | draft `dns-source-class-refused` red: line 10 `expected key or '{' for server, got LBRACKET` (`p-class-clean.log`) |
| AC-12 | Partial (pending) | `internal/plugins/system-dns-cmd/cmd` unit pass (`u-resolver.log`) | draft `show-system-dns` red: `parse config: line 16 ... LBRACKET` (`g12-clean.log`); `TestRegisteredWireMethods` pending |
| AC-13 | Partial (pending) | `dns-resolver-forward-dhcpv4-replay` draft passes clean and red (`c23b-replay.log`, out-gb11); resolver `awaitReplay` | promotion pending; review NOTE 15: dhcpserver answers before the replay, so AC-13 holds strictly for the resolver only |
| AC-14 | Partial | `TestOption6PassThroughDefault` | `test/dhcp/dhcp-server-dns-pass-through.ci` not written; owed |
| AC-15 | Done (unit) | `TestRDNSSPassThroughDefault` (`internal/component/iface/ra_dns_source_test.go`) | |
| AC-16b | Partial (pending) | `TestModeRequiredEachConsumer` (system, dnsresolver, pool) | draft `dns-source-mode-required-consumers` red, same LBRACKET parse (`p-mode-clean.log`) |
| AC-20 | Done | `TestForwardPassThroughDefault`; `dns-resolver-forward-default`, `dns-resolver-forward-dhcpv4-default` pass clean, red under break (`c23b-fwd-default.log`, `c23b-dhcpv4-default.log`) | |
| AC-25 | Partial | `TestLeaseShortWhileReceivedUnusable` | `test/dhcp/dhcp-server-short-lease-wan-down.ci` not written; owed |
| AC-26 | Done | `rfc/short/rfc2548.md` line 22 `Optional use cases | lns`; tags 36f54771b5; records `rfc/discrimination/rfc2548/RFC2548-2.7.6-1.json`, `RFC2548-2.7.7-1.json` (83f4316387, `rec-RFC2548-*.log`) | `./le rfc check` re-run pending (below) |
| AC-27 | Done (unit) | `TestSelectDropsHostOnlyAddressesForHandedOutLists`, the consumer pass-through cases, RFC8106-5.1-2 record | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| unit tests of the TDD table | Done | located by `func` grep: dhcpserver `handler_dns_source_test.go`, iface `ra_dns_source_test.go`, ra `advertiser_rdnss_test.go`, dnsresolver `config_tx_test.go`, dnssource `self_refused_test.go`, `select_test.go`, authradius `extract_dns_test.go`, pool `dns_source_test.go` | all packages pass (`out-gb12/u-*.log`) except the three pending reds below |
| `.ci` promoted | Done | `test/plugin/dns-resolver-forward-received.ci`, `dns-resolver-forward-default.ci`, `dns-resolver-forward-dhcpv4-default.ci`, `test/pppoe/dns-resolver-forward-ppp-lease.ci` | |
| `.ci` drafts | pending | `test/draft/{parse,plugin,l2tp,pppoe}/` | listed under Pre-Commit "Files Exist" |
| `.ci` not written | owed | the three `test/dhcp/` files | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| four consumer YANG modules | Done | each `uses dns-source` |
| `show system dns` YANG and handler | Done | `internal/plugins/system-dns-cmd/` |
| `docs/comparison.md`, `features/lan-dns-resolver.md` | pending | Documentation Updates |

### Audit Summary
- **Total items:** 16 ACs
- **Done:** AC-6, AC-8, AC-15, AC-20, AC-26, AC-27
- **Partial:** AC-7, AC-9, AC-10, AC-11, AC-12, AC-13, AC-14, AC-16b, AC-25, each waiting on a pending fix or an owed `.ci`, none on a scope cut
- **Skipped:** none
- **Changed:** A-5, D-19 (Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Point 2 and 3, LAN resolver: falls through its own list, default pass-through | functional | `dns-resolver-forward-received`, `-default`, `-dhcpv4-default`, `-ppp-lease`: PASS in `out-gb12/g12-clean.log`, FAIL under each break (`c23b-fwd-*.log`, `c23b-dhcpv4-default.log`, `c23b-ppp-lease.log`) |
| Point 2 and 3, DHCPv4 option 6 and RA RDNSS | unit, functional owed | `TestOption6FromDNSSource`, `TestOption6PassThroughDefault`, `TestLeaseShortWhileReceivedUnusable`, `TestRDNSSPassThroughDefault`, `TestRDNSSWithdrawnAdvertisedLifetimeZero` pass (`u-dhcp.log`, `u-iface.log`, `u-ra.log`); the three `test/dhcp/` `.ci` are owed |
| Point 4, RADIUS DNS reaches the subscriber | functional, interop pending | draft `l2tp-pool-dns-source-radius` PASS clean, FAIL under the extract and pool breaks; `radius-subscriber-dns` **pending**: FreeRADIUS container exited 255 before the probe (`out-gb12/interop-dns-clean.log`) |
| Operator sees every consumer's selection | functional pending | `doctor-dns-source` draft PASS clean, red under three breaks (`c23b-doctor-*.log`); `show-system-dns` draft **pending** |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| RFC 7084 L-11 DNSSL half | out of this spec's scope from the start | `plan/post-release/features/spec-router-advertisement-options-out-of-scope.md` |
| IPv6 subscriber DNS (RFC 6911 attribute 169, PPP RA RDNSS, DHCPv6 option 23) | D-4 | `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md` |
| LAN DHCPv6 DNS_SERVERS consumer (RFC 7084 L-10) | needs a LAN DHCPv6 server | `plan/post-release/features/spec-dhcpv6-server.md` |

Every pending item in this closure is in scope and stays here until fixed.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | not recorded yet: the gate waits for the pending fixes |
| `./le spec review check` | not run |
| Rounds | 1 so far (`tmp/session/2026-11-07-104251/scratch/c23b-review-r1.md`: 4 BLOCKER, 8 ISSUE, 5 NOTE) |
| Reviewer lenses used | logic and wiring, RFC quotes, goroutines, D-9 and D-16, docs |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | no `.ci` reached 23b behaviour | Wiring Test rows | 67eda8e941, c85d2718f2; rest **pending** |
| 2 | BLOCKER | AC-26 rows untagged | `rfc/short/rfc2548.md` | 36f54771b5, 83f4316387 |
| 3 | BLOCKER | unbounded RDNSS withdrawal queue silences the unit's RAs | `internal/plugins/iface/ra/advertiser.go` | fffefb45f9, sibling /64 queue ad3cecbf68 |
| 4 | BLOCKER | WAN link-local RDNSS advertised on the LAN | `internal/component/iface/ra_dns_source.go` | fffefb45f9 (D-19) |
| 5 | ISSUE | wire-methods snapshot lacks `ze-system-dns:show-system-dns` | `internal/component/plugin/all/testdata/wire-methods.snapshot` | 67f97a55f4; `TestRegisteredWireMethods` re-run **pending** (`u-wire.log`: build failed) |
| 6 | ISSUE | resolver publishes unbound listeners and a disabled selection | `internal/plugins/dnsresolver/publish.go` | a5356d64e0 |
| 7 | ISSUE | rejected-snapshot counter bound to the Nop registry | dhcpserver and dnsresolver `register.go` | 7d8d935e4f |
| 8 | ISSUE | doctor-dns-source-self-unserved text contradicts D-6 | `internal/core/diagnostic/codes.go` | b525648a71 |
| 9 | ISSUE | documentation drift | Doc checklist rows 2, 4, 5, 15 | 3f5eb284d9; rows 1 and 11 **pending** |
| 10 | ISSUE | a consumer's publication outlives it | `internal/component/config/system/resolv.go` | ac5ab17dad, f6f6b0a7dd |
| 11 | ISSUE | RFC8106-5.1-1 advertising side unproven | `advertiser_rdnss_test.go` | 83f4316387 |
| 12 | ISSUE | spec-named unit tests missing | `internal/core/dnssource` | 0b21d66b9f, 769ec28eca |

NOTEs 13 to 17 recorded in the review file; NOTE 15 bears on AC-13 (above).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/dns-resolver-forward-received.ci`, `test/plugin/dns-resolver-forward-default.ci` | yes | `ls`, 3.6K and 3.3K |
| `test/plugin/dns-resolver-forward-dhcpv4-default.ci`, `test/pppoe/dns-resolver-forward-ppp-lease.ci` | yes | `git ls-files test` |
| `test/parse/dns-source-class-refused.ci`, `test/l2tp/l2tp-pool-dns-source-radius.ci`, `test/plugin/show-system-dns.ci` | no, draft only | `test/draft/parse/`, `test/draft/l2tp/`, `test/draft/plugin/` |
| `test/dhcp/dhcp-server-dns-source-self.ci`, `dhcp-server-dns-pass-through.ci`, `dhcp-server-short-lease-wan-down.ci` | no | `ls test/dhcp` holds none; owed |
| `test/interop-radius-subscriber/scenarios/radius-subscriber-dns/` | yes | `git ls-files`: `ppp-options`, `users`, `ze.conf` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| all | unit packages pass | `out-gb12/u-dnssource.log`, `u-dhcp.log`, `u-iface.log`, `u-ra.log`, `u-ndp.log`, `u-l2tp.log`, `u-resolver.log`, `u-system.log`: `ok` |
| AC-26 | records present | `rfc/discrimination/rfc2548/RFC2548-2.7.6-1.json`, `RFC2548-2.7.7-1.json`, `rfc/discrimination/rfc8106/RFC8106-5.1-1.json`, `RFC8106-5.1-2.json` (83f4316387) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `forward dns-source source received` | `test/plugin/dns-resolver-forward-received.ci` | PASS clean, FAIL under break |
| resolver with no `forward server` and no `dns-source` | `test/plugin/dns-resolver-forward-default.ci` | PASS clean, FAIL under break |
| `system dns-source source self` | `test/draft/parse/dns-source-class-refused.ci` | **pending** |
| Access-Accept with MS DNS | `test/draft/l2tp/l2tp-pool-dns-source-radius.ci` | PASS clean, FAIL under break; promotion **pending** |
| subnet `dns-source source self`, pass-through, D-9 | none | owed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-3 | confirmed | `selectSubscriberDNS` called once per IP request, `pool/register.go` line 317 |
| A-5 | broken | `internal/plugins/system-dns-cmd/cmd/show_dns.go` `registerShowSystemDNS`; Mistake Log row |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| pool default `radius`, `configured`, `received` | `docs/guide/configuration.md` anchor `pool/dns_source.go -- poolDNSDefault, selectSubscriberDNS` | yes |
| metrics series | `docs/plugin-development/metrics.md` lines 250-252 | yes |
| comparison and feature level | `docs/comparison.md`, `features/lan-dns-resolver.md` | **pending** |

### Pending: owed runs before the Review Gate can close
| Item | Owed run |
|------|----------|
| parse drafts `dns-source-class-refused`, `dns-source-mode-required-consumers` | `./le test bgp parse --draft -a`, clean and under each break, then promote to `test/parse/` |
| `l2tp-pool-dns-source-received`, `show-system-dns`, `dns-resolver-forward-dhcpv4-replay`, `l2tp-pool-dns-source-radius`, `-configured`, `doctor-dns-source` | `./le test bgp pppoe --draft -a`, `./le test bgp l2tp --draft -a`, `./le test bgp plugin --draft -a`, then promote |
| three `test/dhcp/` `.ci` | write per `/ze-test`, then `./le test functional dhcp` |
| `radius-subscriber-dns` | `./le test integration interop-radius-subscriber`, clean and under the extract break |
| `TestRegisteredWireMethods` | `./le job run label unit-pkg command go test -tags <ze tags> -run TestRegisteredWireMethods ./internal/component/plugin/all/` |
| hub `TestStreamingOneShotAnswersCarryData` | same form, `./cmd/ze/hub/` |
| fixture `TestEveryDaemonStoppingFixtureDeclaresStopsDaemon` | same form, `./internal/test/fixture/` |
| compound gate (`internal/test/fixture/l2tp_pool_dns_source.go` lines 195, 227, 326) | `./le arch compound-guard check` |
| refuse gate | `./le arch refuse-by-default check` |
| RFC records stale in `out-gb12/rfc-check.log`: RFC7084-4.3-6, RFC7084-4.3-8, RFC9915-6.3-2 (producer-changed) and four `./le rfc move` file renames | `./le rfc check` |
| review round 2 | `/ze-review` over the complete diff, then `./le spec review record` |
