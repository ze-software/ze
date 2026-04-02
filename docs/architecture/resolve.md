# Resolution Component

<!-- source: internal/component/resolve/resolvers.go -- Resolvers container -->

The resolution component (`internal/component/resolve/`) consolidates all external
data resolution services under a unified tree. Each resolver keeps its own typed API
and is constructed explicitly at hub startup.

## Structure

| Package | Purpose | Cache |
|---------|---------|-------|
| `resolve/` | `Resolvers` container struct | N/A |
| `resolve/cache/` | Generic TTL cache (map + mutex + expiry) | Shared by Cymru, PeeringDB, IRR |
| `resolve/dns/` | DNS resolver (miekg/dns wire protocol) | Own TTL-from-response LRU cache |
| `resolve/cymru/` | Team Cymru ASN name resolution via TXT DNS | 1h via shared cache |
| `resolve/peeringdb/` | PeeringDB HTTP client for prefix counts | 1s rate limit between API calls |
| `resolve/irr/` | IRR whois client for AS-SET expansion | N/A (callers manage staleness) |

<!-- source: internal/component/resolve/dns/resolver.go -- DNS resolver -->
<!-- source: internal/component/resolve/cymru/cymru.go -- Cymru resolver -->
<!-- source: internal/component/resolve/peeringdb/client.go -- PeeringDB client -->
<!-- source: internal/component/resolve/irr/client.go -- IRR whois client -->

## Construction

Hub startup creates a single `Resolvers` struct with one shared DNS instance.
Cymru receives a TXT resolver function wired to the DNS resolver. PeeringDB
and IRR are created independently with their configured server addresses.

<!-- source: cmd/ze/hub/main.go -- newResolvers function -->

## Consumers

| Consumer | Resolver | Entry Point |
|----------|----------|-------------|
| Web UI ASN decoration | Cymru | `decorator_asn.go` via `NewASNNameDecoratorFromCymru` |
| Looking glass graph | Cymru | `LGConfig.DecorateASN` callback |
| Prefix update command | PeeringDB | `prefix_update.go` imports `resolve/peeringdb` |

## Dependencies

```
cymru --> resolve/dns (sibling import, TXT queries)
peeringdb --> resolve/irr (AS-SET name validation)
```

These are genuine data dependencies, not architectural coupling.

## DNS Config

DNS resolver configuration comes from YANG (`ze-dns-conf.yang`):
`environment/dns` with leaves: `server`, `timeout`, `cache-size`, `cache-ttl`.

<!-- source: internal/component/resolve/dns/schema/ze-dns-conf.yang -- DNS YANG config -->
