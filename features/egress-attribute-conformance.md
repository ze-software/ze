# Egress attribute conformance (RFC 4271 Section 5)

## Meta

| Field | Value |
|-------|-------|
| Name | Egress attribute conformance (RFC 4271 Section 5) |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/reactor |
| Real-path tests | test/plugin/local-pref-strip-ebgp.ci, test/plugin/med-not-propagated-across-as.ci, test/plugin/med-removal-before-decision.ci, test/plugin/med-locally-set-reaches-peer.ci, test/plugin/med-ibgp-post-selection-removal.ci, test/plugin/med-ibgp-ordinary-post-selection-removal.ci, test/plugin/rfc4271-partial-unknown-transitive.ci, test/plugin/rfc4271-attr-flags-low-nibble-relay.ci, test/plugin/rfc8092-large-community-no-repeat-egress.ci |
| Interop | bgp/bgp-local-pref-strip-gobgp, bgp/bgp-med-across-as-gobgp, bgp/bgp-attribute-default-localpref-gobgp, bgp/bgp-relay-withdraw-shape-frr, bgp/bgp-attr-flags-low-nibble-gobgp, bgp/bgp-large-community-no-repeat-gobgp, bgp/bgp-med-ibgp-post-selection-removal-gobgp, bgp/bgp-med-ibgp-ordinary-post-selection-removal-gobgp |
| RFCs | rfc4271, rfc8092 |
| Docs | docs/features/bgp-protocol.md |
| Doc review | 2026-11-23: updated for the RFC 4271 Section 9.1.2.2 post-selection MED preservation row added to docs/features/bgp-protocol.md; matches applyFactsMED in internal/component/bgp/reactor/forward_med.go, ordinary selected-route unit proof in rfc4271_ordinary_med_readvertisement_test.go, and GoBGP interop scenario bgp-med-ibgp-ordinary-post-selection-removal-gobgp. Earlier, 2026-11-23: the sections added to docs/features/bgp-protocol.md (the bgp-rib per-family selection deferral under update-delay max-delay, and bgp aggregate) add to, and contradict nothing in, this declaration. Earlier, 2026-10-19: re-read after the 2026-10-19 docs/features/bgp-protocol.md edits, which added the two RFC 9774 AS_SET rows and the draft-ietf-idr-linklocal-capability 32-octet next-hop row; no claim here names AS_PATH segments or a 32-octet next hop, so the earlier review stands. 2026-10-08: re-read after the 2026-10-08 bgp-protocol.md edit, which only rewrote the next-hop wire-form row (relayed link-local retention, VPN-IPv6 unspecified pair); no claim here names next-hop encoding; Section 5.1.3 own-address refusal still at egressNextHopIsPeerOwn and originatedNextHopIsPeerOwn in forward_next_hop.go; other anchors unchanged |
| Defect review | 2026-11-23: RFC 4271 Section 9.1.2.2 iBGP MED post-selection preservation implemented and backed by ordinary-path unit and GoBGP interop proof; journal NEXT_HOP rail and LOCAL_PREF producer rows taken as open |
| Extra criteria | supported: each egress rule asserted on every rail that writes an UPDATE = test/plugin/egress-rules-every-rail.ci |

## Description

Each rule is enforced at one site and applies on every rail that writes an UPDATE (announce, forward, route server): LOCAL_PREF is never sent on an external session (5.1.5); a received MULTI_EXIT_DISC is not relayed to another neighboring AS (5.1.4), with the route-server client exempt per RFC 7947 Section 2.2.3; a selected MULTI_EXIT_DISC is preserved against raw post-selection egress removal on internal readvertisement (9.1.2.2) without synthesizing MED or overriding an explicit egress Set; no peer is sent its own address as NEXT_HOP (5.1.3); the Partial bit is set on an unrecognized transitive optional attribute (Sections 5 and 9); withdrawals are applied before announces (4.3); a relayed withdrawal creates no attribute (4.3 and 6.3); and attributes are inserted by ascending type code with MP_UNREACH first. The final session writer also clears the four unused low-order bits of every Attribute Flags octet (4.3) and drops a repeated LARGE_COMMUNITY value whichever producer added it (RFC 8092 Section 3), on the raw rail too. <!-- source: internal/component/bgp/reactor/session_egress_normalize.go -- normalizeTransmittedUpdate --> <!-- source: internal/component/bgp/reactor/forward_next_hop.go -- Section 5.1.3 --> <!-- source: internal/component/bgp/reactor/forward_med.go -- Section 5.1.4, Section 9.1.2.2 --> <!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- Section 5.1.5 -->
