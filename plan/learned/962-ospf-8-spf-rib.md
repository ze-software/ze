# 962 - OSPFv2 SPF and RIB install

## Context

Implemented `plan/spec-ospf-8-spf-rib.md`: intra-area OSPFv2 SPF, route table, Loc-RIB install, ECMP, SPF throttle, snapshots, and route-install tests.

## Decisions

- Mirror IS-IS structure, not code: `spf/graph.go`, `spf/spf.go`, `spf/route.go`, `spf/install.go`, `spf/computer.go`, plus `spf_wiring.go` in the plugin root.
- Keep OSPF route install as Loc-RIB insertion only. `redistevents` remains redistribution-only for later specs.
- Publish one `locrib.Path` per equal-cost next-hop with distinct `Instance`; OSPF resolves intra-area preference before insertion because `locrib.Path` has no route-type field.
- Live kernel RTPROT_ZE, multipath, and withdrawal assertions need Linux raw sockets/veth and stay with spec-ospf-13 FRR/QEMU, matching the IS-IS split.

## Gotchas

- Existing Loc-RIB ECMP expansion was incomplete: it carried siblings only when the primary best path changed. Protocols inserting a later equal-cost sibling need a membership-only `ChangeUpdate`, otherwise sysrib/fibkernel see only the first next-hop.
- Membership-only `ChangeUpdate` must not carry the new sibling's `ForwardHandle` when the selected best path did not change. That handle belongs to the inserted sibling, not the `Best` path in the event.
- Loc-RIB snapshot replay is a separate ECMP path from live mutation dispatch. sysrib must carry `PathGroup.ECMPNextHops` into the synthetic add during startup, otherwise routes installed before sysrib subscribes collapse to the primary next-hop.
- Interface-down self-LSA refresh has two parts: schedule origination without deadlocking engine stop paths, and keep the down active interface in the topology long enough to preserve area membership. Origination, not the topology snapshot, suppresses links for a down active interface and flushes its stale Network-LSA. Passive and loopback stubs stay advertised.
- `.ci` command parsing uses whitespace splitting, not shell quoting. Quoted `-run` regexes are passed literally. With tmpfs files, commands run from the tmpfs directory, so repo-relative `go test ./...` commands fail unless the test is kept in a non-tmpfs `.ci` or run from a script that finds the repo.

## Verification anchors

- `TestOnChangeDispatchesECMPMembershipChanges` covers Loc-RIB membership-only dispatch.
- `TestSysRIBConsumesLocRIBECMPMembership` covers `Change.ECMP` reaching sysrib `BestChangeEntry.ECMPPaths`.
- `TestSysRIBReplaysLocRIBECMPMembership` covers startup replay of a pre-existing Loc-RIB ECMP PathGroup.
- `TestSysRIBOSPFLocRIBAdminDistanceArbitration` covers OSPF/static selection through Loc-RIB and sysrib.
- `TestOSPFTopologyRetainsDownActiveArea` and `TestOSPFOriginateDownActiveInterfaceKeepsEmptyArea` cover carrier-down area retention, link suppression, and stale Network-LSA flush while retaining passive/loopback stubs.

## Files

None recorded.
