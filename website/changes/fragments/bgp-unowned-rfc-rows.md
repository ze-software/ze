---
spec: bgp-unowned-rfc-rows
topic: BGP
found-by: rfc-check
---

## Before

On a session that negotiated the Link-Local Next Hop capability, Ze accepted
a 32-octet IPv6 next hop without checking that it held a global address
followed by a link-local one. When Ze advertised a route whose next hop was
another router on the subnet it shares with the peer, it sent that router's
global address alone, without the router's link-local address. An SR Policy
route could carry an SRv6 Binding SID only in the type-20 sub-TLV, so a peer
that reads only the older type-13 form got none, and `ze bgp decode` showed a
received type-13 SRv6 Binding SID as raw hex.

## After

A 32-octet next hop on such a session must be a global address (or `::`)
followed by a link-local one, or two link-local addresses. Anything else is
treated as a withdrawal of the routes in that UPDATE, as the capability's
draft requires; a wrong next-hop length still resets the session.

<!-- source: internal/component/bgp/message/rfc7606_linklocal.go -- validateLinkLocalNextHopPair, linkLocalNextHopPairWellFormed -->

A next hop that names another router on the shared subnet now carries that
router's link-local address after its global one, as RFC 2545 Section 3 asks.
Ze reads it from the kernel neighbor table and follows the table as it
changes, with or without an `interface` block in the config. When the table
names no usable entry for that router, the global address goes alone and a
rate-limited warning names the next hop and the reason. When the table cannot
be read, one warning per peer names the cause, and `ze doctor` reports it. A
route already sent with the global address alone is not sent again when the
table later names the link-local address.

<!-- source: internal/component/bgp/reactor/link_scope.go -- linkLocalNextHop, thirdPartyLinkLocal -->
<!-- source: internal/component/bgp/reactor/reactor_iface.go -- startNeighborWatch, neighborWatchWorker -->

`binding-sid srv6 <ipv6-address>` on an SR Policy route writes the SRv6
Binding SID in the type-13 sub-TLV at Length 18, for peers that followed the
early drafts. `srv6-binding-sid` (type 20) stays the form RFC 9830 recommends.
An IPv4, IPv4-mapped or zoned address is refused when the config loads.
`ze bgp decode` shows a received type-13 sub-TLV of Length 18 as its SRv6 SID.

<!-- source: internal/component/bgp/plugins/nlri/srpolicy/config.go -- buildBindingSIDSRv6SubTLV, parseBindingSIDSRv6 -->
<!-- source: internal/component/bgp/plugins/nlri/srpolicy/attr_json.go -- bindingSIDToJSON -->

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
