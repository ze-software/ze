# 1029 - OSPFv2 opaque-LSA framework (RFC 5250)

## Context

OSPFv2 shipped with opaque LSAs (types 9/10/11) recognised by the codec but
inert: the codec already round-tripped opaque bodies verbatim, the types leaf
classified them (`IsOpaque()`), and `OptionO` existed, but there was no active
carrier. ext-1 added the carrier the four consumers (ext-2 TE, ext-3 Router
Information, ext-4 Extended-Link/Prefix, ext-9 Grace-LSA) plug into: scope-correct
flooding, the Link State ID Opaque Type/ID split, O-bit DD negotiation, generic
4-byte-aligned TLV helpers, and a consumer registry. The carrier interprets NO
opaque body and opaque LSAs never become SPF vertices (RFC 5250 §3).

## Decisions

- Modelled the carrier as a process-global registration API (`registerOpaqueConsumer(opaqueType, scope, onOriginate, onReceive)`, `opaque_registry.go`) populated at consumer `init()`, discovered by the engine at startup, over a per-engine registry, because opaque types are owned globally (RFC 5250 §9) and consumers stay self-contained. The function is UNEXPORTED on purpose: every consumer lives in the same package, so an exported name would invite an out-of-package registration the carrier is not designed for.
- The Link State ID split is a codec-layer pair, `OpaqueType()` / `OpaqueID()` on `LSA` (`packet/lsa_opaque.go`), not a field on the types leaf.
- Origination is a pull model: `OnOriginate(router) []OpaqueOrigination` returns the full desired set each self-LSA pass; an unchanged return floods nothing (idempotent), reusing `OriginateSelf`/`OriginateLinkSelf` sequencing rather than a new origination path.
- Reused the three existing LSDB stores by scope: Type 9 -> link store (broaden `isLinkLSAType`), Type 10 -> per-area store, Type 11 -> a NEW AS-wide opaque store parallel to `asExternal`. No new LSDB key type; the Opaque Type/ID split lives at the codec layer, not the types leaf.
- The O-bit is a DD-only signal (not part of the Hello E/N match), so adjacency with non-opaque peers is unaffected (RFC 5250 §3.1).
- §5 Type-11 reachability reads the SPF route table (`spf.RouterReachable`), reusing the Type-5 ASBR reachability; no separate tracker. Types 9/10 are always reachable.

## Consequences

- ext-2/3/4/9 register their Opaque Type and own their TLV bodies; the carrier names no consumer. Removing a consumer removes its `RegisterOpaqueConsumer` call and all its behaviour.
- Any new AS-wide opaque store MUST be added to BOTH `aging.go` `Tick` and `RefreshSelf`, or Type-11 self-LSAs never refresh/purge.
- Surfacing per-neighbour opaque capability to flooding is NEW plumbing (`Neighbor.Options -> FloodNeighbor.OpaqueCapable -> NeighborInfo.OpaqueCapable`), not a pure read (assumption A-4 was wrong).
- The five `ze_ospf_opaque_*` metric series are owned by this component (`opaque.go`), not by ospf-13.

## Gotchas

- Assumption A-4 ("the O-bit gate is a read, not new plumbing") was BROKEN: the JSON `Snapshot` and the flooding `FloodNeighbor`/`FloodNeighbors` did not carry Options; only the internal `Neighbor` struct did. A field had to be threaded through.
- Before ext-1, Type-11 opaque LSAs misrouted to the per-area store because `ASExternal()` returns false for Type 11; scope routing must be explicit in `dbForLocked`.
- Injected LSP/harness diagnostics reported stale `undefined method` and `too many arguments` errors from a mid-edit snapshot; `go vet` + `go test` on the final tree were authoritative and clean (same lesson as 972).
- `shouldDropByArea` returned false for opaque types, so Type-11 was not dropped in stub/NSSA until the §3.1 rule was added.

## Files

- `internal/plugins/ospf/opaque_registry.go`, `internal/plugins/ospf/opaque.go`
- `internal/plugins/ospf/packet/opaque_tlv.go`, `internal/plugins/ospf/packet/lsa_opaque.go`
- `internal/plugins/ospf/lsdb/opaque_as.go`, `internal/plugins/ospf/lsdb/{lsdb,link_scope,flooding,origination,aging}.go`
- `internal/plugins/ospf/neighbor/{neighbor,table,dd}.go`, `internal/plugins/ospf/instance.go`, `internal/plugins/ospf/config.go`
- `internal/plugins/ospf/{register,cmd_show,show_database}.go`, `internal/plugins/ospf/spf/computer.go`
- `internal/plugins/ospf/yang/ze-ospf-{conf,cmd}.yang`
- `test/ospf/ospf-opaque-*.ci`, `test/interop/scenarios/ospf-opaque-frr/`
- `docs/guide/ospf.md`, `docs/guide/command-reference.md`, `docs/architecture/wire/ospf.md`, `rfc/short/rfc5250.md`
