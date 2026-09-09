# OSPF interface state machine

`internal/plugins/ospf/iface` is the per-interface runtime: Hello timers, DR and
BDR election, passive and loopback records, interface snapshots and OSPF events.

## Decisions

- **The ISM is its own package, not part of the engine.** The IS-IS circuit
  pattern keeps per-interface runtime state next to its timers, snapshots and
  event sinks.
  <!-- source: internal/plugins/ospf/iface/iface.go -- Interface -->
  <!-- source: internal/plugins/ospf/iface/ism.go -- ism -->
- **The ISM keeps its own neighbour records, separate from the NSM.** RFC 2328
  DR election needs only heard, 2-Way, priority, declared DR and declared BDR.
  <!-- source: internal/plugins/ospf/iface/election.go -- electDRBDR, chooseDeclaredDR -->
- **Hello DR and BDR fields carry interface ADDRESSES, not Router IDs.** The RFC
  Hello fields identify the interface address on the attached network.
- **The ISM publishes through event-sink callbacks.** A direct import of the
  LSDB or the NSM would create a cycle inside the plugin.
- **Passive and loopback interfaces get records with no raw socket.**
  Router-LSA generation still needs the stub-link interface inventory.

## Constraints on callers

- A config reload recreates an interface runtime when either half of
  `reconcile`'s restart test fires, and the two halves ask different questions.
  `interfaceGlobalParamsChanged` reads the config OUTSIDE the interface's own
  block, and there the router id and the area type are the whole set, because
  they are what the runtime stamps into a packet: otherwise Hellos advertise a
  stale E-bit, N-bit or identity. `interfaceParamsEqual` reads the interface's
  own block, and a change to any field it compares recreates the runtime, the
  `cost` leaf included. Recreating a runtime empties its neighbor map, clears
  its DR and its BDR, and reports the interface down.
  <!-- source: internal/plugins/ospf/instance.go -- reconcile, interfaceParamsEqual, interfaceGlobalParamsChanged -->
- A `reference-bandwidth` change is NOT in either set. The cost reaches the wire
  through the engine's origination topology rather than through the runtime, so
  a re-priced interface takes its new cost through `SetCost` and keeps its
  neighbors. `Cost` is the one `Config` field no running behavior reads: the two
  snapshots are its only readers in this package. The interface's OWN `cost`
  leaf still recreates the runtime, because `interfaceParamsEqual` compares
  `Cost` and `HasCost`. The owner ruled on the router-wide leaf on 2026-09-09
  and the per-interface leaf was left as it was.
  <!-- source: internal/plugins/ospf/iface/iface.go -- Interface.SetCost, snapshotLocked, DetailSnapshot -->
  <!-- source: internal/plugins/ospf/instance.go -- repriceInterfaceLocked -->
- BackupSeen requires a 2-Way Hello before it shortens the Wait timer. A one-way
  Hello otherwise triggers a premature DR election.
- Neighbour inactivity scheduling uses the exact next `LastSeen` plus
  `RouterDeadInterval` deadline. A coarse dead-interval ticker is wrong.
- A priority-zero broadcast interface still sends and hears Hellos. It starts
  DROther and never becomes DR or BDR.

## Traps

- Link-down keeps the configured interface record and marks its runtime Down.
  Deleting the record loses the state needed when the link returns.
- A test that uses Router IDs only misses that the Hello DR and BDR fields are
  interface addresses. Keep the source address and the router id distinct in
  OSPF tests.
