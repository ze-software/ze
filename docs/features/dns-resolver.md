# DNS Resolver

<!-- source: internal/component/resolve/dns/resolver.go -- miekg/dns resolver with cache -->
<!-- source: internal/component/resolve/dns/cache.go -- O(1) LRU cache with TTL -->
<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- system DNS config -->

Built-in DNS resolver component providing cached DNS queries to all Ze components.
Uses `github.com/miekg/dns` (the library CoreDNS is built on).

This is the client side: Ze asking somebody else. Ze also answers queries, from
two authoritative plugins that share one server harness
(`internal/core/dnsserver`): `as112` (see
[AS112 guide](../guide/as112.md#answer-policy) for the answer policy the harness
enforces) and `geodns` (per-source-IP answers, `service { geodns { ... } }`).
The two sides share no configuration and no cache.

| Feature | Description |
|---------|-------------|
| Static name servers | `system { name-server [8.8.8.8 1.1.1.1]; }` sets upstream DNS servers |
| resolv.conf writer | Writes configured servers to resolv-conf-path at startup |
| DHCP integration | Static servers take priority over DHCP-discovered DNS |
| Query types | A, AAAA, TXT, PTR, CNAME, MX, NS, SRV |
| LRU cache | O(1) operations, configurable size and max TTL |
| TTL-aware | Respects response TTL, caps at configured maximum, honors TTL=0 (do not cache) |
| Concurrent safe | Mutex-protected cache, safe for multi-goroutine use |
| System fallback | No configured servers uses `/etc/resolv.conf`; if that is missing or empty, queries fail closed with `no DNS server configured` |
| Timeout control | Per-resolver configurable timeout (1-60 seconds) |
| Cache management | List entries, inspect by name, selective delete by name/type, flush all, reset counters |
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

When `name-server` is configured, DHCP-discovered DNS servers do not overwrite
resolv.conf. When no static servers are configured, DHCP writes DNS servers to
resolv-conf-path as before (last-writer-wins across interfaces).

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
clear dns cache                                 # Flush all entries and reset counters
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
They work on any command output, including `show traceroute`, `show bgp summary`, etc.
In `monitor traceroute | log` mode, they enrich the hop legend.

## Reload behavior

DNS resolver settings and resolv.conf are applied at startup. Changing
`name-server` or `dns` settings via config reload requires a process restart
to take effect.
