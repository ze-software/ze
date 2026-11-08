# Spec: home-cpe-23-dns-source-fall-through

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Ze's own name lookups never follow learned resolvers: `cmd/ze/hub/main_system.go` `newResolvers` ("Called once at hub startup", from `cmd/ze/hub/main.go`) fixes ONE server, the first static `system name-server`, else the first server `resolveSystemDNS` reads once from resolv.conf. A CPE whose resolvers arrive later by DHCP or IPCP resolves with whatever the file held at boot, never fails over to a second server, and ignores a reload of `name-server`. That is incorrect behaviour on an accepted configuration (P1). The per-consumer source lists, the RADIUS-supplied subscriber DNS and the learned upstreams for the LAN resolver are the feature half; see "Split" below |
| RFC gaps | RFC7084-4.3-12 |
| Depends | Implementation: `spec-home-cpe-13-pppoe-client-ipcp-dns.md` (the IPCP-learned `ppp` lease; the origin registry it describes is already in the tree). A LAN DHCPv6 consumer exists only after `post-release/features/spec-dhcpv6-server.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md. This spec answers the
umbrella's open decision "Learned DNS for LAN clients (child 13)".

RFC gaps, in full: RFC7084-4.3-12 (L-11) is closed for its RDNSS half only,
when a list names a received or system source; the DNSSL half stays with
`post-release/features/spec-router-advertisement-options-out-of-scope.md`.
RFC 5625 (BCP 152) is followed in this spec (owner decision D-2); its
requirement ids do not exist until its summary is written, which is owed work
here ("RFC 5625 in this spec" below).

IPv6 subscriber DNS (RFC 6911 attribute 169, RDNSS in the PPP RA, DHCPv6
option 23 to subscribers) is excluded from this spec by owner decision D-4 and
owned by `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (RFC 5737 and RFC 3849 prefixes, `example.net`).

## Task

Thomas, dictated: "we should have an option which allow people to settle if
they want to use a particular dns or when we go via L2TP, there will be passed
one via RADIUS. So something like that can be passed through. We should
consider DNS-received, DNS-configured, what is being used by the local code. We
want the router itself to use the received or configured DNS, and if we want to
get the end user the configured or received DNS, the full configuration option
should be there. Also we may want to provide the configuration as a backup to
the receive and vice versa. So to have a fall through of the list, to have the
DNS fall flow."

Earlier owner rule (`ai/META-DECISIONS.md`, Goal and scope): RFC 9463 (DNR) is
not in the first release, and learned resolvers are shaped so DNR can extend
them without undoing work.

Read for meaning, the requirement has four points:

| # | Point |
|---|-------|
| 1 | Resolver addresses come from three kinds of source: configured by the operator, received from the network (DHCPv4, DHCPv6, IPCP, RA), and, on the access side, supplied by RADIUS for one subscriber |
| 2 | Several things consume resolver addresses: the router itself, and every path that hands resolvers to an end user (LAN clients, PPP subscribers). Each needs the full choice of sources |
| 3 | Each consumer takes an ordered list of sources and falls through it: configured as a backup to received, or received as a backup to configured |
| 4 | A RADIUS-supplied resolver passes through to the subscriber |

The design delivers one record-shaped resolver registry fed by every source,
and one ordered fall-through list per consumer, evaluated by one shared rule.

### Owner decisions (2026-11-08)

| # | Thomas, as stated | Reading recorded in this spec | Answers |
|---|-------------------|-------------------------------|---------|
| D-1 | "The fall-through rule should be explicitly configured." | Every consumer's `dns-source` container carries a `mode` leaf the operator sets: `first-usable` (the first source with usable entries wins) or `merge` (every source, in list order). The leaf has no YANG default. A list naming more than one source without `mode` is refused at `ze config validate`, commit and boot, with the fix in the message. A one-source list needs no `mode`, because both modes give the same answer for it | O-1 |
| D-2 | "RFC 5625 inside child 23" | RFC 5625 (BCP 152) is followed in this spec, and it is listed for the home-CPE role (the `fttx-edge` profile, which already carries RFC 7084) as decided in scope. Writing its summary and enrolling it is owed work here, through the RFC tooling | O-7 |
| D-3 | "By default the behavior should be to pass the DNS through." | With no `dns-source` config, each downstream consumer passes on the DNS it received rather than substituting Ze's own address or its configured servers: a PPP subscriber gets the RADIUS-supplied DNS, then the pool's configured servers when RADIUS gives none; LAN clients (DHCPv4 option 6, RA RDNSS) get the WAN-learned servers, then the configured ones when nothing was learned. Each built-in default is a stated list with a stated mode, documented and shown, never a hidden choice. **Interpretation flagged for Thomas's confirmation** (P-1, P-2 under "Open owner questions") | O-2 |
| D-4 | "spec of its own ... I don't want to delay it, but I don't want to hold back the [home-CPE] umbrella completion. We need the feature, but not for the test I want to do in my house." | IPv6 subscriber DNS is a separate spec, `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md`, queued after the home-CPE umbrella, which it does not block. This spec excludes it | O-6 |

### Split

