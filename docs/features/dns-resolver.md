# DNS Resolver

<!-- source: internal/component/dns/resolver.go -- miekg/dns resolver with cache -->
<!-- source: internal/component/dns/cache.go -- O(1) LRU cache with TTL -->
<!-- source: internal/component/dns/schema/ze-dns-conf.yang -- YANG config schema -->

Built-in DNS resolver component providing cached DNS queries to all Ze components.
Uses `github.com/miekg/dns` (the library CoreDNS is built on).

| Feature | Description |
|---------|-------------|
| Configurable server | YANG `environment/dns/server` sets upstream DNS |
| Query types | A, AAAA, TXT, PTR, CNAME, MX, NS, SRV |
| LRU cache | O(1) operations, configurable size and max TTL |
| TTL-aware | Respects response TTL, caps at configured maximum, honors TTL=0 (do not cache) |
| Concurrent safe | Mutex-protected cache, safe for multi-goroutine use |
| System fallback | Empty server config uses `/etc/resolv.conf` (resolved once at startup) |
| Timeout control | Per-resolver configurable timeout (1-60 seconds) |
| Env override | `ze.dns.server` overrides config file DNS server |

Configured under `environment { dns { } }` in the config file.
