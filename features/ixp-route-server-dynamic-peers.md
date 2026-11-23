# IXP Route Server Dynamic Peers

## Meta

| Field | Value |
|-------|-------|
| Name | IXP Route Server Dynamic Peers |
| Kind | protocol |
| Scope | partial |
| Scope gaps | Dynamic-peer journal rows of 2026-08-12 and 2026-08-13 remain open |
| Level | experimental |
| Components | internal/component/bgp/plugins/rs, internal/component/bgp/reactor |
| Real-path tests | test/plugin/dynamic-peer-applies-group-filters.ci, test/plugin/dynamic-peer-negotiates-configured-families.ci, test/plugin/dynamic-peer-gets-group-role-capability.ci |
| RFCs | rfc7947 |
| Docs | docs/guide/route-reflection.md |
| Doc review | 2026-11-23: route-reflection.md updated for RFC 7947 Section 2.1 pre-policy Adj-RIB-In retention (`update-filtered-received`) |
| Defect review | 2026-11-23: RFC 7947 Section 2.1 pre-policy Adj-RIB-In retention implemented (`spec-rfc7947-adj-rib-in-accepts-filtered-updates`); dynamic-peer journal rows of 2026-08-12 and 2026-08-13 remain open |
| Extra criteria | supported: dynamic route-server peers against a third-party client = bgp/bgp-route-server-dynamic-frr |

## Description

Route server (`bgp-rs`) supports dynamic peers for IXP deployments. Peers connect dynamically and inherit configuration from a peer group template. RS-client role and per-peer community filtering. Route-server client peers (`rs-client true`) retain ingress-denied UPDATEs in `bgp-adj-rib-in` marked import-filtered on the `update-filtered` rail while omitting them from Loc-RIB, forwarding, and replay (RFC 7947 Section 2.1). <!-- source: internal/component/bgp/reactor/reactor_notify.go -- retainDeniedIngress, enqueueFilteredIngress -->
