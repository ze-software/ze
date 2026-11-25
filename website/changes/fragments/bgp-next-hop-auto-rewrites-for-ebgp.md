---
spec: bgp-next-hop-auto-rewrites-for-ebgp
topic: BGP
feature: bgp-protocol
found-by: interop
---

## Before

Forwarded BGP routes relayed to ordinary eBGP peers under the default `auto`
mode preserved the received next hop instead of rewriting it to Ze's session
address, causing external peers to reject relayed routes whose original next
hop was unreachable. Egress next-hop rewriting was split between
`session { next-hop }` and `policy { modify { set { next-hop } } }`, where
session-level `auto` ran after export policy and silently overwrote conditional
next-hop rules on eBGP, and `policy { modify }` rejected IPv6 next hops. In
addition, raw locally originated UPDATEs could advertise a peer's own address
as the next hop, BGP-LS eBGP IPv4 rewrites emitted mapped IPv6 addresses rather
than native IPv4 (and BGP-LS VPN omitted the required 8-octet zero Route
Distinguisher), FlowSpec scope processing could alter cleared fields, and an
unreadable interface table could misattribute shared scoped BGP listeners.

## After

All BGP egress next-hop rewriting is unified in the export filter chain and
`session { next-hop }` is removed; configs still naming `session { next-hop }`
fail validation with guidance to use export policy. Ze automatically appends
built-in `next-hop:auto` at the tail of every export chain unless an
unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is
already present. Under `next-hop:auto`, ordinary eBGP peers rewrite forwarded
routes to Ze's local session address (withholding announcements while still
sending withdrawals when no usable local address exists), while iBGP peers,
route-server clients, and locally originated routes preserve their received or
originator-chosen next hop.

Operators can write built-in `next-hop:self`, `next-hop:unchanged`, or
`next-hop:auto` directly in an export filter list, or use conditional
`policy { modify <name> { set { next-hop self | unchanged | auto | <ipv4/ipv6>; } } }`
rules. Explicit IPv4 and global IPv6 next hops work on both import and export
chains, while egress-only modes (`self`, `unchanged`, `auto`) are refused on
import chains and `inactive:next-hop:auto` is refused in favor of
`next-hop:unchanged`. Trailing `next-hop:auto` fills only when no earlier
filter set a next hop (and leaves raw UPDATE overrides that modified next-hop
bytes untouched), leaves unconfigured export chains subject to RFC 8212
default-deny, and surfaces alongside other automatic filter members in
`show bgp peer` and implied-filter inspection.

Raw locally originated UPDATEs now enforce the peer-own next-hop prohibition
(matching scoped IPv6 peers zone-free), BGP-LS uses native 4/16/32-octet next
hops and BGP-LS VPN prepends an 8-octet zero Route Distinguisher per address
(12/24/48 octets), FlowSpec scope processing preserves empty and opt-in
redirect targets, and shared scoped BGP listeners fail closed on
interface-table read errors without dropping sockets whose claimant count is
unknown.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
