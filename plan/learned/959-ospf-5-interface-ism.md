# 959 -- OSPF 5 Interface ISM

## Context

OSPFv2 had raw transport and config plumbing, but no per-interface runtime to originate or consume Hellos. The ISM needed to match RFC 2328 while staying separate from the future Neighbor State Machine and LSDB work. The goal was a Ze-style per-interface worker, consistent with IS-IS circuit structure, that owns Hello timers, DR/BDR election, passive and loopback records, interface snapshots, and OSPF events.

## Decisions

- Chose `internal/plugins/ospf/iface` over folding ISM into the OSPF engine because the IS-IS circuit pattern keeps per-interface runtime state close to timers, snapshots, and event sinks.
- Chose separate ISM-level neighbour records over premature NSM state because RFC 2328 DR election only needs heard, 2-Way, priority, declared DR, and declared BDR.
- Chose OSPF interface addresses for Hello DR/BDR fields over Router IDs because RFC Hello fields identify the interface address on the attached network.
- Chose event-sink callbacks over direct LSDB or NSM imports because ospf-6 and ospf-7 must consume state without creating plugin-internal cycles.
- Chose passive and loopback records without raw sockets over skipping them because later Router-LSA generation still needs stub-link interface inventory.

## Consequences

- OSPFv2 and OSPFv3 should mirror the IS-IS per-interface runtime and snapshot style, but must not share wire, LSA, LSDB, auth, or SPF packages.
- A config reload that changes router ID or area type must recreate or refresh runtimes, otherwise Hellos advertise stale E/N-bit and identity state.
- BackupSeen must require a 2-Way Hello before shortening the Wait timer, otherwise one-way Hellos can trigger premature DR election.
- Neighbour inactivity scheduling must use the exact next LastSeen plus RouterDeadInterval deadline, not a coarse dead-interval ticker.
- Priority-zero broadcast interfaces still send and hear Hellos, but start DROther and never become DR or BDR.

## Gotchas

- `lsp rename` corrupted several same-package Go identifiers during a method rename; inspect rename diffs in this area before trusting language-server edits.
- Tests that only used Router IDs missed that Hello DR/BDR fields are interface addresses; keep source address and router ID distinct in future OSPF tests.
- Darwin cannot exercise the daemon runtime fixture because iface backend support is absent, so the functional runtime test has a Darwin skip and must run under Linux or QEMU later.
- Link-down should keep the configured interface record and mark its runtime Down; deleting the record loses state needed when the link comes back.

## Files

- `internal/plugins/ospf/iface/iface.go`
- `internal/plugins/ospf/iface/ism.go`
- `internal/plugins/ospf/iface/election.go`
- `internal/plugins/ospf/iface/iface_test.go`
- `internal/plugins/ospf/instance.go`
- `internal/plugins/ospf/instance_test.go`
- `internal/plugins/ospf/events.go`
- `internal/plugins/ospf/events_test.go`
- `internal/plugins/ospf/transport/transport.go`
- `internal/plugins/ospf/transport/metrics_test.go`
- `internal/plugins/ospf/config.go`
- `internal/plugins/ospf/config_test.go`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang`
- `test/ospf/ospf-interface.ci`
- `test/ospf/ospf-interface-runtime.ci`
- `docs/guide/configuration.md`
- `docs/plugin-development/metrics.md`
- `docs/functional-tests.md`
- `docs/DESIGN.md`
- `docs/architecture/core-design.md`
- `docs/guide/plugins.md`
- `docs/architecture/config/syntax.md`
