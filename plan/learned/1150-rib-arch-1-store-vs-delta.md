# 1150 -- rib-arch-1: Central Per-Protocol RIB Store vs Event-Bus Delta Model

## Context

The BGP RIB publishes best-path changes as `BestChangeBatch` deltas on the event bus
(`internal/component/bgp/plugins/rib/events/events.go`, emitted via
`rib_bestchange.go`; bridged to generic redistribution by `EmitBestChange`,
`internal/component/bgp/redistribute/producer.go`). Each consumer rebuilds its own
view from the delta stream. The open design question (rib-arch-1) was whether to replace
this with an engine-owned central per-protocol store consumers query directly. Triage
recorded the move trigger as "a second consumer beyond bgp-redistribute makes the delta
model painful." This spec was the DECISION, not speculative construction.

## Decisions

- **Keep the event-bus delta model; do not build a central per-protocol store.** Chosen
  over an engine-owned store because a store would not remove the expensive work and is
  not a cleaner abstraction — see Consequences.
- The second-consumer trigger DID fire: `flowexport`'s `bgpEnrichBuilder`
  (`internal/plugins/flowexport/enrichbgp.go,107`) is now a genuine second production
  consumer that accumulates its own `map[netip.Prefix]enrich.ASEntry` and rebuilds a
  prefix→AS radix tree. Assumption A-1 ("bgp-redistribute is the only delta consumer") is
  therefore **broken**. The decision overrides the naive "trigger fired → build store"
  reading with the reasons below.
- Rejected building the store because: (1) flowexport's real cost is its O(N) radix
  rebuild, which a store snapshot cannot remove; (2) a store's payload equals
  `BestChangeEntry` (it must carry BGP-specific OriginAS/ASPath); (3) the arbitrated
  Loc-RIB (`internal/core/rib/locrib`, consumed by sysrib) is ALREADY the engine-owned
  central store for consumers wanting a protocol-agnostic materialized view; (4) the two
  delta consumers want different query shapes, so one store fits neither.

## Consequences

- Consumers needing a protocol-agnostic materialized best-path view should subscribe to
  the arbitrated Loc-RIB / sysrib Stream B, NOT rebuild from BGP-RIB Stream A.
- Consumers needing BGP-specific attributes (AS_PATH, communities) legitimately consume
  the BGP-RIB `BestChangeBatch` delta stream; the arbitrated Loc-RIB does not carry them.
- **Revised revisit trigger:** build a central per-protocol store only when ≥2 consumers
  need the SAME materialized query shape over BGP-specific attributes (e.g. both want a
  point-queryable "current best-path set with AS_PATH"). The 2026-07-08 "any second
  consumer" trigger is superseded by this sharper one.
- The `BestChangeBatch` JSON wire contract for forked plugins stays the single contract;
  no store query API is introduced.

## Gotchas

- There are TWO best-change streams with identical type names: `ribevents.BestChangeBatch`
  (BGP RIB, Stream A, `internal/component/bgp/plugins/rib/events`) and
  `sysribevents.BestChangeBatch` (arbitrated system RIB, Stream B,
  `internal/component/sysrib/events`). The FIB plugins consume Stream B, not Stream A.
  Conflating them is the main analysis trap.
- `BestChangeEntry.ECMPNextHops` and `BackupNextHop` are `json:"-"` in-process hints set
  only on the sysrib/Loc-RIB path (`sysrib.go/1095`), never on the BGP-RIB producer
  path (`rib_bestchange.go` sets a single scalar next-hop). Do not assume the
  cross-process delta carries ECMP.

## Files

- `plan/spec-rib-arch-1-protorib-store.md` (decision recorded, then closed)
- No production code changed.
