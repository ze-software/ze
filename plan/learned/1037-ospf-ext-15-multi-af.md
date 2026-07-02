# 1037 - OSPFv3 Multiple Address Families (RFC 5838)

## Context

Map address families to OSPFv3 Instance-ID ranges (RFC 5838 §2.1: ipv6-unicast
0-31, ipv6-multicast 32-63, ipv4-unicast 64-95, ipv4-multicast 96-127) and spawn
one unified-engine instance per configured AF, each with its own LSDB / topology
/ route table, without re-opening the RFC 5340 codec. Adds the AF-bit (Options
0x000100) to Hello/DD for adjacency gating, and IPv4-over-OSPFv3 (advertise IPv4
routes as OSPFv3 AS-External LSAs).

## Decisions

- One `v6EngineSet` (`register_multiaf.go`) owning one `*engine` per AF, replacing the single v6 engine (`eng6`); each engine gets `setMetrics`/`setConfig` per spawn, deduped by name under an `af=v3` label.
- `newEngineWithCodec` renamed to `newEngineWithCodecAF(t, codec, af)`; the engine carries `af addressFamily` + `multiAF atomic.Bool`. `newEngine(t)` delegates with `afIPv4Unicast`.
- `spf.NewInstallerFamily(loc, fam)` routes each AF's routes to the correct RIB family; `ze_ospf_routes_installed` gains an `af` label.
- The AF-bit is set only in Hello and DD (RFC 5838 §2.4), and gating is checked in BOTH (a Hello-only gate lets a peer that sets it in Hello but not DD reach Full on a non-default AF). Default AF (IPv6-unicast) does NOT require the bit (backward compat).

## Consequences

- Future per-AF features attach to the v6EngineSet, not a single v6 engine; ext-16 (IPsec) had to hook its installer into the per-v6-engine lifecycle, not a single eng6.
- The IPv4-over-OSPFv3 redistribution injector must NOT unconditionally divert IPv4 routes: with no ipv4-unicast AF configured, the divert target engine is absent and redistribution silently no-ops.
- `v6PrefixToNetip` and the forwarding-address decode are AF-aware (4-byte for IPv4 AFs, 16-byte for IPv6).

## Gotchas

- REGRESSION found in review: `SetV4OverV3Injector` wired unconditionally made `injectorFor(IPv4Unicast)` always return the v4-over-v3 wrapper, never falling back to the OSPFv2 engine; with no ipv4-unicast AF, OSPFv2 IPv4 redistribution (connected/BGP/static -> Type-5) silently DROPPED. Fixed with an `OptionalInjector.Active()` interface: the wrapper reports whether its AF engine exists, and `injectorFor` falls back to `c.inj` when inactive (evaluated per-inject, correct across runtime AF add/remove). Regression test `TestRedistFallsBackToOSPFv2WhenNoV4AF`.
- Integration into main (already carrying ext-1/2/3/12) was a 5-way reconcile of instance.go/register.go/config.go/cmd_show.go/yang; the full OSPF test suite passing is the proof no prior spec's block was dropped. The `newEngineWithCodec`->`newEngineWithCodecAF` rename had to fold in ext-12's mInstanceMismatch/onInstanceMismatch init AND ext-15's af/multiAF init.
- The new `ze-show:ospf-ipv6` wire method needs `make generate` to land in `wire-methods.snapshot` + `all.go`; deferred to commit time (generated files are cross-session dirty).
- Injected diagnostics reported stale `v6PrefixToNetip` arg-count errors mid-integration; go vet + go test on the final tree were authoritative and clean.

## Files

- `internal/plugins/ospf/multiaf.go`, `register_multiaf.go`, `instance_snapshots.go` (+ multiaf tests)
- `internal/plugins/ospf/{instance,register,config,cmd_show,dispatcher}.go`, `afstrategy_v6.go`, `codec_v6.go`, `encoder_v6.go`, `origination_v6*.go`, `redistribute/consumer.go`, `redistribute.go`
- `internal/plugins/ospf/spf/install.go`, `spf_wiring.go`, `v3/types/{options,hello,dbdesc}.go`
- `internal/plugins/ospf/yang/ze-ospf-{conf,cmd}.yang`
- `test/ospf/ospf-multiaf-*.ci`, `test/interop/scenarios/ospf-multiaf-{frr,v4-frr}/`
- `rfc/short/rfc5838.md`, `docs/{guide/ospf,architecture/wire/ospfv3,plugin-development/metrics}.md`
