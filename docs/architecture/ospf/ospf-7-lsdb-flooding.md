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
- **Application validation precedes every LS Update disposition.** A malformed
  understood extension LSA is neither stored, acknowledged nor reflooded, even
  when self-originated or MaxAge. Valid companion LSAs continue through the
  update. Unknown opaque applications are not parsed as known extensions.
  <!-- source: internal/plugins/ospf/lsdb/flooding.go -- ReceiveUpdate -->
  <!-- source: internal/plugins/ospf/opaque.go -- wireOpaqueDelivery -->
- **Native link-scope extension LSAs share the interface lifetime.** E-Link
  and link-scope Router Information LSAs enter the link store, not the area
  store. Origination, purge and interface release notify readers outside the
  LSDB lock, including the asynchronous native BGP-LS snapshot publisher.
  <!-- source: internal/plugins/ospf/types/lstype.go -- LSType.LinkLocal -->
  <!-- source: internal/plugins/ospf/lsdb/link_scope.go -- installLinkOriginated, ReleaseLink -->
  <!-- source: internal/plugins/ospf/neighbor/lsreq.go -- lookupLSAHeaderLocked, lookupLSALocked -->
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
- Neighbor and interface callbacks MUST enqueue self-LSA origination. Physical
  and virtual interfaces share one capacity-one notification channel. The
  maintenance worker consumes a request before reading current topology, so
  changes during an active pass can queue another pass. A full channel
  coalesces requests without blocking receive processing on topology queries.
  Full-state transitions use the same queue. Interface-down callbacks can hold
  the engine lock, so they also MUST return without inline origination.
  <!-- source: internal/plugins/ospf/instance.go -- originateSelfLSAsDeferred, startNeighborRetransmitLoop, startInterfaceLocked -->
  <!-- source: internal/plugins/ospf/virtual_link.go -- startVirtualInterface -->
  <!-- source: internal/plugins/ospf/bfd_client.go -- neighborEventSinkValue -->
- SR Adj-SID Full/Down hooks use the same origination queue after the label
  lifecycle operation. The adjacency manager serializes allocation and
  withdrawal with OSPFv3 origination and TI-LFA label reads. An install emission
  completes before the label becomes visible to either address family's
  origination. Withdrawal clears the advertisement and completes its FIB
  emission before freeing the label for reuse.
  <!-- source: internal/plugins/ospf/sr_adjsid.go -- srAdjManager, srAdjNeighborFull, srAdjNeighborLost -->
  <!-- source: internal/plugins/ospf/sr_tilfa.go -- adjLabelForRouter -->
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
