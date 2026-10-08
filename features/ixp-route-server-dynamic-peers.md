# IXP Route Server Dynamic Peers

## Meta

| Field | Value |
|-------|-------|
| Name | IXP Route Server Dynamic Peers |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 7947 Section 2.1 Adj-RIB-In holds post-policy routes (plan/immediate/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md) |
| Level | experimental |
| Components | internal/component/bgp/plugins/rs, internal/component/bgp/reactor |
| Real-path tests | test/plugin/dynamic-peer-applies-group-filters.ci, test/plugin/dynamic-peer-negotiates-configured-families.ci, test/plugin/dynamic-peer-gets-group-role-capability.ci |
| RFCs | rfc7947 |
| Docs | docs/guide/route-reflection.md |
| Doc review | 2026-10-08: route-reflection.md 2026-10-08 edit adds only the bgp-rr readiness-on-replay-error paragraph; nothing on rs dynamic peers or the Adj-RIB-In gap changed, 2026-10-07 review stands |
| Defect review | 2026-10-07: judged at HEAD; plan/immediate/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md open; dynamic-peer journal rows of 2026-08-12 and 2026-08-13 taken as open |
| Extra criteria | supported: dynamic route-server peers against a third-party client = bgp/bgp-route-server-dynamic-frr |

## Description

Route server (`bgp-rs`) supports dynamic peers for IXP deployments. Peers connect dynamically and inherit configuration from a peer group template. RS-client role and per-peer community filtering. RFC 7947 Section 2.1 is not met: an UPDATE that an ingress filter denies never reaches the Adj-RIB-In, so `show bgp adj-rib-in` holds the post-policy routes. <!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyMessageReceiver -->