`plan/README.md` asks for a split when obligations belong to different release
categories. The router consumer (AC-1 to AC-5) fixes the defect in the Release
reason and is a release fix, together with the router half of the `mode`
refusal (AC-16, AC-17). The other consumers, the RADIUS source, the
pass-through defaults (AC-14, AC-15) and a received RA source are features. The
RFC 5625 work (AC-18, AC-19) binds the LAN resolver and the DHCP server, both
shipped, so a MUST they miss is a defect in an implemented capability and
lands in 23a. At the move to `ready` this spec splits into 23a (registry,
router consumer, RFC 5625, `release/fixes/`, P1) and 23b (the other consumers
and sources, P3, folder per O-9), every AC below landing in exactly one of
them. The design stays in one document until then because the record
shape and the list grammar are one decision.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/system-plugins.md`, `ai/doors/forwarding.md`, `ai/doors/access-security.md`, `ai/doors/core.md`, `ai/doors/hub-engine.md`
  → Constraint: `dnsresolver`, `dhcpserver`, the iface RA sender and the L2TP `pool` plugin can each run in another process; a plain Go call to `system.ProcessResolvers()` from them mutates a disconnected copy and fails silently. Selections reach them as an EventBus event carrying a self-contained value, with a replay request on subscribe (the `system-rib` `best-change` and `replay-request` pair is the precedent)
  → Constraint: a removed config root arrives as `{}`; a consumer whose `dns-source` list is removed reverts to its default list, never to "no change"
  → Constraint: `ze config validate` never runs a live `OnConfigVerify`; every list-grammar check (classes allowed per consumer) is a YANG `must` or an `InProcessConfigVerifier`
  → Constraint (hub door): "Something built only at startup ignores every reload" (`swapAAABundle` precedent). The internal resolver is exactly that today; the fix joins the one reload path `doReload`
- [ ] `docs/architecture/dns/lan-resolver.md` "Forwarding", "Reload"
  → Constraint: `resolver.forward` tries the `forward server` entries in order, one pass, equal time share; a reload that changes only `forward` keeps every socket. A learned-upstream change reuses that publish path (no rebind) and never serves under a half-applied list
- [ ] `docs/features/interfaces.md` row "DNS from DHCP to `/tmp/resolv.conf`, tracked per lease"
  → Decision: `system.Resolvers` stays the only writer of resolv.conf
- [ ] `docs/architecture/l2tp/subscriber-session-model.md`
  → Constraint: subscriber auth, pool and shaper reach a session through `l2tp/subscriber` handler registries shared by L2TP and PPPoE; RADIUS DNS reaches IPCP through `l2tp.AuthMetadata` and the pool plugin, the path Framed-IP and Framed-Pool already take
- [ ] `plan/release/fixes/spec-home-cpe-13-pppoe-client-ipcp-dns.md`
  → Constraint: child 13 owns the client IPCP request, the `ppp` lease and `show interface pppoe-client`; this spec consumes the lease and changes none of it

### RFC Summaries (Scope: config, with protocol consumers)
- [ ] `rfc/short/rfc1877.md` (enrolled; use cases fttx-edge, lns)
  → Constraint: Section 1.1: "This Configuration Option defines a method for negotiating with the remote peer the address of the primary DNS server to be used on the local end of the link." The LNS is that remote peer for a subscriber; the address it Naks with is consumer 5's output
- [ ] `rfc/short/rfc8106.md` (enrolled; optional for fttx-edge)
  → Constraint: RFC8106-5.1-1 "A value of zero means that the RDNSS addresses MUST no longer be used." An advertised RDNSS whose source is withdrawn is sent once with Lifetime 0. The host-side rows (RFC8106-1.2-1, -1.2-2, -5.3.1-x) are gaps; receiving RDNSS is O-4
- [ ] `rfc/short/rfc7084.md` (enrolled; fttx-edge)
  → Constraint: RFC7084-4.3-11 (L-10, DHCPv6 DNS_SERVERS) has no LAN DHCPv6 server to carry it; RFC7084-4.3-12 (L-11) RDNSS is advertised from static config only today; RFC7084-4.3-19 (L-12, SHOULD) asks that WAN-learned options reach the LAN: the `received` class is that path
- [ ] `rfc/short/rfc2132.md` (enrolled; fttx-edge)
  → Constraint: option 6 length is a multiple of 4; the existing encoder holds it, a source only changes which addresses fill it
- [ ] `rfc/short/rfc6911.md` (enrolment out-of-scope, optional for lns)
  → Constraint: Section 3.2: "This Attribute MAY be included multiple times in Access- Accept packets when the intention is for a NAS to announce more than one DNS server address to an RG/host." `extractAuthMetadata` reads none of types 168 to 172. Attribute 169 (IPv6 subscriber DNS) is excluded here by D-4 and owned by `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md`
- [ ] RFC 2548 (no summary; text at `rfc/full/rfc2548.txt`)
  → Constraint: Section 2.7.6: "The MS-Primary-DNS-Server Attribute is used to indicate the address of the primary Domain Name Server (DNS) [16, 17] server to be used by the PPP peer." No profile lists it, so no summary is written (`ai/META-DECISIONS.md`); listing it for `lns` is O-5
- [ ] RFC 3646 (no summary; text at `rfc/full/rfc3646.txt`)
  → Constraint: Section 6, quoted above `system.Resolvers`: "Where DNS parameters such as the domain searchlist or DNS servers have been manually configured, these parameters SHOULD NOT be overridden by DHCP." It fixes the router default order (configured before received)
- [ ] RFC 5625 (BCP 152; text at `rfc/full/rfc5625.txt`; no summary yet, written by this spec)
  → Decision (D-2): followed in this spec and listed for `fttx-edge` as in scope. Its summary, extraction walk and profile enrolment are owed here ("RFC 5625 in this spec")
  → Constraint: Section 3: "Except when required to enforce an active security or network policy (such as maintaining a pre-authentication "walled garden"), end-users SHOULD be able to send their DNS queries to specified upstream resolvers, thereby bypassing the proxy altogether." The pass-through defaults (D-3) of the DHCPv4 and RA consumers are this sentence: a LAN client is handed the upstream servers, not Ze's address
  → Constraint: Section 5.1: "It is therefore RECOMMENDED that gateways SHOULD support device- administrator configuration of values for the "Domain Name Server" DHCP option." The `configured` class and the explicit list keep it: an operator who wants `dns-server` to win writes it first
  → Constraint: Section 5.3: "If the DHCP lease is obtained whilst the WAN link is down, then the DHCP client (and hence the DNS client) will not receive the correct values until the DHCP lease is renewed." Under the pass-through default a LAN client that leased before the WAN came up holds the configured servers (or none) until it renews; R-2 and O-8 carry it
  → Constraint: Section 5.2: "it is RECOMMENDED that the default value for this option SHOULD be empty, and that this option MUST NOT be sent to clients when no value is configured." Binds the DHCP server's option 15 (`domain-name`), audited in AC-19
  → Constraint: Sections 4.1 to 4.5 and 6.1 to 6.3 bind a DNS proxy: "proxies MUST ignore any unknown DNS flags and proxy those packets as usual" (4.1), "DNS proxies MUST therefore be prepared to receive and forward queries over TCP" (4.4.1), "As per Section 4.1, proxies MUST NOT refuse to proxy such packets" (4.4.2, the OPT RR), "proxies SHOULD be capable of forwarding UDP packets up to a payload size of at least 4096 octets" (4.4.3), "The DNS proxy in a gateway SHOULD NOT, by default, be accessible from the WAN interfaces of the device" (6.2). Ze's LAN resolver is a caching forwarder that originates its own upstream query (`docs/architecture/dns/lan-resolver.md` "The upstream query": OPT at 1232 octets, DO clear, client CD never forwarded), so which proxy rows bind it, and how, is decided by the extraction walk against the RFC text, never assumed here (AC-18)
- [ ] RFC 9463 (DNR; not in the first release; text not in the tree)
  → Constraint: the record reserves name, priority and transport fields so a DNR source adds records without changing the record or the list grammar (A-4)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/resolv.go` (224L) - `RegisterResolverOrigin(name, rank)` registry, panics on a duplicate name or rank; `ResolverOrigin.Source(iface, unit)`; `Resolvers` is the only resolv.conf writer. `servers()` returns the static `name-server` list alone when non-empty, else every lease ordered by origin rank, interface, unit, de-duplicated. `SetLease` with an empty list withdraws. A lease is a list of strings only: no lifetime, no expiry, no family. `ProcessResolvers()` is a process global
- [ ] `internal/plugins/iface/dhcp/register.go` - registers origins `dhcpv4` (100) and `dhcpv6` (200); the client is built in-process by `iface.SetDHCPClientFactory` and handed `system.ProcessResolvers()`. Its `resolv_linux.go` sets DHCPv4 option 6 and DHCPv6 option 23 leases (option 23 is requested in the DHCPv6 exchange)
- [ ] `internal/component/iface/register.go` - `pppoeClientResolverOrigin` (`ppp`, 300); the PPPoE client sets the IPCP lease (child 13)
- [ ] iface package `iface.go` `DHCPPayload` (read by symbol) - the `interface/dhcp/lease-*` events already carry `dns-all`, `lease-time` and `received`
- [ ] `cmd/ze/hub/main_system.go` `newResolvers` and `cmd/ze/hub/main.go` - the internal resolver gets `Server = NameServers[0]` at startup only; `ProcessResolvers().Configure` runs at startup and on reload, the internal resolver is never rebuilt
- [ ] `internal/component/resolve/dns/resolver.go` `NewResolver` - one `server` string; when empty, `resolveSystemDNS` reads the first server of the resolv.conf path, else `/etc/resolv.conf`, once; `query` has no failover
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` `name-server` - its description already promises "its own resolver asks these servers" (plural), which the code does not do
- [ ] `internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang` - `service dns-resolver forward server` (ordered-by user, max 8, `ip`, `port`); its container description says LAN hosts learn the resolver from DHCPv4 option 6, RDNSS or IPCP, and nothing puts it there automatically
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - subnet `dns-server` leaf-list (IPv4, max 63, order kept). No DHCPv6 server and no DHCP server show command exist
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` `router-advertisement rdnss { server (max 8); lifetime }` - static only
- [ ] `internal/component/l2tp/plugins/pool/register.go` - IPCP DNS for a subscriber is `pool.dnsPrimary`/`dnsSecondary` of the default or Framed-Pool named pool, also on the Framed-IP path
- [ ] `internal/component/l2tp/plugins/pool/yang/ze-l2tp-pool-conf.yang` - `l2tp pool ipv4 dns-primary|dns-secondary`, same under `named-pool`
- [ ] `internal/component/l2tp/plugins/authradius/extract.go` `extractAuthMetadata` - reads Framed-IP, netmask, Framed-Pool, timeouts, Filter-Id, Framed-Route, IPv6 prefixes; no DNS attribute. The radius dictionary defines `VendorMicrosoft = 311` but no MS DNS attribute ids
- [ ] `internal/component/l2tp/ppp/ra_send.go` `raSender.send` - the subscriber RA sets M and O and carries no RDNSS; the PPP DHCPv6 code defines `D6OptDNSServers` (23) and nothing sends it, so an IPv6 subscriber is told to fetch other configuration and gets no resolver
- [ ] accel-ppp 06f64b1 (2026-03-16) `accel-pppd/radius/radius.c` `rad_proc_attrs` and `accel-pppd/ppp/ipcp_opt_dns.c` `ev_dns`: MS-Primary-DNS-Server and MS-Secondary-DNS-Server from the Access-Accept overwrite the configured `[dns]` value, each option independently
- [ ] vyos-1x 611550d23 (2026-09-01): `src/conf_mode/system_host-name.py` `get_config`: `system name-server` takes an IP or an interface name; an interface name adds the `dhcp-<if>` and `dhcpv6-<if>` hostsd tags (servers learned there) in list order. `src/etc/ppp/ip-up.d/98-vyos-pppoe-setup-nameservers`: PPPoE peer DNS is stored under tag `dhcp-<if>` unless `no-peer-dns`. `src/conf_mode/service_dns_forwarding.py` `apply`: recursor tags in order: its own `name-server`, then `system` if set, then `dhcp-<if>`/`dhcpv6-<if>` for each `dhcp <if>`. `data/templates/accel-ppp/config_name_server.j2`: PPPoE/L2TP server `name-server` writes accel-ppp `[dns]` and `[ipv6-dns]`

