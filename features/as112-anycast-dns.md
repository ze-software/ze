# AS112 Anycast DNS

## Meta

| Field | Value |
|-------|-------|
| Name | AS112 Anycast DNS |
| Page | docs/guide/as112.md |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 7858 and RFC 8484 MUST gaps for DoT and DoH, RFC 7535 is not proven, no DNS interop |
| Level | experimental |
| Components | internal/plugins/as112 |
| Real-path tests | test/plugin/as112-enable.ci, test/plugin/as112-health.ci, test/plugin/as112-dot.ci, test/plugin/as112-doh.ci, test/plugin/as112-healthcheck-announce.ci, test/plugin/redistribute-as112-announce.ci, test/parse/as112-config.ci |
| RFCs | rfc7534, rfc7535, rfc7858, rfc8484 |
| Docs | docs/guide/as112.md |
| Doc review | 2026-10-07: checked asn default 112 in internal/plugins/as112/yang/ze-as112-conf.yang; every source anchor resolves |
| Defect review | 2026-10-07: journal guard-added-to-one-half-of-a-pair.md row names the as112 DNS path; journal rows naming a Component, not each re-verified here: guard-added-to-one-half-of-a-pair.md:33, guard-added-to-one-half-of-a-pair.md:35, guard-added-to-one-half-of-a-pair.md:47, validated-value-discarded-by-its-caller.md:14, zero-value-as-valid-answer.md:47 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Authoritative sink for misdirected RFC 1918 / link-local reverse-DNS queries (RFC 7534) and the EMPTY.AS112.ARPA DNAME-redirection zone (RFC 7535). Four fixed anycast host addresses (never operator-typed), registered against the iface address-ownership registry and bound via `IP_FREEBIND`. Optional `allow-from` client-source access list (loopback always permitted). BGP integration composes existing `healthcheck`/`watchdog`/`update`-block mechanisms: the anycast route announces only once a probe against a real anycast service address confirms the DNS service is healthy, with an operator-chosen community and optional AS112-origin AS_PATH override. `show as112`, `as112 health [target <ip>]`, `ze_as112_*` metrics. The four covering prefixes can also be BGP-originated directly through the redistribute path (`redistribute { destination bgp { import as112 } }`) with a configurable origin ASN (`asn`, default 112) and `community`, health-gated by the `watchdog` serving-state check (RFC 7534 Section 3.3). <!-- source: internal/plugins/as112/redistribute.go -- registerAS112Sources --> <!-- source: internal/plugins/as112/register.go -- as112 registration --> <!-- source: internal/component/iface/address_owner.go -- address-ownership registry --> Compile-out-able with the `ze_as112` build tag: default-on in `ZE_FEATURES`, dropped from `ze-stripped` / bare `ze_core` builds, with its config block rejected as unknown. <!-- source: feature-gates.txt -- ze_as112 -->
