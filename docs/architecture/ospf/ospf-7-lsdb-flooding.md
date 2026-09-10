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
- **The engine maintenance worker retries self-origination every second.**
  It starts for any enrolled interface, including passive and loopback
  interfaces, and reads the current topology on each pass. Unchanged bodies
  do not flood, and MinLSInterval defers changed bodies until a later pass.
  <!-- source: internal/plugins/ospf/instance.go -- openInterfaces, openConfiguredInterface, startNeighborRetransmitLoop -->
  <!-- source: internal/plugins/ospf/lsdb/origination.go -- OriginateRouter, OriginateNetwork -->
- **Cost-only reloads preserve adjacency and regenerate self-LSA metrics.**
  Adding, changing, or removing an explicit interface cost updates the enrolled
  topology and the CLI runtime cost in place, as a reference-bandwidth reload
  does. Both address families use that topology, including passive and loopback
  interfaces. MinLSInterval still bounds publication, and the maintenance
  worker retries any deferred change.
  <!-- source: internal/plugins/ospf/instance.go -- reconcile, repriceInterfaceLocked, lsdbTopology, originateSelfLSAs -->

## Constraints on callers

- Engine dispatch gates LS Update and LS Ack by neighbor state before the LSDB
  processes them. Test at the dispatcher level, not only at package level.
- Type 5 cleanup is AS-wide. A newer AS-external LSA clears retransmits for that
  key in every normal area, and a Type 5 purge is retained until every relevant
  area has acknowledged it.
- Stub and NSSA Type 5 filtering runs on receive AND on summary and lookup
  visibility, so the DD and LS Request paths cannot leak AS-external LSAs into
  those areas.
- Interface-down callbacks MUST enqueue deferred origination because their
  caller can hold the engine lock. Physical and virtual interfaces share one
  capacity-one notification channel, and a full channel coalesces the request.
  The maintenance worker consumes notifications outside that lock.
  <!-- source: internal/plugins/ospf/instance.go -- originateSelfLSAsDeferred, startNeighborRetransmitLoop, startInterfaceLocked -->
  <!-- source: internal/plugins/ospf/virtual_link.go -- startVirtualInterface -->
- Lazy maintenance registration and shutdown cancellation share `spawnMu`.
  Nested locking MUST take `mu` before `spawnMu`. Shutdown MUST release
  `spawnMu` before taking `mu` or waiting for the worker. Cancellation prevents
  new registration, and shutdown joins any origination already active in the
  worker.
  <!-- source: internal/plugins/ospf/instance.go -- startNeighborRetransmitLoop, originateSelfLSAs, shutdown -->

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
- An immediate origination attempt does not guarantee publication. RFC 2328
  Section 12.4 permits MinLSInterval to delay the next instance. The maintenance
  ticker remains the retry source after a coalesced notification is consumed.
  <!-- source: internal/plugins/ospf/instance.go -- reconcile, startNeighborRetransmitLoop -->
- A Type 5 area label on a retransmit entry is TRANSMISSION scope, not database
  scope. Deleting or replacing the AS-wide LSA considers every normal area.
- Any store that the aging tick walks must also be walked by the self-refresh
  pass. The two iterate the same set, or self-LSAs expire silently.
  <!-- source: internal/plugins/ospf/lsdb/aging.go -- Tick, RefreshSelf -->
