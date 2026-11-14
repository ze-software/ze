---
spec: rfc-verdict-fix-ospf
topic: OSPF
found-by: review
---

## Before

On broadcast, NBMA and point-to-multipoint links, Ze as an OSPF graceful
restart helper identified the restarting neighbour by its Advertising Router
instead of the Grace-LSA's interface address. The OSPF cryptographic replay
check kept one sequence mark per packet type, not per neighbour. A segment
routing next hop's own NP and E flags were applied only when the best path
ended at it. An unwanted self-originated LSA received back from a neighbour
could be re-originated instead of flushed. When the only remaining key of a
key chain had expired, Ze refused every packet signed with it.

<!-- source: internal/plugins/ospf/gr_helper.go -- graceRestarterV4 -->
<!-- source: internal/plugins/ospf/auth_keystore.go -- replaySlotFor, lastKeyExtendedIndex -->
<!-- source: internal/plugins/ospf/sr_install.go -- forwarding -->
<!-- source: internal/plugins/ospf/lsdb/origination.go -- handleSelfReceived -->

## After

The helper identifies the restarter by the Grace-LSA interface address on
shared media, as FRR and BIRD do. The replay mark is kept per neighbour. The
next hop's own flags apply whenever it advertised the Prefix-SID. An unwanted
self-originated LSA is flushed at MaxAge. The last key of a chain stays usable
after its lifetime ends, as RFC 5709 Section 3.2 directs. Each fix is proven
against FRR 10.7.1 or by unit tests, and the interop scenarios for the SR,
flush and last-key fixes go red when the fix is reverted.

<!-- source: internal/le/interoplab/bgp/check_ospf_sr_anycast.go -- checkOSPFSRAnycastNextHop -->
<!-- source: internal/le/interoplab/bgp/check_ospf_self_flush.go -- ospfSelfLSAFlushOperations -->

## Corrects

none

## Exposure

OSPFv2 deployments using graceful restart helper mode on shared media,
cryptographic authentication, or key chains whose last key expires. OSPFv2
and OSPFv3 deployments using segment routing with anycast or multiply
advertised Prefix-SIDs: one installer serves both address families.

## Upstream

none

## Asked by

none
