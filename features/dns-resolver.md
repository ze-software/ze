# DNS Resolver

## Meta

| Field | Value |
|-------|-------|
| Name | DNS Resolver |
| Page | docs/features/dns-resolver.md |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/resolve/dns |
| Real-path tests | test/plugin/dns-cache-show.ci, test/plugin/dns-lookup-show.ci, test/plugin/clear-dns-cache.ci, test/plugin/dns-stub-lookup.ci, test/plugin/dns-stub-answer-change.ci |
| RFCs | rfc1035 |
| Docs | docs/features/dns-resolver.md |
| Doc review | 2026-11-09: checked this description and the docs/features/dns-resolver.md changes since 2026-10-07 (failover on no answer or a server failure, the resolv-conf-path refusal, the dns-source list, its classes and refusals) against internal/component/resolve/dns/resolver.go (Resolver loop, statusFromRcode), internal/component/config/validators_resolvconf.go, internal/component/config/system/resolv.go (routerDefault) and internal/core/dnssource (written.go, select.go); 2026-10-07: checked the fail-closed error 'no DNS server configured' |
| Defect review | 2026-10-07: audit found no open immediate spec and no journal row against the resolver |
| Stub evidence | test/plugin/dns-stub-lookup.ci, test/plugin/dns-stub-answer-change.ci |
| Extra criteria | supported: resolution against a real recursive server = test/plugin/dns-recursive-lookup.ci |

## Description

Built-in cached DNS resolver for all components. Asks the router's servers in order, passing a query to the next server when one gives no answer or answers a server failure. With no `system dns-source` written, those servers are the configured `system.name-server` list when it has an entry, else the servers learned from DHCPv4, DHCPv6 and PPPoE leases and router advertisement RDNSS, or the host's `/etc/resolv.conf` while none of them lists a server; `system dns-source` sets the classes, their mode and the interfaces `received` reads, and `show system dns` prints the learned records and the selection. It follows every lease change and reload, and fails closed with `no DNS server configured` when no source supplies a server. It does not silently fall back to public recursive resolvers. Cache management: `show dns cache list/record`, `clear dns cache all/record/stats` (flush/selective delete/stats reset).
