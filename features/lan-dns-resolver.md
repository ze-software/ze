# LAN DNS Resolver

## Meta

| Field | Value |
|-------|-------|
| Name | LAN DNS Resolver |
| Page | docs/guide/dns-resolver.md |
| Kind | protocol |
| Scope | partial |
| Scope gaps | forwarding only: no recursion from the root (spec-dns-resolver-recursion), no DNSSEC validation (spec-dns-resolver-dnssec-validation), no DoT or DoH listener |
| Level | experimental |
| Components | internal/plugins/dnsresolver |
| Real-path tests | test/plugin/dns-resolver-config-refused.ci, test/plugin/dns-resolver-listener-conflict.ci, test/plugin/dns-resolver-forward.ci, test/plugin/dns-resolver-acl-refused.ci, test/plugin/dns-resolver-timeout.ci, test/plugin/dns-resolver-timeout-recovers.ci, test/plugin/dns-resolver-failover.ci, test/plugin/dns-resolver-show.ci, test/reload/dns-resolver-reload.ci |
| Interop | bgp/dns-resolver-forward-unbound, bgp/dns-resolver-dhcp-lan, l2tp/dns-resolver-l2tp-subscriber |
| RFCs | rfc1035, rfc2181, rfc4035 |
| Docs | docs/guide/dns-resolver.md, docs/architecture/dns/lan-resolver.md |
| Doc review | 2026-10-28: checked every claim against internal/plugins/dnsresolver (server.go answer, resolve.go forward and absorb, cache.go, limit.go, doctor.go); the quick-start config passes ze config validate |
| Defect review | 2026-10-27: no plan/release/fixes spec and no journal row names internal/plugins/dnsresolver |

## Description

Caching LAN DNS resolver (`service dns-resolver`) for the hosts behind Ze. It answers recursive queries on the listener addresses the operator names (UDP and TCP, wildcard addresses refused), only from `allow-from` source prefixes, and resolves by forwarding to the ordered `forward server` list with failover, TCP retry on truncation and RFC 5452 reply matching. Answer and negative caches with TTL caps, trust ranking and bailiwick checks; bounds on outstanding resolutions, per-client rate and per-query work. `show dns-resolver`, `clear dns-resolver cache all`, two `ze doctor` checks and `ze_dns_resolver_*` metrics. It does not recurse from the root and does not validate DNSSEC. <!-- source: internal/plugins/dnsresolver/register.go -- dnsresolver registration --> <!-- source: internal/plugins/dnsresolver/resolve.go -- resolver.forward, resolver.absorb -->
