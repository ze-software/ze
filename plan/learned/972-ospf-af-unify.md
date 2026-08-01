# 972 - OSPFv3 unified OSPF engine and redistribution follow-up

## Context

OSPFv3 started as a separate-plugin design, but the user chose one `ospf` engine with address-family-aware seams. The mature OSPFv2 engine already owned FSM, LSDB, flooding, SPF, CLI, and config, while OSPFv3 needed IPv6 wire/transport handling and a different prefix model. Completion required FRR interop across adjacency, route install, broadcast, Link-LSA, NSSA, and BGP-sourced v6 redistribution. The hardest failures were not pure encoders: link-scope LSAs, Loading request drains, and generic redistribution wiring all had to be correct.

## Decisions

- Chose one engine with Transport, Codec, and AFPrefixStrategy seams over a separate `ospfv3` engine because FSM, flooding, DR election, and SPF are already address-family-neutral.
- Chose `ospfv3/{types,packet,transport}` as leaves consumed by `ospf` over a shared package because the import guard and one-way dependency preserve module tiers.
- Chose `netip.Addr` neighbor reachability plus a strategy-owned next-hop source over v4 `[4]byte` link data because OSPFv3 next-hop is the neighbor link-local per interface.
- Chose `lsdb.OriginateSelf` with caller-provided v6 wire bytes over embedding the OSPFv3 codec in LSDB because sequencing and flooding stay shared while wire format stays AF-specific.
- Chose real GoBGP peering for v6 redistribution interop over fake source registration because the goal was producer to BGP RIB to OSPFv3 to FRR.

## Consequences

- Future OSPFv3 work belongs in engine strategies, link-scope LSDB handling, or `ospfv3` codec/transport leaves; do not create a second OSPFv3 engine.
- Scope-typed OSPFv3 LS Types must be classified through helpers such as `ASExternal`, `NSSA`, and `InterAreaRouter`, not OSPFv2 numeric constants.
- Link-LSAs are interface-scoped: DD summary, LS Request lookup, ack, aging, refresh, release, and snapshots all need the arrival interface.
- BGP redistribution source registration must happen at init, and route-change production must use the generic `redistevents` path.
- A final-adjacency failure needs DD/LSReq drain and LSDB acceptance checks before changing Router-LSA topology.

## Gotchas

- FRR route absence looked like Router-LSA reachability, but advertising Exchange/Loading neighbors was wrong; Link-LSAs had to participate in database exchange so Loading could reach Full.
- OSPFv3 AS-External LSAs first landed in per-area storage because code compared `key.Type == 5`; scope-aware type helpers are required anywhere storage or flood scope matters.
- `import bgp` rejected `ebgp` routes until `ImportRule.Accept` allowed umbrella-origin matches while preserving loop prevention on `route.Origin`.
- BGP source registration after peer parsing is too late for config validation; BGP redistribution sources must register from package init.
- Injected LSP diagnostics reported stale undefined symbols during this work; grounded `go test` results were authoritative.
- A doc-test failure can surface as a Go compile error in a package imported by docs tooling, not as prose drift.

## Files

- `internal/plugins/ospf/instance.go`
- `internal/plugins/ospf/codec*.go`, `encoder_v6.go`, `afstrategy_v6*.go`
- `internal/plugins/ospf/origination_v6*.go`, `internal/plugins/ospf/nssa.go`, `internal/plugins/ospf/redistribute/consumer.go`
- `internal/plugins/ospf/lsdb/*.go`, `internal/plugins/ospf/neighbor/*.go`, `internal/plugins/ospf/types/lstype.go`
- `internal/plugins/ospf/v3/{types,packet,transport}/`
- `internal/component/bgp/redistribute/`, `internal/component/bgp/plugins/rib/rib_bestchange.go`, `internal/component/config/redistribute/route.go`
- `test/interop/scenarios/ospf-v6-*`, `test/interop/interop.py`, `test/interop/Dockerfile.ze`, `test/interop/daemons`
- `docs/guide/ospf.md`, `docs/guide/configuration.md`, `docs/features.md`, `docs/architecture/wire/ospfv3.md`, `docs/architecture/core-design.md`
