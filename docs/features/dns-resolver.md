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
| Router resolver selection | One rule picks the router's servers: the configured `name-server` list when it has an entry, else every learned record (DHCPv4, DHCPv6, PPPoE IPCP, then the host's `/etc/resolv.conf`) |
| resolv.conf writer | Writes the router's servers to resolv-conf-path, and rewrites it whenever a source changes |
| Learned DNS | Static servers take priority over the servers a DHCP lease or a PPPoE session supplied |
| Query types | A, AAAA, TXT, PTR, CNAME, MX, NS, SRV |
| LRU cache | O(1) operations, configurable size and max TTL |
| TTL-aware | Respects response TTL, caps at configured maximum, honors TTL=0 (do not cache) |
| Concurrent safe | Mutex-protected cache, safe for multi-goroutine use |
| Server failover | The internal resolver asks the router's servers in order. A server that gives no answer passes the query to the next one |
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
| `resolv-conf-path` | `/tmp/resolv.conf` | Path for resolv.conf. Default suits gokrazy (read-only rootfs). Empty disables writing. |
| `timeout` | 5 | Query timeout in seconds for each server asked (1-60) |
| `cache-size` | 10000 | Maximum cached entries (0 disables caching) |
| `cache-ttl` | 86400 | Maximum cache TTL in seconds (0 uses response TTL only) |

## Where the router's servers come from

Ze keeps the static `name-server` list and, apart from it, one record per
learned source: one per DHCP client (DHCPv4 or DHCPv6, per interface and
unit), one per PPPoE client session (the servers IPCP supplied, RFC 1877,
primary first), and one for the host's own `/etc/resolv.conf`. A record
carries how long it stays valid: a DHCPv4 lease until the lease expires
(until withdrawn for an infinite lease), a DHCPv6 Reply until the valid
lifetime of its last binding runs out, an IPCP lease and the host's file
until withdrawn. When a record's validity ends, Ze withdraws it at that
moment, whether or not its client said so.

On every change, one rule selects the router's servers from them, walking
the list `system dns-source` names. Two classes of source can be named
there: `configured` (the `name-server` list) and `received` (every learned
record, DHCPv4 first, then DHCPv6, then PPPoE, then the host's file, each by
interface then unit, in the order each source sent them). The `mode` says
how two classes combine:

| `mode` | The router's servers |
|--------|----------------------|
| `first-usable` | the first class in the list that holds a usable server, alone; a later class is a backup, used only while the earlier ones hold none |
| `merge` | every class, in list order, a server already listed not repeated; Ze's internal resolver tries a later server when an earlier one does not answer |

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

A list naming one source needs no `mode`, because both modes give it the
same answer.

<!-- source: internal/component/config/system/dns_source.go -- RouterDNSSource, routerDNSSourceRefusals, validateRouterDNSSourceTree -->
<!-- source: internal/core/dnssource/select.go -- NewWrittenList, List.Received, ModeRequiredError, ReceivedInterfaceError, ClassRefusedError -->

An address that cannot name a resolver (the unspecified address, a multicast
address, 255.255.255.255) is skipped, whatever its source. A loopback
address such as the 127.0.0.53 of a host stub resolver stays, because Ze's
own lookups can reach it.

<!-- source: internal/component/config/system/resolv.go -- Resolvers, routerDefault, Resolvers.records, ValidUntil, Resolvers.armExpiryLocked -->
<!-- source: internal/core/dnssource/select.go -- Select, Usable -->
<!-- source: internal/plugins/iface/dhcp/resolv_linux.go -- setV4Resolvers, setV6Resolvers -->

The host's `/etc/resolv.conf` keeps a Ze on a standard distribution
resolving when no `name-server` is configured and no lease supplies one. Ze
reads it at startup and on every config reload, not when the file changes.
Ze never reads it when it is the same file as `resolv-conf-path` (symlinks
are followed), because Ze would then read back the servers it wrote.
<!-- source: internal/component/config/system/resolv_host.go -- readHost, sameFile -->
<!-- source: internal/component/config/system/register_dns.go -- hostResolverOrigin -->

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
