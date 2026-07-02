# 1040 - OSPF NBMA + Point-to-Multipoint network types (RFC 2328/5340)

## Context

Adds the NBMA and point-to-multipoint (PtMP) interface network types to both
address families, on top of the delivered broadcast + point-to-point base.
Built in an isolated worktree (base-only), integrated into main by diff+apply.

## Decisions

- NBMA is broadcast-like: it elects a DR/BDR (Waiting state, WaitTimer), originates a Network-LSA when DR, but Hellos are UNICAST to statically configured neighbours (no multicast). PtMP is point-to-point-like: no DR election, per-neighbour unicast Hellos, Router-LSA emits one Type-1 p2p link per Full neighbour plus a host-route stub (/32 IPv4, /128 LA-bit IPv6), and NO Network-LSA / NO subnet stub.
- Non-broadcast flooding: `floodExcept`/`FlushDelayedAcks` fan out one unicast per Flood-eligible neighbour on NBMA/PtMP (no DR-relay suppression), preserving the ext-1 opaque-capable-neighbour gate.
- Config: `network-type {nbma, point-to-multipoint}` + `poll-interval` + an `nbma-neighbor` list (v4 keyed by address, v6 by router-id + optional link-local) on both AF interface lists. NBMA fields behind a pointer sub-struct `NBMA *nbmaConfig` (accessors) to keep `interfaceConfig` under golangci's 160-byte rangeValCopy threshold.

## Consequences

- `types.MetricFromBytes` now accepts 0 on the wire (PtMP host routes are cost 0; FRR emits them); `NewMetric` config validation still rejects 0. A 0-cost link no longer fails the whole Router-LSA decode.
- Base broadcast + p2p interfaces are byte-for-byte unchanged (verified): the ISM Start switch, election gate (`!= broadcast && != NBMA`), WaitTimer gate, and mask check only widened additively.
- An NBMA interface MUST have >=1 configured neighbour (ErrNBMANoNeighbors) - with an empty list it unicasts to nobody and forms no adjacency; PtMP with an empty list is valid (multicast discovery).

## Gotchas

- RFC 2328 §9.5.1: an INELIGIBLE (Priority-0) NBMA neighbour receives periodic/poll Hellos ONLY when the local router is DR or BDR; otherwise only the one-shot Start Hello. The first cut polled every configured neighbour regardless of priority + local ISM state (O(n^2) poll traffic on a priority-0 mesh); gate the periodic/poll target on `n.Priority == 0 && state != DR && state != Backup`.
- The mask check widened to broadcast/NBMA/PtMP is RFC-2328-§10.5-strict (only p2p + virtual links are exempt); PtMP peers on differing subnets (a common Cisco/FRR pattern) would be rejected - a known interop consideration, kept strict per the RFC.
- v6 NBMA: a silent (never-heard) neighbour cannot be polled without a configured link-local (the link-local is otherwise learned from the neighbour's first Hello - a bootstrap chicken-and-egg).

## Files

- `internal/plugins/ospf/iface/{nbma,ism,iface}.go`, `neighbor/{neighbor,nsm}.go`, `lsdb/{flooding,origination}.go`, `origination_v6.go`, `origination_v6_link.go`, `types/metric.go`, `config.go`, `instance.go`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang`
- `test/ospf/ospf-{nbma-config,nbma,ptmp}.ci`, `test/ospfv3/ospfv3-{nbma-config,nbma,ptmp}.ci`, `test/interop/scenarios/{ospf,ospfv3}-{nbma,ptmp}-frr/`
- `docs/guide/ospf.md`, `docs/features.md`, `docs/guide/configuration.md`
