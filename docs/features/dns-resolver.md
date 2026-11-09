# DNS Resolver

<!-- source: internal/component/resolve/dns/resolver.go -- miekg/dns resolver with cache -->
<!-- source: internal/component/resolve/dns/cache.go -- O(1) LRU cache with TTL -->
<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- system DNS config -->

Built-in DNS resolver component providing cached DNS queries to all Ze components.
Uses `github.com/miekg/dns` (the library CoreDNS is built on).

This is the client side: Ze asking somebody else. Ze also answers queries, from
three plugins that share one server harness (`internal/core/dnsserver`): two
authoritative ones, `as112` (see
[AS112 guide](../guide/as112.md#answer-policy) for the answer policy the harness
enforces) and `geodns` (per-source-IP answers, `service { geodns { ... } }`),
and the LAN resolver `dnsresolver` (`service { dns-resolver { ... } }`), a
caching forwarder that answers recursive queries from customer LANs
([LAN DNS resolver guide](../guide/dns-resolver.md)).
<!-- source: internal/plugins/dnsresolver/server.go -- newResolverServer -->
The client side shares no configuration and no cache with any of them.

| Feature | Description |
|---------|-------------|
| Static name servers | `system { name-server [8.8.8.8 1.1.1.1]; }` sets upstream DNS servers |
| Router resolver selection | One rule picks the router's servers: the configured `name-server` list when it has an entry, else every learned record (DHCPv4, DHCPv6, PPPoE IPCP, RA RDNSS), and the host's `/etc/resolv.conf` only while none of those lists a server |
| resolv.conf writer | Writes the router's servers to resolv-conf-path, and rewrites it whenever a source changes |
| Learned DNS | Under the default list, static servers take priority over the servers a DHCP lease, a PPPoE session or a router advertisement supplied |
| Query types | A, AAAA, TXT, PTR, CNAME, MX, NS, SRV |
| LRU cache | O(1) operations, configurable size and max TTL |
| TTL-aware | Respects response TTL, caps at configured maximum, honors TTL=0 (do not cache) |
| Concurrent safe | Mutex-protected cache, safe for multi-goroutine use |
| Server failover | The internal resolver asks the router's servers in order. A server that gives no answer, or answers SERVFAIL, REFUSED or any RCODE other than NOERROR and NXDOMAIN, passes the query to the next one. When every server fails, the caller gets the last server failure, never an empty success |
| Fail closed | With no server selected, queries fail with `no DNS server configured`. No public resolver is ever added |
| Timeout control | Per-resolver configurable timeout (1-60 seconds) |
| Cache management | List entries, inspect by name, and clear with `clear dns cache all/record/stats`: flush everything, delete by name/type, or reset the counters |
| `\| resolve` pipe | Reverse DNS enrichment for IP addresses in any command's JSON output |
| `\| origin` pipe | ASN/network enrichment for IP addresses via Team Cymru DNS queries |

<!-- source: internal/component/config/system/resolv.go -- routerDefault, Resolvers.refresh -->
<!-- source: internal/component/resolve/dns/resolver.go -- Resolver.query, unansweredError -->

## Configuration

```
system {
    name-server [8.8.8.8 1.1.1.1]
    dns {
        resolv-conf-path /tmp/resolv.conf
        timeout 5
        cache-size 10000
        cache-ttl 86400
    }
}
```

| Option | Default | Description |
|--------|---------|-------------|
| `name-server` | (none) | Static DNS servers: the `configured` source of the router's list. Under the default list, when set, they are the router's servers: the internal resolver asks them in order, and resolv.conf lists them all. |
| `dns-source` | `source [ configured received ]`, `mode first-usable` | The ordered sources of the router's servers and how they combine (next section). |
| `resolv-conf-path` | `/tmp/resolv.conf` | Path for resolv.conf. Default suits gokrazy (read-only rootfs). Empty disables writing. Any other value must be an absolute path in clean form (no `..` or repeated `/`): `ze config validate` and commit refuse it otherwise, and the refusal names the path to write. |
| `timeout` | 5 | Query timeout in seconds for each server asked (1-60) |
| `cache-size` | 10000 | Maximum cached entries (0 disables caching) |
| `cache-ttl` | 86400 | Maximum cache TTL in seconds (0 uses response TTL only) |

## Where the router's servers come from

Ze keeps the static `name-server` list and, apart from it, one record per
learned source: one per DHCP client (DHCPv4 or DHCPv6, per interface and
unit), one per PPPoE client session (the servers IPCP supplied, RFC 1877,
primary first), one per unit that accepts Router Advertisements (the RDNSS
servers, RFC 8106), and one for the host's own `/etc/resolv.conf`. A record
carries how long it stays valid: a DHCPv4 lease until the lease expires
(until withdrawn for an infinite lease), a DHCPv6 Reply until the valid
lifetime of its last binding runs out, an RA record until its last
server's Lifetime runs out (a pppoe-client's until its session ends, if
that comes first), an IPCP lease and the host's file until withdrawn. When a record's validity ends, Ze withdraws it at that moment,
whether or not its client said so.

