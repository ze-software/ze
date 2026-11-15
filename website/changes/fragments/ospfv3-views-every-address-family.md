---
spec: ospfv3-views-every-address-family
topic: OSPF
feature: ospf
found-by: interop
---

## Before

With several OSPFv3 address families configured (RFC 5838), every `show ospf
ipv6` view except `show ospf ipv6` and `show ospf ipv6 instance` read only the
IPv6-unicast instance. An IPv4-unicast instance with a Full adjacency showed no
neighbor, no database and no SPF history, and `show ospf ipv6 neighbor`
answered an empty list when IPv4 unicast was the only family. No command showed
the routes an OSPFv3 instance computed, and `debug ipv6 ospf inject lsa` could
only reach the IPv6-unicast database.

<!-- source: internal/plugins/ospf/register.go -- runOSPFEngine -->

## After

Every `show ospf ipv6` view (neighbor, interface, SPF, graceful restart,
segment routing, every database view and both router-information views)
answers for each running address family, and each row names its
`address-family` and `instance-id`, which `| table` shows as the first two
columns. An optional `address-family ipv6-unicast|ipv6-multicast|ipv4-unicast|ipv4-multicast`
keyword narrows a view to one instance; a family that is not running is refused
with the running ones named. The new `show ospf ipv6 route` lists the routes each
OSPFv3 instance computed. `debug ipv6 ospf inject lsa` takes the same keyword
to pick the database that receives the LSA, and stays refused until injection
is armed. `show ospf ipv6 graceful-restart` and `show ospf ipv6 segment-routing`
now answer a list, one record per address family, where they answered one
object.

<!-- source: internal/plugins/ospf/debug_wiring.go -- v3Views -->
<!-- source: internal/plugins/ospf/register_multiaf.go -- selectInstances, afTaggedRow -->
<!-- source: internal/plugins/ospf/inject_v3.go -- debugInjectIntoAF -->

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