**Behavior to preserve:**
- With no new config, the router (resolv.conf) and the LAN resolver's upstreams select exactly what they select today (default lists below)
- With no new config and no learned or RADIUS DNS present, DHCPv4 option 6, RA RDNSS and subscriber IPCP carry exactly what they carry today, because their pass-through defaults fall through to `configured`
- resolv.conf written only by `system.Resolvers`; a source withdraws only its own servers
- Origin rank order DHCPv4, DHCPv6, PPP inside the received class
- The LAN resolver keeps its sockets when only its upstream list changes

**Behavior to change:**
- Ze's internal resolver follows the router selection live, every server in order
- Each consumer gains a `dns-source` container: an ordered `source` list and an explicit `mode` (D-1); learned records carry an expiry and are dropped when stale
- A `dns-source` list naming more than one source without `mode` is refused at validate, commit and boot (D-1)
- Pass-through by default (D-3): with no `dns-source` config, LAN clients receive the WAN-learned servers before the subnet's or unit's configured ones, and a PPP subscriber receives the RADIUS MS-Primary/Secondary-DNS-Server before the pool's. Today the DHCP server hands out only its configured `dns-server`, the RA only its configured `rdnss server`, and the pool only its `dns-primary`/`dns-secondary`. Every repository user of those leaves is converted or re-validated in the same work (`ai/META-DECISIONS.md`, "A change to what Ze does"), listed under "Files to Modify"

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| Resolver origin registry | `internal/component/config/system/resolv.go` `RegisterResolverOrigin`, `ResolverOrigin`, `ResolverSource` | each lease protocol registers a name and rank from its own package | extends: each record gains family, lifetime and the reserved DNR fields; registration stays the only way a protocol joins |
| resolv.conf writer | `resolv.go` `Resolvers.SetLease`, `servers`, `flush` | static list wins, else leases by rank | extends: `servers()` becomes the router consumer's evaluation of its list by the shared rule; the built-in default `configured, received` with mode `first-usable` reproduces today's output byte for byte, because today's rule (static list alone when non-empty, else every lease) is exactly first-usable over those two classes |
| Internal resolver | `cmd/ze/hub/main_system.go` `newResolvers`; `internal/component/resolve/dns/resolver.go` `NewResolver`, `resolveSystemDNS` | one server fixed at startup | replaces: the single server and `resolveSystemDNS` are deleted (no layering); the resolver holds the ordered router selection, updated on every change and reload, and tries each in order within its timeout |
| DHCP lease events | iface `TopicDHCPLease*`, `DHCPPayload.DNSAll` | announce leases with their DNS on the EventBus | does not apply as the carrier: the registry publishes after merge and expiry, so no consumer re-derives lease state; the events stay for their existing readers |
| Cross-process delivery | `internal/core/events` `RegisterNamespace`, `events.Register[T]`; `system-rib` `best-change` + `replay-request` | typed events to in-process and external subscribers | extends: a new namespace carries the record set and the selections, with replay on subscribe |
| LAN resolver upstreams | dnsresolver `resolver.forward`, `resolverInstaller.install` (`docs/architecture/dns/lan-resolver.md`) | forwards to `forward server` in order | extends: the forward list is the evaluated list; `configured` is `forward server`; a change republishes without rebinding |
| DHCPv4 option 6 | `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` `dns-server` | static per subnet | extends: option 6 is the subnet's evaluated list; `configured` is `dns-server` |
| RA RDNSS | `internal/component/iface/yang/ze-iface-conf.yang` `rdnss`; the ndp encoder `BuildRA` | static per unit, one option, max 8 | extends: RDNSS is the unit's evaluated IPv6 list; a withdrawn address is advertised once with Lifetime 0 |
| Subscriber IPCP DNS | `internal/component/l2tp/plugins/pool/register.go` IP response; `l2tp.AuthMetadata` | pool `dns-primary`/`dns-secondary` | extends: the evaluated list fills primary then secondary; `radius` reads two new `AuthMetadata` fields |
| RADIUS Access-Accept extraction | `internal/component/l2tp/plugins/authradius/extract.go` `extractAuthMetadata` | Framed-* attributes into `AuthMetadata` | extends: vendor 311 types 28 and 29 (type 169 is D-4's separate spec) |
| Ranked registration precedent | aaa `Backend.Priority`, `backendRegistry.Build` | producers declare a rank, the consumer sorts | the shape the origin rank already copies; unchanged |
| IPCP-learned lease | child 13 | PPPoE client leases IPCP DNS under origin `ppp` | does not apply: consumed as a received record, unchanged |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A source changes: a DHCPv4, DHCPv6 or IPCP lease is set, renewed, withdrawn or expires; a commit changes `system name-server`, a consumer's own configured list or a `dns-source` list; a RADIUS Access-Accept arrives for a subscriber (per session, not registry state)

### Transformation Path
1. The producer calls `SetLease` with addresses and, new, a lifetime (DHCP valid lifetime; IPCP: until withdrawn; RDNSS: option lifetime)
2. The registry stores a record (Record shape) and schedules its expiry in one long-lived sweeper; at expiry the record is withdrawn as if the producer had sent an empty list
3. After any change the registry evaluates the router list, rewrites resolv.conf (unchanged writer), hands the ordered list to the internal resolver, and publishes one event carrying the full record set and the router selection
4. Each consumer plugin subscribes (replay on start), evaluates its own list over the records with the shared rule, filters by its family, and applies: dnsresolver swaps its forward list; dhcpserver uses it on the next reply; the RA sender sends an unsolicited RA with the new RDNSS and Lifetime 0 for withdrawn addresses; the pool uses it for the next IPCP response
5. Each consumer publishes its selection (consumer, servers, class used) on the same namespace; `show system dns` renders the records and every selection received
6. Subscriber path: `extractAuthMetadata` stores MS-Primary and MS-Secondary in `AuthMetadata`; the pool's evaluation uses them where its list names `radius`

### Record shape

| Field | Type | Description |
|-------|------|-------------|
| origin | registered origin | `dhcpv4`, `dhcpv6`, `ppp`, later `ra` and `dnr`, each registered by its producer |
| interface, unit | string | the lease owner, as today |
| addresses | ordered list of IP | server order as received; family is per address |
| received | time | when the producer last refreshed the record |
| expires | time or none | absolute end of validity; none means valid until withdrawn |
| priority | uint16 | DNR Service Priority; 0 means not a DNR record (reserved) |
| name | string | DNR authentication domain name; empty for plain DNS (reserved) |
| transport | set of names | empty means DNS over UDP and TCP port 53; DNR adds DoT, DoH, DoQ (reserved) |
| port, path | uint16, string | DNR SvcParams port and dohpath (reserved) |

A consumer that speaks plain DNS only skips a record whose `transport` is not empty; before DNR none exists.

### Source classes (the list grammar)

| Class | Meaning | Allowed in |
|-------|---------|-----------|
| `configured` | the consumer's own static list: router `system name-server`; resolver `forward server`; DHCP subnet `dns-server`; RA `rdnss server`; pool `dns-primary`, `dns-secondary` | every consumer |
| `received` | every unexpired learned record, by origin rank, interface, unit; an optional `interface` leaf-list narrows it to named interfaces (the VyOS `name-server <if>` and `dhcp <if>` forms) | every consumer |
| `system` | the router consumer's current selection | every consumer except the router |
| `self` | Ze's LAN resolver address on the consumer's link (O-3 fixes which address) | DHCP subnet, RA, pool; the router only when the resolver listens on loopback |
| `radius` | the subscriber's MS-Primary/Secondary-DNS-Server (IPv4 only; IPv6 subscriber DNS is D-4's separate spec) | pool only |

