# Typed-family NLRI discard (RFC 7606 Section 5.4)

## Meta

| Field | Value |
|-------|-------|
| Name | Typed-family NLRI discard (RFC 7606 Section 5.4) |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/plugins/nlri |
| Real-path tests | test/plugin/rfc7606-54-discard-unrecognized-nlri.ci, test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci, test/plugin/rfc7606-54-bgpls-override-propagates.ci |
| Interop | bgp/bgp-rfc7606-typed-nlri-discard |
| RFCs | rfc7606 |
| Docs | docs/features/bgp-protocol.md |
| Stub evidence | bgp/bgp-rfc7606-typed-nlri-discard |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; BGP-LS preserves unknown NLRI types and MUP discards per the anchored plugins |
| Defect review | 2026-10-07: no plan/immediate spec or journal row found naming these NLRI plugins |
| Extra criteria | supported: MCAST-VPN unrecognized route type discarded through the daemon = test/plugin/rfc7606-54-discard-unrecognized-mcast-vpn-nlri.ci |

## Description

Answered per family by the plugin that owns it, because Section 5.4 leaves the ruling to the family's own document. EVPN, MCAST-VPN and MUP discard an unrecognized route type. BGP-LS does not: an unrecognized NLRI type is its common case, and RFC 9552 Section 5.1 requires those to be preserved and propagated rather than treated as an error, so only that NLRI is reported unparsed and decoding continues. <!-- source: internal/component/bgp/plugins/nlri/mup/rfc7606.go -- typed-NLRI ruling --> <!-- source: internal/component/bgp/plugins/nlri/ls/plugin.go -- RFC 9552 Section 5.1 preservation -->
