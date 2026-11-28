---
spec: srv6-bestpath-resolvability
topic: BGP
---

## Before

Ze checked whether a received SRv6 Service SID had a covering underlay route only when programming the forwarding table, after BGP best-path selection and cross-protocol route arbitration had already picked a winner. When the preferred BGP path carried an unreachable SRv6 Service SID, it won best-path, was re-advertised to downstream peers, and blocked both reachable runner-up BGP paths and static backup routes at worse administrative distances, leaving the destination unreachable in the forwarding table with no `show bgp rib` or `show doctor` explanation.

## After

Ze now verifies reachability of the reconstructed SRv6 Service SID before BGP best-path selection across unicast, VPN, and EVPN families (RFC 9252 Sections 5 and 6). Paths whose Service SID cannot resolve in the routing table are excluded from best-path candidacy so reachable runner-up BGP paths or static backups win and install cleanly, while the excluded paths remain retained in Adj-RIB-In and automatically enter or leave selection when IPv6 locator reachability changes with no new BGP UPDATE required. Excluded routes appear in `show bgp rib | match srv6-sid-unresolvable` with `"srv6-sid-resolvable": false` and `"ineligible-reason": "srv6-sid-unresolvable"`, trigger the `doctor-bgp-srv6-sid-unresolvable` warning in `show doctor`, and increment `ze_rib_srv6_sid_unresolvable_exclusions_total`. Operators who steer traffic through an alternate policy or operate a transit route reflector can bypass the BGP reachability check per scope or per address family with `srv6-sid-resolvability { ignore true; family <name> { ignore true|false; } }` under `bgp`, `group`, or `peer`, with immediate re-evaluation on live reload and rollback while keeping strict resolvability as the default and preserving the forwarding-table guard as defense-in-depth.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
