# 960 -- OSPF 6 Neighbor NSM

## Context
OSPFv2 had interface Hellos and DR election from spec-ospf-5, but no Neighbor State Machine to turn 2-Way neighbours into Full adjacencies. The goal was to add the RFC 2328 §10 NSM, Database Description exchange, LS Request drain, adjacency events, metrics, and snapshot data while keeping OSPFv2 structurally aligned with IS-IS. The implementation stays in `internal/plugins/ospf/neighbor/`, with OSPFv3 kept separate for its later package tree.

## Decisions
- Chose ISM-driven NSM events over NSM Hello parsing because RFC 2328 splits §9 interface/election logic from §10 neighbour exchange.
- Chose an area-scoped LSDB interface over a concrete LSDB import because spec-ospf-7 owns storage, flooding, and install policy.
- Chose nil-LSDB no-request mode over request-everything because pre-LSDB Exchange would otherwise enter Loading and retransmit LSReqs forever.
- Chose OSPF payload budget (`InterfaceMTU - IPv4HeaderLen`) over full interface MTU for DD, LSReq, and LSUpdate chunks because raw IPv4 send adds the IP header.
- Chose dispatcher-backed adjacency fixtures over direct table calls because packet-handler features must prove registration, checksum/decode, area/ifindex lookup, and handler routing.
- Chose admission-time Down reaping over immediate deletion because snapshots keep recent Down state without letting churn permanently exhaust `maxNeighbors`.

## Consequences
- spec-ospf-7 must call `SetLSDB` with `Lookup(area,key)`, `LookupLSA(area,key)`, `Install(area,lsa)`, and `Summary(area)` before LS Request synchronization can request real LSAs.
- Future packet-handler tests should feed encoded packets through the dispatcher, not only call package handlers directly.
- OSPF packet chunking must budget for the transport envelope when the sender owns payload bytes but not the IP header.
- OSPFv2 now mirrors IS-IS shape (`types`, `packet`, `iface`, `neighbor`, later `lsdb`, `spf`) while keeping v2 and v3 packages separate.
- CLI `show ospf neighbor` and FRR interop remain spec-ospf-13 work; this child provides the snapshot API and dispatcher-backed runtime fixture.

## Gotchas
- Treating nil LSDB as "request every header" is a Loading deadlock, not a harmless placeholder.
- The DD Interface MTU field still advertises the full interface MTU even though chunk capacities must subtract the IPv4 header.
- A functional `.ci` that delegates to a Go fixture can still miss packet dispatch if that fixture calls `HandleDBDesc` directly.
- Down neighbours retained for operator snapshots still count in the map unless reaped before admission.
- DD ExStart slave-side negotiation must require I+M+MS, not just I+MS.

## Files
- `internal/plugins/ospf/neighbor/{doc.go,neighbor.go,nsm.go,table.go,dd.go,lsreq.go,nsm_test.go}`
- `internal/plugins/ospf/iface/iface.go`
- `internal/plugins/ospf/instance.go`
- `internal/plugins/ospf/events.go`
- `internal/plugins/ospf/adjacency_full_test.go`
- `test/ospf/ospf-neighbor.ci`
- `docs/plugin-development/metrics.md`
- `docs/functional-tests.md`
- `docs/DESIGN.md`
- `docs/architecture/core-design.md`
- `docs/guide/plugins.md`
- `docs/guide/configuration.md`
- `docs/architecture/config/syntax.md`