A source's entries are **usable** when they are unexpired, of the consumer's
family, and pass the validity check (unicast, not unspecified, no line break:
R-6 and the `appendServers` guard). The selection rule is one pure function in
a core leaf shared by every consumer, and it takes the list and the mode
(D-1):

| Mode | Rule | Answer for a one-source list |
|------|------|------------------------------|
| `first-usable` | walk the list in order; the first source with at least one usable entry wins, and its entries are the answer, in their order | that source's usable entries |
| `merge` | walk the list in order and append every source's usable entries, a server already listed not repeated | that source's usable entries |

The consumer then applies its own bound (8 RDNSS addresses, 63 for option 6,
two IPCP slots filled primary then secondary), keeping list order and logging
what it drops. An empty answer means the consumer offers none, which is what
an absent leaf means today. Under `merge` the router hands every answer
to the internal resolver, so a configured backup is tried at query time when
the received servers do not answer; under `first-usable` it is used only once
no received entry is usable.

### When a `mode` is required (D-1)

| `dns-source` config | `mode` | Verdict |
|---------------------|--------|---------|
| container absent | n/a | the consumer's built-in default (next table): a stated list with a stated mode |
| one `source` | absent or set | accepted; both modes give the same answer, so nothing is chosen silently. A set `mode` is kept, so adding a second source later needs no new leaf |
| two or more `source` entries | set | accepted |
| two or more `source` entries | absent | refused at `ze config validate`, commit and boot |

The refusal text, for the router consumer (each consumer names its own path):

| Part | Text |
|------|------|
| What | `system dns-source names 2 sources (configured, received) and no mode` |
| Why | `Ze cannot tell whether a later source is a backup or an addition` |
| Fix | `set system dns-source mode first-usable (a later source is used only when the earlier ones have no usable server), or set system dns-source mode merge (every source, in list order)` |

The check is one function in the core leaf beside the rule. Each consumer
calls it from its `InProcessConfigVerifier` (so `ze config validate` refuses),
from its `OnConfigVerify` (commit and reload) and from the config parse its
startup runs (boot), because the in-process verifier does not run at daemon
startup (`ai/doors/config-yang.md`, Traps). The same three call sites carry the
per-consumer class check (AC-11).

### Built-in default lists (D-1, D-3)

A default applies only while the consumer's `dns-source` container is absent.
Once the operator writes any `source`, the default is gone entirely: it never
lends its `mode` to an operator-written list. Each default is written in the
YANG `description` of the consumer's `dns-source` container, in the operator
guide, and shown by `show system dns` with the word `default` beside it, so no
mode choice is hidden.

