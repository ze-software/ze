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

Built-in cached DNS resolver for all components. Uses configured `system.name-server` or resolv.conf, and fails closed with `no DNS server configured` when neither is available. It does not silently fall back to public recursive resolvers. Cache management: `show dns cache list/record`, `clear dns cache` (flush/selective delete/stats reset).
