# Connected Routes

## Meta

| Field | Value |
|-------|-------|
| Name | Connected Routes |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/connected |
| Real-path tests | test/plugin/connected-distance-arbitration.ci, test/plugin/connected-distance-raised-loses.ci |
| Docs | docs/guide/redistribution.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; Loc-RIB distance arbitration is what connected-distance-*.ci assert |
| Defect review | 2026-10-07: plan/immediate/spec-connected-static-reach-the-locrib.md and plan/immediate/spec-fixit-redistribution-chain-drops-silently.md open |
| Extra criteria | supported: an interface address add announces the prefix to a BGP peer and its removal withdraws it = test/plugin/connected-redistribute-announce-withdraw.ci |

## Description

Redistribute directly connected interface prefixes into BGP via `redistribute { import connected }`. Subscribes to interface address events; emits RouteChangeBatch on address add/remove. Reference-counted: multiple addresses on the same prefix emit one announcement, withdrawn only when the last address is removed. IPv4 and IPv6. The prefix is also a path in the shared Loc-RIB, so `rib { distance { connected N } }` ranks it against BGP, OSPF, IS-IS and static for the same prefix, and it makes a next-hop covered only by an interface prefix resolvable. Ze still programs no kernel route for it: the protocol declares that the OS installs its entries, so a connected winner WITHDRAWS the route Ze had programmed rather than adding a second one beside the kernel's own. <!-- source: internal/plugins/connected/connected.go -- route observer --> <!-- source: internal/plugins/connected/locrib.go -- the Loc-RIB path and its distance --> <!-- source: internal/plugins/connected/events/events.go -- redistevents producer and the OS-installed declaration --> <!-- source: internal/plugins/static/backend_linux.go -- netlink multipath backend -->
