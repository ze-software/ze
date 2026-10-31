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
- **IS-IS -> BGP (producer, AC-1/AC-2/AC-7):** FRR advertises its own loopback
  10.98.0.1/32 into IS-IS. Ze learns it over the adjacency, redistributes it into
  BGP (`redistribute { destination bgp { import isis } }`), and gobgp, Ze's eBGP
  peer, must hold it in its RIB. The BGP peer is a third router on purpose: the
  route's IGP next hop is FRR, and RFC 4271 Section 5.1.3 withholds a route from
  the peer that is its own next hop, so an FRR BGP peer could never receive it.

## Topology

```
  FRR isisd  <--- IS-IS (L2, AF_PACKET) --->  Ze  <--- eBGP --->  gobgp
   (router isis, area 49.0001,                 (isis + redistribute)  (AS 65002)
    lo 10.98.0.1/32 passive)
```

## Files

- `ze.conf`   -- Ze: `isis { ... }` over the L2 link to FRR, a connected/static
  prefix, an eBGP peer (gobgp, 172.30.0.5), and `redistribute { destination bgp
  { import isis } destination isis { import connected static bgp } }`.
- `frr.conf`  -- FRR: `router isis` on the shared link, same area, so it forms an
  adjacency with Ze and learns the redistributed reachability, and a passive
  `lo` carrying 10.98.0.1/32 for Ze to learn.
- `gobgp.toml` -- gobgp: the eBGP peer that receives the IS-IS route Ze exports.
- `internal/le/interoplab/bgp/check_engine.go` `checkScenario` executes the
  `scenarioOperations` and `scenarioExtras` registered for `isis-redist-frr`:
  the adjacency is up in FRR, FRR installs Ze's static 10.99.0.0/24 via IS-IS,
  and gobgp's RIB holds FRR's 10.98.0.1/32.

## Status

MANDATORY and not deferrable (spec-isis-0 umbrella "Test + interop wiring"). It
runs under the Linux Docker/QEMU interop harness only (raw L2 + FRR isisd); it
CANNOT run on darwin. It depends on the IS-IS interop harness extensions delivered
by isis-13 (an IS-IS-aware peer class with `wait_adjacency` / `has_isis_route`); the
config and assertions here are the contract that harness must satisfy.