An RA record exists only for a unit whose `ipv6 accept-ra` is set above 0,
or for an enabled `pppoe-client` whose `ipv6 accept-ra` is set above 0: the
RAs of that client's WAN session arrive on its session device, which carries
the client's name, and the record is named after the client.
The kernel processes the RA, and only when its `accept_ra` lets it; it then
hands the RDNSS option to Ze. Ze checks the option as RFC 8106 Section 5.3.1
says (a Length of at least 3 and odd, unicast addresses) and discards an
invalid one with a warning naming the interface and the Length. Each server
lives for the option's Lifetime from receipt, a later RA with the same
server moves its expiry, Lifetime 0 removes it at once, and a new server is
listed first (RFC 8106 Section 6.2). One unit keeps at most 8 RA servers;
past that, the one that expires first is dropped. A unit that loses its
`accept-ra`, or is removed, has its RA record withdrawn on that commit, and
so does a pppoe-client. A pppoe-client's RA record is also withdrawn when
its session ends, together with its IPCP lease, whatever Lifetime its
servers had left: they were reached over the session device, which ends
with the session, and the next session's RAs build a fresh record. An
interface unit's RA record is not tied to its device that way: it stays
until its Lifetime runs out, a Lifetime 0 arrives, or a commit removes the
unit.
<!-- source: internal/component/iface/ra_rdnss.go -- raRDNSS.handle, raServerList.apply, raRDNSS.withdrawPPPoESession -->
<!-- source: internal/component/iface/pppoe_client.go -- PPPoEClient.runSession, PPPoEClient.endSessionLeases -->
<!-- source: internal/core/ndp/rdnss.go -- ParseRDNSS -->

On every change, one rule selects the router's servers from them, walking
the list `system dns-source` names. Two classes of source can be named
there: `configured` (the `name-server` list) and `received` (every learned
record, DHCPv4 first, then DHCPv6, then PPPoE, then RA RDNSS, each by
interface then unit, in the order each source sent them). The host's file
is a fallback inside `received`: its servers are listed only while no other
learned record lists a server. The `mode` says
how two classes combine:

| `mode` | The router's servers |
|--------|----------------------|
| `first-usable` | the first class in the list that holds a usable server, alone; a later class is a backup, used only while the earlier ones hold none |
| `merge` | every class, in list order, a server already listed not repeated; Ze's internal resolver tries a later server when an earlier one does not answer or answers a server failure |

With no `dns-source` written, the default is `source [ configured received ]`
with `mode first-usable`. That is what Ze always did:

| Situation, under the default | The router's servers |
|-----------|----------------------|
| `name-server` configured | the static servers alone; a learned server never overrides them (RFC 3646 Section 6) |
| no `name-server` | every learned record's servers, in the received order above |
| a renewal reply omits the DNS option, or a lease ends or expires | that lease's servers leave the selection; the other sources' stay (RFC 9915 Section 18.2.10.5 for DHCPv6) |
| every source withdrawn | no server: queries fail closed, and resolv.conf holds the ze header with no `nameserver` line once ze has written it. A file ze never wrote is left alone |

Two examples. To keep your own resolvers and fall back to the ISP's only
when none is configured, write nothing: that is the default. To prefer the
ISP's and keep your own as a backup asked after them:

```
system {
    name-server [ 198.51.100.53 ]
    dns-source {
        source [ received configured ]
        mode merge
    }
}
```

`received-interface [ eth1 ]` narrows `received` to the leases learned on the
named interfaces, in the order listed, then by origin, then by unit. An
interface that holds no lease yet is accepted, because it may learn one
later.

Ze refuses, at `ze config validate`, at commit, at boot and at reload:

| Written | Refusal |
|---------|---------|
| two sources and no `mode` | `system dns-source names 2 sources (configured, received) and no mode`: Ze cannot tell whether a later source is a backup or an addition. Set `mode first-usable` or `mode merge` |
| `received-interface` without `received` in `source` | the filter would select nothing. Add `source received`, or delete `received-interface` |
| `source self` | Ze cannot be its own upstream. Name upstream servers with `name-server`, or use `source received` |
| `source radius` | RADIUS resolvers belong to one PPP subscriber, not the router |
| a `dns-source` container with no `source` | an empty list offers no resolver. Name a source, or delete `dns-source` to take the default |
| one class twice in `source` | its second place can never contribute a server. Delete the repeat |
| a `source` or `mode` word Ze does not know | Ze cannot tell what the word selects. Use a class from the list below, and `first-usable` or `merge` |

A list naming one source needs no `mode`, because both modes give it the
same answer.

<!-- source: internal/component/config/system/dns_source.go -- RouterDNSSource, routerDNSSourceRefusals, validateRouterDNSSourceTree -->
<!-- source: internal/core/dnssource/select.go -- NewWrittenList, List.Received, ModeRequiredError, ReceivedInterfaceError, ClassRefusedError -->

An address that cannot name a resolver (the unspecified address, a multicast
address, 255.255.255.255) is skipped, whatever its source. A loopback
address such as the 127.0.0.53 of a host stub resolver stays, because Ze's
own lookups can reach it.

<!-- source: internal/component/config/system/resolv.go -- Resolvers, routerDefault, Resolvers.records, ValidFor, Resolvers.armExpiryLocked -->
<!-- source: internal/core/dnssource/select.go -- Select, Usable -->
<!-- source: internal/plugins/iface/dhcp/resolv_linux.go -- setV4Resolvers, setV6Resolvers -->

