# Typed-family NLRI discard (RFC 7606 Section 5.4)

## Meta

| Field | Value |
|-------|-------|
| Name | Typed-family NLRI discard (RFC 7606 Section 5.4) |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/plugins/nlri |
| Real-path tests | test/plugin/rfc7606-54-discard-unrecognized-nlri.ci, test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci, test/plugin/rfc7606-54-bgpls-override-propagates.ci, test/plugin/mup-t1st-malformed-treat-as-withdraw.ci, test/plugin/mup-dsd-without-prefix-sid-withdraw.ci, test/plugin/mup-dsd-ebgp-prefix-sid-discarded-withdraw.ci, test/plugin/mup-dsd-nexthop-locator-mismatch-withdraw.ci, test/plugin/mup-dsd-locator-unresolved-kept.ci, test/plugin/mup-isd-without-prefix-sid-withdraw.ci, test/plugin/mup-isd-originator-mismatch-withdraw.ci |
| Interop | bgp/bgp-rfc7606-typed-nlri-discard, bgp/bgp-mup-malformed-t1st-withdraw-gobgp, bgp/bgp-mup-dsd-gobgp |
| RFCs | rfc7606, draft-ietf-bess-mup-safi |
| Docs | docs/features/bgp-protocol.md |
| Stub evidence | bgp/bgp-rfc7606-typed-nlri-discard |
| Doc review | 2026-11-28: updated docs/features/bgp-protocol.md and this description for draft-ietf-bess-mup-safi Sections 3.3.3 (ISD) and 3.3.6 (DSD) Prefix-SID presence and IGP locator-originator validation, including EBGP Prefix-SID discard and unresolved keep-and-warn (doctor-bgp-mup-locator-unresolved). Earlier, 2026-11-23: the sections added to docs/features/bgp-protocol.md (the bgp-rib per-family selection deferral under update-delay max-delay, and bgp aggregate) add to, and contradict nothing in, this declaration. Earlier, 2026-10-19: re-read after the 2026-10-19 docs/features/bgp-protocol.md edits, which added the two RFC 9774 AS_SET rows and the draft-ietf-idr-linklocal-capability 32-octet next-hop row; no claim here names AS_PATH segments or a 32-octet next hop, so the earlier review stands. 2026-10-18: bgp-protocol.md gained the MUP Section 3.3.6 UPDATE-level check (a DSD route with no Prefix-SID is withdrawn), which this description now states. 2026-10-08: re-read after the 2026-10-08 bgp-protocol.md edit, which only rewrote the next-hop wire-form row (relayed link-local retention, VPN-IPv6 unspecified pair); no claim here names next-hop encoding; RFC 7606 Section 5.4 row untouched, 2026-10-07 review stands |
| Defect review | 2026-10-07: no plan/immediate spec or journal row found naming these NLRI plugins |
| Extra criteria | supported: MCAST-VPN unrecognized route type discarded through the daemon = test/plugin/rfc7606-54-discard-unrecognized-mcast-vpn-nlri.ci |

## Description

Answered per family by the plugin that owns it, because Section 5.4 leaves the ruling to the family's own document. EVPN, MCAST-VPN and MUP discard an unrecognized route type. BGP-LS does not: an unrecognized NLRI type is its common case, and RFC 9552 Section 5.1 requires those to be preserved and propagated rather than treated as an error, so only that NLRI is reported unparsed and decoding continues. MUP also answers a third verdict: a Type 1 Session Transformed route whose mandatory fields overrun its declared Length is treated as withdrawn (draft-ietf-bess-mup-safi Section 3.1.3.1), so an earlier generation of that route is withdrawn and the withdrawal relayed, while one Ze cannot key is discarded. Both Direct Segment Discovery (Section 3.3.6) and Interwork Segment Discovery (Section 3.3.3) routes are checked through an UPDATE-level check registered beside MUP's recognizer: either route type announced with no Prefix-SID attribute (including when RFC 8669 Section 4 discards Prefix-SID on an EBGP peer without `accept-srv6-prefix-sid true`) is treated as withdrawn, and a DSD `MP_REACH_NLRI` next hop (Section 3.3.6) or an ISD NLRI Address field (Section 3.3.3, ignoring the `MP_REACH_NLRI` next hop) that disagrees with the IS-IS SRv6 locator originator is treated as withdrawn with `doctor-bgp-mup-locator-mismatch`, while an unresolved locator keeps the route unverified and warns with `doctor-bgp-mup-locator-unresolved`. <!-- source: internal/component/bgp/plugins/nlri/mup/rfc7606.go -- recognizerFor --> <!-- source: internal/component/bgp/plugins/nlri/mup/rfc7606.go -- updateCheckFor, checkSegmentDiscovery, checkDSDOriginator, checkISDOriginator --> <!-- source: internal/core/bgp/nlri/nlritype/nlritype.go -- Retain --> <!-- source: internal/component/bgp/plugins/nlri/mup/rfc7606.go -- typed-NLRI ruling --> <!-- source: internal/component/bgp/plugins/nlri/ls/plugin.go -- RFC 9552 Section 5.1 preservation -->
