# OSPF LSDB, origination and flooding

`internal/plugins/ospf/lsdb` owns raw-byte LSA storage, freshness comparison,
self-origination, flooding, retransmit, acknowledgement, MaxAge purge retention,
LSRefresh and MaxSequenceNumber restart (RFC 2328 Sections 12 to 14).

## Decisions

- **Self-LSAs are fully regenerated, never edited incrementally.** This is the
  IS-IS origination model. A link record change rebuilds the LSA body.
  <!-- source: internal/plugins/ospf/lsdb/origination.go -- OriginateRouter, OriginateNetwork -->
- **Retransmit lists live in `lsdb`, not in `neighbor`.** Flooding policy, ack
  policy, purge retention and Type 5 AS-wide scope need one owner.
  <!-- source: internal/plugins/ospf/lsdb/flooding.go -- ReceiveUpdate, ReceiveAck, RetransmitTick -->
- **The neighbor Loading path drains an LS Request entry only after the LSDB
  confirms it accepted an equal or newer instance.** Otherwise Loading bypasses
  the flooding receive policy.
- **Type 5 LSAs are stored once in an AS-wide store.** Area scoping is applied
  at the visibility and retransmit edges.
  <!-- source: internal/plugins/ospf/lsdb/lsdb.go -- LSDB, Install -->
- **Self-origination re-runs from the engine 1-second timer and skips unchanged
  bodies.** MinLSInterval then DEFERS a change instead of dropping it.

## Constraints on callers

- Engine dispatch gates LS Update and LS Ack by neighbor state before the LSDB
  processes them. Test at the dispatcher level, not only at package level.
- Type 5 cleanup is AS-wide. A newer AS-external LSA clears retransmits for that
  key in every normal area, and a Type 5 purge is retained until every relevant
  area has acknowledged it.
- Stub and NSSA Type 5 filtering runs on receive AND on summary and lookup
  visibility, so the DD and LS Request paths cannot leak AS-external LSAs into
  those areas.

## Traps

- Running neighbor Loading after an LSDB receive is safe only while the neighbor
  path never calls `Install`. A neighbor-side install reinserts LSAs that
  flooding rejected for checksum, MinLSArrival, unknown MaxAge or stub and NSSA
  Type 5 policy.
- MaxSequenceNumber wrap completes only when the acknowledged MaxAge purge
  deletes the entry and resets the own-sequence record to
  `InitialSequenceNumber`.
- Loss of the DR role flushes stale self Network-LSAs. Stopping origination is
  not enough.
- MinLSInterval needs a retry source. A change event inside the interval is
  retried by a timer after the interval expires.
- A Type 5 area label on a retransmit entry is TRANSMISSION scope, not database
  scope. Deleting or replacing the AS-wide LSA considers every normal area.
- Any store that the aging tick walks must also be walked by the self-refresh
  pass. The two iterate the same set, or self-LSAs expire silently.
  <!-- source: internal/plugins/ospf/lsdb/aging.go -- Tick, RefreshSelf -->
