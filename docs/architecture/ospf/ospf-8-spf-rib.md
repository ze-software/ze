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
  `Instance`.** `locrib.Path` has no route-type field, so OSPF resolves
  intra-area preference BEFORE insertion.
  <!-- source: internal/plugins/ospf/spf/route.go -- BuildRoutes -->

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
