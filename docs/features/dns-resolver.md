# DNS Resolver

<!-- source: internal/component/resolve/dns/resolver.go -- miekg/dns resolver with cache -->
<!-- source: internal/component/resolve/dns/cache.go -- O(1) LRU cache with TTL -->
<!-- source: internal/component/config/system/schema/ze-system-conf.yang -- system DNS config -->

Built-in DNS resolver component providing cached DNS queries to all Ze components.
Uses `github.com/miekg/dns` (the library CoreDNS is built on).

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

## Reload behavior

DNS resolver settings and resolv.conf are applied at startup. Changing
`name-server` or `dns` settings via config reload requires a process restart
to take effect.
