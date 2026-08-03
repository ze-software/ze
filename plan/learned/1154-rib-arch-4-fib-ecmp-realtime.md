# 1154 -- rib-arch-4: BGP Multipath ECMP to the FIB in Realtime

## Context

BGP equal-cost multipath selection existed (`SelectMultipath`, bestpath.go) but its
siblings reached only the `show bgp rib best` display. The realtime best-change producer
(`checkBestPathChange`, rib_bestchange.go) called `SelectBest` and mirrored ONE
`locrib.Path` (single next-hop) into the shared Loc-RIB, so a BGP multipath best installed a
single kernel next-hop, never an ECMP group. rib-arch-4 delivers the full N-nexthop set so
the FIB installs BGP ECMP.

## Decisions

- **Populate the Loc-RIB, not the BGP best-change event.** The FIB consumes sysrib's Stream B
  (built from Loc-RIB Changes in the default in-process deployment, sysrib.go), NOT the BGP
  event bus; `BestChangeEntry.ECMPNextHops` is `json:"-"` and never reaches the FIB. So a fix at
  the BGP producer's event (design A) is dead.
- **Design C over design B.** BGP arbitrates ONE best across peers, so it inserts one Loc-RIB
  Path per prefix. Design B (one Path per next-hop with synthetic Instances, IS-IS style) is
  hazardous: non-ADD-PATH pathIDs all collide at 0, and it needs hot-path stale-Instance
  reconciliation. Design C attaches the equal-cost set to the single Path via a new
  `Path.ECMP []netip.Addr`; `siblingNextHops` returns it directly. No Instance scheme, no
  reconciliation; a shrinking set just carries a shorter `Path.ECMP` (the Loc-RIB replaces the
  single Path each change). Reuses the tested Change.ECMP -> ecmpNextHops -> ECMPPaths ->
  buildMultiPath machinery unchanged.
- **Fix the same-best short-circuit.** `checkBestPathChange`'s same-best test compares the best
  peer/next-hop/metric, NOT the sibling set, so it suppressed ECMP-membership changes. Both the
  short-circuit and the full path now call a shared `mirrorToLocRIB` closure; the Loc-RIB dedups
  a true no-op via Path.Equal.

## Consequences

- BGP multipath now installs a kernel ECMP entry via the same path IS-IS/OSPF use.
- `Path.ECMP` is a fourth carry-through field (like Labels/BackupNextHop): excluded from `key()`
  and `Equal`, so it never affects arbitration; membership-only changes route through insert()'s
  `ecmpChanged` branch.
- Touching the widely-imported `locrib` pulled its whole reverse-dep closure into
  `ze-lint-changed`, surfacing LATENT pre-existing breakage: 6 reactor test mocks missing
  `DrainPeerSync` (added to `ReactorLifecycle` earlier, mocks never updated) + 2 firewall lint
  issues. A core-package change forces this cleanup; budget for it.

## Gotchas

- `show rib` exposes `ecmp-paths`, so BGP ECMP -> sysrib is verifiable in a `.ci` WITHOUT QEMU;
  the kernel `buildMultiPath` install is already covered by IS-IS/OSPF route-install tests.
- `ecmp-paths` excludes the winner's own next-hop; the full set is `next-hop` + `ecmp-paths`.
- A latent broken test mock in a core-package's reverse-dep closure only fails to compile once a
  change touches that closure; it can sit green on main for a long time.

## Files

- `internal/core/rib/locrib/candidate.go` -- `Path.ECMP` field
- `internal/core/rib/locrib/manager.go` -- `siblingNextHops` returns `best.ECMP`
- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- SelectMultipath + `mirrorToLocRIB`
- `internal/core/rib/locrib/locrib_test.go`, `.../rib/rib_ecmp_test.go`, `test/plugin/fib-ecmp-realtime.ci`
