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
| resolv.conf writer | Writes the configured servers, else every DHCP lease's servers, to resolv-conf-path, and rewrites it whenever a source changes |
| DHCP integration | Static servers take priority over DHCP-discovered DNS |
| Query types | A, AAAA, TXT, PTR, CNAME, MX, NS, SRV |
| LRU cache | O(1) operations, configurable size and max TTL |
| TTL-aware | Respects response TTL, caps at configured maximum, honors TTL=0 (do not cache) |
| Concurrent safe | Mutex-protected cache, safe for multi-goroutine use |
| System fallback | No configured servers uses `/etc/resolv.conf`; if that is missing or empty, queries fail closed with `no DNS server configured` |
| Timeout control | Per-resolver configurable timeout (1-60 seconds) |
| Cache management | List entries, inspect by name, and clear with `clear dns cache all/record/stats`: flush everything, delete by name/type, or reset the counters |
| `\| resolve` pipe | Reverse DNS enrichment for IP addresses in any command's JSON output |
| `\| origin` pipe | ASN/network enrichment for IP addresses via Team Cymru DNS queries |

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
| `name-server` | (none) | Static DNS servers. First server used by ze internal resolver. All written to resolv.conf. |
| `resolv-conf-path` | `/tmp/resolv.conf` | Path for resolv.conf. Default suits gokrazy (read-only rootfs). Empty disables writing. |
| `timeout` | 5 | Query timeout in seconds (1-60) |
| `cache-size` | 10000 | Maximum cached entries (0 disables caching) |
| `cache-ttl` | 86400 | Maximum cache TTL in seconds (0 uses response TTL only) |

## DHCP interaction

ze keeps the resolver servers of each source apart: the static `name-server`
list, and one list per DHCP client (DHCPv4 or DHCPv6, per interface and unit).
resolv.conf is rebuilt from them on every change:

| Situation | resolv.conf lists |
|-----------|-------------------|
| `name-server` configured | the static servers alone; DHCP never overrides them (RFC 3646 Section 6) |
| no `name-server` | every DHCP lease's servers: DHCPv4 clients first, then DHCPv6, each by interface then unit, in the order each server sent them, a repeated server listed once |
| a renewal reply omits the DNS option, or a lease ends | that lease's servers leave the file; the other sources' stay (RFC 9915 Section 18.2.10.5 for DHCPv6) |
| every source withdrawn | the ze header with no `nameserver` line, once ze has written the file; a file ze never wrote is left alone |

<!-- source: internal/component/config/system/resolv.go -- Resolvers -->
<!-- source: internal/plugins/iface/dhcp/resolv_linux.go -- setV4Resolvers, setV6Resolvers -->

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

DNS resolver settings are applied at startup. Changing `name-server` or `dns`
settings via config reload requires a process restart to reach the internal
resolver. resolv.conf follows a reload: a reload that changes or removes
`name-server` rewrites it, and removing every static server hands it back to
the DHCP leases.
<!-- source: cmd/ze/hub/main_system.go -- applyResolvConf -->
