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
| Doc review | 2026-10-07: checked the fail-closed error 'no DNS server configured' (internal/component/resolve/dns/resolver.go) |
| Defect review | 2026-10-07: audit found no open immediate spec and no journal row against the resolver |
| Stub evidence | test/plugin/dns-stub-lookup.ci, test/plugin/dns-stub-answer-change.ci |
| Extra criteria | supported: resolution against a real recursive server = test/plugin/dns-recursive-lookup.ci |

## Description

Built-in cached DNS resolver for all components. Asks the router's servers in order, passing a query to the next server when one gives no answer: the configured `system.name-server` list when it has an entry, else the servers learned from DHCPv4, DHCPv6 and PPPoE leases, then the host's `/etc/resolv.conf`. It follows every lease change and reload, and fails closed with `no DNS server configured` when no source supplies a server. It does not silently fall back to public recursive resolvers. Cache management: `show dns cache list/record`, `clear dns cache all/record/stats` (flush/selective delete/stats reset).
