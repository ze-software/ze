# OSPF SPF and RIB install

`internal/plugins/ospf/spf` computes the shortest-path tree and the route table.
`spf_wiring.go` in the plugin root installs the result into the Loc-RIB.

## Decisions

- **The package mirrors the IS-IS structure, not its code**: `graph.go`,
  `spf.go`, `route.go`, `install.go`, `computer.go`, plus `spf_wiring.go` in the
  plugin root.
  <!-- source: internal/plugins/ospf/spf/computer.go -- Computer -->
  <!-- source: internal/plugins/ospf/spf_wiring.go -- initSPF, configureSPF -->
- **Route install is a Loc-RIB insertion only.** Redistribution stays in the
  redistribution events path and never touches the kernel.
  <!-- source: internal/plugins/ospf/spf/install.go -- Installer -->
- **One `locrib.Path` per equal-cost next-hop, each with a distinct
  `Instance`.** OSPF resolves intra-area preference before insertion. Resolved
  physical adjacencies retain their interface and `OnLink` flag, including ECMP
  siblings with the same IPv6 link-local address on different interfaces.
  <!-- source: internal/plugins/ospf/spf/route.go -- BuildRoutes -->
- **OSPFv3 resolves a first hop by adjacency, not address.** The native graph
  preserves each root link's local Interface ID. `NextHopSource` returns the
  address, physical interface and Router ID of the Full neighbor on that link.
  Parallel links to one router remain distinct; broadcast and LFA resolution use
  the same scoped source. An address-only resolver is used only for OSPFv2.
  <!-- source: internal/plugins/ospf/afstrategy_v6.go -- v6RouterLinks, v6NextHop -->
  <!-- source: internal/plugins/ospf/neighbor/table.go -- NextHopOnLink -->
- **Native reachability is one completed SPF view, not prefix ownership.**
  `Reachability()` captures immutable per-area trees and inter-area border
  candidates under one lock. A router with no advertised prefix can still be
  reachable. Area-scoped queries never borrow a path from another area;
  AS-scoped queries may use resolved inter-area candidates. OSPFv3 network
  identities are resolved from the real DR and Interface ID, not the graph's
  synthetic vertex ID.
  Known-origin queries distinguish an input absent from the completed run from a
  known unreachable origin. The native graph builders retain live source
  advertisers by flooding scope, including RI-only inputs, and exact decoded
  Network-LSA identities before graph-key selection. Inter-area and external
  calculation retain unresolved ASBR targets and AS-scope input advertisers.
  New LSAs do not become negative reachability evidence merely because they
  arrived after the captured native input.
  <!-- source: internal/plugins/ospf/spf/reachability.go -- ReachabilitySnapshot, Reachability -->
  `RouterReachable()` uses this view for Type-11 opaque processing too, so a
  prefixless reachable originator is accepted and removed areas invalidate it.
  <!-- source: internal/plugins/ospf/spf/computer.go -- RouterReachable -->
- **Publication rejects obsolete computations.** Monotonic run tokens keep an
  earlier-started calculation from replacing a later published result.
  Root, area, area-policy and virtual-link configuration changes invalidate
  readiness until a matching calculation publishes. Captured views remain
  immutable across later runs and configuration changes.
  <!-- source: internal/plugins/ospf/spf/computer.go -- Run, SetRoot, SetAreas, SetAreaConfigs -->
  <!-- source: internal/plugins/ospf/spf/transitarea.go -- SetVirtualLinks -->
- **BGP-LS is notified after every published SPF result.** The post-run hook
  retains Segment Routing installation first, then queues native BGP-LS
  publication. Reachability changes therefore propagate even when the IP
  route delta is empty.
  <!-- source: internal/plugins/ospf/spf_wiring.go -- initSPF -->

## Traps

- **A protocol that inserts a later equal-cost sibling needs a membership-only
  `ChangeUpdate`.** Loc-RIB ECMP expansion originally carried siblings only when
  the best path changed, so sysrib and fibkernel saw one next-hop.
- A membership-only `ChangeUpdate` must NOT carry the new sibling's
  `ForwardHandle` when the selected best path did not change. That handle
  belongs to the inserted sibling, not to the `Best` path in the event.
- Loc-RIB snapshot replay is a separate ECMP path from live mutation dispatch.
  sysrib carries `PathGroup.ECMPNextHops` into the synthetic add at startup, or
  routes installed before sysrib subscribes collapse to the primary next-hop.
- **Interface-down self-LSA refresh has two halves.** Origination is scheduled
  without deadlocking the engine stop path, and the down active interface stays
  in the topology long enough to preserve area membership. Origination, not the
  topology snapshot, suppresses the links of a down active interface and flushes
  its stale Network-LSA. Passive and loopback stubs stay advertised.
- `.ci` command parsing splits on whitespace and does not apply shell quoting. A
  quoted `-run` regex is passed literally. With `tmpfs` files the command runs
  from the tmpfs directory, so a repo-relative `go test ./...` fails.
