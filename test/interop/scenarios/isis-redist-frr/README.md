# Scenario: isis-redist-frr (IS-IS redistribution interop with FRR isisd)

Owner spec: `plan/spec-isis-11-redistribution.md` (redistribution wiring). The
broader IS-IS <-> FRR interop harness (an IS-IS-aware peer class, `router isis`
config, LSDB/route assertions over an L2 veth) is owned by
`plan/spec-isis-13-cli-diag-interop.md`; this directory contributes the
redistribution scenario that isis-13's harness runs.

## What it proves

FRR `isisd` installs IS-IS reachability that Ze redistributed into IS-IS, and Ze
redistributes IS-IS routes out to BGP:

- **Connected/static/BGP -> IS-IS (consumer, AC-3/AC-4/AC-5/AC-6):** Ze imports a
  connected prefix (and a static and a BGP prefix) into its IS-IS LSPs as Extended
  IP Reachability (TLV 135). FRR, adjacent over the L2 link, must learn the prefix
  via IS-IS and install it (`show isis route` / `show ip route isis`). The up/down
  bit is honoured (0 on a same-level advertisement; 1 only on a down-level leak,
  RFC 2966); TLV 135 carries no external bit (RFC 5305 sec 4).
- **IS-IS -> BGP (producer, AC-1/AC-2/AC-7):** an IS-IS route Ze learns from FRR is
  redistributed into BGP (`redistribute { destination bgp { import isis } }`) and
  appears in a BGP peer's RIB; withdrawing the IS-IS route withdraws it from BGP.

## Topology

```
  FRR isisd  <--- IS-IS (L2, AF_PACKET) --->  Ze  <--- eBGP --->  BGP peer (FRR/gobgp)
   (router isis, area 49.0001)                 (isis + redistribute)
```

## Files

- `ze.conf`   -- Ze: `isis { ... }` over the L2 link to FRR, a connected/static
  prefix, an eBGP peer, and `redistribute { destination bgp { import isis }
  destination isis { import connected static bgp } }`.
- `frr.conf`  -- FRR: `router isis` on the shared link, same area, so it forms an
  adjacency with Ze and learns the redistributed reachability.
- `internal/le/interoplab/bgp/check_engine.go` `checkScenario` executes the
  `scenarioOperations` and `scenarioExtras` registered for `isis-redist-frr`;
  those assertions require FRR to install the redistributed prefixes and an
  IS-IS route to reach the BGP peer.

## Status

MANDATORY and not deferrable (spec-isis-0 umbrella "Test + interop wiring"). It
runs under the Linux Docker/QEMU interop harness only (raw L2 + FRR isisd); it
CANNOT run on darwin. It depends on the IS-IS interop harness extensions delivered
by isis-13 (an IS-IS-aware peer class with `wait_adjacency` / `has_isis_route`); the
config and assertions here are the contract that harness must satisfy.
