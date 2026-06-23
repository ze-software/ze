# 961 -- OSPF 7 LSDB Flooding

## Context
OSPFv2 had packet codecs, raw IPv4 transport, interface ISM, and neighbor NSM, but no LSDB, self-LSA origination, flooding, retransmit, ack, aging, or purge lifecycle. The goal was to implement RFC 2328 §12-14 under `internal/plugins/ospf/lsdb/`, keep OSPFv2 structurally aligned with IS-IS, and keep OSPFv3 separate.

## Decisions
- Chose the IS-IS-style full regeneration model for Router-LSA and Network-LSA origination instead of incremental link-record edits.
- Kept retransmit lists in `lsdb`, not `neighbor`, because flooding policy, ack policy, purge retention, and Type 5 AS-wide scope all need one owner.
- Made `neighbor.HandleLSUpdate` drain LS Request entries only after `lsdb.Lookup` confirms the LSDB accepted an equal-or-newer instance, preventing Loading from bypassing flooding receive policy.
- Stored Type 5 LSAs once in an AS-wide store, while area-scoping visibility and retransmit decisions at the edges.
- Re-ran self-origination from the engine's 1 s timer and skipped unchanged LSA bodies, so MinLSInterval defers changes instead of dropping them.

## Consequences
- `internal/plugins/ospf/lsdb` owns raw-byte LSA storage, freshness comparison, origination, flooding, retransmit, delayed/direct acks, MaxAge purge retention, LSRefresh, and MaxSequenceNumber restart.
- Engine dispatch must gate LS Update and LS Ack by neighbor state before LSDB processing. Tests should include dispatcher-level coverage, not only package-level calls.
- Type 5 cleanup is AS-wide: a newer AS-external LSA clears retransmits for the key in all normal areas, and a Type 5 purge is retained until every relevant area has acked.
- Stub/NSSA Type 5 filtering happens both on receive and on LSDB summary/lookup visibility, so DD/LSReq paths cannot leak AS-external LSAs into those areas.
- OSPFv3 must get its own `internal/plugins/ospfv3/lsdb` implementation. Do not share OSPFv2 wire, LSA, LSDB, auth, or SPF packages.

## Gotchas
- Running neighbor Loading after LSDB receive is safe only if the neighbor path never calls `Install`; otherwise it can reinsert LSAs that flooding rejected for checksum, MinLSArrival, unknown MaxAge, or stub/NSSA Type 5 policy.
- MaxSequenceNumber wrap is not complete until the acknowledged MaxAge purge deletes the entry and resets the own-sequence record to restart at InitialSequenceNumber.
- DR loss must flush stale self Network-LSAs, not just stop originating new ones.
- MinLSInterval needs a retry source. A change event inside the interval must be retried by a timer after the interval expires.
- Type 5 area labels on retransmit entries are transmission scope, not database scope. Deleting or replacing the AS-wide LSA must consider every normal area.

## Files
- `internal/plugins/ospf/lsdb/{entry.go,lsdb.go,origination.go,flooding.go,aging.go}`
- `internal/plugins/ospf/lsdb/{lsdb_test.go,origination_test.go,flooding_test.go,aging_test.go}`
- `internal/plugins/ospf/{instance.go,config.go,lsdb_flooding_test.go}`
- `internal/plugins/ospf/neighbor/{neighbor.go,table.go,lsreq.go,nsm_test.go}`
- `test/ospf/ospf-flooding.ci`
- `docs/architecture/wire/ospf.md`
- `docs/plugin-development/metrics.md`
- `docs/functional-tests.md`
- `docs/architecture/core-design.md`
- `docs/DESIGN.md`
- `docs/guide/configuration.md`
- `docs/guide/plugins.md`
