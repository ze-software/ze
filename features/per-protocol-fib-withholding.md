# Per-protocol FIB withholding

## Meta

| Field | Value |
|-------|-------|
| Name | Per-protocol FIB withholding |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/sysrib |
| Real-path tests | test/plugin/fib-withhold-controller-programs-no-route.ci, test/plugin/fib-withhold-one-protocol-keeps-the-other.ci |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; the withhold list and its sweep live in internal/component/sysrib as the anchors name |
| Defect review | 2026-10-07: no plan/immediate spec or journal row found naming the withhold path |

## Description

`rib { fib-withhold [ bgp isis ]; }` names the protocols whose selected routes ze does not write to the forwarding table. The list is empty by default, so every protocol is programmed. It accepts a name only when a protocol registered it, and a commit that names anything else is refused with the names ze accepts. A withheld route is not a dropped route. It stays in the Loc-RIB, and `show rib` and `show ecmp-groups` report it with the equal-cost paths that competed for the prefix, because both commands read the RIB rather than the last group written to the FIB. It still wins the prefix on administrative distance when its distance is the lower one. `redistribute` still moves it into another protocol, and the protocol's own event stream still carries it. Only `system-rib/best-change`, the stream every FIB writer reads, drops it, and the prefix leaves the next-hop table with it, so `show nexthop-table` stops listing it under its next-hop. A commit that changes the list sweeps the routes already programmed: ze withdraws each prefix it has programmed for a protocol the list now names, programs each prefix whose protocol the list no longer names, and rewrites the multipath group of a prefix that lost or gained a member. The sweep is what makes the change reach a converged router, which sends no further update for a prefix. Permitting a protocol programs its prefixes the way selection does, so a next-hop no route in the Loc-RIB covers is programmed all the same: such a gateway is on-link on an OSPF or IS-IS link whose connected route no plugin inserted, and reading it as unreachable would leave the prefix out of the forwarding table for good. An SRv6 Service SID that reaches nothing is the one refusal the sweep keeps. The setting covers every FIB writer. They all read one publish stream, so it cannot write BGP into VPP and withhold it from the kernel. A withheld protocol is no member of an equal-cost group either, so traffic does not reach its gateway through a multipath route a permitted protocol won. A prefix leaving the RIB is withdrawn from the FIB only where ze has an install outstanding for it, so a route ze never programmed produces no delete and no FIB sync error. <!-- source: internal/component/sysrib/yang/ze-rib-conf.yang -- fib-withhold leaf-list --> <!-- source: internal/component/sysrib/fibimport.go -- recordWithheldWinner, applyFIBImport, fibStateChange, untrackNextHops --> <!-- source: internal/component/sysrib/ecmp.go -- ecmpCollect, ecmpRIBGroup --> <!-- source: internal/component/sysrib/sysrib.go -- recomputeBest, fibEntry, programmedByZe, showNHTable -->
