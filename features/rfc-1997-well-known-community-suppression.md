# RFC 1997 well-known community suppression

## Meta

| Field | Value |
|-------|-------|
| Name | RFC 1997 well-known community suppression |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/reactor |
| Real-path tests | test/plugin/wellknown-no-export-egress.ci, test/plugin/wellknown-no-advertise-egress.ci, test/plugin/wellknown-no-export-withdraw-egress.ci |
| Interop | bgp/bgp-wellknown-noexport-frr |
| RFCs | rfc1997 |
| Docs | docs/guide/bgp-policy.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; source-payload scan and treat-as-withdraw are what wellknown-no-export-withdraw-egress.ci asserts |
| Defect review | 2026-10-07: no plan/immediate spec or journal row found naming the well-known community gate |

## Description

NO_EXPORT, NO_ADVERTISE and NO_EXPORT_SUBCONFED are honored on egress automatically, on both forward rails, with no configuration and no switch. The scan runs once per UPDATE over the SOURCE payload the peer sent, never over a policy chain's wire override, so an export policy that strips NO_EXPORT does not license the leak: the route arrived carrying it. A destination the gate refuses is sent the withdrawal of every route that UPDATE names rather than nothing (RFC 7606 Section 2 treat-as-withdraw), so a route already advertised is taken back. This is the ordering that separates Ze from FRR and BIRD, which apply the route-map first, where stripping the community there restores advertisement. <!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- scanWellKnownEgress over the source payload -->
