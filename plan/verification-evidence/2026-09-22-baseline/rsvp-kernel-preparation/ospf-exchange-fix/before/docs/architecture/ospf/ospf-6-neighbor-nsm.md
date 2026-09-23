# OSPF neighbor state machine

`internal/plugins/ospf/neighbor` turns a 2-Way neighbour into a Full adjacency:
the RFC 2328 Section 10 NSM, the Database Description exchange, the LS Request
drain, adjacency events and snapshots.

## Decisions

- **The ISM drives NSM events. The NSM does not parse Hellos.** RFC 2328 splits
  the Section 9 interface and election logic from the Section 10 neighbour
  exchange.
  <!-- source: internal/plugins/ospf/neighbor/nsm.go -- nsm -->
- **The NSM sees the LSDB through an area-scoped interface, not a concrete
  import.** The `lsdb` package owns storage, flooding and install policy.
  <!-- source: internal/plugins/ospf/neighbor/table.go -- SetLSDB -->
- **A nil LSDB means request nothing.** Treating nil as "request every header"
  is a Loading deadlock: Exchange enters Loading and retransmits LS Requests
  forever.
- **Chunking budgets the OSPF payload, `InterfaceMTU - IPv4HeaderLen`, not the
  full interface MTU.** The raw IPv4 send adds the IP header. The DD Interface
  MTU field still advertises the full interface MTU.
  <!-- source: internal/plugins/ospf/neighbor/dd.go -- ddHeaderCapacity -->
  <!-- source: internal/plugins/ospf/neighbor/lsreq.go -- lsReqEntryCapacity, lsUpdateBodyCapacity -->
- **Down neighbours are reaped at admission, not deleted at once.** Snapshots
  keep recent Down state, and churn cannot permanently exhaust `maxNeighbors`.
  <!-- source: internal/plugins/ospf/neighbor/table.go -- Table -->

## Constraints on callers

- The LSDB owner calls `SetLSDB` with lookup, install and summary access before
  LS Request synchronization can request real LSAs.
- A packet-handler test feeds encoded packets through the dispatcher. A test
  that calls `HandleDBDesc` directly proves no registration, checksum, decode,
  area or ifindex lookup.

## Traps

- DD ExStart slave-side negotiation requires I, M and MS together. Requiring I
  and MS alone accepts a packet it must reject.
- A functional test that delegates to a Go fixture still misses packet dispatch
  when the fixture calls the handler directly.