| Consumer | YANG home | Default list | Default mode | Against today |
|----------|-----------|--------------|--------------|---------------|
| 1 Router (resolv.conf and Ze's own lookups) | `system dns-source` | `configured`, `received` | `first-usable` | same output (RFC 3646 Section 6 order) |
| 2 LAN resolver upstreams | `service dns-resolver forward dns-source` | `configured` | n/a (one source) | same (P-2 asks Thomas to confirm the pass-through reading leaves it so) |
| 3 DHCPv4 option 6 | `service dhcp-server shared-network subnet dns-source` | `received`, `configured` | `first-usable` | changes: WAN-learned IPv4 servers are handed out while one is usable; `dns-server` only when none is (P-1) |
| 4 RA RDNSS | `interface ... unit ... ipv6 router-advertisement rdnss dns-source` | `received`, `configured` | `first-usable` | changes: WAN-learned IPv6 servers are advertised while one is usable; `rdnss server` only when none is (P-1) |
| 5 Subscriber IPCP (LNS and PPPoE server) | `l2tp pool ipv4 dns-source`, `l2tp pool named-pool dns-source` | `radius`, `configured` | `first-usable` | changes: an Access-Accept's MS-Primary/Secondary-DNS-Server are offered; the pool's `dns-primary`/`dns-secondary` only when RADIUS gives none |

`first-usable` is the default mode of every multi-source default because it is
the literal reading of "backup" in the Task and of "pass through": the
received value is handed on as received, and the backup appears only when
nothing was received. With no learned or RADIUS DNS present, consumers 3 to 5
fall through to `configured` and match today byte for byte (AC-5).

YANG shape: one grouping declared once in `ze-system-conf.yang` and `uses`d by
each consumer module. It defines a container `dns-source` holding a leaf
`mode` (enumeration `first-usable`, `merge`; no default) and an
ordered-by-user `list source` keyed by `class` (enumeration of the five
classes), with an optional `leaf-list interface` valid only under `received`.
The `set` forms read `set system dns-source source received`,
`set system dns-source source configured`,
`set system dns-source mode first-usable`. The class and mode checks are the
verifier calls above, not YANG `must`: no `-conf` module in the tree uses
`must` today, and a verifier gives the refusal its fix text. The class
vocabulary is closed on purpose: it names roles of a source, not protocols.
Protocols join through `RegisterResolverOrigin` and appear inside `received`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Producers to registry | `SetLease` in the hub process (DHCP client via the iface factory, PPPoE client in iface), as today | No |
| Registry to internal resolver | the hub hands the router selection to the `resolve/dns` resolver on change and on reload (both in the hub process) | No |
| Registry to plugin consumers | EventBus namespace (`system-dns` proposed) with record-set and selection events and a replay request; JSON kebab-case | No |
| RADIUS to pool | two new `l2tp.AuthMetadata` fields, read by the pool as Framed-Pool is | No |

### Changed-Wire Census
| Wire | Role (writer / reader) | Carrier | Source evidence | Migration or preservation disposition |
|------|------------------------|---------|-----------------|---------------------------------------|
| resolv.conf | writer | `resolv.go` `flush` | sole writer | preserve: identical bytes under default lists |
| DHCPv4 option 6 | writer | dhcpserver reply builder | static leaf-list today | content changes under the pass-through default while a received IPv4 server is usable (D-3); identical when none is; encoding unchanged |
| RA RDNSS option | writer | ndp `BuildRA` | static today | content changes under the pass-through default while a received IPv6 server is usable (D-3); identical when none is; adds the Lifetime 0 withdrawal |
| IPCP options 129/131 from the LNS | writer | pool IP response | pool leaves today | content changes under the pass-through default when the Access-Accept carries MS DNS (D-3); identical when it does not |
| RADIUS vendor 311 types 28/29 | reader | `extractAuthMetadata` | not read today | new read |
| EventBus `system-dns` | writer and reader | registry, consumers | new | new; the plugin protocol owes no backward compatibility before release |

### Integration Points
- `system.RegisterResolverOrigin` (signature unchanged); `Resolvers.SetLease` gains a lifetime, both callers updated in the same change
- child 13's lease: lifetime "until withdrawn"
- `show system dns` (new): records (origin, interface, unit, addresses, expires) and selections (consumer, list, mode, `default` when the built-in default applies, class or classes used, servers), on every command surface

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | resolv.conf still written only by `Resolvers`; consumers read records only from the event |
| No unintended coupling | Yes | plugins never touch the registry's state; they receive values over the bus |
| No duplicated functionality | Yes | one selection rule shared by every consumer; the single-server resolver path is deleted |
| Zero-copy preserved where applicable | Yes | option encoders unchanged; selection runs on a lease or config change, never per packet or per query |
| Registration over hardcoding, outbound | Yes | each consumer owns its list in its own YANG and publishes its own selection; `show system dns` renders whatever selections arrive |
| Registration over hardcoding, inbound | Yes | a new learned protocol registers an origin; the registry, the rule and the show never name it. Searched: `RegisterResolverOrigin` callers (`internal/plugins/iface/dhcp/register.go`, `internal/component/iface/register.go`), `compareResolverSources`, `servers()` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Every lease producer runs in the hub process | the DHCP client is built by `iface.SetDHCPClientFactory`; the PPPoE client lives in iface | a forked producer's `SetLease` writes a dead copy | `./le plugin boundary check` and a unit test | unvalidated |
| A-2 | First-class-wins is what "backup" means | Thomas: "provide the configuration as a backup to the receive and vice versa" | operators expect merge | owner answer D-1: the operator states the mode, so no reading is imposed; `first-usable` is the default mode of the built-in lists only | validated (D-1) |
| A-3 | A subscriber keeps the DNS negotiated at IPCP until it renegotiates | IPCP has no unsolicited update; LNS renegotiation is child 15 | none | RFC 1661 reading | unvalidated |
| A-4 | RFC 9463 records fit the reserved fields | DNR carries a name, priority, addresses and SvcParams (from memory; the text is not in the tree) | the record grows a field at DNR time | fetch `rfc/full/rfc9463.txt` and compare before `ready` | unvalidated |
| A-5 | `show system dns` has an owning command package for `system` operational data | not yet checked | a new `*-cmd` plugin is owed | `./le cli ownership` at implementation | unvalidated |
| A-6 | The internal resolver may hold several servers with no public contract change | `resolve/dns` is internal | none | grep the callers of `ResolverConfig.Server` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Default lists change today's output | golden diff of resolv.conf, option 6, RDNSS, IPCP | AC-5 asserts byte equality under defaults |
| R-2 | A LAN DHCP client keeps a received server after the WAN source is gone, or keeps the configured one after the WAN comes up (RFC 5625 Section 5.3). The pass-through default (D-3) makes this the common case, not an opt-in | stale option 6 in a client lease | O-8; the guide states the lease-length interaction |
| R-8 | A box that is not a CPE (a PXE provisioning server with `dns-server` set, itself a DHCP client on its management port) starts handing its management lease's resolvers to the provisioned hosts under the pass-through default | option 6 differs from `dns-server` on a provisioning box | P-1 asks Thomas which reading he meant; until he answers, the guide names the explicit `dns-source source configured` that keeps today's behaviour, and the conversion of repository configs keeps every existing expectation |
| R-3 | A loop: router `self` while the LAN resolver forwards to `system` | the resolver forwarding to itself | a `must` refuses the pair; unit test |
| R-4 | One timer per record leaks goroutines | goroutine count grows with leases | one long-lived sweeper (`ai/rules/goroutine-lifecycle.md`) |
| R-5 | A consumer in another process misses the first selection | empty upstream list after restart | replay on subscribe; AC-13 |
| R-6 | RADIUS sends a non-unicast or 0.0.0.0 DNS | subscriber gets an unusable resolver | the same validity check as `isValidSubscriberIP`; invalid values ignored with a log line |
| R-7 | IPv6 addresses offered to an IPv4 consumer | option 6 with an IPv6 value | the rule filters by family before a class can win |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | name resolution on the router, LAN clients or subscribers |
| How is it reverted? | single commit revert per phase; config leaves are additive |
| Who else touches this path? | child 13 (lease producer), child 8 (RA on the delegated prefix), child 2 (DHCP subnets), `spec-dhcpv6-server.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| DHCPv4 lease with option 6, no `name-server` | → | registry, router selection, internal resolver | `test/plugin/dns-source-router-follows-lease.ci` |
| `system dns-source` sources `received`, `configured`, mode `first-usable`, both present | → | selection rule | `TestRouterSourceOrderReceivedFirst` |
| `system dns-source` sources `received`, `configured`, mode `merge` | → | selection rule, internal resolver order | `TestRouterSourceMerge`, `test/plugin/dns-source-router-merge.ci` |
| `system dns-source` with two sources and no `mode`, through `ze config validate` and `ze start` | → | the core-leaf mode check from the verifier and the boot parse | `test/parse/dns-source-mode-required.ci` |
| `service dns-resolver forward dns-source source received` | → | dnsresolver forward list from the event | `test/plugin/dns-resolver-forward-received.ci` |
| subnet `dns-source source self` | → | dhcpserver option 6 | `test/dhcp/dhcp-server-dns-source-self.ci` |
| subnet with `dns-server` and no `dns-source`, WAN DHCPv4 lease with option 6 | → | dhcpserver option 6 under the pass-through default | `test/dhcp/dhcp-server-dns-pass-through.ci` |
| RA `rdnss dns-source source received`, then the lease withdrawn | → | RA sender, Lifetime 0 | `TestRDNSSWithdrawnAdvertisedLifetimeZero` |
| Access-Accept with MS-Primary-DNS-Server, pool with no `dns-source` | → | `extractAuthMetadata`, pool response under the default `radius`, `configured` | `TestPoolIPCPDNSFromRadius`, `test/l2tp/l2tp-pool-dns-source-radius.ci`, interop scenario `radius-subscriber-dns` under `test/interop-radius-subscriber/` |
| A LAN client query to the LAN resolver over TCP, and with an OPT RR | → | dnsresolver listener and forward path | `TestRFC5625ProxyTCPAndOPT` (dnsresolver), plus each further row the RFC 5625 walk binds (AC-18) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | No `name-server`; a DHCPv4 lease arrives after start with 192.0.2.53, 192.0.2.54 | Ze's internal resolver queries 192.0.2.53, then 192.0.2.54 when the first does not answer | none |
| AC-2 | `name-server` changed by reload | the internal resolver uses the new list without a restart | none |
| AC-3 | A lease expires with no renewal | its servers leave resolv.conf and the internal resolver at expiry, and the next class in the list is used | none |
| AC-4 | `system dns-source` sources `received`, `configured`, mode `first-usable`, both present | received servers are used; configured only when no received entry is usable | none |
| AC-5 | No consumer has a `dns-source` container, and no learned or RADIUS DNS exists | resolv.conf, option 6, RDNSS and IPCP bytes equal today's | none |
| AC-6 | `forward dns-source source received` with a `ppp` lease | the LAN resolver forwards to the IPCP servers; withdrawing the lease moves it to the next class without rebinding a listener | none |
| AC-7 | DHCP subnet `dns-source source self` | option 6 carries the LAN resolver address chosen under O-3 | none |
| AC-8 | RA `rdnss dns-source source received` with a DHCPv6 lease carrying 2001:db8::53 | RDNSS carries 2001:db8::53 with a Lifetime not above the lease's remaining valid lifetime; on withdrawal one RA carries it with Lifetime 0 | none |
| AC-9 | Access-Accept with MS-Primary-DNS-Server 198.51.100.53, pool with no `dns-source` (default `radius`, `configured`, `first-usable`) | the subscriber's IPCP Configure-Nak offers 198.51.100.53 as primary | none |
| AC-10 | Access-Accept with no DNS attribute, same pool | the pool's `dns-primary` is offered | none |
| AC-11 | A list naming a class its consumer refuses (`radius` under `system dns-source`) | `ze config validate`, commit and boot refuse it, naming the path and the allowed classes | none |
| AC-12 | `show system dns` | answers records and every consumer's selection with its list, mode, the `default` marker where the built-in default applies, and the class used, on every command surface | none |
| AC-13 | A consumer plugin restarts while leases exist | it applies the current selection from the replay before answering | none |
| AC-14 | DHCP subnet with `dns-server 198.51.100.1` and no `dns-source`; a WAN DHCPv4 lease carries 192.0.2.53 | option 6 carries 192.0.2.53; once the lease is withdrawn or expires, the next reply carries 198.51.100.1 | none |
| AC-15 | RA unit with `rdnss server 2001:db8:1::53` and no `dns-source`; a WAN DHCPv6 lease carries 2001:db8::53 | RDNSS carries 2001:db8::53; on withdrawal one RA carries it with Lifetime 0 and the next carries 2001:db8:1::53 | none |
| AC-16 | Any consumer's `dns-source` with two sources and no `mode` | `ze config validate`, commit and `ze start` refuse it with the what, why and fix text above, naming that consumer's path; the same list with `mode` set is accepted; a one-source list without `mode` is accepted | none |
| AC-17 | `system dns-source` sources `received`, `configured`, mode `merge`, lease 192.0.2.53, `name-server 198.51.100.53` | resolv.conf lists 192.0.2.53 then 198.51.100.53; the internal resolver queries 198.51.100.53 when 192.0.2.53 does not answer | none |
| AC-18 | The RFC 5625 summary's rows that bind the LAN resolver (Sections 4 and 6, as the walk decides) | each bound MUST is met by the LAN resolver and has a test that a recorded break turns red (`./le rfc record`); a SHOULD it misses is reported in the optional figure | none |
| AC-19 | DHCP subnet with no `domain-name` | no reply carries option 15 (RFC 5625 Section 5.2) | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Home user sets no DNS; WAN is PPPoE | IPCP lease → registry → resolv.conf and internal resolver; LAN DHCPv4 clients get the IPCP servers by the pass-through default | `access-up` DNS clause (child 19), extended to Ze's own lookups and to option 6 |
| 2 | Operator prefers own resolvers, ISP's as backup | `system dns-source` sources `configured`, `received`, mode `first-usable` | `TestRouterSourceOrderConfiguredFirst` and `test/plugin/dns-source-router-follows-lease.ci` |
| 3 | LNS operator's RADIUS sets a subscriber's DNS, with no DNS config on the pool beyond its `dns-primary` | Access-Accept → `AuthMetadata` → pool default `radius`, `configured` | interop `radius-subscriber-dns` |
| 4 | Operator writes two sources and forgets the mode | `ze config validate` refuses with the fix | `test/parse/dns-source-mode-required.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSelectFirstNonEmptyClass` | the core leaf's package test | `first-usable`: rule order, family filter, validity filter, empty result | |
| `TestSelectMergeInOrderDeduplicated` | the core leaf's package test | `merge`: list order kept, a repeated server listed once, family filter | |
| `TestModeRequiredForMultiSourceList` | the core leaf's package test | AC-16: two sources without mode refused with the what, why and fix parts; one source accepted without mode; two with mode accepted | |
| `TestDefaultListIsStatedNotLent` | the core leaf's package test | an absent container yields the consumer's stated default list and mode; a written list never takes the default's mode | |
| `TestRouterSourceMerge` | system package `resolv_test.go` | AC-17 | |
| `TestOption6PassThroughDefault` | dhcpserver `handler_test.go` | AC-14 | |
| `TestRDNSSPassThroughDefault` | iface `reconcile_ra_test.go` | AC-15 | |
| `TestModeRequiredEachConsumer` | each consumer's config test | AC-16 through the consumer's verifier and its boot parse | |
| `TestRFC5625ProxyTCPAndOPT` | dnsresolver `forward_test.go` | AC-18, the Section 4.4.1 and 4.4.2 rows if the walk binds them; one test per further bound row | |
| `TestOption15AbsentWhenUnconfigured` | dhcpserver `handler_test.go` | AC-19 | |
| `TestRecordExpiryWithdraws` | system package `resolv_test.go` | AC-3 | |
| `TestDefaultListsReproduceToday` | system package `resolv_test.go` | AC-5, router half | |
| `TestInternalResolverFollowsSelection` | `resolve/dns` package `resolver_test.go` | AC-1, AC-2 | |
| `TestRouterSourceOrderReceivedFirst`, `TestRouterSourceOrderConfiguredFirst` | system package `resolv_test.go` | AC-4 | |
| `TestForwardListFromSelectionNoRebind` | dnsresolver `config_tx_test.go` | AC-6 | |
| `TestOption6FromDNSSource` | dhcpserver `handler_test.go` | AC-7, AC-5 | |
| `TestRDNSSWithdrawnAdvertisedLifetimeZero` | iface `reconcile_ra_test.go` | AC-8 | |
| `TestExtractMSDNSServers` | authradius `extract_test.go` | parse and R-6 | |
| `TestPoolIPCPDNSFromRadius`, `TestPoolIPCPDNSRadiusAbsent` | pool `register_test.go` | AC-9, AC-10 | |
| `TestDNSSourceClassRefused` | each consumer's config test | AC-11 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| record lifetime | 0..4294967295 s | 4294967295 (infinite) | N/A | N/A |
| RDNSS addresses per option | 1..8 | 8 | N/A | 9 (first 8 in list order, logged) |
| option 6 addresses | 1..63 | 63 | N/A | 64 (first 63, logged) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dns-source-router-follows-lease` | `test/plugin/dns-source-router-follows-lease.ci` | AC-1, AC-2, AC-3 through the daemon | |
| `dns-resolver-forward-received` | `test/plugin/dns-resolver-forward-received.ci` | AC-6, AC-13 | |
| `dhcp-server-dns-source-self` | `test/dhcp/dhcp-server-dns-source-self.ci` | AC-7 | |
| `dns-source-class-refused` | `test/parse/dns-source-class-refused.ci` | AC-11 | |
| `dns-source-mode-required` | `test/parse/dns-source-mode-required.ci` | AC-16 through `ze config validate` and `ze start` | |
| `dns-source-router-merge` | `test/plugin/dns-source-router-merge.ci` | AC-17 through the daemon | |
| `dhcp-server-dns-pass-through` | `test/dhcp/dhcp-server-dns-pass-through.ci` | AC-14 | |
| `show-system-dns` | `test/plugin/show-system-dns.ci` | AC-12 | |
| `l2tp-pool-dns-source-radius` | `test/l2tp/l2tp-pool-dns-source-radius.ci` | AC-9, AC-10 through the LNS | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `radius-subscriber-dns` | `test/interop-radius-subscriber/` | FreeRADIUS replying MS-Primary-DNS-Server; read its `dictionary.microsoft` first | AC-9 on the wire | |
| `access-up` DNS clause | child 19 | Ze CPE, Ze LNS | story 1 | |

## Files to Modify
- `internal/component/config/system/resolv.go` - record shape, expiry sweeper, router evaluation, event publish
- `internal/component/config/system/yang/ze-system-conf.yang` - the `dns-source` grouping (container, `mode`, `source` list) and `system dns-source`; correct the `name-server` description
- system package `system.go` - extract the router list
- `cmd/ze/hub/main_system.go`, `cmd/ze/hub/main.go` - hand the selection to the internal resolver at start and on change; reload path
- `internal/component/resolve/dns/resolver.go` - ordered server list, failover; `resolveSystemDNS` deleted
- DHCP client `resolv_linux.go` and iface `pppoe_client.go` - pass the lifetime
- `internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang`, and dnsresolver `config.go`, `config_tx.go`, `state.go` - `forward dns-source`
- `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang`, and dhcpserver `config.go`, `handler.go` - subnet `dns-source`
- `internal/component/iface/yang/ze-iface-conf.yang`, and iface `config_ra.go`, `reconcile_ra.go` - `rdnss dns-source`, Lifetime 0 withdrawal
- radius dictionary `dict.go` - MS DNS attribute ids
- `internal/component/l2tp/plugins/authradius/extract.go`, and l2tp `session_metadata.go` - DNS fields
- `internal/component/l2tp/plugins/pool/register.go`, `internal/component/l2tp/plugins/pool/yang/ze-l2tp-pool-conf.yang` - pool `dns-source`
- `docs/architecture/dns/lan-resolver.md`, `docs/guide/dns-resolver.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md`, `docs/features/interfaces.md`, `docs/architecture/config/syntax.md`
- every `dns-source` container's YANG `description` states that consumer's built-in default list and mode (D-3), and `docs/guide/dns-resolver.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md` and the DHCP server guide state the pass-through default, the change against today, and the explicit `source configured` that keeps the old behaviour
- Repository users of the changed defaults (D-3), converted or re-validated in the same work: the configs and `.ci` bodies naming `dns-server`, `rdnss` or `dns-primary`/`dns-secondary` (17, 2 and 26 files under `test/` today, including `test/parse/`, `test/l2tp/`, `test/plugin/`, `test/interop/scenarios/dhcpv4-rebind-two-servers-dnsmasq/`, `test/interop/scenarios/dhcpv4-renew-nak-dnsmasq/`, `test/interop/scenarios/dns-resolver-dhcp-lan/`, `test/interop-pppoe/scenarios/`, `test/interop-topology/cpe/`, `test/interop-topology/scenarios/two-ze-pppoe-link/`; the set is re-derived by grep at implementation). Each one whose expectation would now change because a learned or RADIUS DNS is present gets either its expectation corrected to the pass-through answer, or an explicit `dns-source` stating what it tests; a test that stays green only because no learned source exists is re-checked that it still proves what it claims
- `rfc/short/rfc5625.md` (new, through `/ze-rfc`), `rfc/extraction/rfc5625.json` (the extraction walk), the `fttx-edge` listing and any `dns` technology listing the RFC tooling derives from the summary's use-case field
- `docs/architecture/hub-architecture.md` (declared by the `cmd/ze/hub` files: the internal resolver wiring), `docs/architecture/l2tp/bng-1-radius-attributes.md` (declared by authradius: the MS DNS attributes), `docs/research/l2tpv2-ze-integration.md` (declared by the l2tp code: checked, edited only if it describes subscriber DNS)

## Files to Create
- a core leaf package holding the record value type and the selection rule (name fixed at implementation; standard library imports only, per `ai/doors/core.md`)
- the `system-dns` event namespace registration beside the registry
- the functional tests named above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | the grouping in `ze-system-conf.yang`, its uses in four consumer modules, and the `show system dns` command YANG in its owning package (A-5) |
| YANG validation constraints | Yes | a `must` per consumer restricting classes; `interface` only under `received` |
| YANG custom validators | Yes | `interface` entries completed from the iface name resolver (`docs/architecture/iface/logical-name-resolution.md`) |
| CLI commands/flags | Yes | `show system dns` |
| CLI grammar (keyword before value) | Yes | `source <class>`, then `interface <name>` |
| Editor autocomplete | Yes | enum classes automatic; interface names by `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/plugin/show-system-dns.ci` |
| Pipe completeness | Yes | the show answer goes through the pipe processor |
| Env var registration | No | no env var |
| Doctor check for runtime dependencies | Yes | a check that warns when the router selection is empty while a LAN consumer lists `system` or `received`; no file, socket or port is added |
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
| 6 | Has a user guide page? | Yes | `docs/guide/dns-resolver.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md`, `docs/guide/migrating-from-vyos.md` |
| 7 | Wire format changed? | No | encoders unchanged; only the source of their content |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` (new event namespace) |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8106.md` (RFC8106-5.1-1, router side), `rfc/short/rfc7084.md` (L-11 RDNSS, L-12), `rfc/short/rfc5625.md` (new, D-2) |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, the VyOS DNS rows |
| 12 | Internal architecture changed? | Yes | `docs/architecture/dns/lan-resolver.md`; `docs/architecture/core-design.md` section 19 if the core leaf adds an import row |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` (event namespace), `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation by `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-23-dns-source-fall-through.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/dns-resolver.md`, the pool examples in `docs/guide/l2tp.md` |

### VyOS mapping (input to child 10)

| VyOS (vyos-1x 611550d23) | Ze form | Class |
|--------------------------|---------|-------|
| `system name-server 192.0.2.53` | `system name-server 192.0.2.53` | equivalent |
| `system name-server eth0` (servers learned on eth0) | `system dns-source source received interface eth0` | equivalent once implemented |
| `system name-server 192.0.2.53` plus `system name-server eth0` | `system dns-source` sources `configured`, then `received interface eth0`, mode `merge` | equivalent: VyOS merges by tag order, and the migration writes `merge` explicitly |
| `interfaces pppoe pppoe0 no-peer-dns` | leave that interface out of `received` | conditional on child 13 |
| `service dns forwarding name-server X` | `service dns-resolver forward server X` | equivalent |
| `service dns forwarding system` (with `name-server X`) | `forward dns-source` sources `configured`, `system`, mode `merge` | equivalent |
| `service dns forwarding dhcp eth0` | `forward dns-source source received interface eth0` (mode `merge` when listed with other sources) | equivalent |
| `service dhcp-server ... option name-server X` | subnet `dns-server X` and `dns-source source configured` | equivalent only with the explicit list: without it Ze's pass-through default (D-3) hands out WAN-learned servers first, which VyOS does not |
| `service pppoe-server name-server X`, `vpn l2tp remote-access name-server X` | `l2tp pool ipv4 dns-primary`, `dns-secondary` | equivalent for two IPv4 servers when RADIUS sends no MS DNS; with it, Ze's default (D-3) offers RADIUS's, as accel-ppp does |
| accel-ppp RADIUS MS DNS override | pool default `radius`, `configured`, `first-usable` | equivalent when RADIUS sends both servers or neither. accel-ppp overwrites each option independently, so RADIUS sending only the primary keeps the configured secondary there; Ze's `first-usable` offers the RADIUS primary alone and `merge` offers the RADIUS primary then the pool's primary. Neither mode copies the per-slot overwrite, which is recorded as a difference |

## Implementation Steps

1. **Phase: Wiring** - core leaf with the record type, the rule in both modes and the mode check; registry publishes the event; internal resolver takes a list. Tests: `TestSelectFirstNonEmptyClass`, `TestSelectMergeInOrderDeduplicated`, `TestModeRequiredForMultiSourceList`, `TestInternalResolverFollowsSelection`, `test/plugin/dns-source-router-follows-lease.ci`, all failing first
2. **Phase: Router consumer (23a)** - lifetime on `SetLease`, expiry sweeper, `system dns-source` with `mode`, stated default, the mode refusal at validate, commit and boot, reload. AC-1 to AC-5, AC-16 (router), AC-17
3. **Phase: RFC 5625 (23a)** - write `rfc/short/rfc5625.md` through `/ze-rfc` with use case `fttx-edge`, run the extraction walk both ways, then implement and test every row it binds to the LAN resolver and the DHCP server, recording each break with `./le rfc record`. AC-18, AC-19
4. **Phase: LAN resolver** - forward `dns-source` over the event, no rebind. AC-6, AC-13
5. **Phase: DHCPv4 option 6 and RA RDNSS** - pass-through defaults and explicit lists. AC-7, AC-8, AC-14, AC-15
6. **Phase: Subscriber** - MS DNS extraction, pool `dns-source`, default `radius`, `configured`. AC-9, AC-10, interop
7. **Phase: Conversion** - every repository user of the changed defaults converted or re-validated (Files to Modify), in the same work as phases 5 and 6
8. **Phase: Show, doctor, metrics, docs** - AC-11, AC-12, AC-16 for every consumer

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | no expired record is ever selected; the family filter runs before a class can win |
| Naming | JSON keys kebab-case; class names identical in YANG, show output and docs |
| Data flow | plugins read records only from the event; resolv.conf only from `Resolvers` |
| Rule: no-layering | the single-server resolver path is deleted, not kept as a fallback |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| shared selection rule | its package test |
| internal resolver follows the selection | `TestInternalResolverFollowsSelection` and `test/plugin/dns-source-router-follows-lease.ci` |
| `show system dns` | `test/plugin/show-system-dns.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | learned and RADIUS addresses are unicast, carry no line break (the existing `appendServers` guard stays), and are bounded per consumer |
| Rogue source | a LAN host cannot become a `received` source: only clients Ze runs on configured interfaces register records |
| Loop | `self` and `system` combinations that make the resolver forward to itself are refused (R-3) |

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
- The router consumer is a defect, not a feature: the internal resolver's single server is chosen once at hub start, and the `name-server` YANG description promises more than the code does.
- The IPv6 subscriber path hands out no resolver at all: the PPP RA sets O and nothing sends DHCPv6 option 23. Owned by `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md` (D-4); subscriber IPv6 itself is not up yet (`plan/post-release/features/spec-l2tp-ipv6-subscriber.md`).

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One registry; each consumer evaluates its own list with one shared rule | a central evaluator that reads every consumer's config path | the central form names features in shared code (`ai/META-DECISIONS.md`, modular engine); each consumer owns its list in its own YANG |
| Records reach plugins as an EventBus event with replay | plugins call `system.ProcessResolvers()` | the plugins can run forked; a Go call writes a dead copy (`ai/doors/system.md`) |
| A closed class vocabulary over an open protocol set | per-consumer lists of protocol names (`dhcpv4`, `ppp`) | protocols register; a class names a role, so a new protocol needs no YANG edit; the `interface` filter covers the VyOS per-interface form |
| The operator states the fall-through mode: `mode first-usable` or `mode merge`, no YANG default, required once a list names two sources (D-1) | one fixed rule (first-usable); a defaulted `mode` leaf | Thomas: "The fall-through rule should be explicitly configured." A defaulted leaf would be the silent choice he ruled out, and a one-source list has nothing to choose, so it needs no mode |
| The mode refusal and the class check run from the core leaf in each consumer's in-process verifier, `OnConfigVerify` and boot parse | YANG `must` | no `-conf` module uses `must` today, a verifier gives the fix text, and the in-process verifier alone misses boot (`ai/doors/config-yang.md`, Traps) |
| Downstream consumers pass the DNS through by default: DHCPv4 and RA `received`, `configured`; pool `radius`, `configured`; all `first-usable` (D-3, interpretation flagged as P-1) | keep `configured` alone, as today | Thomas: "By default the behavior should be to pass the DNS through." RFC 5625 Section 3: "end-users SHOULD be able to send their DNS queries to specified upstream resolvers, thereby bypassing the proxy altogether." This supersedes the earlier row that kept `configured` for LAN consumers |
| The router and the LAN resolver keep today's defaults | pass-through for them too | the router already passes received servers through when nothing is configured (RFC 3646 Section 6 order); the LAN resolver is the proxy itself, not a downstream consumer (P-2) |
| Built-in defaults are stated lists with stated modes, shown with a `default` marker, and never lend their mode to a written list | an implicit mode for absent config | D-1 and D-3 together: the default is documented, not hidden |
| RFC 5625 is followed here and listed for `fttx-edge`; which proxy rows bind the LAN resolver is decided by the extraction walk (D-2) | guidance only, unlisted | Thomas: "RFC 5625 inside child 23" |
| IPv6 subscriber DNS is a separate spec after the umbrella (D-4) | in this spec | Thomas: "spec of its own"; subscriber IPv6 does not come up before `spec-l2tp-ipv6-subscriber` anyway |
| Delete the single-server internal resolver | keep `Server` as a fallback | `ai/rules/no-layering.md` |

## Known Limitations
- LAN DHCPv6 DNS_SERVERS (RFC 7084 L-10) needs a LAN DHCPv6 server: `post-release/features/spec-dhcpv6-server.md`; its consumer joins this grammar when it lands.
- DNSSL and DHCP option 15 search lists: `post-release/features/spec-router-advertisement-options-out-of-scope.md`.
- DNR (RFC 9463): after the first release; the record reserves its fields.
- IPv6 subscriber DNS (RFC 6911 attribute 169, RDNSS in the PPP RA, DHCPv6 option 23): `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md` (D-4).
- accel-ppp's per-option RADIUS DNS overwrite is not reproduced by either mode (VyOS mapping table).
- A LAN DHCPv4 client keeps option 6 until it renews (RFC 5625 Section 5.3), and FORCERENEW is not implemented; O-8 decides whether lease length shortens while the selection comes from `received`.

## Open owner questions

Decided on 2026-11-08 (see "Owner decisions"): O-1 by D-1, O-2 by D-3, O-6 by
D-4, O-7 by D-2.

Confirmation of the D-3 reading:

| # | Decision | Options | What each answer changes |
|---|----------|---------|--------------------------|
| P-1 | Pass-through for LAN clients when the operator also configured servers on the subnet or unit | (a) `received`, `configured`: the WAN-learned servers win and `dns-server`/`rdnss server` are the backup (recorded now: "rather than ... the configured servers"); (b) `configured`, `received`: a configured server wins and the WAN-learned ones pass through only when none is configured | (a) changes option 6 and RDNSS on every box that both learns DNS and sets `dns-server`, a PXE provisioning box included (R-8); (b) changes them only where nothing was configured. AC-14, AC-15 and the defaults table follow the answer |
| P-2 | Whether pass-through also covers the LAN resolver's upstreams | (a) no, it keeps `configured` (recorded now: the resolver is the proxy, not a downstream consumer); (b) yes, `configured`, `system` or `received`, `configured` | (b) makes a resolver with no `forward server` forward to the learned servers instead of having no upstream |

Still open:

| # | Decision | Options | What each answer changes |
|---|----------|---------|--------------------------|
| O-3 | Which address `self` means | the dns-resolver listener address on the consumer's link; or the DHCP server identifier, the RA source, the pool gateway | which address a LAN client or subscriber is told to query |
| O-4 | Receive RDNSS from RAs on the WAN as a `received` origin (RFC 8106 is optional for fttx-edge) | in this spec (netlink ND user options) or later | brings RFC8106-1.2-1 and the host rows into scope |
| O-5 | List RFC 2548 for the `lns` profile | yes (summary written, MS DNS rows counted) or no (implemented, uncounted) | conformance figures only |
| O-8 | DHCPv4 lease length while option 6 comes from `received` and the WAN is down | leave it; or shorten leases (RFC 5625 Section 5.3 suggests 60 s) | stale-resolver exposure on LAN clients |
| O-9 | Folder and priority of the feature half (23b) | `release/fixes` P3 beside the other home-CPE children, or `post-release/features` | release inventory |

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code: RFC
8106 Section 5.1 above the Lifetime 0 withdrawal, RFC 2548 Section 2.7.6 above
the MS DNS extraction; the RFC 3646 Section 6 quote stays above the router rule.
RFC 5625 Section 3 ("end-users SHOULD be able to send their DNS queries to
specified upstream resolvers, thereby bypassing the proxy altogether.") above
the pass-through defaults of the DHCPv4 and RA consumers; RFC 5625 Section 5.2
above the option 15 omission; each RFC 5625 proxy row the walk binds quoted
above the LAN resolver code that meets it.

### RFC 5625 in this spec (D-2)

| Step | Owed work | Tool |
|------|-----------|------|
| 1 | Write `rfc/short/rfc5625.md`, use case `fttx-edge`, implementer kind declared, every row a verbatim span of its section | `/ze-rfc` |
| 2 | Extraction walk, forward and reverse, signed off in `rfc/extraction/rfc5625.json`; it decides which Section 4 and 6 proxy rows bind the LAN resolver (a caching forwarder that originates its own upstream query) and records the reason for each that does not | the RFC extraction tooling (`docs/contributing/rfc-conformance-gates.md`) |
| 3 | Each bound MUST met and tested, each tag carrying a discrimination record | `./le rfc record`, `./le rfc check` |
| 4 | The profile listing for `fttx-edge`, as the RFC tooling derives it from the summary | the RFC tooling, never a hand-edited list |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (RFC 2548, 3646 and 9463 have none under the owner rule, O-5; RFC 5625's is written by this spec, D-2)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test functional plugin`, `./le test functional dhcp`, `./le test functional parse`, `./le test functional l2tp`, `./le test integration interop-radius-subscriber`, `./le rfc check` (the RFC 5625 rows), and the scoped package tests of the changed packages through `./le job run`
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
- [ ] **Commit B:** `remove plan/release/fixes/spec-home-cpe-23-dns-source-fall-through.md` only, in the same `./le commit create` script
