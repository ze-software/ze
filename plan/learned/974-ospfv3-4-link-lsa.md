# 974 -- ospfv3-4-link-lsa

## Context
OSPFv3 (RFC 5340) requires each router to originate a link-local-scoped Link-LSA
(type 0x0008) per attached link, carrying its IPv6 link-local address (neighbor
next-hop) and the link's configured prefixes; the Designated Router then aggregates
all attached routers' Link-LSA prefixes into a Network-referencing Intra-Area-Prefix-LSA
so the transit LAN subnet is advertised into the area exactly once. The unified OSPF
engine already formed v6 broadcast adjacencies, elected DR/BDR, and originated the
Network-LSA, so broadcast routing worked via the Router-referencing Intra-Area-Prefix-LSA,
but Link-LSAs were absent. The LSDB modeled only area and AS flooding scope; link-local
scope (flood on the originating link only, never re-flood) did not exist. The goal was
RFC-completeness: advertise the transit subnet the canonical OSPFv3 way and supply
neighbor link-local next-hops through the LSDB.

## Decisions
- Modeled link scope as a separate per-interface store (`links map[string]*areaDB` in
  lsdb.go) reusing `areaDB`'s entry/aging machinery, chosen over overloading the area
  store with a synthetic per-link area key because it keeps the v4/area flooding paths
  byte-identical and makes "flood on one link only" explicit.
- Reused the existing `packet.LinkLSA.WriteTo`/`DecodeLinkLSA` codec rather than writing
  a fresh encoder, since the wire code already existed and was buffer-first.
- Reused the existing Network-referenced-prefix route path (`afstrategy_v6.go`) instead
  of a Link-LSA-specific install path; the DR only needed to originate the aggregated LSA.
- DR aggregation excludes NU/LA prefix-option bits and link-local-unicast addresses, dedupes,
  and sorts; link-local addresses are never copied into the Intra-Area-Prefix-LSA (RFC §4.4.3.9).
- This is v6-only; OSPFv2 originates no Link-LSA and never touches the link store.

## Consequences
- The LSDB now has three flooding scopes (area, AS, link); link scope is additive and
  released on interface-down/reload, bounding memory by interface x neighbor count.
- Link-scope LSAs were also threaded through Database Description summaries and LS Request
  lookup, so OSPFv3 neighbors can drain the request list and reach Full (Loading no longer stalls).
- The default `show ospf database` (full snapshot) surfaces Link-LSAs with interface +
  link-local address; type-filtered subviews still omit them.

## Gotchas
- Pure origination/flooding was insufficient: without link-scoped DD summaries and LSReq
  lookup, FRR received the LSAs but Ze stayed in Loading with a non-empty request list. The
  fix was to carry the interface-scoped LSDB through summary/request lookup, NOT to advertise
  non-Full neighbors in Router-LSAs.
- `types.LSType.String()` did not name scope-typed OSPFv3 values (0x0008, 0x2007, 0x4005);
  database and metric output rendered raw values until stable semantic names were added.
- A 2-node shared-LAN interop CANNOT validate DR prefix aggregation (the subnet is connected
  on both routers, so neither learns it via OSPF); a faithful test needs a third router.
  No 3-node scenario was built; validation is unit + in-process, recorded as a spec-authorized
  Known Limitation.
- AC-8 has snapshot/API coverage only; no `test/ui/ospf-v6-link-database.ci` command test was
  added (spec-authorized fallback).

## Files
- `internal/plugins/ospf/lsdb/lsdb.go` -- `links` per-interface store, snapshot `Links`, metrics
- `internal/plugins/ospf/lsdb/link_scope.go` -- link-scope install/lookup/age/refresh/release/flood
- `internal/plugins/ospf/lsdb/flooding.go` -- link-scoped receive/ack on arrival interface, no propagation
- `internal/plugins/ospf/origination_v6_link.go` -- self Link-LSA origination + DR Network-referencing aggregation
- `internal/plugins/ospf/origination_v6.go` -- origination hook + stale self-LSA flush
- `internal/plugins/ospf/afstrategy_v6.go`, `instance.go` -- Network-referenced prefix install wiring
- `internal/plugins/ospf/show_database.go` + lsdb snapshot -- Link-LSA rendering
- Tests: `lsdb_linkscope_test.go`, `origination_v6_link_test.go`, `afstrategy_v6_test.go`,
  `show_database_test.go`, `internal/plugins/ospf/v3/packet/lsa_link_test.go`
