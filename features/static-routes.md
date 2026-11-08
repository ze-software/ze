# Static Routes

## Meta

| Field | Value |
|-------|-------|
| Name | Static Routes |
| Page | docs/guide/static-routes.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/static |
| Real-path tests | test/static/static-boot-apply.ci, test/static/static-distance-beats-ebgp.ci, test/static/static-distance-loses-to-ebgp.ci, test/static/static-interface-nexthop-no-backend.ci, test/static/static-interface-nexthop-pppoe-absent.ci, test/static/static-interface-nexthop-pppoe-follows-device.ci, test/static/static-interface-nexthop-pppoe-same-commit.ci, test/static/static-no-fib-block-loads-the-fib-plugin.ci, test/static/static-per-route-isolation.ci, test/static/static-reload-add.ci, test/static/static-reload-empty-section-withdraws.ci, test/static/static-reload-remove.ci, test/static/static-show.ci, test/static/static-table-interface.ci |
| Docs | docs/guide/static-routes.md |
| Doc review | 2026-11-08: the on-demand interface clause matches internal/plugins/static/link.go (ensureLinkLocked reads the device state, setLinkLocked and markDevice set nhState.waiting from LinkUp and LinkDown and reprogram) and inject.go (programRouteLocked programs the active members and withdraws when none is left); docs/guide/static-routes.md "Interface-only next-hops" and "Mixed ECMP" describe the same behaviour, proven by static-interface-nexthop-pppoe-absent.ci, -same-commit.ci and -follows-device.ci. Earlier, 2026-10-07: every source anchor in the Description resolves to its file and symbol; the MAIN-table Loc-RIB path and FIB plugin autoload are exercised by static-distance-*.ci and static-no-fib-block-loads-the-fib-plugin.ci |
| Defect review | 2026-10-07: spec-fixit-redistribution-chain-drops-silently closed 2026-10-31; spec-static-route-tag-reaches-no-consumer and spec-connected-static-reach-the-locrib closed 2026-10-08 |
| Extra criteria | supported: BFD-tracked failover removes and restores an ECMP next-hop through the daemon = test/static/static-bfd-failover.ci |

## Description

Config-driven static route plugin with named routing tables (policy-based routing), interface-only next-hops (PPPoE/GRE tunnels), mixed ECMP (gateway + interface-only in same group), next-hops through an interface Ze creates on demand (a `pppoe-client`), which wait as inactive members while the device is absent and join or leave the group as it comes up or goes, with no reload, ECMP (multiple active next-hops), per-next-hop weighted load balancing, BFD-tracked failover (next-hop removed from ECMP group on session DOWN, re-added on UP), blackhole/reject, IPv4/IPv6, config reload reconciliation, and redistribute integration (`redistribute { import static }`). Named tables resolved via routing-table registry; non-default table routes are PBR-only (not redistributed into BGP) and are programmed directly, via netlink multipath or VPP via GoVPP. A MAIN-table route is a Loc-RIB path instead: `rib { distance { static N } }` ranks it against BGP, OSPF and IS-IS for the same prefix, and the FIB plugin programs the winner as `RTPROT_ZE`, so one writer owns the entry. Such a route needs a FIB plugin and Ze loads one: static declares that it needs a data plane, each FIB plugin declares the data plane it programs, and the engine resolves the pair against the backend `interface { backend }` selects, so no `fib { }` block is required and an explicit one always wins. <!-- source: internal/plugins/static/register.go -- plugin registration --> <!-- source: internal/plugins/static/locrib.go -- main-table Loc-RIB insertion and the declared distance --> <!-- source: internal/plugins/static/inject.go -- BFD integration, ECMP group management --> <!-- source: internal/plugins/static/link.go -- ensureLinkLocked, setLinkLocked: device presence as a next-hop activity input -->
