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
| `name-server` | (none) | Static DNS servers. When set, they are the router's servers: the internal resolver asks them in order, and resolv.conf lists them all. |
| `resolv-conf-path` | `/tmp/resolv.conf` | Path for resolv.conf. Default suits gokrazy (read-only rootfs). Empty disables writing. |
| `timeout` | 5 | Query timeout in seconds for each server asked (1-60) |
| `cache-size` | 10000 | Maximum cached entries (0 disables caching) |
| `cache-ttl` | 86400 | Maximum cache TTL in seconds (0 uses response TTL only) |

## Where the router's servers come from

Ze keeps the static `name-server` list and, apart from it, one record per
learned source: one per DHCP client (DHCPv4 or DHCPv6, per interface and
unit), one per PPPoE client session (the servers IPCP supplied, RFC 1877,
primary first), and one for the host's own `/etc/resolv.conf`. On every change, one rule selects the router's servers
from them. The rule takes the configured servers first, and the learned
records only when no server is configured:

| Situation | The router's servers |
|-----------|----------------------|
| `name-server` configured | the static servers alone; a learned server never overrides them (RFC 3646 Section 6) |
| no `name-server` | every learned record's servers: DHCPv4 first, then DHCPv6, then PPPoE, then the host's `/etc/resolv.conf`, each by interface then unit, in the order each source sent them. A server already listed is not repeated |
| a renewal reply omits the DNS option, or a lease ends | that lease's servers leave the selection; the other sources' stay (RFC 9915 Section 18.2.10.5 for DHCPv6) |
| every source withdrawn | no server: queries fail closed, and resolv.conf holds the ze header with no `nameserver` line once ze has written it. A file ze never wrote is left alone |

An address that cannot name a resolver (the unspecified address, a multicast
address, 255.255.255.255) is skipped, whatever its source. A loopback
address such as the 127.0.0.53 of a host stub resolver stays, because Ze's
own lookups can reach it.

<!-- source: internal/component/config/system/resolv.go -- Resolvers, routerDefault, Resolvers.records -->
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

A reload that changes `name-server` reaches the internal resolver and
resolv.conf at once: the reload re-runs the selection, and removing every
static server hands both back to the learned records. The `dns` settings
(`timeout`, `cache-size`, `cache-ttl`) are applied at startup, and a change to
them needs a process restart.
<!-- source: cmd/ze/hub/main_system.go -- applyResolvConf, newResolvers -->
