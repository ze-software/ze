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

- A config reload that changes the router id or an area type recreates or
  refreshes the runtimes. Otherwise Hellos advertise a stale E-bit, N-bit or
  identity.
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