The host's `/etc/resolv.conf` keeps a Ze on a standard distribution
resolving when no `name-server` is configured and no lease supplies one. It
stands in for a lease and never joins one: while a DHCPv4, DHCPv6, PPPoE or
RA record lists a server, the host's servers are left out, so a container's
resolver (Docker's 127.0.0.11) is never written beside the servers IPCP
supplied. When the last such record expires or is withdrawn, the host's
servers return. Ze reads it at startup and on every config reload, not when the file changes.
Ze never reads it when it is the same file as `resolv-conf-path` (symlinks
are followed), because Ze would then read back the servers it wrote.
<!-- source: internal/component/config/system/resolv_host.go -- readHost, sameFile -->
<!-- source: internal/component/config/system/register_dns.go -- hostResolverOrigin -->
<!-- source: internal/component/config/system/resolv.go -- RegisterFallbackResolverOrigin -->
<!-- source: internal/core/dnssource/record.go -- liveAddresses -->

The selection is the one source of truth for every reader:

| Reader | What it does with the selection |
|--------|---------------------------------|
| Ze's internal resolver | Asks every selected server in order, and follows the selection on each lease change and reload |
| resolv-conf-path | Lists every selected server |
| A plugin, in process or external | Subscribes to the `system-dns` event namespace (`docs/architecture/api/process-protocol.md`, "The `system-dns` namespace") |

<!-- source: cmd/ze/hub/main_system.go -- newResolvers -->
<!-- source: cmd/ze/hub/main.go -- PublishOn -->

The local commands `ze resolve dns` and `ze resolve cymru` run without a
daemon, so they cannot see a lease. They ask the server that `--server`
(`--dns-server` for cymru) names, else the servers of the host's
`/etc/resolv.conf`, else they fail closed.
<!-- source: internal/component/resolve/cli/local_servers.go -- localServers -->

### The `dns-source` list and the learned record

`dns-source` is one YANG grouping, declared in `ze-types`, so every consumer
that hands resolver addresses on writes the same list. The router's
`system dns-source`, the LAN resolver's upstreams (`service dns-resolver
forward dns-source`), a DHCP subnet's option 6 (`service dhcp-server
shared-network subnet dns-source`) and a unit's RA RDNSS (`interface ... unit
... ipv6 router-advertisement rdnss dns-source`) and a subscriber pool's IPCP
DNS (`l2tp pool ipv4 dns-source`, `l2tp pool named-pool dns-source`) use it.

| Leaf | Shape |
|------|-------|
| `source` | the classes, in the order the consumer falls through them, each class at most once |
| `mode` | `first-usable` or `merge`, with no default. A list of two or more classes needs one |
| `received-interface` | interface names, in order. It narrows `received` and nothing else |

| Class | What it lists |
|-------|---------------|
| `received` | every unexpired learned record, by origin rank, then interface, then unit |
| `configured` | the consumer's own static list: for the router, `system name-server` |
| `self` | Ze's own address on the interface the client is on |
| `radius` | the resolvers RADIUS gives one PPP subscriber: the Access-Accept's MS-Primary-DNS-Server and MS-Secondary-DNS-Server (RFC 2548), allowed only in a pool's list |

A class names a role, never a protocol. A protocol that learns resolvers
registers an origin with a rank, and its records appear inside `received`.
Each consumer declares the classes it cannot use, with the reason and the
fix it prints: the router refuses `self` and `radius`. A consumer's built-in
default applies only while its `dns-source` container is absent. Once the
container is written, the default no longer applies, its `mode` included.

Each consumer also states who receives its servers. The router and the LAN
resolver's upstreams serve Ze itself, so a loopback server such as
127.0.0.53 or an IPv6 link-local server is usable there. DHCPv4 option 6,
RA RDNSS and a subscriber's IPCP hand addresses to another host, so the
selection drops a loopback address (127.0.0.0/8, ::1) and an IPv6
link-local address with no zone from them, logs each drop with the
consumer's path, and falls through to the next entry or class. IPv4
link-local (169.254.0.0/16) is kept: a server there on the client's own
link is reachable from it.
<!-- source: internal/core/dnssource/select.go -- Audience, HostOnly, appendUsable -->

A consumer plugin, in the engine process or its own, subscribes to the
`system-dns` snapshot, asks for a replay once it has started, and keeps the
newest snapshot's records with `dnssource.Follower`, which drops a
snapshot older than the last it kept. A plugin process receives the JSON
form and hands it to `Accept`. A component in the daemon (the interface
component, for RDNSS) subscribes to the typed handle
`dnssource.SnapshotEvent`, hands the value to `AcceptSnapshot`, and emits
`dnssource.SnapshotRequest` after it subscribes. A plugin that receives a
snapshot it cannot decode keeps its previous learned resolvers, logs the
cause at WARN, counts it in
`ze_dns_source_snapshots_rejected_total{consumer}`, and applies the next
snapshot, including one delivered in the same batch.
<!-- source: internal/core/dnssource/follow.go -- Follower, Accept, AcceptSnapshot, RejectedCounter -->
<!-- source: internal/plugins/dnsresolver/register.go -- learnedSnapshotHandler -->
<!-- source: internal/plugins/dhcpserver/register.go -- learnedSnapshotHandler -->
<!-- source: internal/core/dnssource/register.go -- SnapshotEvent, SnapshotRequest -->

Each consumer also publishes what its lists chose, so `show system dns` and
`show doctor` (the daemon form of `ze doctor`) can read every consumer from
the daemon. A consumer emits
`(system-dns, selection)` carrying a `dnssource.Publication`: its plugin
name and one `ConsumerSelection` per list (the config path, the instance it
belongs to, such as the subnet, the unit or the pool, the sources and mode,
`default` while the built-in default applies, the classes used, the servers,
`self`, the servers the `self` class contributed, and for the LAN resolver
`serves`, the listener addresses whose UDP socket is bound now: a configured
listener that is not bound is not listed, so `ze doctor` warns about a `self`
address nothing answers on). It publishes its
whole set after it starts and whenever a selection changes, because the
registry replaces the consumer's previous set with each publication: a list
the operator removed leaves by being absent. The registry keys the sets by
the consumer's name and names no consumer itself. The router's own
selection stays in the snapshot.

| Consumer | Instance | Published when |
|----------|----------|----------------|
| `dns-resolver` | none (one forward list) | a policy build, after start, and each listener bind or unbind; a disabled resolver publishes an empty set |
| `dhcpserver` | the subnet prefix | each config install and each fresh snapshot; `self` is the listened device's address inside the subnet |
| `interface` | `<interface> unit <unit>` | each RA reconcile; the servers are the ones the RDNSS option carries |
| `l2tp-pool` | none for the default pool, the name for a named pool | joining the bus, each pool install and each fresh snapshot |

RADIUS names a subscriber's servers when the session asks, so the pool's
published selection is what a session whose RADIUS reply carries no MS DNS
server is offered: the `radius` class contributes nothing to it.
<!-- source: internal/core/dnssource/record.go -- ConsumerSelection, NewConsumerSelection, Publication -->
<!-- source: internal/plugins/dhcpserver/publish.go -- dhcpPublisher -->
<!-- source: internal/component/iface/ra_dns_source.go -- publishRASelections -->
<!-- source: internal/component/l2tp/plugins/pool/publish.go -- publishPools -->
<!-- source: internal/component/config/system/resolv.go -- PublishOn, SetPublication, Consumers -->

`show system dns` answers `records` (every learned record) and `consumers`:
the router first, under the consumer name `system`, then every consumer that
has published, sorted by name. A consumer that has not published yet is
absent: its plugin is not running or has not received its first snapshot.
<!-- source: internal/plugins/system-dns-cmd/cmd/show_dns.go -- showSystemDNS, newSystemDNSAnswer -->

The registry exports `ze_dns_source_records{origin}`, the learned records
each origin holds (zero once its leases are withdrawn), and
`ze_dns_source_selection_changes_total{consumer}`, one per change of a
consumer's published selection after its first, the router's under
`system`. A repeated publication of the same answer is not a change.
<!-- source: internal/component/config/system/resolv_metrics.go -- SetMetrics, observe -->

`show doctor` warns, for the router and every published list, when a list
that names `received` selected no server (`doctor-dns-source-empty`), when a
`received-interface` entry names an interface that holds no learned record
(`doctor-dns-source-interface-silent`), and when a `self` server is an
address no `service dns-resolver` listener answers on
(`doctor-dns-source-self-unserved`). Each message names the list's path and
instance. The local `ze doctor` does not run these checks: the registry they
read is filled only in the daemon.
<!-- source: internal/component/config/system/doctor_dns_source.go -- dnsSourceFindings -->

| Origin | Rank | Learned from |
|--------|------|--------------|
| `dhcpv4` | 100 | DHCPv4 option 6 |
| `dhcpv6` | 200 | DHCPv6 DNS Recursive Name Server option |
| `ppp` | 300 | IPCP on a PPPoE client session (RFC 1877) |
| `ra` | 400 | Router Advertisement RDNSS (RFC 8106) |
| `host` | 500 | the host's `/etc/resolv.conf`, registered as a fallback |

A lower rank lists first. A fallback origin's records are listed only while
no other record lists an address.

Each learned record holds its origin and rank, the interface and unit that
learned it (both empty for the host's file), the addresses in the order the
source sent them, when it was received and when it expires. A record with no
expiry is valid until its source withdraws it. The record also reserves
`priority`, `name`, `transport`, `port` and `path` for RFC 9463 encrypted
resolver discovery (DNR), which Ze does not implement yet. A record that
names a `transport` is not plain DNS on port 53, so `received` skips it. The
JSON form of a record is in `docs/architecture/api/process-protocol.md`,
"The `system-dns` namespace".

<!-- source: internal/component/config/yang/modules/ze-types.yang -- dns-source-class, dns-source -->
<!-- source: internal/core/dnssource/select.go -- Class, ParseClass, ParseMode, NewList, NewWrittenList, Refusal, EmptyListError, RepeatedSourceError -->
<!-- source: internal/core/dnssource/record.go -- Record, CompareRecords, liveAddresses -->
<!-- source: internal/component/config/system/dns_source.go -- readWrittenDNSSource, unknownDNSSourceClassError -->
<!-- source: internal/plugins/iface/dhcp/register.go -- dhcpv4ResolverOrigin, dhcpv6ResolverOrigin -->
<!-- source: internal/component/iface/register.go -- pppoeClientResolverOrigin, raResolverOrigin -->
<!-- source: internal/component/config/system/register_dns.go -- hostResolverOrigin -->

## Cache management

The DNS cache can be inspected and managed at runtime via CLI commands.

**Inspection:**

```
show dns cache stats                            # Hit/miss/eviction counters + rates
show dns cache list                             # All non-expired entries (sorted by TTL)
show dns cache record example.com               # Entries for a specific name
```

**Clearing:**

```
clear dns cache all                             # Flush all entries and reset counters (bare `clear dns cache` is refused)
clear dns cache stats                           # Zero counters without removing entries
clear dns cache record example.com              # Delete entries for a name (all types)
clear dns cache record example.com type AAAA    # Delete a single name+type entry
```

## Pipe operators

Two pipe operators enrich JSON output from any command with DNS-based lookups:

| Pipe | Description |
|------|-------------|
| `\| resolve` | Adds a `<key>-name` field with the PTR (reverse DNS) hostname for each IP address value in the JSON output. Uses the system DNS resolver with cache. 500ms timeout per lookup. |
| `\| origin` | Adds `<key>-asn`, `<key>-as-name`, and `<key>-prefix` fields for each IP address value via Team Cymru DNS queries. 2s timeout. |

Both pipes walk JSON values, detect IP addresses, and add sibling fields.
They work on any command output, including `show traceroute`, `show bgp`, etc.
In `monitor traceroute | log` mode, they enrich the hop legend.

## Reload behavior

A reload that changes `name-server` or `dns-source` reaches the internal
resolver and resolv.conf at once: the reload re-runs the selection with the
new list, and under the default list removing every static server hands both
back to the learned records. A reload whose `dns-source` Ze refuses is
refused whole, and the running list stays. The `dns` settings
(`timeout`, `cache-size`, `cache-ttl`) are applied at startup, and a change to
them needs a process restart.
<!-- source: cmd/ze/hub/main_system.go -- applyResolvConf, newResolvers -->
<!-- source: internal/component/config/system/system.go -- ExtractSystemConfig, SystemConfig.DNSSource -->
