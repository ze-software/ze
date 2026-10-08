# Egress attribute conformance (RFC 4271 Section 5)

## Meta

| Field | Value |
|-------|-------|
| Name | Egress attribute conformance (RFC 4271 Section 5) |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 4271 iBGP MED re-advertisement (plan/immediate/spec-rfc4271-med-ibgp-readvertisement.md) |
| Level | experimental |
| Components | internal/component/bgp/reactor |
| Real-path tests | test/plugin/local-pref-strip-ebgp.ci, test/plugin/med-not-propagated-across-as.ci, test/plugin/med-removal-before-decision.ci, test/plugin/med-locally-set-reaches-peer.ci, test/plugin/rfc4271-partial-unknown-transitive.ci |
| Interop | bgp/bgp-local-pref-strip-gobgp, bgp/bgp-med-across-as-gobgp, bgp/bgp-attribute-default-localpref-gobgp, bgp/bgp-relay-withdraw-shape-frr |
| RFCs | rfc4271 |
| Docs | docs/features/bgp-protocol.md |
| Doc review | 2026-10-08: re-read after the 2026-10-08 bgp-protocol.md edit, which only rewrote the next-hop wire-form row (relayed link-local retention, VPN-IPv6 unspecified pair); no claim here names next-hop encoding; Section 5.1.3 own-address refusal still at egressNextHopIsPeerOwn and originatedNextHopIsPeerOwn in forward_next_hop.go; other anchors unchanged |
| Defect review | 2026-10-07: plan/immediate/spec-rfc4271-med-ibgp-readvertisement.md open; journal NEXT_HOP rail and LOCAL_PREF producer rows taken as open |
| Extra criteria | supported: each egress rule asserted on every rail that writes an UPDATE = test/plugin/egress-rules-every-rail.ci |

## Description

Each rule is enforced at one site and applies on every rail that writes an UPDATE (announce, forward, route server): LOCAL_PREF is never sent on an external session (5.1.5); a received MULTI_EXIT_DISC is not relayed to another neighboring AS (5.1.4), with the route-server client exempt per RFC 7947 Section 2.2.3; no peer is sent its own address as NEXT_HOP (5.1.3); the Partial bit is set on an unrecognized transitive optional attribute (Sections 5 and 9); withdrawals are applied before announces (4.3); a relayed withdrawal creates no attribute (4.3 and 6.3); and attributes are inserted by ascending type code with MP_UNREACH first. <!-- source: internal/component/bgp/reactor/forward_next_hop.go -- Section 5.1.3 --> <!-- source: internal/component/bgp/reactor/forward_med.go -- Section 5.1.4 --> <!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- Section 5.1.5 -->
