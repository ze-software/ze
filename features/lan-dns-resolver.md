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
| Real-path tests | test/plugin/dns-resolver-config-refused.ci, test/plugin/dns-resolver-listener-conflict.ci, test/plugin/dns-resolver-forward.ci, test/plugin/dns-resolver-acl-refused.ci, test/plugin/dns-resolver-timeout.ci, test/plugin/dns-resolver-timeout-recovers.ci, test/plugin/dns-resolver-failover.ci, test/plugin/dns-resolver-show.ci, test/reload/dns-resolver-reload.ci, test/plugin/dns-resolver-forward-default.ci, test/plugin/dns-resolver-forward-dhcpv4-default.ci, test/plugin/dns-resolver-forward-received.ci, test/pppoe/dns-resolver-forward-ppp-lease.ci |
| Interop | bgp/dns-resolver-forward-unbound, bgp/dns-resolver-dhcp-lan, l2tp/dns-resolver-l2tp-subscriber |
| RFCs | rfc1035, rfc2181, rfc4035 |
| Docs | docs/guide/dns-resolver.md, docs/architecture/dns/lan-resolver.md |
| Doc review | 2026-11-09: rechecked this description and the `forward dns-source` rows and pass-through default of docs/guide/dns-resolver.md and the forwarding list of docs/architecture/dns/lan-resolver.md against internal/plugins/dnsresolver (config.go forwardDefault, forwardRefusals and parseForwardSources, state.go forwardUpstreams and resolver.relearn, publish.go) and internal/core/dnssource (select.go List.Received); 2026-10-28: checked every other claim against server.go answer, resolve.go forward and absorb, cache.go, limit.go and doctor.go, and the quick-start config passes ze config validate |
| Defect review | 2026-11-09: open: plan/release/fixes/spec-dropped-error-4-system-plugins.md names internal/plugins/dnsresolver; spec-home-cpe-23b-dns-source-consumers closed 2026-11-09 |

## Description

Caching LAN DNS resolver (`service dns-resolver`) for the hosts behind Ze. It answers recursive queries on the listener addresses the operator names (UDP and TCP, wildcard addresses refused), only from `allow-from` source prefixes, and resolves by forwarding, with failover, TCP retry on truncation and RFC 5452 reply matching, to the upstreams its `forward dns-source` list selects. In that list `configured` is the ordered `forward server` list and `received` the resolvers Ze learned from DHCPv4, DHCPv6, IPCP and RA RDNSS, narrowed by `received-interface`; two sources combine `first-usable` or `merge`, and `self` and `radius` are refused. With no list written it asks `forward server` when one is set, else the learned resolvers, and it follows each lease change without rebinding its listeners or emptying its caches. Answer and negative caches with TTL caps, trust ranking and bailiwick checks; bounds on outstanding resolutions, per-client rate and per-query work. `show dns resolver`, `clear dns resolver cache all`, its selection in `show system dns`, two `ze doctor` checks of its own and the `doctor-dns-source-*` warnings over its list, and `ze_dns_resolver_*` metrics. It does not recurse from the root and does not validate DNSSEC. <!-- source: internal/plugins/dnsresolver/register.go -- dnsresolver registration --> <!-- source: internal/plugins/dnsresolver/resolve.go -- resolver.forward, resolver.absorb --> <!-- source: internal/plugins/dnsresolver/config.go -- forwardDefault, forwardRefusals, parseForwardSources --> <!-- source: internal/plugins/dnsresolver/state.go -- forwardUpstreams, resolver.relearn --> <!-- source: internal/component/config/system/doctor_dns_source.go -- doctor-dns-source warnings -->
